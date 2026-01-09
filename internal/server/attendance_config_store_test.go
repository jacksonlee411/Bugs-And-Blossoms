package server

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fixedRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fixedRows) Close() {}
func (r *fixedRows) Err() error {
	return r.err
}
func (r *fixedRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *fixedRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *fixedRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *fixedRows) Scan(dest ...any) error {
	row := fakeRow{vals: r.rows[r.idx-1]}
	return row.Scan(dest...)
}
func (r *fixedRows) Values() ([]any, error) { return nil, nil }
func (r *fixedRows) RawValues() [][]byte    { return nil }
func (r *fixedRows) Conn() *pgx.Conn        { return nil }

type scanErrorRows struct {
	called bool
}

func (r *scanErrorRows) Close() {}
func (r *scanErrorRows) Err() error {
	return nil
}
func (r *scanErrorRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *scanErrorRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *scanErrorRows) Next() bool {
	if r.called {
		return false
	}
	r.called = true
	return true
}
func (r *scanErrorRows) Scan(...any) error      { return errors.New("scan err") }
func (r *scanErrorRows) Values() ([]any, error) { return nil, nil }
func (r *scanErrorRows) RawValues() [][]byte    { return nil }
func (r *scanErrorRows) Conn() *pgx.Conn        { return nil }

func TestAttendanceConfigStore_GetTimeProfileAsOf(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin err")
		}))
		_, _, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 1}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, _, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no rows", func(t *testing.T) {
		tx := &stubTx{rowErr: pgx.ErrNoRows}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, ok, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Fatal("expected ok=false")
		}
	})

	t.Run("no rows commit error", func(t *testing.T) {
		tx := &stubTx{rowErr: pgx.ErrNoRows, commitErr: errors.New("commit err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, _, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, _, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{
			"Default", "active", "2026-01-01", "09:00", "18:00",
			1, 2, 3, "NONE", 0, int64(99),
		}}}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		v, ok, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
		if v.EffectiveDate != "2026-01-01" || v.ShiftStartLocal != "09:00" || v.LastEventDBID != 99 {
			t.Fatalf("v=%+v", v)
		}
	})

	t.Run("success commit error", func(t *testing.T) {
		tx := &stubTx{
			row: &stubRow{vals: []any{
				"Default", "active", "2026-01-01", "09:00", "18:00",
				1, 2, 3, "NONE", 0, int64(99),
			}},
			commitErr: errors.New("commit err"),
		}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, _, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAttendanceConfigStore_ListTimeProfileVersions(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin err")
		}))
		_, err := s.ListTimeProfileVersions(context.Background(), "t1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 1}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListTimeProfileVersions(context.Background(), "t1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListTimeProfileVersions(context.Background(), "t1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows scan error", func(t *testing.T) {
		tx := &stubTx{rows: &scanErrorRows{}}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListTimeProfileVersions(context.Background(), "t1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		tx := &stubTx{rows: &fixedRows{
			rows: [][]any{{"n", "active", "2026-01-01", "09:00", "18:00", 0, 0, 0, "NONE", 0, int64(1)}},
			err:  errors.New("rows err"),
		}}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListTimeProfileVersions(context.Background(), "t1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &fixedRows{rows: [][]any{{"n", "active", "2026-01-01", "09:00", "18:00", 0, 0, 0, "NONE", 0, int64(1)}}},
			commitErr: errors.New("commit err"),
		}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListTimeProfileVersions(context.Background(), "t1", 0)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		rows := &fixedRows{rows: [][]any{
			{"n", "active", "2026-01-01", "09:00", "18:00", 0, 0, 0, "NONE", 0, int64(1)},
			{"n2", "active", "2025-01-01", "10:00", "19:00", 1, 2, 3, "FLOOR", 15, int64(2)},
		}}
		tx := &stubTx{rows: rows}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		out, err := s.ListTimeProfileVersions(context.Background(), "t1", 2)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 2 || out[0].LastEventDBID != 1 || out[1].OvertimeRoundingMode != "FLOOR" {
			t.Fatalf("out=%+v", out)
		}
	})

	t.Run("success with limit clamp", func(t *testing.T) {
		rows := &fixedRows{rows: [][]any{
			{"n", "active", "2026-01-01", "09:00", "18:00", 0, 0, 0, "NONE", 0, int64(1)},
		}}
		tx := &stubTx{rows: rows}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		out, err := s.ListTimeProfileVersions(context.Background(), "t1", 999)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 1 {
			t.Fatalf("out=%+v", out)
		}
	})
}

