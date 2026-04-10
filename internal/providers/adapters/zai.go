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

// ZAIAdapter implements a OpenAI-compatible provider adapter for Z.AI (Zhipu AI).
type ZAIAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
	models  map[string]providers.ModelInfo
	aliases map[string]string
}

// NewZAIAdapter creates a new adapter with the default global base URL.
func NewZAIAdapter(apiKey string) *ZAIAdapter {
	return NewZAIAdapterWithEndpoint(apiKey, "https://api.z.ai/api/paas/v4")
}

// NewZAIAdapterWithEndpoint creates a new adapter with a custom base URL.
func NewZAIAdapterWithEndpoint(apiKey, baseURL string) *ZAIAdapter {
	a := &ZAIAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
		models:  defaultZAIModels(),
		aliases: make(map[string]string),
	}
	a.initAliases()
	return a
}

// defaultZAIModels defines all models with costs, context, and capabilities.
func defaultZAIModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		// glm family (131K tokens context unless noted otherwise)
		"glm-5.1": {
			ID:               "glm-5.1",
			Aliases:          []string{"glm5.1", "glm5.1.0"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0014,
			CostPer1kOutput:  0.0044,
			// Flags added inline for compatibility with OpenAI-like models
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-5": {
			ID:               "glm-5",
			Aliases:          []string{"glm5"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0010,
			CostPer1kOutput:  0.0032,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-5-turbo": {
			ID:               "glm-5-turbo",
			Aliases:          []string{"glm5-turbo"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0012,
			CostPer1kOutput:  0.0040,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
		},
		// glm family older generations
		"glm-4.7": {
			ID:               "glm-4.7",
			Aliases:          []string{"glm4.7"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0006,
			CostPer1kOutput:  0.0022,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.7-flash": {
			ID:               "glm-4.7-flash",
			Aliases:          []string{"glm4.7-flash"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0,
			CostPer1kOutput:  0.0,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.7-flashx": {
			ID:               "glm-4.7-flashx",
			Aliases:          []string{"glm4.7-flashx"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.00007,
			CostPer1kOutput:  0.00040,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.6": {
			ID:               "glm-4.6",
			Aliases:          []string{"glm4.6"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0006,
			CostPer1kOutput:  0.0022,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.5": {
			ID:               "glm-4.5",
			Aliases:          []string{"glm4.5"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0006,
			CostPer1kOutput:  0.0022,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.5-air": {
			ID:               "glm-4.5-air",
			Aliases:          []string{"glm4.5-air"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0002,
			CostPer1kOutput:  0.0011,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.5-x": {
			ID:               "glm-4.5-x",
			Aliases:          []string{"glm4.5-x"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0022,
			CostPer1kOutput:  0.0089,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.5-airx": {
			ID:               "glm-4.5-airx",
			Aliases:          []string{"glm4.5-airx"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0011,
			CostPer1kOutput:  0.0045,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"glm-4.5-flash": {
			ID:               "glm-4.5-flash",
			Aliases:          []string{"glm4.5-flash"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0,
			CostPer1kOutput:  0.0,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		// 4.32B 128K context
		"glm-4-32b-0414-128k": {
			ID:               "glm-4-32b-0414-128k",
			Aliases:          []string{"glm4-32b-0414-128k"},
			MaxContextTokens: 128000,
			CostPer1kInput:   0.0001,
			CostPer1kOutput:  0.0001,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		// Vision-capable variants
		"glm-5v-turbo": {
			ID:               "glm-5v-turbo",
			Aliases:          []string{"glm5v-turbo"},
			MaxContextTokens: 131000,
			CostPer1kInput:   0.0012,
			CostPer1kOutput:  0.0040,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
		},
		"glm-4.6v": {
			ID:               "glm-4.6v",
			Aliases:          []string{"glm4.6v"},
			MaxContextTokens: 32000,
			CostPer1kInput:   0.0003,
			CostPer1kOutput:  0.0009,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
		},
		"glm-4.6v-flash": {
			ID:               "glm-4.6v-flash",
			Aliases:          []string{"glm4.6v-flash"},
			MaxContextTokens: 32000,
			CostPer1kInput:   0.0,
			CostPer1kOutput:  0.0,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
		},
		"glm-4.6v-flashx": {
			ID:               "glm-4.6v-flashx",
			Aliases:          []string{"glm4.6v-flashx"},
			MaxContextTokens: 32000,
			CostPer1kInput:   0.00004,
			CostPer1kOutput:  0.00040,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
		},
		"glm-4.5v": {
			ID:               "glm-4.5v",
			Aliases:          []string{"glm4.5v"},
			MaxContextTokens: 16000,
			CostPer1kInput:   0.0006,
			CostPer1kOutput:  0.0018,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
		},
		// Optional: a few more alias-friendly entries could map to existing IDs
	}
}

func (a *ZAIAdapter) initAliases() {
	// Map common aliases to canonical IDs
	for k, v := range a.models {
		// Populate alias map with each alias -> canonical ID
		for _, alias := range v.Aliases {
			a.aliases[alias] = k
		}
	}
}

func (a *ZAIAdapter) ID() providers.ProviderID { return "zai" }

func (a *ZAIAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapVision,
		providers.CapJSON, providers.CapSystemMsg, providers.CapReasoning,
	}
}

func (a *ZAIAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type zaiMessage struct {
	Role             string        `json:"role"`
	Content          any           `json:"content"`
	ToolCalls        []zaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
}

type zaiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function zaiFunction `json:"function"`
}

type zaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type zaiTool struct {
	Type     string    `json:"type"`
	Function zaiToolFn `json:"function"`
}

type zaiToolFn struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type zaiRequest struct {
	Model       string       `json:"model"`
	Messages    []zaiMessage `json:"messages"`
	Tools       []zaiTool    `json:"tools,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Stream      bool         `json:"stream"`
	Stop        []string     `json:"stop,omitempty"`
}

type zaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      zaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

// Generate implements the Generate call for ZAI.
func (a *ZAIAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	messages := a.convertMessages(req)
	var temp *float64
	if req.Temperature != nil {
		v := *req.Temperature
		temp = &v
	}
	oaiReq := zaiRequest{
		Model:       a.NormalizeModel(req.Model),
		Messages:    messages,
		Temperature: temp,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zai api error %d: %s", resp.StatusCode, string(respBody))
	}

	var zaiResp zaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&zaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(zaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := zaiResp.Choices[0]
	// Build the final response
	result := &providers.GenerateResponse{
		Usage: providers.Usage{
			PromptTokens:     zaiResp.Usage.PromptTokens,
			CompletionTokens: zaiResp.Usage.CompletionTokens,
			TotalTokens:      zaiResp.Usage.TotalTokens,
		},
		Model:        zaiResp.Model,
		FinishReason: choice.FinishReason,
		ProviderID:   string(a.ID()),
	}

	var content string
	// content may be string or other JSON; format safely
	switch v := choice.Message.Content.(type) {
	case string:
		content = v
	default:
		if v != nil {
			b, _ := json.Marshal(v)
			content = string(b)
		}
	}
	// Append any reasoning_content if present
	if choice.Message.ReasoningContent != "" {
		if content != "" {
			content = content + "\n" + choice.Message.ReasoningContent
		} else {
			content = choice.Message.ReasoningContent
		}
	}

	result.Message = providers.Message{
		Role:      providers.MessageRole(choice.Message.Role),
		Content:   content,
		Timestamp: time.Now().UTC(),
	}

	// ToolCalls conversion (if any)
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
			result.Message.ToolCalls = append(result.Message.ToolCalls, providers.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

// GenerateStream provides a simple streaming wrapper around Generate.
func (a *ZAIAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *ZAIAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("zai: api_key is required")
	}
	return nil
}

func (a *ZAIAdapter) NormalizeModel(model string) string {
	if info, ok := a.models[model]; ok {
		return info.ID
	}
	for _, info := range a.models {
		for _, alias := range info.Aliases {
			if alias == model {
				return info.ID
			}
		}
	}
	return model
}

func (a *ZAIAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

// convertMessages converts a provider GenerateRequest into zaiMessage slice.
func (a *ZAIAdapter) convertMessages(req providers.GenerateRequest) []zaiMessage {
	var messages []zaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, zaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		oaiMsg := zaiMessage{Role: string(m.Role), Content: m.Content}
		if m.ToolResult != nil {
			oaiMsg.ToolCallID = m.ToolResult.CallID
			oaiMsg.Content = m.ToolResult.Result
		}
		messages = append(messages, oaiMsg)
	}
	return messages
}

// mustMarshal is a tiny helper to marshal a json.RawMessage-like input safely.
func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
