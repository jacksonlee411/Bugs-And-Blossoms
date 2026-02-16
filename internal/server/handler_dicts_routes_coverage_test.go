package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHandlerWithOptions_DictRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Login to get sid cookie.
	login := httptest.NewRequest(http.MethodPost, "http://localhost/iam/api/sessions", stringsReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	login.Host = "localhost"
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	sidCookie := loginRec.Result().Cookies()[0]

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	post := func(path string, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, stringsReader(body))
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := get("/iam/api/dicts?as_of=2026-01-01"); rec.Code != http.StatusOK {
		t.Fatalf("dicts status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := get("/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&status=all"); rec.Code != http.StatusOK {
		t.Fatalf("dict values status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts/values", `{"dict_code":"org_type","code":"30","label":"X","enabled_on":"2026-01-01","request_code":"r1"}`); rec.Code != http.StatusConflict {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts/values:disable", `{"dict_code":"org_type","code":"10","disabled_on":"2026-01-01","request_code":"r1"}`); rec.Code != http.StatusConflict {
		t.Fatalf("disable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts/values:correct", `{"dict_code":"org_type","code":"10","label":"X","correction_day":"2026-01-01","request_code":"r1"}`); rec.Code != http.StatusConflict {
		t.Fatalf("correct status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := get("/iam/api/dicts/values/audit?dict_code=org_type&code=10&limit=10"); rec.Code != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

func mustAllowlistPathFromWd(t *testing.T, wd string) string {
	t.Helper()
	p := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	return p
}
