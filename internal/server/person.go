package server

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type Person struct {
	UUID        string
	Pernr       string
	DisplayName string
	Status      string
	CreatedAt   time.Time
}

type PersonOption struct {
	UUID        string
	Pernr       string
	DisplayName string
}

type PersonStore interface {
	ListPersons(ctx context.Context, tenantID string) ([]Person, error)
	CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error)
	FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error)
	ListPersonOptions(ctx context.Context, tenantID string, q string, limit int) ([]PersonOption, error)
}

type personPGStore struct {
	pool pgBeginner
}

func newPersonPGStore(pool pgBeginner) PersonStore {
	return &personPGStore{pool: pool}
}

var pernrDigitsMax8Re = regexp.MustCompile(`^[0-9]{1,8}$`)

func normalizePernr(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("pernr is required")
	}
	if !pernrDigitsMax8Re.MatchString(raw) {
		return "", errors.New("pernr must be 1-8 digits")
	}
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		raw = "0"
	}
	return raw, nil
}

func (s *personPGStore) ListPersons(ctx context.Context, tenantID string) ([]Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT person_uuid::text, pernr, display_name, status, created_at
FROM person.persons
WHERE tenant_id = $1::uuid
ORDER BY created_at DESC, person_uuid DESC
LIMIT 200
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.UUID, &p.Pernr, &p.DisplayName, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *personPGStore) CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return Person{}, err
	}

	canonical, err := normalizePernr(pernr)
	if err != nil {
		return Person{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return Person{}, errors.New("display_name is required")
	}

	var p Person
	p.Pernr = canonical
	p.DisplayName = displayName
	p.Status = "active"
	if err := tx.QueryRow(ctx, `
INSERT INTO person.persons (tenant_id, pernr, display_name, status)
VALUES ($1::uuid, $2::text, $3::text, 'active')
RETURNING person_uuid::text, created_at
`, tenantID, canonical, displayName).Scan(&p.UUID, &p.CreatedAt); err != nil {
		return Person{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Person{}, err
	}
	return p, nil
}

func (s *personPGStore) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return Person{}, err
	}

	canonical, err := normalizePernr(pernr)
	if err != nil {
		return Person{}, err
	}

	var p Person
	if err := tx.QueryRow(ctx, `
SELECT person_uuid::text, pernr, display_name, status, created_at
FROM person.persons
WHERE tenant_id = $1::uuid AND pernr = $2::text
`, tenantID, canonical).Scan(&p.UUID, &p.Pernr, &p.DisplayName, &p.Status, &p.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Person{}, pgx.ErrNoRows
		}
		return Person{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Person{}, err
	}
	return p, nil
}

