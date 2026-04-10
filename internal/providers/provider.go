// Package providers defines the provider adapter interface and registry.
//
// Every LLM provider (OpenAI, Anthropic, Ollama, etc.) implements the Provider
// interface. Provider-specific logic lives behind adapter boundaries — never
// scattered through the scheduler or agent core.
package providers

import (
	"context"
	"fmt"
	"time"
)

// ProviderID identifies a provider adapter.
type ProviderID string

func (id ProviderID) String() string { return string(id) }

// MessageRole distinguishes conversation participants.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

func (r MessageRole) String() string { return string(r) }

// CapabilityFlag describes what a provider supports.
type CapabilityFlag string

const (
	CapTools      CapabilityFlag = "tools"
	CapStreaming  CapabilityFlag = "streaming"
	CapVision     CapabilityFlag = "vision"
	CapAudioIn    CapabilityFlag = "audio_in"
	CapAudioOut   CapabilityFlag = "audio_out"
	CapReasoning  CapabilityFlag = "reasoning"
	CapJSON       CapabilityFlag = "json_mode"
	CapSystemMsg  CapabilityFlag = "system_message"
	CapMultiModal CapabilityFlag = "multimodal"
)

// CostClass categorizes a provider's pricing tier.
type CostClass string

const (
	CostFree      CostClass = "free"
	CostCheap     CostClass = "cheap"
	CostMedium    CostClass = "medium"
	CostExpensive CostClass = "expensive"
	CostVariable  CostClass = "variable"
)

func (c CostClass) String() string { return string(c) }

// AuthMode describes how a provider authenticates.
type AuthMode string

const (
	AuthAPIKey      AuthMode = "api_key"
	AuthBearerToken AuthMode = "bearer_token"
	AuthNone        AuthMode = "none"
	AuthCustom      AuthMode = "custom"
)

// ToolDefinition describes a tool that the model can invoke.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a model's request to invoke a tool.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult is the output of executing a tool call.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Result  string `json:"result"`
	IsError bool   `json:"is_error,omitempty"`
}

// Message is a single message in a conversation.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
	Name       string      `json:"name,omitempty"`
	Model      string      `json:"model,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
}

// StreamChunk is a single chunk in a streaming response.
type StreamChunk struct {
	Content      string     `json:"content,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Done         bool       `json:"done"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Model        string     `json:"model,omitempty"`
	Usage        *Usage     `json:"usage,omitempty"`
}

// Usage tracks token consumption for a request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// GenerateRequest is the input to a provider's generate endpoint.
type GenerateRequest struct {
	// Model is the model identifier to use.
	Model string `json:"model"`

	// Messages is the conversation history.
	Messages []Message `json:"messages"`

	// SystemPrompt overrides the system-level instruction.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Tools available for the model to call.
	Tools []ToolDefinition `json:"tools,omitempty"`

	// Temperature controls randomness (0.0-2.0). nil = provider default.
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Stream requests a streaming response.
	Stream bool `json:"stream"`

	// StopSequences stops generation when encountered.
	StopSequences []string `json:"stop_sequences,omitempty"`

	// Metadata is provider-specific key-value data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// GenerateResponse is the output from a provider's generate endpoint.
type GenerateResponse struct {
	Message      Message `json:"message"`
	Usage        Usage   `json:"usage"`
	Model        string  `json:"model"`
	FinishReason string  `json:"finish_reason"`
	ProviderID   string  `json:"provider_id"`
}

// ProviderConfig holds configuration for instantiating a provider.
type ProviderConfig struct {
	ID           ProviderID        `json:"id"`
	Name         string            `json:"name"`
	AuthMode     AuthMode          `json:"auth_mode"`
	BaseURL      string            `json:"base_url"`
	APIKey       string            `json:"api_key,omitempty"`
	Capabilities []CapabilityFlag  `json:"capabilities"`
	Models       []ModelInfo       `json:"models"`
	RateLimit    *RateLimitConfig  `json:"rate_limit,omitempty"`
	CostClass    CostClass         `json:"cost_class"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ModelInfo describes a model available through a provider.
type ModelInfo struct {
	ID               string           `json:"id"`
	Aliases          []string         `json:"aliases,omitempty"`
	MaxContextTokens int              `json:"max_context_tokens"`
	Capabilities     []CapabilityFlag `json:"capabilities,omitempty"`
	CostPer1kInput   float64          `json:"cost_per_1k_input,omitempty"`
	CostPer1kOutput  float64          `json:"cost_per_1k_output,omitempty"`
}

// RateLimitConfig provides rate limiting hints.
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute,omitempty"`
	TokensPerMinute   int `json:"tokens_per_minute,omitempty"`
	BurstSize         int `json:"burst_size,omitempty"`
}

// Provider is the interface that every LLM provider adapter must implement.
type Provider interface {
	// ID returns the provider identifier.
	ID() ProviderID

	// Capabilities returns what this provider supports.
	Capabilities() []CapabilityFlag

	// HasCapability checks if a specific capability is supported.
	HasCapability(cap CapabilityFlag) bool

	// Generate sends a request and returns the full response.
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)

	// GenerateStream sends a request and returns a channel of chunks.
	GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)

	// ValidateConfig checks that the provider configuration is valid.
	ValidateConfig(config ProviderConfig) error

	// NormalizeModel converts aliases to canonical model IDs.
	NormalizeModel(model string) string

	// Close cleans up any resources (HTTP clients, connections).
	Close() error
}

// ProviderRegistry manages provider adapters.
type ProviderRegistry interface {
	// Register adds a provider adapter.
	Register(ctx context.Context, provider Provider) error

	// Get returns a provider by ID.
	Get(ctx context.Context, id ProviderID) (Provider, error)

	// List returns all registered provider IDs.
	List(ctx context.Context) []ProviderID

	// Configure applies configuration to a registered provider.
	Configure(ctx context.Context, id ProviderID, config ProviderConfig) error

	// GetConfig returns the current configuration for a provider.
	GetConfig(ctx context.Context, id ProviderID) (*ProviderConfig, error)

	// ResolveProvider finds a provider for a model string.
	// Handles aliases and provider prefixes like "openai/gpt-4o".
	ResolveProvider(ctx context.Context, modelRef string) (Provider, string, error)

	// Close shuts down all providers.
	Close() error
}

// ProviderNotFoundError indicates the requested provider is not registered.
type ProviderNotFoundError struct {
	ID ProviderID
}

func (e *ProviderNotFoundError) Error() string {
	return fmt.Sprintf("provider not found: %s", e.ID)
}

// CapabilityNotSupportedError indicates the provider lacks a capability.
type CapabilityNotSupportedError struct {
	Provider   ProviderID
	Capability CapabilityFlag
}

func (e *CapabilityNotSupportedError) Error() string {
	return fmt.Sprintf("provider %s does not support %s", e.Provider, e.Capability)
}
