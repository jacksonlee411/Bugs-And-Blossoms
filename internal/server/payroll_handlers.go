package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

var uuidRandReader io.Reader = rand.Reader

func handlePayrollPeriods(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))

	switch r.Method {
	case http.MethodGet:
		payGroup := strings.TrimSpace(r.URL.Query().Get("pay_group"))
		periods, err := store.ListPayPeriods(r.Context(), tenant.ID, payGroup)
		if err != nil {
			writePage(w, r, renderPayrollPeriods(nil, asOf, err.Error()))
			return
		}
		writePage(w, r, renderPayrollPeriods(periods, asOf, ""))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderPayrollPeriods(nil, asOf, "bad form"))
			return
		}
		payGroup := strings.TrimSpace(r.Form.Get("pay_group"))
		startDate := strings.TrimSpace(r.Form.Get("start_date"))
		endDateExclusive := strings.TrimSpace(r.Form.Get("end_date_exclusive"))

		if _, err := store.CreatePayPeriod(r.Context(), tenant.ID, payGroup, startDate, endDateExclusive); err != nil {
			periods, _ := store.ListPayPeriods(r.Context(), tenant.ID, "")
			writePage(w, r, renderPayrollPeriods(periods, asOf, err.Error()))
			return
		}

		loc := "/org/payroll-periods"
		if asOf != "" {
			loc += "?as_of=" + url.QueryEscape(asOf)
		}
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePayrollRuns(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	filterPayPeriodID := strings.TrimSpace(r.URL.Query().Get("pay_period_id"))

	switch r.Method {
	case http.MethodGet:
		periods, err := store.ListPayPeriods(r.Context(), tenant.ID, "")
		if err != nil {
			writePage(w, r, renderPayrollRuns(nil, nil, asOf, filterPayPeriodID, err.Error()))
			return
		}
		runs, err := store.ListPayrollRuns(r.Context(), tenant.ID, filterPayPeriodID)
		if err != nil {
			writePage(w, r, renderPayrollRuns(nil, periods, asOf, filterPayPeriodID, err.Error()))
			return
		}
		writePage(w, r, renderPayrollRuns(runs, periods, asOf, filterPayPeriodID, ""))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderPayrollRuns(nil, nil, asOf, filterPayPeriodID, "bad form"))
			return
		}
		payPeriodID := strings.TrimSpace(r.Form.Get("pay_period_id"))
		run, err := store.CreatePayrollRun(r.Context(), tenant.ID, payPeriodID)
		if err != nil {
			periods, _ := store.ListPayPeriods(r.Context(), tenant.ID, "")
			runs, _ := store.ListPayrollRuns(r.Context(), tenant.ID, filterPayPeriodID)
			writePage(w, r, renderPayrollRuns(runs, periods, asOf, filterPayPeriodID, err.Error()))
			return
		}

		loc := "/org/payroll-runs/" + url.PathEscape(run.ID)
		if asOf != "" {
			loc += "?as_of=" + url.QueryEscape(asOf)
		}
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePayrollRunDetail(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	runID, ok := requireRunIDFromPath(w, r, "/org/payroll-runs/")
	if !ok {
		return
	}

	run, err := store.GetPayrollRun(r.Context(), tenant.ID, runID)
	if err != nil {
		writePage(w, r, renderPayrollRun(runID, asOf, PayrollRun{}, err.Error()))
		return
	}
	writePage(w, r, renderPayrollRun(runID, asOf, run, ""))
}

func handlePayrollRunCalculate(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	runID, ok := requireRunIDFromPath(w, r, "/org/payroll-runs/")
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if _, err := store.CalculatePayrollRun(r.Context(), tenant.ID, runID); err != nil {
		writePage(w, r, renderPayrollRun(runID, asOf, PayrollRun{}, err.Error()))
		return
	}

	loc := "/org/payroll-runs/" + url.PathEscape(runID)
	if asOf != "" {
		loc += "?as_of=" + url.QueryEscape(asOf)
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

func handlePayrollRunFinalize(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	runID, ok := requireRunIDFromPath(w, r, "/org/payroll-runs/")
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if _, err := store.FinalizePayrollRun(r.Context(), tenant.ID, runID); err != nil {
		writePage(w, r, renderPayrollRun(runID, asOf, PayrollRun{}, err.Error()))
		return
	}

	loc := "/org/payroll-runs/" + url.PathEscape(runID)
	if asOf != "" {
		loc += "?as_of=" + url.QueryEscape(asOf)
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

func handlePayrollPeriodsAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		periods, err := store.ListPayPeriods(r.Context(), tenant.ID, strings.TrimSpace(r.URL.Query().Get("pay_group")))
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(periods)
		return
	case http.MethodPost:
		var req struct {
			PayGroup         string `json:"pay_group"`
			StartDate        string `json:"start_date"`
			EndDateExclusive string `json:"end_date_exclusive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		period, err := store.CreatePayPeriod(r.Context(), tenant.ID, req.PayGroup, req.StartDate, req.EndDateExclusive)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "create_failed", "create failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(period)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePayrollRunsAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		runs, err := store.ListPayrollRuns(r.Context(), tenant.ID, strings.TrimSpace(r.URL.Query().Get("pay_period_id")))
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(runs)
		return
	case http.MethodPost:
		var req struct {
			PayPeriodID string `json:"pay_period_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		run, err := store.CreatePayrollRun(r.Context(), tenant.ID, req.PayPeriodID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "create_failed", "create failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(run)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePayrollBalancesAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
	if personUUID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "missing_person_uuid", "person_uuid is required")
		return
	}
	taxYearRaw := strings.TrimSpace(r.URL.Query().Get("tax_year"))
	if taxYearRaw == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "missing_tax_year", "tax_year is required")
		return
	}
	taxYear, err := strconv.Atoi(taxYearRaw)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_tax_year", "invalid tax_year")
		return
	}
	if taxYear < 2000 || taxYear > 9999 {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_tax_year", "tax_year out of range")
		return
	}

	bal, err := store.GetPayrollBalances(r.Context(), tenant.ID, personUUID, taxYear)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
			return
		}
		if isBadRequestError(err) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if isPgInvalidInput(err) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", "bad request")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "get_failed", "get failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(bal)
}

func handlePayrollIITSADAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req PayrollIITSADUpsertInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	res, err := store.UpsertPayrollIITSAD(r.Context(), tenant.ID, req)
	if err != nil {
		switch msg := pgErrorMessage(err); msg {
		case "STAFFING_IDEMPOTENCY_REUSED", "STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED":
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, msg, msg)
			return
		default:
			if isBadRequestError(err) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", err.Error())
				return
			}
			if isPgInvalidInput(err) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", "bad request")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "upsert_failed", "upsert failed")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(res)
}

func handlePayslips(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	runID, ok := requireRunIDFromPath(w, r, "/org/payroll-runs/")
	if !ok {
		return
	}

	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	payslips, err := store.ListPayslips(r.Context(), tenant.ID, runID)
	if err != nil {
		writePage(w, r, renderPayslips(runID, asOf, nil, err.Error()))
		return
	}
	writePage(w, r, renderPayslips(runID, asOf, payslips, ""))
}

func handlePayslipDetail(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	runID, ok := requireRunIDFromPath(w, r, "/org/payroll-runs/")
	if !ok {
		return
	}
	prefix := "/org/payroll-runs/" + runID + "/payslips/"
	path := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(path, prefix) {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return
	}
	payslipID := strings.TrimPrefix(path, prefix)
	payslipID = strings.TrimPrefix(payslipID, "/")
	if strings.Contains(payslipID, "/") {
		payslipID = strings.Split(payslipID, "/")[0]
	}
	payslipID = strings.TrimSpace(payslipID)
	if payslipID == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return
	}

	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	run, err := store.GetPayrollRun(r.Context(), tenant.ID, runID)
	if err != nil {
		writePage(w, r, renderPayslipDetail(runID, payslipID, asOf, PayrollRun{}, PayslipDetail{}, err.Error()))
		return
	}

	p, err := store.GetPayslip(r.Context(), tenant.ID, payslipID)
	if err != nil {
		writePage(w, r, renderPayslipDetail(runID, payslipID, asOf, run, PayslipDetail{}, err.Error()))
		return
	}
	writePage(w, r, renderPayslipDetail(runID, payslipID, asOf, run, p, ""))
}

