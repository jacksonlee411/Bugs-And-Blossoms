package server

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type cubeBoxOrgUnitDetailsStore interface {
	OrgUnitStore
	orgUnitDetailsExtFieldStore
}

type cubeBoxOrgUnitDetailsExecutor struct {
	store cubeBoxOrgUnitDetailsStore
}

type cubeBoxOrgUnitListExecutor struct {
	store OrgUnitStore
}

type cubeBoxOrgUnitSearchExecutor struct {
	store OrgUnitStore
}

type cubeBoxOrgUnitAuditExecutor struct {
	store OrgUnitStore
}

const (
	cubeBoxOrgUnitListDefaultUserPage = 1
	cubeBoxOrgUnitListDefaultPageSize = 100
)

type orgUnitSearchAmbiguousError struct {
	Query      string
	Candidates []OrgUnitSearchCandidate
	AsOf       string
}

func (e *orgUnitSearchAmbiguousError) Error() string {
	return "org_unit_search_ambiguous"
}

func (e *orgUnitSearchAmbiguousError) QueryCandidates() []cubebox.QueryCandidate {
	if e == nil {
		return nil
	}
	items := make([]cubebox.QueryCandidate, 0, len(e.Candidates))
	for _, candidate := range e.Candidates {
		normalized := cubebox.NormalizeQueryCandidate(cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: candidate.OrgCode,
			Name:      candidate.Name,
			AsOf:      e.AsOf,
			Status:    candidate.Status,
		})
		if normalized == nil {
			continue
		}
		items = append(items, *normalized)
	}
	return items
}

func newCubeBoxOrgUnitRegisteredExecutors(store OrgUnitStore) ([]cubebox.RegisteredExecutor, error) {
	if store == nil {
		return nil, errors.New("orgunit store required")
	}
	items := make([]cubebox.RegisteredExecutor, 0, 4)
	if detailsStore, ok := store.(cubeBoxOrgUnitDetailsStore); ok {
		items = append(items, cubebox.RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			OptionalParams: []string{"include_disabled"},
			Executor: cubeBoxOrgUnitDetailsExecutor{
				store: detailsStore,
			},
		})
	}
	items = append(items,
		cubebox.RegisteredExecutor{
			ExecutorKey:    "orgunit.list",
			RequiredParams: []string{"as_of"},
			OptionalParams: []string{"include_disabled", "parent_org_code", "all_org_units", "keyword", "status", "is_business_unit", "page", "size"},
			Executor: cubeBoxOrgUnitListExecutor{
				store: store,
			},
		},
		cubebox.RegisteredExecutor{
			ExecutorKey:    "orgunit.search",
			RequiredParams: []string{"query", "as_of"},
			OptionalParams: []string{"include_disabled"},
			Executor: cubeBoxOrgUnitSearchExecutor{
				store: store,
			},
		},
		cubebox.RegisteredExecutor{
			ExecutorKey:    "orgunit.audit",
			RequiredParams: []string{"org_code"},
			OptionalParams: []string{"limit"},
			Executor: cubeBoxOrgUnitAuditExecutor{
				store: store,
			},
		},
	)
	return items, nil
}

func (e cubeBoxOrgUnitDetailsExecutor) ValidateParams(raw map[string]any) (map[string]any, error) {
	params, err := normalizeOrgUnitCommonParams(raw)
	if err != nil {
		return nil, err
	}
	orgCode, err := normalizeOptionalOrgCode(raw["org_code"], "org_code")
	if err != nil {
		return nil, err
	}
	params["org_code"] = orgCode
	return params, nil
}

