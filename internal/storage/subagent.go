package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zclaw/zclaw/internal/agents"
)

// SubAgentRepository implements SubAgentRegistry backed by SQLite.
type SubAgentRepository struct {
	db *DB
}

// NewSubAgentRepository creates a new SubAgentRepository and ensures tables exist.
func NewSubAgentRepository(db *DB) *SubAgentRepository {
	r := &SubAgentRepository{db: db}
	// Ensure schema exists. Best-effort; ignore error in constructor to avoid init-time hard failures.
	_ = r.ensureTables(context.Background())
	return r
}

// ensureTables creates required tables if they do not exist.
func (r *SubAgentRepository) ensureTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS subagents (
            id TEXT PRIMARY KEY,
            parent_id TEXT NOT NULL,
            child_agent_id TEXT,
            state TEXT NOT NULL,
            task_description TEXT,
            result TEXT,
            created_at TEXT NOT NULL,
            completed_at TEXT
        )`,
		`CREATE TABLE IF NOT EXISTS agent_templates (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            description TEXT NOT NULL DEFAULT '',
            config_json TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (datetime('now'))
        )`,
	}
	for _, s := range stmts {
		if _, err := r.db.sqlite.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	return nil
}

// Spawn creates a new sub-agent record in the database. The actual execution
// lifecycle is managed by the scheduler/runner components.
func (r *SubAgentRepository) Spawn(ctx context.Context, req agents.SubAgentRequest) (*agents.SubAgent, error) {
	// ID generation lives in storage/id.go (newID)
	id := newID()
	now := time.Now().UTC()

	// For a fresh spawn, set child_agent_id to NULL and state to spawned.
	_, err := r.db.sqlite.ExecContext(ctx,
		`INSERT INTO subagents (id, parent_id, child_agent_id, state, task_description, result, created_at, completed_at)
         VALUES (?, ?, NULL, ?, ?, NULL, ?, NULL)`,
		id, req.ParentID, string(agents.SubAgentStateSpawned), req.TaskDescription, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("spawn subagent: %w", err)
	}

	return r.Get(ctx, id)
}

// Get retrieves a sub-agent by its ID.
func (r *SubAgentRepository) Get(ctx context.Context, id string) (*agents.SubAgent, error) {
	var sa agents.SubAgent
	var parentID string
	var createdAt string
	var completedAt sql.NullTime

	row := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id, parent_id, state, task_description, result, created_at, completed_at
         FROM subagents WHERE id = ?`, id,
	)
	if err := row.Scan(&sa.ID, &parentID, &sa.State, &sa.TaskDescription, &sa.Result, &createdAt, &completedAt); err != nil {
		return nil, fmt.Errorf("get subagent %s: %w", id, err)
	}
	sa.ParentID = parentID
	sa.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if completedAt.Valid {
		t := completedAt.Time
		sa.CompletedAt = &t
	}
	// ChildID is not currently stored in agent-facing SubAgent; omitted.
	return &sa, nil
}

// ListByParent lists all sub-agents for a given parent.
func (r *SubAgentRepository) ListByParent(ctx context.Context, parentID string) ([]agents.SubAgent, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		`SELECT id, parent_id, state, task_description, result, created_at, completed_at
         FROM subagents WHERE parent_id = ?`, parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list subagents by parent: %w", err)
	}
	defer rows.Close()

	var out []agents.SubAgent
	for rows.Next() {
		var sa agents.SubAgent
		var parentID string
		var createdAt string
		var completedAt sql.NullTime
		if err := rows.Scan(&sa.ID, &parentID, &sa.State, &sa.TaskDescription, &sa.Result, &createdAt, &completedAt); err != nil {
			continue
		}
		sa.ParentID = parentID
		_ = sa
		// Completed time handling below

		sa.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if completedAt.Valid {
			t := completedAt.Time
			sa.CompletedAt = &t
		}
		out = append(out, sa)
	}
	return out, nil
}

// Cancel marks the sub-agent as timed out/cancelled.
func (r *SubAgentRepository) Cancel(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.sqlite.ExecContext(ctx,
		`UPDATE subagents SET state = ?, completed_at = ? WHERE id = ?`,
		string(agents.SubAgentStateTimedOut), now, id,
	)
	if err != nil {
		return fmt.Errorf("cancel subagent %s: %w", id, err)
	}
	return nil
}

// GetResults returns the results for all sub-agents under a given parent.
func (r *SubAgentRepository) GetResults(ctx context.Context, parentID string) (map[string]string, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		`SELECT id, result FROM subagents WHERE parent_id = ?`, parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get subagent results: %w", err)
	}
	defer rows.Close()
	results := make(map[string]string)
	for rows.Next() {
		var id, res string
		if err := rows.Scan(&id, &res); err != nil {
			continue
		}
		results[id] = res
	}
	return results, nil
}

// --- Template Registry implementation (Agent templates) ---

// Instantiate creates a new agent in the system based on the named template.
func (r *SubAgentRepository) Instantiate(ctx context.Context, parentID agents.AgentID, templateName string) (*agents.Agent, error) {
	tmpl, err := r.GetTemplate(ctx, templateName)
	if err != nil {
		return nil, err
	}
	// Build a base CreateAgentRequest from the template defaults.
	req := agents.CreateAgentRequest{
		Name:        templateName + "-child",
		Description: tmpl.Description,
		Provider:    agents.ProviderAssignment{ProviderID: tmpl.DefaultProvider, Model: tmpl.DefaultModel, SystemPrompt: tmpl.SystemPromptTemplate},
		Schedule:    tmpl.DefaultSchedule,
		Policy:      tmpl.DefaultPolicy,
		Metadata:    map[string]string{},
		Tags:        []string{},
	}
	ar := NewAgentRepository(r.db)
	return ar.Create(ctx, req)
}

