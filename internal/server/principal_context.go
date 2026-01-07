package server

import "context"

type Principal struct {
	ID               string
	TenantID         string
	RoleSlug         string
	Status           string
	Email            string
	KratosIdentityID string
}

type principalContextKey struct{}

func withPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

func currentPrincipal(ctx context.Context) (Principal, bool) {
	v := ctx.Value(principalContextKey{})
	if v == nil {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}
