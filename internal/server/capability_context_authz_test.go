package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveCapabilityContext(t *testing.T) {
	t.Run("missing required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
		_, err := resolveCapabilityContext(context.Background(), req, capabilityContextInput{
			CapabilityKey:       "",
			BusinessUnitID:      "10000001",
			AsOf:                "2026-01-01",
			RequireBusinessUnit: true,
		})
		if err == nil || err.Code != capabilityReasonContextRequired {
			t.Fatalf("err=%+v", err)
		}
		if got := statusCodeForCapabilityContextError(err.Code); got != http.StatusBadRequest {
			t.Fatalf("status=%d", got)
		}
	})

	t.Run("actor scope mismatch fail closed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		ctx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin"})
		_, err := resolveCapabilityContext(ctx, req, capabilityContextInput{
			CapabilityKey:       "staffing.assignment_create.field_policy",
			BusinessUnitID:      "10000001",
			AsOf:                "2026-01-01",
			RequireBusinessUnit: true,
		})
		if err == nil || err.Code != capabilityReasonContextMismatch {
			t.Fatalf("err=%+v", err)
		}
		if got := statusCodeForCapabilityContextError(err.Code); got != http.StatusForbidden {
			t.Fatalf("status=%d", got)
		}
	})

	t.Run("missing required business unit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
		_, err := resolveCapabilityContext(context.Background(), req, capabilityContextInput{
			CapabilityKey:       "staffing.assignment_create.field_policy",
			AsOf:                "2026-01-01",
			RequireBusinessUnit: true,
		})
		if err == nil || err.Code != capabilityReasonContextRequired {
			t.Fatalf("err=%+v", err)
		}
	})

	t.Run("superadmin scope is authoritative", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		ctx := withPrincipal(context.Background(), Principal{RoleSlug: "superadmin"})
		got, err := resolveCapabilityContext(ctx, req, capabilityContextInput{
			CapabilityKey:       "staffing.assignment_create.field_policy",
			BusinessUnitID:      "10000001",
			AsOf:                "2026-01-01",
			RequireBusinessUnit: true,
		})
		if err != nil {
			t.Fatalf("err=%+v", err)
		}
		if got.ActorScope != actorScopeSaaS {
			t.Fatalf("actor_scope=%q", got.ActorScope)
		}
	})

	t.Run("tenant org-level context allows empty business unit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/setid-strategy-registry", nil)
		got, err := resolveCapabilityContext(context.Background(), req, capabilityContextInput{
			CapabilityKey:       "staffing.assignment_create.field_policy",
			AsOf:                "2026-01-01",
			RequireBusinessUnit: false,
		})
		if err != nil {
			t.Fatalf("err=%+v", err)
		}
		if got.ActorScope != actorScopeTenant {
			t.Fatalf("actor_scope=%q", got.ActorScope)
		}
	})
}

func TestRequestActorScope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	if got := requestActorScope(req); got != "" {
		t.Fatalf("scope=%q", got)
	}
	req.Header.Set("x-actor-scope", " SAAS ")
	if got := requestActorScope(req); got != "saas" {
		t.Fatalf("scope=%q", got)
	}
}

func TestCapabilityDynamicRelations(t *testing.T) {
	t.Run("tenant viewer limited to preloaded business unit", func(t *testing.T) {
		ctx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-viewer"})
		relations := preloadCapabilityDynamicRelations(ctx, "10000001")
		if !relations.actorManages("10000001", "2026-01-01") {
			t.Fatal("expected manage=true")
		}
		if relations.actorManages("10000002", "2026-01-01") {
			t.Fatal("expected manage=false")
		}
	})

	t.Run("tenant admin can manage all", func(t *testing.T) {
		ctx := withPrincipal(context.Background(), Principal{RoleSlug: "tenant-admin"})
		relations := preloadCapabilityDynamicRelations(ctx, "10000001")
		if !relations.actorManages("10000002", "2026-01-01") {
			t.Fatal("expected manage=true")
		}
	})

	t.Run("invalid target or as_of is rejected", func(t *testing.T) {
		relations := preloadCapabilityDynamicRelations(context.Background(), "10000001")
		if relations.actorManages("", "2026-01-01") {
			t.Fatal("expected manage=false")
		}
		if relations.actorManages("10000001", "") {
			t.Fatal("expected manage=false")
		}
	})
}
