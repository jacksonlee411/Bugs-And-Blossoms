package superadmin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSuperadminSessionMiddleware_RedirectsToLogin_WhenMissingCookie(t *testing.T) {
	h := newAuthedHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/superadmin/login" {
		t.Fatalf("location=%q", loc)
	}
}

func TestSuperadminLogin_GET(t *testing.T) {
	h := newAuthedHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := httptest.NewRequest(http.MethodGet, "/superadmin/login", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SuperAdmin Login") {
		t.Fatal("missing title")
	}
}

func TestSuperadminLogin_POST_InvalidCredentials(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")
	principals := newMemoryPrincipalStore()
	sessions := newMemorySessionStore()

	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
		IdentityProvider: stubIdentityProvider{err: errInvalidCredentials},
		Principals:       principals,
		Sessions:         sessions,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/superadmin/login", strings.NewReader("email=a@b.com&password=bad"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid credentials") {
		t.Fatal("expected invalid credentials message")
	}
}

func TestSuperadminLogin_POST_MissingFields(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")
	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
		IdentityProvider: stubIdentityProvider{},
		Principals:       newMemoryPrincipalStore(),
		Sessions:         newMemorySessionStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/superadmin/login", strings.NewReader("email=admin@example.invalid&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestSuperadminLogin_POST_IDPError(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")
	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
		IdentityProvider: stubIdentityProvider{err: errors.New("boom")},
		Principals:       newMemoryPrincipalStore(),
		Sessions:         newMemorySessionStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/superadmin/login", strings.NewReader("email=admin@example.invalid&password=pw"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

type errPrincipalStore struct{}

func (errPrincipalStore) UpsertFromKratos(context.Context, string, string) (superadminPrincipal, error) {
	return superadminPrincipal{}, errors.New("boom")
}
func (errPrincipalStore) GetByID(context.Context, string) (superadminPrincipal, bool, error) {
	return superadminPrincipal{}, false, nil
}

func TestSuperadminLogin_POST_PrincipalError(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")
	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
		IdentityProvider: stubIdentityProvider{ident: authenticatedIdentity{Email: "admin@example.invalid", KratosIdentityID: "kid-1"}},
		Principals:       errPrincipalStore{},
		Sessions:         newMemorySessionStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/superadmin/login", strings.NewReader("email=admin@example.invalid&password=pw"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

type errSessionStore struct{}

func (errSessionStore) Create(context.Context, string, time.Time, string, string) (string, error) {
	return "", errors.New("boom")
}
func (errSessionStore) Lookup(context.Context, string) (superadminSession, bool, error) {
	return superadminSession{}, false, nil
}
func (errSessionStore) Revoke(context.Context, string) error { return nil }

func TestSuperadminLogin_POST_SessionError(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")
	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
		IdentityProvider: stubIdentityProvider{ident: authenticatedIdentity{Email: "admin@example.invalid", KratosIdentityID: "kid-1"}},
		Principals:       newMemoryPrincipalStore(),
		Sessions:         errSessionStore{},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/superadmin/login", strings.NewReader("email=admin@example.invalid&password=pw"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestSuperadminLogin_POST_Success_SetsCookieAndAllowsAccess(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")
	principals := newMemoryPrincipalStore()
	sessions := newMemorySessionStore()
	idp := stubIdentityProvider{
		ident: authenticatedIdentity{Email: "admin@example.invalid", KratosIdentityID: "kid-1"},
	}

	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.tenants") {
					return &stubRows{vals: nil}, nil
				}
				if strings.Contains(sql, "FROM iam.tenant_domains") {
					return &stubRows{vals: nil}, nil
				}
				return &stubRows{vals: nil}, nil
			},
		},
		IdentityProvider: idp,
		Principals:       principals,
		Sessions:         sessions,
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/superadmin/login", strings.NewReader("email=admin@example.invalid&password=pw"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusFound {
		t.Fatalf("status=%d", loginRec.Code)
	}
	if loc := loginRec.Header().Get("Location"); loc != "/superadmin/tenants" {
		t.Fatalf("location=%q", loc)
	}

	setCookie := loginRec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, saSidCookieName+"=") || !strings.Contains(setCookie, "Path=/superadmin") || !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("unexpected Set-Cookie=%q", setCookie)
	}

	saSid := strings.TrimPrefix(strings.Split(setCookie, ";")[0], saSidCookieName+"=")
	if saSid == "" {
		t.Fatal("expected sa_sid value")
	}

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: saSid})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestSuperadminLogout_ClearsCookie(t *testing.T) {
	h := newAuthedHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/logout", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/superadmin/login" {
		t.Fatalf("location=%q", loc)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), saSidCookieName+"=") {
		t.Fatal("expected Set-Cookie")
	}
}

