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
	*(dest[2].(*string)) = ""
	*(dest[3].(*string)) = ""
	*(dest[4].(*string)) = ""
	*(dest[5].(*string)) = ""
	*(dest[6].(*string)) = ""
	*(dest[7].(*string)) = "Name"
	*(dest[8].(*string)) = "active"
	*(dest[9].(*string)) = "1.0"
	*(dest[10].(*string)) = "2026-01-01"
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "", "org1", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing org_unit_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing job_profile_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen position id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A")
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A")
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("snapshot error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{row3Err: errors.New("row3")}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			tx.row3 = &stubRow{vals: []any{"pos1", "org1", "", "", "", "jp1", "", "A", "active", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			tx.row3 = &stubRow{vals: []any{"pos1", "org1", "", "S2601", "2026-01-01", "00000000-0000-0000-0000-000000000001", "JP1", "A", "active", "2.50", "2026-01-01"}}
			return tx, nil
		}))
		p, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "00000000-0000-0000-0000-000000000001", " 2.50 ", " A ")
		if err != nil {
			t.Fatal(err)
		}
		if p.ID != "pos1" {
			t.Fatalf("expected pos1, got %q", p.ID)
		}
		if p.OrgUnitID != "org1" {
			t.Fatalf("expected org1, got %q", p.OrgUnitID)
		}
		if p.JobProfileID == "" {
			t.Fatal("expected job_profile_id")
		}
		if p.JobCatalogSetID != "S2601" {
			t.Fatalf("expected jobcatalog_setid=S2601, got %q", p.JobCatalogSetID)
		}
		if p.CapacityFTE != "2.50" {
			t.Fatalf("expected capacity_fte=2.50, got %q", p.CapacityFTE)
		}
	})

	t.Run("ok (default capacity)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			tx.row3 = &stubRow{vals: []any{"pos1", "org1", "", "S2601", "2026-01-01", "jp1", "", "A", "active", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		p, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A")
		if err != nil {
			t.Fatal(err)
		}
		if p.CapacityFTE != "1.0" {
			t.Fatalf("expected capacity_fte=1.0, got %q", p.CapacityFTE)
		}
	})
}

func TestStaffingPGStore_UpdatePositionCurrent(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing position_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing patch", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "", "", "", "", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen event id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{execErr: errors.New("exec"), execErrAt: 2}
			tx.row = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("select error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{row2Err: errors.New("scan")}
			tx.row = &stubRow{vals: []any{"evt1"}}
			return tx, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"evt1"}}
			tx.row2 = &stubRow{vals: []any{"pos1", "org1", "", "", "", "", "", "Name", "disabled", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "org1", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"evt1"}}
			tx.row2 = &stubRow{vals: []any{"pos1", "org1", "mgr1", "", "", "", "", "Name", "disabled", "1.25", "2026-01-01"}}
			return tx, nil
		}))
		p, err := store.UpdatePositionCurrent(context.Background(), "t1", " pos1 ", "2026-01-01", " org1 ", " mgr1 ", "", " 1.25 ", " Name ", " disabled ")
		if err != nil {
			t.Fatal(err)
		}
		if p.ID != "pos1" {
			t.Fatalf("expected pos1, got %q", p.ID)
		}
		if p.LifecycleStatus != "disabled" {
			t.Fatalf("expected lifecycle_status=disabled, got %q", p.LifecycleStatus)
		}
		if p.CapacityFTE != "1.25" {
			t.Fatalf("expected capacity_fte=1.25, got %q", p.CapacityFTE)
		}
	})

	t.Run("ok (job_profile_id patch)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"evt1"}}
			tx.row2 = &stubRow{vals: []any{"pos1", "org1", "mgr1", "", "", "", "", "Name", "disabled", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		if _, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "", "", "00000000-0000-0000-0000-000000000002", "", "", ""); err != nil {
			t.Fatal(err)
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
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing person_uuid", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing position_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("existing id query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen assignment id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentGenIDErrorTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("count error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentCountErrorTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentEventIDErrorTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{execErr: errors.New("exec"), execErrAt: 2}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}}
			return tx, nil
		}))
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (create)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		a, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
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
		_, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ok (with fte)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		a, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "", "0.5")
		if err != nil {
			t.Fatal(err)
		}
		if a.AssignmentID != "as1" {
			t.Fatalf("expected as1, got %q", a.AssignmentID)
		}
	})

	t.Run("ok (with status)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &staffingAssignmentQueryTx{stubTx: &stubTx{}}
			return tx, nil
		}))
		a, err := store.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "inactive", "")
		if err != nil {
			t.Fatal(err)
		}
		if a.Status != "inactive" {
			t.Fatalf("expected inactive, got %q", a.Status)
		}
	})
}

