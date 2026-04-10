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

type TogetherAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
	models  map[string]providers.ModelInfo
}

func NewTogetherAdapter(apiKey string) *TogetherAdapter {
	return &TogetherAdapter{
		apiKey:  apiKey,
		baseURL: "https://api.together.xyz/v1",
		client:  &http.Client{Timeout: 120 * time.Second},
		models:  defaultTogetherModels(),
	}
}

func defaultTogetherModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"meta-llama/Llama-3.3-70B-Instruct-Turbo": {
			ID: "meta-llama/Llama-3.3-70B-Instruct-Turbo", MaxContextTokens: 128000,
			Capabilities:   []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput: 0.00088, CostPer1kOutput: 0.00264,
		},
		"meta-llama/Llama-3.1-405B-Instruct-Turbo": {
			ID: "meta-llama/Llama-3.1-405B-Instruct-Turbo", MaxContextTokens: 128000,
			Capabilities:   []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput: 0.0035, CostPer1kOutput: 0.0035,
		},
		"Qwen/Qwen2.5-72B-Instruct-Turbo": {
			ID: "Qwen/Qwen2.5-72B-Instruct-Turbo", MaxContextTokens: 32000,
			Capabilities:   []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput: 0.00088, CostPer1kOutput: 0.00264,
		},
		"deepseek-ai/DeepSeek-R1": {
			ID: "deepseek-ai/DeepSeek-R1", MaxContextTokens: 128000,
			Capabilities:   []providers.CapabilityFlag{providers.CapReasoning, providers.CapStreaming, providers.CapSystemMsg},
			CostPer1kInput: 0.0035, CostPer1kOutput: 0.0035,
		},
		"mistralai/Mixtral-8x7B-Instruct-v0.1": {
			ID: "mistralai/Mixtral-8x7B-Instruct-v0.1", MaxContextTokens: 32000,
			Capabilities:   []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput: 0.0006, CostPer1kOutput: 0.0006,
		},
		"google/gemma-2-27b-it": {
			ID: "google/gemma-2-27b-it", MaxContextTokens: 8000,
			Capabilities:   []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput: 0.0008, CostPer1kOutput: 0.0008,
		},
	}
}

func (a *TogetherAdapter) ID() providers.ProviderID { return "together" }

func (a *TogetherAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg}
}

func (a *TogetherAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *TogetherAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
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

func (a *TogetherAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *TogetherAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("together: api_key is required")
	}
	return nil
}

func (a *TogetherAdapter) NormalizeModel(model string) string {
	if info, ok := a.models[model]; ok {
		return info.ID
	}
	return model
}

func (a *TogetherAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func (a *TogetherAdapter) convertMessages(req providers.GenerateRequest) []openaiMessage {
	var messages []openaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (a *TogetherAdapter) doRequest(ctx context.Context, oaiReq *openaiRequest) (*providers.GenerateResponse, error) {
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
		return nil, fmt.Errorf("together api error %d: %s", resp.StatusCode, string(respBody))
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
