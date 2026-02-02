package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
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
	req.OrgCode = strings.TrimSpace(req.OrgCode)
	req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
	req.RequestCode = strings.TrimSpace(req.RequestCode)
	if req.EffectiveDate == "" || req.RequestCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "effective_date/request_code required")
		return
	}
	if (req.OrgUnitID == "" && req.OrgCode == "") || (req.OrgUnitID != "" && req.OrgCode != "") {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_unit_id or org_code required")
		return
	}

	orgUnitID := req.OrgUnitID
	if req.OrgCode != "" {
		orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, req.OrgCode)
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
		orgUnitID = strconv.Itoa(orgID)
	}
	if _, err := parseOrgID8(orgUnitID); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
		return
	}

	if err := store.SetBusinessUnitCurrent(r.Context(), tenant.ID, req.EffectiveDate, orgUnitID, req.IsBusinessUnit, req.RequestCode); err != nil {
		writeInternalAPIError(w, r, err, "orgunit_set_business_unit_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"org_unit_id":      req.OrgUnitID,
		"is_business_unit": req.IsBusinessUnit,
	})
}
