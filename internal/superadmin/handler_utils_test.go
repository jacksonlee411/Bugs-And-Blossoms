package superadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("X-Request-Id", "rid")
	if got := requestID(r); got != "rid" {
		t.Fatalf("got=%q", got)
	}

	r2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	if got := requestID(r2); got == "" {
		t.Fatal("expected request id")
	}
}

func TestSuperadminWritesEnabled(t *testing.T) {
	t.Setenv("SUPERADMIN_WRITE_MODE", "")
	if !superadminWritesEnabled() {
		t.Fatal("expected enabled")
	}

	t.Setenv("SUPERADMIN_WRITE_MODE", "enabled")
	if !superadminWritesEnabled() {
		t.Fatal("expected enabled")
	}

	t.Setenv("SUPERADMIN_WRITE_MODE", "disabled")
	if superadminWritesEnabled() {
		t.Fatal("expected disabled")
	}
}

func TestTenantIDFromPath(t *testing.T) {
	if got, ok := tenantIDFromPath("/superadmin/tenants/t1/disable"); !ok || got != "t1" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
	if _, ok := tenantIDFromPath("/bad"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := tenantIDFromPath("/x/tenants/t1/disable"); ok {
		t.Fatal("expected invalid")
	}
	if _, ok := tenantIDFromPath("/superadmin/x/t1/disable"); ok {
		t.Fatal("expected invalid")
	}
}

func TestInsertAudit_MissingActor(t *testing.T) {
	tx := &stubTx{}
	if err := insertAudit(context.Background(), tx, "", "action", "00000000-0000-0000-0000-000000000001", nil, "rid"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthzRequirementForRoute(t *testing.T) {
	object, action, ok := authzRequirementForRoute(http.MethodGet, "/superadmin/login")
	if !ok || object == "" || action == "" {
		t.Fatalf("expected ok got ok=%v object=%q action=%q", ok, object, action)
	}
	object, action, ok = authzRequirementForRoute(http.MethodPost, "/superadmin/login")
	if !ok || object == "" || action == "" {
		t.Fatalf("expected ok got ok=%v object=%q action=%q", ok, object, action)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/superadmin/login"); ok {
		t.Fatal("expected no check")
	}
	object, action, ok = authzRequirementForRoute(http.MethodPost, "/superadmin/logout")
	if !ok || object == "" || action == "" {
		t.Fatalf("expected ok got ok=%v object=%q action=%q", ok, object, action)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/superadmin/logout"); ok {
		t.Fatal("expected no check")
	}

	object, action, ok = authzRequirementForRoute(http.MethodGet, "/superadmin/tenants")
	if !ok || object == "" || action == "" {
		t.Fatalf("expected ok got ok=%v object=%q action=%q", ok, object, action)
	}
	object, action, ok = authzRequirementForRoute(http.MethodPost, "/superadmin/tenants")
	if !ok || object == "" || action == "" {
		t.Fatalf("expected ok got ok=%v object=%q action=%q", ok, object, action)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/superadmin/tenants"); ok {
		t.Fatal("expected no check")
	}
	object, action, ok = authzRequirementForRoute(http.MethodPost, "/superadmin/tenants/t1/disable")
	if !ok || object == "" || action == "" {
		t.Fatalf("expected ok got ok=%v object=%q action=%q", ok, object, action)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/superadmin/tenants/t1/disable"); ok {
		t.Fatal("expected no check")
	}
}
