package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
)

type SetID struct {
	SetID  string
	Name   string
	Status string
}

type BusinessUnit struct {
	BusinessUnitID string
	Name           string
	Status         string
}

type SetIDMappingRow struct {
	BusinessUnitID string
	SetID          string
}

type SetIDGovernanceStore interface {
	EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error
	ListSetIDs(ctx context.Context, tenantID string) ([]SetID, error)
	CreateSetID(ctx context.Context, tenantID string, setID string, name string, requestID string, initiatorID string) error
	ListBusinessUnits(ctx context.Context, tenantID string) ([]BusinessUnit, error)
	CreateBusinessUnit(ctx context.Context, tenantID string, businessUnitID string, name string, requestID string, initiatorID string) error
	ListMappings(ctx context.Context, tenantID string, recordGroup string) ([]SetIDMappingRow, error)
	PutMappings(ctx context.Context, tenantID string, recordGroup string, mappings map[string]string, requestID string, initiatorID string) error
}

type setidPGStore struct {
	pool pgBeginner
}

func newSetIDPGStore(pool pgBeginner) SetIDGovernanceStore {
	return &setidPGStore{pool: pool}
}

func (s *setidPGStore) withTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
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

func (s *setidPGStore) EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, initiatorID)
		return err
	})
}

func (s *setidPGStore) ListSetIDs(ctx context.Context, tenantID string) ([]SetID, error) {
	var out []SetID
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT setid, name, status
FROM orgunit.setids
WHERE tenant_id = $1::uuid
ORDER BY setid ASC
`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r SetID
			if err := rows.Scan(&r.SetID, &r.Name, &r.Status); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *setidPGStore) CreateSetID(ctx context.Context, tenantID string, setID string, name string, requestID string, initiatorID string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
SELECT orgunit.submit_setid_event(
  gen_random_uuid(),
  $1::uuid,
  'CREATE',
  $2::text,
  jsonb_build_object('name', $3::text),
  $4::text,
  $5::uuid
);
`, tenantID, setID, name, requestID, initiatorID)
		return err
	})
}

func (s *setidPGStore) ListBusinessUnits(ctx context.Context, tenantID string) ([]BusinessUnit, error) {
	var out []BusinessUnit
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
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

func (s *setidPGStore) CreateBusinessUnit(ctx context.Context, tenantID string, businessUnitID string, name string, requestID string, initiatorID string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
SELECT orgunit.submit_business_unit_event(
  gen_random_uuid(),
  $1::uuid,
  'CREATE',
  $2::text,
  jsonb_build_object('name', $3::text),
  $4::text,
  $5::uuid
);
`, tenantID, businessUnitID, name, requestID, initiatorID)
		return err
	})
}

func (s *setidPGStore) ListMappings(ctx context.Context, tenantID string, recordGroup string) ([]SetIDMappingRow, error) {
	var out []SetIDMappingRow
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT business_unit_id, setid
FROM orgunit.set_control_mappings
WHERE tenant_id = $1::uuid
  AND record_group = $2::text
ORDER BY business_unit_id ASC
`, tenantID, recordGroup)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r SetIDMappingRow
			if err := rows.Scan(&r.BusinessUnitID, &r.SetID); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *setidPGStore) PutMappings(ctx context.Context, tenantID string, recordGroup string, mappings map[string]string, requestID string, initiatorID string) error {
	if len(mappings) == 0 {
		return errors.New("no mappings provided")
	}

	type pair struct {
		BusinessUnitID string
		SetID          string
	}

	pairs := make([]pair, 0, len(mappings))
	for bu, sid := range mappings {
		pairs = append(pairs, pair{BusinessUnitID: bu, SetID: sid})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].BusinessUnitID < pairs[j].BusinessUnitID })

	var b strings.Builder
	b.WriteString("[")
	for i := range pairs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"business_unit_id":`)
		b.WriteString(strconvQuote(pairs[i].BusinessUnitID))
		b.WriteString(`,"setid":`)
		b.WriteString(strconvQuote(pairs[i].SetID))
		b.WriteString("}")
	}
	b.WriteString("]")

	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
SELECT orgunit.put_set_control_mappings(
  gen_random_uuid(),
  $1::uuid,
  $2::text,
  $3::jsonb,
  $4::text,
  $5::uuid
);
`, tenantID, recordGroup, []byte(b.String()), requestID, initiatorID)
		return err
	})
}

