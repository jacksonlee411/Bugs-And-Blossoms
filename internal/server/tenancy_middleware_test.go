package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type errorTenancyResolver struct{}

func (errorTenancyResolver) ResolveTenant(context.Context, string) (Tenant, bool, error) {
	return Tenant{}, false, errors.New("boom")
}

type stubTenancyResolver struct {
	tenant Tenant
	ok     bool
	err    error
}

func (s stubTenancyResolver) ResolveTenant(context.Context, string) (Tenant, bool, error) {
	return s.tenant, s.ok, s.err
}

type stubSessionStore struct {
	lookupSID string
	sess      Session
	ok        bool
	err       error
}

func (*stubSessionStore) Create(context.Context, string, string, time.Time, string, string) (string, error) {
	panic("unused")
}

func (s *stubSessionStore) Lookup(_ context.Context, sid string) (Session, bool, error) {
	s.lookupSID = sid
	return s.sess, s.ok, s.err
}

func (*stubSessionStore) Revoke(context.Context, string) error { return nil }

type stubPrincipalStore struct {
	p   Principal
	ok  bool
	err error
}

func (*stubPrincipalStore) GetOrCreateTenantAdmin(context.Context, string) (Principal, error) {
	panic("unused")
}

func (*stubPrincipalStore) UpsertFromKratos(context.Context, string, string, string, string) (Principal, error) {
	panic("unused")
}

func (s *stubPrincipalStore) GetByID(context.Context, string, string) (Principal, bool, error) {
	return s.p, s.ok, s.err
}

func mustInternalAPIClassifier(t *testing.T) *routing.Classifier {
	t.Helper()

	c, err := routing.NewClassifier(routing.Allowlist{
		Version: 1,
		Entrypoints: map[string]routing.Entrypoint{
			"server": {
				Routes: []routing.Route{
					{Path: "/org/api/org-units", Methods: []string{"GET"}, RouteClass: "internal_api"},
				},
			},
		},
	}, "server")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestWithTenantAndSession_ResolveError(t *testing.T) {
	h := withTenantAndSession(nil, errorTenancyResolver{}, newMemoryPrincipalStore(), newMemorySessionStore(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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
	h := withTenantAndSession(nil, errorTenancyResolver{}, newMemoryPrincipalStore(), newMemorySessionStore(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestWithTenantAndSession_HealthBypass(t *testing.T) {
	nextCalled := false
	h := withTenantAndSession(nil, errorTenancyResolver{}, newMemoryPrincipalStore(), newMemorySessionStore(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next")
	}
}

func TestWithTenantAndSession_TenantNotFound(t *testing.T) {
	h := withTenantAndSession(nil, stubTenancyResolver{ok: false}, newMemoryPrincipalStore(), newMemorySessionStore(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithTenantAndSession_SessionsBypassInjectsTenant(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	nextCalled := false
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, newMemoryPrincipalStore(), newMemorySessionStore(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if got, ok := currentTenant(r.Context()); !ok || got.ID != tnt.ID {
			t.Fatalf("tenant=%+v ok=%v", got, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next")
	}
}

func TestWithTenantAndSession_MissingSIDRedirects(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, newMemoryPrincipalStore(), newMemorySessionStore(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Result().Header.Get("Location"); loc != "/app/login" {
		t.Fatalf("location=%q", loc)
	}
}

func TestWithTenantAndSession_SessionLookupError(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	sessions := &stubSessionStore{err: errors.New("boom")}
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, newMemoryPrincipalStore(), sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithTenantAndSession_SessionNotFoundClearsCookie(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	sessions := &stubSessionStore{ok: false}
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, newMemoryPrincipalStore(), sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sidCookieName && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatalf("expected %s cookie cleared", sidCookieName)
	}
}

func TestWithTenantAndSession_SessionTenantMismatchClearsCookie(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	sessions := &stubSessionStore{
		ok:   true,
		sess: Session{TenantID: "t2", PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)},
	}
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, newMemoryPrincipalStore(), sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sidCookieName && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatalf("expected %s cookie cleared", sidCookieName)
	}
}

func TestWithTenantAndSession_SessionTenantMismatchInternalAPI_Returns401(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	classifier := mustInternalAPIClassifier(t)
	sessions := &stubSessionStore{
		ok:   true,
		sess: Session{TenantID: "t2", PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)},
	}
	h := withTenantAndSession(classifier, stubTenancyResolver{tenant: tnt, ok: true}, newMemoryPrincipalStore(), sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sidCookieName && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatalf("expected %s cookie cleared", sidCookieName)
	}
}

func TestWithTenantAndSession_PrincipalLookupError(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	sessions := &stubSessionStore{ok: true, sess: Session{TenantID: "t1", PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)}}
	principals := &stubPrincipalStore{err: errors.New("boom")}
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, principals, sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWithTenantAndSession_PrincipalNotFoundOrInactiveClearsCookie(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	sessions := &stubSessionStore{ok: true, sess: Session{TenantID: "t1", PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)}}

	for name, principals := range map[string]*stubPrincipalStore{
		"not_found": {ok: false},
		"inactive":  {ok: true, p: Principal{ID: "p1", TenantID: "t1", Status: "disabled", RoleSlug: "tenant-admin"}},
	} {
		t.Run(name, func(t *testing.T) {
			h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, principals, sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("unexpected next")
			}))

			req := httptest.NewRequest(http.MethodGet, "/app", nil)
			req.Host = "localhost:8080"
			req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusFound {
				t.Fatalf("status=%d", rec.Code)
			}
			var cleared bool
			for _, c := range rec.Result().Cookies() {
				if c.Name == sidCookieName && c.MaxAge < 0 {
					cleared = true
					break
				}
			}
			if !cleared {
				t.Fatalf("expected %s cookie cleared", sidCookieName)
			}
		})
	}
}

func TestWithTenantAndSession_PrincipalNotFoundOrInactiveInternalAPI_Returns401(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	classifier := mustInternalAPIClassifier(t)
	sessions := &stubSessionStore{ok: true, sess: Session{TenantID: "t1", PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)}}

	principals := &stubPrincipalStore{ok: true, p: Principal{ID: "p1", TenantID: "t1", Status: "disabled", RoleSlug: "tenant-admin"}}
	h := withTenantAndSession(classifier, stubTenancyResolver{tenant: tnt, ok: true}, principals, sessions, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sidCookieName && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatalf("expected %s cookie cleared", sidCookieName)
	}
}

func TestWithTenantAndSession_SuccessInjectsPrincipal(t *testing.T) {
	tnt := Tenant{ID: "t1", Domain: "localhost", Name: "Local"}
	sessions := &stubSessionStore{ok: true, sess: Session{TenantID: "t1", PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)}}
	principals := &stubPrincipalStore{ok: true, p: Principal{ID: "p1", TenantID: "t1", Status: "active", RoleSlug: "tenant-admin"}}
	nextCalled := false
	h := withTenantAndSession(nil, stubTenancyResolver{tenant: tnt, ok: true}, principals, sessions, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if p, ok := currentPrincipal(r.Context()); !ok || p.ID != "p1" {
			t.Fatalf("principal=%+v ok=%v", p, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	req.Host = "localhost:8080"
	req.AddCookie(&http.Cookie{Name: sidCookieName, Value: "sid1"})
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
