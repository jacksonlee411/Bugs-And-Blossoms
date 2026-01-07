package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type errorTenancyResolver struct{}

func (errorTenancyResolver) ResolveTenant(context.Context, string) (Tenant, bool, error) {
	return Tenant{}, false, errors.New("boom")
}

func TestWithTenantAndSession_ResolveError(t *testing.T) {
	h := withTenantAndSession(errorTenancyResolver{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithTenantAndSession_AssetsBypass(t *testing.T) {
	nextCalled := false
	h := withTenantAndSession(errorTenancyResolver{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/assets", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next")
	}
}

func TestNewHandlerWithOptions_MissingTenancyResolver(t *testing.T) {
	_, err := NewHandlerWithOptions(HandlerOptions{
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
