package adapters

import (
	"github.com/zclaw/zclaw/internal/providers"
)

// MistralAdapter provides OpenAI-compatible chat/completions against Mistral AI endpoints.
type MistralAdapter struct {
	*OpenAIAdapter
}

func NewMistralAdapter(apiKey string) *MistralAdapter {
	return &MistralAdapter{OpenAIAdapter: NewOpenAIAdapter(apiKey, "https://api.mistral.ai/v1")}
}

func (a *MistralAdapter) ID() providers.ProviderID { return "mistral" }

// The common OpenAI-compatible implementation is reused via embedding.
// If Mistral has a different path in the future, override Generate accordingly.
