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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

type OrgUnitNode struct {
	ID             string
	OrgCode        string
	Name           string
	IsBusinessUnit bool
	CreatedAt      time.Time
}

type OrgUnitStore interface {
	OrgUnitNodesCurrentReader
	OrgUnitNodesCurrentWriter
	OrgUnitNodesCurrentRenamer
	OrgUnitNodesCurrentMover
	OrgUnitNodesCurrentDisabler
	OrgUnitNodesCurrentBusinessUnitSetter
	OrgUnitCodeResolver
}

type OrgUnitNodesCurrentReader interface {
	ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error)
}

type OrgUnitNodesCurrentWriter interface {
	CreateNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgCode string, name string, parentID string, isBusinessUnit bool) (OrgUnitNode, error)
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

type OrgUnitNodesCurrentBusinessUnitSetter interface {
	SetBusinessUnitCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, isBusinessUnit bool, requestCode string) error
}

type OrgUnitCodeResolver interface {
	ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error)
	ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error)
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

func parseOrgID8(input string) (int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, errors.New("org_id is required")
	}
	if len(trimmed) != 8 {
		return 0, errors.New("org_id must be 8 digits")
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, errors.New("org_id must be 8 digits")
	}
	if value < 10000000 || value > 99999999 {
		return 0, errors.New("org_id must be 8 digits")
	}
	return value, nil
}

func parseOptionalOrgID8(input string) (int, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, false, nil
	}
	value, err := parseOrgID8(trimmed)
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func (s *orgUnitPGStore) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return 0, err
	}

	orgID, err := orgunitpkg.ResolveOrgID(ctx, tx, tenantID, orgCode)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return orgID, nil
}

func (s *orgUnitPGStore) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	orgCode, err := orgunitpkg.ResolveOrgCode(ctx, tx, tenantID, orgID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orgCode, nil
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
  SELECT org_id, name, is_business_unit
  FROM orgunit.get_org_snapshot($1::uuid, $2::date)
)
SELECT
  s.org_id::text,
  c.org_code,
  s.name,
  s.is_business_unit,
  e.transaction_time
FROM snapshot s
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = $1::uuid
 AND c.org_id = s.org_id
