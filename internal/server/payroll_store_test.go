package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type payPeriodRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *payPeriodRows) Close()                        {}
func (r *payPeriodRows) Err() error                    { return r.err }
func (r *payPeriodRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payPeriodRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payPeriodRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payPeriodRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "pp1"
	*(dest[1].(*string)) = "monthly"
	*(dest[2].(*string)) = "2026-01-01"
	*(dest[3].(*string)) = "2026-02-01"
	*(dest[4].(*string)) = "open"
	*(dest[5].(*string)) = ""
	return nil
}
func (r *payPeriodRows) Values() ([]any, error) { return nil, nil }
func (r *payPeriodRows) RawValues() [][]byte    { return nil }
func (r *payPeriodRows) Conn() *pgx.Conn        { return nil }

type payrollRunRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *payrollRunRows) Close()                        {}
func (r *payrollRunRows) Err() error                    { return r.err }
func (r *payrollRunRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payrollRunRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payrollRunRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payrollRunRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "run1"
	*(dest[1].(*string)) = "pp1"
	*(dest[2].(*string)) = "draft"
	*(dest[3].(*string)) = ""
	*(dest[4].(*string)) = ""
	*(dest[5].(*string)) = ""
	*(dest[6].(*string)) = "2026-01-01T00:00:00Z"
	return nil
}
func (r *payrollRunRows) Values() ([]any, error) { return nil, nil }
func (r *payrollRunRows) RawValues() [][]byte    { return nil }
func (r *payrollRunRows) Conn() *pgx.Conn        { return nil }

type calcPayrollTx struct {
	*stubTx
	rowN     int
	rowErrAt int
	rowErr   error
}

func (t *calcPayrollTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	t.rowN++
	if t.rowErrAt == t.rowN {
		return &stubRow{err: t.rowErr}
	}
	switch t.rowN {
	case 1:
		return &stubRow{vals: []any{"pp1"}}
	case 2:
		return &stubRow{vals: []any{"evt_start"}}
	case 3:
		return &stubRow{vals: []any{"evt_finish"}}
	case 4:
		return &stubRow{vals: []any{"run1", "pp1", "calculated", "2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z", "", "2026-01-01T00:00:00Z"}}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

type finalizePayrollTx struct {
	*stubTx
	rowN     int
	rowErrAt int
	rowErr   error
}

func (t *finalizePayrollTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	t.rowN++
	if t.rowErrAt == t.rowN {
		return &stubRow{err: t.rowErr}
	}
	switch t.rowN {
	case 1:
		return &stubRow{vals: []any{"pp1"}}
	case 2:
		return &stubRow{vals: []any{"evt_finalize"}}
	case 3:
		return &stubRow{vals: []any{"run1", "pp1", "finalized", "2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z", "2026-01-01T00:00:02Z", "2026-01-01T00:00:00Z"}}
	default:
		return &stubRow{err: errors.New("unexpected QueryRow")}
	}
}

func TestPayrollPGStore_ListPayPeriods(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error (with pay_group)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "Monthly")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payPeriodRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payPeriodRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit"), rows: &stubRows{empty: true}}, nil
		}))
		_, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success empty", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{empty: true}}, nil
		}))
		out, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 0 {
			t.Fatalf("expected empty, got %d", len(out))
		}
	})

	t.Run("success non-empty", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payPeriodRows{}}, nil
		}))
		out, err := store.ListPayPeriods(context.Background(), "t1", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || out[0].ID != "pp1" {
			t.Fatalf("unexpected result: %#v", out)
		}
	})
}

func TestPayrollPGStore_CreatePayPeriod(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing pay_group", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("pay_group not lower", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "Monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("start_date invalid", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "bad", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("end_date_exclusive invalid", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "bad")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("end_date_exclusive not after start", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen pay_period_id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("gen")}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen event_id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:     &stubRow{vals: []any{"pp1"}},
				row2Err: errors.New("event"),
			}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				execErr:   errors.New("submit"),
				execErrAt: 2,
				row:       &stubRow{vals: []any{"pp1"}},
				row2:      &stubRow{vals: []any{"evt1"}},
			}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				commitErr: errors.New("commit"),
				row:       &stubRow{vals: []any{"pp1"}},
				row2:      &stubRow{vals: []any{"evt1"}},
			}, nil
		}))
		_, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:  &stubRow{vals: []any{"pp1"}},
				row2: &stubRow{vals: []any{"evt1"}},
			}, nil
		}))
		period, err := store.CreatePayPeriod(context.Background(), "t1", "monthly", "2026-01-01", "2026-02-01")
		if err != nil {
			t.Fatal(err)
		}
		if period.ID != "pp1" || period.PayGroup != "monthly" {
			t.Fatalf("unexpected period: %#v", period)
		}
	})
}

func TestPayrollPGStore_ListPayrollRuns(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error (with pay_period_id)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRunRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRunRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{commitErr: errors.New("commit"), rows: &stubRows{empty: true}}, nil
		}))
		_, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success empty", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &stubRows{empty: true}}, nil
		}))
		out, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 0 {
			t.Fatalf("expected empty, got %d", len(out))
		}
	})

	t.Run("success non-empty", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRunRows{}}, nil
		}))
		out, err := store.ListPayrollRuns(context.Background(), "t1", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || out[0].ID != "run1" {
			t.Fatalf("unexpected result: %#v", out)
		}
	})
}

