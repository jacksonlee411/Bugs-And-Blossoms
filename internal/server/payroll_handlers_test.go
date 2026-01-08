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