// GetTemplate loads a template by name from the agent_templates table.
func (r *SubAgentRepository) GetTemplate(ctx context.Context, name string) (*agents.AgentTemplate, error) {
	var tpl agents.AgentTemplate
	var cfg string
	row := r.db.sqlite.QueryRowContext(ctx,
		`SELECT name, description, config_json FROM agent_templates WHERE name = ?`, name,
	)
	if err := row.Scan(&tpl.Name, &tpl.Description, &cfg); err != nil {
		return nil, fmt.Errorf("get template %s: %w", name, err)
	}
	if cfg != "" {
		if err := json.Unmarshal([]byte(cfg), &tpl); err != nil {
			return nil, fmt.Errorf("unmarshal template config: %w", err)
		}
	}
	return &tpl, nil
}

// List returns all templates stored.
func (r *SubAgentRepository) List(ctx context.Context) ([]agents.AgentTemplate, error) {
	rows, err := r.db.sqlite.QueryContext(ctx, `SELECT name, description, config_json FROM agent_templates`)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()
	var out []agents.AgentTemplate
	for rows.Next() {
		var t agents.AgentTemplate
		var cfg string
		if err := rows.Scan(&t.Name, &t.Description, &cfg); err != nil {
			continue
		}
		if cfg != "" {
			if err := json.Unmarshal([]byte(cfg), &t); err != nil {
				continue
			}
		}
		out = append(out, t)
	}
	return out, nil
}

// Create stores a new template configuration in the database.
func (r *SubAgentRepository) Create(ctx context.Context, t agents.AgentTemplate) error {
	b, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}
	_, err = r.db.sqlite.ExecContext(ctx,
		`INSERT INTO agent_templates (name, description, config_json, created_at) VALUES (?, ?, ?, datetime('now'))`,
		t.Name, t.Description, string(b),
	)
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}
	return nil
}

// TemplateRepository implements TemplateRegistry for agent templates.
type TemplateRepository struct {
	db *DB
}

func NewTemplateRepository(db *DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

// Get loads a template by name.
func (t *TemplateRepository) Get(ctx context.Context, name string) (*agents.AgentTemplate, error) {
	var tpl agents.AgentTemplate
	var cfg string
	row := t.db.sqlite.QueryRowContext(ctx,
		`SELECT name, description, config_json FROM agent_templates WHERE name = ?`, name,
	)
	if err := row.Scan(&tpl.Name, &tpl.Description, &cfg); err != nil {
		return nil, fmt.Errorf("get template %s: %w", name, err)
	}
	if cfg != "" {
		if err := json.Unmarshal([]byte(cfg), &tpl); err != nil {
			return nil, fmt.Errorf("unmarshal template config: %w", err)
		}
	}
	return &tpl, nil
}

// List loads all templates.
func (t *TemplateRepository) List(ctx context.Context) ([]agents.AgentTemplate, error) {
	rows, err := t.db.sqlite.QueryContext(ctx, `SELECT name, description, config_json FROM agent_templates`)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()
	var out []agents.AgentTemplate
	for rows.Next() {
		var tpl agents.AgentTemplate
		var cfg string
		if err := rows.Scan(&tpl.Name, &tpl.Description, &cfg); err != nil {
			continue
		}
		if cfg != "" {
			if err := json.Unmarshal([]byte(cfg), &tpl); err != nil {
				continue
			}
		}
		out = append(out, tpl)
	}
	return out, nil
}

// Create stores a new template.
func (t *TemplateRepository) Create(ctx context.Context, tmpl agents.AgentTemplate) error {
	b, err := json.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}
	_, err = t.db.sqlite.ExecContext(ctx,
		`INSERT INTO agent_templates (name, description, config_json, created_at) VALUES (?, ?, ?, datetime('now'))`,
		tmpl.Name, tmpl.Description, string(b),
	)
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}
	return nil
}

// Delete removes a template.
func (t *TemplateRepository) Delete(ctx context.Context, name string) error {
	_, err := t.db.sqlite.ExecContext(ctx, `DELETE FROM agent_templates WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete template %s: %w", name, err)
	}
	return nil
}

// Instantiate uses a template to create a new agent under the given parent.
func (t *TemplateRepository) Instantiate(ctx context.Context, parentID agents.AgentID, templateName string) (*agents.Agent, error) {
	tmpl, err := t.Get(ctx, templateName)
	if err != nil {
		return nil, err
	}
	req := agents.CreateAgentRequest{
		Name:        templateName + "-child",
		Description: tmpl.Description,
		Provider:    agents.ProviderAssignment{ProviderID: tmpl.DefaultProvider, Model: tmpl.DefaultModel, SystemPrompt: tmpl.SystemPromptTemplate},
		Schedule:    tmpl.DefaultSchedule,
		Policy:      tmpl.DefaultPolicy,
		Metadata:    map[string]string{},
		Tags:        []string{},
	}
	ar := NewAgentRepository(t.db)
	return ar.Create(ctx, req)
}

// InstantiateHub is provided in case external code uses the interface and needs a stable entry point.
// It simply forwards to Instantiate of the TemplateRepository.
func (r *SubAgentRepository) InstantiateTemplate(ctx context.Context, parentID agents.AgentID, templateName string) (*agents.Agent, error) {
	tr := NewTemplateRepository(r.db)
	return tr.Instantiate(ctx, parentID, templateName)
}

// Delete removes a template by name.
func (r *SubAgentRepository) Delete(ctx context.Context, name string) error {
	_, err := r.db.sqlite.ExecContext(ctx, `DELETE FROM agent_templates WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete template %s: %w", name, err)
	}
	return nil
}

// (no leftover sqlNullTime helpers)
