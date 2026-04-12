package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type assistantCreatePolicyStore struct {
	*orgUnitMemoryStore
	fieldConfigs    []orgUnitTenantFieldConfig
	fieldConfigsErr error
}

func (s assistantCreatePolicyStore) ListEnabledTenantFieldConfigsAsOf(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
	if s.fieldConfigsErr != nil {
		return nil, s.fieldConfigsErr
	}
	return append([]orgUnitTenantFieldConfig(nil), s.fieldConfigs...), nil
}

func newAssistantFallbackGateway() *assistantModelGateway {
	return &assistantModelGateway{
		config: assistantModelConfig{
			ProviderRouting: assistantProviderRouting{Strategy: "priority_failover", FallbackEnabled: true},
			Providers: []assistantModelProviderConfig{{
				Name:      "openai",
				Enabled:   true,
				Model:     "gpt-5-codex",
				Endpoint:  "https://api.openai.com/v1",
				TimeoutMS: 1000,
				Retries:   0,
				Priority:  1,
				KeyRef:    "OPENAI_API_KEY",
			}},
		},
		adapters: map[string]assistantProviderAdapter{
			"openai": assistantAdapterFunc(func(context.Context, string, assistantModelProviderConfig) ([]byte, error) {
				return []byte(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01","user_visible_reply":"已生成草案，请确认。","readiness":"ready_for_confirm"}`), nil
			}),
		},
	}
}

func TestAssistantCreateTurn_PrechecksMissingCreatePolicyDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	previous := defaultSetIDStrategyRegistryStore
	defer func() { defaultSetIDStrategyRegistryStore = previous }()
	defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
		resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string, _ string) (setIDFieldDecision, error) {
			switch fieldKey {
			case orgUnitCreateFieldOrgCode:
				return setIDFieldDecision{FieldKey: fieldKey, Required: true, Maintainable: true}, nil
			case orgUnitCreateFieldOrgType:
				return setIDFieldDecision{FieldKey: fieldKey, Required: false, Maintainable: true}, nil
			default:
				return setIDFieldDecision{}, nil
			}
		},
	}

	store := assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatalf("seed parent err=%v", err)
	}
	svc := &assistantConversationService{
		orgStore:     store,
		writeSvc:     assistantWriteServiceStub{store: store.orgUnitMemoryStore},
		modelGateway: newAssistantFallbackGateway(),
		byID:         make(map[string]*assistantConversation),
		byActorID:    make(map[string][]string),
	}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	conversation := svc.createConversation("tenant-1", principal)

	created, err := svc.createTurn(context.Background(), "tenant-1", principal, conversation.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("createTurn err=%v", err)
	}
	turn := created.Turns[len(created.Turns)-1]
	if !assistantTurnHasValidationCode(turn, "FIELD_REQUIRED_VALUE_MISSING") {
		t.Fatalf("expected FIELD_REQUIRED_VALUE_MISSING, got=%v", turn.DryRun.ValidationErrors)
	}
	if turn.Phase != assistantPhaseAwaitMissingFields {
		t.Fatalf("expected await_missing_fields, got=%q", turn.Phase)
	}
	if len(turn.MissingFields) != 1 || turn.MissingFields[0] != "org_code" {
		t.Fatalf("unexpected missing fields=%v", turn.MissingFields)
	}
	if turn.DryRun.CreateOrgUnitProjection == nil {
		t.Fatal("expected create projection snapshot")
	}
	if got := turn.DryRun.CreateOrgUnitProjection.Projection.Readiness; got != "missing_fields" {
		t.Fatalf("readiness=%q", got)
	}
}

