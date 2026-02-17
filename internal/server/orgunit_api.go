package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitBusinessUnitAPIRequest struct {
	OrgUnitID         string          `json:"org_unit_id"`
	OrgCode           string          `json:"org_code"`
	EffectiveDate     string          `json:"effective_date"`
	IsBusinessUnit    bool            `json:"is_business_unit"`
	RequestCode       string          `json:"request_code"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

func handleOrgUnitsBusinessUnitAPI(w http.ResponseWriter, r *http.Request, dep any) {
	if writeSvc, ok := dep.(orgunitservices.OrgUnitWriteService); ok {
		handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_set_business_unit_failed", func(ctx context.Context, tenantID string) (string, string, error) {
			var req orgUnitBusinessUnitAPIRequest
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				return "", "", errOrgUnitBadJSON
			}
			if len(req.ExtLabelsSnapshot) > 0 {
				return "", "", newBadRequestError(orgUnitErrPatchFieldNotAllowed)
			}

			req.OrgUnitID = strings.TrimSpace(req.OrgUnitID)
			req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
			req.RequestCode = strings.TrimSpace(req.RequestCode)
			if req.EffectiveDate == "" {
				return "", "", newBadRequestError("effective_date required")
			}
			if req.OrgUnitID != "" || strings.TrimSpace(req.OrgCode) == "" {
				return "", "", newBadRequestError("org_code required")
			}

			initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
			err := writeSvc.SetBusinessUnit(ctx, tenantID, orgunitservices.SetBusinessUnitRequest{
				EffectiveDate:  req.EffectiveDate,
				OrgCode:        req.OrgCode,
				IsBusinessUnit: req.IsBusinessUnit,
				Ext:            req.Ext,
				InitiatorUUID:  initiatorUUID,
			})
			return req.OrgCode, req.EffectiveDate, err
		})
		return
	}

	store, ok := dep.(OrgUnitStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
		return
	}
	handleOrgUnitsBusinessUnitAPIStoreLegacy(w, r, store)
}

func handleOrgUnitsBusinessUnitAPIStoreLegacy(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req orgUnitBusinessUnitAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.OrgUnitID = strings.TrimSpace(req.OrgUnitID)
	req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
	req.RequestCode = strings.TrimSpace(req.RequestCode)
	if req.EffectiveDate == "" || req.RequestCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "effective_date/request_code required")
		return
	}
	if req.OrgUnitID != "" || req.OrgCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
		return
	}

	normalizedCode, err := orgunitpkg.NormalizeOrgCode(req.OrgCode)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		return
	}

	orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalizedCode)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}
	orgUnitID := strconv.Itoa(orgID)

	if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
		return
	}

	if err := store.SetBusinessUnitCurrent(r.Context(), tenant.ID, req.EffectiveDate, orgUnitID, req.IsBusinessUnit, req.RequestCode); err != nil {
		writeInternalAPIError(w, r, err, "orgunit_set_business_unit_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"org_code":         normalizedCode,
		"effective_date":   req.EffectiveDate,
		"is_business_unit": req.IsBusinessUnit,
	})
}

type orgUnitListItem struct {
	OrgCode        string `json:"org_code"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	IsBusinessUnit *bool  `json:"is_business_unit,omitempty"`
	HasChildren    *bool  `json:"has_children,omitempty"`
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
	OrgID          int       `json:"org_id"`
	OrgCode        string    `json:"org_code"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	ParentOrgCode  string    `json:"parent_org_code"`
	ParentName     string    `json:"parent_name"`
	IsBusinessUnit bool      `json:"is_business_unit"`
	ManagerPernr   string    `json:"manager_pernr"`
	ManagerName    string    `json:"manager_name"`
	FullNamePath   string    `json:"full_name_path"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	EventUUID      string    `json:"event_uuid"`
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
	EventID                int64           `json:"event_id"`
	EventUUID              string          `json:"event_uuid"`
	EventType              string          `json:"event_type"`
	EffectiveDate          string          `json:"effective_date"`
	TxTime                 time.Time       `json:"tx_time"`
	InitiatorName          string          `json:"initiator_name"`
	InitiatorEmployeeID    string          `json:"initiator_employee_id"`
	RequestCode            string          `json:"request_code"`
	Reason                 string          `json:"reason"`
	IsRescinded            bool            `json:"is_rescinded"`
	RescindedByEventUUID   string          `json:"rescinded_by_event_uuid"`
	RescindedByTxTime      time.Time       `json:"rescinded_by_tx_time"`
	RescindedByRequestCode string          `json:"rescinded_by_request_code"`
	Payload                json.RawMessage `json:"payload"`
	BeforeSnapshot         json.RawMessage `json:"before_snapshot"`
	AfterSnapshot          json.RawMessage `json:"after_snapshot"`
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
	GridMode          bool
	Keyword           string
	Status            string // "", "active", "disabled"
	SortField         string
	ExtSortFieldKey   string
	SortOrder         string
	ExtFilterFieldKey string
	ExtFilterValue    string
	Paginate          bool
	Page              int
	PageSize          int
}

