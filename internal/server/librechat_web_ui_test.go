package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibreChatWebUIRedirectsWithoutSession(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("formal entry", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/assistant/librechat", nil)
		req.Host = "localhost:8080"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Result().Header.Get("Location"); loc != "/app/login" {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("protected static", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/librechat-web/registerSW.js", nil)
		req.Host = "localhost:8080"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Result().Header.Get("Location"); loc != "/app/login" {
			t.Fatalf("location=%q", loc)
		}
	})
}

func TestLibreChatWebUIServesIndexAndProtectedAssets(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
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
	sidCookie := loginRec.Result().Cookies()[0]

	t.Run("formal entry index", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/assistant/librechat", nil)
		req.Host = "localhost:8080"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<title>LibreChat</title>") {
			t.Fatalf("missing title body=%q", body)
		}
		if !strings.Contains(body, `<base href="/assets/librechat-web/" />`) {
			t.Fatalf("missing formal base href body=%q", body)
		}
	})

	t.Run("formal entry spa fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/assistant/librechat/c/abc", nil)
		req.Host = "localhost:8080"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "<title>LibreChat</title>") {
			t.Fatalf("unexpected body=%q", rec.Body.String())
		}
	})

	t.Run("protected static file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/librechat-web/registerSW.js", nil)
		req.Host = "localhost:8080"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, "serviceWorker.register('./sw.js'") {
			t.Fatalf("unexpected body=%q", body)
		}
	})
}
