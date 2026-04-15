package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	cubeboxmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

func TestCubeBoxLinkHelpersAndMappings(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	cancelAt := now.Add(2 * time.Minute)
	completedAt := now.Add(3 * time.Minute)

	assistantConv := &assistantConversation{
		ConversationID: " conv-1 ",
		TenantID:       " tenant-1 ",
		ActorID:        " actor-1 ",
		ActorRole:      " tenant-admin ",
		State:          assistantStateConfirmed,
		CurrentPhase:   assistantPhaseAwaitCommitConfirm,
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Minute),
		Turns: []*assistantTurn{
			nil,
			{
				TurnID:              " turn-1 ",
				UserInput:           " hello ",
				State:               assistantStateConfirmed,
				Phase:               assistantPhaseAwaitCommitConfirm,
				RiskTier:            " high ",
				RequestID:           " req-1 ",
				TraceID:             " trace-1 ",
				PolicyVersion:       " policy-v1 ",
				CompositionVersion:  " comp-v1 ",
				MappingVersion:      " map-v1 ",
				Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部"},
				RouteDecision:       assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "route-v1", KnowledgeSnapshotDigest: "knowledge-v1", ResolverContractVersion: "resolver-v1", DecisionSource: assistantRouteDecisionSourceKnowledgeRuntimeV1},
				Clarification:       &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots},
				Candidates:          []assistantCandidate{{CandidateID: "cand-1", CandidateCode: "FLOWER-A", Name: "鲜花组织"}},
				Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
				DryRun:              assistantDryRunResult{Explain: "计划已生成", ValidationErrors: []string{"missing_parent_ref_text"}},
				ResolvedCandidateID: " cand-1 ",
				SelectedCandidateID: " cand-1 ",
				AmbiguityCount:      1,
				Confidence:          0.9,
				ResolutionSource:    " auto ",
				PendingDraftSummary: " 等待确认 ",
				MissingFields:       []string{"effective_date"},
				CommitResult:        &assistantCommitResult{OrgCode: "ORG-1", EventUUID: "evt-1"},
				CommitReply:         &assistantCommitReply{Message: "提交成功"},
				ReplyNLG:            &assistantRenderReplyResponse{Text: "已生成回复", Stage: "draft", Kind: "info"},
				ErrorCode:           " error-code ",
				CreatedAt:           now,
				UpdatedAt:           now.Add(time.Minute),
			},
		},
		Transitions: []assistantStateTransition{
			{
				ID:         7,
				TurnID:     " turn-1 ",
				TurnAction: " confirm ",
				RequestID:  " req-1 ",
				TraceID:    " trace-1 ",
				FromState:  assistantStateValidated,
				ToState:    assistantStateConfirmed,
				FromPhase:  assistantPhaseAwaitCandidateConfirm,
				ToPhase:    assistantPhaseAwaitCommitConfirm,
				ReasonCode: " confirmed ",
				ActorID:    " actor-1 ",
				ChangedAt:  now.Add(30 * time.Second),
			},
		},
	}

	t.Run("assistant conversation maps to cubebox domain", func(t *testing.T) {
		got := mapAssistantConversation(assistantConv)
		if got == nil || got.ConversationID != "conv-1" || got.ActorRole != "tenant-admin" {
			t.Fatalf("conversation=%+v", got)
		}
		if len(got.Turns) != 1 || got.Turns[0].TurnID != "turn-1" {
			t.Fatalf("turns=%+v", got.Turns)
		}
		if got.Turns[0].Intent["action"] != assistantIntentCreateOrgUnit {
			t.Fatalf("intent=%+v", got.Turns[0].Intent)
		}
		if len(got.Turns[0].Candidates) != 1 || got.Turns[0].Candidates[0]["candidate_id"] != "cand-1" {
			t.Fatalf("candidates=%+v", got.Turns[0].Candidates)
		}
		if len(got.Transitions) != 1 || got.Transitions[0].TurnAction != "confirm" {
			t.Fatalf("transitions=%+v", got.Transitions)
		}
	})

	t.Run("cubebox conversation maps back to assistant types", func(t *testing.T) {
		conversation := &cubeboxdomain.Conversation{
			ConversationID: " conv-2 ",
			TenantID:       " ",
			ActorID:        " actor-2 ",
			ActorRole:      " viewer ",
			State:          assistantStateValidated,
			CurrentPhase:   assistantPhaseAwaitMissingFields,
			CreatedAt:      now,
			UpdatedAt:      now.Add(time.Minute),
			Turns: []cubeboxdomain.ConversationTurn{
				{
					TurnID:              " turn-2 ",
					UserInput:           " hi ",
					State:               assistantStateValidated,
					Phase:               assistantPhaseAwaitMissingFields,
					RiskTier:            " low ",
					RequestID:           " req-2 ",
					TraceID:             " trace-2 ",
					PolicyVersion:       " policy-v2 ",
					CompositionVersion:  " comp-v2 ",
					MappingVersion:      " map-v2 ",
					Intent:              map[string]any{"action": assistantIntentPlanOnly},
					RouteDecision:       map[string]any{"route_kind": assistantRouteKindKnowledgeQA},
					Clarification:       map[string]any{"status": assistantClarificationStatusOpen},
					Candidates:          []map[string]any{{"candidate_id": "cand-2"}},
					Plan:                map[string]any{"title": "仅生成计划"},
					DryRun:              map[string]any{"explain": "需要补充信息"},
					ResolvedCandidateID: " cand-2 ",
					SelectedCandidateID: " cand-2 ",
					AmbiguityCount:      2,
					Confidence:          0.4,
					ResolutionSource:    " manual ",
					PendingDraftSummary: " 需要补充 ",
					MissingFields:       []string{"entity_name"},
					CommitResult:        map[string]any{"org_code": "ORG-2"},
					CommitReply:         map[string]any{"message": "等待提交", "kind": "info"},
					ReplyNLG:            map[string]any{"text": "请补充名称"},
					ErrorCode:           " err-2 ",
					CreatedAt:           now,
					UpdatedAt:           now.Add(time.Minute),
				},
			},
			Transitions: []cubeboxdomain.StateTransition{
				{
					ID:         9,
					TurnID:     " turn-2 ",
					TurnAction: " create ",
					RequestID:  " req-2 ",
					TraceID:    " trace-2 ",
					FromState:  "init",
					ToState:    assistantStateValidated,
					FromPhase:  "init",
					ToPhase:    assistantPhaseAwaitMissingFields,
					ReasonCode: " created ",
					ActorID:    " actor-2 ",
					ChangedAt:  now,
				},
			},
		}
		got := assistantConversationFromCubeBox(conversation, "tenant-fallback")
		if got == nil || got.TenantID != "tenant-fallback" || got.ActorRole != "viewer" {
			t.Fatalf("conversation=%+v", got)
		}
		if len(got.Turns) != 1 || got.Turns[0].TurnID != "turn-2" {
			t.Fatalf("turns=%+v", got.Turns)
		}
		if got.Turns[0].Intent.Action != assistantIntentPlanOnly {
			t.Fatalf("intent=%+v", got.Turns[0].Intent)
		}
		if got.Turns[0].ReplyNLG == nil || got.Turns[0].ReplyNLG.Text != "请补充名称" {
			t.Fatalf("reply=%+v", got.Turns[0].ReplyNLG)
		}
		if got.CurrentPhase == "" || got.Transitions[0].ReasonCode != "created" {
			t.Fatalf("derived=%+v transitions=%+v", got, got.Transitions)
		}
	})

	t.Run("invalid cubebox turn json returns nil", func(t *testing.T) {
		turn := assistantTurnFromCubeBox(cubeboxdomain.ConversationTurn{
			TurnID:     "turn-bad",
			Intent:     map[string]any{"action": func() {}},
			CreatedAt:  now,
			UpdatedAt:  now,
			State:      assistantStateValidated,
			RiskTier:   "low",
			UserInput:  "hello",
			RequestID:  "req",
			TraceID:    "trace",
			Phase:      assistantPhaseAwaitCommitConfirm,
			Confidence: 0.1,
		})
		if turn != nil {
			t.Fatalf("expected nil turn, got %+v", turn)
		}
	})

	t.Run("invalid cubebox turn nested payloads return nil", func(t *testing.T) {
		cases := []struct {
			name string
			turn cubeboxdomain.ConversationTurn
		}{
			{name: "route decision", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, RouteDecision: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "clarification", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, Clarification: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "plan", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, Plan: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "dry run", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, DryRun: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "candidates", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, Candidates: []map[string]any{{"bad": func() {}}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "commit result", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, CommitResult: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "commit reply", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, CommitReply: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
			{name: "reply nlg", turn: cubeboxdomain.ConversationTurn{TurnID: "turn-bad", Intent: map[string]any{"action": assistantIntentPlanOnly}, ReplyNLG: map[string]any{"bad": func() {}}, State: assistantStateValidated, RiskTier: "low", UserInput: "hello", RequestID: "req", TraceID: "trace", Phase: assistantPhaseAwaitCommitConfirm, Confidence: 0.1, CreatedAt: now, UpdatedAt: now}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if turn := assistantTurnFromCubeBox(tc.turn); turn != nil {
					t.Fatalf("expected nil turn, got %+v", turn)
				}
			})
		}
	})

		t.Run("json helpers and poll uri", func(t *testing.T) {
			var out assistantIntentSpec
			if err := remarshalJSON(map[string]any{"action": assistantIntentPlanOnly}, &out); err != nil || out.Action != assistantIntentPlanOnly {
				t.Fatalf("out=%+v err=%v", out, err)
		}
		if err := remarshalJSON(map[string]any{"bad": func() {}}, &out); err == nil {
			t.Fatal("expected marshal error")
		}
		if err := remarshalJSON(nil, &out); err != nil {
			t.Fatalf("nil remarshal err=%v", err)
		}
		if got := assistantJSONMap(map[string]any{"k": "v"}); got["k"] != "v" {
			t.Fatalf("json map=%+v", got)
		}
			if got := assistantJSONMap([]string{"bad"}); got != nil {
				t.Fatalf("expected nil map, got %+v", got)
			}
			if got := assistantJSONMap(map[string]any{}); got != nil {
				t.Fatalf("expected nil map for empty object, got %+v", got)
			}
			if got := assistantJSONMap("null"); got != nil {
				t.Fatalf("expected nil map for scalar, got %+v", got)
			}
			if got := assistantJSONMapSlice([]map[string]any{{"k": "v"}}); len(got) != 1 || got[0]["k"] != "v" {
				t.Fatalf("json map slice=%+v", got)
			}
			if got := assistantJSONMapSlice(map[string]any{"bad": "shape"}); got != nil {
				t.Fatalf("expected nil map slice, got %+v", got)
			}
			if got := assistantJSONMapSlice([]map[string]any{}); got != nil {
				t.Fatalf("expected nil map slice for empty array, got %+v", got)
			}
			if got := cubeboxTaskPollURI(" task-1 "); got != "/internal/cubebox/tasks/task-1" {
				t.Fatalf("poll uri=%q", got)
			}
		})

	t.Run("task response mappings", func(t *testing.T) {
		if got := mapAssistantTaskReceipt(nil); got != nil {
			t.Fatalf("expected nil receipt, got %+v", got)
		}
		receipt := mapAssistantTaskReceipt(&assistantTaskAsyncReceipt{
			TaskID:      " task-1 ",
			TaskType:    " commit ",
			Status:      " queued ",
			WorkflowID:  " wf-1 ",
			SubmittedAt: now,
		})
		if receipt == nil || receipt.TaskID != "task-1" || receipt.PollURI != "/internal/cubebox/tasks/task-1" {
			t.Fatalf("receipt=%+v", receipt)
		}
		if got := mapAssistantTaskDetail(nil); got != nil {
			t.Fatalf("expected nil detail, got %+v", got)
		}
		detail := mapAssistantTaskDetail(&assistantTaskDetailResponse{
			TaskID:            " task-2 ",
			TaskType:          " confirm ",
			Status:            " running ",
			DispatchStatus:    " dispatching ",
			Attempt:           1,
			MaxAttempts:       3,
			LastErrorCode:     " err ",
			WorkflowID:        " wf-2 ",
			RequestID:         " req-2 ",
			TraceID:           " trace-2 ",
			ConversationID:    " conv-2 ",
			TurnID:            " turn-2 ",
			SubmittedAt:       now,
			CancelRequestedAt: &cancelAt,
			CompletedAt:       &completedAt,
			UpdatedAt:         now.Add(time.Minute),
			ContractSnapshot: assistantTaskContractSnapshot{
				IntentSchemaVersion:      "intent-v1",
				CompilerContractVersion:  "compiler-v1",
				CapabilityMapVersion:     "cap-v1",
				SkillManifestDigest:      "skill-v1",
				ContextHash:              "ctx-v1",
				IntentHash:               "intent-hash",
				PlanHash:                 "plan-hash",
				KnowledgeSnapshotDigest:  "knowledge-v1",
				RouteCatalogVersion:      "route-v1",
				ResolverContractVersion:  "resolver-v1",
				ContextTemplateVersion:   "context-v1",
				ReplyGuidanceVersion:     "reply-v1",
				PolicyContextDigest:      "policy-v1",
				EffectivePolicyVersion:   "effective-v1",
				ResolvedSetID:            "S2601",
				SetIDSource:              "custom",
				PrecheckProjectionDigest: "precheck-v1",
				MutationPolicyVersion:    "mutation-v1",
			},
		})
		if detail == nil || detail.TaskID != "task-2" || detail.ContractSnapshot.PlanHash != "plan-hash" {
			t.Fatalf("detail=%+v", detail)
		}
		if got := mapAssistantTaskCancelResponse(nil); got != nil {
			t.Fatalf("expected nil cancel response, got %+v", got)
		}
		cancelResp := mapAssistantTaskCancelResponse(&assistantTaskCancelResponse{
			assistantTaskDetailResponse: assistantTaskDetailResponse{
				TaskID:         " task-3 ",
				TaskType:       " commit ",
				Status:         " canceled ",
				DispatchStatus: " canceled ",
				RequestID:      " req-3 ",
				ConversationID: " conv-3 ",
				TurnID:         " turn-3 ",
				SubmittedAt:    now,
				UpdatedAt:      now,
			},
			CancelAccepted: true,
		})
		if cancelResp == nil || !cancelResp.CancelAccepted || cancelResp.TaskID != "task-3" {
			t.Fatalf("cancelResp=%+v", cancelResp)
		}
	})

	t.Run("error helpers map to formal errors", func(t *testing.T) {
		if !errors.Is(cubeboxConversationForbidden(), cubeboxservices.ErrConversationForbidden) {
			t.Fatalf("forbidden err=%v", cubeboxConversationForbidden())
		}
		if !errors.Is(cubeboxTenantMismatch(), cubeboxservices.ErrTenantMismatch) {
			t.Fatalf("tenant mismatch err=%v", cubeboxTenantMismatch())
		}
		if !errors.Is(cubeboxTaskNotFound(), cubeboxservices.ErrTaskNotFound) {
			t.Fatalf("task not found err=%v", cubeboxTaskNotFound())
		}
	})
}