func TestSASidTTLFromEnv_DefaultAndInvalid(t *testing.T) {
	t.Setenv("SA_SID_TTL_HOURS", "")
	if got := saSidTTLFromEnv(); got != 8*time.Hour {
		t.Fatalf("got=%s", got)
	}

	t.Setenv("SA_SID_TTL_HOURS", "nope")
	if got := saSidTTLFromEnv(); got != 8*time.Hour {
		t.Fatalf("got=%s", got)
	}

	t.Setenv("SA_SID_TTL_HOURS", "-1")
	if got := saSidTTLFromEnv(); got != 8*time.Hour {
		t.Fatalf("got=%s", got)
	}

	t.Setenv("SA_SID_TTL_HOURS", "1")
	if got := saSidTTLFromEnv(); got != time.Hour {
		t.Fatalf("got=%s", got)
	}
}

func TestNewSASID_ReadError(t *testing.T) {
	old := saSidRandReader
	t.Cleanup(func() { saSidRandReader = old })
	saSidRandReader = errReader{}

	if _, _, err := newSASID(); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryPrincipalStore_ReadError(t *testing.T) {
	old := superadminPrincipalRandReader
	t.Cleanup(func() { superadminPrincipalRandReader = old })
	superadminPrincipalRandReader = errReader{}

	s := newMemoryPrincipalStore()
	if _, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryPrincipalStore_KratosMismatch(t *testing.T) {
	s := newMemoryPrincipalStore()
	if _, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-2"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSuperadminSessionMiddleware_DisabledPrincipal_FailClosed(t *testing.T) {
	principals := newMemoryPrincipalStore()
	sessions := newMemorySessionStore()
	p, err := principals.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
	if err != nil {
		t.Fatal(err)
	}
	p.Status = "disabled"
	principals.byKey[p.Email] = p
	principals.byID[p.ID] = p
	saSid, err := sessions.Create(context.Background(), p.ID, time.Now().Add(time.Hour), "ip", "ua")
	if err != nil {
		t.Fatal(err)
	}

	protected := withSuperadminSession(sessions, principals, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("unexpected next")
	}))

	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: saSid})
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/superadmin/login" {
		t.Fatalf("location=%q", loc)
	}
}

func TestReadSASID_EmptyValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
	req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: ""})
	if _, ok := readSASID(req); ok {
		t.Fatal("expected missing")
	}
}

