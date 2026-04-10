package adapters

import (
	"github.com/zclaw/zclaw/internal/providers"
)

// GroqAdapter is an OpenAI-compatible adapter for Groq's inferencing API.
// It reuses the OpenAIAdapter for request/response shapes, but targets Groq's endpoint.
type GroqAdapter struct {
	*OpenAIAdapter
}

func NewGroqAdapter(apiKey string) *GroqAdapter {
	return &GroqAdapter{
		OpenAIAdapter: NewOpenAIAdapter(apiKey, "https://api.groq.com/openai/v1"),
	}
}

// ID returns the provider identifier for Groq.
func (a *GroqAdapter) ID() providers.ProviderID { return "groq" }
