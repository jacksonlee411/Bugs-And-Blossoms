package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	NeedsRecalc    bool   `json:"needs_recalc"`
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
	ID       string `json:"id"`
	ItemCode string `json:"item_code"`
	ItemKind string `json:"item_kind"`
	Amount   string `json:"amount"`

	CalcMode  string `json:"calc_mode"`
	TaxBearer string `json:"tax_bearer"`
	TargetNet string `json:"target_net"`
	IITDelta  string `json:"iit_delta"`

	Meta json.RawMessage `json:"meta"`
}

type PayslipItemInput struct {
	ID          string `json:"id"`
	ItemCode    string `json:"item_code"`
	ItemKind    string `json:"item_kind"`
	Currency    string `json:"currency"`
	CalcMode    string `json:"calc_mode"`
	TaxBearer   string `json:"tax_bearer"`
	Amount      string `json:"amount"`
	LastEventID string `json:"last_event_id"`
	UpdatedAt   string `json:"updated_at"`
}

type PayslipSocialInsuranceItem struct {
	InsuranceType     string `json:"insurance_type"`
	BaseAmount        string `json:"base_amount"`
	EmployeeAmount    string `json:"employee_amount"`
	EmployerAmount    string `json:"employer_amount"`
	PolicyEffectiveAt string `json:"policy_effective_date"`
}

type PayslipDetail struct {
	Payslip
	Items []PayslipItem `json:"items"`

	ItemInputs []PayslipItemInput `json:"item_inputs"`

	SocialInsuranceItems         []PayslipSocialInsuranceItem `json:"social_insurance_items"`
	SocialInsuranceEmployeeTotal string                       `json:"social_insurance_employee_total"`
	SocialInsuranceEmployerTotal string                       `json:"social_insurance_employer_total"`
}

type PayrollBalances struct {
	TenantID           string `json:"tenant_id"`
	PersonUUID         string `json:"person_uuid"`
	TaxYear            int    `json:"tax_year"`
	FirstTaxMonth      int    `json:"first_tax_month"`
	LastTaxMonth       int    `json:"last_tax_month"`
	YTDIncome          string `json:"ytd_income"`
	YTDSpecialDeduct   string `json:"ytd_special_deduction"`
	YTDStandardDeduct  string `json:"ytd_standard_deduction"`
	YTDTaxableIncome   string `json:"ytd_taxable_income"`
	YTDIITTaxLiability string `json:"ytd_iit_tax_liability"`
	YTDIITWithheld     string `json:"ytd_iit_withheld"`
	YTDIITCredit       string `json:"ytd_iit_credit"`
}

type PayrollRecalcRequestSummary struct {
	RecalcRequestID string `json:"recalc_request_id"`
	PersonUUID      string `json:"person_uuid"`
	AssignmentID    string `json:"assignment_id"`
	EffectiveDate   string `json:"effective_date"`
	HitPayPeriodID  string `json:"hit_pay_period_id"`
	CreatedAt       string `json:"created_at"`
	Applied         bool   `json:"applied"`
}

type PayrollRecalcApplication struct {
	ApplicationID     string `json:"application_id"`
	EventID           string `json:"event_id"`
	RecalcRequestID   string `json:"recalc_request_id"`
	TargetRunID       string `json:"target_run_id"`
	TargetPayPeriodID string `json:"target_pay_period_id"`
	CreatedAt         string `json:"created_at"`
}

type PayrollRecalcAdjustmentSummary struct {
	ItemKind string `json:"item_kind"`
	ItemCode string `json:"item_code"`
	Amount   string `json:"amount"`
}

