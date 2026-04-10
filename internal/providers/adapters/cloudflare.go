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

// CloudflareAIGatewayAdapter routes OpenAI-compatible requests through Cloudflare AI Gateway.
type CloudflareAIGatewayAdapter struct {
	*OpenAIAdapter
	accountID string
	gatewayID string
}

func NewCloudflareAIGatewayAdapter(accountID, gatewayID, apiKey string) *CloudflareAIGatewayAdapter {
	base := fmt.Sprintf("https://gateway.ai.cloudflare.com/v1/%s/%s/openai/chat/completions", accountID, gatewayID)
	return &CloudflareAIGatewayAdapter{OpenAIAdapter: NewOpenAIAdapter(apiKey, base), accountID: accountID, gatewayID: gatewayID}
}

func (a *CloudflareAIGatewayAdapter) ID() providers.ProviderID { return "cloudflare-gateway" }

// Override Generate to call Cloudflare gateway directly (bypassing OpenAI path appending).
func (a *CloudflareAIGatewayAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
	// Reuse internal message conversion from OpenAI adapter
	messages := a.OpenAIAdapter.convertMessages(req)
	oaiReq := openaiRequest{
		Model:       a.NormalizeModel(req.Model),
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.StopSequences,
	}
	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal cloudflare request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Authorization header remains the same to proxy to underlying provider
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey())

	resp, err := a.OpenAIAdapter.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cloudflare gateway error: %s", resp.Status)
	}
	var oaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response from Cloudflare gateway")
	}
	choice := oaiResp.Choices[0]
	result := &providers.GenerateResponse{Usage: oaiResp.Usage, Model: oaiResp.Model, ProviderID: string(a.ID())}
	// choice.Message.Content is interface{}; format to string for compatibility
	result.Message = providers.Message{Role: providers.RoleAssistant, Content: fmt.Sprintf("%v", choice.Message.Content), Timestamp: time.Now().UTC()}
	return result, nil
}

func (a *CloudflareAIGatewayAdapter) apiKey() string {
	// Access to the underlying OpenAIAdapter's API key (private in this package but accessible here)
	// If not accessible, fall back to empty string.
	return a.OpenAIAdapter.apiKey
}
