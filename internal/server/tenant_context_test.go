package server

import (
	"context"
	"testing"
)

func TestTenantContext(t *testing.T) {
	tenant := Tenant{ID: "t1", Domain: "localhost", Name: "T"}
	ctx := withTenant(context.Background(), tenant)

	got, ok := currentTenant(ctx)
	if !ok {
		t.Fatal("expected tenant")
	}
	if got.ID != tenant.ID || got.Domain != tenant.Domain || got.Name != tenant.Name {
		t.Fatalf("got=%+v", got)
	}
}
