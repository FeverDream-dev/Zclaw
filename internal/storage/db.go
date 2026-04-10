// Package storage provides the SQLite-backed storage layer.
//
// Uses modernc.org/sqlite (pure Go, CGO-free) with WAL mode for concurrent
// reads. All queries use prepared statements. The storage layer implements
// the repositories needed by agents, providers, scheduler, and memory packages.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zclaw/zclaw/internal/agents"
	"github.com/zclaw/zclaw/internal/providers"
)

// DB is the main database handle.
type DB struct {
	sqlite *sql.DB
	path   string
}

// Open creates or opens a SQLite database at the given path.
// It enables WAL mode and configures connection pooling.
func Open(ctx context.Context, path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Enable WAL mode for concurrent reads.
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign keys.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Busy timeout for write contention.
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	// Connection pool settings for single-node.
	db.SetMaxOpenConns(1) // SQLite single-writer
	db.SetMaxIdleConns(1)

	return &DB{sqlite: db, path: path}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.sqlite.Close()
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}

// BeginTx starts a transaction.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return db.sqlite.BeginTx(ctx, opts)
}

// Migrate runs all pending database migrations.
func (db *DB) Migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		migrationV1,
		migrationV2,
		migrationV3,
		migrationV4,
		migrationV5,
		migrationV6,
		migrationV7,
		migrationV8,
		migrationV9,
	}

	for i, m := range migrations {
		if i == 0 {
			// Create migrations tracking table.
			if _, err := db.sqlite.ExecContext(ctx, m); err != nil {
				return fmt.Errorf("create migrations table: %w", err)
			}
			continue
		}

		var count int
		err := db.sqlite.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", i,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", i, err)
		}
		if count > 0 {
			continue // Already applied.
		}

		if _, err := db.sqlite.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("apply migration %d: %w", i, err)
		}
		if _, err := db.sqlite.ExecContext(ctx,
			"INSERT INTO schema_migrations (version) VALUES (?)", i,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", i, err)
		}
	}

	return nil
}

// Migration SQL statements.

const migrationV1 = `
CREATE TABLE IF NOT EXISTS agents (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'created',
	provider_id TEXT NOT NULL,
	model TEXT NOT NULL,
	fallback_provider TEXT NOT NULL DEFAULT '',
	fallback_model TEXT NOT NULL DEFAULT '',
	utility_provider TEXT NOT NULL DEFAULT '',
	utility_model TEXT NOT NULL DEFAULT '',
	system_prompt TEXT NOT NULL DEFAULT '',
	max_context_tokens INTEGER NOT NULL DEFAULT 0,
	temperature REAL NOT NULL DEFAULT 0.7,
	workspace_path TEXT NOT NULL DEFAULT '',
	metadata TEXT NOT NULL DEFAULT '{}',
	tags TEXT NOT NULL DEFAULT '[]',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	last_active_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_agents_state ON agents(state);
CREATE INDEX IF NOT EXISTS idx_agents_provider ON agents(provider_id);
`

const migrationV2 = `
CREATE TABLE IF NOT EXISTS agent_schedules (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	cron TEXT NOT NULL DEFAULT '',
	heartbeat_interval_seconds INTEGER NOT NULL DEFAULT 0,
	timezone TEXT NOT NULL DEFAULT 'UTC',
	active_start_hour INTEGER NOT NULL DEFAULT 0,
	active_end_hour INTEGER NOT NULL DEFAULT 23,
	active_days TEXT NOT NULL DEFAULT '[]',
	jitter_seconds INTEGER NOT NULL DEFAULT 30,
	enabled INTEGER NOT NULL DEFAULT 1,
	UNIQUE(agent_id)
);

CREATE INDEX IF NOT EXISTS idx_schedules_agent ON agent_schedules(agent_id);
CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON agent_schedules(enabled);
`

const migrationV3 = `
CREATE TABLE IF NOT EXISTS agent_policies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	allow_shell INTEGER NOT NULL DEFAULT 0,
	allow_browser INTEGER NOT NULL DEFAULT 0,
	allow_background_subagents INTEGER NOT NULL DEFAULT 0,
	max_memory_mb INTEGER NOT NULL DEFAULT 256,
	max_cpu_fraction REAL NOT NULL DEFAULT 0.5,
	timeout_seconds INTEGER NOT NULL DEFAULT 300,
	max_concurrent_tasks INTEGER NOT NULL DEFAULT 1,
	network_mode TEXT NOT NULL DEFAULT 'restricted',
	filesystem_mode TEXT NOT NULL DEFAULT 'read-write',
	UNIQUE(agent_id)
);
`

