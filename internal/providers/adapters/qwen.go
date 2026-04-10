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

type QwenAdapter struct {
	baseURL string
	client  *http.Client
	key     string
	models  map[string]providers.ModelInfo
}

func NewQwenAdapter(apiKey string) *QwenAdapter {
	return &QwenAdapter{
		baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		client:  &http.Client{Timeout: 120 * time.Second},
		key:     apiKey,
		models:  defaultQwenModels(),
	}
}

func defaultQwenModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"qwen-max":    {ID: "qwen-max"},
		"qwen-plus":   {ID: "qwen-plus"},
		"qwen-turbo":  {ID: "qwen-turbo"},
		"qwen-vl-max": {ID: "qwen-vl-max"},
	}
}

func (a *QwenAdapter) ID() providers.ProviderID { return "qwen" }

func (a *QwenAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON}
}

func (a *QwenAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type qwenRequest struct {
	Model    string              `json:"model"`
	Contents []map[string]string `json:"contents"`
	Stream   bool                `json:"stream"`
}

type qwenResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

func (a *QwenAdapter) NormalizeModel(model string) string {
	if m, ok := a.models[model]; ok {
		return m.ID
	}
	return model
}

func (a *QwenAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	// Build minimal Qwen request payload
	contents := []map[string]string{}
	for _, m := range req.Messages {
		contents = append(contents, map[string]string{"role": string(m.Role), "text": m.Content})
	}
	payload := qwenRequest{Model: a.NormalizeModel(req.Model), Contents: contents, Stream: req.Stream}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal qwen request: %w", err)
	}
	url := a.baseURL + "/generate"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.key != "" {
		q := httpReq.URL.Query()
		q.Add("key", a.key)
		httpReq.URL.RawQuery = q.Encode()
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var gr qwenResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("decode qwen response: %w", err)
	}
	text := ""
	if len(gr.Choices) > 0 {
		text = gr.Choices[0].Text
	}
	respOut := &providers.GenerateResponse{
		Message:    providers.Message{Role: providers.RoleAssistant, Content: text, Timestamp: time.Now().UTC()},
		Usage:      providers.Usage{TotalTokens: gr.Usage.TotalTokens},
		Model:      gr.Model,
		ProviderID: string(a.ID()),
	}
	return respOut, nil
}

func (a *QwenAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *QwenAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("qwen: api_key is required")
	}
	return nil
}

func (a *QwenAdapter) Close() error { return nil }
