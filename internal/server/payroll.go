package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type PayPeriod struct {
	ID               string `json:"id"`
	PayGroup         string `json:"pay_group"`
	StartDate        string `json:"start_date"`
	EndDateExclusive string `json:"end_date_exclusive"`
	Status           string `json:"status"`
	ClosedAt         string `json:"closed_at"`
}

type PayrollRun struct {
	ID             string `json:"id"`
	PayPeriodID    string `json:"pay_period_id"`
	RunState       string `json:"run_state"`
	CalcStartedAt  string `json:"calc_started_at"`
	CalcFinishedAt string `json:"calc_finished_at"`
	FinalizedAt    string `json:"finalized_at"`
	CreatedAt      string `json:"created_at"`
}

type Payslip struct {
	ID            string `json:"id"`
	RunID         string `json:"run_id"`
	PayPeriodID   string `json:"pay_period_id"`
	PersonUUID    string `json:"person_uuid"`
	AssignmentID  string `json:"assignment_id"`
	Currency      string `json:"currency"`
	GrossPay      string `json:"gross_pay"`
	NetPay        string `json:"net_pay"`
	EmployerTotal string `json:"employer_total"`
}

type PayslipItem struct {
	ID       string          `json:"id"`
	ItemCode string          `json:"item_code"`
	ItemKind string          `json:"item_kind"`
	Amount   string          `json:"amount"`
	Meta     json.RawMessage `json:"meta"`
}

type PayslipDetail struct {
	Payslip
	Items []PayslipItem `json:"items"`
}

type PayrollStore interface {
	ListPayPeriods(ctx context.Context, tenantID string, payGroup string) ([]PayPeriod, error)
	CreatePayPeriod(ctx context.Context, tenantID string, payGroup string, startDate string, endDateExclusive string) (PayPeriod, error)

	ListPayrollRuns(ctx context.Context, tenantID string, payPeriodID string) ([]PayrollRun, error)
	CreatePayrollRun(ctx context.Context, tenantID string, payPeriodID string) (PayrollRun, error)
	GetPayrollRun(ctx context.Context, tenantID string, runID string) (PayrollRun, error)
	CalculatePayrollRun(ctx context.Context, tenantID string, runID string) (PayrollRun, error)
	FinalizePayrollRun(ctx context.Context, tenantID string, runID string) (PayrollRun, error)

	ListPayslips(ctx context.Context, tenantID string, runID string) ([]Payslip, error)
	GetPayslip(ctx context.Context, tenantID string, payslipID string) (PayslipDetail, error)
}