func TestPayrollPGStore_CreatePayrollRun(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing pay_period_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen run_id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("gen")}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen event_id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:     &stubRow{vals: []any{"run1"}},
				row2Err: errors.New("event"),
			}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				execErr:   errors.New("submit"),
				execErrAt: 2,
				row:       &stubRow{vals: []any{"run1"}},
				row2:      &stubRow{vals: []any{"evt1"}},
			}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("created_at query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:     &stubRow{vals: []any{"run1"}},
				row2:    &stubRow{vals: []any{"evt1"}},
				row3Err: errors.New("created_at"),
			}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				commitErr: errors.New("commit"),
				row:       &stubRow{vals: []any{"run1"}},
				row2:      &stubRow{vals: []any{"evt1"}},
				row3:      &stubRow{vals: []any{"2026-01-01T00:00:00Z"}},
			}, nil
		}))
		_, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row:  &stubRow{vals: []any{"run1"}},
				row2: &stubRow{vals: []any{"evt1"}},
				row3: &stubRow{vals: []any{"2026-01-01T00:00:00Z"}},
			}, nil
		}))
		run, err := store.CreatePayrollRun(context.Background(), "t1", "pp1")
		if err != nil {
			t.Fatal(err)
		}
		if run.ID != "run1" || run.PayPeriodID != "pp1" || run.CreatedAt == "" {
			t.Fatalf("unexpected run: %#v", run)
		}
	})
}

func TestPayrollPGStore_GetPayrollRun(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.GetPayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.GetPayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing run_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.GetPayrollRun(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.GetPayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				commitErr: errors.New("commit"),
				row:       &stubRow{vals: []any{"run1", "pp1", "draft", "", "", "", "2026-01-01T00:00:00Z"}},
			}, nil
		}))
		_, err := store.GetPayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{
				row: &stubRow{vals: []any{"run1", "pp1", "draft", "", "", "", "2026-01-01T00:00:00Z"}},
			}, nil
		}))
		run, err := store.GetPayrollRun(context.Background(), "t1", "run1")
		if err != nil {
			t.Fatal(err)
		}
		if run.ID != "run1" || run.PayPeriodID != "pp1" {
			t.Fatalf("unexpected run: %#v", run)
		}
	})
}

func TestPayrollPGStore_CalculatePayrollRun(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing run_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{}}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("pay_period_id query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{}, rowErrAt: 1, rowErr: errors.New("pp")}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event start id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{}, rowErrAt: 2, rowErr: errors.New("evt")}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc start exec error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{execErr: errors.New("start"), execErrAt: 2}}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event finish id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{}, rowErrAt: 3, rowErr: errors.New("evt")}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc finish exec error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{execErr: errors.New("finish"), execErrAt: 3}}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("run query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{}, rowErrAt: 4, rowErr: errors.New("run")}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{commitErr: errors.New("commit")}}, nil
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &calcPayrollTx{stubTx: &stubTx{}}, nil
		}))
		run, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err != nil {
			t.Fatal(err)
		}
		if run.RunState == "" {
			t.Fatalf("unexpected run: %#v", run)
		}
	})
}

func TestPayrollPGStore_FinalizePayrollRun(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing run_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{}}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("pay_period_id query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{}, rowErrAt: 1, rowErr: errors.New("pp")}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{}, rowErrAt: 2, rowErr: errors.New("evt")}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("finalize exec error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{execErr: errors.New("finalize"), execErrAt: 2}}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("run query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{}, rowErrAt: 3, rowErr: errors.New("run")}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{commitErr: errors.New("commit")}}, nil
		}))
		_, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &finalizePayrollTx{stubTx: &stubTx{}}, nil
		}))
		run, err := store.FinalizePayrollRun(context.Background(), "t1", "run1")
		if err != nil {
			t.Fatal(err)
		}
		if run.RunState != "finalized" {
			t.Fatalf("unexpected run: %#v", run)
		}
	})
}

func TestPayrollRender(t *testing.T) {
	_ = renderPayrollPeriods(nil, "", "")
	_ = renderPayrollPeriods([]PayPeriod{{ID: "pp1", PayGroup: "monthly", StartDate: "2026-01-01", EndDateExclusive: "2026-02-01", Status: "open"}}, "2026-01-01", "err")

	_ = renderPayrollRuns(nil, nil, "", "", "")
	_ = renderPayrollRuns([]PayrollRun{{ID: "run1", PayPeriodID: "pp1", RunState: "draft"}}, []PayPeriod{{ID: "pp1", PayGroup: "monthly", StartDate: "2026-01-01", EndDateExclusive: "2026-02-01"}}, "2026-01-01", "pp1", "err")

	_ = renderPayrollRun("run1", "", PayrollRun{}, "err")
	_ = renderPayrollRun("run1", "2026-01-01", PayrollRun{ID: "run1", PayPeriodID: "pp1", RunState: "draft"}, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/bad/prefix", nil)
	_, _ = requireRunIDFromPath(rec, req, "/org/payroll-runs/")
}
