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

	cubeboxmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

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

	t.Run("turn action gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations/conv-1/turns/turn-1:confirm", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTurnActionAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
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

	t.Run("task detail gate unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/tasks/task-1", nil)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-1"}))

		handleCubeBoxTaskDetailAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
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
		{name: "auth_snapshot_expired", err: cubeboxservices.ErrAuthSnapshotExpired, wantHTTP: http.StatusForbidden, wantCode: "ai_actor_auth_snapshot_expired"},
		{name: "role_drift", err: cubeboxservices.ErrRoleDriftDetected, wantHTTP: http.StatusForbidden, wantCode: "ai_actor_role_drift_detected"},
		{name: "task_state_invalid", err: cubeboxservices.ErrTaskStateInvalid, wantHTTP: http.StatusConflict, wantCode: "assistant_task_state_invalid"},
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

type stubCubeBoxLegacyFacade struct {
	reply map[string]any
}

func (s stubCubeBoxLegacyFacade) ListConversations(context.Context, string, string, int, string) ([]cubeboxdomain.ConversationListItem, string, error) {
	return nil, "", nil
}
func (s stubCubeBoxLegacyFacade) GetConversation(context.Context, string, string, string) (*cubeboxdomain.Conversation, error) {
	return nil, nil
}
func (s stubCubeBoxLegacyFacade) CreateConversation(context.Context, string, cubeboxservices.Principal) (*cubeboxdomain.Conversation, error) {
	return nil, nil
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
}

type runtimeHealthyFileStore struct {
	healthyErr error
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
