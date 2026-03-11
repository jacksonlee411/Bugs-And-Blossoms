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
)

func TestAssistantConversationHandlers_ExtraErrorBranches(t *testing.T) {
	store := newOrgUnitMemoryStore()
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	conv := svc.createConversation("tenant-1", principal)

	rec := httptest.NewRecorder()
	req := assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations/"+conv.ConversationID, "", true, true)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-x"}))
	handleAssistantConversationDetailAPI(rec, req, svc)
	if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "tenant_mismatch" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}

	pgSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	pgSvc.pool = assistFakeTxBeginner{err: context.DeadlineExceeded}
	rec = httptest.NewRecorder()
	handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations", "", true, true), pgSvc)
	if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_conversation_create_failed" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}

	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"仅生成计划"}`, true, true)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-x"}))
	handleAssistantConversationTurnsAPI(rec, req, svc)
	if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "tenant_mismatch" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}

	created, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划")
	if err != nil {
		t.Fatalf("create turn err=%v", err)
	}
	turnID := created.Turns[0].TurnID
	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns/"+turnID+":confirm", `{}`, true, true)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-x"}))
	handleAssistantTurnActionAPI(rec, req, svc)
	if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != "tenant_mismatch" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}

	originalAnnotateFn := assistantAnnotateIntentPlanFn
	assistantAnnotateIntentPlanFn = func(string, string, string, *assistantIntentSpec, *assistantPlanSummary, *assistantDryRunResult) error {
		return errAssistantPlanDeterminismViolation
	}
	defer func() { assistantAnnotateIntentPlanFn = originalAnnotateFn }()

	if _, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划"); !errors.Is(err, errAssistantPlanDeterminismViolation) {
		t.Fatalf("unexpected err=%v", err)
	}
	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"仅生成计划"}`, true, true)
	handleAssistantConversationTurnsAPI(rec, req, svc)
	if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "ai_plan_determinism_violation" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}
}

func TestAssistantConversationTurns_ModelGatewayErrorMappings(t *testing.T) {
	newSvc := func(config assistantModelConfig) (*assistantConversationService, string) {
		store := newOrgUnitMemoryStore()
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.modelGateway = &assistantModelGateway{
			config: config,
			adapters: map[string]assistantProviderAdapter{
				"openai": assistantAdapterFunc(func(_ context.Context, _ string, provider assistantModelProviderConfig) ([]byte, error) {
					switch strings.TrimSpace(provider.Model) {
					case "timeout":
						return nil, errAssistantModelTimeout
					case "rate_limited":
						return nil, errAssistantModelRateLimited
					default:
						return []byte(`{"action":"plan_only"}`), nil
					}
				}),
			},
		}
		conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		return svc, conv.ConversationID
	}

	t.Setenv("OPENAI_API_KEY", "dummy")
	cases := []struct {
		name       string
		config     assistantModelConfig
		wantStatus int
		wantCode   string
	}{
		{
			name:       "provider unavailable",
			config:     assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: nil},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "ai_model_provider_unavailable",
		},
		{
			name: "timeout",
			config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{
				{Name: "openai", Enabled: true, Model: "timeout", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			}},
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   "ai_model_timeout",
		},
		{
			name: "rate limited",
			config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{
				{Name: "openai", Enabled: true, Model: "rate_limited", Endpoint: "https://api.openai.com/v1", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			}},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "ai_model_rate_limited",
		},
		{
			name: "config invalid",
			config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{
				{Name: "bad-provider", Enabled: true, Model: "m", Endpoint: "builtin://openai", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			}},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "ai_model_config_invalid",
		},
		{
			name: "secret missing",
			config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{
				{Name: "openai", Enabled: true, Model: "m", Endpoint: "https://example.invalid", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "MISSING_OPENAI_KEY"},
			}},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "ai_model_secret_missing",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, conversationID := newSvc(tc.config)
			rec := httptest.NewRecorder()
			req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conversationID+"/turns", `{"user_input":"测试模型错误映射"}`, true, true)
			handleAssistantConversationTurnsAPI(rec, req, svc)
			if rec.Code != tc.wantStatus || assistantDecodeErrCode(t, rec) != tc.wantCode {
				t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
			}
		})
	}
}

