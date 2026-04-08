package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestAssistant240DHelperCoverage(t *testing.T) {
	now := time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC)
	turn := assistantTaskSampleTurn(now)
	turn.Intent.IntentSchemaVersion = " intent_v1 "
	turn.Plan.CompilerContractVersion = " compiler_v1 "
	turn.Plan.CapabilityMapVersion = " map_v1 "
	turn.Plan.SkillManifestDigest = " digest_v1 "
	turn.Intent.ContextHash = " ctx_hash "
	turn.Intent.IntentHash = " intent_hash "
	turn.DryRun.PlanHash = " plan_hash "
	turn.RequestID = " req_turn "
	turn.TraceID = " trace_turn "

	if snap := assistantBuildTaskSnapshotFromTurn(nil); snap != (assistantTaskContractSnapshot{}) {
		t.Fatalf("nil snapshot=%+v", snap)
	}
	snap := assistantBuildTaskSnapshotFromTurn(turn)
	if snap.IntentSchemaVersion != "intent_v1" || snap.CompilerContractVersion != "compiler_v1" || snap.PlanHash != "plan_hash" {
		t.Fatalf("snapshot=%+v", snap)
	}

	req, err := assistantBuildTaskSubmitRequestFromTurn(" conv_1 ", turn)
	if err != nil {
		t.Fatalf("build submit request err=%v", err)
	}
	if req.ConversationID != "conv_1" || req.TurnID != turn.TurnID || req.RequestID != "req_turn" || req.TraceID != "trace_turn" {
		t.Fatalf("submit request=%+v", req)
	}
	if _, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", nil); !errors.Is(err, errAssistantTurnNotFound) {
		t.Fatalf("nil turn err=%v", err)
	}
	badTurn := assistantTaskSampleTurn(now)
	badTurn.Plan.CompilerContractVersion = ""
	if _, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", badTurn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
		t.Fatalf("incomplete snapshot err=%v", err)
	}
	routeTurn := assistantTaskSampleTurn(now)
	routeTurn.RouteDecision = assistantIntentRouteDecision{
		RouteKind:               assistantRouteKindBusinessAction,
		IntentID:                "org.orgunit_create",
		CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
		ConfidenceBand:          assistantRouteConfidenceHigh,
		KnowledgeSnapshotDigest: "sha256:route",
		RouteCatalogVersion:     "2026-03-11.v1",
		ResolverContractVersion: "resolver_contract_v1",
		DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
	}
	routeTurn.Plan.KnowledgeSnapshotDigest = "sha256:route"
	routeTurn.Plan.RouteCatalogVersion = "2026-03-11.v1"
	routeTurn.Plan.ResolverContractVersion = "resolver_contract_v1"
	routeTurn.Plan.ContextTemplateVersion = assistantContextTemplateVersionV1
	routeTurn.Plan.ReplyGuidanceVersion = "2026-03-11.v1"
	if _, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", routeTurn); err != nil {
		t.Fatalf("route turn submit err=%v", err)
	}
	routeTurn.Plan.KnowledgeSnapshotDigest = "sha256:drift"
	if _, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", routeTurn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
		t.Fatalf("route audit mismatch err=%v", err)
	}

	if principal := assistantTaskExecutionPrincipal(nil); principal != (Principal{}) {
		t.Fatalf("nil principal=%+v", principal)
	}
	principal := assistantTaskExecutionPrincipal(&assistantConversation{ActorID: " actor_1 ", TenantID: " tenant_1 ", ActorRole: " admin "})
	if principal.ID != "actor_1" || principal.TenantID != "tenant_1" || principal.RoleSlug != "admin" {
		t.Fatalf("principal=%+v", principal)
	}

	if code := assistantTaskErrorCode(nil); code != "" {
		t.Fatalf("nil error code=%q", code)
	}
	if code := assistantTaskErrorCode(errors.New(" code_x ")); code != "code_x" {
		t.Fatalf("trimmed error code=%q", code)
	}
}

func TestAssistantIdempotencyTaskReceiptRestoreCoverage(t *testing.T) {
	if _, err := assistantRestoreTaskReceiptFromIdempotency(assistantIdempotencyClaim{ErrorCode: errAssistantConfirmationRequired.Error()}); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("error code restore err=%v", err)
	}
	if _, err := assistantRestoreTaskReceiptFromIdempotency(assistantIdempotencyClaim{}); !errors.Is(err, errAssistantRequestInProgress) {
		t.Fatalf("in progress err=%v", err)
	}
	if _, err := assistantRestoreTaskReceiptFromIdempotency(assistantIdempotencyClaim{Body: []byte("{")}); err == nil {
		t.Fatal("invalid json should fail")
	}

	expected := assistantTaskAsyncReceipt{
		TaskID:      "task_1",
		TaskType:    assistantTaskTypeAsyncPlan,
		Status:      assistantTaskStatusQueued,
		WorkflowID:  "wf_1",
		SubmittedAt: time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC),
		PollURI:     "/internal/assistant/tasks/task_1",
	}
	body, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal receipt err=%v", err)
	}
	restored, err := assistantRestoreTaskReceiptFromIdempotency(assistantIdempotencyClaim{Body: body})
	if err != nil {
		t.Fatalf("restore receipt err=%v", err)
	}
	if *restored != expected {
		t.Fatalf("restored=%+v expected=%+v", *restored, expected)
	}
}

func TestAssistantFinalizeIdempotencyJSONSuccessTxCoverage(t *testing.T) {
	svc := &assistantConversationService{}
	key := assistantIdempotencyKey{TenantID: "tenant_1", ConversationID: "conv_1", TurnID: "turn_1", TurnAction: "commit", RequestID: "req_1"}
	ctx := context.Background()
	if err := svc.finalizeIdempotencyJSONSuccessTx(ctx, &assistFakeTx{}, key, http.StatusAccepted, map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("marshal error should fail")
	}

	called := 0
	tx := &assistFakeTx{execFn: func(sql string, args ...any) (pgconn.CommandTag, error) {
		called++
		if !strings.Contains(sql, "UPDATE iam.assistant_idempotency") {
			t.Fatalf("sql=%s", sql)
		}
		if got := args[5]; got != http.StatusAccepted {
			t.Fatalf("http status arg=%v", got)
		}
		if body, ok := args[6].(string); !ok || !strings.Contains(body, "task_1") {
			t.Fatalf("body arg=%T %v", args[6], args[6])
		}
		if hash, ok := args[7].(string); !ok || strings.TrimSpace(hash) == "" {
			t.Fatalf("hash arg=%T %v", args[7], args[7])
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}}
	receipt := assistantTaskAsyncReceipt{TaskID: "task_1", TaskType: assistantTaskTypeAsyncPlan, Status: assistantTaskStatusQueued, WorkflowID: "wf_1", SubmittedAt: time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC), PollURI: "/internal/assistant/tasks/task_1"}
	if err := svc.finalizeIdempotencyJSONSuccessTx(ctx, tx, key, http.StatusAccepted, receipt); err != nil {
		t.Fatalf("finalize json success err=%v", err)
	}
	if called != 1 {
		t.Fatalf("exec called=%d", called)
	}
}
