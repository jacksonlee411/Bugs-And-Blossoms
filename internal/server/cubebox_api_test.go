package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cubeboxmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

const testCubeBoxTenantUUID = "11111111-1111-1111-1111-111111111111"

func TestCubeBoxConversationAndTaskAPIWrappers(t *testing.T) {
	t.Run("conversation detail missing service", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxConversationDetailAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("conversation detail method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxConversationDetailAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("conversation turns gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxConversationTurnsAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), errAssistantGateUnavailable.Error()) {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("conversation turns method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1/turns", nil)
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("turn action gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTurnActionAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("turn action method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", nil)
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("reply action gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:reply", strings.NewReader(`{"locale":"zh"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTurnActionAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tasks gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{"conversation_id":"conv-1","turn_id":"turn-1","task_type":"confirm","request_id":"req-1","contract_snapshot":{"plan_hash":"p"}}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTasksAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tasks method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks", nil)
		handleCubeBoxTasksAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task detail gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTaskDetailAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task detail method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1", nil)
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task action gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTaskActionAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task action method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1:cancel", nil)
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCubeBoxReplyActionUsesFacade(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:reply", strings.NewReader(`{"locale":"zh","fallback_text":"摘要"}`))
	req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))

	facade := cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{
		reply: map[string]any{
			"text":            "已生成回复",
			"kind":            "info",
			"stage":           "draft",
			"conversation_id": "conv-1",
			"turn_id":         "turn-1",
		},
	})

	handleCubeBoxTurnActionAPI(rec, req, facade)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["text"] != "已生成回复" || payload["turn_id"] != "turn-1" {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestCubeBoxTaskAPIMapsFormalFacadeErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", nil)
	if !assistantWriteTaskError(rec, req, cubeboxservices.ErrPlanContractMismatch) {
		t.Fatal("expected error to be handled")
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ai_plan_contract_version_mismatch") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestCubeBoxTurnActionMapsFormalCommitErrors(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		wantHTTP int
		wantCode string
	}{
		{name: "confirmation_required", err: cubeboxservices.ErrConfirmationRequired, wantHTTP: http.StatusConflict, wantCode: "conversation_confirmation_required"},
		{name: "confirmation_expired", err: cubeboxservices.ErrConfirmationExpired, wantHTTP: http.StatusConflict, wantCode: "conversation_confirmation_expired"},
		{name: "state_invalid", err: cubeboxservices.ErrConversationStateInvalid, wantHTTP: http.StatusConflict, wantCode: "conversation_state_invalid"},
		{name: "candidate_not_found", err: errAssistantCandidateNotFound, wantHTTP: http.StatusUnprocessableEntity, wantCode: "assistant_candidate_not_found"},
		{name: "route_non_business_blocked", err: errAssistantRouteNonBusinessBlocked, wantHTTP: http.StatusConflict, wantCode: "ai_route_non_business_blocked"},
		{name: "route_decision_missing", err: errAssistantRouteDecisionMissing, wantHTTP: http.StatusConflict, wantCode: errAssistantRouteDecisionMissing.Error()},
		{name: "route_runtime_invalid", err: errAssistantRouteRuntimeInvalid, wantHTTP: http.StatusUnprocessableEntity, wantCode: errAssistantRouteRuntimeInvalid.Error()},
		{name: "route_action_conflict", err: errAssistantRouteActionConflict, wantHTTP: http.StatusUnprocessableEntity, wantCode: errAssistantRouteActionConflict.Error()},
		{name: "unsupported_intent", err: errAssistantUnsupportedIntent, wantHTTP: http.StatusUnprocessableEntity, wantCode: "assistant_intent_unsupported"},
		{name: "action_authz_denied", err: errAssistantActionAuthzDenied, wantHTTP: http.StatusForbidden, wantCode: errAssistantActionAuthzDenied.Error()},
		{name: "action_risk_gate_denied", err: errAssistantActionRiskGateDenied, wantHTTP: http.StatusConflict, wantCode: errAssistantActionRiskGateDenied.Error()},
		{name: "plan_contract_version_mismatch", err: errAssistantPlanContractVersionMismatch, wantHTTP: http.StatusConflict, wantCode: "ai_plan_contract_version_mismatch"},
		{name: "version_tuple_stale", err: errAssistantVersionTupleStale, wantHTTP: http.StatusConflict, wantCode: "ai_version_tuple_stale"},
		{name: "auth_snapshot_expired", err: cubeboxservices.ErrAuthSnapshotExpired, wantHTTP: http.StatusForbidden, wantCode: "ai_actor_auth_snapshot_expired"},
		{name: "assistant_auth_snapshot_expired", err: errAssistantAuthSnapshotExpired, wantHTTP: http.StatusForbidden, wantCode: "ai_actor_auth_snapshot_expired"},
		{name: "role_drift", err: cubeboxservices.ErrRoleDriftDetected, wantHTTP: http.StatusForbidden, wantCode: "ai_actor_role_drift_detected"},
		{name: "assistant_role_drift", err: errAssistantRoleDriftDetected, wantHTTP: http.StatusForbidden, wantCode: "ai_actor_role_drift_detected"},
		{name: "task_state_invalid", err: cubeboxservices.ErrTaskStateInvalid, wantHTTP: http.StatusConflict, wantCode: "assistant_task_state_invalid"},
		{name: "required_check_failed", err: errAssistantActionRequiredCheckFailed, wantHTTP: http.StatusConflict, wantCode: errAssistantActionRequiredCheckFailed.Error()},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:commit", nil)
			writeCubeBoxTurnActionError(rec, req, tc.err)
			if rec.Code != tc.wantHTTP {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Fatalf("body=%s", rec.Body.String())
			}
		})
	}
}

func TestCubeBoxConversationsAPIPaths(t *testing.T) {
	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/internal/cubebox/conversations", nil)
		handleCubeBoxConversationsAPI(rec, req, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations", nil)
		handleCubeBoxConversationsAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationsAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid page size", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations?page_size=bad", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationsAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("list cursor invalid rejects before fallback", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations?cursor=bad", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationsAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "cubebox_conversation_cursor_invalid") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})

	t.Run("list success via legacy facade", func(t *testing.T) {
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})
		assistantSvc := newAssistantConversationService(nil, nil)
		if _, err := assistantSvc.createConversationWithContext(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "admin"}); err != nil {
			t.Fatalf("create conversation: %v", err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations?page_size=5", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: testCubeBoxTenantUUID}))
		facade := newCubeBoxFacade(nil, assistantSvc, fileSvc)
		if _, _, err := facade.ListConversations(context.Background(), testCubeBoxTenantUUID, "actor-1", 5, ""); err != nil {
			t.Fatalf("facade list conversations: %v", err)
		}
		handleCubeBoxConversationsAPI(rec, req, facade)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "items") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("create conversation success", func(t *testing.T) {
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})
		assistantSvc := newAssistantConversationService(nil, nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: testCubeBoxTenantUUID}))
		facade := newCubeBoxFacade(nil, assistantSvc, fileSvc)
		if _, err := facade.CreateConversation(context.Background(), testCubeBoxTenantUUID, cubeboxmodule.Principal{ID: "actor-1", RoleSlug: "admin"}); err != nil {
			t.Fatalf("facade create conversation: %v", err)
		}
		handleCubeBoxConversationsAPI(rec, req, facade)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "conv_") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCubeBoxConversationDetailAndTurnHandlers(t *testing.T) {
	t.Run("detail tenant missing and principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1", nil)
		handleCubeBoxConversationDetailAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationDetailAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("detail invalid path", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationDetailAPI(rec, req, cubeboxmodule.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("detail get success", func(t *testing.T) {
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})
		assistantSvc := newAssistantConversationService(nil, nil)
		conversation, err := assistantSvc.createConversationWithContext(context.Background(), testCubeBoxTenantUUID, Principal{ID: "actor-1"})
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/"+conversation.ConversationID, nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: testCubeBoxTenantUUID}))
		facade := newCubeBoxFacade(nil, assistantSvc, fileSvc)
		handleCubeBoxConversationDetailAPI(rec, req, facade)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), conversation.ConversationID) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("detail delete unexpected error", func(t *testing.T) {
		facade := cubeboxservices.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{
			getConversationFn: func(context.Context, string, string, string) (*cubeboxdomain.Conversation, error) {
				return nil, errors.New("boom")
			},
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationDetailAPI(rec, req, facade)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("detail delete forbidden", func(t *testing.T) {
		facade := cubeboxservices.NewFacade(nil, nil, nil, stubCubeBoxLegacyFacade{
			getConversationFn: func(context.Context, string, string, string) (*cubeboxdomain.Conversation, error) {
				return nil, cubeboxservices.ErrConversationForbidden
			},
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationDetailAPI(rec, req, facade)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("turns invalid json and empty input", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"   "}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("turns tenant missing principal missing and invalid path", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("turns success and error mapping", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{
			createTurnFn: func(_ context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, userInput string) (*cubeboxdomain.Conversation, error) {
				if tenantID != "tenant-1" || principal.ID != "actor-1" || conversationID != "conv-1" || userInput != "hello" {
					t.Fatalf("unexpected args tenant=%q principal=%+v conversation=%q userInput=%q", tenantID, principal, conversationID, userInput)
				}
				return &cubeboxdomain.Conversation{ConversationID: "conv-1"}, nil
			},
		})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "conv-1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{
			createTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.Conversation, error) {
				return nil, errAssistantPlanBoundaryViolation
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{
			createTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.Conversation, error) {
				return nil, errAssistantGateUnavailable
			},
		})
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns", strings.NewReader(`{"user_input":"hello"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxConversationTurnsAPI(rec, req, stubCubeBoxWriter{
			createTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.Conversation, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCubeBoxTaskAndActionHandlers(t *testing.T) {
	t.Run("tasks tenant missing and principal missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{"conversation_id":"conv-1","turn_id":"turn-1","task_type":"assistant_async_plan","request_id":"req-1","contract_snapshot":{"plan_hash":"plan"}}`))
		handleCubeBoxTasksAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{"conversation_id":"conv-1","turn_id":"turn-1","task_type":"assistant_async_plan","request_id":"req-1","contract_snapshot":{"plan_hash":"plan"}}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxTasksAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tasks invalid json and internal error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTasksAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{"conversation_id":"conv-1","turn_id":"turn-1","task_type":"assistant_async_plan","request_id":"req-1","contract_snapshot":{"intent_schema_version":"intent.v1","compiler_contract_version":"compiler.v1","capability_map_version":"cap.v1","skill_manifest_digest":"skill","context_hash":"ctx","intent_hash":"intent","plan_hash":"plan"}}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTasksAPI(rec, req, stubCubeBoxWriter{
			submitTaskFn: func(context.Context, string, cubeboxmodule.Principal, cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tasks success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks", strings.NewReader(`{"conversation_id":"conv-1","turn_id":"turn-1","task_type":"assistant_async_plan","request_id":"req-1","trace_id":"trace-1","contract_snapshot":{"intent_schema_version":"intent.v1","compiler_contract_version":"compiler.v1","capability_map_version":"cap.v1","skill_manifest_digest":"skill","context_hash":"ctx","intent_hash":"intent","plan_hash":"plan","route_catalog_version":"route.v1"}}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTasksAPI(rec, req, stubCubeBoxWriter{
			submitTaskFn: func(_ context.Context, tenantID string, principal cubeboxmodule.Principal, req cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
				if tenantID != "tenant-1" || principal.ID != "actor-1" || req.ContractSnapshot.RouteCatalogVersion != "route.v1" {
					t.Fatalf("unexpected req tenant=%q principal=%+v req=%+v", tenantID, principal, req)
				}
				return &cubeboxdomain.TaskReceipt{TaskID: "task-1"}, nil
			},
		})
		if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "task-1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task detail method tenant principal invalid path not found internal error and success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1", nil)
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{
			getTaskFn: func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskDetail, error) {
				return nil, cubeboxservices.ErrTaskNotFound
			},
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{
			getTaskFn: func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskDetail, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskDetailAPI(rec, req, stubCubeBoxWriter{
			getTaskFn: func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskDetail, error) {
				return &cubeboxdomain.TaskDetail{TaskID: "task-1"}, nil
			},
		})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "task-1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("task action method tenant principal invalid path error internal error and success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1:cancel", nil)
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:other", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{
			cancelTaskFn: func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskCancelResponse, error) {
				return nil, cubeboxservices.ErrTaskCancelNotAllowed
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{
			cancelTaskFn: func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskCancelResponse, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/tasks/task-1:cancel", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTaskActionAPI(rec, req, stubCubeBoxWriter{
			cancelTaskFn: func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskCancelResponse, error) {
				return &cubeboxdomain.TaskCancelResponse{TaskDetail: cubeboxdomain.TaskDetail{TaskID: "task-1"}, CancelAccepted: true}, nil
			},
		})
		if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "task-1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCubeBoxTurnActionConfirmCommitReplyBranches(t *testing.T) {
	t.Run("turn action tenant missing principal missing invalid path and unsupported action", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", nil)
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:other", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("confirm bad json and success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", strings.NewReader(`{`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", strings.NewReader(`{"candidate_id":"cand-1"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			confirmTurnFn: func(_ context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, candidateID string) (*cubeboxdomain.Conversation, error) {
				if tenantID != "tenant-1" || principal.ID != "actor-1" || conversationID != "conv-1" || turnID != "turn-1" || candidateID != "cand-1" {
					t.Fatalf("unexpected args tenant=%q principal=%+v conversationID=%q turnID=%q candidateID=%q", tenantID, principal, conversationID, turnID, candidateID)
				}
				return &cubeboxdomain.Conversation{ConversationID: "conv-1"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("confirm error mappings", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", strings.NewReader(`{"candidate_id":"cand-1"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			confirmTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string, string) (*cubeboxdomain.Conversation, error) {
				return nil, cubeboxservices.ErrConfirmationRequired
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("commit fallback error and success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:commit", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			commitTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:commit", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			commitTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
				return &cubeboxdomain.TaskReceipt{TaskID: "task-1"}, nil
			},
		})
		if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "task-1") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("commit stable code and bad request fallback", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:commit", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			commitTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
				return nil, errors.New("ORG_CODE_INVALID")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:commit", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1", RoleSlug: "admin"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			commitTurnFn: func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
				return nil, newBadRequestError("bad request")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("reply bad json and reply error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:reply", strings.NewReader(`{`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:reply", strings.NewReader(`{"locale":"zh"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			renderReplyFn: func(context.Context, string, cubeboxmodule.Principal, string, string, map[string]any) (map[string]any, error) {
				return nil, errAssistantTurnNotFound
			},
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:reply", strings.NewReader(`{"locale":"zh"}`))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))
		handleCubeBoxTurnActionAPI(rec, req, stubCubeBoxWriter{
			renderReplyFn: func(context.Context, string, cubeboxmodule.Principal, string, string, map[string]any) (map[string]any, error) {
				return nil, errAssistantConversationForbidden
			},
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCubeBoxModelsAndErrorHelpers(t *testing.T) {
	t.Run("models method and missing facade", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/models", nil)
		handleCubeBoxModelsAPI(rec, req, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/models", nil)
		handleCubeBoxModelsAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/models", nil)
		facade := cubeboxmodule.NewFacade(nil, cubeboxRuntimeProbe{assistant: &assistantConversationService{
			modelGateway: &assistantModelGateway{
				config: assistantModelConfig{
					Providers: []assistantModelProviderConfig{
						{Name: "openai", Enabled: true, Model: "gpt-5.4", Endpoint: "builtin://openai", TimeoutMS: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"},
					},
				},
			},
		}}, nil, nil)
		handleCubeBoxModelsAPI(rec, req, facade)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "gpt-5.4") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("models empty and error branch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/models", nil)
		facade := cubeboxmodule.NewFacade(nil, cubeboxRuntimeProbe{}, nil, nil)
		handleCubeBoxModelsAPI(rec, req, facade)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"models":null`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/models", nil)
		facade = cubeboxmodule.NewFacade(nil, stubCubeBoxRuntimeProbe{modelsErr: errors.New("models failed")}, nil, nil)
		handleCubeBoxModelsAPI(rec, req, facade)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "cubebox_models_unavailable") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("conversation and reply error helpers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/conversations/conv-1", nil)

		rec := httptest.NewRecorder()
		writeCubeBoxConversationError(rec, req, cubeboxservices.ErrConversationCursorInvalid)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxConversationError(rec, req, cubeboxservices.ErrConversationNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxConversationError(rec, req, cubeboxservices.ErrTenantMismatch)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxConversationError(rec, req, cubeboxservices.ErrConversationForbidden)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxConversationError(rec, req, errors.New("boom"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantConversationForbidden)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantConversationNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantPlanSchemaConstrainedDecodeFailed)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantPlanBoundaryViolation)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantRuntimeUnavailable)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantTenantMismatch)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errAssistantGateUnavailable)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnError(rec, req, errors.New("boom"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantConversationNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantTurnNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrConversationNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrTurnNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantIdempotencyKeyConflict)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrIdempotencyConflict)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantRequestInProgress)
		if rec.Code != http.StatusConflict || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("status=%d retryAfter=%q body=%s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantCandidateNotFound)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantConfirmationRequired)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrConfirmationRequired)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantConfirmationExpired)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrConfirmationExpired)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantConversationStateInvalid)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrConversationStateInvalid)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrAuthSnapshotExpired)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrRoleDriftDetected)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, cubeboxservices.ErrTaskStateInvalid)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errAssistantGateUnavailable)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errors.New("ORG_CODE_INVALID"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, newBadRequestError("bad request"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxTurnActionError(rec, req, errors.New("boom"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxReplyError(rec, req, errAssistantTenantMismatch)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxReplyError(rec, req, errAssistantConversationNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxReplyError(rec, req, errAssistantTurnNotFound)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxReplyError(rec, req, errAssistantGateUnavailable)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		writeCubeBoxReplyError(rec, req, errors.New("boom"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

type stubCubeBoxLegacyFacade struct {
	reply                map[string]any
	getConversationFn    func(context.Context, string, string, string) (*cubeboxdomain.Conversation, error)
	createConversationFn func(context.Context, string, cubeboxservices.Principal) (*cubeboxdomain.Conversation, error)
}

type stubCubeBoxWriter struct {
	createConversationFn func(context.Context, string, cubeboxmodule.Principal) (*cubeboxdomain.Conversation, error)
	createTurnFn         func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.Conversation, error)
	confirmTurnFn        func(context.Context, string, cubeboxmodule.Principal, string, string, string) (*cubeboxdomain.Conversation, error)
	commitTurnFn         func(context.Context, string, cubeboxmodule.Principal, string, string) (*cubeboxdomain.TaskReceipt, error)
	submitTaskFn         func(context.Context, string, cubeboxmodule.Principal, cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error)
	getTaskFn            func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskDetail, error)
	cancelTaskFn         func(context.Context, string, cubeboxmodule.Principal, string) (*cubeboxdomain.TaskCancelResponse, error)
	renderReplyFn        func(context.Context, string, cubeboxmodule.Principal, string, string, map[string]any) (map[string]any, error)
}

func (s stubCubeBoxWriter) CreateConversation(ctx context.Context, tenantID string, principal cubeboxmodule.Principal) (*cubeboxdomain.Conversation, error) {
	if s.createConversationFn == nil {
		return nil, nil
	}
	return s.createConversationFn(ctx, tenantID, principal)
}

func (s stubCubeBoxWriter) CreateTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, userInput string) (*cubeboxdomain.Conversation, error) {
	if s.createTurnFn == nil {
		return nil, nil
	}
	return s.createTurnFn(ctx, tenantID, principal, conversationID, userInput)
}

func (s stubCubeBoxWriter) ConfirmTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, candidateID string) (*cubeboxdomain.Conversation, error) {
	if s.confirmTurnFn == nil {
		return nil, nil
	}
	return s.confirmTurnFn(ctx, tenantID, principal, conversationID, turnID, candidateID)
}

func (s stubCubeBoxWriter) CommitTurn(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string) (*cubeboxdomain.TaskReceipt, error) {
	if s.commitTurnFn == nil {
		return nil, nil
	}
	return s.commitTurnFn(ctx, tenantID, principal, conversationID, turnID)
}

func (s stubCubeBoxWriter) SubmitTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, req cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
	if s.submitTaskFn == nil {
		return nil, nil
	}
	return s.submitTaskFn(ctx, tenantID, principal, req)
}

func (s stubCubeBoxWriter) GetTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, taskID string) (*cubeboxdomain.TaskDetail, error) {
	if s.getTaskFn == nil {
		return nil, nil
	}
	return s.getTaskFn(ctx, tenantID, principal, taskID)
}

func (s stubCubeBoxWriter) CancelTask(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, taskID string) (*cubeboxdomain.TaskCancelResponse, error) {
	if s.cancelTaskFn == nil {
		return nil, nil
	}
	return s.cancelTaskFn(ctx, tenantID, principal, taskID)
}

func (s stubCubeBoxWriter) RenderReply(ctx context.Context, tenantID string, principal cubeboxmodule.Principal, conversationID string, turnID string, req map[string]any) (map[string]any, error) {
	if s.renderReplyFn == nil {
		return nil, nil
	}
	return s.renderReplyFn(ctx, tenantID, principal, conversationID, turnID, req)
}

func (s stubCubeBoxLegacyFacade) ListConversations(context.Context, string, string, int, string) ([]cubeboxdomain.ConversationListItem, string, error) {
	return nil, "", nil
}
func (s stubCubeBoxLegacyFacade) GetConversation(ctx context.Context, tenantID string, actorID string, conversationID string) (*cubeboxdomain.Conversation, error) {
	if s.getConversationFn == nil {
		return nil, nil
	}
	return s.getConversationFn(ctx, tenantID, actorID, conversationID)
}
func (s stubCubeBoxLegacyFacade) CreateConversation(ctx context.Context, tenantID string, principal cubeboxservices.Principal) (*cubeboxdomain.Conversation, error) {
	if s.createConversationFn == nil {
		return nil, nil
	}
	return s.createConversationFn(ctx, tenantID, principal)
}
func (s stubCubeBoxLegacyFacade) CreateTurn(context.Context, string, cubeboxservices.Principal, string, string) (*cubeboxdomain.Conversation, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) ConfirmTurn(context.Context, string, cubeboxservices.Principal, string, string, string) (*cubeboxdomain.Conversation, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) CommitTurn(context.Context, string, cubeboxservices.Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) SubmitTask(context.Context, string, cubeboxservices.Principal, cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) GetTask(context.Context, string, cubeboxservices.Principal, string) (*cubeboxdomain.TaskDetail, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) CancelTask(context.Context, string, cubeboxservices.Principal, string) (*cubeboxdomain.TaskCancelResponse, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) ExecuteTaskWorkflow(context.Context, string, cubeboxservices.Principal, *cubeboxdomain.Conversation, string) (cubeboxservices.TaskWorkflowExecutionResult, error) {
	return cubeboxservices.TaskWorkflowExecutionResult{}, nil
}
func (s stubCubeBoxLegacyFacade) RenderReply(context.Context, string, cubeboxservices.Principal, string, string, map[string]any) (map[string]any, error) {
	return s.reply, nil
}

func TestCubeBoxConversationDeleteUsesFormalSemantics(t *testing.T) {
	fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})
	assistantSvc := newAssistantConversationService(nil, nil)
	_, err := assistantSvc.createConversationWithContext(context.Background(), "tenant-1", Principal{ID: "actor-1"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	facade := newCubeBoxFacade(nil, assistantSvc, fileSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/internal/cubebox/conversations/conv-1", nil)
	req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

	handleCubeBoxConversationDetailAPI(rec, req, facade)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCubeBoxRuntimeStatusAPI(t *testing.T) {
	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/runtime-status", nil)

		handleCubeBoxRuntimeStatusAPI(rec, req, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing facade", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		handleCubeBoxRuntimeStatusAPI(rec, req, nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("assistant and file store missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		facade := cubeboxmodule.NewFacade(nil, nil, nil, nil)

		handleCubeBoxRuntimeStatusAPI(rec, req, facade)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload cubeboxdomain.RuntimeStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != "unavailable" {
			t.Fatalf("status=%+v", payload)
		}
		if payload.Backend.Reason != "assistant_service_missing" || payload.FileStore.Reason != "file_store_missing" {
			t.Fatalf("payload=%+v", payload)
		}
	})

	t.Run("knowledge degraded but model and file healthy", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		svc := &assistantConversationService{
			modelGateway: &assistantModelGateway{},
			knowledgeErr: errors.New("knowledge unavailable"),
		}
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{})
		facade := newCubeBoxFacade(nil, svc, fileSvc)

		handleCubeBoxRuntimeStatusAPI(rec, req, facade)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload cubeboxdomain.RuntimeStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != "degraded" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.KnowledgeRuntime.Reason != "knowledge_runtime_unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.ModelGateway.Healthy != "healthy" || payload.FileStore.Healthy != "healthy" {
			t.Fatalf("payload=%+v", payload)
		}
	})

	t.Run("model gateway unavailable and file store unhealthy", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		svc := &assistantConversationService{
			modelGateway: &assistantModelGateway{},
			gatewayErr:   errors.New("gateway unavailable"),
		}
		fileSvc := cubeboxservices.NewFileService(&runtimeHealthyFileStore{healthyErr: errors.New("disk unavailable")})
		facade := newCubeBoxFacade(nil, svc, fileSvc)

		handleCubeBoxRuntimeStatusAPI(rec, req, facade)
		var payload cubeboxdomain.RuntimeStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != "unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.ModelGateway.Reason != "model_gateway_unavailable" || payload.FileStore.Reason != "file_store_unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
	})

	t.Run("file metadata repo unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/runtime-status", nil)
		svc := &assistantConversationService{
			modelGateway: &assistantModelGateway{},
		}
		fileSvc := cubeboxservices.NewFileService(runtimeHealthyFileRepo{healthyErr: errors.New("repo unavailable")}, runtimeHealthyObjectStore{})
		facade := newCubeBoxFacade(nil, svc, fileSvc)

		handleCubeBoxRuntimeStatusAPI(rec, req, facade)
		var payload cubeboxdomain.RuntimeStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Status != "unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
		if payload.FileStore.Reason != "file_store_unavailable" {
			t.Fatalf("payload=%+v", payload)
		}
	})
}

type runtimeHealthyFileStore struct {
	healthyErr error
}

type runtimeHealthyFileRepo struct {
	healthyErr error
}

type runtimeHealthyObjectStore struct{}

type stubCubeBoxRuntimeProbe struct {
	models    []cubeboxdomain.ModelEntry
	modelsErr error
}

func (s stubCubeBoxRuntimeProbe) BackendStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	return cubeboxdomain.RuntimeComponentStatus{Healthy: "healthy"}
}

func (s stubCubeBoxRuntimeProbe) KnowledgeRuntimeStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	return cubeboxdomain.RuntimeComponentStatus{Healthy: "healthy"}
}

func (s stubCubeBoxRuntimeProbe) ModelGatewayStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	return cubeboxdomain.RuntimeComponentStatus{Healthy: "healthy"}
}

func (s stubCubeBoxRuntimeProbe) Models(context.Context) ([]cubeboxdomain.ModelEntry, error) {
	return s.models, s.modelsErr
}

func (s *runtimeHealthyFileStore) List(_ context.Context, _ string, _ string) ([]cubeboxservices.FileRecord, error) {
	return nil, nil
}

func (s *runtimeHealthyFileStore) Save(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ io.Reader) (cubeboxservices.FileRecord, error) {
	return cubeboxservices.FileRecord{}, nil
}

func (s *runtimeHealthyFileStore) Delete(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

func (s *runtimeHealthyFileStore) Healthy(context.Context) error {
	return s.healthyErr
}

func (s runtimeHealthyFileRepo) ListFiles(context.Context, string, string, int32) ([]cubeboxservices.FileMetadata, error) {
	return nil, nil
}
func (s runtimeHealthyFileRepo) ListFileLinks(context.Context, string, string) ([]cubeboxservices.FileLinkRef, error) {
	return nil, nil
}
func (s runtimeHealthyFileRepo) ListTenantFileLinks(context.Context, string) ([]cubeboxservices.FileLinkRef, error) {
	return nil, nil
}
func (s runtimeHealthyFileRepo) GetFile(context.Context, string, string) (cubeboxservices.FileMetadata, error) {
	return cubeboxservices.FileMetadata{}, nil
}
func (s runtimeHealthyFileRepo) ConversationExists(context.Context, string, string) (bool, error) {
	return false, nil
}
func (s runtimeHealthyFileRepo) CreateFile(context.Context, string, cubeboxservices.FileObject, string, string, string, time.Time) (cubeboxservices.FileMetadata, []cubeboxservices.FileLinkRef, error) {
	return cubeboxservices.FileMetadata{}, nil, nil
}
func (s runtimeHealthyFileRepo) CountFileLinks(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (s runtimeHealthyFileRepo) DeleteFile(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (s runtimeHealthyFileRepo) InsertFileCleanupJob(context.Context, string, cubeboxservices.FileCleanupJob, time.Time) error {
	return nil
}
func (s runtimeHealthyFileRepo) Healthy(context.Context, string) error { return s.healthyErr }

func (runtimeHealthyObjectStore) SaveObject(context.Context, string, string, string, string, io.Reader) (cubeboxservices.FileObject, error) {
	return cubeboxservices.FileObject{}, nil
}
func (runtimeHealthyObjectStore) DeleteObject(context.Context, string) error { return nil }
func (runtimeHealthyObjectStore) Healthy(context.Context) error              { return nil }

func TestAssistantNamespaceSegment(t *testing.T) {
	t.Parallel()

	if !assistantNamespaceSegment("assistant") {
		t.Fatal("assistant should be accepted")
	}
	if !assistantNamespaceSegment("cubebox") {
		t.Fatal("cubebox should be accepted")
	}
	if assistantNamespaceSegment("other") {
		t.Fatal("other should be rejected")
	}
}