type PayrollRecalcRequestDetail struct {
	RecalcRequestID string `json:"recalc_request_id"`

	TriggerEventID  string `json:"trigger_event_id"`
	TriggerSource   string `json:"trigger_source"`
	RequestID       string `json:"request_id"`
	InitiatorID     string `json:"initiator_id"`
	TransactionTime string `json:"transaction_time"`
	CreatedAt       string `json:"created_at"`

	PersonUUID    string `json:"person_uuid"`
	AssignmentID  string `json:"assignment_id"`
	EffectiveDate string `json:"effective_date"`

	HitPayPeriodID string `json:"hit_pay_period_id"`
	HitRunID       string `json:"hit_run_id"`
	HitPayslipID   string `json:"hit_payslip_id"`

	Application        *PayrollRecalcApplication        `json:"application,omitempty"`
	AdjustmentsSummary []PayrollRecalcAdjustmentSummary `json:"adjustments_summary,omitempty"`
}

type PayrollIITSADUpsertInput struct {
	EventID     string `json:"event_id"`
	PersonUUID  string `json:"person_uuid"`
	TaxYear     int    `json:"tax_year"`
	TaxMonth    int    `json:"tax_month"`
	Amount      string `json:"amount"`
	RequestID   string `json:"request_id"`
	RequestIDIn string `json:"-"`
}

type PayrollIITSADUpsertResult struct {
	EventID    string `json:"event_id"`
	PersonUUID string `json:"person_uuid"`
	TaxYear    int    `json:"tax_year"`
	TaxMonth   int    `json:"tax_month"`
	Amount     string `json:"amount"`
	RequestID  string `json:"request_id"`
}

type badRequestError struct {
	msg string
}

func (e *badRequestError) Error() string { return e.msg }

func newBadRequestError(msg string) error { return &badRequestError{msg: msg} }

