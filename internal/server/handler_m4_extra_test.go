package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func TestLogin_RejectsInvalidIdentityRole(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1", RoleSlug: "not-a-role"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope routing.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("bad json: %v body=%q", err, rec.Body.String())
	}
	if envelope.Code != "invalid_identity_role" {
		t.Fatalf("code=%q body=%s", envelope.Code, rec.Body.String())
	}
}

func TestHandler_InternalAssignmentEventRoutes(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	store := assignmentStoreStub{
		listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil },
		upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
			return Assignment{}, nil
		},
		correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
			return "e-correct", nil
		},
		rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
			return "e-rescind", nil
		},
	}

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1", RoleSlug: authz.RoleTenantAdmin}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
		AssignmentStore:  store,
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d", loginRec.Code)
	}
	var sidCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "sid" && c.Value != "" {
			sidCookie = c
			break
		}
	}
	if sidCookie == nil || sidCookie.Name != "sid" || sidCookie.Value == "" {
		t.Fatalf("unexpected sid cookie: %#v", sidCookie)
	}

	req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","replacement_payload":{}}`))
	req.Host = "localhost:8080"
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_uuid":"a1","target_effective_date":"2026-01-01","payload":{}}`))
	req2.Host = "localhost:8080"
	req2.AddCookie(sidCookie)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestHandler_SetIDGovernanceRoutes(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1", RoleSlug: authz.RoleTenantAdmin}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
		SetIDStore:       scopeAPIStore{},
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d", loginRec.Code)
	}
	var sidCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "sid" && c.Value != "" {
			sidCookie = c
			break
		}
	}
	if sidCookie == nil || sidCookie.Name != "sid" || sidCookie.Value == "" {
		t.Fatalf("unexpected sid cookie: %#v", sidCookie)
	}

	doReq := func(method string, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "localhost:8080"
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := doReq(http.MethodGet, "/org/api/scope-packages", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("scope packages should be retired: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/org/api/owned-scope-packages", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("owned scope packages should be retired: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/org/api/scope-subscriptions", `{}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusNotFound {
		t.Fatalf("scope subscriptions should be retired: status=%d body=%s", rec.Code, rec.Body.String())
	}

	if rec := doReq(http.MethodPost, "/org/api/setid-strategy-registry", `{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"setid","org_level":"business_unit","business_unit_id":"10000001","required":true,"visible":true,"default_rule_ref":"rule://a1","default_value":"a1","priority":200,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","request_id":"r1"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("setid strategy registry post status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/org/api/setid-strategy-registry?as_of=2026-01-01&capability_key=staffing.assignment_create.field_policy&field_key=field_x", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("setid strategy registry get status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/org/api/setid-strategy-registry", `{"capability_key":"staffing.assignment_create.field_policy","owner_module":"staffing","field_key":"field_x","personalization_mode":"setid","org_level":"tenant","required":false,"visible":true,"default_value":"fallback","priority":100,"explain_required":true,"is_stable":true,"change_policy":"plan_required","effective_date":"2026-01-01","request_id":"r-fallback"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("setid strategy registry fallback status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/org/api/setid-strategy-registry:disable", `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","org_level":"business_unit","business_unit_id":"10000001","effective_date":"2026-01-01","disable_as_of":"2026-01-02","request_id":"r-disable"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("setid strategy registry disable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/internal/rules/evaluate", `{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_id":"10000001","as_of":"2026-01-01","request_id":"req-eval"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("internal rules evaluate status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/internal/policies/draft", `{"capability_key":"org.policy_activation.manage","draft_policy_version":"2026-03-01","operator":"tester"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("internal policy draft status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/internal/policies/activate", `{"capability_key":"org.policy_activation.manage","target_policy_version":"2026-03-01","operator":"tester"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("internal policy activate status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/internal/policies/state?capability_key=org.policy_activation.manage", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("internal policy state status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/internal/policies/rollback", `{"capability_key":"org.policy_activation.manage","target_policy_version":"2026-02-23","operator":"tester"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("internal policy rollback status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/internal/functional-areas/state", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("internal functional area state status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/internal/functional-areas/switch", `{"functional_area_key":"staffing","enabled":false,"operator":"tester"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("internal functional area switch off status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodPost, "/internal/functional-areas/switch", `{"functional_area_key":"staffing","enabled":true,"operator":"tester"}`, map[string]string{
		"Content-Type": "application/json",
	}); rec.Code != http.StatusOK {
		t.Fatalf("internal functional area switch on status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_id=10000001&as_of=2026-01-01&setid=A0001&level=brief", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("setid explain get status=%d body=%s", rec.Code, rec.Body.String())
	}
}
