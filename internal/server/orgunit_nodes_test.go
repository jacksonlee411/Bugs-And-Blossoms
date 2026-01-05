package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestOrgUnitMemoryStore(t *testing.T) {
	s := newOrgUnitMemoryStore()
	s.now = func() time.Time { return time.Unix(123, 0).UTC() }

	if _, err := s.CreateNode(context.Background(), "t1", "Hello World"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodes(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
	if nodes[0].Name != "Hello World" {
		t.Fatalf("name=%q", nodes[0].Name)
	}
	if nodes[0].CreatedAt != time.Unix(123, 0).UTC() {
		t.Fatalf("created_at=%s", nodes[0].CreatedAt)
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
	_, _ = store.CreateNode(context.Background(), "t1", "A")

	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
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

func TestHandleOrgNodes_POST_BadForm(t *testing.T) {
	store := newOrgUnitMemoryStore()
	body := bytes.NewBufferString("%zz")
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_EmptyName(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", bytes.NewBufferString("name="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

type errStore struct{}

func (errStore) ListNodes(context.Context, string) ([]OrgUnitNode, error) { return nil, errBoom{} }
func (errStore) CreateNode(context.Context, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, errBoom{}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestHandleOrgNodes_GET_StoreError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/nodes", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_CreateError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", bytes.NewBufferString("name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, errStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleOrgNodes_POST_SuccessRedirect(t *testing.T) {
	store := newOrgUnitMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/org/nodes", bytes.NewBufferString("name=A"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
	rec := httptest.NewRecorder()

	handleOrgNodes(rec, req, store)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
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

func TestRenderOrgNodes(t *testing.T) {
	out := renderOrgNodes(nil, Tenant{Name: "T"}, "")
	if out == "" {
		t.Fatal("expected output")
	}
	out2 := renderOrgNodes([]OrgUnitNode{{ID: "1", Name: "N"}}, Tenant{Name: "T"}, "err")
	if out2 == "" {
		t.Fatal("expected output")
	}
}

func TestOrgUnitPGStore_ListAndCreate(t *testing.T) {
	pool := &fakeBeginner{}
	store := newOrgUnitPGStore(pool)

	nodes, err := store.ListNodes(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("nodes=%+v", nodes)
	}

	created, err := store.CreateNode(context.Background(), "t1", "A")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "n2" || created.Name != "A" {
		t.Fatalf("created=%+v", created)
	}
}

func TestOrgUnitPGStore_ListNodes_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				queryErr: errors.New("query"),
			}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows_err", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ListNodes(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestOrgUnitPGStore_CreateNode_Errors(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("created_at_scan", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"n1"}}
			tx.row2Err = errors.New("row2")
			return tx, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit", func(t *testing.T) {
		store := newOrgUnitPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"n1"}}
			tx.row2 = &stubRow{vals: []any{time.Unix(1, 0).UTC()}}
			return tx, nil
		}))
		_, err := store.CreateNode(context.Background(), "t1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNewHandlerWithOptions_BadDBURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://%zz")
	t.Setenv("ALLOWLIST_PATH", "../../config/routing/allowlist.yaml")
	t.Setenv("TENANTS_PATH", "../../config/tenants.yaml")

	_, err := NewHandlerWithOptions(HandlerOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

type beginnerFunc func(ctx context.Context) (pgx.Tx, error)

func (f beginnerFunc) Begin(ctx context.Context) (pgx.Tx, error) { return f(ctx) }

type stubTx struct {
	execErr   error
	queryErr  error
	commitErr error
	rowErr    error
	row2Err   error

	rows *stubRows
	row  pgx.Row
	row2 pgx.Row
}

func (t *stubTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *stubTx) Commit(context.Context) error              { return t.commitErr }
func (t *stubTx) Rollback(context.Context) error            { return nil }
func (t *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *stubTx) Conn() *pgx.Conn { return nil }

func (t *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

func (t *stubTx) QueryRow(context.Context, string, ...any) pgx.Row {
	if t.rowErr != nil {
		return &stubRow{err: t.rowErr}
	}
	if t.row != nil {
		r := t.row
		t.row = nil
		return r
	}
	if t.row2Err != nil {
		return &stubRow{err: t.row2Err}
	}
	if t.row2 != nil {
		return t.row2
	}
	return fakeRow{}
}

type stubRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return r.err }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *stubRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	return (&fakeRows{}).Scan(dest...)
}
func (r *stubRows) Values() ([]any, error) { return nil, nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

type stubRow struct {
	vals []any
	err  error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return fakeRow{vals: r.vals}.Scan(dest...)
}

type fakeBeginner struct {
	beginCount int
}

func (b *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	b.beginCount++
	return &fakeTx{beginCount: b.beginCount}, nil
}

type fakeTx struct {
	beginCount int
	queryRowN  int
	committed  bool
	rolled     bool
}

func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &fakeRows{idx: 0}, nil
}

func (t *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row {
	t.queryRowN++
	switch t.queryRowN {
	case 1:
		return fakeRow{vals: []any{"n2"}}
	default:
		return fakeRow{vals: []any{time.Unix(456, 0).UTC()}}
	}
}

func (t *fakeTx) Commit(context.Context) error   { t.committed = true; return nil }
func (t *fakeTx) Rollback(context.Context) error { t.rolled = true; return nil }

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return fakeBatchResults{} }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

type fakeBatchResults struct{}

func (fakeBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (fakeBatchResults) Query() (pgx.Rows, error)         { return &fakeRows{}, nil }
func (fakeBatchResults) QueryRow() pgx.Row                { return fakeRow{} }
func (fakeBatchResults) Close() error                     { return nil }

type fakeRows struct {
	idx int
}

func (r *fakeRows) Close()                        {}
func (r *fakeRows) Err() error                    { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *fakeRows) Next() bool {
	if r.idx > 0 {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRows) Scan(dest ...any) error {
	*(dest[0].(*string)) = "n1"
	*(dest[1].(*string)) = "Node"
	*(dest[2].(*time.Time)) = time.Unix(123, 0).UTC()
	return nil
}
func (r *fakeRows) Values() ([]any, error) { return nil, nil }
func (r *fakeRows) RawValues() [][]byte    { return nil }
func (r *fakeRows) Conn() *pgx.Conn        { return nil }

type fakeRow struct {
	vals []any
}

func (r fakeRow) Scan(dest ...any) error {
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.vals[i].(string)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		}
	}
	return nil
}
