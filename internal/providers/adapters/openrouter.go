package adapters

import (
	"fmt"

	"github.com/zclaw/zclaw/internal/providers"
)

type OpenRouterAdapter struct {
	*OpenAIAdapter
}

func NewOpenRouterAdapter(apiKey string) *OpenRouterAdapter {
	return &OpenRouterAdapter{
		OpenAIAdapter: NewOpenAIAdapter(apiKey, "https://openrouter.ai/api/v1"),
	}
}

func (a *OpenRouterAdapter) ID() providers.ProviderID { return "openrouter" }

func (a *OpenRouterAdapter) Capabilities() []providers.CapabilityFlag {
	return []providers.CapabilityFlag{
		providers.CapTools, providers.CapStreaming, providers.CapVision,
		providers.CapJSON, providers.CapSystemMsg,
	}
}

func (a *OpenRouterAdapter) HasCapability(cap providers.CapabilityFlag) bool {
	for _, c := range a.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (a *OpenRouterAdapter) ValidateConfig(config providers.ProviderConfig) error {
	if config.APIKey == "" {
		return fmt.Errorf("openrouter: api_key is required")
	}
	return nil
}

func (a *OpenRouterAdapter) NormalizeModel(model string) string {
	prefixAliases := map[string]string{
		"anthropic/claude-4-sonnet": "anthropic/claude-sonnet-4-20250514",
		"google/gemini-2":           "google/gemini-2.0-flash-001",
		"meta/llama3":               "meta-llama/llama-3.3-70b-instruct",
	}
	if canonical, ok := prefixAliases[model]; ok {
		return canonical
	}
	return model
}

func (a *OpenRouterAdapter) Close() error {
	return a.OpenAIAdapter.Close()
}
