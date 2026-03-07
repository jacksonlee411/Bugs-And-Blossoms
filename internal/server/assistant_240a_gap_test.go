package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAssistant240A_ActionRegistryGaps(t *testing.T) {
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}

	t.Run("refresh turn version tuple determinism and fallback branches", func(t *testing.T) {
		svc := &assistantConversationService{orgStore: store}
		oldHashFn := assistantPlanHashFn
		defer func() { assistantPlanHashFn = oldHashFn }()

		assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string { return "" }
		turn := &assistantTurn{
			Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			Plan:   assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			DryRun: assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, ""),
		}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantPlanDeterminismViolation) {
			t.Fatalf("expected determinism violation, got %v", err)
		}

		assistantPlanHashFn = oldHashFn
		assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string { return "" }
		turn = &assistantTurn{
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}},
			ResolvedCandidateID: "c1",
			DryRun:              assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, nil, "c1"),
		}
		if err := svc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantPlanDeterminismViolation) {
			t.Fatalf("expected determinism violation on resolved branch, got %v", err)
		}

		assistantPlanHashFn = oldHashFn
		errSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store, searchErr: errors.New("search failed")}}
		turn = &assistantTurn{
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", AsOf: "2026-01-01"}},
			ResolvedCandidateID: "c1",
			DryRun:              assistantBuildDryRun(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, nil, "c1"),
		}
		if err := errSvc.refreshTurnVersionTuple(context.Background(), "tenant_1", turn); err == nil || err.Error() != "search failed" {
			t.Fatalf("expected search failed, got %v", err)
		}
	})

	t.Run("validate version tuple fallback and event mismatch", func(t *testing.T) {
		turn := &assistantTurn{
			Intent:     assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:       assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates: []assistantCandidate{{CandidateID: "c1"}},
		}
		turn.Plan.VersionTuple = []byte(`{"parent_candidate_id":"c1"}`)
		if err := (&assistantConversationService{}).validateTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantVersionTupleStale) {
			t.Fatalf("expected stale from tuple fallback lookup, got %v", err)
		}

		now := time.Now().UTC()
		eventSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{details: OrgUnitNodeDetails{OrgID: 7, OrgCode: "FLOWER-A", EventUUID: "evt_new", UpdatedAt: now}}}
		turn = &assistantTurn{
			Intent:     assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:       assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 7}},
		}
		turn.Plan.VersionTuple = []byte(`{"parent_candidate_id":"c1","parent_event_uuid":"evt_old","parent_updated_at":"` + now.Format(time.RFC3339Nano) + `"}`)
		if err := eventSvc.validateTurnVersionTuple(context.Background(), "tenant_1", turn); !errors.Is(err, errAssistantVersionTupleStale) {
			t.Fatalf("expected stale on event uuid mismatch, got %v", err)
		}
	})

	t.Run("resolve assistant candidate org id branches", func(t *testing.T) {
		svc := &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: store}}
		if orgID, err := svc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{OrgID: 99}); err != nil || orgID != 99 {
			t.Fatalf("expected direct org id, got orgID=%d err=%v", orgID, err)
		}
		candidateIDSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{resolveErr: errOrgUnitNotFound, search: []OrgUnitSearchCandidate{{OrgID: 19, OrgCode: "FLOWER-A", Name: "鲜花组织"}}}}
		if orgID, err := candidateIDSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{CandidateID: "cid-1", AsOf: "2026-01-01"}); err != nil || orgID != 19 {
			t.Fatalf("expected candidate-id search fallback, got orgID=%d err=%v", orgID, err)
		}
		if _, err := svc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{}); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("expected candidate not found, got %v", err)
		}

		searchSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{
			orgUnitMemoryStore: store,
			resolveErr:         errOrgUnitNotFound,
			search:             []OrgUnitSearchCandidate{{OrgID: 0, OrgCode: "SKIP"}, {OrgID: 17, OrgCode: "FLOWER-A", Name: "鲜花组织"}},
		}}
		if orgID, err := searchSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{CandidateCode: "FLOWER-A", AsOf: "2026-01-01"}); err != nil || orgID != 17 {
			t.Fatalf("expected code search hit, got orgID=%d err=%v", orgID, err)
		}

		nameSvc := &assistantConversationService{orgStore: assistantOrgStoreStub{search: []OrgUnitSearchCandidate{{OrgID: 23, Name: "鲜花组织"}}}}
		if orgID, err := nameSvc.resolveAssistantCandidateOrgID(context.Background(), "tenant_1", assistantCandidate{Name: "鲜花组织", AsOf: "2026-01-01"}); err != nil || orgID != 23 {
			t.Fatalf("expected name search hit, got orgID=%d err=%v", orgID, err)
		}
	})
}