type setidMemoryStore struct {
	setids        map[string]map[string]SetID
	businessUnits map[string]map[string]BusinessUnit
	mappings      map[string]map[string]map[string]string // tenant -> record_group -> bu -> setid
}

func newSetIDMemoryStore() SetIDGovernanceStore {
	return &setidMemoryStore{
		setids:        make(map[string]map[string]SetID),
		businessUnits: make(map[string]map[string]BusinessUnit),
		mappings:      make(map[string]map[string]map[string]string),
	}
}

func (s *setidMemoryStore) EnsureBootstrap(_ context.Context, tenantID string, _ string) error {
	if s.setids[tenantID] == nil {
		s.setids[tenantID] = make(map[string]SetID)
	}
	if s.businessUnits[tenantID] == nil {
		s.businessUnits[tenantID] = make(map[string]BusinessUnit)
	}
	if s.mappings[tenantID] == nil {
		s.mappings[tenantID] = make(map[string]map[string]string)
	}
	if s.mappings[tenantID][setid.RecordGroupJobCatalog] == nil {
		s.mappings[tenantID][setid.RecordGroupJobCatalog] = make(map[string]string)
	}

	s.setids[tenantID]["SHARE"] = SetID{SetID: "SHARE", Name: "Shared", Status: "active"}
	s.businessUnits[tenantID]["BU000"] = BusinessUnit{BusinessUnitID: "BU000", Name: "Default BU", Status: "active"}
	s.mappings[tenantID][setid.RecordGroupJobCatalog]["BU000"] = "SHARE"
	return nil
}

func (s *setidMemoryStore) ListSetIDs(_ context.Context, tenantID string) ([]SetID, error) {
	var out []SetID
	for _, v := range s.setids[tenantID] {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SetID < out[j].SetID })
	return out, nil
}

func (s *setidMemoryStore) CreateSetID(_ context.Context, tenantID string, setID string, name string, _ string, _ string) error {
	setID = strings.ToUpper(strings.TrimSpace(setID))
	if setID == "" {
		return errors.New("setid is required")
	}
	if setID == "SHARE" {
		return errors.New("SETID_RESERVED: SHARE is reserved")
	}
	if s.setids[tenantID] == nil {
		s.setids[tenantID] = make(map[string]SetID)
	}
	if _, ok := s.setids[tenantID][setID]; ok {
		return errors.New("SETID_ALREADY_EXISTS")
	}
	s.setids[tenantID][setID] = SetID{SetID: setID, Name: name, Status: "active"}
	return nil
}

func (s *setidMemoryStore) ListBusinessUnits(_ context.Context, tenantID string) ([]BusinessUnit, error) {
	var out []BusinessUnit
	for _, v := range s.businessUnits[tenantID] {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BusinessUnitID < out[j].BusinessUnitID })
	return out, nil
}

func (s *setidMemoryStore) CreateBusinessUnit(_ context.Context, tenantID string, businessUnitID string, name string, _ string, _ string) error {
	businessUnitID = strings.ToUpper(strings.TrimSpace(businessUnitID))
	if businessUnitID == "" {
		return errors.New("business_unit_id is required")
	}
	if s.businessUnits[tenantID] == nil {
		s.businessUnits[tenantID] = make(map[string]BusinessUnit)
	}
	if _, ok := s.businessUnits[tenantID][businessUnitID]; ok {
		return errors.New("BUSINESS_UNIT_ALREADY_EXISTS")
	}
	s.businessUnits[tenantID][businessUnitID] = BusinessUnit{BusinessUnitID: businessUnitID, Name: name, Status: "active"}
	if s.mappings[tenantID] == nil {
		s.mappings[tenantID] = make(map[string]map[string]string)
	}
	if s.mappings[tenantID][setid.RecordGroupJobCatalog] == nil {
		s.mappings[tenantID][setid.RecordGroupJobCatalog] = make(map[string]string)
	}
	s.mappings[tenantID][setid.RecordGroupJobCatalog][businessUnitID] = "SHARE"
	return nil
}

