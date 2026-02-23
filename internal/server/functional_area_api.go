package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type functionalAreaStateItem struct {
	FunctionalAreaKey string `json:"functional_area_key"`
	LifecycleStatus   string `json:"lifecycle_status"`
	Enabled           bool   `json:"enabled"`
}

type functionalAreaStateResponse struct {
	TenantID string                    `json:"tenant_id"`
	Items    []functionalAreaStateItem `json:"items"`
}

type functionalAreaSwitchRequest struct {
	FunctionalAreaKey string `json:"functional_area_key"`
	Enabled           bool   `json:"enabled"`
	Operator          string `json:"operator"`
}

func handleInternalFunctionalAreaStateAPI(w http.ResponseWriter, r *http.Request) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !canViewSetIDFullExplain(r.Context()) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, scopeReasonActorScopeForbidden, "actor scope forbidden")
		return
	}

	keys := make([]string, 0, len(functionalAreaLifecycleByKey))
	for key := range functionalAreaLifecycleByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]functionalAreaStateItem, 0, len(keys))
	for _, key := range keys {
		lifecycle := strings.TrimSpace(functionalAreaLifecycleByKey[key])
		items = append(items, functionalAreaStateItem{
			FunctionalAreaKey: key,
			LifecycleStatus:   lifecycle,
			Enabled:           defaultFunctionalAreaSwitchStore.isEnabled(tenant.ID, key),
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(functionalAreaStateResponse{
		TenantID: tenant.ID,
		Items:    items,
	})
}

func handleInternalFunctionalAreaSwitchAPI(w http.ResponseWriter, r *http.Request) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !canViewSetIDFullExplain(r.Context()) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, scopeReasonActorScopeForbidden, "actor scope forbidden")
		return
	}

	var req functionalAreaSwitchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.FunctionalAreaKey = strings.TrimSpace(strings.ToLower(req.FunctionalAreaKey))
	req.Operator = strings.TrimSpace(req.Operator)
	if req.FunctionalAreaKey == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "functional_area_key required")
		return
	}
	lifecycle, ok := functionalAreaLifecycleByKey[req.FunctionalAreaKey]
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, functionalAreaMissingCode, functionalAreaErrorMessage(functionalAreaMissingCode))
		return
	}
	lifecycle = strings.TrimSpace(lifecycle)
	if lifecycle != functionalAreaLifecycleActive {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, functionalAreaNotActiveCode, functionalAreaErrorMessage(functionalAreaNotActiveCode))
		return
	}

	defaultFunctionalAreaSwitchStore.setEnabled(tenant.ID, req.FunctionalAreaKey, req.Enabled)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(functionalAreaStateItem{
		FunctionalAreaKey: req.FunctionalAreaKey,
		LifecycleStatus:   lifecycle,
		Enabled:           defaultFunctionalAreaSwitchStore.isEnabled(tenant.ID, req.FunctionalAreaKey),
	})
}
