package superadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActorFromContext_Missing(t *testing.T) {
	if _, ok := actorFromContext(context.Background()); ok {
		t.Fatal("expected missing")
	}
}

func TestWithBasicAuth_MissingEnv(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "")

	h := withBasicAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.SetBasicAuth("admin", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
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

func TestWithBasicAuth_SetsActor(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	h := withBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFromContext(r.Context())
		if !ok || actor != "admin" {
			t.Fatalf("actor ok=%v actor=%q", ok, actor)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.SetBasicAuth("admin", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
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
