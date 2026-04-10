package adapters

// xAI (Grok) adapter: OpenAI-compatible via Grok API.
// This adapter simply reuses the OpenAIAdapter with the Grok base URL.

import (
	"github.com/zclaw/zclaw/internal/providers"
)

type XAIAdapter struct {
	*OpenAIAdapter
}

func NewXAIAdapter(apiKey string) *XAIAdapter {
	return &XAIAdapter{OpenAIAdapter: NewOpenAIAdapter(apiKey, "https://api.x.ai/v1")}
}

func (a *XAIAdapter) ID() providers.ProviderID { return "xai" }
