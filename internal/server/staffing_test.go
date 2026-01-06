package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type positionRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *positionRows) Close()                        {}
func (r *positionRows) Err() error                    { return r.err }
func (r *positionRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *positionRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *positionRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *positionRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "pos1"
	*(dest[1].(*string)) = "org1"
	*(dest[2].(*string)) = "Name"
	*(dest[3].(*string)) = "2026-01-01"
	return nil
}
func (r *positionRows) Values() ([]any, error) { return nil, nil }
func (r *positionRows) RawValues() [][]byte    { return nil }
func (r *positionRows) Conn() *pgx.Conn        { return nil }

type assignmentRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *assignmentRows) Close()                        {}
func (r *assignmentRows) Err() error                    { return r.err }
func (r *assignmentRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *assignmentRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *assignmentRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *assignmentRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "as1"
	*(dest[1].(*string)) = "p1"
	*(dest[2].(*string)) = "pos1"
	*(dest[3].(*string)) = "active"
	*(dest[4].(*string)) = "2026-01-01"
	return nil
}
func (r *assignmentRows) Values() ([]any, error) { return nil, nil }
func (r *assignmentRows) RawValues() [][]byte    { return nil }
func (r *assignmentRows) Conn() *pgx.Conn        { return nil }

type staffingQueryTx struct {
	*stubTx
	rows pgx.Rows
}

func (t *staffingQueryTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

type staffingAssignmentQueryTx struct {
	*stubTx
	rowN int
}

func (t *staffingAssignmentQueryTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	t.rowN++
	switch t.rowN {
	case 1:
		return &stubRow{err: pgx.ErrNoRows}
	case 2:
		return &stubRow{vals: []any{"as1"}}
	case 3:
		return &stubRow{vals: []any{0}}
	case 4:
		return &stubRow{vals: []any{"evt1"}}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

type staffingAssignmentGenIDErrorTx struct {
	*stubTx
	rowN int
}

func (t *staffingAssignmentGenIDErrorTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	t.rowN++
	switch t.rowN {
	case 1:
		return &stubRow{err: pgx.ErrNoRows}
	case 2:
		return &stubRow{err: errors.New("gen")}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

type staffingAssignmentUpdateQueryTx struct {
	*stubTx
	rowN int
}

func (t *staffingAssignmentUpdateQueryTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	t.rowN++
	switch t.rowN {
	case 1:
		return &stubRow{vals: []any{"as1"}}
	case 2:
		return &stubRow{vals: []any{1}}
	case 3:
		return &stubRow{vals: []any{"evt1"}}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

type staffingAssignmentCountErrorTx struct {
	*stubTx
	rowN int
}

func (t *staffingAssignmentCountErrorTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	t.rowN++
	switch t.rowN {
	case 1:
		return &stubRow{err: pgx.ErrNoRows}
	case 2:
		return &stubRow{vals: []any{"as1"}}
	case 3:
		return &stubRow{err: errors.New("count")}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

type staffingAssignmentEventIDErrorTx struct {
	*stubTx
	rowN int
}

func (t *staffingAssignmentEventIDErrorTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	t.rowN++
	switch t.rowN {
	case 1:
		return &stubRow{err: pgx.ErrNoRows}
	case 2:
		return &stubRow{vals: []any{"as1"}}
	case 3:
		return &stubRow{vals: []any{0}}
	case 4:
		return &stubRow{err: errors.New("event")}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

func TestStaffingPGStore_ListPositionsCurrent(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{}, rows: &positionRows{scanErr: errors.New("scan")}}
			return tx, nil
		}))
		_, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{}, rows: &positionRows{err: errors.New("rows")}}
			return tx, nil
		}))
		_, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}, rows: &positionRows{}}
			return tx, nil
		}))
		_, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{}, rows: &positionRows{}}
			return tx, nil
		}))
		ps, err := store.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(ps) != 1 {
			t.Fatalf("expected 1 position, got %d", len(ps))
		}
	})
}

