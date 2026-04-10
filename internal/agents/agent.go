// Package agents provides the agent registry and lifecycle management.
//
// Agents are logical entities stored as database rows, not running processes.
// An agent becomes "active" only when executing a task; otherwise it is idle
// metadata (config, schedules, memory pointers, queue state).
package agents

import (
	"context"
	"fmt"
	"time"
)

// AgentState represents the lifecycle state of an agent.
type AgentState string

const (
	StateCreated  AgentState = "created"
	StateActive   AgentState = "active"
	StatePaused   AgentState = "paused"
	StateDisabled AgentState = "disabled"
	StateArchived AgentState = "archived"
)

func (s AgentState) String() string { return string(s) }

// ValidTransitions defines the legal state transitions.
var ValidTransitions = map[AgentState][]AgentState{
	StateCreated:  {StateActive, StateDisabled, StateArchived},
	StateActive:   {StatePaused, StateDisabled, StateArchived},
	StatePaused:   {StateActive, StateDisabled, StateArchived},
	StateDisabled: {StateActive, StateArchived},
	StateArchived: {},
}

// IsValidTransition checks whether moving from -> to is allowed.
func IsValidTransition(from, to AgentState) bool {
	for _, allowed := range ValidTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// AgentID is the unique identifier for an agent.
type AgentID string

func (id AgentID) String() string { return string(id) }

// ScheduleConfig defines when and how an agent should wake up.
type ScheduleConfig struct {
	// Cron expression for periodic wakeups (e.g., "*/5 * * * *").
	Cron string `json:"cron,omitempty"`

	// HeartbeatInterval is the time between heartbeat checks.
	// Zero means no periodic heartbeat.
	HeartbeatInterval time.Duration `json:"heartbeat_interval,omitempty"`

	// ActiveHours defines when the agent is allowed to run.
	// Empty means always active.
	ActiveHours *ActiveHours `json:"active_hours,omitempty"`

	// JitterSeconds adds random delay to scheduled wakeups to avoid thundering herd.
	JitterSeconds int `json:"jitter_seconds,omitempty"`

	// Enabled controls whether the schedule is active.
	Enabled bool `json:"enabled"`
}

// ActiveHours defines the time window when an agent is allowed to run.
type ActiveHours struct {
	// Timezone for interpreting Start/End (e.g., "America/Sao_Paulo").
	Timezone string `json:"timezone"`

	// Start is the hour (0-23) when the agent becomes active.
	StartHour int `json:"start_hour"`

	// End is the hour (0-23) when the agent stops being active.
	EndHour int `json:"end_hour"`

	// Days of week (0=Sunday, 6=Saturday). Empty means every day.
	Days []int `json:"days,omitempty"`
}

// PolicyConfig defines what an agent is allowed to do.
type PolicyConfig struct {
	// AllowShell enables shell command execution in tool-worker.
	AllowShell bool `json:"allow_shell"`

	// AllowBrowser enables browser automation.
	AllowBrowser bool `json:"allow_browser"`

	// AllowBackgroundSubagents enables spawning sub-agents.
	AllowBackgroundSubagents bool `json:"allow_background_subagents"`

	// MaxMemoryMB limits per-task memory usage.
	MaxMemoryMB int `json:"max_memory_mb,omitempty"`

	// MaxCPUFraction limits per-task CPU (0.0-1.0).
	MaxCPUFraction float64 `json:"max_cpu_fraction,omitempty"`

	// TimeoutSeconds limits task execution time.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// MaxConcurrentTasks limits how many tasks can run simultaneously.
	MaxConcurrentTasks int `json:"max_concurrent_tasks,omitempty"`

	// NetworkMode controls network access: "full", "restricted", "none".
	NetworkMode string `json:"network_mode,omitempty"`

	// FilesystemMode controls filesystem access: "read-write", "read-only", "none".
	FilesystemMode string `json:"filesystem_mode,omitempty"`
}

// ProviderAssignment defines which provider and model an agent uses.
type ProviderAssignment struct {
	// ProviderID identifies the provider adapter (e.g., "openai", "anthropic").
	ProviderID string `json:"provider_id"`

	// Model is the model identifier to use (e.g., "gpt-4o-mini").
	Model string `json:"model"`

	// FallbackProvider is used if the primary provider fails.
	FallbackProvider string `json:"fallback_provider,omitempty"`

	// FallbackModel is the model for the fallback provider.
	FallbackModel string `json:"fallback_model,omitempty"`

	// UtilityProvider is used for maintenance/heartbeat turns (cheap model).
	UtilityProvider string `json:"utility_provider,omitempty"`

	// UtilityModel is the cheap model for maintenance turns.
	UtilityModel string `json:"utility_model,omitempty"`

	// SystemPrompt is the agent's system prompt.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// MaxContextTokens limits the context window.
	MaxContextTokens int `json:"max_context_tokens,omitempty"`

	// Temperature controls response randomness (0.0-2.0).
	Temperature float64 `json:"temperature,omitempty"`
}

// Agent represents a logical agent entity.
type Agent struct {
	// ID is the unique agent identifier.
	ID AgentID `json:"id"`

	// Name is a human-readable agent name.
	Name string `json:"name"`

	// Description is an optional longer description.
	Description string `json:"description,omitempty"`

	// State is the current lifecycle state.
	State AgentState `json:"state"`

	// Provider defines which LLM provider and model this agent uses.
	Provider ProviderAssignment `json:"provider"`

	// Schedule defines when this agent wakes up.
	Schedule ScheduleConfig `json:"schedule"`

	// Policy defines what this agent is allowed to do.
	Policy PolicyConfig `json:"policy"`

	// WorkspacePath is the filesystem path for this agent's files.
	WorkspacePath string `json:"workspace_path,omitempty"`

	// Metadata is arbitrary key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Tags are freeform labels for filtering.
	Tags []string `json:"tags,omitempty"`

	// CreatedAt is when the agent was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the agent was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// LastActiveAt is when the agent last ran a task.
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
}

// CreateAgentRequest holds the parameters for creating a new agent.
type CreateAgentRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Provider    ProviderAssignment `json:"provider"`
	Schedule    ScheduleConfig     `json:"schedule"`
	Policy      PolicyConfig       `json:"policy"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
}

// UpdateAgentRequest holds the parameters for updating an agent.
type UpdateAgentRequest struct {
	Name        *string             `json:"name,omitempty"`
	Description *string             `json:"description,omitempty"`
	Provider    *ProviderAssignment `json:"provider,omitempty"`
	Schedule    *ScheduleConfig     `json:"schedule,omitempty"`
	Policy      *PolicyConfig       `json:"policy,omitempty"`
	State       *AgentState         `json:"state,omitempty"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
}

