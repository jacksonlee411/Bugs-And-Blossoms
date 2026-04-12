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
	OrgID          int
	ID             string
	OrgCode        string
	Name           string
	Status         string
	IsBusinessUnit bool
	HasChildren    bool
	CreatedAt      time.Time
}

type OrgUnitChild struct {
	OrgID          int
	OrgNodeKey     string
	OrgCode        string
	Name           string
	Status         string
	IsBusinessUnit bool
	HasChildren    bool
}

type OrgUnitNodeDetails struct {
	OrgID            int
	OrgNodeKey       string
	OrgCode          string
	Name             string
	Status           string
	ParentID         int
	ParentOrgNodeKey string
	ParentCode       string
	ParentName       string
	IsBusinessUnit   bool
	ManagerPernr     string
	ManagerName      string
	PathIDs          []int
	PathOrgNodeKeys  []string
	FullNamePath     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	EventUUID        string
}

type OrgUnitSearchResult struct {
	TargetOrgID      int      `json:"-"`
	TargetOrgNodeKey string   `json:"-"`
	TargetOrgCode    string   `json:"target_org_code"`
	TargetName       string   `json:"target_name"`
	PathOrgIDs       []int    `json:"-"`
	PathOrgNodeKeys  []string `json:"-"`
	PathOrgCodes     []string `json:"path_org_codes,omitempty"`
	TreeAsOf         string   `json:"tree_as_of"`
}

type OrgUnitSearchCandidate struct {
	OrgID      int
	OrgNodeKey string
	OrgCode    string
	Name       string
	Status     string
}

type OrgUnitNodeVersion struct {
	EventID       int64
	EventUUID     string
	EffectiveDate string
	EventType     string
}

type OrgUnitNodeAuditEvent struct {
	EventID              int64
	EventUUID            string
	OrgID                int
	OrgNodeKey           string
	EventType            string
	EffectiveDate        string
	TxTime               time.Time
	InitiatorName        string
	InitiatorEmployeeID  string
	RequestID            string
	Reason               string
	Payload              json.RawMessage
	BeforeSnapshot       json.RawMessage
	AfterSnapshot        json.RawMessage
	RescindOutcome       string
	IsRescinded          bool
	RescindedByEventUUID string
	RescindedByTxTime    time.Time
	RescindedByRequestID string
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
		"FIELD_NOT_MAINTAINABLE":                   "字段为系统维护，不允许手动输入",
		"DEFAULT_RULE_REQUIRED":                    "字段被设为系统维护，但未配置默认规则",
		"DEFAULT_RULE_EVAL_FAILED":                 "默认规则执行失败，请检查规则配置",
		"FIELD_POLICY_EXPR_INVALID":                "默认规则表达式不合法",
		"FIELD_OPTION_NOT_ALLOWED":                 "字段值不在允许范围内，请重新选择",
		"FIELD_REQUIRED_VALUE_MISSING":             "必填字段缺少有效值，请补全后重试",
		"policy_missing":                           "未找到匹配的字段策略，请刷新页面后重试",
		"policy_conflict_ambiguous":                "字段策略冲突，请联系管理员修复策略配置",
		"policy_version_required":                  "缺少策略版本，请刷新页面后重试",
		"policy_version_conflict":                  "策略版本已变化，请刷新页面后重试",
		"ORG_CODE_EXHAUSTED":                       "组织编码已耗尽，请调整编码规则后重试",
		"ORG_CODE_CONFLICT":                        "组织编码冲突，请重试",
		"FIELD_POLICY_SCOPE_OVERLAP":               "同一字段作用域的策略生效区间重叠",
		"ORG_FIELD_POLICY_NOT_FOUND":               "未找到匹配的字段策略",
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
	SetBusinessUnitCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, isBusinessUnit bool, requestID string) error
}

type OrgUnitCodeResolver interface {
	ResolveOrgNodeKeyByCode(ctx context.Context, tenantID string, orgCode string) (string, error)
	ResolveOrgCodeByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) (string, error)
	ResolveOrgCodesByNodeKeys(ctx context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error)
}

type OrgUnitNodeChildrenReader interface {
	ListChildren(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error)
}

type orgUnitNodeChildrenByKeyReader interface {
	ListChildrenByNodeKey(ctx context.Context, tenantID string, parentOrgNodeKey string, asOfDate string) ([]OrgUnitChild, error)
}

type OrgUnitNodeDetailsReader interface {
	GetNodeDetails(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error)
}

type orgUnitNodeDetailsByKeyReader interface {
	GetNodeDetailsByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string) (OrgUnitNodeDetails, error)
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

type orgUnitNodeVersionByKeyReader interface {
	ListNodeVersionsByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) ([]OrgUnitNodeVersion, error)
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

type orgUnitNodesVisibilityByKeyReader interface {
	ListChildrenWithVisibilityByNodeKey(ctx context.Context, tenantID string, parentOrgNodeKey string, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error)
	GetNodeDetailsWithVisibilityByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error)
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

func parseOrgNodeKey(input string) (string, error) {
	orgNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(input)
	if err != nil {
		return "", errors.New("org_node_key invalid")
	}
	return orgNodeKey, nil
}

func parseOptionalOrgNodeKey(input string) (string, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false, nil
	}
	value, err := normalizeOrgNodeKeyInput(trimmed)
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func parseLegacyOrgID8Digits(input string) (int, error) {
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

func normalizeOrgNodeKeyInput(input string) (string, error) {
	if orgNodeKey, err := parseOrgNodeKey(input); err == nil {
		return orgNodeKey, nil
	}
	orgID, err := parseLegacyOrgID8Digits(input)
	if err != nil {
		return "", err
	}
	return encodeOrgNodeKeyFromID(orgID)
}

func parseOrgID8(input string) (int, error) {
	if orgNodeKey, err := parseOrgNodeKey(input); err == nil {
		return decodeOrgNodeKeyToID(orgNodeKey)
	}
	return parseLegacyOrgID8Digits(input)
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

func encodeOrgNodeKeyFromID(orgID int) (string, error) {
	return orgunitpkg.EncodeOrgNodeKey(int64(orgID))
}

func decodeOrgNodeKeyToID(orgNodeKey string) (int, error) {
	normalized, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return 0, err
	}
	decoded, err := orgunitpkg.DecodeOrgNodeKey(normalized)
	if err != nil {
		return 0, err
	}
	return int(decoded), nil
}

func decodeOrgNodeKeysToIDs(orgNodeKeys []string) ([]int, error) {
	if len(orgNodeKeys) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(orgNodeKeys))
	for _, orgNodeKey := range orgNodeKeys {
		orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
		if err != nil {
			return nil, err
		}
		out = append(out, orgID)
	}
	return out, nil
}

