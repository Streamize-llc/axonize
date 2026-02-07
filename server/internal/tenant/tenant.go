package tenant

import "context"

type contextKey struct{}

// DefaultTenantID is used for single-tenant (self-hosted) deployments.
const DefaultTenantID = "default"

// WithTenantID stores the tenant ID in the context.
func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext extracts the tenant ID from the context.
// Returns DefaultTenantID if not set.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(contextKey{}).(string); ok && id != "" {
		return id
	}
	return DefaultTenantID
}