func TestAssistantTurnAction_RequestInProgressMappings(t *testing.T) {
	makeSvc := func(turn *assistantTurn, action string, requestHash string) *assistantConversationService {
		tx := &assistFakeTx{}
		now := time.Now().UTC()
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"conv_1", "tenant-1", "actor-1", "tenant-admin", assistantStateValidated, assistantConversationPhaseFromLegacyState(assistantStateValidated), now, now}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "SELECT request_hash"):
				return &assistFakeRow{vals: []any{requestHash, "pending", nil, nil, []byte(nil)}}
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
		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svc.pool = assistFakeTxBeginner{tx: tx}
		_ = action
		return svc
	}

	baseTurn := &assistantTurn{
		TurnID:             "turn_1",
		UserInput:          "输入",
		State:              assistantStateValidated,
		RiskTier:           "high",
		RequestID:          "req_1",
		TraceID:            "trace_1",
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		Intent: assistantIntentSpec{
			Action:              assistantIntentCreateOrgUnit,
			ParentRefText:       "鲜花组织",
			EntityName:          "运营部",
			EffectiveDate:       "2026-01-01",
			IntentSchemaVersion: assistantIntentSchemaVersionV1,
		},
		Plan:           assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
		Candidates:     []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
		AmbiguityCount: 1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	baseTurn.Plan.SkillManifestDigest = "digest"

	confirmSvc := makeSvc(baseTurn, "confirm", assistantHashText("confirm\n"))
	rec := httptest.NewRecorder()
	req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv_1/turns/turn_1:confirm", `{}`, true, true)
	handleAssistantTurnActionAPI(rec, req, confirmSvc)
	if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "request_in_progress" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}
	if rec.Header().Get("Retry-After") != assistantDefaultRetryAfterSecs {
		t.Fatalf("retry-after=%s", rec.Header().Get("Retry-After"))
	}

	commitTurn := *baseTurn
	commitTurn.State = assistantStateConfirmed
	commitTurn.ResolvedCandidateID = "c1"
	commitSvc := makeSvc(&commitTurn, "commit", assistantHashText("commit\n"))
	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv_1/turns/turn_1:commit", `{}`, true, true)
	handleAssistantTurnActionAPI(rec, req, commitSvc)
	if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "request_in_progress" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}
	if rec.Header().Get("Retry-After") != assistantDefaultRetryAfterSecs {
		t.Fatalf("retry-after=%s", rec.Header().Get("Retry-After"))
	}
}

func TestAssistantTurnAction_IdempotencyConflictMappings(t *testing.T) {
	now := time.Now().UTC()
	makeSvc := func(turn *assistantTurn, requestHash string) *assistantConversationService {
		tx := &assistFakeTx{}
		tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "FROM iam.assistant_conversations"):
				return &assistFakeRow{vals: []any{"conv_1", "tenant-1", "actor-1", "tenant-admin", assistantStateValidated, assistantConversationPhaseFromLegacyState(assistantStateValidated), now, now}}
			case strings.Contains(sql, "INSERT INTO iam.assistant_idempotency"):
				return &assistFakeRow{err: pgx.ErrNoRows}
			case strings.Contains(sql, "SELECT request_hash"):
				return &assistFakeRow{vals: []any{"different-hash", "done", 409, "", []byte(nil)}}
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
		svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		svc.pool = assistFakeTxBeginner{tx: tx}
		_ = requestHash
		return svc
	}

	confirmTurn := &assistantTurn{
		TurnID:             "turn_1",
		UserInput:          "输入",
		State:              assistantStateValidated,
		RequestID:          "req_1",
		TraceID:            "trace_1",
		Intent:             assistantIntentSpec{Action: "plan_only"},
		Plan:               assistantBuildPlan(assistantIntentSpec{Action: "plan_only"}),
		PolicyVersion:      capabilityPolicyVersionBaseline,
		CompositionVersion: capabilityPolicyVersionBaseline,
		MappingVersion:     capabilityPolicyVersionBaseline,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	confirmTurn.Plan.SkillManifestDigest = "digest"
	confirmSvc := makeSvc(confirmTurn, assistantHashText("confirm\n"))
	rec := httptest.NewRecorder()
	req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv_1/turns/turn_1:confirm", `{}`, true, true)
	handleAssistantTurnActionAPI(rec, req, confirmSvc)
	if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "idempotency_key_conflict" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}

	commitTurn := *confirmTurn
	commitTurn.State = assistantStateConfirmed
	commitSvc := makeSvc(&commitTurn, assistantHashText("commit\n"))
	rec = httptest.NewRecorder()
	req = assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/conv_1/turns/turn_1:commit", `{}`, true, true)
	handleAssistantTurnActionAPI(rec, req, commitSvc)
	if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "idempotency_key_conflict" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}
}