func TestStaffingMemoryStore(t *testing.T) {
	s := newStaffingMemoryStore()

	t.Run("create position invalid", func(t *testing.T) {
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "", "org1", "", "", "A"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "", "jp1", "", "A"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "", "", "A"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create position ok", func(t *testing.T) {
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "org1", "jp1", "", "A"); err != nil {
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

	t.Run("update position invalid", func(t *testing.T) {
		if _, err := s.UpdatePositionCurrent(context.Background(), "t1", "pos1", "", "", "", "", "", "", ""); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.UpdatePositionCurrent(context.Background(), "t1", "", "2026-01-01", "", "", "", "", "", ""); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "", "", "", "", "", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update position not found", func(t *testing.T) {
		if _, err := s.UpdatePositionCurrent(context.Background(), "t1", "missing", "2026-01-01", "", "", "", "", "A", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update position ok", func(t *testing.T) {
		positions, err := s.ListPositionsCurrent(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(positions) != 1 {
			t.Fatalf("expected 1, got %d", len(positions))
		}

		updated, err := s.UpdatePositionCurrent(context.Background(), "t1", positions[0].ID, "2026-02-01", "org2", "mgr1", "jp1", "2.5", "B", "disabled")
		if err != nil {
			t.Fatal(err)
		}
		if updated.OrgUnitID != "org2" || updated.ReportsToID != "mgr1" || updated.JobProfileID != "jp1" || updated.Name != "B" || updated.LifecycleStatus != "disabled" || updated.EffectiveAt != "2026-02-01" {
			t.Fatalf("unexpected updated position: %+v", updated)
		}
	})

	t.Run("upsert invalid", func(t *testing.T) {
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "", "p1", "pos1", "", ""); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "", "pos1", "", ""); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "", "", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("upsert ok", func(t *testing.T) {
		a, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-01", "p1", "pos1", "inactive", "")
		if err != nil {
			t.Fatal(err)
		}
		if a.Status != "inactive" {
			t.Fatalf("expected inactive, got %q", a.Status)
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

type staffingOrgStoreStub struct {
	listFn    func(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
	resolveFn func(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error)
}

func (s staffingOrgStoreStub) ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
	return s.listFn(ctx, tenantID, asOfDate)
}

func (staffingOrgStoreStub) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}

func (staffingOrgStoreStub) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (staffingOrgStoreStub) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}
func (staffingOrgStoreStub) DisableNodeCurrent(context.Context, string, string, string) error {
	return nil
}
func (staffingOrgStoreStub) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return nil
}

func (s staffingOrgStoreStub) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	if s.resolveFn != nil {
		return s.resolveFn(ctx, tenantID, orgUnitID, asOfDate)
	}
	if strings.TrimSpace(orgUnitID) == "" {
		return "", errors.New("org_unit_id is required")
	}
	return "S2601", nil
}

type jobStoreErrStub struct{ JobCatalogStore }

func (jobStoreErrStub) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, errors.New("profiles fail")
}

type positionStoreStub struct {
	listFn   func(ctx context.Context, tenantID string, asOfDate string) ([]Position, error)
	createFn func(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileID string, capacityFTE string, name string) (Position, error)
	updateFn func(ctx context.Context, tenantID string, positionID string, effectiveDate string, orgUnitID string, reportsToPositionID string, jobProfileID string, capacityFTE string, name string, lifecycleStatus string) (Position, error)
}

func (s positionStoreStub) ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]Position, error) {
	return s.listFn(ctx, tenantID, asOfDate)
}

func (s positionStoreStub) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileID string, capacityFTE string, name string) (Position, error) {
	return s.createFn(ctx, tenantID, effectiveDate, orgUnitID, jobProfileID, capacityFTE, name)
}

