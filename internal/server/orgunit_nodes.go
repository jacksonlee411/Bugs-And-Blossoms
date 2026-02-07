package server

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

type OrgUnitNode struct {
	ID             string
	OrgCode        string
	Name           string
	Status         string
	IsBusinessUnit bool
	CreatedAt      time.Time
}

type OrgUnitChild struct {
	OrgID       int
	OrgCode     string
	Name        string
	Status      string
	HasChildren bool
}

type OrgUnitNodeDetails struct {
	OrgID          int
	OrgCode        string
	Name           string
	Status         string
	ParentID       int
	ParentCode     string
	ParentName     string
	IsBusinessUnit bool
	ManagerPernr   string
	ManagerName    string
	PathIDs        []int
	FullNamePath   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	EventUUID      string
}

type OrgUnitSearchResult struct {
	TargetOrgID   int      `json:"target_org_id"`
	TargetOrgCode string   `json:"target_org_code"`
	TargetName    string   `json:"target_name"`
	PathOrgIDs    []int    `json:"path_org_ids"`
	PathOrgCodes  []string `json:"path_org_codes,omitempty"`
	TreeAsOf      string   `json:"tree_as_of"`
}

type OrgUnitSearchCandidate struct {
	OrgID   int
	OrgCode string
	Name    string
	Status  string
}

type OrgUnitNodeVersion struct {
	EventID       int64
	EventUUID     string
	EffectiveDate string
	EventType     string
}

type OrgUnitNodeEffectiveDateCorrector interface {
	CorrectNodeEffectiveDate(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, newEffectiveDate string, requestID string) error
}

var errOrgUnitNotFound = errors.New("org_unit_not_found")

const (
	orgNodeDeleteRecordReason = "UI 删除记录（错误数据）"
	orgNodeDeleteOrgReason    = "UI 删除组织（错误建档）"
)

func orgNodeWriteErrorMessage(err error) string {
	code := strings.TrimSpace(err.Error())
	messages := map[string]string{
		"ORG_CODE_INVALID":                   "组织编码无效",
		"ORG_CODE_NOT_FOUND":                 "组织编码不存在",
		"ORG_EVENT_NOT_FOUND":                "未找到该生效日记录",
		"ORG_REQUEST_ID_CONFLICT":            "请求编号冲突，请刷新后重试",
		"ORG_REPLAY_FAILED":                  "重放失败，操作已回滚",
		"ORG_ROOT_DELETE_FORBIDDEN":          "根组织不允许删除",
		"ORG_HAS_CHILDREN_CANNOT_DELETE":     "存在下级组织，不能删除",
		"ORG_HAS_DEPENDENCIES_CANNOT_DELETE": "存在下游依赖，不能删除",
		"ORG_EVENT_RESCINDED":                "该记录已删除",
		"EFFECTIVE_DATE_INVALID":             "生效日期无效",
		"ORG_INVALID_ARGUMENT":               "请求参数不完整",
	}
	if msg, ok := messages[code]; ok {
		return msg
	}
	return err.Error()
}

func newOrgNodeRequestID(prefix string) string {
	id, _ := uuidv7.NewString()
	return prefix + ":" + id
}

func canEditOrgNodes(ctx context.Context) bool {
	p, ok := currentPrincipal(ctx)
	if !ok {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(p.RoleSlug))
	if role == "" {
		return false
	}
	return role == authz.RoleTenantAdmin || role == authz.RoleSuperadmin
}

type OrgUnitStore interface {
	OrgUnitNodesCurrentReader
	OrgUnitNodesCurrentWriter
	OrgUnitNodesCurrentRenamer
	OrgUnitNodesCurrentMover
	OrgUnitNodesCurrentDisabler
	OrgUnitNodesCurrentBusinessUnitSetter
	OrgUnitCodeResolver
	OrgUnitNodeChildrenReader
	OrgUnitNodeDetailsReader
	OrgUnitNodeSearchReader
	OrgUnitNodeSearchCandidatesReader
	OrgUnitNodeVersionReader
	OrgUnitTreeAsOfReader
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
	ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error)
}

type OrgUnitNodeChildrenReader interface {
	ListChildren(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error)
}

type OrgUnitNodeDetailsReader interface {
	GetNodeDetails(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error)
}

type OrgUnitNodeSearchReader interface {
	SearchNode(ctx context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error)
}

type OrgUnitNodeSearchCandidatesReader interface {
	SearchNodeCandidates(ctx context.Context, tenantID string, query string, asOfDate string, limit int) ([]OrgUnitSearchCandidate, error)
}

type OrgUnitNodeVersionReader interface {
	ListNodeVersions(ctx context.Context, tenantID string, orgID int) ([]OrgUnitNodeVersion, error)
}

type OrgUnitTreeAsOfReader interface {
	MaxEffectiveDateOnOrBefore(ctx context.Context, tenantID string, asOfDate string) (string, bool, error)
	MinEffectiveDate(ctx context.Context, tenantID string) (string, bool, error)
}

type orgUnitNodesVisibilityReader interface {
	ListNodesCurrentWithVisibility(ctx context.Context, tenantID string, asOfDate string, includeDisabled bool) ([]OrgUnitNode, error)
	ListChildrenWithVisibility(ctx context.Context, tenantID string, parentID int, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error)
	GetNodeDetailsWithVisibility(ctx context.Context, tenantID string, orgID int, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error)
	SearchNodeWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, includeDisabled bool) (OrgUnitSearchResult, error)
	SearchNodeCandidatesWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, limit int, includeDisabled bool) ([]OrgUnitSearchCandidate, error)
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

func rejectDeprecatedAsOf(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := r.URL.Query()["as_of"]; ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "deprecated_as_of", "deprecated as_of")
		return false
	}
	return true
}

func requireTreeAsOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !rejectDeprecatedAsOf(w, r) {
		return "", false
	}
	value := strings.TrimSpace(r.URL.Query().Get("tree_as_of"))
	if value == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_tree_as_of", "tree_as_of required")
		return "", false
	}
	if _, err := time.Parse(asOfLayout, value); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_tree_as_of", "invalid tree_as_of")
		return "", false
	}
	return value, true
}

func resolveTreeAsOfForPage(w http.ResponseWriter, r *http.Request, store OrgUnitStore, tenantID string) (string, bool) {
	if !rejectDeprecatedAsOf(w, r) {
		return "", false
	}
	value := strings.TrimSpace(r.URL.Query().Get("tree_as_of"))
	if value != "" {
		if _, err := time.Parse(asOfLayout, value); err == nil {
			return value, true
		}
	}

	systemDay := currentUTCDateString()
	resolved, ok, err := store.MaxEffectiveDateOnOrBefore(r.Context(), tenantID, systemDay)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tree_as_of_error", "tree_as_of error")
		return "", false
	}
	if !ok {
		minValue, minOK, err := store.MinEffectiveDate(r.Context(), tenantID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tree_as_of_error", "tree_as_of error")
			return "", false
		}
		if minOK {
			resolved = minValue
		} else {
			resolved = systemDay
		}
	}

	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		q := r.URL.Query()
		q.Set("tree_as_of", resolved)
		u := *r.URL
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return "", false
	}

	return resolved, true
}

func treeAsOfFromForm(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !rejectDeprecatedAsOf(w, r) {
		return "", false
	}
	value := strings.TrimSpace(r.Form.Get("tree_as_of"))
	if value == "" {
		value = strings.TrimSpace(r.URL.Query().Get("tree_as_of"))
	}
	if value == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_tree_as_of", "tree_as_of required")
		return "", false
	}
	if _, err := time.Parse(asOfLayout, value); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_tree_as_of", "invalid tree_as_of")
		return "", false
	}
	return value, true
}

func parseOptionalTreeAsOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("tree_as_of"))
	if value == "" {
		return "", true
	}
	if _, err := time.Parse(asOfLayout, value); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_tree_as_of", "invalid tree_as_of")
		return "", false
	}
	return value, true
}

func parseIncludeDisabled(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func includeDisabledFromURL(r *http.Request) bool {
	return parseIncludeDisabled(r.URL.Query().Get("include_disabled"))
}

func includeDisabledFromFormOrURL(r *http.Request) bool {
	if parseIncludeDisabled(r.Form.Get("include_disabled")) {
		return true
	}
	return includeDisabledFromURL(r)
}

func includeDisabledQuerySuffix(includeDisabled bool) string {
	if includeDisabled {
		return "&include_disabled=1"
	}
	return ""
}

func listNodesCurrentByVisibility(ctx context.Context, store OrgUnitStore, tenantID string, asOfDate string, includeDisabled bool) ([]OrgUnitNode, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityReader); ok {
			return vStore.ListNodesCurrentWithVisibility(ctx, tenantID, asOfDate, true)
		}
	}
	return store.ListNodesCurrent(ctx, tenantID, asOfDate)
}

func listChildrenByVisibility(ctx context.Context, store OrgUnitStore, tenantID string, parentID int, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityReader); ok {
			return vStore.ListChildrenWithVisibility(ctx, tenantID, parentID, asOfDate, true)
		}
	}
	return store.ListChildren(ctx, tenantID, parentID, asOfDate)
}

func getNodeDetailsByVisibility(ctx context.Context, store OrgUnitStore, tenantID string, orgID int, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityReader); ok {
			return vStore.GetNodeDetailsWithVisibility(ctx, tenantID, orgID, asOfDate, true)
		}
	}
	return store.GetNodeDetails(ctx, tenantID, orgID, asOfDate)
}

func searchNodeByVisibility(ctx context.Context, store OrgUnitStore, tenantID string, query string, asOfDate string, includeDisabled bool) (OrgUnitSearchResult, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityReader); ok {
			return vStore.SearchNodeWithVisibility(ctx, tenantID, query, asOfDate, true)
		}
	}
	return store.SearchNode(ctx, tenantID, query, asOfDate)
}

func searchNodeCandidatesByVisibility(ctx context.Context, store OrgUnitStore, tenantID string, query string, asOfDate string, limit int, includeDisabled bool) ([]OrgUnitSearchCandidate, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityReader); ok {
			return vStore.SearchNodeCandidatesWithVisibility(ctx, tenantID, query, asOfDate, limit, true)
		}
	}
	return store.SearchNodeCandidates(ctx, tenantID, query, asOfDate, limit)
}

func orgUnitStatusLabel(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "disabled") {
		return "无效"
	}
	return "有效"
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

func (s *orgUnitPGStore) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	codes, err := orgunitpkg.ResolveOrgCodes(ctx, tx, tenantID, orgIDs)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return codes, nil
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
	 AND v.org_id = s.org_id
	 AND v.status = 'active'
 AND v.validity @> $2::date
 AND v.parent_id IS NULL
JOIN orgunit.org_events e
  ON e.id = v.last_event_id
ORDER BY v.node_path
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
		n.Status = "active"
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

