package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitBusinessUnitAPIRequest struct {
	OrgUnitID      string `json:"org_unit_id"`
	OrgCode        string `json:"org_code"`
	EffectiveDate  string `json:"effective_date"`
	IsBusinessUnit bool   `json:"is_business_unit"`
	RequestCode    string `json:"request_code"`
}

func handleOrgUnitsBusinessUnitAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
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
	IsBusinessUnit *bool  `json:"is_business_unit,omitempty"`
	HasChildren    *bool  `json:"has_children,omitempty"`
}

type orgUnitCreateAPIRequest struct {
	OrgCode        string `json:"org_code"`
	Name           string `json:"name"`
	EffectiveDate  string `json:"effective_date"`
	ParentOrgCode  string `json:"parent_org_code"`
	IsBusinessUnit bool   `json:"is_business_unit"`
	ManagerPernr   string `json:"manager_pernr"`
}

type orgUnitRenameAPIRequest struct {
	OrgCode       string `json:"org_code"`
	NewName       string `json:"new_name"`
	EffectiveDate string `json:"effective_date"`
}

type orgUnitMoveAPIRequest struct {
	OrgCode          string `json:"org_code"`
	NewParentOrgCode string `json:"new_parent_org_code"`
	EffectiveDate    string `json:"effective_date"`
}

type orgUnitDisableAPIRequest struct {
	OrgCode       string `json:"org_code"`
	EffectiveDate string `json:"effective_date"`
}

type orgUnitEnableAPIRequest struct {
	OrgCode       string `json:"org_code"`
	EffectiveDate string `json:"effective_date"`
}

type orgUnitCorrectionPatchRequest struct {
	EffectiveDate  *string `json:"effective_date"`
	Name           *string `json:"name"`
	ParentOrgCode  *string `json:"parent_org_code"`
	IsBusinessUnit *bool   `json:"is_business_unit"`
	ManagerPernr   *string `json:"manager_pernr"`
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
)

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

		parentCode := strings.TrimSpace(r.URL.Query().Get("parent_org_code"))
		if parentCode != "" {
			normalized, err := orgunitpkg.NormalizeOrgCode(parentCode)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
				return
			}
			parentID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalized)
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

			children, err := store.ListChildren(r.Context(), tenant.ID, parentID, asOf)
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
				items = append(items, orgUnitListItem{
					OrgCode:     child.OrgCode,
					Name:        child.Name,
					HasChildren: &hasChildren,
				})
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"as_of":     asOf,
				"org_units": items,
			})
			return
		}

		nodes, err := store.ListNodesCurrent(r.Context(), tenant.ID, asOf)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_list_failed")
			return
		}

		items := make([]orgUnitListItem, 0, len(nodes))
		for _, node := range nodes {
			isBU := node.IsBusinessUnit
			items = append(items, orgUnitListItem{
				OrgCode:        node.OrgCode,
				Name:           node.Name,
				IsBusinessUnit: &isBU,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"as_of":     asOf,
			"org_units": items,
		})
		return
	case http.MethodPost:
		if writeSvc == nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_service_missing", "orgunit service missing")
			return
		}
		var req orgUnitCreateAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
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

func handleOrgUnitsRenameAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_rename_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitRenameAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Rename(ctx, tenantID, orgunitservices.RenameOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			NewName:       req.NewName,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsMoveAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_move_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitMoveAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Move(ctx, tenantID, orgunitservices.MoveOrgUnitRequest{
			EffectiveDate:    req.EffectiveDate,
			OrgCode:          req.OrgCode,
			NewParentOrgCode: req.NewParentOrgCode,
			InitiatorUUID:    initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsDisableAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_disable_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitDisableAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Disable(ctx, tenantID, orgunitservices.DisableOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
			InitiatorUUID: initiatorUUID,
		})
		return req.OrgCode, req.EffectiveDate, err
	})
}

func handleOrgUnitsEnableAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
	handleOrgUnitWriteAction(w, r, writeSvc, "orgunit_enable_failed", func(ctx context.Context, tenantID string) (string, string, error) {
		var req orgUnitEnableAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", errOrgUnitBadJSON
		}
		req.EffectiveDate = orgUnitDefaultDate(req.EffectiveDate)
		initiatorUUID := orgUnitInitiatorUUID(ctx, tenantID)
		err := writeSvc.Enable(ctx, tenantID, orgunitservices.EnableOrgUnitRequest{
			EffectiveDate: req.EffectiveDate,
			OrgCode:       req.OrgCode,
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
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
	code := strings.TrimSpace(err.Error())
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

	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
}

func orgUnitAPIStatusForCode(code string) (int, bool) {
	switch code {
	case orgUnitErrCodeInvalid,
		orgUnitErrEffectiveDate,
		orgUnitErrPatchFieldNotAllowed,
		orgUnitErrPatchRequired,
		orgUnitErrManagerInvalid:
		return http.StatusBadRequest, true
	case orgUnitErrCodeNotFound,
		orgUnitErrParentNotFound,
		orgUnitErrEventNotFound,
		orgUnitErrManagerNotFound:
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
		orgUnitErrHighRiskReorderForbidden:
		return http.StatusConflict, true
	default:
		return 0, false
	}
}
