package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

func handleAssistantTasksAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		assistantWriteGateUnavailable(w, r)
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req assistantTaskSubmitRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	receipt, err := svc.submitTask(r.Context(), tenant.ID, principal, req)
	if err != nil {
		if assistantWriteTaskError(w, r, err) {
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_task_dispatch_failed", "assistant task dispatch failed")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func handleAssistantTaskDetailAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		assistantWriteGateUnavailable(w, r)
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	taskID, ok := extractAssistantTaskIDFromPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid task path")
		return
	}
	detail, err := svc.getTask(r.Context(), tenant.ID, principal, taskID)
	if err != nil {
		if assistantWriteTaskError(w, r, err) {
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_task_dispatch_failed", "assistant task dispatch failed")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func handleAssistantTaskActionAPI(w http.ResponseWriter, r *http.Request, svc *assistantConversationService) {
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		assistantWriteGateUnavailable(w, r)
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	taskID, action, ok := extractAssistantTaskActionPath(r.URL.Path)
	if !ok || action != "cancel" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid task action path")
		return
	}
	resp, err := svc.cancelTask(r.Context(), tenant.ID, principal, taskID)
	if err != nil {
		if assistantWriteTaskError(w, r, err) {
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_task_dispatch_failed", "assistant task dispatch failed")
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func assistantWriteTaskError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, errAssistantConversationNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
	case errors.Is(err, errAssistantTurnNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
	case errors.Is(err, errAssistantTaskNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "assistant_task_not_found", "assistant task not found")
	case errors.Is(err, errAssistantConversationForbidden):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, errAssistantTenantMismatch):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
	case assistantIsGateUnavailableError(err):
		assistantWriteGateUnavailable(w, r)
	case errors.Is(err, errAssistantIdempotencyKeyConflict):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "idempotency_key_conflict", "idempotency key conflict")
	case errors.Is(err, errAssistantRequestInProgress):
		w.Header().Set("Retry-After", assistantDefaultRetryAfterSecs)
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "request_in_progress", "request in progress")
	case errors.Is(err, errAssistantTaskCancelNotAllowed):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "assistant_task_cancel_not_allowed", "assistant task cancel not allowed")
	case errors.Is(err, errAssistantTaskStateInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "assistant_task_state_invalid", "assistant task state invalid")
	case errors.Is(err, errAssistantPlanContractVersionMismatch):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_contract_version_mismatch", "ai plan contract version mismatch")
	case errors.Is(err, errAssistantPlanDeterminismViolation):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_determinism_violation", "ai plan determinism violation")
	case assistantTaskRequestValidationError(err):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_request", err.Error())
	default:
		return false
	}
	return true
}

func assistantTaskRequestValidationError(err error) bool {
	message := strings.TrimSpace(err.Error())
	switch message {
	case "conversation_id required",
		"turn_id required",
		"task_type required",
		"task_type invalid",
		"request_id required",
		"contract_snapshot incomplete":
		return true
	default:
		return false
	}
}
