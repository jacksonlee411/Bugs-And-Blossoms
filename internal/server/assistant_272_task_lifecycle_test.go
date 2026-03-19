package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type assistant272ActionCase struct {
	name              string
	action            string
	intent            assistantIntentSpec
	candidates        []assistantCandidate
	resolvedCandidate string
	expectEventType   string
	userInput         string
}

func assistant272ActionCases() []assistant272ActionCase {
	return []assistant272ActionCase{
		{
			name:              "create",
			action:            assistantIntentCreateOrgUnit,
			intent:            assistantIntentSpec{Action: assistantIntentCreateOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_create", IntentHash: "intent_create", ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
			candidates:        []assistantCandidate{{CandidateID: "parent_1", CandidateCode: "FLOWER-A", Name: "鲜花组织", OrgID: 10000000, IsActive: true}},
			resolvedCandidate: "parent_1",
			expectEventType:   "CREATE",
			userInput:         "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
		},
		{
			name:            "add_version",
			action:          assistantIntentAddOrgUnitVersion,
			intent:          assistantIntentSpec{Action: assistantIntentAddOrgUnitVersion, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_add", IntentHash: "intent_add", OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营一部"},
			expectEventType: "UPDATE",
			userInput:       "为 FLOWER-C 在 2026-02-01 新增版本并改名为运营一部",
		},
		{
			name:            "insert_version",
			action:          assistantIntentInsertOrgUnitVersion,
			intent:          assistantIntentSpec{Action: assistantIntentInsertOrgUnitVersion, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_insert", IntentHash: "intent_insert", OrgCode: "FLOWER-C", EffectiveDate: "2026-02-01", NewName: "运营二部"},
			expectEventType: "UPDATE",
			userInput:       "为 FLOWER-C 在 2026-02-01 插入版本并改名为运营二部",
		},
		{
			name:            "correct",
			action:          assistantIntentCorrectOrgUnit,
			intent:          assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_correct", IntentHash: "intent_correct", OrgCode: "FLOWER-C", TargetEffectiveDate: "2026-01-01", NewName: "运营中心"},
			expectEventType: "UPDATE",
			userInput:       "更正 FLOWER-C 在 2026-01-01 的名称为运营中心",
		},
		{
			name:            "disable",
			action:          assistantIntentDisableOrgUnit,
			intent:          assistantIntentSpec{Action: assistantIntentDisableOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_disable", IntentHash: "intent_disable", OrgCode: "FLOWER-C", EffectiveDate: "2026-05-01"},
			expectEventType: "DISABLE",
			userInput:       "停用 FLOWER-C，自 2026-05-01 生效",
		},
		{
			name:            "enable",
			action:          assistantIntentEnableOrgUnit,
			intent:          assistantIntentSpec{Action: assistantIntentEnableOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_enable", IntentHash: "intent_enable", OrgCode: "FLOWER-C", EffectiveDate: "2026-06-01"},
			expectEventType: "ENABLE",
			userInput:       "启用 FLOWER-C，自 2026-06-01 生效",
		},
		{
			name:              "move",
			action:            assistantIntentMoveOrgUnit,
			intent:            assistantIntentSpec{Action: assistantIntentMoveOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_move", IntentHash: "intent_move", OrgCode: "FLOWER-C", EffectiveDate: "2026-04-01", NewParentRefText: "鲜花组织"},
			candidates:        []assistantCandidate{{CandidateID: "parent_1", CandidateCode: "FLOWER-A", Name: "鲜花组织", OrgID: 10000000, IsActive: true}},
			resolvedCandidate: "parent_1",
			expectEventType:   "MOVE",
			userInput:         "将 FLOWER-C 于 2026-04-01 调整到鲜花组织下",
		},
		{
			name:            "rename",
			action:          assistantIntentRenameOrgUnit,
			intent:          assistantIntentSpec{Action: assistantIntentRenameOrgUnit, IntentSchemaVersion: assistantIntentSchemaVersionV1, ContextHash: "ctx_rename", IntentHash: "intent_rename", OrgCode: "FLOWER-C", EffectiveDate: "2026-03-01", NewName: "运营平台部"},
			expectEventType: "RENAME",
			userInput:       "将 FLOWER-C 于 2026-03-01 更名为运营平台部",
		},
	}
}

func assistant272BuildConfirmedTurn(t *testing.T, svc *assistantConversationService, tc assistant272ActionCase, now time.Time) *assistantTurn {
	t.Helper()
	turn := &assistantTurn{
		TurnID:              "turn_" + tc.name,
		UserInput:           tc.userInput,
		State:               assistantStateConfirmed,
		RequestID:           "req_" + tc.name,
		TraceID:             "trace_" + tc.name,
		PolicyVersion:       capabilityPolicyVersionBaseline,
		CompositionVersion:  capabilityPolicyVersionBaseline,
		MappingVersion:      capabilityPolicyVersionBaseline,
		Intent:              tc.intent,
		Plan:                assistantBuildPlan(tc.intent),
		Candidates:          append([]assistantCandidate(nil), tc.candidates...),
		ResolvedCandidateID: tc.resolvedCandidate,
		DryRun:              assistantBuildDryRun(tc.intent, tc.candidates, tc.resolvedCandidate),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	turn.Plan.SkillManifestDigest = "skill_" + tc.name
	assistantTestAttachBusinessRoute(turn)
	if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err != nil {
		t.Fatalf("refreshTurnVersionTuple action=%s err=%v", tc.action, err)
	}
	assistantRefreshTurnDerivedFields(turn)
	return turn
}

func assistant272Conversation(turn *assistantTurn, now time.Time) *assistantConversation {
	return &assistantConversation{
		ConversationID: "conv_1",
		TenantID:       "tenant_1",
		ActorID:        "actor_1",
		ActorRole:      "tenant-admin",
		State:          turn.State,
		CurrentPhase:   turn.Phase,
		CreatedAt:      now,
		UpdatedAt:      now,
		Turns:          []*assistantTurn{turn},
	}
}

func assistant272TurnSnapshotRow(t *testing.T, turn *assistantTurn) []any {
	t.Helper()
	intentJSON, err := json.Marshal(turn.Intent)
	if err != nil {
		t.Fatalf("marshal intent err=%v", err)
	}
	planJSON, err := json.Marshal(turn.Plan)
	if err != nil {
		t.Fatalf("marshal plan err=%v", err)
	}
	dryRunJSON, err := json.Marshal(turn.DryRun)
	if err != nil {
		t.Fatalf("marshal dryrun err=%v", err)
	}
	return []any{intentJSON, planJSON, dryRunJSON}
}

func TestAssistant272PrepareCommitTurn_ActionMatrix(t *testing.T) {
	originalAuthorizer := assistantLoadAuthorizerFn
	originalDefinitions := capabilityDefinitionByKey
	defer func() {
		assistantLoadAuthorizerFn = originalAuthorizer
		capabilityDefinitionByKey = originalDefinitions
	}()
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}

	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	capabilityDefinitionByKey = map[string]capabilityDefinition{}
	for _, tc := range assistant272ActionCases() {
		spec, ok := assistantLookupDefaultActionSpec(tc.action)
		if !ok {
			t.Fatalf("missing spec action=%s", tc.action)
		}
		capabilityDefinitionByKey[spec.CapabilityKey] = capabilityDefinition{CapabilityKey: spec.CapabilityKey, Status: routeCapabilityStatusActive, ActivationState: routeCapabilityStatusActive}
	}

	now := time.Now().UTC()
	for _, tc := range assistant272ActionCases() {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &assistantWriteServiceRecorder{}
			svc := newAssistantConversationService(store, recorder)
			turn := assistant272BuildConfirmedTurn(t, svc, tc, now)
			conversation := assistant272Conversation(turn, now)
			prepared, result, err := svc.prepareCommitTurn(context.Background(), conversation, turn, Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "tenant_1")
			if err != nil {
				t.Fatalf("prepareCommitTurn action=%s err=%v result=%+v", tc.action, err, result)
			}
			if prepared.Adapter == nil {
				t.Fatalf("prepareCommitTurn action=%s missing adapter", tc.action)
			}

			assistantLoadAuthorizerFn = func() (authorizer, error) {
				return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil
			}
			_, result, err = svc.prepareCommitTurn(context.Background(), assistant272Conversation(assistant272BuildConfirmedTurn(t, svc, tc, now), now), assistant272BuildConfirmedTurn(t, svc, tc, now), Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "tenant_1")
			if !errors.Is(err, errAssistantActionAuthzDenied) {
				t.Fatalf("prepareCommitTurn reject action=%s err=%v result=%+v", tc.action, err, result)
			}
			assistantLoadAuthorizerFn = func() (authorizer, error) {
				return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
			}
		})
	}

	t.Run("route boundary failures short-circuit before adapter lookup", func(t *testing.T) {
		recorder := &assistantWriteServiceRecorder{}
		svc := newAssistantConversationService(store, recorder)
		turn := assistant272BuildConfirmedTurn(t, svc, assistant272ActionCases()[0], now)
		turn.RouteDecision = assistantIntentRouteDecision{}
		conversation := assistant272Conversation(turn, now)
		_, result, err := svc.prepareCommitTurn(context.Background(), conversation, turn, Principal{ID: "actor_1", RoleSlug: "tenant-admin"}, "tenant_1")
		if !errors.Is(err, errAssistantRouteDecisionMissing) {
			t.Fatalf("prepareCommitTurn missing route err=%v result=%+v", err, result)
		}
	})
}

func TestAssistant272SubmitCommitTaskWorkflowAndPoll_ActionMatrix(t *testing.T) {
	originalAuthorizer := assistantLoadAuthorizerFn
	originalDefinitions := capabilityDefinitionByKey
	defer func() {
		assistantLoadAuthorizerFn = originalAuthorizer
		capabilityDefinitionByKey = originalDefinitions
	}()
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}

	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	capabilityDefinitionByKey = map[string]capabilityDefinition{}
	for _, tc := range assistant272ActionCases() {
		spec, ok := assistantLookupDefaultActionSpec(tc.action)
		if !ok {
			t.Fatalf("missing spec action=%s", tc.action)
		}
		capabilityDefinitionByKey[spec.CapabilityKey] = capabilityDefinition{CapabilityKey: spec.CapabilityKey, Status: routeCapabilityStatusActive, ActivationState: routeCapabilityStatusActive}
	}

	now := time.Now().UTC()
	for _, tc := range assistant272ActionCases() {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &assistantWriteServiceRecorder{}
			svc := newAssistantConversationService(store, recorder)
			turn := assistant272BuildConfirmedTurn(t, svc, tc, now)
			conversation := assistant272Conversation(turn, now)
			turnRows := &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}

			submitTx := &assistFakeTx{}
			submitTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_conversations"):
					return &assistFakeRow{vals: assistantConversationRowWithRole(conversation.ConversationID, conversation.ActorID, conversation.ActorRole, conversation.State, now)}
				case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
					return &assistFakeRow{vals: []any{1}}
				case strings.Contains(sql, "SELECT request_hash, status"):
					return &assistFakeRow{err: pgx.ErrNoRows}
				case strings.Contains(sql, "FROM iam.assistant_tasks"):
					return &assistFakeRow{err: pgx.ErrNoRows}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			submitTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_turns"):
					return turnRows, nil
				case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
					return &assistFakeRows{}, nil
				default:
					return &assistFakeRows{}, nil
				}
			}
			submitTx.execFn = func(string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag(""), nil
			}
			svc.pool = assistFakeTxBeginner{tx: submitTx}

			receipt, err := svc.submitCommitTaskPG(context.Background(), "tenant_1", Principal{ID: conversation.ActorID, RoleSlug: conversation.ActorRole}, conversation.ConversationID, turn.TurnID)
			if err != nil {
				t.Fatalf("submitCommitTaskPG action=%s err=%v", tc.action, err)
			}
			if strings.TrimSpace(receipt.TaskID) == "" || receipt.Status != assistantTaskStatusQueued {
				t.Fatalf("unexpected receipt=%+v", receipt)
			}

			req, err := assistantBuildTaskSubmitRequestFromTurn(conversation.ConversationID, turn)
			if err != nil {
				t.Fatalf("build task request action=%s err=%v", tc.action, err)
			}
			requestHash, err := assistantTaskRequestHash(req)
			if err != nil {
				t.Fatalf("task hash action=%s err=%v", tc.action, err)
			}
			task := assistantTaskRecordFromSubmitRequest("tenant_1", req, requestHash, now)
			task.TaskID = receipt.TaskID
			task.WorkflowID = receipt.WorkflowID

			workflowTx := &assistFakeTx{}
			workflowTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "SELECT intent_json, plan_json, dry_run_json"):
					return &assistFakeRow{vals: assistant272TurnSnapshotRow(t, turn)}
				case strings.Contains(sql, "FROM iam.assistant_conversations"):
					return &assistFakeRow{vals: assistantConversationRowWithRole(conversation.ConversationID, conversation.ActorID, conversation.ActorRole, conversation.State, now)}
				case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
					return &assistFakeRow{vals: []any{int64(1)}}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			workflowTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_turns"):
					return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil
				case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
					return &assistFakeRows{}, nil
				default:
					return &assistFakeRows{}, nil
				}
			}
			workflowTx.execFn = func(string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag(""), nil
			}
			if err := svc.executeAssistantTaskWorkflowTx(context.Background(), workflowTx, "tenant_1", &task, now.Add(time.Second)); err != nil {
				t.Fatalf("executeAssistantTaskWorkflowTx action=%s err=%v", tc.action, err)
			}
			if task.Status != assistantTaskStatusSucceeded || task.CompletedAt == nil || task.LastErrorCode != "" {
				t.Fatalf("unexpected task after workflow=%+v", task)
			}
			cachedConversation, ok := svc.getCachedConversation(conversation.ConversationID)
			if !ok {
				t.Fatalf("expected cached conversation action=%s", tc.action)
			}
			cachedTurn := assistantLookupTurn(cachedConversation, turn.TurnID)
			if cachedConversation.State != assistantStateCommitted || cachedTurn == nil || cachedTurn.State != assistantStateCommitted {
				t.Fatalf("unexpected cached conversation=%+v turn=%+v", cachedConversation, cachedTurn)
			}
			if cachedTurn.CommitResult == nil || cachedTurn.CommitResult.EventType != tc.expectEventType {
				t.Fatalf("unexpected commit result=%+v", cachedTurn.CommitResult)
			}

			dispatchTx := &assistFakeTx{}
			dispatchTx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				if strings.Contains(sql, "FROM iam.assistant_task_dispatch_outbox") {
					return &assistFakeRows{}, nil
				}
				return &assistFakeRows{}, nil
			}
			pollTx := &assistFakeTx{}
			pollTx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_tasks"):
					return &assistFakeRow{vals: assistantTaskRowValues(task)}
				case strings.Contains(sql, "SELECT actor_id"):
					return &assistFakeRow{vals: []any{conversation.ActorID}}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			beginCount := 0
			svc.pool = assistTaskTxBeginner{beginFn: func(context.Context, pgx.TxOptions) (pgx.Tx, error) {
				beginCount++
				if beginCount == 1 {
					return dispatchTx, nil
				}
				return pollTx, nil
			}}
			detail, err := svc.getTaskPG(context.Background(), "tenant_1", Principal{ID: conversation.ActorID, RoleSlug: conversation.ActorRole}, task.TaskID)
			if err != nil {
				t.Fatalf("getTaskPG action=%s err=%v", tc.action, err)
			}
			if detail.Status != assistantTaskStatusSucceeded || detail.TaskID != task.TaskID || detail.ConversationID != conversation.ConversationID || detail.TurnID != turn.TurnID {
				t.Fatalf("unexpected task detail=%+v", detail)
			}
		})
	}
}
