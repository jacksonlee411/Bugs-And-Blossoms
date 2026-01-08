package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type OrgUnitNode struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type OrgUnitStore interface {
	OrgUnitNodesCurrentReader
	OrgUnitNodesCurrentWriter
	OrgUnitNodesCurrentRenamer
	OrgUnitNodesCurrentMover
	OrgUnitNodesCurrentDisabler
}

type OrgUnitNodesCurrentReader interface {
	ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
}

type OrgUnitNodesCurrentWriter interface {
	CreateNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, name string, parentID string) (OrgUnitNode, error)
}

type OrgUnitNodesCurrentRenamer interface {
	RenameNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, newName string) error
}

type OrgUnitNodesCurrentMover interface {
	MoveNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error
}

type OrgUnitNodesCurrentDisabler interface {
	DisableNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string) error
}

type orgUnitPGStore struct {
	pool pgBeginner
}

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func newOrgUnitPGStore(pool pgBeginner) OrgUnitStore {
	return &orgUnitPGStore{pool: pool}
}

func (s *orgUnitPGStore) ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
WITH snapshot AS (
  SELECT org_id, name
  FROM orgunit.get_org_snapshot($1::uuid, $2::date)
)
SELECT
  s.org_id::text,
  s.name,
  e.transaction_time
FROM snapshot s
JOIN orgunit.org_unit_versions v
  ON v.tenant_id = $1::uuid
 AND v.hierarchy_type = 'OrgUnit'
 AND v.org_id = s.org_id
 AND v.status = 'active'
 AND v.validity @> $2::date
JOIN orgunit.org_events e
  ON e.id = v.last_event_id
ORDER BY e.transaction_time DESC
`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNode
	for rows.Next() {
		var n OrgUnitNode
		if err := rows.Scan(&n.ID, &n.Name, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) CreateNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, name string, parentID string) (OrgUnitNode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitNode{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitNode{}, err
	}

	if strings.TrimSpace(effectiveDate) == "" {
		return OrgUnitNode{}, errors.New("effective_date is required")
	}

	var orgID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&orgID); err != nil {
		return OrgUnitNode{}, err
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return OrgUnitNode{}, err
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
  'OrgUnit',
  $3::uuid,
  'CREATE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, orgID, effectiveDate, []byte(payload), eventID, tenantID)
	if err != nil {
		return OrgUnitNode{}, err
	}

	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
SELECT transaction_time
FROM orgunit.org_events
WHERE tenant_id = $1::uuid AND event_id = $2::uuid
`, tenantID, eventID).Scan(&createdAt); err != nil {
		return OrgUnitNode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNode{}, err
	}

	return OrgUnitNode{ID: orgID, Name: name, CreatedAt: createdAt}, nil
}

