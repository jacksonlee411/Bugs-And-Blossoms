package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistTaskTxBeginner struct {
	beginFn func(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

func (b assistTaskTxBeginner) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return b.beginFn(ctx, opts)
}

func assistantTestCreateOrgUnitProjectionSnapshot() *assistantCreateOrgUnitProjectionSnapshot {
	return &assistantCreateOrgUnitProjectionSnapshot{
		PolicyContextContractVersion:      orgunitservices.CreateOrgUnitPolicyContextContractVersionV1,
		PrecheckProjectionContractVersion: orgunitservices.CreateOrgUnitPrecheckProjectionContractV1,
		PolicyContext: orgunitservices.CreateOrgUnitPolicyContextV1{
			TenantID:            "tenant_1",
			CapabilityKey:       orgUnitCreateFieldPolicyCapabilityKey,
			EffectiveDate:       "2026-01-01",
			BusinessUnitOrgCode: "FLOWER-A",
			BusinessUnitNodeKey: "10000001",
			ResolvedSetID:       "S2601",
			SetIDSource:         "custom",
			PolicyContextDigest: "ctx_digest",
		},
		Projection: orgunitservices.CreateOrgUnitPrecheckProjectionV1{
			Readiness:              "ready",
			FieldDecisions:         []orgunitservices.CreateOrgUnitFieldDecisionV1{{FieldKey: "name", Visible: true, Maintainable: true, FieldPayloadKey: "name", AllowedValueCodes: []string{}}},
			PendingDraftSummary:    "上级组织：FLOWER-A；新建组织：运营部；生效日期：2026-01-01",
			EffectivePolicyVersion: "epv1:test",
			MutationPolicyVersion:  orgunitservices.CreateOrgUnitMutationPolicyVersionV1,
			ResolvedSetID:          "S2601",
			SetIDSource:            "custom",
			PolicyExplain:          "计划已生成，等待确认后可提交",
			ProjectionDigest:       "projection_digest",
		},
	}
}

func assistantTestOrgUnitVersionProjectionSnapshot(action string) *assistantOrgUnitVersionProjectionSnapshot {
	action = strings.TrimSpace(action)
	capabilityKey := orgUnitAddVersionFieldPolicyCapabilityKey
	intent := string(orgunitservices.OrgUnitWriteIntentAddVersion)
	effectiveDate := "2026-01-01"
	targetEffectiveDate := ""
	fieldKey := "name"
	fieldPayloadKey := "name"
	pendingDraftSummary := "目标组织：FLOWER-C；新名称：运营一部；生效日期：2026-01-01"
	policyContextContractVersion := orgunitservices.OrgUnitAppendVersionPolicyContextContractVersionV1
	precheckProjectionContractVersion := orgunitservices.OrgUnitAppendVersionPrecheckProjectionContractV1
	mutationPolicyVersion := orgunitservices.OrgUnitAppendVersionMutationPolicyVersionV1
	switch action {
	case assistantIntentInsertOrgUnitVersion:
		capabilityKey = orgUnitInsertVersionFieldPolicyCapabilityKey
		intent = string(orgunitservices.OrgUnitWriteIntentInsertVersion)
		pendingDraftSummary = "目标组织：FLOWER-C；插入版本名称：运营二部；生效日期：2026-01-01"
	case assistantIntentCorrectOrgUnit:
		capabilityKey = orgUnitCorrectFieldPolicyCapabilityKey
		intent = orgunitservices.OrgUnitMaintainIntentCorrect
		effectiveDate = ""
		targetEffectiveDate = "2026-01-01"
		pendingDraftSummary = "目标组织：FLOWER-C；目标生效日期：2026-01-01；更正名称：运营中心"
		policyContextContractVersion = orgunitservices.OrgUnitMaintainPolicyContextContractVersionV1
		precheckProjectionContractVersion = orgunitservices.OrgUnitMaintainPrecheckProjectionContractV1
		mutationPolicyVersion = orgunitservices.OrgUnitMaintainMutationPolicyVersionV1
	case assistantIntentRenameOrgUnit:
		capabilityKey = orgUnitWriteFieldPolicyCapabilityKey
		intent = orgunitservices.OrgUnitMaintainIntentRename
		effectiveDate = "2026-03-01"
		pendingDraftSummary = "目标组织：FLOWER-C；新名称：运营平台部；生效日期：2026-03-01"
		policyContextContractVersion = orgunitservices.OrgUnitMaintainPolicyContextContractVersionV1
		precheckProjectionContractVersion = orgunitservices.OrgUnitMaintainPrecheckProjectionContractV1
		mutationPolicyVersion = orgunitservices.OrgUnitMaintainMutationPolicyVersionV1
	case assistantIntentMoveOrgUnit:
		capabilityKey = orgUnitWriteFieldPolicyCapabilityKey
		intent = orgunitservices.OrgUnitMaintainIntentMove
		effectiveDate = "2026-04-01"
		fieldKey = "parent_org_code"
		fieldPayloadKey = "new_parent_org_code"
		pendingDraftSummary = "目标组织：FLOWER-C；新上级：FLOWER-A；生效日期：2026-04-01"
		policyContextContractVersion = orgunitservices.OrgUnitMaintainPolicyContextContractVersionV1
		precheckProjectionContractVersion = orgunitservices.OrgUnitMaintainPrecheckProjectionContractV1
		mutationPolicyVersion = orgunitservices.OrgUnitMaintainMutationPolicyVersionV1
	}
	return &assistantOrgUnitVersionProjectionSnapshot{
		PolicyContextContractVersion:      policyContextContractVersion,
		PrecheckProjectionContractVersion: precheckProjectionContractVersion,
		PolicyContext: assistantOrgUnitVersionPolicyContext{
			TenantID:            "tenant_1",
			CapabilityKey:       capabilityKey,
			Intent:              intent,
			EffectiveDate:       effectiveDate,
			TargetEffectiveDate: targetEffectiveDate,
			OrgCode:             "FLOWER-C",
			OrgNodeKey:          "10000003",
			ResolvedSetID:       "S2601",
			SetIDSource:         "custom",
			PolicyContextDigest: "ctx_digest_" + action,
		},
		Projection: assistantOrgUnitVersionProjection{
			Readiness:              "ready",
			FieldDecisions:         []assistantOrgUnitVersionFieldDecision{{FieldKey: fieldKey, Visible: true, Maintainable: true, FieldPayloadKey: fieldPayloadKey, AllowedValueCodes: []string{}}},
			PendingDraftSummary:    pendingDraftSummary,
			EffectivePolicyVersion: "epv1:" + action,
			MutationPolicyVersion:  mutationPolicyVersion,
			ResolvedSetID:          "S2601",
			SetIDSource:            "custom",
			PolicyExplain:          "计划已生成，等待确认后可提交",
			ProjectionDigest:       "projection_digest_" + action,
		},
	}
}

func assistantTaskSampleTurn(now time.Time) *assistantTurn {
	intent := assistantIntentSpec{
		Action:              assistantIntentCreateOrgUnit,
		IntentSchemaVersion: assistantIntentSchemaVersionV1,
		ContextHash:         "ctx_hash",
		IntentHash:          "intent_hash",
		EffectiveDate:       "2026-01-01",
	}
	plan := assistantBuildPlan(intent)
	plan.SkillManifestDigest = "skill_digest"
	return assistantTestAttachBusinessRoute(&assistantTurn{
		TurnID:             "turn_1",
		UserInput:          "创建组织",
		State:              assistantStateValidated,
		RiskTier:           "high",
		RequestID:          "req_turn",
		TraceID:            "trace_turn",
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		Intent:             intent,
		Plan:               plan,
		Candidates: []assistantCandidate{
			{CandidateID: "c1", CandidateCode: "FLOWER-A", Name: "A"},
		},
		ResolvedCandidateID: "c1",
		AmbiguityCount:      0,
		Confidence:          0.9,
		ResolutionSource:    "auto",
		DryRun: assistantDryRunResult{
			WouldCommit:             false,
			PlanHash:                "plan_hash",
			CreateOrgUnitProjection: assistantTestCreateOrgUnitProjectionSnapshot(),
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func assistantTaskSampleAppendTurn(now time.Time, action string) *assistantTurn {
	action = strings.TrimSpace(action)
	intent := assistantIntentSpec{
		Action:              action,
		IntentSchemaVersion: assistantIntentSchemaVersionV1,
		ContextHash:         "ctx_hash",
		IntentHash:          "intent_hash",
		OrgCode:             "FLOWER-C",
	}
	userInput := "新增组织版本"
	switch action {
	case assistantIntentCorrectOrgUnit:
		intent.TargetEffectiveDate = "2026-01-01"
		intent.NewName = "运营中心"
		userInput = "更正组织"
	case assistantIntentRenameOrgUnit:
		intent.EffectiveDate = "2026-03-01"
		intent.NewName = "运营平台部"
		userInput = "重命名组织"
	case assistantIntentMoveOrgUnit:
		intent.EffectiveDate = "2026-04-01"
		intent.NewParentRefText = "鲜花组织"
		userInput = "移动组织"
	case assistantIntentInsertOrgUnitVersion:
		intent.EffectiveDate = "2026-01-01"
		intent.NewName = "运营二部"
		userInput = "插入组织版本"
	default:
		intent.EffectiveDate = "2026-01-01"
		intent.NewName = "运营一部"
	}
	plan := assistantBuildPlan(intent)
	plan.SkillManifestDigest = "skill_digest"
	return assistantTestAttachOrgUnitVersionProjection(&assistantTurn{
		TurnID:             "turn_append_1",
		UserInput:          userInput,
		State:              assistantStateValidated,
		RiskTier:           "high",
		RequestID:          "req_turn",
		TraceID:            "trace_turn",
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		Intent:             intent,
		Plan:               plan,
		Candidates: []assistantCandidate{
			{CandidateID: "c1", CandidateCode: "FLOWER-A", Name: "A"},
		},
		ResolvedCandidateID: "c1",
		AmbiguityCount:      0,
		Confidence:          0.9,
		ResolutionSource:    "auto",
		DryRun: assistantDryRunResult{
			WouldCommit:              false,
			PlanHash:                 "plan_hash",
			OrgUnitVersionProjection: assistantTestOrgUnitVersionProjectionSnapshot(action),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil)
}

func assistantTaskSnapshotFromTurn(turn *assistantTurn) assistantTaskContractSnapshot {
	return assistantBuildTaskSnapshotFromTurn(turn)
}

func assistantTaskSampleRequest(turn *assistantTurn) assistantTaskSubmitRequest {
	return assistantTaskSubmitRequest{
		ConversationID:   "conv_1",
		TurnID:           turn.TurnID,
		TaskType:         assistantTaskTypeAsyncPlan,
		RequestID:        "task_req_1",
		TraceID:          "trace_1",
		ContractSnapshot: assistantTaskSnapshotFromTurn(turn),
	}
}

func assistantTaskRowValues(record assistantTaskRecord) []any {
	lastError := sql.NullString{}
	if strings.TrimSpace(record.LastErrorCode) != "" {
		lastError = sql.NullString{String: record.LastErrorCode, Valid: true}
	}
	traceID := sql.NullString{}
	if strings.TrimSpace(record.TraceID) != "" {
		traceID = sql.NullString{String: record.TraceID, Valid: true}
	}
	var cancelRequestedAt any
	if record.CancelRequestedAt != nil {
		cancelRequestedAt = *record.CancelRequestedAt
	}
	var completedAt any
	if record.CompletedAt != nil {
		completedAt = *record.CompletedAt
	}
	return []any{
		record.TaskID,
		record.TenantID,
		record.ConversationID,
		record.TurnID,
		record.TaskType,
		record.RequestID,
		record.RequestHash,
		record.WorkflowID,
		record.Status,
		record.DispatchStatus,
		record.DispatchAttempt,
		record.DispatchDeadlineAt,
		record.Attempt,
		record.MaxAttempts,
		lastError,
		traceID,
		record.ContractSnapshot.IntentSchemaVersion,
		record.ContractSnapshot.CompilerContractVersion,
		record.ContractSnapshot.CapabilityMapVersion,
		record.ContractSnapshot.SkillManifestDigest,
		record.ContractSnapshot.ContextHash,
		record.ContractSnapshot.IntentHash,
		record.ContractSnapshot.PlanHash,
		record.SubmittedAt,
		cancelRequestedAt,
		completedAt,
		record.CreatedAt,
		record.UpdatedAt,
	}
}

func mustJSONMarshal(t *testing.T, v any) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal err=%v", err)
	}
	return data
}

func TestAssistantTaskStore_UtilityValidationAndWrappers(t *testing.T) {
	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	req := assistantTaskSampleRequest(turn)

	t.Run("utility helpers", func(t *testing.T) {
		if !assistantTaskStatusTerminal(assistantTaskStatusSucceeded) || assistantTaskStatusTerminal("running") {
			t.Fatal("assistantTaskStatusTerminal mismatch")
		}
		if !assistantTaskStatusCancellable(assistantTaskStatusQueued) || assistantTaskStatusCancellable(assistantTaskStatusSucceeded) {
			t.Fatal("assistantTaskStatusCancellable mismatch")
		}
		if assistantTaskDispatchBackoff(0) != 300*time.Millisecond {
			t.Fatalf("unexpected backoff for 0")
		}
		if assistantTaskDispatchBackoff(2) != 600*time.Millisecond {
			t.Fatalf("unexpected backoff for 2")
		}
		if assistantTaskDispatchBackoff(100) != 2*time.Second {
			t.Fatalf("unexpected backoff cap")
		}
		workflowID := assistantTaskWorkflowID(" t1 ", " c1 ", " turn1 ", " req1 ")
		if workflowID != "assistant_async_orchestration_v1:t1:c1:turn1:req1" {
			t.Fatalf("workflow id=%s", workflowID)
		}

		hash1, err := assistantTaskRequestHash(req)
		if err != nil || strings.TrimSpace(hash1) == "" {
			t.Fatalf("request hash err=%v hash=%q", err, hash1)
		}
		req2 := req
		req2.RequestID = "task_req_2"
		hash2, err := assistantTaskRequestHash(req2)
		if err != nil || hash1 == hash2 {
			t.Fatalf("request hash not changed err=%v h1=%s h2=%s", err, hash1, hash2)
		}
		origMarshalFn := assistantTaskMarshalFn
		assistantTaskMarshalFn = func(any) ([]byte, error) { return nil, fmt.Errorf("marshal failed") }
		if _, err := assistantTaskRequestHash(req); err == nil || !strings.Contains(err.Error(), "marshal failed") {
			t.Fatalf("expected marshal failure err=%v", err)
		}
		assistantTaskMarshalFn = origMarshalFn
	})

	t.Run("submit request and snapshot validation", func(t *testing.T) {
		cases := []struct {
			name string
			req  assistantTaskSubmitRequest
			want string
		}{
			{name: "missing_conversation", req: assistantTaskSubmitRequest{}, want: "conversation_id required"},
			{name: "missing_turn", req: assistantTaskSubmitRequest{ConversationID: "conv"}, want: "turn_id required"},
			{name: "missing_task_type", req: assistantTaskSubmitRequest{ConversationID: "conv", TurnID: "turn"}, want: "task_type required"},
			{name: "invalid_task_type", req: assistantTaskSubmitRequest{ConversationID: "conv", TurnID: "turn", TaskType: "x"}, want: "task_type invalid"},
			{name: "missing_request_id", req: assistantTaskSubmitRequest{ConversationID: "conv", TurnID: "turn", TaskType: assistantTaskTypeAsyncPlan}, want: "request_id required"},
			{name: "incomplete_snapshot", req: assistantTaskSubmitRequest{ConversationID: "conv", TurnID: "turn", TaskType: assistantTaskTypeAsyncPlan, RequestID: "r1"}, want: "contract_snapshot incomplete"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := assistantTaskValidateSubmitRequest(tc.req); err == nil || err.Error() != tc.want {
					t.Fatalf("err=%v want=%s", err, tc.want)
				}
			})
		}
		if err := assistantTaskValidateSubmitRequest(req); err != nil {
			t.Fatalf("valid req err=%v", err)
		}

		if err := assistantTaskValidateSnapshotAgainstTurn(req.ContractSnapshot, nil); !errors.Is(err, errAssistantTurnNotFound) {
			t.Fatalf("unexpected nil turn err=%v", err)
		}
		badSnap := req.ContractSnapshot
		badSnap.PlanHash = "other"
		if err := assistantTaskValidateSnapshotAgainstTurn(badSnap, turn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("unexpected mismatch err=%v", err)
		}
		routeAuditDriftTurn := *turn
		routeAuditDriftTurn.Plan = turn.Plan
		routeAuditDriftTurn.Plan.KnowledgeSnapshotDigest = "sha256:route"
		routeAuditDriftTurn.Plan.RouteCatalogVersion = "2026-03-11.v1"
		routeAuditDriftTurn.Plan.ResolverContractVersion = "resolver_contract_v1"
		routeAuditDriftTurn.Plan.ContextTemplateVersion = assistantContextTemplateVersionV1
		routeAuditDriftTurn.Plan.ReplyGuidanceVersion = "2026-03-11.v1"
		routeAuditDriftTurn.RouteDecision = assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			KnowledgeSnapshotDigest: "sha256:route",
			RouteCatalogVersion:     "2026-03-11.v1",
			ResolverContractVersion: "resolver_contract_v1",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
		}
		routeAuditDriftTurn.Plan.ContextTemplateVersion = ""
		if err := assistantTaskValidateSnapshotAgainstTurn(req.ContractSnapshot, &routeAuditDriftTurn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected route audit mismatch err=%v", err)
		}
		if err := assistantTaskValidateSnapshotAgainstTurn(req.ContractSnapshot, turn); err != nil {
			t.Fatalf("snapshot should match err=%v", err)
		}

		appendTurn := assistantTaskSampleAppendTurn(now, assistantIntentAddOrgUnitVersion)
		appendReq, err := assistantBuildTaskSubmitRequestFromTurn("conv_append", appendTurn)
		if err != nil {
			t.Fatalf("append submit request err=%v", err)
		}
		if appendReq.ContractSnapshot.PolicyContextDigest == "" || appendReq.ContractSnapshot.MutationPolicyVersion == "" {
			t.Fatalf("append snapshot should include policy contract=%+v", appendReq.ContractSnapshot)
		}
		appendBadSnap := appendReq.ContractSnapshot
		appendBadSnap.MutationPolicyVersion = ""
		if err := assistantTaskValidateSnapshotAgainstTurn(appendBadSnap, appendTurn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected append policy mismatch err=%v", err)
		}
	})

	t.Run("record adapters and wrapper forwarding", func(t *testing.T) {
		record := assistantTaskRecord{
			TaskID:            "task_1",
			TaskType:          assistantTaskTypeAsyncPlan,
			Status:            assistantTaskStatusQueued,
			WorkflowID:        "wf_1",
			SubmittedAt:       now,
			DispatchStatus:    assistantTaskDispatchPending,
			Attempt:           1,
			MaxAttempts:       3,
			RequestID:         "req_1",
			TraceID:           "trace_1",
			ConversationID:    "conv_1",
			TurnID:            "turn_1",
			UpdatedAt:         now,
			ContractSnapshot:  req.ContractSnapshot,
			CancelRequestedAt: &now,
			CompletedAt:       &now,
		}
		receipt := assistantTaskReceiptFromRecord(record)
		if receipt.TaskID != "task_1" || receipt.PollURI != "/internal/assistant/tasks/task_1" {
			t.Fatalf("unexpected receipt=%+v", receipt)
		}
		detail := assistantTaskDetailFromRecord(record)
		if detail.TaskID != "task_1" || detail.TraceID != "trace_1" || detail.ContractSnapshot.PlanHash == "" {
			t.Fatalf("unexpected detail=%+v", detail)
		}

		var nilSvc *assistantConversationService
		if _, err := nilSvc.submitTask(context.Background(), "tenant_1", Principal{ID: "actor_1"}, req); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
			t.Fatalf("nil submit err=%v", err)
		}
		if _, err := nilSvc.getTask(context.Background(), "tenant_1", Principal{ID: "actor_1"}, "task_1"); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
			t.Fatalf("nil get err=%v", err)
		}
		if _, err := nilSvc.cancelTask(context.Background(), "tenant_1", Principal{ID: "actor_1"}, "task_1"); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
			t.Fatalf("nil cancel err=%v", err)
		}

		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
		if _, err := svc.submitTask(context.Background(), "tenant_1", Principal{ID: "actor_1"}, req); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("submit wrapper should forward to pg err=%v", err)
		}
		if _, err := svc.getTask(context.Background(), "tenant_1", Principal{ID: "actor_1"}, "task_1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("get wrapper should forward to pg err=%v", err)
		}
		if _, err := svc.cancelTask(context.Background(), "tenant_1", Principal{ID: "actor_1"}, "task_1"); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("cancel wrapper should forward to pg err=%v", err)
		}
	})
}

