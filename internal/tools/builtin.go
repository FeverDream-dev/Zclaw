package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WebFetchTool fetches a URL and returns the response body as text.
type WebFetchTool struct {
	spec ToolSpec
}

func NewWebFetchTool() *WebFetchTool {
	t := &WebFetchTool{}
	t.spec = ToolSpec{
		ID:          "web_fetch",
		Name:        "WebFetch",
		Description: "Fetch URL content via HTTP GET",
		Category:    CatWeb,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"url": map[string]any{"type": "string"}},
			"required":   []string{"url"},
		},
		RequiredPermissions: []string{"network"},
		Timeout:             15 * time.Second,
	}
	return t
}

func (t *WebFetchTool) Spec() ToolSpec { return t.spec }

func (t *WebFetchTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	url, _ := params["url"].(string)
	if strings.TrimSpace(url) == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing url", Duration: time.Since(start)}, errors.New("missing url")
	}
	// Respect tool timeout while keeping cancellation from ctx
	var cancel context.CancelFunc
	ctx2, c := context.WithTimeout(ctx, t.spec.Timeout)
	cancel = c
	defer cancel()

	req, err := http.NewRequestWithContext(ctx2, http.MethodGet, url, nil)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	if h, ok := params["headers"].(map[string]string); ok {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: ok, Output: string(body), Duration: time.Since(start)}, nil
}

// FileReadTool reads a file from the agent workspace.
type FileReadTool struct{ spec ToolSpec }

func NewFileReadTool() *FileReadTool {
	t := &FileReadTool{}
	t.spec = ToolSpec{
		ID:          "file_read",
		Name:        "FileRead",
		Description: "Read a file from workspace",
		Category:    CatFile,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []string{"path"},
		},
		Timeout: 10 * time.Second,
	}
	return t
}
func (t *FileReadTool) Spec() ToolSpec { return t.spec }
func (t *FileReadTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	path, _ := params["path"].(string)
	if path == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing path", Duration: time.Since(start)}, errors.New("missing path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
}

// FileWriteTool writes content to a file in the workspace.
type FileWriteTool struct{ spec ToolSpec }

func NewFileWriteTool() *FileWriteTool {
	t := &FileWriteTool{}
	t.spec = ToolSpec{
		ID:          "file_write",
		Name:        "FileWrite",
		Description: "Write content to a file in workspace",
		Category:    CatFile,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}},
			"required":   []string{"path", "content"},
		},
		Timeout: 10 * time.Second,
	}
	return t
}
func (t *FileWriteTool) Spec() ToolSpec { return t.spec }
func (t *FileWriteTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)
	if path == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing path", Duration: time.Since(start)}, errors.New("missing path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: "written", Duration: time.Since(start)}, nil
}

// ShellExecTool executes a shell command.
type ShellExecTool struct{ spec ToolSpec }

func NewShellExecTool() *ShellExecTool {
	t := &ShellExecTool{}
	t.spec = ToolSpec{
		ID:          "shell_exec",
		Name:        "ShellExec",
		Description: "Execute a shell command",
		Category:    CatShell,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"command": map[string]any{"type": "string"}, "workdir": map[string]any{"type": "string"}},
			"required":   []string{"command"},
		},
		Timeout: 30 * time.Second,
	}
	return t
}
func (t *ShellExecTool) Spec() ToolSpec { return t.spec }
func (t *ShellExecTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	cmdStr, _ := params["command"].(string)
	workdir, _ := params["workdir"].(string)
	if cmdStr == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing command", Duration: time.Since(start)}, errors.New("missing command")
	}
	ctx2, cancel := context.WithTimeout(ctx, t.spec.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx2, "bash", "-lc", cmdStr)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	combined := out.String() + errOut.String()
	if ctx2.Err() == context.DeadlineExceeded {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Output: combined, Error: "command timeout", Duration: time.Since(start)}, ctx2.Err()
	}
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Output: combined, Error: err.Error(), Duration: time.Since(start)}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: combined, Duration: time.Since(start)}, nil
}

// HTTPRequestTool performs an HTTP request with configurable method/headers/body.
type HTTPRequestTool struct{ spec ToolSpec }

func NewHTTPRequestTool() *HTTPRequestTool {
	t := &HTTPRequestTool{}
	t.spec = ToolSpec{
		ID:          "http_request",
		Name:        "HTTPRequest",
		Description: "Make an arbitrary HTTP request",
		Category:    CatHTTP,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"method": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"}, "headers": map[string]any{"type": "object"}, "body": map[string]any{"type": "string"}},
			"required":   []string{"method", "url"},
		},
		Timeout: 20 * time.Second,
	}
	return t
}
func (t *HTTPRequestTool) Spec() ToolSpec { return t.spec }
func (t *HTTPRequestTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	method, _ := params["method"].(string)
	url, _ := params["url"].(string)
	if url == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing url", Duration: time.Since(start)}, errors.New("missing url")
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), url, strings.NewReader(coerceString(params["body"])))
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	if h, ok := params["headers"].(map[string]string); ok {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: ok, Output: string(b), Duration: time.Since(start)}, nil
}

// JSONParseTool parses JSON data and optionally queries it with a dot-path.
type JSONParseTool struct{ spec ToolSpec }

func NewJSONParseTool() *JSONParseTool {
	t := &JSONParseTool{}
	t.spec = ToolSpec{
		ID:          "json_parse",
		Name:        "JSONParse",
		Description: "Parse JSON and select a path",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"json": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}},
			"required":   []string{"json"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *JSONParseTool) Spec() ToolSpec { return t.spec }
func (t *JSONParseTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	dataStr, _ := params["json"].(string)
	var root interface{}
	if err := json.Unmarshal([]byte(dataStr), &root); err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
	}
	path, _ := params["path"].(string)
	if path != "" {
		parts := strings.Split(path, ".")
		cur := root
		for _, p := range parts {
			if m, ok := cur.(map[string]interface{}); ok {
				cur = m[p]
			} else {
				cur = nil
				break
			}
		}
		if cur == nil {
			return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "path not found", Duration: time.Since(start)}, nil
		}
		b, _ := json.Marshal(cur)
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
	}
	b, _ := json.Marshal(root)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b), Duration: time.Since(start)}, nil
}

// WaitTool sleeps for a given duration (seconds).
type WaitTool struct{ spec ToolSpec }

func NewWaitTool() *WaitTool {
	t := &WaitTool{}
	t.spec = ToolSpec{
		ID:          "wait",
		Name:        "Wait",
		Description: "Sleep for a duration",
		Category:    CatSystem,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"duration": map[string]any{"type": "number"}},
			"required":   []string{"duration"},
		},
		Timeout: 60 * time.Second,
	}
	return t
}
func (t *WaitTool) Spec() ToolSpec { return t.spec }
func (t *WaitTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	dur, _ := params["duration"].(float64)
	if dur <= 0 {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "invalid duration", Duration: time.Since(start)}, nil
	}
	select {
	case <-ctx.Done():
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: ctx.Err().Error(), Duration: time.Since(start)}, ctx.Err()
	case <-time.After(time.Duration(dur) * time.Second):
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: "wait complete", Duration: time.Since(start)}, nil
	}
}

// Helpers
func coerceString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
