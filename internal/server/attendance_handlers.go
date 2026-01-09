package server

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

func handleAttendancePunches(w http.ResponseWriter, r *http.Request, store TimePunchStore, personStore PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	loc := shanghaiLocation()

	personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
	fromDate := strings.TrimSpace(r.URL.Query().Get("from_date"))
	toDate := strings.TrimSpace(r.URL.Query().Get("to_date"))
	if fromDate == "" {
		fromDate = asOf
	}
	if toDate == "" {
		toDate = fromDate
	}

	fromMid, err := parseDateInLocation(fromDate, loc)
	if err != nil {
		writePage(w, r, renderAttendancePunches(nil, nil, tenant, asOf, personUUID, fromDate, toDate, "from_date 无效: "+err.Error()))
		return
	}
	toMid, err := parseDateInLocation(toDate, loc)
	if err != nil {
		writePage(w, r, renderAttendancePunches(nil, nil, tenant, asOf, personUUID, fromDate, toDate, "to_date 无效: "+err.Error()))
		return
	}
	if toMid.Before(fromMid) {
		writePage(w, r, renderAttendancePunches(nil, nil, tenant, asOf, personUUID, fromDate, toDate, "to_date 必须 >= from_date"))
		return
	}

	persons, err := personStore.ListPersons(r.Context(), tenant.ID)
	if err != nil {
		writePage(w, r, renderAttendancePunches(nil, nil, tenant, asOf, personUUID, fromDate, toDate, err.Error()))
		return
	}

	fromUTC := fromMid.UTC()
	toUTC := toMid.AddDate(0, 0, 1).UTC()

	list := func() ([]TimePunch, string) {
		if personUUID == "" {
			return nil, ""
		}
		punches, err := store.ListTimePunchesForPerson(r.Context(), tenant.ID, personUUID, fromUTC, toUTC, 2000)
		if err != nil {
			return nil, err.Error()
		}
		return punches, ""
	}

	switch r.Method {
	case http.MethodGet:
		punches, errMsg := list()
		writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, errMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			punches, errMsg := list()
			writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "bad form")))
			return
		}

		principal, ok := currentPrincipal(r.Context())
		if !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_missing", "principal missing")
			return
		}

		op := strings.TrimSpace(r.Form.Get("op"))
		switch op {
		case "manual":
			postPersonUUID := strings.TrimSpace(r.Form.Get("person_uuid"))
			postPunchAt := strings.TrimSpace(r.Form.Get("punch_at"))
			postPunchType := strings.ToUpper(strings.TrimSpace(r.Form.Get("punch_type")))
			note := strings.TrimSpace(r.Form.Get("note"))

			if postPersonUUID == "" {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "person_uuid is required")))
				return
			}

			if postPunchAt == "" {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, postPersonUUID, fromDate, toDate, mergeMsg(errMsg, "punch_at is required")))
				return
			}

			punchAtLocal, err := time.ParseInLocation("2006-01-02T15:04", postPunchAt, loc)
			if err != nil {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, postPersonUUID, fromDate, toDate, mergeMsg(errMsg, "punch_at 无效: "+err.Error())))
				return
			}

			if postPunchType != "IN" && postPunchType != "OUT" {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, postPersonUUID, fromDate, toDate, mergeMsg(errMsg, "punch_type must be IN|OUT")))
				return
			}

			payload := map[string]any{"source": "ui"}
			if note != "" {
				payload["note"] = note
			}
			payloadJSON, _ := json.Marshal(payload)

			if _, err := store.SubmitTimePunch(r.Context(), tenant.ID, principal.ID, submitTimePunchParams{
				PersonUUID:     postPersonUUID,
				PunchTime:      punchAtLocal,
				PunchType:      postPunchType,
				SourceProvider: "MANUAL",
				Payload:        payloadJSON,
			}); err != nil {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, postPersonUUID, fromDate, toDate, mergeMsg(errMsg, err.Error())))
				return
			}

			http.Redirect(w, r, "/org/attendance-punches?as_of="+url.QueryEscape(asOf)+"&person_uuid="+url.QueryEscape(postPersonUUID)+"&from_date="+url.QueryEscape(fromDate)+"&to_date="+url.QueryEscape(toDate), http.StatusSeeOther)
			return

		case "import":
			csvText := r.Form.Get("csv")
			if len(csvText) > 512*1024 {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "csv too large")))
				return
			}

			lines := splitNonEmptyLines(csvText)
			if len(lines) == 0 {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "csv is required")))
				return
			}
			if len(lines) > 2000 {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "too many lines (max 2000)")))
				return
			}

			var events []submitTimePunchParams
			for i, line := range lines {
				parts := strings.Split(line, ",")
				if len(parts) != 3 {
					punches, errMsg := list()
					writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "line "+strconv.Itoa(i+1)+": expected 3 columns")))
					return
				}
				u := strings.TrimSpace(parts[0])
				at := strings.TrimSpace(parts[1])
				typ := strings.ToUpper(strings.TrimSpace(parts[2]))
				if u == "" {
					punches, errMsg := list()
					writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "line "+strconv.Itoa(i+1)+": person_uuid is required")))
					return
				}
				tm, err := time.ParseInLocation("2006-01-02T15:04", at, loc)
				if err != nil {
					punches, errMsg := list()
					writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "line "+strconv.Itoa(i+1)+": punch_at invalid")))
					return
				}
				if typ != "IN" && typ != "OUT" {
					punches, errMsg := list()
					writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "line "+strconv.Itoa(i+1)+": punch_type must be IN|OUT")))
					return
				}
				payloadJSON, _ := json.Marshal(map[string]any{"source": "import"})
				events = append(events, submitTimePunchParams{
					PersonUUID:     u,
					PunchTime:      tm,
					PunchType:      typ,
					SourceProvider: "IMPORT",
					Payload:        payloadJSON,
				})
			}

			if err := store.ImportTimePunches(r.Context(), tenant.ID, principal.ID, events); err != nil {
				punches, errMsg := list()
				writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, err.Error())))
				return
			}

			http.Redirect(w, r, "/org/attendance-punches?as_of="+url.QueryEscape(asOf)+"&from_date="+url.QueryEscape(fromDate)+"&to_date="+url.QueryEscape(toDate), http.StatusSeeOther)
			return
		default:
			punches, errMsg := list()
			writePage(w, r, renderAttendancePunches(punches, persons, tenant, asOf, personUUID, fromDate, toDate, mergeMsg(errMsg, "unsupported op")))
			return
		}
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type attendancePunchesAPIRequest struct {
	EventID          string          `json:"event_id"`
	PersonUUID       string          `json:"person_uuid"`
	PunchTime        string          `json:"punch_time"`
	PunchType        string          `json:"punch_type"`
	SourceProvider   string          `json:"source_provider"`
	Payload          json.RawMessage `json:"payload"`
	SourceRawPayload json.RawMessage `json:"source_raw_payload"`
	DeviceInfo       json.RawMessage `json:"device_info"`
}