func isBadRequestError(err error) bool {
	var badReq *badRequestError
	return errors.As(err, &badReq) && badReq != nil
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

	ListPayrollRecalcRequests(ctx context.Context, tenantID string, personUUID string, state string) ([]PayrollRecalcRequestSummary, error)
	GetPayrollRecalcRequest(ctx context.Context, tenantID string, recalcRequestID string) (PayrollRecalcRequestDetail, error)
	ApplyPayrollRecalcRequest(ctx context.Context, tenantID string, initiatorID string, recalcRequestID string, targetRunID string) (PayrollRecalcApplication, error)

	GetPayrollBalances(ctx context.Context, tenantID string, personUUID string, taxYear int) (PayrollBalances, error)
	UpsertPayrollIITSAD(ctx context.Context, tenantID string, in PayrollIITSADUpsertInput) (PayrollIITSADUpsertResult, error)

	ListSocialInsurancePolicyVersions(ctx context.Context, tenantID string, asOfDate string) ([]SocialInsurancePolicyVersion, error)
	UpsertSocialInsurancePolicyVersion(ctx context.Context, tenantID string, in SocialInsurancePolicyUpsertInput) (SocialInsurancePolicyUpsertResult, error)
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

type SocialInsurancePolicyVersion struct {
	PolicyID      string `json:"policy_id"`
	CityCode      string `json:"city_code"`
	HukouType     string `json:"hukou_type"`
	InsuranceType string `json:"insurance_type"`
	EffectiveDate string `json:"effective_date"`
	EmployerRate  string `json:"employer_rate"`
	EmployeeRate  string `json:"employee_rate"`
	BaseFloor     string `json:"base_floor"`
	BaseCeiling   string `json:"base_ceiling"`
	RoundingRule  string `json:"rounding_rule"`
	Precision     int    `json:"precision"`
}

type SocialInsurancePolicyUpsertInput struct {
	EventID       string          `json:"event_id"`
	CityCode      string          `json:"city_code"`
	HukouType     string          `json:"hukou_type"`
	InsuranceType string          `json:"insurance_type"`
	EffectiveDate string          `json:"effective_date"`
	EmployerRate  string          `json:"employer_rate"`
	EmployeeRate  string          `json:"employee_rate"`
	BaseFloor     string          `json:"base_floor"`
	BaseCeiling   string          `json:"base_ceiling"`
	RoundingRule  string          `json:"rounding_rule"`
	Precision     int             `json:"precision"`
	RulesConfig   json.RawMessage `json:"rules_config"`
}

type SocialInsurancePolicyUpsertResult struct {
	PolicyID      string `json:"policy_id"`
	LastEventDBID int64  `json:"last_event_db_id"`
	InsuranceType string `json:"insurance_type"`
	EffectiveDate string `json:"effective_date"`
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
	  needs_recalc,
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
	  needs_recalc,
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
		if err := rows.Scan(&r.ID, &r.PayPeriodID, &r.RunState, &r.NeedsRecalc, &r.CalcStartedAt, &r.CalcFinishedAt, &r.FinalizedAt, &r.CreatedAt); err != nil {
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
		NeedsRecalc: false,
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
	  needs_recalc,
	  COALESCE(calc_started_at::text, '') AS calc_started_at,
	  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
	  COALESCE(finalized_at::text, '') AS finalized_at,
	  created_at::text
	FROM staffing.payroll_runs
	WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, runID).Scan(&out.ID, &out.PayPeriodID, &out.RunState, &out.NeedsRecalc, &out.CalcStartedAt, &out.CalcFinishedAt, &out.FinalizedAt, &out.CreatedAt); err != nil {
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
		  needs_recalc,
		  COALESCE(calc_started_at::text, '') AS calc_started_at,
		  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
		  COALESCE(finalized_at::text, '') AS finalized_at,
		  created_at::text
		FROM staffing.payroll_runs
		WHERE tenant_id = $1::uuid AND id = $2::uuid
		`, tenantID, runID).Scan(&out.ID, &out.PayPeriodID, &out.RunState, &out.NeedsRecalc, &out.CalcStartedAt, &out.CalcFinishedAt, &out.FinalizedAt, &out.CreatedAt); err != nil {
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
	  needs_recalc,
	  COALESCE(calc_started_at::text, '') AS calc_started_at,
	  COALESCE(calc_finished_at::text, '') AS calc_finished_at,
	  COALESCE(finalized_at::text, '') AS finalized_at,
	  created_at::text
	FROM staffing.payroll_runs
	WHERE tenant_id = $1::uuid AND id = $2::uuid
	`, tenantID, runID).Scan(&out.ID, &out.PayPeriodID, &out.RunState, &out.NeedsRecalc, &out.CalcStartedAt, &out.CalcFinishedAt, &out.FinalizedAt, &out.CreatedAt); err != nil {
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
		  calc_mode,
		  tax_bearer,
		  COALESCE(target_net::text, '') AS target_net,
		  COALESCE(iit_delta::text, '') AS iit_delta,
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
		if err := rows.Scan(&item.ID, &item.ItemCode, &item.ItemKind, &item.Amount, &item.CalcMode, &item.TaxBearer, &item.TargetNet, &item.IITDelta, &metaText); err != nil {
			return PayslipDetail{}, err
		}
		item.Meta = json.RawMessage(metaText)
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return PayslipDetail{}, err
	}

	rowsInputs, err := tx.Query(ctx, `
	SELECT
	  id::text,
	  item_code,
	  item_kind,
	  currency::text,
	  calc_mode,
	  tax_bearer,
	  amount::text,
	  last_event_id::text,
	  updated_at::text
	FROM staffing.payslip_item_inputs
	WHERE tenant_id = $1::uuid
	  AND run_id = $2::uuid
	  AND person_uuid = $3::uuid
	  AND assignment_id = $4::uuid
	ORDER BY item_code ASC, id::text ASC
	`, tenantID, out.RunID, out.PersonUUID, out.AssignmentID)
	if err != nil {
		return PayslipDetail{}, err
	}
	defer rowsInputs.Close()

	for rowsInputs.Next() {
		var input PayslipItemInput
		if err := rowsInputs.Scan(&input.ID, &input.ItemCode, &input.ItemKind, &input.Currency, &input.CalcMode, &input.TaxBearer, &input.Amount, &input.LastEventID, &input.UpdatedAt); err != nil {
			return PayslipDetail{}, err
		}
		out.ItemInputs = append(out.ItemInputs, input)
	}
	if err := rowsInputs.Err(); err != nil {
		return PayslipDetail{}, err
	}

	if err := tx.QueryRow(ctx, `
	SELECT
	  COALESCE(sum(employee_amount), 0)::text,
	  COALESCE(sum(employer_amount), 0)::text
	FROM staffing.payslip_social_insurance_items
	WHERE tenant_id = $1::uuid AND payslip_id = $2::uuid
	`, tenantID, payslipID).Scan(&out.SocialInsuranceEmployeeTotal, &out.SocialInsuranceEmployerTotal); err != nil {
		return PayslipDetail{}, err
	}

	rowsSI, err := tx.Query(ctx, `
	SELECT
	  i.insurance_type,
	  i.base_amount::text,
	  i.employee_amount::text,
	  i.employer_amount::text,
	  e.effective_date::text
	FROM staffing.payslip_social_insurance_items i
	JOIN staffing.social_insurance_policy_events e
	  ON e.tenant_id = i.tenant_id AND e.id = i.policy_last_event_id
	WHERE i.tenant_id = $1::uuid AND i.payslip_id = $2::uuid
	ORDER BY i.insurance_type ASC, i.id ASC
	`, tenantID, payslipID)
	if err != nil {
		return PayslipDetail{}, err
	}
	defer rowsSI.Close()

	for rowsSI.Next() {
		var item PayslipSocialInsuranceItem
		if err := rowsSI.Scan(&item.InsuranceType, &item.BaseAmount, &item.EmployeeAmount, &item.EmployerAmount, &item.PolicyEffectiveAt); err != nil {
			return PayslipDetail{}, err
		}
		out.SocialInsuranceItems = append(out.SocialInsuranceItems, item)
	}
	if err := rowsSI.Err(); err != nil {
		return PayslipDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayslipDetail{}, err
	}
	return out, nil
}

func (s *staffingPGStore) ListPayrollRecalcRequests(ctx context.Context, tenantID string, personUUID string, state string) ([]PayrollRecalcRequestSummary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	personUUID = strings.TrimSpace(personUUID)
	state = strings.TrimSpace(state)
	if state != "" && state != "pending" && state != "applied" {
		return nil, errors.New("state must be pending|applied")
	}

	query := `
SELECT
  r.recalc_request_id::text,
  r.person_uuid::text,
  r.assignment_id::text,
  r.effective_date::text,
  r.hit_pay_period_id::text,
  r.created_at::text,
  (a.id IS NOT NULL) AS applied
FROM staffing.payroll_recalc_requests r
LEFT JOIN staffing.payroll_recalc_applications a
  ON a.tenant_id = r.tenant_id AND a.recalc_request_id = r.recalc_request_id
WHERE r.tenant_id = $1::uuid
`
	args := []any{tenantID}

	if personUUID != "" {
		args = append(args, personUUID)
		query += ` AND r.person_uuid = $2::uuid`
	}

	if state == "pending" {
		query += ` AND a.id IS NULL`
	} else if state == "applied" {
		query += ` AND a.id IS NOT NULL`
	}

	query += `
ORDER BY r.created_at DESC, r.id DESC
LIMIT 500
`

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PayrollRecalcRequestSummary
	for rows.Next() {
		var r PayrollRecalcRequestSummary
		if err := rows.Scan(&r.RecalcRequestID, &r.PersonUUID, &r.AssignmentID, &r.EffectiveDate, &r.HitPayPeriodID, &r.CreatedAt, &r.Applied); err != nil {
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

func (s *staffingPGStore) GetPayrollRecalcRequest(ctx context.Context, tenantID string, recalcRequestID string) (PayrollRecalcRequestDetail, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollRecalcRequestDetail{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollRecalcRequestDetail{}, err
	}

	recalcRequestID = strings.TrimSpace(recalcRequestID)
	if recalcRequestID == "" {
		return PayrollRecalcRequestDetail{}, errors.New("recalc_request_id is required")
	}

	var out PayrollRecalcRequestDetail
	if err := tx.QueryRow(ctx, `
SELECT
  recalc_request_id::text,
  trigger_event_id::text,
  trigger_source,
  request_id,
  initiator_id::text,
  transaction_time::text,
  created_at::text,
  person_uuid::text,
  assignment_id::text,
  effective_date::text,
  hit_pay_period_id::text,
  hit_run_id::text,
  COALESCE(hit_payslip_id::text, '') AS hit_payslip_id
FROM staffing.payroll_recalc_requests
WHERE tenant_id = $1::uuid AND recalc_request_id = $2::uuid
`, tenantID, recalcRequestID).Scan(
		&out.RecalcRequestID,
		&out.TriggerEventID,
		&out.TriggerSource,
		&out.RequestID,
		&out.InitiatorID,
		&out.TransactionTime,
		&out.CreatedAt,
		&out.PersonUUID,
		&out.AssignmentID,
		&out.EffectiveDate,
		&out.HitPayPeriodID,
		&out.HitRunID,
		&out.HitPayslipID,
	); err != nil {
		return PayrollRecalcRequestDetail{}, err
	}

	var app PayrollRecalcApplication
	if err := tx.QueryRow(ctx, `
SELECT
  id::text,
  event_id::text,
  recalc_request_id::text,
  target_run_id::text,
  target_pay_period_id::text,
  created_at::text
FROM staffing.payroll_recalc_applications
WHERE tenant_id = $1::uuid AND recalc_request_id = $2::uuid
`, tenantID, recalcRequestID).Scan(
		&app.ApplicationID,
		&app.EventID,
		&app.RecalcRequestID,
		&app.TargetRunID,
		&app.TargetPayPeriodID,
		&app.CreatedAt,
	); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return PayrollRecalcRequestDetail{}, err
		}
	} else {
		out.Application = &app
	}

	rows, err := tx.Query(ctx, `
SELECT
  item_kind,
  item_code,
  COALESCE(sum(amount), 0)::text
FROM staffing.payroll_adjustments
WHERE tenant_id = $1::uuid AND recalc_request_id = $2::uuid
GROUP BY item_kind, item_code
ORDER BY item_kind ASC, item_code ASC
`, tenantID, recalcRequestID)
	if err != nil {
		return PayrollRecalcRequestDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var s PayrollRecalcAdjustmentSummary
		if err := rows.Scan(&s.ItemKind, &s.ItemCode, &s.Amount); err != nil {
			return PayrollRecalcRequestDetail{}, err
		}
		out.AdjustmentsSummary = append(out.AdjustmentsSummary, s)
	}
	if err := rows.Err(); err != nil {
		return PayrollRecalcRequestDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollRecalcRequestDetail{}, err
	}
	return out, nil
}

func (s *staffingPGStore) ApplyPayrollRecalcRequest(ctx context.Context, tenantID string, initiatorID string, recalcRequestID string, targetRunID string) (PayrollRecalcApplication, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollRecalcApplication{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollRecalcApplication{}, err
	}

	initiatorID = strings.TrimSpace(initiatorID)
	if initiatorID == "" {
		return PayrollRecalcApplication{}, errors.New("initiator_id is required")
	}
	recalcRequestID = strings.TrimSpace(recalcRequestID)
	if recalcRequestID == "" {
		return PayrollRecalcApplication{}, errors.New("recalc_request_id is required")
	}
	targetRunID = strings.TrimSpace(targetRunID)
	if targetRunID == "" {
		return PayrollRecalcApplication{}, errors.New("target_run_id is required")
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return PayrollRecalcApplication{}, err
	}
	requestID := eventID

	var applicationDBID int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_payroll_recalc_apply_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::uuid,
  $5::text,
  $6::uuid
)
`, eventID, tenantID, recalcRequestID, targetRunID, requestID, initiatorID).Scan(&applicationDBID); err != nil {
		return PayrollRecalcApplication{}, err
	}
	if applicationDBID <= 0 {
		return PayrollRecalcApplication{}, errors.New("unexpected application_db_id")
	}

	var out PayrollRecalcApplication
	if err := tx.QueryRow(ctx, `
SELECT
  id::text,
  event_id::text,
  recalc_request_id::text,
  target_run_id::text,
  target_pay_period_id::text,
  created_at::text
FROM staffing.payroll_recalc_applications
WHERE tenant_id = $1::uuid AND id = $2
`, tenantID, applicationDBID).Scan(
		&out.ApplicationID,
		&out.EventID,
		&out.RecalcRequestID,
		&out.TargetRunID,
		&out.TargetPayPeriodID,
		&out.CreatedAt,
	); err != nil {
		return PayrollRecalcApplication{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollRecalcApplication{}, err
	}
	return out, nil
}

func (s *staffingPGStore) ListSocialInsurancePolicyVersions(ctx context.Context, tenantID string, asOfDate string) ([]SocialInsurancePolicyVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	asOfDate = strings.TrimSpace(asOfDate)
	if asOfDate == "" {
		return nil, errors.New("as_of is required")
	}
	if _, err := time.Parse("2006-01-02", asOfDate); err != nil {
		return nil, errors.New("as_of invalid: " + err.Error())
	}

	rows, err := tx.Query(ctx, `
SELECT
  policy_id::text,
  city_code,
  hukou_type,
  insurance_type,
  lower(validity)::text AS effective_date,
  employer_rate::text,
  employee_rate::text,
  base_floor::text,
  base_ceiling::text,
  rounding_rule,
  precision
FROM staffing.social_insurance_policy_versions
WHERE tenant_id = $1::uuid
  AND validity @> $2::date
ORDER BY insurance_type ASC, policy_id::text ASC
`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SocialInsurancePolicyVersion
	for rows.Next() {
		var p SocialInsurancePolicyVersion
		if err := rows.Scan(
			&p.PolicyID,
			&p.CityCode,
			&p.HukouType,
			&p.InsuranceType,
			&p.EffectiveDate,
			&p.EmployerRate,
			&p.EmployeeRate,
			&p.BaseFloor,
			&p.BaseCeiling,
			&p.RoundingRule,
			&p.Precision,
		); err != nil {
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

func (s *staffingPGStore) UpsertSocialInsurancePolicyVersion(ctx context.Context, tenantID string, in SocialInsurancePolicyUpsertInput) (SocialInsurancePolicyUpsertResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SocialInsurancePolicyUpsertResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return SocialInsurancePolicyUpsertResult{}, err
	}

	in.EventID = strings.TrimSpace(in.EventID)
	if in.EventID == "" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("event_id is required")
	}

	in.CityCode = strings.TrimSpace(in.CityCode)
	if in.CityCode == "" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("city_code is required")
	}
	in.CityCode = strings.ToUpper(in.CityCode)

	in.HukouType = strings.ToLower(strings.TrimSpace(in.HukouType))
	if in.HukouType == "" {
		in.HukouType = "default"
	}
	if in.HukouType != "default" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("hukou_type not supported")
	}

	in.InsuranceType = strings.ToUpper(strings.TrimSpace(in.InsuranceType))
	if in.InsuranceType == "" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("insurance_type is required")
	}
	switch in.InsuranceType {
	case "PENSION", "MEDICAL", "UNEMPLOYMENT", "INJURY", "MATERNITY", "HOUSING_FUND":
	default:
		return SocialInsurancePolicyUpsertResult{}, errors.New("insurance_type invalid")
	}

	in.EffectiveDate = strings.TrimSpace(in.EffectiveDate)
	if in.EffectiveDate == "" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", in.EffectiveDate); err != nil {
		return SocialInsurancePolicyUpsertResult{}, errors.New("effective_date invalid: " + err.Error())
	}

	in.EmployerRate = strings.TrimSpace(in.EmployerRate)
	in.EmployeeRate = strings.TrimSpace(in.EmployeeRate)
	in.BaseFloor = strings.TrimSpace(in.BaseFloor)
	in.BaseCeiling = strings.TrimSpace(in.BaseCeiling)
	if in.EmployerRate == "" || in.EmployeeRate == "" || in.BaseFloor == "" || in.BaseCeiling == "" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("rates and base are required")
	}

	in.RoundingRule = strings.ToUpper(strings.TrimSpace(in.RoundingRule))
	if in.RoundingRule == "" {
		return SocialInsurancePolicyUpsertResult{}, errors.New("rounding_rule is required")
	}
	switch in.RoundingRule {
	case "HALF_UP", "CEIL":
	default:
		return SocialInsurancePolicyUpsertResult{}, errors.New("rounding_rule invalid")
	}
	if in.Precision < 0 || in.Precision > 2 {
		return SocialInsurancePolicyUpsertResult{}, errors.New("precision invalid")
	}

	rulesConfig := in.RulesConfig
	if len(rulesConfig) == 0 {
		rulesConfig = []byte(`{}`)
	} else {
		var v any
		if err := json.Unmarshal(rulesConfig, &v); err != nil {
			return SocialInsurancePolicyUpsertResult{}, errors.New("rules_config invalid json: " + err.Error())
		}
		if _, ok := v.(map[string]any); !ok {
			return SocialInsurancePolicyUpsertResult{}, errors.New("rules_config must be an object")
		}
	}

	var policyID string
	err = tx.QueryRow(ctx, `
SELECT id::text
FROM staffing.social_insurance_policies
WHERE tenant_id = $1::uuid
  AND city_code = $2::text
  AND hukou_type = $3::text
  AND insurance_type = $4::text
`, tenantID, in.CityCode, in.HukouType, in.InsuranceType).Scan(&policyID)
	if err != nil && err != pgx.ErrNoRows {
		return SocialInsurancePolicyUpsertResult{}, err
	}
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&policyID); err != nil {
			return SocialInsurancePolicyUpsertResult{}, err
		}
	}

	eventType := "CREATE"
	{
		var hasEvent bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM staffing.social_insurance_policy_events
  WHERE tenant_id = $1::uuid AND policy_id = $2::uuid
  LIMIT 1
)
`, tenantID, policyID).Scan(&hasEvent); err != nil {
			return SocialInsurancePolicyUpsertResult{}, err
		}
		if hasEvent {
			eventType = "UPDATE"
		}
	}

	payloadBytes, _ := json.Marshal(map[string]any{
		"employer_rate": in.EmployerRate,
		"employee_rate": in.EmployeeRate,
		"base_floor":    in.BaseFloor,
		"base_ceiling":  in.BaseCeiling,
		"rounding_rule": in.RoundingRule,
		"precision":     in.Precision,
		"rules_config":  json.RawMessage(rulesConfig),
	})

	var eventDBID int64
	if err := tx.QueryRow(ctx, `
	SELECT staffing.submit_social_insurance_policy_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::text,
  $5::text,
  $6::text,
  $7::text,
  $8::date,
  $9::jsonb,
  $10::text,
  $11::uuid
);
`, in.EventID, tenantID, policyID, in.CityCode, in.HukouType, in.InsuranceType, eventType, in.EffectiveDate, payloadBytes, in.EventID, tenantID).Scan(&eventDBID); err != nil {
		return SocialInsurancePolicyUpsertResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SocialInsurancePolicyUpsertResult{}, err
	}
	return SocialInsurancePolicyUpsertResult{
		PolicyID:      policyID,
		LastEventDBID: eventDBID,
		InsuranceType: in.InsuranceType,
		EffectiveDate: in.EffectiveDate,
	}, nil
}

