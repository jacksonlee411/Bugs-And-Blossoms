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
	*(dest[1].(*string)) = "10000001"
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "", "10000001", "", "", "A")
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

	t.Run("invalid org_unit_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "bad", "jp1", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing job_profile_uuid", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen position id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
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
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			tx.row3 = &stubRow{vals: []any{"pos1", "10000001", "", "", "", "jp1", "", "A", "active", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		_, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"pos1"}}
			tx.row2 = &stubRow{vals: []any{"evt1"}}
			tx.row3 = &stubRow{vals: []any{"pos1", "10000001", "", "S2601", "2026-01-01", "00000000-0000-0000-0000-000000000001", "JP1", "A", "active", "2.50", "2026-01-01"}}
			return tx, nil
		}))
		p, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "00000000-0000-0000-0000-000000000001", " 2.50 ", " A ")
		if err != nil {
			t.Fatal(err)
		}
		if p.PositionUUID != "pos1" {
			t.Fatalf("expected pos1, got %q", p.PositionUUID)
		}
		if p.OrgUnitID != "10000001" {
			t.Fatalf("expected 10000001, got %q", p.OrgUnitID)
		}
		if p.JobProfileUUID == "" {
			t.Fatal("expected job_profile_uuid")
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
			tx.row3 = &stubRow{vals: []any{"pos1", "10000001", "", "S2601", "2026-01-01", "jp1", "", "A", "active", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		p, err := store.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A")
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
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "10000001", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "10000001", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing effective_date", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "", "10000001", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing position_uuid", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "", "2026-01-01", "10000001", "", "", "", "A", "disabled")
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

	t.Run("invalid org_unit_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "bad", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen event id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "10000001", "", "", "", "A", "disabled")
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
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "10000001", "", "", "", "A", "disabled")
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
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "10000001", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"evt1"}}
			tx.row2 = &stubRow{vals: []any{"pos1", "10000001", "", "", "", "", "", "Name", "disabled", "1.0", "2026-01-01"}}
			return tx, nil
		}))
		_, err := store.UpdatePositionCurrent(context.Background(), "t1", "pos1", "2026-01-01", "10000001", "", "", "", "A", "disabled")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"evt1"}}
			tx.row2 = &stubRow{vals: []any{"pos1", "10000001", "mgr1", "", "", "", "", "Name", "disabled", "1.25", "2026-01-01"}}
			return tx, nil
		}))
		p, err := store.UpdatePositionCurrent(context.Background(), "t1", " pos1 ", "2026-01-01", " 10000001 ", " mgr1 ", "", " 1.25 ", " Name ", " disabled ")
		if err != nil {
			t.Fatal(err)
		}
		if p.PositionUUID != "pos1" {
			t.Fatalf("expected pos1, got %q", p.PositionUUID)
		}
		if p.LifecycleStatus != "disabled" {
			t.Fatalf("expected lifecycle_status=disabled, got %q", p.LifecycleStatus)
		}
		if p.CapacityFTE != "1.25" {
			t.Fatalf("expected capacity_fte=1.25, got %q", p.CapacityFTE)
		}
	})

	t.Run("ok (job_profile_uuid patch)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"evt1"}}
			tx.row2 = &stubRow{vals: []any{"pos1", "10000001", "mgr1", "", "", "", "", "Name", "disabled", "1.0", "2026-01-01"}}
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

	t.Run("missing position_uuid", func(t *testing.T) {
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
		if a.AssignmentUUID != "as1" {
			t.Fatalf("expected as1, got %q", a.AssignmentUUID)
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
		if a.AssignmentUUID != "as1" {
			t.Fatalf("expected as1, got %q", a.AssignmentUUID)
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
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "", "10000001", "", "", "A"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "", "jp1", "", "A"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "", "", "A"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create position ok", func(t *testing.T) {
		if _, err := s.CreatePositionCurrent(context.Background(), "t1", "2026-01-01", "10000001", "jp1", "", "A"); err != nil {
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

		updated, err := s.UpdatePositionCurrent(context.Background(), "t1", positions[0].PositionUUID, "2026-02-01", "10000002", "mgr1", "jp1", "2.5", "B", "disabled")
		if err != nil {
			t.Fatal(err)
		}
		if updated.OrgUnitID != "10000002" || updated.ReportsToPositionUUID != "mgr1" || updated.JobProfileUUID != "jp1" || updated.Name != "B" || updated.LifecycleStatus != "disabled" || updated.EffectiveAt != "2026-02-01" {
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

	t.Run("upsert default status active", func(t *testing.T) {
		a, err := s.UpsertPrimaryAssignmentForPerson(context.Background(), "t1", "2026-01-02", "p2", "pos2", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if a.Status != "active" {
			t.Fatalf("expected active, got %q", a.Status)
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
	listFn            func(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
	resolveFn         func(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error)
	resolveOrgIDFn    func(ctx context.Context, tenantID string, orgCode string) (int, error)
	resolveOrgCodeFn  func(ctx context.Context, tenantID string, orgID int) (string, error)
	resolveOrgCodesFn func(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error)
}

func (s staffingOrgStoreStub) ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
	return s.listFn(ctx, tenantID, asOfDate)
}

func (staffingOrgStoreStub) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
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
func (s staffingOrgStoreStub) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	if s.resolveOrgIDFn != nil {
		return s.resolveOrgIDFn(ctx, tenantID, orgCode)
	}
	return 10000001, nil
}
func (s staffingOrgStoreStub) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	if s.resolveOrgCodeFn != nil {
		return s.resolveOrgCodeFn(ctx, tenantID, orgID)
	}
	return "ORG-1", nil
}

func (s staffingOrgStoreStub) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	if s.resolveOrgCodesFn != nil {
		return s.resolveOrgCodesFn(ctx, tenantID, orgIDs)
	}
	out := make(map[int]string, len(orgIDs))
	for _, orgID := range orgIDs {
		code, err := s.ResolveOrgCode(ctx, tenantID, orgID)
		if err != nil {
			return nil, err
		}
		out[orgID] = code
	}
	return out, nil
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

func (staffingOrgStoreStub) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	return []OrgUnitChild{}, nil
}

func (staffingOrgStoreStub) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	return OrgUnitNodeDetails{}, nil
}

func (staffingOrgStoreStub) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	return OrgUnitSearchResult{}, nil
}
func (staffingOrgStoreStub) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	return []OrgUnitSearchCandidate{}, nil
}
func (staffingOrgStoreStub) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	return []OrgUnitNodeVersion{}, nil
}
func (staffingOrgStoreStub) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}
func (staffingOrgStoreStub) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, nil
}

type jobStoreErrStub struct{ JobCatalogStore }

func (jobStoreErrStub) ListJobProfiles(context.Context, string, string, string) ([]JobProfile, error) {
	return nil, errors.New("profiles fail")
}

type positionStoreStub struct {
	listFn   func(ctx context.Context, tenantID string, asOfDate string) ([]Position, error)
	createFn func(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileUUID string, capacityFTE string, name string) (Position, error)
	updateFn func(ctx context.Context, tenantID string, positionUUID string, effectiveDate string, orgUnitID string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error)
}

func (s positionStoreStub) ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]Position, error) {
	return s.listFn(ctx, tenantID, asOfDate)
}

