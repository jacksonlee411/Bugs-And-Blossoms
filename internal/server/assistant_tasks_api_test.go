package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssistantTaskHandlers_CoverageMatrix(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	t.Run("submit handler branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks", "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", "{}", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", "{}", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", "{}", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", "{", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "bad_json" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", `{"conversation_id":"conv","turn_id":"turn","task_type":"assistant_async_plan","request_id":"req","contract_snapshot":{"intent_schema_version":"v1","compiler_contract_version":"v1","capability_map_version":"v1","skill_manifest_digest":"d","context_hash":"c","intent_hash":"i","plan_hash":"p"}}`, true, true), svc)
		if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "assistant_task_workflow_unavailable" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("detail handler branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1", "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/task/task-1", "", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", true, true), svc)
		if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "assistant_task_workflow_unavailable" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("action handler branches", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1:cancel", "", true, true), svc)
		if rec.Code != http.StatusMethodNotAllowed || assistantDecodeErrCode(t, rec) != "method_not_allowed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1:cancel", "", true, true), nil)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_service_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1:cancel", "", false, true), svc)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "tenant_missing" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1:cancel", "", true, false), svc)
		if rec.Code != http.StatusUnauthorized || assistantDecodeErrCode(t, rec) != "unauthorized" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1:retry", "", true, true), svc)
		if rec.Code != http.StatusBadRequest || assistantDecodeErrCode(t, rec) != "invalid_request" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}

		rec = httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1:cancel", "", true, true), svc)
		if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "assistant_task_workflow_unavailable" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})
}

func TestAssistantTaskPathExtractors(t *testing.T) {
	if taskID, ok := extractAssistantTaskIDFromPath("/internal/assistant/tasks/task-1"); !ok || taskID != "task-1" {
		t.Fatalf("extract task id failed: %s %v", taskID, ok)
	}
	if _, ok := extractAssistantTaskIDFromPath("/internal/assistant/tasks/ "); ok {
		t.Fatal("expected invalid empty task id")
	}
	if _, ok := extractAssistantTaskIDFromPath("/internal/assistant/task/task-1"); ok {
		t.Fatal("expected invalid task namespace")
	}

	taskID, action, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task-1:cancel")
	if !ok || taskID != "task-1" || action != "cancel" {
		t.Fatalf("extract task action failed: %s %s %v", taskID, action, ok)
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task-1"); ok {
		t.Fatal("expected invalid task action without separator")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/:cancel"); ok {
		t.Fatal("expected invalid empty task id")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task-1:"); ok {
		t.Fatal("expected invalid empty task action")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/task/task-1:cancel"); ok {
		t.Fatal("expected invalid action namespace")
	}
}

func TestAssistantTaskRequestValidationError(t *testing.T) {
	if !assistantTaskRequestValidationError(assertionError("task_type invalid")) {
		t.Fatal("task_type invalid should be validation error")
	}
	if assistantTaskRequestValidationError(assertionError("unexpected_error")) {
		t.Fatal("unexpected errors should not be treated as validation")
	}
}

type assertionError string

func (e assertionError) Error() string {
	return string(e)
}