func TestStaffingPGStore_CreatePositionCurrent(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing org_unit_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen position id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen event id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{row2Err: errors.New("row2")}
			tx.row = &stubRow{vals: []any{"pos1"}}
			return tx, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		p, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", " A ")
		if err != nil {
			t.Fatal(err)
		}
		if p.ID != "pos1" {
			t.Fatalf("expected pos1, got %q", p.ID)
		}
	})
}

func TestStaffingPGStore_ListAssignmentsForPerson(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{}, rows: &assignmentRows{scanErr: errors.New("scan")}}
			return tx, nil
		}))
		_, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{}, rows: &assignmentRows{err: errors.New("rows")}}
			return tx, nil
		}))
		_, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}, rows: &assignmentRows{}}
			return tx, nil
		}))
		_, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingQueryTx{stubTx: &stubTx{}, rows: &assignmentRows{}}
			return tx, nil
		}))
		as, err := store.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err != nil {
			t.Fatal(err)
		}
		if len(as) != 1 {
			t.Fatalf("expected 1 assignment, got %d", len(as))
		}
	})
}

func TestStaffingPGStore_UpsertPrimaryAssignmentForPerson(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing person_uuid", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing position_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("existing id query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen assignment id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentGenIDErrorTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("count error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentCountErrorTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentEventIDErrorTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{execErr: errors.New("exec"), execErrAt: 2}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (create)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		a, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err != nil {
			t.Fatal(err)
		}
		if a.AssignmentID != "as1" {
			t.Fatalf("expected as1, got %q", a.AssignmentID)
		}
	})

	t.Run("ok (update)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentUpdateQueryTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1")
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestStaffingMemoryStore(t *testing.T) {
	s := newStaffingMemoryStore()

	t.Run("create position invalid", func(t *testing.T) {
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "", "org1", "A"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "", "A"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create position ok", func(t *testing.T) {
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "A"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("list positions ok", func(t *testing.T) {
		positions, err := s.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(positions) != 1 {
			t.Fatalf("expected 1, got %d", len(positions))
		}
	})

	t.Run("upsert invalid", func(t *testing.T) {
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "", "p1", "pos1"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "", "pos1"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("upsert ok", func(t *testing.T) {
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("list assignments ok", func(t *testing.T) {
		as, err := s.ListAssignmentsForPerson(context.Background(), "t1", "2026-01-01", "p1")
		if err != nil {
			t.Fatal(err)
		}
		if len(as) != 1 {
			t.Fatalf("expected 1, got %d", len(as))
		}
	})

	t.Run("list assignments empty tenant", func(t *testing.T) {
		as, err := s.ListAssignmentsForPerson(context.Background(), "t2", "2026-01-01", "p1")
		if err != nil {
			t.Fatal(err)
		}
		if as != nil {
			t.Fatalf("expected nil, got %+v", as)
		}
	})
}

type orgStoreStub struct {
	listFn func(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
}

func (s orgStoreStub) ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
	return s.listFn(ctx, tenantID, asOfDate)
}

func (orgStoreStub) CreateNodeCurrent(context.Context, string, string, string, string) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}

func (orgStoreStub) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (orgStoreStub) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (orgStoreStub) DisableNodeCurrent(context.Context, string, string, string) error { return nil }

type positionStoreStub struct {
	listFn   func(ctx context.Context, tenantID string, asOfDate string) ([]Position, error)
	createFn func(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, name string) (Position, error)
}

func (s positionStoreStub) ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]Position, error) {
	return s.listFn(ctx, tenantID, asOfDate)
}

func (s positionStoreStub) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, name string) (Position, error) {
	return s.createFn(ctx, tenantID, effectiveDate, orgUnitID, name)
}

type assignmentStoreStub struct {
	listFn   func(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error)
	upsertFn func(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string) (Assignment, error)
}

func (s assignmentStoreStub) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error) {
	return s.listFn(ctx, tenantID, asOfDate, personUUID)
}

func (s assignmentStoreStub) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string) (Assignment, error) {
	return s.upsertFn(ctx, tenantID, effectiveDate, personUUID, positionID)
}

