package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zclaw/zclaw/internal/providers"
)

type GeminiAdapter struct {
	baseURL string
	key     string
	client  *http.Client
	models  map[string]providers.ModelInfo
}

func NewGeminiAdapter(apiKey string) *GeminiAdapter {
	return &GeminiAdapter{
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		key:     apiKey,
		client:  &http.Client{Timeout: 120 * time.Second},
		models:  defaultGeminiModels(),
	}
}

func defaultGeminiModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"gemini-2.0-flash":      {ID: "gemini-2.0-flash"},
		"gemini-2.0-flash-lite": {ID: "gemini-2.0-flash-lite"},
		"gemini-1.5-pro":        {ID: "gemini-1.5-pro"},
	}
}

func (a *GeminiAdapter) ID() providers.ProviderID { return "gemini" }

func (a *GeminiAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON, providers.CapSystemMsg}
}

func (a *GeminiAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type geminiReq struct {
	Model    string                   `json:"model"`
	Contents []map[string]interface{} `json:"contents"`
	Stream   bool                     `json:"stream"`
}

type geminiResp struct {
	Choices []struct {
		Text    string `json:"text"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

func (a *GeminiAdapter) NormalizeModel(model string) string {
	if m, ok := a.models[model]; ok {
		return m.ID
	}
	return model
}

func (a *GeminiAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	// Build a simple Gemini-like request. The real API uses contents/parts; here we approximate.
	contents := []map[string]interface{}{}
	for _, m := range req.Messages {
		contents = append(contents, map[string]interface{}{"role": string(m.Role), "text": m.Content})
	}
	payload := geminiReq{
		Model:    a.NormalizeModel(req.Model),
		Contents: contents,
		Stream:   req.Stream,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}
	httpURL := a.baseURL + "/generate" // placeholder path, API-specific
	httpReq, err := http.NewRequestWithContext(ctx, "POST", httpURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.key != "" {
		// Gemini typically uses API key; include as query parameter if accepted by API
		q := httpReq.URL.Query()
		q.Add("key", a.key)
		httpReq.URL.RawQuery = q.Encode()
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Decode generically to extract text content
	var g geminiResp
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		// Return error if the API response cannot be decoded
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	text := ""
	if len(g.Choices) > 0 {
		if c := g.Choices[0]; c.Text != "" {
			text = c.Text
		} else if c := c.Message.Content; c != "" {
			text = c
		}
	}

	respOut := &providers.GenerateResponse{
		Usage:      providers.Usage{TotalTokens: g.Usage.TotalTokens, PromptTokens: g.Usage.PromptTokens, CompletionTokens: g.Usage.CompletionTokens},
		Model:      g.Model,
		ProviderID: string(a.ID()),
		Message:    providers.Message{Role: providers.RoleAssistant, Content: text, Timestamp: time.Now().UTC()},
	}
	return respOut, nil
}

func (a *GeminiAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk, 1)
	go func() {
		defer close(ch)
		resp, err := a.Generate(ctx, req)
		if err != nil {
			ch <- providers.StreamChunk{Done: true, FinishReason: err.Error()}
			return
		}
		ch <- providers.StreamChunk{Content: resp.Message.Content, Done: true, Model: resp.Model, Usage: &resp.Usage, FinishReason: resp.FinishReason}
	}()
	return ch, nil
}

func (a *GeminiAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("gemini: api_key is required")
	}
	return nil
}

func (a *GeminiAdapter) Close() error {
	// Nothing special to close
	return nil
}
