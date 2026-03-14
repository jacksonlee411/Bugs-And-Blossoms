package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func assistant243BusinessRouteDecision() assistantIntentRouteDecision {
	return assistantIntentRouteDecision{
		RouteKind:               assistantRouteKindBusinessAction,
		IntentID:                "org.orgunit_create",
		CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
		ConfidenceBand:          assistantRouteConfidenceHigh,
		RouteCatalogVersion:     "2026-03-11.v1",
		KnowledgeSnapshotDigest: "sha256:test",
		ResolverContractVersion: "resolver_contract_v1",
		DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
	}
}

func TestAssistantAPI243_CreateTurnHandlerErrorMappings(t *testing.T) {
	origRouteFn := assistantBuildIntentRouteDecisionFn
	origAuthzFn := assistantLoadAuthorizerFn
	defer func() {
		assistantBuildIntentRouteDecisionFn = origRouteFn
		assistantLoadAuthorizerFn = origAuthzFn
	}()
	assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime) (assistantIntentRouteDecision, error) {
		return assistant243BusinessRouteDecision(), nil
	}
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}

	cases := []struct {
		name         string
		spec         assistantActionSpec
		wantStatus   int
		wantErrorKey string
	}{
		{
			name: "action spec missing",
			spec: assistantActionSpec{
				ID:            "",
				Version:       "v1",
				CapabilityKey: "org.orgunit_create.field_policy",
				PlanTitle:     "create",
				PlanSummary:   "create",
				Security: assistantActionSecuritySpec{
					AuthObject: "org.setid_capability_config",
					AuthAction: "admin",
					RiskTier:   "high",
				},
				Handler: assistantActionHandlerSpec{CommitAdapterKey: "orgunit_create_v1"},
			},
			wantStatus:   http.StatusUnprocessableEntity,
			wantErrorKey: errAssistantActionSpecMissing.Error(),
		},
		{
			name: "action risk gate denied",
			spec: assistantActionSpec{
				ID:            assistantIntentCreateOrgUnit,
				Version:       "v1",
				CapabilityKey: "org.orgunit_create.field_policy",
				PlanTitle:     "create",
				PlanSummary:   "create",
				Security: assistantActionSecuritySpec{
					AuthObject: "org.setid_capability_config",
					AuthAction: "admin",
					RiskTier:   "extreme",
				},
				Handler: assistantActionHandlerSpec{CommitAdapterKey: "orgunit_create_v1"},
			},
			wantStatus:   http.StatusConflict,
			wantErrorKey: errAssistantActionRiskGateDenied.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newOrgUnitMemoryStore()
			if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
				t.Fatalf("create org err=%v", err)
			}
			svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
			svc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{
				assistantIntentCreateOrgUnit: tc.spec,
			}}
			conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})

			rec := httptest.NewRecorder()
			req := assistantReqWithContext(
				http.MethodPost,
				"/internal/assistant/conversations/"+conv.ConversationID+"/turns",
				`{"user_input":"在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"}`,
				true,
				true,
			)
			handleAssistantConversationTurnsAPI(rec, req, svc)
			if rec.Code != tc.wantStatus || assistantDecodeErrCode(t, rec) != tc.wantErrorKey {
				t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
			}
		})
	}
}