func handleAttendancePunchesAPI(w http.ResponseWriter, r *http.Request, store TimePunchStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "principal_missing", "principal missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		personUUID := strings.TrimSpace(r.URL.Query().Get("person_uuid"))
		if personUUID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "missing_person_uuid", "person_uuid is required")
			return
		}

		now := time.Now().UTC()
		from := now.Add(-24 * time.Hour)
		to := now

		if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
			tm, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_from", "invalid from")
				return
			}
			from = tm.UTC()
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
			tm, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_to", "invalid to")
				return
			}
			to = tm.UTC()
		}

		limit := 200
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_limit", "invalid limit")
				return
			}
			limit = n
		}
		if limit > 1000 {
			limit = 1000
		}
		if limit < 1 {
			limit = 1
		}

		punches, err := store.ListTimePunchesForPerson(r.Context(), tenant.ID, personUUID, from, to, limit)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "list_failed", "list failed")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant":      tenant.ID,
			"person_uuid": personUUID,
			"from":        from.Format(time.RFC3339Nano),
			"to":          to.Format(time.RFC3339Nano),
			"punches":     punches,
		})
		return

	case http.MethodPost:
		var req attendancePunchesAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}

		if strings.TrimSpace(req.PersonUUID) == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "missing_person_uuid", "person_uuid is required")
			return
		}

		punchTime, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(req.PunchTime))
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_punch_time", "invalid punch_time")
			return
		}

		req.PunchType = strings.ToUpper(strings.TrimSpace(req.PunchType))
		if req.PunchType != "IN" && req.PunchType != "OUT" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_punch_type", "invalid punch_type")
			return
		}

		req.SourceProvider = strings.ToUpper(strings.TrimSpace(req.SourceProvider))
		if req.SourceProvider == "" {
			req.SourceProvider = "MANUAL"
		}
		if req.SourceProvider != "MANUAL" && req.SourceProvider != "IMPORT" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_source_provider", "invalid source_provider")
			return
		}

		p, err := store.SubmitTimePunch(r.Context(), tenant.ID, principal.ID, submitTimePunchParams{
			EventID:          strings.TrimSpace(req.EventID),
			PersonUUID:       strings.TrimSpace(req.PersonUUID),
			PunchTime:        punchTime.UTC(),
			PunchType:        req.PunchType,
			SourceProvider:   req.SourceProvider,
			Payload:          req.Payload,
			SourceRawPayload: req.SourceRawPayload,
			DeviceInfo:       req.DeviceInfo,
		})
		if err != nil {
			if isSTAFFING_IDEMPOTENCY_REUSED(err) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "idempotency_reused", "idempotency reused")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "submit_failed", "submit failed")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(p)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func parseDateInLocation(date string, loc *time.Location) (time.Time, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return time.Time{}, errors.New("empty date")
	}
	tm, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, loc), nil
}

func splitNonEmptyLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.Split(raw, "\n")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func renderAttendancePunches(punches []TimePunch, persons []Person, tenant Tenant, asOf string, personUUID string, fromDate string, toDate string, errMsg string) string {
	loc := shanghaiLocation()

	b := strings.Builder{}
	b.WriteString("<h1>Attendance / Punches</h1>")
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.Name) + `</code> (<code>` + html.EscapeString(tenant.ID) + `</code>)</p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code></p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Query</h2>`)
	b.WriteString(`<form method="GET" action="/org/attendance-punches" hx-get="/org/attendance-punches" hx-target="#content" hx-push-url="true">`)
	b.WriteString(`<input type="hidden" name="as_of" value="` + html.EscapeString(asOf) + `"/>`)
	b.WriteString(`<label>Person <select name="person_uuid">`)
	b.WriteString(`<option value=""></option>`)
	for _, p := range persons {
		selected := ""
		if p.UUID == personUUID {
			selected = ` selected`
		}
		b.WriteString(`<option value="` + html.EscapeString(p.UUID) + `"` + selected + `>` + html.EscapeString(p.DisplayName) + ` (` + html.EscapeString(p.Pernr) + `) / ` + html.EscapeString(p.UUID) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>From <input type="date" name="from_date" value="` + html.EscapeString(fromDate) + `"/></label><br/>`)
	b.WriteString(`<label>To <input type="date" name="to_date" value="` + html.EscapeString(toDate) + `"/></label><br/>`)
	b.WriteString(`<button type="submit">Query</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Manual</h2>`)
	b.WriteString(`<form method="POST" action="/org/attendance-punches?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="op" value="manual"/>`)
	b.WriteString(`<label>Person <select name="person_uuid">`)
	b.WriteString(`<option value=""></option>`)
	for _, p := range persons {
		selected := ""
		if p.UUID == personUUID {
			selected = ` selected`
		}
		b.WriteString(`<option value="` + html.EscapeString(p.UUID) + `"` + selected + `>` + html.EscapeString(p.DisplayName) + ` (` + html.EscapeString(p.Pernr) + `) / ` + html.EscapeString(p.UUID) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>Punch At (Beijing) <input type="datetime-local" name="punch_at" required/></label><br/>`)
	b.WriteString(`<label>Type <select name="punch_type"><option value="IN">IN</option><option value="OUT">OUT</option></select></label><br/>`)
	b.WriteString(`<label>Note <input type="text" name="note"/></label><br/>`)
	b.WriteString(`<button type="submit">Submit</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Import (CSV)</h2>`)
	b.WriteString(`<form method="POST" action="/org/attendance-punches?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="op" value="import"/>`)
	b.WriteString(`<p>Format: <code>person_uuid,punch_at,punch_type</code> (punch_at uses Beijing datetime-local <code>YYYY-MM-DDTHH:MM</code>)</p>`)
	b.WriteString(`<textarea name="csv" rows="8" style="width:100%"></textarea><br/>`)
	b.WriteString(`<button type="submit">Import</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Results</h2>`)
	if personUUID == "" {
		b.WriteString(`<p>(pick a person)</p>`)
		return b.String()
	}
	if len(punches) == 0 {
		b.WriteString(`<p>(no punches)</p>`)
		return b.String()
	}

	b.WriteString(`<table border="1" cellpadding="4" cellspacing="0">`)
	b.WriteString(`<tr><th>Punch Time (Beijing)</th><th>Type</th><th>Source</th><th>Event</th><th>Tx Time (UTC)</th></tr>`)
	for _, p := range punches {
		bt := p.PunchTime.In(loc).Format("2006-01-02 15:04")
		tx := p.TransactionTime.UTC().Format(time.RFC3339)
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(bt) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.PunchType) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.SourceProvider) + `</td>`)
		b.WriteString(`<td><code>` + html.EscapeString(p.EventID) + `</code></td>`)
		b.WriteString(`<td><code>` + html.EscapeString(tx) + `</code></td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	return b.String()
}

func shanghaiLocation() *time.Location {
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}
