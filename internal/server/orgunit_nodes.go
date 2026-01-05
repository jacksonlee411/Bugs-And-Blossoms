package server

import (
	"context"
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

func handleOrgNodes(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		nodes, err := store.ListNodes(r.Context(), tenant.ID)
		if err != nil {
			writePage(w, r, renderOrgNodes(nodes, tenant, err.Error()))
			return
		}
		writePage(w, r, renderOrgNodes(nodes, tenant, ""))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			writePage(w, r, renderOrgNodes(nil, tenant, "bad form"))
			return
		}
		name := strings.TrimSpace(r.Form.Get("name"))
		if name == "" {
			nodes, _ := store.ListNodes(r.Context(), tenant.ID)
			writePage(w, r, renderOrgNodes(nodes, tenant, "name is required"))
			return
		}
		if _, err := store.CreateNode(r.Context(), tenant.ID, name); err != nil {
			nodes, _ := store.ListNodes(r.Context(), tenant.ID)
			writePage(w, r, renderOrgNodes(nodes, tenant, err.Error()))
			return
		}
		http.Redirect(w, r, "/org/nodes", http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderOrgNodes(nodes []OrgUnitNode, tenant Tenant, errMsg string) string {
	var b strings.Builder
	b.WriteString("<h1>OrgUnit</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	b.WriteString(`<form method="POST" action="/org/nodes">`)
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
