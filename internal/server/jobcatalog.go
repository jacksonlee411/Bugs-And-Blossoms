package server

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
)

type JobFamilyGroup struct {
	ID           string
	Code         string
	Name         string
	IsActive     bool
	EffectiveDay string
}

type JobCatalogStore interface {
	ListBusinessUnits(ctx context.Context, tenantID string) ([]BusinessUnit, error)
	ResolveSetID(ctx context.Context, tenantID string, businessUnitID string, recordGroup string) (string, error)
	CreateJobFamilyGroup(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string) error
	ListJobFamilyGroups(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobFamilyGroup, string, error)
}

type jobcatalogPGStore struct {
	pool pgBeginner
}

func newJobCatalogPGStore(pool pgBeginner) JobCatalogStore {
	return &jobcatalogPGStore{pool: pool}
}

type jobcatalogMemoryStore struct {
	businessUnits map[string][]BusinessUnit
	groups        map[string]map[string][]JobFamilyGroup // tenant -> bu -> groups
}

func newJobCatalogMemoryStore() JobCatalogStore {
	return &jobcatalogMemoryStore{
		businessUnits: make(map[string][]BusinessUnit),
		groups:        make(map[string]map[string][]JobFamilyGroup),
	}
}

func (s *jobcatalogMemoryStore) ensure(tenantID string) {
	if _, ok := s.businessUnits[tenantID]; !ok {
		s.businessUnits[tenantID] = []BusinessUnit{{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}}
	}
	if s.groups[tenantID] == nil {
		s.groups[tenantID] = make(map[string][]JobFamilyGroup)
	}
}

func (s *jobcatalogMemoryStore) ListBusinessUnits(_ context.Context, tenantID string) ([]BusinessUnit, error) {
	s.ensure(tenantID)
	return append([]BusinessUnit(nil), s.businessUnits[tenantID]...), nil
}

func (s *jobcatalogMemoryStore) ResolveSetID(_ context.Context, tenantID string, _ string, _ string) (string, error) {
	s.ensure(tenantID)
	return "SHARE", nil
}

func (s *jobcatalogMemoryStore) CreateJobFamilyGroup(_ context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, _ string) error {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	if s.groups[tenantID][businessUnitID] == nil {
		s.groups[tenantID][businessUnitID] = []JobFamilyGroup{}
	}
	s.groups[tenantID][businessUnitID] = append(s.groups[tenantID][businessUnitID], JobFamilyGroup{
		ID:           strconv.Itoa(len(s.groups[tenantID][businessUnitID]) + 1),
		Code:         code,
		Name:         name,
		IsActive:     true,
		EffectiveDay: effectiveDate,
	})
	return nil
}

func (s *jobcatalogMemoryStore) ListJobFamilyGroups(_ context.Context, tenantID string, businessUnitID string, _ string) ([]JobFamilyGroup, string, error) {
	s.ensure(tenantID)
	if businessUnitID == "" {
		businessUnitID = "BU000"
	}
	return append([]JobFamilyGroup(nil), s.groups[tenantID][businessUnitID]...), "SHARE", nil
}

func (s *jobcatalogPGStore) withTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *jobcatalogPGStore) ListBusinessUnits(ctx context.Context, tenantID string) ([]BusinessUnit, error) {
	var out []BusinessUnit
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID)
		if err != nil {
			return err
		}

		rows, err := tx.Query(ctx, `
SELECT business_unit_id, name, status
FROM orgunit.business_units
WHERE tenant_id = $1::uuid
ORDER BY business_unit_id ASC
`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r BusinessUnit
			if err := rows.Scan(&r.BusinessUnitID, &r.Name, &r.Status); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *jobcatalogPGStore) ResolveSetID(ctx context.Context, tenantID string, businessUnitID string, recordGroup string) (string, error) {
	var out string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID)
		if err != nil {
			return err
		}

		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, recordGroup)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

func (s *jobcatalogPGStore) CreateJobFamilyGroup(ctx context.Context, tenantID string, businessUnitID string, effectiveDate string, code string, name string, description string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}

		resolved, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}

		var groupID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&groupID); err != nil {
			return err
		}
		var eventID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
			return err
		}

		payload := `{"code":` + strconv.Quote(code) +
			`,"name":` + strconv.Quote(name)
		if strings.TrimSpace(description) != "" {
			payload += `,"description":` + strconv.Quote(description)
		} else {
			payload += `,"description":null`
		}
		payload += `}`

		_, err = tx.Exec(ctx, `
SELECT jobcatalog.submit_job_family_group_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'CREATE',
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
);
`, eventID, tenantID, resolved, groupID, effectiveDate, []byte(payload), "ui:"+eventID, tenantID)
		return err
	})
}