func (s *staffingPGStore) ListPayPeriods(ctx context.Context, tenantID string, payGroup string) ([]PayPeriod, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	payGroup = strings.TrimSpace(payGroup)
	if payGroup != "" {
		payGroup = strings.ToLower(payGroup)
	}

	var rows pgRows
	if payGroup == "" {
		rows, err = tx.Query(ctx, `
SELECT
  id::text,
  pay_group,
  lower(period)::text AS start_date,
  upper(period)::text AS end_date_exclusive,
  status,
  COALESCE(closed_at::text, '') AS closed_at
FROM staffing.pay_periods
WHERE tenant_id = $1::uuid
ORDER BY lower(period) DESC, id::text ASC
`, tenantID)
	} else {
		rows, err = tx.Query(ctx, `
SELECT
  id::text,
  pay_group,
  lower(period)::text AS start_date,
  upper(period)::text AS end_date_exclusive,
  status,
  COALESCE(closed_at::text, '') AS closed_at
FROM staffing.pay_periods
WHERE tenant_id = $1::uuid
  AND pay_group = $2::text
ORDER BY lower(period) DESC, id::text ASC
`, tenantID, payGroup)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PayPeriod
	for rows.Next() {
		var p PayPeriod
		if err := rows.Scan(&p.ID, &p.PayGroup, &p.StartDate, &p.EndDateExclusive, &p.Status, &p.ClosedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) CreatePayPeriod(ctx context.Context, tenantID string, payGroup string, startDate string, endDateExclusive string) (PayPeriod, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayPeriod{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayPeriod{}, err
	}

	payGroup = strings.TrimSpace(payGroup)
	if payGroup == "" {
		return PayPeriod{}, errors.New("pay_group is required")
	}
	if payGroup != strings.ToLower(payGroup) {
		return PayPeriod{}, errors.New("pay_group must be lower")
	}

	startDate = strings.TrimSpace(startDate)
	endDateExclusive = strings.TrimSpace(endDateExclusive)
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return PayPeriod{}, errors.New("start_date invalid: " + err.Error())
	}
	end, err := time.Parse("2006-01-02", endDateExclusive)
	if err != nil {
		return PayPeriod{}, errors.New("end_date_exclusive invalid: " + err.Error())
	}
	if !end.After(start) {
		return PayPeriod{}, errors.New("end_date_exclusive must be after start_date")
	}

	var payPeriodID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&payPeriodID); err != nil {
		return PayPeriod{}, err
	}
	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return PayPeriod{}, err
	}

	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_pay_period_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  daterange($5::date, $6::date, '[)'),
  $7::text,
  $8::uuid
)
`, eventID, tenantID, payPeriodID, payGroup, startDate, endDateExclusive, eventID, tenantID); err != nil {
		return PayPeriod{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayPeriod{}, err
	}

	return PayPeriod{
		ID:               payPeriodID,
		PayGroup:         payGroup,
		StartDate:        startDate,
		EndDateExclusive: endDateExclusive,
		Status:           "open",
		ClosedAt:         "",
	}, nil
}

func (s *staffingPGStore) ListPayrollRuns(ctx context.Context, tenantID string, payPeriodID string) ([]PayrollRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	payPeriodID = strings.TrimSpace(payPeriodID)

	var rows pgRows
	if payPeriodID == "" {
		rows, err = tx.Query(ctx, `
SELECT
  id::text,
  pay_period_id::text,
  run_state,
  COALESCE(calc_started_at::text, '') AS calc_started_at,
  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
  COALESCE(finalized_at::text, '') AS finalized_at,
  created_at::text
FROM staffing.payroll_runs
WHERE tenant_id = $1::uuid
ORDER BY created_at DESC, id::text ASC
`, tenantID)
	} else {
		rows, err = tx.Query(ctx, `
SELECT
  id::text,
  pay_period_id::text,
  run_state,
  COALESCE(calc_started_at::text, '') AS calc_started_at,
  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
  COALESCE(finalized_at::text, '') AS finalized_at,
  created_at::text
FROM staffing.payroll_runs
WHERE tenant_id = $1::uuid
  AND pay_period_id = $2::uuid
ORDER BY created_at DESC, id::text ASC
`, tenantID, payPeriodID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PayrollRun
	for rows.Next() {
		var r PayrollRun
		if err := rows.Scan(&r.ID, &r.PayPeriodID, &r.RunState, &r.CalcStartedAt, &r.CalcFinishedAt, &r.FinalizedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) CreatePayrollRun(ctx context.Context, tenantID string, payPeriodID string) (PayrollRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollRun{}, err
	}

	payPeriodID = strings.TrimSpace(payPeriodID)
	if payPeriodID == "" {
		return PayrollRun{}, errors.New("pay_period_id is required")
	}

	var runID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&runID); err != nil {
		return PayrollRun{}, err
	}
	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return PayrollRun{}, err
	}

	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'CREATE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
)
`, eventID, tenantID, runID, payPeriodID, eventID, tenantID); err != nil {
		return PayrollRun{}, err
	}

	var createdAt string
	if err := tx.QueryRow(ctx, `
SELECT created_at::text
FROM staffing.payroll_runs
WHERE tenant_id = $1::uuid AND id = $2::uuid
`, tenantID, runID).Scan(&createdAt); err != nil {
		return PayrollRun{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollRun{}, err
	}

	return PayrollRun{
		ID:          runID,
		PayPeriodID: payPeriodID,
		RunState:    "draft",
		CreatedAt:   createdAt,
	}, nil
}