func (s *orgUnitPGStore) RenameNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, newName string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if strings.TrimSpace(effectiveDate) == "" {
		return errors.New("effective_date is required")
	}

	if strings.TrimSpace(orgID) == "" {
		return errors.New("org_id is required")
	}
	if strings.TrimSpace(newName) == "" {
		return errors.New("new_name is required")
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return err
	}

	payload := `{"new_name":` + strconv.Quote(newName) + `}`

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'RENAME',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, orgID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgUnitPGStore) MoveNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if strings.TrimSpace(effectiveDate) == "" {
		return errors.New("effective_date is required")
	}

	if strings.TrimSpace(orgID) == "" {
		return errors.New("org_id is required")
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return err
	}

	payload := `{}`
	if strings.TrimSpace(newParentID) != "" {
		payload = `{"new_parent_id":` + strconv.Quote(newParentID) + `}`
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'MOVE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, orgID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgUnitPGStore) DisableNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if strings.TrimSpace(effectiveDate) == "" {
		return errors.New("effective_date is required")
	}

	if strings.TrimSpace(orgID) == "" {
		return errors.New("org_id is required")
	}

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::uuid,
  'DISABLE',
  $4::date,
  '{}'::jsonb,
  $5::text,
  $6::uuid
)
`, eventID, tenantID, orgID, effectiveDate, eventID, tenantID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

type orgUnitMemoryStore struct {
	nodes map[string][]OrgUnitNode
	now   func() time.Time
}

func newOrgUnitMemoryStore() *orgUnitMemoryStore {
	return &orgUnitMemoryStore{
		nodes: make(map[string][]OrgUnitNode),
		now:   time.Now,
	}
}

func (s *orgUnitMemoryStore) listNodes(tenantID string) ([]OrgUnitNode, error) {
	return append([]OrgUnitNode(nil), s.nodes[tenantID]...), nil
}

func (s *orgUnitMemoryStore) createNode(tenantID string, name string) (OrgUnitNode, error) {
	n := OrgUnitNode{
		ID:        "mem-" + strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		Name:      name,
		CreatedAt: s.now(),
	}
	s.nodes[tenantID] = append([]OrgUnitNode{n}, s.nodes[tenantID]...)
	return n, nil
}

func (s *orgUnitMemoryStore) ListNodesCurrent(_ context.Context, tenantID string, _ string) ([]OrgUnitNode, error) {
	return s.listNodes(tenantID)
}

func (s *orgUnitMemoryStore) CreateNodeCurrent(_ context.Context, tenantID string, _ string, name string, _ string) (OrgUnitNode, error) {
	return s.createNode(tenantID, name)
}

func (s *orgUnitMemoryStore) RenameNodeCurrent(_ context.Context, tenantID string, _ string, orgID string, newName string) error {
	if strings.TrimSpace(orgID) == "" {
		return errors.New("org_id is required")
	}
	if strings.TrimSpace(newName) == "" {
		return errors.New("new_name is required")
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == orgID {
			nodes[i].Name = newName
			s.nodes[tenantID] = nodes
			return nil
		}
	}
	return errors.New("org_id not found")
}

func (s *orgUnitMemoryStore) MoveNodeCurrent(_ context.Context, tenantID string, _ string, orgID string, _ string) error {
	if strings.TrimSpace(orgID) == "" {
		return errors.New("org_id is required")
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == orgID {
			return nil
		}
	}
	return errors.New("org_id not found")
}

func (s *orgUnitMemoryStore) DisableNodeCurrent(_ context.Context, tenantID string, _ string, orgID string) error {
	if strings.TrimSpace(orgID) == "" {
		return errors.New("org_id is required")
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == orgID {
			s.nodes[tenantID] = append(nodes[:i], nodes[i+1:]...)
			return nil
		}
	}
	return errors.New("org_id not found")
}

func handleOrgNodes(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	listNodes := func(errHint string) ([]OrgUnitNode, string) {
		mergeMsg := func(hint string, msg string) string {
			if hint == "" {
				return msg
			}
			if msg == "" {
				return hint
			}
			return hint + "；" + msg
		}

		nodes, err := store.ListNodesCurrent(r.Context(), tenant.ID, asOf)
		if err != nil {
			return nil, mergeMsg(errHint, err.Error())
		}
		return nodes, errHint
	}

	switch r.Method {
	case http.MethodGet:
		nodes, errMsg := listNodes("")
		writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			nodes, errMsg := listNodes("bad form")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
			return
		}
		action := strings.TrimSpace(strings.ToLower(r.Form.Get("action")))
		if action == "" {
			action = "create"
		}

		if action == "rename" || action == "move" || action == "disable" {
			effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
			if effectiveDate == "" {
				effectiveDate = asOf
			}
			if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
				nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
				return
			}

			orgID := strings.TrimSpace(r.Form.Get("org_id"))
			if orgID == "" {
				nodes, errMsg := listNodes("org_id is required")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
				return
			}

			switch action {
			case "rename":
				newName := strings.TrimSpace(r.Form.Get("new_name"))
				if newName == "" {
					nodes, errMsg := listNodes("new_name is required")
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
					return
				}

				if err := store.RenameNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID, newName); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
					return
				}
			case "move":
				newParentID := strings.TrimSpace(r.Form.Get("new_parent_id"))

				if err := store.MoveNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID, newParentID); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
					return
				}
			case "disable":

				if err := store.DisableNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
					return
				}
			default:
			}

			http.Redirect(w, r, "/org/nodes?as_of="+effectiveDate, http.StatusSeeOther)
			return
		}

		name := strings.TrimSpace(r.Form.Get("name"))
		if name == "" {
			nodes, errMsg := listNodes("name is required")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		parentID := strings.TrimSpace(r.Form.Get("parent_id"))
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
			return
		}

		if _, err := store.CreateNodeCurrent(r.Context(), tenant.ID, effectiveDate, name, parentID); err != nil {
			nodes, errMsg := listNodes(err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
			return
		}

		http.Redirect(w, r, "/org/nodes?as_of="+effectiveDate, http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderOrgNodes(nodes []OrgUnitNode, tenant Tenant, errMsg string, asOf string) string {
	var b strings.Builder
	b.WriteString("<h1>OrgUnit</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<form method="GET" action="/org/nodes">`)
	b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" /></label> `)
	b.WriteString(`<button type="submit">Apply</button>`)
	b.WriteString(`</form>`)

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	postAction := "/org/nodes?as_of=" + html.EscapeString(asOf)
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
	b.WriteString(`<label>Parent ID (optional) <input name="parent_id" /></label><br/>`)
	b.WriteString(`<label>Name <input name="name" /></label>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString("<h2>Nodes</h2>")
	if len(nodes) == 0 {
		b.WriteString("<p>(none)</p>")
		return b.String()
	}

	b.WriteString("<ul>")
	for _, n := range nodes {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(n.Name) + " <code>" + html.EscapeString(n.ID) + "</code>")
		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="rename" />`)
		b.WriteString(`<input type="hidden" name="org_id" value="` + html.EscapeString(n.ID) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<label>New Name <input name="new_name" value="` + html.EscapeString(n.Name) + `" /></label> `)
		b.WriteString(`<button type="submit">Rename</button>`)
		b.WriteString(`</form>`)

		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="move" />`)
		b.WriteString(`<input type="hidden" name="org_id" value="` + html.EscapeString(n.ID) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<label>New Parent ID (optional) <input name="new_parent_id" /></label> `)
		b.WriteString(`<button type="submit">Move</button>`)
		b.WriteString(`</form>`)

		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="disable" />`)
		b.WriteString(`<input type="hidden" name="org_id" value="` + html.EscapeString(n.ID) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<button type="submit">Disable</button>`)
		b.WriteString(`</form>`)
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	return b.String()
}
