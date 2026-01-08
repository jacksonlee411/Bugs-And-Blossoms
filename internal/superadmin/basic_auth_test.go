package superadmin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithBasicAuth_MissingEnv(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "")

	h := withBasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.SetBasicAuth("admin", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithBasicAuth_BypassesWhenEnvMissing(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "")

	called := false
	h := withBasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.SetBasicAuth("admin", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected next")
	}
}

func TestWithBasicAuth_WrongCreds(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "secret")

	h := withBasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.SetBasicAuth("admin", "bad")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate")
	}
}

func TestWithBasicAuth_MissingHeader(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "secret")

	h := withBasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithBasicAuth_Success(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "secret")

	called := false
	h := withBasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected next")
	}
}

func TestWithBasicAuth_BypassesHealth(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "")

	h := withBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}