func handlePayslipNetGuaranteedIITItems(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))

	runID, ok := requireRunIDFromPath(w, r, "/org/payroll-runs/")
	if !ok {
		return
	}
	prefix := "/org/payroll-runs/" + runID + "/payslips/"
	path := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(path, prefix) {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return
	}
	payslipID := strings.TrimPrefix(path, prefix)
	payslipID = strings.TrimPrefix(payslipID, "/")
	if strings.Contains(payslipID, "/") {
		payslipID = strings.Split(payslipID, "/")[0]
	}
	payslipID = strings.TrimSpace(payslipID)
	if payslipID == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		run, _ := store.GetPayrollRun(r.Context(), tenant.ID, runID)
		p, _ := store.GetPayslip(r.Context(), tenant.ID, payslipID)
		writePageWithStatus(w, r, http.StatusBadRequest, renderPayslipDetail(runID, payslipID, asOf, run, p, "bad form"))
		return
	}

	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_missing", "principal missing")
		return
	}

	eventType := strings.TrimSpace(r.Form.Get("event_type"))
	itemCode := strings.TrimSpace(r.Form.Get("item_code"))
	targetNet := strings.TrimSpace(r.Form.Get("target_net"))
	requestID := strings.TrimSpace(r.Form.Get("request_id"))
	if requestID == "" {
		if v, err := newUUIDv4(); err == nil {
			requestID = v
		}
	}

	if err := store.SubmitPayslipNetGuaranteedIITItem(r.Context(), tenant.ID, principal.ID, runID, payslipID, eventType, itemCode, targetNet, requestID); err != nil {
		msg := stablePgMessage(err)
		status := http.StatusInternalServerError

		switch pgErrorMessage(err) {
		case "STAFFING_IDEMPOTENCY_REUSED", "STAFFING_PAYROLL_RUN_FINALIZED_READONLY":
			status = http.StatusConflict
		default:
			if strings.HasPrefix(pgErrorMessage(err), "STAFFING_") {
				status = http.StatusUnprocessableEntity
			}
			if isBadRequestError(err) || isPgInvalidInput(err) {
				status = http.StatusBadRequest
			}
		}

		run, _ := store.GetPayrollRun(r.Context(), tenant.ID, runID)
		p, _ := store.GetPayslip(r.Context(), tenant.ID, payslipID)
		writePageWithStatus(w, r, status, renderPayslipDetail(runID, payslipID, asOf, run, p, msg))
		return
	}

	loc := "/org/payroll-runs/" + url.PathEscape(runID) + "/payslips/" + url.PathEscape(payslipID)
	if asOf != "" {
		loc += "?as_of=" + url.QueryEscape(asOf)
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

func handlePayslipNetGuaranteedIITItemsAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "principal_missing", "principal missing")
		return
	}

	runID, ok := requireFirstSegmentFromPath(w, r, "/org/api/payroll-runs/", routing.RouteClassInternalAPI)
	if !ok {
		return
	}
	prefix := "/org/api/payroll-runs/" + runID + "/payslips/"
	path := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(path, prefix) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
		return
	}
	payslipID := strings.TrimPrefix(path, prefix)
	payslipID = strings.TrimPrefix(payslipID, "/")
	if strings.Contains(payslipID, "/") {
		payslipID = strings.Split(payslipID, "/")[0]
	}
	payslipID = strings.TrimSpace(payslipID)
	if payslipID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
		return
	}

	var req struct {
		EventType string `json:"event_type"`
		ItemCode  string `json:"item_code"`
		TargetNet string `json:"target_net"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.ItemCode = strings.TrimSpace(req.ItemCode)
	req.RequestID = strings.TrimSpace(req.RequestID)

	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		eventType = "UPSERT"
	}
	eventType = strings.ToUpper(eventType)
	if eventType != "UPSERT" && eventType != "DELETE" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "event_type_invalid", "event_type must be UPSERT|DELETE")
		return
	}

	if err := store.SubmitPayslipNetGuaranteedIITItem(r.Context(), tenant.ID, principal.ID, runID, payslipID, eventType, req.ItemCode, req.TargetNet, req.RequestID); err != nil {
		switch msg := pgErrorMessage(err); msg {
		case "STAFFING_IDEMPOTENCY_REUSED", "STAFFING_PAYROLL_RUN_FINALIZED_READONLY":
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, msg, msg)
			return
		default:
			if strings.HasPrefix(msg, "STAFFING_") {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, msg, msg)
				return
			}
			if isBadRequestError(err) || isPgInvalidInput(err) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", "bad request")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "submit_failed", "submit failed")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func handlePayslipsAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if runID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "run_id_missing", "run_id is required")
		return
	}

	payslips, err := store.ListPayslips(r.Context(), tenant.ID, runID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payslips)
}

func handlePayslipAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	path := strings.TrimSpace(r.URL.Path)
	prefix := "/org/api/payslips/"
	if !strings.HasPrefix(path, prefix) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
		return
	}

	payslipID := strings.TrimPrefix(path, prefix)
	payslipID = strings.TrimPrefix(payslipID, "/")
	if strings.Contains(payslipID, "/") {
		payslipID = strings.Split(payslipID, "/")[0]
	}
	payslipID = strings.TrimSpace(payslipID)
	if payslipID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
		return
	}

	payslip, err := store.GetPayslip(r.Context(), tenant.ID, payslipID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "get_failed", "get failed")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payslip)
}

func handlePayrollSocialInsurancePolicies(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = currentUTCDateString()
	}

	switch r.Method {
	case http.MethodGet:
		versions, err := store.ListSocialInsurancePolicyVersions(r.Context(), tenant.ID, asOf)
		if err != nil {
			writePage(w, r, renderPayrollSocialInsurancePolicies(nil, asOf, "", err.Error()))
			return
		}

		eventID, err := newUUIDv4()
		if err != nil {
			writePage(w, r, renderPayrollSocialInsurancePolicies(versions, asOf, "", err.Error()))
			return
		}
		writePage(w, r, renderPayrollSocialInsurancePolicies(versions, asOf, eventID, ""))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderPayrollSocialInsurancePolicies(nil, asOf, "", "bad form"))
			return
		}

		eventID := strings.TrimSpace(r.Form.Get("event_id"))
		if eventID == "" {
			v, err := newUUIDv4()
			if err != nil {
				writePage(w, r, renderPayrollSocialInsurancePolicies(nil, asOf, "", err.Error()))
				return
			}
			eventID = v
		}

		cityCode := strings.TrimSpace(r.Form.Get("city_code"))
		hukouType := strings.TrimSpace(r.Form.Get("hukou_type"))
		insuranceType := strings.TrimSpace(r.Form.Get("insurance_type"))
		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		employerRate := strings.TrimSpace(r.Form.Get("employer_rate"))
		employeeRate := strings.TrimSpace(r.Form.Get("employee_rate"))
		baseFloor := strings.TrimSpace(r.Form.Get("base_floor"))
		baseCeiling := strings.TrimSpace(r.Form.Get("base_ceiling"))
		roundingRule := strings.TrimSpace(r.Form.Get("rounding_rule"))
		precisionText := strings.TrimSpace(r.Form.Get("precision"))
		rulesConfigText := strings.TrimSpace(r.Form.Get("rules_config_json"))

		precision, err := strconv.Atoi(precisionText)
		if err != nil {
			versions, _ := store.ListSocialInsurancePolicyVersions(r.Context(), tenant.ID, asOf)
			writePage(w, r, renderPayrollSocialInsurancePolicies(versions, asOf, eventID, "precision invalid"))
			return
		}

		var rulesConfig json.RawMessage
		if rulesConfigText == "" {
			rulesConfig = json.RawMessage(`{}`)
		} else {
			var v any
			if err := json.Unmarshal([]byte(rulesConfigText), &v); err != nil {
				versions, _ := store.ListSocialInsurancePolicyVersions(r.Context(), tenant.ID, asOf)
				writePage(w, r, renderPayrollSocialInsurancePolicies(versions, asOf, eventID, "rules_config_json invalid"))
				return
			}
			if _, ok := v.(map[string]any); !ok {
				versions, _ := store.ListSocialInsurancePolicyVersions(r.Context(), tenant.ID, asOf)
				writePage(w, r, renderPayrollSocialInsurancePolicies(versions, asOf, eventID, "rules_config_json must be object"))
				return
			}
			rulesConfig = json.RawMessage(rulesConfigText)
		}

		_, err = store.UpsertSocialInsurancePolicyVersion(r.Context(), tenant.ID, SocialInsurancePolicyUpsertInput{
			EventID:       eventID,
			CityCode:      cityCode,
			HukouType:     hukouType,
			InsuranceType: insuranceType,
			EffectiveDate: effectiveDate,
			EmployerRate:  employerRate,
			EmployeeRate:  employeeRate,
			BaseFloor:     baseFloor,
			BaseCeiling:   baseCeiling,
			RoundingRule:  roundingRule,
			Precision:     precision,
			RulesConfig:   rulesConfig,
		})
		if err != nil {
			versions, _ := store.ListSocialInsurancePolicyVersions(r.Context(), tenant.ID, asOf)
			writePage(w, r, renderPayrollSocialInsurancePolicies(versions, asOf, eventID, err.Error()))
			return
		}

		loc := "/org/payroll-social-insurance-policies"
		if asOf != "" {
			loc += "?as_of=" + url.QueryEscape(asOf)
		}
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePayrollSocialInsurancePoliciesAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
		if asOf == "" {
			asOf = currentUTCDateString()
		}
		versions, err := store.ListSocialInsurancePolicyVersions(r.Context(), tenant.ID, asOf)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(versions)
		return
	case http.MethodPost:
		var req SocialInsurancePolicyUpsertInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if strings.TrimSpace(req.EventID) == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "event_id_missing", "event_id is required")
			return
		}

		out, err := store.UpsertSocialInsurancePolicyVersion(r.Context(), tenant.ID, req)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "upsert_failed", "upsert failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePayrollRecalcRequests(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	reqs, err := store.ListPayrollRecalcRequests(r.Context(), tenant.ID, personUUID, state)
	if err != nil {
		writePage(w, r, renderPayrollRecalcRequests(nil, asOf, personUUID, state, err.Error()))
		return
	}

	writePage(w, r, renderPayrollRecalcRequests(reqs, asOf, personUUID, state, ""))
}

func handlePayrollRecalcRequestDetail(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	recalcRequestID, ok := requireFirstSegmentFromPath(w, r, "/org/payroll-recalc-requests/", routing.RouteClassUI)
	if !ok {
		return
	}

	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	d, err := store.GetPayrollRecalcRequest(r.Context(), tenant.ID, recalcRequestID)
	if err != nil {
		writePage(w, r, renderPayrollRecalcRequestDetail(recalcRequestID, asOf, PayrollRecalcRequestDetail{}, PayrollRecalcApplication{}, err.Error()))
		return
	}
	var app PayrollRecalcApplication
	if d.Application != nil {
		app = *d.Application
	}
	writePage(w, r, renderPayrollRecalcRequestDetail(recalcRequestID, asOf, d, app, ""))
}

func handlePayrollRecalcRequestApply(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	recalcRequestID, ok := requireFirstSegmentFromPath(w, r, "/org/payroll-recalc-requests/", routing.RouteClassUI)
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if err := r.ParseForm(); err != nil {
		d, _ := store.GetPayrollRecalcRequest(r.Context(), tenant.ID, recalcRequestID)
		writePageWithStatus(w, r, http.StatusUnprocessableEntity, renderPayrollRecalcRequestDetail(recalcRequestID, asOf, d, PayrollRecalcApplication{}, "bad form"))
		return
	}

	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_missing", "principal missing")
		return
	}

	targetRunID := strings.TrimSpace(r.Form.Get("target_run_id"))
	if targetRunID == "" {
		d, _ := store.GetPayrollRecalcRequest(r.Context(), tenant.ID, recalcRequestID)
		writePageWithStatus(w, r, http.StatusUnprocessableEntity, renderPayrollRecalcRequestDetail(recalcRequestID, asOf, d, PayrollRecalcApplication{}, "target_run_id is required"))
		return
	}

	_, err := store.ApplyPayrollRecalcRequest(r.Context(), tenant.ID, principal.ID, recalcRequestID, targetRunID)
	if err != nil {
		msg := stablePgMessage(err)
		status := http.StatusUnprocessableEntity
		if msg == "STAFFING_PAYROLL_RECALC_ALREADY_APPLIED" {
			status = http.StatusConflict
		}
		if msg == "STAFFING_PAYROLL_RECALC_REQUEST_NOT_FOUND" {
			status = http.StatusNotFound
		}

		d, _ := store.GetPayrollRecalcRequest(r.Context(), tenant.ID, recalcRequestID)
		writePageWithStatus(w, r, status, renderPayrollRecalcRequestDetail(recalcRequestID, asOf, d, PayrollRecalcApplication{}, msg))
		return
	}

	loc := "/org/payroll-recalc-requests/" + url.PathEscape(recalcRequestID)
	if asOf != "" {
		loc += "?as_of=" + url.QueryEscape(asOf)
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

func handlePayrollRecalcRequestsAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	reqs, err := store.ListPayrollRecalcRequests(r.Context(), tenant.ID, personUUID, state)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", "bad request")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(reqs)
}

func handlePayrollRecalcRequestAPI(w http.ResponseWriter, r *http.Request, store PayrollStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	seg, ok := requireFirstSegmentFromPath(w, r, "/org/api/payroll-recalc-requests/", routing.RouteClassInternalAPI)
	if !ok {
		return
	}
	isApply := strings.HasSuffix(seg, ":apply")
	recalcRequestID := seg
	if isApply {
		recalcRequestID = strings.TrimSuffix(seg, ":apply")
	}

	switch r.Method {
	case http.MethodGet:
		if isApply {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
			return
		}
		d, err := store.GetPayrollRecalcRequest(r.Context(), tenant.ID, recalcRequestID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
				return
			}
			if isPgInvalidInput(err) || isBadRequestError(err) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", "bad request")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "get_failed", "get failed")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(d)
		return

	case http.MethodPost:
		if !isApply {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "not_found", "not found")
			return
		}

		principal, ok := currentPrincipal(r.Context())
		if !ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "principal_missing", "principal missing")
			return
		}

		var req struct {
			TargetRunID string `json:"target_run_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}

		req.TargetRunID = strings.TrimSpace(req.TargetRunID)
		if req.TargetRunID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "target_run_id_missing", "target_run_id is required")
			return
		}

		app, err := store.ApplyPayrollRecalcRequest(r.Context(), tenant.ID, principal.ID, recalcRequestID, req.TargetRunID)
		if err != nil {
			switch msg := stablePgMessage(err); msg {
			case "STAFFING_PAYROLL_RECALC_REQUEST_NOT_FOUND":
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, msg, msg)
				return
			case "STAFFING_PAYROLL_RECALC_ALREADY_APPLIED":
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, msg, msg)
				return
			default:
				if strings.HasPrefix(msg, "STAFFING_") {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, msg, msg)
					return
				}
				if isPgInvalidInput(err) || isBadRequestError(err) {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_request", "bad request")
					return
				}
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "apply_failed", "apply failed")
				return
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"recalc_request_id":    app.RecalcRequestID,
			"target_run_id":        app.TargetRunID,
			"target_pay_period_id": app.TargetPayPeriodID,
		})
		return

	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(uuidRandReader, b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func stablePgMessage(err error) string {
	msg := pgErrorMessage(err)
	if isStableDBCode(msg) {
		return msg
	}
	return err.Error()
}

func isStableDBCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || code == "UNKNOWN" {
		return false
	}
	for i := 0; i < len(code); i++ {
		ch := code[i]
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '_' {
			continue
		}
		return false
	}
	if code[0] < 'A' || code[0] > 'Z' {
		return false
	}
	return true
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func requireFirstSegmentFromPath(w http.ResponseWriter, r *http.Request, prefix string, rc routing.RouteClass) (string, bool) {
	path := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(path, prefix) {
		routing.WriteError(w, r, rc, http.StatusNotFound, "not_found", "not found")
		return "", false
	}

	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		routing.WriteError(w, r, rc, http.StatusNotFound, "not_found", "not found")
		return "", false
	}
	if strings.Contains(rest, "/") {
		rest = strings.Split(rest, "/")[0]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		routing.WriteError(w, r, rc, http.StatusNotFound, "not_found", "not found")
		return "", false
	}
	return rest, true
}

