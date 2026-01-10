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
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
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

	listRecalcErr error
	listRecalcOut []PayrollRecalcRequestSummary

	getRecalcErr error
	getRecalcOut PayrollRecalcRequestDetail

	applyRecalcErr error
	applyRecalcOut PayrollRecalcApplication
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

func (s stubPayrollStore) ListPayrollRecalcRequests(_ context.Context, _ string, _ string, _ string) ([]PayrollRecalcRequestSummary, error) {
	if s.listRecalcErr != nil {
		return nil, s.listRecalcErr
	}
	return s.listRecalcOut, nil
}

func (s stubPayrollStore) GetPayrollRecalcRequest(_ context.Context, _ string, _ string) (PayrollRecalcRequestDetail, error) {
	if s.getRecalcErr != nil {
		return PayrollRecalcRequestDetail{}, s.getRecalcErr
	}
	return s.getRecalcOut, nil
}

func (s stubPayrollStore) ApplyPayrollRecalcRequest(_ context.Context, _ string, _ string, _ string, _ string) (PayrollRecalcApplication, error) {
	if s.applyRecalcErr != nil {
		return PayrollRecalcApplication{}, s.applyRecalcErr
	}
	return s.applyRecalcOut, nil
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

	t.Run("get run error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-runs/run1/payslips/ps1?as_of=2026-01-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayslipDetail(rec, req, stubPayrollStore{getRunErr: errors.New("get_run_failed")})
		if !strings.Contains(rec.Body.String(), "get_run_failed") {
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

func TestToString(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("got=%q", got)
	}
	if got := toString("x"); got != "x" {
		t.Fatalf("got=%q", got)
	}
	if got := toString(float64(12.5)); got != "12.5" {
		t.Fatalf("got=%q", got)
	}
	if got := toString(true); got != "true" {
		t.Fatalf("got=%q", got)
	}
	if got := toString(map[string]any{"a": 1}); !strings.Contains(got, "\"a\"") {
		t.Fatalf("got=%q", got)
	}
	if got := toString(make(chan int)); got != "" {
		t.Fatalf("got=%q", got)
	}
}

func TestRenderPayslipDetail_NetGuaranteedIIT(t *testing.T) {
	html := renderPayslipDetail(
		"run1",
		"ps1",
		"2026-01-01",
		PayrollRun{RunState: "calculated", NeedsRecalc: true},
		PayslipDetail{
			Payslip: Payslip{
				ID:            "ps1",
				RunID:         "run1",
				PersonUUID:    "person1",
				AssignmentID:  "asmt1",
				Currency:      "CNY",
				GrossPay:      "100.00",
				NetPay:        "100.00",
				EmployerTotal: "0.00",
			},
			ItemInputs: []PayslipItemInput{{
				ID:          "in1",
				ItemCode:    "EARNING_LONG_SERVICE_AWARD",
				ItemKind:    "earning",
				Currency:    "CNY",
				CalcMode:    "net_guaranteed_iit",
				TaxBearer:   "employer",
				Amount:      "20000.00",
				LastEventID: "evt1",
				UpdatedAt:   "2026-01-01T00:00:00Z",
			}},
			Items: []PayslipItem{{
				ID:        "it1",
				ItemCode:  "EARNING_LONG_SERVICE_AWARD",
				ItemKind:  "earning",
				Amount:    "25000.00",
				CalcMode:  "net_guaranteed_iit",
				TaxBearer: "employer",
				TargetNet: "20000.00",
				IITDelta:  "5000.00",
				Meta: json.RawMessage(`{
					"tax_year": 2026,
					"tax_month": 1,
					"group_target_net": 20000,
					"group_solved_gross": 25000,
					"group_delta_iit": 5000,
					"base_income": 10000,
					"base_iit_withhold": 100,
					"iterations": 12
				}`),
			}},
		},
		"",
	)
	if !strings.Contains(html, "Net Guaranteed IIT") {
		t.Fatalf("html=%s", html)
	}
	if !strings.Contains(html, "needs_recalc=true") {
		t.Fatalf("html=%s", html)
	}
	if !strings.Contains(html, "EARNING_LONG_SERVICE_AWARD") {
		t.Fatalf("html=%s", html)
	}
	if !strings.Contains(html, "explain:") {
		t.Fatalf("html=%s", html)
	}
}

func TestRenderPayslipDetail_FinalizedAndBadMeta(t *testing.T) {
	html := renderPayslipDetail(
		"run1",
		"ps1",
		"",
		PayrollRun{RunState: "finalized"},
		PayslipDetail{
			Payslip: Payslip{
				ID:            "ps1",
				RunID:         "run1",
				PersonUUID:    "person1",
				AssignmentID:  "asmt1",
				Currency:      "CNY",
				GrossPay:      "100.00",
				NetPay:        "100.00",
				EmployerTotal: "0.00",
			},
			Items: []PayslipItem{{
				ID:        "it1",
				ItemCode:  "EARNING_LONG_SERVICE_AWARD",
				ItemKind:  "earning",
				Amount:    "25000.00",
				CalcMode:  "net_guaranteed_iit",
				TaxBearer: "employer",
				TargetNet: "20000.00",
				IITDelta:  "5000.00",
				Meta:      json.RawMessage(`{`),
			}},
		},
		"",
	)
	if !strings.Contains(html, "(finalized: read-only)") {
		t.Fatalf("html=%s", html)
	}
	if strings.Contains(html, "explain:") {
		t.Fatalf("html=%s", html)
	}
}

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

func TestSelectedAttr(t *testing.T) {
	if got := selectedAttr(true); got != " selected" {
		t.Fatalf("got=%q", got)
	}
	if got := selectedAttr(false); got != "" {
		t.Fatalf("got=%q", got)
	}
}

func TestStablePgMessage(t *testing.T) {
	if got := stablePgMessage(&pgconn.PgError{Message: "STAFFING_X"}); got != "STAFFING_X" {
		t.Fatalf("got=%q", got)
	}
	if got := stablePgMessage(errors.New("boom")); got != "boom" {
		t.Fatalf("got=%q", got)
	}
}

func TestWritePageWithStatus_NonHX(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app?as_of=2026-02-01", nil)
	writePageWithStatus(rec, req, http.StatusTeapot, "<p>hi</p>")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hi") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestRequireFirstSegmentFromPath_TrimmedEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/%20/extra", nil)
	_, ok := requireFirstSegmentFromPath(rec, req, "/org/api/payroll-recalc-requests/", routing.RouteClassInternalAPI)
	if ok {
		t.Fatal("expected ok=false")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandlePayrollRecalcRequests(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests", nil)
		handlePayrollRecalcRequests(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequests(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("list error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests?as_of=2026-02-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequests(rec, req, stubPayrollStore{listRecalcErr: errors.New("list_failed")})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "list_failed") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests?as_of=2026-02-01&state=pending", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequests(rec, req, stubPayrollStore{
			listRecalcOut: []PayrollRecalcRequestSummary{{RecalcRequestID: "rr1", CreatedAt: "2026-02-01T00:00:00Z"}},
		})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "rr1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandlePayrollRecalcRequestDetail(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests/rr1", nil)
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("prefix mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-request/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests/rr1?as_of=2026-02-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{getRecalcErr: errors.New("get_failed")})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "get_failed") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests/rr1?as_of=2026-02-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{getRecalcOut: PayrollRecalcRequestDetail{RecalcRequestID: "rr1"}})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "target_run_id") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ok (applied)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests/rr1?as_of=2026-02-01", nil)
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestDetail(rec, req, stubPayrollStore{getRecalcOut: PayrollRecalcRequestDetail{
			RecalcRequestID: "rr1",
			Application:     &PayrollRecalcApplication{ApplicationID: "1"},
		}})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "application_id") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandlePayrollRecalcRequestApply(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply", nil)
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/payroll-recalc-requests/rr1/apply", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad form", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply?as_of=2026-02-01", strings.NewReader("%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{getRecalcOut: PayrollRecalcRequestDetail{RecalcRequestID: "rr1"}})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply", strings.NewReader("target_run_id=run2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("target_run_id missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply", strings.NewReader("target_run_id="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{getRecalcOut: PayrollRecalcRequestDetail{RecalcRequestID: "rr1"}})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "target_run_id is required") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("apply error (conflict)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply", strings.NewReader("target_run_id=run2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{
			getRecalcOut:   PayrollRecalcRequestDetail{RecalcRequestID: "rr1"},
			applyRecalcErr: &pgconn.PgError{Message: "STAFFING_PAYROLL_RECALC_ALREADY_APPLIED"},
		})
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "STAFFING_PAYROLL_RECALC_ALREADY_APPLIED") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("apply error (not found)", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply", strings.NewReader("target_run_id=run2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{
			getRecalcOut:   PayrollRecalcRequestDetail{RecalcRequestID: "rr1"},
			applyRecalcErr: &pgconn.PgError{Message: "STAFFING_PAYROLL_RECALC_REQUEST_NOT_FOUND"},
		})
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "STAFFING_PAYROLL_RECALC_REQUEST_NOT_FOUND") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("prefix mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-request/rr1/apply", strings.NewReader("target_run_id=run2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/payroll-recalc-requests/rr1/apply?as_of=2026-02-01", strings.NewReader("target_run_id=run2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestApply(rec, req, stubPayrollStore{applyRecalcOut: PayrollRecalcApplication{RecalcRequestID: "rr1"}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if rec.Header().Get("Location") != "/org/payroll-recalc-requests/rr1?as_of=2026-02-01" {
			t.Fatalf("location=%q", rec.Header().Get("Location"))
		}
	})
}

func TestHandlePayrollRecalcRequestsAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests", nil)
		handlePayrollRecalcRequestsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestsAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestsAPI(rec, req, stubPayrollStore{listRecalcErr: errors.New("bad")})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestsAPI(rec, req, stubPayrollStore{listRecalcOut: []PayrollRecalcRequestSummary{{RecalcRequestID: "rr1"}}})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "rr1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandlePayrollRecalcRequestAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/rr1", nil)
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("prefix mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-request/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get :apply not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/rr1:apply", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{getRecalcErr: pgx.ErrNoRows})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get bad request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{getRecalcErr: &pgconn.PgError{Code: "22P02"}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{getRecalcOut: PayrollRecalcRequestDetail{RecalcRequestID: "rr1"}})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "rr1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get failed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/org/api/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{getRecalcErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post without :apply not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post target_run_id missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{applyRecalcErr: &pgconn.PgError{Message: "STAFFING_PAYROLL_RECALC_REQUEST_NOT_FOUND"}})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post already applied", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{applyRecalcErr: &pgconn.PgError{Message: "STAFFING_PAYROLL_RECALC_ALREADY_APPLIED"}})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post staffing error -> 422", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{applyRecalcErr: &pgconn.PgError{Message: "STAFFING_PAYROLL_RECALC_TARGET_RUN_NOT_EDITABLE"}})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad request -> 400", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{applyRecalcErr: &pgconn.PgError{Code: "22P02"}})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post failed -> 500", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{applyRecalcErr: errors.New("boom")})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post ok", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/org/api/payroll-recalc-requests/rr1:apply/extra", strings.NewReader(`{"target_run_id":"run2"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{applyRecalcOut: PayrollRecalcApplication{RecalcRequestID: "rr1", TargetRunID: "run2", TargetPayPeriodID: "pp2"}})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "target_pay_period_id") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/org/api/payroll-recalc-requests/rr1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		handlePayrollRecalcRequestAPI(rec, req, stubPayrollStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestRenderPayrollRecalcRequestDetail(t *testing.T) {
	t.Run("render error", func(t *testing.T) {
		out := renderPayrollRecalcRequestDetail("rr1", "2026-02-01", PayrollRecalcRequestDetail{}, PayrollRecalcApplication{}, "boom")
		if !strings.Contains(out, "boom") {
			t.Fatalf("out=%s", out)
		}
	})

	t.Run("render pending + apply form", func(t *testing.T) {
		out := renderPayrollRecalcRequestDetail("rr1", "2026-02-01", PayrollRecalcRequestDetail{RecalcRequestID: "rr1"}, PayrollRecalcApplication{}, "")
		if !strings.Contains(out, "Apply") || !strings.Contains(out, "target_run_id") {
			t.Fatalf("out=%s", out)
		}
	})

	t.Run("render applied + adjustments", func(t *testing.T) {
		out := renderPayrollRecalcRequestDetail(
			"rr1",
			"",
			PayrollRecalcRequestDetail{
				RecalcRequestID: "rr1",
				AdjustmentsSummary: []PayrollRecalcAdjustmentSummary{{
					ItemKind: "earning",
					ItemCode: "EARNING_BASE_SALARY",
					Amount:   "100.00",
				}},
			},
			PayrollRecalcApplication{
				ApplicationID:     "1",
				EventID:           "e1",
				TargetRunID:       "run2",
				TargetPayPeriodID: "pp2",
				CreatedAt:         "2026-02-02T00:00:00Z",
			},
			"",
		)
		if !strings.Contains(out, "Application") || !strings.Contains(out, "Adjustments Summary") || !strings.Contains(out, "EARNING_BASE_SALARY") {
			t.Fatalf("out=%s", out)
		}
	})
}