func TestAssistantTaskStore_RecordScanAndSQLHelpers(t *testing.T) {
	now := time.Now().UTC()
	record := assistantTaskRecord{
		TaskID:             "8fdad8d2-6bd2-4710-86e3-8b53537def11",
		TenantID:           "tenant_1",
		ConversationID:     "conv_1",
		TurnID:             "turn_1",
		TaskType:           assistantTaskTypeAsyncPlan,
		RequestID:          "req_1",
		RequestHash:        "hash_1",
		WorkflowID:         "wf_1",
		Status:             assistantTaskStatusQueued,
		DispatchStatus:     assistantTaskDispatchPending,
		DispatchAttempt:    0,
		DispatchDeadlineAt: now.Add(time.Minute),
		Attempt:            0,
		MaxAttempts:        3,
		LastErrorCode:      "err_code",
		TraceID:            "trace_1",
		ContractSnapshot: assistantTaskContractSnapshot{
			IntentSchemaVersion:     assistantIntentSchemaVersionV1,
			CompilerContractVersion: assistantCompilerContractVersionV1,
			CapabilityMapVersion:    assistantCapabilityMapVersionV1,
			SkillManifestDigest:     "digest",
			ContextHash:             "ctx",
			IntentHash:              "intent",
			PlanHash:                "plan",
		},
		SubmittedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	t.Run("scan and load helpers", func(t *testing.T) {
		if _, exists, err := scanAssistantTaskRecord(&assistFakeRow{err: pgx.ErrNoRows}); err != nil || exists {
			t.Fatalf("scan no rows err=%v exists=%v", err, exists)
		}
		if _, _, err := scanAssistantTaskRecord(&assistFakeRow{err: errors.New("scan failed")}); err == nil {
			t.Fatal("expected scan error")
		}
		got, exists, err := scanAssistantTaskRecord(&assistFakeRow{vals: assistantTaskRowValues(record)})
		if err != nil || !exists || got.TaskID != record.TaskID || got.LastErrorCode != "err_code" {
			t.Fatalf("scan record err=%v exists=%v got=%+v", err, exists, got)
		}

		tx := &assistFakeTx{}
		var sqlSeen string
		tx.queryRowFn = func(sqlText string, _ ...any) pgx.Row {
			sqlSeen = sqlText
			return &assistFakeRow{vals: assistantTaskRowValues(record)}
		}
		if _, exists, err := svc.loadAssistantTaskBySubmitKeyTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1", "req_1", false); err != nil || !exists || strings.Contains(sqlSeen, "FOR UPDATE") {
			t.Fatalf("submit key query err=%v exists=%v sql=%s", err, exists, sqlSeen)
		}
		if _, exists, err := svc.loadAssistantTaskBySubmitKeyTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1", "req_1", true); err != nil || !exists || !strings.Contains(sqlSeen, "FOR UPDATE") {
			t.Fatalf("submit key for update err=%v exists=%v sql=%s", err, exists, sqlSeen)
		}

		if _, exists, err := svc.loadAssistantTaskByIDTx(context.Background(), tx, "tenant_1", record.TaskID, false); err != nil || !exists || strings.Contains(sqlSeen, "FOR UPDATE") {
			t.Fatalf("id query err=%v exists=%v sql=%s", err, exists, sqlSeen)
		}
		if _, exists, err := svc.loadAssistantTaskByIDTx(context.Background(), tx, "tenant_1", record.TaskID, true); err != nil || !exists || !strings.Contains(sqlSeen, "FOR UPDATE") {
			t.Fatalf("id for update err=%v exists=%v sql=%s", err, exists, sqlSeen)
		}
	})

	t.Run("write state and outbox helpers", func(t *testing.T) {
		tx := &assistFakeTx{}
		tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		if err := svc.insertAssistantTaskTx(context.Background(), tx, record); err != nil {
			t.Fatalf("insert task err=%v", err)
		}
		if err := svc.updateAssistantTaskStateTx(context.Background(), tx, record); err != nil {
			t.Fatalf("update task err=%v", err)
		}
		if err := svc.insertAssistantTaskOutboxTx(context.Background(), tx, "tenant_1", record.TaskID, "wf_1", now); err != nil {
			t.Fatalf("insert outbox err=%v", err)
		}
		if err := svc.updateAssistantTaskOutboxStateTx(context.Background(), tx, 1, assistantTaskDispatchPending, 1, now, now); err != nil {
			t.Fatalf("update outbox err=%v", err)
		}
		if err := svc.markAssistantTaskOutboxCanceledTx(context.Background(), tx, "tenant_1", record.TaskID, now); err != nil {
			t.Fatalf("cancel outbox err=%v", err)
		}
	})

	t.Run("actor and event helpers", func(t *testing.T) {
		tx := &assistFakeTx{}
		tx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: pgx.ErrNoRows} }
		if err := svc.ensureAssistantTaskActorTx(context.Background(), tx, "tenant_1", "conv_1", "actor_1"); !errors.Is(err, errAssistantConversationNotFound) {
			t.Fatalf("unexpected ensure actor no rows err=%v", err)
		}
		tx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: errors.New("query actor failed")} }
		if err := svc.ensureAssistantTaskActorTx(context.Background(), tx, "tenant_1", "conv_1", "actor_1"); err == nil || !strings.Contains(err.Error(), "query actor failed") {
			t.Fatalf("unexpected ensure actor query err=%v", err)
		}
		tx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{vals: []any{"actor_2"}} }
		if err := svc.ensureAssistantTaskActorTx(context.Background(), tx, "tenant_1", "conv_1", "actor_1"); !errors.Is(err, errAssistantConversationForbidden) {
			t.Fatalf("unexpected ensure actor mismatch err=%v", err)
		}
		tx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{vals: []any{"actor_1"}} }
		if err := svc.ensureAssistantTaskActorTx(context.Background(), tx, "tenant_1", "conv_1", "actor_1"); err != nil {
			t.Fatalf("ensure actor success err=%v", err)
		}

		if err := svc.insertAssistantTaskEventTx(context.Background(), tx, "tenant_1", record.TaskID, "", assistantTaskStatusQueued, "queued", "", map[string]any{"x": make(chan int)}, now); err == nil {
			t.Fatal("expected payload marshal err")
		}
		if err := svc.insertAssistantTaskEventTx(context.Background(), tx, "tenant_1", record.TaskID, "", assistantTaskStatusQueued, "queued", "", nil, now); err != nil {
			t.Fatalf("insert event nil payload err=%v", err)
		}
		if err := svc.insertAssistantTaskEventTx(context.Background(), tx, "tenant_1", record.TaskID, assistantTaskStatusQueued, assistantTaskStatusRunning, "running", "err", map[string]any{"ok": true}, now); err != nil {
			t.Fatalf("insert event with payload err=%v", err)
		}
	})

	t.Run("outbox query and snapshot helpers", func(t *testing.T) {
		tx := &assistFakeTx{}
		tx.queryFn = func(string, ...any) (pgx.Rows, error) { return nil, errors.New("query failed") }
		if _, err := svc.selectAssistantTaskOutboxPendingTx(context.Background(), tx, 5); err == nil {
			t.Fatal("expected select outbox query err")
		}
		tx.queryFn = func(string, ...any) (pgx.Rows, error) {
			return &assistFakeRows{rows: [][]any{{"bad"}}}, nil
		}
		if _, err := svc.selectAssistantTaskOutboxPendingTx(context.Background(), tx, 5); err == nil {
			t.Fatal("expected select outbox scan err")
		}
		tx.queryFn = func(string, ...any) (pgx.Rows, error) {
			return &assistFakeRows{rows: [][]any{
				{int64(1), "tenant_1", record.TaskID, "wf_1", assistantTaskDispatchPending, 0, now, now, now},
			}}, nil
		}
		outbox, err := svc.selectAssistantTaskOutboxPendingTx(context.Background(), tx, 5)
		if err != nil || len(outbox) != 1 || outbox[0].TaskID != record.TaskID {
			t.Fatalf("select outbox err=%v outbox=%+v", err, outbox)
		}
		tx.queryFn = func(string, ...any) (pgx.Rows, error) {
			return &assistFakeRows{err: errors.New("rows err")}, nil
		}
		if _, err := svc.selectAssistantTaskOutboxPendingTx(context.Background(), tx, 5); err == nil || !strings.Contains(err.Error(), "rows err") {
			t.Fatalf("expected rows err, got=%v", err)
		}

		tx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: errors.New("select turn failed")} }
		if _, err := svc.loadAssistantTurnContractSnapshotTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1"); err == nil {
			t.Fatal("expected load snapshot query err")
		}
		tx.queryRowFn = func(string, ...any) pgx.Row {
			return &assistFakeRow{vals: []any{[]byte("{"), []byte("{}"), []byte("{}")}}
		}
		if _, err := svc.loadAssistantTurnContractSnapshotTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1"); err == nil {
			t.Fatal("expected intent unmarshal err")
		}
		intentJSON, _ := json.Marshal(assistantIntentSpec{IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx", IntentHash: "intent"})
		tx.queryRowFn = func(string, ...any) pgx.Row {
			return &assistFakeRow{vals: []any{intentJSON, []byte("{"), []byte("{}")}}
		}
		if _, err := svc.loadAssistantTurnContractSnapshotTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1"); err == nil {
			t.Fatal("expected plan unmarshal err")
		}
		planJSON, _ := json.Marshal(assistantPlanSummary{CompilerContractVersion: assistantCompilerContractVersionV1, CapabilityMapVersion: assistantCapabilityMapVersionV1, SkillManifestDigest: "digest"})
		tx.queryRowFn = func(string, ...any) pgx.Row {
			return &assistFakeRow{vals: []any{intentJSON, planJSON, []byte("{")}}
		}
		if _, err := svc.loadAssistantTurnContractSnapshotTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1"); err == nil {
			t.Fatal("expected dryrun unmarshal err")
		}
		dryRunJSON, _ := json.Marshal(assistantDryRunResult{PlanHash: "plan"})
		tx.queryRowFn = func(string, ...any) pgx.Row {
			return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
		}
		snapshot, err := svc.loadAssistantTurnContractSnapshotTx(context.Background(), tx, "tenant_1", "conv_1", "turn_1")
		if err != nil || snapshot.PlanHash != "plan" {
			t.Fatalf("load snapshot err=%v snapshot=%+v", err, snapshot)
		}
	})
}

