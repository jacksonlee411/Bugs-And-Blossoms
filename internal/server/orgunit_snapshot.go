package server

import (
	"context"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

type OrgUnitSnapshotRow struct {
	OrgID        string
	OrgCode      string
	ParentID     string
	Name         string
	FullNamePath string
	Depth        int
	ManagerUUID  string
	NodePath     string
}

type OrgUnitSnapshotStore interface {
	GetSnapshot(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitSnapshotRow, error)
	CreateOrgUnit(ctx context.Context, tenantID string, effectiveDate string, name string, parentID string) (string, error)
}

type orgUnitSnapshotPGStore struct {
	pool pgBeginner
}

func newOrgUnitSnapshotPGStore(pool pgBeginner) OrgUnitSnapshotStore {
	return &orgUnitSnapshotPGStore{pool: pool}
}

func (s *orgUnitSnapshotPGStore) GetSnapshot(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitSnapshotRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  s.org_id::text,
	  COALESCE(c.org_code, ''),
	  COALESCE(s.parent_id::text, ''),
	  s.name,
	  COALESCE(s.full_name_path, ''),
	  s.depth,
	  COALESCE(s.manager_uuid::text, ''),
	  s.node_path::text
	FROM orgunit.get_org_snapshot($1::uuid, $2::date) s
	LEFT JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = s.org_id
	ORDER BY node_path
	`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitSnapshotRow
	for rows.Next() {
		var row OrgUnitSnapshotRow
		if err := rows.Scan(&row.OrgID, &row.OrgCode, &row.ParentID, &row.Name, &row.FullNamePath, &row.Depth, &row.ManagerUUID, &row.NodePath); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitSnapshotPGStore) CreateOrgUnit(ctx context.Context, tenantID string, effectiveDate string, name string, parentID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	if _, ok, err := parseOptionalOrgID8(parentID); err != nil {
		return "", err
	} else if ok {
		parentID = strings.TrimSpace(parentID)
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return "", err
	}

	payload := `{"name":` + strconv.Quote(name)
	if strings.TrimSpace(parentID) != "" {
		payload += `,"parent_id":` + strconv.Quote(parentID)
	}
	payload += `}`

	_, err = tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  $3::int,
  'CREATE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, nil, effectiveDate, []byte(payload), eventID, tenantID)
	if err != nil {
		return "", err
	}

	var orgID int
	if err := tx.QueryRow(ctx, `
SELECT org_id
FROM orgunit.org_events
WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
`, tenantID, eventID).Scan(&orgID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return strconv.Itoa(orgID), nil
}

func handleOrgSnapshot(w http.ResponseWriter, r *http.Request, store OrgUnitSnapshotStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}
	createdID := strings.TrimSpace(r.URL.Query().Get("created_id"))

	if store == nil {
		writePage(w, r, renderOrgSnapshot(nil, tenant, asOf, createdID, "store not configured"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := store.GetSnapshot(r.Context(), tenant.ID, asOf)
		if err != nil {
			writePage(w, r, renderOrgSnapshot(nil, tenant, asOf, createdID, err.Error()))
			return
		}
		writePage(w, r, renderOrgSnapshot(rows, tenant, asOf, createdID, ""))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderOrgSnapshot(nil, tenant, asOf, "", "bad form"))
			return
		}

		name := strings.TrimSpace(r.Form.Get("name"))
		parentID := strings.TrimSpace(r.Form.Get("parent_id"))
		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if name == "" {
			rows, _ := store.GetSnapshot(r.Context(), tenant.ID, asOf)
			writePage(w, r, renderOrgSnapshot(rows, tenant, asOf, "", "name is required"))
			return
		}

		createdID, err := store.CreateOrgUnit(r.Context(), tenant.ID, effectiveDate, name, parentID)
		if err != nil {
			rows, _ := store.GetSnapshot(r.Context(), tenant.ID, asOf)
			writePage(w, r, renderOrgSnapshot(rows, tenant, asOf, "", err.Error()))
			return
		}
		http.Redirect(w, r, "/org/snapshot?as_of="+effectiveDate+"&created_id="+createdID, http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderOrgSnapshot(rows []OrgUnitSnapshotRow, tenant Tenant, asOfDate string, createdID string, errMsg string) string {
	var b strings.Builder
	b.WriteString("<h1>OrgUnit Snapshot</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<form method="GET" action="/org/snapshot">`)
	b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOfDate) + `" /></label> `)
	b.WriteString(`<button type="submit">Apply</button>`)
	b.WriteString(`</form>`)

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}
	if createdID != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #0a0;color:#0a0">created <code>` + html.EscapeString(createdID) + `</code></div>`)
	}

	b.WriteString(`<h2>Create</h2>`)
	b.WriteString(`<form method="POST" action="/org/snapshot?as_of=` + html.EscapeString(asOfDate) + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOfDate) + `" /></label><br/>`)
	b.WriteString(`<label>Name <input name="name" /></label><br/>`)
	b.WriteString(`<label>Parent ID (optional) <input name="parent_id" /></label><br/>`)
	b.WriteString(`<button type="submit">Submit CREATE</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Rows</h2>`)
	if len(rows) == 0 {
		b.WriteString("<p>(none)</p>")
		return b.String()
	}

	b.WriteString(`<table border="1" cellpadding="6" cellspacing="0">`)
	b.WriteString(`<thead><tr><th>Depth</th><th>Org Code</th><th>Name</th><th>Full Name</th><th>Org ID</th><th>Parent ID</th><th>Node Path</th></tr></thead>`)
	b.WriteString(`<tbody>`)
	for _, r := range rows {
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(strconv.Itoa(r.Depth)) + "</td>")
		orgCode := r.OrgCode
		if strings.TrimSpace(orgCode) == "" {
			orgCode = "(missing org_code)"
		}
		b.WriteString("<td><code>" + html.EscapeString(orgCode) + "</code></td>")
		b.WriteString("<td>" + html.EscapeString(r.Name) + "</td>")
		b.WriteString("<td>" + html.EscapeString(r.FullNamePath) + "</td>")
		b.WriteString("<td><code>" + html.EscapeString(r.OrgID) + "</code></td>")
		b.WriteString("<td><code>" + html.EscapeString(r.ParentID) + "</code></td>")
		b.WriteString("<td><code>" + html.EscapeString(r.NodePath) + "</code></td>")
		b.WriteString("</tr>")
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
