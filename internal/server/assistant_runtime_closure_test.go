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

type assistantFlakyActionRegistry struct {
	calls int
	spec  assistantActionSpec
}

func (r *assistantFlakyActionRegistry) Lookup(string) (assistantActionSpec, bool) {
	r.calls++
	if r.calls == 1 {
		return r.spec, true
	}
	return assistantActionSpec{}, false
}

func TestAssistant240C_RuntimeAndCoverageBranches(t *testing.T) {
	assistantResetCreatePolicyRegistryStoreForTest()
	store := newOrgUnitMemoryStore()
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatal(err)
	}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	spec, _ := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)

	t.Run("evaluate and risk branches", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		defer func() { assistantLoadAuthorizerFn = original }()

		if decision := assistantCheckRiskGate(assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, Turn: &assistantTurn{State: assistantStateValidated}}); decision.Allowed || !errors.Is(decision.Error, errAssistantActionRiskGateDenied) {
			t.Fatalf("unexpected decision=%+v", decision)
		}
		if decision := assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageConfirm, TenantID: "tenant-1", Principal: principal, Action: spec, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, RouteDecision: assistantTestBusinessRouteDecision(assistantIntentCreateOrgUnit)}); decision.Allowed || !errors.Is(decision.Error, errAssistantConfirmationRequired) {
			t.Fatalf("unexpected decision=%+v", decision)
		}
		full := assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageCommit, TenantID: "tenant-1", Principal: principal, Action: spec, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-01-01"}, RouteDecision: assistantTestBusinessRouteDecision(assistantIntentCreateOrgUnit), Turn: &assistantTurn{State: assistantStateConfirmed}, ResolvedID: "c1", DryRun: &assistantDryRunResult{}})
		if !full.Allowed {
			t.Fatalf("unexpected decision=%+v", full)
		}
	})

	t.Run("helper branches", func(t *testing.T) {
		if got := assistantDryRunValidationErrorsForGate(assistantActionGateInput{ResolvedID: "", DryRun: &assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}}}); len(got) != 1 {
			t.Fatalf("unexpected errors=%v", got)
		}
		if got := assistantHydratedDryRunForGate(assistantStateValidated, assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, ""); len(got.ValidationErrors) == 0 {
			t.Fatalf("expected rebuilt validation errors, got %+v", got)
		}
		conversation := &assistantConversation{}
		turn := &assistantTurn{TurnID: "turn-1", RequestID: "req-1", TraceID: "trace-1", State: assistantStateConfirmed}
		result, err := assistantApplyGateDecision(conversation, turn, principal, "commit", assistantActionGateDecision{Allowed: false, Error: errAssistantActionRiskGateDenied, ErrorCode: errAssistantActionRiskGateDenied.Error(), ReasonCode: "blocked"})
		if !errors.Is(err, errAssistantActionRiskGateDenied) || result.Transition == nil || turn.ErrorCode == "" {
			t.Fatalf("unexpected result=%+v turn=%+v err=%v", result, turn, err)
		}
	})

	t.Run("compile merge and idempotency helpers", func(t *testing.T) {
		skill, _ := assistantCompileIntentToPlansWithSpec(assistantIntentSpec{Action: assistantIntentPlanOnly}, "", assistantActionSpec{})
		if skill.RiskTier != "low" || len(skill.RequiredChecks) == 0 {
			t.Fatalf("unexpected skill=%+v", skill)
		}
		if got := assistantRiskTierForIntent(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); got != "high" {
			t.Fatalf("risk=%s", got)
		}
		if got := assistantRiskTierForIntent(assistantIntentSpec{Action: assistantIntentPlanOnly}); got != "low" {
			t.Fatalf("risk=%s", got)
		}
		if got := assistantRiskTierForIntent(assistantIntentSpec{Action: "unknown_action"}); got != "low" {
			t.Fatalf("fallback risk=%s", got)
		}
		cases := []error{errAssistantActionSpecMissing, errAssistantActionCapabilityUnregistered, errAssistantActionAuthzDenied, errAssistantActionRiskGateDenied, errAssistantActionRequiredCheckFailed}
		for _, errValue := range cases {
			if _, code, ok := assistantIdempotencyErrorPayload(errValue); !ok || code != errValue.Error() {
				t.Fatalf("payload err=%v code=%s ok=%v", errValue, code, ok)
			}
			if got := assistantErrorFromIdempotencyCode(errValue.Error()); !errors.Is(got, errValue) {
				t.Fatalf("restore err=%v got=%v", errValue, got)
			}
		}
	})

	t.Run("api mappings for new gate errors", func(t *testing.T) {
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		conv := svc.createConversation("tenant-1", principal)
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil }
		defer func() { assistantLoadAuthorizerFn = original }()
		rec := httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"}`, true, true), svc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != errAssistantActionAuthzDenied.Error() {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("api handler edge mappings", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		defer func() { assistantLoadAuthorizerFn = original }()

		unsupportedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		unsupportedSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		conv := unsupportedSvc.createConversation("tenant-1", principal)
		rec := httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"}`, true, true), unsupportedSvc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "assistant_intent_unsupported" {
			t.Fatalf("unsupported create status=%d body=%s", rec.Code, rec.Body.String())
		}

		specMissingSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		specMissingSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentPlanOnly: {CapabilityKey: "org.assistant_conversation.manage"}}}
		conv = specMissingSvc.createConversation("tenant-1", principal)
		rec = httptest.NewRecorder()
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"仅生成计划"}`, true, true), specMissingSvc)
		if rec.Code != http.StatusOK {
			t.Fatalf("spec missing create status=%d body=%s", rec.Code, rec.Body.String())
		}

		riskSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		riskSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentPlanOnly: {ID: assistantIntentPlanOnly, CapabilityKey: "org.assistant_conversation.manage", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "extreme"}}}}
		conv = riskSvc.createConversation("tenant-1", principal)
		rec = httptest.NewRecorder()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		handleAssistantConversationTurnsAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns", `{"user_input":"仅生成计划"}`, true, true), riskSvc)
		if rec.Code != http.StatusOK {
			t.Fatalf("risk create status=%d body=%s", rec.Code, rec.Body.String())
		}

		confirmUnsupportedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		confirmUnsupportedSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		confirmConv := confirmUnsupportedSvc.createConversation("tenant-1", principal)
		confirmTurn := assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-unsupported", State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}, ResolvedCandidateID: "c1"}, nil)
		confirmUnsupportedSvc.mu.Lock()
		confirmUnsupportedSvc.byID[confirmConv.ConversationID].Turns = append(confirmUnsupportedSvc.byID[confirmConv.ConversationID].Turns, confirmTurn)
		confirmUnsupportedSvc.mu.Unlock()
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+confirmConv.ConversationID+"/turns/turn-unsupported:confirm", `{}`, true, true), confirmUnsupportedSvc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "assistant_intent_unsupported" {
			t.Fatalf("unsupported confirm status=%d body=%s", rec.Code, rec.Body.String())
		}

		confirmAuthzSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		confirmConv = confirmAuthzSvc.createConversation("tenant-1", principal)
		confirmTurn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-authz", State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}, ResolvedCandidateID: "c1"}, nil)
		confirmAuthzSvc.mu.Lock()
		confirmAuthzSvc.byID[confirmConv.ConversationID].Turns = append(confirmAuthzSvc.byID[confirmConv.ConversationID].Turns, confirmTurn)
		confirmAuthzSvc.mu.Unlock()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil }
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+confirmConv.ConversationID+"/turns/turn-authz:confirm", `{}`, true, true), confirmAuthzSvc)
		if rec.Code != http.StatusForbidden || assistantDecodeErrCode(t, rec) != errAssistantActionAuthzDenied.Error() {
			t.Fatalf("authz confirm status=%d body=%s", rec.Code, rec.Body.String())
		}

		confirmCandidateSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		confirmConv = confirmCandidateSvc.createConversation("tenant-1", principal)
		candidateSnapshot := assistantTestCreateOrgUnitProjectionSnapshot()
		candidateSnapshot.Projection.Readiness = "candidate_confirmation_required"
		candidateSnapshot.Projection.CandidateConfirmationRequirements = []string{"resolved_candidate"}
		confirmTurn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-candidate", State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}, AmbiguityCount: 2, ResolvedCandidateID: "c1"}, candidateSnapshot)
		confirmCandidateSvc.mu.Lock()
		confirmCandidateSvc.byID[confirmConv.ConversationID].Turns = append(confirmCandidateSvc.byID[confirmConv.ConversationID].Turns, confirmTurn)
		confirmCandidateSvc.mu.Unlock()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		rec = httptest.NewRecorder()
		handleAssistantTurnActionAPI(rec, assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+confirmConv.ConversationID+"/turns/turn-candidate:confirm", `{"candidate_id":"missing"}`, true, true), confirmCandidateSvc)
		if rec.Code != http.StatusUnprocessableEntity || assistantDecodeErrCode(t, rec) != "assistant_candidate_not_found" {
			t.Fatalf("candidate confirm status=%d body=%s", rec.Code, rec.Body.String())
		}

		commitServiceMissingSvc := newAssistantConversationService(store, nil)
		commitConv := commitServiceMissingSvc.createConversation("tenant-1", principal)
		commitTurn := assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-service", State: assistantStateConfirmed, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), PolicyVersion: capabilityPolicyVersionBaseline, CompositionVersion: capabilityPolicyVersionBaseline, MappingVersion: capabilityPolicyVersionBaseline, Candidates: []assistantCandidate{{CandidateID: "FLOWER-A", CandidateCode: "FLOWER-A", Name: "鲜花组织", OrgID: 10000000, IsActive: true}}, ResolvedCandidateID: "FLOWER-A"}, nil)
		if err := commitServiceMissingSvc.refreshTurnVersionTuple(context.Background(), "tenant-1", commitTurn); err != nil {
			t.Fatalf("refresh missing service turn err=%v", err)
		}
		commitServiceMissingSvc.mu.Lock()
		commitServiceMissingSvc.byID[commitConv.ConversationID].Turns = append(commitServiceMissingSvc.byID[commitConv.ConversationID].Turns, commitTurn)
		commitServiceMissingSvc.mu.Unlock()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		if _, err := assistantCommitTurnSyncForTest(commitServiceMissingSvc, context.Background(), "tenant-1", principal, commitConv.ConversationID, "turn-service"); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("service missing commit err=%v", err)
		}

		commitCandidateSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		commitConv = commitCandidateSvc.createConversation("tenant-1", principal)
		commitTurn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-missing-candidate", State: assistantStateConfirmed, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), PolicyVersion: capabilityPolicyVersionBaseline, CompositionVersion: capabilityPolicyVersionBaseline, MappingVersion: capabilityPolicyVersionBaseline}, nil)
		commitCandidateSvc.mu.Lock()
		commitCandidateSvc.byID[commitConv.ConversationID].Turns = append(commitCandidateSvc.byID[commitConv.ConversationID].Turns, commitTurn)
		commitCandidateSvc.mu.Unlock()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		if _, err := assistantCommitTurnSyncForTest(commitCandidateSvc, context.Background(), "tenant-1", principal, commitConv.ConversationID, "turn-missing-candidate"); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("candidate commit err=%v", err)
		}

		commitAuthzSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		commitConv = commitAuthzSvc.createConversation("tenant-1", principal)
		commitTurn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-authz", State: assistantStateConfirmed, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), PolicyVersion: capabilityPolicyVersionBaseline, CompositionVersion: capabilityPolicyVersionBaseline, MappingVersion: capabilityPolicyVersionBaseline, Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}, ResolvedCandidateID: "c1"}, nil)
		commitAuthzSvc.mu.Lock()
		commitAuthzSvc.byID[commitConv.ConversationID].Turns = append(commitAuthzSvc.byID[commitConv.ConversationID].Turns, commitTurn)
		commitAuthzSvc.mu.Unlock()
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: false, enforced: true}, nil }
		if _, err := assistantCommitTurnSyncForTest(commitAuthzSvc, context.Background(), "tenant-1", principal, commitConv.ConversationID, "turn-authz"); !errors.Is(err, errAssistantActionAuthzDenied) {
			t.Fatalf("authz commit err=%v", err)
		}
	})

	t.Run("persistence dead and direct branches", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		defer func() { assistantLoadAuthorizerFn = original }()

		createPGSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		createPGSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentPlanOnly: {ID: assistantIntentPlanOnly, CapabilityKey: "org.assistant_conversation.manage", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "low"}}}}
		if _, err := createPGSvc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "仅生成计划"); !errors.Is(err, errAssistantServiceMissing) {
			t.Fatalf("expected begin tx service missing, got %v", err)
		}

		confirmSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		confirmSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentCreateOrgUnit: spec}}
		conversation := &assistantConversation{TenantID: "tenant-1"}
		pendingSnapshot := assistantTestCreateOrgUnitProjectionSnapshot()
		pendingSnapshot.Projection.Readiness = "candidate_confirmation_required"
		pendingSnapshot.Projection.CandidateConfirmationRequirements = []string{"resolved_candidate"}
		turn := assistantTestAttachCreateOrgUnitProjection(&assistantTurn{State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}), Candidates: []assistantCandidate{{CandidateID: "c1"}, {CandidateID: "c2"}}, AmbiguityCount: 2, ResolvedCandidateID: "c1"}, pendingSnapshot)
		if _, err := confirmSvc.applyConfirmTurn(conversation, turn, principal, ""); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("expected ambiguity confirm required, got %v", err)
		}
	})

	t.Run("direct create confirm commit branches", func(t *testing.T) {
		original := assistantLoadAuthorizerFn
		assistantLoadAuthorizerFn = func() (authorizer, error) { return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil }
		defer func() { assistantLoadAuthorizerFn = original }()

		specMissingSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		specMissingSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentPlanOnly: {CapabilityKey: "org.assistant_conversation.manage"},
		}}
		conv := specMissingSvc.createConversation("tenant-1", principal)
		created, err := specMissingSvc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划")
		if err != nil {
			t.Fatalf("expected uncertain clarification turn, got %v", err)
		}
		last := created.Turns[len(created.Turns)-1]
		if last.Phase != assistantPhaseAwaitClarification {
			t.Fatalf("expected await_clarification, got %q", last.Phase)
		}

		riskDeniedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		riskDeniedSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentPlanOnly: {ID: assistantIntentPlanOnly, CapabilityKey: "org.assistant_conversation.manage", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "extreme"}},
		}}
		conv = riskDeniedSvc.createConversation("tenant-1", principal)
		if _, err := riskDeniedSvc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划"); err != nil {
			t.Fatalf("expected uncertain clarification without risk gate, got %v", err)
		}

		refreshErrSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		refreshErrSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentPlanOnly: {ID: assistantIntentPlanOnly, CapabilityKey: "org.assistant_conversation.manage", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "low"}},
		}}
		conv = refreshErrSvc.createConversation("tenant-1", principal)
		originalPlanHash := assistantPlanHashFn
		assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string { return "" }
		if _, err := refreshErrSvc.createTurn(context.Background(), "tenant-1", principal, conv.ConversationID, "仅生成计划"); !errors.Is(err, errAssistantPlanDeterminismViolation) {
			assistantPlanHashFn = originalPlanHash
			t.Fatalf("expected determinism violation from refresh, got %v", err)
		}
		assistantPlanHashFn = originalPlanHash

		now := time.Now().UTC()
		makeCreateTurnPGTx := func(actorID string) *assistFakeTx {
			tx := &assistFakeTx{}
			tx.queryRowFn = func(sql string, _ ...any) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM iam.assistant_conversations"):
					return &assistFakeRow{vals: []any{"conv_pg", "tenant_1", actorID, "tenant-admin", assistantStateValidated, assistantPhaseAwaitCandidateConfirm, now, now}}
				case strings.Contains(sql, "INSERT INTO iam.assistant_state_transitions"):
					return &assistFakeRow{vals: []any{nil}}
				default:
					return &assistFakeRow{err: pgx.ErrNoRows}
				}
			}
			return tx
		}

		pgSpecMissingSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		pgSpecMissingSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentPlanOnly: {CapabilityKey: "org.assistant_conversation.manage"},
		}}
		pgSpecMissingSvc.pool = assistFakeTxBeginner{tx: makeCreateTurnPGTx(principal.ID)}
		created, err = pgSpecMissingSvc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "仅生成计划")
		if err != nil {
			t.Fatalf("expected pg uncertain clarification turn, got %v", err)
		}
		if len(created.Turns) == 0 || created.Turns[len(created.Turns)-1].Phase != assistantPhaseAwaitClarification {
			t.Fatalf("expected pg await_clarification, got turns=%+v", created.Turns)
		}

		pgRiskDeniedSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		pgRiskDeniedSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentPlanOnly: {ID: assistantIntentPlanOnly, CapabilityKey: "org.assistant_conversation.manage", Security: assistantActionSecuritySpec{AuthObject: "org.setid_capability_config", AuthAction: "admin", RiskTier: "extreme"}},
		}}
		pgRiskDeniedSvc.pool = assistFakeTxBeginner{tx: makeCreateTurnPGTx(principal.ID)}
		if _, err := pgRiskDeniedSvc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "仅生成计划"); err != nil {
			t.Fatalf("expected pg uncertain clarification without risk gate, got %v", err)
		}

		pgRefreshErrSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		pgRefreshErrSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
			assistantIntentCreateOrgUnit: {
				ID:            assistantIntentCreateOrgUnit,
				Version:       "v1",
				PlanTitle:     "创建组织",
				PlanSummary:   "生成创建组织计划，待确认后提交",
				CapabilityKey: spec.CapabilityKey,
				Security:      spec.Security,
				Handler:       spec.Handler,
			},
		}}
		pgRefreshErrSvc.pool = assistFakeTxBeginner{tx: makeCreateTurnPGTx(principal.ID)}
		pgOriginalPlanHash := assistantPlanHashFn
		assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string { return "" }
		if _, err := pgRefreshErrSvc.createTurnPG(context.Background(), "tenant_1", principal, "conv_pg", "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantPlanDeterminismViolation) {
			assistantPlanHashFn = pgOriginalPlanHash
			t.Fatalf("expected pg determinism violation, got %v", err)
		}
		assistantPlanHashFn = pgOriginalPlanHash

		confirmSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		confirmSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentCreateOrgUnit: spec}}
		conversation := &assistantConversation{TenantID: "tenant-1"}
		turn := assistantTestAttachCreateOrgUnitProjection(&assistantTurn{State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, AmbiguityCount: 2}, nil)
		if _, err := confirmSvc.applyConfirmTurn(conversation, turn, principal, ""); !errors.Is(err, errAssistantConfirmationRequired) {
			t.Fatalf("expected confirm required, got %v", err)
		}
		turn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, AmbiguityCount: 2, Candidates: []assistantCandidate{{CandidateID: "c1"}}}, nil)
		if _, err := confirmSvc.applyConfirmTurn(conversation, turn, principal, "missing"); !errors.Is(err, errAssistantCandidateNotFound) {
			t.Fatalf("expected candidate not found, got %v", err)
		}
		flakyConfirm := &assistantFlakyActionRegistry{spec: assistantActionSpec{ID: assistantIntentCreateOrgUnit, Version: "v1", CapabilityKey: spec.CapabilityKey, Security: assistantActionSecuritySpec{AuthObject: spec.Security.AuthObject, AuthAction: spec.Security.AuthAction, RiskTier: spec.Security.RiskTier}, Handler: spec.Handler}}
		confirmSvc.actionRegistry = flakyConfirm
		turn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{State: assistantStateValidated, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}, ResolvedCandidateID: "c1", AmbiguityCount: 1}, nil)
		if _, err := confirmSvc.applyConfirmTurn(conversation, turn, principal, "c1"); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("expected refresh unsupported intent, got %v", err)
		}

		commitSvc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		commitSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{}}
		turn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{State: assistantStateConfirmed, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}}, nil)
		if _, err := commitSvc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant-1"); !errors.Is(err, errAssistantUnsupportedIntent) {
			t.Fatalf("expected commit unsupported intent, got %v", err)
		}
		commitSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentCreateOrgUnit: {ID: assistantIntentCreateOrgUnit, Version: "v1", CapabilityKey: spec.CapabilityKey, Security: assistantActionSecuritySpec{AuthObject: spec.Security.AuthObject, AuthAction: spec.Security.AuthAction, RiskTier: "extreme"}, Handler: spec.Handler}}}
		policyVersion, compositionVersion, mappingVersion := assistantTurnVersionSnapshot(spec.CapabilityKey)
		turn = assistantTestAttachCreateOrgUnitProjection(&assistantTurn{TurnID: "turn-risk", RequestID: "req-risk", TraceID: "trace-risk", State: assistantStateConfirmed, Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, Plan: assistantPlanSummary{CapabilityKey: spec.CapabilityKey}, PolicyVersion: policyVersion, CompositionVersion: compositionVersion, MappingVersion: mappingVersion, Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A", OrgID: 1}}, ResolvedCandidateID: "c1", DryRun: assistantDryRunResult{}}, nil)
		result, err := commitSvc.applyCommitTurn(context.Background(), conversation, turn, principal, "tenant-1")
		if !errors.Is(err, errAssistantActionRiskGateDenied) || result.Transition == nil {
			t.Fatalf("expected commit gate denial, result=%+v err=%v", result, err)
		}
	})
}
