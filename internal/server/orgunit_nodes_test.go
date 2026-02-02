package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestParseOrgID8(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "len_mismatch", input: "123", wantErr: true},
		{name: "non_digit", input: "12ab5678", wantErr: true},
		{name: "out_of_range", input: "00000000", wantErr: true},
		{name: "ok", input: "10000001", wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOrgID8(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseOptionalOrgID8(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, ok, err := parseOptionalOrgID8("")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, _, err := parseOptionalOrgID8("bad"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("ok", func(t *testing.T) {
		got, ok, err := parseOptionalOrgID8("10000001")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if got != 10000001 {
			t.Fatalf("expected 10000001, got %d", got)
		}
	})
}

func TestOrgUnitMemoryStore(t *testing.T) {
	s := newOrgUnitMemoryStore()
	s.now = func() time.Time { return time.Unix(123, 0).UTC() }

	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "Hello World", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, "Hello"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
	if nodes[0].Name != "Hello" {
		t.Fatalf("name=%q", nodes[0].Name)
	}
	if nodes[0].CreatedAt != time.Unix(123, 0).UTC() {
		t.Fatalf("created_at=%s", nodes[0].CreatedAt)
	}
	if _, err := s.ResolveOrgID(context.Background(), "t1", "A001"); !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		t.Fatalf("expected ErrOrgCodeNotFound, got %v", err)
	}
	if _, err := s.ResolveOrgCode(context.Background(), "t1", 10000001); !errors.Is(err, orgunitpkg.ErrOrgIDNotFound) {
		t.Fatalf("expected ErrOrgIDNotFound, got %v", err)
	}
}

func TestOrgUnitPGStore_ResolveSetID(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid org id", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "nope", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"S2601"}}
			return tx, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"S2601"}}
			return tx, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveSetID(ctx, "t1", "10000001", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got != "S2601" {
			t.Fatalf("expected S2601, got %q", got)
		}
	})
}

func TestOrgUnitPGStore_ResolveOrgID(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgID(ctx, "t1", "A1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveOrgID(ctx, "t1", "A1")
		if err != nil || got != 10000001 {
			t.Fatalf("got=%d err=%v", got, err)
		}
	})
}

func TestOrgUnitPGStore_ResolveOrgCode(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("resolve")}, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"A1"}}
			return tx, nil
		})).(*orgUnitPGStore)
		if _, err := store.ResolveOrgCode(ctx, "t1", 10000001); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"A1"}}
			return tx, nil
		})).(*orgUnitPGStore)
		got, err := store.ResolveOrgCode(ctx, "t1", 10000001)
		if err != nil || got != "A1" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})
}

func TestOrgUnitMemoryStore_ResolveSetID(t *testing.T) {
	store := newOrgUnitMemoryStore()

	if _, err := store.ResolveSetID(context.Background(), "t1", "", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
	got, err := store.ResolveSetID(context.Background(), "t1", "10000001", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "S2601" {
		t.Fatalf("expected S2601, got %q", got)
	}
}

func TestOrgUnitMemoryStore_RenameNodeCurrent_Errors(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "", "B"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002", "B"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitMemoryStore_RenameNodeCurrent_Success(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, "B"); err != nil {
		t.Fatalf("err=%v", err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "B" {
		t.Fatalf("nodes=%v", nodes)
	}
}

func TestOrgUnitMemoryStore_MoveDisableNodeCurrent(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID, ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000002"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", created.ID); err != nil {
		t.Fatalf("err=%v", err)
	}

	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Fatalf("len=%d", len(nodes))
	}
}

func TestOrgUnitMemoryStore_SetBusinessUnitCurrent(t *testing.T) {
	s := newOrgUnitMemoryStore()
	created, err := s.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", "", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", "10000002", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-06", created.ID, true, "r1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	nodes, err := s.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || !nodes[0].IsBusinessUnit {
		t.Fatalf("nodes=%v", nodes)
	}
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_Errors(t *testing.T) {
	cases := []struct {
		name          string
		store         *orgUnitPGStore
		effectiveDate string
		orgID         string
		requestID     string
		randError     bool
	}{
		{
			name: "begin error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin fail")
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "set_config error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec fail"), execErrAt: 1}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "missing effective date",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			effectiveDate: "",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "missing org id",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "",
			requestID:     "r1",
		},
		{
			name: "event id error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
			randError:     true,
		},
		{
			name: "savepoint error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{
					row:       &stubRow{vals: []any{"e1"}},
					execErr:   errors.New("exec fail"),
					execErrAt: 2,
				}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "submit error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{row: &stubRow{vals: []any{"e1"}}, execErr: errors.New("exec fail"), execErrAt: 3}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
		{
			name: "commit error",
			store: &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{row: &stubRow{vals: []any{"e1"}}, commitErr: errors.New("commit fail")}, nil
			})},
			effectiveDate: "2026-01-01",
			orgID:         "10000001",
			requestID:     "r1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := func() {
				if err := tc.store.SetBusinessUnitCurrent(context.Background(), "t1", tc.effectiveDate, tc.orgID, true, tc.requestID); err == nil {
					t.Fatal("expected error")
				}
			}
			if tc.randError {
				withRandReader(t, randErrReader{}, call)
				return
			}
			call()
		})
	}
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_Success(t *testing.T) {
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return &stubTx{row: &stubRow{vals: []any{"e1"}}}, nil
	})}
	if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, ""); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_Idempotent(t *testing.T) {
	dupErr := &pgconn.PgError{Code: "23505", ConstraintName: "org_events_one_per_day_unique"}

	t.Run("already-set", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:       &stubRow{vals: []any{true}},
				execErr:   dupErr,
				execErrAt: 3,
			}, nil
		})}
		if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:       &stubRow{vals: []any{false}},
				execErr:   dupErr,
				execErrAt: 3,
			}, nil
		})}
		if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