func (s positionStoreStub) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgUnitID string, jobProfileUUID string, capacityFTE string, name string) (Position, error) {
	return s.createFn(ctx, tenantID, effectiveDate, orgUnitID, jobProfileUUID, capacityFTE, name)
}

func (s positionStoreStub) UpdatePositionCurrent(ctx context.Context, tenantID string, positionUUID string, effectiveDate string, orgUnitID string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (Position, error) {
	return s.updateFn(ctx, tenantID, positionUUID, effectiveDate, orgUnitID, reportsToPositionUUID, jobProfileUUID, capacityFTE, name, lifecycleStatus)
}

type assignmentStoreStub struct {
	listFn    func(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error)
	upsertFn  func(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (Assignment, error)
	correctFn func(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error)
	rescindFn func(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error)
}

func (s assignmentStoreStub) ListAssignmentsForPerson(ctx context.Context, tenantID string, asOfDate string, personUUID string) ([]Assignment, error) {
	return s.listFn(ctx, tenantID, asOfDate, personUUID)
}

func (s assignmentStoreStub) UpsertPrimaryAssignmentForPerson(ctx context.Context, tenantID string, effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (Assignment, error) {
	return s.upsertFn(ctx, tenantID, effectiveDate, personUUID, positionUUID, status, allocatedFte)
}

func (s assignmentStoreStub) CorrectAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (string, error) {
	if s.correctFn == nil {
		return "", errors.New("not implemented")
	}
	return s.correctFn(ctx, tenantID, assignmentUUID, targetEffectiveDate, replacementPayload)
}

func (s assignmentStoreStub) RescindAssignmentEvent(ctx context.Context, tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (string, error) {
	if s.rescindFn == nil {
		return "", errors.New("not implemented")
	}
	return s.rescindFn(ctx, tenantID, assignmentUUID, targetEffectiveDate, payload)
}

func TestStaffingHandlers_JSONRoundTrip(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", bytes.NewReader([]byte(`{"effective_date":"2026-01-01","org_code":"ORG-1","job_profile_uuid":"jp1","name":"A"}`)))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()

	handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
		createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
			return Position{PositionUUID: "pos1", OrgUnitID: "10000001", Name: "A", EffectiveAt: "2026-01-01"}, nil
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var p staffingPositionAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.PositionUUID != "pos1" {
		t.Fatalf("unexpected: %+v", p)
	}
	if p.OrgCode != "ORG-1" {
		t.Fatalf("unexpected org_code: %+v", p)
	}
}

func TestStaffingHandlers_AsOfRequired_InternalAPI(t *testing.T) {
	t.Run("positions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("assignments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/assignments?person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleAssignmentsAPI(rec, req, assignmentStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
