package superadmin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func newTestHandler(t *testing.T, pool pgBeginner) authedHandler {
	t.Helper()
	return newAuthedHandler(t, pool)
}

func TestTenantsIndex_Success(t *testing.T) {
	pool := stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}}, nil
			}
			if strings.Contains(sql, "FROM iam.tenant_domains") {
				return &stubRows{vals: [][]any{{"t1", "a.local", true}, {"t1", "b.local", false}}}, nil
			}
			return nil, errors.New("unexpected query")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	}

	h := newTestHandler(t, pool)

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SuperAdmin / Tenants") {
		t.Fatal("missing title")
	}
	if !strings.Contains(rec.Body.String(), "a.local") {
		t.Fatal("missing domain")
	}
}

func TestTenantsIndex_IgnoresUnknownTenantDomains(t *testing.T) {
	pool := stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}}, nil
			}
			if strings.Contains(sql, "FROM iam.tenant_domains") {
				return &stubRows{vals: [][]any{
					{"unknown", "x.local", false},
					{"t1", "a.local", true},
				}}, nil
			}
			return nil, errors.New("unexpected query")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	}

	h := newTestHandler(t, pool)

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsIndex_PrimaryOnlyFirstWins(t *testing.T) {
	pool := stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}}, nil
			}
			if strings.Contains(sql, "FROM iam.tenant_domains") {
				return &stubRows{vals: [][]any{
					{"t1", "a.local", true},
					{"t1", "b.local", true},
				}}, nil
			}
			return nil, errors.New("unexpected query")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	}

	h := newTestHandler(t, pool)

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "a.local") || !strings.Contains(rec.Body.String(), "b.local") {
		t.Fatal("expected both domains in body")
	}
}

func TestTenantsIndex_InactiveTenant(t *testing.T) {
	pool := stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", false}}}, nil
			}
			if strings.Contains(sql, "FROM iam.tenant_domains") {
				return &stubRows{vals: nil}, nil
			}
			return nil, errors.New("unexpected query")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	}

	h := newTestHandler(t, pool)

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no") {
		t.Fatal("expected inactive marker")
	}
	if !strings.Contains(rec.Body.String(), "Enable") {
		t.Fatal("expected enable action")
	}
}

func TestTenantsIndex_QueryError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return nil, errors.New("boom") },
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsIndex_Empty(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: nil}, nil
			}
			return &stubRows{vals: nil}, nil
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "(none)") {
		t.Fatal("expected empty marker")
	}
}

func TestTenantsIndex_ScanError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}, scanErrAt: 1}, nil
			}
			return &stubRows{}, nil
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsIndex_RowsErr(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{}, err: errors.New("rows err")}, nil
			}
			return &stubRows{}, nil
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsIndex_DomainQueryError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}}, nil
			}
			return nil, errors.New("boom")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsIndex_DomainScanError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}}, nil
			}
			if strings.Contains(sql, "FROM iam.tenant_domains") {
				return &stubRows{vals: [][]any{{"t1", "a.local", true}}, scanErrAt: 1}, nil
			}
			return nil, errors.New("unexpected")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsIndex_DomainRowsErr(t *testing.T) {
	h := newTestHandler(t, stubPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.tenants") {
				return &stubRows{vals: [][]any{{"t1", "Tenant 1", true}}}, nil
			}
			if strings.Contains(sql, "FROM iam.tenant_domains") {
				return &stubRows{vals: [][]any{}, err: errors.New("rows err")}, nil
			}
			return nil, errors.New("unexpected")
		},
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
	})

	req := h.newRequest(http.MethodGet, "/superadmin/tenants", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_WriteDisabled(t *testing.T) {
	t.Setenv("SUPERADMIN_WRITE_MODE", "disabled")
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_ParseFormError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", errReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_InvalidInput(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=&hostname="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_InvalidHostname(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local:8080"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_BeginError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return nil, errors.New("boom") },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_InsertTenantError(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{err: errors.New("row err")} },
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_InsertDomainError(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{vals: []any{"t1"}} },
		execErrAt:  1,
		execErr:    errors.New("exec err"),
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_AuditError(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{vals: []any{"t1"}} },
		execErrAt:  4,
		execErr:    errors.New("audit err"),
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_SetTenantForBootstrapError(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{vals: []any{"t1"}} },
		execErrAt:  2,
		execErr:    errors.New("exec err"),
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_BootstrapTimeProfileError(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{vals: []any{"t1"}} },
		execErrAt:  3,
		execErr:    errors.New("exec err"),
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_CommitError(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{vals: []any{"t1"}} },
		commitErr:  errors.New("commit err"),
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantsCreate_Success(t *testing.T) {
	tx := &stubTx{
		queryRowFn: func(string, ...any) pgx.Row { return stubRow{vals: []any{"t1"}} },
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants", strings.NewReader("name=x&hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Request-Id", "rid")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTenantToggle_BadPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/superadmin/tenants", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalCtxKey{}, superadminPrincipal{ID: "p1", Status: "active"}))
	rec := httptest.NewRecorder()
	handleTenantToggle(rec, req, stubPool{beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil }}, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_ExecError(t *testing.T) {
	tx := &stubTx{
		execErrAt: 1,
		execErr:   errors.New("exec err"),
	}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/disable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_BeginError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return nil, errors.New("boom") },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/enable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_CommitError(t *testing.T) {
	tx := &stubTx{commitErr: errors.New("commit err")}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/enable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_EnableSuccess(t *testing.T) {
	tx := &stubTx{}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/enable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_DisableSuccess(t *testing.T) {
	tx := &stubTx{}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/00000000-0000-0000-0000-000000000001/disable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_WriteDisabled(t *testing.T) {
	t.Setenv("SUPERADMIN_WRITE_MODE", "disabled")
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/enable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantToggle_AuditError(t *testing.T) {
	tx := &stubTx{execErrAt: 2, execErr: errors.New("audit err")}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/enable", nil)
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_InvalidHost(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=bad host"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_WriteDisabled(t *testing.T) {
	t.Setenv("SUPERADMIN_WRITE_MODE", "disabled")
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_ParseFormError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", errReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTenantBindDomain_BadPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/superadmin/tenants", nil)
	req = req.WithContext(context.WithValue(req.Context(), principalCtxKey{}, superadminPrincipal{ID: "p1", Status: "active"}))
	rec := httptest.NewRecorder()
	handleTenantBindDomain(rec, req, stubPool{beginFn: func(context.Context) (pgx.Tx, error) { return &stubTx{}, nil }})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_BeginError(t *testing.T) {
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return nil, errors.New("boom") },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_ExecError(t *testing.T) {
	tx := &stubTx{execErrAt: 1, execErr: errors.New("exec err")}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_AuditError(t *testing.T) {
	tx := &stubTx{execErrAt: 2, execErr: errors.New("audit err")}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_CommitError(t *testing.T) {
	tx := &stubTx{commitErr: errors.New("commit err")}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/t1/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTenantBindDomain_Success(t *testing.T) {
	tx := &stubTx{}
	h := newTestHandler(t, stubPool{
		beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil },
		queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil },
	})

	req := h.newRequest(http.MethodPost, "/superadmin/tenants/00000000-0000-0000-0000-000000000001/domains", strings.NewReader("hostname=x.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
}
