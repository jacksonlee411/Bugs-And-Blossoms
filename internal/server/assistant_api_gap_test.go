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
				"openai": assistantDeterministicProviderAdapter{},
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
				{Name: "openai", Enabled: true, Model: "m", Endpoint: "simulate://timeout", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
			}},
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   "ai_model_timeout",
		},
		{
			name: "rate limited",
			config: assistantModelConfig{ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true}, Providers: []assistantModelProviderConfig{
				{Name: "openai", Enabled: true, Model: "m", Endpoint: "simulate://rate-limit", TimeoutMS: 1, Retries: 0, Priority: 1, KeyRef: "OPENAI_API_KEY"},
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
				return &assistFakeRow{vals: []any{"conv_1", "tenant-1", "actor-1", "tenant-admin", assistantStateValidated, now, now}}
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
				return &assistFakeRow{vals: []any{"conv_1", "tenant-1", "actor-1", "tenant-admin", assistantStateValidated, now, now}}
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
	if _, err := svc.commitTurn(context.Background(), "tenant-1", principal, "conv_1", "turn_1"); err == nil {
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