func (s positionStoreStub) UpdatePositionCurrent(ctx context.Context, tenantID string, positionID string, effectiveDate string, orgUnitID string, reportsToPositionID string, jobProfileID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
	return s.updateFn(ctx, tenantID, positionID, effectiveDate, orgUnitID, reportsToPositionID, jobProfileID, capacityFTE, name, lifecycleStatus)
}

type assignmentStoreStub struct {
	listFn    func(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error)
	upsertFn  func(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, allocatedFte string) (Assignment, error)
	correctFn func(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error)
	rescindFn func(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error)
}

func (s assignmentStoreStub) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error) {
	return s.listFn(ctx, tenantID, asOfDate, personUUID)
}

func (s assignmentStoreStub) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionID string, status string, allocatedFte string) (Assignment, error) {
	return s.upsertFn(ctx, tenantID, effectiveDate, personUUID, positionID, status, allocatedFte)
}

func (s assignmentStoreStub) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	if s.correctFn == nil {
		return "", errors.New("not implemented")
	}
	return s.correctFn(ctx, tenantID, assignmentID, targetEffectiveDate, replacementPayload)
}

func (s assignmentStoreStub) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	if s.rescindFn == nil {
		return "", errors.New("not implemented")
	}
	return s.rescindFn(ctx, tenantID, assignmentID, targetEffectiveDate, payload)
}