func (s *staffingPGStore) GetPayrollRun(ctx context.Context, tenantID string, runID string) (PayrollRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollRun{}, err
	}

	runID = strings.TrimSpace(runID)
	if runID == "" {
		return PayrollRun{}, errors.New("run_id is required")
	}

	var out PayrollRun
	if err := tx.QueryRow(ctx, `
SELECT
  id::text,
  pay_period_id::text,
  run_state,
  COALESCE(calc_started_at::text, '') AS calc_started_at,
  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
  COALESCE(finalized_at::text, '') AS finalized_at,
  created_at::text
FROM staffing.payroll_runs
WHERE tenant_id = $1::uuid AND id = $2::uuid
`, tenantID, runID).Scan(&out.ID, &out.PayPeriodID, &out.RunState, &out.CalcStartedAt, &out.CalcFinishedAt, &out.FinalizedAt, &out.CreatedAt); err != nil {
		return PayrollRun{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollRun{}, err
	}
	return out, nil
}

func (s *staffingPGStore) CalculatePayrollRun(ctx context.Context, tenantID string, runID string) (PayrollRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return PayrollRun{}, errors.New("run_id is required")
	}

	var payPeriodID string
	{
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return PayrollRun{}, err
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
			return PayrollRun{}, err
		}

		if err := tx.QueryRow(ctx, `
	SELECT pay_period_id::text
	FROM staffing.payroll_runs
	WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, runID).Scan(&payPeriodID); err != nil {
			return PayrollRun{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return PayrollRun{}, err
		}
	}

	{
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return PayrollRun{}, err
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
			return PayrollRun{}, err
		}

		var eventIDStart string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventIDStart); err != nil {
			return PayrollRun{}, err
		}
		if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_payroll_run_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::uuid,
	  'CALC_START',
	  '{}'::jsonb,
	  $5::text,
	  $6::uuid
	)
	`, eventIDStart, tenantID, runID, payPeriodID, eventIDStart, tenantID); err != nil {
			return PayrollRun{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return PayrollRun{}, err
		}
	}

	{
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return PayrollRun{}, err
		}
		defer func() { _ = tx.Rollback(context.Background()) }()

		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
			return PayrollRun{}, err
		}

		var eventIDFinish string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventIDFinish); err != nil {
			return PayrollRun{}, err
		}
		if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_payroll_run_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::uuid,
	  'CALC_FINISH',
	  '{}'::jsonb,
	  $5::text,
	  $6::uuid
	)
	`, eventIDFinish, tenantID, runID, payPeriodID, eventIDFinish, tenantID); err != nil {
			cause := err
			code := pgErrorMessage(err)

			failPayload := `{"error_code":` + strconv.Quote(code) + `}`
			{
				txFail, failErr := s.pool.Begin(ctx)
				if failErr != nil {
					return PayrollRun{}, cause
				}
				defer func() { _ = txFail.Rollback(context.Background()) }()

				if _, failErr := txFail.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); failErr != nil {
					return PayrollRun{}, cause
				}

				var eventIDFail string
				if failErr := txFail.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventIDFail); failErr != nil {
					return PayrollRun{}, cause
				}
				_, _ = txFail.Exec(ctx, `
	SELECT staffing.submit_payroll_run_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::uuid,
	  'CALC_FAIL',
	  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
	`, eventIDFail, tenantID, runID, payPeriodID, []byte(failPayload), eventIDFail, tenantID)
				_ = txFail.Commit(ctx)
			}

			return PayrollRun{}, cause
		}

		var out PayrollRun
		if err := tx.QueryRow(ctx, `
	SELECT
	  id::text,
	  pay_period_id::text,
	  run_state,
	  COALESCE(calc_started_at::text, '') AS calc_started_at,
	  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
	  COALESCE(finalized_at::text, '') AS finalized_at,
	  created_at::text
	FROM staffing.payroll_runs
	WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, runID).Scan(&out.ID, &out.PayPeriodID, &out.RunState, &out.CalcStartedAt, &out.CalcFinishedAt, &out.FinalizedAt, &out.CreatedAt); err != nil {
			return PayrollRun{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return PayrollRun{}, err
		}
		return out, nil
	}
}

