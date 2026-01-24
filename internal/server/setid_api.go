package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type setidCreateAPIRequest struct {
	SetID     string `json:"setid"`
	Name      string `json:"name"`
	RequestID string `json:"request_id"`
}

func handleSetIDsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req setidCreateAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.SetID = strings.TrimSpace(req.SetID)
	req.Name = strings.TrimSpace(req.Name)
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.SetID == "" || req.Name == "" || req.RequestID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "setid/name/request_id required")
		return
	}

	if err := store.EnsureBootstrap(r.Context(), tenant.ID, tenant.ID); err != nil {
		writeInternalAPIError(w, r, err, "bootstrap_failed")
		return
	}

	if err := store.CreateSetID(r.Context(), tenant.ID, req.SetID, req.Name, req.RequestID, tenant.ID); err != nil {
		writeInternalAPIError(w, r, err, "setid_create_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"setid":  strings.ToUpper(req.SetID),
		"status": "active",
	})
}

type setidBindingAPIRequest struct {
	OrgUnitID     string `json:"org_unit_id"`
	SetID         string `json:"setid"`
	EffectiveDate string `json:"effective_date"`
	RequestID     string `json:"request_id"`
}

func handleSetIDBindingsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req setidBindingAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.OrgUnitID = strings.TrimSpace(req.OrgUnitID)
	req.SetID = strings.TrimSpace(req.SetID)
	req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.OrgUnitID == "" || req.SetID == "" || req.EffectiveDate == "" || req.RequestID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_unit_id/setid/effective_date/request_id required")
		return
	}
	if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
		return
	}

	if err := store.BindSetID(r.Context(), tenant.ID, req.OrgUnitID, req.EffectiveDate, req.SetID, req.RequestID, tenant.ID); err != nil {
		writeInternalAPIError(w, r, err, "setid_binding_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"org_unit_id":    req.OrgUnitID,
		"setid":          strings.ToUpper(req.SetID),
		"effective_date": req.EffectiveDate,
	})
}

type globalSetIDAPIRequest struct {
	Name      string `json:"name"`
	RequestID string `json:"request_id"`
}

func handleGlobalSetIDsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	var req globalSetIDAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.Name == "" || req.RequestID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "name/request_id required")
		return
	}

	actorScope := strings.TrimSpace(r.Header.Get("X-Actor-Scope"))
	if actorScope == "" {
		actorScope = strings.TrimSpace(r.Header.Get("x-actor-scope"))
	}
	if strings.ToLower(actorScope) != "saas" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "actor_scope_forbidden", "actor scope forbidden")
		return
	}

	if err := store.CreateGlobalSetID(r.Context(), req.Name, req.RequestID, tenant.ID, actorScope); err != nil {
		writeInternalAPIError(w, r, err, "global_setid_create_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"setid":  "SHARE",
		"status": "active",
	})
}

func writeInternalAPIError(w http.ResponseWriter, r *http.Request, err error, defaultCode string) {
	code := stablePgMessage(err)
	status := http.StatusInternalServerError
	if isStableDBCode(code) {
		status = http.StatusUnprocessableEntity
	}
	if isBadRequestError(err) || isPgInvalidInput(err) {
		status = http.StatusBadRequest
	}
	if code == "" || code == "UNKNOWN" {
		code = defaultCode
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, defaultCode)
}