func requireRunIDFromPath(w http.ResponseWriter, r *http.Request, prefix string) (string, bool) {
	path := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(path, prefix) {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return "", false
	}

	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return "", false
	}
	if strings.Contains(rest, "/") {
		rest = strings.Split(rest, "/")[0]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "not_found", "not found")
		return "", false
	}
	return rest, true
}

func renderPayrollRecalcRequests(reqs []PayrollRecalcRequestSummary, asOf string, personUUID string, state string, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payroll Recalc Requests</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}

	filterAction := "/org/payroll-recalc-requests"
	if asOf != "" {
		filterAction += "?as_of=" + url.QueryEscape(asOf)
	}
	b.WriteString(`<form method="GET" action="` + html.EscapeString(filterAction) + `">`)
	b.WriteString(`<label>person_uuid <input name="person_uuid" value="` + html.EscapeString(personUUID) + `"></label> `)
	b.WriteString(`<label>state <select name="state">`)
	b.WriteString(`<option value=""` + selectedAttr(state == "") + `>(all)</option>`)
	b.WriteString(`<option value="pending"` + selectedAttr(state == "pending") + `>pending</option>`)
	b.WriteString(`<option value="applied"` + selectedAttr(state == "applied") + `>applied</option>`)
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Filter</button>`)
	b.WriteString(`</form>`)

	if len(reqs) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
		b.WriteString(`<p><a href="/org/payroll-periods`)
		if asOf != "" {
			b.WriteString(`?as_of=` + url.QueryEscape(asOf))
		}
		b.WriteString(`">Back to Payroll</a></p>`)
		return b.String()
	}

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
	b.WriteString(`<tr><th>created_at</th><th>recalc_request_id</th><th>person_uuid</th><th>assignment_id</th><th>effective_date</th><th>hit_pay_period_id</th><th>applied</th></tr>`)
	for _, r := range reqs {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(r.CreatedAt) + `</td>`)
		b.WriteString(`<td><a href="/org/payroll-recalc-requests/` + url.PathEscape(r.RecalcRequestID))
		if asOf != "" {
			b.WriteString(`?as_of=` + url.QueryEscape(asOf))
		}
		b.WriteString(`"><code>` + html.EscapeString(r.RecalcRequestID) + `</code></a></td>`)
		b.WriteString(`<td><code>` + html.EscapeString(r.PersonUUID) + `</code></td>`)
		b.WriteString(`<td><code>` + html.EscapeString(r.AssignmentID) + `</code></td>`)
		b.WriteString(`<td>` + html.EscapeString(r.EffectiveDate) + `</td>`)
		b.WriteString(`<td><code>` + html.EscapeString(r.HitPayPeriodID) + `</code></td>`)
		b.WriteString(`<td>` + html.EscapeString(fmt.Sprintf("%t", r.Applied)) + `</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	b.WriteString(`<p><a href="/org/payroll-periods`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Payroll</a></p>`)
	return b.String()
}

