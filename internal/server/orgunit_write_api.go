package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type orgUnitWritePatchAPIRequest struct {
	Name              *string         `json:"name"`
	ParentOrgCode     *string         `json:"parent_org_code"`
	Status            *string         `json:"status"`
	IsBusinessUnit    *bool           `json:"is_business_unit"`
	ManagerPernr      *string         `json:"manager_pernr"`
	Ext               map[string]any  `json:"ext"`
	ExtLabelsSnapshot json.RawMessage `json:"ext_labels_snapshot"`
}

type orgUnitWriteAPIRequest struct {
	Intent              string                      `json:"intent"`
	OrgCode             string                      `json:"org_code"`
	EffectiveDate       string                      `json:"effective_date"`
	TargetEffectiveDate string                      `json:"target_effective_date"`
	RequestCode         string                      `json:"request_code"`
	Patch               orgUnitWritePatchAPIRequest `json:"patch"`
}

type orgUnitWriteAPIResponse struct {
	OrgCode       string         `json:"org_code"`
	EffectiveDate string         `json:"effective_date"`
	EventType     string         `json:"event_type"`
	EventUUID     string         `json:"event_uuid"`
	Fields        map[string]any `json:"fields,omitempty"`
}

func handleOrgUnitsWriteAPI(w http.ResponseWriter, r *http.Request, writeSvc orgunitservices.OrgUnitWriteService) {
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

	var req orgUnitWriteAPIRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	if len(req.Patch.ExtLabelsSnapshot) > 0 {
		writeOrgUnitServiceError(w, r, newBadRequestError(orgUnitErrPatchFieldNotAllowed), "orgunit_write_failed")
		return
	}

	result, err := writeSvc.Write(r.Context(), tenant.ID, orgunitservices.WriteOrgUnitRequest{
		Intent:              strings.TrimSpace(req.Intent),
		OrgCode:             strings.TrimSpace(req.OrgCode),
		EffectiveDate:       strings.TrimSpace(req.EffectiveDate),
		TargetEffectiveDate: strings.TrimSpace(req.TargetEffectiveDate),
		RequestCode:         strings.TrimSpace(req.RequestCode),
		Patch: orgunitservices.OrgUnitWritePatch{
			Name:           req.Patch.Name,
			ParentOrgCode:  req.Patch.ParentOrgCode,
			Status:         req.Patch.Status,
			IsBusinessUnit: req.Patch.IsBusinessUnit,
			ManagerPernr:   req.Patch.ManagerPernr,
			Ext:            req.Patch.Ext,
		},
		InitiatorUUID: orgUnitInitiatorUUID(r.Context(), tenant.ID),
	})
	if err != nil {
		if errors.Is(err, errOrgUnitBadJSON) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		writeOrgUnitServiceError(w, r, err, "orgunit_write_failed")
		return
	}

	resp := orgUnitWriteAPIResponse{
		OrgCode:       result.OrgCode,
		EffectiveDate: result.EffectiveDate,
		EventType:     result.EventType,
		EventUUID:     result.EventUUID,
		Fields:        result.Fields,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