func TestAssistantTurnAction_RequiresIntentClarificationBeforeConfirm(t *testing.T) {
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatalf("create node err=%v", err)
	}
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	conv := svc.createConversation("tenant-1", principal)
	created, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门")
	if err != nil {
		t.Fatalf("create turn err=%v", err)
	}
	turn := latestTurn(created)
	if turn == nil {
		t.Fatal("expected turn")
	}
	if got := strings.Join(turn.DryRun.ValidationErrors, ","); !strings.Contains(got, "missing_effective_date") {
		t.Fatalf("expected missing_effective_date, got=%v", turn.DryRun.ValidationErrors)
	}

	rec := httptest.NewRecorder()
	path := "/internal/assistant/conversations/" + conv.ConversationID + "/turns/" + turn.TurnID + ":confirm"
	handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, path, `{}`, true, true), svc)
	if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != "conversation_confirmation_required" {
		t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
	}

	svc.mu.Lock()
	live := svc.byID[conv.ConversationID]
	if live == nil || len(live.Turns) == 0 {
		svc.mu.Unlock()
		t.Fatal("expected live turn")
	}
	liveTurn := live.Turns[len(live.Turns)-1]
	liveTurn.State = assistantStateConfirmed
	liveTurn.ResolvedCandidateID = "FLOWER-A"
	liveTurn.Candidates = []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织", IsActive: true}}
	liveTurn.DryRun.ValidationErrors = []string{"missing_effective_date"}
	svc.mu.Unlock()

	if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, conv.ConversationID, turn.TurnID); !errors.Is(err, errAssistantConfirmationRequired) {
		t.Fatalf("commit err=%v", err)
	}
}

func TestAssistantConversationTurns_RuntimeConfigErrorMappings(t *testing.T) {
	store := newOrgUnitMemoryStore()
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

	makeReq := func(conversationID string) *http.Request {
		return assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conversationID+"/turns", `{"user_input":"测试模型配置错误"}`, true, true)
	}

	t.Run("runtime config invalid", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.gatewayErr = errAssistantRuntimeConfigInvalid
		conv := svc.createConversation("tenant-1", principal)
		rec := httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, makeReq(conv.ConversationID), svc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "ai_runtime_config_invalid" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})

	t.Run("runtime config missing", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		svc.gatewayErr = errAssistantRuntimeConfigMissing
		conv := svc.createConversation("tenant-1", principal)
		rec := httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, makeReq(conv.ConversationID), svc)
		if rec.Code != http.StatusServiceUnavailable || assistantDecodeErrCode(t, rec) != "ai_runtime_config_missing" {
			t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
		}
	})
}

