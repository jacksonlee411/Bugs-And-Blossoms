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

func handleAttendanceTimeProfile(w http.ResponseWriter, r *http.Request, store AttendanceConfigStore) {
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

	current, okCurrent, err := store.GetTimeProfileAsOf(r.Context(), tenant.ID, asOf)
	if err != nil {
		writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, nil, nil, err.Error(), nil))
		return
	}
	versions, err := store.ListTimeProfileVersions(r.Context(), tenant.ID, 50)
	if err != nil {
		writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, map[string]string{}, nil, err.Error(), nil))
		return
	}

	switch r.Method {
	case http.MethodGet:
		var cur *TimeProfileVersion
		if okCurrent {
			cur = &current
		}
		writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, map[string]string{}, cur, "", versions))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, map[string]string{}, cur, "bad form", versions))
			return
		}

		principal, ok := currentPrincipal(r.Context())
		if !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_missing", "principal missing")
			return
		}

		op := strings.TrimSpace(r.Form.Get("op"))
		if op != "save" {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, map[string]string{}, cur, "unsupported op", versions))
			return
		}

		form := map[string]string{
			"effective_date":                 strings.TrimSpace(r.Form.Get("effective_date")),
			"shift_start_local":              strings.TrimSpace(r.Form.Get("shift_start_local")),
			"shift_end_local":                strings.TrimSpace(r.Form.Get("shift_end_local")),
			"late_tolerance_minutes":         strings.TrimSpace(r.Form.Get("late_tolerance_minutes")),
			"early_leave_tolerance_minutes":  strings.TrimSpace(r.Form.Get("early_leave_tolerance_minutes")),
			"overtime_min_minutes":           strings.TrimSpace(r.Form.Get("overtime_min_minutes")),
			"overtime_rounding_mode":         strings.ToUpper(strings.TrimSpace(r.Form.Get("overtime_rounding_mode"))),
			"overtime_rounding_unit_minutes": strings.TrimSpace(r.Form.Get("overtime_rounding_unit_minutes")),
			"name":                           strings.TrimSpace(r.Form.Get("name")),
		}

		if form["effective_date"] == "" {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "effective_date is required", versions))
			return
		}
		if _, err := parseDateInLocation(form["effective_date"], loc); err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "effective_date 无效", versions))
			return
		}
		if form["shift_start_local"] == "" || form["shift_end_local"] == "" {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "shift_start_local and shift_end_local are required", versions))
			return
		}
		startMin, err := parseHHMMToMinutes(form["shift_start_local"])
		if err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "shift_start_local 无效", versions))
			return
		}
		endMin, err := parseHHMMToMinutes(form["shift_end_local"])
		if err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "shift_end_local 无效", versions))
			return
		}
		if endMin <= startMin {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "shift_end_local must be > shift_start_local", versions))
			return
		}

		lateTol, err := parseOptionalNonNegInt(form["late_tolerance_minutes"])
		if err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "late_tolerance_minutes 无效", versions))
			return
		}
		earlyTol, err := parseOptionalNonNegInt(form["early_leave_tolerance_minutes"])
		if err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "early_leave_tolerance_minutes 无效", versions))
			return
		}
		otMin, err := parseOptionalNonNegInt(form["overtime_min_minutes"])
		if err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "overtime_min_minutes 无效", versions))
			return
		}
		otUnit, err := parseOptionalNonNegInt(form["overtime_rounding_unit_minutes"])
		if err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "overtime_rounding_unit_minutes 无效", versions))
			return
		}
		otMode := form["overtime_rounding_mode"]
		if otMode == "" {
			otMode = "NONE"
		}
		if otMode != "NONE" && otMode != "FLOOR" && otMode != "CEIL" && otMode != "NEAREST" {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, "overtime_rounding_mode must be NONE|FLOOR|CEIL|NEAREST", versions))
			return
		}

		payload := map[string]any{
			"shift_start_local":              form["shift_start_local"],
			"shift_end_local":                form["shift_end_local"],
			"late_tolerance_minutes":         lateTol,
			"early_leave_tolerance_minutes":  earlyTol,
			"overtime_min_minutes":           otMin,
			"overtime_rounding_mode":         otMode,
			"overtime_rounding_unit_minutes": otUnit,
			"lifecycle_status":               "active",
		}
		if form["name"] != "" {
			payload["name"] = form["name"]
		}

		if err := store.UpsertTimeProfile(r.Context(), tenant.ID, principal.ID, form["effective_date"], payload); err != nil {
			var cur *TimeProfileVersion
			if okCurrent {
				cur = &current
			}
			writePage(w, r, renderAttendanceTimeProfile(tenant, asOf, form, cur, err.Error(), versions))
			return
		}

		http.Redirect(w, r, "/org/attendance-time-profile?as_of="+url.QueryEscape(asOf), http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleAttendanceHolidayCalendar(w http.ResponseWriter, r *http.Request, store AttendanceConfigStore) {
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

	month := strings.TrimSpace(r.URL.Query().Get("month"))
	if month == "" {
		month = asOf[:7]
	}
	monthStart, monthEnd, err := monthBounds(month, loc)
	if err != nil {
		writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, nil, "month 无效"))
		return
	}

	load := func() ([]HolidayDayOverride, map[string]HolidayDayOverride, string) {
		overrides, err := store.ListHolidayDayOverrides(r.Context(), tenant.ID, monthStart.Format("2006-01-02"), monthEnd.Format("2006-01-02"), 5000)
		if err != nil {
			return nil, nil, err.Error()
		}
		byDate := make(map[string]HolidayDayOverride, len(overrides))
		for _, o := range overrides {
			byDate[o.DayDate] = o
		}
		return overrides, byDate, ""
	}

	switch r.Method {
	case http.MethodGet:
		_, byDate, errMsg := load()
		writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, errMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			_, byDate, _ := load()
			writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "bad form"))
			return
		}

		principal, ok := currentPrincipal(r.Context())
		if !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_missing", "principal missing")
			return
		}

		op := strings.TrimSpace(r.Form.Get("op"))
		switch op {
		case "day_set":
			dayDate := strings.TrimSpace(r.Form.Get("day_date"))
			dayType := strings.ToUpper(strings.TrimSpace(r.Form.Get("day_type")))
			holidayCode := strings.TrimSpace(r.Form.Get("holiday_code"))
			note := strings.TrimSpace(r.Form.Get("note"))

			if dayDate == "" {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "day_date is required"))
				return
			}
			if _, err := parseDateInLocation(dayDate, loc); err != nil {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "day_date 无效"))
				return
			}
			if dayType != "WORKDAY" && dayType != "RESTDAY" && dayType != "LEGAL_HOLIDAY" {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "day_type must be WORKDAY|RESTDAY|LEGAL_HOLIDAY"))
				return
			}

			payload := map[string]any{
				"day_type": dayType,
			}
			if holidayCode != "" {
				payload["holiday_code"] = holidayCode
			}
			if note != "" {
				payload["note"] = note
			}
			if err := store.SetHolidayDayOverride(r.Context(), tenant.ID, principal.ID, dayDate, payload); err != nil {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, err.Error()))
				return
			}

			http.Redirect(w, r, "/org/attendance-holiday-calendar?as_of="+url.QueryEscape(asOf)+"&month="+url.QueryEscape(month), http.StatusSeeOther)
			return

		case "day_clear":
			dayDate := strings.TrimSpace(r.Form.Get("day_date"))
			if dayDate == "" {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "day_date is required"))
				return
			}
			if _, err := parseDateInLocation(dayDate, loc); err != nil {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "day_date 无效"))
				return
			}

			if err := store.ClearHolidayDayOverride(r.Context(), tenant.ID, principal.ID, dayDate); err != nil {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, err.Error()))
				return
			}

			http.Redirect(w, r, "/org/attendance-holiday-calendar?as_of="+url.QueryEscape(asOf)+"&month="+url.QueryEscape(month), http.StatusSeeOther)
			return

		case "import_csv":
			csvText := r.Form.Get("csv")
			if len(csvText) > 256*1024 {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "csv too large"))
				return
			}
			lines := splitNonEmptyLines(csvText)
			if len(lines) == 0 {
				_, byDate, _ := load()
				writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "csv is required"))
				return
			}

			for i, line := range lines {
				parts := strings.Split(line, ",")
				if len(parts) < 2 || len(parts) > 4 {
					_, byDate, _ := load()
					writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "line "+strconv.Itoa(i+1)+": expected 2-4 columns"))
					return
				}
				dayDate := strings.TrimSpace(parts[0])
				dayType := strings.ToUpper(strings.TrimSpace(parts[1]))
				var holidayCode string
				var note string
				if len(parts) >= 3 {
					holidayCode = strings.TrimSpace(parts[2])
				}
				if len(parts) >= 4 {
					note = strings.TrimSpace(parts[3])
				}

				if dayDate == "" {
					_, byDate, _ := load()
					writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "line "+strconv.Itoa(i+1)+": day_date is required"))
					return
				}
				if _, err := parseDateInLocation(dayDate, loc); err != nil {
					_, byDate, _ := load()
					writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "line "+strconv.Itoa(i+1)+": invalid day_date"))
					return
				}
				if dayType != "WORKDAY" && dayType != "RESTDAY" && dayType != "LEGAL_HOLIDAY" {
					_, byDate, _ := load()
					writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "line "+strconv.Itoa(i+1)+": invalid day_type"))
					return
				}

				payload := map[string]any{"day_type": dayType}
				if holidayCode != "" {
					payload["holiday_code"] = holidayCode
				}
				if note != "" {
					payload["note"] = note
				}
				if err := store.SetHolidayDayOverride(r.Context(), tenant.ID, principal.ID, dayDate, payload); err != nil {
					_, byDate, _ := load()
					writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "line "+strconv.Itoa(i+1)+": "+err.Error()))
					return
				}
			}

			http.Redirect(w, r, "/org/attendance-holiday-calendar?as_of="+url.QueryEscape(asOf)+"&month="+url.QueryEscape(month), http.StatusSeeOther)
			return

		default:
			_, byDate, _ := load()
			writePage(w, r, renderAttendanceHolidayCalendar(tenant, asOf, month, byDate, "unsupported op"))
			return
		}
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func parseOptionalNonNegInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errors.New("invalid int")
	}
	return n, nil
}