func decodeOptionalOrgNodeKeyToID(orgNodeKey string) (int, error) {
	if strings.TrimSpace(orgNodeKey) == "" {
		return 0, nil
	}
	return decodeOrgNodeKeyToID(orgNodeKey)
}

func hydrateOrgUnitNodeCompat(item *OrgUnitNode) error {
	if item == nil {
		return nil
	}
	orgID, err := decodeOptionalOrgNodeKeyToID(item.ID)
	if err != nil {
		return err
	}
	item.OrgID = orgID
	return nil
}

func hydrateOrgUnitChildCompat(item *OrgUnitChild) error {
	if item == nil {
		return nil
	}
	orgID, err := decodeOptionalOrgNodeKeyToID(item.OrgNodeKey)
	if err != nil {
		return err
	}
	item.OrgID = orgID
	return nil
}

func hydrateOrgUnitNodeDetailsCompat(details *OrgUnitNodeDetails) error {
	if details == nil {
		return nil
	}
	orgID, err := decodeOptionalOrgNodeKeyToID(details.OrgNodeKey)
	if err != nil {
		return err
	}
	parentID, err := decodeOptionalOrgNodeKeyToID(details.ParentOrgNodeKey)
	if err != nil {
		return err
	}
	pathIDs, err := decodeOrgNodeKeysToIDs(details.PathOrgNodeKeys)
	if err != nil {
		return err
	}
	details.OrgID = orgID
	details.ParentID = parentID
	details.PathIDs = pathIDs
	return nil
}

func hydrateOrgUnitSearchResultCompat(result *OrgUnitSearchResult) error {
	if result == nil {
		return nil
	}
	targetOrgID, err := decodeOptionalOrgNodeKeyToID(result.TargetOrgNodeKey)
	if err != nil {
		return err
	}
	pathOrgIDs, err := decodeOrgNodeKeysToIDs(result.PathOrgNodeKeys)
	if err != nil {
		return err
	}
	result.TargetOrgID = targetOrgID
	result.PathOrgIDs = pathOrgIDs
	return nil
}

func hydrateOrgUnitSearchCandidateCompat(item *OrgUnitSearchCandidate) error {
	if item == nil {
		return nil
	}
	orgID, err := decodeOptionalOrgNodeKeyToID(item.OrgNodeKey)
	if err != nil {
		return err
	}
	item.OrgID = orgID
	return nil
}

func hydrateOrgUnitNodeAuditEventCompat(item *OrgUnitNodeAuditEvent) error {
	if item == nil {
		return nil
	}
	orgID, err := decodeOptionalOrgNodeKeyToID(item.OrgNodeKey)
	if err != nil {
		return err
	}
	item.OrgID = orgID
	return nil
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

type orgUnitNodeAuditByKeyReader interface {
	ListNodeAuditEventsByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, limit int) ([]OrgUnitNodeAuditEvent, error)
}

func listNodeAuditEvents(ctx context.Context, store OrgUnitStore, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
	reader, ok := store.(orgUnitNodeAuditReader)
	if !ok {
		return []OrgUnitNodeAuditEvent{}, nil
	}
	return reader.ListNodeAuditEvents(ctx, tenantID, orgID, limit)
}

func listNodeAuditEventsByNodeKey(ctx context.Context, store OrgUnitStore, tenantID string, orgNodeKey string, limit int) ([]OrgUnitNodeAuditEvent, error) {
	if reader, ok := store.(orgUnitNodeAuditByKeyReader); ok {
		return reader.ListNodeAuditEventsByNodeKey(ctx, tenantID, orgNodeKey, limit)
	}
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return nil, err
	}
	return listNodeAuditEvents(ctx, store, tenantID, orgID, limit)
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

func listChildrenByVisibilityByNodeKey(ctx context.Context, store OrgUnitStore, tenantID string, parentOrgNodeKey string, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityByKeyReader); ok {
			return vStore.ListChildrenWithVisibilityByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate, true)
		}
	}
	if reader, ok := store.(orgUnitNodeChildrenByKeyReader); ok {
		return reader.ListChildrenByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate)
	}
	parentID, err := decodeOrgNodeKeyToID(parentOrgNodeKey)
	if err != nil {
		return nil, err
	}
	return listChildrenByVisibility(ctx, store, tenantID, parentID, asOfDate, includeDisabled)
}

func getNodeDetailsByVisibility(ctx context.Context, store OrgUnitStore, tenantID string, orgID int, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityReader); ok {
			return vStore.GetNodeDetailsWithVisibility(ctx, tenantID, orgID, asOfDate, true)
		}
	}
	return store.GetNodeDetails(ctx, tenantID, orgID, asOfDate)
}

func getNodeDetailsByVisibilityByNodeKey(ctx context.Context, store OrgUnitStore, tenantID string, orgNodeKey string, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	if includeDisabled {
		if vStore, ok := store.(orgUnitNodesVisibilityByKeyReader); ok {
			return vStore.GetNodeDetailsWithVisibilityByNodeKey(ctx, tenantID, orgNodeKey, asOfDate, true)
		}
	}
	if reader, ok := store.(orgUnitNodeDetailsByKeyReader); ok {
		return reader.GetNodeDetailsByNodeKey(ctx, tenantID, orgNodeKey, asOfDate)
	}
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return getNodeDetailsByVisibility(ctx, store, tenantID, orgID, asOfDate, includeDisabled)
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

func listNodeVersionsByNodeKey(ctx context.Context, store OrgUnitStore, tenantID string, orgNodeKey string) ([]OrgUnitNodeVersion, error) {
	if reader, ok := store.(orgUnitNodeVersionByKeyReader); ok {
		return reader.ListNodeVersionsByNodeKey(ctx, tenantID, orgNodeKey)
	}
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return nil, err
	}
	return store.ListNodeVersions(ctx, tenantID, orgID)
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

func (s *orgUnitPGStore) ResolveOrgNodeKeyByCode(ctx context.Context, tenantID string, orgCode string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	orgNodeKey, err := orgunitpkg.ResolveOrgNodeKeyByCode(ctx, tx, tenantID, orgCode)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orgNodeKey, nil
}

func (s *orgUnitPGStore) ResolveOrgCodeByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	orgCode, err := orgunitpkg.ResolveOrgCodeByNodeKey(ctx, tx, tenantID, orgNodeKey)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orgCode, nil
}