func (e cubeBoxOrgUnitDetailsExecutor) Execute(ctx context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
	orgCode := strings.TrimSpace(params["org_code"].(string))
	asOf := strings.TrimSpace(params["as_of"].(string))
	includeDisabled := params["include_disabled"].(bool)

	orgNodeKey, err := e.store.ResolveOrgNodeKeyByCode(ctx, strings.TrimSpace(request.TenantID), orgCode)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}

	details, err := getNodeDetailsByVisibilityByNodeKey(ctx, e.store, strings.TrimSpace(request.TenantID), orgNodeKey, asOf, includeDisabled)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}

	detailsOrgNodeKey := strings.TrimSpace(details.OrgNodeKey)
	if detailsOrgNodeKey == "" {
		detailsOrgNodeKey = orgNodeKey
	}
	extFields, err := buildOrgUnitDetailsExtFieldsByNodeKey(ctx, e.store, strings.TrimSpace(request.TenantID), detailsOrgNodeKey, asOf)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	resp := orgUnitDetailsAPIResponse{
		AsOf:      asOf,
		ExtFields: []orgUnitExtFieldAPIItem{},
		OrgUnit: orgUnitDetailsAPIItem{
			OrgCode:        details.OrgCode,
			Name:           details.Name,
			Status:         strings.TrimSpace(details.Status),
			ParentOrgCode:  details.ParentCode,
			ParentName:     details.ParentName,
			IsBusinessUnit: details.IsBusinessUnit,
			ManagerPernr:   details.ManagerPernr,
			ManagerName:    details.ManagerName,
			FullNamePath:   details.FullNamePath,
			CreatedAt:      details.CreatedAt,
			UpdatedAt:      details.UpdatedAt,
			EventUUID:      details.EventUUID,
		},
	}
	resp.ExtFields = extFields
	if resp.ExtFields == nil {
		resp.ExtFields = []orgUnitExtFieldAPIItem{}
	}
	payload, err := marshalStructPayload(resp)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	return cubebox.ExecuteResult{
		Payload: payload,
		ConfirmedEntity: orgUnitQueryEntity(cubebox.QueryEntity{
			Intent:            strings.TrimSpace(request.PlanIntent),
			EntityKey:         details.OrgCode,
			AsOf:              asOf,
			SourceExecutorKey: "orgunit.details",
			ParentOrgCode:     details.ParentCode,
		}),
	}, nil
}

func (e cubeBoxOrgUnitListExecutor) ValidateParams(raw map[string]any) (map[string]any, error) {
	params, err := normalizeOrgUnitCommonParams(raw)
	if err != nil {
		return nil, err
	}
	if value, ok := raw["parent_org_code"]; ok && value != nil {
		parentOrgCode, present, err := normalizeMaybeOrgCode(value, "parent_org_code")
		if err != nil {
			return nil, err
		}
		if present {
			params["parent_org_code"] = parentOrgCode
		}
	}
	if value, ok := raw["all_org_units"]; ok && value != nil {
		allOrgUnits, ok := value.(bool)
		if !ok {
			return nil, newBadRequestError("all_org_units invalid")
		}
		params["all_org_units"] = allOrgUnits
	}
	if value, ok := raw["keyword"]; ok && value != nil {
		keyword, err := normalizeOptionalString(value)
		if err != nil {
			return nil, newBadRequestError("keyword invalid")
		}
		params["keyword"] = keyword
	}
	if value, ok := raw["status"]; ok && value != nil {
		status, err := normalizeOrgUnitListStatus(value)
		if err != nil {
			return nil, err
		}
		if status != "" {
			params["status"] = status
		}
	}
	if value, ok := raw["is_business_unit"]; ok && value != nil {
		isBusinessUnit, ok := value.(bool)
		if !ok {
			return nil, newBadRequestError("is_business_unit invalid")
		}
		params["is_business_unit"] = isBusinessUnit
	}
	if value, ok := raw["page"]; ok && value != nil {
		page, err := normalizeNonNegativeInt(value, "page")
		if err != nil {
			return nil, err
		}
		params["page"] = page
	}
	if value, ok := raw["size"]; ok && value != nil {
		size, err := normalizePositiveInt(value, "size")
		if err != nil {
			return nil, err
		}
		if size > orgUnitListMaxPageSize {
			return nil, newBadRequestError("size invalid")
		}
		params["size"] = size
	}
	return params, nil
}

