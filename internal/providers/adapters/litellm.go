package adapters

import (
	"fmt"

	"github.com/zclaw/zclaw/internal/providers"
)

type LiteLLMAdapter struct {
	*OpenAIAdapter
}

func NewLiteLLMAdapter(apiKey, baseURL string) *LiteLLMAdapter {
	if baseURL == "" {
		baseURL = "http://localhost:4000/v1"
	}
	return &LiteLLMAdapter{
		OpenAIAdapter: NewOpenAIAdapter(apiKey, baseURL),
	}
}

func (a *LiteLLMAdapter) ID() providers.ProviderID { return "litellm" }

func (a *LiteLLMAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapVision,
		providers.CapJSON, providers.CapSystemMsg,
	}
}

func (a *LiteLLMAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *LiteLLMAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.BaseURL == "" {
		return fmt.Errorf("litellm: base_url is required")
	}
	return nil
}

func (a *LiteLLMAdapter) NormalizeModel(model string) string {
	return model
}

func (a *LiteLLMAdapter) Close() error {
	return a.OpenAIAdapter.Close()
}