func TestAssistantServiceHelpers_PoolWrappersAndPathEdges(t *testing.T) {
	svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	svc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

	if _, err := svc.createConversationWithContext(context.Background(), "tenant-1", principal); err == nil {
		t.Fatal("expected createConversationWithContext pg error")
	}
	if _, err := svc.getConversation("tenant-1", "actor-1", "conv_1"); err == nil {
		t.Fatal("expected getConversation pg error")
	}
	if _, err := svc.createTurn(context.Background(), "tenant-1", principal, "conv_1", "仅生成计划"); err == nil {
		t.Fatal("expected createTurn pg error")
	}
	if _, err := svc.confirmTurn("tenant-1", principal, "conv_1", "turn_1", ""); err == nil {
		t.Fatal("expected confirmTurn pg error")
	}
	if _, err := assistantCommitTurnSyncForTest(svc, context.Background(), "tenant-1", principal, "conv_1", "turn_1"); err == nil {
		t.Fatal("expected commitTurn pg error")
	}

	memorySvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
	conv := memorySvc.createConversation("tenant-1", principal)
	if _, err := memorySvc.createTurn(context.Background(), "tenant-x", principal, conv.ConversationID, "仅生成计划"); !errors.Is(err, errAssistantTenantMismatch) {
		t.Fatalf("unexpected err=%v", err)
	}
	original := capabilityDefinitionByKey
	capabilityDefinitionByKey = map[string]capabilityDefinition{}
	if _, err := memorySvc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
		t.Fatalf("unexpected err=%v", err)
	}
	capabilityDefinitionByKey = original

	if _, ok := extractConversationTurnsPathConversationID("/internal/assistant/conversations/conv-1/turns/extra"); ok {
		t.Fatal("expected invalid turns path length")
	}
	if _, _, _, ok := extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/turn-1:confirm/extra"); ok {
		t.Fatal("expected invalid action path length")
	}
	if _, _, _, ok := extractAssistantTurnActionPath("/internal/assistant/conversations/conv-1/turns/turn-1:   "); ok {
		t.Fatal("expected invalid empty action")
	}
}

func TestAssistantConversationsList_HandlerAndServiceBranches(t *testing.T) {
	store := newOrgUnitMemoryStore()
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	conv1 := svc.createConversation("tenant-1", principal)
	conv2 := svc.createConversation("tenant-1", principal)
	_ = svc.createConversation("tenant-1", principal)

	svc.mu.Lock()
	if stored := svc.byID[conv2.ConversationID]; stored != nil {
		stored.Turns = []*assistantTurn{nil, &assistantTurn{
			TurnID:    "turn_1",
			UserInput: "u",
			State:     assistantStateDraft,
			RiskTier:  "low",
		}}
		stored.UpdatedAt = conv1.UpdatedAt
	}
	svc.byID["conv_nil"] = nil
	svc.byID["conv_other_actor"] = &assistantConversation{
		ConversationID: "conv_other_actor",
		TenantID:       "tenant-1",
		ActorID:        "actor-2",
		UpdatedAt:      time.Now().UTC(),
	}
	svc.byID["conv_other_tenant"] = &assistantConversation{
		ConversationID: "conv_other_tenant",
		TenantID:       "tenant-2",
		ActorID:        "actor-1",
		UpdatedAt:      time.Now().UTC(),
	}
	svc.mu.Unlock()

	items, cursor, err := svc.listConversations(context.Background(), "tenant-1", "actor-1", 0, "")
	if err != nil {
		t.Fatalf("list conversations err=%v", err)
	}
	if len(items) < 2 || cursor != "" {
		t.Fatalf("unexpected list result len=%d cursor=%q", len(items), cursor)
	}
	hasLastTurn := false
	for _, item := range items {
		if item.LastTurn != nil {
			hasLastTurn = true
		}
	}
	if !hasLastTurn {
		t.Fatalf("expected at least one item with last turn: %+v", items)
	}

	_, firstCursor, err := svc.listConversations(context.Background(), "tenant-1", "actor-1", 1, "")
	if err != nil || firstCursor == "" {
		t.Fatalf("unexpected first page err=%v cursor=%q", err, firstCursor)
	}

	items2, nextCursor, err := svc.listConversations(context.Background(), "tenant-1", "actor-1", 1, firstCursor)
	if err != nil {
		t.Fatalf("list conversations page2 err=%v", err)
	}
	if len(items2) > 1 {
		t.Fatalf("unexpected page2 result len=%d next=%q", len(items2), nextCursor)
	}

	rec := httptest.NewRecorder()
	handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?page_size=0", "", true, true), svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations?page_size=999", "", true, true), svc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	pgSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	pgSvc.pool = assistFakeTxBeginner{err: errors.New("begin failed")}
	if _, _, err := pgSvc.listConversations(context.Background(), "tenant-1", "actor-1", 20, ""); err == nil {
		t.Fatal("expected pg list error")
	}
	rec = httptest.NewRecorder()
	handleAssistantConversationsAPI(rec, assistantReqWithContext(http.MethodGet, "/internal/assistant/conversations", "", true, true), pgSvc)
	if rec.Code != http.StatusInternalServerError || assistantDecodeErrCode(t, rec) != "assistant_conversation_list_failed" {
		t.Fatalf("status=%d code=%s", rec.Code, assistantDecodeErrCode(t, rec))
	}
}

