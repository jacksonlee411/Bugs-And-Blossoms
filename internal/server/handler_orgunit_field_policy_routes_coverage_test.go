package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerWithOptions_OrgUnitFieldPolicyRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000ab",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: orgUnitStoreWithFieldPolicies{OrgUnitStore: newOrgUnitMemoryStore()},
	})
	if err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRequest(http.MethodPost, "http://localhost/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	login.Host = "localhost"
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	sidCookie := loginRec.Result().Cookies()[0]

	check := func(method string, path string, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s status=%d body=%s", method, path, rec.Code, rec.Body.String())
		}
	}

	check(http.MethodPost, "/org/api/org-units/field-policies", `{"field_key":"","enabled_on":"","request_code":""}`, http.StatusBadRequest)
	check(http.MethodPost, "/org/api/org-units/field-policies:disable", `{"field_key":"","disabled_on":"","request_code":""}`, http.StatusBadRequest)
	check(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?as_of=2026-01-01", "", http.StatusBadRequest)
}
