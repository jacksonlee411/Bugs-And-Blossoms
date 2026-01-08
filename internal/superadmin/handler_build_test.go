package superadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestNewHandler_DefaultAllowlistNotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	t.Setenv("SUPERADMIN_DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	_, err = NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMustNewHandler_Panics(t *testing.T) {
	t.Setenv("SUPERADMIN_DATABASE_URL", "")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustNewHandler()
}

func TestNewHandlerWithOptions_RootRedirect(t *testing.T) {
	h := newAuthedHandler(t, stubPool{
		beginFn: func(ctx context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/superadmin/tenants" {
		t.Fatalf("location=%q", loc)
	}
}

func TestNewHandlerWithOptions_NoAuthzCheckForUnknownPath(t *testing.T) {
	h := newAuthedHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/unknown", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandlerWithOptions_AllowlistPathFromEnv(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("nil handler")
	}
}

func TestNewHandlerWithOptions_LoadAllowlistError(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "allowlist.yaml")
	if err := os.WriteFile(tmp, []byte("not: [valid: yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", tmp)
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	_, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_ClassifierError(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "allowlist.yaml")
	if err := os.WriteFile(tmp, []byte("version: 1\nentrypoints:\n  server:\n    routes: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", tmp)
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	_, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_PoolNil_DBEnvMissing(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("SUPERADMIN_DATABASE_URL", "")

	_, err = NewHandlerWithOptions(HandlerOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_PoolNil_BadDSN(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("SUPERADMIN_DATABASE_URL", "not-a-dsn")

	_, err = NewHandlerWithOptions(HandlerOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_PoolNil_Success(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("SUPERADMIN_DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	h, err := NewHandlerWithOptions(HandlerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("nil handler")
	}
}

func TestNewHandlerWithOptions_LoadAuthorizerError(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Clean(filepath.Join(wd, "..", "..", "config", "access", "policy.csv"))
	t.Setenv("AUTHZ_MODEL_PATH", filepath.Join(t.TempDir(), "missing-model.conf"))
	t.Setenv("AUTHZ_POLICY_PATH", policy)
	t.Setenv("AUTHZ_MODE", "enforce")

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

func TestMustNewHandler_Success(t *testing.T) {
	allowlist, err := defaultAllowlistPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALLOWLIST_PATH", allowlist)
	t.Setenv("SUPERADMIN_DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "admin")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "admin")

	h := MustNewHandler()
	if h == nil {
		t.Fatal("nil handler")
	}
}

func TestNewHandlerWithOptions_HealthEndpoints(t *testing.T) {
	t.Setenv("SUPERADMIN_BASIC_AUTH_USER", "")
	t.Setenv("SUPERADMIN_BASIC_AUTH_PASS", "")
	t.Setenv("AUTHZ_MODE", "enforce")

	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool: stubPool{
			beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d", rec2.Code)
	}
}
