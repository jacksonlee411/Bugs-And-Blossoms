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

type stubPayrollStore struct {
	listPayPeriodsErr     error
	listPayPeriodsPeriods []PayPeriod

	createPayPeriodErr error
	createPayPeriodOut PayPeriod

	listRunsErr  error
	listRunsRuns []PayrollRun

	createRunErr error
	createRunOut PayrollRun

	getRunErr error
	getRunOut PayrollRun

	calcErr error
	calcOut PayrollRun

	finalizeErr error
	finalizeOut PayrollRun

	listPayslipsErr error
	listPayslipsOut []Payslip

	getPayslipErr error
	getPayslipOut PayslipDetail

	getBalancesErr error
	getBalancesOut PayrollBalances

	upsertIITSADErr error
	upsertIITSADOut PayrollIITSADUpsertResult

	listSIVersionsErr error
	listSIVersionsOut []SocialInsurancePolicyVersion

	upsertSIErr error
	upsertSIOut SocialInsurancePolicyUpsertResult
}

func (s stubPayrollStore) ListPayPeriods(_ context.Context, _ string, _ string) ([]PayPeriod, error) {
	if s.listPayPeriodsErr != nil {
		return nil, s.listPayPeriodsErr
	}
	return s.listPayPeriodsPeriods, nil
}
func (s stubPayrollStore) CreatePayPeriod(_ context.Context, _ string, _ string, _ string, _ string) (PayPeriod, error) {
	if s.createPayPeriodErr != nil {
		return PayPeriod{}, s.createPayPeriodErr
	}
	return s.createPayPeriodOut, nil
}
func (s stubPayrollStore) ListPayrollRuns(_ context.Context, _ string, _ string) ([]PayrollRun, error) {
	if s.listRunsErr != nil {
		return nil, s.listRunsErr
	}
	return s.listRunsRuns, nil
}
func (s stubPayrollStore) CreatePayrollRun(_ context.Context, _ string, _ string) (PayrollRun, error) {
	if s.createRunErr != nil {
		return PayrollRun{}, s.createRunErr
	}
	return s.createRunOut, nil
}
func (s stubPayrollStore) GetPayrollRun(_ context.Context, _ string, _ string) (PayrollRun, error) {
	if s.getRunErr != nil {
		return PayrollRun{}, s.getRunErr
	}
	return s.getRunOut, nil
}
func (s stubPayrollStore) CalculatePayrollRun(_ context.Context, _ string, _ string) (PayrollRun, error) {
	if s.calcErr != nil {
		return PayrollRun{}, s.calcErr
	}
	return s.calcOut, nil
}
func (s stubPayrollStore) FinalizePayrollRun(_ context.Context, _ string, _ string) (PayrollRun, error) {
	if s.finalizeErr != nil {
		return PayrollRun{}, s.finalizeErr
	}
	return s.finalizeOut, nil
}

func (s stubPayrollStore) ListPayslips(_ context.Context, _ string, _ string) ([]Payslip, error) {
	if s.listPayslipsErr != nil {
		return nil, s.listPayslipsErr
	}
	return s.listPayslipsOut, nil
}

func (s stubPayrollStore) GetPayslip(_ context.Context, _ string, _ string) (PayslipDetail, error) {
	if s.getPayslipErr != nil {
		return PayslipDetail{}, s.getPayslipErr
	}
	return s.getPayslipOut, nil
}

func (s stubPayrollStore) GetPayrollBalances(_ context.Context, _ string, _ string, _ int) (PayrollBalances, error) {
	if s.getBalancesErr != nil {
		return PayrollBalances{}, s.getBalancesErr
	}
	return s.getBalancesOut, nil
}

func (s stubPayrollStore) UpsertPayrollIITSAD(_ context.Context, _ string, _ PayrollIITSADUpsertInput) (PayrollIITSADUpsertResult, error) {
	if s.upsertIITSADErr != nil {
		return PayrollIITSADUpsertResult{}, s.upsertIITSADErr
	}
	return s.upsertIITSADOut, nil
}

