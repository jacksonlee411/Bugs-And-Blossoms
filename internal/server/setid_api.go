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

type setidCreateAPIRequest struct {
	SetID         string `json:"setid"`
	Name          string `json:"name"`
	EffectiveDate string `json:"effective_date"`
	RequestID     string `json:"request_id"`
}

func handleSetIDsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if err := store.EnsureBootstrap(r.Context(), tenant.ID, tenant.ID); err != nil {
			writeInternalAPIError(w, r, err, "bootstrap_failed")
			return
		}

		setids, err := store.ListSetIDs(r.Context(), tenant.ID)
		if err != nil {
			writeInternalAPIError(w, r, err, "setid_list_failed")
			return
		}

		type item struct {
			SetID    string `json:"setid"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			IsShared bool   `json:"is_shared"`
		}
		out := make([]item, 0, len(setids))
		for _, s := range setids {
			out = append(out, item{
				SetID:    s.SetID,
				Name:     s.Name,
				Status:   s.Status,
				IsShared: s.IsShared,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant_id": tenant.ID,
			"setids":    out,
		})
		return

	case http.MethodPost:
		var req setidCreateAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}

		req.SetID = strings.TrimSpace(req.SetID)
		req.Name = strings.TrimSpace(req.Name)
		req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
		req.RequestID = strings.TrimSpace(req.RequestID)
		if req.SetID == "" || req.Name == "" || req.RequestID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "setid/name/request_id required")
			return
		}
		if req.EffectiveDate == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "effective_date required")
			return
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}

		if err := store.EnsureBootstrap(r.Context(), tenant.ID, tenant.ID); err != nil {
			writeInternalAPIError(w, r, err, "bootstrap_failed")
			return
		}

		if err := store.CreateSetID(r.Context(), tenant.ID, req.SetID, req.Name, req.EffectiveDate, req.RequestID, tenant.ID); err != nil {
			writeInternalAPIError(w, r, err, "setid_create_failed")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"setid":  strings.ToUpper(req.SetID),
			"status": "active",
		})
		return

	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type setidBindingAPIRequest struct {
	OrgUnitID     string `json:"org_unit_id"`
	OrgCode       string `json:"org_code"`
	SetID         string `json:"setid"`
	EffectiveDate string `json:"effective_date"`
	RequestID     string `json:"request_id"`
}

func handleSetIDBindingsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore, orgStore OrgUnitStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
		if asOf == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "as_of required")
			return
		}
		if _, err := time.Parse("2006-01-02", asOf); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
			return
		}

		rows, err := store.ListSetIDBindings(r.Context(), tenant.ID, asOf)
		if err != nil {
			writeInternalAPIError(w, r, err, "setid_bindings_list_failed")
			return
		}
		if rows == nil {
			rows = []SetIDBindingRow{}
		}

		type row struct {
			OrgUnitID string `json:"org_unit_id"`
			SetID     string `json:"setid"`
			ValidFrom string `json:"valid_from"`
			ValidTo   string `json:"valid_to"`
		}
		out := make([]row, 0, len(rows))
		for _, it := range rows {
			out = append(out, row{
				OrgUnitID: it.OrgUnitID,
				SetID:     it.SetID,
				ValidFrom: it.ValidFrom,
				ValidTo:   it.ValidTo,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant_id": tenant.ID,
			"as_of":     asOf,
			"bindings":  out,
		})
		return

	case http.MethodPost:
		var req setidBindingAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}

		req.OrgUnitID = strings.TrimSpace(req.OrgUnitID)
		req.SetID = strings.TrimSpace(req.SetID)
		req.EffectiveDate = strings.TrimSpace(req.EffectiveDate)
		req.RequestID = strings.TrimSpace(req.RequestID)
		if req.OrgUnitID != "" || req.OrgCode == "" || req.SetID == "" || req.EffectiveDate == "" || req.RequestID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code/setid/effective_date/request_id required")
			return
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
			return
		}

		normalizedCode, err := orgunitpkg.NormalizeOrgCode(req.OrgCode)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
			return
		}
		orgID, err := orgStore.ResolveOrgID(r.Context(), tenant.ID, normalizedCode)
		if err != nil {
			switch {
			case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
			case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
			default:
				writeInternalAPIError(w, r, err, "setid_resolve_org_code_failed")
			}
			return
		}

		if err := store.BindSetID(r.Context(), tenant.ID, strconv.Itoa(orgID), req.EffectiveDate, req.SetID, req.RequestID, tenant.ID); err != nil {
			writeInternalAPIError(w, r, err, "setid_binding_failed")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"org_code":       normalizedCode,
			"setid":          strings.ToUpper(req.SetID),
			"effective_date": req.EffectiveDate,
		})
		return

	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

type globalSetIDAPIRequest struct {
	Name      string `json:"name"`
	RequestID string `json:"request_id"`
}

func handleGlobalSetIDsAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	if r.Method == http.MethodGet {
		globalSetids, err := store.ListGlobalSetIDs(r.Context())
		if err != nil {
			writeInternalAPIError(w, r, err, "global_setid_list_failed")
			return
		}
		if globalSetids == nil {
			globalSetids = []SetID{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(globalSetids)
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
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, scopeReasonActorScopeForbidden, "actor scope forbidden")
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
