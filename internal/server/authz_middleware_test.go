package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
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

	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/org/setid", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_AllowsWhenNotEnforced(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withAuthz(mustTestClassifier(t), stubAuthorizer{allowed: false, enforced: false}, next)

	req := httptest.NewRequest(http.MethodPost, "/org/job-catalog", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
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
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/nodes"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/nodes"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/nodes"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPatch, "/org/nodes"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/snapshot"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPatch, "/org/snapshot"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/setid"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/setid"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/setid"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/job-catalog"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/job-catalog"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/job-catalog"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/positions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/positions"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/positions"); ok {
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
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/assignments"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/assignments"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/assignments"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/org/api/assignments"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/org/api/assignments"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodDelete, "/org/api/assignments"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodGet, "/person/persons"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPost, "/person/persons"); !ok {
		t.Fatal("expected ok=true")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/person/persons"); ok {
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
	tmp := t.TempDir()
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
	dir := t.TempDir()
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
	tmp := t.TempDir()
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
	dir := t.TempDir()
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
	tmp := t.TempDir()
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
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/setid"); ok {
		t.Fatal("expected ok=false")
	}
	if _, _, ok := authzRequirementForRoute(http.MethodPut, "/org/job-catalog"); ok {
		t.Fatal("expected ok=false")
	}
}