type rollbackErrTx struct {
	*stubTx
	rollbackErr error
}

func (t *rollbackErrTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "ROLLBACK TO SAVEPOINT") {
		return pgconn.CommandTag{}, t.rollbackErr
	}
	return t.stubTx.Exec(ctx, sql, args...)
}

func TestOrgUnitPGStore_SetBusinessUnitCurrent_RollbackError(t *testing.T) {
	tx := &rollbackErrTx{
		stubTx: &stubTx{
			row:       &stubRow{vals: []any{"e1"}},
			execErr:   errors.New("exec fail"),
			execErrAt: 3,
		},
		rollbackErr: errors.New("rollback fail"),
	}
	store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if err := store.SetBusinessUnitCurrent(context.Background(), "t1", "2026-01-01", "10000001", true, "r1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleOrgNodes_MissingTenant(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	handleOrgNodes(rec, req, newOrgUnitMemoryStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_GET_HX(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || bytes.Contains(rec.Body.Bytes(), []byte("<!doctype html>")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)

	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "A") {
		t.Fatalf("unexpected body: %q", body)
	}
}

type errStore struct{}

func (errStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, errBoom{}
}
func (errStore) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, errBoom{}
}
func (errStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return errBoom{}
}
func (errStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return errBoom{}
}
func (errStore) DisableNodeCurrent(context.Context, string, string, string) error { return errBoom{} }
func (errStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return errBoom{}
}
func (errStore) ResolveOrgID(context.Context, string, string) (int, error)   { return 0, errBoom{} }
func (errStore) ResolveOrgCode(context.Context, string, int) (string, error) { return "", errBoom{} }

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

type emptyErr struct{}

func (emptyErr) Error() string { return "" }

type emptyErrStore struct{}

func (emptyErrStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, emptyErr{}
}
func (emptyErrStore) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, emptyErr{}
}
func (emptyErrStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return emptyErr{}
}
func (emptyErrStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return emptyErr{}
}
func (emptyErrStore) ResolveOrgID(context.Context, string, string) (int, error) {
	return 0, emptyErr{}
}
func (emptyErrStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "", emptyErr{}
}

func TestHandleOrgNodes_GET_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-06", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("boom")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_GET_BadAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=bad", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("invalid as_of")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_BadForm(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_BadForm_MergesEmptyStoreError(t *testing.T) {
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, emptyErrStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "bad form") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_BadForm_MergesNonEmptyStoreError(t *testing.T) {
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	bodyOut := rec.Body.String()
	if !strings.Contains(bodyOut, "bad form") || !strings.Contains(bodyOut, "boom") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_EmptyName(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", bytes.NewBufferString("name="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("name is required")) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleOrgNodes_POST_SuccessRedirect(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("name=A&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?as_of=2026-01-06" {
		t.Fatalf("location=%q", loc)
	}
}

type writeSpyStore struct {
	createCalled  int
	renameCalled  int
	moveCalled    int
	disableCalled int

	argsCreate  []string
	argsRename  []string
	argsMove    []string
	argsDisable []string

	err error
}

func (s *writeSpyStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return []OrgUnitNode{{ID: "c1", Name: "C"}}, nil
}
func (s *writeSpyStore) CreateNodeCurrent(_ context.Context, tenantID string, effectiveDate string, name string, parentID string, _ bool) (OrgUnitNode, error) {
	s.createCalled++
	s.argsCreate = []string{tenantID, effectiveDate, name, parentID}
	if s.err != nil {
		return OrgUnitNode{}, s.err
	}
	return OrgUnitNode{ID: "10000001", Name: name, CreatedAt: time.Unix(1, 0).UTC()}, nil
}
func (s *writeSpyStore) RenameNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newName string) error {
	s.renameCalled++
	s.argsRename = []string{tenantID, effectiveDate, orgID, newName}
	return s.err
}
func (s *writeSpyStore) MoveNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	s.moveCalled++
	s.argsMove = []string{tenantID, effectiveDate, orgID, newParentID}
	return s.err
}
func (s *writeSpyStore) DisableNodeCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string) error {
	s.disableCalled++
	s.argsDisable = []string{tenantID, effectiveDate, orgID}
	return s.err
}
func (s *writeSpyStore) SetBusinessUnitCurrent(_ context.Context, _ string, _ string, _ string, _ bool, _ string) error {
	return s.err
}
func (s *writeSpyStore) ResolveOrgID(context.Context, string, string) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return 10000001, nil
}
func (s *writeSpyStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "A001", nil
}