func (s *staffingPGStore) FinalizePayrollRun(ctx context.Context, tenantID string, runID string) (PayrollRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollRun{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollRun{}, err
	}

	runID = strings.TrimSpace(runID)
	if runID == "" {
		return PayrollRun{}, errors.New("run_id is required")
	}

	var payPeriodID string
	if err := tx.QueryRow(ctx, `
SELECT pay_period_id::text
FROM staffing.payroll_runs
WHERE tenant_id = $1::uuid AND id = $2::uuid
`, tenantID, runID).Scan(&payPeriodID); err != nil {
		return PayrollRun{}, err
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return PayrollRun{}, err
	}
	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_payroll_run_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  'FINALIZE',
  '{}'::jsonb,
  $5::text,
  $6::uuid
)
`, eventID, tenantID, runID, payPeriodID, eventID, tenantID); err != nil {
		return PayrollRun{}, err
	}

	var out PayrollRun
	if err := tx.QueryRow(ctx, `
SELECT
  id::text,
  pay_period_id::text,
  run_state,
  COALESCE(calc_started_at::text, '') AS calc_started_at,
  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
  COALESCE(finalized_at::text, '') AS finalized_at,
  created_at::text
FROM staffing.payroll_runs
WHERE tenant_id = $1::uuid AND id = $2::uuid
`, tenantID, runID).Scan(&out.ID, &out.PayPeriodID, &out.RunState, &out.CalcStartedAt, &out.CalcFinishedAt, &out.FinalizedAt, &out.CreatedAt); err != nil {
		return PayrollRun{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollRun{}, err
	}
	return out, nil
}

func (s *staffingPGStore) ListPayslips(ctx context.Context, tenantID string, runID string) ([]Payslip, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, errors.New("run_id is required")
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  id::text,
	  run_id::text,
	  pay_period_id::text,
	  person_uuid::text,
	  assignment_id::text,
	  currency::text,
	  gross_pay::text,
	  net_pay::text,
	  employer_total::text
	FROM staffing.payslips
	WHERE tenant_id = $1::uuid AND run_id = $2::uuid
	ORDER BY person_uuid::text ASC, assignment_id::text ASC, id::text ASC
	`, tenantID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Payslip
	for rows.Next() {
		var p Payslip
		if err := rows.Scan(&p.ID, &p.RunID, &p.PayPeriodID, &p.PersonUUID, &p.AssignmentID, &p.Currency, &p.GrossPay, &p.NetPay, &p.EmployerTotal); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *staffingPGStore) GetPayslip(ctx context.Context, tenantID string, payslipID string) (PayslipDetail, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayslipDetail{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayslipDetail{}, err
	}

	payslipID = strings.TrimSpace(payslipID)
	if payslipID == "" {
		return PayslipDetail{}, errors.New("payslip_id is required")
	}

	var out PayslipDetail
	if err := tx.QueryRow(ctx, `
	SELECT
	  id::text,
	  run_id::text,
	  pay_period_id::text,
	  person_uuid::text,
	  assignment_id::text,
	  currency::text,
	  gross_pay::text,
	  net_pay::text,
	  employer_total::text
	FROM staffing.payslips
	WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, payslipID).Scan(&out.ID, &out.RunID, &out.PayPeriodID, &out.PersonUUID, &out.AssignmentID, &out.Currency, &out.GrossPay, &out.NetPay, &out.EmployerTotal); err != nil {
		return PayslipDetail{}, err
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  id::text,
	  item_code,
	  item_kind,
	  amount::text,
	  meta::text
	FROM staffing.payslip_items
	WHERE tenant_id = $1::uuid AND payslip_id = $2::uuid
	ORDER BY id ASC
	`, tenantID, payslipID)
	if err != nil {
		return PayslipDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item PayslipItem
		var metaText string
		if err := rows.Scan(&item.ID, &item.ItemCode, &item.ItemKind, &item.Amount, &metaText); err != nil {
			return PayslipDetail{}, err
		}
		item.Meta = json.RawMessage(metaText)
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return PayslipDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayslipDetail{}, err
	}
	return out, nil
}

func pgErrorMessage(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		msg := strings.TrimSpace(pgErr.Message)
		if msg != "" {
			return msg
		}
	}
	return "UNKNOWN"
}

type pgRows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}
