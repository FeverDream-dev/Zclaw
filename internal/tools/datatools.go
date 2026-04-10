package tools

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"time"
)

// CSVReadTool reads a CSV file and returns a JSON array of objects (each row).
type CSVReadTool struct{ spec ToolSpec }

func NewCSVReadTool() *CSVReadTool {
	t := &CSVReadTool{}
	t.spec = ToolSpec{
		ID:          "csv_read",
		Name:        "CSVRead",
		Description: "Read CSV file into JSON",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []string{"path"},
		},
		Timeout: 15 * time.Second,
	}
	return t
}
func (t *CSVReadTool) Spec() ToolSpec { return t.spec }
func (t *CSVReadTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: "missing path"}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error()}, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error()}, err
	}
	if len(records) == 0 {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: "[]"}, nil
	}
	headers := records[0]
	out := make([]map[string]string, 0, len(records)-1)
	for _, row := range records[1:] {
		if len(row) != len(headers) {
			continue
		}
		m := make(map[string]string)
		for i, h := range headers {
			m[h] = row[i]
		}
		out = append(out, m)
	}
	b, _ := json.Marshal(out)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b)}, nil
}

// TextSearchTool searches text with a regex and returns matches.
type TextSearchTool struct{ spec ToolSpec }

func NewTextSearchTool() *TextSearchTool {
	t := &TextSearchTool{}
	t.spec = ToolSpec{
		ID:          "text_search",
		Name:        "TextSearch",
		Description: "Regex search over text",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}, "pattern": map[string]any{"type": "string"}},
			"required":   []string{"text", "pattern"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *TextSearchTool) Spec() ToolSpec { return t.spec }
func (t *TextSearchTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	text, _ := params["text"].(string)
	pattern, _ := params["pattern"].(string)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error()}, err
	}
	lines := strings.Split(text, "\n")
	matches := []string{}
	for i, line := range lines {
		if re.MatchString(line) {
			matches = append(matches, line)
			_ = i
		}
	}
	b, _ := json.Marshal(struct {
		Matches []string `json:"matches"`
	}{Matches: matches})
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b)}, nil
}

// TextReplaceTool performs find/replace on text.
type TextReplaceTool struct{ spec ToolSpec }

func NewTextReplaceTool() *TextReplaceTool {
	t := &TextReplaceTool{}
	t.spec = ToolSpec{
		ID:          "text_replace",
		Name:        "TextReplace",
		Description: "Find and replace text",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}, "old": map[string]any{"type": "string"}, "new": map[string]any{"type": "string"}},
			"required":   []string{"text", "old", "new"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *TextReplaceTool) Spec() ToolSpec { return t.spec }
func (t *TextReplaceTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	text, _ := params["text"].(string)
	oldStr, _ := params["old"].(string)
	newStr, _ := params["new"].(string)
	res := strings.ReplaceAll(text, oldStr, newStr)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: res}, nil
}

// Base64EncodeTool encodes a string to Base64.
type Base64EncodeTool struct{ spec ToolSpec }

func NewBase64EncodeTool() *Base64EncodeTool {
	t := &Base64EncodeTool{}
	t.spec = ToolSpec{
		ID:          "base64_encode",
		Name:        "Base64Encode",
		Description: "Encode to Base64",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"input": map[string]any{"type": "string"}},
			"required":   []string{"input"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *Base64EncodeTool) Spec() ToolSpec { return t.spec }
func (t *Base64EncodeTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	in, _ := params["input"].(string)
	enc := base64.StdEncoding.EncodeToString([]byte(in))
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: enc}, nil
}

// Base64DecodeTool decodes a Base64 string.
type Base64DecodeTool struct{ spec ToolSpec }

func NewBase64DecodeTool() *Base64DecodeTool {
	t := &Base64DecodeTool{}
	t.spec = ToolSpec{
		ID:          "base64_decode",
		Name:        "Base64Decode",
		Description: "Decode from Base64",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"input": map[string]any{"type": "string"}},
			"required":   []string{"input"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *Base64DecodeTool) Spec() ToolSpec { return t.spec }
func (t *Base64DecodeTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	in, _ := params["input"].(string)
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return &ToolResult{ToolID: ToolID(t.spec.ID), Success: false, Error: err.Error()}, err
	}
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b)}, nil
}

// HashTool computes SHA256 and MD5 hashes of the input.
type HashTool struct{ spec ToolSpec }

func NewHashTool() *HashTool {
	t := &HashTool{}
	t.spec = ToolSpec{
		ID:          "hash",
		Name:        "Hash",
		Description: "Compute SHA256 and MD5 hashes",
		Category:    CatData,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"input": map[string]any{"type": "string"}},
			"required":   []string{"input"},
		},
		Timeout: 5 * time.Second,
	}
	return t
}
func (t *HashTool) Spec() ToolSpec { return t.spec }
func (t *HashTool) Execute(ctx context.Context, toolID string, params map[string]any) (*ToolResult, error) {
	in, _ := params["input"].(string)
	sha := sha256.Sum256([]byte(in))
	md5sum := md5.Sum([]byte(in))
	res := map[string]string{
		"sha256": fmtHex(sha[:]),
		"md5":    fmtHex(md5sum[:]),
	}
	b, _ := json.Marshal(res)
	return &ToolResult{ToolID: ToolID(t.spec.ID), Success: true, Output: string(b)}, nil
}

// helpers
func fmtHex(b []byte) string {
	return hex.EncodeToString(b)
}