const migrationV4 = `
CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	state TEXT NOT NULL DEFAULT 'pending',
	priority INTEGER NOT NULL DEFAULT 0,
	attempt INTEGER NOT NULL DEFAULT 0,
	max_attempts INTEGER NOT NULL DEFAULT 3,
	input TEXT NOT NULL DEFAULT '',
	output TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	model_used TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	token_usage TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	started_at TEXT,
	completed_at TEXT,
	scheduled_at TEXT,
	timeout_seconds INTEGER NOT NULL DEFAULT 300
);

CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(agent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_state ON tasks(state);
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled ON tasks(scheduled_at) WHERE state = 'pending';
`

const migrationV5 = `
CREATE TABLE IF NOT EXISTS provider_configs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	auth_mode TEXT NOT NULL DEFAULT 'api_key',
	base_url TEXT NOT NULL DEFAULT '',
	api_key TEXT NOT NULL DEFAULT '',
	capabilities TEXT NOT NULL DEFAULT '[]',
	models TEXT NOT NULL DEFAULT '[]',
	rate_limit TEXT NOT NULL DEFAULT '{}',
	cost_class TEXT NOT NULL DEFAULT 'variable',
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

const migrationV6 = `
CREATE TABLE IF NOT EXISTS conversations (
	id TEXT PRIMARY KEY,
	agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	token_count INTEGER NOT NULL DEFAULT 0,
	metadata TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	role TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	tool_calls TEXT NOT NULL DEFAULT '[]',
	tool_result TEXT,
	model TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	seq INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, seq);
