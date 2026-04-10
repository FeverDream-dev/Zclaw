package adapters

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/zclaw/zclaw/internal/providers"
)

// BedrockAdapter is an AWS Bedrock adapter. Signing with SigV4 is non-trivial and requires AWS SDK.
// For now, this adapter provides the types and interfaces, but Execute is not implemented.
type BedrockAdapter struct {
	baseURL string
	client  *http.Client
	key     string
	secret  string
	models  map[string]providers.ModelInfo
}

func NewBedrockAdapter(baseURL, accessKey, secretKey string) *BedrockAdapter {
	return &BedrockAdapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
		key:     accessKey,
		secret:  secretKey,
		models:  defaultBedrockModels(),
	}
}

func defaultBedrockModels() map[string]providers.ModelInfo {
	return map[string]providers.ModelInfo{
		"claude-3-5-sonnet": {ID: "claude-3-5-sonnet"},
		"claude-3-haiku":    {ID: "claude-3-haiku"},
		"llama-3-1-70b":     {ID: "llama-3-1-70b"},
		"titan-text":        {ID: "titan-text"},
	}
}

func (a *BedrockAdapter) ID() providers.ProviderID { return "bedrock" }

func (a *BedrockAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{providers.CapTools, providers.CapStreaming, providers.CapJSON}
}

func (a *BedrockAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *BedrockAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	// Not yet implemented due to AWS SigV4 signing complexity.
	return nil, fmt.Errorf("bedrock: not yet implemented: AWS SigV4 signing requires AWS SDK")
}

func (a *BedrockAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
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

func (a *BedrockAdapter) ValidateConfig(config providers.ProviderConfig) error {
	// Accept any config for wiring; real validation would require AWS SDK specifics
	return nil
}

func (a *BedrockAdapter) NormalizeModel(model string) string {
	if _, ok := a.models[model]; ok {
		return model
	}
	return model
}

func (a *BedrockAdapter) Close() error {
	return nil
}
