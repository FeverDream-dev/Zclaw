package providers

import (
	"context"
	"errors"
	"testing"
)

func TestProviderNotFoundError(t *testing.T) {
	err := &ProviderNotFoundError{ID: "nonexistent"}
	if err.Error() != "provider not found: nonexistent" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestCapabilityNotSupportedError(t *testing.T) {
	err := &CapabilityNotSupportedError{Provider: "test", Capability: CapVision}
	want := "provider test does not support vision"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestCostClassString(t *testing.T) {
	tests := []struct {
		c    CostClass
		want string
	}{
		{CostFree, "free"},
		{CostCheap, "cheap"},
		{CostMedium, "medium"},
		{CostExpensive, "expensive"},
	}
	for _, tt := range tests {
		if tt.c.String() != tt.want {
			t.Errorf("CostClass(%s).String() = %q, want %q", tt.c, tt.c.String(), tt.want)
		}
	}
}

func TestMessageRoleString(t *testing.T) {
	if RoleUser.String() != "user" {
		t.Errorf("RoleUser.String() = %q", RoleUser.String())
	}
	if RoleSystem.String() != "system" {
		t.Errorf("RoleSystem.String() = %q", RoleSystem.String())
	}
}

func TestLocalRegistry(t *testing.T) {
	reg := NewLocalRegistry()
	ctx := context.Background()

	ids := reg.List(ctx)
	if len(ids) != 0 {
		t.Errorf("new registry should be empty, got %d", len(ids))
	}

	_, err := reg.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
	var perr *ProviderNotFoundError
	if !errors.As(err, &perr) {
		t.Error("expected ProviderNotFoundError")
	}
}
