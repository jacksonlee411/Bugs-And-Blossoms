package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type orgUnitBusinessUnitAPIRequest struct {
	OrgUnitID      string `json:"org_unit_id"`
	EffectiveDate  string `json:"effective_date"`
	IsBusinessUnit bool   `json:"is_business_unit"`
	RequestID      string `json:"request_id"`
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
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.OrgUnitID == "" || req.EffectiveDate == "" || req.RequestID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_unit_id/effective_date/request_id required")
		return
	}
	if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
		return
	}

	if err := store.SetBusinessUnitCurrent(r.Context(), tenant.ID, req.EffectiveDate, req.OrgUnitID, req.IsBusinessUnit, req.RequestID); err != nil {
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
