package server

import (
	"context"
	"encoding/json"
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
	*(dest[3].(*bool)) = false
	*(dest[4].(*string)) = ""
	*(dest[5].(*string)) = ""
	*(dest[6].(*string)) = ""
	*(dest[7].(*string)) = "2026-01-01T00:00:00Z"
	return nil
}
func (r *payrollRunRows) Values() ([]any, error) { return nil, nil }
func (r *payrollRunRows) RawValues() [][]byte    { return nil }
func (r *payrollRunRows) Conn() *pgx.Conn        { return nil }

type payslipRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *payslipRows) Close()                        {}
func (r *payslipRows) Err() error                    { return r.err }
func (r *payslipRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payslipRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payslipRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payslipRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "ps1"
	*(dest[1].(*string)) = "run1"
	*(dest[2].(*string)) = "pp1"
	*(dest[3].(*string)) = "person1"
	*(dest[4].(*string)) = "asmt1"
	*(dest[5].(*string)) = "CNY"
	*(dest[6].(*string)) = "100.00"
	*(dest[7].(*string)) = "100.00"
	*(dest[8].(*string)) = "0.00"
	return nil
}
func (r *payslipRows) Values() ([]any, error) { return nil, nil }
func (r *payslipRows) RawValues() [][]byte    { return nil }
func (r *payslipRows) Conn() *pgx.Conn        { return nil }

type payslipItemRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *payslipItemRows) Close()                        {}
func (r *payslipItemRows) Err() error                    { return r.err }
func (r *payslipItemRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payslipItemRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payslipItemRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payslipItemRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "it1"
	*(dest[1].(*string)) = "EARNING_BASE_SALARY"
	*(dest[2].(*string)) = "earning"
	*(dest[3].(*string)) = "100.00"
	*(dest[4].(*string)) = "amount"
	*(dest[5].(*string)) = "employee"
	*(dest[6].(*string)) = ""
	*(dest[7].(*string)) = ""
	*(dest[8].(*string)) = "{}"
	return nil
}
func (r *payslipItemRows) Values() ([]any, error) { return nil, nil }
func (r *payslipItemRows) RawValues() [][]byte    { return nil }
func (r *payslipItemRows) Conn() *pgx.Conn        { return nil }

type payslipItemInputRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *payslipItemInputRows) Close()                        {}
func (r *payslipItemInputRows) Err() error                    { return r.err }
func (r *payslipItemInputRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payslipItemInputRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payslipItemInputRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payslipItemInputRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "in1"
	*(dest[1].(*string)) = "EARNING_LONG_SERVICE_AWARD"
	*(dest[2].(*string)) = "earning"
	*(dest[3].(*string)) = "CNY"
	*(dest[4].(*string)) = "net_guaranteed_iit"
	*(dest[5].(*string)) = "employer"
	*(dest[6].(*string)) = "20000.00"
	*(dest[7].(*string)) = "evt1"
	*(dest[8].(*string)) = "2026-01-01T00:00:00Z"
	return nil
}
func (r *payslipItemInputRows) Values() ([]any, error) { return nil, nil }
func (r *payslipItemInputRows) RawValues() [][]byte    { return nil }
func (r *payslipItemInputRows) Conn() *pgx.Conn        { return nil }

type siPolicyVersionRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *siPolicyVersionRows) Close()                        {}
func (r *siPolicyVersionRows) Err() error                    { return r.err }
func (r *siPolicyVersionRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *siPolicyVersionRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *siPolicyVersionRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *siPolicyVersionRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "p1"
	*(dest[1].(*string)) = "CN-310000"
	*(dest[2].(*string)) = "default"
	*(dest[3].(*string)) = "PENSION"
	*(dest[4].(*string)) = "2026-01-01"
	*(dest[5].(*string)) = "0.16"
	*(dest[6].(*string)) = "0.08"
	*(dest[7].(*string)) = "0.00"
	*(dest[8].(*string)) = "99999.99"
	*(dest[9].(*string)) = "HALF_UP"
	*(dest[10].(*int)) = 2
	return nil
}
func (r *siPolicyVersionRows) Values() ([]any, error) { return nil, nil }
func (r *siPolicyVersionRows) RawValues() [][]byte    { return nil }
func (r *siPolicyVersionRows) Conn() *pgx.Conn        { return nil }

type payslipSocialInsuranceItemRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *payslipSocialInsuranceItemRows) Close()                        {}
func (r *payslipSocialInsuranceItemRows) Err() error                    { return r.err }
func (r *payslipSocialInsuranceItemRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payslipSocialInsuranceItemRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payslipSocialInsuranceItemRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payslipSocialInsuranceItemRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "PENSION"
	*(dest[1].(*string)) = "100.00"
	*(dest[2].(*string)) = "8.00"
	*(dest[3].(*string)) = "16.00"
	*(dest[4].(*string)) = "2026-01-01"
	return nil
}
func (r *payslipSocialInsuranceItemRows) Values() ([]any, error) { return nil, nil }
func (r *payslipSocialInsuranceItemRows) RawValues() [][]byte    { return nil }
func (r *payslipSocialInsuranceItemRows) Conn() *pgx.Conn        { return nil }

type payrollRecalcRequestRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
	applied bool
}

func (r *payrollRecalcRequestRows) Close()                        {}
func (r *payrollRecalcRequestRows) Err() error                    { return r.err }
func (r *payrollRecalcRequestRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payrollRecalcRequestRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payrollRecalcRequestRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payrollRecalcRequestRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "rr1"
	*(dest[1].(*string)) = "person1"
	*(dest[2].(*string)) = "assign1"
	*(dest[3].(*string)) = "2026-01-15"
	*(dest[4].(*string)) = "pp1"
	*(dest[5].(*string)) = "2026-02-01T00:00:00Z"
	*(dest[6].(*bool)) = r.applied
	return nil
}
func (r *payrollRecalcRequestRows) Values() ([]any, error) { return nil, nil }
func (r *payrollRecalcRequestRows) RawValues() [][]byte    { return nil }
func (r *payrollRecalcRequestRows) Conn() *pgx.Conn        { return nil }

type payrollRecalcAdjustmentRows struct {
	empty   bool
	nextN   int
	scanErr error
	err     error
}

func (r *payrollRecalcAdjustmentRows) Close()                        {}
func (r *payrollRecalcAdjustmentRows) Err() error                    { return r.err }
func (r *payrollRecalcAdjustmentRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *payrollRecalcAdjustmentRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *payrollRecalcAdjustmentRows) Next() bool {
	if r.empty {
		return false
	}
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *payrollRecalcAdjustmentRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*string)) = "earning"
	*(dest[1].(*string)) = "EARNING_BASE_SALARY"
	*(dest[2].(*string)) = "100.00"
	return nil
}
func (r *payrollRecalcAdjustmentRows) Values() ([]any, error) { return nil, nil }
func (r *payrollRecalcAdjustmentRows) RawValues() [][]byte    { return nil }
func (r *payrollRecalcAdjustmentRows) Conn() *pgx.Conn        { return nil }