func TestAssistantHelper_LatestTurnAndTaskActionPathBranches(t *testing.T) {
	if got := latestTurn(nil); got != nil {
		t.Fatalf("expected nil latest turn, got=%+v", got)
	}
	if got := latestTurn(&assistantConversation{}); got != nil {
		t.Fatalf("expected nil latest turn for empty conversation, got=%+v", got)
	}
	if got := latestTurn(&assistantConversation{Turns: []*assistantTurn{nil, nil}}); got != nil {
		t.Fatalf("expected nil latest turn for all nil turns, got=%+v", got)
	}
	turn := &assistantTurn{TurnID: "turn_x"}
	if got := latestTurn(&assistantConversation{Turns: []*assistantTurn{nil, turn}}); got == nil || got.TurnID != turn.TurnID {
		t.Fatalf("unexpected latest turn=%+v", got)
	}

	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/task_1:   "); ok {
		t.Fatal("expected invalid empty task action")
	}
	if _, _, ok := extractAssistantTaskActionPath("/internal/assistant/tasks/   :cancel"); ok {
		t.Fatal("expected invalid empty task id")
	}
}

func TestAssistantCreateTurn_KnowledgeRuntimeErrorBranches(t *testing.T) {
	store := newOrgUnitMemoryStore()
	_, _ = store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true)
	svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

	conv := svc.createConversation("tenant-1", principal)
	svc.knowledgeRuntime = nil
	svc.knowledgeErr = errAssistantRuntimeConfigInvalid
	if _, err := svc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划"); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("expected knowledge runtime load error, got=%v", err)
	}

	conv2 := svc.createConversation("tenant-1", principal)
	svc.knowledgeErr = nil
	svc.knowledgeRuntime = &assistantKnowledgeRuntime{
		RouteCatalogVersion: "2026-03-11.v1",
		actionView:          map[string]map[string]assistantActionViewPack{},
		interpretation: map[string]map[string]assistantInterpretationPack{
			"knowledge.general_qa": {"zh": {PackID: "knowledge.general_qa", Locale: "zh"}},
		},
	}
	if _, err := svc.createTurn(
		context.Background(),
		"tenant-1",
		principal,
		conv2.ConversationID,
		"在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01",
	); !errors.Is(err, errAssistantRuntimeConfigInvalid) {
		t.Fatalf("expected plan context build error, got=%v", err)
	}
}

func TestAssistantIntentClarificationAndDryRunNonBusinessCoverage(t *testing.T) {
	if !assistantTurnRequiresIntentClarification(&assistantTurn{
		Intent: assistantIntentSpec{RouteKind: assistantRouteKindKnowledgeQA},
	}) {
		t.Fatal("non-business route should require clarification")
	}
	if !assistantTurnRequiresIntentClarification(&assistantTurn{
		RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, ClarificationRequired: true},
	}) {
		t.Fatal("route decision clarification should require clarification")
	}
	if got := assistantDryRunValidationExplain([]string{"non_business_route"}); !strings.Contains(got, "非业务动作请求") {
		t.Fatalf("unexpected explain=%q", got)
	}
}