func renderPayrollRecalcRequestDetail(recalcRequestID string, asOf string, d PayrollRecalcRequestDetail, applied PayrollRecalcApplication, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payroll Recalc Request</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}
	b.WriteString(`<p>recalc_request_id: <code>` + html.EscapeString(recalcRequestID) + `</code></p>`)

	if d.RecalcRequestID != "" {
		b.WriteString(`<h2>Request</h2>`)
		b.WriteString(`<ul>`)
		b.WriteString(`<li>trigger_event_id: <code>` + html.EscapeString(d.TriggerEventID) + `</code></li>`)
		b.WriteString(`<li>request_id: <code>` + html.EscapeString(d.RequestID) + `</code></li>`)
		b.WriteString(`<li>initiator_id: <code>` + html.EscapeString(d.InitiatorID) + `</code></li>`)
		b.WriteString(`<li>transaction_time: <code>` + html.EscapeString(d.TransactionTime) + `</code></li>`)
		b.WriteString(`<li>created_at: <code>` + html.EscapeString(d.CreatedAt) + `</code></li>`)
		b.WriteString(`<li>person_uuid: <code>` + html.EscapeString(d.PersonUUID) + `</code></li>`)
		b.WriteString(`<li>assignment_id: <code>` + html.EscapeString(d.AssignmentID) + `</code></li>`)
		b.WriteString(`<li>effective_date: <code>` + html.EscapeString(d.EffectiveDate) + `</code></li>`)
		b.WriteString(`<li>hit_pay_period_id: <code>` + html.EscapeString(d.HitPayPeriodID) + `</code></li>`)
		b.WriteString(`<li>hit_run_id: <code>` + html.EscapeString(d.HitRunID) + `</code></li>`)
		b.WriteString(`<li>hit_payslip_id: <code>` + html.EscapeString(d.HitPayslipID) + `</code></li>`)
		b.WriteString(`</ul>`)
	}

	if applied.ApplicationID != "" {
		b.WriteString(`<h2>Application</h2>`)
		b.WriteString(`<ul>`)
		b.WriteString(`<li>application_id: <code>` + html.EscapeString(applied.ApplicationID) + `</code></li>`)
		b.WriteString(`<li>event_id: <code>` + html.EscapeString(applied.EventID) + `</code></li>`)
		b.WriteString(`<li>target_run_id: <code>` + html.EscapeString(applied.TargetRunID) + `</code></li>`)
		b.WriteString(`<li>target_pay_period_id: <code>` + html.EscapeString(applied.TargetPayPeriodID) + `</code></li>`)
		b.WriteString(`<li>created_at: <code>` + html.EscapeString(applied.CreatedAt) + `</code></li>`)
		b.WriteString(`</ul>`)
	} else if d.RecalcRequestID != "" {
		b.WriteString(`<h2>Apply</h2>`)
		postAction := "/org/payroll-recalc-requests/" + url.PathEscape(recalcRequestID) + "/apply"
		if asOf != "" {
			postAction += "?as_of=" + url.QueryEscape(asOf)
		}
		b.WriteString(`<form method="POST" action="` + html.EscapeString(postAction) + `">`)
		b.WriteString(`<label>target_run_id <input name="target_run_id" required></label><br>`)
		b.WriteString(`<button type="submit">Apply</button>`)
		b.WriteString(`</form>`)
		b.WriteString(`<p><a href="/org/payroll-runs`)
		if asOf != "" {
			b.WriteString(`?as_of=` + url.QueryEscape(asOf))
		}
		b.WriteString(`">Open Payroll Runs</a></p>`)
	}

	b.WriteString(`<h2>Adjustments Summary</h2>`)
	if len(d.AdjustmentsSummary) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>item_kind</th><th>item_code</th><th>amount</th></tr>`)
		for _, s := range d.AdjustmentsSummary {
			b.WriteString(`<tr>`)
			b.WriteString(`<td>` + html.EscapeString(s.ItemKind) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(s.ItemCode) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(s.Amount) + `</td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}

	b.WriteString(`<p><a href="/org/payroll-recalc-requests`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Recalc Requests</a></p>`)

	return b.String()
}

func selectedAttr(selected bool) string {
	if selected {
		return ` selected`
	}
	return ""
}

func renderPayrollPeriods(periods []PayPeriod, asOf string, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payroll Periods</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}
	b.WriteString(`<p><a href="/org/payroll-social-insurance-policies`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Social Insurance Policies</a></p>`)
	b.WriteString(`<p><a href="/org/payroll-recalc-requests`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Payroll Recalc Requests</a></p>`)

	b.WriteString(`<h2>Create</h2>`)
	postAction := "/org/payroll-periods"
	if asOf != "" {
		postAction += "?as_of=" + url.QueryEscape(asOf)
	}
	b.WriteString(`<form method="POST" action="` + html.EscapeString(postAction) + `">`)
	b.WriteString(`<label>pay_group <input name="pay_group" placeholder="monthly" required></label><br>`)
	b.WriteString(`<label>start_date <input type="date" name="start_date" required></label><br>`)
	b.WriteString(`<label>end_date_exclusive <input type="date" name="end_date_exclusive" required></label><br>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>List</h2>`)
	if len(periods) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
		return b.String()
	}
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
	b.WriteString(`<tr><th>id</th><th>pay_group</th><th>start</th><th>end(excl)</th><th>status</th><th>closed_at</th></tr>`)
	for _, p := range periods {
		b.WriteString(`<tr>`)
		b.WriteString(`<td><code>` + html.EscapeString(p.ID) + `</code></td>`)
		b.WriteString(`<td>` + html.EscapeString(p.PayGroup) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.StartDate) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.EndDateExclusive) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.Status) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.ClosedAt) + `</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	b.WriteString(`<p><a href="/org/payroll-runs`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Go to Payroll Runs</a></p>`)

	return b.String()
}

func renderPayrollRuns(runs []PayrollRun, periods []PayPeriod, asOf string, filterPayPeriodID string, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payroll Runs</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}
	b.WriteString(`<p><a href="/org/payroll-social-insurance-policies`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Social Insurance Policies</a></p>`)

	postAction := "/org/payroll-runs"
	if asOf != "" {
		postAction += "?as_of=" + url.QueryEscape(asOf)
	}
	if filterPayPeriodID != "" {
		sep := "?"
		if strings.Contains(postAction, "?") {
			sep = "&"
		}
		postAction += sep + "pay_period_id=" + url.QueryEscape(filterPayPeriodID)
	}

	b.WriteString(`<h2>Create</h2>`)
	b.WriteString(`<form method="POST" action="` + html.EscapeString(postAction) + `">`)
	b.WriteString(`<label>pay_period_id <select name="pay_period_id" required>`)
	b.WriteString(`<option value="">(select)</option>`)
	for _, p := range periods {
		sel := ""
		if filterPayPeriodID != "" && filterPayPeriodID == p.ID {
			sel = ` selected`
		}
		b.WriteString(`<option value="` + html.EscapeString(p.ID) + `"` + sel + `>`)
		b.WriteString(html.EscapeString(p.PayGroup + " [" + p.StartDate + "," + p.EndDateExclusive + ")"))
		b.WriteString(`</option>`)
	}
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>List</h2>`)
	if len(runs) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
		return b.String()
	}
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
	b.WriteString(`<tr><th>id</th><th>pay_period_id</th><th>state</th><th>calc_started_at</th><th>calc_finished_at</th><th>finalized_at</th></tr>`)
	for _, run := range runs {
		b.WriteString(`<tr>`)
		b.WriteString(`<td><a href="/org/payroll-runs/` + url.PathEscape(run.ID))
		if asOf != "" {
			b.WriteString(`?as_of=` + url.QueryEscape(asOf))
		}
		b.WriteString(`"><code>` + html.EscapeString(run.ID) + `</code></a></td>`)
		b.WriteString(`<td><code>` + html.EscapeString(run.PayPeriodID) + `</code></td>`)
		b.WriteString(`<td>` + html.EscapeString(run.RunState) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(run.CalcStartedAt) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(run.CalcFinishedAt) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(run.FinalizedAt) + `</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	b.WriteString(`<p><a href="/org/payroll-periods`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Payroll Periods</a></p>`)

	return b.String()
}

func renderPayrollRun(runID string, asOf string, run PayrollRun, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payroll Run</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}
	b.WriteString(`<p>run_id: <code>` + html.EscapeString(runID) + `</code></p>`)

	if run.ID != "" {
		b.WriteString(`<ul>`)
		b.WriteString(`<li>pay_period_id: <code>` + html.EscapeString(run.PayPeriodID) + `</code></li>`)
		b.WriteString(`<li>state: <code>` + html.EscapeString(run.RunState) + `</code></li>`)
		b.WriteString(`<li>calc_started_at: <code>` + html.EscapeString(run.CalcStartedAt) + `</code></li>`)
		b.WriteString(`<li>calc_finished_at: <code>` + html.EscapeString(run.CalcFinishedAt) + `</code></li>`)
		b.WriteString(`<li>finalized_at: <code>` + html.EscapeString(run.FinalizedAt) + `</code></li>`)
		b.WriteString(`</ul>`)
	}

	postSuffix := ""
	if asOf != "" {
		postSuffix = "?as_of=" + url.QueryEscape(asOf)
	}

	b.WriteString(`<h2>Actions</h2>`)
	b.WriteString(`<form method="POST" action="/org/payroll-runs/` + url.PathEscape(runID) + `/calculate` + postSuffix + `">`)
	b.WriteString(`<button type="submit">Calculate</button>`)
	b.WriteString(`</form>`)
	b.WriteString(`<form method="POST" action="/org/payroll-runs/` + url.PathEscape(runID) + `/finalize` + postSuffix + `">`)
	b.WriteString(`<button type="submit">Finalize</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<p><a href="/org/payroll-runs/` + url.PathEscape(runID) + `/payslips` + postSuffix + `">Payslips</a></p>`)

	b.WriteString(`<p><a href="/org/payroll-runs`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Payroll Runs</a></p>`)
	return b.String()
}

func renderPayslips(runID string, asOf string, payslips []Payslip, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payslips</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}
	b.WriteString(`<p>run_id: <code>` + html.EscapeString(runID) + `</code></p>`)

	if len(payslips) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>id</th><th>person_uuid</th><th>assignment_id</th><th>currency</th><th>gross_pay</th><th>net_pay</th><th>employer_total</th></tr>`)
		for _, p := range payslips {
			b.WriteString(`<tr>`)
			b.WriteString(`<td><a href="/org/payroll-runs/` + url.PathEscape(runID) + `/payslips/` + url.PathEscape(p.ID))
			if asOf != "" {
				b.WriteString(`?as_of=` + url.QueryEscape(asOf))
			}
			b.WriteString(`"><code>` + html.EscapeString(p.ID) + `</code></a></td>`)
			b.WriteString(`<td><code>` + html.EscapeString(p.PersonUUID) + `</code></td>`)
			b.WriteString(`<td><code>` + html.EscapeString(p.AssignmentID) + `</code></td>`)
			b.WriteString(`<td>` + html.EscapeString(p.Currency) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(p.GrossPay) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(p.NetPay) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(p.EmployerTotal) + `</td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}

	b.WriteString(`<p><a href="/org/payroll-runs/` + url.PathEscape(runID))
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Payroll Run</a></p>`)

	return b.String()
}

func renderPayslipDetail(runID string, payslipID string, asOf string, run PayrollRun, payslip PayslipDetail, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payslip</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}
	b.WriteString(`<p>run_id: <code>` + html.EscapeString(runID) + `</code></p>`)
	b.WriteString(`<p>payslip_id: <code>` + html.EscapeString(payslipID) + `</code></p>`)

	if payslip.ID != "" {
		b.WriteString(`<ul>`)
		b.WriteString(`<li>person_uuid: <code>` + html.EscapeString(payslip.PersonUUID) + `</code></li>`)
		b.WriteString(`<li>assignment_id: <code>` + html.EscapeString(payslip.AssignmentID) + `</code></li>`)
		b.WriteString(`<li>currency: <code>` + html.EscapeString(payslip.Currency) + `</code></li>`)
		b.WriteString(`<li>gross_pay: <code>` + html.EscapeString(payslip.GrossPay) + `</code></li>`)
		b.WriteString(`<li>net_pay: <code>` + html.EscapeString(payslip.NetPay) + `</code></li>`)
		b.WriteString(`<li>employer_total: <code>` + html.EscapeString(payslip.EmployerTotal) + `</code></li>`)
		b.WriteString(`</ul>`)
	}

	b.WriteString(`<h2>Net Guaranteed IIT (Inputs)</h2>`)
	if run.RunState == "finalized" {
		b.WriteString(`<p><em>(finalized: read-only)</em></p>`)
	} else {
		postAction := "/org/payroll-runs/" + url.PathEscape(runID) + "/payslips/" + url.PathEscape(payslipID) + "/net-guaranteed-iit-items"
		if asOf != "" {
			postAction += "?as_of=" + url.QueryEscape(asOf)
		}
		reqID, _ := newUUIDv4()
		b.WriteString(`<form method="POST" action="` + html.EscapeString(postAction) + `">`)
		b.WriteString(`<input type="hidden" name="event_type" value="UPSERT">`)
		b.WriteString(`<input type="hidden" name="request_id" value="` + html.EscapeString(reqID) + `">`)
		b.WriteString(`<label>item_code <input name="item_code" value="EARNING_LONG_SERVICE_AWARD" required></label> `)
		b.WriteString(`<label>target_net <input name="target_net" placeholder="20000.00" required></label> `)
		b.WriteString(`<button type="submit">Upsert</button>`)
		b.WriteString(`</form>`)

		if run.NeedsRecalc {
			b.WriteString(`<p style="color:#b00020">needs_recalc=true（请重新计算该 run）</p>`)
		}
	}

	var ngInputs []PayslipItemInput
	for _, in := range payslip.ItemInputs {
		if in.CalcMode == "net_guaranteed_iit" {
			ngInputs = append(ngInputs, in)
		}
	}
	if len(ngInputs) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>item_code</th><th>target_net</th><th>updated_at</th><th>last_event_id</th><th>actions</th></tr>`)
		for _, in := range ngInputs {
			b.WriteString(`<tr>`)
			b.WriteString(`<td>` + html.EscapeString(in.ItemCode) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(in.Amount) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(in.UpdatedAt) + `</td>`)
			b.WriteString(`<td><code>` + html.EscapeString(in.LastEventID) + `</code></td>`)
			b.WriteString(`<td>`)
			if run.RunState != "finalized" {
				delAction := "/org/payroll-runs/" + url.PathEscape(runID) + "/payslips/" + url.PathEscape(payslipID) + "/net-guaranteed-iit-items"
				if asOf != "" {
					delAction += "?as_of=" + url.QueryEscape(asOf)
				}
				delReqID, _ := newUUIDv4()
				b.WriteString(`<form method="POST" action="` + html.EscapeString(delAction) + `" style="margin:0">`)
				b.WriteString(`<input type="hidden" name="event_type" value="DELETE">`)
				b.WriteString(`<input type="hidden" name="item_code" value="` + html.EscapeString(in.ItemCode) + `">`)
				b.WriteString(`<input type="hidden" name="request_id" value="` + html.EscapeString(delReqID) + `">`)
				b.WriteString(`<button type="submit">Delete</button>`)
				b.WriteString(`</form>`)
			} else {
				b.WriteString(`<em>(read-only)</em>`)
			}
			b.WriteString(`</td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}

	b.WriteString(`<h2>Net Guaranteed IIT (Results)</h2>`)
	var ngItems []PayslipItem
	for _, item := range payslip.Items {
		if item.CalcMode == "net_guaranteed_iit" {
			ngItems = append(ngItems, item)
		}
	}
	if len(ngItems) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>item_code</th><th>target_net</th><th>gross_amount</th><th>iit_delta</th><th>net_after_iit</th></tr>`)
		for _, item := range ngItems {
			b.WriteString(`<tr>`)
			b.WriteString(`<td>` + html.EscapeString(item.ItemCode) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.TargetNet) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.Amount) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.IITDelta) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.TargetNet) + `</td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)

		var explain map[string]any
		if err := json.Unmarshal(ngItems[0].Meta, &explain); err == nil {
			keys := []string{
				"tax_year",
				"tax_month",
				"group_target_net",
				"group_solved_gross",
				"group_delta_iit",
				"base_income",
				"base_iit_withhold",
				"iterations",
			}
			b.WriteString(`<p>explain:</p>`)
			b.WriteString(`<pre style="margin:0">`)
			for _, k := range keys {
				if v, ok := explain[k]; ok {
					b.WriteString(html.EscapeString(k) + `=` + html.EscapeString(toString(v)) + "\n")
				}
			}
			b.WriteString(`</pre>`)
		}
	}

	b.WriteString(`<h2>Items</h2>`)
	if len(payslip.Items) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>id</th><th>item_code</th><th>item_kind</th><th>amount</th><th>calc_mode</th><th>tax_bearer</th><th>target_net</th><th>iit_delta</th><th>meta</th></tr>`)
		for _, item := range payslip.Items {
			b.WriteString(`<tr>`)
			b.WriteString(`<td><code>` + html.EscapeString(item.ID) + `</code></td>`)
			b.WriteString(`<td>` + html.EscapeString(item.ItemCode) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.ItemKind) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.Amount) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.CalcMode) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.TaxBearer) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.TargetNet) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.IITDelta) + `</td>`)
			b.WriteString(`<td><pre style="margin:0">` + html.EscapeString(string(item.Meta)) + `</pre></td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}

	b.WriteString(`<h2>Social Insurance</h2>`)
	if len(payslip.SocialInsuranceItems) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<p>employee_total: <code>` + html.EscapeString(payslip.SocialInsuranceEmployeeTotal) + `</code> `)
		b.WriteString(`employer_total: <code>` + html.EscapeString(payslip.SocialInsuranceEmployerTotal) + `</code></p>`)
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>insurance_type</th><th>base_amount</th><th>employee_amount</th><th>employer_amount</th><th>policy_effective_date</th></tr>`)
		for _, item := range payslip.SocialInsuranceItems {
			b.WriteString(`<tr>`)
			b.WriteString(`<td>` + html.EscapeString(item.InsuranceType) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.BaseAmount) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.EmployeeAmount) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.EmployerAmount) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.PolicyEffectiveAt) + `</td>`)
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}

	b.WriteString(`<p><a href="/org/payroll-runs/` + url.PathEscape(runID) + `/payslips`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Payslips</a></p>`)

	return b.String()
}

