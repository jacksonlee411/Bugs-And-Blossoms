package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerWithOptions_OrgUnitWriteRoutes_AreWired(t *testing.T) {
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
		OrgUnitStore:        newOrgUnitMemoryStore(),
		OrgUnitWriteService: fakeOrgUnitWriteService{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Login to get sid cookie.
	login := httptest.NewRequest(http.MethodPost, "http://localhost/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	login.Host = "localhost"
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	sidCookie := loginRec.Result().Cookies()[0]

	t.Run("write-capabilities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/org/api/org-units/write-capabilities?intent=create_org&org_code=A001&effective_date=2026-01-01", nil)
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("expected route to be wired, got 404")
		}
	})

	t.Run("write", func(t *testing.T) {
		body := `{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_code":"r1","patch":{"name":"Root A"}}`
		req := httptest.NewRequest(http.MethodPost, "http://localhost/org/api/org-units/write", strings.NewReader(body))
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("expected route to be wired, got 404")
		}
	})
}