func (e cubeBoxOrgUnitListExecutor) Execute(ctx context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
	asOf := strings.TrimSpace(params["as_of"].(string))
	includeDisabled := params["include_disabled"].(bool)

	pageReq := orgUnitListPageRequest{
		AsOf:            asOf,
		IncludeDisabled: includeDisabled,
	}
	if value, ok := params["all_org_units"]; ok {
		pageReq.AllOrgUnits = value.(bool)
	}

	if value, ok := params["parent_org_code"]; ok {
		parentOrgCode := strings.TrimSpace(value.(string))
		if parentOrgCode != "" {
			parentOrgNodeKey, err := e.store.ResolveOrgNodeKeyByCode(ctx, strings.TrimSpace(request.TenantID), parentOrgCode)
			if err != nil {
				return cubebox.ExecuteResult{}, err
			}
			pageReq.ParentOrgNodeKey = &parentOrgNodeKey
		}
	}
	if value, ok := params["keyword"]; ok {
		pageReq.Keyword = strings.TrimSpace(value.(string))
	}
	if value, ok := params["status"]; ok {
		pageReq.Status = strings.TrimSpace(value.(string))
	}
	if value, ok := params["is_business_unit"]; ok {
		isBusinessUnit := value.(bool)
		pageReq.IsBusinessUnit = &isBusinessUnit
	}
	pageValue, sizeValue := cubeBoxOrgUnitListPageControls(params)
	pageReq.Limit = sizeValue
	pageReq.Offset = orgUnitListOffsetFromUserPage(pageValue, pageReq.Limit)

	items, total, err := listOrgUnitListPage(ctx, e.store, strings.TrimSpace(request.TenantID), pageReq)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}

	resp := orgUnitListResponse{
		AsOf:            asOf,
		IncludeDisabled: includeDisabled,
		OrgUnits:        items,
		Page:            &pageValue,
		Size:            &sizeValue,
		Total:           &total,
	}
	if resp.OrgUnits == nil {
		resp.OrgUnits = []orgUnitListItem{}
	}
	payload, err := marshalStructPayload(resp)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	return cubebox.ExecuteResult{
		Payload: payload,
	}, nil
}

func cubeBoxOrgUnitListPageControls(params map[string]any) (int, int) {
	pageValue := cubeBoxOrgUnitListDefaultUserPage
	if page, ok := params["page"].(int); ok {
		pageValue = page
	}
	pageValue = orgUnitListNormalizeUserPage(pageValue)

	sizeValue := cubeBoxOrgUnitListDefaultPageSize
	if size, ok := params["size"].(int); ok {
		sizeValue = size
	}
	return pageValue, sizeValue
}

func orgUnitListNormalizeUserPage(page int) int {
	if page <= 0 {
		return cubeBoxOrgUnitListDefaultUserPage
	}
	return page
}

func orgUnitListOffsetFromUserPage(page int, limit int) int {
	if limit <= 0 {
		return 0
	}
	page = orgUnitListNormalizeUserPage(page)
	if page <= 1 {
		return 0
	}
	return (page - 1) * limit
}

func (e cubeBoxOrgUnitSearchExecutor) ValidateParams(raw map[string]any) (map[string]any, error) {
	params, err := normalizeOrgUnitCommonParams(raw)
	if err != nil {
		return nil, err
	}
	query, err := normalizeRequiredString(raw["query"], "query")
	if err != nil {
		return nil, err
	}
	params["query"] = query
	return params, nil
}

