package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	OrgID          int
	OrgCode        string
	Name           string
	Status         string
	IsBusinessUnit bool
	HasChildren    bool
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

type OrgUnitNodeAuditEvent struct {
	EventID                int64
	EventUUID              string
	OrgID                  int
	EventType              string
	EffectiveDate          string
	TxTime                 time.Time
	InitiatorName          string
	InitiatorEmployeeID    string
	RequestCode            string
	Reason                 string
	Payload                json.RawMessage
	BeforeSnapshot         json.RawMessage
	AfterSnapshot          json.RawMessage
	RescindOutcome         string
	IsRescinded            bool
	RescindedByEventUUID   string
	RescindedByTxTime      time.Time
	RescindedByRequestCode string
}

type OrgUnitNodeEffectiveDateCorrector interface {
	CorrectNodeEffectiveDate(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, newEffectiveDate string, requestID string) error
}

var errOrgUnitNotFound = errors.New("org_unit_not_found")

const (
	orgNodeDeleteRecordReason = "UI 删除记录（错误数据）"
	orgNodeDeleteOrgReason    = "UI 删除组织（错误建档）"
	orgNodeAuditPageSize      = 20
)

func orgNodeWriteErrorMessage(err error) string {
	code := strings.TrimSpace(err.Error())
	messages := map[string]string{
		"ORG_CODE_INVALID":    "组织编码无效",
		"ORG_CODE_NOT_FOUND":  "组织编码不存在",
		"ORG_EVENT_NOT_FOUND": "未找到该生效日记录",
		// One-day-slot-per-effective-date: writes that target an already-occupied effective_date are rejected.
		// UI should guide users to either pick another effective_date (add/insert) or use correction to change the existing record.
		"EVENT_DATE_CONFLICT":                      "生效日期冲突：该生效日已存在记录。请修改“生效日期”（新增/插入记录）或使用“修正”修改该生效日记录后重试。",
		"ORG_REQUEST_ID_CONFLICT":                  "请求编号冲突，请刷新后重试",
		"ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET": "该记录不支持同日状态纠错",
		"ORG_ROOT_DELETE_FORBIDDEN":                "根组织不允许删除",
		"ORG_HAS_CHILDREN_CANNOT_DELETE":           "存在下级组织，不能删除",
		"ORG_HAS_DEPENDENCIES_CANNOT_DELETE":       "存在下游依赖，不能删除",
		"ORG_EVENT_RESCINDED":                      "该记录已删除",
		"ORG_HIGH_RISK_REORDER_FORBIDDEN":          "该变更会触发高风险全量重放，请改用新增/插入记录",
		"ORGUNIT_CODES_WRITE_FORBIDDEN":            "系统写入权限异常（ORGUNIT_CODES_WRITE_FORBIDDEN），请联系管理员",
		"EFFECTIVE_DATE_INVALID":                   "生效日期无效",
		"ORG_INVALID_ARGUMENT":                     "请求参数不完整",
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

func orgUnitInitiatorUUID(ctx context.Context, tenantID string) string {
	p, ok := currentPrincipal(ctx)
	if ok {
		candidate := strings.TrimSpace(p.ID)
		if candidate != "" {
			if _, err := uuid.Parse(candidate); err == nil {
				return candidate
			}
		}
	}
	return strings.TrimSpace(tenantID)
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

func orgNodeAuditLimitFromURL(r *http.Request) int {
	v := strings.TrimSpace(r.URL.Query().Get("limit"))
	if v == "" {
		return orgNodeAuditPageSize
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return orgNodeAuditPageSize
	}
	if n <= 0 {
		return orgNodeAuditPageSize
	}
	return n
}

func orgNodeActiveTabFromURL(r *http.Request) string {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tab")))
	if v == "change" {
		return "change"
	}
	return "basic"
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

type orgUnitNodeAuditReader interface {
	ListNodeAuditEvents(ctx context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error)
}

func listNodeAuditEvents(ctx context.Context, store OrgUnitStore, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
	reader, ok := store.(orgUnitNodeAuditReader)
	if !ok {
		return []OrgUnitNodeAuditEvent{}, nil
	}
	return reader.ListNodeAuditEvents(ctx, tenantID, orgID, limit)
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

func orgUnitBusinessUnitText(isBusinessUnit bool) string {
	if isBusinessUnit {
		return "是"
	}
	return "否"
}

func normalizeOrgUnitTargetStatus(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "active", "enabled", "有效":
		return "active", nil
	case "disabled", "inactive", "无效":
		return "disabled", nil
	default:
		return "", errors.New("target_status invalid")
	}
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
	  v.is_business_unit,
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
		if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.IsBusinessUnit, &item.HasChildren); err != nil {
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
	  v.is_business_unit,
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
		if err := rows.Scan(&item.OrgID, &item.OrgCode, &item.Name, &item.Status, &item.IsBusinessUnit, &item.HasChildren); err != nil {
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

func (s *orgUnitPGStore) ListNodeAuditEvents(ctx context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = orgNodeAuditPageSize
	}

	rows, err := tx.Query(ctx, `
	SELECT
	  e.id,
	  e.event_uuid::text,
	  e.org_id,
	  e.event_type,
	  e.effective_date,
	  e.tx_time,
	  COALESCE(e.initiator_name, ''),
	  COALESCE(e.initiator_employee_id, ''),
	  COALESCE(e.request_code, ''),
	  COALESCE(e.reason, ''),
	  COALESCE(e.payload, '{}'::jsonb),
	  e.before_snapshot,
	  e.after_snapshot,
	  COALESCE(e.rescind_outcome, ''),
	  (re.event_uuid IS NOT NULL) AS is_rescinded,
	  COALESCE(re.event_uuid::text, ''),
	  COALESCE(re.tx_time, 'epoch'::timestamptz),
	  COALESCE(re.request_code, '')
	FROM orgunit.org_events e
	LEFT JOIN LATERAL (
	  SELECT r.event_uuid, r.tx_time, r.request_code
	  FROM orgunit.org_events r
	  WHERE r.tenant_uuid = e.tenant_uuid
	    AND r.org_id = e.org_id
	    AND r.event_type IN ('RESCIND_EVENT', 'RESCIND_ORG')
	    AND r.payload->>'target_event_uuid' = e.event_uuid::text
	  ORDER BY r.tx_time DESC, r.id DESC
	  LIMIT 1
	) re ON true
	WHERE e.tenant_uuid = $1::uuid
	  AND e.org_id = $2::int
	ORDER BY e.tx_time DESC, e.id DESC
	LIMIT $3::int
	`, tenantID, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNodeAuditEvent
	for rows.Next() {
		var item OrgUnitNodeAuditEvent
		var effective time.Time
		var payload []byte
		var before []byte
		var after []byte
		if err := rows.Scan(
			&item.EventID,
			&item.EventUUID,
			&item.OrgID,
			&item.EventType,
			&effective,
			&item.TxTime,
			&item.InitiatorName,
			&item.InitiatorEmployeeID,
			&item.RequestCode,
			&item.Reason,
			&payload,
			&before,
			&after,
			&item.RescindOutcome,
			&item.IsRescinded,
			&item.RescindedByEventUUID,
			&item.RescindedByTxTime,
			&item.RescindedByRequestCode,
		); err != nil {
			return nil, err
		}
		item.EffectiveDate = effective.Format(asOfLayout)
		item.Payload = json.RawMessage(payload)
		if len(before) > 0 {
			item.BeforeSnapshot = json.RawMessage(before)
		}
		if len(after) > 0 {
			item.AfterSnapshot = json.RawMessage(after)
		}
		if !item.IsRescinded {
			item.RescindedByEventUUID = ""
			item.RescindedByRequestCode = ""
			item.RescindedByTxTime = time.Time{}
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
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

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
	`, eventID, tenantID, nil, effectiveDate, []byte(payload), eventID, initiatorUUID)
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
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

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
	`, eventID, tenantID, orgID, effectiveDate, []byte(payload), eventID, initiatorUUID); err != nil {
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
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

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
	`, eventID, tenantID, orgID, effectiveDate, []byte(payload), eventID, initiatorUUID); err != nil {
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
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

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
	`, eventID, tenantID, orgID, effectiveDate, eventID, initiatorUUID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgUnitPGStore) CorrectNodeEffectiveDate(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, newEffectiveDate string, requestCode string) error {
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
	if strings.TrimSpace(requestCode) == "" {
		return errors.New("request_code is required")
	}
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

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
	`, tenantID, orgID, targetEffectiveDate, []byte(patch), requestCode, initiatorUUID).Scan(&correctionUUID); err != nil {
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
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
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
		`, eventID, tenantID, orgID, effectiveDate, []byte(payload), requestCode, initiatorUUID); err != nil {
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_set_business_unit;`); rbErr != nil {
			return rbErr
		}
		var pgErr *pgconn.PgError
		dayConflict := strings.Contains(err.Error(), "EVENT_DATE_CONFLICT")
		if errors.As(err, &pgErr) && pgErr != nil && pgErr.Code == "23505" && pgErr.ConstraintName == "org_events_one_per_day_unique" {
			dayConflict = true
		}
		if dayConflict {
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

func (s *orgUnitMemoryStore) IsOrgTreeInitialized(_ context.Context, tenantID string) (bool, error) {
	return len(s.nodes[tenantID]) > 0, nil
}

func (s *orgUnitMemoryStore) ResolveAppendFacts(_ context.Context, tenantID string, orgID int, _ string) (orgUnitAppendFacts, error) {
	facts := orgUnitAppendFacts{
		TreeInitialized: len(s.nodes[tenantID]) > 0,
	}
	orgIDStr := strconv.Itoa(orgID)
	for _, node := range s.nodes[tenantID] {
		if node.ID != orgIDStr {
			continue
		}
		facts.TargetExistsAsOf = true
		facts.TargetStatusAsOf = strings.TrimSpace(node.Status)
		if facts.TargetStatusAsOf == "" {
			facts.TargetStatusAsOf = "active"
		}
		facts.IsRoot = strings.EqualFold(strings.TrimSpace(node.OrgCode), "ROOT")
		break
	}
	return facts, nil
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

func (s *orgUnitMemoryStore) ListNodeAuditEvents(_ context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
	orgIDStr := strconv.Itoa(orgID)
	for _, node := range s.nodes[tenantID] {
		if node.ID != orgIDStr {
			continue
		}
		if limit <= 0 {
			limit = orgNodeAuditPageSize
		}
		events := []OrgUnitNodeAuditEvent{{
			EventID:             1,
			EventUUID:           node.ID,
			OrgID:               orgID,
			EventType:           "RENAME",
			EffectiveDate:       "2026-01-01",
			TxTime:              s.now(),
			InitiatorName:       "system",
			InitiatorEmployeeID: "system",
			RequestCode:         "memory",
			Payload:             json.RawMessage(`{"op":"RENAME"}`),
		}}
		return events, nil
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

func (s *orgUnitMemoryStore) ListEnabledTenantFieldConfigsAsOf(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
	return []orgUnitTenantFieldConfig{}, nil
}

func (s *orgUnitMemoryStore) GetOrgUnitVersionExtSnapshot(_ context.Context, _ string, _ int, _ string) (orgUnitVersionExtSnapshot, error) {
	return orgUnitVersionExtSnapshot{
		VersionValues:  map[string]any{},
		VersionLabels:  map[string]string{},
		EventLabels:    map[string]string{},
		LastEventID:    0,
		HasVersionData: true,
	}, nil
}