func TestAssistantTaskStore_SubmitTaskPG(t *testing.T) {
	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	req := assistantTaskSampleRequest(turn)
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}

	makeTx := func(actorID string, turnRows pgx.Rows, submitKeyRow pgx.Row, execFailNeedle string, commitErr error) *assistFakeTx {
		tx := &assistFakeTx{commitErr: commitErr}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", actorID, assistantStateValidated, now)}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				if submitKeyRow != nil {
					return submitKeyRow
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				if turnRows != nil {
					return turnRows, nil
				}
				return &assistFakeRows{}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if execFailNeedle != "" && strings.Contains(sql, execFailNeedle) {
				return pgconn.NewCommandTag(""), errors.New("exec failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		return tx
	}

	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	t.Run("validation and conversation gates", func(t *testing.T) {
		if _, err := svc.submitTaskPG(nil, "tenant_1", principal, assistantTaskSubmitRequest{}); err == nil || !strings.Contains(err.Error(), "conversation_id required") {
			t.Fatalf("expected validation err for nil ctx req=%v", err)
		}

		origMarshalFn := assistantTaskMarshalFn
		assistantTaskMarshalFn = func(any) ([]byte, error) { return nil, fmt.Errorf("hash marshal failed") }
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "hash marshal failed") {
			t.Fatalf("expected request hash marshal err=%v", err)
		}
		assistantTaskMarshalFn = origMarshalFn

		svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("unexpected begin err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_2", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); !errors.Is(err, errAssistantConversationForbidden) {
			t.Fatalf("unexpected actor mismatch err=%v", err)
		}

		loadConversationErrTx := makeTx("actor_1", nil, nil, "", nil)
		loadConversationErrTx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: errors.New("load conversation failed")} }
		svc.pool = assistFakeTxBeginner{tx: loadConversationErrTx}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "load conversation failed") {
			t.Fatalf("unexpected load conversation err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{}, nil, "", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); !errors.Is(err, errAssistantTurnNotFound) {
			t.Fatalf("unexpected turn missing err=%v", err)
		}

		badReq := req
		badReq.ContractSnapshot.PlanHash = "other_plan"
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, badReq); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("unexpected snapshot mismatch err=%v", err)
		}
	})

	t.Run("idempotency branches", func(t *testing.T) {
		submitKeyErrTx := makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "", nil)
		submitKeyErrTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", "actor_1", assistantStateValidated, now)}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{err: errors.New("submit key query failed")}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: submitKeyErrTx}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "submit key query failed") {
			t.Fatalf("unexpected submit key query err=%v", err)
		}

		hash, _ := assistantTaskRequestHash(req)
		existing := assistantTaskRecord{
			TaskID:             "71bb42fb-fcdd-4ff0-ad78-501f6dc5270a",
			TenantID:           "tenant_1",
			ConversationID:     "conv_1",
			TurnID:             "turn_1",
			TaskType:           assistantTaskTypeAsyncPlan,
			RequestID:          req.RequestID,
			RequestHash:        "other_hash",
			WorkflowID:         "wf_1",
			Status:             assistantTaskStatusQueued,
			DispatchStatus:     assistantTaskDispatchPending,
			DispatchDeadlineAt: now.Add(time.Minute),
			MaxAttempts:        3,
			ContractSnapshot:   req.ContractSnapshot,
			SubmittedAt:        now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, &assistFakeRow{vals: assistantTaskRowValues(existing)}, "", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); !errors.Is(err, errAssistantIdempotencyKeyConflict) {
			t.Fatalf("unexpected idempotency err=%v", err)
		}

		existing.RequestHash = hash
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, &assistFakeRow{vals: assistantTaskRowValues(existing)}, "", errors.New("commit failed"))}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("unexpected commit err=%v", err)
		}

		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, &assistFakeRow{vals: assistantTaskRowValues(existing)}, "", nil)}
		if got, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err != nil || got.TaskID != existing.TaskID {
			t.Fatalf("unexpected existing receipt=%+v err=%v", got, err)
		}
	})

	t.Run("insert and commit branches", func(t *testing.T) {
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "INSERT INTO iam.assistant_tasks", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("unexpected insert task err=%v", err)
		}
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "INSERT INTO iam.assistant_task_events", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("unexpected insert event err=%v", err)
		}
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "INSERT INTO iam.assistant_task_dispatch_outbox", nil)}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("unexpected insert outbox err=%v", err)
		}
		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "", nil)}
		if got, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err != nil || strings.TrimSpace(got.TaskID) == "" {
			t.Fatalf("unexpected create receipt=%+v err=%v", got, err)
		}

		svc.pool = assistFakeTxBeginner{tx: makeTx("actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil, "", errors.New("create commit failed"))}
		if _, err := svc.submitTaskPG(context.Background(), "tenant_1", principal, req); err == nil || !strings.Contains(err.Error(), "create commit failed") {
			t.Fatalf("unexpected create commit err=%v", err)
		}
	})
}

