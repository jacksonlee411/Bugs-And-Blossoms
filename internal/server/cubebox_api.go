package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	cubeboxmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

type cubeBoxConversationWriter interface {
	CreateConversation(ctx context.Context, tenantID string, principal cubeboxmodule.Principal) (*cubeboxdomain.Conversation, error)
	CreateTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, userInput string) (*cubeboxdomain.Conversation, error)
	ConfirmTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, candidateID string) (*cubeboxdomain.Conversation, error)
	CommitTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string) (*cubeboxdomain.TaskReceipt, error)
	SubmitTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, req cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error)
	GetTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, taskID string) (*cubeboxdomain.TaskDetail, error)
	CancelTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, taskID string) (*cubeboxdomain.TaskCancelResponse, error)
	RenderReply(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, req map[string]any) (map[string]any, error)
}

func handleCubeBoxConversationsAPI(w http.ResponseWriter, r *http.Request, facade *cubeboxmodule.Facade) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_service_missing", "cubebox service missing")
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

	if r.Method == http.MethodGet {
		pageSize := 20
		if rawPageSize := strings.TrimSpace(r.URL.Query().Get("page_size")); rawPageSize != "" {
			parsed, err := strconv.Atoi(rawPageSize)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid page_size")
				return
			}
			pageSize = parsed
		}
		items, nextCursor, err := facade.ListConversations(r.Context(), tenant.ID, principal.ID, pageSize, strings.TrimSpace(r.URL.Query().Get("cursor")))
		if err != nil {
			if errors.Is(err, cubeboxservices.ErrConversationCursorInvalid) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "cubebox_conversation_cursor_invalid", "cubebox conversation cursor invalid")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_list_failed", "cubebox conversation list failed")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Items      []cubeboxdomain.ConversationListItem `json:"items"`
			NextCursor string                               `json:"next_cursor"`
		}{
			Items:      items,
			NextCursor: nextCursor,
		})
		return
	}

	conversation, err := facade.CreateConversation(r.Context(), tenant.ID, cubeboxmodule.Principal{
		ID:       principal.ID,
		RoleSlug: principal.RoleSlug,
	})
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_create_failed", "cubebox conversation create failed")
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func handleCubeBoxConversationDetailAPI(w http.ResponseWriter, r *http.Request, facade *cubeboxmodule.Facade) {
	if facade == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_service_missing", "cubebox service missing")
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
	conversationID, ok := extractConversationIDFromPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid conversation path")
		return
	}

	switch r.Method {
	case http.MethodGet:
		conversation, err := facade.GetConversation(r.Context(), tenant.ID, principal.ID, conversationID)
		if err != nil {
			writeCubeBoxConversationError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, conversation)
	case http.MethodDelete:
		err := facade.DeleteConversation(r.Context(), tenant.ID, principal.ID, conversationID)
		if err != nil {
			switch {
			case errors.Is(err, cubeboxservices.ErrDeleteBlockedByTask):
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "cubebox_conversation_delete_blocked_by_running_task", "cubebox conversation delete blocked by running task")
			default:
				writeCubeBoxConversationError(w, r, err)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		routingWriteMethodNotAllowed(w, r)
	}
}

func handleCubeBoxConversationTurnsAPI(w http.ResponseWriter, r *http.Request, facade cubeBoxConversationWriter) {
	if r.Method != http.MethodPost {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
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
	conversationID, ok := extractConversationTurnsPathConversationID(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid turns path")
		return
	}

	var req assistantCreateTurnRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	userInput := strings.TrimSpace(req.UserInput)
	if userInput == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_request", "user_input required")
		return
	}
	conversation, err := facade.CreateTurn(r.Context(), tenant.ID, cubeboxmodule.Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, conversationID, userInput)
	if err != nil {
		writeCubeBoxTurnError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func handleCubeBoxTurnActionAPI(w http.ResponseWriter, r *http.Request, facade cubeBoxConversationWriter) {
	if r.Method != http.MethodPost {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
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
	conversationID, turnID, action, ok := extractAssistantTurnActionPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid turn action path")
		return
	}
	actor := cubeboxmodule.Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}

	switch action {
	case "confirm":
		var req assistantConfirmRequest
		if hasRequestBody(r) {
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
				return
			}
		}
		conversation, err := facade.ConfirmTurn(r.Context(), tenant.ID, actor, conversationID, turnID, strings.TrimSpace(req.CandidateID))
		if err != nil {
			writeCubeBoxTurnActionError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, conversation)
	case "commit":
		receipt, err := facade.CommitTurn(r.Context(), tenant.ID, actor, conversationID, turnID)
		if err != nil {
			writeCubeBoxTurnActionError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, receipt)
	case "reply":
		req := map[string]any{}
		if hasRequestBody(r) {
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
				return
			}
		}
		reply, err := facade.RenderReply(r.Context(), tenant.ID, actor, conversationID, turnID, req)
		if err != nil {
			writeCubeBoxReplyError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, reply)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "assistant action unsupported")
	}
}

