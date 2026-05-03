package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestListAuthzAPICatalogEntries_ProjectsCoverageFacts(t *testing.T) {
	facts := AuthzCoverageFacts{
		AllowlistRoutes: []AuthzAllowlistRoute{
			{Entrypoint: "server", Method: "GET", Path: "/org/api/org-units", RouteClass: string(routing.RouteClassInternalAPI), OwnerModule: "orgunit"},
			{Entrypoint: "server", Method: "GET", Path: "/healthz", RouteClass: string(routing.RouteClassOps), OwnerModule: "platform"},
			{Entrypoint: "server", Method: "POST", Path: "/iam/api/sessions", RouteClass: string(routing.RouteClassInternalAPI), OwnerModule: "iam"},
		},
		Routes: []AuthzRouteCoverage{
			{Method: "GET", Path: "/org/api/org-units", Object: authz.ObjectOrgUnitOrgUnits, Action: authz.ActionRead, Surface: authz.CapabilitySurfaceTenantAPI, Key: "orgunit.orgunits:read"},
			{Method: "POST", Path: "/iam/api/sessions", Object: authz.ObjectIAMSession, Action: authz.ActionAdmin, Surface: authz.CapabilitySurfaceTenantAPI, Key: "iam.session:admin"},
		},
		ToolOverlays: []AuthzToolOverlayCoverage{
			{Method: "GET", Path: "/org/api/org-units", CubeBoxCallable: true, Surface: authz.CapabilitySurfaceTenantAPI},
		},
		Registry: authz.ListAuthzCapabilities(),
	}

	entries, err := ListAuthzAPICatalogEntries(facts, authzAPICatalogFilter{})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("entries=%v", entries)
	}
	protected := findAPICatalogEntry(entries, "GET", "/org/api/org-units")
	if protected.AuthzCapabilityKey != "orgunit.orgunits:read" || protected.ResourceObject != authz.ObjectOrgUnitOrgUnits || !protected.CubeBoxCallable {
		t.Fatalf("protected=%+v", protected)
	}
	if protected.AccessControl != accessControlProtected || protected.OwnerModule != "orgunit" || !protected.Assignable {
		t.Fatalf("protected=%+v", protected)
	}
	if health := findAPICatalogEntry(entries, "GET", "/healthz"); health.Path != "" {
		t.Fatalf("health route leaked into API catalog: %+v", health)
	}
	if session := findAPICatalogEntry(entries, "POST", "/iam/api/sessions"); session.Path != "" {
		t.Fatalf("non-assignable capability route leaked into API catalog: %+v", session)
	}
}

func TestListAuthzAPICatalogEntries_FiltersByCapabilityKey(t *testing.T) {
	entries, err := ListAuthzAPICatalogEntries(AuthzCoverageFacts{
		AllowlistRoutes: ListAuthzAllowlistRoutes(testAuthzAllowlist()),
		Routes:          ListAuthzRouteCoverage(),
		Registry:        authz.ListAuthzCapabilities(),
	}, authzAPICatalogFilter{AuthzCapabilityKey: "orgunit.orgunits:read"})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("expected orgunit read API entries")
	}
	for _, entry := range entries {
		if entry.AuthzCapabilityKey != "orgunit.orgunits:read" {
			t.Fatalf("unexpected entry: %+v", entry)
		}
		if entry.AccessControl != accessControlProtected || !entry.Assignable {
			t.Fatalf("non-protected or non-assignable entry leaked into capability filter: %+v", entry)
		}
	}
}

func TestListAuthzAPICatalogEntries_FailsClosedWhenProtectedRouteHasNoRequirement(t *testing.T) {
	_, err := ListAuthzAPICatalogEntries(AuthzCoverageFacts{
		AllowlistRoutes: []AuthzAllowlistRoute{
			{Entrypoint: "server", Method: "GET", Path: "/org/api/org-units", RouteClass: string(routing.RouteClassInternalAPI), OwnerModule: "orgunit"},
		},
		Registry: authz.ListAuthzCapabilities(),
	}, authzAPICatalogFilter{})

	if err == nil {
		t.Fatal("expected missing authz requirement error")
	}
}

func TestHandleAuthzAPICatalogAPI_DefaultAndFilters(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/api-catalog?authz_capability_key=orgunit.orgunits:read", nil)
	rec := httptest.NewRecorder()

	handleAuthzAPICatalogAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload authzAPICatalogResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.APIEntries) == 0 {
		t.Fatal("expected API catalog entries")
	}
	for _, entry := range payload.APIEntries {
		if entry.AuthzCapabilityKey != "orgunit.orgunits:read" {
			t.Fatalf("unexpected entry: %+v", entry)
		}
	}
}

func TestHandleAuthzAPICatalogAPI_UsesAuthzPolicyPathEnv(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.ArtifactDir()
	policy := filepath.Join(dir, "policy.csv")
	if err := os.WriteFile(policy, []byte("p, role:tenant-admin, *, iam.authz, read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("AUTHZ_POLICY_PATH", policy)
	t.Setenv("ALLOWLIST_PATH", filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))

	req := httptest.NewRequest(http.MethodGet, "/iam/api/authz/api-catalog?authz_capability_key=orgunit.orgunits:read", nil)
	rec := httptest.NewRecorder()

	handleAuthzAPICatalogAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleAuthzAPICatalogAPI_RejectsInvalidCapabilityKey(t *testing.T) {
	for _, target := range []string{
		"/iam/api/authz/api-catalog?authz_capability_key=not-a-key",
		"/iam/api/authz/api-catalog?authz_capability_key=superadmin.tenants:read",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()

		handleAuthzAPICatalogAPI(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleAuthzAPICatalogAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/iam/api/authz/api-catalog", nil)
	rec := httptest.NewRecorder()

	handleAuthzAPICatalogAPI(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func findAPICatalogEntry(entries []authzAPICatalogEntry, method string, path string) authzAPICatalogEntry {
	for _, entry := range entries {
		if entry.Method == method && entry.Path == path {
			return entry
		}
	}
	return authzAPICatalogEntry{}
}
