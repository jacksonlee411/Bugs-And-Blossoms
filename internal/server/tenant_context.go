package server

import "context"

type tenantCtxKey struct{}

func withTenant(ctx context.Context, tenant Tenant) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenant)
}

func currentTenant(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(tenantCtxKey{}).(Tenant)
	return t, ok
}
