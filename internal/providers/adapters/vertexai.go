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

type VertexAIAdapter struct {
	accessToken string
	projectID   string
	region      string
	baseURL     string
	client      *http.Client
	models      map[string]providers.ModelInfo
}

func NewVertexAIAdapter(accessToken, projectID, region string) *VertexAIAdapter {
	if region == "" {
		region = "us-central1"
	}
	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/endpoints/openapi", region, projectID, region)
	return &VertexAIAdapter{
		accessToken: accessToken,
		projectID:   projectID,
		region:      region,
		baseURL:     baseURL,
		client:      &http.Client{Timeout: 180 * time.Second},
		models:      defaultVertexAIModels(),
	}
}

func defaultVertexAIModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"google/gemini-2.0-flash": {
			ID: "google/gemini-2.0-flash", Aliases: []string{"gemini-2.0-flash"},
			MaxContextTokens: 1048576,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"google/gemini-2.0-flash-lite": {
			ID: "google/gemini-2.0-flash-lite", Aliases: []string{"gemini-2.0-flash-lite"},
			MaxContextTokens: 1048576,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"google/gemini-1.5-pro": {
			ID: "google/gemini-1.5-pro", Aliases: []string{"gemini-1.5-pro"},
			MaxContextTokens: 2097152,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"google/gemini-1.5-flash": {
			ID: "google/gemini-1.5-flash", Aliases: []string{"gemini-1.5-flash"},
			MaxContextTokens: 1048576,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg},
		},
		"anthropic/claude-3.5-sonnet": {
			ID: "anthropic/claude-3.5-sonnet", Aliases: []string{"claude-3.5-sonnet"},
			MaxContextTokens: 200000,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapSystemMsg},
		},
		"meta/llama-3.3-70b-instruct": {
			ID: "meta/llama-3.3-70b-instruct", Aliases: []string{"llama-3.3-70b"},
			MaxContextTokens: 131072,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapSystemMsg},
		},
		"mistral/mistral-large": {
			ID: "mistral/mistral-large", Aliases: []string{"mistral-large"},
			MaxContextTokens: 131072,
			Capabilities:     []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapSystemMsg},
		},
	}
}

func (a *VertexAIAdapter) ID() providers.ProviderID { return "vertex-ai" }

func (a *VertexAIAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapVision, providers.CapJSON, providers.CapSystemMsg}
}

func (a *VertexAIAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *VertexAIAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
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

func (a *VertexAIAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *VertexAIAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("vertex-ai: access_token (api_key) is required")
	}
	return nil
}

func (a *VertexAIAdapter) NormalizeModel(model string) string {
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

func (a *VertexAIAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}

func (a *VertexAIAdapter) convertMessages(req providers.GenerateRequest) []openaiMessage {
	var messages []openaiMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{Role: string(m.Role), Content: m.Content})
	}
	return messages
}

func (a *VertexAIAdapter) doRequest(ctx context.Context, oaiReq *openaiRequest) (*providers.GenerateResponse, error) {
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.accessToken)
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vertex-ai api error %d: %s", resp.StatusCode, string(respBody))
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
