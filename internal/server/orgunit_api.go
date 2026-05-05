package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitBusinessUnitAPIRequest struct {
	OrgCode           string          `json:"org_code"`
	EffectiveDate     string          `json:"effective_date"`
	IsBusinessUnit    bool            `json:"is_business_unit"`
	RequestID         string          `json:"request_id"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

func handleOrgUnitsBusinessUnitAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_set_business_unit_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitBusinessUnitAPIRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", "", errOrgUnitBadJSON
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		if len(req.ExtLabelsSnapshot) > 0 {
			return "", "", newBadRequestError(orgUnitErrPatchFieldNotAllowed)
		}

		req.RequestID = strings.TrimSpace(req.RequestID)
		effectiveDate, err := parseRequiredDay(req.EffectiveDate, "effective_date")
		if err != nil {
			return "", "", err
		}
		req.EffectiveDate = effectiveDate
		if strings.TrimSpace(req.OrgCode) == "" {
			return "", "", newBadRequestError("org_code required")
		}
		if err := ensureCurrentPrincipalOrgCodeScopeAllows(ctx, scope.store, scope.runtime, tenantID, req.OrgCode, req.EffectiveDate); err != nil {
			return "", "", err
		}

		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err = writeSvc.SetBusinessUnit(ctx, tenantID, orgunitservices.SetBusinessUnitRequest{
			EffectiveDate:  req.EffectiveDate,
			OrgCode:        req.OrgCode,
			IsBusinessUnit: req.IsBusinessUnit,
			Ext:            req.Ext,
			InitiatorUUID:  initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

type orgUnitListItem struct {
	OrgCode            string   `json:"org_code"`
	OrgNodeKey         string   `json:"org_node_key,omitempty"`
	Name               string   `json:"name"`
	Status             string   `json:"status"`
	IsBusinessUnit     *bool    `json:"is_business_unit,omitempty"`
	HasChildren        *bool    `json:"has_children,omitempty"`
	HasVisibleChildren *bool    `json:"has_visible_children,omitempty"`
	PathOrgNodeKeys    []string `json:"-"`
}

type orgUnitListResponse struct {
	AsOf            string            `json:"as_of"`
	IncludeDisabled bool              `json:"include_disabled"`
	Page            *int              `json:"page,omitempty"`
	Size            *int              `json:"size,omitempty"`
	Total           *int              `json:"total,omitempty"`
	OrgUnits        []orgUnitListItem `json:"org_units"`
}

type orgUnitDetailsAPIItem struct {
	OrgCode          string    `json:"org_code"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	ParentOrgNodeKey string    `json:"parent_org_node_key"`
	ParentOrgCode    string    `json:"parent_org_code"`
	ParentName       string    `json:"parent_name"`
	IsBusinessUnit   bool      `json:"is_business_unit"`
	ManagerPernr     string    `json:"manager_pernr"`
	ManagerName      string    `json:"manager_name"`
	FullNamePath     string    `json:"full_name_path"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	EventUUID        string    `json:"event_uuid"`
}

type orgUnitExtFieldAPIItem struct {
	FieldKey           string  `json:"field_key"`
	LabelI18nKey       *string `json:"label_i18n_key"`
	Label              *string `json:"label,omitempty"`
	ValueType          string  `json:"value_type"`
	DataSourceType     string  `json:"data_source_type"`
	Value              any     `json:"value"`
	DisplayValue       *string `json:"display_value"`
	DisplayValueSource string  `json:"display_value_source"`
}

type orgUnitDetailsAPIResponse struct {
	AsOf      string                   `json:"as_of"`
	OrgUnit   orgUnitDetailsAPIItem    `json:"org_unit"`
	ExtFields []orgUnitExtFieldAPIItem `json:"ext_fields"`
}

type orgUnitVersionAPIItem struct {
	EventID       int64  `json:"event_id"`
	EventUUID     string `json:"event_uuid"`
	EffectiveDate string `json:"effective_date"`
	EventType     string `json:"event_type"`
}

type orgUnitVersionsAPIResponse struct {
	OrgCode  string                  `json:"org_code"`
	Versions []orgUnitVersionAPIItem `json:"versions"`
}

type orgUnitAuditAPIItem struct {
	EventID              int64           `json:"event_id"`
	EventUUID            string          `json:"event_uuid"`
	EventType            string          `json:"event_type"`
	EffectiveDate        string          `json:"effective_date"`
	TxTime               time.Time       `json:"tx_time"`
	InitiatorName        string          `json:"initiator_name"`
	InitiatorEmployeeID  string          `json:"initiator_employee_id"`
	RequestID            string          `json:"request_id"`
	Reason               string          `json:"reason"`
	IsRescinded          bool            `json:"is_rescinded"`
	RescindedByEventUUID string          `json:"rescinded_by_event_uuid"`
	RescindedByTxTime    time.Time       `json:"rescinded_by_tx_time"`
	RescindedByRequestID string          `json:"rescinded_by_request_id"`
	Payload              json.RawMessage `json:"payload"`
	BeforeSnapshot       json.RawMessage `json:"before_snapshot"`
	AfterSnapshot        json.RawMessage `json:"after_snapshot"`
}

type orgUnitAuditAPIResponse struct {
	OrgCode string                `json:"org_code"`
	Limit   int                   `json:"limit"`
	HasMore bool                  `json:"has_more"`
	Events  []orgUnitAuditAPIItem `json:"events"`
}

type orgUnitCreateAPIRequest struct {
	OrgCode           string          `json:"org_code"`
	Name              string          `json:"name"`
	EffectiveDate     string          `json:"effective_date"`
	ParentOrgCode     string          `json:"parent_org_code"`
	IsBusinessUnit    bool            `json:"is_business_unit"`
	ManagerPernr      string          `json:"manager_pernr"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitRenameAPIRequest struct {
	OrgCode           string          `json:"org_code"`
	NewName           string          `json:"new_name"`
	EffectiveDate     string          `json:"effective_date"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitMoveAPIRequest struct {
	OrgCode           string          `json:"org_code"`
	NewParentOrgCode  string          `json:"new_parent_org_code"`
	EffectiveDate     string          `json:"effective_date"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitDisableAPIRequest struct {
	OrgCode           string          `json:"org_code"`
	EffectiveDate     string          `json:"effective_date"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitEnableAPIRequest struct {
	OrgCode           string          `json:"org_code"`
	EffectiveDate     string          `json:"effective_date"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitCorrectionPatchRequest struct {
	EffectiveDate     *string         `json:"effective_date"`
	Name              *string         `json:"name"`
	ParentOrgCode     *string         `json:"parent_org_code"`
	IsBusinessUnit    *bool           `json:"is_business_unit"`
	ManagerPernr      *string         `json:"manager_pernr"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitCorrectionAPIRequest struct {
	OrgCode       string                        `json:"org_code"`
	EffectiveDate string                        `json:"effective_date"`
	Patch         orgUnitCorrectionPatchRequest `json:"patch"`
	RequestID     string                        `json:"request_id"`
}

type orgUnitStatusCorrectionAPIRequest struct {
	OrgCode       string `json:"org_code"`
	EffectiveDate string `json:"effective_date"`
	TargetStatus  string `json:"target_status"`
	RequestID     string `json:"request_id"`
}

type orgUnitRescindRecordAPIRequest struct {
	OrgCode       string `json:"org_code"`
	EffectiveDate string `json:"effective_date"`
	RequestID     string `json:"request_id"`
	Reason        string `json:"reason"`
}

type orgUnitRescindOrgAPIRequest struct {
	OrgCode   string `json:"org_code"`
	RequestID string `json:"request_id"`
	Reason    string `json:"reason"`
}

var errOrgUnitBadJSON = errors.New("orgunit_bad_json")

const (
	orgUnitErrCodeInvalid                 = "ORG_CODE_INVALID"
	orgUnitErrCodeNotFound                = "ORG_CODE_NOT_FOUND"
	orgUnitErrInvalidArgument             = "ORG_INVALID_ARGUMENT"
	orgUnitErrEffectiveDate               = "EFFECTIVE_DATE_INVALID"
	orgUnitErrPatchFieldNotAllowed        = "PATCH_FIELD_NOT_ALLOWED"
	orgUnitErrPatchRequired               = "PATCH_REQUIRED"
	orgUnitErrEventNotFound               = "ORG_EVENT_NOT_FOUND"
	orgUnitErrParentNotFound              = "PARENT_NOT_FOUND_AS_OF"
	orgUnitErrManagerInvalid              = "MANAGER_PERNR_INVALID"
	orgUnitErrManagerNotFound             = "MANAGER_PERNR_NOT_FOUND"
	orgUnitErrManagerInactive             = "MANAGER_PERNR_INACTIVE"
	orgUnitErrEffectiveOutOfRange         = "EFFECTIVE_DATE_OUT_OF_RANGE"
	orgUnitErrEventDateConflict           = "EVENT_DATE_CONFLICT"
	orgUnitErrRequestDuplicate            = "REQUEST_DUPLICATE"
	orgUnitErrEnableRequired              = "ORG_ENABLE_REQUIRED"
	orgUnitErrRequestIDConflict           = "ORG_REQUEST_ID_CONFLICT"
	orgUnitErrStatusCorrectionUnsupported = "ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET"
	orgUnitErrRootDeleteForbidden         = "ORG_ROOT_DELETE_FORBIDDEN"
	orgUnitErrHasChildrenCannotDelete     = "ORG_HAS_CHILDREN_CANNOT_DELETE"
	orgUnitErrHasDependenciesCannotDelete = "ORG_HAS_DEPENDENCIES_CANNOT_DELETE"
	orgUnitErrEventRescinded              = "ORG_EVENT_RESCINDED"
	orgUnitErrHighRiskReorderForbidden    = "ORG_HIGH_RISK_REORDER_FORBIDDEN"

	orgUnitErrFieldDefinitionNotFound            = "ORG_FIELD_DEFINITION_NOT_FOUND"
	orgUnitErrFieldConfigInvalidDataSourceConfig = "ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG"
	orgUnitErrFieldConfigAlreadyEnabled          = "ORG_FIELD_CONFIG_ALREADY_ENABLED"
	orgUnitErrFieldConfigSlotExhausted           = "ORG_FIELD_CONFIG_SLOT_EXHAUSTED"
	orgUnitErrFieldConfigNotFound                = "ORG_FIELD_CONFIG_NOT_FOUND"
	orgUnitErrFieldConfigDisabledOnInvalid       = "ORG_FIELD_CONFIG_DISABLED_ON_INVALID"
	orgUnitErrFieldOptionsFieldNotEnabled        = "ORG_FIELD_OPTIONS_FIELD_NOT_ENABLED_AS_OF"
	orgUnitErrFieldOptionsNotSupported           = "ORG_FIELD_OPTIONS_NOT_SUPPORTED"
	orgUnitErrExtQueryFieldNotAllowed            = "ORG_EXT_QUERY_FIELD_NOT_ALLOWED"
	orgUnitErrFieldNotMaintainable               = "FIELD_NOT_MAINTAINABLE"
	orgUnitErrDefaultRuleRequired                = "DEFAULT_RULE_REQUIRED"
	orgUnitErrDefaultRuleEvalFailed              = "DEFAULT_RULE_EVAL_FAILED"
	orgUnitErrFieldPolicyExprInvalid             = "FIELD_POLICY_EXPR_INVALID"
	orgUnitErrFieldOptionNotAllowed              = "FIELD_OPTION_NOT_ALLOWED"
	orgUnitErrFieldRequiredValueMissing          = "FIELD_REQUIRED_VALUE_MISSING"
	orgUnitErrFieldPolicyMissing                 = "policy_missing"
	orgUnitErrFieldPolicyConflict                = "policy_conflict_ambiguous"
	orgUnitErrOrgCodeExhausted                   = "ORG_CODE_EXHAUSTED"
	orgUnitErrOrgCodeConflict                    = "ORG_CODE_CONFLICT"
	orgUnitErrFieldPolicyScopeOverlap            = "FIELD_POLICY_SCOPE_OVERLAP"
	orgUnitErrFieldPolicyNotFound                = "ORG_FIELD_POLICY_NOT_FOUND"
)

const (
	orgUnitListModeGrid        = "grid"
	orgUnitListStatusAll       = "all"
	orgUnitListStatusActive    = "active"
	orgUnitListStatusInactive  = "inactive"
	orgUnitListStatusDisabled  = "disabled"
	orgUnitListSortCode        = "code"
	orgUnitListSortName        = "name"
	orgUnitListSortStatus      = "status"
	orgUnitListSortOrderAsc    = "asc"
	orgUnitListSortOrderDesc   = "desc"
	orgUnitListDefaultPage     = 0
	orgUnitListDefaultPageSize = 20
	orgUnitListMaxPageSize     = 200
)

type orgUnitListQueryOptions struct {
	GridMode           bool
	AllOrgUnits        bool
	IncludeDescendants *bool
	Keyword            string
	Status             string // "", "active", "disabled"
	IsBusinessUnit     *bool
	SortField          string
	ExtSortFieldKey    string
	SortOrder          string
	ExtFilterFieldKey  string
	ExtFilterValue     string
	Paginate           bool
	Page               int
	PageSize           int
}

func parseOrgUnitListQueryOptions(values url.Values) (orgUnitListQueryOptions, bool, error) {
	hasKey := func(key string) bool {
		_, ok := values[key]
		return ok
	}

	opts := orgUnitListQueryOptions{
		Page:     orgUnitListDefaultPage,
		PageSize: orgUnitListDefaultPageSize,
	}

	hasAny := false
	if strings.EqualFold(strings.TrimSpace(values.Get("mode")), orgUnitListModeGrid) {
		hasAny = true
		opts.GridMode = true
	}

	if hasKey("all_org_units") {
		hasAny = true
		allOrgUnits, err := parseOrgUnitListQueryBool(values.Get("all_org_units"), "all_org_units")
		if err != nil {
			return orgUnitListQueryOptions{}, false, err
		}
		opts.AllOrgUnits = allOrgUnits
	}

	if hasKey("include_descendants") {
		hasAny = true
		includeDescendants, err := parseOrgUnitListQueryBool(values.Get("include_descendants"), "include_descendants")
		if err != nil {
			return orgUnitListQueryOptions{}, false, err
		}
		opts.IncludeDescendants = &includeDescendants
	}

	if hasKey("q") {
		hasAny = true
		opts.Keyword = strings.TrimSpace(values.Get("q"))
	}

	if hasKey("status") {
		hasAny = true
		raw := strings.ToLower(strings.TrimSpace(values.Get("status")))
		switch raw {
		case "", orgUnitListStatusAll:
			opts.Status = ""
		case orgUnitListStatusActive:
			opts.Status = orgUnitListStatusActive
		case orgUnitListStatusInactive, orgUnitListStatusDisabled:
			opts.Status = orgUnitListStatusDisabled
		default:
			return orgUnitListQueryOptions{}, false, errors.New("status invalid")
		}
	}

	if hasKey("is_business_unit") {
		hasAny = true
		isBusinessUnit, err := parseOrgUnitListQueryBool(values.Get("is_business_unit"), "is_business_unit")
		if err != nil {
			return orgUnitListQueryOptions{}, false, err
		}
		opts.IsBusinessUnit = &isBusinessUnit
	}

	sortPresent := hasKey("sort")
	orderPresent := hasKey("order")

	if sortPresent {
		hasAny = true
		raw := strings.ToLower(strings.TrimSpace(values.Get("sort")))
		switch {
		case raw == orgUnitListSortCode || raw == orgUnitListSortName || raw == orgUnitListSortStatus:
			opts.SortField = raw
		case strings.HasPrefix(raw, "ext:"):
			opts.ExtSortFieldKey = strings.TrimSpace(strings.TrimPrefix(raw, "ext:"))
			if opts.ExtSortFieldKey == "" {
				return orgUnitListQueryOptions{}, false, errors.New("sort invalid")
			}
		case raw == "":
			return orgUnitListQueryOptions{}, false, errors.New("sort invalid")
		default:
			return orgUnitListQueryOptions{}, false, errors.New("sort invalid")
		}
		opts.SortOrder = orgUnitListSortOrderAsc
	}

	if orderPresent {
		hasAny = true
		if !sortPresent {
			return orgUnitListQueryOptions{}, false, errors.New("order requires sort")
		}
		raw := strings.ToLower(strings.TrimSpace(values.Get("order")))
		switch raw {
		case orgUnitListSortOrderAsc, orgUnitListSortOrderDesc:
			opts.SortOrder = raw
		case "":
			return orgUnitListQueryOptions{}, false, errors.New("order invalid")
		default:
			return orgUnitListQueryOptions{}, false, errors.New("order invalid")
		}
	}

	pagePresent := hasKey("page")
	sizePresent := hasKey("size")
	if pagePresent || sizePresent {
		hasAny = true
		opts.Paginate = true

		if pagePresent {
			raw := strings.TrimSpace(values.Get("page"))
			if raw == "" {
				return orgUnitListQueryOptions{}, false, errors.New("page invalid")
			}
			page, err := strconv.Atoi(raw)
			if err != nil || page < 0 {
				return orgUnitListQueryOptions{}, false, errors.New("page invalid")
			}
			opts.Page = page
		} else {
			opts.Page = orgUnitListDefaultPage
		}

		if sizePresent {
			raw := strings.TrimSpace(values.Get("size"))
			if raw == "" {
				return orgUnitListQueryOptions{}, false, errors.New("size invalid")
			}
			size, err := strconv.Atoi(raw)
			if err != nil || size <= 0 || size > orgUnitListMaxPageSize {
				return orgUnitListQueryOptions{}, false, errors.New("size invalid")
			}
			opts.PageSize = size
		} else {
			opts.PageSize = orgUnitListDefaultPageSize
		}
	}

	extFilterKeyPresent := hasKey("ext_filter_field_key")
	extFilterValuePresent := hasKey("ext_filter_value")
	if extFilterKeyPresent || extFilterValuePresent {
		hasAny = true
		if !extFilterKeyPresent || !extFilterValuePresent {
			return orgUnitListQueryOptions{}, false, errors.New("ext_filter requires ext_filter_field_key and ext_filter_value")
		}
		opts.ExtFilterFieldKey = strings.TrimSpace(values.Get("ext_filter_field_key"))
		opts.ExtFilterValue = strings.TrimSpace(values.Get("ext_filter_value"))
		if opts.ExtFilterFieldKey == "" {
			return orgUnitListQueryOptions{}, false, errors.New("ext_filter_field_key invalid")
		}
	}

	if (opts.ExtFilterFieldKey != "" || opts.ExtSortFieldKey != "") && !opts.GridMode && !opts.Paginate {
		return orgUnitListQueryOptions{}, false, errors.New("ext query requires mode=grid or pagination")
	}

	return opts, hasAny, nil
}

func parseOrgUnitListQueryBool(raw string, field string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, errors.New(field + " invalid")
	}
}

func orgNodeKeyFromCompat(orgNodeKey string, orgID int) (string, error) {
	if strings.TrimSpace(orgNodeKey) != "" {
		return normalizeOrgNodeKeyInput(orgNodeKey)
	}
	if orgID > 0 {
		return encodeOrgNodeKeyFromID(orgID)
	}
	return "", nil
}

func runtimeStoreFromVariadic(runtime []authzRuntimeStore) authzRuntimeStore {
	if len(runtime) == 0 {
		return nil
	}
	return runtime[0]
}

type orgUnitScopeDeps struct {
	store   OrgUnitStore
	runtime authzRuntimeStore
}

func orgUnitScopeDepsFromVariadic(deps []orgUnitScopeDeps) orgUnitScopeDeps {
	if len(deps) == 0 {
		return orgUnitScopeDeps{}
	}
	return deps[0]
}

func ensureCurrentPrincipalOrgNodeScopeAllows(ctx context.Context, store OrgUnitStore, runtime authzRuntimeStore, tenantID string, orgNodeKey string, asOf string) error {
	if runtime == nil {
		return nil
	}
	orgNodeKey = strings.TrimSpace(orgNodeKey)
	if orgNodeKey == "" {
		return nil
	}
	if store == nil {
		return errAuthzRuntimeUnavailable
	}
	scopeFilter, err := orgUnitReadScopeFilterFromRuntime(ctx, runtime, tenantID)
	if err != nil {
		return err
	}
	targetAsOf, err := orgUnitScopeCheckAsOf(ctx, store, tenantID, asOf)
	if err != nil {
		return err
	}
	readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: store})
	resolved, err := readSvc.Resolve(ctx, orgunitservices.OrgUnitResolveRequest{
		TenantID:        tenantID,
		AsOf:            targetAsOf,
		ScopeFilter:     scopeFilter,
		OrgNodeKeys:     []string{orgNodeKey},
		IncludeDisabled: true,
		Caller:          "orgunit.scope.write",
	})
	if err != nil {
		return err
	}
	if len(resolved) == 0 {
		return orgunitservices.ErrOrgUnitReadScopeForbidden
	}
	return nil
}

func ensureCurrentPrincipalOrgCodeScopeAllows(ctx context.Context, store OrgUnitStore, runtime authzRuntimeStore, tenantID string, orgCode string, asOf string) error {
	if runtime == nil {
		return nil
	}
	orgCode = strings.TrimSpace(orgCode)
	if orgCode == "" {
		return nil
	}
	if store == nil {
		return errAuthzRuntimeUnavailable
	}
	scopeFilter, err := orgUnitReadScopeFilterFromRuntime(ctx, runtime, tenantID)
	if err != nil {
		return err
	}
	targetAsOf, err := orgUnitScopeCheckAsOf(ctx, store, tenantID, asOf)
	if err != nil {
		return err
	}
	readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: store})
	resolved, err := readSvc.Resolve(ctx, orgunitservices.OrgUnitResolveRequest{
		TenantID:        tenantID,
		AsOf:            targetAsOf,
		ScopeFilter:     scopeFilter,
		OrgCodes:        []string{orgCode},
		IncludeDisabled: true,
		Caller:          "orgunit.scope.write",
	})
	if err != nil {
		return err
	}
	if len(resolved) == 0 {
		return orgunitservices.ErrOrgUnitReadScopeForbidden
	}
	return nil
}

func resolveOrgUnitReadNodeForCurrentPrincipal(ctx context.Context, store OrgUnitStore, runtime authzRuntimeStore, tenantID string, orgCode string, asOf string, includeDisabled bool, caller string) (orgunitservices.OrgUnitReadNode, error) {
	orgCode = strings.TrimSpace(orgCode)
	if orgCode == "" {
		return orgunitservices.OrgUnitReadNode{}, orgunitservices.ErrOrgUnitReadInvalidArgument
	}
	if store == nil {
		return orgunitservices.OrgUnitReadNode{}, errAuthzRuntimeUnavailable
	}
	scopeFilter, err := orgUnitReadScopeFilterFromRuntime(ctx, runtime, tenantID)
	if err != nil {
		return orgunitservices.OrgUnitReadNode{}, err
	}
	readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: store})
	resolved, err := readSvc.Resolve(ctx, orgunitservices.OrgUnitResolveRequest{
		TenantID:        tenantID,
		AsOf:            asOf,
		ScopeFilter:     scopeFilter,
		OrgCodes:        []string{orgCode},
		IncludeDisabled: includeDisabled,
		Caller:          caller,
	})
	if err != nil {
		return orgunitservices.OrgUnitReadNode{}, err
	}
	if len(resolved) == 0 {
		return orgunitservices.OrgUnitReadNode{}, orgunitservices.ErrOrgUnitReadScopeForbidden
	}
	return resolved[0], nil
}

func resolveOrgUnitHistoryTargetForCurrentPrincipal(ctx context.Context, store OrgUnitStore, runtime authzRuntimeStore, tenantID string, orgCode string) (string, string, error) {
	orgCode = strings.TrimSpace(orgCode)
	if orgCode == "" {
		return "", "", orgunitservices.ErrOrgUnitReadInvalidArgument
	}
	if store == nil {
		return "", "", errAuthzRuntimeUnavailable
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return "", "", err
	}
	orgNodeKey, err := store.ResolveOrgNodeKeyByCode(ctx, tenantID, normalized)
	if err != nil {
		return "", "", err
	}
	orgNodeKey, err = normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return "", "", err
	}
	if err := ensureCurrentPrincipalHistoryOrgNodeScopeAllows(ctx, store, runtime, tenantID, orgNodeKey); err != nil {
		return "", "", err
	}
	return normalized, orgNodeKey, nil
}

func ensureCurrentPrincipalHistoryOrgNodeScopeAllows(ctx context.Context, store OrgUnitStore, runtime authzRuntimeStore, tenantID string, orgNodeKey string) error {
	if runtime == nil {
		return nil
	}
	targetOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return err
	}
	if store == nil {
		return errAuthzRuntimeUnavailable
	}
	scopes, err := currentPrincipalOrgScopes(ctx, runtime, tenantID)
	if err != nil {
		return err
	}
	hasDescendantScope := false
	for _, scope := range scopes {
		boundOrgNodeKey, err := normalizeOrgNodeKeyInput(scope.OrgNodeKey)
		if err != nil {
			continue
		}
		if targetOrgNodeKey == boundOrgNodeKey {
			return nil
		}
		hasDescendantScope = hasDescendantScope || scope.IncludeDescendants
	}
	if !hasDescendantScope {
		return orgunitservices.ErrOrgUnitReadScopeForbidden
	}

	targetAsOf, err := orgUnitScopeCheckAsOf(ctx, store, tenantID, "")
	if err != nil {
		return err
	}
	_, pathOrgNodeKeys, err := orgUnitScopePathOrgNodeKeys(ctx, store, tenantID, targetOrgNodeKey, targetAsOf)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		if !scope.IncludeDescendants {
			continue
		}
		boundOrgNodeKey, err := normalizeOrgNodeKeyInput(scope.OrgNodeKey)
		if err != nil {
			continue
		}
		for _, pathOrgNodeKey := range pathOrgNodeKeys {
			if normalizedPathKey, err := normalizeOrgNodeKeyInput(pathOrgNodeKey); err == nil && normalizedPathKey == boundOrgNodeKey {
				return nil
			}
		}
	}
	return orgunitservices.ErrOrgUnitReadScopeForbidden
}

func orgUnitScopeCheckAsOf(ctx context.Context, store OrgUnitStore, tenantID string, asOf string) (string, error) {
	targetAsOf := strings.TrimSpace(asOf)
	if targetAsOf != "" {
		return targetAsOf, nil
	}
	maxAsOf, ok, err := store.MaxEffectiveDateOnOrBefore(ctx, tenantID, time.Now().UTC().Format(asOfLayout))
	if err != nil {
		return "", err
	}
	if ok && strings.TrimSpace(maxAsOf) != "" {
		return strings.TrimSpace(maxAsOf), nil
	}
	return time.Now().UTC().Format(asOfLayout), nil
}

func currentPrincipalOrgScopes(ctx context.Context, runtime authzRuntimeStore, tenantID string) ([]principalOrgScope, error) {
	if runtime == nil {
		return nil, nil
	}
	principal, ok := currentPrincipal(ctx)
	if !ok || strings.TrimSpace(principal.ID) == "" {
		return nil, errAuthzPrincipalMissing
	}
	return principalOrgScopes(ctx, runtime, tenantID, principal.ID)
}

func principalOrgScopes(ctx context.Context, runtime authzRuntimeStore, tenantID string, principalID string) ([]principalOrgScope, error) {
	if runtime == nil {
		return nil, nil
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, errAuthzPrincipalMissing
	}
	return runtime.OrgScopesForPrincipal(ctx, tenantID, principalID, authz.AuthzCapabilityKey(authz.ObjectOrgUnitOrgUnits, authz.ActionRead))
}

func writeOrgUnitScopeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errAuthzRuntimeUnavailable):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "authz_runtime_unavailable", "authz runtime unavailable")
	case errors.Is(err, errAuthzPrincipalMissing):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "principal_missing", "principal missing")
	case errors.Is(err, errAuthzOrgScopeRequired), errors.Is(err, errAuthzScopeForbidden), isOrgUnitReadAuthzError(err):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "authz_scope_forbidden", "authz scope forbidden")
	default:
		writeInternalAPIError(w, r, err, "authz_scope_check_failed")
	}
}

func isOrgUnitAuthzScopeError(err error) bool {
	return errors.Is(err, errAuthzRuntimeUnavailable) ||
		errors.Is(err, errAuthzPrincipalMissing) ||
		errors.Is(err, errAuthzOrgScopeRequired) ||
		errors.Is(err, errAuthzScopeForbidden) ||
		isOrgUnitReadAuthzError(err)
}

func isOrgUnitReadAuthzError(err error) bool {
	return errors.Is(err, orgunitservices.ErrOrgUnitReadScopeRequired) ||
		errors.Is(err, orgunitservices.ErrOrgUnitReadScopeForbidden)
}

func writeOrgUnitReadServiceError(w http.ResponseWriter, r *http.Request, err error, fallbackCode string) {
	switch {
	case isOrgUnitReadAuthzError(err):
		writeOrgUnitScopeError(w, r, errAuthzScopeForbidden)
	case errors.Is(err, orgunitservices.ErrOrgUnitReadExtQueryNotAllowed):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrExtQueryFieldNotAllowed, "ext query not allowed")
	case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
	case errors.Is(err, orgunitservices.ErrOrgUnitReadInvalidArgument):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid request")
	case errors.Is(err, orgunitservices.ErrOrgUnitReadNotFound), errors.Is(err, orgunitpkg.ErrOrgCodeNotFound), errors.Is(err, errOrgUnitNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
	case errors.Is(err, orgunitservices.ErrOrgUnitReadSafePathUnavailable):
		writeInternalAPIError(w, r, err, "orgunit_safe_path_unavailable")
	default:
		writeInternalAPIError(w, r, err, fallbackCode)
	}
}

func handleOrgUnitsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, writeSvc orgunitservices.OrgUnitWriteService, runtime ...authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		asOf, err := parseRequiredQueryDay(r, "as_of")
		if err != nil {
			writeInternalDayFieldError(w, r, err)
			return
		}
		q := r.URL.Query()
		includeDisabled := parseIncludeDisabled(q.Get("include_disabled"))

		listOpts, hasListOpts, err := parseOrgUnitListQueryOptions(q)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		parentCode := strings.TrimSpace(q.Get("parent_org_code"))

		if hasListOpts {
			readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: store})
			scopeFilter, err := orgUnitReadScopeFilterFromRuntime(r.Context(), runtimeStoreFromVariadic(runtime), tenant.ID)
			if err != nil {
				writeOrgUnitScopeError(w, r, err)
				return
			}

			var pagePtr *int
			var sizePtr *int
			var totalPtr *int
			limit := 0
			offset := 0
			if listOpts.Paginate {
				limit = listOpts.PageSize
				offset = listOpts.Page * listOpts.PageSize
				pagePtr = &listOpts.Page
				sizePtr = &listOpts.PageSize
			}

			nodes, total, err := readSvc.List(r.Context(), orgunitservices.OrgUnitListRequest{
				TenantID:           tenant.ID,
				AsOf:               asOf,
				ScopeFilter:        scopeFilter,
				ParentOrgCode:      parentCode,
				IncludeDescendants: listOpts.IncludeDescendants,
				AllOrgUnits:        listOpts.AllOrgUnits,
				Keyword:            listOpts.Keyword,
				Status:             listOpts.Status,
				IsBusinessUnit:     listOpts.IsBusinessUnit,
				SortField:          listOpts.SortField,
				ExtSortFieldKey:    listOpts.ExtSortFieldKey,
				SortOrder:          listOpts.SortOrder,
				ExtFilterFieldKey:  listOpts.ExtFilterFieldKey,
				ExtFilterValue:     listOpts.ExtFilterValue,
				IncludeDisabled:    includeDisabled,
				Limit:              limit,
				Offset:             offset,
				Caller:             "orgunit.http.list",
			})
			if err != nil {
				writeOrgUnitReadServiceError(w, r, err, "orgunit_list_failed")
				return
			}
			items := orgUnitListItemsFromReadNodes(nodes)

			if listOpts.Paginate {
				totalPtr = &total
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(orgUnitListResponse{
				AsOf:            asOf,
				IncludeDisabled: includeDisabled,
				Page:            pagePtr,
				Size:            sizePtr,
				Total:           totalPtr,
				OrgUnits:        items,
			})
			return
		}

		readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: store})
		scopeFilter, err := orgUnitReadScopeFilterFromRuntime(r.Context(), runtimeStoreFromVariadic(runtime), tenant.ID)
		if err != nil {
			writeOrgUnitScopeError(w, r, err)
			return
		}

		if parentCode != "" {
			normalizedParentCode, err := orgunitpkg.NormalizeOrgCode(parentCode)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
				return
			}
			children, err := readSvc.Children(r.Context(), orgunitservices.OrgUnitChildrenRequest{
				TenantID:        tenant.ID,
				AsOf:            asOf,
				ScopeFilter:     scopeFilter,
				ParentOrgCode:   normalizedParentCode,
				IncludeDisabled: includeDisabled,
				Caller:          "orgunit.http.children",
			})
			if err != nil {
				writeOrgUnitReadServiceError(w, r, err, "orgunit_list_children_failed")
				return
			}
			items := orgUnitListItemsFromReadNodes(children)

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(orgUnitListResponse{
				AsOf:            asOf,
				IncludeDisabled: includeDisabled,
				OrgUnits:        items,
			})
			return
		}

		roots, err := readSvc.VisibleRoots(r.Context(), orgunitservices.OrgUnitReadRequest{
			TenantID:        tenant.ID,
			AsOf:            asOf,
			ScopeFilter:     scopeFilter,
			IncludeDisabled: includeDisabled,
			Caller:          "orgunit.http.roots",
		})
		if err != nil {
			writeOrgUnitReadServiceError(w, r, err, "orgunit_list_failed")
			return
		}
		items := orgUnitListItemsFromReadNodes(roots)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(orgUnitListResponse{
			AsOf:            asOf,
			IncludeDisabled: includeDisabled,
			OrgUnits:        items,
		})
		return
	case http.MethodPost:
		if writeSvc == nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
			return
		}
		var req orgUnitCreateAPIRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if len(req.ExtLabelsSnapshot) > 0 {
			writeOrgUnitServiceError(w, r, newBadRequestError(orgUnitErrPatchFieldNotAllowed), "orgunit_create_failed")
			return
		}
		effectiveDate, err := parseRequiredDay(req.EffectiveDate, "effective_date")
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", err.Error())
			return
		}
		req.EffectiveDate = effectiveDate
		if strings.TrimSpace(req.ParentOrgCode) != "" {
			if err := ensureCurrentPrincipalOrgCodeScopeAllows(r.Context(), store, runtimeStoreFromVariadic(runtime), tenant.ID, req.ParentOrgCode, req.EffectiveDate); err != nil {
				writeOrgUnitScopeError(w, r, err)
				return
			}
		}

		result, err := writeSvc.Create(r.Context(), tenant.ID, orgunitservices.CreateOrgUnitRequest{
			EffectiveDate:  req.EffectiveDate,
			OrgCode:        req.OrgCode,
			Name:           req.Name,
			ParentOrgCode:  req.ParentOrgCode,
			IsBusinessUnit: req.IsBusinessUnit,
			ManagerPernr:   req.ManagerPernr,
			Ext:            req.Ext,
			InitiatorUUID:  orgUnitInitiatorUUID(r.Context(), tenant.ID),
		})
		if err != nil {
			writeOrgUnitServiceError(w, r, err, "orgunit_create_failed")
			return
		}

		writeOrgUnitResult(w, r, http.StatusCreated, result)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handleOrgUnitsDetailsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, runtime ...authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	asOf, err := parseRequiredQueryDay(r, "as_of")
	if err != nil {
		writeInternalDayFieldError(w, r, err)
		return
	}
	includeDisabled := parseIncludeDisabled(r.URL.Query().Get("include_disabled"))

	rawCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	if rawCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
		return
	}
	resolved, err := resolveOrgUnitReadNodeForCurrentPrincipal(r.Context(), store, runtimeStoreFromVariadic(runtime), tenant.ID, rawCode, asOf, includeDisabled, "orgunit.http.details")
	if err != nil {
		writeOrgUnitReadServiceError(w, r, err, "orgunit_details_failed")
		return
	}
	orgNodeKey := strings.TrimSpace(resolved.OrgNodeKey)

	details, err := getNodeDetailsByVisibilityByNodeKey(r.Context(), store, tenant.ID, orgNodeKey, asOf, includeDisabled)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		writeInternalAPIError(w, r, err, "orgunit_details_failed")
		return
	}

	resp := orgUnitDetailsAPIResponse{
		AsOf:      asOf,
		ExtFields: []orgUnitExtFieldAPIItem{},
		OrgUnit: orgUnitDetailsAPIItem{
			OrgCode:          details.OrgCode,
			Name:             details.Name,
			Status:           strings.TrimSpace(details.Status),
			ParentOrgNodeKey: strings.TrimSpace(details.ParentOrgNodeKey),
			ParentOrgCode:    details.ParentCode,
			ParentName:       details.ParentName,
			IsBusinessUnit:   details.IsBusinessUnit,
			ManagerPernr:     details.ManagerPernr,
			ManagerName:      details.ManagerName,
			FullNamePath:     details.FullNamePath,
			CreatedAt:        details.CreatedAt,
			UpdatedAt:        details.UpdatedAt,
			EventUUID:        details.EventUUID,
		},
	}

	extStore, ok := store.(orgUnitDetailsExtFieldStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}
	detailsOrgNodeKey := strings.TrimSpace(details.OrgNodeKey)
	if detailsOrgNodeKey == "" {
		detailsOrgNodeKey = orgNodeKey
	}
	extFields, err := buildOrgUnitDetailsExtFieldsByNodeKey(r.Context(), extStore, tenant.ID, detailsOrgNodeKey, asOf)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		writeInternalAPIError(w, r, err, "orgunit_details_ext_fields_failed")
		return
	}
	resp.ExtFields = extFields

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func handleOrgUnitsVersionsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, runtime ...authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	rawCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	if rawCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
		return
	}
	normalized, orgNodeKey, err := resolveOrgUnitHistoryTargetForCurrentPrincipal(r.Context(), store, runtimeStoreFromVariadic(runtime), tenant.ID, rawCode)
	if err != nil {
		writeOrgUnitReadServiceError(w, r, err, "orgunit_versions_failed")
		return
	}

	versions, err := listNodeVersionsByNodeKey(r.Context(), store, tenant.ID, orgNodeKey)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		writeInternalAPIError(w, r, err, "orgunit_versions_failed")
		return
	}

	items := make([]orgUnitVersionAPIItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, orgUnitVersionAPIItem{
			EventID:       v.EventID,
			EventUUID:     v.EventUUID,
			EffectiveDate: v.EffectiveDate,
			EventType:     v.EventType,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(orgUnitVersionsAPIResponse{
		OrgCode:  normalized,
		Versions: items,
	})
}

func handleOrgUnitsAuditAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, runtime ...authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	rawCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	if rawCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
		return
	}
	normalized, orgNodeKey, err := resolveOrgUnitHistoryTargetForCurrentPrincipal(r.Context(), store, runtimeStoreFromVariadic(runtime), tenant.ID, rawCode)
	if err != nil {
		writeOrgUnitReadServiceError(w, r, err, "orgunit_audit_failed")
		return
	}

	limit := orgNodeAuditLimitFromURL(r)
	rows, err := listNodeAuditEventsByNodeKey(r.Context(), store, tenant.ID, orgNodeKey, limit+1)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		writeInternalAPIError(w, r, err, "orgunit_audit_failed")
		return
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(orgUnitAuditAPIResponse{
		OrgCode: normalized,
		Limit:   limit,
		HasMore: hasMore,
		Events:  items,
	})
}

func handleOrgUnitsSearchAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, runtime ...authzRuntimeStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	asOf, err := parseRequiredQueryDay(r, "as_of")
	if err != nil {
		writeInternalDayFieldError(w, r, err)
		return
	}
	includeDisabled := parseIncludeDisabled(r.URL.Query().Get("include_disabled"))

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "query required")
		return
	}

	readSvc := orgunitservices.NewOrgUnitReadService(orgUnitReadStoreAdapter{store: store})
	scopeFilter, err := orgUnitReadScopeFilterFromRuntime(r.Context(), runtimeStoreFromVariadic(runtime), tenant.ID)
	if err != nil {
		writeOrgUnitScopeError(w, r, err)
		return
	}
	nodes, err := readSvc.Search(r.Context(), orgunitservices.OrgUnitSearchRequest{
		TenantID:        tenant.ID,
		AsOf:            asOf,
		ScopeFilter:     scopeFilter,
		Query:           query,
		IncludeDisabled: includeDisabled,
		Limit:           8,
		Caller:          "orgunit.http.search",
	})
	if err != nil {
		writeOrgUnitReadServiceError(w, r, err, "orgunit_search_failed")
		return
	}
	if len(nodes) > 1 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(orgUnitSearchCandidatesAPIResponse{
			Ambiguous:          true,
			ErrorCode:          "org_unit_search_ambiguous",
			CandidateSource:    "execution_error",
			CandidateCount:     len(nodes),
			CannotSilentSelect: true,
			Candidates:         orgUnitSearchCandidateAPIItemsFromReadNodes(nodes, asOf),
			TreeAsOf:           asOf,
		})
		return
	}
	if len(nodes) == 0 {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
		return
	}

	result := orgUnitSearchResultFromReadNode(nodes[0], asOf)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

type orgUnitSearchCandidateAPIItem struct {
	OrgCode string `json:"org_code"`
	Name    string `json:"name"`
	Status  string `json:"status,omitempty"`
	AsOf    string `json:"as_of,omitempty"`
}

type orgUnitSearchCandidatesAPIResponse struct {
	Ambiguous          bool                            `json:"ambiguous"`
	ErrorCode          string                          `json:"error_code"`
	CandidateSource    string                          `json:"candidate_source"`
	CandidateCount     int                             `json:"candidate_count"`
	CannotSilentSelect bool                            `json:"cannot_silent_select"`
	Candidates         []orgUnitSearchCandidateAPIItem `json:"candidates"`
	TreeAsOf           string                          `json:"tree_as_of"`
}

func orgUnitSearchCandidateAPIItemsFromReadNodes(nodes []orgunitservices.OrgUnitReadNode, asOf string) []orgUnitSearchCandidateAPIItem {
	items := make([]orgUnitSearchCandidateAPIItem, 0, len(nodes))
	for _, node := range nodes {
		orgCode := strings.TrimSpace(node.OrgCode)
		if orgCode == "" {
			continue
		}
		status := strings.TrimSpace(node.Status)
		if status == "" {
			status = orgUnitListStatusActive
		}
		items = append(items, orgUnitSearchCandidateAPIItem{
			OrgCode: orgCode,
			Name:    strings.TrimSpace(node.Name),
			Status:  status,
			AsOf:    strings.TrimSpace(asOf),
		})
	}
	return items
}

func handleOrgUnitsRenameAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_rename_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitRenameAPIRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		if len(req.ExtLabelsSnapshot) > 0 {
			return "", "", newBadRequestError(orgUnitErrPatchFieldNotAllowed)
		}
		effectiveDate, err := parseRequiredDay(req.EffectiveDate, "effective_date")
		if err != nil {
			return "", "", err
		}
		req.EffectiveDate = effectiveDate
		if err := ensureCurrentPrincipalOrgCodeScopeAllows(ctx, scope.store, scope.runtime, tenantID, req.OrgCode, req.EffectiveDate); err != nil {
			return "", "", err
		}
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err = writeSvc.Rename(ctx, tenantID, orgunitservices.RenameOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			NewName:       req.NewName,
			Ext:           req.Ext,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsMoveAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_move_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitMoveAPIRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		if len(req.ExtLabelsSnapshot) > 0 {
			return "", "", newBadRequestError(orgUnitErrPatchFieldNotAllowed)
		}
		effectiveDate, err := parseRequiredDay(req.EffectiveDate, "effective_date")
		if err != nil {
			return "", "", err
		}
		req.EffectiveDate = effectiveDate
		if err := ensureCurrentPrincipalOrgCodeScopeAllows(ctx, scope.store, scope.runtime, tenantID, req.OrgCode, req.EffectiveDate); err != nil {
			return "", "", err
		}
		if strings.TrimSpace(req.NewParentOrgCode) != "" {
			if err := ensureCurrentPrincipalOrgCodeScopeAllows(ctx, scope.store, scope.runtime, tenantID, req.NewParentOrgCode, req.EffectiveDate); err != nil {
				return "", "", err
			}
		}
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err = writeSvc.Move(ctx, tenantID, orgunitservices.MoveOrgUnitRequest{
			EffectiveDate:    req.EffectiveDate,
			OrgCode:          req.OrgCode,
			NewParentOrgCode: req.NewParentOrgCode,
			Ext:              req.Ext,
			InitiatorUUID:    initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsDisableAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_disable_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitDisableAPIRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		if len(req.ExtLabelsSnapshot) > 0 {
			return "", "", newBadRequestError(orgUnitErrPatchFieldNotAllowed)
		}
		effectiveDate, err := parseRequiredDay(req.EffectiveDate, "effective_date")
		if err != nil {
			return "", "", err
		}
		req.EffectiveDate = effectiveDate
		if err := ensureCurrentPrincipalOrgCodeScopeAllows(ctx, scope.store, scope.runtime, tenantID, req.OrgCode, req.EffectiveDate); err != nil {
			return "", "", err
		}
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err = writeSvc.Disable(ctx, tenantID, orgunitservices.DisableOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			Ext:           req.Ext,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsEnableAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_enable_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitEnableAPIRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		if len(req.ExtLabelsSnapshot) > 0 {
			return "", "", newBadRequestError(orgUnitErrPatchFieldNotAllowed)
		}
		effectiveDate, err := parseRequiredDay(req.EffectiveDate, "effective_date")
		if err != nil {
			return "", "", err
		}
		req.EffectiveDate = effectiveDate
		if err := ensureCurrentPrincipalOrgCodeScopeAllows(ctx, scope.store, scope.runtime, tenantID, req.OrgCode, req.EffectiveDate); err != nil {
			return "", "", err
		}
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err = writeSvc.Enable(ctx, tenantID, orgunitservices.EnableOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			Ext:           req.Ext,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsCorrectionsAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if writeSvc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
		return
	}

	var req orgUnitCorrectionAPIRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	// Fail-closed: clients must not submit ext_labels_snapshot; server generates it.
	if len(req.Patch.ExtLabelsSnapshot) > 0 {
		writeOrgUnitServiceError(w, r, newBadRequestError(orgUnitErrPatchFieldNotAllowed), "orgunit_correct_failed")
		return
	}
	if err := ensureCurrentPrincipalOrgCodeScopeAllows(r.Context(), scope.store, scope.runtime, tenant.ID, req.OrgCode, req.EffectiveDate); err != nil {
		writeOrgUnitScopeError(w, r, err)
		return
	}
	if req.Patch.ParentOrgCode != nil && strings.TrimSpace(*req.Patch.ParentOrgCode) != "" {
		if err := ensureCurrentPrincipalOrgCodeScopeAllows(r.Context(), scope.store, scope.runtime, tenant.ID, *req.Patch.ParentOrgCode, req.EffectiveDate); err != nil {
			writeOrgUnitScopeError(w, r, err)
			return
		}
	}

	result, err := writeSvc.Correct(r.Context(), tenant.ID, orgunitservices.CorrectOrgUnitRequest{
		OrgCode:             req.OrgCode,
		TargetEffectiveDate: req.EffectiveDate,
		RequestID:           req.RequestID,
		InitiatorUUID:       orgUnitInitiatorUUID(r.Context(), tenant.ID),
		Patch: orgunitservices.OrgUnitCorrectionPatch{
			EffectiveDate:  req.Patch.EffectiveDate,
			Name:           req.Patch.Name,
			ParentOrgCode:  req.Patch.ParentOrgCode,
			IsBusinessUnit: req.Patch.IsBusinessUnit,
			ManagerPernr:   req.Patch.ManagerPernr,
			Ext:            req.Patch.Ext,
		},
	})
	if err != nil {
		writeOrgUnitServiceError(w, r, err, "orgunit_correct_failed")
		return
	}

	writeOrgUnitResult(w, r, http.StatusOK, result)
}

func handleOrgUnitsStatusCorrectionsAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if writeSvc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
		return
	}

	var req orgUnitStatusCorrectionAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	if err := ensureCurrentPrincipalOrgCodeScopeAllows(r.Context(), scope.store, scope.runtime, tenant.ID, req.OrgCode, req.EffectiveDate); err != nil {
		writeOrgUnitScopeError(w, r, err)
		return
	}

	result, err := writeSvc.CorrectStatus(r.Context(), tenant.ID, orgunitservices.CorrectStatusOrgUnitRequest{
		OrgCode:             req.OrgCode,
		TargetEffectiveDate: req.EffectiveDate,
		TargetStatus:        req.TargetStatus,
		RequestID:           req.RequestID,
		InitiatorUUID:       orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeOrgUnitServiceError(w, r, err, "orgunit_correct_status_failed")
		return
	}

	writeOrgUnitResult(w, r, http.StatusOK, result)
}

func handleOrgUnitsRescindsAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if writeSvc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
		return
	}

	var req orgUnitRescindRecordAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	if err := ensureCurrentPrincipalOrgCodeScopeAllows(r.Context(), scope.store, scope.runtime, tenant.ID, req.OrgCode, req.EffectiveDate); err != nil {
		writeOrgUnitScopeError(w, r, err)
		return
	}

	result, err := writeSvc.RescindRecord(r.Context(), tenant.ID, orgunitservices.RescindRecordOrgUnitRequest{
		OrgCode:             req.OrgCode,
		TargetEffectiveDate: req.EffectiveDate,
		RequestID:           req.RequestID,
		Reason:              req.Reason,
		InitiatorUUID:       orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeOrgUnitServiceError(w, r, err, "orgunit_rescind_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"org_code":       result.OrgCode,
		"effective_date": result.EffectiveDate,
		"operation":      "RESCIND_EVENT",
		"request_id":     req.RequestID,
	})
}

func handleOrgUnitsRescindsOrgAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService, scopeDeps ...orgUnitScopeDeps) {
	scope := orgUnitScopeDepsFromVariadic(scopeDeps)
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if writeSvc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
		return
	}

	var req orgUnitRescindOrgAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	if err := ensureCurrentPrincipalOrgCodeScopeAllows(r.Context(), scope.store, scope.runtime, tenant.ID, req.OrgCode, ""); err != nil {
		writeOrgUnitScopeError(w, r, err)
		return
	}

	result, err := writeSvc.RescindOrg(r.Context(), tenant.ID, orgunitservices.RescindOrgUnitRequest{
		OrgCode:       req.OrgCode,
		RequestID:     req.RequestID,
		Reason:        req.Reason,
		InitiatorUUID: orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		writeOrgUnitServiceError(w, r, err, "orgunit_rescind_org_failed")
		return
	}

	rescindedEvents := 0
	if raw, ok := result.Fields["rescinded_events"]; ok {
		switch v := raw.(type) {
		case int:
			rescindedEvents = v
		case int32:
			rescindedEvents = int(v)
		case int64:
			rescindedEvents = int(v)
		case float64:
			rescindedEvents = int(v)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"org_code":         result.OrgCode,
		"operation":        "RESCIND_ORG",
		"request_id":       req.RequestID,
		"rescinded_events": rescindedEvents,
	})
}

func handleOrgUnitWriteAction(
	w http.ResponseWriter,
	r *http.Request,
	writeSvc orgunitservices.OrgUnitWriteService,
	defaultCode string,
	read func(ctx context.Context, tenantID string) (string, string, error),
) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if writeSvc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
		return
	}

	orgCode, effectiveDate, err := read(r.Context(), tenant.ID)
	if err != nil {
		if errors.Is(err, errOrgUnitBadJSON) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		if writeInternalDayFieldError(w, r, err) {
			return
		}
		if isOrgUnitAuthzScopeError(err) {
			writeOrgUnitScopeError(w, r, err)
			return
		}
		writeOrgUnitServiceError(w, r, err, defaultCode)
		return
	}

	normalizedCode := strings.TrimSpace(orgCode)
	if normalized, err := orgunitpkg.NormalizeOrgCode(normalizedCode); err == nil {
		normalizedCode = normalized
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"org_code":       normalizedCode,
		"effective_date": effectiveDate,
	})
}

func writeOrgUnitResult(w http.ResponseWriter, r *http.Request, status int, result orgunittypes.OrgUnitResult) {
	payload := map[string]any{
		"org_code":       result.OrgCode,
		"effective_date": result.EffectiveDate,
	}
	if len(result.Fields) > 0 {
		payload["fields"] = result.Fields
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeOrgUnitServiceError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	code := strings.TrimSpace(stablePgMessage(err))
	if code == "" {
		code = strings.TrimSpace(err.Error())
	}
	status, ok := orgUnitAPIStatusForCode(code)
	message := defaultCode

	if !ok {
		if isBadRequestError(err) || isPgInvalidInput(err) {
			status = http.StatusBadRequest
			if !isStableDBCode(code) {
				code = "invalid_request"
				message = err.Error()
			}
		} else if isStableDBCode(code) {
			status = http.StatusUnprocessableEntity
		} else {
			status = http.StatusInternalServerError
			code = defaultCode
		}
	}

	// Web 端目前优先展示 ErrorEnvelope.message；对已知稳定 code 提供可读提示，避免只看到 "orgunit_*_failed"。
	if message == defaultCode && isStableDBCode(code) {
		if mapped := orgNodeWriteErrorMessage(errors.New(code)); strings.TrimSpace(mapped) != "" && mapped != code {
			message = mapped
		}
	}

	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
}

func writeInternalAPIError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	code := strings.TrimSpace(defaultCode)
	if code == "" {
		code = "internal_error"
	}
	message := code
	if err != nil {
		if stable := strings.TrimSpace(stablePgMessage(err)); stable != "" {
			message = stable
		} else if raw := strings.TrimSpace(err.Error()); raw != "" {
			message = raw
		}
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, code, message)
}

func orgUnitAPIStatusForCode(code string) (int, bool) {
	switch code {
	case orgUnitErrCodeInvalid,
		orgUnitErrInvalidArgument,
		orgUnitErrEffectiveDate,
		orgUnitErrPatchFieldNotAllowed,
		orgUnitErrPatchRequired,
		orgUnitErrManagerInvalid,
		orgUnitErrExtQueryFieldNotAllowed,
		orgUnitErrFieldConfigInvalidDataSourceConfig,
		orgUnitErrFieldNotMaintainable,
		orgUnitErrDefaultRuleRequired,
		orgUnitErrDefaultRuleEvalFailed,
		orgUnitErrFieldPolicyExprInvalid,
		orgUnitErrFieldOptionNotAllowed,
		orgUnitErrFieldRequiredValueMissing:
		return http.StatusBadRequest, true
	case orgUnitErrCodeNotFound,
		orgUnitErrParentNotFound,
		orgUnitErrEventNotFound,
		orgUnitErrManagerNotFound,
		orgUnitErrFieldDefinitionNotFound,
		orgUnitErrFieldConfigNotFound,
		orgUnitErrFieldPolicyNotFound,
		orgUnitErrFieldOptionsFieldNotEnabled,
		orgUnitErrFieldOptionsNotSupported:
		return http.StatusNotFound, true
	case orgUnitErrManagerInactive,
		orgUnitErrEffectiveOutOfRange,
		orgUnitErrEventDateConflict,
		orgUnitErrRequestDuplicate,
		orgUnitErrEnableRequired,
		orgUnitErrRequestIDConflict,
		orgUnitErrStatusCorrectionUnsupported,
		orgUnitErrRootDeleteForbidden,
		orgUnitErrHasChildrenCannotDelete,
		orgUnitErrHasDependenciesCannotDelete,
		orgUnitErrEventRescinded,
		orgUnitErrHighRiskReorderForbidden,
		orgUnitErrFieldConfigAlreadyEnabled,
		orgUnitErrFieldConfigSlotExhausted,
		orgUnitErrFieldConfigDisabledOnInvalid,
		orgUnitErrOrgCodeExhausted,
		orgUnitErrOrgCodeConflict,
		orgUnitErrFieldPolicyScopeOverlap,
		orgUnitErrFieldPolicyConflict:
		return http.StatusConflict, true
	case orgUnitErrFieldPolicyMissing:
		return http.StatusUnprocessableEntity, true
	default:
		return 0, false
	}
}
