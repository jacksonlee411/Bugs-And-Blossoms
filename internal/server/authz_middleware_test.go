package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type stubAuthorizer struct {
	allowed  bool
	enforced bool
	err      error
}

func (a stubAuthorizer) Authorize(string, string, string, string) (bool, bool, error) {
	return a.allowed, a.enforced, a.err
}

func mustTestClassifier(t *testing.T) *routing.Classifier {
	t.Helper()

	c, err := routing.NewClassifier(routing.Allowlist{Version: 1, Entrypoints: map[string]routing.Entrypoint{
		"server": {Routes: []routing.Route{{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"}}},
	}}, "server")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestWithAuthz_AllowsBypassRoutes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_LoginForbiddenWhenEnforced(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_SkipsWhenNoRequirement(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/org/unprotected", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !nextCalled {
		t.Fatalf("status=%d next=%v", rec.Code, nextCalled)
	}
}

func TestWithAuthz_AnonymousRole(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: true, enforced: true}, next)

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_ForbiddenWhenEnforced(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/org/api/setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_OrgUnitRescindForbiddenWhenEnforced(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/rescinds", strings.NewReader(`{"org_code":"A001","effective_date":"2026-01-01","request_id":"r1","reason":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "forbidden") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestWithAuthz_ShareReadForbidden(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/org/api/global-setids", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "share_read_forbidden") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestWithAuthz_AllowsWhenNotEnforced(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: false}, next)

	req := httptest.NewRequest(http.MethodPost, "/jobcatalog/api/catalog/actions", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_AuthzError(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: true, err: os.ErrInvalid}, next)

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_TenantMissing(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: true, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthzRequirementForRoute(t *testing.T) {
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/unknown"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/sessions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/sessions"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/iam/api/dicts"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts:disable"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts:disable"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts/values"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts/values"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/iam/api/dicts/values"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts/values:disable"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts/values:disable"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts/values:correct"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts/values:correct"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts/values/audit"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts/values/audit"); ok {
		t.Fatal("expected ok=false")
	}
	if object, action, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts:release"); !ok || object != authz.ObjectIAMDictRelease || action != authz.ActionAdmin {
		t.Fatalf("release requirement mismatch object=%q action=%q ok=%v", object, action, ok)
	}
	if object, action, ok := authzRequirementForRoute(http.MethodPost, "/iam/api/dicts:release:preview"); !ok || object != authz.ObjectIAMDictRelease || action != authz.ActionAdmin {
		t.Fatalf("release preview requirement mismatch object=%q action=%q ok=%v", object, action, ok)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/iam/api/dicts:release"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/logout"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/logout"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/jobcatalog/api/catalog"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/jobcatalog/api/catalog"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/jobcatalog/api/catalog/actions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/jobcatalog/api/catalog/actions"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/positions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/positions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/positions"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/positions:options"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/positions:options"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/assignments"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/assignments"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/api/assignments"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/assignment-events:correct"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/assignment-events:correct"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/assignment-events:rescind"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/assignment-events:rescind"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/setids"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/setids"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/setids"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/setid-bindings"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/setid-bindings"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/setid-bindings"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/scope-packages"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/scope-packages"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/scope-packages/p1/disable"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/owned-scope-packages"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/scope-subscriptions"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/global-setids"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/global-setids"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/api/global-setids"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/global-scope-packages"); ok {
		t.Fatal("expected ok=false")
	}
	if object, action, ok := authzRequirementForRoute(http.MethodGet, "/org/api/setid-strategy-registry"); !ok || object != authz.ObjectOrgSetIDCapability || action != authz.ActionRead {
		t.Fatalf("expected setid strategy read mapped to org.setid_capability_config, got ok=%v object=%q action=%q", ok, object, action)
	}
	if object, action, ok := authzRequirementForRoute(http.MethodPost, "/org/api/setid-strategy-registry"); !ok || object != authz.ObjectOrgSetIDCapability || action != authz.ActionAdmin {
		t.Fatalf("expected setid strategy admin mapped to org.setid_capability_config, got ok=%v object=%q action=%q", ok, object, action)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/setid-strategy-registry"); ok {
		t.Fatal("expected ok=false")
	}
	if object, action, ok := authzRequirementForRoute(http.MethodGet, "/org/api/setid-explain"); !ok || object != authz.ObjectOrgSetIDCapability || action != authz.ActionRead {
		t.Fatalf("expected setid explain mapped to org.setid_capability_config, got ok=%v object=%q action=%q", ok, object, action)
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/setid-explain"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/org-units"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-definitions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-definitions"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-configs"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-configs"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/org-units/field-configs"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-configs:enable-candidates"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-configs:enable-candidates"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-configs:disable"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-configs:disable"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-policies"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-policies"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-policies:disable"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-policies:disable"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/field-policies:resolve-preview"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/fields:options"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/fields:options"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/mutation-capabilities"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/mutation-capabilities"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/append-capabilities"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/append-capabilities"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/write-capabilities"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/write-capabilities"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/details"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/details"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/versions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/versions"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/audit"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/audit"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/search"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/search"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/rename"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/rename"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/move"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/disable"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/enable"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/write"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/corrections"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/status-corrections"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/rescinds"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/rescinds/org"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/rescinds"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/org-units/set-business-unit"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/org-units/set-business-unit"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, ""); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/person/api/persons"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/person/api/persons"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/person/api/persons"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/person/api/persons:options"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/person/api/persons:options"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/person/api/persons:by-pernr"); ok {
		t.Fatal("expected ok=false")
	}
}

func TestDefaultAuthzPaths_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.ArtifactDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if _, err := defaultAuthzModelPath(); err == nil {
		t.Fatal("expected error")
	}
	if _, err := defaultAuthzPolicyPath(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_WithEnvPaths(t *testing.T) {
	dir := t.ArtifactDir()
	model := filepath.Join(dir, "model.conf")
	policy := filepath.Join(dir, "policy.csv")

	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("p, role:tenant-admin, t1, jobcatalog.catalog, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", policy)
	t.Setenv("AUTHZ_MODE", "enforce")

	a, err := loadAuthorizer()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err := a.Authorize("role:tenant-admin", "t1", "jobcatalog.catalog", "read")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !allowed || !enforced {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}
}

func TestLoadAuthorizer_InvalidMode(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Use real repo files to avoid path resolution complexity.
	t.Setenv("AUTHZ_MODEL_PATH", filepath.Join(wd, "..", "..", "config", "access", "model.conf"))
	t.Setenv("AUTHZ_POLICY_PATH", filepath.Join(wd, "..", "..", "config", "access", "policy.csv"))
	t.Setenv("AUTHZ_MODE", "nope")

	if _, err := loadAuthorizer(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_DefaultPaths_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.ArtifactDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	t.Setenv("AUTHZ_MODEL_PATH", "")
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	if _, err := loadAuthorizer(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_DefaultPaths_Success(t *testing.T) {
	t.Setenv("AUTHZ_MODEL_PATH", "")
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	a, err := loadAuthorizer()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	allowed, enforced, err := a.Authorize("role:tenant-admin", "00000000-0000-0000-0000-000000000001", "jobcatalog.catalog", "read")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !allowed || !enforced {
		t.Fatalf("allowed=%v enforced=%v", allowed, enforced)
	}
}

func TestLoadAuthorizer_NewAuthorizerError(t *testing.T) {
	dir := t.ArtifactDir()
	model := filepath.Join(dir, "model.conf")
	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", dir)
	t.Setenv("AUTHZ_MODE", "enforce")

	if _, err := loadAuthorizer(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadAuthorizer_DefaultPolicyPath_Error(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.ArtifactDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	model := filepath.Join(tmp, "model.conf")
	if err := os.WriteFile(model, []byte(`
[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AUTHZ_MODEL_PATH", model)
	t.Setenv("AUTHZ_POLICY_PATH", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	if _, err := loadAuthorizer(); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthzRequirementForRoute_UnsupportedMethods(t *testing.T) {
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/api/setids"); ok {
		t.Fatal("expected ok=false")
	}
}

func TestPathMatchRouteTemplate(t *testing.T) {
	if !pathMatchRouteTemplate("/org/api/example/123/disable", "/org/api/example/{id}/disable") {
		t.Fatal("expected match")
	}
	if pathMatchRouteTemplate("/org/api/example/123/disable", "/org/api/example/{id}") {
		t.Fatal("expected length mismatch")
	}
	if pathMatchRouteTemplate("/org//positions", "/org/{id}/positions") {
		t.Fatal("expected empty segment mismatch")
	}
	if pathMatchRouteTemplate("/org/api/example/123/disable", "/org/api/example/{id}/enable") {
		t.Fatal("expected segment mismatch")
	}
}

func TestSplitRouteSegments(t *testing.T) {
	got := splitRouteSegments(" /org/api/positions ")
	if len(got) != 3 || got[0] != "org" || got[1] != "api" || got[2] != "positions" {
		t.Fatalf("segments=%v", got)
	}
	if splitRouteSegments("   ") != nil {
		t.Fatal("expected nil")
	}
}

func TestRouteTemplateIsParamSegment(t *testing.T) {
	if !routeTemplateIsParamSegment("{id}") {
		t.Fatal("expected param segment")
	}
	if routeTemplateIsParamSegment("{}") {
		t.Fatal("expected false for empty param")
	}
	if routeTemplateIsParamSegment("id") {
		t.Fatal("expected false for plain text")
	}
	if routeTemplateIsParamSegment("{id") {
		t.Fatal("expected false for missing suffix")
	}
	if routeTemplateIsParamSegment("id}") {
		t.Fatal("expected false for missing prefix")
	}
}