func (s *orgUnitPGStore) ListNodesCurrentWithVisibility(ctx context.Context, tenantID string, asOfDate string, includeDisabled bool) ([]OrgUnitNode, error) {
	if !includeDisabled {
		return s.ListNodesCurrent(ctx, tenantID, asOfDate)
	}

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
  v.org_id::text,
  c.org_code,
  v.name,
  v.status,
  v.is_business_unit,
  e.transaction_time
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = $1::uuid
 AND c.org_id = v.org_id
JOIN orgunit.org_events e
  ON e.id = v.last_event_id
WHERE v.tenant_uuid = $1::uuid
  AND v.validity @> $2::date
  AND v.parent_id IS NULL
ORDER BY v.node_path
`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNode
	for rows.Next() {
		var n OrgUnitNode
		if err := rows.Scan(&n.ID, &n.OrgCode, &n.Name, &n.Status, &n.IsBusinessUnit, &n.CreatedAt); err != nil {
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

func (s *orgUnitPGStore) ListBusinessUnitsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
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
 AND v.org_id = s.org_id
 AND v.status = 'active'
 AND v.validity @> $2::date
JOIN orgunit.org_events e
  ON e.id = v.last_event_id
WHERE s.is_business_unit
ORDER BY v.node_path
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

func (s *orgUnitPGStore) ListChildren(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
	SELECT EXISTS (
	  SELECT 1
	  FROM orgunit.org_unit_versions
	  WHERE tenant_uuid = $1::uuid
	    AND org_id = $2::int
	    AND status = 'active'
	    AND validity @> $3::date
	)
	`, tenantID, parentID, asOfDate).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errOrgUnitNotFound
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  v.org_id,
	  c.org_code,
	  v.name,
	  EXISTS (
	    SELECT 1
	    FROM orgunit.org_unit_versions child
	    WHERE child.tenant_uuid = $1::uuid
	      AND child.parent_id = v.org_id
	      AND child.status = 'active'
	      AND child.validity @> $3::date
	  ) AS has_children
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = v.org_id
	WHERE v.tenant_uuid = $1::uuid
	  AND v.parent_id = $2::int
	  AND v.status = 'active'
	  AND v.validity @> $3::date
	ORDER BY v.node_path
	`, tenantID, parentID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitChild
	for rows.Next() {
		var item OrgUnitChild
		if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.HasChildren); err != nil {
			return nil, err
		}
		item.Status = "active"
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) ListChildrenWithVisibility(ctx context.Context, tenantID string, parentID int, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error) {
	if !includeDisabled {
		return s.ListChildren(ctx, tenantID, parentID, asOfDate)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
	SELECT EXISTS (
	  SELECT 1
	  FROM orgunit.org_unit_versions
	  WHERE tenant_uuid = $1::uuid
	    AND org_id = $2::int
	    AND validity @> $3::date
	)
	`, tenantID, parentID, asOfDate).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errOrgUnitNotFound
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  v.org_id,
	  c.org_code,
	  v.name,
	  v.status,
	  EXISTS (
	    SELECT 1
	    FROM orgunit.org_unit_versions child
	    WHERE child.tenant_uuid = $1::uuid
	      AND child.parent_id = v.org_id
	      AND child.validity @> $3::date
	  ) AS has_children
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = v.org_id
	WHERE v.tenant_uuid = $1::uuid
	  AND v.parent_id = $2::int
	  AND v.validity @> $3::date
	ORDER BY v.node_path
	`, tenantID, parentID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitChild
	for rows.Next() {
		var item OrgUnitChild
		if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.Status, &item.HasChildren); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) GetNodeDetails(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitNodeDetails{}, err
	}

	var details OrgUnitNodeDetails
	if err := tx.QueryRow(ctx, `
	SELECT
	  v.org_id,
	  c.org_code,
	  v.name,
	  v.status,
	  COALESCE(v.parent_id, 0) AS parent_id,
	  COALESCE(pc.org_code, '') AS parent_org_code,
	  COALESCE(pv.name, '') AS parent_name,
	  v.is_business_unit,
	  COALESCE(p.pernr, '') AS manager_pernr,
	  COALESCE(p.display_name, '') AS manager_name,
	  v.path_ids,
	  COALESCE(v.full_name_path, '') AS full_name_path,
	  c.created_at,
	  e.transaction_time,
	  e.event_uuid
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = v.org_id
	JOIN orgunit.org_events e
	  ON e.id = v.last_event_id
	LEFT JOIN orgunit.org_unit_codes pc
	  ON pc.tenant_uuid = $1::uuid
	 AND pc.org_id = v.parent_id
	LEFT JOIN orgunit.org_unit_versions pv
	  ON pv.tenant_uuid = $1::uuid
	 AND pv.org_id = v.parent_id
	 AND pv.status = 'active'
	 AND pv.validity @> $3::date
	LEFT JOIN person.persons p
	  ON p.tenant_uuid = $1::uuid
	 AND p.person_uuid = v.manager_uuid
	WHERE v.tenant_uuid = $1::uuid
	  AND v.org_id = $2::int
	  AND v.status = 'active'
	  AND v.validity @> $3::date
	LIMIT 1
	`, tenantID, orgID, asOfDate).Scan(
		&details.OrgID,
		&details.OrgCode,
		&details.Name,
		&details.Status,
		&details.ParentID,
		&details.ParentCode,
		&details.ParentName,
		&details.IsBusinessUnit,
		&details.ManagerPernr,
		&details.ManagerName,
		&details.PathIDs,
		&details.FullNamePath,
		&details.CreatedAt,
		&details.UpdatedAt,
		&details.EventUUID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrgUnitNodeDetails{}, errOrgUnitNotFound
		}
		return OrgUnitNodeDetails{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return details, nil
}

func (s *orgUnitPGStore) GetNodeDetailsWithVisibility(ctx context.Context, tenantID string, orgID int, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	if !includeDisabled {
		return s.GetNodeDetails(ctx, tenantID, orgID, asOfDate)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitNodeDetails{}, err
	}

	var details OrgUnitNodeDetails
	if err := tx.QueryRow(ctx, `
	SELECT
	  v.org_id,
	  c.org_code,
	  v.name,
	  v.status,
	  COALESCE(v.parent_id, 0) AS parent_id,
	  COALESCE(pc.org_code, '') AS parent_org_code,
	  COALESCE(pv.name, '') AS parent_name,
	  v.is_business_unit,
	  COALESCE(p.pernr, '') AS manager_pernr,
	  COALESCE(p.display_name, '') AS manager_name,
	  v.path_ids,
	  COALESCE(v.full_name_path, '') AS full_name_path,
	  c.created_at,
	  e.transaction_time,
	  e.event_uuid
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = v.org_id
	JOIN orgunit.org_events e
	  ON e.id = v.last_event_id
	LEFT JOIN orgunit.org_unit_codes pc
	  ON pc.tenant_uuid = $1::uuid
	 AND pc.org_id = v.parent_id
	LEFT JOIN orgunit.org_unit_versions pv
	  ON pv.tenant_uuid = $1::uuid
	 AND pv.org_id = v.parent_id
	 AND pv.validity @> $3::date
	LEFT JOIN person.persons p
	  ON p.tenant_uuid = $1::uuid
	 AND p.person_uuid = v.manager_uuid
	WHERE v.tenant_uuid = $1::uuid
	  AND v.org_id = $2::int
	  AND v.validity @> $3::date
	LIMIT 1
	`, tenantID, orgID, asOfDate).Scan(
		&details.OrgID,
		&details.OrgCode,
		&details.Name,
		&details.Status,
		&details.ParentID,
		&details.ParentCode,
		&details.ParentName,
		&details.IsBusinessUnit,
		&details.ManagerPernr,
		&details.ManagerName,
		&details.PathIDs,
		&details.FullNamePath,
		&details.CreatedAt,
		&details.UpdatedAt,
		&details.EventUUID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrgUnitNodeDetails{}, errOrgUnitNotFound
		}
		return OrgUnitNodeDetails{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return details, nil
}

func (s *orgUnitPGStore) SearchNode(ctx context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return OrgUnitSearchResult{}, errors.New("query is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitSearchResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitSearchResult{}, err
	}

	var result OrgUnitSearchResult
	var pathIDs []int
	found := false

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		if err := tx.QueryRow(ctx, `
		SELECT v.org_id, c.org_code, v.name, v.path_ids
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND c.org_id = v.org_id
		WHERE v.tenant_uuid = $1::uuid
		  AND v.status = 'active'
		  AND v.validity @> $3::date
		  AND c.org_code = $2::text
		LIMIT 1
		`, tenantID, normalized, asOfDate).Scan(&result.TargetOrgID, &result.TargetOrgCode, &result.TargetName, &pathIDs); err == nil {
			found = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return OrgUnitSearchResult{}, err
		}
	}

	if !found {
		if err := tx.QueryRow(ctx, `
		SELECT v.org_id, c.org_code, v.name, v.path_ids
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND c.org_id = v.org_id
		WHERE v.tenant_uuid = $1::uuid
		  AND v.status = 'active'
		  AND v.validity @> $3::date
		  AND v.name ILIKE $2::text
		ORDER BY v.node_path
		LIMIT 1
		`, tenantID, "%"+trimmed+"%", asOfDate).Scan(&result.TargetOrgID, &result.TargetOrgCode, &result.TargetName, &pathIDs); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return OrgUnitSearchResult{}, errOrgUnitNotFound
			}
			return OrgUnitSearchResult{}, err
		}
	}

	result.PathOrgIDs = append([]int(nil), pathIDs...)
	result.TreeAsOf = asOfDate

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitSearchResult{}, err
	}
	return result, nil
}

func (s *orgUnitPGStore) SearchNodeWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, includeDisabled bool) (OrgUnitSearchResult, error) {
	if !includeDisabled {
		return s.SearchNode(ctx, tenantID, query, asOfDate)
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return OrgUnitSearchResult{}, errors.New("query is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitSearchResult{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitSearchResult{}, err
	}

	var result OrgUnitSearchResult
	var pathIDs []int
	found := false

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		if err := tx.QueryRow(ctx, `
		SELECT v.org_id, c.org_code, v.name, v.path_ids
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND c.org_id = v.org_id
		WHERE v.tenant_uuid = $1::uuid
		  AND v.validity @> $3::date
		  AND c.org_code = $2::text
		LIMIT 1
		`, tenantID, normalized, asOfDate).Scan(&result.TargetOrgID, &result.TargetOrgCode, &result.TargetName, &pathIDs); err == nil {
			found = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return OrgUnitSearchResult{}, err
		}
	}

	if !found {
		if err := tx.QueryRow(ctx, `
		SELECT v.org_id, c.org_code, v.name, v.path_ids
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND c.org_id = v.org_id
		WHERE v.tenant_uuid = $1::uuid
		  AND v.validity @> $3::date
		  AND v.name ILIKE $2::text
		ORDER BY v.node_path
		LIMIT 1
		`, tenantID, "%"+trimmed+"%", asOfDate).Scan(&result.TargetOrgID, &result.TargetOrgCode, &result.TargetName, &pathIDs); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return OrgUnitSearchResult{}, errOrgUnitNotFound
			}
			return OrgUnitSearchResult{}, err
		}
	}

	result.PathOrgIDs = append([]int(nil), pathIDs...)
	result.TreeAsOf = asOfDate

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitSearchResult{}, err
	}
	return result, nil
}

func (s *orgUnitPGStore) SearchNodeCandidates(ctx context.Context, tenantID string, query string, asOfDate string, limit int) ([]OrgUnitSearchCandidate, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, errors.New("query is required")
	}
	if limit <= 0 {
		limit = 8
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		rows, err := tx.Query(ctx, `
		SELECT v.org_id, c.org_code, v.name
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND c.org_id = v.org_id
		WHERE v.tenant_uuid = $1::uuid
		  AND v.status = 'active'
		  AND v.validity @> $3::date
		  AND c.org_code = $2::text
		LIMIT 1
		`, tenantID, normalized, asOfDate)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var out []OrgUnitSearchCandidate
		for rows.Next() {
			var item OrgUnitSearchCandidate
			if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name); err != nil {
				return nil, err
			}
			item.Status = "active"
			out = append(out, item)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(out) > 0 {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return out, nil
		}
	}

	rows, err := tx.Query(ctx, `
	SELECT v.org_id, c.org_code, v.name
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = v.org_id
	WHERE v.tenant_uuid = $1::uuid
	  AND v.status = 'active'
	  AND v.validity @> $3::date
	  AND v.name ILIKE $2::text
	ORDER BY v.node_path
	LIMIT $4::int
	`, tenantID, "%"+trimmed+"%", asOfDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitSearchCandidate
	for rows.Next() {
		var item OrgUnitSearchCandidate
		if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name); err != nil {
			return nil, err
		}
		item.Status = "active"
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errOrgUnitNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) SearchNodeCandidatesWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, limit int, includeDisabled bool) ([]OrgUnitSearchCandidate, error) {
	if !includeDisabled {
		return s.SearchNodeCandidates(ctx, tenantID, query, asOfDate, limit)
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, errors.New("query is required")
	}
	if limit <= 0 {
		limit = 8
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		rows, err := tx.Query(ctx, `
		SELECT v.org_id, c.org_code, v.name, v.status
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND c.org_id = v.org_id
		WHERE v.tenant_uuid = $1::uuid
		  AND v.validity @> $3::date
		  AND c.org_code = $2::text
		LIMIT 1
		`, tenantID, normalized, asOfDate)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var out []OrgUnitSearchCandidate
		for rows.Next() {
			var item OrgUnitSearchCandidate
			if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.Status); err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(out) > 0 {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return out, nil
		}
	}

	rows, err := tx.Query(ctx, `
	SELECT v.org_id, c.org_code, v.name, v.status
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND c.org_id = v.org_id
	WHERE v.tenant_uuid = $1::uuid
	  AND v.validity @> $3::date
	  AND v.name ILIKE $2::text
	ORDER BY v.node_path
	LIMIT $4::int
	`, tenantID, "%"+trimmed+"%", asOfDate, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitSearchCandidate
	for rows.Next() {
		var item OrgUnitSearchCandidate
		if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errOrgUnitNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) ListNodeVersions(ctx context.Context, tenantID string, orgID int) ([]OrgUnitNodeVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
	SELECT e.id, e.event_uuid, e.effective_date, e.event_type
	FROM orgunit.org_events_effective e
	WHERE e.tenant_uuid = $1::uuid
	  AND e.org_id = $2::int
	ORDER BY e.effective_date, e.id
	`, tenantID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNodeVersion
	for rows.Next() {
		var item OrgUnitNodeVersion
		var effective time.Time
		if err := rows.Scan(&item.EventID, &item.EventUUID, &effective, &item.EventType); err != nil {
			return nil, err
		}
		item.EffectiveDate = effective.Format("2006-01-02")
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *orgUnitPGStore) MaxEffectiveDateOnOrBefore(ctx context.Context, tenantID string, asOfDate string) (string, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", false, err
	}

	var value *time.Time
	if err := tx.QueryRow(ctx, `
	SELECT max(effective_date)
	FROM orgunit.org_events_effective
	WHERE tenant_uuid = $1::uuid
	  AND effective_date <= $2::date
	`, tenantID, asOfDate).Scan(&value); err != nil {
		return "", false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", false, err
	}
	if value == nil {
		return "", false, nil
	}
	return value.Format(asOfLayout), true, nil
}

func (s *orgUnitPGStore) MinEffectiveDate(ctx context.Context, tenantID string) (string, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", false, err
	}

	var value *time.Time
	if err := tx.QueryRow(ctx, `
	SELECT min(effective_date)
	FROM orgunit.org_events_effective
	WHERE tenant_uuid = $1::uuid
	`, tenantID).Scan(&value); err != nil {
		return "", false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", false, err
	}
	if value == nil {
		return "", false, nil
	}
	return value.Format(asOfLayout), true, nil
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

