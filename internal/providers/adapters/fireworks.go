package adapters

import (
	"github.com/zclaw/zclaw/internal/providers"
)

// FireworksAdapter provides OpenAI-compatible access to Fireworks AI API.
type FireworksAdapter struct {
	*OpenAIAdapter
}

func NewFireworksAdapter(apiKey string) *FireworksAdapter {
	return &FireworksAdapter{OpenAIAdapter: NewOpenAIAdapter(apiKey, "https://api.fireworks.ai/inference/v1")}
}

func (a *FireworksAdapter) ID() providers.ProviderID { return "fireworks" }