func TestStaffingHandlers(t *testing.T) {
	t.Run("handlePositions tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions", nil)
		rec := httptest.NewRecorder()
		handlePositions(rec, req, newOrgUnitMemoryStore(), &staffingMemoryStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req, newOrgUnitMemoryStore(), &staffingMemoryStore{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions store errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) { return nil, errors.New("org") }},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
			},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions list error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, errors.New("list") },
			},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post bad form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", nil)
		req.Body = errReadCloser{}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn:   func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string) (Position, error) { return Position{}, nil },
			},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=bad&org_unit_id=org1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn:   func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string) (Position, error) { return Position{}, nil },
			},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post create error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-01&org_unit_id=org1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string) (Position, error) {
					return Position{}, errors.New("create")
				},
			},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&org_unit_id=org1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string) (Position, error) {
					return Position{ID: "pos1"}, nil
				},
			},
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post default effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("org_unit_id=org1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(_ context.Context, _ string, effectiveDate string, _ string, _ string) (Position, error) {
					if effectiveDate != "2026-01-01" {
						return Position{}, errors.New("unexpected effectiveDate")
					}
					return Position{ID: "pos1"}, nil
				},
			},
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }},
		)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions", nil)
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI get error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) { return nil, errors.New("list") },
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI get ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil },
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader("{bad"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"effective_date":"bad","org_unit_id":"org1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post create error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"effective_date":"2026-01-01","org_unit_id":"org1","name":"A"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			createFn: func(context.Context, string, string, string, string) (Position, error) {
				return Position{}, errors.New("create")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"org_unit_id":"org1","name":"A"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			createFn: func(context.Context, string, string, string, string) (Position, error) {
				return Position{ID: "pos1"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI missing person_uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI get ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			listFn: func(context.Context, string, string, string) ([]Assignment, error) {
				return []Assignment{{AssignmentID: "as1"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI get error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, errors.New("list") },
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", strings.NewReader("{bad"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"effective_date":"bad","person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post upsert error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string) (Assignment, error) {
				return Assignment{}, errors.New("upsert")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string) (Assignment, error) {
				return Assignment{AssignmentID: "as1"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments", nil)
		rec := httptest.NewRecorder()
		handleAssignments(rec, req, &staffingMemoryStore{}, &staffingMemoryStore{}, newPersonMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req, &staffingMemoryStore{}, &staffingMemoryStore{}, newPersonMemoryStore())
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments positions error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, errors.New("list") }},
			&staffingMemoryStore{},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments pernr resolve error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments?as_of=2026-01-01&pernr=BAD", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			&staffingMemoryStore{},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments get ok (no person)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments get ok (pernr resolves and lists)", func(t *testing.T) {
		pstore := newPersonMemoryStore()
		_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodGet, "/org/assignments?as_of=2026-01-01&pernr=1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) {
				return []Assignment{{AssignmentID: "as1"}}, nil
			}},
			pstore,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments get list error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, errors.New("list") }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post bad form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req.Body = errReadCloser{}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01&person_uuid=p1", strings.NewReader("effective_date=bad&person_uuid=p1&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post missing pernr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-01&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post pernr resolve error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-01&pernr=BAD&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post pernr not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-01&pernr=2&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post upsert error", func(t *testing.T) {
		pstore := newPersonMemoryStore()
		_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-01&pernr=1&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string) (Assignment, error) {
					return Assignment{}, errors.New("upsert")
				},
			},
			pstore,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post ok", func(t *testing.T) {
		pstore := newPersonMemoryStore()
		_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&pernr=1&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string) (Assignment, error) {
					return Assignment{AssignmentID: "as1"}, nil
				},
			},
			pstore,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post ok (person_uuid already resolved)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&person_uuid=p1&position_id=pos1&pernr=1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string) (Assignment, error) {
					return Assignment{AssignmentID: "as1"}, nil
				},
			},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post ok (person_uuid resolved, pernr empty)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&person_uuid=p1&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string) (Assignment, error) {
					return Assignment{AssignmentID: "as1"}, nil
				},
			},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/assignments?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req, &staffingMemoryStore{}, &staffingMemoryStore{}, newPersonMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("renderPositions empty nodes branch", func(t *testing.T) {
		_ = renderPositions(nil, nil, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "")
	})
	t.Run("renderPositions with nodes and positions", func(t *testing.T) {
		_ = renderPositions([]Position{{ID: "pos1", OrgUnitID: "org1", Name: "A", EffectiveAt: "2026-01-01"}}, []OrgUnitNode{{ID: "org1", Name: "B"}, {ID: "org2", Name: "A"}}, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "err")
	})
	t.Run("renderAssignments without person branch", func(t *testing.T) {
		_ = renderAssignments(nil, nil, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "", "", "", "")
	})
	t.Run("renderAssignments with assignments", func(t *testing.T) {
		_ = renderAssignments([]Assignment{{AssignmentID: "as1", PersonUUID: "p1", PositionID: "pos1", Status: "active", EffectiveAt: "2026-01-01"}}, []Position{{ID: "pos1", Name: "A"}}, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "p1", "1", "A", "")
	})

	t.Run("handleAssignmentsAPI invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=bad&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI tenant missing on post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"org_unit_id":"org1"}`)))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestStaffingHandlers_JSONRoundTrip(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"org_unit_id":"org1","name":"A"}`)))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()

	handlePositionsAPI(rec, req, positionStoreStub{
		createFn: func(context.Context, string, string, string, string) (Position, error) {
			return Position{ID: "pos1", OrgUnitID: "org1", Name: "A", EffectiveAt: "2026-01-01"}, nil
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var p Position
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "pos1" {
		t.Fatalf("unexpected: %+v", p)
	}
}

func TestMergeMsg(t *testing.T) {
	if mergeMsg("", "") != "" {
		t.Fatal("expected empty")
	}
	if mergeMsg("a", "") != "a" {
		t.Fatal("expected a")
	}
	if mergeMsg("", "b") != "b" {
		t.Fatal("expected b")
	}
	if mergeMsg("a", "b") != "a；b" {
		t.Fatal("expected merged")
	}
}

func TestStaffingHandlers_ParseDefaultDates(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/positions", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()

	handlePositions(rec, req,
		orgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
			return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
		}},
		positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStaffingHandlers_DefaultAsOf_InternalAPI(t *testing.T) {
	t.Run("positions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()

		handlePositionsAPI(rec, req, positionStoreStub{
			listFn: func(_ context.Context, _ string, asOf string) ([]Position, error) {
				if _, err := time.Parse("2006-01-02", asOf); err != nil {
					return nil, err
				}
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("assignments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()

		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			listFn: func(_ context.Context, _ string, asOf string, _ string) ([]Assignment, error) {
				if _, err := time.Parse("2006-01-02", asOf); err != nil {
					return nil, err
				}
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestStaffingHandlers_DefaultAsOf_UI(t *testing.T) {
	pstore := newPersonMemoryStore()
	_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

	req := httptest.NewRequest(http.MethodGet, "/org/assignments?pernr=1", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
	rec := httptest.NewRecorder()

	handleAssignments(rec, req,
		positionStoreStub{
			listFn: func(_ context.Context, _ string, asOf string) ([]Position, error) {
				if _, err := time.Parse("2006-01-02", asOf); err != nil {
					return nil, err
				}
				return []Position{}, nil
			},
		},
		assignmentStoreStub{
			listFn: func(_ context.Context, _ string, _ string, _ string) ([]Assignment, error) {
				return []Assignment{}, nil
			},
		},
		pstore,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}