func TestStaffingHandlers(t *testing.T) {
	t.Run("handlePositions tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions", nil)
		rec := httptest.NewRecorder()
		handlePositions(rec, req, newOrgUnitMemoryStore(), &staffingMemoryStore{}, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req, newOrgUnitMemoryStore(), &staffingMemoryStore{}, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions store errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) { return nil, errors.New("org") }},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
			},
			nil,
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
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, errors.New("list") },
			},
			nil,
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
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
					return Position{}, nil
				},
			},
			nil,
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
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
					return Position{}, nil
				},
			},
			nil,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post create error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-01&org_unit_id=org1&job_profile_id=jp1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
					return Position{}, errors.New("create")
				},
			},
			nil,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&org_unit_id=org1&job_profile_id=jp1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
					return Position{ID: "pos1"}, nil
				},
			},
			nil,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post ok (preserve org_unit_id)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&org_unit_id=org1&job_profile_id=1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(_ context.Context, _ string, _ string, orgUnitID string, jobProfileID string, _ string, _ string) (Position, error) {
					if orgUnitID != "org1" || jobProfileID != "1" {
						return Position{}, errors.New("unexpected job profile fields")
					}
					return Position{ID: "pos1"}, nil
				},
			},
			nil,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if !strings.Contains(loc, "org_unit_id=org1") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("handlePositions with job store ok", func(t *testing.T) {
		jobStore := newJobCatalogMemoryStore().(*jobcatalogMemoryStore)
		_ = jobStore.CreateJobProfile(context.Background(), "t1", "S2601", "2026-01-01", "JP1", "Job Profile 1", "", nil, "")

		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01&org_unit_id=org1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{ID: "pos1", Name: "A"}}, nil
			}},
			jobStore,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions with resolve error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01&org_unit_id=org1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{
				listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
					return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
				},
				resolveFn: func(context.Context, string, string, string) (string, error) {
					return "", errors.New("resolve")
				},
			},
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }},
			newJobCatalogMemoryStore(),
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions with job store errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01&org_unit_id=org1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }},
			jobStoreErrStub{JobCatalogStore: newJobCatalogMemoryStore()},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post update ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&position_id=pos1&lifecycle_status=disabled"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) {
					return []Position{{ID: "pos1", LifecycleStatus: "active"}}, nil
				},
				updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
					return Position{ID: "pos1", LifecycleStatus: "disabled"}, nil
				},
				createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
					return Position{}, errors.New("unexpected create")
				},
			},
			nil,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post update error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&position_id=pos1&lifecycle_status=disabled"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) {
					return []Position{{ID: "pos1", LifecycleStatus: "active"}}, nil
				},
				updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
					return Position{}, errors.New("update")
				},
				createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
					return Position{}, errors.New("unexpected create")
				},
			},
			nil,
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositions post default effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("org_unit_id=org1&job_profile_id=jp1&name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePositions(rec, req,
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{
				listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
				createFn: func(_ context.Context, _ string, effectiveDate string, _ string, _ string, _ string, _ string) (Position, error) {
					if effectiveDate != "2026-01-01" {
						return Position{}, errors.New("unexpected effectiveDate")
					}
					return Position{ID: "pos1"}, nil
				},
			},
			nil,
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
			staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
				return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
			}},
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }},
			nil,
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

	t.Run("handlePositionsAPI get stable error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return nil, &pgconn.PgError{Message: "STAFFING_INVALID_ARGUMENT"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
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
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"effective_date":"2026-01-01","org_unit_id":"org1","job_profile_id":"jp1","name":"A"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, errors.New("create")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"org_unit_id":"org1","job_profile_id":"jp1","name":"A"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{ID: "pos1"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post update ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"position_id":"pos1","lifecycle_status":"disabled"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{ID: "pos1", LifecycleStatus: "disabled"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post error conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"org_unit_id":"org1","job_profile_id":"jp1","name":"A"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post error unprocessable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"position_id":"pos1","lifecycle_status":"disabled"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Message: "STAFFING_POSITION_DISABLED_AS_OF"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post error bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"position_id":"pos1","lifecycle_status":"disabled"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePositionsAPI post error invalid input", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"position_id":"pos1","lifecycle_status":"disabled"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, positionStoreStub{
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		if rec.Code != http.StatusBadRequest {
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

	t.Run("handleAssignmentsAPI get stable error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01&person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			listFn: func(context.Context, string, string, string) ([]Assignment, error) {
				return nil, &pgconn.PgError{Message: "STAFFING_INVALID_ARGUMENT"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
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

	t.Run("handleAssignmentsAPI post invalid status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1","status":"bad"}`)))
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
			upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
				return Assignment{}, errors.New("upsert")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
				return Assignment{AssignmentID: "as1"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post error conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
				return Assignment{}, &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post error unprocessable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
				return Assignment{}, &pgconn.PgError{Message: "STAFFING_POSITION_DISABLED_AS_OF"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post error bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
				return Assignment{}, newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentsAPI post error invalid input", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", bytes.NewReader([]byte(`{"person_uuid":"p1","position_id":"pos1"}`)))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{
			upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
				return Assignment{}, &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
		})
		if rec.Code != http.StatusBadRequest {
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

	t.Run("handleAssignmentEventsCorrectAPI tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader("{bad"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI invalid target date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"bad","replacement_payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, &staffingMemoryStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsCorrectAPI ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:correct", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","replacement_payload":{"position_id":"p1"}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsCorrectAPI(rec, req, assignmentStoreStub{
			correctFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "e1", nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI error conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignmentEventsRescindAPI error unprocessable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/assignment-events:rescind", strings.NewReader(`{"assignment_id":"a1","target_effective_date":"2026-01-01","payload":{}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentEventsRescindAPI(rec, req, assignmentStoreStub{
			rescindFn: func(context.Context, string, string, string, json.RawMessage) (string, error) {
				return "", &pgconn.PgError{Message: "STAFFING_ASSIGNMENT_EVENT_NOT_FOUND"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
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
		if rec.Code != http.StatusBadRequest {
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

	t.Run("handleAssignments get ok (as_of defaults)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/assignments", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/org/assignments?as_of=") {
			t.Fatalf("location=%q", loc)
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

	t.Run("handleAssignments post invalid status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01&person_uuid=p1", strings.NewReader("effective_date=2026-01-01&person_uuid=p1&position_id=pos1&status=bad"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{}, nil }},
			assignmentStoreStub{listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil }},
			newPersonMemoryStore(),
		)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "status 无效") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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
				upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
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
				upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
					return Assignment{AssignmentID: "as1"}, nil
				},
			},
			pstore,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post status=inactive ok", func(t *testing.T) {
		pstore := newPersonMemoryStore()
		_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&pernr=1&position_id=pos1&status=inactive"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(_ context.Context, _ string, _ string, _ string, _ string, status string, _ string) (Assignment, error) {
					if status != "inactive" {
						return Assignment{}, errors.New("expected status=inactive")
					}
					return Assignment{AssignmentID: "as1"}, nil
				},
			},
			pstore,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("handleAssignments post ok (effective_date defaults)", func(t *testing.T) {
		pstore := newPersonMemoryStore()
		_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("pernr=1&position_id=pos1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
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
		pstore := newPersonMemoryStore()
		p, _ := pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&person_uuid="+p.UUID+"&position_id=pos1&pernr=1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
					return Assignment{AssignmentID: "as1"}, nil
				},
			},
			pstore,
		)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handleAssignments post rejects pernr/person_uuid mismatch", func(t *testing.T) {
		pstore := newPersonMemoryStore()
		_, _ = pstore.CreatePerson(context.Background(), "t1", "1", "A")

		req := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&person_uuid=bad&position_id=pos1&pernr=1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handleAssignments(rec, req,
			positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return []Position{{ID: "pos1"}}, nil }},
			assignmentStoreStub{
				listFn: func(context.Context, string, string, string) ([]Assignment, error) { return []Assignment{}, nil },
				upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
					return Assignment{}, errors.New("unexpected upsert")
				},
			},
			pstore,
		)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "pernr/person_uuid 不一致") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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
				upsertFn: func(context.Context, string, string, string, string, string, string) (Assignment, error) {
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
		_ = renderPositions(nil, nil, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "", "", nil, "")
	})
	t.Run("renderPositions with nodes and positions", func(t *testing.T) {
		_ = renderPositions([]Position{{ID: "pos1", OrgUnitID: "org1", Name: "A", EffectiveAt: "2026-01-01"}}, []OrgUnitNode{{ID: "org1", Name: "B"}, {ID: "org2", Name: "A"}}, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "", "", nil, "err")
	})
	t.Run("renderPositions org unit missing in nodes", func(t *testing.T) {
		out := renderPositions(nil, []OrgUnitNode{{ID: "org1", Name: "Org"}}, Tenant{ID: "t1", Name: "T"}, "2026-01-01", "missing", "", nil, "")
		if !strings.Contains(out, "missing") {
			t.Fatalf("unexpected output: %s", out)
		}
	})
	t.Run("renderPositions job catalog context", func(t *testing.T) {
		out := renderPositions(
			[]Position{
				{ID: "pos1", OrgUnitID: "org1", Name: "A", EffectiveAt: "2026-01-01", LifecycleStatus: "active", JobCatalogSetID: "SHARE", JobProfileID: "1", JobProfileCode: "JP1"},
				{ID: "pos2", OrgUnitID: "org2", Name: "", EffectiveAt: "2026-01-02", LifecycleStatus: "disabled", JobCatalogSetID: "", JobProfileID: "2", JobProfileCode: ""},
			},
			[]OrgUnitNode{{ID: "org1", Name: "Org", IsBusinessUnit: true}, {ID: "org2", Name: "Org2"}},
			Tenant{ID: "t1", Name: "T"},
			"2026-01-01",
			"org1",
			"SHARE",
			[]JobProfile{{ID: "1", Code: "JP1"}, {ID: "2", Code: "JP2"}},
			"err",
		)
		if !strings.Contains(out, "SetID") {
			t.Fatalf("unexpected output: %s", out)
		}
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
	req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"org_unit_id":"org1","job_profile_id":"jp1","name":"A"}`)))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()

	handlePositionsAPI(rec, req, positionStoreStub{
		createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
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
		staffingOrgStoreStub{listFn: func(context.Context, string, string) ([]OrgUnitNode, error) {
			return []OrgUnitNode{{ID: "org1", Name: "Org"}}, nil
		}},
		positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }},
		nil,
	)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/org/positions?as_of=") {
		t.Fatalf("location=%q", loc)
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

func TestFilterActivePositions(t *testing.T) {
	got := filterActivePositions([]Position{
		{ID: "a", LifecycleStatus: "active"},
		{ID: "b", LifecycleStatus: "disabled"},
		{ID: "c", LifecycleStatus: ""},
	})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("unexpected result: %+v", got)
	}
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
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/org/assignments?as_of=") || !strings.Contains(loc, "&pernr=1") {
		t.Fatalf("location=%q", loc)
	}
}