type asOfSpyStore struct {
	gotAsOf string
}

func (s *asOfSpyStore) ListNodesCurrent(_ context.Context, _ string, asOf string) ([]OrgUnitNode, error) {
	s.gotAsOf = asOf
	return []OrgUnitNode{{ID: "c1", Name: "C"}}, nil
}
func (s *asOfSpyStore) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}
func (s *asOfSpyStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *asOfSpyStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (s *asOfSpyStore) DisableNodeCurrent(context.Context, string, string, string) error { return nil }
func (s *asOfSpyStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return nil
}
func (s *asOfSpyStore) ResolveOrgID(context.Context, string, string) (int, error) {
	return 10000001, nil
}
func (s *asOfSpyStore) ResolveOrgCode(context.Context, string, int) (string, error) {
	return "A001", nil
}

func TestHandleOrgNodes_POST_Rename_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=10000001&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 1 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
	if got := strings.Join(store.argsRename, "|"); got != "t1|2026-01-05|10000001|New" {
		t.Fatalf("args=%q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "/org/nodes?as_of=2026-01-05" {
		t.Fatalf("location=%q", loc)
	}
}

func TestHandleOrgNodes_POST_Rename_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=10000001&new_name=New")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsRename, "|"); got != "t1|2026-01-06|10000001|New" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_InvalidFlag(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_id", created.ID)
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "maybe")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "is_business_unit 无效") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_Success(t *testing.T) {
	store := newOrgUnitMemoryStore()
	created, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_id", created.ID)
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "true")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	nodes, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || !nodes[0].IsBusinessUnit {
		t.Fatalf("nodes=%v", nodes)
	}
}

func TestHandleOrgNodes_POST_SetBusinessUnit_StoreError(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	form := url.Values{}
	form.Set("action", "set_business_unit")
	form.Set("org_id", "10000001")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "true")

	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleOrgNodes_POST_Rename_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=rename&org_id=10000001&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_BadEffectiveDate(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("name=A&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.createCalled != 0 {
		t.Fatalf("createCalled=%d", store.createCalled)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "effective_date 无效") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_BusinessUnitFalse(t *testing.T) {
	store := &writeSpyStore{}
	form := url.Values{}
	form.Set("name", "A")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "no")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.createCalled != 1 {
		t.Fatalf("createCalled=%d", store.createCalled)
	}
}

