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

type MoonshotAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
	models  map[string]providers.ModelInfo
}

func NewMoonshotAdapter(apiKey string) *MoonshotAdapter {
	return &MoonshotAdapter{
		apiKey:  apiKey,
		baseURL: "https://api.moonshot.cn/v1",
		client:  &http.Client{Timeout: 120 * time.Second},
		models:  defaultMoonshotModels(),
	}
}

func defaultMoonshotModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"moonshot-v1-8k": {
			ID: "moonshot-v1-8k", Aliases: []string{"moonshot-8k"},
			MaxContextTokens: 8192,
			Capabilities:     []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00042, CostPer1kOutput: 0.00126,
		},
		"moonshot-v1-32k": {
			ID: "moonshot-v1-32k", Aliases: []string{"moonshot-32k"},
			MaxContextTokens: 32768,
			Capabilities:     []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00084, CostPer1kOutput: 0.00252,
		},
		"moonshot-v1-128k": {
			ID: "moonshot-v1-128k", Aliases: []string{"moonshot-128k"},
			MaxContextTokens: 131072,
			Capabilities:     []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
			CostPer1kInput:   0.00168, CostPer1kOutput: 0.00504,
		},
		"kimi-latest": {
			ID: "kimi-latest", Aliases: []string{"kimi"},
			MaxContextTokens: 131072,
			Capabilities:     []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg},
		},
	}
}

func (a *MoonshotAdapter) ID() providers.ProviderID { return "moonshot" }

func (a *MoonshotAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg}
}

func (a *MoonshotAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *MoonshotAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
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

func (a *MoonshotAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk, 64)
	go func() {
		defer close(ch)
		resp, err := a.Generate(ctx, req)
		if err != nil {
			ch <- providers.StreamChunk{Done: true, FinishReason: "error"}
			return
		}
		ch <- providers.StreamChunk{Content: resp.Message.Content, Done: true, FinishReason: resp.FinishReason, Model: resp.Model, Usage: &resp.Usage}
	}()
	return ch, nil
}

func (a *MoonshotAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("moonshot: api_key is required")
	}
	return nil
}

func (a *MoonshotAdapter) NormalizeModel(model string) string {
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

func (a *MoonshotAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func (a *MoonshotAdapter) convertMessages(req providers.GenerateRequest) []openaiMessage {
	var messages []openaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (a *MoonshotAdapter) doRequest(ctx context.Context, oaiReq *openaiRequest) (*providers.GenerateResponse, error) {
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
		return nil, fmt.Errorf("moonshot api error %d: %s", resp.StatusCode, string(respBody))
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
	return result, nil
}