func (s *setidMemoryStore) ListMappings(_ context.Context, tenantID string, recordGroup string) ([]SetIDMappingRow, error) {
	var out []SetIDMappingRow
	for bu, sid := range s.mappings[tenantID][recordGroup] {
		out = append(out, SetIDMappingRow{BusinessUnitID: bu, SetID: sid})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BusinessUnitID < out[j].BusinessUnitID })
	return out, nil
}

func (s *setidMemoryStore) PutMappings(_ context.Context, tenantID string, recordGroup string, mappings map[string]string, _ string, _ string) error {
	if s.mappings[tenantID] == nil {
		s.mappings[tenantID] = make(map[string]map[string]string)
	}
	if s.mappings[tenantID][recordGroup] == nil {
		s.mappings[tenantID][recordGroup] = make(map[string]string)
	}
	for bu, sid := range mappings {
		s.mappings[tenantID][recordGroup][strings.ToUpper(bu)] = strings.ToUpper(sid)
	}
	return nil
}

func handleSetID(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	initiatorID := tenant.ID
	if err := store.EnsureBootstrap(r.Context(), tenant.ID, initiatorID); err != nil {
		writePage(w, r, renderSetIDPage(nil, nil, nil, tenant, asOf, err.Error()))
		return
	}

	list := func(errHint string) (setids []SetID, bus []BusinessUnit, mappings []SetIDMappingRow, errMsg string) {
		mergeMsg := func(hint string, msg string) string {
			if hint == "" {
				return msg
			}
			if msg == "" {
				return hint
			}
			return hint + "；" + msg
		}

		setids, err := store.ListSetIDs(r.Context(), tenant.ID)
		if err != nil {
			return nil, nil, nil, mergeMsg(errHint, err.Error())
		}
		bus, err = store.ListBusinessUnits(r.Context(), tenant.ID)
		if err != nil {
			return nil, nil, nil, mergeMsg(errHint, err.Error())
		}
		mappings, err = store.ListMappings(r.Context(), tenant.ID, setid.RecordGroupJobCatalog)
		if err != nil {
			return nil, nil, nil, mergeMsg(errHint, err.Error())
		}
		return setids, bus, mappings, errHint
	}

	switch r.Method {
	case http.MethodGet:
		sids, bus, mappings, errMsg := list("")
		writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			sids, bus, mappings, errMsg := list("bad form")
			writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
			return
		}

		action := strings.TrimSpace(strings.ToLower(r.Form.Get("action")))
		if action == "" {
			action = "create_setid"
		}

		switch action {
		case "create_setid":
			sid := strings.TrimSpace(r.Form.Get("setid"))
			name := strings.TrimSpace(r.Form.Get("name"))
			if sid == "" || name == "" {
				sids, bus, mappings, errMsg := list("setid/name is required")
				writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
				return
			}
			reqID := "ui:setid:create:" + sid
			if err := store.CreateSetID(r.Context(), tenant.ID, sid, name, reqID, initiatorID); err != nil {
				sids, bus, mappings, errMsg := list(err.Error())
				writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
				return
			}
		case "create_bu":
			bu := strings.TrimSpace(r.Form.Get("business_unit_id"))
			name := strings.TrimSpace(r.Form.Get("name"))
			if bu == "" || name == "" {
				sids, bus, mappings, errMsg := list("business_unit_id/name is required")
				writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
				return
			}
			reqID := "ui:bu:create:" + bu
			if err := store.CreateBusinessUnit(r.Context(), tenant.ID, bu, name, reqID, initiatorID); err != nil {
				sids, bus, mappings, errMsg := list(err.Error())
				writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
				return
			}
		case "save_mappings":
			m := make(map[string]string)
			for k, vs := range r.PostForm {
				if !strings.HasPrefix(k, "map_") || len(vs) == 0 {
					continue
				}
				bu := strings.TrimPrefix(k, "map_")
				m[bu] = strings.TrimSpace(vs[0])
			}
			if len(m) == 0 {
				sids, bus, mappings, errMsg := list("no mapping changes")
				writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
				return
			}
			reqID := "ui:mappings:jobcatalog"
			if err := store.PutMappings(r.Context(), tenant.ID, setid.RecordGroupJobCatalog, m, reqID, initiatorID); err != nil {
				sids, bus, mappings, errMsg := list(err.Error())
				writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
				return
			}
		default:
			sids, bus, mappings, errMsg := list("unknown action")
			writePage(w, r, renderSetIDPage(sids, bus, mappings, tenant, asOf, errMsg))
			return
		}

		http.Redirect(w, r, "/org/setid?as_of="+url.QueryEscape(asOf), http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func renderSetIDPage(setids []SetID, businessUnits []BusinessUnit, mappings []SetIDMappingRow, tenant Tenant, asOf string, errMsg string) string {
	var b strings.Builder
	b.WriteString("<h1>SetID Governance</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString(`<p><a href="/org/job-catalog?as_of=` + html.EscapeString(asOf) + `" hx-get="/org/job-catalog?as_of=` + html.EscapeString(asOf) + `" hx-target="#content" hx-push-url="true">Go to Job Catalog</a></p>`)

	if errMsg != "" {
		b.WriteString(`<div style="padding:8px;border:1px solid #c00;color:#c00">` + html.EscapeString(errMsg) + `</div>`)
	}

	b.WriteString("<h2>SetIDs</h2>")
	b.WriteString(`<form method="POST" action="/org/setid?as_of=` + html.EscapeString(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_setid" />`)
	b.WriteString(`<label>SetID <input name="setid" maxlength="5" /></label> `)
	b.WriteString(`<label>Name <input name="name" /></label> `)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>setid</th><th>name</th><th>status</th></tr></thead><tbody>`)
	for _, s := range setids {
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(s.SetID) + "</td>")
		b.WriteString("<td>" + html.EscapeString(s.Name) + "</td>")
		b.WriteString("<td>" + html.EscapeString(s.Status) + "</td>")
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")

	b.WriteString("<h2>Business Units</h2>")
	b.WriteString(`<form method="POST" action="/org/setid?as_of=` + html.EscapeString(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="action" value="create_bu" />`)
	b.WriteString(`<label>BU <input name="business_unit_id" maxlength="5" /></label> `)
	b.WriteString(`<label>Name <input name="name" /></label> `)
	b.WriteString(`<button type="submit">Create</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>business_unit_id</th><th>name</th><th>status</th></tr></thead><tbody>`)
	for _, bu := range businessUnits {
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(bu.BusinessUnitID) + "</td>")
		b.WriteString("<td>" + html.EscapeString(bu.Name) + "</td>")
		b.WriteString("<td>" + html.EscapeString(bu.Status) + "</td>")
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")

	mappingByBU := make(map[string]string, len(mappings))
	for _, m := range mappings {
		mappingByBU[m.BusinessUnitID] = m.SetID
	}

	b.WriteString("<h2>Mappings (jobcatalog)</h2>")
	b.WriteString(`<form method="POST" action="/org/setid?as_of=` + html.EscapeString(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="action" value="save_mappings" />`)
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr><th>business_unit_id</th><th>setid</th></tr></thead><tbody>`)
	for _, bu := range businessUnits {
		current := mappingByBU[bu.BusinessUnitID]
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(bu.BusinessUnitID) + "</td>")
		b.WriteString(`<td><select name="map_` + html.EscapeString(bu.BusinessUnitID) + `">`)
		for _, sid := range setids {
			if sid.Status != "active" {
				continue
			}
			selected := ""
			if sid.SetID == current {
				selected = " selected"
			}
			b.WriteString(`<option value="` + html.EscapeString(sid.SetID) + `"` + selected + `>` + html.EscapeString(sid.SetID) + `</option>`)
		}
		b.WriteString(`</select></td>`)
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	b.WriteString(`<button type="submit">Save Mappings</button>`)
	b.WriteString(`</form>`)

	return b.String()
}

func strconvQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
