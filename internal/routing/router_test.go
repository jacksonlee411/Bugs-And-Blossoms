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

func TestRouter_PathPattern_RoutesAndMethodNotAllowed(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"superadmin": {Routes: []Route{
				{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"},
				{Path: "/superadmin/tenants/{tenant_id}/disable", Methods: []string{"POST"}, RouteClass: "ui"},
			}},
		},
	}
	c, err := NewClassifier(a, "superadmin")
	if err != nil {
		t.Fatal(err)
	}

	r := NewRouter(c)
	r.Handle(RouteClassUI, http.MethodPost, "/superadmin/tenants/{tenant_id}/disable", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	okReq := httptest.NewRequest(http.MethodPost, "/superadmin/tenants/abc/disable", nil)
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("status=%d", okRec.Code)
	}

	badMethodReq := httptest.NewRequest(http.MethodGet, "/superadmin/tenants/abc/disable", nil)
	badMethodRec := httptest.NewRecorder()
	r.ServeHTTP(badMethodRec, badMethodReq)
	if badMethodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", badMethodRec.Code)
	}
}

func TestRouter_PathPattern_MergeMethodsAndRecover(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"superadmin": {Routes: []Route{
				{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"},
				{Path: "/superadmin/tenants/{tenant_id}/disable", Methods: []string{"GET", "POST"}, RouteClass: "ui"},
			}},
		},
	}
	c, err := NewClassifier(a, "superadmin")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(c)

	r.Handle(RouteClassUI, http.MethodPost, "/superadmin/tenants/{tenant_id}/disable", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	panicReq := httptest.NewRequest(http.MethodPost, "/superadmin/tenants/t1/disable", nil)
	panicRec := httptest.NewRecorder()
	r.ServeHTTP(panicRec, panicReq)
	if panicRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", panicRec.Code)
	}

	r.Handle(RouteClassUI, http.MethodGet, "/superadmin/tenants/{tenant_id}/disable", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	panicGetReq := httptest.NewRequest(http.MethodGet, "/superadmin/tenants/t1/disable", nil)
	panicGetRec := httptest.NewRecorder()
	r.ServeHTTP(panicGetRec, panicGetReq)
	if panicGetRec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", panicGetRec.Code)
	}

	r.Handle(RouteClassUI, http.MethodGet, "/superadmin/tenants/{tenant_id}/disable", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	okReq := httptest.NewRequest(http.MethodGet, "/superadmin/tenants/t1/disable", nil)
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("status=%d", okRec.Code)
	}
}