func TestAttendanceConfigStore_UpsertTimeProfile(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin err")
		}))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 1}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective date required", func(t *testing.T) {
		tx := &stubTx{}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "", map[string]any{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exists query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create then exec error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{false}},
			execErr:   errors.New("exec err"),
			execErrAt: 2,
		}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"shift_start_local": "09:00"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update then exec error", func(t *testing.T) {
		tx := &stubTx{
			row:       &stubRow{vals: []any{true}},
			execErr:   errors.New("exec err"),
			execErrAt: 2,
		}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"shift_start_local": "09:00"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update success", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{true}}}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"shift_start_local": "09:00"}); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{true}}, commitErr: errors.New("commit err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"shift_start_local": "09:00"}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAttendanceConfigStore_HolidayDayOverrides(t *testing.T) {
	t.Run("list begin error", func(t *testing.T) {
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin err")
		}))
		_, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list set tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 1}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list scan error", func(t *testing.T) {
		tx := &stubTx{rows: &scanErrorRows{}}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 0)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list rows err", func(t *testing.T) {
		tx := &stubTx{rows: &fixedRows{
			rows: [][]any{{"2026-01-01", "WORKDAY", "", "", int64(1)}},
			err:  errors.New("rows err"),
		}}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 0)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list commit error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &fixedRows{rows: [][]any{{"2026-01-01", "WORKDAY", "", "", int64(1)}}},
			commitErr: errors.New("commit err"),
		}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		_, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 999999)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("list success", func(t *testing.T) {
		rows := &fixedRows{rows: [][]any{
			{"2026-01-01", "LEGAL_HOLIDAY", "NY", "note", int64(10)},
		}}
		tx := &stubTx{rows: rows}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		out, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 10)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 1 || out[0].DayType != "LEGAL_HOLIDAY" || out[0].LastEventDBID != 10 {
			t.Fatalf("out=%+v", out)
		}
	})

	t.Run("set requires day_date", func(t *testing.T) {
		tx := &stubTx{}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "", map[string]any{"day_type": "WORKDAY"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("clear requires day_date", func(t *testing.T) {
		tx := &stubTx{}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set begin error", func(t *testing.T) {
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin err")
		}))
		if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"day_type": "WORKDAY"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 1}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"day_type": "WORKDAY"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 2}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"day_type": "WORKDAY"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"day_type": "WORKDAY"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set success", func(t *testing.T) {
		tx := &stubTx{}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01", map[string]any{"day_type": "WORKDAY"}); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("clear success", func(t *testing.T) {
		tx := &stubTx{}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("clear begin error", func(t *testing.T) {
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin err")
		}))
		if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("clear tenant error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 1}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("clear exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec err"), execErrAt: 2}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("clear commit error", func(t *testing.T) {
		tx := &stubTx{commitErr: errors.New("commit err")}
		s := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil }))
		if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAttendanceConfigStore_MemoryStore_ImplementsInterface(t *testing.T) {
	s := newStaffingMemoryStore()
	if _, ok, err := s.GetTimeProfileAsOf(context.Background(), "t1", "2026-01-01"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if vs, err := s.ListTimeProfileVersions(context.Background(), "t1", 10); err != nil || vs != nil {
		t.Fatalf("vs=%v err=%v", vs, err)
	}
	if err := s.UpsertTimeProfile(context.Background(), "t1", "p1", "2026-01-01", map[string]any{}); err != nil {
		t.Fatalf("err=%v", err)
	}
	if ds, err := s.ListHolidayDayOverrides(context.Background(), "t1", "2026-01-01", "2026-02-01", 10); err != nil || ds != nil {
		t.Fatalf("ds=%v err=%v", ds, err)
	}
	if err := s.SetHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01", map[string]any{}); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.ClearHolidayDayOverride(context.Background(), "t1", "p1", "2026-01-01"); err != nil {
		t.Fatalf("err=%v", err)
	}
}
