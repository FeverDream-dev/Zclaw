package agents

import (
	"context"
	"time"
)

// SubAgentRequest defines a request to spawn a child sub-agent.
type SubAgentRequest struct {
	// ParentID is the ID of the parent agent under which this sub-agent runs.
	ParentID string
	// Name is the human-friendly name for the sub-agent.
	Name string
	// TaskDescription describes the task assigned to the sub-agent.
	TaskDescription string
	// ProviderOverride allows overriding the provider for this sub-agent.
	ProviderOverride *ProviderAssignment
	// ModelOverride overrides the model for this sub-agent.
	ModelOverride string
	// Timeout in seconds for the sub-agent's task.
	Timeout int
}

// SubAgentState represents the lifecycle state of a sub-agent.
type SubAgentState string

const (
	SubAgentStateSpawned   SubAgentState = "spawned"
	SubAgentStateRunning   SubAgentState = "running"
	SubAgentStateCompleted SubAgentState = "completed"
	SubAgentStateFailed    SubAgentState = "failed"
	SubAgentStateTimedOut  SubAgentState = "timed_out"
)

func (s SubAgentState) String() string { return string(s) }

// SubAgent represents a spawned child agent created by a parent.
type SubAgent struct {
	ID              string
	ParentID        string
	Name            string
	State           SubAgentState
	TaskDescription string
	Result          string
	CreatedAt       time.Time
	CompletedAt     *time.Time
}

// SubAgentRegistry manages lifecycle of sub-agents.
type SubAgentRegistry interface {
	// Spawn creates a new sub-agent under the given parent.
	Spawn(ctx context.Context, req SubAgentRequest) (*SubAgent, error)
	// Get retrieves a sub-agent by its ID.
	Get(ctx context.Context, id string) (*SubAgent, error)
	// ListByParent lists all sub-agents for a given parent agent.
	ListByParent(ctx context.Context, parentID string) ([]SubAgent, error)
	// Cancel cancels a running sub-agent, marking it timed out or failed.
	Cancel(ctx context.Context, id string) error
	// GetResults returns the results of all sub-agents under a given parent.
	GetResults(ctx context.Context, parentID string) (map[string]string, error)
}

// AgentTemplate defines a reusable blueprint for creating agents from templates.
type AgentTemplate struct {
	Name                 string
	Description          string
	DefaultProvider      string
	DefaultModel         string
	DefaultPolicy        PolicyConfig
	DefaultSchedule      ScheduleConfig
	SystemPromptTemplate string
}

// TemplateRegistry can Create/Get/List/Delete templates and Instantiate new agents from them.
type TemplateRegistry interface {
	Create(ctx context.Context, t AgentTemplate) error
	Get(ctx context.Context, name string) (*AgentTemplate, error)
	List(ctx context.Context) ([]AgentTemplate, error)
	Delete(ctx context.Context, name string) error
	// Instantiate creates a new agent in the system based on the template and registers it under the given parent.
	Instantiate(ctx context.Context, parentID AgentID, templateName string) (*Agent, error)
}
