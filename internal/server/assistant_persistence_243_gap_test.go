package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func assistant243CreateTurnPGTx(now time.Time, actorID string, turnRows pgx.Rows) *assistFakeTx {
	tx := &assistFakeTx{}
	tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM iam.assistant_conversations"):
			return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_pg", actorID, assistantStateValidated, now)}
		case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
			return &assistFakeRow{vals: []any{int64(1)}}
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
	tx.execFn = func(string, ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag(""), nil
	}
	return tx
}

func TestAssistantPersistence243_CreateTurnPGBranches(t *testing.T) {
	origRouteFn := assistantBuildIntentRouteDecisionFn
	origResumeFn := assistantResumeFromClarificationFn
	origClarificationFn := assistantBuildClarificationDecisionFn
	origAuthzFn := assistantLoadAuthorizerFn
	origCapability := capabilityDefinitionByKey
	defer func() {
		assistantBuildIntentRouteDecisionFn = origRouteFn
		assistantResumeFromClarificationFn = origResumeFn
		assistantBuildClarificationDecisionFn = origClarificationFn
		assistantLoadAuthorizerFn = origAuthzFn
		capabilityDefinitionByKey = origCapability
	}()
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}

	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	now := time.Now().UTC()

	t.Run("route decision error from builder", func(t *testing.T) {
		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svc.pool = assistFakeTxBeginner{tx: assistant243CreateTurnPGTx(now, "actor_1", nil)}
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime, *assistantTurn) (assistantIntentRouteDecision, error) {
			return assistantIntentRouteDecision{}, errAssistantRouteRuntimeInvalid
		}
		if _, err := svc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "仅生成计划"); !errors.Is(err, errAssistantRouteRuntimeInvalid) {
			t.Fatalf("expected route runtime invalid err=%v", err)
		}
	})

	t.Run("resume restores action and candidate selection", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		pending := &assistantTurn{
			TurnID:             "turn_pending",
			UserInput:          "在鲜花组织之下，新建一个名为待定的部门",
			State:              assistantStateValidated,
			RequestID:          "req_pending",
			TraceID:            "trace_pending",
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
			Intent: assistantIntentSpec{
				Action:              assistantIntentCreateOrgUnit,
				ParentRefText:       "鲜花组织",
				EntityName:          "待定",
				EffectiveDate:       "2026-01-01",
				IntentSchemaVersion: assistantIntentSchemaVersionV1,
			},
			Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Clarification: &assistantClarificationDecision{
				Status: assistantClarificationStatusOpen,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		pending.Plan.SkillManifestDigest = "skill_digest"
		assistantRefreshTurnDerivedFields(pending)

		svc.pool = assistFakeTxBeginner{
			tx: assistant243CreateTurnPGTx(now, "actor_1", &assistFakeRows{rows: [][]any{assistantTurnRowValues(pending)}}),
		}
		assistantResumeFromClarificationFn = func(_ *assistantTurn, _ string, _ assistantIntentSpec) assistantClarificationResumeResult {
			return assistantClarificationResumeResult{
				Intent: assistantIntentSpec{
					Action:              assistantIntentCreateOrgUnit,
					ParentRefText:       "鲜花组织",
					EntityName:          "运营部",
					EffectiveDate:       "2026-01-01",
					IntentSchemaVersion: assistantIntentSchemaVersionV1,
				},
				ResolvedCandidateID: "FLOWER-A",
				SelectedCandidateID: "FLOWER-A",
			}
		}
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime, *assistantTurn) (assistantIntentRouteDecision, error) {
			return assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindKnowledgeQA,
				IntentID:                "knowledge.general_qa",
				ConfidenceBand:          assistantRouteConfidenceLow,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          "s",
			}, nil
		}

		got, err := svc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "继续")
		if err != nil {
			t.Fatalf("createTurnPG err=%v", err)
		}
		last := latestTurn(got)
		if last == nil || last.Intent.Action != assistantIntentCreateOrgUnit || last.ResolvedCandidateID != "FLOWER-A" || last.ResolutionSource != assistantResolutionUserConfirmed {
			t.Fatalf("unexpected turn=%+v", last)
		}
	})

	t.Run("capability unregistered maps to plan boundary violation", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.pool = assistFakeTxBeginner{tx: assistant243CreateTurnPGTx(now, "actor_1", nil)}
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime, *assistantTurn) (assistantIntentRouteDecision, error) {
			return assistant243BusinessRouteDecision(), nil
		}
		capabilityDefinitionByKey = map[string]capabilityDefinition{}
		if _, err := svc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
			t.Fatalf("expected boundary violation err=%v", err)
		}
		capabilityDefinitionByKey = origCapability
	})

	t.Run("route audit mismatch", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		runtime, err := assistantLoadKnowledgeRuntimeFn()
		if err != nil {
			t.Fatalf("load runtime err=%v", err)
		}
		runtime.ContextTemplateVersion = ""
		runtime.ReplyGuidanceVersion = ""
		svc.knowledgeRuntime = runtime
		svc.pool = assistFakeTxBeginner{tx: assistant243CreateTurnPGTx(now, "actor_1", nil)}
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime, *assistantTurn) (assistantIntentRouteDecision, error) {
			return assistant243BusinessRouteDecision(), nil
		}
		if _, err := svc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected route audit mismatch err=%v", err)
		}
	})

	t.Run("route gate denied returns action gate error", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.pool = assistFakeTxBeginner{tx: assistant243CreateTurnPGTx(now, "actor_1", nil)}
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime, *assistantTurn) (assistantIntentRouteDecision, error) {
			return assistant243BusinessRouteDecision(), nil
		}
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil
		}
		if _, err := svc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantActionAuthzDenied) {
			t.Fatalf("expected action authz denied err=%v", err)
		}
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}
	})
}