func TestAssistantAPI243_TurnActionHandlerErrorMappings(t *testing.T) {
	origAuthzFn := assistantLoadAuthorizerFn
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}
	defer func() { assistantLoadAuthorizerFn = origAuthzFn }()

	t.Run("confirm clarification mappings", func(t *testing.T) {
		makeTurn := func(clarification *assistantClarificationDecision) *assistantTurn {
			now := time.Now().UTC()
			turn := &assistantTurn{
				TurnID:             "turn_confirm_map",
				UserInput:          "仅生成计划",
				State:              assistantStateValidated,
				RequestID:          "req_confirm_map",
				TraceID:            "trace_confirm_map",
				PolicyVersion:      capabilityPolicyVersionBaseline,
				CompositionVersion: capabilityPolicyVersionBaseline,
				MappingVersion:     capabilityPolicyVersionBaseline,
				Intent:             assistantIntentSpec{Action: assistantIntentPlanOnly, IntentSchemaVersion: assistantIntentSchemaVersionV1},
				Plan:               assistantBuildPlan(assistantIntentSpec{Action: assistantIntentPlanOnly}),
				CreatedAt:          now,
				UpdatedAt:          now,
				Clarification:      clarification,
			}
			turn.Plan.SkillManifestDigest = "skill_digest"
			assistantRefreshTurnDerivedFields(turn)
			return turn
		}

		cases := []struct {
			name     string
			clar     *assistantClarificationDecision
			wantCode string
		}{
			{
				name:     "rounds exhausted",
				clar:     &assistantClarificationDecision{Status: assistantClarificationStatusExhausted, ClarificationKind: assistantClarificationKindMissingSlots},
				wantCode: errAssistantClarificationRoundsExhausted.Error(),
			},
			{
				name:     "manual hint required",
				clar:     &assistantClarificationDecision{Status: assistantClarificationStatusAborted, ClarificationKind: assistantClarificationKindMissingSlots},
				wantCode: errAssistantManualHintRequired.Error(),
			},
			{
				name: "clarification runtime invalid",
				clar: &assistantClarificationDecision{
					Status:            assistantClarificationStatusOpen,
					ClarificationKind: assistantClarificationKindMissingSlots,
					AwaitPhase:        assistantPhaseAwaitMissingFields,
					CurrentRound:      1,
					MaxRounds:         2,
				},
				wantCode: errAssistantClarificationRuntimeInvalid.Error(),
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
				conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
				turn := makeTurn(tc.clar)
				svc.mu.Lock()
				svc.byID[conv.ConversationID].Turns = []*assistantTurn{turn}
				svc.mu.Unlock()

				rec := httptest.NewRecorder()
				req := assistantReqWithContext(http.MethodPost, "/internal/assistant/conversations/"+conv.ConversationID+"/turns/"+turn.TurnID+":confirm", `{}`, true, true)
				handleAssistantTurnActionAPI(rec, req, svc)
				if rec.Code != http.StatusConflict || assistantDecodeErrCode(t, rec) != tc.wantCode {
					t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
				}
			})
		}
	})

	t.Run("commit clarification and route mappings", func(t *testing.T) {
		baseReq := func(turnID string) *http.Request {
			req := httptest.NewRequest(http.MethodPost, "http://localhost/internal/assistant/conversations/conv_1/turns/"+turnID+":commit", strings.NewReader(`{}`))
			ctx := withTenant(req.Context(), Tenant{ID: "tenant_1"})
			ctx = withPrincipal(ctx, Principal{ID: "actor_1", RoleSlug: "tenant-admin"})
			return req.WithContext(ctx)
		}

		cases := []struct {
			name     string
			err      error
			wantCode string
			wantHTTP int
		}{
			{
				name:     "clarification required",
				err:      errAssistantClarificationRequired,
				wantCode: errAssistantClarificationRequired.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "clarification rounds exhausted",
				err:      errAssistantClarificationRoundsExhausted,
				wantCode: errAssistantClarificationRoundsExhausted.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "manual hint required",
				err:      errAssistantManualHintRequired,
				wantCode: errAssistantManualHintRequired.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "clarification runtime invalid",
				err:      errAssistantClarificationRuntimeInvalid,
				wantCode: errAssistantClarificationRuntimeInvalid.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "route non business blocked",
				err:      errAssistantRouteNonBusinessBlocked,
				wantCode: errAssistantRouteNonBusinessBlocked.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "route clarification required",
				err:      errAssistantRouteClarificationRequired,
				wantCode: errAssistantRouteClarificationRequired.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "route decision missing",
				err:      errAssistantRouteDecisionMissing,
				wantCode: errAssistantRouteDecisionMissing.Error(),
				wantHTTP: http.StatusConflict,
			},
			{
				name:     "route runtime invalid",
				err:      errAssistantRouteRuntimeInvalid,
				wantCode: errAssistantRouteRuntimeInvalid.Error(),
				wantHTTP: http.StatusUnprocessableEntity,
			},
			{
				name:     "route action conflict",
				err:      errAssistantRouteActionConflict,
				wantCode: errAssistantRouteActionConflict.Error(),
				wantHTTP: http.StatusUnprocessableEntity,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
				svc.pool = assistFakeTxBeginner{err: tc.err}

				rec := httptest.NewRecorder()
				handleAssistantTurnActionAPI(rec, baseReq("turn_1"), svc)
				if rec.Code != tc.wantHTTP || assistantDecodeErrCode(t, rec) != tc.wantCode {
					t.Fatalf("status=%d code=%s body=%s", rec.Code, assistantDecodeErrCode(t, rec), rec.Body.String())
				}
			})
		}
	})
}