func TestAssistant240A_APIAndIntentGaps(t *testing.T) {
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

	t.Run("turns api maps schema constrained decode failure", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.gatewayErr = errAssistantPlanSchemaConstrainedDecodeFailed
		conv := svc.createConversation("tenant-1", principal)
		rec := httptest.NewRecorder()
		req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"测试"}`, true, true)
		handleAssistantConversationTurnsAPI(rec, req, svc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "ai_plan_schema_constrained_decode_failed" {
			t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
		}
	})

	t.Run("create turn bubbles refresh version tuple errors", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		conv := svc.createConversation("tenant-1", principal)
		if _, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("expected unsupported intent, got %v", err)
		}
	})

	t.Run("turn action api maps version tuple stale", func(t *testing.T) {
		now := time.Now().UTC()
		turn := &assistantTurn{
			TurnID:              "turn_1",
			UserInput:           "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
			State:               assistantStateConfirmed,
			Phase:               assistantPhaseAwaitCommitConfirm,
			RiskTier:            "high",
			RequestID:           "req_1",
			TraceID:             "trace_1",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01", IntentSchemaVersion: assistantIntentSchemaVersionV1},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}},
			ResolvedCandidateID: "c1",
			UpdatedAt:           now,
			CreatedAt:           now,
		}
		turn.Plan.CompilerContractVersion = assistantCompilerContractVersionV1
		turn.Plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
		turn.Plan.SkillManifestDigest = assistantSkillManifestDigest([]string{"org.orgunit_create"})
		turn.Plan.VersionTuple = []byte("null")
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"conv_1", "tenant-1", principal.ID, principal.RoleSlug, assistantStateConfirmed, assistantPhaseAwaitCommitConfirm, now, now}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
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
		tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.pool = assistFakeTxBeginner{tx: tx}
		rec := httptest.NewRecorder()
		req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv_1/turns/turn_1:commit", `{}`, true, true)
		handleAssistantTurnActionAPI(rec, req, svc)
		if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "ai_version_tuple_stale" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("intent retry fallback uses local extractor", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "dummy")
		attempts := 0
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.modelGateway = &assistantModelGateway{
			config: assistantModelConfig{
				ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
				Providers:       []assistantModelProviderConfig{{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1000, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"}},
			},
			adapters: map[string]assistantProviderAdapter{
				"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
					attempts++
					if attempts == 1 {
						return []byte(`{"action":"create_orgunit"}`), nil
					}
					return nil, errAssistantPlanSchemaConstrainedDecodeFailed
				}),
			},
		}
		resolved, err := svc.resolveIntent(context.Background(), "tenant-1", "conv-1", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
		if err != nil {
			t.Fatalf("resolve intent err=%v", err)
		}
		if resolved.ProviderName != "deterministic" || attempts != 2 || resolved.Intent.Action != assistantIntentCreateOrgUnit {
			t.Fatalf("unexpected resolved=%+v attempts=%d", resolved, attempts)
		}
	})

	t.Run("http client factory is callable", func(t *testing.T) {
		if assistantDefaultOpenAIHTTPClient() != nil {
			t.Fatal("expected nil default helper client")
		}
		_ = assistantOpenAIHTTPClientFactory()
	})
}

func TestAssistant240A_PersistenceGaps(t *testing.T) {
	ctx := context.Background()
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(ctx, "tenant_1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}

	t.Run("create turn pg tolerates org not found candidate search", func(t *testing.T) {
		svc := newAssistantConversationService(assistantOrgStoreStub{orgUnitMemoryStore: store, searchErr: errOrgUnitNotFound}, assistantWriteServiceStub{store: store})
		now := time.Now().UTC()
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"conv_pg", "tenant_1", principal.ID, principal.RoleSlug, assistantStateDraft, assistantPhaseIdle, now, now}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		svc.pool = assistFakeTxBeginner{tx: tx}
		conversation, err := svc.createTurnPG(ctx, "tenant_1", principal, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
		if err != nil {
			t.Fatalf("createTurnPG err=%v", err)
		}
		if len(conversation.Turns) != 1 || conversation.Turns[0].AmbiguityCount != 0 {
			t.Fatalf("unexpected conversation=%+v", conversation)
		}
	})

	t.Run("create turn pg bubbles refresh tuple error", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		now := time.Now().UTC()
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_conversations") {
				return &assistFakeRow{vals: []any{"conv_pg", "tenant_1", principal.ID, principal.RoleSlug, assistantStateDraft, assistantPhaseIdle, now, now}}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		svc.pool = assistFakeTxBeginner{tx: tx}
		if _, err := svc.createTurnPG(ctx, "tenant_1", principal, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("expected unsupported intent, got %v", err)
		}
	})

	t.Run("apply confirm refresh error and commit adapter fallback", func(t *testing.T) {
		unsupportedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		unsupportedSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		conversation := &assistantConversation{TenantID: "tenant_1", State: assistantStateValidated, CurrentPhase: assistantPhaseAwaitCandidateConfirm, UpdatedAt: time.Now().UTC()}
		turn := &assistantTurn{
			TurnID:              "turn_1",
			State:               assistantStateValidated,
			Phase:               assistantPhaseAwaitCandidateConfirm,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}},
			ResolvedCandidateID: "c1",
			AmbiguityCount:      1,
		}
		if _, err := unsupportedSvc.applyConfirmTurn(conversation, turn, principal, "c1"); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("expected unsupported intent, got %v", err)
		}

		commitSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		conversation = &assistantConversation{ConversationID: "conv_1", TenantID: "tenant_1", ActorID: principal.ID, ActorRole: principal.RoleSlug, State: assistantStateConfirmed, CurrentPhase: assistantPhaseAwaitCommitConfirm, UpdatedAt: time.Now().UTC()}
		turn = &assistantTurn{
			TurnID:              "turn_1",
			State:               assistantStateConfirmed,
			Phase:               assistantPhaseAwaitCommitConfirm,
			RequestID:           "req_1",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}},
			ResolvedCandidateID: "c1",
			UpdatedAt:           time.Now().UTC(),
		}
		if err := commitSvc.refreshTurnVersionTuple(ctx, "tenant_1", turn); err != nil {
			t.Fatalf("refresh tuple err=%v", err)
		}
		turn.Plan.CommitAdapterKey = ""
		if result, err := commitSvc.applyCommitTurn(ctx, conversation, turn, principal, "tenant_1"); err != nil || result.Transition == nil || turn.CommitResult == nil {
			t.Fatalf("unexpected result=%+v err=%v turn=%+v", result, err, turn)
		}
	})

	t.Run("load conversation restores error code and upsert nil candidates", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		now := time.Now().UTC()
		turn := &assistantTurn{
			TurnID:             "turn_1",
			UserInput:          "输入",
			State:              assistantStateValidated,
			RiskTier:           "high",
			RequestID:          "req_1",
			TraceID:            "trace_1",
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
			Intent:             assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"},
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			ErrorCode:          "boom",
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		row := assistantTurnRowValues(turn)
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "FROM iam.assistant_conversations") {
				return &assistFakeRow{vals: []any{"conv_1", "tenant_1", principal.ID, principal.RoleSlug, assistantStateValidated, assistantPhaseAwaitCommitConfirm, now, now}}
			}
			return &assistFakeRow{err: pgx.ErrNoRows}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: [][]any{row}}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		loaded, err := svc.loadConversationTx(ctx, tx, "tenant_1", "conv_1", true)
		if err != nil || len(loaded.Turns) != 1 || loaded.Turns[0].ErrorCode != "boom" {
			t.Fatalf("unexpected loaded=%+v err=%v", loaded, err)
		}

		upsertTx := &assistFakeTx{}
		upsertTx.queryRowFn = func(string, ...any) pgx.Row { return &assistFakeRow{vals: []any{int64(1)}} }
		nilCandidateTurn := &assistantTurn{
			TurnID:             "turn_nil",
			UserInput:          "输入",
			State:              assistantStateValidated,
			RiskTier:           "low",
			RequestID:          "req_nil",
			TraceID:            "trace_nil",
			PolicyVersion:      capabilityPolicyVersionBaseline,
			CompositionVersion: capabilityPolicyVersionBaseline,
			MappingVersion:     capabilityPolicyVersionBaseline,
			Intent:             assistantIntentSpec{Action: "plan_only"},
			Plan:               assistantBuildPlan(assistantIntentSpec{Action: "plan_only"}),
			DryRun:             assistantBuildDryRun(assistantIntentSpec{Action: "plan_only"}, nil, ""),
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := svc.upsertTurnTx(ctx, upsertTx, "tenant_1", "conv_1", nilCandidateTurn); err != nil {
			t.Fatalf("upsert nil candidates err=%v", err)
		}
	})

	t.Run("commit turn pg success path persists turn transition and finalize", func(t *testing.T) {
		now := time.Now().UTC()
		turn := &assistantTurn{
			TurnID:              "turn_success",
			UserInput:           "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
			State:               assistantStateConfirmed,
			Phase:               assistantPhaseAwaitCommitConfirm,
			RiskTier:            "high",
			RequestID:           "req_success",
			TraceID:             "trace_success",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01", IntentSchemaVersion: assistantIntentSchemaVersionV1},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}},
			ResolvedCandidateID: "c1",
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		turn.Plan.CompilerContractVersion = assistantCompilerContractVersionV1
		turn.Plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
		turn.Plan.SkillManifestDigest = assistantSkillManifestDigest([]string{"org.orgunit_create"})
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		if err := svc.refreshTurnVersionTuple(ctx, "tenant_1", turn); err != nil {
			t.Fatalf("refresh tuple err=%v", err)
		}
		row := assistantTurnRowValues(turn)
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"conv_success", "tenant_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, assistantPhaseAwaitCommitConfirm, now, now}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{vals: []any{1}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
				return &assistFakeRow{vals: []any{int64(1)}}
			default:
				return &assistFakeRow{err: pgx.ErrNoRows}
			}
		}
		tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_turns"):
				return &assistFakeRows{rows: [][]any{row}}, nil
			case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
				return &assistFakeRows{}, nil
			default:
				return &assistFakeRows{}, nil
			}
		}
		tx.execFn = func(string, ...any) (pgconn.CommandTag, error) { return pgconn.NewCommandTag(""), nil }
		svc.pool = assistFakeTxBeginner{tx: tx}
		committed, err := svc.commitTurnPG(ctx, "tenant_1", principal, "conv_success", "turn_success")
		if err != nil || len(committed.Turns) != 1 || committed.Turns[0].State != assistantStateCommitted || committed.Turns[0].CommitResult == nil {
			t.Fatalf("unexpected committed=%+v err=%v", committed, err)
		}
	})

	t.Run("commit turn pg success path persistence failure branches", func(t *testing.T) {
		now := time.Now().UTC()
		baseTurn := &assistantTurn{
			TurnID:              "turn_branch",
			UserInput:           "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
			State:               assistantStateConfirmed,
			Phase:               assistantPhaseAwaitCommitConfirm,
			RiskTier:            "high",
			RequestID:           "req_branch",
			TraceID:             "trace_branch",
			PolicyVersion:       capabilityPolicyVersionBaseline,
			CompositionVersion:  capabilityPolicyVersionBaseline,
			MappingVersion:      capabilityPolicyVersionBaseline,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01", IntentSchemaVersion: assistantIntentSchemaVersionV1},
			Plan:                assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 10000000}},
			ResolvedCandidateID: "c1",
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		baseTurn.Plan.CompilerContractVersion = assistantCompilerContractVersionV1
		baseTurn.Plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
		baseTurn.Plan.SkillManifestDigest = assistantSkillManifestDigest([]string{"org.orgunit_create"})
		prepareRow := func(t *testing.T) []any {
			t.Helper()
			svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
			turn := *baseTurn
			if err := svc.refreshTurnVersionTuple(ctx, "tenant_1", &turn); err != nil {
				t.Fatalf("refresh tuple err=%v", err)
			}
			return assistantTurnRowValues(&turn)
		}
		makeSvc := func(t *testing.T, execNeedle string, execErr error, transitionErr error, commitErr error) *assistantConversationService {
			t.Helper()
			row := prepareRow(t)
			tx := &assistFakeTx{commitErr: commitErr}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_conversations"):
					return &assistFakeRow{vals: []any{"conv_branch", "tenant_1", principal.ID, principal.RoleSlug, assistantStateConfirmed, assistantPhaseAwaitCommitConfirm, now, now}}
				case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
					return &assistFakeRow{vals: []any{1}}
				case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions") && transitionErr != nil:
					return &assistFakeRow{err: transitionErr}
				case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
					return &assistFakeRow{vals: []any{int64(1)}}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			tx.queryFn = func(sql string, _ ...any) (pgx.Rows, error) {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_turns"):
					return &assistFakeRows{rows: [][]any{row}}, nil
				case strings.Contains(sql, "FROM iam.assistant_state_transitions"):
					return &assistFakeRows{}, nil
				default:
					return &assistFakeRows{}, nil
				}
			}
			tx.execFn = func(sql string, _ ...any) (pgconn.CommandTag, error) {
				if execErr != nil && strings.Contains(sql, execNeedle) {
					return pgconn.NewCommandTag(""), execErr
				}
				return pgconn.NewCommandTag(""), nil
			}
			svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
			svc.pool = assistFakeTxBeginner{tx: tx}
			return svc
		}
		cases := []struct {
			name          string
			execNeedle    string
			execErr       error
			transitionErr error
			commitErr     error
			want          string
		}{
			{name: "upsert failed", execNeedle: "INSERT INTO iam.assistant_turns", execErr: errors.New("upsert failed"), want: "upsert failed"},
			{name: "update failed", execNeedle: "UPDATE iam.assistant_conversations", execErr: errors.New("update failed"), want: "update failed"},
			{name: "transition failed", transitionErr: errors.New("transition failed"), want: "transition failed"},
			{name: "finalize failed", execNeedle: "UPDATE iam.assistant_idempotency", execErr: errors.New("finalize failed"), want: "finalize failed"},
			{name: "commit failed", commitErr: errors.New("commit failed"), want: "commit failed"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := makeSvc(t, tc.execNeedle, tc.execErr, tc.transitionErr, tc.commitErr)
				if _, err := svc.commitTurnPG(ctx, "tenant_1", principal, "conv_branch", "turn_branch"); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected %q, got %v", tc.want, err)
				}
			})
		}
	})

	t.Run("idempotency mapping covers version tuple stale", func(t *testing.T) {
		if err := assistantErrorFromIdempotencyCode(errAssistantVersionTupleStale.Error()); !errors.Is(err, errAssistantVersionTupleStale) {
			t.Fatalf("unexpected err=%v", err)
		}
		if status, code, ok := assistantIdempotencyErrorPayload(errAssistantVersionTupleStale); !ok || status != http.StatusConflict || code != errAssistantVersionTupleStale.Error() {
			t.Fatalf("unexpected payload status=%d code=%s ok=%v", status, code, ok)
		}
	})
}