type orgUnitListPageRequest struct {
	AsOf              string
	IncludeDisabled   bool
	ParentID          *int
	Keyword           string
	Status            string // "", "active", "disabled"
	SortField         string
	ExtSortFieldKey   string
	SortOrder         string
	ExtFilterFieldKey string
	ExtFilterValue    string
	Limit             int
	Offset            int
}

type orgUnitListPageReader interface {
	ListOrgUnitsPage(ctx context.Context, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error)
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

func listOrgUnitListPage(ctx context.Context, store OrgUnitStore, tenantID string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
	if pager, ok := store.(orgUnitListPageReader); ok {
		return pager.ListOrgUnitsPage(ctx, tenantID, req)
	}

	var items []orgUnitListItem
	if req.ParentID != nil {
		children, err := listChildrenByVisibility(ctx, store, tenantID, *req.ParentID, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, 0, err
		}
		items = make([]orgUnitListItem, 0, len(children))
		for _, child := range children {
			isBU := child.IsBusinessUnit
			hasChildren := child.HasChildren
			status := strings.TrimSpace(child.Status)
			if status == "" {
				status = orgUnitListStatusActive
			}
			items = append(items, orgUnitListItem{
				OrgCode:        child.OrgCode,
				Name:           child.Name,
				Status:         status,
				IsBusinessUnit: &isBU,
				HasChildren:    &hasChildren,
			})
		}
	} else {
		nodes, err := listNodesCurrentByVisibility(ctx, store, tenantID, req.AsOf, req.IncludeDisabled)
		if err != nil {
			return nil, 0, err
		}
		items = make([]orgUnitListItem, 0, len(nodes))
		for _, node := range nodes {
			isBU := node.IsBusinessUnit
			status := strings.TrimSpace(node.Status)
			if status == "" {
				status = orgUnitListStatusActive
			}
			items = append(items, orgUnitListItem{
				OrgCode:        node.OrgCode,
				Name:           node.Name,
				Status:         status,
				IsBusinessUnit: &isBU,
			})
		}
	}

	items = filterOrgUnitListItems(items, req.Keyword, req.Status)
	if req.SortField != "" {
		sortOrgUnitListItems(items, req.SortField, req.SortOrder)
	}

	total := len(items)
	if req.Limit <= 0 {
		return items, total, nil
	}

	start := req.Offset
	if start < 0 {
		start = 0
	}
	if start >= total {
		return []orgUnitListItem{}, total, nil
	}

	end := start + req.Limit
	if end > total {
		end = total
	}

	return items[start:end], total, nil
}

func filterOrgUnitListItems(items []orgUnitListItem, keyword string, status string) []orgUnitListItem {
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))

	if normalizedKeyword == "" && normalizedStatus == "" {
		return items
	}

	out := make([]orgUnitListItem, 0, len(items))
	for _, item := range items {
		itemStatus := strings.ToLower(strings.TrimSpace(item.Status))
		if itemStatus == "" {
			itemStatus = orgUnitListStatusActive
		}

		if normalizedStatus != "" && itemStatus != normalizedStatus {
			continue
		}

		if normalizedKeyword != "" {
			code := strings.ToLower(item.OrgCode)
			name := strings.ToLower(item.Name)
			if !strings.Contains(code, normalizedKeyword) && !strings.Contains(name, normalizedKeyword) {
				continue
			}
		}

		out = append(out, item)
	}

	return out
}

