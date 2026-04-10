package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ToolID string
type ToolCategory string

const (
	CatWeb     ToolCategory = "web"
	CatFile    ToolCategory = "file"
	CatCode    ToolCategory = "code"
	CatShell   ToolCategory = "shell"
	CatHTTP    ToolCategory = "http"
	CatBrowser ToolCategory = "browser"
	CatData    ToolCategory = "data"
	CatSystem  ToolCategory = "system"
)

// ToolSpec describes a tool's interface and constraints.
type ToolSpec struct {
	ID                  ToolID         `json:"id"`
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	Category            ToolCategory   `json:"category"`
	Parameters          map[string]any `json:"parameters"`
	RequiredPermissions []string       `json:"required_permissions"`
	Timeout             time.Duration  `json:"timeout"`
}

// ToolResult is the result of a tool execution.
type ToolResult struct {
	ToolID    ToolID        `json:"tool_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error"`
	Artifacts []string      `json:"artifacts"`
	Duration  time.Duration `json:"duration"`
}

// ToolExecutor is implemented by all tools to execute a named operation.
type ToolExecutor interface {
	Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error)
	Spec() ToolSpec
}

// ToolRegistry allows registering and fetching tools.
type ToolRegistry interface {
	Register(executor ToolExecutor) error
	Get(toolID string) (ToolExecutor, bool)
	List() []ToolSpec
	ListByCategory(cat ToolCategory) []ToolSpec
	Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error)
}

// LocalToolRegistry is an in-process registry for tools.
type LocalToolRegistry struct {
	mu        sync.RWMutex
	executors map[string]ToolExecutor
	specs     map[string]ToolSpec
}

// NewLocalToolRegistry creates an empty registry.
func NewLocalToolRegistry() *LocalToolRegistry {
	return &LocalToolRegistry{
		executors: make(map[string]ToolExecutor),
		specs:     make(map[string]ToolSpec),
	}
}

// Register adds or replaces a tool in the registry.
func (r *LocalToolRegistry) Register(executor ToolExecutor) error {
	spec := executor.Spec()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[string(spec.ID)] = executor
	r.specs[string(spec.ID)] = spec
	return nil
}

// Get retrieves a tool by its ID.
func (r *LocalToolRegistry) Get(toolID string) (ToolExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ex, ok := r.executors[toolID]
	return ex, ok
}

// List returns all registered tool specs.
func (r *LocalToolRegistry) List() []ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]ToolSpec, 0, len(r.specs))
	for _, s := range r.specs {
		specs = append(specs, s)
	}
	return specs
}

// ListByCategory filters specs by category.
func (r *LocalToolRegistry) ListByCategory(cat ToolCategory) []ToolSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []ToolSpec{}
	for _, s := range r.specs {
		if s.Category == cat {
			out = append(out, s)
		}
	}
	return out
}

// Execute runs a tool by ID.
func (r *LocalToolRegistry) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	ex, ok := r.Get(toolID)
	if !ok {
		return nil, fmt.Errorf("tool %s not found", toolID)
	}
	return ex.Execute(ctx, toolID, params)
}
