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

type OllamaCloudAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
	models  map[string]providers.ModelInfo
}

func NewOllamaCloudAdapter(apiKey string) *OllamaCloudAdapter {
	return &OllamaCloudAdapter{
		apiKey:  apiKey,
		baseURL: "https://ollama.com/api",
		client:  &http.Client{Timeout: 300 * time.Second},
		models:  defaultOllamaCloudModels(),
	}
}

func NewOllamaCloudAdapterWithBaseURL(apiKey, baseURL string) *OllamaCloudAdapter {
	if baseURL == "" {
		baseURL = "https://ollama.com/api"
	}
	return &OllamaCloudAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 300 * time.Second},
		models:  defaultOllamaCloudModels(),
	}
}

func defaultOllamaCloudModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"gpt-oss:120b": {
			ID: "gpt-oss:120b", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"gpt-oss:20b": {
			ID: "gpt-oss:20b", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"deepseek-v3.1:671b": {
			ID: "deepseek-v3.1:671b", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapSystemMsg},
		},
		"deepseek-v3.2": {
			ID: "deepseek-v3.2", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"qwen3-coder-next": {
			ID: "qwen3-coder-next", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapSystemMsg},
		},
		"glm-5.1": {
			ID: "glm-5.1", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"kimi-k2.5": {
			ID: "kimi-k2.5", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapVision, providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"devstral-small-2:24b": {
			ID: "devstral-small-2:24b", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapVision, providers.CapTools, providers.CapSystemMsg},
		},
		"devstral-2:123b": {
			ID: "devstral-2:123b", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapSystemMsg},
		},
		"nemotron-3-super:120b": {
			ID: "nemotron-3-super:120b", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"gemini-3-flash-preview": {
			ID: "gemini-3-flash-preview", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapVision, providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
		"minimax-m2.5": {
			ID: "minimax-m2.5", MaxContextTokens: 131072,
			Capabilities: []providers.CapabilityFlag{providers.CapTools, providers.CapReasoning, providers.CapSystemMsg},
		},
	}
}

func (a *OllamaCloudAdapter) ID() providers.ProviderID { return "ollama-cloud" }

func (a *OllamaCloudAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapReasoning, providers.CapSystemMsg,
	}
}

func (a *OllamaCloudAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

type ollamaCloudReq struct {
	Model    string            `json:"model"`
	Messages []ollamaCloudMsg  `json:"messages"`
	Stream   bool              `json:"stream"`
	Tools    []ollamaCloudTool `json:"tools,omitempty"`
	Options  ollamaCloudOpts   `json:"options,omitempty"`
}

type ollamaCloudMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaCloudTool struct {
	Type     string            `json:"type"`
	Function ollamaCloudToolFn `json:"function"`
}

type ollamaCloudToolFn struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ollamaCloudOpts struct {
	Temperature *float64 `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
}

type ollamaCloudResp struct {
	Model           string         `json:"model"`
	Message         ollamaCloudMsg `json:"message"`
	Done            bool           `json:"done"`
	DoneReason      string         `json:"done_reason,omitempty"`
	TotalDuration   int64          `json:"total_duration,omitempty"`
	EvalCount       int            `json:"eval_count,omitempty"`
	PromptEvalCount int            `json:"prompt_eval_count,omitempty"`
}

func (a *OllamaCloudAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	model := req.Model
	if model == "" {
		model = "gpt-oss:120b"
	}

	oReq := ollamaCloudReq{Model: model, Stream: false}
	if req.SystemPrompt != "" {
		oReq.Messages = append(oReq.Messages, ollamaCloudMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		oReq.Messages = append(oReq.Messages, ollamaCloudMsg{Role: string(m.Role), Content: m.Content})
	}
	for _, t := range req.Tools {
		oReq.Tools = append(oReq.Tools, ollamaCloudTool{
			Type:     "function",
			Function: ollamaCloudToolFn{Name: t.Name, Description: t.Description, Parameters: t.Parameters},
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat", bytes.NewReader(body))
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
		return nil, fmt.Errorf("ollama-cloud api error %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp ollamaCloudResp
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
		FinishReason: apiResp.DoneReason,
		ProviderID:   string(a.ID()),
	}, nil
}

func (a *OllamaCloudAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *OllamaCloudAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("ollama-cloud: api_key is required")
	}
	return nil
}

func (a *OllamaCloudAdapter) NormalizeModel(model string) string {
	if info, ok := a.models[model]; ok {
		return info.ID
	}
	return model
}

func (a *OllamaCloudAdapter) Close() error {
	a.client.CloseIdleConnections()
	return nil
}
