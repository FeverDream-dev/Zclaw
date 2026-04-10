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

type DeepSeekAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
	models  map[string]providers.ModelInfo
}

func NewDeepSeekAdapter(apiKey string) *DeepSeekAdapter {
	return &DeepSeekAdapter{
		apiKey:  apiKey,
		baseURL: "https://api.deepseek.com",
		client:  &http.Client{Timeout: 180 * time.Second},
		models:  defaultDeepSeekModels(),
	}
}

func defaultDeepSeekModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"deepseek-chat": {
			ID: "deepseek-chat", Aliases: []string{"deepseek-v3", "deepseek"},
			MaxContextTokens: 65536,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00027, CostPer1kOutput: 0.0011,
		},
		"deepseek-reasoner": {
			ID: "deepseek-reasoner", Aliases: []string{"deepseek-r1"},
			MaxContextTokens: 65536,
			Capabilities:     []providers.CapabilityFlag{providers.CapReasoning, providers.CapStreaming, providers.CapSystemMsg},
			CostPer1kInput:   0.00055, CostPer1kOutput: 0.00219,
		},
		"deepseek-coder": {
			ID: "deepseek-coder", Aliases: []string{"deepseek-coder-v2"},
			MaxContextTokens: 131072,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00014, CostPer1kOutput: 0.00028,
		},
		"deepseek-coder-v2-lite": {
			ID:               "deepseek-coder-v2-lite",
			MaxContextTokens: 131072,
			Capabilities:     []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00008, CostPer1kOutput: 0.00016,
		},
	}
}

func (a *DeepSeekAdapter) ID() providers.ProviderID { return "deepseek" }

func (a *DeepSeekAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg, providers.CapReasoning}
}

func (a *DeepSeekAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *DeepSeekAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	oaiReq := openaiRequest{
		Model:       a.NormalizeModel(req.Model),
		Messages:    a.convertMessages(req),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}
	for _, t := range req.Tools {
		oaiReq.Tools = append(oaiReq.Tools, openaiTool{Type: "function", Function: openaiToolFn{Name: t.Name, Description: t.Description, Parameters: t.Parameters}})
	}
	return a.doRequest(ctx, &oaiReq)
}

func (a *DeepSeekAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk, 64)
	go func() {
		defer close(ch)
		resp, err := a.Generate(ctx, req)
		if err != nil {
			ch <- providers.StreamChunk{Done: true, FinishReason: "error"}
			return
		}
		ch <- providers.StreamChunk{Content: resp.Message.Content, ToolCalls: resp.Message.ToolCalls, Done: true, FinishReason: resp.FinishReason, Model: resp.Model, Usage: &resp.Usage}
	}()
	return ch, nil
}

func (a *DeepSeekAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("deepseek: api_key is required")
	}
	return nil
}

func (a *DeepSeekAdapter) NormalizeModel(model string) string {
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

func (a *DeepSeekAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func (a *DeepSeekAdapter) convertMessages(req providers.GenerateRequest) []openaiMessage {
	var messages []openaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (a *DeepSeekAdapter) doRequest(ctx context.Context, oaiReq *openaiRequest) (*providers.GenerateResponse, error) {
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/chat/completions", bytes.NewReader(body))
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
		return nil, fmt.Errorf("deepseek api error %d: %s", resp.StatusCode, string(respBody))
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
		Usage: providers.Usage{PromptTokens: oaiResp.Usage.PromptTokens, CompletionTokens: oaiResp.Usage.CompletionTokens, TotalTokens: oaiResp.Usage.TotalTokens},
		Model: oaiResp.Model, FinishReason: choice.FinishReason, ProviderID: string(a.ID()),
	}
	result.Message = providers.Message{Role: providers.MessageRole(choice.Message.Role), Content: fmt.Sprintf("%v", choice.Message.Content), Timestamp: time.Now().UTC()}
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		result.Message.ToolCalls = append(result.Message.ToolCalls, providers.ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: args})
	}
	return result, nil
}