func TestCubeBoxLegacyFacadeBehavior(t *testing.T) {
	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	principal := cubeboxmodule.Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

	t.Run("nil assistant returns gate unavailable", func(t *testing.T) {
		legacy := cubeboxLegacyFacade{}
		if _, _, err := legacy.ListConversations(context.Background(), "tenant-1", "actor-1", 10, ""); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("list err=%v", err)
		}
		if _, err := legacy.GetConversation(context.Background(), "tenant-1", "actor-1", "conv-1"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("get err=%v", err)
		}
		if _, err := legacy.CreateConversation(context.Background(), "tenant-1", principal); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("create err=%v", err)
		}
		if _, err := legacy.CreateTurn(context.Background(), "tenant-1", principal, "conv-1", "hello"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("create turn err=%v", err)
		}
		if _, err := legacy.ConfirmTurn(context.Background(), "tenant-1", principal, "conv-1", "turn-1", "cand-1"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("confirm err=%v", err)
		}
		if _, err := legacy.CommitTurn(context.Background(), "tenant-1", principal, "conv-1", "turn-1"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("commit err=%v", err)
		}
		if _, err := legacy.SubmitTask(context.Background(), "tenant-1", principal, cubeboxdomain.TaskSubmitRequest{}); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("submit task err=%v", err)
		}
		if _, err := legacy.GetTask(context.Background(), "tenant-1", principal, "task-1"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("get task err=%v", err)
		}
		if _, err := legacy.CancelTask(context.Background(), "tenant-1", principal, "task-1"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("cancel task err=%v", err)
		}
		if _, err := legacy.ExecuteTaskWorkflow(context.Background(), "tenant-1", principal, nil, "turn-1"); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("execute err=%v", err)
		}
		if _, err := legacy.RenderReply(context.Background(), "tenant-1", principal, "conv-1", "turn-1", nil); !errors.Is(err, errAssistantGateUnavailable) {
			t.Fatalf("render err=%v", err)
		}
	})

	t.Run("list get create and reply use assistant service", func(t *testing.T) {
		svc := newAssistantConversationService(nil, nil)
		created := svc.createConversation("tenant-1", Principal{ID: principal.ID, RoleSlug: principal.RoleSlug})
		created.Turns = []*assistantTurn{{
			TurnID:        "turn-1",
			UserInput:     "hello",
			State:         assistantStateValidated,
			Phase:         assistantPhaseAwaitMissingFields,
			RiskTier:      "low",
			RequestID:     "req-1",
			TraceID:       "trace-1",
			CreatedAt:     now,
			UpdatedAt:     now,
			MissingFields: []string{"entity_name"},
		}}
		assistantRefreshConversationDerivedFields(created)
		svc.byID[created.ConversationID] = cloneConversation(created)
		legacy := cubeboxLegacyFacade{assistant: svc}

		items, next, err := legacy.ListConversations(context.Background(), "tenant-1", principal.ID, 10, "")
		if err != nil || len(items) != 1 || next != "" || items[0].LastTurn == nil || items[0].LastTurn.TurnID != "turn-1" {
			t.Fatalf("items=%+v next=%q err=%v", items, next, err)
		}
		got, err := legacy.GetConversation(context.Background(), "tenant-1", principal.ID, created.ConversationID)
		if err != nil || got == nil || got.ConversationID != created.ConversationID {
			t.Fatalf("conversation=%+v err=%v", got, err)
		}
		created2, err := legacy.CreateConversation(context.Background(), "tenant-1", principal)
		if err != nil || created2 == nil || created2.ActorRole != principal.RoleSlug {
			t.Fatalf("created2=%+v err=%v", created2, err)
		}
		createdTurn, err := legacy.CreateTurn(context.Background(), "tenant-1", principal, created.ConversationID, "仅生成计划")
		if err != nil || createdTurn == nil || len(createdTurn.Turns) == 0 {
			t.Fatalf("createdTurn=%+v err=%v", createdTurn, err)
		}
		reply, err := legacy.RenderReply(context.Background(), "tenant-1", principal, created.ConversationID, "turn-1", map[string]any{"locale": "zh", "fallback_text": "请补充名称"})
		if err != nil || reply["conversation_id"] != created.ConversationID || reply["turn_id"] != "turn-1" {
			t.Fatalf("reply=%+v err=%v", reply, err)
		}
	})

	t.Run("confirm turn propagates invalid state", func(t *testing.T) {
		svc := newAssistantConversationService(nil, nil)
		internalPrincipal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
		now := time.Now().UTC()
		turn := &assistantTurn{
			TurnID:    "turn-1",
			State:     assistantStateCanceled,
			RequestID: "req-1",
			TraceID:   "trace-1",
			UserInput: "仅生成计划",
			RiskTier:  "low",
			Intent:    assistantIntentSpec{Action: assistantIntentPlanOnly},
			Plan:      assistantBuildPlan(assistantIntentSpec{Action: assistantIntentPlanOnly}),
			DryRun:    assistantDryRunResult{PlanHash: "plan-hash"},
			CreatedAt: now,
			UpdatedAt: now,
		}
		assistantRefreshTurnDerivedFields(turn)
		conv := &assistantConversation{
			ConversationID: "conv-1",
			TenantID:       "tenant-1",
			ActorID:        internalPrincipal.ID,
			ActorRole:      internalPrincipal.RoleSlug,
			State:          assistantStateCanceled,
			CurrentPhase:   turn.Phase,
			CreatedAt:      now,
			UpdatedAt:      now,
			Turns:          []*assistantTurn{turn},
		}
		svc.byID[conv.ConversationID] = conv
		legacy := cubeboxLegacyFacade{assistant: svc}
		if _, err := legacy.ConfirmTurn(context.Background(), "tenant-1", cubeboxmodule.Principal{ID: internalPrincipal.ID, RoleSlug: internalPrincipal.RoleSlug}, conv.ConversationID, turn.TurnID, ""); !errors.Is(err, errAssistantConversationStateInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("render reply rejects invalid payload", func(t *testing.T) {
		legacy := cubeboxLegacyFacade{assistant: newAssistantConversationService(nil, nil)}
		if _, err := legacy.RenderReply(context.Background(), "tenant-1", principal, "conv-1", "turn-1", map[string]any{"bad": func() {}}); err == nil {
			t.Fatal("expected marshal error")
		}
	})

	t.Run("render reply nil payload uses zero-value request", func(t *testing.T) {
		legacy := cubeboxLegacyFacade{assistant: newAssistantConversationService(nil, nil)}
		reply, err := legacy.RenderReply(context.Background(), "tenant-1", principal, "conv-1", "turn-1", nil)
		if !errors.Is(err, errAssistantConversationNotFound) {
			t.Fatalf("reply=%+v err=%v", reply, err)
		}
	})

		t.Run("conversation and task errors map to formal errors", func(t *testing.T) {
			legacy := cubeboxLegacyFacade{assistant: newAssistantConversationService(nil, nil)}
			conv := legacy.assistant.createConversation("tenant-1", Principal{ID: principal.ID, RoleSlug: principal.RoleSlug})

		if _, err := legacy.GetConversation(context.Background(), "tenant-x", principal.ID, conv.ConversationID); !errors.Is(err, cubeboxservices.ErrTenantMismatch) {
			t.Fatalf("tenant mismatch err=%v", err)
		}
		if _, err := legacy.GetConversation(context.Background(), "tenant-1", "other-actor", conv.ConversationID); !errors.Is(err, cubeboxservices.ErrConversationForbidden) {
			t.Fatalf("forbidden err=%v", err)
		}
		if _, err := legacy.GetConversation(context.Background(), "tenant-1", principal.ID, "missing"); !errors.Is(err, cubeboxservices.ErrConversationNotFound) {
			t.Fatalf("not found err=%v", err)
		}
			if _, err := legacy.GetTask(context.Background(), "tenant-1", principal, "task-missing"); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
				t.Fatalf("expected workflow unavailable, got %v", err)
			}
		})

		t.Run("legacy facade preserves service-side raw errors", func(t *testing.T) {
			svc := newAssistantConversationService(nil, nil)
			legacy := cubeboxLegacyFacade{assistant: svc}

			if _, _, err := legacy.ListConversations(context.Background(), "tenant-1", principal.ID, 10, "%%%"); err == nil || !strings.Contains(err.Error(), errAssistantConversationCursorInvalid.Error()) {
				t.Fatalf("expected cursor error, got %v", err)
			}
			if _, err := legacy.GetTask(context.Background(), "tenant-1", principal, "task-1"); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
				t.Fatalf("expected workflow unavailable, got %v", err)
			}
			if _, err := legacy.SubmitTask(context.Background(), "tenant-1", principal, cubeboxdomain.TaskSubmitRequest{}); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
				t.Fatalf("expected submit workflow unavailable, got %v", err)
			}
			if _, err := legacy.CommitTurn(context.Background(), "tenant-1", principal, "conv-1", "turn-1"); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
				t.Fatalf("expected commit workflow unavailable, got %v", err)
			}
			if _, err := legacy.CancelTask(context.Background(), "tenant-1", principal, "task-1"); !errors.Is(err, errAssistantTaskWorkflowUnavailable) {
				t.Fatalf("expected cancel workflow unavailable, got %v", err)
			}
		})

		t.Run("bridge surfaces additional raw and success paths", func(t *testing.T) {
			svc := newAssistantConversationService(nil, nil)
			legacy := cubeboxLegacyFacade{assistant: svc}
			svc.byID["conv-corrupted"] = nil
			if _, err := legacy.GetConversation(context.Background(), "tenant-1", principal.ID, "conv-corrupted"); !errors.Is(err, errAssistantConversationCorrupted) {
				t.Fatalf("expected corrupted error, got %v", err)
			}

			svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
			if _, err := legacy.CreateConversation(context.Background(), "tenant-1", principal); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected create begin error, got %v", err)
			}
			if _, err := legacy.CreateTurn(context.Background(), "tenant-1", principal, "conv-1", "hello"); err == nil || !strings.Contains(err.Error(), "begin failed") {
				t.Fatalf("expected create turn begin error, got %v", err)
			}

			svc = newAssistantConversationService(nil, nil)
			internalPrincipal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
			legacy = cubeboxLegacyFacade{assistant: svc}
			svc.pool = assistFakeTxBeginner{err: errors.New("workflow begin failed")}
			if _, err := legacy.ExecuteTaskWorkflow(nil, "tenant-1", cubeboxmodule.Principal{ID: internalPrincipal.ID, RoleSlug: internalPrincipal.RoleSlug}, &cubeboxdomain.Conversation{ConversationID: "conv-1"}, "turn-1"); err == nil || !strings.Contains(err.Error(), "workflow begin failed") {
				t.Fatalf("expected workflow begin error, got %v", err)
			}
		})

		t.Run("submit get cancel commit and workflow bridge pg task path", func(t *testing.T) {
			svc, principalInternal, confirmedTurn, txNow := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		confirmedTurn.TurnID = "turn-commit"
		confirmedTurn.RequestID = "req-commit"
		confirmedTurn.TraceID = "trace-commit"
		confirmedTurn.CreatedAt = txNow
		confirmedTurn.UpdatedAt = txNow
		svc.byID["conv_1"] = &assistantConversation{
			ConversationID: "conv_1",
			TenantID:       "tenant_1",
			ActorID:        principalInternal.ID,
			ActorRole:      principalInternal.RoleSlug,
			State:          assistantStateConfirmed,
			CurrentPhase:   assistantPhaseAwaitCommitConfirm,
			CreatedAt:      txNow,
			UpdatedAt:      txNow,
			Turns:          []*assistantTurn{confirmedTurn},
		}
		submitTaskRow := assistantTaskRecord{
			TaskID:             "task_submit_1",
			TenantID:           "tenant_1",
			ConversationID:     "conv_1",
			TurnID:             confirmedTurn.TurnID,
			TaskType:           assistantTaskTypeAsyncPlan,
			RequestID:          "req-submit",
			RequestHash:        "hash-submit",
			WorkflowID:         assistantTaskWorkflowID("tenant_1", "conv_1", confirmedTurn.TurnID, "req-submit"),
			Status:             assistantTaskStatusQueued,
			DispatchStatus:     assistantTaskDispatchPending,
			DispatchAttempt:    0,
			DispatchDeadlineAt: txNow.Add(time.Minute),
			Attempt:            0,
			MaxAttempts:        3,
			TraceID:            "trace-submit",
			ContractSnapshot:   assistantTaskSnapshotFromTurn(confirmedTurn),
			SubmittedAt:        txNow,
			CreatedAt:          txNow,
			UpdatedAt:          txNow,
		}
		submitTx := makeAssistantTaskBridgeTx(t, principalInternal, confirmedTurn, submitTaskRow, txNow)
		svc.pool = assistFakeTxBeginner{tx: submitTx}
		legacy := cubeboxLegacyFacade{assistant: svc}
		taskPrincipal := cubeboxmodule.Principal{ID: principalInternal.ID, RoleSlug: principalInternal.RoleSlug}

		receipt, err := legacy.SubmitTask(context.Background(), "tenant_1", taskPrincipal, cubeboxdomain.TaskSubmitRequest{
			ConversationID: "conv_1",
			TurnID:         confirmedTurn.TurnID,
			TaskType:       assistantTaskTypeAsyncPlan,
			RequestID:      "req-submit",
			TraceID:        "trace-submit",
			ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
				IntentSchemaVersion:      submitTaskRow.ContractSnapshot.IntentSchemaVersion,
				CompilerContractVersion:  submitTaskRow.ContractSnapshot.CompilerContractVersion,
				CapabilityMapVersion:     submitTaskRow.ContractSnapshot.CapabilityMapVersion,
				SkillManifestDigest:      submitTaskRow.ContractSnapshot.SkillManifestDigest,
				ContextHash:              submitTaskRow.ContractSnapshot.ContextHash,
				IntentHash:               submitTaskRow.ContractSnapshot.IntentHash,
				PlanHash:                 submitTaskRow.ContractSnapshot.PlanHash,
				KnowledgeSnapshotDigest:  submitTaskRow.ContractSnapshot.KnowledgeSnapshotDigest,
				RouteCatalogVersion:      submitTaskRow.ContractSnapshot.RouteCatalogVersion,
				ResolverContractVersion:  submitTaskRow.ContractSnapshot.ResolverContractVersion,
				ContextTemplateVersion:   submitTaskRow.ContractSnapshot.ContextTemplateVersion,
				ReplyGuidanceVersion:     submitTaskRow.ContractSnapshot.ReplyGuidanceVersion,
				PolicyContextDigest:      submitTaskRow.ContractSnapshot.PolicyContextDigest,
				EffectivePolicyVersion:   submitTaskRow.ContractSnapshot.EffectivePolicyVersion,
				ResolvedSetID:            submitTaskRow.ContractSnapshot.ResolvedSetID,
				SetIDSource:              submitTaskRow.ContractSnapshot.SetIDSource,
				PrecheckProjectionDigest: submitTaskRow.ContractSnapshot.PrecheckProjectionDigest,
				MutationPolicyVersion:    submitTaskRow.ContractSnapshot.MutationPolicyVersion,
			},
		})
		if err != nil || receipt == nil || strings.TrimSpace(receipt.TaskID) == "" || receipt.TaskType != assistantTaskTypeAsyncPlan || receipt.Status != assistantTaskStatusQueued || receipt.PollURI != "/internal/cubebox/tasks/"+receipt.TaskID {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
		submitTaskRow.TaskID = receipt.TaskID
		submitTaskRow.WorkflowID = receipt.WorkflowID
		submitTaskRow.Status = receipt.Status

		getTx := makeAssistantTaskBridgeGetCancelTx(submitTaskRow, principalInternal, txNow, false)
		svc.pool = assistFakeTxBeginner{tx: getTx}
		detail, err := legacy.GetTask(context.Background(), "tenant_1", taskPrincipal, submitTaskRow.TaskID)
		if err != nil || detail == nil || detail.TaskID != submitTaskRow.TaskID || detail.ContractSnapshot.PlanHash != submitTaskRow.ContractSnapshot.PlanHash {
			t.Fatalf("detail=%+v err=%v", detail, err)
		}

		cancelTx := makeAssistantTaskBridgeGetCancelTx(submitTaskRow, principalInternal, txNow, true)
		svc.pool = assistFakeTxBeginner{tx: cancelTx}
		cancelResp, err := legacy.CancelTask(context.Background(), "tenant_1", taskPrincipal, submitTaskRow.TaskID)
		if err != nil || cancelResp == nil || !cancelResp.CancelAccepted || cancelResp.TaskID != submitTaskRow.TaskID {
			t.Fatalf("cancelResp=%+v err=%v", cancelResp, err)
		}

		commitTaskRow := submitTaskRow
		commitTaskRow.TaskID = "task_commit_1"
		commitTaskRow.RequestID = confirmedTurn.RequestID
		commitTaskRow.TraceID = confirmedTurn.TraceID
		commitTaskRow.WorkflowID = assistantTaskWorkflowID("tenant_1", "conv_1", confirmedTurn.TurnID, confirmedTurn.RequestID)
		commitReq, err := assistantBuildTaskSubmitRequestFromTurn("conv_1", confirmedTurn)
		if err != nil {
			t.Fatalf("build commit req err=%v", err)
		}
		commitHash, err := assistantTaskRequestHash(commitReq)
		if err != nil {
			t.Fatalf("build commit hash err=%v", err)
		}
		commitTaskRow.RequestHash = commitHash
		commitTaskRow.ContractSnapshot = assistantTaskSnapshotFromTurn(confirmedTurn)
		commitTx := newAssistantCommitTx(txNow, principalInternal.ID, principalInternal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		commitTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantConversationRowWithRole("conv_1", principalInternal.ID, principalInternal.RoleSlug, assistantStateConfirmed, txNow)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "SELECT request_hash, status, http_status, error_code, response_body"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(commitTaskRow)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		svc.pool = assistFakeTxBeginner{tx: commitTx}
		commitReceipt, err := legacy.CommitTurn(context.Background(), "tenant_1", taskPrincipal, "conv_1", confirmedTurn.TurnID)
		if err != nil || commitReceipt == nil || commitReceipt.TaskID != commitTaskRow.TaskID {
			t.Fatalf("commitReceipt=%+v err=%v", commitReceipt, err)
		}

		workflowTx := newAssistantCommitTx(txNow, principalInternal.ID, principalInternal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		svc.pool = assistFakeTxBeginner{tx: workflowTx}
		result, err := legacy.ExecuteTaskWorkflow(context.Background(), "tenant_1", taskPrincipal, mapAssistantConversation(svc.byID["conv_1"]), confirmedTurn.TurnID)
		if err != nil || result.Conversation == nil {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})

	t.Run("workflow bridge early returns and commit error", func(t *testing.T) {
		svc, principalInternal, confirmedTurn, txNow := newAssistantCommitCoverageEnv(t, assistantStateConfirmed)
		svc.byID["conv_1"] = &assistantConversation{
			ConversationID: "conv_1",
			TenantID:       "tenant_1",
			ActorID:        principalInternal.ID,
			ActorRole:      principalInternal.RoleSlug,
			State:          assistantStateConfirmed,
			CurrentPhase:   assistantPhaseAwaitCommitConfirm,
			CreatedAt:      txNow,
			UpdatedAt:      txNow,
			Turns:          []*assistantTurn{confirmedTurn},
		}
		taskPrincipal := cubeboxmodule.Principal{ID: principalInternal.ID, RoleSlug: principalInternal.RoleSlug}
		legacy := cubeboxLegacyFacade{assistant: svc}

		tx := &assistFakeTx{}
		svc.pool = assistFakeTxBeginner{tx: tx}
		result, err := legacy.ExecuteTaskWorkflow(context.Background(), "tenant_1", taskPrincipal, mapAssistantConversation(svc.byID["conv_1"]), "turn-missing")
		if err != nil || result.ApplyErrorCode != cubeboxservices.ErrTurnNotFound.Error() {
			t.Fatalf("result=%+v err=%v", result, err)
		}

		svc.pool = assistFakeTxBeginner{tx: tx}
		result, err = legacy.ExecuteTaskWorkflow(context.Background(), "tenant_1", taskPrincipal, nil, confirmedTurn.TurnID)
		if err != nil || result.ApplyErrorCode != cubeboxservices.ErrConversationNotFound.Error() {
			t.Fatalf("result=%+v err=%v", result, err)
		}

		commitErrTx := newAssistantCommitTx(txNow, principalInternal.ID, principalInternal.RoleSlug, assistantStateConfirmed, &assistFakeRows{rows: [][]any{assistantTurnRowValues(confirmedTurn)}})
		commitErrTx.commitErr = errors.New("commit failed")
		svc.pool = assistFakeTxBeginner{tx: commitErrTx}
		_, err = legacy.ExecuteTaskWorkflow(context.Background(), "tenant_1", taskPrincipal, mapAssistantConversation(svc.byID["conv_1"]), confirmedTurn.TurnID)
		if err == nil || !strings.Contains(err.Error(), "commit failed") {
			t.Fatalf("expected commit error, got %v", err)
		}
	})

		t.Run("runtime probe models and statuses", func(t *testing.T) {
			probe := cubeboxRuntimeProbe{}
		if got := probe.BackendStatus(context.Background()); got.Reason != "assistant_service_missing" {
			t.Fatalf("backend=%+v", got)
		}
		if got := probe.KnowledgeRuntimeStatus(context.Background()); got.Reason != "knowledge_runtime_missing" {
			t.Fatalf("knowledge=%+v", got)
		}
		if got := probe.ModelGatewayStatus(context.Background()); got.Reason != "model_gateway_missing" {
			t.Fatalf("model gateway=%+v", got)
		}
		models, err := probe.Models(context.Background())
		if err != nil || models != nil {
			t.Fatalf("models=%+v err=%v", models, err)
		}

		probe = cubeboxRuntimeProbe{assistant: &assistantConversationService{
			modelGateway: &assistantModelGateway{
				config: assistantModelConfig{
					Providers: []assistantModelProviderConfig{
						{Name: "openai", Enabled: true, Model: "gpt-5.4", Endpoint: "builtin://openai", TimeoutMS: 1, Priority: 1, KeyRef: "OPENAI_API_KEY"},
						{Name: "openai", Enabled: true, Model: "gpt-5.4-mini", Endpoint: "builtin://openai", TimeoutMS: 1, Priority: 2, KeyRef: "OPENAI_API_KEY"},
					},
				},
			},
		}}
		models, err = probe.Models(context.Background())
		if err != nil || len(models) != 2 || models[0].Provider != "openai" {
			t.Fatalf("models=%+v err=%v", models, err)
		}

			probe = cubeboxRuntimeProbe{assistant: &assistantConversationService{
				modelGateway: &assistantModelGateway{},
				gatewayErr:   errors.New("gateway down"),
			}}
			if got := probe.ModelGatewayStatus(context.Background()); got.Reason != "model_gateway_unavailable" {
				t.Fatalf("model gateway=%+v", got)
			}
			healthyProbe := cubeboxRuntimeProbe{assistant: &assistantConversationService{knowledgeErr: errors.New("knowledge down")}}
			if got := healthyProbe.KnowledgeRuntimeStatus(context.Background()); got.Reason != "knowledge_runtime_unavailable" {
				t.Fatalf("knowledge degraded=%+v", got)
			}
			if got := healthyProbe.BackendStatus(context.Background()); got.Healthy != "healthy" {
				t.Fatalf("backend=%+v", got)
			}
		})
}

func makeAssistantTaskBridgeTx(t *testing.T, principal Principal, turn *assistantTurn, task assistantTaskRecord, now time.Time) *assistFakeTx {
	t.Helper()
	return &assistFakeTx{
		queryRowFn: func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", principal.ID, assistantStateConfirmed, now)}
			case strings.Contains(sql, "INSERT INTO iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(task)}
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		},
		queryFn: func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		},
		execFn: func(sql string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(sql), nil
		},
	}
}

func makeAssistantTaskBridgeGetCancelTx(task assistantTaskRecord, principal Principal, now time.Time, cancelAccepted bool) *assistFakeTx {
	row := task
	if cancelAccepted {
		row.Status = assistantTaskStatusQueued
		row.DispatchStatus = assistantTaskDispatchPending
	}
	return &assistFakeTx{
		queryRowFn: func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_tasks"):
				return &assistFakeRow{vals: assistantTaskRowValues(row)}
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{principal.ID}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		},
		queryFn: func(sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "FROM iam.assistant_task_events") {
				return &assistFakeRows{}, nil
			}
			return &assistFakeRows{}, nil
		},
		execFn: func(sql string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(sql), nil
		},
	}
}
