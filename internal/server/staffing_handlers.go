package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

func handlePositions(w http.ResponseWriter, r *http.Request, orgStore OrgUnitStore, store PositionStore, jobStore JobCatalogStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	businessUnitID := strings.TrimSpace(r.URL.Query().Get("business_unit_id"))

	nodes, err := orgStore.ListNodesCurrent(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderPositions(nil, nil, tenant, asOf, nil, businessUnitID, "", nil, stablePgMessage(err)))
		return
	}
	positions, err := store.ListPositionsCurrent(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderPositions(nil, nodes, tenant, asOf, nil, businessUnitID, "", nil, stablePgMessage(err)))
		return
	}

	var businessUnits []BusinessUnit
	var jobProfiles []JobProfile
	resolvedSetID := ""
	jobCatalogMsg := ""
	if jobStore != nil {
		bus, err := jobStore.ListBusinessUnits(r.Context(), tenant.ID)
		if err != nil {
			jobCatalogMsg = mergeMsg(jobCatalogMsg, stablePgMessage(err))
		} else {
			businessUnits = bus
		}

		if businessUnitID != "" {
			profiles, resolved, err := jobStore.ListJobProfiles(r.Context(), tenant.ID, businessUnitID, asOf)
			if err != nil {
				jobCatalogMsg = mergeMsg(jobCatalogMsg, stablePgMessage(err))
			} else {
				jobProfiles = profiles
				resolvedSetID = resolved
			}
		}
	}

	switch r.Method {
	case http.MethodGet:
		writePage(w, r, renderPositions(positions, nodes, tenant, asOf, businessUnits, businessUnitID, resolvedSetID, jobProfiles, jobCatalogMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderPositions(positions, nodes, tenant, asOf, businessUnits, businessUnitID, resolvedSetID, jobProfiles, mergeMsg(jobCatalogMsg, "bad form")))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			writePage(w, r, renderPositions(positions, nodes, tenant, asOf, businessUnits, businessUnitID, resolvedSetID, jobProfiles, mergeMsg(jobCatalogMsg, "effective_date 无效: "+err.Error())))
			return
		}

		positionID := strings.TrimSpace(r.Form.Get("position_id"))
		orgUnitID := strings.TrimSpace(r.Form.Get("org_unit_id"))
		businessUnitID := strings.TrimSpace(r.Form.Get("business_unit_id"))
		reportsToPositionID := strings.TrimSpace(r.Form.Get("reports_to_position_id"))
		jobProfileID := strings.TrimSpace(r.Form.Get("job_profile_id"))
		name := strings.TrimSpace(r.Form.Get("name"))
		lifecycleStatus := strings.TrimSpace(r.Form.Get("lifecycle_status"))

		if positionID == "" {
			if _, err := store.CreatePositionCurrent(r.Context(), tenant.ID, effectiveDate, orgUnitID, businessUnitID, jobProfileID, name); err != nil {
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, businessUnits, businessUnitID, resolvedSetID, jobProfiles, mergeMsg(jobCatalogMsg, stablePgMessage(err))))
				return
			}
		} else {
			if _, err := store.UpdatePositionCurrent(r.Context(), tenant.ID, positionID, effectiveDate, orgUnitID, businessUnitID, reportsToPositionID, jobProfileID, name, lifecycleStatus); err != nil {
				writePage(w, r, renderPositions(positions, nodes, tenant, asOf, businessUnits, businessUnitID, resolvedSetID, jobProfiles, mergeMsg(jobCatalogMsg, stablePgMessage(err))))
				return
			}
		}

		redirectURL := "/org/positions?as_of=" + url.QueryEscape(effectiveDate)
		if businessUnitID != "" {
			redirectURL += "&business_unit_id=" + url.QueryEscape(businessUnitID)
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type staffingPositionsAPIRequest struct {
	EffectiveDate       string `json:"effective_date"`
	PositionID          string `json:"position_id"`
	OrgUnitID           string `json:"org_unit_id"`
	BusinessUnitID      string `json:"business_unit_id"`
	ReportsToPositionID string `json:"reports_to_position_id"`
	JobProfileID        string `json:"job_profile_id"`
	Name                string `json:"name"`
	LifecycleStatus     string `json:"lifecycle_status"`
}

func handlePositionsAPI(w http.ResponseWriter, r *http.Request, store PositionStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	switch r.Method {
	case http.MethodGet:
		positions, err := store.ListPositionsCurrent(r.Context(), tenant.ID, asOf)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":     asOf,
			"tenant":    tenant.ID,
			"positions": positions,
		})
		return
	case http.MethodPost:
		var req staffingPositionsAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if req.EffectiveDate == "" {
			req.EffectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}

		var p Position
		var err error
		if strings.TrimSpace(req.PositionID) == "" {
			p, err = store.CreatePositionCurrent(r.Context(), tenant.ID, req.EffectiveDate, req.OrgUnitID, req.BusinessUnitID, req.JobProfileID, req.Name)
		} else {
			p, err = store.UpdatePositionCurrent(r.Context(), tenant.ID, req.PositionID, req.EffectiveDate, req.OrgUnitID, req.BusinessUnitID, req.ReportsToPositionID, req.JobProfileID, req.Name, req.LifecycleStatus)
		}
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			switch pgErrorMessage(err) {
			case "STAFFING_IDEMPOTENCY_REUSED":
				status = http.StatusConflict
			default:
				if strings.HasPrefix(code, "STAFFING_") {
					status = http.StatusUnprocessableEntity
				}
				if isBadRequestError(err) || isPgInvalidInput(err) {
					status = http.StatusBadRequest
				}
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "upsert failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(p)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type staffingAssignmentsAPIRequest struct {
	EffectiveDate string `json:"effective_date"`
	PersonUUID    string `json:"person_uuid"`
	PositionID    string `json:"position_id"`
	BaseSalary    string `json:"base_salary"`
	AllocatedFte  string `json:"allocated_fte"`
}

func handleAssignmentsAPI(w http.ResponseWriter, r *http.Request, store AssignmentStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	switch r.Method {
	case http.MethodGet:
		personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
		if personUUID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "missing_person_uuid", "person_uuid is required")
			return
		}
		assigns, err := store.ListAssignmentsForPerson(r.Context(), tenant.ID, asOf, personUUID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":       asOf,
			"tenant":      tenant.ID,
			"person_uuid": personUUID,
			"assignments": assigns,
		})
		return
	case http.MethodPost:
		var req staffingAssignmentsAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if req.EffectiveDate == "" {
			req.EffectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
		a, err := store.UpsertPrimaryAssignmentForPerson(r.Context(), tenant.ID, req.EffectiveDate, req.PersonUUID, req.PositionID, req.BaseSalary, req.AllocatedFte)
		if err != nil {
			code := stablePgMessage(err)
			status := http.StatusInternalServerError
			switch pgErrorMessage(err) {
			case "STAFFING_IDEMPOTENCY_REUSED":
				status = http.StatusConflict
			default:
				if strings.HasPrefix(code, "STAFFING_") {
					status = http.StatusUnprocessableEntity
				}
				if isBadRequestError(err) || isPgInvalidInput(err) {
					status = http.StatusBadRequest
				}
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, "upsert failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(a)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleAssignments(w http.ResponseWriter, r *http.Request, positionStore PositionStore, assignmentStore AssignmentStore, personStore PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	positions, err := positionStore.ListPositionsCurrent(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderAssignments(nil, nil, tenant, asOf, "", "", "", stablePgMessage(err)))
		return
	}
	positions = filterActivePositions(positions)

	pernr := strings.TrimSpace(r.URL.Query().Get("pernr"))
	personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
	displayName := ""
	if pernr != "" && personUUID == "" {
		p, err := personStore.FindPersonByPernr(r.Context(), tenant.ID, pernr)
		if err != nil {
			writePage(w, r, renderAssignments(nil, positions, tenant, asOf, "", pernr, "", stablePgMessage(err)))
			return
		}
		personUUID = p.UUID
		pernr = p.Pernr
		displayName = p.DisplayName
	}

	list := func() ([]Assignment, string) {
		if personUUID == "" {
			return nil, ""
		}
		assigns, err := assignmentStore.ListAssignmentsForPerson(r.Context(), tenant.ID, asOf, personUUID)
		if err != nil {
			return nil, stablePgMessage(err)
		}
		return assigns, ""
	}

	switch r.Method {
	case http.MethodGet:
		assigns, errMsg := list()
		writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, errMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "bad form")))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "effective_date 无效: "+err.Error())))
			return
		}

		postPernr := strings.TrimSpace(r.Form.Get("pernr"))
		postPersonUUID := strings.TrimSpace(r.Form.Get("person_uuid"))
		positionID := strings.TrimSpace(r.Form.Get("position_id"))
		baseSalary := strings.TrimSpace(r.Form.Get("base_salary"))
		allocatedFte := strings.TrimSpace(r.Form.Get("allocated_fte"))

		if postPernr != "" {
			p, err := personStore.FindPersonByPernr(r.Context(), tenant.ID, postPernr)
			if err != nil {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, stablePgMessage(err))))
				return
			}
			if postPersonUUID != "" && postPersonUUID != p.UUID {
				assigns, errMsg := list()
				writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "pernr/person_uuid 不一致")))
				return
			}
			postPersonUUID = p.UUID
			postPernr = p.Pernr
			displayName = p.DisplayName
		} else if postPersonUUID == "" {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, personUUID, pernr, displayName, mergeMsg(errMsg, "pernr is required")))
			return
		}

		if _, err := assignmentStore.UpsertPrimaryAssignmentForPerson(r.Context(), tenant.ID, effectiveDate, postPersonUUID, positionID, baseSalary, allocatedFte); err != nil {
			assigns, errMsg := list()
			writePage(w, r, renderAssignments(assigns, positions, tenant, asOf, postPersonUUID, postPernr, displayName, mergeMsg(errMsg, stablePgMessage(err))))
			return
		}

		if postPernr != "" {
			http.Redirect(w, r, "/org/assignments?as_of="+url.QueryEscape(effectiveDate)+"&pernr="+url.QueryEscape(postPernr), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/org/assignments?as_of="+url.QueryEscape(effectiveDate)+"&person_uuid="+url.QueryEscape(postPersonUUID), http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func mergeMsg(a string, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "；" + b
}

func renderPositions(
	positions []Position,
	nodes []OrgUnitNode,
	tenant Tenant,
	asOf string,
	businessUnits []BusinessUnit,
	businessUnitID string,
	resolvedSetID string,
	jobProfiles []JobProfile,
	errMsg string,
) string {
	b := strings.Builder{}
	b.WriteString("<h1>Staffing / Positions</h1>")
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.Name) + `</code> (<code>` + html.EscapeString(tenant.ID) + `</code>)</p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code> | <a href="/org/assignments?as_of=` + url.QueryEscape(asOf) + `">Assignments</a></p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Job Catalog Context</h2>`)
	b.WriteString(`<form method="GET" action="/org/positions">`)
	b.WriteString(`<input type="hidden" name="as_of" value="` + html.EscapeString(asOf) + `" />`)
	b.WriteString(`<label>Business Unit <select name="business_unit_id">`)
	b.WriteString(`<option value="">(not set)</option>`)
	for _, bu := range businessUnits {
		if bu.Status != "active" {
			continue
		}
		selected := ""
		if bu.BusinessUnitID == businessUnitID {
			selected = " selected"
		}
		b.WriteString(
			`<option value="` + html.EscapeString(bu.BusinessUnitID) + `"` + selected + `>` +
				html.EscapeString(bu.BusinessUnitID) + `</option>`,
		)
	}
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Load</button>`)
	b.WriteString(`</form>`)
	if businessUnitID != "" && resolvedSetID != "" {
		b.WriteString(`<p>Resolved SetID: <code>` + html.EscapeString(resolvedSetID) + `</code></p>`)
	}

	b.WriteString(`<h2>Create</h2>`)
	b.WriteString(`<form method="POST" action="/org/positions?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Org Unit <select name="org_unit_id">`)
	if len(nodes) == 0 {
		b.WriteString(`<option value="">(no org units)</option>`)
	} else {
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
		for _, n := range nodes {
			b.WriteString(`<option value="` + html.EscapeString(n.ID) + `">` + html.EscapeString(n.Name) + ` (` + html.EscapeString(n.ID) + `)</option>`)
		}
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Business Unit <select name="business_unit_id">`)
	b.WriteString(`<option value="">(not set)</option>`)
	for _, bu := range businessUnits {
		if bu.Status != "active" {
			continue
		}
		selected := ""
		if bu.BusinessUnitID == businessUnitID {
			selected = " selected"
		}
		b.WriteString(`<option value="` + html.EscapeString(bu.BusinessUnitID) + `"` + selected + `>` + html.EscapeString(bu.BusinessUnitID) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Job Profile <select name="job_profile_id">`)
	b.WriteString(`<option value="">(not set)</option>`)
	for _, jp := range jobProfiles {
		label := jp.Code + " (" + jp.ID + ")"
		b.WriteString(`<option value="` + html.EscapeString(jp.ID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Name <input type="text" name="name" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Update / Disable</h2>`)
	b.WriteString(`<form method="POST" action="/org/positions?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Position <select name="position_id">`)
	if len(positions) == 0 {
		b.WriteString(`<option value="">(no positions)</option>`)
	} else {
		for _, p := range positions {
			label := p.ID
			if p.Name != "" {
				label = p.Name + " (" + p.ID + ")"
			}
			b.WriteString(`<option value="` + html.EscapeString(p.ID) + `">` + html.EscapeString(label) + `</option>`)
		}
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Org Unit <select name="org_unit_id">`)
	b.WriteString(`<option value="">(no change)</option>`)
	if len(nodes) > 0 {
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
		for _, n := range nodes {
			b.WriteString(`<option value="` + html.EscapeString(n.ID) + `">` + html.EscapeString(n.Name) + ` (` + html.EscapeString(n.ID) + `)</option>`)
		}
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Business Unit <select name="business_unit_id">`)
	b.WriteString(`<option value="">(no change)</option>`)
	for _, bu := range businessUnits {
		if bu.Status != "active" {
			continue
		}
		b.WriteString(`<option value="` + html.EscapeString(bu.BusinessUnitID) + `">` + html.EscapeString(bu.BusinessUnitID) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Reports To <select name="reports_to_position_id">`)
	b.WriteString(`<option value="">(no change)</option>`)
	for _, p := range filterActivePositions(positions) {
		label := p.ID
		if p.Name != "" {
			label = p.Name + " (" + p.ID + ")"
		}
		b.WriteString(`<option value="` + html.EscapeString(p.ID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Job Profile <select name="job_profile_id">` +
		`<option value="">(no change)</option>` +
		`<option value="__CLEAR__">(clear)</option>`)
	for _, jp := range jobProfiles {
		label := jp.Code + " (" + jp.ID + ")"
		b.WriteString(`<option value="` + html.EscapeString(jp.ID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Name <input type="text" name="name" placeholder="(no change)" /></label><br/>`)
	b.WriteString(`<label>Lifecycle <select name="lifecycle_status">` +
		`<option value="">(no change)</option>` +
		`<option value="active">active</option>` +
		`<option value="disabled">disabled</option>` +
		`</select></label><br/>`)
	b.WriteString(`<button type="submit">Update</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Current</h2>`)
	if len(positions) == 0 {
		b.WriteString("<p>(empty)</p>")
		return b.String()
	}

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr>` +
		`<th>effective_date</th><th>position_id</th><th>reports_to_position_id</th><th>business_unit_id</th><th>jobcatalog_setid</th><th>job_profile</th><th>lifecycle_status</th><th>org_unit_id</th><th>name</th>` +
		`</tr></thead><tbody>`)
	for _, p := range positions {
		jobProfileLabel := ""
		if p.JobProfileID != "" {
			jobProfileLabel = p.JobProfileID
			if p.JobProfileCode != "" {
				jobProfileLabel = p.JobProfileCode + " (" + p.JobProfileID + ")"
			}
		}
		b.WriteString(`<tr><td><code>` + html.EscapeString(p.EffectiveAt) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.ID) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.ReportsToID) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.BusinessUnitID) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.JobCatalogSetID) + `</code></td>` +
			`<td><code>` + html.EscapeString(jobProfileLabel) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.LifecycleStatus) + `</code></td>` +
			`<td><code>` + html.EscapeString(p.OrgUnitID) + `</code></td>` +
			`<td>` + html.EscapeString(p.Name) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func filterActivePositions(in []Position) []Position {
	out := make([]Position, 0, len(in))
	for _, p := range in {
		if p.LifecycleStatus == "active" {
			out = append(out, p)
		}
	}
	return out
}

func renderAssignments(assignments []Assignment, positions []Position, tenant Tenant, asOf string, personUUID string, pernr string, displayName string, errMsg string) string {
	b := strings.Builder{}
	b.WriteString("<h1>Staffing / Assignments</h1>")
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.Name) + `</code> (<code>` + html.EscapeString(tenant.ID) + `</code>)</p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code> | <a href="/org/positions?as_of=` + url.QueryEscape(asOf) + `">Positions</a></p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Select Person</h2>`)
	b.WriteString(`<form method="GET" action="/org/assignments">`)
	b.WriteString(`<input type="hidden" name="as_of" value="` + html.EscapeString(asOf) + `" />`)
	b.WriteString(`<label>Pernr <input type="text" name="pernr" value="` + html.EscapeString(pernr) + `" /></label> `)
	b.WriteString(`<button type="submit">Load</button>`)
	b.WriteString(`</form>`)

	if personUUID != "" {
		label := pernr
		if displayName != "" {
			label = pernr + " / " + displayName
		}
		b.WriteString(`<p>Person: <code>` + html.EscapeString(label) + `</code> (<code>` + html.EscapeString(personUUID) + `</code>)</p>`)
	}

	b.WriteString(`<h2>Upsert Primary</h2>`)
	b.WriteString(`<form method="POST" action="/org/assignments?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Pernr <input type="text" name="pernr" value="` + html.EscapeString(pernr) + `" /></label><br/>`)
	b.WriteString(`<input type="hidden" name="person_uuid" value="` + html.EscapeString(personUUID) + `" />`)
	b.WriteString(`<label>Position <select name="position_id">`)
	if len(positions) == 0 {
		b.WriteString(`<option value="">(no positions)</option>`)
	} else {
		for _, p := range positions {
			label := p.ID
			if p.Name != "" {
				label = p.Name + " (" + p.ID + ")"
			}
			b.WriteString(`<option value="` + html.EscapeString(p.ID) + `">` + html.EscapeString(label) + `</option>`)
		}
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Allocated FTE <input type="number" name="allocated_fte" step="0.01" min="0.01" max="1.00" value="1.0" /></label><br/>`)
	b.WriteString(`<label>Base Salary (CNY) <input type="number" name="base_salary" step="0.01" min="0" /></label><br/>`)
	b.WriteString(`<button type="submit">Submit</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Timeline</h2>`)
	if personUUID == "" {
		b.WriteString("<p>(select a person first)</p>")
		return b.String()
	}
	if len(assignments) == 0 {
		b.WriteString("<p>(empty)</p>")
		return b.String()
	}

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr>` +
		`<th>effective_date</th><th>assignment_id</th><th>position_id</th><th>status</th>` +
		`</tr></thead><tbody>`)
	for _, a := range assignments {
		b.WriteString(`<tr><td><code>` + html.EscapeString(a.EffectiveAt) + `</code></td>` +
			`<td><code>` + html.EscapeString(a.AssignmentID) + `</code></td>` +
			`<td><code>` + html.EscapeString(a.PositionID) + `</code></td>` +
			`<td>` + html.EscapeString(a.Status) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