func parseHHMMToMinutes(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	tm, err := time.Parse("15:04", raw)
	if err != nil {
		return 0, err
	}
	return tm.Hour()*60 + tm.Minute(), nil
}

func monthBounds(month string, loc *time.Location) (time.Time, time.Time, error) {
	m, err := time.ParseInLocation("2006-01", month, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start := time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	return start, end, nil
}

func baselineDayType(day time.Time) string {
	iso := day.Weekday()
	if iso == time.Saturday || iso == time.Sunday {
		return "RESTDAY"
	}
	return "WORKDAY"
}

func renderAttendanceTimeProfile(tenant Tenant, asOf string, form map[string]string, current *TimeProfileVersion, errMsg string, versions []TimeProfileVersion) string {
	if form == nil {
		form = map[string]string{}
	}
	var b strings.Builder
	b.WriteString(`<h1>Attendance / TimeProfile</h1>`)
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.ID) + `</code></p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code> (timezone: Asia/Shanghai)</p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Current (as-of)</h2>`)
	if current == nil {
		b.WriteString(`<p>(no active time profile as-of)</p>`)
	} else {
		raw, _ := json.Marshal(current)
		b.WriteString(`<pre>` + html.EscapeString(string(raw)) + `</pre>`)
	}

	b.WriteString(`<h2>Save</h2>`)
	b.WriteString(`<form method="POST" action="/org/attendance-time-profile?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="op" value="save"/>`)
	b.WriteString(`<label>effective_date <input type="date" name="effective_date" value="` + html.EscapeString(form["effective_date"]) + `" required/></label><br/>`)
	b.WriteString(`<label>shift_start_local <input type="time" name="shift_start_local" value="` + html.EscapeString(form["shift_start_local"]) + `" required/></label><br/>`)
	b.WriteString(`<label>shift_end_local <input type="time" name="shift_end_local" value="` + html.EscapeString(form["shift_end_local"]) + `" required/></label><br/>`)
	b.WriteString(`<label>late_tolerance_minutes <input type="number" min="0" name="late_tolerance_minutes" value="` + html.EscapeString(form["late_tolerance_minutes"]) + `"/></label><br/>`)
	b.WriteString(`<label>early_leave_tolerance_minutes <input type="number" min="0" name="early_leave_tolerance_minutes" value="` + html.EscapeString(form["early_leave_tolerance_minutes"]) + `"/></label><br/>`)
	b.WriteString(`<label>overtime_min_minutes <input type="number" min="0" name="overtime_min_minutes" value="` + html.EscapeString(form["overtime_min_minutes"]) + `"/></label><br/>`)
	b.WriteString(`<label>overtime_rounding_mode <select name="overtime_rounding_mode">`)
	for _, opt := range []string{"NONE", "FLOOR", "CEIL", "NEAREST"} {
		sel := ""
		if strings.ToUpper(form["overtime_rounding_mode"]) == opt {
			sel = " selected"
		}
		b.WriteString(`<option value="` + opt + `"` + sel + `>` + opt + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<label>overtime_rounding_unit_minutes <input type="number" min="0" name="overtime_rounding_unit_minutes" value="` + html.EscapeString(form["overtime_rounding_unit_minutes"]) + `"/></label><br/>`)
	b.WriteString(`<label>name <input type="text" name="name" value="` + html.EscapeString(form["name"]) + `"/></label><br/>`)
	b.WriteString(`<button type="submit">Save</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Versions</h2>`)
	if len(versions) == 0 {
		b.WriteString(`<p>(no versions)</p>`)
		return b.String()
	}
	b.WriteString(`<table border="1" cellpadding="4" cellspacing="0">`)
	b.WriteString(`<tr><th>effective_date</th><th>shift</th><th>tolerance</th><th>overtime</th><th>name</th><th>last_event_id</th></tr>`)
	for _, v := range versions {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(v.EffectiveDate) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.ShiftStartLocal) + `–` + html.EscapeString(v.ShiftEndLocal) + `</td>`)
		b.WriteString(`<td>late=` + strconv.Itoa(v.LateToleranceMinutes) + ` early=` + strconv.Itoa(v.EarlyLeaveToleranceMinutes) + `</td>`)
		b.WriteString(`<td>min=` + strconv.Itoa(v.OvertimeMinMinutes) + ` mode=` + html.EscapeString(v.OvertimeRoundingMode) + ` unit=` + strconv.Itoa(v.OvertimeRoundingUnitMinutes) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(v.Name) + `</td>`)
		b.WriteString(`<td><code>` + strconv.FormatInt(v.LastEventDBID, 10) + `</code></td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	return b.String()
}

func renderAttendanceHolidayCalendar(tenant Tenant, asOf string, month string, byDate map[string]HolidayDayOverride, errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Attendance / HolidayCalendar</h1>`)
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.ID) + `</code></p>`)
	b.WriteString(`<p>As-of: <code>` + html.EscapeString(asOf) + `</code> (timezone: Asia/Shanghai)</p>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Month</h2>`)
	b.WriteString(`<form method="GET" action="/org/attendance-holiday-calendar" hx-get="/org/attendance-holiday-calendar" hx-target="#content" hx-push-url="true">`)
	b.WriteString(`<input type="hidden" name="as_of" value="` + html.EscapeString(asOf) + `"/>`)
	b.WriteString(`<label>month <input type="month" name="month" value="` + html.EscapeString(month) + `"/></label>`)
	b.WriteString(`<button type="submit">Go</button>`)
	b.WriteString(`</form>`)

	loc := shanghaiLocation()
	monthStart, monthEnd, err := monthBounds(month, loc)
	if err != nil {
		b.WriteString(`<p>(invalid month)</p>`)
		return b.String()
	}

	b.WriteString(`<h2>Overrides</h2>`)
	b.WriteString(`<table border="1" cellpadding="4" cellspacing="0">`)
	b.WriteString(`<tr><th>date</th><th>weekday</th><th>baseline</th><th>effective</th><th>override?</th><th>actions</th></tr>`)
	for d := monthStart; d.Before(monthEnd); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		base := baselineDayType(d)
		eff := base
		override := ""
		var code string
		var note string
		if byDate != nil {
			if o, ok := byDate[ds]; ok {
				eff = o.DayType
				override = "yes"
				code = o.HolidayCode
				note = o.Note
			}
		}

		b.WriteString(`<tr>`)
		b.WriteString(`<td><code>` + ds + `</code></td>`)
		b.WriteString(`<td>` + d.Weekday().String() + `</td>`)
		b.WriteString(`<td>` + base + `</td>`)
		b.WriteString(`<td>` + eff + `</td>`)
		b.WriteString(`<td>` + override + `</td>`)
		b.WriteString(`<td>`)
		b.WriteString(`<form method="POST" action="/org/attendance-holiday-calendar?as_of=` + url.QueryEscape(asOf) + `&month=` + url.QueryEscape(month) + `" style="display:inline">`)
		b.WriteString(`<input type="hidden" name="op" value="day_set"/>`)
		b.WriteString(`<input type="hidden" name="day_date" value="` + ds + `"/>`)
		b.WriteString(`<select name="day_type">`)
		for _, opt := range []string{"WORKDAY", "RESTDAY", "LEGAL_HOLIDAY"} {
			sel := ""
			if eff == opt {
				sel = " selected"
			}
			b.WriteString(`<option value="` + opt + `"` + sel + `>` + opt + `</option>`)
		}
		b.WriteString(`</select>`)
		b.WriteString(`<input type="text" name="holiday_code" value="` + html.EscapeString(code) + `" placeholder="holiday_code" size="10"/>`)
		b.WriteString(`<input type="text" name="note" value="` + html.EscapeString(note) + `" placeholder="note" size="16"/>`)
		b.WriteString(`<button type="submit">Set</button>`)
		b.WriteString(`</form>`)
		b.WriteString(` `)
		b.WriteString(`<form method="POST" action="/org/attendance-holiday-calendar?as_of=` + url.QueryEscape(asOf) + `&month=` + url.QueryEscape(month) + `" style="display:inline">`)
		b.WriteString(`<input type="hidden" name="op" value="day_clear"/>`)
		b.WriteString(`<input type="hidden" name="day_date" value="` + ds + `"/>`)
		b.WriteString(`<button type="submit">Clear</button>`)
		b.WriteString(`</form>`)
		b.WriteString(`</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)

	b.WriteString(`<h2>Import (CSV)</h2>`)
	b.WriteString(`<form method="POST" action="/org/attendance-holiday-calendar?as_of=` + url.QueryEscape(asOf) + `&month=` + url.QueryEscape(month) + `">`)
	b.WriteString(`<input type="hidden" name="op" value="import_csv"/>`)
	b.WriteString(`<p>Format: <code>YYYY-MM-DD,DAY_TYPE[,HOLIDAY_CODE][,NOTE]</code> (DAY_TYPE: WORKDAY|RESTDAY|LEGAL_HOLIDAY)</p>`)
	b.WriteString(`<textarea name="csv" rows="8" style="width:100%"></textarea><br/>`)
	b.WriteString(`<button type="submit">Import</button>`)
	b.WriteString(`</form>`)

	return b.String()
}