func (s *staffingPGStore) GetPayrollBalances(ctx context.Context, tenantID string, personUUID string, taxYear int) (PayrollBalances, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollBalances{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollBalances{}, err
	}

	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return PayrollBalances{}, newBadRequestError("person_uuid is required")
	}
	if taxYear < 2000 || taxYear > 9999 {
		return PayrollBalances{}, newBadRequestError("tax_year out of range")
	}

	var out PayrollBalances
	if err := tx.QueryRow(ctx, `
	SELECT
	  tenant_id::text,
	  person_uuid::text,
	  tax_year,
	  first_tax_month,
	  last_tax_month,
	  ytd_income::text,
	  ytd_special_deduction::text,
	  ytd_standard_deduction::text,
	  ytd_taxable_income::text,
	  ytd_iit_tax_liability::text,
	  ytd_iit_withheld::text,
	  ytd_iit_credit::text
	FROM staffing.payroll_balances
	WHERE tenant_id = $1::uuid
	  AND tax_entity_id = $1::uuid
	  AND person_uuid = $2::uuid
	  AND tax_year = $3::int
	`, tenantID, personUUID, taxYear).Scan(
		&out.TenantID,
		&out.PersonUUID,
		&out.TaxYear,
		&out.FirstTaxMonth,
		&out.LastTaxMonth,
		&out.YTDIncome,
		&out.YTDSpecialDeduct,
		&out.YTDStandardDeduct,
		&out.YTDTaxableIncome,
		&out.YTDIITTaxLiability,
		&out.YTDIITWithheld,
		&out.YTDIITCredit,
	); err != nil {
		return PayrollBalances{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollBalances{}, err
	}
	return out, nil
}