CREATE INDEX IF NOT EXISTS idx_conversations_agent ON conversations(agent_id);
`

const migrationV7 = `
CREATE TABLE IF NOT EXISTS artifacts (
	id TEXT PRIMARY KEY,
	agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	task_id TEXT,
	filename TEXT NOT NULL,
	file_path TEXT NOT NULL,
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	size_bytes INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	expires_at TEXT,
	metadata TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_artifacts_agent ON artifacts(agent_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_expires ON artifacts(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	agent_id TEXT,
	task_id TEXT,
	action TEXT NOT NULL,
	details TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_agent ON audit_log(agent_id);
CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(created_at);
`

const migrationV8 = `
CREATE TABLE IF NOT EXISTS subagents (
	id TEXT PRIMARY KEY,
	parent_id TEXT NOT NULL,
	child_agent_id TEXT,
	state TEXT NOT NULL DEFAULT 'spawned',
	task_description TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	completed_at TEXT,
	FOREIGN KEY (parent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_subagents_parent ON subagents(parent_id);
CREATE INDEX IF NOT EXISTS idx_subagents_state ON subagents(state);

CREATE TABLE IF NOT EXISTS agent_templates (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	config_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE agents ADD COLUMN parent_id TEXT NOT NULL DEFAULT '';
`

const migrationV9 = `
CREATE TABLE IF NOT EXISTS tool_executions (
	id TEXT PRIMARY KEY,
	agent_id TEXT NOT NULL,
	task_id TEXT,
	tool_id TEXT NOT NULL,
	params TEXT NOT NULL DEFAULT '{}',
	result TEXT NOT NULL DEFAULT '',
	success INTEGER NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tool_exec_agent ON tool_executions(agent_id);
CREATE INDEX IF NOT EXISTS idx_tool_exec_time ON tool_executions(created_at);

CREATE TABLE IF NOT EXISTS connections (
	id TEXT PRIMARY KEY,
	agent_id TEXT,
	type TEXT NOT NULL,
	config TEXT NOT NULL DEFAULT '{}',
	state TEXT NOT NULL DEFAULT 'stopped',
	last_activity TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_connections_agent ON connections(agent_id);
`

// AgentRepository implements agents.Registry using SQLite.
type AgentRepository struct {
	db *DB
}

// NewAgentRepository creates a new agent repository.
func NewAgentRepository(db *DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create inserts a new agent.
func (r *AgentRepository) Create(ctx context.Context, req agents.CreateAgentRequest) (*agents.Agent, error) {
	id := agents.AgentID(newID())
	now := time.Now().UTC()

	workspacePath := req.Metadata["workspace_path"]
	if workspacePath == "" {
		workspacePath = fmt.Sprintf("data/workspaces/%s", id)
	}

	_, err := r.db.sqlite.ExecContext(ctx,
		`INSERT INTO agents (id, name, description, state, provider_id, model,
			fallback_provider, fallback_model, utility_provider, utility_model,
			system_prompt, max_context_tokens, temperature, workspace_path,
			metadata, tags, created_at, updated_at)
		VALUES (?, ?, ?, 'created', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(id), req.Name, req.Description,
		req.Provider.ProviderID, req.Provider.Model,
		req.Provider.FallbackProvider, req.Provider.FallbackModel,
		req.Provider.UtilityProvider, req.Provider.UtilityModel,
		req.Provider.SystemPrompt, req.Provider.MaxContextTokens,
		req.Provider.Temperature, workspacePath,
		"{}", "[]", now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	// Insert schedule.
	_, err = r.db.sqlite.ExecContext(ctx,
		`INSERT INTO agent_schedules (agent_id, cron, heartbeat_interval_seconds,
			timezone, active_start_hour, active_end_hour, jitter_seconds, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		string(id), req.Schedule.Cron,
		int(req.Schedule.HeartbeatInterval.Seconds()),
		"UTC", 0, 23, req.Schedule.JitterSeconds, req.Schedule.Enabled,
	)
	if err != nil {
		return nil, fmt.Errorf("insert schedule: %w", err)
	}

	// Insert policy.
	_, err = r.db.sqlite.ExecContext(ctx,
		`INSERT INTO agent_policies (agent_id, allow_shell, allow_browser,
			allow_background_subagents, max_memory_mb, max_cpu_fraction,
			timeout_seconds, max_concurrent_tasks, network_mode, filesystem_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(id),
		boolToInt(req.Policy.AllowShell), boolToInt(req.Policy.AllowBrowser),
		boolToInt(req.Policy.AllowBackgroundSubagents),
		req.Policy.MaxMemoryMB, req.Policy.MaxCPUFraction,
		req.Policy.TimeoutSeconds, req.Policy.MaxConcurrentTasks,
		req.Policy.NetworkMode, req.Policy.FilesystemMode,
	)
	if err != nil {
		return nil, fmt.Errorf("insert policy: %w", err)
	}

	return r.Get(ctx, id)
}

// Get retrieves a single agent by ID with its schedule and policy.
func (r *AgentRepository) Get(ctx context.Context, id agents.AgentID) (*agents.Agent, error) {
	var a agents.Agent
	var createdAt, updatedAt string
	var lastActive sql.NullString

	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id, name, description, state, provider_id, model,
			fallback_provider, fallback_model, utility_provider, utility_model,
			system_prompt, max_context_tokens, temperature, workspace_path,
			created_at, updated_at, last_active_at
		FROM agents WHERE id = ?`, string(id),
	).Scan(&a.ID, &a.Name, &a.Description, &a.State,
		&a.Provider.ProviderID, &a.Provider.Model,
		&a.Provider.FallbackProvider, &a.Provider.FallbackModel,
		&a.Provider.UtilityProvider, &a.Provider.UtilityModel,
		&a.Provider.SystemPrompt, &a.Provider.MaxContextTokens,
		&a.Provider.Temperature, &a.WorkspacePath,
		&createdAt, &updatedAt, &lastActive,
	)
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", id, err)
	}

	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastActive.Valid && lastActive.String != "" {
		t, _ := time.Parse(time.RFC3339, lastActive.String)
		a.LastActiveAt = &t
	}

	// Load schedule.
	r.loadSchedule(ctx, &a)
	// Load policy.
	r.loadPolicy(ctx, &a)

	return &a, nil
}

func (r *AgentRepository) loadSchedule(ctx context.Context, a *agents.Agent) {
	var cron, tz string
	var intervalSec, jitter, enabled int
	var startH, endH int

	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT cron, heartbeat_interval_seconds, timezone, active_start_hour,
			active_end_hour, jitter_seconds, enabled
		FROM agent_schedules WHERE agent_id = ?`, string(a.ID),
	).Scan(&cron, &intervalSec, &tz, &startH, &endH, &jitter, &enabled)
	if err != nil {
		return
	}

	a.Schedule = agents.ScheduleConfig{
		Cron:              cron,
		HeartbeatInterval: time.Duration(intervalSec) * time.Second,
		ActiveHours: &agents.ActiveHours{
			Timezone:  tz,
			StartHour: startH,
			EndHour:   endH,
		},
		JitterSeconds: jitter,
		Enabled:       enabled == 1,
	}
}

func (r *AgentRepository) loadPolicy(ctx context.Context, a *agents.Agent) {
	var allowShell, allowBrowser, allowSubagents int
	var maxMem int
	var maxCPU float64
	var timeout, maxConcurrent int
	var networkMode, fsMode string

	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT allow_shell, allow_browser, allow_background_subagents,
			max_memory_mb, max_cpu_fraction, timeout_seconds,
			max_concurrent_tasks, network_mode, filesystem_mode
		FROM agent_policies WHERE agent_id = ?`, string(a.ID),
	).Scan(&allowShell, &allowBrowser, &allowSubagents,
		&maxMem, &maxCPU, &timeout, &maxConcurrent,
		&networkMode, &fsMode,
	)
	if err != nil {
		return
	}

	a.Policy = agents.PolicyConfig{
		AllowShell:               allowShell == 1,
		AllowBrowser:             allowBrowser == 1,
		AllowBackgroundSubagents: allowSubagents == 1,
		MaxMemoryMB:              maxMem,
		MaxCPUFraction:           maxCPU,
		TimeoutSeconds:           timeout,
		MaxConcurrentTasks:       maxConcurrent,
		NetworkMode:              networkMode,
		FilesystemMode:           fsMode,
	}
}

// Update modifies an existing agent.
func (r *AgentRepository) Update(ctx context.Context, id agents.AgentID, req agents.UpdateAgentRequest) (*agents.Agent, error) {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.State != nil {
		if !agents.IsValidTransition(existing.State, *req.State) {
			return nil, fmt.Errorf("invalid state transition: %s -> %s", existing.State, *req.State)
		}
		existing.State = *req.State
	}
	if req.Provider != nil {
		existing.Provider = *req.Provider
	}

	_, err = r.db.sqlite.ExecContext(ctx,
		`UPDATE agents SET name=?, description=?, state=?, provider_id=?, model=?,
			fallback_provider=?, fallback_model=?, utility_provider=?, utility_model=?,
			system_prompt=?, max_context_tokens=?, temperature=?, updated_at=?
		WHERE id=?`,
		existing.Name, existing.Description, existing.State,
		existing.Provider.ProviderID, existing.Provider.Model,
		existing.Provider.FallbackProvider, existing.Provider.FallbackModel,
		existing.Provider.UtilityProvider, existing.Provider.UtilityModel,
		existing.Provider.SystemPrompt, existing.Provider.MaxContextTokens,
		existing.Provider.Temperature,
		time.Now().UTC().Format(time.RFC3339), string(id),
	)
	if err != nil {
		return nil, fmt.Errorf("update agent %s: %w", id, err)
	}

	if req.Schedule != nil {
		_, err = r.db.sqlite.ExecContext(ctx,
			`UPDATE agent_schedules SET cron=?, heartbeat_interval_seconds=?,
				jitter_seconds=?, enabled=? WHERE agent_id=?`,
			req.Schedule.Cron, int(req.Schedule.HeartbeatInterval.Seconds()),
			req.Schedule.JitterSeconds, boolToInt(req.Schedule.Enabled), string(id),
		)
		if err != nil {
			return nil, fmt.Errorf("update schedule: %w", err)
		}
	}

	if req.Policy != nil {
		_, err = r.db.sqlite.ExecContext(ctx,
			`UPDATE agent_policies SET allow_shell=?, allow_browser=?,
				allow_background_subagents=?, max_memory_mb=?, max_cpu_fraction=?,
				timeout_seconds=?, max_concurrent_tasks=?, network_mode=?, filesystem_mode=?
			WHERE agent_id=?`,
			boolToInt(req.Policy.AllowShell), boolToInt(req.Policy.AllowBrowser),
			boolToInt(req.Policy.AllowBackgroundSubagents),
			req.Policy.MaxMemoryMB, req.Policy.MaxCPUFraction,
			req.Policy.TimeoutSeconds, req.Policy.MaxConcurrentTasks,
			req.Policy.NetworkMode, req.Policy.FilesystemMode, string(id),
		)
		if err != nil {
			return nil, fmt.Errorf("update policy: %w", err)
		}
	}

	return r.Get(ctx, id)
}

// Delete permanently removes an agent and its related data.
func (r *AgentRepository) Delete(ctx context.Context, id agents.AgentID) error {
	_, err := r.db.sqlite.ExecContext(ctx, "DELETE FROM agents WHERE id = ?", string(id))
	if err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}
	return nil
}

// List returns agents matching the filter.
func (r *AgentRepository) List(ctx context.Context, filter agents.AgentFilter) (*agents.AgentList, error) {
	query := "SELECT id FROM agents WHERE 1=1"
	args := []any{}

	if filter.State != "" {
		query += " AND state = ?"
		args = append(args, string(filter.State))
	}
	if filter.Provider != "" {
		query += " AND provider_id = ?"
		args = append(args, filter.Provider)
	}

	// Count total.
	var total int
	countQuery := "SELECT COUNT(*) FROM agents WHERE 1=1"
	countArgs := args
	if filter.State != "" {
		countQuery += " AND state = ?"
	}
	if filter.Provider != "" {
		countQuery += " AND provider_id = ?"
	}
	err := r.db.sqlite.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count agents: %w", err)
	}

	// Apply ordering and pagination.
	switch filter.OrderBy {
	case "name":
		query += " ORDER BY name"
	case "created_at":
		query += " ORDER BY created_at"
	default:
		query += " ORDER BY id"
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.sqlite.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agentList []agents.Agent
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		agent, err := r.Get(ctx, agents.AgentID(id))
		if err != nil {
			continue
		}
		agentList = append(agentList, *agent)
	}

	if agentList == nil {
		agentList = []agents.Agent{}
	}

	return &agents.AgentList{
		Agents:  agentList,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: offset+limit < total,
	}, nil
}

// TransitionState changes an agent's lifecycle state.
func (r *AgentRepository) TransitionState(ctx context.Context, id agents.AgentID, newState agents.AgentState) (*agents.Agent, error) {
	return r.Update(ctx, id, agents.UpdateAgentRequest{State: &newState})
}

// Count returns the total number of agents, optionally filtered by state.
func (r *AgentRepository) Count(ctx context.Context, state agents.AgentState) (int, error) {
	if state != "" {
		var count int
		err := r.db.sqlite.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM agents WHERE state = ?", string(state),
		).Scan(&count)
		return count, err
	}
	var count int
	err := r.db.sqlite.QueryRowContext(ctx, "SELECT COUNT(*) FROM agents").Scan(&count)
	return count, err
}

// GetBySchedule returns agents whose schedules indicate they should run now.
func (r *AgentRepository) GetBySchedule(ctx context.Context, now time.Time) ([]agents.Agent, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		`SELECT a.id FROM agents a
		JOIN agent_schedules s ON a.id = s.agent_id
		WHERE a.state = 'active' AND s.enabled = 1`,
	)
	if err != nil {
		return nil, fmt.Errorf("get scheduled agents: %w", err)
	}
	defer rows.Close()

	var result []agents.Agent
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		agent, err := r.Get(ctx, agents.AgentID(id))
		if err != nil {
			continue
		}
		result = append(result, *agent)
	}
	return result, nil
}

// ProviderConfigRepository manages provider configuration persistence.
type ProviderConfigRepository struct {
	db *DB
}

// NewProviderConfigRepository creates a new provider config repository.
func NewProviderConfigRepository(db *DB) *ProviderConfigRepository {
	return &ProviderConfigRepository{db: db}
}

// Save persists a provider configuration.
func (r *ProviderConfigRepository) Save(ctx context.Context, config providers.ProviderConfig) error {
	_, err := r.db.sqlite.ExecContext(ctx,
		`INSERT INTO provider_configs (id, name, auth_mode, base_url, api_key,
			capabilities, models, rate_limit, cost_class, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, auth_mode=excluded.auth_mode, base_url=excluded.base_url,
			api_key=excluded.api_key, capabilities=excluded.capabilities,
			models=excluded.models, rate_limit=excluded.rate_limit,
			cost_class=excluded.cost_class, metadata=excluded.metadata,
			updated_at=excluded.updated_at`,
		string(config.ID), config.Name, string(config.AuthMode),
		config.BaseURL, config.APIKey, "[]", "[]", "{}",
		string(config.CostClass), "{}",
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// Get retrieves a provider configuration by ID.
func (r *ProviderConfigRepository) Get(ctx context.Context, id providers.ProviderID) (*providers.ProviderConfig, error) {
	var config providers.ProviderConfig

	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id, name, auth_mode, base_url, api_key, cost_class
		FROM provider_configs WHERE id = ?`, string(id),
	).Scan(&config.ID, &config.Name, &config.AuthMode,
		&config.BaseURL, &config.APIKey, &config.CostClass,
	)
	if err != nil {
		return nil, fmt.Errorf("get provider config %s: %w", id, err)
	}
	return &config, nil
}

// List returns all provider configurations.
func (r *ProviderConfigRepository) List(ctx context.Context) ([]providers.ProviderConfig, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		"SELECT id, name, auth_mode, base_url, cost_class FROM provider_configs",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []providers.ProviderConfig
	for rows.Next() {
		var c providers.ProviderConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.AuthMode, &c.BaseURL, &c.CostClass); err != nil {
			continue
		}
		configs = append(configs, c)
	}
	return configs, nil
}

// Helper functions.

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
