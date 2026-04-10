package agents

import (
	"testing"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from, to AgentState
		want     bool
	}{
		{StateCreated, StateActive, true},
		{StateCreated, StateDisabled, true},
		{StateCreated, StateArchived, true},
		{StateCreated, StatePaused, false},
		{StateActive, StatePaused, true},
		{StateActive, StateDisabled, true},
		{StateActive, StateActive, false},
		{StatePaused, StateActive, true},
		{StateDisabled, StateActive, true},
		{StateArchived, StateActive, false},
	}
	for _, tt := range tests {
		got := IsValidTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCreateAgentRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateAgentRequest
		wantErr bool
	}{
		{
			name: "valid",
			req:  CreateAgentRequest{Name: "test", Provider: ProviderAssignment{ProviderID: "openai", Model: "gpt-4o-mini"}},
		},
		{
			name:    "missing name",
			req:     CreateAgentRequest{Provider: ProviderAssignment{ProviderID: "openai", Model: "gpt-4o-mini"}},
			wantErr: true,
		},
		{
			name:    "missing provider",
			req:     CreateAgentRequest{Name: "test"},
			wantErr: true,
		},
		{
			name:    "missing model",
			req:     CreateAgentRequest{Name: "test", Provider: ProviderAssignment{ProviderID: "openai"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentStateString(t *testing.T) {
	if StateActive.String() != "active" {
		t.Errorf("StateActive.String() = %q, want %q", StateActive.String(), "active")
	}
}

func TestProviderAssignmentDefaults(t *testing.T) {
	pa := ProviderAssignment{
		ProviderID: "openai",
		Model:      "gpt-4o-mini",
	}
	if pa.ProviderID != "openai" {
		t.Errorf("expected openai, got %s", pa.ProviderID)
	}
	if pa.Temperature != 0 {
		t.Errorf("expected default temperature 0, got %f", pa.Temperature)
	}
}
