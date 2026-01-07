package superadmin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type stubAuthorizer struct {
	allowed  bool
	enforced bool
	err      error
}

func (s stubAuthorizer) Authorize(string, string, string, string) (bool, bool, error) {
	return s.allowed, s.enforced, s.err
}

func TestWithAuthz_BypassesHealth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := withAuthz(nil, stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_Error(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := withAuthz(nil, stubAuthorizer{err: errors.New("boom")}, next)

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_Forbidden(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := withAuthz(nil, stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_ShadowAllows(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := withAuthz(nil, stubAuthorizer{allowed: false, enforced: false}, next)

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithAuthz_NoCheckPassesThrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := withAuthz(&routing.Classifier{}, stubAuthorizer{allowed: false, enforced: true}, next)

	req := httptest.NewRequest(http.MethodGet, "/something-else", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}
