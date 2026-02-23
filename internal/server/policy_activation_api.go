package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type policyActivationRequest struct {
	CapabilityKey       string `json:"capability_key"`
	TargetPolicyVersion string `json:"target_policy_version"`
	DraftPolicyVersion  string `json:"draft_policy_version"`
	Operator            string `json:"operator"`
}

func handleInternalPolicyStateAPI(w http.ResponseWriter, r *http.Request) {
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
	capabilityKey := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("capability_key")))
	if capabilityKey == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "capability_key required")
		return
	}
	state, err := defaultPolicyActivationRuntime.state(tenant.ID, capabilityKey)
	if err != nil {
		writePolicyActivationError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(state)
}

func handleInternalPolicyDraftAPI(w http.ResponseWriter, r *http.Request) {
	handleInternalPolicyMutationAPI(w, r, func(tenantID string, req policyActivationRequest) (capabilityPolicyState, error) {
		return defaultPolicyActivationRuntime.setDraft(tenantID, req.CapabilityKey, req.DraftPolicyVersion, req.Operator)
	})
}

func handleInternalPolicyActivateAPI(w http.ResponseWriter, r *http.Request) {
	handleInternalPolicyMutationAPI(w, r, func(tenantID string, req policyActivationRequest) (capabilityPolicyState, error) {
		return defaultPolicyActivationRuntime.activate(tenantID, req.CapabilityKey, req.TargetPolicyVersion, req.Operator)
	})
}

func handleInternalPolicyRollbackAPI(w http.ResponseWriter, r *http.Request) {
	handleInternalPolicyMutationAPI(w, r, func(tenantID string, req policyActivationRequest) (capabilityPolicyState, error) {
		return defaultPolicyActivationRuntime.rollback(tenantID, req.CapabilityKey, req.TargetPolicyVersion, req.Operator)
	})
}

func handleInternalPolicyMutationAPI(w http.ResponseWriter, r *http.Request, mutator func(tenantID string, req policyActivationRequest) (capabilityPolicyState, error)) {
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
	var req policyActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.CapabilityKey = strings.ToLower(strings.TrimSpace(req.CapabilityKey))
	req.Operator = strings.TrimSpace(req.Operator)
	if req.CapabilityKey == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "capability_key required")
		return
	}
	state, err := mutator(tenant.ID, req)
	if err != nil {
		writePolicyActivationError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(state)
}

func writePolicyActivationError(w http.ResponseWriter, r *http.Request, err error) {
	code := strings.TrimSpace(err.Error())
	status := http.StatusInternalServerError
	message := "internal error"
	switch code {
	case functionalAreaMissingCode:
		status = http.StatusNotFound
		message = "functional area missing"
	case policyActivationCodeVersionRequired:
		status = http.StatusBadRequest
		message = "policy version required"
	case policyActivationCodeDraftMissing:
		status = http.StatusConflict
		message = "policy draft missing"
	case policyActivationCodeRollbackMissing:
		status = http.StatusConflict
		message = "policy rollback unavailable"
	}
	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
}