func (s *staffingPGStore) UpsertPayrollIITSAD(ctx context.Context, tenantID string, in PayrollIITSADUpsertInput) (PayrollIITSADUpsertResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PayrollIITSADUpsertResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return PayrollIITSADUpsertResult{}, err
	}

	in.EventID = strings.TrimSpace(in.EventID)
	in.PersonUUID = strings.TrimSpace(in.PersonUUID)
	in.Amount = strings.TrimSpace(in.Amount)
	in.RequestIDIn = strings.TrimSpace(in.RequestID)

	if in.EventID == "" {
		return PayrollIITSADUpsertResult{}, newBadRequestError("event_id is required")
	}
	if in.PersonUUID == "" {
		return PayrollIITSADUpsertResult{}, newBadRequestError("person_uuid is required")
	}
	if in.TaxYear < 2000 || in.TaxYear > 9999 {
		return PayrollIITSADUpsertResult{}, newBadRequestError("tax_year out of range")
	}
	if in.TaxMonth < 1 || in.TaxMonth > 12 {
		return PayrollIITSADUpsertResult{}, newBadRequestError("tax_month out of range")
	}
	if in.Amount == "" {
		return PayrollIITSADUpsertResult{}, newBadRequestError("amount is required")
	}
	if in.RequestIDIn == "" {
		in.RequestIDIn = in.EventID
	}

	var eventDBID int64
	if err := tx.QueryRow(ctx, `
	SELECT staffing.submit_iit_special_additional_deduction_claim_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  $4::int,
	  $5::smallint,
	  $6::numeric,
	  $7::text,
	  $8::uuid
	);
	`, in.EventID, tenantID, in.PersonUUID, in.TaxYear, in.TaxMonth, in.Amount, in.RequestIDIn, tenantID).Scan(&eventDBID); err != nil {
		return PayrollIITSADUpsertResult{}, err
	}
	if eventDBID <= 0 {
		return PayrollIITSADUpsertResult{}, errors.New("unexpected event_db_id")
	}

	if err := tx.Commit(ctx); err != nil {
		return PayrollIITSADUpsertResult{}, err
	}
	return PayrollIITSADUpsertResult{
		EventID:    in.EventID,
		PersonUUID: in.PersonUUID,
		TaxYear:    in.TaxYear,
		TaxMonth:   in.TaxMonth,
		Amount:     in.Amount,
		RequestID:  in.RequestIDIn,
	}, nil
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

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		return strings.TrimSpace(pgErr.Code)
	}
	return ""
}

func isPgInvalidInput(err error) bool {
	switch pgErrorCode(err) {
	case "22P02", "22003", "22007", "22008":
		return true
	default:
		return false
	}
}

type pgRows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}
