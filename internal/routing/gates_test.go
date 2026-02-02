package routing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGateA_NoNonVersionedAPI(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"server": {Routes: []Route{{Path: "/api/v1/ping", Methods: []string{"GET"}, RouteClass: "public_api"}}},
		},
	}

	for _, ep := range a.Entrypoints {
		for _, r := range ep.Routes {
			if strings.HasPrefix(r.Path, "/api/") && !strings.HasPrefix(r.Path, "/api/v1/") {
				t.Fatalf("non-versioned api route: %s", r.Path)
			}
		}
	}
}

func TestGateB_AllowlistLoadsAndEntrypointsPresent(t *testing.T) {
	t.Parallel()

	_, err := NewClassifier(Allowlist{Version: 1, Entrypoints: map[string]Entrypoint{}}, "server")
	if err == nil {
		t.Fatal("expected error")
	}

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"server":     {Routes: []Route{{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"}}},
			"superadmin": {Routes: []Route{{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"}}},
		},
	}
	_, err = NewClassifier(a, "server")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGateC_JSONOnlyErrorsForAPIAndWebhook(t *testing.T) {
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

	tests := []struct {
		path string
	}{
		{path: "/org/api/unknown"},
		{path: "/api/v1/unknown"},
		{path: "/webhooks/foo/unknown"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("path=%s status=%d", tt.path, rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("path=%s content-type=%q", tt.path, ct)
		}
	}

	uiReq := httptest.NewRequest(http.MethodGet, "/orgunit/unknown", nil)
	uiRec := httptest.NewRecorder()
	r.ServeHTTP(uiRec, uiReq)
	if uiRec.Code != http.StatusNotFound {
		t.Fatalf("ui status=%d", uiRec.Code)
	}
	if !strings.HasPrefix(uiRec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("ui content-type=%q", uiRec.Header().Get("Content-Type"))
	}

	uiJSONReq := httptest.NewRequest(http.MethodGet, "/orgunit/unknown", nil)
	uiJSONReq.Header.Set("Accept", "application/json")
	uiJSONRec := httptest.NewRecorder()
	r.ServeHTTP(uiJSONRec, uiJSONReq)
	if !strings.HasPrefix(uiJSONRec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("ui json content-type=%q", uiJSONRec.Header().Get("Content-Type"))
	}
}

func TestGateC_MethodNotAllowed(t *testing.T) {
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
	r.Handle(RouteClassOps, http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
}
