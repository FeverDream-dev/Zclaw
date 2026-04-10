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

type OpenAIAdapter struct {
	apiKey  string
	baseURL string
	models  map[string]providers.ModelInfo
	client  *http.Client
}

func NewOpenAIAdapter(apiKey, baseURL string) *OpenAIAdapter {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	models := defaultOpenAIModels()
	return &OpenAIAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		models:  models,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func defaultOpenAIModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"gpt-4o": {
			ID: "gpt-4o", Aliases: []string{"gpt4o"},
			MaxContextTokens: 128000,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.0025, CostPer1kOutput: 0.01,
		},
		"gpt-4o-mini": {
			ID: "gpt-4o-mini", Aliases: []string{"gpt4o-mini", "gpt-4o-mini"},
			MaxContextTokens: 128000,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00015, CostPer1kOutput: 0.0006,
		},
		"gpt-4-turbo": {
			ID: "gpt-4-turbo", Aliases: []string{"gpt4-turbo", "gpt-4-turbo"},
			MaxContextTokens: 128000,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON},
			CostPer1kInput:   0.01, CostPer1kOutput: 0.03,
		},
		"o1": {
			ID: "o1", MaxContextTokens: 200000,
			Capabilities:   []providers.CapabilityFlag{providers.CapReasoning, providers.CapTools, providers.CapVision},
			CostPer1kInput: 0.015, CostPer1kOutput: 0.06,
		},
		"o1-mini": {
			ID: "o1-mini", Aliases: []string{"o1mini"},
			MaxContextTokens: 128000,
			Capabilities:     []providers.CapabilityFlag{providers.CapReasoning},
			CostPer1kInput:   0.003, CostPer1kOutput: 0.012,
		},
	}
}

func (a *OpenAIAdapter) ID() providers.ProviderID { return "openai" }

func (a *OpenAIAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapVision,
		providers.CapJSON, providers.CapSystemMsg, providers.CapReasoning,
	}
}

func (a *OpenAIAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string       `json:"type"`
	Function openaiToolFn `json:"function"`
}

type openaiToolFn struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Tools       []openaiTool    `json:"tools,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream"`
	Stop        []string        `json:"stop,omitempty"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

func (a *OpenAIAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	messages := a.convertMessages(req)
	oaiReq := openaiRequest{
		Model:       a.NormalizeModel(req.Model),
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			oaiReq.Tools = append(oaiReq.Tools, openaiTool{
				Type: "function",
				Function: openaiToolFn{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(body))
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
		return nil, fmt.Errorf("openai api error %d: %s", resp.StatusCode, string(respBody))
	}

	var oaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := oaiResp.Choices[0]
	result := &providers.GenerateResponse{
		Usage: providers.Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		},
		Model:        oaiResp.Model,
		FinishReason: choice.FinishReason,
		ProviderID:   string(a.ID()),
	}

	result.Message = providers.Message{
		Role:      providers.MessageRole(choice.Message.Role),
		Content:   fmt.Sprintf("%v", choice.Message.Content),
		Timestamp: time.Now().UTC(),
	}

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		result.Message.ToolCalls = append(result.Message.ToolCalls, providers.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return result, nil
}

func (a *OpenAIAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *OpenAIAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("openai: api_key is required")
	}
	return nil
}

func (a *OpenAIAdapter) NormalizeModel(model string) string {
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

func (a *OpenAIAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func (a *OpenAIAdapter) convertMessages(req providers.GenerateRequest) []openaiMessage {
	var messages []openaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		oaiMsg := openaiMessage{Role: string(m.Role), Content: m.Content}
		for _, tc := range m.ToolCalls {
			args, _ := json.Marshal(tc.Arguments)
			oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, openaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: openaiFunction{
					Name:      tc.Name,
					Arguments: string(args),
				},
			})
		}
		if m.ToolResult != nil {
			oaiMsg.ToolCallID = m.ToolResult.CallID
			oaiMsg.Content = m.ToolResult.Result
		}
		messages = append(messages, oaiMsg)
	}
	return messages
}