func TestAssistantTaskStore_GetTaskAndCancelTaskPG(t *testing.T) {
	wd := mustGetwd(t)
	t.Setenv("ALLOWLIST_PATH", mustAllowlistPathFromWd(t, wd))
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	turn.State = assistantStateConfirmed
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	record := assistantTaskRecord{
		TaskID:             "c2784a4b-b884-4018-8d4f-31fa4b40db69",
		TenantID:           "tenant_1",
		ConversationID:     "conv_1",
		TurnID:             "turn_1",
		TaskType:           assistantTaskTypeAsyncPlan,
		RequestID:          "req_1",
		RequestHash:        "hash_1",
		WorkflowID:         "wf_1",
		Status:             assistantTaskStatusQueued,
		DispatchStatus:     assistantTaskDispatchPending,
		DispatchDeadlineAt: now.Add(time.Minute),
		MaxAttempts:        2,
		TraceID:            "trace_1",
		ContractSnapshot:   assistantTaskSnapshotFromTurn(turn),
		SubmittedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	svc := newAssistantConversationService(store, nil)
	svc.commitAdapterRegistry = assistantCommitAdapterRegistryMap{adapters: map[string]assistantCommitAdapter{
		"orgunit_create_v1": assistantCommitAdapterStub{result: &assistantCommitResult{OrgCode: "ORG_NEW", EventUUID: "evt_1"}},
	}}
	if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
		t.Fatalf("refreshTurnVersionTuple err=%v", err)
	}
	assistantRefreshTurnDerivedFields(turn)
	record.ContractSnapshot = assistantTaskSnapshotFromTurn(turn)

	t.Run("getTaskPG", func(t *testing.T) {
		svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
		if _, err := svc.getTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "begin failed") {
			t.Fatalf("unexpected get begin err=%v", err)
		}
		txGet := &assistFakeTx{}
		txGet.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
				return &assistFakeRows{}, nil
			}
			return &assistFakeRows{}, nil
		}
		txGet.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(record)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_2"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: txGet}
		if _, err := svc.getTaskPG(context.Background(), "tenant_1", principal, record.TaskID); !errors.Is(err, errAssistantConversationForbidden) {
			t.Fatalf("unexpected get forbidden err=%v", err)
		}

		txGet.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if _, err := svc.getTaskPG(context.Background(), "tenant_1", principal, record.TaskID); !errors.Is(err, errAssistantTaskNotFound) {
			t.Fatalf("unexpected get not found err=%v", err)
		}
		txGet.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_tasks") {
				return &assistFakeRow{err: errors.New("load task failed")}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		if _, err := svc.getTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "load task failed") {
			t.Fatalf("unexpected get load err=%v", err)
		}

		txGet.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(record)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_1"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		txGet.commitErr = errors.New("commit failed")
		if _, err := svc.getTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("unexpected get commit err=%v", err)
		}
		txGet.commitErr = nil
		if got, err := svc.getTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err != nil || got.TaskID != record.TaskID {
			t.Fatalf("unexpected get success got=%+v err=%v", got, err)
		}
		if got, err := svc.getTaskPG(nil, "tenant_1", principal, record.TaskID); err != nil || got.TaskID != record.TaskID {
			t.Fatalf("unexpected get success with nil ctx got=%+v err=%v", got, err)
		}
	})

	t.Run("cancelTaskPG", func(t *testing.T) {
		svc.pool = assistFakeTxBeginner{err: errors.New("begin cancel failed")}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "begin cancel failed") {
			t.Fatalf("unexpected cancel begin err=%v", err)
		}

		txCancel := &assistFakeTx{}
		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(record)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_1"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		txCancel.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		svc.pool = assistFakeTxBeginner{tx: txCancel}
		if got, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err != nil || !got.CancelAccepted || got.Status != assistantTaskStatusCanceled {
			t.Fatalf("unexpected cancel success got=%+v err=%v", got, err)
		}

		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_tasks") {
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); !errors.Is(err, errAssistantTaskNotFound) {
			t.Fatalf("unexpected cancel task not found err=%v", err)
		}
		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_tasks") {
				return &assistFakeRow{err: errors.New("load cancel task failed")}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "load cancel task failed") {
			t.Fatalf("unexpected cancel load err=%v", err)
		}

		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(record)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_2"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); !errors.Is(err, errAssistantConversationForbidden) {
			t.Fatalf("unexpected cancel forbidden err=%v", err)
		}

		terminal := record
		terminal.Status = assistantTaskStatusSucceeded
		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(terminal)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_1"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if got, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err != nil || got.CancelAccepted {
			t.Fatalf("unexpected terminal cancel got=%+v err=%v", got, err)
		}

		invalid := record
		invalid.Status = "paused"
		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(invalid)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_1"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); !errors.Is(err, errAssistantTaskCancelNotAllowed) {
			t.Fatalf("unexpected cancel not allowed err=%v", err)
		}

		txCancel.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(record)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"actor_1"}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		txCancel.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
				return pgconn.NewCommandTag(""), errors.New("update cancel failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "update cancel failed") {
			t.Fatalf("unexpected cancel update err=%v", err)
		}
		txCancel.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
				return pgconn.NewCommandTag(""), errors.New("insert cancel event failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "insert cancel event failed") {
			t.Fatalf("unexpected cancel event err=%v", err)
		}
		eventInsertCount := 0
		txCancel.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
				eventInsertCount++
				if eventInsertCount == 2 {
					return pgconn.NewCommandTag(""), errors.New("insert second cancel event failed")
				}
			}
			return pgconn.NewCommandTag(""), nil
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "insert second cancel event failed") {
			t.Fatalf("unexpected cancel second event err=%v", err)
		}
		txCancel.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
				return pgconn.NewCommandTag(""), errors.New("cancel outbox failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "cancel outbox failed") {
			t.Fatalf("unexpected cancel outbox err=%v", err)
		}
		txCancel.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		txCancel.commitErr = errors.New("cancel commit failed")
		if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, record.TaskID); err == nil || !strings.Contains(err.Error(), "cancel commit failed") {
			t.Fatalf("unexpected cancel commit err=%v", err)
		}
		txCancel.commitErr = nil
		if got, err := svc.cancelTaskPG(nil, "tenant_1", principal, record.TaskID); err != nil || !got.CancelAccepted {
			t.Fatalf("unexpected cancel with nil ctx got=%+v err=%v", got, err)
		}
	})
}