func TestMemorySessionStore_Create_ReadError(t *testing.T) {
	old := saSidRandReader
	t.Cleanup(func() { saSidRandReader = old })
	saSidRandReader = errReader{}

	s := newMemorySessionStore()
	if _, err := s.Create(context.Background(), "p1", time.Now().Add(time.Hour), "ip", "ua"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemorySessionStore_Lookup_RevokedAndExpired(t *testing.T) {
	s := newMemorySessionStore()
	now := time.Now()
	s.bySID["revoked"] = superadminSession{PrincipalID: "p1", ExpiresAt: now.Add(time.Hour), RevokedAt: &now}
	s.bySID["expired"] = superadminSession{PrincipalID: "p1", ExpiresAt: now.Add(-time.Second)}

	if _, ok, err := s.Lookup(context.Background(), "revoked"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.Lookup(context.Background(), "expired"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.Lookup(context.Background(), "missing"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestMemoryPrincipalStore_Existing_StatusDisabled(t *testing.T) {
	s := newMemoryPrincipalStore()
	p, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
	if err != nil {
		t.Fatal(err)
	}
	p.Status = "disabled"
	s.byKey[p.Email] = p
	s.byID[p.ID] = p

	if _, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryPrincipalStore_Existing_SetsKratosIDWhenMissing(t *testing.T) {
	s := newMemoryPrincipalStore()
	p, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
	if err != nil {
		t.Fatal(err)
	}
	p.KratosIdentityID = ""
	s.byKey[p.Email] = p
	s.byID[p.ID] = p

	got, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.KratosIdentityID != "kid-1" {
		t.Fatalf("got=%+v", got)
	}
}

func TestMemoryPrincipalStore_GetByID(t *testing.T) {
	s := newMemoryPrincipalStore()
	p, err := s.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetByID(context.Background(), p.ID)
	if err != nil || !ok || got.ID != p.ID {
		t.Fatalf("ok=%v err=%v got=%+v", ok, err, got)
	}

	if _, ok, err := s.GetByID(context.Background(), "missing"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

type stubSessionStore struct {
	lookupFn func(ctx context.Context, saSid string) (superadminSession, bool, error)
}

func (s stubSessionStore) Create(context.Context, string, time.Time, string, string) (string, error) {
	return "", nil
}
func (s stubSessionStore) Lookup(ctx context.Context, saSid string) (superadminSession, bool, error) {
	return s.lookupFn(ctx, saSid)
}
func (s stubSessionStore) Revoke(context.Context, string) error { return nil }

type stubPrincipalStore struct {
	getByIDFn func(ctx context.Context, principalID string) (superadminPrincipal, bool, error)
}

func (s stubPrincipalStore) UpsertFromKratos(context.Context, string, string) (superadminPrincipal, error) {
	return superadminPrincipal{}, nil
}
func (s stubPrincipalStore) GetByID(ctx context.Context, principalID string) (superadminPrincipal, bool, error) {
	return s.getByIDFn(ctx, principalID)
}

func TestSuperadminSessionMiddleware_ErrorsAndNotFound(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected next") })

	t.Run("lookup_error", func(t *testing.T) {
		h := withSuperadminSession(
			stubSessionStore{lookupFn: func(context.Context, string) (superadminSession, bool, error) {
				return superadminSession{}, false, errors.New("boom")
			}},
			stubPrincipalStore{getByIDFn: func(context.Context, string) (superadminPrincipal, bool, error) {
				return superadminPrincipal{}, false, nil
			}},
			next,
		)
		req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
		req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: "x"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("session_not_found", func(t *testing.T) {
		h := withSuperadminSession(
			stubSessionStore{lookupFn: func(context.Context, string) (superadminSession, bool, error) {
				return superadminSession{}, false, nil
			}},
			stubPrincipalStore{getByIDFn: func(context.Context, string) (superadminPrincipal, bool, error) {
				return superadminPrincipal{}, false, nil
			}},
			next,
		)
		req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
		req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: "x"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("principal_lookup_error", func(t *testing.T) {
		h := withSuperadminSession(
			stubSessionStore{lookupFn: func(context.Context, string) (superadminSession, bool, error) {
				return superadminSession{PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)}, true, nil
			}},
			stubPrincipalStore{getByIDFn: func(context.Context, string) (superadminPrincipal, bool, error) {
				return superadminPrincipal{}, false, errors.New("boom")
			}},
			next,
		)
		req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
		req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: "x"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("principal_not_found", func(t *testing.T) {
		h := withSuperadminSession(
			stubSessionStore{lookupFn: func(context.Context, string) (superadminSession, bool, error) {
				return superadminSession{PrincipalID: "p1", ExpiresAt: time.Now().Add(time.Hour)}, true, nil
			}},
			stubPrincipalStore{getByIDFn: func(context.Context, string) (superadminPrincipal, bool, error) {
				return superadminPrincipal{}, false, nil
			}},
			next,
		)
		req := httptest.NewRequest(http.MethodGet, "/superadmin/tenants", nil)
		req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: "x"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestInsertAudit_PayloadDefault(t *testing.T) {
	tx := &stubTx{}
	if err := insertAudit(context.Background(), tx, "p1", "action", "00000000-0000-0000-0000-000000000001", nil, "rid"); err != nil {
		t.Fatal(err)
	}
}

type stubFullPool struct {
	stubPool
}

func (stubFullPool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (stubFullPool) QueryRow(context.Context, string, ...any) pgx.Row {
	return stubRow{err: pgx.ErrNoRows}
}

func TestNewHandlerWithOptions_KratosEnvError(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("AUTHZ_MODE", "enforce")
	t.Setenv("KRATOS_PUBLIC_URL", "ftp://x")

	_, err = NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_PoolImplementsQueryExecer(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("AUTHZ_MODE", "enforce")

	_, err = NewHandlerWithOptions(HandlerOptions{
		Pool: stubFullPool{
			stubPool: stubPool{
				beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
			},
		},
		IdentityProvider: stubIdentityProvider{},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSuperadminLogin_POST_ParseFormError(t *testing.T) {
	t.Setenv("AUTHZ_MODE", "enforce")

	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
		IdentityProvider: stubIdentityProvider{},
		Principals:       newMemoryPrincipalStore(),
		Sessions:         newMemorySessionStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/superadmin/login", errReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestSuperadminLogout_NoCookie(t *testing.T) {
	h := newAuthedHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := httptest.NewRequest(http.MethodPost, "/superadmin/logout", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTenantsCreate_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handleTenantsCreate(rec, req, stubPool{beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil }})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTenantToggle_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/superadmin/tenants/t1/enable", nil)
	rec := httptest.NewRecorder()
	handleTenantToggle(rec, req, stubPool{beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil }}, true)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTenantBindDomain_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handleTenantBindDomain(rec, req, stubPool{beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil }})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}
