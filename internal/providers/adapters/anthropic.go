package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zclaw/zclaw/internal/providers"
)

type AnthropicAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewAnthropicAdapter(apiKey, baseURL string) *AnthropicAdapter {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &AnthropicAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *AnthropicAdapter) ID() providers.ProviderID { return "anthropic" }

func (a *AnthropicAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapVision,
		providers.CapJSON, providers.CapSystemMsg,
	}
}

func (a *AnthropicAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

func (a *AnthropicAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	model := req.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	anthropicReq := anthropicRequest{
		Model:     model,
		MaxTokens: orDefault(req.MaxTokens, 4096),
		System:    req.SystemPrompt,
		Stream:    false,
	}

	for _, m := range req.Messages {
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	for _, t := range req.Tools {
		anthropicReq.Tools = append(anthropicReq.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	if req.Temperature != nil {
		anthropicReq.Temperature = req.Temperature
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic api error %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	result := &providers.GenerateResponse{
		Model:        apiResp.Model,
		FinishReason: apiResp.StopReason,
		ProviderID:   string(a.ID()),
		Usage: providers.Usage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		},
	}

	var content string
	var toolCalls []providers.ToolCall
	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			var args map[string]any
			json.Unmarshal(block.Input, &args)
			toolCalls = append(toolCalls, providers.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	result.Message = providers.Message{
		Role:      providers.RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
		Timestamp: time.Now().UTC(),
	}

	return result, nil
}

func (a *AnthropicAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk, 64)
	go func() {
		defer close(ch)
		resp, err := a.Generate(ctx, req)
		if err != nil {
			ch <- providers.StreamChunk{Done: true, FinishReason: "error"}
			return
		}
		ch <- providers.StreamChunk{
			Content:      resp.Message.Content,
			ToolCalls:    resp.Message.ToolCalls,
			Done:         true,
			FinishReason: resp.FinishReason,
			Model:        resp.Model,
			Usage:        &resp.Usage,
		}
	}()
	return ch, nil
}

func (a *AnthropicAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("anthropic: api_key is required")
	}
	return nil
}

func (a *AnthropicAdapter) NormalizeModel(model string) string {
	aliases := map[string]string{
		"claude-4-sonnet":   "claude-sonnet-4-20250514",
		"claude-4-opus":     "claude-opus-4-20250514",
		"claude-3.5":        "claude-3-5-sonnet-20241022",
		"claude-3.5-sonnet": "claude-3-5-sonnet-20241022",
		"claude-3-haiku":    "claude-3-5-haiku-20241022",
	}
	if canonical, ok := aliases[model]; ok {
		return canonical
	}
	return model
}

func (a *AnthropicAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