func (e cubeBoxOrgUnitSearchExecutor) Execute(ctx context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
	query := strings.TrimSpace(params["query"].(string))
	asOf := strings.TrimSpace(params["as_of"].(string))
	includeDisabled := params["include_disabled"].(bool)
	candidates, err := searchNodeCandidatesByVisibility(ctx, e.store, strings.TrimSpace(request.TenantID), query, asOf, 3, includeDisabled)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	if len(candidates) > 1 {
		return cubebox.ExecuteResult{}, &orgUnitSearchAmbiguousError{
			Query:      query,
			Candidates: append([]OrgUnitSearchCandidate(nil), candidates...),
			AsOf:       asOf,
		}
	}
	result, err := searchNodeByVisibility(ctx, e.store, strings.TrimSpace(request.TenantID), query, asOf, includeDisabled)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}

	pathOrgNodeKeys := append([]string(nil), result.PathOrgNodeKeys...)
	if len(pathOrgNodeKeys) > 0 {
		codes, err := e.store.ResolveOrgCodesByNodeKeys(ctx, strings.TrimSpace(request.TenantID), pathOrgNodeKeys)
		if err != nil {
			return cubebox.ExecuteResult{}, err
		}
		pathCodes := make([]string, 0, len(pathOrgNodeKeys))
		for _, orgNodeKey := range pathOrgNodeKeys {
			if code, ok := codes[orgNodeKey]; ok && strings.TrimSpace(code) != "" {
				pathCodes = append(pathCodes, code)
			}
		}
		result.PathOrgCodes = pathCodes
	}

	payload, err := marshalStructPayload(result)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	return cubebox.ExecuteResult{
		Payload: payload,
		ConfirmedEntity: orgUnitQueryEntity(cubebox.QueryEntity{
			Intent:            strings.TrimSpace(request.PlanIntent),
			EntityKey:         result.TargetOrgCode,
			AsOf:              asOf,
			SourceExecutorKey: "orgunit.search",
			TargetOrgCode:     result.TargetOrgCode,
		}),
	}, nil
}

func (e cubeBoxOrgUnitAuditExecutor) ValidateParams(raw map[string]any) (map[string]any, error) {
	orgCode, err := normalizeOptionalOrgCode(raw["org_code"], "org_code")
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"org_code": orgCode,
		"limit":    orgNodeAuditPageSize,
	}
	if value, ok := raw["limit"]; ok && value != nil {
		limit, err := normalizePositiveInt(value, "limit")
		if err != nil {
			return nil, err
		}
		params["limit"] = limit
	}
	return params, nil
}

func (e cubeBoxOrgUnitAuditExecutor) Execute(ctx context.Context, request cubebox.ExecuteRequest, params map[string]any) (cubebox.ExecuteResult, error) {
	orgCode := strings.TrimSpace(params["org_code"].(string))
	limit := params["limit"].(int)

	orgNodeKey, err := e.store.ResolveOrgNodeKeyByCode(ctx, strings.TrimSpace(request.TenantID), orgCode)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	rows, err := listNodeAuditEventsByNodeKey(ctx, e.store, strings.TrimSpace(request.TenantID), orgNodeKey, limit+1)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]orgUnitAuditAPIItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, orgUnitAuditAPIItem{
			EventID:              row.EventID,
			EventUUID:            row.EventUUID,
			EventType:            row.EventType,
			EffectiveDate:        row.EffectiveDate,
			TxTime:               row.TxTime,
			InitiatorName:        row.InitiatorName,
			InitiatorEmployeeID:  row.InitiatorEmployeeID,
			RequestID:            row.RequestID,
			Reason:               row.Reason,
			IsRescinded:          row.IsRescinded,
			RescindedByEventUUID: row.RescindedByEventUUID,
			RescindedByTxTime:    row.RescindedByTxTime,
			RescindedByRequestID: row.RescindedByRequestID,
			Payload:              row.Payload,
			BeforeSnapshot:       row.BeforeSnapshot,
			AfterSnapshot:        row.AfterSnapshot,
		})
	}

	resp := orgUnitAuditAPIResponse{
		OrgCode: orgCode,
		Limit:   limit,
		HasMore: hasMore,
		Events:  items,
	}
	if resp.Events == nil {
		resp.Events = []orgUnitAuditAPIItem{}
	}
	payload, err := marshalStructPayload(resp)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	return cubebox.ExecuteResult{
		Payload: payload,
		ConfirmedEntity: orgUnitQueryEntity(cubebox.QueryEntity{
			Intent:            strings.TrimSpace(request.PlanIntent),
			EntityKey:         orgCode,
			SourceExecutorKey: "orgunit.audit",
		}),
	}, nil
}