func TestAssistantTaskStore_DispatchAndExecute(t *testing.T) {
	wd := mustGetwd(t)
	t.Setenv("ALLOWLIST_PATH", mustAllowlistPathFromWd(t, wd))
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	snapshot := assistantTaskSnapshotFromTurn(turn)
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	makeTask := func(taskID string) assistantTaskRecord {
		return assistantTaskRecord{
			TaskID:             taskID,
			TenantID:           "tenant_1",
			ConversationID:     "conv_1",
			TurnID:             turn.TurnID,
			TaskType:           assistantTaskTypeAsyncPlan,
			RequestID:          "req_1",
			RequestHash:        "hash_1",
			WorkflowID:         "wf_1",
			Status:             assistantTaskStatusQueued,
			DispatchStatus:     assistantTaskDispatchPending,
			DispatchDeadlineAt: now.Add(time.Minute),
			MaxAttempts:        2,
			ContractSnapshot:   snapshot,
			SubmittedAt:        now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}

	intentJSON, _ := json.Marshal(turn.Intent)
	planJSON, _ := json.Marshal(turn.Plan)
	dryRunJSON, _ := json.Marshal(turn.DryRun)

	newDispatchExecuteFixture := func(t *testing.T) (*assistantConversationService, assistantTaskRecord, *assistantTurn) {
		t.Helper()

		localTurn := assistantTaskSampleTurn(now)
		localTurn.State = assistantStateConfirmed
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatal(err)
		}

		localSvc := newAssistantConversationService(store, nil)
		localSvc.commitAdapterRegistry = assistantCommitAdapterRegistryMap{adapters: map[string]assistantCommitAdapter{
			"orgunit_create_v1": assistantCommitAdapterStub{result: &assistantCommitResult{OrgCode: "ORG_NEW", EventUUID: "evt_1"}},
		}}
		if err := localSvc.refreshTurnVersionTuple(context.Background(), "tenant_1", localTurn); err != nil {
			t.Fatalf("refreshTurnVersionTuple err=%v", err)
		}
		assistantRefreshTurnDerivedFields(localTurn)

		record := makeTask("dispatch-execute-task")
		record.ContractSnapshot = assistantTaskSnapshotFromTurn(localTurn)
		return localSvc, record, localTurn
	}

	t.Run("dispatch early paths", func(t *testing.T) {
		if err := (*assistantConversationService)(nil).dispatchAssistantTasks(context.Background(), "tenant_1", 1); err != nil {
			t.Fatalf("nil dispatch err=%v", err)
		}
		emptySvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		if err := emptySvc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err != nil {
			t.Fatalf("no pool dispatch err=%v", err)
		}

		localSvc, record, localTurn := newDispatchExecuteFixture(t)
		localSvc.pool = assistFakeTxBeginner{err: errors.New("begin dispatch failed")}
		if err := localSvc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "begin dispatch failed") {
			t.Fatalf("unexpected dispatch begin err=%v", err)
		}

		selectErrTx := &assistFakeTx{}
		selectErrTx.queryFn = func(string, ...any) (pgx.Rows, error) { return nil, errors.New("select outbox failed") }
		localSvc.pool = assistFakeTxBeginner{tx: selectErrTx}
		if err := localSvc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "select outbox failed") {
			t.Fatalf("unexpected dispatch select err=%v", err)
		}

		dispatchTx := &assistFakeTx{}
		dispatchTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
				return &assistFakeRows{rows: [][]any{
					{int64(1), "tenant_1", "missing-task", "wf_m", assistantTaskDispatchPending, 0, now, now, now},
					{int64(2), "tenant_1", record.TaskID, "wf_1", assistantTaskDispatchPending, 0, now, now, now},
				}}, nil
			}
			return &assistFakeRows{}, nil
		}
		dispatchTx.queryRowFn = func(sql string, args ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				taskID := ""
				if len(args) >= 2 {
					if v, ok := args[1].(string); ok {
						taskID = v
					}
				}
				if taskID == "missing-task" {
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
				succeeded := record
				succeeded.Status = assistantTaskStatusSucceeded
				return &assistFakeRow{vals: assistantTaskRowValues(succeeded)}
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRow{vals: []any{
					mustJSONMarshal(t, localTurn.Intent),
					mustJSONMarshal(t, localTurn.Plan),
					mustJSONMarshal(t, localTurn.DryRun),
				}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		dispatchTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		localSvc.pool = assistFakeTxBeginner{tx: dispatchTx}
		if err := localSvc.dispatchAssistantTasks(context.Background(), "tenant_1", 0); err != nil {
			t.Fatalf("dispatch success err=%v", err)
		}

		loadErrDispatchTx := &assistFakeTx{}
		loadErrDispatchTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
				return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", "task-err", "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
			}
			return &assistFakeRows{}, nil
		}
		loadErrDispatchTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_tasks") {
				return &assistFakeRow{err: errors.New("load task failed")}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		localSvc.pool = assistFakeTxBeginner{tx: loadErrDispatchTx}
		if err := localSvc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "load task failed") {
			t.Fatalf("unexpected dispatch load err=%v", err)
		}
	})

	t.Run("execute core branches and direct markers", func(t *testing.T) {
		localSvc, record, localTurn := newDispatchExecuteFixture(t)
		intentJSON := mustJSONMarshal(t, localTurn.Intent)
		planJSON := mustJSONMarshal(t, localTurn.Plan)
		dryRunJSON := mustJSONMarshal(t, localTurn.DryRun)

		execTx := &assistFakeTx{}
		execTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		execTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", "actor_1", assistantStateConfirmed, now)}
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		execTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: [][]any{assistantTurnRowValues(localTurn)}}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}

		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), execTx, "tenant_1", nil, now); !errors.Is(err, errAssistantTaskStateInvalid) {
			t.Fatalf("unexpected execute nil task err=%v", err)
		}
		terminalTask := record
		terminalTask.Status = assistantTaskStatusSucceeded
		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), execTx, "tenant_1", &terminalTask, now); err != nil {
			t.Fatalf("terminal execute err=%v", err)
		}
		running := record
		running.Status = assistantTaskStatusQueued
		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), execTx, "tenant_1", &running, now); err != nil || running.Status != assistantTaskStatusSucceeded {
			t.Fatalf("execute success err=%v running=%+v", err, running)
		}

		errLoadTx := &assistFakeTx{}
		errLoadTx.execFn = execTx.execFn
		errLoadTx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: errors.New("load snapshot failed")} }
		failedTask := record
		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), errLoadTx, "tenant_1", &failedTask, now); err == nil || !strings.Contains(err.Error(), "load snapshot failed") {
			t.Fatalf("unexpected execute load err=%v", err)
		}
		errLoadTx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{err: pgx.ErrNoRows} }
		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), errLoadTx, "tenant_1", &failedTask, now); !errors.Is(err, errAssistantTaskStateInvalid) {
			t.Fatalf("unexpected execute pgx no rows err=%v", err)
		}

		mismatch := record
		mismatch.ContractSnapshot.PlanHash = "other_plan"
		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), execTx, "tenant_1", &mismatch, now); err != nil || mismatch.Status != assistantTaskStatusManualTakeoverNeeded {
			t.Fatalf("execute mismatch err=%v mismatch=%+v", err, mismatch)
		}
		emptyPlan := record
		emptyPlan.ContractSnapshot.PlanHash = ""
		blankPlanTurn := *localTurn
		blankPlanTurn.DryRun = localTurn.DryRun
		blankPlanTurn.DryRun.PlanHash = ""
		blankDryRunJSON := mustJSONMarshal(t, blankPlanTurn.DryRun)
		emptyPlanTx := &assistFakeTx{}
		emptyPlanTx.execFn = execTx.execFn
		emptyPlanTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", "actor_1", assistantStateConfirmed, now)}
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRow{vals: []any{intentJSON, planJSON, blankDryRunJSON}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		emptyPlanTx.queryFn = execTx.queryFn
		if err := localSvc.executeAssistantTaskWorkflowTx(context.Background(), emptyPlanTx, "tenant_1", &emptyPlan, now); err != nil || emptyPlan.Status != assistantTaskStatusManualTakeoverNeeded {
			t.Fatalf("execute empty plan err=%v emptyPlan=%+v", err, emptyPlan)
		}

		markerTask := record
		if err := localSvc.markAssistantTaskDispatchFailureTx(context.Background(), execTx, &markerTask, now); err != nil {
			t.Fatalf("mark dispatch failure err=%v", err)
		}
		markerTask = record
		if err := localSvc.markAssistantTaskDispatchDeadlineExceededTx(context.Background(), execTx, &markerTask, now); err != nil {
			t.Fatalf("mark deadline exceeded err=%v", err)
		}

		brokenUpdateTx := &assistFakeTx{}
		brokenUpdateTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
				return pgconn.NewCommandTag(""), errors.New("update dispatch failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		markerTask = record
		if err := localSvc.markAssistantTaskDispatchFailureTx(context.Background(), brokenUpdateTx, &markerTask, now); err == nil || !strings.Contains(err.Error(), "update dispatch failed") {
			t.Fatalf("unexpected mark dispatch failure update err=%v", err)
		}
		brokenEventTx := &assistFakeTx{}
		brokenEventTx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
				return pgconn.NewCommandTag(""), errors.New("event insert failed")
			}
			return pgconn.NewCommandTag(""), nil
		}
		markerTask = record
		if err := localSvc.markAssistantTaskDispatchDeadlineExceededTx(context.Background(), brokenEventTx, &markerTask, now); err == nil || !strings.Contains(err.Error(), "event insert failed") {
			t.Fatalf("unexpected mark deadline event err=%v", err)
		}
	})

	t.Run("dispatchAssistantTasks", func(t *testing.T) {
		t.Run("dispatch not-exists update outbox error", func(t *testing.T) {
			tx := &assistFakeTx{}
			tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", "missing", "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
				}
				return &assistFakeRows{}, nil
			}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row { return &assistFakeRow{err: pgx.ErrNoRows} }
			tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
					return pgconn.NewCommandTag(""), errors.New("update outbox failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			svc.pool = assistFakeTxBeginner{tx: tx}
			if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "update outbox failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("dispatch terminal update outbox error", func(t *testing.T) {
			task := makeTask("terminal")
			task.Status = assistantTaskStatusSucceeded
			tx := &assistFakeTx{}
			tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", task.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
				}
				return &assistFakeRows{}, nil
			}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				if strings.Contains(sql, "FROM iam.assistant_tasks") {
					return &assistFakeRow{vals: assistantTaskRowValues(task)}
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
					return pgconn.NewCommandTag(""), errors.New("update terminal outbox failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			svc.pool = assistFakeTxBeginner{tx: tx}
			if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "update terminal outbox failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("dispatch deadline exceeded path", func(t *testing.T) {
			task := makeTask("deadline")
			task.DispatchDeadlineAt = now.Add(-time.Minute)
			tx := &assistFakeTx{}
			tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", task.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
				}
				return &assistFakeRows{}, nil
			}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				if strings.Contains(sql, "FROM iam.assistant_tasks") {
					return &assistFakeRow{vals: assistantTaskRowValues(task)}
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
			svc.pool = assistFakeTxBeginner{tx: tx}
			if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("dispatch execute error retry and dead-letter branches", func(t *testing.T) {
			retryTask := makeTask("retry")
			retryTask.MaxAttempts = 3
			deadTask := makeTask("dead")
			deadTask.MaxAttempts = 1
			tx := &assistFakeTx{}
			tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{rows: [][]any{
						{int64(1), "tenant_1", retryTask.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now},
						{int64(2), "tenant_1", deadTask.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now},
					}}, nil
				}
				return &assistFakeRows{}, nil
			}
			tx.queryRowFn = func(sql string, args ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_tasks"):
					taskID := args[1].(string)
					if taskID == retryTask.TaskID {
						return &assistFakeRow{vals: assistantTaskRowValues(retryTask)}
					}
					return &assistFakeRow{vals: assistantTaskRowValues(deadTask)}
				case strings.Contains(sql, "FROM iam.assistant_turns"):
					return &assistFakeRow{err: pgx.ErrNoRows}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
			svc.pool = assistFakeTxBeginner{tx: tx}
			if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 2); err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("dispatch execute success with outbox update error", func(t *testing.T) {
			task := makeTask("success")
			tx := &assistFakeTx{}
			tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", task.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
				}
				return &assistFakeRows{}, nil
			}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_tasks"):
					return &assistFakeRow{vals: assistantTaskRowValues(task)}
				case strings.Contains(sql, "FROM iam.assistant_turns"):
					return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
					return pgconn.NewCommandTag(""), errors.New("outbox started update failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			svc.pool = assistFakeTxBeginner{tx: tx}
			if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "outbox started update failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	})

	t.Run("executeAssistantTaskWorkflowTx", func(t *testing.T) {
		t.Run("execute workflow granular errors", func(t *testing.T) {
			task := makeTask("exec-err")
			tx := &assistFakeTx{}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				if strings.Contains(sql, "FROM iam.assistant_turns") {
					return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
					return pgconn.NewCommandTag(""), errors.New("update execute failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), tx, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "update execute failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					return pgconn.NewCommandTag(""), errors.New("event execute failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			task = makeTask("exec-event")
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), tx, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "event execute failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	})
}

func TestAssistantTaskStore_ResidualErrorMatrix(t *testing.T) {
	now := time.Now().UTC()
	turn := assistantTaskSampleTurn(now)
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	base := assistantTaskRecord{
		TaskID:             "task_base",
		TenantID:           "tenant_1",
		ConversationID:     "conv_1",
		TurnID:             turn.TurnID,
		TaskType:           assistantTaskTypeAsyncPlan,
		RequestID:          "req_1",
		RequestHash:        "hash_1",
		WorkflowID:         "wf_1",
		Status:             assistantTaskStatusQueued,
		DispatchStatus:     assistantTaskDispatchPending,
		DispatchDeadlineAt: now.Add(time.Minute),
		Attempt:            0,
		MaxAttempts:        3,
		ContractSnapshot:   assistantTaskSnapshotFromTurn(turn),
		SubmittedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	makeTask := func(taskID string) assistantTaskRecord {
		task := base
		task.TaskID = taskID
		return task
	}

	intentJSON, _ := json.Marshal(turn.Intent)
	planJSON, _ := json.Marshal(turn.Plan)
	dryRunJSON, _ := json.Marshal(turn.DryRun)
	dryRunEmptyPlanJSON, _ := json.Marshal(assistantDryRunResult{PlanHash: ""})

	t.Run("cancelTaskPG residual stoplines", func(t *testing.T) {
		t.Run("cancel terminal commit error", func(t *testing.T) {
			task := makeTask("task-terminal")
			task.Status = assistantTaskStatusSucceeded
			tx := &assistFakeTx{commitErr: errors.New("terminal commit failed")}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_tasks"):
					return &assistFakeRow{vals: assistantTaskRowValues(task)}
				case strings.Contains(sql, "FROM iam.assistant_conversations"):
					return &assistFakeRow{vals: []any{"actor_1"}}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
			svc.pool = assistFakeTxBeginner{tx: tx}
			if _, err := svc.cancelTaskPG(context.Background(), "tenant_1", principal, task.TaskID); err == nil || !strings.Contains(err.Error(), "terminal commit failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	})

	t.Run("dispatchAssistantTasks residual stoplines", func(t *testing.T) {
		t.Run("dispatch nil ctx defaults to background", func(t *testing.T) {
			tx := &assistFakeTx{}
			tx.queryFn = func(string, ...any) (pgx.Rows, error) { return &assistFakeRows{}, nil }
			svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
			svc.pool = assistFakeTxBeginner{tx: tx}
			if err := svc.dispatchAssistantTasks(nil, "tenant_1", 1); err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("dispatch deadline paths and retry/dead-letter errors", func(t *testing.T) {
			svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

			t.Run("deadline mark failure", func(t *testing.T) {
				task := makeTask("task-deadline-mark")
				task.DispatchDeadlineAt = now.Add(-time.Minute)
				tx := &assistFakeTx{}
				tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
					if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
						return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", task.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
					}
					return &assistFakeRows{}, nil
				}
				tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
					if strings.Contains(sql, "FROM iam.assistant_tasks") {
						return &assistFakeRow{vals: assistantTaskRowValues(task)}
					}
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
				tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
						return pgconn.NewCommandTag(""), errors.New("deadline mark failed")
					}
					return pgconn.NewCommandTag(""), nil
				}
				svc.pool = assistFakeTxBeginner{tx: tx}
				if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "deadline mark failed") {
					t.Fatalf("unexpected err=%v", err)
				}
			})

			t.Run("deadline outbox update failure", func(t *testing.T) {
				task := makeTask("task-deadline-outbox")
				task.DispatchDeadlineAt = now.Add(-time.Minute)
				tx := &assistFakeTx{}
				tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
					if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
						return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", task.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
					}
					return &assistFakeRows{}, nil
				}
				tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
					if strings.Contains(sql, "FROM iam.assistant_tasks") {
						return &assistFakeRow{vals: assistantTaskRowValues(task)}
					}
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
				tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
						return pgconn.NewCommandTag(""), errors.New("deadline outbox failed")
					}
					return pgconn.NewCommandTag(""), nil
				}
				svc.pool = assistFakeTxBeginner{tx: tx}
				if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "deadline outbox failed") {
					t.Fatalf("unexpected err=%v", err)
				}
			})

			t.Run("dead-letter mark failure and outbox failure", func(t *testing.T) {
				deadTask := makeTask("task-dead-letter")
				deadTask.MaxAttempts = 1
				txMarkErr := &assistFakeTx{}
				txMarkErr.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
					if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
						return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", deadTask.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
					}
					return &assistFakeRows{}, nil
				}
				txMarkErr.queryRowFn = func(sql string, _ ...any) pgx.Row {
					switch {
					case strings.Contains(sql, "FROM iam.assistant_tasks"):
						return &assistFakeRow{vals: assistantTaskRowValues(deadTask)}
					case strings.Contains(sql, "FROM iam.assistant_turns"):
						return &assistFakeRow{err: pgx.ErrNoRows}
					default:
						return &assistFakeRow{err: pgx.ErrNoRows}
					}
				}
				txMarkErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
						return pgconn.NewCommandTag(""), errors.New("mark failure update failed")
					}
					return pgconn.NewCommandTag(""), nil
				}
				svc.pool = assistFakeTxBeginner{tx: txMarkErr}
				if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "mark failure update failed") {
					t.Fatalf("unexpected err=%v", err)
				}

				txOutboxErr := &assistFakeTx{}
				txOutboxErr.queryFn = txMarkErr.queryFn
				txOutboxErr.queryRowFn = txMarkErr.queryRowFn
				txOutboxErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
						return pgconn.NewCommandTag(""), errors.New("dead-letter outbox failed")
					}
					return pgconn.NewCommandTag(""), nil
				}
				svc.pool = assistFakeTxBeginner{tx: txOutboxErr}
				if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "dead-letter outbox failed") {
					t.Fatalf("unexpected err=%v", err)
				}
			})

			t.Run("retry branch update and outbox failures", func(t *testing.T) {
				retryTask := makeTask("task-retry")
				retryTask.MaxAttempts = 5

				txUpdateErr := &assistFakeTx{}
				txUpdateErr.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
					if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
						return &assistFakeRows{rows: [][]any{{int64(1), "tenant_1", retryTask.TaskID, "wf", assistantTaskDispatchPending, 0, now, now, now}}}, nil
					}
					return &assistFakeRows{}, nil
				}
				txUpdateErr.queryRowFn = func(sql string, _ ...any) pgx.Row {
					switch {
					case strings.Contains(sql, "FROM iam.assistant_tasks"):
						return &assistFakeRow{vals: assistantTaskRowValues(retryTask)}
					case strings.Contains(sql, "FROM iam.assistant_turns"):
						return &assistFakeRow{err: pgx.ErrNoRows}
					default:
						return &assistFakeRow{err: pgx.ErrNoRows}
					}
				}
				txUpdateErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
						return pgconn.NewCommandTag(""), errors.New("retry state update failed")
					}
					return pgconn.NewCommandTag(""), nil
				}
				svc.pool = assistFakeTxBeginner{tx: txUpdateErr}
				if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "retry state update failed") {
					t.Fatalf("unexpected err=%v", err)
				}

				txOutboxErr := &assistFakeTx{}
				txOutboxErr.queryFn = txUpdateErr.queryFn
				txOutboxErr.queryRowFn = txUpdateErr.queryRowFn
				txOutboxErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE iam.assistant_task_dispatch_outbox") {
						return pgconn.NewCommandTag(""), errors.New("retry outbox failed")
					}
					return pgconn.NewCommandTag(""), nil
				}
				svc.pool = assistFakeTxBeginner{tx: txOutboxErr}
				if err := svc.dispatchAssistantTasks(context.Background(), "tenant_1", 1); err == nil || !strings.Contains(err.Error(), "retry outbox failed") {
					t.Fatalf("unexpected err=%v", err)
				}
			})
		})
	})

	t.Run("executeAssistantTaskWorkflowTx residual stoplines", func(t *testing.T) {
		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		makeRunningTask := func(taskID string) assistantTaskRecord {
			task := makeTask(taskID)
			task.Status = assistantTaskStatusRunning
			task.Attempt = 1
			task.DispatchStatus = assistantTaskDispatchStarted
			return task
		}

		t.Run("mismatch branch update and event errors", func(t *testing.T) {
			task := makeRunningTask("task-mismatch-update")
			task.ContractSnapshot.PlanHash = "other-plan"
			txUpdateErr := &assistFakeTx{}
			txUpdateErr.queryRowFn = func(sql string, _ ...any) pgx.Row {
				if strings.Contains(sql, "FROM iam.assistant_turns") {
					return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			txUpdateErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
					return pgconn.NewCommandTag(""), errors.New("mismatch update failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txUpdateErr, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "mismatch update failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			task = makeRunningTask("task-mismatch-event1")
			task.ContractSnapshot.PlanHash = "other-plan"
			txEvent1Err := &assistFakeTx{}
			txEvent1Err.queryRowFn = txUpdateErr.queryRowFn
			txEvent1Err.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					return pgconn.NewCommandTag(""), errors.New("mismatch event1 failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txEvent1Err, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "mismatch event1 failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			task = makeRunningTask("task-mismatch-event2")
			task.ContractSnapshot.PlanHash = "other-plan"
			txEvent2Err := &assistFakeTx{}
			txEvent2Err.queryRowFn = txUpdateErr.queryRowFn
			eventCount := 0
			txEvent2Err.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					eventCount++
					if eventCount == 2 {
						return pgconn.NewCommandTag(""), errors.New("mismatch event2 failed")
					}
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txEvent2Err, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "mismatch event2 failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("empty plan branch success and errors", func(t *testing.T) {
			task := makeRunningTask("task-empty-plan")
			task.ContractSnapshot.PlanHash = ""
			txSuccess := &assistFakeTx{}
			txSuccess.queryRowFn = func(sql string, _ ...any) pgx.Row {
				if strings.Contains(sql, "FROM iam.assistant_turns") {
					return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunEmptyPlanJSON}}
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			txSuccess.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txSuccess, "tenant_1", &task, now); err != nil || task.Status != assistantTaskStatusManualTakeoverNeeded {
				t.Fatalf("unexpected err=%v task=%+v", err, task)
			}

			task = makeRunningTask("task-empty-plan-update")
			task.ContractSnapshot.PlanHash = ""
			txUpdateErr := &assistFakeTx{}
			txUpdateErr.queryRowFn = txSuccess.queryRowFn
			txUpdateErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
					return pgconn.NewCommandTag(""), errors.New("empty plan update failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txUpdateErr, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "empty plan update failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			task = makeRunningTask("task-empty-plan-event1")
			task.ContractSnapshot.PlanHash = ""
			txEvent1Err := &assistFakeTx{}
			txEvent1Err.queryRowFn = txSuccess.queryRowFn
			txEvent1Err.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					return pgconn.NewCommandTag(""), errors.New("empty plan event1 failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txEvent1Err, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "empty plan event1 failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			task = makeRunningTask("task-empty-plan-event2")
			task.ContractSnapshot.PlanHash = ""
			txEvent2Err := &assistFakeTx{}
			txEvent2Err.queryRowFn = txSuccess.queryRowFn
			eventCount := 0
			txEvent2Err.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					eventCount++
					if eventCount == 2 {
						return pgconn.NewCommandTag(""), errors.New("empty plan event2 failed")
					}
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txEvent2Err, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "empty plan event2 failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})

		t.Run("success branch update and event errors", func(t *testing.T) {
			task := makeRunningTask("task-success-update")
			txUpdateErr := &assistFakeTx{}
			txUpdateErr.queryRowFn = func(sql string, _ ...any) pgx.Row {
				if strings.Contains(sql, "FROM iam.assistant_turns") {
					return &assistFakeRow{vals: []any{intentJSON, planJSON, dryRunJSON}}
				}
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
			txUpdateErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
					return pgconn.NewCommandTag(""), errors.New("success update failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txUpdateErr, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "success update failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			task = makeRunningTask("task-success-event")
			txEventErr := &assistFakeTx{}
			txEventErr.queryRowFn = txUpdateErr.queryRowFn
			txEventErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					return pgconn.NewCommandTag(""), errors.New("success event failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), txEventErr, "tenant_1", &task, now); err == nil || !strings.Contains(err.Error(), "success event failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	})

	t.Run("direct marker residual stoplines", func(t *testing.T) {
		t.Run("mark failure/deadline direct errors", func(t *testing.T) {
			svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
			task := makeTask("task-mark")

			txFailureEventErr := &assistFakeTx{}
			txFailureEventErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO iam.assistant_task_events") {
					return pgconn.NewCommandTag(""), errors.New("mark failure event failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.markAssistantTaskDispatchFailureTx(context.Background(), txFailureEventErr, &task, now); err == nil || !strings.Contains(err.Error(), "mark failure event failed") {
				t.Fatalf("unexpected err=%v", err)
			}

			task = makeTask("task-deadline-update")
			txDeadlineUpdateErr := &assistFakeTx{}
			txDeadlineUpdateErr.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE iam.assistant_tasks") {
					return pgconn.NewCommandTag(""), errors.New("deadline update failed")
				}
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.markAssistantTaskDispatchDeadlineExceededTx(context.Background(), txDeadlineUpdateErr, &task, now); err == nil || !strings.Contains(err.Error(), "deadline update failed") {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	})
}