func handleCubeBoxTasksAPI(w http.ResponseWriter, r *http.Request, facade cubeBoxConversationWriter) {
	if r.Method != http.MethodPost {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
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
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	receipt, err := facade.SubmitTask(r.Context(), tenant.ID, cubeboxmodule.Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, cubeboxdomain.TaskSubmitRequest{
		ConversationID: req.ConversationID,
		TurnID:         req.TurnID,
		TaskType:       req.TaskType,
		RequestID:      req.RequestID,
		TraceID:        req.TraceID,
		ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:      req.ContractSnapshot.IntentSchemaVersion,
			CompilerContractVersion:  req.ContractSnapshot.CompilerContractVersion,
			CapabilityMapVersion:     req.ContractSnapshot.CapabilityMapVersion,
			SkillManifestDigest:      req.ContractSnapshot.SkillManifestDigest,
			ContextHash:              req.ContractSnapshot.ContextHash,
			IntentHash:               req.ContractSnapshot.IntentHash,
			PlanHash:                 req.ContractSnapshot.PlanHash,
			KnowledgeSnapshotDigest:  req.ContractSnapshot.KnowledgeSnapshotDigest,
			RouteCatalogVersion:      req.ContractSnapshot.RouteCatalogVersion,
			ResolverContractVersion:  req.ContractSnapshot.ResolverContractVersion,
			ContextTemplateVersion:   req.ContractSnapshot.ContextTemplateVersion,
			ReplyGuidanceVersion:     req.ContractSnapshot.ReplyGuidanceVersion,
			PolicyContextDigest:      req.ContractSnapshot.PolicyContextDigest,
			EffectivePolicyVersion:   req.ContractSnapshot.EffectivePolicyVersion,
			ResolvedSetID:            req.ContractSnapshot.ResolvedSetID,
			SetIDSource:              req.ContractSnapshot.SetIDSource,
			PrecheckProjectionDigest: req.ContractSnapshot.PrecheckProjectionDigest,
			MutationPolicyVersion:    req.ContractSnapshot.MutationPolicyVersion,
		},
	})
	if err != nil {
		if assistantWriteTaskError(w, r, err) {
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_task_dispatch_failed", "cubebox task dispatch failed")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func handleCubeBoxTaskDetailAPI(w http.ResponseWriter, r *http.Request, facade cubeBoxConversationWriter) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
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
	task, err := facade.GetTask(r.Context(), tenant.ID, cubeboxmodule.Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, taskID)
	if err != nil {
		if errors.Is(err, cubeboxservices.ErrTaskNotFound) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "cubebox_task_not_found", "cubebox task not found")
			return
		}
		if assistantWriteTaskError(w, r, err) {
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_task_load_failed", "cubebox task load failed")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func handleCubeBoxTaskActionAPI(w http.ResponseWriter, r *http.Request, facade cubeBoxConversationWriter) {
	if r.Method != http.MethodPost {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
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
	resp, err := facade.CancelTask(r.Context(), tenant.ID, cubeboxmodule.Principal{ID: principal.ID, RoleSlug: principal.RoleSlug}, taskID)
	if err != nil {
		if assistantWriteTaskError(w, r, err) {
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_task_cancel_failed", "cubebox task cancel failed")
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func handleCubeBoxModelsAPI(w http.ResponseWriter, r *http.Request, facade *cubeboxmodule.Facade) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_service_missing", "cubebox service missing")
		return
	}
	models, err := facade.Models(r.Context())
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_models_unavailable", "cubebox models unavailable")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Models []cubeboxdomain.ModelEntry `json:"models"`
	}{Models: models})
}

func handleCubeBoxRuntimeStatusAPI(w http.ResponseWriter, r *http.Request, facade *cubeboxmodule.Facade) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	if facade == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_service_missing", "cubebox service missing")
		return
	}
	writeJSON(w, http.StatusOK, facade.RuntimeStatus(r.Context()))
}

func writeCubeBoxConversationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, cubeboxservices.ErrConversationCursorInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "cubebox_conversation_cursor_invalid", "cubebox conversation cursor invalid")
	case errors.Is(err, cubeboxservices.ErrConversationNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
	case errors.Is(err, cubeboxservices.ErrTenantMismatch):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
	case errors.Is(err, cubeboxservices.ErrConversationForbidden):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_conversation_load_failed", "cubebox conversation load failed")
	}
}