func (s *orgUnitPGStore) CorrectNodeEffectiveDate(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, newEffectiveDate string, requestID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if strings.TrimSpace(targetEffectiveDate) == "" {
		return errors.New("effective_date is required")
	}
	if strings.TrimSpace(newEffectiveDate) == "" {
		return errors.New("effective_date is required")
	}
	if strings.TrimSpace(requestID) == "" {
		return errors.New("request_id is required")
	}

	patch := `{"effective_date":` + strconv.Quote(newEffectiveDate) + `}`
	var correctionUUID string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event_correction(
  $1::uuid,
  $2::int,
  $3::date,
  $4::jsonb,
  $5::text,
  $6::uuid
)
`, tenantID, orgID, targetEffectiveDate, []byte(patch), requestID, tenantID).Scan(&correctionUUID); err != nil {
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
		Status:         "active",
		IsBusinessUnit: isBusinessUnit,
		CreatedAt:      s.now(),
	}
	s.nodes[tenantID] = append([]OrgUnitNode{n}, s.nodes[tenantID]...)
	return n, nil
}

func (s *orgUnitMemoryStore) ListNodesCurrent(_ context.Context, tenantID string, _ string) ([]OrgUnitNode, error) {
	return s.listNodes(tenantID)
}

func (s *orgUnitMemoryStore) ListNodesCurrentWithVisibility(_ context.Context, tenantID string, _ string, _ bool) ([]OrgUnitNode, error) {
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

func (s *orgUnitMemoryStore) ResolveOrgCodes(_ context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	out := make(map[int]string)
	if len(orgIDs) == 0 {
		return out, nil
	}
	byID := make(map[int]string)
	for _, node := range s.nodes[tenantID] {
		id, err := strconv.Atoi(node.ID)
		if err != nil {
			continue
		}
		byID[id] = node.OrgCode
	}
	for _, orgID := range orgIDs {
		code, ok := byID[orgID]
		if !ok {
			return nil, orgunitpkg.ErrOrgIDNotFound
		}
		out[orgID] = code
	}
	return out, nil
}

func (s *orgUnitMemoryStore) ListChildren(_ context.Context, tenantID string, parentID int, _ string) ([]OrgUnitChild, error) {
	parentIDStr := strconv.Itoa(parentID)
	for _, node := range s.nodes[tenantID] {
		if node.ID == parentIDStr {
			return []OrgUnitChild{}, nil
		}
	}
	return nil, errOrgUnitNotFound
}

func (s *orgUnitMemoryStore) ListChildrenWithVisibility(ctx context.Context, tenantID string, parentID int, asOfDate string, _ bool) ([]OrgUnitChild, error) {
	return s.ListChildren(ctx, tenantID, parentID, asOfDate)
}

func (s *orgUnitMemoryStore) GetNodeDetails(_ context.Context, tenantID string, orgID int, _ string) (OrgUnitNodeDetails, error) {
	orgIDStr := strconv.Itoa(orgID)
	for _, node := range s.nodes[tenantID] {
		if node.ID == orgIDStr {
			return OrgUnitNodeDetails{
				OrgID:          orgID,
				OrgCode:        node.OrgCode,
				Name:           node.Name,
				Status:         strings.TrimSpace(node.Status),
				IsBusinessUnit: node.IsBusinessUnit,
				PathIDs:        []int{orgID},
				FullNamePath:   node.Name,
				CreatedAt:      node.CreatedAt,
				UpdatedAt:      node.CreatedAt,
				EventUUID:      "",
			}, nil
		}
	}
	return OrgUnitNodeDetails{}, errOrgUnitNotFound
}

func (s *orgUnitMemoryStore) GetNodeDetailsWithVisibility(ctx context.Context, tenantID string, orgID int, asOfDate string, _ bool) (OrgUnitNodeDetails, error) {
	return s.GetNodeDetails(ctx, tenantID, orgID, asOfDate)
}

func (s *orgUnitMemoryStore) SearchNode(_ context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return OrgUnitSearchResult{}, errors.New("query is required")
	}

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		for _, node := range s.nodes[tenantID] {
			if node.OrgCode == normalized {
				id, convErr := strconv.Atoi(node.ID)
				if convErr != nil {
					break
				}
				return OrgUnitSearchResult{
					TargetOrgID:   id,
					TargetOrgCode: node.OrgCode,
					TargetName:    node.Name,
					PathOrgIDs:    []int{id},
					TreeAsOf:      asOfDate,
				}, nil
			}
		}
	}

	lower := strings.ToLower(trimmed)
	for _, node := range s.nodes[tenantID] {
		if strings.Contains(strings.ToLower(node.Name), lower) {
			id, convErr := strconv.Atoi(node.ID)
			if convErr != nil {
				break
			}
			return OrgUnitSearchResult{
				TargetOrgID:   id,
				TargetOrgCode: node.OrgCode,
				TargetName:    node.Name,
				PathOrgIDs:    []int{id},
				TreeAsOf:      asOfDate,
			}, nil
		}
	}

	return OrgUnitSearchResult{}, errOrgUnitNotFound
}

func (s *orgUnitMemoryStore) SearchNodeWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, _ bool) (OrgUnitSearchResult, error) {
	return s.SearchNode(ctx, tenantID, query, asOfDate)
}

func (s *orgUnitMemoryStore) SearchNodeCandidates(_ context.Context, tenantID string, query string, _ string, limit int) ([]OrgUnitSearchCandidate, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, errors.New("query is required")
	}
	if limit <= 0 {
		limit = 8
	}
	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		for _, node := range s.nodes[tenantID] {
			if node.OrgCode == normalized {
				id, convErr := strconv.Atoi(node.ID)
				if convErr != nil {
					break
				}
				return []OrgUnitSearchCandidate{{OrgID: id, OrgCode: node.OrgCode, Name: node.Name, Status: strings.TrimSpace(node.Status)}}, nil
			}
		}
	}

	lower := strings.ToLower(trimmed)
	var out []OrgUnitSearchCandidate
	for _, node := range s.nodes[tenantID] {
		if strings.Contains(strings.ToLower(node.Name), lower) {
			id, convErr := strconv.Atoi(node.ID)
			if convErr != nil {
				continue
			}
			out = append(out, OrgUnitSearchCandidate{OrgID: id, OrgCode: node.OrgCode, Name: node.Name, Status: strings.TrimSpace(node.Status)})
			if len(out) >= limit {
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, errOrgUnitNotFound
	}
	return out, nil
}

func (s *orgUnitMemoryStore) SearchNodeCandidatesWithVisibility(ctx context.Context, tenantID string, query string, asOfDate string, limit int, _ bool) ([]OrgUnitSearchCandidate, error) {
	return s.SearchNodeCandidates(ctx, tenantID, query, asOfDate, limit)
}

func (s *orgUnitMemoryStore) ListNodeVersions(_ context.Context, tenantID string, orgID int) ([]OrgUnitNodeVersion, error) {
	orgIDStr := strconv.Itoa(orgID)
	for _, node := range s.nodes[tenantID] {
		if node.ID == orgIDStr {
			return []OrgUnitNodeVersion{{
				EventID:       1,
				EventUUID:     "",
				EffectiveDate: "2026-01-01",
				EventType:     "RENAME",
			}}, nil
		}
	}
	return nil, errOrgUnitNotFound
}

func (s *orgUnitMemoryStore) MaxEffectiveDateOnOrBefore(_ context.Context, tenantID string, asOfDate string) (string, bool, error) {
	if len(s.nodes[tenantID]) == 0 {
		return "", false, nil
	}
	if _, err := time.Parse(asOfLayout, asOfDate); err != nil {
		return "", false, err
	}
	return asOfDate, true, nil
}

func (s *orgUnitMemoryStore) MinEffectiveDate(_ context.Context, tenantID string) (string, bool, error) {
	if len(s.nodes[tenantID]) == 0 {
		return "", false, nil
	}
	return s.now().UTC().Format(asOfLayout), true, nil
}

func handleOrgNodes(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	handleOrgNodesWithWriteService(w, r, store, nil)
}

func handleOrgNodesWithWriteService(w http.ResponseWriter, r *http.Request, store OrgUnitStore, writeSvc orgunitservices.OrgUnitWriteService) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var treeAsOf string
	if r.Method == http.MethodGet {
		resolved, ok := resolveTreeAsOfForPage(w, r, store, tenant.ID)
		if !ok {
			return
		}
		treeAsOf = resolved
	} else {
		resolved, ok := requireTreeAsOf(w, r)
		if !ok {
			return
		}
		treeAsOf = resolved
	}
	includeDisabled := includeDisabledFromURL(r)
	canEdit := canEditOrgNodes(r.Context())

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

		nodes, err := listNodesCurrentByVisibility(r.Context(), store, tenant.ID, treeAsOf, includeDisabled)
		if err != nil {
			return nil, mergeMsg(errHint, err.Error())
		}
		return nodes, errHint
	}

	switch r.Method {
	case http.MethodGet:
		nodes, errMsg := listNodes("")
		writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			nodes, errMsg := listNodes("bad form")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
			return
		}
		resolvedTreeAsOf, ok := treeAsOfFromForm(w, r)
		if !ok {
			return
		}
		treeAsOf = resolvedTreeAsOf
		includeDisabled = includeDisabledFromFormOrURL(r)
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
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
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
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return "", false
			}
			return strconv.Itoa(orgID), true
		}

		if action == "add_record" || action == "insert_record" || action == "delete_record" || action == "delete_org" {
			effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
			if effectiveDate == "" {
				nodes, errMsg := listNodes("effective_date is required")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return
			}
			if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
				nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return
			}

			orgID, ok := resolveOrgID(r.Form.Get("org_code"), "org_code", true)
			if !ok {
				return
			}
			orgIDInt, _ := strconv.Atoi(orgID)

			versions, err := store.ListNodeVersions(r.Context(), tenant.ID, orgIDInt)
			if err != nil {
				nodes, errMsg := listNodes(err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return
			}
			if len(versions) == 0 {
				nodes, errMsg := listNodes("no versions found")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return
			}

			currentEffectiveDate := strings.TrimSpace(r.Form.Get("current_effective_date"))
			if currentEffectiveDate != "" {
				if _, err := time.Parse("2006-01-02", currentEffectiveDate); err != nil {
					nodes, errMsg := listNodes("current_effective_date 无效: " + err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			}

			minDate := ""
			maxDate := ""
			prevDate := ""
			nextDate := ""
			dateExists := false
			selectedExists := false
			for _, v := range versions {
				if v.EffectiveDate == "" {
					continue
				}
				if minDate == "" || v.EffectiveDate < minDate {
					minDate = v.EffectiveDate
				}
				if maxDate == "" || v.EffectiveDate > maxDate {
					maxDate = v.EffectiveDate
				}
				if v.EffectiveDate == effectiveDate {
					dateExists = true
				}
				if currentEffectiveDate != "" {
					if v.EffectiveDate == currentEffectiveDate {
						selectedExists = true
					}
					if v.EffectiveDate < currentEffectiveDate {
						if prevDate == "" || v.EffectiveDate > prevDate {
							prevDate = v.EffectiveDate
						}
					}
					if v.EffectiveDate > currentEffectiveDate {
						if nextDate == "" || v.EffectiveDate < nextDate {
							nextDate = v.EffectiveDate
						}
					}
				}
			}

			if currentEffectiveDate != "" && !selectedExists {
				nodes, errMsg := listNodes("record not found")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return
			}

			switch action {
			case "add_record":
				if dateExists || (maxDate != "" && effectiveDate <= maxDate) {
					nodes, errMsg := listNodes("effective_date conflict")
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			case "insert_record":
				if dateExists {
					nodes, errMsg := listNodes("effective_date conflict")
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
				if currentEffectiveDate != "" {
					if nextDate == "" {
						if maxDate != "" && effectiveDate <= maxDate {
							nodes, errMsg := listNodes("effective_date conflict")
							writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
							return
						}
					} else {
						if prevDate != "" && effectiveDate <= prevDate {
							nodes, errMsg := listNodes("effective_date must be between existing records")
							writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
							return
						}
						if effectiveDate >= nextDate {
							nodes, errMsg := listNodes("effective_date must be between existing records")
							writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
							return
						}
					}
				} else if minDate != "" && maxDate != "" {
					if effectiveDate <= minDate || effectiveDate >= maxDate {
						nodes, errMsg := listNodes("effective_date must be between existing records")
						writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
						return
					}
				}
			case "delete_record":
				if len(versions) <= 1 {
					nodes, errMsg := listNodes("cannot delete last record")
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
				if !dateExists {
					nodes, errMsg := listNodes("record not found")
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			}

			if action == "delete_record" {
				requestID := strings.TrimSpace(r.Form.Get("request_id"))
				if requestID == "" {
					requestID = newOrgNodeRequestID("ui:orgunit:rescind_event")
				}
				reason := strings.TrimSpace(r.Form.Get("reason"))
				if reason == "" {
					reason = orgNodeDeleteRecordReason
				}
				if _, err := writeSvc.RescindRecord(r.Context(), tenant.ID, orgunitservices.RescindRecordOrgUnitRequest{
					OrgCode:             r.Form.Get("org_code"),
					TargetEffectiveDate: effectiveDate,
					RequestID:           requestID,
					Reason:              reason,
				}); err != nil {
					nodes, errMsg := listNodes(orgNodeWriteErrorMessage(err))
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			} else if action == "delete_org" {
				requestID := strings.TrimSpace(r.Form.Get("request_id"))
				if requestID == "" {
					requestID = newOrgNodeRequestID("ui:orgunit:rescind_org")
				}
				reason := strings.TrimSpace(r.Form.Get("reason"))
				if reason == "" {
					reason = orgNodeDeleteOrgReason
				}
				if _, err := writeSvc.RescindOrg(r.Context(), tenant.ID, orgunitservices.RescindOrgUnitRequest{
					OrgCode:   r.Form.Get("org_code"),
					RequestID: requestID,
					Reason:    reason,
				}); err != nil {
					nodes, errMsg := listNodes(orgNodeWriteErrorMessage(err))
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			} else {
				changeType := strings.TrimSpace(strings.ToLower(r.Form.Get("record_change_type")))
				if changeType == "" {
					changeType = "rename"
				}

				switch changeType {
				case "rename":
					name := strings.TrimSpace(r.Form.Get("name"))
					if name == "" {
						baseEffectiveDate := strings.TrimSpace(r.Form.Get("current_effective_date"))
						if baseEffectiveDate == "" {
							baseEffectiveDate = effectiveDate
						}
						baseDetails, err := getNodeDetailsByVisibility(r.Context(), store, tenant.ID, orgIDInt, baseEffectiveDate, includeDisabled)
						if err != nil {
							nodes, errMsg := listNodes(err.Error())
							writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
							return
						}
						name = strings.TrimSpace(baseDetails.Name)
					}
					if name == "" {
						nodes, errMsg := listNodes("name is required")
						writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
						return
					}
					if err := store.RenameNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID, name); err != nil {
						nodes, errMsg := listNodes(err.Error())
						writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
						return
					}
				case "move":
					parentCode := strings.TrimSpace(r.Form.Get("parent_org_code"))
					newParentID, ok := resolveOrgID(parentCode, "parent_org_code", false)
					if !ok {
						return
					}
					if err := store.MoveNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID, newParentID); err != nil {
						nodes, errMsg := listNodes(err.Error())
						writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
						return
					}
				case "set_business_unit":
					isBusinessUnit, err := parseBusinessUnitFlag(r.Form.Get("is_business_unit"))
					if err != nil {
						nodes, errMsg := listNodes(err.Error())
						writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
						return
					}
					reqID := "ui:orgunit:record:set-business-unit:" + orgID + ":" + effectiveDate
					if err := store.SetBusinessUnitCurrent(r.Context(), tenant.ID, effectiveDate, orgID, isBusinessUnit, reqID); err != nil {
						nodes, errMsg := listNodes(err.Error())
						writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
						return
					}
				default:
					nodes, errMsg := listNodes("record_change_type invalid")
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			}

			http.Redirect(w, r, "/org/nodes?tree_as_of="+url.QueryEscape(treeAsOf)+includeDisabledQuerySuffix(includeDisabled), http.StatusSeeOther)
			return
		}

		if action == "rename" || action == "move" || action == "disable" || action == "set_business_unit" {
			effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
			if effectiveDate == "" {
				effectiveDate = treeAsOf
			}
			if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
				nodes, errMsg := listNodes("effective_date 无效: " + err.Error())
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
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
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}

				if err := store.RenameNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID, newName); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			case "move":
				newParentID, ok := resolveOrgID(r.Form.Get("new_parent_code"), "new_parent_code", false)
				if !ok {
					return
				}
				if err := store.MoveNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID, newParentID); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			case "disable":

				if err := store.DisableNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgID); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			case "set_business_unit":
				isBusinessUnit, err := parseBusinessUnitFlag(r.Form.Get("is_business_unit"))
				if err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
				reqID := "ui:orgunit:set-business-unit:" + orgID + ":" + effectiveDate
				if err := store.SetBusinessUnitCurrent(r.Context(), tenant.ID, effectiveDate, orgID, isBusinessUnit, reqID); err != nil {
					nodes, errMsg := listNodes(err.Error())
					writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
					return
				}
			}

			http.Redirect(w, r, "/org/nodes?tree_as_of="+url.QueryEscape(treeAsOf)+includeDisabledQuerySuffix(includeDisabled), http.StatusSeeOther)
			return
		}

		orgCode := r.Form.Get("org_code")
		if orgCode == "" {
			nodes, errMsg := listNodes("org_code is required")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
			return
		}

		name := strings.TrimSpace(r.Form.Get("name"))
		if name == "" {
			nodes, errMsg := listNodes("name is required")
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
			return
		}

		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		if effectiveDate == "" {
			effectiveDate = treeAsOf
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
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
			return
		}

		isBusinessUnit, err := parseBusinessUnitFlag(r.Form.Get("is_business_unit"))
		if err != nil {
			nodes, errMsg := listNodes(err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
			return
		}

		if _, err := store.CreateNodeCurrent(r.Context(), tenant.ID, effectiveDate, orgCode, name, parentID, isBusinessUnit); err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeInvalid) {
				nodes, errMsg := listNodes("org_code invalid")
				writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
				return
			}
			nodes, errMsg := listNodes(err.Error())
			writePage(w, r, renderOrgNodes(nodes, tenant, errMsg, treeAsOf, includeDisabled, canEdit))
			return
		}

		http.Redirect(w, r, "/org/nodes?tree_as_of="+url.QueryEscape(treeAsOf)+includeDisabledQuerySuffix(includeDisabled), http.StatusSeeOther)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func handleOrgNodeChildren(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	treeAsOf, ok := requireTreeAsOf(w, r)
	if !ok {
		return
	}
	includeDisabled := includeDisabledFromURL(r)

	parentIDRaw := strings.TrimSpace(r.URL.Query().Get("parent_id"))
	if parentIDRaw == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "parent_id_required", "parent_id required")
		return
	}
	parentID, err := parseOrgID8(parentIDRaw)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "parent_id_invalid", "parent_id invalid")
		return
	}

	children, err := listChildrenByVisibility(r.Context(), store, tenant.ID, parentID, treeAsOf, includeDisabled)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_children_error", "org nodes children error")
		return
	}

	var b strings.Builder
	for _, child := range children {
		b.WriteString(`<sl-tree-item slot="children" data-org-id="`)
		b.WriteString(strconv.Itoa(child.OrgID))
		b.WriteString(`" data-org-code="`)
		b.WriteString(html.EscapeString(child.OrgCode))
		b.WriteString(`" data-has-children="`)
		b.WriteString(strconv.FormatBool(child.HasChildren))
		b.WriteString(`"`)
		if child.HasChildren {
			b.WriteString(` lazy`)
		}
		b.WriteString(`>`)
		b.WriteString(html.EscapeString(child.Name))
		if strings.EqualFold(strings.TrimSpace(child.Status), "disabled") {
			b.WriteString(` <span class="org-node-status-tag">(无效)</span>`)
		}
		b.WriteString(`</sl-tree-item>`)
	}

	writeContent(w, r, b.String())
}