func sortOrgUnitListItems(items []orgUnitListItem, sortField string, sortOrder string) {
	normalizedField := strings.ToLower(strings.TrimSpace(sortField))
	desc := strings.EqualFold(strings.TrimSpace(sortOrder), orgUnitListSortOrderDesc)

	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]

		var cmp int
		switch normalizedField {
		case orgUnitListSortName:
			cmp = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
		case orgUnitListSortStatus:
			cmp = strings.Compare(strings.ToLower(left.Status), strings.ToLower(right.Status))
		case orgUnitListSortCode:
			fallthrough
		default:
			cmp = strings.Compare(strings.ToLower(left.OrgCode), strings.ToLower(right.OrgCode))
		}

		// Stable tie-breaker.
		if cmp == 0 {
			cmp = strings.Compare(strings.ToLower(left.OrgCode), strings.ToLower(right.OrgCode))
		}

		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func handleOrgUnitsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore, writeSvc orgunitservices.OrgUnitWriteService) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		asOf, err := orgUnitAPIAsOf(r)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
			return
		}
		q := r.URL.Query()
		includeDisabled := parseIncludeDisabled(q.Get("include_disabled"))

		listOpts, hasListOpts, err := parseOrgUnitListQueryOptions(q)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}

		var parentID *int
		parentCode := strings.TrimSpace(q.Get("parent_org_code"))
		if parentCode != "" {
			normalized, err := orgunitpkg.NormalizeOrgCode(parentCode)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
				return
			}
			resolvedID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalized)
			if err != nil {
				switch {
				case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
				case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
				default:
					writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
				}
				return
			}
			parentID = &resolvedID
		}

		if hasListOpts {
			if listOpts.ExtFilterFieldKey != "" || listOpts.ExtSortFieldKey != "" {
				if _, ok := store.(orgUnitListPageReader); !ok {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrExtQueryFieldNotAllowed, "ext query not allowed")
					return
				}
			}

			req := orgUnitListPageRequest{
				AsOf:              asOf,
				IncludeDisabled:   includeDisabled,
				ParentID:          parentID,
				Keyword:           listOpts.Keyword,
				Status:            listOpts.Status,
				SortField:         listOpts.SortField,
				ExtSortFieldKey:   listOpts.ExtSortFieldKey,
				SortOrder:         listOpts.SortOrder,
				ExtFilterFieldKey: listOpts.ExtFilterFieldKey,
				ExtFilterValue:    listOpts.ExtFilterValue,
			}

			var pagePtr *int
			var sizePtr *int
			var totalPtr *int
			if listOpts.Paginate {
				req.Limit = listOpts.PageSize
				req.Offset = listOpts.Page * listOpts.PageSize
				pagePtr = &listOpts.Page
				sizePtr = &listOpts.PageSize
			}

			items, total, err := listOrgUnitListPage(r.Context(), store, tenant.ID, req)
			if err != nil {
				if errors.Is(err, errOrgUnitNotFound) {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
					return
				}
				if errors.Is(err, errOrgUnitExtQueryFieldNotAllowed) {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, orgUnitErrExtQueryFieldNotAllowed, "ext query not allowed")
					return
				}
				writeInternalAPIError(w, r, err, "orgunit_list_failed")
				return
			}

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

		// 兼容旧语义：roots/children 请求不带 server-mode 参数时返回全量列表。
		if parentID != nil {
			children, err := listChildrenByVisibility(r.Context(), store, tenant.ID, *parentID, asOf, includeDisabled)
			if err != nil {
				if errors.Is(err, errOrgUnitNotFound) {
					routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
					return
				}
				writeInternalAPIError(w, r, err, "orgunit_list_children_failed")
				return
			}

			items := make([]orgUnitListItem, 0, len(children))
			for _, child := range children {
				hasChildren := child.HasChildren
				isBU := child.IsBusinessUnit
				status := strings.TrimSpace(child.Status)
				if status == "" {
					status = "active"
				}
				items = append(items, orgUnitListItem{
					OrgCode:        child.OrgCode,
					Name:           child.Name,
					Status:         status,
					IsBusinessUnit: &isBU,
					HasChildren:    &hasChildren,
				})
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(orgUnitListResponse{
				AsOf:            asOf,
				IncludeDisabled: includeDisabled,
				OrgUnits:        items,
			})
			return
		}

		nodes, err := listNodesCurrentByVisibility(r.Context(), store, tenant.ID, asOf, includeDisabled)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_list_failed")
			return
		}

		items := make([]orgUnitListItem, 0, len(nodes))
		for _, node := range nodes {
			isBU := node.IsBusinessUnit
			status := strings.TrimSpace(node.Status)
			if status == "" {
				status = "active"
			}
			items = append(items, orgUnitListItem{
				OrgCode:        node.OrgCode,
				Name:           node.Name,
				Status:         status,
				IsBusinessUnit: &isBU,
			})
		}

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
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)

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

func handleOrgUnitsDetailsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	asOf, err := orgUnitAPIAsOf(r)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}
	includeDisabled := parseIncludeDisabled(r.URL.Query().Get("include_disabled"))

	rawCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	if rawCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
		return
	}
	normalized, err := orgunitpkg.NormalizeOrgCode(rawCode)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		return
	}

	orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalized)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}

	details, err := getNodeDetailsByVisibility(r.Context(), store, tenant.ID, orgID, asOf, includeDisabled)
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
			OrgID:          details.OrgID,
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

	extStore, ok := store.(orgUnitDetailsExtFieldStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}
	extFields, err := buildOrgUnitDetailsExtFields(r.Context(), extStore, tenant.ID, orgID, asOf)
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

func handleOrgUnitsVersionsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
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
	normalized, err := orgunitpkg.NormalizeOrgCode(rawCode)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		return
	}

	orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalized)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}

	versions, err := store.ListNodeVersions(r.Context(), tenant.ID, orgID)
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

func handleOrgUnitsAuditAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
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
	normalized, err := orgunitpkg.NormalizeOrgCode(rawCode)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		return
	}

	orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalized)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}

	limit := orgNodeAuditLimitFromURL(r)
	rows, err := listNodeAuditEvents(r.Context(), store, tenant.ID, orgID, limit+1)
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
			EventID:                row.EventID,
			EventUUID:              row.EventUUID,
			EventType:              row.EventType,
			EffectiveDate:          row.EffectiveDate,
			TxTime:                 row.TxTime,
			InitiatorName:          row.InitiatorName,
			InitiatorEmployeeID:    row.InitiatorEmployeeID,
			RequestCode:            row.RequestCode,
			Reason:                 row.Reason,
			IsRescinded:            row.IsRescinded,
			RescindedByEventUUID:   row.RescindedByEventUUID,
			RescindedByTxTime:      row.RescindedByTxTime,
			RescindedByRequestCode: row.RescindedByRequestCode,
			Payload:                row.Payload,
			BeforeSnapshot:         row.BeforeSnapshot,
			AfterSnapshot:          row.AfterSnapshot,
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

func handleOrgUnitsSearchAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	asOf, err := orgUnitAPIAsOf(r)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}
	includeDisabled := parseIncludeDisabled(r.URL.Query().Get("include_disabled"))

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "query required")
		return
	}

	result, err := searchNodeByVisibility(r.Context(), store, tenant.ID, query, asOf, includeDisabled)
	if err != nil {
		if errors.Is(err, errOrgUnitNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_unit_not_found", "org unit not found")
			return
		}
		writeInternalAPIError(w, r, err, "orgunit_search_failed")
		return
	}

	if len(result.PathOrgIDs) > 0 {
		codes, err := store.ResolveOrgCodes(r.Context(), tenant.ID, result.PathOrgIDs)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_codes_failed")
			return
		}
		pathCodes := make([]string, 0, len(result.PathOrgIDs))
		for _, id := range result.PathOrgIDs {
			if code, ok := codes[id]; ok && strings.TrimSpace(code) != "" {
				pathCodes = append(pathCodes, code)
			}
		}
		result.PathOrgCodes = pathCodes
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func handleOrgUnitsRenameAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Rename(ctx, tenantID, orgunitservices.RenameOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			NewName:       req.NewName,
			Ext:           req.Ext,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsMoveAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Move(ctx, tenantID, orgunitservices.MoveOrgUnitRequest{
			EffectiveDate:    req.EffectiveDate,
			OrgCode:          req.OrgCode,
			NewParentOrgCode: req.NewParentOrgCode,
			Ext:              req.Ext,
			InitiatorUUID:    initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsDisableAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Disable(ctx, tenantID, orgunitservices.DisableOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			Ext:           req.Ext,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsEnableAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Enable(ctx, tenantID, orgunitservices.EnableOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			Ext:           req.Ext,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsCorrectionsAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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

func handleOrgUnitsStatusCorrectionsAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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

func handleOrgUnitsRescindsAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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

func handleOrgUnitsRescindsOrgAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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

func orgUnitAPIAsOf(r *http.Request) (string, error) {
	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		return "", err
	}
	return asOf, nil
}

func orgUnitDefaultDate(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Now().UTC().Format("2006-01-02")
	}
	return value
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

func orgUnitAPIStatusForCode(code string) (int, bool) {
	switch code {
	case orgUnitErrCodeInvalid,
		orgUnitErrInvalidArgument,
		orgUnitErrEffectiveDate,
		orgUnitErrPatchFieldNotAllowed,
		orgUnitErrPatchRequired,
		orgUnitErrManagerInvalid,
		orgUnitErrExtQueryFieldNotAllowed,
		orgUnitErrFieldConfigInvalidDataSourceConfig:
		return http.StatusBadRequest, true
	case orgUnitErrCodeNotFound,
		orgUnitErrParentNotFound,
		orgUnitErrEventNotFound,
		orgUnitErrManagerNotFound,
		orgUnitErrFieldDefinitionNotFound,
		orgUnitErrFieldConfigNotFound,
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
		orgUnitErrFieldConfigDisabledOnInvalid:
		return http.StatusConflict, true
	default:
		return 0, false
	}
}
