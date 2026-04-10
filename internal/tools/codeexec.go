package tools

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PythonExecTool runs Python code in a subprocess. It accepts either a "code" string
// containing Python source or a "script" path to an existing file.
type PythonExecTool struct{ spec ToolSpec }

func NewPythonExecTool() *PythonExecTool {
	t := &PythonExecTool{}
	t.spec = ToolSpec{
		ID:          "python_exec",
		Name:        "PythonExec",
		Description: "Execute Python code in a subprocess",
		Category:    CatCode,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"code": map[string]any{"type": "string"}, "script": map[string]any{"type": "string"}},
			"required":   []string{"code"},
		},
		Timeout: 60 * time.Second,
	}
	return t
}
func (t *PythonExecTool) Spec() ToolSpec { return t.spec }
func (t *PythonExecTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	code, _ := params["code"].(string)
	script, _ := params["script"].(string)
	var cmd *exec.Cmd
	if script != "" {
		cmd = exec.CommandContext(ctx, "python3", script)
	} else if code != "" {
		// Write code to a temp file and run it
		tmpDir := os.TempDir()
		f, err := os.CreateTemp(tmpDir, "temp_code_*.py")
		if err != nil {
			return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
		}
		if _, err := f.WriteString(code); err != nil {
			f.Close()
			return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
		}
		f.Close()
		cmd = exec.CommandContext(ctx, "python3", f.Name())
		// Ensure temp file gets cleaned up after execution
		defer os.Remove(f.Name())
	} else {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "no code or script provided", Duration: time.Since(start)}, nil
	}
	output, err := cmd.CombinedOutput()
	dur := time.Since(start)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Output: string(output), Error: err.Error(), Duration: dur}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(output), Duration: dur}, nil
}

// JavaScriptExecTool runs Node.js code in a subprocess.
type JavaScriptExecTool struct{ spec ToolSpec }

func NewJavaScriptExecTool() *JavaScriptExecTool {
	t := &JavaScriptExecTool{}
	t.spec = ToolSpec{
		ID:          "js_exec",
		Name:        "JavaScriptExec",
		Description: "Execute JavaScript code with Node.js",
		Category:    CatCode,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"code": map[string]any{"type": "string"}, "script": map[string]any{"type": "string"}},
			"required":   []string{"code"},
		},
		Timeout: 60 * time.Second,
	}
	return t
}
func (t *JavaScriptExecTool) Spec() ToolSpec { return t.spec }
func (t *JavaScriptExecTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	code, _ := params["code"].(string)
	script, _ := params["script"].(string)
	var cmd *exec.Cmd
	if script != "" {
		cmd = exec.CommandContext(ctx, "node", script)
	} else if code != "" {
		tmp, err := os.CreateTemp(os.TempDir(), "temp_js_*.js")
		if err != nil {
			return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
		}
		if _, err := tmp.WriteString(code); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error(), Duration: time.Since(start)}, err
		}
		tmp.Close()
		cmd = exec.CommandContext(ctx, "node", tmp.Name())
		defer os.Remove(tmp.Name())
	} else {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "no code or script provided", Duration: time.Since(start)}, nil
	}
	out, err := cmd.CombinedOutput()
	dur := time.Since(start)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Output: string(out), Error: err.Error(), Duration: dur}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(out), Duration: dur}, nil
}

// GoEvalTool evaluates a simple Go expression by delegating to a shell arithmetic expansion.
type GoEvalTool struct{ spec ToolSpec }

func NewGoEvalTool() *GoEvalTool {
	t := &GoEvalTool{}
	t.spec = ToolSpec{
		ID:          "go_eval",
		Name:        "GoEval",
		Description: "Evaluate a simple Go expression (subset)",
		Category:    CatCode,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"expr": map[string]any{"type": "string"}},
			"required":   []string{"expr"},
		},
		Timeout: 15 * time.Second,
	}
	return t
}
func (t *GoEvalTool) Spec() ToolSpec { return t.spec }
func (t *GoEvalTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	start := time.Now()
	expr, _ := params["expr"].(string)
	if expr == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing expr", Duration: time.Since(start)}, nil
	}
	// Use shell arithmetic to evaluate a simple numeric expression
	cmd := exec.CommandContext(ctx, "bash", "-lc", "echo $(("+expr+"))")
	out, err := cmd.CombinedOutput()
	dur := time.Since(start)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Output: string(out), Error: err.Error(), Duration: dur}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: strings.TrimSpace(string(out)), Duration: dur}, nil
}