JOIN orgunit.org_unit_versions v
  ON v.tenant_uuid = $1::uuid
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
		if err := rows.Scan(&n.ID, &n.OrgCode, &n.Name, &n.IsBusinessUnit, &n.CreatedAt); err != nil {
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

func (s *orgUnitPGStore) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	if _, err := parseOrgID8(orgUnitID); err != nil {
		return "", err
	}

	out, err := setid.Resolve(ctx, tx, tenantID, orgUnitID, asOfDate)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return out, nil
}
func (s *orgUnitPGStore) CreateNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgCode string, name string, parentID string, isBusinessUnit bool) (OrgUnitNode, error) {
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

	normalizedCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return OrgUnitNode{}, err
	}

	if _, ok, err := parseOptionalOrgID8(parentID); err != nil {
		return OrgUnitNode{}, err
	} else if ok {
		parentID = strings.TrimSpace(parentID)
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return OrgUnitNode{}, err
	}

	payload := `{"org_code":` + strconv.Quote(normalizedCode) + `,"name":` + strconv.Quote(name)
	if strings.TrimSpace(parentID) != "" {
		payload += `,"parent_id":` + strconv.Quote(parentID)
	}
	payload += `,"is_business_unit":` + strconv.FormatBool(isBusinessUnit)
	payload += `}`

	_, err = tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::int,
  'CREATE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, nil, effectiveDate, []byte(payload), eventID, tenantID)
	if err != nil {
		return OrgUnitNode{}, err
	}

	var orgID int
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
SELECT org_id, transaction_time
FROM orgunit.org_events
WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
`, tenantID, eventID).Scan(&orgID, &createdAt); err != nil {
		return OrgUnitNode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNode{}, err
	}

	return OrgUnitNode{ID: strconv.Itoa(orgID), OrgCode: normalizedCode, Name: name, IsBusinessUnit: isBusinessUnit, CreatedAt: createdAt}, nil
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

	if _, err := parseOrgID8(orgID); err != nil {
		return err
	}
	if strings.TrimSpace(newName) == "" {
		return errors.New("new_name is required")
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}

	payload := `{"new_name":` + strconv.Quote(newName) + `}`

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::int,
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

	if _, err := parseOrgID8(orgID); err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}

	payload := `{}`
	if _, ok, err := parseOptionalOrgID8(newParentID); err != nil {
		return err
	} else if ok {
		payload = `{"new_parent_id":` + strconv.Quote(newParentID) + `}`
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::int,
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

	if _, err := parseOrgID8(orgID); err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::int,
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

func (s *orgUnitPGStore) SetBusinessUnitCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, isBusinessUnit bool, requestCode string) error {
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
	if _, err := parseOrgID8(orgID); err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}
	if strings.TrimSpace(requestCode) == "" {
		requestCode = eventID
	}

	payload := `{"is_business_unit":` + strconv.FormatBool(isBusinessUnit) + `}`

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_set_business_unit;`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
  $2::uuid,
  'OrgUnit',
  $3::int,
  'SET_BUSINESS_UNIT',
  $4::date,
  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
	`, eventID, tenantID, orgID, effectiveDate, []byte(payload), requestCode, tenantID); err != nil {
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_set_business_unit;`); rbErr != nil {
			return rbErr
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr != nil && pgErr.Code == "23505" && pgErr.ConstraintName == "org_events_one_per_day_unique" {
			var current bool
			if queryErr := tx.QueryRow(ctx, `
		SELECT is_business_unit
		FROM orgunit.org_unit_versions
		WHERE tenant_uuid = $1::uuid
		  AND hierarchy_type = 'OrgUnit'
		  AND org_id = $2::int
		  AND status = 'active'
		  AND validity @> $3::date
		ORDER BY lower(validity) DESC
		LIMIT 1;
	`, tenantID, orgID, effectiveDate).Scan(&current); queryErr == nil && current == isBusinessUnit {
				return tx.Commit(ctx)
			}
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

type orgUnitMemoryStore struct {
	nodes  map[string][]OrgUnitNode
	now    func() time.Time
	nextID int
}

func newOrgUnitMemoryStore() *orgUnitMemoryStore {
	return &orgUnitMemoryStore{
		nodes:  make(map[string][]OrgUnitNode),
		now:    time.Now,
		nextID: 10000000,
	}
}

func (s *orgUnitMemoryStore) listNodes(tenantID string) ([]OrgUnitNode, error) {
	return append([]OrgUnitNode(nil), s.nodes[tenantID]...), nil
}

func (s *orgUnitMemoryStore) createNode(tenantID string, orgCode string, name string, isBusinessUnit bool) (OrgUnitNode, error) {
	normalizedCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return OrgUnitNode{}, err
	}
	id := s.nextID
	s.nextID++
	n := OrgUnitNode{
		ID:             strconv.Itoa(id),
		OrgCode:        normalizedCode,
		Name:           name,
		IsBusinessUnit: isBusinessUnit,
		CreatedAt:      s.now(),
	}
	s.nodes[tenantID] = append([]OrgUnitNode{n}, s.nodes[tenantID]...)
	return n, nil
}

func (s *orgUnitMemoryStore) ListNodesCurrent(_ context.Context, tenantID string, _ string) ([]OrgUnitNode, error) {
	return s.listNodes(tenantID)
}

func (s *orgUnitMemoryStore) ResolveSetID(_ context.Context, _ string, orgUnitID string, _ string) (string, error) {
	if _, err := parseOrgID8(orgUnitID); err != nil {
		return "", err
	}
	return "S2601", nil
}

func (s *orgUnitMemoryStore) CreateNodeCurrent(_ context.Context, tenantID string, _ string, orgCode string, name string, _ string, isBusinessUnit bool) (OrgUnitNode, error) {
	return s.createNode(tenantID, orgCode, name, isBusinessUnit)
}

func (s *orgUnitMemoryStore) RenameNodeCurrent(_ context.Context, tenantID string, _ string, orgID string, newName string) error {
	if _, err := parseOrgID8(orgID); err != nil {
		return err
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
	if _, err := parseOrgID8(orgID); err != nil {
		return err
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
	if _, err := parseOrgID8(orgID); err != nil {
		return err
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

func (s *orgUnitMemoryStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, _ string, orgID string, isBusinessUnit bool, _ string) error {
	if _, err := parseOrgID8(orgID); err != nil {
		return err
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == orgID {
			nodes[i].IsBusinessUnit = isBusinessUnit
			s.nodes[tenantID] = nodes
			return nil
		}
	}
	return errors.New("org_id not found")
}

func (s *orgUnitMemoryStore) ResolveOrgID(_ context.Context, tenantID string, orgCode string) (int, error) {
	normalizedCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return 0, err
	}
	for _, node := range s.nodes[tenantID] {
		if node.OrgCode == normalizedCode {
			return strconv.Atoi(node.ID)
		}
	}
	return 0, orgunitpkg.ErrOrgCodeNotFound
}

func (s *orgUnitMemoryStore) ResolveOrgCode(_ context.Context, tenantID string, orgID int) (string, error) {
	for _, node := range s.nodes[tenantID] {
		if node.ID == strconv.Itoa(orgID) {
			return node.OrgCode, nil
		}
	}
	return "", orgunitpkg.ErrOrgIDNotFound
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

		parseBusinessUnitFlag := func(v string) (bool, error) {
			if strings.TrimSpace(v) == "" {
				return false, nil
			}
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "1", "true", "on", "yes":
				return true, nil
			case "0", "false", "off", "no":
				return false, nil
			default:
				return false, errors.New("is_business_unit 无效")
			}
		}

		resolveOrgID := func(code string, field string, required bool) (string, bool) {
			if code == "" {
				if !required {
					return "", true
				}
				nodes, errMsg := listNodes(field + " is required")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
				return "", false
			}
			orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, code)
			if err != nil {
				msg := field + " invalid"
				switch {
				case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
					msg = field + " invalid"
				case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
					msg = field + " not found"
				default:
					msg = err.Error()
				}
				nodes, errMsg := listNodes(msg)
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
				return "", false
			}
			return strconv.Itoa(orgID), true
		}

		if action == "rename" || action == "move" || action == "disable" || action == "set_business_unit" {
			effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
			if effectiveDate == "" {
				effectiveDate = asOf
			}
			if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
				nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
				return
			}

			orgID, ok := resolveOrgID(r.Form.Get("org_code"), "org_code", true)
			if !ok {
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
				newParentID, ok := resolveOrgID(r.Form.Get("new_parent_code"), "new_parent_code", false)
				if !ok {
					return
				}
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
			case "set_business_unit":
				isBusinessUnit, err := parseBusinessUnitFlag(r.Form.Get("is_business_unit"))
				if err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
					return
				}
				reqID := "ui:orgunit:set-business-unit:" + orgID + ":" + effectiveDate
				if err := store.SetBusinessUnitCurrent(r.Context(), tenant.ID, effectiveDate, orgID, isBusinessUnit, reqID); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
					return
				}
			}

			http.Redirect(w, r, "/org/nodes?as_of="+effectiveDate, http.StatusSeeOther)
			return
		}

		orgCode := r.Form.Get("org_code")
		if orgCode == "" {
			nodes, errMsg := listNodes("org_code is required")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
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
		parentID := ""
		if r.Form.Get("parent_code") != "" {
			resolvedID, ok := resolveOrgID(r.Form.Get("parent_code"), "parent_code", false)
			if !ok {
				return
			}
			parentID = resolvedID
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
			return
		}

		isBusinessUnit, err := parseBusinessUnitFlag(r.Form.Get("is_business_unit"))
		if err != nil {
			nodes, errMsg := listNodes(err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
			return
		}

		if _, err := store.CreateNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgCode, name, parentID, isBusinessUnit); err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeInvalid) {
				nodes, errMsg := listNodes("org_code invalid")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, asOf))
				return
			}
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
	b.WriteString(`<label>Org Code <input name="org_code" /></label><br/>`)
	b.WriteString(`<label>Parent Code (optional) <input name="parent_code" /></label><br/>`)
	b.WriteString(`<label>Name <input name="name" /></label> `)
	b.WriteString(`<label>Is Business Unit <input type="checkbox" name="is_business_unit" value="true" /></label>`)
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
		codeLabel := n.OrgCode
		if strings.TrimSpace(codeLabel) == "" {
			codeLabel = "(missing org_code)"
		}
		b.WriteString(html.EscapeString(n.Name) + " <code>" + html.EscapeString(codeLabel) + "</code>")
		if n.IsBusinessUnit {
			b.WriteString(` <span>(BU)</span>`)
		}
		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="rename" />`)
		b.WriteString(`<input type="hidden" name="org_code" value="` + html.EscapeString(n.OrgCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<label>New Name <input name="new_name" value="` + html.EscapeString(n.Name) + `" /></label> `)
		b.WriteString(`<button type="submit">Rename</button>`)
		b.WriteString(`</form>`)

		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="move" />`)
		b.WriteString(`<input type="hidden" name="org_code" value="` + html.EscapeString(n.OrgCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<label>New Parent Code (optional) <input name="new_parent_code" /></label> `)
		b.WriteString(`<button type="submit">Move</button>`)
		b.WriteString(`</form>`)

		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="disable" />`)
		b.WriteString(`<input type="hidden" name="org_code" value="` + html.EscapeString(n.OrgCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		b.WriteString(`<button type="submit">Disable</button>`)
		b.WriteString(`</form>`)

		b.WriteString(`<form method="POST" action="` + postAction + `" style="margin-top:4px">`)
		b.WriteString(`<input type="hidden" name="action" value="set_business_unit" />`)
		b.WriteString(`<input type="hidden" name="org_code" value="` + html.EscapeString(n.OrgCode) + `" />`)
		b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
		checked := ""
		if n.IsBusinessUnit {
			checked = " checked"
		}
		b.WriteString(`<label>Is Business Unit <input type="checkbox" name="is_business_unit" value="true"` + checked + ` /></label> `)
		b.WriteString(`<button type="submit">Set BU</button>`)
		b.WriteString(`</form>`)
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	return b.String()
}