func (s *personPGStore) ListPersonOptions(ctx context.Context, tenantID string, q string, limit int) ([]PersonOption, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	q = strings.TrimSpace(q)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	pernrPrefix := ""
	if q != "" {
		if canonical, err := normalizePernr(q); err == nil {
			pernrPrefix = canonical
		}
	}

	rows, err := tx.Query(ctx, `
SELECT person_uuid::text, pernr, display_name
FROM person.persons
WHERE tenant_id = $1::uuid
  AND (
    $2::text = '' OR pernr LIKE ($2::text || '%')
    OR display_name ILIKE ('%' || $3::text || '%')
  )
ORDER BY display_name ASC, pernr ASC
LIMIT $4::int
`, tenantID, pernrPrefix, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PersonOption
	for rows.Next() {
		var p PersonOption
		if err := rows.Scan(&p.UUID, &p.Pernr, &p.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

type personMemoryStore struct {
	byTenant map[string][]Person
}

func newPersonMemoryStore() PersonStore {
	return &personMemoryStore{
		byTenant: make(map[string][]Person),
	}
}

func (s *personMemoryStore) ListPersons(_ context.Context, tenantID string) ([]Person, error) {
	return append([]Person(nil), s.byTenant[tenantID]...), nil
}

func (s *personMemoryStore) CreatePerson(_ context.Context, tenantID string, pernr string, displayName string) (Person, error) {
	canonical, err := normalizePernr(pernr)
	if err != nil {
		return Person{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return Person{}, errors.New("display_name is required")
	}
	for _, p := range s.byTenant[tenantID] {
		if p.Pernr == canonical {
			return Person{}, errors.New("pernr already exists")
		}
	}
	p := Person{
		UUID:        "person-" + canonical,
		Pernr:       canonical,
		DisplayName: displayName,
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	s.byTenant[tenantID] = append(s.byTenant[tenantID], p)
	return p, nil
}

func (s *personMemoryStore) FindPersonByPernr(_ context.Context, tenantID string, pernr string) (Person, error) {
	canonical, err := normalizePernr(pernr)
	if err != nil {
		return Person{}, err
	}
	for _, p := range s.byTenant[tenantID] {
		if p.Pernr == canonical {
			return p, nil
		}
	}
	return Person{}, pgx.ErrNoRows
}

func (s *personMemoryStore) ListPersonOptions(ctx context.Context, tenantID string, q string, limit int) ([]PersonOption, error) {
	q = strings.TrimSpace(q)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var out []PersonOption
	for _, p := range s.byTenant[tenantID] {
		if q == "" || strings.Contains(strings.ToLower(p.DisplayName), strings.ToLower(q)) || strings.HasPrefix(p.Pernr, strings.TrimLeft(q, "0")) {
			out = append(out, PersonOption{UUID: p.UUID, Pernr: p.Pernr, DisplayName: p.DisplayName})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func handlePersons(w http.ResponseWriter, r *http.Request, store PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		ps, err := store.ListPersons(r.Context(), tenant.ID)
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		writePage(w, r, renderPersons(ps, tenant, asOf, msg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			ps, _ := store.ListPersons(r.Context(), tenant.ID)
			writePage(w, r, renderPersons(ps, tenant, asOf, "bad form"))
			return
		}
		pernr := strings.TrimSpace(r.Form.Get("pernr"))
		displayName := strings.TrimSpace(r.Form.Get("display_name"))
		if _, err := store.CreatePerson(r.Context(), tenant.ID, pernr, displayName); err != nil {
			ps, _ := store.ListPersons(r.Context(), tenant.ID)
			writePage(w, r, renderPersons(ps, tenant, asOf, err.Error()))
			return
		}
		http.Redirect(w, r, "/person/persons?as_of="+url.QueryEscape(asOf), http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func renderPersons(persons []Person, tenant Tenant, asOf string, errMsg string) string {
	var b strings.Builder
	b.WriteString("<h1>Person</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	if errMsg != "" {
		b.WriteString(`<p style="color:red">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Create</h2>`)
	b.WriteString(`<form method="POST" action="/person/persons?as_of=` + html.EscapeString(asOf) + `">`)
	b.WriteString(`<label>Pernr <input name="pernr" /></label><br/>`)
	b.WriteString(`<label>Display Name <input name="display_name" /></label><br/>`)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>List</h2>`)
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>pernr</th><th>display_name</th><th>status</th><th>person_uuid</th></tr></thead><tbody>`)
	for _, p := range persons {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(p.Pernr) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.DisplayName) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(p.Status) + `</td>`)
		b.WriteString(`<td><code>` + html.EscapeString(p.UUID) + `</code></td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func handlePersonOptionsAPI(w http.ResponseWriter, r *http.Request, store PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	items, err := store.ListPersonOptions(r.Context(), tenant.ID, q, limit)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "PERSON_INTERNAL", "person internal")
		return
	}

	type item struct {
		PersonUUID  string `json:"person_uuid"`
		Pernr       string `json:"pernr"`
		DisplayName string `json:"display_name"`
	}
	type resp struct {
		Items []item `json:"items"`
	}

	out := resp{Items: make([]item, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, item{
			PersonUUID:  it.UUID,
			Pernr:       it.Pernr,
			DisplayName: it.DisplayName,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func handlePersonByPernrAPI(w http.ResponseWriter, r *http.Request, store PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	raw := strings.TrimSpace(r.URL.Query().Get("pernr"))
	if raw == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "PERSON_PERNR_INVALID", "pernr invalid")
		return
	}
	if _, err := normalizePernr(raw); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "PERSON_PERNR_INVALID", "pernr invalid")
		return
	}

	p, err := store.FindPersonByPernr(r.Context(), tenant.ID, raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "PERSON_NOT_FOUND", "person not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "PERSON_INTERNAL", "person internal")
		return
	}

	type resp struct {
		PersonUUID  string `json:"person_uuid"`
		Pernr       string `json:"pernr"`
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp{
		PersonUUID:  p.UUID,
		Pernr:       p.Pernr,
		DisplayName: p.DisplayName,
		Status:      p.Status,
	})
}
