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
	ListNodes(ctx context.Context, tenantID string) ([]OrgUnitNode, error)
	CreateNode(ctx context.Context, tenantID string, name string) (OrgUnitNode, error)
}

type OrgUnitNodesV4Reader interface {
	ListNodesV4(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
}

type OrgUnitNodesV4Writer interface {
	CreateNodeV4(ctx context.Context, tenantID string, effectiveDate string, name string, parentID string) (OrgUnitNode, error)
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

func (s *orgUnitPGStore) ListNodes(ctx context.Context, tenantID string) ([]OrgUnitNode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT node_id::text, name, created_at
FROM orgunit.nodes
WHERE tenant_id = $1::uuid
ORDER BY created_at DESC
`, tenantID)
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

func (s *orgUnitPGStore) ListNodesV4(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
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

func (s *orgUnitPGStore) CreateNode(ctx context.Context, tenantID string, name string) (OrgUnitNode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitNode{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitNode{}, err
	}

	payload := []byte(`{"name":` + strconv.Quote(name) + `}`)

	var id string
	row := tx.QueryRow(ctx, `
SELECT orgunit.submit_orgunit_event(
  $1::uuid,
  'node_created',
  $2::jsonb
)::text
`, tenantID, payload)
	if err := row.Scan(&id); err != nil {
		return OrgUnitNode{}, err
	}

	var createdAt time.Time
	row2 := tx.QueryRow(ctx, `
SELECT created_at
FROM orgunit.nodes
WHERE tenant_id = $1::uuid AND node_id = $2::uuid
`, tenantID, id)
	if err := row2.Scan(&createdAt); err != nil {
		return OrgUnitNode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNode{}, err
	}

	return OrgUnitNode{ID: id, Name: name, CreatedAt: createdAt}, nil
}

func (s *orgUnitPGStore) CreateNodeV4(ctx context.Context, tenantID string, effectiveDate string, name string, parentID string) (OrgUnitNode, error) {
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

func (s *orgUnitMemoryStore) ListNodes(_ context.Context, tenantID string) ([]OrgUnitNode, error) {
	return append([]OrgUnitNode(nil), s.nodes[tenantID]...), nil
}

func (s *orgUnitMemoryStore) CreateNode(_ context.Context, tenantID string, name string) (OrgUnitNode, error) {
	n := OrgUnitNode{
		ID:        "mem-" + strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		Name:      name,
		CreatedAt: s.now(),
	}
	s.nodes[tenantID] = append([]OrgUnitNode{n}, s.nodes[tenantID]...)
	return n, nil
}

func (s *orgUnitMemoryStore) ListNodesV4(_ context.Context, tenantID string, _ string) ([]OrgUnitNode, error) {
	return s.ListNodes(context.Background(), tenantID)
}

func (s *orgUnitMemoryStore) CreateNodeV4(_ context.Context, tenantID string, _ string, name string, _ string) (OrgUnitNode, error) {
	return s.CreateNode(context.Background(), tenantID, name)
}

func handleOrgNodes(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	preferRead := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("read")))
	if preferRead == "" {
		preferRead = "v4"
	}
	if preferRead != "legacy" && preferRead != "v4" {
		preferRead = "v4"
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

		if preferRead == "v4" {
			if _, err := time.Parse("2006-01-02", asOf); err != nil {
				nodes, legacyErr := store.ListNodes(r.Context(), tenant.ID)
				if legacyErr != nil {
					return nil, mergeMsg(errHint, "as_of 无效，且 legacy 读取失败: "+legacyErr.Error())
				}
				return nodes, mergeMsg(errHint, "as_of 无效，已回退到 legacy: "+err.Error())
			}

			v4Store, ok := store.(OrgUnitNodesV4Reader)
			if !ok {
				nodes, err := store.ListNodes(r.Context(), tenant.ID)
				if err != nil {
					return nil, mergeMsg(errHint, "v4 reader 未配置，且 legacy 读取失败: "+err.Error())
				}
				return nodes, mergeMsg(errHint, "v4 reader 未配置，已回退到 legacy")
			}

			nodesV4, err := v4Store.ListNodesV4(r.Context(), tenant.ID, asOf)
			if err == nil && len(nodesV4) > 0 {
				return nodesV4, mergeMsg(errHint, "")
			}

			nodes, legacyErr := store.ListNodes(r.Context(), tenant.ID)
			if legacyErr != nil {
				if err != nil {
					return nil, mergeMsg(errHint, "v4 读取失败: "+err.Error()+"；且 legacy 读取失败: "+legacyErr.Error())
				}
				return nil, mergeMsg(errHint, "legacy 读取失败: "+legacyErr.Error())
			}
			if err != nil {
				return nodes, mergeMsg(errHint, "v4 读取失败，已回退到 legacy: "+err.Error())
			}
			return nodes, mergeMsg(errHint, "v4 快照为空，已回退到 legacy")
		}

		nodes, err := store.ListNodes(r.Context(), tenant.ID)
		if err != nil {
			return nil, err.Error()
		}
		return nodes, errHint
	}

	switch r.Method {
	case http.MethodGet:
		nodes, errMsg := listNodes("")
		writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			nodes, errMsg := listNodes("bad form")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
			return
		}
		name := strings.TrimSpace(r.Form.Get("name"))
		if name == "" {
			nodes, errMsg := listNodes("name is required")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
			return
		}
		if preferRead == "v4" {
			effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
			if effectiveDate == "" {
				effectiveDate = asOf
			}
			parentID := strings.TrimSpace(r.Form.Get("parent_id"))
			if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
				nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
				return
			}

			v4Writer, ok := store.(OrgUnitNodesV4Writer)
			if !ok {
				nodes, errMsg := listNodes("v4 writer 未配置：请切回 legacy 模式写入")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
				return
			}

			if _, err := v4Writer.CreateNodeV4(r.Context(), tenant.ID, effectiveDate, name, parentID); err != nil {
				nodes, errMsg := listNodes(err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
				return
			}
			http.Redirect(w, r, "/org/nodes?read=v4&as_of="+effectiveDate, http.StatusSeeOther)
			return
		}

		if _, err := store.CreateNode(r.Context(), tenant.ID, name); err != nil {
			nodes, errMsg := listNodes(err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, preferRead, asOf))
			return
		}
		redirectTo := "/org/nodes"
		if r.URL.RawQuery != "" {
			redirectTo += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, redirectTo, http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderOrgNodes(nodes []OrgUnitNode, tenant Tenant, errMsg string, readMode string, asOf string) string {
	var b strings.Builder
	b.WriteString("<h1>OrgUnit</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString("<p>Read: <code>" + html.EscapeString(readMode) + "</code></p>")
	b.WriteString(`<p><a href="/org/nodes?read=legacy&as_of=` + html.EscapeString(asOf) + `">Use legacy read</a> | <a href="/org/nodes?read=v4&as_of=` + html.EscapeString(asOf) + `">Use v4 read</a></p>`)
	if readMode == "v4" {
		b.WriteString(`<form method="GET" action="/org/nodes">`)
		b.WriteString(`<input type="hidden" name="read" value="v4" />`)
		b.WriteString(`<label>As-of <input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<button type="submit">Apply</button>`)
		b.WriteString(`</form>`)
	}

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	postAction := "/org/nodes"
	if readMode == "v4" {
		postAction += "?read=v4&as_of=" + html.EscapeString(asOf)
	}
	b.WriteString(`<form method="POST" action="` + postAction + `">`)
	if readMode == "v4" {
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label><br/>`)
		b.WriteString(`<label>Parent ID (optional) <input name="parent_id" /></label><br/>`)
	}
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
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	return b.String()
}