func TestAssistantAPI243_CreateTurnAndHelperBranches(t *testing.T) {
	origRouteFn := assistantBuildIntentRouteDecisionFn
	origResumeFn := assistantResumeFromClarificationFn
	origPlanHashFn := assistantPlanHashFn
	origAuthzFn := assistantLoadAuthorizerFn
	origCapability := capabilityDefinitionByKey
	defer func() {
		assistantBuildIntentRouteDecisionFn = origRouteFn
		assistantResumeFromClarificationFn = origResumeFn
		assistantPlanHashFn = origPlanHashFn
		assistantLoadAuthorizerFn = origAuthzFn
		capabilityDefinitionByKey = origCapability
	}()
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}

	t.Run("resume action fallback keeps business action", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		runtime, err := assistantLoadKnowledgeRuntimeFn()
		if err != nil {
			t.Fatalf("load runtime err=%v", err)
		}
		svc.knowledgeRuntime = runtime
		conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		pending := &assistantTurn{
			TurnID:        "turn_pending",
			State:         assistantStateValidated,
			RequestID:     "req_pending",
			TraceID:       "trace_pending",
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "待补全", EffectiveDate: "2026-01-01"},
			Plan:          assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen},
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}
		pending.Plan.SkillManifestDigest = "skill_digest"
		assistantRefreshTurnDerivedFields(pending)
		svc.mu.Lock()
		svc.byID[conv.ConversationID].Turns = []*assistantTurn{pending}
		svc.mu.Unlock()

		assistantResumeFromClarificationFn = func(_ *assistantTurn, _ string, _ assistantIntentSpec) assistantClarificationResumeResult {
			return assistantClarificationResumeResult{Intent: assistantIntentSpec{
				Action:        assistantIntentCreateOrgUnit,
				ParentRefText: "鲜花组织",
				EntityName:    "运营部",
				EffectiveDate: "2026-01-01",
			}}
		}
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime) (assistantIntentRouteDecision, error) {
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

		got, err := svc.createTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, conv.ConversationID, "继续执行")
		if err != nil {
			t.Fatalf("create turn err=%v", err)
		}
		last := latestTurn(got)
		if last == nil || strings.TrimSpace(last.Intent.Action) != assistantIntentCreateOrgUnit {
			t.Fatalf("expected action restore, turn=%+v", last)
		}
	})

	t.Run("capability unregistered maps to plan boundary violation", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime) (assistantIntentRouteDecision, error) {
			return assistant243BusinessRouteDecision(), nil
		}
		capabilityDefinitionByKey = map[string]capabilityDefinition{}
		if _, err := svc.createTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantPlanBoundaryViolation) {
			t.Fatalf("expected boundary violation err=%v", err)
		}
		capabilityDefinitionByKey = origCapability
	})

	t.Run("refresh version tuple error", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("create node err=%v", err)
		}
		svc := newAssistantConversationService(store, assistantWriteServiceStub{store: store})
		conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime) (assistantIntentRouteDecision, error) {
			return assistant243BusinessRouteDecision(), nil
		}
		assistantPlanHashFn = func(assistantIntentSpec, assistantPlanSummary, assistantDryRunResult) string { return "" }
		if _, err := svc.createTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantPlanDeterminismViolation) {
			t.Fatalf("expected refresh err=%v", err)
		}
		assistantPlanHashFn = origPlanHashFn
	})

	t.Run("route audit mismatch after snapshot apply", func(t *testing.T) {
		store := newOrgUnitMemoryStore()
		if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
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
		assistantBuildIntentRouteDecisionFn = func(string, assistantResolveIntentResult, assistantIntentSpec, *assistantKnowledgeRuntime) (assistantIntentRouteDecision, error) {
			return assistant243BusinessRouteDecision(), nil
		}
		conv := svc.createConversation("tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		if _, err := svc.createTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, conv.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01"); !errors.Is(err, errAssistantPlanContractVersionMismatch) {
			t.Fatalf("expected route audit mismatch err=%v", err)
		}
	})

	t.Run("helper branches for clarification and pending turn", func(t *testing.T) {
		if !assistantTurnRequiresIntentClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen},
		}) {
			t.Fatal("open clarification should require clarification")
		}

		if got := assistantLatestPendingTurn(&assistantConversation{
			Turns: []*assistantTurn{{TurnID: "turn_draft", State: assistantStateDraft}},
		}); got != nil {
			t.Fatalf("draft turn should not be pending, got=%+v", got)
		}

		missing := &assistantTurn{
			TurnID:    "turn_missing",
			State:     assistantStateValidated,
			Intent:    assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			DryRun:    assistantDryRunResult{ValidationErrors: []string{"missing_effective_date"}},
			Plan:      assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			RequestID: "req_missing",
			TraceID:   "trace_missing",
		}
		if got := assistantLatestPendingTurn(&assistantConversation{Turns: []*assistantTurn{missing}}); got == nil || got.TurnID != missing.TurnID {
			t.Fatalf("expected missing-fields turn pending, got=%+v", got)
		}

		clean := &assistantTurn{
			TurnID:    "turn_clean",
			State:     assistantStateValidated,
			Intent:    assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			DryRun:    assistantDryRunResult{},
			Plan:      assistantBuildPlan(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}),
			RequestID: "req_clean",
			TraceID:   "trace_clean",
		}
		if got := assistantLatestPendingTurn(&assistantConversation{Turns: []*assistantTurn{clean}}); got != nil {
			t.Fatalf("clean validated turn should not be pending, got=%+v", got)
		}
	})
}