func TestAssistantRouteHandlerMappings(t *testing.T) {
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}

	t.Run("create turn route error mappings", func(t *testing.T) {
		cases := []struct {
			err      error
			status   int
			wantCode string
		}{
			{err: errAssistantRouteRuntimeInvalid, status: http.StatusUnprocessableEntity, wantCode: errAssistantRouteRuntimeInvalid.Error()},
			{err: errAssistantRouteCatalogMissing, status: http.StatusServiceUnavailable, wantCode: errAssistantRouteCatalogMissing.Error()},
			{err: errAssistantRouteActionConflict, status: http.StatusUnprocessableEntity, wantCode: errAssistantRouteActionConflict.Error()},
			{err: errAssistantRouteDecisionMissing, status: http.StatusConflict, wantCode: errAssistantRouteDecisionMissing.Error()},
		}
		originalBuildRoute := assistantBuildIntentRouteDecisionFn
		defer func() { assistantBuildIntentRouteDecisionFn = originalBuildRoute }()
		for _, tc := range cases {
			assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime, *assistantTurn) (assistantIntentRouteDecision, error) {
				return assistantIntentRouteDecision{}, tc.err
			}
			svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
			conv := svc.createConversation("tenant-1", principal)
			rec := httptest.NewRecorder()
			handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"测试"}`, true, true), svc)
			if rec.Code != tc.status || assistantDecodeErrCode(t, rec) != tc.wantCode {
				t.Fatalf("err=%v status=%d code=%s body=%s", tc.err, rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
			}
		}
	})

	t.Run("confirm route error mappings", func(t *testing.T) {
		cases := []struct {
			name     string
			intent   assistantIntentSpec
			route    assistantIntentRouteDecision
			status   int
			wantCode string
		}{
			{name: "non business", intent: assistantIntentSpec{Action: assistantIntentPlanOnly}, route: assistantIntentRouteDecision{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa", ConfidenceBand: assistantRouteConfidenceLow, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, status: http.StatusConflict, wantCode: errAssistantRouteNonBusinessBlocked.Error()},
			{name: "clarification", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, route: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentCreateOrgUnit}, ConfidenceBand: assistantRouteConfidenceMedium, ClarificationRequired: true, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, status: http.StatusConflict, wantCode: errAssistantRouteClarificationRequired.Error()},
			{name: "missing", intent: assistantIntentSpec{Action: assistantIntentPlanOnly}, route: assistantIntentRouteDecision{}, status: http.StatusConflict, wantCode: errAssistantRouteDecisionMissing.Error()},
			{name: "runtime invalid", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, route: assistantIntentRouteDecision{RouteKind: "bad"}, status: http.StatusUnprocessableEntity, wantCode: errAssistantRouteRuntimeInvalid.Error()},
			{name: "action conflict", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, route: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", CandidateActionIDs: []string{assistantIntentRenameOrgUnit}, ConfidenceBand: assistantRouteConfidenceHigh, RouteCatalogVersion: "v1", KnowledgeSnapshotDigest: "d", ResolverContractVersion: "r", DecisionSource: "s"}, status: http.StatusUnprocessableEntity, wantCode: errAssistantRouteActionConflict.Error()},
		}
		for _, tc := range cases {
			svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
			conv := svc.createConversation("tenant-1", principal)
			turn := &assistantTurn{
				TurnID:              "turn_1",
				State:               assistantStateValidated,
				RequestID:           "req_1",
				TraceID:             "trace_1",
				Intent:              tc.intent,
				RouteDecision:       tc.route,
				Candidates:          []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}},
				ResolvedCandidateID: "c1",
				CreatedAt:           time.Now().UTC(),
				UpdatedAt:           time.Now().UTC(),
			}
			assistantRefreshTurnDerivedFields(turn)
			svc.byID[conv.ConversationID].Turns = []*assistantTurn{turn}
			rec := httptest.NewRecorder()
			handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns/turn_1:confirm", `{}`, true, true), svc)
			if rec.Code != tc.status || assistantDecodeErrCode(t, rec) != tc.wantCode {
				t.Fatalf("%s status=%d code=%s body=%s", tc.name, rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
			}
		}
	})
}
