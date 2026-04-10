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

type MiniMaxAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
	models  map[string]providers.ModelInfo
}

func NewMiniMaxAdapter(apiKey string) *MiniMaxAdapter {
	return &MiniMaxAdapter{
		apiKey:  apiKey,
		baseURL: "https://api.minimax.chat/v1",
		client:  &http.Client{Timeout: 120 * time.Second},
		models:  defaultMiniMaxModels(),
	}
}

func defaultMiniMaxModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"MiniMax-Text-01": {
			ID: "MiniMax-Text-01", MaxContextTokens: 1000000,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		"MiniMax-M1": {
			ID: "MiniMax-M1", MaxContextTokens: 1000000,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg, providers.CapReasoning},
		},
		"abab6.5s-chat": {
			ID: "abab6.5s-chat", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		"abab6.5-chat": {
			ID: "abab6.5-chat", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
		"abab6.5g-chat": {
			ID: "abab6.5g-chat", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
	}
}

func (a *MiniMaxAdapter) ID() providers.ProviderID { return "minimax" }

func (a *MiniMaxAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg}
}

func (a *MiniMaxAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *MiniMaxAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
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

func (a *MiniMaxAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *MiniMaxAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("minimax: api_key is required")
	}
	return nil
}

func (a *MiniMaxAdapter) NormalizeModel(model string) string {
	if info, ok := a.models[model]; ok {
		return info.ID
	}
	return model
}

func (a *MiniMaxAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func (a *MiniMaxAdapter) convertMessages(req providers.GenerateRequest) []openaiMessage {
	var messages []openaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (a *MiniMaxAdapter) doRequest(ctx context.Context, oaiReq *openaiRequest) (*providers.GenerateResponse, error) {
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/text/chatcompletion_v2", bytes.NewReader(body))
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
		return nil, fmt.Errorf("minimax api error %d: %s", resp.StatusCode, string(respBody))
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
