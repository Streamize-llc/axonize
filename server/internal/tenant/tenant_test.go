package tenant

import (
	"context"
	"testing"
)

func TestWithTenantID_and_FromContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tn_abc123")

	got := FromContext(ctx)
	if got != "tn_abc123" {
		t.Errorf("FromContext = %q, want %q", got, "tn_abc123")
	}
}

func TestFromContext_Default(t *testing.T) {
	ctx := context.Background()
	got := FromContext(ctx)
	if got != DefaultTenantID {
		t.Errorf("FromContext = %q, want %q", got, DefaultTenantID)
	}
}

func TestFromContext_EmptyString(t *testing.T) {
	ctx := WithTenantID(context.Background(), "")
	got := FromContext(ctx)
	if got != DefaultTenantID {
		t.Errorf("FromContext with empty string = %q, want %q", got, DefaultTenantID)
	}
}

func TestWithTenantID_Override(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tn_first")
	ctx = WithTenantID(ctx, "tn_second")

	got := FromContext(ctx)
	if got != "tn_second" {
		t.Errorf("FromContext after override = %q, want %q", got, "tn_second")
	}
}

func TestDefaultTenantID_Value(t *testing.T) {
	if DefaultTenantID != "default" {
		t.Errorf("DefaultTenantID = %q, want %q", DefaultTenantID, "default")
	}
}