func TestAssistantPersistence243_LoadConversationAndUpsertBranches(t *testing.T) {
	now := time.Now().UTC()
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})

	t.Run("loadConversationTx keeps present clarification", func(t *testing.T) {
		turn := assistantTaskSampleTurn(now)
		turn.Clarification = &assistantClarificationDecision{
			Status:            assistantClarificationStatusResolved,
			ClarificationKind: assistantClarificationKindMissingSlots,
		}
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_conversations") {
				return &assistFakeRow{vals: assistantPersistenceConversationRow("conv_1", "actor_1", assistantStateValidated, now)}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: [][]any{assistantTurnRowValues(turn)}}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		loaded, err := svc.loadConversationTx(context.Background(), tx, "tenant_1", "conv_1", false)
		if err != nil {
			t.Fatalf("load conversation err=%v", err)
		}
		if len(loaded.Turns) != 1 || loaded.Turns[0].Clarification == nil || loaded.Turns[0].Clarification.Status != assistantClarificationStatusResolved {
			t.Fatalf("unexpected loaded turn=%+v", loaded.Turns)
		}
	})

	t.Run("upsertTurnTx clarification runtime invalid", func(t *testing.T) {
		turn := &assistantTurn{
			TurnID:             "turn_invalid_clar",
			UserInput:          "输入",
			State:              assistantStateValidated,
			RequestID:          "req_invalid_clar",
			TraceID:            "trace_invalid_clar",
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
			Intent:             assistantIntentSpec{Action: assistantIntentPlanOnly, IntentSchemaVersion: assistantIntentSchemaVersionV1},
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentPlanOnly}),
			Clarification: &assistantClarificationDecision{
				Status:            assistantClarificationStatusOpen,
				ClarificationKind: assistantClarificationKindMissingSlots,
				AwaitPhase:        assistantPhaseAwaitMissingFields,
				CurrentRound:      1,
				MaxRounds:         2,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		turn.Plan.SkillManifestDigest = "skill_digest"
		tx := &assistFakeTx{}
		if err := svc.upsertTurnTx(context.Background(), tx, "tenant_1", "conv_1", turn); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("expected clarification runtime invalid err=%v", err)
		}
	})

	t.Run("upsertTurnTx route audit mismatch", func(t *testing.T) {
		route := assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceHigh,
			RouteCatalogVersion:     "2026-03-11.v1",
			KnowledgeSnapshotDigest: "sha256:test",
			ResolverContractVersion: "resolver_contract_v1",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
		}
		intent := assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			IntentSchemaVersion: assistantIntentSchemaVersionV1,
			ParentRefText:       "鲜花组织",
			EntityName:          "运营部",
			EffectiveDate:       "2026-01-01",
		}
		turn := &assistantTurn{
			TurnID:              "turn_route_mismatch",
			UserInput:           "输入",
			State:               assistantStateValidated,
			RequestID:           "req_route_mismatch",
			TraceID:             "trace_route_mismatch",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
			Intent:              intent,
			RouteDecision:       route,
			Plan:                assistantBuildPlan(intent),
			DryRun:              assistantDryRunResult{PlanHash: "plan_hash"},
			Candidates:          []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织"}},
			ResolvedCandidateID: "FLOWER-A",
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		turn.Plan.SkillManifestDigest = "skill_digest"
		turn.Plan.KnowledgeSnapshotDigest = route.KnowledgeSnapshotDigest
		turn.Plan.RouteCatalogVersion = route.RouteCatalogVersion
		turn.Plan.ResolverContractVersion = route.ResolverContractVersion
		turn.Plan.ContextTemplateVersion = ""
		turn.Plan.ReplyGuidanceVersion = "2026-03-11.v1"
		tx := &assistFakeTx{}
		if err := svc.upsertTurnTx(context.Background(), tx, "tenant_1", "conv_1", turn); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected route audit mismatch err=%v", err)
		}
	})
}
