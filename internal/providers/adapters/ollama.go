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

type OllamaAdapter struct {
	baseURL string
	client  *http.Client
}

func NewOllamaAdapter(baseURL string) *OllamaAdapter {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaAdapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

func (a *OllamaAdapter) ID() providers.ProviderID { return "ollama" }

func (a *OllamaAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapSystemMsg,
	}
}

func (a *OllamaAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type ollamaRequest struct {
	Model    string        `json:"model"`
	Messages []ollamaMsg   `json:"messages"`
	Stream   bool          `json:"stream"`
	Tools    []ollamaTool  `json:"tools,omitempty"`
	Options  ollamaOptions `json:"options,omitempty"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaTool struct {
	Type     string       `json:"type"`
	Function ollamaToolFn `json:"function"`
}

type ollamaToolFn struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ollamaOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ollamaResponse struct {
	Model           string    `json:"model"`
	Message         ollamaMsg `json:"message"`
	Done            bool      `json:"done"`
	TotalDuration   int64     `json:"total_duration,omitempty"`
	EvalCount       int       `json:"eval_count,omitempty"`
	PromptEvalCount int       `json:"prompt_eval_count,omitempty"`
}

func (a *OllamaAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	model := req.Model
	if model == "" {
		model = "llama3.2"
	}

	oReq := ollamaRequest{
		Model:  model,
		Stream: false,
	}

	if req.SystemPrompt != "" {
		oReq.Messages = append(oReq.Messages, ollamaMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		oReq.Messages = append(oReq.Messages, ollamaMsg{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	for _, t := range req.Tools {
		oReq.Tools = append(oReq.Tools, ollamaTool{
			Type: "function",
			Function: ollamaToolFn{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	if req.Temperature != nil {
		oReq.Options.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		oReq.Options.NumPredict = req.MaxTokens
	}

	body, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama api error %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &providers.GenerateResponse{
		Message: providers.Message{
			Role:      providers.MessageRole(apiResp.Message.Role),
			Content:   apiResp.Message.Content,
			Timestamp: time.Now().UTC(),
		},
		Usage: providers.Usage{
			PromptTokens:     apiResp.PromptEvalCount,
			CompletionTokens: apiResp.EvalCount,
			TotalTokens:      apiResp.PromptEvalCount + apiResp.EvalCount,
		},
		Model:        apiResp.Model,
		FinishReason: "stop",
		ProviderID:   string(a.ID()),
	}, nil
}

func (a *OllamaAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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
			Done:         true,
			FinishReason: resp.FinishReason,
			Model:        resp.Model,
			Usage:        &resp.Usage,
		}
	}()
	return ch, nil
}

func (a *OllamaAdapter) ValidateConfig(config providers.ProviderConfig) error {
	return nil
}

func (a *OllamaAdapter) NormalizeModel(model string) string {
	return model
}

func (a *OllamaAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}