func TestHandleOrgNodes_POST_Create_InvalidBusinessUnitFlag(t *testing.T) {
	store := &writeSpyStore{}
	form := url.Values{}
	form.Set("name", "A")
	form.Set("effective_date", "2026-01-06")
	form.Set("is_business_unit", "maybe")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "is_business_unit 无效") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Create_Error_ShowsError(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("name=A&effective_date=2026-01-06")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=move&org_id=10000001&new_parent_id=10000002&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.moveCalled != 1 {
		t.Fatalf("moveCalled=%d", store.moveCalled)
	}
	if got := strings.Join(store.argsMove, "|"); got != "t1|2026-01-05|10000001|10000002" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Disable_UsesStore(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=disable&org_id=10000001&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 1 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if got := strings.Join(store.argsDisable, "|"); got != "t1|2026-01-05|10000001" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Disable_BadEffectiveDate(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=disable&org_id=10000001&effective_date=bad")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.disableCalled != 0 {
		t.Fatalf("disableCalled=%d", store.disableCalled)
	}
	if bodyOut := rec.Body.String(); !bytes.Contains([]byte(bodyOut), []byte("effective_date 无效")) {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Move_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=move&org_id=10000001&new_parent_id=10000002&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_Disable_Error_ShowsErrorAndNodes(t *testing.T) {
	store := &writeSpyStore{err: errors.New("boom")}
	body := bytes.NewBufferString("action=disable&org_id=10000001&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "boom") || !strings.Contains(bodyOut, "C") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_MergesErrorHints(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("name=")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=bad", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if bodyOut := rec.Body.String(); !strings.Contains(bodyOut, "invalid as_of") {
		t.Fatalf("unexpected body: %q", bodyOut)
	}
}

func TestHandleOrgNodes_POST_DefaultsEffectiveDateToAsOf(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("name=A")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := strings.Join(store.argsCreate, "|"); got != "t1|2026-01-06|A|" {
		t.Fatalf("args=%q", got)
	}
}

func TestHandleOrgNodes_POST_Rename_MissingOrgID(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=&new_name=New&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
}

func TestHandleOrgNodes_POST_Rename_MissingNewName(t *testing.T) {
	store := &writeSpyStore{}
	body := bytes.NewBufferString("action=rename&org_id=10000001&new_name=&effective_date=2026-01-05")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes?as_of=2026-01-06", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if store.renameCalled != 0 {
		t.Fatalf("renameCalled=%d", store.renameCalled)
	}
}

func TestHandleOrgNodes_GET_DefaultAsOf_UsesToday(t *testing.T) {
	store := &asOfSpyStore{}
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/org/nodes?as_of=") {
		t.Fatalf("location=%q", loc)
	}
	wantAsOf := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(loc, wantAsOf) {
		t.Fatalf("location=%q wantAsOf=%q", loc, wantAsOf)
	}
}

func TestHandleOrgNodes_MethodNotAllowed(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPut, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_TenantMissing(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodGet, "/org/nodes?as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRenderOrgNodes(t *testing.T) {
	out := renderOrgNodes(nil, Tenant{Name: "T"}, "", "2026-01-06")
	if out == "" {
		t.Fatal("expected output")
	}
	out2 := renderOrgNodes([]OrgUnitNode{{ID: "1", Name: "N", IsBusinessUnit: true}}, Tenant{Name: "T"}, "err", "2026-01-06")
	if out2 == "" {
		t.Fatal("expected output")
	}
	if !strings.Contains(out2, "(BU)") || !strings.Contains(out2, "checked") {
		t.Fatalf("unexpected output: %q", out2)
	}
}

func TestOrgUnitPGStore_ListNodesCurrent_AndCreateCurrent(t *testing.T) {
	pool := &fakeBeginner{}
	store := &orgUnitPGStore{pool: pool}

	nodes, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodes)
	}

	createdCurrent, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "Current", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if createdCurrent.ID != "10000001" || createdCurrent.Name != "Current" || !createdCurrent.CreatedAt.Equal(time.Unix(789, 0).UTC()) {
		t.Fatalf("created=%+v", createdCurrent)
	}

	createdWithParent, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "CurrentWithParent", "10000002", false)
	if err != nil {
		t.Fatal(err)
	}
	if createdWithParent.ID == "" {
		t.Fatal("expected id")
	}
}

func TestOrgUnitPGStore_ListNodesCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ListNodesCurrent(context.Background(), "t1", "2026-01-06")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_CreateNodeCurrent_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date_required", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("parent_id_invalid", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "bad", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("org_id_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id_error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})}
		withRandReader(t, randErrReader{}, func() {
			if _, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("transaction_time_scan", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{10000001}}
			tx.row2 = &stubRow{err: errors.New("row2")}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_exec", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{10000001}}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{10000001}}
			tx.row2 = &stubRow{vals: []any{time.Unix(1, 0).UTC()}}
			return tx, nil
		})}
		_, err := store.CreateNodeCurrent(context.Background(), "t1", "2026-01-06", "A", "", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_RenameMoveDisableCurrent(t *testing.T) {
	t.Run("rename_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("rename_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("new_name_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_error", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			withRandReader(t, randErrReader{}, func() {
				if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
					t.Fatal("expected error")
				}
			})
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.RenameNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "New"); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("move_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("move_success_with_parent", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "10000002"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("move_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("new_parent_invalid", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", "bad"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_error", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			withRandReader(t, randErrReader{}, func() {
				if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
					t.Fatal("expected error")
				}
			})
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.MoveNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001", ""); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("disable_success", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"e1"}}
			return tx, nil
		})}
		if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("disable_errors", func(t *testing.T) {
		t.Run("begin", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return nil, errors.New("begin")
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("set_config", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{execErr: errors.New("exec")}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("effective_date_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("org_id_required", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", ""); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("event_id_error", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				return &stubTx{}, nil
			})}
			withRandReader(t, randErrReader{}, func() {
				if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
					t.Fatal("expected error")
				}
			})
		})
		t.Run("submit_exec", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("commit", func(t *testing.T) {
			store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
				tx := &stubTx{commitErr: errors.New("commit")}
				tx.row = &stubRow{vals: []any{"e1"}}
				return tx, nil
			})}
			if err := store.DisableNodeCurrent(context.Background(), "t1", "2026-01-06", "10000001"); err == nil {
				t.Fatal("expected error")
			}
		})
	})
}
