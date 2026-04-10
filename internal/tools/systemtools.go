package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ListFilesTool lists directory contents.
type ListFilesTool struct{ spec ToolSpec }

func NewListFilesTool() *ListFilesTool {
	t := &ListFilesTool{}
	t.spec = ToolSpec{
		ID:          "list_files",
		Name:        "ListFiles",
		Description: "List directory contents",
		Category:    CatFile,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []string{"path"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *ListFilesTool) Spec() ToolSpec { return t.spec }
func (t *ListFilesTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	p, _ := params["path"].(string)
	if p == "" {
		p = "."
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	b, _ := jsonMarshal(names)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
}

// DiskUsageTool reports disk usage for a directory (recursively).
type DiskUsageTool struct{ spec ToolSpec }

func NewDiskUsageTool() *DiskUsageTool {
	t := &DiskUsageTool{}
	t.spec = ToolSpec{
		ID:          "disk_usage",
		Name:        "DiskUsage",
		Description: "Compute disk usage for a path",
		Category:    CatSystem,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []string{"path"},
		},
		Timeout: 30 * time.Second,
	}
	return t
}
func (t *DiskUsageTool) Spec() ToolSpec { return t.spec }
func (t *DiskUsageTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	root, _ := params["path"].(string)
	if root == "" {
		root = "/"
	}
	var results = map[string]int64{}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			fi, ferr := os.Stat(path)
			if ferr == nil {
				results[path] = fi.Size()
			}
		}
		return nil
	})
	b, _ := jsonMarshal(results)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
}

// EnvTool reads environment variables (optionally filtered by an allowlist).
type EnvTool struct{ spec ToolSpec }

func NewEnvTool() *EnvTool {
	t := &EnvTool{}
	t.spec = ToolSpec{
		ID:          "env",
		Name:        "Env",
		Description: "Environment variables",
		Category:    CatSystem,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"allow": map[string]any{"type": "array"}},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *EnvTool) Spec() ToolSpec { return t.spec }
func (t *EnvTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	// Read all env vars or filter by allowlist
	allowRaw, _ := params["allow"].([]string)
	allow := map[string]bool{}
	for _, a := range allowRaw {
		allow[a] = true
	}
	m := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if len(allow) == 0 || allow[key] {
			m[key] = val
		}
	}
	b, _ := jsonMarshal(m)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
}

// TimestampTool returns current time in various formats.
type TimestampTool struct{ spec ToolSpec }

func NewTimestampTool() *TimestampTool {
	t := &TimestampTool{}
	t.spec = ToolSpec{
		ID:          "timestamp",
		Name:        "Timestamp",
		Description: "Current timestamp in various formats",
		Category:    CatSystem,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"formats": map[string]any{"type": "array"}},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *TimestampTool) Spec() ToolSpec { return t.spec }
func (t *TimestampTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	// Simple implementation: always return a fixed set of formats
	now := time.Now().UTC()
	out := map[string]string{
		"unix":    fmt.Sprintf("%d", now.Unix()),
		"rfc3339": now.Format(time.RFC3339),
		"iso8601": now.Format("2006-01-02T15:04:05Z07:00"),
	}
	b, _ := jsonMarshal(out)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