func (s *jobcatalogPGStore) ListJobFamilyGroups(ctx context.Context, tenantID string, businessUnitID string, asOfDate string) ([]JobFamilyGroup, string, error) {
	var out []JobFamilyGroup
	var resolved string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, tenantID); err != nil {
			return err
		}
		v, err := setid.Resolve(ctx, tx, tenantID, businessUnitID, setid.RecordGroupJobCatalog)
		if err != nil {
			return err
		}
		resolved = v

		rows, err := tx.Query(ctx, `
SELECT
  g.id::text,
  g.code,
  v.name,
  v.is_active,
  lower(v.validity)::text
FROM jobcatalog.job_family_groups g
JOIN jobcatalog.job_family_group_versions v
  ON v.tenant_id = $1::uuid
 AND v.setid = $2::text
 AND v.job_family_group_id = g.id
WHERE g.tenant_id = $1::uuid
  AND g.setid = $2::text
  AND v.validity @> $3::date
ORDER BY g.code ASC
`, tenantID, resolved, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var g JobFamilyGroup
			if err := rows.Scan(&g.ID, &g.Code, &g.Name, &g.IsActive, &g.EffectiveDay); err != nil {
				return err
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, resolved, err
}

func handleJobCatalog(w http.ResponseWriter, r *http.Request, store JobCatalogStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	bus, err := store.ListBusinessUnits(r.Context(), tenant.ID)
	if err != nil {
		writePage(w, r, renderJobCatalog(nil, nil, tenant, "", err.Error(), asOf, ""))
		return
	}

	activeBUs := make([]BusinessUnit, 0, len(bus))
	for _, bu := range bus {
		if bu.Status == "active" {
			activeBUs = append(activeBUs, bu)
		}
	}
	sort.Slice(activeBUs, func(i, j int) bool { return activeBUs[i].BusinessUnitID < activeBUs[j].BusinessUnitID })

	buID := strings.TrimSpace(r.URL.Query().Get("business_unit_id"))
	if buID == "" {
		buID = "BU000"
	}

	list := func(errHint string) (groups []JobFamilyGroup, resolved string, errMsg string) {
		mergeMsg := func(hint string, msg string) string {
			if hint == "" {
				return msg
			}
			if msg == "" {
				return hint
			}
			return hint + "；" + msg
		}

		groups, resolved, err := store.ListJobFamilyGroups(r.Context(), tenant.ID, buID, asOf)
		if err != nil {
			return nil, "", mergeMsg(errHint, err.Error())
		}
		return groups, resolved, errHint
	}

	switch r.Method {
	case http.MethodGet:
		groups, resolved, errMsg := list("")
		writePage(w, r, renderJobCatalog(groups, activeBUs, tenant, buID, errMsg, asOf, resolved))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			groups, resolved, errMsg := list("bad form")
			writePage(w, r, renderJobCatalog(groups, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		action := strings.TrimSpace(strings.ToLower(r.Form.Get("action")))
		if action == "" {
			action = "create_job_family_group"
		}
		if action != "create_job_family_group" {
			groups, resolved, errMsg := list("unknown action")
			writePage(w, r, renderJobCatalog(groups, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			groups, resolved, errMsg := list("effective_date 无效: " + err.Error())
			writePage(w, r, renderJobCatalog(groups, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		formBU := strings.TrimSpace(r.Form.Get("business_unit_id"))
		if formBU != "" {
			buID = formBU
		}

		code := strings.TrimSpace(r.Form.Get("code"))
		name := strings.TrimSpace(r.Form.Get("name"))
		desc := strings.TrimSpace(r.Form.Get("description"))
		if code == "" || name == "" {
			groups, resolved, errMsg := list("code/name is required")
			writePage(w, r, renderJobCatalog(groups, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		if err := store.CreateJobFamilyGroup(r.Context(), tenant.ID, buID, effectiveDate, code, name, desc); err != nil {
			groups, resolved, errMsg := list(err.Error())
			writePage(w, r, renderJobCatalog(groups, activeBUs, tenant, buID, errMsg, asOf, resolved))
			return
		}

		http.Redirect(w, r, "/org/job-catalog?business_unit_id="+url.QueryEscape(buID)+"&as_of="+url.QueryEscape(effectiveDate), http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderJobCatalog(groups []JobFamilyGroup, businessUnits []BusinessUnit, tenant Tenant, businessUnitID string, errMsg string, asOf string, resolvedSetID string) string {
	var b strings.Builder
	b.WriteString("<h1>Job Catalog</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<p><a href="/org/setid" hx-get="/org/setid" hx-target="#content" hx-push-url="true">SetID Governance</a></p>`)

	b.WriteString(`<form method="GET" action="/org/job-catalog">`)
	b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" /></label> `)
	b.WriteString(`<label>Business Unit <select name="business_unit_id">`)
	for _, bu := range businessUnits {
		selected := ""
		if bu.BusinessUnitID == businessUnitID {
			selected = " selected"
		}
		b.WriteString(`<option value="` + html.EscapeString(bu.BusinessUnitID) + `"` + selected + `>` + html.EscapeString(bu.BusinessUnitID) + ` - ` + html.EscapeString(bu.Name) + `</option>`)
	}
	b.WriteString(`</select></label> `)
	b.WriteString(`<button type="submit">Apply</button>`)
	b.WriteString(`</form>`)

	if resolvedSetID != "" {
		b.WriteString(`<p>Resolved SetID: <code>` + html.EscapeString(resolvedSetID) + `</code></p>`)
	}

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	postAction := "/org/job-catalog?business_unit_id=" + url.QueryEscape(businessUnitID) + "&as_of=" + url.QueryEscape(asOf)
	b.WriteString(`<h2>Create Job Family Group</h2>`)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_job_family_group" />`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Business Unit <input name="business_unit_id" value="` + html.EscapeString(businessUnitID) + `" maxlength="5" /></label><br/>`)
	b.WriteString(`<label>Code <input name="code" /></label><br/>`)
	b.WriteString(`<label>Name <input name="name" /></label><br/>`)
	b.WriteString(`<label>Description <input name="description" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Job Family Groups</h2>`)
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>code</th><th>name</th><th>active</th><th>effective_date</th><th>id</th></tr></thead><tbody>`)
	for _, g := range groups {
		active := "false"
		if g.IsActive {
			active = "true"
		}
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(g.Code) + "</td>")
		b.WriteString("<td>" + html.EscapeString(g.Name) + "</td>")
		b.WriteString("<td>" + html.EscapeString(active) + "</td>")
		b.WriteString("<td>" + html.EscapeString(g.EffectiveDay) + "</td>")
		b.WriteString("<td><code>" + html.EscapeString(g.ID) + "</code></td>")
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")

	return b.String()
}