// AgentFilter defines query parameters for listing agents.
type AgentFilter struct {
	State    AgentState `json:"state,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	Provider string     `json:"provider,omitempty"`
	Limit    int        `json:"limit,omitempty"`
	Offset   int        `json:"offset,omitempty"`
	OrderBy  string     `json:"order_by,omitempty"`
}

// AgentList is a page of agents.
type AgentList struct {
	Agents  []Agent `json:"agents"`
	Total   int     `json:"total"`
	Limit   int     `json:"limit"`
	Offset  int     `json:"offset"`
	HasMore bool    `json:"has_more"`
}

// Registry manages agent CRUD and state transitions.
type Registry interface {
	// Create adds a new agent. Returns the created agent with ID and timestamps set.
	Create(ctx context.Context, req CreateAgentRequest) (*Agent, error)

	// Get retrieves a single agent by ID.
	Get(ctx context.Context, id AgentID) (*Agent, error)

	// Update modifies an existing agent. Only non-nil fields are updated.
	Update(ctx context.Context, id AgentID, req UpdateAgentRequest) (*Agent, error)

	// Delete permanently removes an agent (hard delete for now).
	Delete(ctx context.Context, id AgentID) error

	// List returns agents matching the filter.
	List(ctx context.Context, filter AgentFilter) (*AgentList, error)

	// TransitionState changes an agent's lifecycle state.
	// Returns an error if the transition is invalid.
	TransitionState(ctx context.Context, id AgentID, newState AgentState) (*Agent, error)

	// Count returns the total number of agents, optionally filtered by state.
	Count(ctx context.Context, state AgentState) (int, error)

	// GetBySchedule returns agents that have schedules matching the current time.
	GetBySchedule(ctx context.Context, now time.Time) ([]Agent, error)
}

// Validate checks that a CreateAgentRequest has all required fields.
func (r *CreateAgentRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if r.Provider.ProviderID == "" {
		return fmt.Errorf("provider_id is required")
	}
	if r.Provider.Model == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}