func TestAssistantConfirmTurn_PrechecksOrgTypeFieldEnablementAfterCandidatePick(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	previous := defaultSetIDStrategyRegistryStore
	defer func() { defaultSetIDStrategyRegistryStore = previous }()
	defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
		resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string, _ string) (setIDFieldDecision, error) {
			switch fieldKey {
			case orgUnitCreateFieldOrgCode:
				return setIDFieldDecision{FieldKey: fieldKey, Required: true, Maintainable: false, DefaultRuleRef: `next_org_code("G", 4)`}, nil
			case orgUnitCreateFieldOrgType:
				return setIDFieldDecision{FieldKey: fieldKey, Required: true, Maintainable: true, ResolvedDefaultVal: "10"}, nil
			default:
				return setIDFieldDecision{}, nil
			}
		},
	}

	store := assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
		t.Fatalf("seed parent A err=%v", err)
	}
	if _, err := store.CreateNodeCurrent(context.Background(), "tenant-1", "2026-01-01", "FLOWER-B", "鲜花组织", "", true); err != nil {
		t.Fatalf("seed parent B err=%v", err)
	}
	svc := &assistantConversationService{
		orgStore:     store,
		writeSvc:     assistantWriteServiceStub{store: store.orgUnitMemoryStore},
		modelGateway: newAssistantFallbackGateway(),
		byID:         make(map[string]*assistantConversation),
		byActorID:    make(map[string][]string),
	}
	principal := Principal{ID: "actor-1", RoleSlug: "tenant-admin"}
	conversation := svc.createConversation("tenant-1", principal)

	created, err := svc.createTurn(context.Background(), "tenant-1", principal, conversation.ConversationID, "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026-01-01")
	if err != nil {
		t.Fatalf("createTurn err=%v", err)
	}
	turn := created.Turns[len(created.Turns)-1]
	if !assistantTurnHasValidationCode(turn, "candidate_confirmation_required") {
		t.Fatalf("expected candidate_confirmation_required, got=%v", turn.DryRun.ValidationErrors)
	}
	if turn.DryRun.CreateOrgUnitProjection == nil {
		t.Fatal("expected create projection snapshot before confirm")
	}
	if got := turn.DryRun.CreateOrgUnitProjection.Projection.Readiness; got != "candidate_confirmation_required" {
		t.Fatalf("readiness=%q", got)
	}

	if _, err := svc.confirmTurn("tenant-1", principal, conversation.ConversationID, turn.TurnID, "FLOWER-A"); err != errAssistantConfirmationRequired {
		t.Fatalf("expected confirmation required, got=%v", err)
	}
	mutatedTurn := svc.byID[conversation.ConversationID].Turns[len(svc.byID[conversation.ConversationID].Turns)-1]
	mutatedTurn.Clarification = nil
	mutatedTurn.ErrorCode = ""
	assistantRefreshTurnDerivedFields(mutatedTurn)
	if _, err := svc.confirmTurn("tenant-1", principal, conversation.ConversationID, turn.TurnID, "FLOWER-A"); err != errAssistantConfirmationRequired {
		t.Fatalf("expected confirmation required after clarification resolved, got=%v", err)
	}

	mutated := svc.byID[conversation.ConversationID].Turns[len(svc.byID[conversation.ConversationID].Turns)-1]
	if mutated.State != assistantStateValidated {
		t.Fatalf("expected validated state, got=%q", mutated.State)
	}
	if !assistantTurnHasValidationCode(mutated, "PATCH_FIELD_NOT_ALLOWED") {
		t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got=%v", mutated.DryRun.ValidationErrors)
	}
	if mutated.DryRun.CreateOrgUnitProjection == nil {
		t.Fatal("expected projection snapshot after candidate pick")
	}
	if got := mutated.DryRun.CreateOrgUnitProjection.Projection.RejectionReasons; len(got) != 1 || got[0] != "PATCH_FIELD_NOT_ALLOWED" {
		t.Fatalf("unexpected rejection reasons=%v", got)
	}
}

