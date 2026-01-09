package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

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

	p, err := store.GetPayslip(r.Context(), tenant.ID, payslipID)
	if err != nil {
		writePage(w, r, renderPayslipDetail(runID, payslipID, asOf, PayslipDetail{}, err.Error()))
		return
	}
	writePage(w, r, renderPayslipDetail(runID, payslipID, asOf, p, ""))
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

func renderPayrollPeriods(periods []PayPeriod, asOf string, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Payroll Periods</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	if asOf != "" {
		b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	}

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

func renderPayslipDetail(runID string, payslipID string, asOf string, payslip PayslipDetail, errMsg string) string {
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

	b.WriteString(`<h2>Items</h2>`)
	if len(payslip.Items) == 0 {
		b.WriteString(`<p><em>(empty)</em></p>`)
	} else {
		b.WriteString(`<table border="1" cellspacing="0" cellpadding="6">`)
		b.WriteString(`<tr><th>id</th><th>item_code</th><th>item_kind</th><th>amount</th><th>meta</th></tr>`)
		for _, item := range payslip.Items {
			b.WriteString(`<tr>`)
			b.WriteString(`<td><code>` + html.EscapeString(item.ID) + `</code></td>`)
			b.WriteString(`<td>` + html.EscapeString(item.ItemCode) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.ItemKind) + `</td>`)
			b.WriteString(`<td>` + html.EscapeString(item.Amount) + `</td>`)
			b.WriteString(`<td><pre style="margin:0">` + html.EscapeString(string(item.Meta)) + `</pre></td>`)
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