func seqBeginner(txs ...pgx.Tx) beginnerFunc {
	n := 0
	return func(context.Context) (pgx.Tx, error) {
		if n >= len(txs) {
			return nil, errors.New("unexpected begin")
		}
		tx := txs[n]
		n++
		return tx, nil
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
		return &stubRow{vals: []any{"run1", "pp1", "finalized", false, "2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z", "2026-01-01T00:00:02Z", "2026-01-01T00:00:00Z"}}
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
				row:       &stubRow{vals: []any{"run1", "pp1", "draft", false, "", "", "", "2026-01-01T00:00:00Z"}},
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
				row: &stubRow{vals: []any{"run1", "pp1", "draft", false, "", "", "", "2026-01-01T00:00:00Z"}},
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
	runRow := &stubRow{vals: []any{"run1", "pp1", "calculated", false, "2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z", "", "2026-01-01T00:00:00Z"}}

	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing run_id", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{execErr: errors.New("exec"), execErrAt: 1}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("pay_period_id query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rowErr: errors.New("pp")}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("pay_period_id commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{row: &stubRow{vals: []any{"pp1"}}, commitErr: errors.New("commit")}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc start begin error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{row: &stubRow{vals: []any{"pp1"}}}))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc start set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{execErr: errors.New("exec"), execErrAt: 1},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc start commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}, commitErr: errors.New("commit")},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc finish begin error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc finish set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{execErr: errors.New("exec"), execErrAt: 1},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event start id error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{rowErr: errors.New("evt")},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc start exec error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}, execErr: errors.New("start"), execErrAt: 2},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event finish id error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{rowErr: errors.New("evt")},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc finish exec error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, execErr: errors.New("finish"), execErrAt: 2},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("calc finish exec error (calc_fail ok)", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, execErr: errors.New("finish"), execErrAt: 2},
			&stubTx{row: &stubRow{vals: []any{"evt_fail"}}},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil || err.Error() != "finish" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("calc finish exec error (calc_fail set tenant error)", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, execErr: errors.New("finish"), execErrAt: 2},
			&stubTx{execErr: errors.New("fail_exec"), execErrAt: 1},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil || err.Error() != "finish" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("calc finish exec error (calc_fail event id error)", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, execErr: errors.New("finish"), execErrAt: 2},
			&stubTx{rowErr: errors.New("fail_row")},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil || err.Error() != "finish" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("run query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, row2Err: errors.New("run")},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, row2: runRow, commitErr: errors.New("commit")},
		))
		_, err := store.CalculatePayrollRun(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(
			&stubTx{row: &stubRow{vals: []any{"pp1"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_start"}}},
			&stubTx{row: &stubRow{vals: []any{"evt_finish"}}, row2: runRow},
		))
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

func TestPgErrorMessage(t *testing.T) {
	if got := pgErrorMessage(errors.New("boom")); got != "UNKNOWN" {
		t.Fatalf("got=%q", got)
	}
	if got := pgErrorMessage(&pgconn.PgError{Message: "PGMSG"}); got != "PGMSG" {
		t.Fatalf("got=%q", got)
	}
}

func TestPayrollPGStore_ListPayslips(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{execErr: errors.New("exec"), execErrAt: 1}))
		_, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing run_id", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		_, err := store.ListPayslips(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{queryErr: errors.New("query")}))
		_, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &payslipRows{scanErr: errors.New("scan")}}))
		_, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &payslipRows{err: errors.New("rows")}}))
		_, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &payslipRows{}, commitErr: errors.New("commit")}))
		_, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &payslipRows{}}))
		got, err := store.ListPayslips(context.Background(), "t1", "run1")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != "ps1" {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_GetPayslip(t *testing.T) {
	headerRow := &stubRow{vals: []any{"ps1", "run1", "pp1", "person1", "asmt1", "CNY", "100.00", "100.00", "0.00"}}
	totalsRow := &stubRow{vals: []any{"0.00", "0.00"}}

	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{execErr: errors.New("exec"), execErrAt: 1}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing payslip_id", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		_, err := store.GetPayslip(context.Background(), "t1", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("header query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rowErr: errors.New("header")}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("items query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{row: headerRow, queryErr: errors.New("items")}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("items scan error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{row: headerRow, rows: &payslipItemRows{scanErr: errors.New("scan")}}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("items rows err", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{row: headerRow, rows: &payslipItemRows{err: errors.New("rows")}}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("item inputs query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:        headerRow,
			rows:       &payslipItemRows{},
			queryErr:   errors.New("inputs query"),
			queryErrAt: 2,
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("item inputs scan error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:   headerRow,
			rows:  &payslipItemRows{},
			rows2: &payslipItemInputRows{scanErr: errors.New("scan")},
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("item inputs rows err", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:   headerRow,
			rows:  &payslipItemRows{},
			rows2: &payslipItemInputRows{empty: true, err: errors.New("rows")},
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("social insurance totals query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:     headerRow,
			rows:    &payslipItemRows{},
			rows2:   &payslipItemInputRows{empty: true},
			row2Err: errors.New("totals"),
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("social insurance items query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:        headerRow,
			row2:       totalsRow,
			rows:       &payslipItemRows{},
			rows2:      &payslipItemInputRows{empty: true},
			queryErr:   errors.New("si query"),
			queryErrAt: 3,
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("social insurance items scan error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:   headerRow,
			row2:  totalsRow,
			rows:  &payslipItemRows{},
			rows2: &payslipItemInputRows{empty: true},
			rows3: &payslipSocialInsuranceItemRows{scanErr: errors.New("scan")},
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("social insurance items rows err", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:   headerRow,
			row2:  totalsRow,
			rows:  &payslipItemRows{},
			rows2: &payslipItemInputRows{empty: true},
			rows3: &payslipSocialInsuranceItemRows{err: errors.New("rows")},
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:       headerRow,
			row2:      totalsRow,
			rows:      &payslipItemRows{},
			rows2:     &payslipItemInputRows{empty: true},
			rows3:     &payslipSocialInsuranceItemRows{empty: true},
			commitErr: errors.New("commit"),
		}))
		_, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:   headerRow,
			row2:  totalsRow,
			rows:  &payslipItemRows{},
			rows2: &payslipItemInputRows{empty: false},
			rows3: &payslipSocialInsuranceItemRows{empty: false},
		}))
		got, err := store.GetPayslip(context.Background(), "t1", "ps1")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != "ps1" || len(got.Items) != 1 || len(got.ItemInputs) != 1 || len(got.SocialInsuranceItems) != 1 {
			t.Fatalf("got=%#v", got)
		}
		if string(got.Items[0].Meta) != "{}" {
			t.Fatalf("meta=%q", string(got.Items[0].Meta))
		}
		if got.ItemInputs[0].CalcMode != "net_guaranteed_iit" {
			t.Fatalf("calc_mode=%q", got.ItemInputs[0].CalcMode)
		}
	})
}

func TestPayrollPGStore_ListSocialInsurancePolicyVersions(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{execErr: errors.New("exec")}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("as_of required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", " ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("as_of invalid", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-99")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{queryErr: errors.New("query")}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &siPolicyVersionRows{scanErr: errors.New("scan")}}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &siPolicyVersionRows{err: errors.New("rows")}}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &siPolicyVersionRows{}, commitErr: errors.New("commit")}))
		_, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rows: &siPolicyVersionRows{}}))
		got, err := store.ListSocialInsurancePolicyVersions(context.Background(), "t1", "2026-01-01")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].PolicyID != "p1" || got[0].Precision != 2 {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_UpsertSocialInsurancePolicyVersion(t *testing.T) {
	validInput := func() SocialInsurancePolicyUpsertInput {
		return SocialInsurancePolicyUpsertInput{
			EventID:       "evt1",
			CityCode:      "cn-310000",
			HukouType:     "",
			InsuranceType: "pension",
			EffectiveDate: "2026-01-01",
			EmployerRate:  "0.16",
			EmployeeRate:  "0.08",
			BaseFloor:     "0.00",
			BaseCeiling:   "99999.99",
			RoundingRule:  "half_up",
			Precision:     2,
		}
	}

	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{execErr: errors.New("exec")}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event_id required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.EventID = " "
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("city_code required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.CityCode = ""
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("hukou_type not supported", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.HukouType = "urban"
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("insurance_type required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.InsuranceType = ""
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("insurance_type invalid", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.InsuranceType = "invalid"
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.EffectiveDate = ""
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("effective_date invalid", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.EffectiveDate = "2026-01-99"
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rates and base required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.EmployeeRate = ""
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rounding_rule required", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.RoundingRule = ""
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rounding_rule invalid", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.RoundingRule = "floor"
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("precision invalid", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.Precision = 3
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rules_config invalid json", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.RulesConfig = json.RawMessage("{")
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rules_config must be object", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{}))
		in := validInput()
		in.RulesConfig = json.RawMessage(`[]`)
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("policy query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{rowErr: errors.New("policy query")}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen policy_id error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:     &stubRow{err: pgx.ErrNoRows},
			row2Err: errors.New("gen"),
		}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("hasEvent query error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:     &stubRow{err: pgx.ErrNoRows},
			row2:    &stubRow{vals: []any{"p1"}},
			row3Err: errors.New("exists"),
		}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:     &stubRow{err: pgx.ErrNoRows},
			row2:    &stubRow{vals: []any{"p1"}},
			row3:    &stubRow{vals: []any{false}},
			row4Err: errors.New("submit"),
		}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			commitErr: errors.New("commit"),
			row:       &stubRow{err: pgx.ErrNoRows},
			row2:      &stubRow{vals: []any{"p1"}},
			row3:      &stubRow{vals: []any{false}},
			row4:      &stubRow{vals: []any{int64(1)}},
		}))
		_, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success create", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:  &stubRow{err: pgx.ErrNoRows},
			row2: &stubRow{vals: []any{"p1"}},
			row3: &stubRow{vals: []any{false}},
			row4: &stubRow{vals: []any{int64(1)}},
		}))
		got, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", validInput())
		if err != nil {
			t.Fatal(err)
		}
		if got.PolicyID != "p1" || got.LastEventDBID != 1 || got.InsuranceType != "PENSION" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("success update", func(t *testing.T) {
		store := newStaffingPGStore(seqBeginner(&stubTx{
			row:  &stubRow{vals: []any{"p1"}},
			row2: &stubRow{vals: []any{true}},
			row3: &stubRow{vals: []any{int64(2)}},
		}))
		in := validInput()
		in.RulesConfig = json.RawMessage(`{"k":1}`)
		got, err := store.UpsertSocialInsurancePolicyVersion(context.Background(), "t1", in)
		if err != nil {
			t.Fatal(err)
		}
		if got.PolicyID != "p1" || got.LastEventDBID != 2 || got.InsuranceType != "PENSION" {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_GetPayrollBalances(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.GetPayrollBalances(context.Background(), "t1", "p1", 2026)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, err := store.GetPayrollBalances(context.Background(), "t1", "p1", 2026)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing person_uuid", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, err := store.GetPayrollBalances(context.Background(), "t1", "", 2026)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tax_year out of range", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		_, err := store.GetPayrollBalances(context.Background(), "t1", "p1", 1999)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query row error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("row")})
		_, err := store.GetPayrollBalances(context.Background(), "t1", "p1", 2026)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{
			row:       &stubRow{vals: []any{"t1", "p1", 2026, 1, 2, "20000.00", "1000.00", "10000.00", "9000.00", "270.00", "270.00", "0.00"}},
			commitErr: errors.New("commit"),
		})
		_, err := store.GetPayrollBalances(context.Background(), "t1", "p1", 2026)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{
			row: &stubRow{vals: []any{"t1", "p1", 2026, 1, 2, "20000.00", "1000.00", "10000.00", "9000.00", "270.00", "270.00", "0.00"}},
		})
		got, err := store.GetPayrollBalances(context.Background(), "t1", "p1", 2026)
		if err != nil {
			t.Fatal(err)
		}
		if got.PersonUUID != "p1" || got.TaxYear != 2026 || got.YTDIncome != "20000.00" {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_UpsertPayrollIITSAD(t *testing.T) {
	validIn := func() PayrollIITSADUpsertInput {
		return PayrollIITSADUpsertInput{
			EventID:    "e1",
			PersonUUID: "p1",
			TaxYear:    2026,
			TaxMonth:   2,
			Amount:     "100.00",
			RequestID:  "",
		}
	}

	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", validIn())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{execErr: errors.New("exec")})
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", validIn())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("validation error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		in := validIn()
		in.TaxMonth = 0
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("validation error (missing event_id)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		in := validIn()
		in.EventID = ""
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("validation error (missing person_uuid)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		in := validIn()
		in.PersonUUID = ""
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("validation error (tax_year out of range)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		in := validIn()
		in.TaxYear = 1999
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("validation error (missing amount)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{})
		in := validIn()
		in.Amount = ""
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", in)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query row error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{rowErr: errors.New("row")})
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", validIn())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unexpected event db id", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{row: &stubRow{vals: []any{int64(0)}}})
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", validIn())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{
			row:       &stubRow{vals: []any{int64(1)}},
			commitErr: errors.New("commit"),
		})
		_, err := store.UpsertPayrollIITSAD(context.Background(), "t1", validIn())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (default request_id)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{row: &stubRow{vals: []any{int64(1)}}})
		got, err := store.UpsertPayrollIITSAD(context.Background(), "t1", validIn())
		if err != nil {
			t.Fatal(err)
		}
		if got.EventID != "e1" || got.RequestID != "e1" || got.Amount != "100.00" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("ok (explicit request_id)", func(t *testing.T) {
		store := newStaffingPGStore(&stubTx{row: &stubRow{vals: []any{int64(1)}}})
		in := validIn()
		in.RequestID = "req1"
		got, err := store.UpsertPayrollIITSAD(context.Background(), "t1", in)
		if err != nil {
			t.Fatal(err)
		}
		if got.EventID != "e1" || got.RequestID != "req1" || got.Amount != "100.00" {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_ListPayrollRecalcRequests(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "boom")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRecalcRequestRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRecalcRequestRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRecalcRequestRows{empty: true}, commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (no filters)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRecalcRequestRows{applied: false}}, nil
		}))
		got, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].RecalcRequestID != "rr1" || got[0].Applied {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("ok (pending)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRecalcRequestRows{applied: false}}, nil
		}))
		got, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "", "pending")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].RecalcRequestID != "rr1" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("ok (applied + person_uuid)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rows: &payrollRecalcRequestRows{applied: true}}, nil
		}))
		got, err := store.ListPayrollRecalcRequests(context.Background(), "t1", "person1", "applied")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].RecalcRequestID != "rr1" || !got[0].Applied {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_GetPayrollRecalcRequest(t *testing.T) {
	headerRow := &stubRow{vals: []any{
		"rr1",
		"evt1",
		"assignment",
		"req1",
		"init1",
		"2026-02-01T00:00:00Z",
		"2026-02-01T00:00:00Z",
		"person1",
		"assign1",
		"2026-01-15",
		"pp1",
		"run1",
		"ps1",
	}}
	appRow := &stubRow{vals: []any{
		"1",
		"app_evt1",
		"rr1",
		"run2",
		"pp2",
		"2026-02-02T00:00:00Z",
	}}

	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing recalc_request_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", " ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request row error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("application row error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2Err: errors.New("app")}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("adjustments query error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2Err: pgx.ErrNoRows, queryErr: errors.New("query")}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("adjustments scan error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2Err: pgx.ErrNoRows, rows: &payrollRecalcAdjustmentRows{scanErr: errors.New("scan")}}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("adjustments rows error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2Err: pgx.ErrNoRows, rows: &payrollRecalcAdjustmentRows{err: errors.New("rows")}}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2Err: pgx.ErrNoRows, rows: &payrollRecalcAdjustmentRows{empty: true}, commitErr: errors.New("commit")}, nil
		}))
		_, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (no application)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2Err: pgx.ErrNoRows, rows: &payrollRecalcAdjustmentRows{empty: true}}, nil
		}))
		got, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err != nil {
			t.Fatal(err)
		}
		if got.RecalcRequestID != "rr1" || got.Application != nil {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("ok (with application + adjustments)", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: headerRow, row2: appRow, rows: &payrollRecalcAdjustmentRows{}}, nil
		}))
		got, err := store.GetPayrollRecalcRequest(context.Background(), "t1", "rr1")
		if err != nil {
			t.Fatal(err)
		}
		if got.RecalcRequestID != "rr1" || got.Application == nil || got.Application.TargetRunID != "run2" || len(got.AdjustmentsSummary) != 1 {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestPayrollPGStore_ApplyPayrollRecalcRequest(t *testing.T) {
	appRow := &stubRow{vals: []any{
		"1",
		"app_evt1",
		"rr1",
		"run2",
		"pp2",
		"2026-02-02T00:00:00Z",
	}}

	t.Run("begin error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing initiator_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", " ", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing recalc_request_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", " ", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing target_run_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", " ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("gen uuid error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("uuid")}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{"e1"}}, row2Err: errors.New("submit")}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unexpected application_db_id", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{"e1"}}, row2: &stubRow{vals: []any{int64(0)}}}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("application fetch error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{"e1"}}, row2: &stubRow{vals: []any{int64(1)}}, row3Err: errors.New("fetch")}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{"e1"}}, row2: &stubRow{vals: []any{int64(1)}}, row3: appRow, commitErr: errors.New("commit")}, nil
		}))
		_, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newStaffingPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{row: &stubRow{vals: []any{"e1"}}, row2: &stubRow{vals: []any{int64(1)}}, row3: appRow}, nil
		}))
		got, err := store.ApplyPayrollRecalcRequest(context.Background(), "t1", "i1", "rr1", "run2")
		if err != nil {
			t.Fatal(err)
		}
		if got.RecalcRequestID != "rr1" || got.TargetPayPeriodID != "pp2" {
			t.Fatalf("got=%#v", got)
		}
	})
}
