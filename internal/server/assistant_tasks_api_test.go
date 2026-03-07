package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

		svcWithPoolErr := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svcWithPoolErr.pool = assistFakeTxBeginner{err: assertionError("begin failed")}
		rec = httptest.NewRecorder()
		handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", `{"conversation_id":"conv","turn_id":"turn","task_type":"assistant_async_plan","request_id":"req","contract_snapshot":{"intent_schema_version":"v1","compiler_contract_version":"v1","capability_map_version":"v1","skill_manifest_digest":"d","context_hash":"c","intent_hash":"i","plan_hash":"p"}}`, true, true), svcWithPoolErr)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_task_dispatch_failed" {
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

		svcWithPoolErr := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svcWithPoolErr.pool = assistFakeTxBeginner{err: assertionError("begin failed")}
		rec = httptest.NewRecorder()
		handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", true, true), svcWithPoolErr)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_task_dispatch_failed" {
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

		svcWithPoolErr := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svcWithPoolErr.pool = assistFakeTxBeginner{err: assertionError("begin failed")}
		rec = httptest.NewRecorder()
		handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/task-1:cancel", "", true, true), svcWithPoolErr)
		if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_task_dispatch_failed" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})
}

func TestAssistantTaskPathExtractors(t *testing.T) {
	if taskID, ok := extractAssistantTaskIDFromPath("/internal/assistant/tasks/task-1"); !ok || taskID != "task-1" {
		t.Fatalf("extract task id failed: %s %v", taskID, ok)
	}
	if _, ok := extractAssistantTaskIDFromPath("/internal/assistant/tasks"); ok {
		t.Fatal("expected invalid path without id segment")
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
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/"); ok {
		t.Fatal("expected invalid empty task action segment")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/:cancel"); ok {
		t.Fatal("expected invalid empty task id")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task-1:"); ok {
		t.Fatal("expected invalid empty task action")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/   :cancel"); ok {
		t.Fatal("expected invalid trimmed-empty task id")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task-1:   "); ok {
		t.Fatal("expected invalid trimmed-empty task action")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/task/task-1:cancel"); ok {
		t.Fatal("expected invalid action namespace")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task-1:cancel/extra"); ok {
		t.Fatal("expected invalid action path with extra segment")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/ "); ok {
		t.Fatal("expected invalid empty action segment after trim")
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

func TestAssistantWriteTaskError_Mappings(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
		wantHTTP int
	}{
		{name: "conversation_not_found", err: errAssistantConversationNotFound, wantCode: "conversation_not_found", wantHTTP: http.StatusNotFound},
		{name: "turn_not_found", err: errAssistantTurnNotFound, wantCode: "conversation_turn_not_found", wantHTTP: http.StatusNotFound},
		{name: "task_not_found", err: errAssistantTaskNotFound, wantCode: "assistant_task_not_found", wantHTTP: http.StatusNotFound},
		{name: "forbidden", err: errAssistantConversationForbidden, wantCode: "forbidden", wantHTTP: http.StatusForbidden},
		{name: "tenant_mismatch", err: errAssistantTenantMismatch, wantCode: "tenant_mismatch", wantHTTP: http.StatusForbidden},
		{name: "workflow_unavailable", err: errAssistantTaskWorkflowUnavailable, wantCode: "assistant_task_workflow_unavailable", wantHTTP: http.StatusServiceUnavailable},
		{name: "idempotency_conflict", err: errAssistantIdempotencyKeyConflict, wantCode: "idempotency_key_conflict", wantHTTP: http.StatusConflict},
		{name: "request_in_progress", err: errAssistantRequestInProgress, wantCode: "request_in_progress", wantHTTP: http.StatusConflict},
		{name: "cancel_not_allowed", err: errAssistantTaskCancelNotAllowed, wantCode: "assistant_task_cancel_not_allowed", wantHTTP: http.StatusConflict},
		{name: "state_invalid", err: errAssistantTaskStateInvalid, wantCode: "assistant_task_state_invalid", wantHTTP: http.StatusConflict},
		{name: "plan_contract_mismatch", err: errAssistantPlanContractVersionMismatch, wantCode: "ai_plan_contract_version_mismatch", wantHTTP: http.StatusConflict},
		{name: "plan_determinism", err: errAssistantPlanDeterminismViolation, wantCode: "ai_plan_determinism_violation", wantHTTP: http.StatusConflict},
		{name: "request_validation", err: assertionError("request_id required"), wantCode: "invalid_request", wantHTTP: http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", true, true)
			rec := httptest.NewRecorder()
			if !assistantWriteTaskError(rec, req, tc.err) {
				t.Fatalf("expected mapping for err=%v", tc.err)
			}
			if rec.Code != tc.wantHTTP || assistantDecodeErrCode(t, rec) != tc.wantCode {
				t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
			}
			if tc.err == errAssistantRequestInProgress && rec.Header().Get("Retry-After") == "" {
				t.Fatal("request_in_progress should set Retry-After")
			}
		})
	}

	rec := httptest.NewRecorder()
	req := assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/task-1", "", true, true)
	if assistantWriteTaskError(rec, req, errors.New("unknown")) {
		t.Fatal("unknown error should not be mapped")
	}
}

func TestAssistantTaskHandlers_SuccessPaths(t *testing.T) {
	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	reqBody, _ := json.Marshal(assistantTaskSampleRequest(turn))

	submitSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	submitTx := &assistFakeTx{}
	submitTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_conversations"):
			return &assistFakeRow{vals: []any{"conv_1", "tenant-1", "actor-1", "tenant-admin", assistantStateValidated, assistantConversationPhaseFromLegacyState(assistantStateValidated), now, now}}
		case strings.Contains(sql, "FROM iam.assistant_tasks"):
			return &assistFakeRow{err: pgx.ErrNoRows}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	submitTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_turns"):
			return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil
		case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
			return &assistFakeRows{}, nil
		default:
			return &assistFakeRows{}, nil
		}
	}
	submitTx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
	submitSvc.pool = assistFakeTxBeginner{tx: submitTx}

	rec := httptest.NewRecorder()
	handleAssistantTasksAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks", string(reqBody), true, true), submitSvc)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit status=%d body=%s", rec.Code, rec.Body.String())
	}

	taskRecord := assistantTaskRecord{
		TaskID:             "9cca7883-af3a-4289-9606-9e4cb4232a3a",
		TenantID:           "tenant-1",
		ConversationID:     "conv_1",
		TurnID:             "turn_1",
		TaskType:           assistantTaskTypeAsyncPlan,
		RequestID:          "req_1",
		RequestHash:        "hash",
		WorkflowID:         "wf_1",
		Status:             assistantTaskStatusQueued,
		DispatchStatus:     assistantTaskDispatchPending,
		DispatchDeadlineAt: now.Add(time.Minute),
		MaxAttempts:        3,
		ContractSnapshot:   assistantTaskSnapshotFromTurn(turn),
		SubmittedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	getSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	beginCount := 0
	getSvc.pool = assistTaskTxBeginner{beginFn: func(context.Context, pgx.TxOptions) (pgx.Tx, error) {
		beginCount++
		if beginCount == 1 {
			dispatchTx := &assistFakeTx{}
			dispatchTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{}, nil
				}
				return &assistFakeRows{}, nil
			}
			dispatchTx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
			return dispatchTx, nil
		}
		mainTx := &assistFakeTx{}
		mainTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(taskRecord)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor-1"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		return mainTx, nil
	}}
	rec = httptest.NewRecorder()
	handleAssistantTaskDetailAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/tasks/"+taskRecord.TaskID, "", true, true), getSvc)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", rec.Code, rec.Body.String())
	}

	cancelSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	cancelTx := &assistFakeTx{}
	cancelTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_tasks"):
			return &assistFakeRow{vals: assistantTaskRowValues(taskRecord)}
		case strings.Contains(sql, "FROM iam.assistant_conversations"):
			return &assistFakeRow{vals: []any{"actor-1"}}
		default:
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
	}
	cancelTx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
	cancelSvc.pool = assistFakeTxBeginner{tx: cancelTx}
	rec = httptest.NewRecorder()
	handleAssistantTaskActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/tasks/"+taskRecord.TaskID+":cancel", "", true, true), cancelSvc)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("cancel status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type assertionError string

func (e assertionError) Error() string {
	return string(e)
}