func renderPayrollSocialInsurancePolicies(versions []SocialInsurancePolicyVersion, asOf string, eventID string, errMsg string) string {
	byType := map[string]SocialInsurancePolicyVersion{}
	for _, v := range versions {
		byType[v.InsuranceType] = v
	}

	defaultCityCode := ""
	if len(versions) > 0 {
		defaultCityCode = versions[0].CityCode
	}

	var b strings.Builder
	b.WriteString(`<h1>Social Insurance Policies</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}

	b.WriteString(`<h2>List (as-of)</h2>`)
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
	b.WriteString(`<tr><th>insurance_type</th><th>effective_date</th><th>employer_rate</th><th>employee_rate</th><th>base_floor</th><th>base_ceiling</th><th>rounding_rule</th><th>precision</th><th>city_code</th><th>hukou_type</th><th>policy_id</th></tr>`)
	for _, t := range []string{"PENSION", "MEDICAL", "UNEMPLOYMENT", "INJURY", "MATERNITY", "HOUSING_FUND"} {
		v, ok := byType[t]
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(t) + `</td>`)
		if !ok {
			b.WriteString(`<td colspan="10"><em>(missing)</em></td>`)
			b.WriteString(`</tr>`)
			continue
		}
		b.WriteString(`<td>` + html.EscapeString(v.EffectiveDate) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.EmployerRate) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.EmployeeRate) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.BaseFloor) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.BaseCeiling) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.RoundingRule) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(strconv.Itoa(v.Precision)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.CityCode) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.HukouType) + `</td>`)
		b.WriteString(`<td><code>` + html.EscapeString(v.PolicyID) + `</code></td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	b.WriteString(`<h2>Upsert Version</h2>`)
	postAction := "/org/payroll-social-insurance-policies"
	if asOf != "" {
		postAction += "?as_of=" + url.QueryEscape(asOf)
	}
	b.WriteString(`<form method="POST" action="` + html.EscapeString(postAction) + `">`)
	b.WriteString(`<input type="hidden" name="event_id" value="` + html.EscapeString(eventID) + `">`)
	b.WriteString(`<label>city_code <input name="city_code" placeholder="CN-310000" value="` + html.EscapeString(defaultCityCode) + `" required></label><br>`)
	b.WriteString(`<input type="hidden" name="hukou_type" value="default">`)
	b.WriteString(`<label>insurance_type <select name="insurance_type" required>`)
	for _, t := range []string{"PENSION", "MEDICAL", "UNEMPLOYMENT", "INJURY", "MATERNITY", "HOUSING_FUND"} {
		b.WriteString(`<option value="` + html.EscapeString(t) + `">` + html.EscapeString(t) + `</option>`)
	}
	b.WriteString(`</select></label><br>`)
	b.WriteString(`<label>effective_date <input type="date" name="effective_date" required></label><br>`)
	b.WriteString(`<label>employer_rate <input name="employer_rate" placeholder="0.16" required></label><br>`)
	b.WriteString(`<label>employee_rate <input name="employee_rate" placeholder="0.08" required></label><br>`)
	b.WriteString(`<label>base_floor <input name="base_floor" placeholder="0.00" required></label><br>`)
	b.WriteString(`<label>base_ceiling <input name="base_ceiling" placeholder="99999.99" required></label><br>`)
	b.WriteString(`<label>rounding_rule <select name="rounding_rule" required><option value="HALF_UP">HALF_UP</option><option value="CEIL">CEIL</option></select></label><br>`)
	b.WriteString(`<label>precision <select name="precision" required><option value="0">0</option><option value="1">1</option><option value="2" selected>2</option></select></label><br>`)
	b.WriteString(`<label>rules_config_json <textarea name="rules_config_json" rows="4" cols="60" placeholder="{}"></textarea></label><br>`)
	b.WriteString(`<button type="submit">Upsert</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<p><a href="/org/payroll-periods`)
	if asOf != "" {
		b.WriteString(`?as_of=` + url.QueryEscape(asOf))
	}
	b.WriteString(`">Back to Payroll</a></p>`)

	return b.String()
}
