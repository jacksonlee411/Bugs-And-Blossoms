package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type snapshotRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *snapshotRows) Close()                        {}
func (r *snapshotRows) Err() error                    { return r.err }
func (r *snapshotRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *snapshotRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *snapshotRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *snapshotRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "org1"
	*(dest[1].(*string)) = "parent1"
	*(dest[2].(*string)) = "Name"
	*(dest[3].(*string)) = "Root / Name"
	*(dest[4].(*int)) = 1
	*(dest[5].(*string)) = "mgr1"
	*(dest[6].(*string)) = "path1"
	return nil
}
func (r *snapshotRows) Values() ([]any, error) { return nil, nil }
func (r *snapshotRows) RawValues() [][]byte    { return nil }
func (r *snapshotRows) Conn() *pgx.Conn        { return nil }

type snapshotQueryTx struct {
	*stubTx
	rows pgx.Rows
}

func (t *snapshotQueryTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

func TestOrgUnitSnapshotPGStore_GetSnapshot(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &snapshotQueryTx{stubTx: &stubTx{}, rows: &snapshotRows{}}
			return tx, nil
		}))
		rows, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &snapshotQueryTx{stubTx: &stubTx{}, rows: &snapshotRows{scanErr: errors.New("scan")}}
			return tx, nil
		}))
		_, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &snapshotQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}, rows: &snapshotRows{}}
			return tx, nil
		}))
		_, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &snapshotQueryTx{stubTx: &stubTx{}, rows: &snapshotRows{err: errors.New("rows")}}
			return tx, nil
		}))
		_, err := store.GetSnapshot(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

type execPlanTx struct {
	*stubTx
	execN   int
	execErr error
}

func (t *execPlanTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.execN++
	if t.execN == 2 {
		return pgconn.CommandTag{}, t.execErr
	}
	return t.stubTx.Exec(ctx, sql, args...)
}

func TestOrgUnitSnapshotPGStore_CreateOrgUnit(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (root)", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"org1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		id, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "Root", "")
		if err != nil {
			t.Fatal(err)
		}
		if id != "org1" {
			t.Fatalf("expected org1, got %q", id)
		}
	})

	t.Run("ok (child payload branch)", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"org1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "Child", "parent1")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen org id error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen event id error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{row2Err: errors.New("row2")}
			tx.row = &stubRow{vals: []any{"org1"}}
			return tx, nil
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &execPlanTx{
				stubTx:  &stubTx{},
				execErr: errors.New("submit"),
				execN:   0,
			}
			tx.row = &stubRow{vals: []any{"org1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newOrgUnitSnapshotPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"org1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.CreateOrgUnit(context.Background(), "t1", "2026-01-01", "A", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

type stubOrgUnitSnapshotStore struct {
	snapshot    []OrgUnitSnapshotRow
	snapshotErr error
	createID    string
	createErr   error
}

func (s *stubOrgUnitSnapshotStore) GetSnapshot(context.Context, string, string) ([]OrgUnitSnapshotRow, error) {
	if s.snapshotErr != nil {
		return nil, s.snapshotErr
	}
	return s.snapshot, nil
}

func (s *stubOrgUnitSnapshotStore) CreateOrgUnit(context.Context, string, string, string, string) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	return s.createID, nil
}

func TestHandleOrgSnapshot(t *testing.T) {
	tenant := Tenant{ID: "00000000-0000-0000-0000-000000000001", Name: "T1"}

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/snapshot", nil)
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/snapshot?as_of=2026-01-01&created_id=x", nil)
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "store not configured") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("get ok (rows empty branch)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/snapshot?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{snapshot: nil})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get ok (rows)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/snapshot?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{
			snapshot: []OrgUnitSnapshotRow{{OrgID: "o1", Name: "A", FullNamePath: "A", NodePath: "x"}},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "OrgUnit Snapshot") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("get error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/snapshot?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{snapshotErr: errors.New("boom")})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post bad form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/snapshot?as_of=2026-01-01", strings.NewReader("%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post missing name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/snapshot?as_of=2026-01-01", strings.NewReader("name="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "name is required") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post create error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/snapshot?as_of=2026-01-01", bytes.NewBufferString("name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{createErr: errors.New("create")})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "create") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/snapshot?as_of=2026-01-01", bytes.NewBufferString("name=A&parent_id=p1&effective_date=2026-01-02"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{createID: "new1"})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if !strings.Contains(loc, "created_id=new1") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/snapshot", nil)
		req = req.WithContext(withTenant(req.Context(), tenant))
		rec := httptest.NewRecorder()
		handleOrgSnapshot(rec, req, &stubOrgUnitSnapshotStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