func orgUnitQueryEntity(entity cubebox.QueryEntity) *cubebox.QueryEntity {
	entity.Domain = "orgunit"
	return cubebox.NormalizeQueryEntity(entity)
}

func normalizeOrgUnitCommonParams(raw map[string]any) (map[string]any, error) {
	asOf, err := normalizeDayParam(raw["as_of"], "as_of")
	if err != nil {
		return nil, err
	}
	includeDisabled, err := normalizeOptionalBool(raw["include_disabled"])
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"as_of":            asOf,
		"include_disabled": includeDisabled,
	}, nil
}

func normalizeDayParam(value any, field string) (string, error) {
	text, err := normalizeRequiredString(value, field)
	if err != nil {
		return "", err
	}
	return parseRequiredDay(text, field)
}

func normalizeOptionalOrgCode(value any, field string) (string, error) {
	text, err := normalizeRequiredString(value, field)
	if err != nil {
		return "", err
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(text)
	if err != nil {
		return "", newBadRequestError(strings.ToLower(field) + " invalid")
	}
	return normalized, nil
}

func normalizeMaybeOrgCode(value any, field string) (string, bool, error) {
	text, err := normalizeOptionalString(value)
	if err != nil {
		return "", false, newBadRequestError(field + " invalid")
	}
	if text == "" {
		return "", false, nil
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(text)
	if err != nil {
		return "", false, newBadRequestError(field + " invalid")
	}
	return normalized, true, nil
}

func normalizeRequiredString(value any, field string) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", newBadRequestError(field + " required")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", newBadRequestError(field + " required")
	}
	return text, nil
}

func normalizeOptionalString(value any) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", errors.New("string expected")
	}
	return strings.TrimSpace(text), nil
}

func normalizeOptionalBool(value any) (bool, error) {
	if value == nil {
		return false, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return parseIncludeDisabled(typed), nil
	default:
		return false, newBadRequestError("include_disabled invalid")
	}
}

func normalizeNonNegativeInt(value any, field string) (int, error) {
	number, err := normalizeJSONInt(value, field)
	if err != nil {
		return 0, err
	}
	if number < 0 {
		return 0, newBadRequestError(field + " invalid")
	}
	return number, nil
}

func normalizePositiveInt(value any, field string) (int, error) {
	number, err := normalizeJSONInt(value, field)
	if err != nil {
		return 0, err
	}
	if number <= 0 {
		return 0, newBadRequestError(field + " invalid")
	}
	return number, nil
}

func normalizeJSONInt(value any, field string) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) {
			return 0, newBadRequestError(field + " invalid")
		}
		return int(typed), nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, newBadRequestError(field + " invalid")
		}
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return 0, newBadRequestError(field + " invalid")
		}
		return parsed, nil
	default:
		return 0, newBadRequestError(field + " invalid")
	}
}

func normalizeOrgUnitListStatus(value any) (string, error) {
	text, err := normalizeRequiredString(value, "status")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(text) {
	case orgUnitListStatusAll:
		return "", nil
	case orgUnitListStatusActive:
		return orgUnitListStatusActive, nil
	case orgUnitListStatusDisabled:
		return orgUnitListStatusDisabled, nil
	default:
		return "", newBadRequestError("status invalid")
	}
}

func marshalStructPayload(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}