func TestAssistantCreatePolicyPrecheck_ProjectionSnapshotCoverage(t *testing.T) {
	t.Run("assistantResolvedCandidateCode branches", func(t *testing.T) {
		if got := assistantResolvedCandidateCode(nil, ""); got != "" {
			t.Fatalf("expected empty, got=%q", got)
		}
		candidates := []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2"}}
		if got := assistantResolvedCandidateCode(candidates, "c1"); got != "FLOWER-A" {
			t.Fatalf("expected code, got=%q", got)
		}
		if got := assistantResolvedCandidateCode(candidates, "c2"); got != "c2" {
			t.Fatalf("expected id fallback, got=%q", got)
		}
		if got := assistantResolvedCandidateCode(candidates, "c9"); got != "c9" {
			t.Fatalf("expected unresolved fallback, got=%q", got)
		}
	})

	t.Run("assistantCreateOrgUnitPolicyParentOrgCode and enrich short-circuit", func(t *testing.T) {
		candidates := []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2", CandidateCode: "FLOWER-B"}}
		if code, ok := assistantCreateOrgUnitPolicyParentOrgCode(assistantIntentSpec{}, nil, "", nil); code != "" || ok {
			t.Fatalf("expected empty parent branch, code=%q ok=%v", code, ok)
		}
		if code, ok := assistantCreateOrgUnitPolicyParentOrgCode(assistantIntentSpec{ParentRefText: "鲜花组织"}, candidates[:1], "", nil); code != "FLOWER-A" || !ok {
			t.Fatalf("expected single candidate parent, code=%q ok=%v", code, ok)
		}
		svc := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigsErr: errors.New("boom")}}
		dry := assistantDryRunResult{Explain: "keep"}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			ParentRefText: "鲜花组织",
			EntityName:    "运营部",
			EffectiveDate: "2026-01-01",
		}, candidates, "", dry); got.Explain != "keep" {
			t.Fatalf("expected ambiguous parent short-circuit, got=%+v", got)
		}
	})

	t.Run("enrichCreateOrgUnitDryRunWithPolicy early returns and success", func(t *testing.T) {
		dry := assistantDryRunResult{Explain: "keep"}
		if got := (*assistantConversationService)(nil).enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected unchanged explain, got=%q", got.Explain)
		}
		svc := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigsErr: errors.New("boom")}}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentPlanOnly}, nil, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected other action unchanged, got=%q", got.Explain)
		}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部"}, nil, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected missing effective date unchanged, got=%q", got.Explain)
		}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部", EffectiveDate: "2026-01-01"}, nil, "", dry); got.Explain != "keep" {
			t.Fatalf("expected unresolved candidate unchanged, got=%q", got.Explain)
		}

		previous := defaultSetIDStrategyRegistryStore
		defer func() { defaultSetIDStrategyRegistryStore = previous }()
		defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
			resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string, _ string) (setIDFieldDecision, error) {
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return setIDFieldDecision{FieldKey: fieldKey, Required: true, DefaultRuleRef: `next_org_code("G", 4)`}, nil
				case orgUnitCreateFieldOrgType:
					return setIDFieldDecision{FieldKey: fieldKey, Required: true, ResolvedDefaultVal: "10"}, nil
				default:
					return setIDFieldDecision{}, nil
				}
			},
		}
		goodStore := assistantCreatePolicyStore{
			orgUnitMemoryStore: newOrgUnitMemoryStore(),
			fieldConfigs: []orgUnitTenantFieldConfig{{
				FieldKey:         orgUnitCreateFieldOrgType,
				ValueType:        "text",
				DataSourceType:   "DICT",
				DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`),
			}},
		}
		if _, err := goodStore.CreateNodeCurrent(context.Background(), "t1", "2026-01-01", "FLOWER-A", "鲜花组织", "", true); err != nil {
			t.Fatalf("seed good store err=%v", err)
		}
		goodSvc := &assistantConversationService{orgStore: goodStore}
		adminCtx := withPrincipal(context.Background(), Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
		result := goodSvc.enrichCreateOrgUnitDryRunWithPolicy(adminCtx, "t1", assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			ParentRefText: "鲜花组织",
			EntityName:    "运营部",
			EffectiveDate: "2026-01-01",
		}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}, "c1", assistantDryRunResult{})
		if len(result.ValidationErrors) != 0 || result.Explain != "计划已生成，等待确认后可提交" {
			t.Fatalf("expected success dry run, got=%+v", result)
		}
		if result.CreateOrgUnitProjection == nil {
			t.Fatal("expected projection snapshot")
		}
		if result.CreateOrgUnitProjection.PolicyContextContractVersion != orgunitservices.CreateOrgUnitPolicyContextContractVersionV1 {
			t.Fatalf("policy context contract=%q", result.CreateOrgUnitProjection.PolicyContextContractVersion)
		}
		if result.CreateOrgUnitProjection.PrecheckProjectionContractVersion != orgunitservices.CreateOrgUnitPrecheckProjectionContractV1 {
			t.Fatalf("projection contract=%q", result.CreateOrgUnitProjection.PrecheckProjectionContractVersion)
		}
		if result.CreateOrgUnitProjection.PolicyContext.PolicyContextDigest == "" {
			t.Fatal("expected policy_context_digest")
		}
		if result.CreateOrgUnitProjection.Projection.ProjectionDigest == "" {
			t.Fatal("expected projection_digest")
		}

		missingDate := goodSvc.enrichCreateOrgUnitDryRunWithPolicy(adminCtx, "t1", assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			ParentRefText: "鲜花组织",
			EntityName:    "运营部",
		}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}, "c1", assistantDryRunResult{ValidationErrors: []string{"missing_effective_date"}})
		if missingDate.CreateOrgUnitProjection == nil {
			t.Fatal("expected projection snapshot for missing effective date")
		}
		if got := missingDate.CreateOrgUnitProjection.Projection.Readiness; got != "missing_fields" {
			t.Fatalf("missing date readiness=%q", got)
		}
		if !assistantSliceHas(missingDate.ValidationErrors, "missing_effective_date") {
			t.Fatalf("expected missing_effective_date validation, got=%v", missingDate.ValidationErrors)
		}

		confirmPending := goodSvc.enrichCreateOrgUnitDryRunWithPolicy(adminCtx, "t1", assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			ParentRefText: "鲜花组织",
			EntityName:    "运营部",
			EffectiveDate: "2026-01-01",
		}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}, {CandidateID: "c2", CandidateCode: "FLOWER-B"}}, "", assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}})
		if confirmPending.CreateOrgUnitProjection == nil {
			t.Fatal("expected projection snapshot for candidate confirmation")
		}
		if got := confirmPending.CreateOrgUnitProjection.Projection.Readiness; got != "candidate_confirmation_required" {
			t.Fatalf("readiness=%q", got)
		}
	})
}