func (s *orgUnitPGStore) ResolveOrgCodesByNodeKeys(ctx context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	resolvedByNodeKey, err := orgunitpkg.ResolveOrgCodesByNodeKeys(ctx, tx, tenantID, orgNodeKeys)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return resolvedByNodeKey, nil
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
	SELECT
	  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
	  c.org_code,
	  v.name,
	  v.is_business_unit,
	  EXISTS (
	    SELECT 1
	    FROM orgunit.org_unit_versions child
	    WHERE child.tenant_uuid = $1::uuid
	      AND `+parentOrgNodeKeyCompatExpr("child")+` = `+orgNodeKeyCompatExpr("v")+`
	      AND child.status = 'active'
	      AND child.validity @> $2::date
	  ) AS has_children,
	  e.transaction_time
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
	JOIN orgunit.org_events e
	  ON e.id = v.last_event_id
	WHERE v.tenant_uuid = $1::uuid
	  AND v.status = 'active'
	  AND v.validity @> $2::date
	  AND `+rootOrgNodeCompatCondition("v")+`
	ORDER BY v.node_path
	`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNode
	for rows.Next() {
		var n OrgUnitNode
		if err := rows.Scan(&n.ID, &n.OrgCode, &n.Name, &n.IsBusinessUnit, &n.HasChildren, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Status = "active"
		if err := hydrateOrgUnitNodeCompat(&n); err != nil {
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
  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
  c.org_code,
  v.name,
  v.status,
  v.is_business_unit,
  EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions child
    WHERE child.tenant_uuid = $1::uuid
      AND `+parentOrgNodeKeyCompatExpr("child")+` = `+orgNodeKeyCompatExpr("v")+`
      AND child.validity @> $2::date
  ) AS has_children,
  e.transaction_time
FROM orgunit.org_unit_versions v
JOIN orgunit.org_unit_codes c
  ON c.tenant_uuid = $1::uuid
 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
JOIN orgunit.org_events e
  ON e.id = v.last_event_id
WHERE v.tenant_uuid = $1::uuid
  AND v.validity @> $2::date
  AND `+rootOrgNodeCompatCondition("v")+`
ORDER BY v.node_path
`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNode
	for rows.Next() {
		var n OrgUnitNode
		if err := rows.Scan(&n.ID, &n.OrgCode, &n.Name, &n.Status, &n.IsBusinessUnit, &n.HasChildren, &n.CreatedAt); err != nil {
			return nil, err
		}
		if err := hydrateOrgUnitNodeCompat(&n); err != nil {
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
	SELECT
	  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
	  c.org_code,
	  v.name,
	  v.is_business_unit,
	  e.transaction_time
	FROM orgunit.org_unit_versions v
	JOIN orgunit.org_unit_codes c
	  ON c.tenant_uuid = $1::uuid
	 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
	JOIN orgunit.org_events e
	  ON e.id = v.last_event_id
	WHERE v.tenant_uuid = $1::uuid
	  AND v.status = 'active'
	  AND v.validity @> $2::date
	  AND v.is_business_unit
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
		if err := hydrateOrgUnitNodeCompat(&n); err != nil {
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
	parentOrgNodeKey, err := encodeOrgNodeKeyFromID(parentID)
	if err != nil {
		return nil, err
	}
	return s.ListChildrenByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate)
}

func (s *orgUnitPGStore) ListChildrenByNodeKey(ctx context.Context, tenantID string, parentOrgNodeKey string, asOfDate string) ([]OrgUnitChild, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	normalizedParentOrgNodeKey, err := normalizeOrgNodeKeyInput(parentOrgNodeKey)
	if err != nil {
		return nil, err
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM orgunit.org_unit_versions
		  WHERE tenant_uuid = $1::uuid
		    AND `+orgNodeKeyCompatExpr("org_unit_versions")+` = $2::text
		    AND status = 'active'
		    AND validity @> $3::date
		)
		`, tenantID, normalizedParentOrgNodeKey, asOfDate).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errOrgUnitNotFound
	}

	rows, err := tx.Query(ctx, `
		SELECT
		  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
		  c.org_code,
		  v.name,
		  v.is_business_unit,
		  EXISTS (
		    SELECT 1
		    FROM orgunit.org_unit_versions child
		    WHERE child.tenant_uuid = $1::uuid
		      AND `+parentOrgNodeKeyCompatExpr("child")+` = `+orgNodeKeyCompatExpr("v")+`
		      AND child.status = 'active'
		      AND child.validity @> $3::date
		  ) AS has_children
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
		WHERE v.tenant_uuid = $1::uuid
		  AND `+parentOrgNodeKeyCompatExpr("v")+` = $2::text
		  AND v.status = 'active'
		  AND v.validity @> $3::date
		ORDER BY v.node_path
		`, tenantID, normalizedParentOrgNodeKey, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitChild
	for rows.Next() {
		var item OrgUnitChild
		if err := rows.Scan(&item.OrgNodeKey, &item.OrgCode, &item.Name, &item.IsBusinessUnit, &item.HasChildren); err != nil {
			return nil, err
		}
		if err := hydrateOrgUnitChildCompat(&item); err != nil {
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
	parentOrgNodeKey, err := encodeOrgNodeKeyFromID(parentID)
	if err != nil {
		return nil, err
	}
	return s.ListChildrenWithVisibilityByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate, includeDisabled)
}

func (s *orgUnitPGStore) ListChildrenWithVisibilityByNodeKey(ctx context.Context, tenantID string, parentOrgNodeKey string, asOfDate string, includeDisabled bool) ([]OrgUnitChild, error) {
	if !includeDisabled {
		return s.ListChildrenByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	normalizedParentOrgNodeKey, err := normalizeOrgNodeKeyInput(parentOrgNodeKey)
	if err != nil {
		return nil, err
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM orgunit.org_unit_versions
		  WHERE tenant_uuid = $1::uuid
		    AND `+orgNodeKeyCompatExpr("org_unit_versions")+` = $2::text
		    AND validity @> $3::date
		)
		`, tenantID, normalizedParentOrgNodeKey, asOfDate).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errOrgUnitNotFound
	}

	rows, err := tx.Query(ctx, `
		SELECT
		  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
		  c.org_code,
		  v.name,
		  v.status,
		  v.is_business_unit,
		  EXISTS (
		    SELECT 1
		    FROM orgunit.org_unit_versions child
		    WHERE child.tenant_uuid = $1::uuid
		      AND `+parentOrgNodeKeyCompatExpr("child")+` = `+orgNodeKeyCompatExpr("v")+`
		      AND child.validity @> $3::date
		  ) AS has_children
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
		WHERE v.tenant_uuid = $1::uuid
		  AND `+parentOrgNodeKeyCompatExpr("v")+` = $2::text
		  AND v.validity @> $3::date
		ORDER BY v.node_path
		`, tenantID, normalizedParentOrgNodeKey, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitChild
	for rows.Next() {
		var item OrgUnitChild
		if err := rows.Scan(&item.OrgNodeKey, &item.OrgCode, &item.Name, &item.Status, &item.IsBusinessUnit, &item.HasChildren); err != nil {
			return nil, err
		}
		if err := hydrateOrgUnitChildCompat(&item); err != nil {
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
	requestedOrgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return s.GetNodeDetailsByNodeKey(ctx, tenantID, requestedOrgNodeKey, asOfDate)
}

func (s *orgUnitPGStore) GetNodeDetailsByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string) (OrgUnitNodeDetails, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitNodeDetails{}, err
	}

	requestedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}

	var details OrgUnitNodeDetails
	var pathOrgNodeKeys []string
	if err := tx.QueryRow(ctx, `
		SELECT
		  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
		  c.org_code,
		  v.name,
		  v.status,
		  `+parentOrgNodeKeyCompatExpr("v")+` AS parent_org_node_key,
		  COALESCE(pc.org_code, '') AS parent_org_code,
		  COALESCE(pv.name, '') AS parent_name,
		  v.is_business_unit,
		  COALESCE(p.pernr, '') AS manager_pernr,
		  COALESCE(p.display_name, '') AS manager_name,
		  `+pathOrgNodeKeysCompatExpr("v")+` AS path_org_node_keys,
		  COALESCE(v.full_name_path, '') AS full_name_path,
		  c.created_at,
		  e.transaction_time,
		  e.event_uuid
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
		JOIN orgunit.org_events e
		  ON e.id = v.last_event_id
		LEFT JOIN orgunit.org_unit_codes pc
		  ON pc.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("pc")+` = `+parentOrgNodeKeyCompatExpr("v")+`
		LEFT JOIN orgunit.org_unit_versions pv
		  ON pv.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("pv")+` = `+parentOrgNodeKeyCompatExpr("v")+`
		 AND pv.status = 'active'
		 AND pv.validity @> $3::date
		LEFT JOIN person.persons p
		  ON p.tenant_uuid = $1::uuid
		 AND p.person_uuid = v.manager_uuid
		WHERE v.tenant_uuid = $1::uuid
		  AND `+orgNodeKeyCompatExpr("v")+` = $2::text
		  AND v.status = 'active'
		  AND v.validity @> $3::date
		LIMIT 1
		`, tenantID, requestedOrgNodeKey, asOfDate).Scan(
		&details.OrgNodeKey,
		&details.OrgCode,
		&details.Name,
		&details.Status,
		&details.ParentOrgNodeKey,
		&details.ParentCode,
		&details.ParentName,
		&details.IsBusinessUnit,
		&details.ManagerPernr,
		&details.ManagerName,
		&pathOrgNodeKeys,
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

	if len(pathOrgNodeKeys) > 0 {
		details.PathOrgNodeKeys = append([]string(nil), pathOrgNodeKeys...)
	}
	if err := hydrateOrgUnitNodeDetailsCompat(&details); err != nil {
		return OrgUnitNodeDetails{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return details, nil
}

func (s *orgUnitPGStore) GetNodeDetailsWithVisibility(ctx context.Context, tenantID string, orgID int, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	requestedOrgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return s.GetNodeDetailsWithVisibilityByNodeKey(ctx, tenantID, requestedOrgNodeKey, asOfDate, includeDisabled)
}

func (s *orgUnitPGStore) GetNodeDetailsWithVisibilityByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string, includeDisabled bool) (OrgUnitNodeDetails, error) {
	if !includeDisabled {
		return s.GetNodeDetailsByNodeKey(ctx, tenantID, orgNodeKey, asOfDate)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return OrgUnitNodeDetails{}, err
	}

	requestedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}

	var details OrgUnitNodeDetails
	var pathOrgNodeKeys []string
	if err := tx.QueryRow(ctx, `
		SELECT
		  `+orgNodeKeyCompatExpr("v")+` AS org_node_key,
		  c.org_code,
		  v.name,
		  v.status,
		  `+parentOrgNodeKeyCompatExpr("v")+` AS parent_org_node_key,
		  COALESCE(pc.org_code, '') AS parent_org_code,
		  COALESCE(pv.name, '') AS parent_name,
		  v.is_business_unit,
		  COALESCE(p.pernr, '') AS manager_pernr,
		  COALESCE(p.display_name, '') AS manager_name,
		  `+pathOrgNodeKeysCompatExpr("v")+` AS path_org_node_keys,
		  COALESCE(v.full_name_path, '') AS full_name_path,
		  c.created_at,
		  e.transaction_time,
		  e.event_uuid
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
		JOIN orgunit.org_events e
		  ON e.id = v.last_event_id
		LEFT JOIN orgunit.org_unit_codes pc
		  ON pc.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("pc")+` = `+parentOrgNodeKeyCompatExpr("v")+`
		LEFT JOIN orgunit.org_unit_versions pv
		  ON pv.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("pv")+` = `+parentOrgNodeKeyCompatExpr("v")+`
		 AND pv.validity @> $3::date
		LEFT JOIN person.persons p
		  ON p.tenant_uuid = $1::uuid
		 AND p.person_uuid = v.manager_uuid
		WHERE v.tenant_uuid = $1::uuid
		  AND `+orgNodeKeyCompatExpr("v")+` = $2::text
		  AND v.validity @> $3::date
		LIMIT 1
		`, tenantID, requestedOrgNodeKey, asOfDate).Scan(
		&details.OrgNodeKey,
		&details.OrgCode,
		&details.Name,
		&details.Status,
		&details.ParentOrgNodeKey,
		&details.ParentCode,
		&details.ParentName,
		&details.IsBusinessUnit,
		&details.ManagerPernr,
		&details.ManagerName,
		&pathOrgNodeKeys,
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

	if len(pathOrgNodeKeys) > 0 {
		details.PathOrgNodeKeys = append([]string(nil), pathOrgNodeKeys...)
	}
	if err := hydrateOrgUnitNodeDetailsCompat(&details); err != nil {
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
	var pathOrgNodeKeys []string
	found := false

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		if err := tx.QueryRow(ctx, `
			SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name, `+pathOrgNodeKeysCompatExpr("v")+` AS path_org_node_keys
			FROM orgunit.org_unit_versions v
			JOIN orgunit.org_unit_codes c
			  ON c.tenant_uuid = $1::uuid
			 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
			WHERE v.tenant_uuid = $1::uuid
			  AND v.status = 'active'
			  AND v.validity @> $3::date
			  AND c.org_code = $2::text
			LIMIT 1
			`, tenantID, normalized, asOfDate).Scan(&result.TargetOrgNodeKey, &result.TargetOrgCode, &result.TargetName, &pathOrgNodeKeys); err == nil {
			found = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return OrgUnitSearchResult{}, err
		}
	}

	if !found {
		if err := tx.QueryRow(ctx, `
			SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name, `+pathOrgNodeKeysCompatExpr("v")+` AS path_org_node_keys
			FROM orgunit.org_unit_versions v
			JOIN orgunit.org_unit_codes c
			  ON c.tenant_uuid = $1::uuid
			 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
			WHERE v.tenant_uuid = $1::uuid
			  AND v.status = 'active'
			  AND v.validity @> $3::date
			  AND v.name ILIKE $2::text
			ORDER BY v.node_path
			LIMIT 1
			`, tenantID, "%"+trimmed+"%", asOfDate).Scan(&result.TargetOrgNodeKey, &result.TargetOrgCode, &result.TargetName, &pathOrgNodeKeys); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return OrgUnitSearchResult{}, errOrgUnitNotFound
			}
			return OrgUnitSearchResult{}, err
		}
	}
	if len(pathOrgNodeKeys) > 0 {
		result.PathOrgNodeKeys = append([]string(nil), pathOrgNodeKeys...)
	}
	if err := hydrateOrgUnitSearchResultCompat(&result); err != nil {
		return OrgUnitSearchResult{}, err
	}
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
	var pathOrgNodeKeys []string
	found := false

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		if err := tx.QueryRow(ctx, `
			SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name, `+pathOrgNodeKeysCompatExpr("v")+` AS path_org_node_keys
			FROM orgunit.org_unit_versions v
			JOIN orgunit.org_unit_codes c
			  ON c.tenant_uuid = $1::uuid
			 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
			WHERE v.tenant_uuid = $1::uuid
			  AND v.validity @> $3::date
			  AND c.org_code = $2::text
			LIMIT 1
			`, tenantID, normalized, asOfDate).Scan(&result.TargetOrgNodeKey, &result.TargetOrgCode, &result.TargetName, &pathOrgNodeKeys); err == nil {
			found = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return OrgUnitSearchResult{}, err
		}
	}

	if !found {
		if err := tx.QueryRow(ctx, `
			SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name, `+pathOrgNodeKeysCompatExpr("v")+` AS path_org_node_keys
			FROM orgunit.org_unit_versions v
			JOIN orgunit.org_unit_codes c
			  ON c.tenant_uuid = $1::uuid
			 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
			WHERE v.tenant_uuid = $1::uuid
			  AND v.validity @> $3::date
			  AND v.name ILIKE $2::text
			ORDER BY v.node_path
			LIMIT 1
			`, tenantID, "%"+trimmed+"%", asOfDate).Scan(&result.TargetOrgNodeKey, &result.TargetOrgCode, &result.TargetName, &pathOrgNodeKeys); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return OrgUnitSearchResult{}, errOrgUnitNotFound
			}
			return OrgUnitSearchResult{}, err
		}
	}
	if len(pathOrgNodeKeys) > 0 {
		result.PathOrgNodeKeys = append([]string(nil), pathOrgNodeKeys...)
	}
	if err := hydrateOrgUnitSearchResultCompat(&result); err != nil {
		return OrgUnitSearchResult{}, err
	}
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
			SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name
			FROM orgunit.org_unit_versions v
			JOIN orgunit.org_unit_codes c
			  ON c.tenant_uuid = $1::uuid
			 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
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
			if err := rows.Scan(&item.OrgNodeKey, &item.OrgCode, &item.Name); err != nil {
				return nil, err
			}
			if err := hydrateOrgUnitSearchCandidateCompat(&item); err != nil {
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
		SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
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
		if err := rows.Scan(&item.OrgNodeKey, &item.OrgCode, &item.Name); err != nil {
			return nil, err
		}
		if err := hydrateOrgUnitSearchCandidateCompat(&item); err != nil {
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
			SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name, v.status
			FROM orgunit.org_unit_versions v
			JOIN orgunit.org_unit_codes c
			  ON c.tenant_uuid = $1::uuid
			 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
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
			if err := rows.Scan(&item.OrgNodeKey, &item.OrgCode, &item.Name, &item.Status); err != nil {
				return nil, err
			}
			if err := hydrateOrgUnitSearchCandidateCompat(&item); err != nil {
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
		SELECT `+orgNodeKeyCompatExpr("v")+` AS org_node_key, c.org_code, v.name, v.status
		FROM orgunit.org_unit_versions v
		JOIN orgunit.org_unit_codes c
		  ON c.tenant_uuid = $1::uuid
		 AND `+orgNodeKeyCompatExpr("c")+` = `+orgNodeKeyCompatExpr("v")+`
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
		if err := rows.Scan(&item.OrgNodeKey, &item.OrgCode, &item.Name, &item.Status); err != nil {
			return nil, err
		}
		if err := hydrateOrgUnitSearchCandidateCompat(&item); err != nil {
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
	requestedOrgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return nil, err
	}
	return s.ListNodeVersionsByNodeKey(ctx, tenantID, requestedOrgNodeKey)
}

func (s *orgUnitPGStore) ListNodeVersionsByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) ([]OrgUnitNodeVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	requestedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
		SELECT e.id, e.event_uuid, e.effective_date, e.event_type
		FROM orgunit.org_events_effective e
		WHERE e.tenant_uuid = $1::uuid
		  AND `+orgNodeKeyCompatExpr("e")+` = $2::text
		ORDER BY e.effective_date, e.id
		`, tenantID, requestedOrgNodeKey)
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
	requestedOrgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return nil, err
	}
	return s.ListNodeAuditEventsByNodeKey(ctx, tenantID, requestedOrgNodeKey, limit)
}

func (s *orgUnitPGStore) ListNodeAuditEventsByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, limit int) ([]OrgUnitNodeAuditEvent, error) {
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

	requestedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
		SELECT
		  e.id,
		  e.event_uuid::text,
		  `+orgNodeKeyCompatExpr("e")+` AS org_node_key,
		  e.event_type,
		  e.effective_date,
		  e.tx_time,
	  COALESCE(e.initiator_name, ''),
	  COALESCE(e.initiator_employee_id, ''),
	  COALESCE(e.request_id, ''),
	  COALESCE(e.reason, ''),
	  COALESCE(e.payload, '{}'::jsonb),
	  e.before_snapshot,
	  e.after_snapshot,
	  COALESCE(e.rescind_outcome, ''),
	  (re.event_uuid IS NOT NULL) AS is_rescinded,
	  COALESCE(re.event_uuid::text, ''),
	  COALESCE(re.tx_time, 'epoch'::timestamptz),
	  COALESCE(re.request_id, '')
	FROM orgunit.org_events e
		LEFT JOIN LATERAL (
		  SELECT r.event_uuid, r.tx_time, r.request_id
		  FROM orgunit.org_events r
		  WHERE r.tenant_uuid = e.tenant_uuid
		    AND `+orgNodeKeyCompatExpr("r")+` = `+orgNodeKeyCompatExpr("e")+`
		    AND r.event_type IN ('RESCIND_EVENT', 'RESCIND_ORG')
		    AND r.payload->>'target_event_uuid' = e.event_uuid::text
	  ORDER BY r.tx_time DESC, r.id DESC
	  LIMIT 1
		) re ON true
		WHERE e.tenant_uuid = $1::uuid
		  AND `+orgNodeKeyCompatExpr("e")+` = $2::text
		ORDER BY e.tx_time DESC, e.id DESC
		LIMIT $3::int
		`, tenantID, requestedOrgNodeKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrgUnitNodeAuditEvent
	for rows.Next() {
		var item OrgUnitNodeAuditEvent
		var effective time.Time
		var eventOrgNodeKey string
		var payload []byte
		var before []byte
		var after []byte
		if err := rows.Scan(
			&item.EventID,
			&item.EventUUID,
			&eventOrgNodeKey,
			&item.EventType,
			&effective,
			&item.TxTime,
			&item.InitiatorName,
			&item.InitiatorEmployeeID,
			&item.RequestID,
			&item.Reason,
			&payload,
			&before,
			&after,
			&item.RescindOutcome,
			&item.IsRescinded,
			&item.RescindedByEventUUID,
			&item.RescindedByTxTime,
			&item.RescindedByRequestID,
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
		item.OrgNodeKey = eventOrgNodeKey
		if err := hydrateOrgUnitNodeAuditEventCompat(&item); err != nil {
			return nil, err
		}
		if !item.IsRescinded {
			item.RescindedByEventUUID = ""
			item.RescindedByRequestID = ""
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

func (s *orgUnitPGStore) ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return "", err
	}

	out, err := setid.Resolve(ctx, tx, tenantID, normalizedOrgNodeKey, asOfDate)
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

	if normalizedParentID, ok, err := parseOptionalOrgNodeKey(parentID); err != nil {
		return OrgUnitNode{}, err
	} else if ok {
		parentID = normalizedParentID
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return OrgUnitNode{}, err
	}
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

	payload := `{"org_code":` + strconv.Quote(normalizedCode) + `,"name":` + strconv.Quote(name)
	if strings.TrimSpace(parentID) != "" {
		payload += `,"parent_org_node_key":` + strconv.Quote(parentID)
	}
	payload += `,"is_business_unit":` + strconv.FormatBool(isBusinessUnit)
	payload += `}`

	_, err = tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  $3::char(8),
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

	var orgNodeKey string
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
SELECT `+orgNodeKeyCompatExpr("e")+` AS org_node_key, e.transaction_time
FROM orgunit.org_events e
WHERE e.tenant_uuid = $1::uuid AND e.event_uuid = $2::uuid
`, tenantID, eventID).Scan(&orgNodeKey, &createdAt); err != nil {
		return OrgUnitNode{}, err
	}
	orgNodeKey, err = normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return OrgUnitNode{}, err
	}
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return OrgUnitNode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return OrgUnitNode{}, err
	}
	return OrgUnitNode{OrgID: orgID, ID: orgNodeKey, OrgCode: normalizedCode, Name: name, IsBusinessUnit: isBusinessUnit, CreatedAt: createdAt}, nil
}

func (s *orgUnitPGStore) RenameNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgNodeKey string, newName string) error {
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

	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
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
  $3::char(8),
  'RENAME',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
		`, eventID, tenantID, normalizedOrgNodeKey, effectiveDate, []byte(payload), eventID, initiatorUUID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgUnitPGStore) MoveNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgNodeKey string, newParentID string) error {
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

	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)

	payload := `{}`
	if normalizedNewParentID, ok, err := parseOptionalOrgNodeKey(newParentID); err != nil {
		return err
	} else if ok {
		payload = `{"new_parent_org_node_key":` + strconv.Quote(normalizedNewParentID) + `}`
	}

	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  $3::char(8),
  'MOVE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
	`, eventID, tenantID, normalizedOrgNodeKey, effectiveDate, []byte(payload), eventID, initiatorUUID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgUnitPGStore) DisableNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgNodeKey string) error {
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

	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
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
  $3::char(8),
  'DISABLE',
  $4::date,
  '{}'::jsonb,
  $5::text,
  $6::uuid
)
	`, eventID, tenantID, normalizedOrgNodeKey, effectiveDate, eventID, initiatorUUID); err != nil {
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
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return err
	}

	patch := `{"effective_date":` + strconv.Quote(newEffectiveDate) + `}`
	var correctionUUID string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event_correction(
  $1::uuid,
  $2::char(8),
  $3::date,
  $4::jsonb,
  $5::text,
  $6::uuid
)
	`, tenantID, orgNodeKey, targetEffectiveDate, []byte(patch), requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *orgUnitPGStore) SetBusinessUnitCurrent(ctx context.Context, tenantID string, effectiveDate string, orgNodeKey string, isBusinessUnit bool, requestID string) error {
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
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}
	initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
	if strings.TrimSpace(requestID) == "" {
		requestID = eventID
	}

	payload := `{"is_business_unit":` + strconv.FormatBool(isBusinessUnit) + `}`

	if _, err := tx.Exec(ctx, `SAVEPOINT sp_set_business_unit;`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
	SELECT orgunit.submit_org_event(
	  $1::uuid,
	  $2::uuid,
	  $3::char(8),
	  'SET_BUSINESS_UNIT',
	  $4::date,
  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
		`, eventID, tenantID, normalizedOrgNodeKey, effectiveDate, []byte(payload), requestID, initiatorUUID); err != nil {
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT sp_set_business_unit;`); rbErr != nil {
			return rbErr
		}
		dayConflict := strings.Contains(err.Error(), "EVENT_DATE_CONFLICT")
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr != nil && pgErr.Code == "23505" && pgErr.ConstraintName == "org_events_one_per_day_unique" {
			dayConflict = true
		}
		if dayConflict {
			var current bool
			if queryErr := tx.QueryRow(ctx, `
			SELECT is_business_unit
			FROM orgunit.org_unit_versions v
			WHERE v.tenant_uuid = $1::uuid
			  AND `+orgNodeKeyCompatExpr("v")+` = $2::text
			  AND v.status = 'active'
		  AND v.validity @> $3::date
		ORDER BY lower(validity) DESC
		LIMIT 1;
	`, tenantID, normalizedOrgNodeKey, effectiveDate).Scan(&current); queryErr == nil && current == isBusinessUnit {
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
	out := append([]OrgUnitNode(nil), s.nodes[tenantID]...)
	for i := range out {
		if err := hydrateOrgUnitNodeCompat(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *orgUnitMemoryStore) createNode(tenantID string, orgCode string, name string, isBusinessUnit bool) (OrgUnitNode, error) {
	normalizedCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return OrgUnitNode{}, err
	}
	id := s.nextID
	s.nextID++
	orgNodeKey, err := encodeOrgNodeKeyFromID(id)
	if err != nil {
		return OrgUnitNode{}, err
	}
	n := OrgUnitNode{
		OrgID:          id,
		ID:             orgNodeKey,
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

func (s *orgUnitMemoryStore) ResolveSetID(_ context.Context, _ string, orgNodeKey string, _ string) (string, error) {
	if _, err := normalizeOrgNodeKeyInput(orgNodeKey); err != nil {
		return "", err
	}
	return "S2601", nil
}

func (s *orgUnitMemoryStore) CreateNodeCurrent(_ context.Context, tenantID string, _ string, orgCode string, name string, _ string, isBusinessUnit bool) (OrgUnitNode, error) {
	return s.createNode(tenantID, orgCode, name, isBusinessUnit)
}

func (s *orgUnitMemoryStore) RenameNodeCurrent(_ context.Context, tenantID string, _ string, orgNodeKey string, newName string) error {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}
	if strings.TrimSpace(newName) == "" {
		return errors.New("new_name is required")
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == normalizedOrgNodeKey {
			nodes[i].Name = newName
			s.nodes[tenantID] = nodes
			return nil
		}
	}
	return errors.New("org_node_key not found")
}

func (s *orgUnitMemoryStore) MoveNodeCurrent(_ context.Context, tenantID string, _ string, orgNodeKey string, _ string) error {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == normalizedOrgNodeKey {
			return nil
		}
	}
	return errors.New("org_node_key not found")
}

func (s *orgUnitMemoryStore) DisableNodeCurrent(_ context.Context, tenantID string, _ string, orgNodeKey string) error {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == normalizedOrgNodeKey {
			s.nodes[tenantID] = append(nodes[:i], nodes[i+1:]...)
			return nil
		}
	}
	return errors.New("org_node_key not found")
}

func (s *orgUnitMemoryStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, _ string, orgNodeKey string, isBusinessUnit bool, _ string) error {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}

	nodes := s.nodes[tenantID]
	for i := range nodes {
		if nodes[i].ID == normalizedOrgNodeKey {
			nodes[i].IsBusinessUnit = isBusinessUnit
			s.nodes[tenantID] = nodes
			return nil
		}
	}
	return errors.New("org_node_key not found")
}

func orgUnitNodeStoredKey(node OrgUnitNode) (string, bool) {
	if key, err := normalizeOrgNodeKeyInput(node.ID); err == nil {
		return key, true
	}
	if node.OrgID > 0 {
		key, err := encodeOrgNodeKeyFromID(node.OrgID)
		if err == nil {
			return key, true
		}
	}
	return "", false
}

func (s *orgUnitMemoryStore) ResolveOrgNodeKeyByCode(_ context.Context, tenantID string, orgCode string) (string, error) {
	normalizedCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return "", err
	}
	for _, node := range s.nodes[tenantID] {
		if node.OrgCode == normalizedCode {
			if orgNodeKey, ok := orgUnitNodeStoredKey(node); ok {
				return orgNodeKey, nil
			}
			break
		}
	}
	return "", orgunitpkg.ErrOrgCodeNotFound
}

func (s *orgUnitMemoryStore) ResolveOrgCodeByNodeKey(_ context.Context, tenantID string, orgNodeKey string) (string, error) {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return "", err
	}
	for _, node := range s.nodes[tenantID] {
		storedKey, ok := orgUnitNodeStoredKey(node)
		if ok && storedKey == normalizedOrgNodeKey {
			return node.OrgCode, nil
		}
	}
	return "", orgunitpkg.ErrOrgNodeKeyNotFound
}

func (s *orgUnitMemoryStore) IsOrgTreeInitialized(_ context.Context, tenantID string) (bool, error) {
	return len(s.nodes[tenantID]) > 0, nil
}

func (s *orgUnitMemoryStore) ResolveAppendFacts(_ context.Context, tenantID string, orgNodeKey string, _ string) (orgUnitAppendFacts, error) {
	facts := orgUnitAppendFacts{
		TreeInitialized: len(s.nodes[tenantID]) > 0,
	}
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return orgUnitAppendFacts{}, err
	}
	for _, node := range s.nodes[tenantID] {
		if node.OrgID != orgID {
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

func (s *orgUnitMemoryStore) ResolveOrgCodesByNodeKeys(_ context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error) {
	out := make(map[string]string)
	if len(orgNodeKeys) == 0 {
		return out, nil
	}
	byKey := make(map[string]string)
	for _, node := range s.nodes[tenantID] {
		byKey[node.ID] = node.OrgCode
	}
	for _, orgNodeKey := range orgNodeKeys {
		code, ok := byKey[orgNodeKey]
		if !ok {
			return nil, orgunitpkg.ErrOrgNodeKeyNotFound
		}
		out[orgNodeKey] = code
	}
	return out, nil
}

func (s *orgUnitMemoryStore) ListChildren(_ context.Context, tenantID string, parentID int, _ string) ([]OrgUnitChild, error) {
	parentOrgNodeKey, err := encodeOrgNodeKeyFromID(parentID)
	if err != nil {
		return nil, err
	}
	return s.ListChildrenByNodeKey(context.Background(), tenantID, parentOrgNodeKey, "")
}

func (s *orgUnitMemoryStore) ListChildrenWithVisibility(ctx context.Context, tenantID string, parentID int, asOfDate string, _ bool) ([]OrgUnitChild, error) {
	parentOrgNodeKey, err := encodeOrgNodeKeyFromID(parentID)
	if err != nil {
		return nil, err
	}
	return s.ListChildrenWithVisibilityByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate, false)
}

func (s *orgUnitMemoryStore) ListChildrenByNodeKey(_ context.Context, tenantID string, parentOrgNodeKey string, _ string) ([]OrgUnitChild, error) {
	normalizedParentOrgNodeKey, err := normalizeOrgNodeKeyInput(parentOrgNodeKey)
	if err != nil {
		return nil, err
	}
	for _, node := range s.nodes[tenantID] {
		storedKey, ok := orgUnitNodeStoredKey(node)
		if ok && storedKey == normalizedParentOrgNodeKey {
			return []OrgUnitChild{}, nil
		}
	}
	return nil, errOrgUnitNotFound
}

func (s *orgUnitMemoryStore) ListChildrenWithVisibilityByNodeKey(ctx context.Context, tenantID string, parentOrgNodeKey string, asOfDate string, _ bool) ([]OrgUnitChild, error) {
	return s.ListChildrenByNodeKey(ctx, tenantID, parentOrgNodeKey, asOfDate)
}

func (s *orgUnitMemoryStore) GetNodeDetails(_ context.Context, tenantID string, orgID int, _ string) (OrgUnitNodeDetails, error) {
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return s.GetNodeDetailsByNodeKey(context.Background(), tenantID, orgNodeKey, "")
}

func (s *orgUnitMemoryStore) GetNodeDetailsByNodeKey(_ context.Context, tenantID string, orgNodeKey string, _ string) (OrgUnitNodeDetails, error) {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	for _, node := range s.nodes[tenantID] {
		storedKey, ok := orgUnitNodeStoredKey(node)
		if ok && storedKey == normalizedOrgNodeKey {
			details := OrgUnitNodeDetails{
				OrgNodeKey:      storedKey,
				OrgCode:         node.OrgCode,
				Name:            node.Name,
				Status:          strings.TrimSpace(node.Status),
				IsBusinessUnit:  node.IsBusinessUnit,
				PathOrgNodeKeys: []string{storedKey},
				FullNamePath:    node.Name,
				CreatedAt:       node.CreatedAt,
				UpdatedAt:       node.CreatedAt,
				EventUUID:       "",
			}
			if err := hydrateOrgUnitNodeDetailsCompat(&details); err != nil {
				return OrgUnitNodeDetails{}, err
			}
			return details, nil
		}
	}
	return OrgUnitNodeDetails{}, errOrgUnitNotFound
}

func (s *orgUnitMemoryStore) GetNodeDetailsWithVisibility(ctx context.Context, tenantID string, orgID int, asOfDate string, _ bool) (OrgUnitNodeDetails, error) {
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return OrgUnitNodeDetails{}, err
	}
	return s.GetNodeDetailsWithVisibilityByNodeKey(ctx, tenantID, orgNodeKey, asOfDate, false)
}

func (s *orgUnitMemoryStore) GetNodeDetailsWithVisibilityByNodeKey(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string, _ bool) (OrgUnitNodeDetails, error) {
	return s.GetNodeDetailsByNodeKey(ctx, tenantID, orgNodeKey, asOfDate)
}

func (s *orgUnitMemoryStore) SearchNode(_ context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return OrgUnitSearchResult{}, errors.New("query is required")
	}

	if normalized, err := orgunitpkg.NormalizeOrgCode(trimmed); err == nil {
		for _, node := range s.nodes[tenantID] {
			if node.OrgCode == normalized {
				orgNodeKey, ok := orgUnitNodeStoredKey(node)
				if !ok {
					continue
				}
				result := OrgUnitSearchResult{
					TargetOrgNodeKey: orgNodeKey,
					TargetOrgCode:    node.OrgCode,
					TargetName:       node.Name,
					PathOrgNodeKeys:  []string{orgNodeKey},
					TreeAsOf:         asOfDate,
				}
				if err := hydrateOrgUnitSearchResultCompat(&result); err != nil {
					return OrgUnitSearchResult{}, err
				}
				return result, nil
			}
		}
	}

	lower := strings.ToLower(trimmed)
	for _, node := range s.nodes[tenantID] {
		if strings.Contains(strings.ToLower(node.Name), lower) {
			orgNodeKey, ok := orgUnitNodeStoredKey(node)
			if !ok {
				continue
			}
			result := OrgUnitSearchResult{
				TargetOrgNodeKey: orgNodeKey,
				TargetOrgCode:    node.OrgCode,
				TargetName:       node.Name,
				PathOrgNodeKeys:  []string{orgNodeKey},
				TreeAsOf:         asOfDate,
			}
			if err := hydrateOrgUnitSearchResultCompat(&result); err != nil {
				return OrgUnitSearchResult{}, err
			}
			return result, nil
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
				orgNodeKey, ok := orgUnitNodeStoredKey(node)
				if !ok {
					continue
				}
				item := OrgUnitSearchCandidate{
					OrgNodeKey: orgNodeKey,
					OrgCode:    node.OrgCode,
					Name:       node.Name,
					Status:     strings.TrimSpace(node.Status),
				}
				if err := hydrateOrgUnitSearchCandidateCompat(&item); err != nil {
					return nil, err
				}
				return []OrgUnitSearchCandidate{item}, nil
			}
		}
	}

	lower := strings.ToLower(trimmed)
	var out []OrgUnitSearchCandidate
	for _, node := range s.nodes[tenantID] {
		if strings.Contains(strings.ToLower(node.Name), lower) {
			orgNodeKey, ok := orgUnitNodeStoredKey(node)
			if !ok {
				continue
			}
			item := OrgUnitSearchCandidate{
				OrgNodeKey: orgNodeKey,
				OrgCode:    node.OrgCode,
				Name:       node.Name,
				Status:     strings.TrimSpace(node.Status),
			}
			if err := hydrateOrgUnitSearchCandidateCompat(&item); err != nil {
				return nil, err
			}
			out = append(out, item)
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
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return nil, err
	}
	return s.ListNodeVersionsByNodeKey(context.Background(), tenantID, orgNodeKey)
}

func (s *orgUnitMemoryStore) ListNodeVersionsByNodeKey(_ context.Context, tenantID string, orgNodeKey string) ([]OrgUnitNodeVersion, error) {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return nil, err
	}
	for _, node := range s.nodes[tenantID] {
		storedKey, ok := orgUnitNodeStoredKey(node)
		if ok && storedKey == normalizedOrgNodeKey {
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
	orgNodeKey, err := encodeOrgNodeKeyFromID(orgID)
	if err != nil {
		return nil, err
	}
	return s.ListNodeAuditEventsByNodeKey(context.Background(), tenantID, orgNodeKey, limit)
}

func (s *orgUnitMemoryStore) ListNodeAuditEventsByNodeKey(_ context.Context, tenantID string, orgNodeKey string, limit int) ([]OrgUnitNodeAuditEvent, error) {
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return nil, err
	}
	for _, node := range s.nodes[tenantID] {
		storedKey, ok := orgUnitNodeStoredKey(node)
		if !ok || storedKey != normalizedOrgNodeKey {
			continue
		}
		if limit <= 0 {
			limit = orgNodeAuditPageSize
		}
		event := OrgUnitNodeAuditEvent{
			EventID:             1,
			EventUUID:           storedKey,
			OrgNodeKey:          storedKey,
			EventType:           "RENAME",
			EffectiveDate:       "2026-01-01",
			TxTime:              s.now(),
			InitiatorName:       "system",
			InitiatorEmployeeID: "system",
			RequestID:           "memory",
			Payload:             json.RawMessage(`{"op":"RENAME"}`),
		}
		if err := hydrateOrgUnitNodeAuditEventCompat(&event); err != nil {
			return nil, err
		}
		return []OrgUnitNodeAuditEvent{event}, nil
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
	return []orgUnitTenantFieldConfig{{
		FieldKey:         orgUnitCreateFieldOrgType,
		ValueType:        "text",
		DataSourceType:   "DICT",
		DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`),
	}}, nil
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

func (s *orgUnitMemoryStore) GetOrgUnitVersionExtSnapshotByNodeKey(_ context.Context, _ string, orgNodeKey string, _ string) (orgUnitVersionExtSnapshot, error) {
	if _, err := normalizeOrgNodeKeyInput(orgNodeKey); err != nil {
		return orgUnitVersionExtSnapshot{}, err
	}
	return orgUnitVersionExtSnapshot{
		VersionValues:  map[string]any{},
		VersionLabels:  map[string]string{},
		EventLabels:    map[string]string{},
		LastEventID:    0,
		HasVersionData: true,
	}, nil
}
