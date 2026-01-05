package routing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouter_PanicBecomes500JSON(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"server": {Routes: []Route{{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"}}},
		},
	}
	c, err := NewClassifier(a, "server")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(c)
	r.Handle(RouteClassPublicAPI, http.MethodGet, "/api/v1/panic", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
}

func TestRouter_MethodNotAllowed_JSONOnly(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"server": {Routes: []Route{{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"}}},
		},
	}
	c, err := NewClassifier(a, "server")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(c)
	r.Handle(RouteClassPublicAPI, http.MethodGet, "/api/v1/ping", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
}

func TestEntrypointClass_Fallback(t *testing.T) {
	t.Parallel()

	if got := entrypointClass(map[string]routeEntry{}, RouteClassUI); got != RouteClassUI {
		t.Fatalf("got=%q", got)
	}
}