func writeCubeBoxTurnError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errAssistantConversationNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
	case errors.Is(err, errAssistantTenantMismatch):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
	case errors.Is(err, errAssistantConversationForbidden):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_plan_schema_constrained_decode_failed", "ai plan schema constrained decode failed")
	case errors.Is(err, errAssistantPlanBoundaryViolation):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "ai_plan_boundary_violation", "ai plan boundary violation")
	case assistantIsRuntimeUnavailableError(err):
		assistantWriteRuntimeUnavailable(w, r)
	case assistantIsGateUnavailableError(err):
		assistantWriteGateUnavailable(w, r)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_turn_create_failed", "cubebox turn create failed")
	}
}

func writeCubeBoxTurnActionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errAssistantConversationNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
	case errors.Is(err, cubeboxservices.ErrConversationNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
	case errors.Is(err, errAssistantTurnNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
	case errors.Is(err, cubeboxservices.ErrTurnNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
	case errors.Is(err, errAssistantIdempotencyKeyConflict):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "idempotency_key_conflict", "idempotency key conflict")
	case errors.Is(err, cubeboxservices.ErrIdempotencyConflict):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "idempotency_key_conflict", "idempotency key conflict")
	case errors.Is(err, errAssistantRequestInProgress):
		w.Header().Set("Retry-After", assistantDefaultRetryAfterSecs)
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "request_in_progress", "request in progress")
	case errors.Is(err, errAssistantConfirmationRequired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
	case errors.Is(err, cubeboxservices.ErrConfirmationRequired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_required", "conversation confirmation required")
	case errors.Is(err, errAssistantConfirmationExpired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_expired", "conversation confirmation expired")
	case errors.Is(err, cubeboxservices.ErrConfirmationExpired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_confirmation_expired", "conversation confirmation expired")
	case errors.Is(err, errAssistantConversationStateInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_state_invalid", "conversation state invalid")
	case errors.Is(err, cubeboxservices.ErrConversationStateInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "conversation_state_invalid", "conversation state invalid")
	case errors.Is(err, errAssistantCandidateNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_candidate_not_found", "assistant candidate not found")
	case errors.Is(err, errAssistantRouteNonBusinessBlocked):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteNonBusinessBlocked.Error(), "assistant route non business blocked")
	case errors.Is(err, errAssistantRouteDecisionMissing):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantRouteDecisionMissing.Error(), "assistant route decision missing")
	case errors.Is(err, errAssistantRouteRuntimeInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteRuntimeInvalid.Error(), "assistant route runtime invalid")
	case errors.Is(err, errAssistantRouteActionConflict):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, errAssistantRouteActionConflict.Error(), "assistant route action conflict")
	case errors.Is(err, errAssistantUnsupportedIntent):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "assistant_intent_unsupported", "assistant intent unsupported")
	case errors.Is(err, errAssistantActionAuthzDenied):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, errAssistantActionAuthzDenied.Error(), "assistant action authz denied")
	case errors.Is(err, errAssistantActionRiskGateDenied):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantActionRiskGateDenied.Error(), "assistant action risk gate denied")
	case errors.Is(err, errAssistantPlanContractVersionMismatch):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_plan_contract_version_mismatch", "ai plan contract version mismatch")
	case errors.Is(err, errAssistantVersionTupleStale):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "ai_version_tuple_stale", "ai version tuple stale")
	case errors.Is(err, errAssistantAuthSnapshotExpired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_auth_snapshot_expired", "ai actor auth snapshot expired")
	case errors.Is(err, errAssistantRoleDriftDetected):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_role_drift_detected", "ai actor role drift detected")
	case errors.Is(err, cubeboxservices.ErrAuthSnapshotExpired):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_auth_snapshot_expired", "ai actor auth snapshot expired")
	case errors.Is(err, cubeboxservices.ErrRoleDriftDetected):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "ai_actor_role_drift_detected", "ai actor role drift detected")
	case errors.Is(err, cubeboxservices.ErrTaskStateInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, "assistant_task_state_invalid", "assistant task state invalid")
	case errors.Is(err, errAssistantActionRequiredCheckFailed):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, errAssistantActionRequiredCheckFailed.Error(), "assistant action required check failed")
	case assistantIsGateUnavailableError(err):
		assistantWriteGateUnavailable(w, r)
	default:
		if status, code, message, ok := assistantResolveCommitError(err); ok {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_turn_action_failed", "cubebox turn action failed")
	}
}

func writeCubeBoxReplyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errAssistantConversationNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_not_found", "conversation not found")
	case errors.Is(err, errAssistantTenantMismatch):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "tenant_mismatch", "tenant mismatch")
	case errors.Is(err, errAssistantConversationForbidden):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, errAssistantTurnNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "conversation_turn_not_found", "conversation turn not found")
	case assistantIsGateUnavailableError(err):
		assistantWriteGateUnavailable(w, r)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "assistant_reply_render_failed", "assistant reply render failed")
	}
}