func (s stubPayrollStore) ListSocialInsurancePolicyVersions(_ context.Context, _ string, _ string) ([]SocialInsurancePolicyVersion, error) {
	if s.listSIVersionsErr != nil {
		return nil, s.listSIVersionsErr
	}
	return s.listSIVersionsOut, nil
}

func (s stubPayrollStore) UpsertSocialInsurancePolicyVersion(_ context.Context, _ string, _ SocialInsurancePolicyUpsertInput) (SocialInsurancePolicyUpsertResult, error) {
	if s.upsertSIErr != nil {
		return SocialInsurancePolicyUpsertResult{}, s.upsertSIErr
	}
	return s.upsertSIOut, nil
}

func TestHandlePayrollPeriods(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-periods", nil)
		handlePayrollPeriods(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get list error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-periods?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{listPayPeriodsErr: errors.New("list")})
		if !strings.Contains(rec.Body.String(), "list") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("get ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-periods?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{listPayPeriodsPeriods: []PayPeriod{{ID: "pp1", PayGroup: "monthly", StartDate: "2026-01-01", EndDateExclusive: "2026-02-01", Status: "open"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "pp1") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post bad form", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-periods", strings.NewReader("%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post create error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-periods?as_of=2026-01-01", strings.NewReader("pay_group=monthly&start_date=2026-01-01&end_date_exclusive=2026-02-01"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{createPayPeriodErr: errors.New("create"), listPayPeriodsPeriods: []PayPeriod{{ID: "pp1"}}})
		if !strings.Contains(rec.Body.String(), "create") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post redirect (with as_of)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-periods?as_of=2026-01-01", strings.NewReader("pay_group=monthly&start_date=2026-01-01&end_date_exclusive=2026-02-01"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{createPayPeriodOut: PayPeriod{ID: "pp1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/org/payroll-periods?as_of=2026-01-01" {
			t.Fatalf("loc=%s", loc)
		}
	})

	t.Run("post redirect (no as_of)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-periods", strings.NewReader("pay_group=monthly&start_date=2026-01-01&end_date_exclusive=2026-02-01"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{createPayPeriodOut: PayPeriod{ID: "pp1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/org/payroll-periods" {
			t.Fatalf("loc=%s", loc)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/payroll-periods", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriods(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandlePayrollRuns(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs", nil)
		handlePayrollRuns(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get periods error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{listPayPeriodsErr: errors.New("periods")})
		if !strings.Contains(rec.Body.String(), "periods") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("get runs error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs?as_of=2026-01-01&pay_period_id=pp1", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{listRunsErr: errors.New("runs"), listPayPeriodsPeriods: []PayPeriod{{ID: "pp1"}}})
		if !strings.Contains(rec.Body.String(), "runs") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("get ok empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{listPayPeriodsPeriods: []PayPeriod{{ID: "pp1"}}, listRunsRuns: nil})
		if !strings.Contains(rec.Body.String(), "(empty)") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post bad form", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs", strings.NewReader("%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post create error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs?as_of=2026-01-01", strings.NewReader("pay_period_id=pp1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{
			createRunErr:          errors.New("create"),
			listPayPeriodsPeriods: []PayPeriod{{ID: "pp1"}},
			listRunsRuns:          []PayrollRun{{ID: "run1"}},
		})
		if !strings.Contains(rec.Body.String(), "create") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs?as_of=2026-01-01", strings.NewReader("pay_period_id=pp1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{createRunOut: PayrollRun{ID: "run1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/org/payroll-runs/run1?as_of=2026-01-01" {
			t.Fatalf("loc=%s", loc)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/payroll-runs", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRuns(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandlePayrollRunDetailAndActions(t *testing.T) {
	t.Run("detail tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1", nil)
		handlePayrollRunDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail wrong prefix", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/wrong", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("detail get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunDetail(rec, req, stubPayrollStore{getRunErr: errors.New("get")})
		if !strings.Contains(rec.Body.String(), "get") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("detail ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunDetail(rec, req, stubPayrollStore{getRunOut: PayrollRun{ID: "run1", PayPeriodID: "pp1", RunState: "draft"}})
		if !strings.Contains(rec.Body.String(), "pp1") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("calculate method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/calculate?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunCalculate(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("calculate tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs/run1/calculate", nil)
		handlePayrollRunCalculate(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("calculate bad path", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunCalculate(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("calculate error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs/run1/calculate?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunCalculate(rec, req, stubPayrollStore{calcErr: errors.New("calc")})
		if !strings.Contains(rec.Body.String(), "calc") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("calculate redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs/run1/calculate?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunCalculate(rec, req, stubPayrollStore{calcOut: PayrollRun{ID: "run1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("finalize method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/finalize?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunFinalize(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("finalize tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs/run1/finalize", nil)
		handlePayrollRunFinalize(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("finalize bad path", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunFinalize(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("finalize error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs/run1/finalize?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunFinalize(rec, req, stubPayrollStore{finalizeErr: errors.New("finalize")})
		if !strings.Contains(rec.Body.String(), "finalize") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("finalize redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-runs/run1/finalize?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunFinalize(rec, req, stubPayrollStore{finalizeOut: PayrollRun{ID: "run1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandlePayrollInternalAPI(t *testing.T) {
	t.Run("periods tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-periods", nil)
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("periods get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-periods", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{listPayPeriodsErr: errors.New("list")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("periods get ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-periods", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{listPayPeriodsPeriods: []PayPeriod{{ID: "pp1"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "pp1") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("periods post bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-periods", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("periods post create error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		body := `{"pay_group":"monthly","start_date":"2026-01-01","end_date_exclusive":"2026-02-01"}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-periods", strings.NewReader(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{createPayPeriodErr: errors.New("create")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("periods post ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		body := `{"pay_group":"monthly","start_date":"2026-01-01","end_date_exclusive":"2026-02-01"}`
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-periods", strings.NewReader(body))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{createPayPeriodOut: PayPeriod{ID: "pp1"}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("periods method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/api/payroll-periods", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("runs get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-runs", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{listRunsErr: errors.New("list")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("runs tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-runs", nil)
		handlePayrollRunsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("runs get ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-runs", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{listRunsRuns: []PayrollRun{{ID: "run1"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "run1") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("runs post bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-runs", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("runs post create error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-runs", bytes.NewBufferString(`{"pay_period_id":"pp1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{createRunErr: errors.New("create")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("runs post ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-runs", bytes.NewBufferString(`{"pay_period_id":"pp1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{createRunOut: PayrollRun{ID: "run1"}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
		var got PayrollRun
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.ID != "run1" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("runs method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/api/payroll-runs", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("periods post ok json body validate", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-periods", strings.NewReader(`{"pay_group":"monthly","start_date":"2026-01-01","end_date_exclusive":"2026-02-01"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollPeriodsAPI(rec, req, stubPayrollStore{createPayPeriodOut: PayPeriod{ID: "pp1"}})
		var got PayPeriod
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.ID != "pp1" {
			t.Fatalf("got=%#v", got)
		}
	})

	t.Run("runs post ok json decode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-runs", strings.NewReader(`{"pay_period_id":"pp1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRunsAPI(rec, req, stubPayrollStore{createRunOut: PayrollRun{ID: "run1"}})
		var got map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&got)
	})

	t.Run("requireRunIDFromPath empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		_, ok := requireRunIDFromPath(rec, req, "/org/payroll-runs/")
		if ok {
			t.Fatal("expected not ok")
		}
	})

	t.Run("requireRunIDFromPath trimmed empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/x", nil)
		req.URL.Path = "/org/payroll-runs/  /calculate"
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		_, ok := requireRunIDFromPath(rec, req, "/org/payroll-runs/")
		if ok {
			t.Fatal("expected not ok")
		}
	})
}

func TestHandlePayslips(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips", nil)
		handlePayslips(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/payroll-runs/run1/payslips", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslips(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("path missing run_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslips(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslips(rec, req, stubPayrollStore{listPayslipsErr: errors.New("list_failed")})
		if !strings.Contains(rec.Body.String(), "list_failed") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslips(rec, req, stubPayrollStore{
			listPayslipsOut: []Payslip{{ID: "ps1", RunID: "run1", PersonUUID: "u1", AssignmentID: "a1", Currency: "CNY", GrossPay: "100.00", NetPay: "100.00", EmployerTotal: "0.00"}},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "ps1") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})
}

func TestHandlePayrollBalancesAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing person_uuid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing tax_year", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid tax_year", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tax_year out of range", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=1999", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{getBalancesErr: pgx.ErrNoRows})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{getBalancesErr: errors.New("get")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad request (store validation)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{getBalancesErr: newBadRequestError("bad")})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bad request (pg invalid input)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{getBalancesErr: &pgconn.PgError{Code: "22P02", Message: "invalid input syntax"}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-balances?person_uuid=p1&tax_year=2026", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollBalancesAPI(rec, req, stubPayrollStore{
			getBalancesOut: PayrollBalances{TenantID: "t1", PersonUUID: "p1", TaxYear: 2026, FirstTaxMonth: 1, LastTaxMonth: 2},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"tax_year":2026`) {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})
}

func TestHandlePayrollIITSADAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1"}`))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-iit-special-additional-deductions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("idempotency reused", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{upsertIITSADErr: &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "STAFFING_IDEMPOTENCY_REUSED") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("month finalized", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{upsertIITSADErr: &pgconn.PgError{Message: "STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED"}})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bad request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{upsertIITSADErr: newBadRequestError("bad")})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "bad") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("internal error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{upsertIITSADErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("bad request (pg invalid input)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{upsertIITSADErr: &pgconn.PgError{Code: "22P02", Message: "invalid input syntax"}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", strings.NewReader(`{"event_id":"e1","person_uuid":"p1","tax_year":2026,"tax_month":2,"amount":"100.00"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollIITSADAPI(rec, req, stubPayrollStore{upsertIITSADOut: PayrollIITSADUpsertResult{EventID: "e1", PersonUUID: "p1", TaxYear: 2026, TaxMonth: 2, Amount: "100.00", RequestID: "e1"}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var got PayrollIITSADUpsertResult
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.EventID != "e1" || got.RequestID != "e1" {
			t.Fatalf("got=%#v", got)
		}
	})
}

func TestHandlePayslipDetail(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/ps1", nil)
		handlePayslipDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("path missing run_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/payroll-runs/run1/payslips/ps1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("path missing payslip_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("path prefix mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/not-payslips/ps1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/ps1?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{getPayslipErr: errors.New("get_failed")})
		if !strings.Contains(rec.Body.String(), "get_failed") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/ps1?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{
			getPayslipOut: PayslipDetail{
				Payslip: Payslip{ID: "ps1", RunID: "run1", Currency: "CNY", GrossPay: "100.00", NetPay: "100.00", EmployerTotal: "0.00"},
				Items:   []PayslipItem{{ID: "it1", ItemCode: "EARNING_BASE_SALARY", ItemKind: "earning", Amount: "100.00", Meta: json.RawMessage(`{}`)}},
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "EARNING_BASE_SALARY") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("ok with social insurance items", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/ps1?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{
			getPayslipOut: PayslipDetail{
				Payslip:                      Payslip{ID: "ps1", RunID: "run1", Currency: "CNY", GrossPay: "100.00", NetPay: "100.00", EmployerTotal: "0.00"},
				Items:                        []PayslipItem{{ID: "it1", ItemCode: "EARNING_BASE_SALARY", ItemKind: "earning", Amount: "100.00", Meta: json.RawMessage(`{}`)}},
				SocialInsuranceEmployeeTotal: "8.00",
				SocialInsuranceEmployerTotal: "16.00",
				SocialInsuranceItems: []PayslipSocialInsuranceItem{{
					InsuranceType:     "PENSION",
					BaseAmount:        "100.00",
					EmployeeAmount:    "8.00",
					EmployerAmount:    "16.00",
					PolicyEffectiveAt: "2026-01-01",
				}},
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "PENSION") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("payslip_id has extra segments", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/ps1/extra", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{getPayslipOut: PayslipDetail{Payslip: Payslip{ID: "ps1"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

type alwaysErrReader struct{}

func (alwaysErrReader) Read([]byte) (int, error) { return 0, errors.New("read err") }

func TestNewUUIDv4(t *testing.T) {
	prev := uuidRandReader
	t.Cleanup(func() { uuidRandReader = prev })

	uuidRandReader = bytes.NewReader(make([]byte, 16))
	got, err := newUUIDv4()
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "00000000-0000-4000-8000-000000000000" {
		t.Fatalf("got=%q", got)
	}

	uuidRandReader = alwaysErrReader{}
	if _, err := newUUIDv4(); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandlePayrollSocialInsurancePolicies(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get list error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{listSIVersionsErr: errors.New("list")})
		if !strings.Contains(rec.Body.String(), "list") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("get uuid error", func(t *testing.T) {
		prev := uuidRandReader
		t.Cleanup(func() { uuidRandReader = prev })
		uuidRandReader = alwaysErrReader{}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{listSIVersionsOut: []SocialInsurancePolicyVersion{{InsuranceType: "PENSION", CityCode: "CN-310000"}}})
		if !strings.Contains(rec.Body.String(), "read err") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("get ok", func(t *testing.T) {
		prev := uuidRandReader
		t.Cleanup(func() { uuidRandReader = prev })
		uuidRandReader = bytes.NewReader(make([]byte, 16))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{listSIVersionsOut: []SocialInsurancePolicyVersion{{InsuranceType: "PENSION", CityCode: "CN-310000"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "00000000-0000-4000-8000-000000000000") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("get defaults as_of", func(t *testing.T) {
		prev := uuidRandReader
		t.Cleanup(func() { uuidRandReader = prev })
		uuidRandReader = bytes.NewReader(make([]byte, 16))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-social-insurance-policies", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{listSIVersionsOut: []SocialInsurancePolicyVersion{{InsuranceType: "PENSION", CityCode: "CN-310000"}}})
		if !strings.Contains(rec.Body.String(), "As-of:") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post bad form", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post event_id missing uuid error", func(t *testing.T) {
		prev := uuidRandReader
		t.Cleanup(func() { uuidRandReader = prev })
		uuidRandReader = alwaysErrReader{}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("precision=2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "read err") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post event_id missing with rules_config_json object", func(t *testing.T) {
		prev := uuidRandReader
		t.Cleanup(func() { uuidRandReader = prev })
		uuidRandReader = bytes.NewReader(make([]byte, 16))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("precision=2&rules_config_json=%7B%7D"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{upsertSIOut: SocialInsurancePolicyUpsertResult{PolicyID: "p1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post precision invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("event_id=evt1&precision=xx"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "precision invalid") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post rules_config_json invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("event_id=evt1&precision=2&rules_config_json=%7B"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "rules_config_json invalid") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post rules_config_json must be object", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("event_id=evt1&precision=2&rules_config_json=%5B%5D"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if !strings.Contains(rec.Body.String(), "rules_config_json must be object") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post upsert error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("event_id=evt1&precision=2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{upsertSIErr: errors.New("upsert")})
		if !strings.Contains(rec.Body.String(), "upsert") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("post redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-social-insurance-policies?as_of=2026-01-01", strings.NewReader("event_id=evt1&precision=2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{upsertSIOut: SocialInsurancePolicyUpsertResult{PolicyID: "p1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/org/payroll-social-insurance-policies?as_of=2026-01-01" {
			t.Fatalf("loc=%s", loc)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePolicies(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandlePayrollSocialInsurancePoliciesAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get list error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{listSIVersionsErr: errors.New("list")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get ok json decode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{listSIVersionsOut: []SocialInsurancePolicyVersion{{PolicyID: "p1"}}})
		var got []SocialInsurancePolicyVersion
		_ = json.NewDecoder(rec.Body).Decode(&got)
	})

	t.Run("get defaults as_of", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-social-insurance-policies", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{listSIVersionsOut: []SocialInsurancePolicyVersion{{PolicyID: "p1"}}})
		var got []SocialInsurancePolicyVersion
		_ = json.NewDecoder(rec.Body).Decode(&got)
	})

	t.Run("post bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-social-insurance-policies", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post event_id missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-social-insurance-policies", strings.NewReader(`{}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post upsert error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-social-insurance-policies", strings.NewReader(`{"event_id":"evt1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{upsertSIErr: errors.New("upsert")})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post ok json decode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-social-insurance-policies", strings.NewReader(`{"event_id":"evt1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{upsertSIOut: SocialInsurancePolicyUpsertResult{PolicyID: "p1"}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
		var got SocialInsurancePolicyUpsertResult
		_ = json.NewDecoder(rec.Body).Decode(&got)
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/api/payroll-social-insurance-policies?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollSocialInsurancePoliciesAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestRenderPayrollSocialInsurancePolicies(t *testing.T) {
	html1 := renderPayrollSocialInsurancePolicies(nil, "", "", "")
	if !strings.Contains(html1, "(missing)") {
		t.Fatalf("html=%s", html1)
	}

	html2 := renderPayrollSocialInsurancePolicies([]SocialInsurancePolicyVersion{{
		PolicyID:      "p1",
		CityCode:      "CN-310000",
		HukouType:     "default",
		InsuranceType: "PENSION",
		EffectiveDate: "2026-01-01",
		EmployerRate:  "0.16",
		EmployeeRate:  "0.08",
		BaseFloor:     "0.00",
		BaseCeiling:   "99999.99",
		RoundingRule:  "HALF_UP",
		Precision:     2,
	}}, "2026-01-01", "evt1", "oops")
	if !strings.Contains(html2, "oops") || !strings.Contains(html2, "CN-310000") || !strings.Contains(html2, "evt1") {
		t.Fatalf("html=%s", html2)
	}
}

func TestHandlePayslipsAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips?run_id=run1", nil)
		handlePayslipsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/api/payslips?run_id=run1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("run_id missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips?run_id=run1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipsAPI(rec, req, stubPayrollStore{listPayslipsErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("ok json decode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips?run_id=run1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipsAPI(rec, req, stubPayrollStore{listPayslipsOut: []Payslip{{ID: "ps1", RunID: "run1"}}})
		var got []Payslip
		_ = json.NewDecoder(rec.Body).Decode(&got)
	})
}

func TestHandlePayslipAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips/ps1", nil)
		handlePayslipAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/api/payslips/ps1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("path prefix mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslipz/ps1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("path missing payslip_id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips/ps1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipAPI(rec, req, stubPayrollStore{getPayslipErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("ok json decode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips/ps1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipAPI(rec, req, stubPayrollStore{getPayslipOut: PayslipDetail{Payslip: Payslip{ID: "ps1"}, Items: []PayslipItem{{ID: "it1", Meta: json.RawMessage(`{}`)}}}})
		var got PayslipDetail
		_ = json.NewDecoder(rec.Body).Decode(&got)
	})

	t.Run("payslip_id has extra segments", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payslips/ps1/extra", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipAPI(rec, req, stubPayrollStore{getPayslipOut: PayslipDetail{Payslip: Payslip{ID: "ps1"}}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