func renderOrgNodeDetails(details OrgUnitNodeDetails, effectiveDate string, treeAsOf string, includeDisabled bool, versions []OrgUnitNodeVersion, canEdit bool, flash string) string {
	parentLabel := "-"
	if details.ParentID != 0 {
		label := details.ParentCode
		if label != "" && details.ParentName != "" {
			label = label + " · " + details.ParentName
		} else if details.ParentName != "" {
			label = details.ParentName
		}
		if label != "" {
			parentLabel = label
		}
	}

	managerLabel := "-"
	if details.ManagerPernr != "" || details.ManagerName != "" {
		managerLabel = strings.TrimSpace(details.ManagerPernr + " " + details.ManagerName)
		if managerLabel == "" {
			managerLabel = "-"
		}
	}

	fullNamePath := details.FullNamePath
	if strings.TrimSpace(fullNamePath) == "" {
		fullNamePath = "-"
	}

	selectedVersion, selectedIdx := selectOrgNodeVersion(effectiveDate, versions)
	currentEffectiveDate := effectiveDate
	currentEventID := ""
	if selectedIdx >= 0 {
		currentEffectiveDate = selectedVersion.EffectiveDate
		if selectedVersion.EventID != 0 {
			currentEventID = strconv.FormatInt(selectedVersion.EventID, 10)
		}
	}

	prevDate := ""
	nextDate := ""
	if selectedIdx > 0 {
		prevDate = versions[selectedIdx-1].EffectiveDate
	}
	if selectedIdx >= 0 && selectedIdx < len(versions)-1 {
		nextDate = versions[selectedIdx+1].EffectiveDate
	}
	minDate := ""
	maxDate := ""
	for _, v := range versions {
		if v.EffectiveDate == "" {
			continue
		}
		if minDate == "" || v.EffectiveDate < minDate {
			minDate = v.EffectiveDate
		}
		if maxDate == "" || v.EffectiveDate > maxDate {
			maxDate = v.EffectiveDate
		}
	}

	successMsg := ""
	if flash == "success" {
		successMsg = "更新成功"
	}
	currentStatus := strings.ToLower(strings.TrimSpace(details.Status))
	if currentStatus == "" {
		currentStatus = "active"
	}
	statusLabel := orgUnitStatusLabel(currentStatus)
	warnMsg := ""
	if !canEdit {
		warnMsg = "无更新权限，无法编辑"
	}
	if flash == "status_disabled_visible" {
		msg := "当前组织为无效状态，可切换为有效"
		if warnMsg == "" {
			warnMsg = msg
		} else {
			warnMsg = warnMsg + "；" + msg
		}
	}

	disabledAttr := ""
	if !canEdit {
		disabledAttr = " disabled"
	}

	includeDisabledValue := "0"
	if includeDisabled {
		includeDisabledValue = "1"
	}

	var b strings.Builder
	b.WriteString(`<div class="org-node-details-panel" data-org-id="` + html.EscapeString(strconv.Itoa(details.OrgID)) + `"`)
	b.WriteString(` data-org-code="` + html.EscapeString(details.OrgCode) + `"`)
	b.WriteString(` data-current-effective-date="` + html.EscapeString(currentEffectiveDate) + `"`)
	b.WriteString(` data-current-event-id="` + html.EscapeString(currentEventID) + `"`)
	b.WriteString(` data-current-name="` + html.EscapeString(details.Name) + `"`)
	b.WriteString(` data-current-status="` + html.EscapeString(currentStatus) + `"`)
	b.WriteString(` data-include-disabled="` + html.EscapeString(includeDisabledValue) + `"`)
	b.WriteString(` data-current-parent-code="` + html.EscapeString(details.ParentCode) + `"`)
	b.WriteString(` data-current-is-business-unit="` + html.EscapeString(strconv.FormatBool(details.IsBusinessUnit)) + `"`)
	b.WriteString(` data-min-effective-date="` + html.EscapeString(minDate) + `"`)
	b.WriteString(` data-max-effective-date="` + html.EscapeString(maxDate) + `"`)
	b.WriteString(` data-prev-effective-date="` + html.EscapeString(prevDate) + `"`)
	b.WriteString(` data-next-effective-date="` + html.EscapeString(nextDate) + `"`)
	b.WriteString(` data-mode="readonly">`)

	b.WriteString(`<div class="org-node-status-messages">`)
	b.WriteString(`<div class="org-node-status-row org-node-status-success">` + html.EscapeString(successMsg) + `</div>`)
	b.WriteString(`<div class="org-node-status-row org-node-status-error"></div>`)
	b.WriteString(`<div class="org-node-status-row org-node-status-warn">` + html.EscapeString(warnMsg) + `</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="org-node-header">`)
	b.WriteString(`<div class="org-node-header-main">`)
	b.WriteString(`<div class="org-node-name">` + html.EscapeString(details.Name) + `</div>`)
	b.WriteString(`<div class="org-node-code">` + html.EscapeString(details.OrgCode) + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<span class="org-node-status-badge">` + html.EscapeString(statusLabel) + `</span>`)
	b.WriteString(`<div class="org-node-header-spacer"></div>`)
	b.WriteString(`<button type="button" class="org-node-edit-btn"` + disabledAttr + `>编辑</button>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="org-node-records">`)
	b.WriteString(`<div class="org-node-records-header">`)
	b.WriteString(`<span class="org-node-records-title">生效记录</span>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-records-actions">`)
	b.WriteString(`<div class="org-node-records-section">`)
	b.WriteString(`<div class="org-node-records-section-title">新版本操作</div>`)
	b.WriteString(`<div class="org-node-records-section-hint">新增/插入将生成新版本，不改写历史</div>`)
	b.WriteString(`<div class="org-node-records-section-actions">`)
	b.WriteString(`<button type="button" class="org-node-record-btn" data-action="add_record"` + disabledAttr + `>新增记录</button>`)
	b.WriteString(`<button type="button" class="org-node-record-btn is-muted" data-action="insert_record"` + disabledAttr + `>插入记录</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-records-section">`)
	b.WriteString(`<div class="org-node-records-section-title">历史修正</div>`)
	b.WriteString(`<div class="org-node-records-section-hint">更正当前选中版本（历史纠错）</div>`)
	b.WriteString(`<div class="org-node-records-section-actions">`)
	b.WriteString(`<button type="button" class="org-node-record-btn is-warning" data-action="correct_record"` + disabledAttr + `>修正记录</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-records-section is-danger">`)
	b.WriteString(`<div class="org-node-records-section-title">危险操作</div>`)
	b.WriteString(`<div class="org-node-records-section-hint">该操作将删除错误数据（通过事件撤销实现），并立即重放版本；操作可审计，不可撤销。</div>`)
	b.WriteString(`<div class="org-node-records-section-actions">`)
	b.WriteString(`<button type="button" class="org-node-record-btn is-danger" data-action="delete_record"` + disabledAttr + `>删除记录（错误数据）</button>`)
	b.WriteString(`<button type="button" class="org-node-record-btn is-danger" data-action="delete_org"` + disabledAttr + `>删除组织（错误建档）</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="org-node-version-nav">`)
	prevDisabled := ""
	if prevDate == "" {
		prevDisabled = " disabled"
	}
	nextDisabled := ""
	if nextDate == "" {
		nextDisabled = " disabled"
	}
	b.WriteString(`<button type="button" class="org-node-version-btn" data-target-date="` + html.EscapeString(prevDate) + `"` + prevDisabled + `>上一条</button>`)
	b.WriteString(`<button type="button" class="org-node-version-btn" data-target-date="` + html.EscapeString(nextDate) + `"` + nextDisabled + `>下一条</button>`)
	b.WriteString(`<div class="org-node-version-spacer"></div>`)
	b.WriteString(`<select class="org-node-version-select">`)
	if len(versions) == 0 {
		b.WriteString(`<option value="` + html.EscapeString(currentEffectiveDate) + `" selected>` + html.EscapeString(currentEffectiveDate) + `</option>`)
	} else {
		for i, v := range versions {
			label := v.EffectiveDate
			if v.EventType != "" {
				label = label + " · " + v.EventType
			}
			selected := ""
			if i == selectedIdx {
				selected = " selected"
			}
			b.WriteString(`<option value="` + html.EscapeString(v.EffectiveDate) + `"` + selected + `>` + html.EscapeString(label) + `</option>`)
		}
	}
	b.WriteString(`</select>`)
	if len(versions) > 0 && selectedIdx >= 0 {
		b.WriteString(`<div class="org-node-version-count">` + strconv.Itoa(selectedIdx+1) + `/` + strconv.Itoa(len(versions)) + `</div>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-records-hint">切换版本后，下方编辑区加载对应记录</div>`)

	b.WriteString(`<div class="org-node-record-form" data-open="false">`)
	b.WriteString(`<div class="org-node-record-form-title">新增记录</div>`)
	b.WriteString(`<form method="POST" class="org-node-record-action-form" action="/org/nodes?tree_as_of=` + html.EscapeString(treeAsOf) + `&include_disabled=` + html.EscapeString(includeDisabledValue) + `">`)
	b.WriteString(`<input type="hidden" name="action" value="add_record" />`)
	b.WriteString(`<input type="hidden" name="tree_as_of" value="` + html.EscapeString(treeAsOf) + `" />`)
	b.WriteString(`<input type="hidden" name="include_disabled" value="` + html.EscapeString(includeDisabledValue) + `" />`)
	b.WriteString(`<input type="hidden" name="current_effective_date" value="` + html.EscapeString(currentEffectiveDate) + `" />`)
	b.WriteString(`<input type="hidden" name="org_code" value="` + html.EscapeString(details.OrgCode) + `" />`)
	b.WriteString(`<input type="hidden" name="request_id" value="" />`)
	b.WriteString(`<input type="hidden" name="reason" value="" />`)
	b.WriteString(`<div class="org-node-record-stepper">`)
	b.WriteString(`<span class="org-node-record-stepper-item" data-step="1">1 意图</span>`)
	b.WriteString(`<span class="org-node-record-stepper-item" data-step="2">2 日期</span>`)
	b.WriteString(`<span class="org-node-record-stepper-item" data-step="3">3 字段</span>`)
	b.WriteString(`<span class="org-node-record-stepper-item" data-step="4">4 确认</span>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-record-step" data-step="1"><div class="org-node-record-intent"></div></div>`)
	b.WriteString(`<div class="org-node-record-step" data-step="2">`)
	b.WriteString(`<label>生效日期 <input type="date" name="effective_date" value="` + html.EscapeString(currentEffectiveDate) + `" /></label>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-record-step" data-step="3">`)
	b.WriteString(`<label class="org-node-record-change">变更类型 `)
	b.WriteString(`<select name="record_change_type">`)
	b.WriteString(`<option value="rename">组织名称</option>`)
	b.WriteString(`<option value="move">上级组织</option>`)
	b.WriteString(`<option value="set_business_unit">业务单元</option>`)
	b.WriteString(`</select></label>`)
	b.WriteString(`<label class="org-node-record-name">组织名称 <input name="name" value="" /></label>`)
	b.WriteString(`<label class="org-node-record-parent">上级组织 <input name="parent_org_code" value="" /></label>`)
	b.WriteString(`<label class="org-node-record-business-unit"><input type="checkbox" name="is_business_unit" value="true" /> 业务单元</label>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-record-step" data-step="4"><div class="org-node-record-summary"></div></div>`)
	b.WriteString(`<div class="org-node-record-form-actions">`)
	b.WriteString(`<button type="button" class="org-node-record-prev">上一步</button>`)
	b.WriteString(`<button type="button" class="org-node-record-next">下一步</button>`)
	b.WriteString(`<button type="submit" class="org-node-record-submit">保存</button>`)
	b.WriteString(`<button type="button" class="org-node-record-cancel">取消</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-record-form-hint"></div>`)
	b.WriteString(`</form>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="org-node-tabs">`)
	b.WriteString(`<button type="button" class="org-node-tab-btn is-active" data-tab="basic">基本信息</button>`)
	b.WriteString(`<button type="button" class="org-node-tab-btn" data-tab="change">修改记录</button>`)
	b.WriteString(`<div class="org-node-tab-spacer"></div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="org-node-readonly" data-panel="readonly">`)
	b.WriteString(`<div class="org-node-info-list" data-tab-content="basic">`)
	b.WriteString(`<div class="org-node-info-item">生效日期：` + html.EscapeString(currentEffectiveDate) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">状态：` + html.EscapeString(statusLabel) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">组织名称：` + html.EscapeString(details.Name) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">组织编码：` + html.EscapeString(details.OrgCode) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">上级组织：` + html.EscapeString(parentLabel) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">部门负责人：` + html.EscapeString(managerLabel) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">组织长名称：` + html.EscapeString(fullNamePath) + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-info-list" data-tab-content="change" style="display:none">`)
	b.WriteString(`<div class="org-node-info-item">组织ID：` + html.EscapeString(strconv.Itoa(details.OrgID)) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">组织ID链：` + html.EscapeString(formatOrgNodePathIDs(details.PathIDs)) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">修改人：-</div>`)
	b.WriteString(`<div class="org-node-info-item">创建日期：` + html.EscapeString(formatOrgNodeDate(details.CreatedAt)) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">修改日期：` + html.EscapeString(formatOrgNodeDate(details.UpdatedAt)) + `</div>`)
	b.WriteString(`<div class="org-node-info-item">UUID：` + html.EscapeString(details.EventUUID) + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="org-node-edit" data-panel="edit">`)
	b.WriteString(`<div class="org-node-edit-header">`)
	b.WriteString(`<div class="org-node-edit-title">组织信息</div>`)
	b.WriteString(`<div class="org-node-header-spacer"></div>`)
	b.WriteString(`<div class="org-node-edit-actions">`)
	b.WriteString(`<button type="button" class="org-node-save-btn"` + disabledAttr + `>保存</button>`)
	b.WriteString(`<button type="button" class="org-node-cancel-btn">取消</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-unsaved" style="display:none">未保存变更</div>`)
	b.WriteString(`<form class="org-node-edit-form" data-org-code="` + html.EscapeString(details.OrgCode) + `"`)
	b.WriteString(` data-original-effective-date="` + html.EscapeString(currentEffectiveDate) + `"`)
	b.WriteString(` data-original-name="` + html.EscapeString(details.Name) + `"`)
	b.WriteString(` data-original-parent-code="` + html.EscapeString(details.ParentCode) + `"`)
	b.WriteString(` data-original-manager-pernr="` + html.EscapeString(details.ManagerPernr) + `"`)
	b.WriteString(` data-original-manager-name="` + html.EscapeString(details.ManagerName) + `">`)
	b.WriteString(`<label>生效日期 <input type="date" name="effective_date" value="` + html.EscapeString(currentEffectiveDate) + `" /></label>`)
	b.WriteString(`<label>组织名称* <input name="name" value="` + html.EscapeString(details.Name) + `" /></label>`)
	b.WriteString(`<label>组织编码* <input name="org_code" value="` + html.EscapeString(details.OrgCode) + `" readonly /></label>`)
	b.WriteString(`<label>上级组织 <input name="parent_org_code" value="` + html.EscapeString(details.ParentCode) + `" /></label>`)
	b.WriteString(`<label>部门负责人 <input name="manager_pernr" value="` + html.EscapeString(details.ManagerPernr) + `" /></label>`)
	b.WriteString(`<label>负责人姓名 <input name="manager_name" value="` + html.EscapeString(details.ManagerName) + `" readonly /></label>`)
	b.WriteString(`<label>组织长名称 <input name="full_name_path" value="` + html.EscapeString(fullNamePath) + `" readonly /></label>`)
	b.WriteString(`</form>`)
	b.WriteString(`<div class="org-node-edit-note">保存失败时保留已编辑内容并提示重试。</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`</div>`)
	return b.String()
}
func handleOrgNodeDetails(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !rejectDeprecatedAsOf(w, r) {
		return
	}
	effectiveDate := strings.TrimSpace(r.URL.Query().Get("effective_date"))
	if effectiveDate != "" {
		if _, err := time.Parse(asOfLayout, effectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
	} else {
		effectiveDate = currentUTCDateString()
	}
	treeAsOf, ok := parseOptionalTreeAsOf(w, r)
	if !ok {
		return
	}
	if treeAsOf == "" {
		treeAsOf = currentUTCDateString()
	}
	includeDisabled := includeDisabledFromURL(r)

	orgIDRaw := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgIDRaw == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "org_id_required", "org_id required")
		return
	}
	orgID, err := parseOrgID8(orgIDRaw)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "org_id_invalid", "org_id invalid")
		return
	}

	details, err := getNodeDetailsByVisibility(r.Context(), store, tenant.ID, orgID, effectiveDate, includeDisabled)
	if err != nil {
		if !includeDisabled && errors.Is(err, errOrgUnitNotFound) {
			fallback, fallbackErr := getNodeDetailsByVisibility(r.Context(), store, tenant.ID, orgID, effectiveDate, true)
			if fallbackErr == nil {
				details = fallback
				includeDisabled = true
				err = nil
			}
		}
	}
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_details_error", "org node details error")
		return
	}

	versions, err := store.ListNodeVersions(r.Context(), tenant.ID, details.OrgID)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_details_error", "org node details error")
		return
	}

	flash := strings.TrimSpace(r.URL.Query().Get("flash"))
	if flash == "" && strings.EqualFold(strings.TrimSpace(details.Status), "disabled") {
		flash = "status_disabled_visible"
	}
	panel := renderOrgNodeDetails(details, effectiveDate, treeAsOf, includeDisabled, versions, canEditOrgNodes(r.Context()), flash)
	writeContent(w, r, panel)
}

func handleOrgNodeDetailsPage(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !rejectDeprecatedAsOf(w, r) {
		return
	}
	effectiveDate := strings.TrimSpace(r.URL.Query().Get("effective_date"))
	if effectiveDate != "" {
		if _, err := time.Parse(asOfLayout, effectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}
	} else {
		effectiveDate = currentUTCDateString()
	}
	treeAsOf, ok := parseOptionalTreeAsOf(w, r)
	if !ok {
		return
	}
	if treeAsOf == "" {
		treeAsOf = currentUTCDateString()
	}
	includeDisabled := includeDisabledFromURL(r)

	orgIDRaw := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgIDRaw == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "org_id_required", "org_id required")
		return
	}
	orgID, err := parseOrgID8(orgIDRaw)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "org_id_invalid", "org_id invalid")
		return
	}

	details, err := getNodeDetailsByVisibility(r.Context(), store, tenant.ID, orgID, effectiveDate, includeDisabled)
	if err != nil {
		if !includeDisabled && errors.Is(err, errOrgUnitNotFound) {
			fallback, fallbackErr := getNodeDetailsByVisibility(r.Context(), store, tenant.ID, orgID, effectiveDate, true)
			if fallbackErr == nil {
				details = fallback
				includeDisabled = true
				err = nil
			}
		}
	}
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_details_error", "org node details error")
		return
	}

	versions, err := store.ListNodeVersions(r.Context(), tenant.ID, details.OrgID)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_details_error", "org node details error")
		return
	}

	nodesURL := "/org/nodes?tree_as_of=" + url.QueryEscape(treeAsOf)
	if includeDisabled {
		nodesURL += "&include_disabled=1"
	}
	escapedNodesURL := html.EscapeString(nodesURL)
	var b strings.Builder
	b.WriteString("<h1>OrgUnit / Details</h1>")
	b.WriteString(`<p>Tenant: <code>` + html.EscapeString(tenant.Name) + `</code> (<code>` + html.EscapeString(tenant.ID) + `</code>)</p>`)
	b.WriteString(`<p>Tree as-of: <code>` + html.EscapeString(treeAsOf) + `</code> | <a href="` + escapedNodesURL + `" hx-get="` + escapedNodesURL + `" hx-target="#content" hx-push-url="true">Back to Org Nodes</a></p>`)
	b.WriteString(renderOrgNodeDetails(details, effectiveDate, treeAsOf, includeDisabled, versions, canEditOrgNodes(r.Context()), ""))
	writePage(w, r, b.String())
}

func handleOrgNodeSearch(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	treeAsOf, ok := requireTreeAsOf(w, r)
	if !ok {
		return
	}
	includeDisabled := includeDisabledFromURL(r)

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "query_required", "query required")
		return
	}

	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "panel" {
		items, err := searchNodeCandidatesByVisibility(r.Context(), store, tenant.ID, query, treeAsOf, 8, includeDisabled)
		if err != nil {
			if errors.Is(err, errOrgUnitNotFound) {
				writeContent(w, r, renderOrgNodeSearchPanel(nil))
				return
			}
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_search_error", "org node search error")
			return
		}
		writeContent(w, r, renderOrgNodeSearchPanel(items))
		return
	}

	result, err := searchNodeByVisibility(r.Context(), store, tenant.ID, query, treeAsOf, includeDisabled)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "org_search_error", "org node search error")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func renderOrgNodeSearchPanel(items []OrgUnitSearchCandidate) string {
	var b strings.Builder
	b.WriteString(`<div class="org-node-search-panel" data-count="`)
	b.WriteString(strconv.Itoa(len(items)))
	b.WriteString(`">`)
	if len(items) == 0 {
		b.WriteString(`<div class="org-node-search-empty">未找到匹配组织</div>`)
		b.WriteString(`</div>`)
		return b.String()
	}
	b.WriteString(`<div class="org-node-search-hint">匹配多个结果时可选择回填</div>`)
	for _, item := range items {
		b.WriteString(`<button type="button" class="org-node-search-item" data-org-id="`)
		b.WriteString(strconv.Itoa(item.OrgID))
		b.WriteString(`" data-org-code="`)
		b.WriteString(html.EscapeString(item.OrgCode))
		b.WriteString(`">`)
		b.WriteString(`<span class="org-node-search-name">` + html.EscapeString(item.Name) + `</span>`)
		b.WriteString(`<span class="org-node-search-code">` + html.EscapeString(item.OrgCode) + `</span>`)
		if strings.EqualFold(strings.TrimSpace(item.Status), "disabled") {
			b.WriteString(`<span class="org-node-status-tag">无效</span>`)
		}
		b.WriteString(`</button>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func formatOrgNodeDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02")
}

func formatOrgNodePathIDs(pathIDs []int) string {
	if len(pathIDs) == 0 {
		return "-"
	}
	out := make([]string, 0, len(pathIDs))
	for _, id := range pathIDs {
		out = append(out, strconv.Itoa(id))
	}
	return strings.Join(out, ".")
}

func selectOrgNodeVersion(asOf string, versions []OrgUnitNodeVersion) (OrgUnitNodeVersion, int) {
	if len(versions) == 0 {
		return OrgUnitNodeVersion{}, -1
	}
	asOfTime, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		return versions[len(versions)-1], len(versions) - 1
	}
	selected := versions[0]
	idx := 0
	for i, v := range versions {
		t, err := time.Parse("2006-01-02", v.EffectiveDate)
		if err != nil {
			continue
		}
		if t.After(asOfTime) {
			break
		}
		selected = v
		idx = i
	}
	return selected, idx
}

func renderOrgNodes(nodes []OrgUnitNode, tenant Tenant, errMsg string, treeAsOf string, includeDisabled bool, canEdit bool) string {
	var b strings.Builder
	b.WriteString(`<div class="org-nodes-shell" id="org-nodes-root" data-can-edit="`)
	if canEdit {
		b.WriteString(`true`)
	} else {
		b.WriteString(`false`)
	}
	b.WriteString(`" data-include-disabled="`)
	if includeDisabled {
		b.WriteString(`true`)
	} else {
		b.WriteString(`false`)
	}
	b.WriteString(`">`)

	includeDisabledValue := "0"
	includeDisabledChecked := ""
	if includeDisabled {
		includeDisabledValue = "1"
		includeDisabledChecked = " checked"
	}

	b.WriteString(`<div class="org-nodes-header">`)
	b.WriteString(`<div class="org-nodes-header-main">`)
	b.WriteString(`<h1 class="org-nodes-title">OrgUnit Details</h1>`)
	b.WriteString(`<div class="org-nodes-meta">Tenant: ` + html.EscapeString(tenant.Name) + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	if errMsg != "" {
		b.WriteString(`<div class="org-node-banner org-node-banner-error">` + html.EscapeString(errMsg) + `</div>`)
	}

	b.WriteString(`<div class="org-nodes-layout">`)
	b.WriteString(`<section class="org-nodes-panel org-nodes-tree-panel">`)
	b.WriteString(`<div class="org-nodes-panel-title">Nodes</div>`)
	b.WriteString(`<div class="org-nodes-panel-hint">当前仅显示根节点。可在右侧详情中查找并编辑组织。</div>`)
	b.WriteString(`<form method="GET" action="/org/nodes" class="org-nodes-tree-asof">`)
	b.WriteString(`<label class="org-nodes-asof-label">生效日期</label>`)
	b.WriteString(`<input type="date" name="tree_as_of" value="` + html.EscapeString(treeAsOf) + `" />`)
	b.WriteString(`<label class="org-node-include-disabled"><input type="checkbox" name="include_disabled" value="1"` + includeDisabledChecked + ` /> 显示无效组织</label>`)
	b.WriteString(`<button type="submit">应用</button>`)
	b.WriteString(`</form>`)
	createDisabled := ""
	if !canEdit {
		createDisabled = " disabled"
	}
	b.WriteString(`<button type="button" class="org-node-create-btn"` + createDisabled + `>新建部门</button>`)
	b.WriteString(`<div class="org-node-tree-wrap">`)
	b.WriteString(`<sl-tree id="org-node-tree" selection="single">`)
	if len(nodes) == 0 {
		b.WriteString(`<div class="org-node-empty">该租户暂无组织数据</div>`)
		b.WriteString(`<sl-tree-item disabled>暂无组织数据</sl-tree-item>`)
	} else {
		for _, n := range nodes {
			codeLabel := n.OrgCode
			if strings.TrimSpace(codeLabel) == "" {
				codeLabel = "(missing org_code)"
			}
			b.WriteString(`<sl-tree-item data-org-id="`)
			b.WriteString(html.EscapeString(n.ID))
			b.WriteString(`" data-org-code="`)
			b.WriteString(html.EscapeString(n.OrgCode))
			b.WriteString(`" data-has-children="true" lazy>`)
			b.WriteString(html.EscapeString(n.Name))
			b.WriteString(` <span class="org-node-code">` + html.EscapeString(codeLabel) + `</span>`)
			if n.IsBusinessUnit {
				b.WriteString(` <span class="org-node-bu">(BU)</span>`)
			}
			if strings.EqualFold(strings.TrimSpace(n.Status), "disabled") {
				b.WriteString(` <span class="org-node-status-tag">(无效)</span>`)
			}
			b.WriteString(`</sl-tree-item>`)
		}
	}
	b.WriteString(`</sl-tree>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	b.WriteString(`<div class="org-nodes-resize-handle" aria-hidden="true"></div>`)
	b.WriteString(`<section class="org-nodes-panel org-nodes-details-panel">`)
	b.WriteString(`<div class="org-node-search-block">`)
	b.WriteString(`<div class="org-node-search-label">查找组织（ID / Code / 名称）</div>`)
	b.WriteString(`<form class="org-node-search-form" method="GET" action="/org/nodes/search">`)
	b.WriteString(`<input type="hidden" name="tree_as_of" value="` + html.EscapeString(treeAsOf) + `" />`)
	b.WriteString(`<input type="hidden" name="include_disabled" value="` + includeDisabledValue + `" />`)
	b.WriteString(`<div class="org-node-search-row">`)
	b.WriteString(`<input name="query" placeholder="输入 ID / Code / 名称" />`)
	b.WriteString(`<button type="submit">查找</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</form>`)
	b.WriteString(`<div id="org-node-search-error" class="org-node-search-error" aria-live="polite"></div>`)
	b.WriteString(`<div id="org-node-search-results" class="org-node-search-results"></div>`)
	b.WriteString(`<div class="org-node-search-helper">未找到匹配组织时提示：未找到匹配组织</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="org-node-details-container" id="org-node-details">`)
	b.WriteString(`<div class="org-node-placeholder">请选择左侧组织，或在上方查找。</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)
	b.WriteString(`</div>`)

	b.WriteString(renderOrgNodeCreateTemplate(treeAsOf, includeDisabled, canEdit))

	b.WriteString(`<script>
(function() {
  const init = () => {
    const root = document.getElementById("org-nodes-root");
    const tree = document.getElementById("org-node-tree");
    if (!root || !tree || !window.htmx) {
      return false;
    }
    if (tree.dataset.orgNodesReady === "true") {
      return true;
    }

    const canEdit = root.dataset.canEdit === "true";
    let lastSelectedOrgId = "";

	    const getTreeAsOf = () => {
	      const params = new URLSearchParams(window.location.search);
	      return params.get("tree_as_of") || "";
	    };

	    const includeDisabledEnabled = () => {
	      const params = new URLSearchParams(window.location.search);
	      const raw = String(params.get("include_disabled") || "").trim().toLowerCase();
	      return raw === "1" || raw === "true" || raw === "yes" || raw === "on";
	    };

    const getDetailsContainer = () => document.getElementById("org-node-details");

    const setSearchError = (msg) => {
      const errorEl = document.getElementById("org-node-search-error");
      if (errorEl) {
        errorEl.textContent = msg || "";
      }
    };

    const setSearchResults = (html) => {
      const target = document.getElementById("org-node-search-results");
      if (!target) {
        return null;
      }
      target.innerHTML = html || "";
      return target.querySelector(".org-node-search-panel");
    };

    const getDetailsPanel = () => {
      const container = getDetailsContainer();
      if (!container) {
        return null;
      }
      return container.querySelector(".org-node-details-panel") || container.querySelector(".org-node-create-panel");
    };

    const setStatus = (panel, kind, msg) => {
      if (!panel) {
        return;
      }
      const el = panel.querySelector(".org-node-status-" + kind);
      if (el) {
        el.textContent = msg || "";
      }
    };

    const clearStatus = (panel) => {
      setStatus(panel, "success", "");
      setStatus(panel, "error", "");
      setStatus(panel, "warn", "");
    };

    const setTab = (panel, tab) => {
      if (!panel) {
        return;
      }
      const tabs = panel.querySelectorAll(".org-node-tab-btn");
      tabs.forEach((btn) => {
        btn.classList.toggle("is-active", btn.dataset.tab === tab);
      });
      const contents = panel.querySelectorAll("[data-tab-content]");
      contents.forEach((el) => {
        el.style.display = el.dataset.tabContent === tab ? "block" : "none";
      });
      panel.dataset.activeTab = tab;
    };

    const setDirty = (panel, dirty) => {
      if (!panel) {
        return;
      }
      panel.dataset.dirty = dirty ? "true" : "false";
      const badge = panel.querySelector(".org-node-unsaved");
      if (badge) {
        badge.style.display = dirty ? "block" : "none";
      }
    };

    const isDirty = () => {
      const panel = getDetailsPanel();
      return panel && panel.dataset.dirty === "true";
    };

    const confirmDiscard = () => {
      if (!isDirty()) {
        return true;
      }
      return window.confirm("未保存变更将丢失，确认继续？");
    };

    const initDetailsPanel = () => {
      const panel = getDetailsPanel();
      if (!panel || panel.dataset.ready === "true") {
        return;
      }
      panel.dataset.ready = "true";
      if (panel.classList.contains("org-node-details-panel")) {
        if (!panel.dataset.mode) {
          panel.dataset.mode = "readonly";
        }
        setTab(panel, panel.dataset.activeTab || "basic");
        if (panel.dataset.orgId) {
          lastSelectedOrgId = panel.dataset.orgId;
        }
      }
    };

	    const loadDetails = (orgId, opts) => {
	      const effectiveDate = opts && opts.effectiveDate ? opts.effectiveDate : "";
	      const treeAsOf = getTreeAsOf();
	      const includeDisabled = includeDisabledEnabled();
	      if (!orgId) {
	        return Promise.resolve();
	      }
	      let url = "/org/nodes/details?org_id=" + encodeURIComponent(orgId);
      if (effectiveDate) {
        url += "&effective_date=" + encodeURIComponent(effectiveDate);
      }
	      if (treeAsOf) {
	        url += "&tree_as_of=" + encodeURIComponent(treeAsOf);
	      }
	      if (includeDisabled) {
	        url += "&include_disabled=1";
	      }
	      if (opts && opts.flash) {
	        url += "&flash=" + encodeURIComponent(opts.flash);
	      }
      return htmx.ajax("GET", url, { target: "#org-node-details", swap: "innerHTML" })
        .then(() => {
          initDetailsPanel();
          return true;
        })
        .catch(() => {
          const panel = getDetailsPanel();
          if (panel) {
            setStatus(panel, "error", "加载失败");
          }
          return false;
        });
    };

	    const loadChildren = (item) => {
      if (!item || item.dataset.loading === "true" || item.lazy !== true) {
        return Promise.resolve();
      }
	      const orgId = item.dataset.orgId;
	      const treeAsOf = getTreeAsOf();
	      const includeDisabled = includeDisabledEnabled();
	      if (!orgId || !treeAsOf) {
	        return Promise.resolve();
	      }
	      item.dataset.loading = "true";
	      let url = "/org/nodes/children?parent_id=" + encodeURIComponent(orgId) + "&tree_as_of=" + encodeURIComponent(treeAsOf);
	      if (includeDisabled) {
	        url += "&include_disabled=1";
	      }
      return htmx.ajax("GET", url, { target: item, swap: "beforeend" })
        .then(() => {
          item.lazy = false;
        })
        .finally(() => {
          delete item.dataset.loading;
        });
    };

    const selectTreeItemByOrgId = (orgId) => {
      if (!orgId) {
        return;
      }
      const item = tree.querySelector("sl-tree-item[data-org-id=\"" + orgId + "\"]");
      if (item) {
        item.selected = true;
        item.scrollIntoView({ block: "center" });
      }
    };

	    const selectByOrgCode = async (orgCode) => {
	      const treeAsOf = getTreeAsOf();
	      const includeDisabled = includeDisabledEnabled();
	      if (!orgCode || !treeAsOf) {
	        return;
	      }
      if (!confirmDiscard()) {
        return;
      }
      setSearchError("");
	      let url = "/org/nodes/search?query=" + encodeURIComponent(orgCode) + "&tree_as_of=" + encodeURIComponent(treeAsOf);
	      if (includeDisabled) {
	        url += "&include_disabled=1";
	      }
      try {
        const resp = await fetch(url, { headers: { "Accept": "application/json" } });
        if (!resp.ok) {
          throw new Error("search failed");
        }
        const data = await resp.json();
        if (!data || !Array.isArray(data.path_org_ids)) {
          throw new Error("invalid response");
        }
        let current = null;
        for (const orgId of data.path_org_ids) {
          let item = tree.querySelector("sl-tree-item[data-org-id=\"" + orgId + "\"]");
          if (!item && current) {
            await loadChildren(current);
            item = tree.querySelector("sl-tree-item[data-org-id=\"" + orgId + "\"]");
          }
          if (!item) {
            break;
          }
          item.expanded = true;
          current = item;
        }
        if (!current) {
          setSearchError("未找到匹配组织");
          return;
        }
        current.selected = true;
        current.scrollIntoView({ block: "center" });
        if (current.dataset && current.dataset.orgId) {
          if (window.localStorage) {
            window.localStorage.setItem("org_nodes_last_org_code", data.target_org_code || orgCode);
          }
          await loadDetails(current.dataset.orgId);
          lastSelectedOrgId = current.dataset.orgId;
        }
      } catch (_) {
        setSearchError("查找失败");
      }
    };

    const showCreatePanel = () => {
      const container = getDetailsContainer();
      if (!container) {
        return;
      }
      if (!confirmDiscard()) {
        return;
      }
      if (!canEdit) {
        const panel = getDetailsPanel();
        if (panel && panel.classList.contains("org-node-details-panel")) {
          setStatus(panel, "warn", "无更新权限，无法编辑");
        } else {
          container.innerHTML = "<div class=\"org-node-status-row org-node-status-warn\">无更新权限，无法编辑</div>";
        }
        return;
      }
      const tpl = document.getElementById("org-node-create-template");
      if (!tpl) {
        return;
      }
      container.innerHTML = tpl.innerHTML;
      initDetailsPanel();
    };

    const recordActionConfig = {
      add_record: { title: "新增记录", hint: "新增记录将追加为最新版本", showFields: true, submit: "保存" },
      insert_record: { title: "插入记录", hint: "生效日需位于相邻记录之间（可早于所选记录）", showFields: true, submit: "保存" },
      delete_record: { title: "删除记录（错误数据）", hint: "该操作将删除错误数据（通过事件撤销实现），并立即重放版本；操作可审计，不可撤销。", showFields: false, submit: "删除记录" },
      delete_org: { title: "删除组织（错误建档）", hint: "该操作将删除错误建档组织（通过事件撤销实现），并立即重放版本；操作可审计，不可撤销。", showFields: false, submit: "删除组织" },
    };

    const parseDate = (value) => {
      if (!value) {
        return null;
      }
      const parts = String(value).split("-");
      if (parts.length !== 3) {
        return null;
      }
      const year = Number(parts[0]);
      const month = Number(parts[1]);
      const day = Number(parts[2]);
      if (!year || !month || !day) {
        return null;
      }
      const date = new Date(Date.UTC(year, month - 1, day));
      if (Number.isNaN(date.getTime())) {
        return null;
      }
      return date;
    };

    const formatDate = (date) => {
      const year = date.getUTCFullYear();
      const month = String(date.getUTCMonth() + 1).padStart(2, "0");
      const day = String(date.getUTCDate()).padStart(2, "0");
      return year + "-" + month + "-" + day;
    };

    const addDays = (value, days) => {
      const date = parseDate(value);
      if (!date) {
        return "";
      }
      date.setUTCDate(date.getUTCDate() + days);
      return formatDate(date);
    };

    const updateRecordChangeFields = (form) => {
      if (!form) {
        return;
      }
      const changeSelect = form.querySelector("select[name=\"record_change_type\"]");
      const changeType = changeSelect ? String(changeSelect.value || "").trim() : "rename";
      const nameRow = form.querySelector(".org-node-record-name");
      const parentRow = form.querySelector(".org-node-record-parent");
      const buRow = form.querySelector(".org-node-record-business-unit");
      if (nameRow) {
        nameRow.style.display = changeType === "rename" ? "grid" : "none";
      }
      if (parentRow) {
        parentRow.style.display = changeType === "move" ? "grid" : "none";
      }
      if (buRow) {
        buRow.style.display = changeType === "set_business_unit" ? "flex" : "none";
      }
    };

    const recordActionLabelMap = {
      add_record: "新增记录",
      insert_record: "插入记录",
      delete_record: "删除记录（错误数据）",
      delete_org: "删除组织（错误建档）",
    };

    const recordChangeLabelMap = {
      rename: "组织名称",
      move: "上级组织",
      set_business_unit: "业务单元",
    };

    const setRecordHint = (form, text, isError) => {
      if (!form) {
        return;
      }
      const hint = form.querySelector(".org-node-record-form-hint");
      if (!hint) {
        return;
      }
      hint.textContent = text || "";
      if (isError) {
        hint.classList.add("is-error");
      } else {
        hint.classList.remove("is-error");
      }
    };

    const getRecordCurrentStep = (form) => {
      const n = Number(form && form.dataset ? form.dataset.currentStep : "1");
      if (!Number.isFinite(n) || n < 1) {
        return 1;
      }
      if (n > 4) {
        return 4;
      }
      return n;
    };

    const setRecordCurrentStep = (form, step) => {
      if (!form || !form.dataset) {
        return;
      }
      const next = Math.min(4, Math.max(1, Number(step) || 1));
      form.dataset.currentStep = String(next);
    };

    const updateRecordSummary = (form) => {
      if (!form) {
        return;
      }
      const summary = form.querySelector(".org-node-record-summary");
      if (!summary) {
        return;
      }
      const actionInput = form.querySelector("input[name=\"action\"]");
      const action = actionInput ? String(actionInput.value || "") : "";
      const actionLabel = recordActionLabelMap[action] || "记录操作";
      const effectiveInput = form.querySelector("input[name=\"effective_date\"]");
      const effectiveDate = effectiveInput ? String(effectiveInput.value || "").trim() : "";
      if (action === "delete_record" || action === "delete_org") {
        summary.textContent = "将执行：" + actionLabel + "。该操作会通过事件撤销删除错误数据并立即重放，操作可审计且不可撤销。";
        return;
      }
      const changeSelect = form.querySelector("select[name=\"record_change_type\"]");
      const changeType = changeSelect ? String(changeSelect.value || "").trim() : "rename";
      const changeLabel = recordChangeLabelMap[changeType] || changeType;
      let detail = "";
      if (changeType === "rename") {
        const nameInput = form.querySelector("input[name=\"name\"]");
        detail = "组织名称：" + (nameInput ? String(nameInput.value || "").trim() : "");
      } else if (changeType === "move") {
        const parentInput = form.querySelector("input[name=\"parent_org_code\"]");
        const parentCode = parentInput ? String(parentInput.value || "").trim() : "";
        detail = "上级组织：" + (parentCode || "(空=设为根组织)");
      } else if (changeType === "set_business_unit") {
        const buInput = form.querySelector("input[name=\"is_business_unit\"]");
        detail = "业务单元：" + ((buInput && buInput.checked) ? "是" : "否");
      }
      summary.textContent = "将执行：" + actionLabel + "，生效日期：" + (effectiveDate || "(未填写)") + "，变更类型：" + changeLabel + (detail ? "，" + detail : "") + "。";
    };

    const updateRecordWizardStep = (form) => {
      if (!form) {
        return;
      }
      const step = getRecordCurrentStep(form);
      const actionInput = form.querySelector("input[name=\"action\"]");
      const action = actionInput ? String(actionInput.value || "") : "add_record";
      const config = recordActionConfig[action] || recordActionConfig.add_record;
      const steps = form.querySelectorAll(".org-node-record-step");
      steps.forEach((node) => {
        const nodeStep = Number(node.dataset.step || "0");
        node.dataset.active = nodeStep === step ? "true" : "false";
      });
      const items = form.querySelectorAll(".org-node-record-stepper-item");
      items.forEach((item) => {
        const nodeStep = Number(item.dataset.step || "0");
        item.classList.toggle("is-active", nodeStep === step);
        item.classList.toggle("is-done", nodeStep > 0 && nodeStep < step);
      });
      const prevBtn = form.querySelector(".org-node-record-prev");
      if (prevBtn) {
        prevBtn.style.display = step > 1 ? "inline-flex" : "none";
      }
      const nextBtn = form.querySelector(".org-node-record-next");
      if (nextBtn) {
        nextBtn.style.display = step < 4 ? "inline-flex" : "none";
      }
      const submitBtn = form.querySelector(".org-node-record-submit");
      if (submitBtn) {
        submitBtn.style.display = step === 4 ? "inline-flex" : "none";
        submitBtn.textContent = config.submit;
      }
      const intent = form.querySelector(".org-node-record-intent");
      if (intent) {
        intent.textContent = "当前操作：" + (recordActionLabelMap[action] || "记录操作") + "。" + config.hint;
      }
      const changeRow = form.querySelector(".org-node-record-change");
      const nameLabel = form.querySelector(".org-node-record-name");
      const parentLabel = form.querySelector(".org-node-record-parent");
      const buLabel = form.querySelector(".org-node-record-business-unit");
      if (changeRow) {
        changeRow.style.display = config.showFields ? "grid" : "none";
      }
      if (config.showFields) {
        updateRecordChangeFields(form);
      } else {
        if (nameLabel) {
          nameLabel.style.display = "none";
        }
        if (parentLabel) {
          parentLabel.style.display = "none";
        }
        if (buLabel) {
          buLabel.style.display = "none";
        }
      }
      updateRecordSummary(form);
      let stepHint = "";
      if (step === 1) {
        stepHint = config.hint;
      } else if (step === 2) {
        stepHint = form.dataset.rangeHint || config.hint;
      } else if (step === 3) {
        stepHint = config.showFields ? "按变更类型填写字段后继续。" : "删除操作无需填写字段，继续确认即可。";
      } else {
        stepHint = "请确认摘要后提交。";
      }
      setRecordHint(form, stepHint, false);
    };

    const validateRecordStep = (form, panel, step) => {
      if (!form) {
        return "";
      }
      const actionInput = form.querySelector("input[name=\"action\"]");
      const action = actionInput ? String(actionInput.value || "") : "add_record";
      const config = recordActionConfig[action] || recordActionConfig.add_record;
      if (step === 2) {
        const effectiveInput = form.querySelector("input[name=\"effective_date\"]");
        const effectiveDate = effectiveInput ? String(effectiveInput.value || "").trim() : "";
        if (!effectiveDate) {
          return "effective_date is required";
        }
        if (!parseDate(effectiveDate)) {
          return "effective_date 无效";
        }
        if (!panel || action === "delete_record" || action === "delete_org") {
          return "";
        }
        const currentEffectiveDate = panel.dataset.currentEffectiveDate || "";
        const prevEffectiveDate = panel.dataset.prevEffectiveDate || "";
        const nextEffectiveDate = panel.dataset.nextEffectiveDate || "";
        const maxEffectiveDate = panel.dataset.maxEffectiveDate || "";
        if (action === "add_record") {
          if (maxEffectiveDate && effectiveDate <= maxEffectiveDate) {
            return "effective_date conflict";
          }
          return "";
        }
        if (action === "insert_record") {
          if (!nextEffectiveDate) {
            if (maxEffectiveDate && effectiveDate <= maxEffectiveDate) {
              return "effective_date conflict";
            }
            return "";
          }
          if (effectiveDate === currentEffectiveDate) {
            return "effective_date conflict";
          }
          if (prevEffectiveDate && effectiveDate <= prevEffectiveDate) {
            return "effective_date must be between existing records";
          }
          if (effectiveDate >= nextEffectiveDate) {
            return "effective_date must be between existing records";
          }
        }
      }
      if (step === 3 && config.showFields) {
        const changeSelect = form.querySelector("select[name=\"record_change_type\"]");
        const changeType = changeSelect ? String(changeSelect.value || "").trim() : "rename";
        if (changeType === "rename") {
          const nameInput = form.querySelector("input[name=\"name\"]");
          if (!nameInput || !String(nameInput.value || "").trim()) {
            return "name is required";
          }
        }
      }
      return "";
    };

    tree.addEventListener("sl-lazy-load", (event) => {
      const item = (event.detail && event.detail.item) || event.target;
      loadChildren(item).catch(() => {});
    });

    tree.addEventListener("sl-selection-change", () => {
      const item = tree.selectedItems && tree.selectedItems.length > 0 ? tree.selectedItems[0] : null;
      const orgId = item && item.dataset ? item.dataset.orgId : "";
      if (!orgId) {
        return;
      }
      if (!confirmDiscard()) {
        selectTreeItemByOrgId(lastSelectedOrgId);
        return;
      }
      if (window.localStorage && item.dataset && item.dataset.orgCode) {
        window.localStorage.setItem("org_nodes_last_org_code", item.dataset.orgCode);
      }
      loadDetails(orgId).then(() => {
        lastSelectedOrgId = orgId;
      });
    });

    document.addEventListener("submit", (event) => {
      const form = event.target;
      if (form && form.classList && form.classList.contains("org-node-search-form")) {
        event.preventDefault();
        const formData = new FormData(form);
        const query = String(formData.get("query") || "").trim();
        if (!query) {
          setSearchError("请输入查找条件");
          return;
        }
	        const treeAsOf = getTreeAsOf();
	        const includeDisabled = includeDisabledEnabled();
	        if (!treeAsOf) {
	          setSearchError("缺少 tree_as_of");
	          return;
	        }
	        setSearchError("");
	        let url = "/org/nodes/search?query=" + encodeURIComponent(query) + "&tree_as_of=" + encodeURIComponent(treeAsOf) + "&format=panel";
	        if (includeDisabled) {
	          url += "&include_disabled=1";
	        }
        fetch(url, { headers: { "Accept": "text/html" } })
          .then((resp) => {
            if (!resp.ok) {
              throw new Error("search failed");
            }
            return resp.text();
          })
          .then((html) => {
            const panel = setSearchResults(html);
            if (!panel) {
              return;
            }
            const count = Number(panel.dataset.count || "0");
            if (count === 1) {
              const item = panel.querySelector(".org-node-search-item");
              if (item && item.dataset && item.dataset.orgCode) {
                selectByOrgCode(item.dataset.orgCode);
              }
            }
          })
          .catch(() => {
            setSearchError("查找失败");
          });
        return;
      }

      if (form && form.classList && form.classList.contains("org-node-edit-form")) {
        event.preventDefault();
        const panel = form.closest(".org-node-details-panel");
        if (!panel) {
          return;
        }
        clearStatus(panel);
        if (!canEdit) {
          setStatus(panel, "warn", "无更新权限，无法编辑");
          return;
        }

        const original = {
          effectiveDate: form.dataset.originalEffectiveDate || "",
          name: form.dataset.originalName || "",
          parentCode: form.dataset.originalParentCode || "",
          managerPernr: form.dataset.originalManagerPernr || "",
          managerName: form.dataset.originalManagerName || "",
        };

        const effectiveInput = form.querySelector("input[name=\"effective_date\"]");
        const parentInput = form.querySelector("input[name=\"parent_org_code\"]");
        const nameInput = form.querySelector("input[name=\"name\"]");
        const managerInput = form.querySelector("input[name=\"manager_pernr\"]");

        const patch = {};
        const newEffectiveDate = effectiveInput ? String(effectiveInput.value || "").trim() : "";
        if (newEffectiveDate && newEffectiveDate !== original.effectiveDate) {
          patch.effective_date = newEffectiveDate;
        }

        const parentCode = parentInput ? String(parentInput.value || "").trim() : "";
        if (parentCode !== original.parentCode) {
          patch.parent_org_code = parentCode;
        }

        const newName = nameInput ? String(nameInput.value || "").trim() : "";
        if (newName !== original.name) {
          if (!newName) {
            setStatus(panel, "error", "组织名称为必填");
            return;
          }
          patch.name = newName;
        }

        const managerPernr = managerInput ? String(managerInput.value || "").trim() : "";
        if (managerPernr !== original.managerPernr) {
          if (!managerPernr) {
            setStatus(panel, "error", "负责人编号为必填");
            return;
          }
          patch.manager_pernr = managerPernr;
        }

        if (Object.keys(patch).length === 0) {
          setStatus(panel, "error", "未检测到变更");
          return;
        }

        if (!form.dataset.requestId) {
          if (window.crypto && typeof window.crypto.randomUUID === "function") {
            form.dataset.requestId = window.crypto.randomUUID();
          } else {
            form.dataset.requestId = "corr-" + Date.now() + "-" + Math.random().toString(16).slice(2);
          }
        }

        const payload = {
          org_code: form.dataset.orgCode || "",
          effective_date: original.effectiveDate,
          patch: patch,
          request_id: form.dataset.requestId,
        };

        fetch("/org/api/org-units/corrections", {
          method: "POST",
          headers: { "Content-Type": "application/json", "Accept": "application/json" },
          body: JSON.stringify(payload),
        })
          .then(async (resp) => {
            let data = null;
            try {
              data = await resp.json();
            } catch (_) {
              data = null;
            }
            if (!resp.ok) {
              const code = data && data.code ? data.code : "";
              const message = data && data.message ? data.message : "";
              const mapping = {
                ORG_CODE_INVALID: "组织编码无效",
                ORG_CODE_NOT_FOUND: "组织编码不存在",
                EFFECTIVE_DATE_INVALID: "生效日期无效",
                EFFECTIVE_DATE_OUT_OF_RANGE: "生效日期超出范围",
                ORG_EVENT_NOT_FOUND: "未找到该生效日记录",
                PARENT_NOT_FOUND_AS_OF: "所选日期上级组织未生效或已失效，请调整日期",
                ORG_PARENT_NOT_FOUND_AS_OF: "所选日期上级组织未生效或已失效，请调整日期",
                MANAGER_PERNR_INVALID: "负责人编号无效",
                MANAGER_PERNR_NOT_FOUND: "负责人不存在",
                MANAGER_PERNR_INACTIVE: "负责人已失效",
                PATCH_FIELD_NOT_ALLOWED: "字段不允许更正",
                PATCH_REQUIRED: "未检测到变更",
                EVENT_DATE_CONFLICT: "生效日期冲突",
                REQUEST_DUPLICATE: "重复请求",
                ORG_ENABLE_REQUIRED: "需要先启用组织",
                ORG_REQUEST_ID_CONFLICT: "请求编号冲突，请刷新后重试",
                ORG_REPLAY_FAILED: "重放失败，操作已回滚",
                ORG_ROOT_DELETE_FORBIDDEN: "根组织不允许删除",
                ORG_HAS_CHILDREN_CANNOT_DELETE: "存在下级组织，不能删除",
                ORG_HAS_DEPENDENCIES_CANNOT_DELETE: "存在下游依赖，不能删除",
                ORG_EVENT_RESCINDED: "该记录已删除",
              };
              const msg = (code && mapping[code]) ? mapping[code] : (message || "保存失败，请重试");
              setStatus(panel, "error", msg);
              if (resp.status === 403) {
                setStatus(panel, "warn", "无更新权限，无法编辑");
              }
              return;
            }
            setDirty(panel, false);
            panel.dataset.mode = "readonly";
            const nextEffectiveDate = (data && data.effective_date) ? data.effective_date : newEffectiveDate || original.effectiveDate;
            if (panel.dataset.orgId) {
              loadDetails(panel.dataset.orgId, { effectiveDate: nextEffectiveDate, flash: "success" });
	            } else {
	              const treeAsOf = getTreeAsOf();
	              const includeDisabled = includeDisabledEnabled();
	              const includeDisabledPart = includeDisabled ? "&include_disabled=1" : "";
	              if (treeAsOf) {
	                window.location.href = "/org/nodes?tree_as_of=" + encodeURIComponent(treeAsOf) + includeDisabledPart;
	              } else if (nextEffectiveDate) {
	                window.location.href = "/org/nodes?tree_as_of=" + encodeURIComponent(nextEffectiveDate) + includeDisabledPart;
	              }
	            }
          })
          .catch(() => {
            setStatus(panel, "error", "请求失败");
          });
        return;
      }

      if (form && form.classList && form.classList.contains("org-node-record-action-form")) {
        const panel = form.closest(".org-node-details-panel");
        const currentStep = getRecordCurrentStep(form);
        if (currentStep < 4) {
          event.preventDefault();
          const err = validateRecordStep(form, panel, currentStep);
          if (err) {
            setRecordHint(form, err, true);
            return;
          }
          setRecordCurrentStep(form, currentStep + 1);
          updateRecordWizardStep(form);
          return;
        }
        const step2Err = validateRecordStep(form, panel, 2);
        if (step2Err) {
          event.preventDefault();
          setRecordCurrentStep(form, 2);
          updateRecordWizardStep(form);
          setRecordHint(form, step2Err, true);
          return;
        }
        const step3Err = validateRecordStep(form, panel, 3);
        if (step3Err) {
          event.preventDefault();
          setRecordCurrentStep(form, 3);
          updateRecordWizardStep(form);
          setRecordHint(form, step3Err, true);
          return;
        }
        const actionInput = form.querySelector("input[name=\"action\"]");
        if (actionInput && (actionInput.value === "delete_record" || actionInput.value === "delete_org")) {
          const confirmText = actionInput.value === "delete_org"
            ? "确认删除该组织（错误建档）？该操作会撤销该组织全部事件并立即重放，且不可撤销。"
            : "确认删除该生效记录（错误数据）？该操作会撤销事件并立即重放，且不可撤销。";
          if (!window.confirm(confirmText)) {
            event.preventDefault();
          }
        }
      }
    });

    document.addEventListener("click", (event) => {
      const searchItem = event.target && event.target.closest ? event.target.closest(".org-node-search-item") : null;
      if (searchItem && searchItem.dataset && searchItem.dataset.orgCode) {
        selectByOrgCode(searchItem.dataset.orgCode);
        return;
      }

      const createBtn = event.target && event.target.closest ? event.target.closest(".org-node-create-btn") : null;
      if (createBtn) {
        showCreatePanel();
        return;
      }

      const editBtn = event.target && event.target.closest ? event.target.closest(".org-node-edit-btn") : null;
      if (editBtn) {
        const panel = editBtn.closest(".org-node-details-panel");
        if (!panel) {
          return;
        }
        clearStatus(panel);
        if (!canEdit) {
          setStatus(panel, "warn", "无更新权限，无法编辑");
          return;
        }
        panel.dataset.mode = "edit";
        return;
      }

      const saveBtn = event.target && event.target.closest ? event.target.closest(".org-node-save-btn") : null;
      if (saveBtn) {
        const panel = saveBtn.closest(".org-node-details-panel");
        if (!panel) {
          return;
        }
        const form = panel.querySelector(".org-node-edit-form");
        if (form) {
          if (typeof form.requestSubmit === "function") {
            form.requestSubmit();
          } else {
            form.submit();
          }
        }
        return;
      }

      const cancelBtn = event.target && event.target.closest ? event.target.closest(".org-node-cancel-btn") : null;
      if (cancelBtn) {
        const panel = cancelBtn.closest(".org-node-details-panel");
        if (!panel) {
          return;
        }
        if (!confirmDiscard()) {
          return;
        }
        const form = panel.querySelector(".org-node-edit-form");
        if (form) {
          const orig = {
            effectiveDate: form.dataset.originalEffectiveDate || "",
            name: form.dataset.originalName || "",
            parentCode: form.dataset.originalParentCode || "",
            managerPernr: form.dataset.originalManagerPernr || "",
            managerName: form.dataset.originalManagerName || "",
          };
          const effectiveInput = form.querySelector("input[name=\"effective_date\"]");
          const nameInput = form.querySelector("input[name=\"name\"]");
          const parentInput = form.querySelector("input[name=\"parent_org_code\"]");
          const managerInput = form.querySelector("input[name=\"manager_pernr\"]");
          const managerNameInput = form.querySelector("input[name=\"manager_name\"]");
          if (effectiveInput) {
            effectiveInput.value = orig.effectiveDate;
          }
          if (nameInput) {
            nameInput.value = orig.name;
          }
          if (parentInput) {
            parentInput.value = orig.parentCode;
          }
          if (managerInput) {
            managerInput.value = orig.managerPernr;
          }
          if (managerNameInput) {
            managerNameInput.value = orig.managerName;
          }
        }
        panel.dataset.mode = "readonly";
        setDirty(panel, false);
        clearStatus(panel);
        return;
      }

      const tabBtn = event.target && event.target.closest ? event.target.closest(".org-node-tab-btn") : null;
      if (tabBtn) {
        const panel = tabBtn.closest(".org-node-details-panel");
        if (!panel) {
          return;
        }
        setTab(panel, tabBtn.dataset.tab || "basic");
        return;
      }

      const recordPrev = event.target && event.target.closest ? event.target.closest(".org-node-record-prev") : null;
      if (recordPrev) {
        const form = recordPrev.closest ? recordPrev.closest(".org-node-record-action-form") : null;
        if (!form) {
          return;
        }
        setRecordCurrentStep(form, getRecordCurrentStep(form) - 1);
        updateRecordWizardStep(form);
        return;
      }

      const recordNext = event.target && event.target.closest ? event.target.closest(".org-node-record-next") : null;
      if (recordNext) {
        const form = recordNext.closest ? recordNext.closest(".org-node-record-action-form") : null;
        if (!form) {
          return;
        }
        const panel = form.closest(".org-node-details-panel");
        const step = getRecordCurrentStep(form);
        const err = validateRecordStep(form, panel, step);
        if (err) {
          setRecordHint(form, err, true);
          return;
        }
        setRecordCurrentStep(form, step + 1);
        updateRecordWizardStep(form);
        return;
      }

      const recordBtn = event.target && event.target.closest ? event.target.closest(".org-node-record-btn") : null;
      if (recordBtn) {
        const panel = recordBtn.closest(".org-node-details-panel");
        if (!panel) {
          return;
        }
        clearStatus(panel);
        if (!canEdit) {
          setStatus(panel, "warn", "无更新权限，无法编辑");
          return;
        }
        if (!confirmDiscard()) {
          return;
        }
        const action = recordBtn.dataset.action || "";
        if (action === "correct_record") {
          panel.dataset.mode = "edit";
          return;
        }
        const config = recordActionConfig[action];
        if (!config) {
          return;
        }
        const formWrap = panel.querySelector(".org-node-record-form");
        if (!formWrap) {
          return;
        }
        formWrap.dataset.open = "true";
        const title = formWrap.querySelector(".org-node-record-form-title");
        if (title) {
          title.textContent = config.title;
        }
        const form = formWrap.querySelector("form");
        if (form) {
          const actionInput = form.querySelector("input[name=\"action\"]");
          if (actionInput) {
            actionInput.value = action;
          }
          const changeSelect = form.querySelector("select[name=\"record_change_type\"]");
          if (changeSelect) {
            changeSelect.value = "rename";
          }
          const currentEffectiveDate = panel.dataset.currentEffectiveDate || "";
          const prevEffectiveDate = panel.dataset.prevEffectiveDate || "";
          const nextEffectiveDate = panel.dataset.nextEffectiveDate || "";
          const maxEffectiveDate = panel.dataset.maxEffectiveDate || "";
          let defaultDate = currentEffectiveDate;
          let hintText = config.hint;
          if (action === "add_record") {
            if (maxEffectiveDate) {
              defaultDate = addDays(maxEffectiveDate, 1);
              hintText = "新增记录日期需晚于 " + maxEffectiveDate;
            }
          }
          if (action === "insert_record") {
            if (!nextEffectiveDate) {
              const base = maxEffectiveDate || currentEffectiveDate;
              if (base) {
                defaultDate = addDays(base, 1);
              }
              if (maxEffectiveDate) {
                hintText = "当前为最晚记录，插入视同新增；日期需晚于 " + maxEffectiveDate;
              } else {
                hintText = "当前为最晚记录，插入视同新增";
              }
            } else {
              if (currentEffectiveDate) {
                defaultDate = addDays(currentEffectiveDate, 1);
              }
              if (prevEffectiveDate) {
                hintText = "插入日期需介于 " + prevEffectiveDate + " ~ " + nextEffectiveDate + " 之间（可早于所选记录）";
              } else {
                hintText = "插入日期需早于 " + nextEffectiveDate + "（可早于所选记录）";
              }
            }
          }
          const effectiveInput = form.querySelector("input[name=\"effective_date\"]");
          if (effectiveInput) {
            effectiveInput.value = defaultDate;
          }
          const nameInput = form.querySelector("input[name=\"name\"]");
          const parentInput = form.querySelector("input[name=\"parent_org_code\"]");
          const buInput = form.querySelector("input[name=\"is_business_unit\"]");
          const currentName = panel.dataset.currentName || "";
          const currentParent = panel.dataset.currentParentCode || "";
          const currentIsBusinessUnit = panel.dataset.currentIsBusinessUnit === "true";
          if (nameInput) {
            nameInput.value = currentName;
          }
          if (parentInput) {
            parentInput.value = currentParent;
          }
          if (buInput) {
            buInput.checked = currentIsBusinessUnit;
          }
          form.dataset.rangeHint = hintText;
          setRecordCurrentStep(form, 1);
          updateRecordWizardStep(form);
        }
        formWrap.scrollIntoView({ block: "nearest" });
        return;
      }

      const recordCancel = event.target && event.target.closest ? event.target.closest(".org-node-record-cancel") : null;
      if (recordCancel) {
        const formWrap = recordCancel.closest(".org-node-record-form");
        if (formWrap) {
          formWrap.dataset.open = "false";
        }
        return;
      }

      const createCancel = event.target && event.target.closest ? event.target.closest(".org-node-create-cancel") : null;
      if (createCancel) {
        const container = getDetailsContainer();
        if (!container) {
          return;
        }
        if (lastSelectedOrgId) {
          loadDetails(lastSelectedOrgId);
        } else {
          container.innerHTML = "<div class=\"org-node-placeholder\">请选择左侧组织，或在上方查找。</div>";
        }
      }
    });

    document.addEventListener("input", (event) => {
      const input = event.target;
      if (!input) {
        return;
      }
      const recordForm = input.closest ? input.closest(".org-node-record-action-form") : null;
      if (recordForm) {
        if (input.name === "record_change_type") {
          updateRecordChangeFields(recordForm);
        }
        updateRecordSummary(recordForm);
        return;
      }
      const form = input.closest ? input.closest(".org-node-edit-form") : null;
      if (!form) {
        return;
      }
      delete form.dataset.requestId;
      const panel = form.closest(".org-node-details-panel");
      if (!panel) {
        return;
      }
      const orig = {
        effectiveDate: form.dataset.originalEffectiveDate || "",
        name: form.dataset.originalName || "",
        parentCode: form.dataset.originalParentCode || "",
        managerPernr: form.dataset.originalManagerPernr || "",
      };
      const effectiveInput = form.querySelector("input[name=\"effective_date\"]");
      const nameInput = form.querySelector("input[name=\"name\"]");
      const parentInput = form.querySelector("input[name=\"parent_org_code\"]");
      const managerInput = form.querySelector("input[name=\"manager_pernr\"]");
      const dirty = (
        (effectiveInput && String(effectiveInput.value || "").trim() !== orig.effectiveDate) ||
        (nameInput && String(nameInput.value || "").trim() !== orig.name) ||
        (parentInput && String(parentInput.value || "").trim() !== orig.parentCode) ||
        (managerInput && String(managerInput.value || "").trim() !== orig.managerPernr)
      );
      setDirty(panel, dirty);
    });

    document.addEventListener("focusout", (event) => {
      const input = event.target;
      if (!input || input.name !== "manager_pernr") {
        return;
      }
      const form = input.closest ? input.closest(".org-node-edit-form") : null;
      if (!form) {
        return;
      }
      const managerNameInput = form.querySelector("input[name=\"manager_name\"]");
      if (!managerNameInput) {
        return;
      }
      const trimmed = String(input.value || "").trim();
      if (!trimmed) {
        managerNameInput.value = "";
        return;
      }
      fetch("/person/api/persons:by-pernr?pernr=" + encodeURIComponent(trimmed), { headers: { "Accept": "application/json" } })
        .then((resp) => {
          if (!resp.ok) {
            managerNameInput.value = "";
            return null;
          }
          return resp.json();
        })
        .then((data) => {
          managerNameInput.value = data && data.display_name ? data.display_name : "";
        })
        .catch(() => {
          managerNameInput.value = "";
        });
    });

    document.addEventListener("change", (event) => {
      const select = event.target;
      if (!select) {
        return;
      }
      const recordForm = select.closest ? select.closest(".org-node-record-action-form") : null;
      if (recordForm) {
        if (select.name === "record_change_type") {
          updateRecordChangeFields(recordForm);
        }
        updateRecordSummary(recordForm);
        return;
      }
      if (!select.classList || !select.classList.contains("org-node-version-select")) {
        return;
      }
      const panel = select.closest(".org-node-details-panel");
      if (!panel) {
        return;
      }
      const targetDate = select.value;
      if (!targetDate) {
        return;
      }
      if (!confirmDiscard()) {
        if (panel.dataset.currentEffectiveDate) {
          select.value = panel.dataset.currentEffectiveDate;
        }
        return;
      }
      if (!panel.dataset.orgId) {
        return;
      }
      loadDetails(panel.dataset.orgId, { effectiveDate: targetDate }).then((ok) => {
        if (!ok && panel.dataset.currentEffectiveDate) {
          select.value = panel.dataset.currentEffectiveDate;
        }
      });
    });

    document.addEventListener("click", (event) => {
      const versionBtn = event.target && event.target.closest ? event.target.closest(".org-node-version-btn") : null;
      if (!versionBtn) {
        return;
      }
      const targetDate = versionBtn.dataset.targetDate || "";
      if (!targetDate) {
        return;
      }
      if (!confirmDiscard()) {
        return;
      }
      const panel = versionBtn.closest(".org-node-details-panel");
      if (!panel || !panel.dataset.orgId) {
        return;
      }
      loadDetails(panel.dataset.orgId, { effectiveDate: targetDate }).catch(() => {});
    });

    window.orgNodes = {
      getTreeAsOf: getTreeAsOf,
      loadChildren: loadChildren,
      loadDetails: loadDetails,
      selectByOrgCode: selectByOrgCode,
      showCreatePanel: showCreatePanel,
    };

    if (window.localStorage) {
      const lastOrgCode = window.localStorage.getItem("org_nodes_last_org_code");
      if (lastOrgCode) {
        selectByOrgCode(lastOrgCode);
      }
    }

    initDetailsPanel();
    tree.dataset.orgNodesReady = "true";
    return true;
  };

  if (init()) {
    return;
  }
  const timer = setInterval(() => {
    if (init()) {
      clearInterval(timer);
    }
  }, 50);
})();
</script>`)

	b.WriteString(`</div>`)
	return b.String()
}

func renderOrgNodeCreateTemplate(treeAsOf string, includeDisabled bool, canEdit bool) string {
	disabledAttr := ""
	if !canEdit {
		disabledAttr = " disabled"
	}
	includeDisabledValue := "0"
	if includeDisabled {
		includeDisabledValue = "1"
	}
	var b strings.Builder
	b.WriteString(`<template id="org-node-create-template">`)
	b.WriteString(`<div class="org-node-create-panel" data-mode="create">`)
	b.WriteString(`<div class="org-node-create-header">新建部门</div>`)
	if !canEdit {
		b.WriteString(`<div class="org-node-status-row org-node-status-warn">无更新权限，无法编辑</div>`)
	}
	b.WriteString(`<form method="POST" action="/org/nodes?tree_as_of=` + html.EscapeString(treeAsOf) + `&include_disabled=` + html.EscapeString(includeDisabledValue) + `">`)
	b.WriteString(`<input type="hidden" name="tree_as_of" value="` + html.EscapeString(treeAsOf) + `" />`)
	b.WriteString(`<input type="hidden" name="include_disabled" value="` + html.EscapeString(includeDisabledValue) + `" />`)
	b.WriteString(`<label>生效日期 <input type="date" name="effective_date" value="` + html.EscapeString(treeAsOf) + `"` + disabledAttr + ` /></label>`)
	b.WriteString(`<label>组织编码* <input name="org_code"` + disabledAttr + ` /></label>`)
	b.WriteString(`<label>组织名称* <input name="name"` + disabledAttr + ` /></label>`)
	b.WriteString(`<label>上级组织编码 <input name="parent_code"` + disabledAttr + ` /></label>`)
	b.WriteString(`<label class="org-node-create-checkbox"><input type="checkbox" name="is_business_unit" value="true"` + disabledAttr + ` /> 业务单元</label>`)
	b.WriteString(`<div class="org-node-create-actions">`)
	b.WriteString(`<button type="submit"` + disabledAttr + `>保存</button>`)
	b.WriteString(`<button type="button" class="org-node-create-cancel">取消</button>`)
	b.WriteString(`</div>`)
	b.WriteString(`</form>`)
	b.WriteString(`</div>`)
	b.WriteString(`</template>`)
	return b.String()
}
