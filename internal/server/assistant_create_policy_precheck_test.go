package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type assistantCreatePolicyStore struct {
	*orgUnitMemoryStore
	fieldConfigs    []orgUnitTenantFieldConfig
	fieldConfigsErr error
}

type assistantNoFieldConfigStore struct{ inner *orgUnitMemoryStore }

func (s assistantNoFieldConfigStore) ListNodesCurrent(ctx context.Context, tenantID string, asOfDate string) ([]OrgUnitNode, error) {
	return s.inner.ListNodesCurrent(ctx, tenantID, asOfDate)
}
func (s assistantNoFieldConfigStore) CreateNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgCode string, name string, parentID string, isBusinessUnit bool) (OrgUnitNode, error) {
	return s.inner.CreateNodeCurrent(ctx, tenantID, effectiveDate, orgCode, name, parentID, isBusinessUnit)
}
func (s assistantNoFieldConfigStore) RenameNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, newName string) error {
	return s.inner.RenameNodeCurrent(ctx, tenantID, effectiveDate, orgID, newName)
}
func (s assistantNoFieldConfigStore) MoveNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, newParentID string) error {
	return s.inner.MoveNodeCurrent(ctx, tenantID, effectiveDate, orgID, newParentID)
}
func (s assistantNoFieldConfigStore) DisableNodeCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string) error {
	return s.inner.DisableNodeCurrent(ctx, tenantID, effectiveDate, orgID)
}
func (s assistantNoFieldConfigStore) SetBusinessUnitCurrent(ctx context.Context, tenantID string, effectiveDate string, orgID string, isBusinessUnit bool, requestID string) error {
	return s.inner.SetBusinessUnitCurrent(ctx, tenantID, effectiveDate, orgID, isBusinessUnit, requestID)
}
func (s assistantNoFieldConfigStore) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	return s.inner.ResolveOrgID(ctx, tenantID, orgCode)
}
func (s assistantNoFieldConfigStore) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	return s.inner.ResolveOrgCode(ctx, tenantID, orgID)
}
func (s assistantNoFieldConfigStore) ResolveOrgCodes(ctx context.Context, tenantID string, orgIDs []int) (map[int]string, error) {
	return s.inner.ResolveOrgCodes(ctx, tenantID, orgIDs)
}
func (s assistantNoFieldConfigStore) ListChildren(ctx context.Context, tenantID string, parentID int, asOfDate string) ([]OrgUnitChild, error) {
	return s.inner.ListChildren(ctx, tenantID, parentID, asOfDate)
}
func (s assistantNoFieldConfigStore) GetNodeDetails(ctx context.Context, tenantID string, orgID int, asOfDate string) (OrgUnitNodeDetails, error) {
	return s.inner.GetNodeDetails(ctx, tenantID, orgID, asOfDate)
}
func (s assistantNoFieldConfigStore) SearchNode(ctx context.Context, tenantID string, query string, asOfDate string) (OrgUnitSearchResult, error) {
	return s.inner.SearchNode(ctx, tenantID, query, asOfDate)
}
func (s assistantNoFieldConfigStore) SearchNodeCandidates(ctx context.Context, tenantID string, query string, asOfDate string, limit int) ([]OrgUnitSearchCandidate, error) {
	return s.inner.SearchNodeCandidates(ctx, tenantID, query, asOfDate, limit)
}
func (s assistantNoFieldConfigStore) ListNodeVersions(ctx context.Context, tenantID string, orgID int) ([]OrgUnitNodeVersion, error) {
	return s.inner.ListNodeVersions(ctx, tenantID, orgID)
}
func (s assistantNoFieldConfigStore) MaxEffectiveDateOnOrBefore(ctx context.Context, tenantID string, asOfDate string) (string, bool, error) {
	return s.inner.MaxEffectiveDateOnOrBefore(ctx, tenantID, asOfDate)
}
func (s assistantNoFieldConfigStore) MinEffectiveDate(ctx context.Context, tenantID string) (string, bool, error) {
	return s.inner.MinEffectiveDate(ctx, tenantID)
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
		resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (setIDFieldDecision, error) {
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
	if len(turn.MissingFields) != 1 || turn.MissingFields[0] != "field_policy" {
		t.Fatalf("unexpected missing fields=%v", turn.MissingFields)
	}
}

func TestAssistantConfirmTurn_PrechecksOrgTypeFieldEnablementAfterCandidatePick(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy")
	previous := defaultSetIDStrategyRegistryStore
	defer func() { defaultSetIDStrategyRegistryStore = previous }()
	defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
		resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (setIDFieldDecision, error) {
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

	if _, err := svc.confirmTurn("tenant-1", principal, conversation.ConversationID, turn.TurnID, "FLOWER-A"); err != errAssistantClarificationRequired {
		t.Fatalf("expected clarification required, got=%v", err)
	}
	mutatedTurn := svc.byID[conversation.ConversationID].Turns[len(svc.byID[conversation.ConversationID].Turns)-1]
	mutatedTurn.Clarification = nil
	mutatedTurn.ErrorCode = ""
	mutatedTurn.RouteDecision.ClarificationRequired = false
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
	if mutated.Phase != assistantPhaseAwaitMissingFields && mutated.Phase != assistantPhaseFailed {
		t.Fatalf("expected await_missing_fields/failed, got=%q", mutated.Phase)
	}
}

func TestAssistantCreatePolicyPrecheck_HelperCoverage(t *testing.T) {
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

	t.Run("resolveCreateOrgUnitBusinessUnitID branches", func(t *testing.T) {
		if _, ok := (*assistantConversationService)(nil).resolveCreateOrgUnitBusinessUnitID(context.Background(), "t1", "FLOWER-A"); ok {
			t.Fatal("expected nil service false")
		}
		if _, ok := (&assistantConversationService{}).resolveCreateOrgUnitBusinessUnitID(context.Background(), "t1", "FLOWER-A"); ok {
			t.Fatal("expected nil store false")
		}
		errStore := assistantOrgStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore(), resolveErr: errors.New("boom")}
		if _, ok := (&assistantConversationService{orgStore: errStore}).resolveCreateOrgUnitBusinessUnitID(context.Background(), "t1", "FLOWER-A"); ok {
			t.Fatal("expected resolve error false")
		}
		zeroStore := assistantOrgStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore(), resolveOrgID: 0}
		if _, ok := (&assistantConversationService{orgStore: zeroStore}).resolveCreateOrgUnitBusinessUnitID(context.Background(), "t1", "FLOWER-A"); ok {
			t.Fatal("expected zero org id false")
		}
		goodStore := assistantOrgStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore(), resolveOrgID: 10000001}
		if got, ok := (&assistantConversationService{orgStore: goodStore}).resolveCreateOrgUnitBusinessUnitID(context.Background(), "t1", "FLOWER-A"); !ok || got != "10000001" {
			t.Fatalf("unexpected business unit id got=%q ok=%v", got, ok)
		}
	})

	t.Run("assistantCreateFieldDecisionResolvedValue branches", func(t *testing.T) {
		if got := assistantCreateFieldDecisionResolvedValue(setIDFieldDecision{ResolvedDefaultVal: "10"}, "20"); got != "20" {
			t.Fatalf("expected provided value, got=%q", got)
		}
		if got := assistantCreateFieldDecisionResolvedValue(setIDFieldDecision{ResolvedDefaultVal: "10"}, ""); got != "10" {
			t.Fatalf("expected default value, got=%q", got)
		}
		if got := assistantCreateFieldDecisionResolvedValue(setIDFieldDecision{DefaultRuleRef: `next_org_code("G", 4)`}, ""); got != "__rule__" {
			t.Fatalf("expected rule sentinel, got=%q", got)
		}
		if got := assistantCreateFieldDecisionResolvedValue(setIDFieldDecision{}, ""); got != "" {
			t.Fatalf("expected empty, got=%q", got)
		}
	})

	t.Run("isCreateOrgTypeFieldEnabled branches", func(t *testing.T) {
		if (&assistantConversationService{}).isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected nil store false")
		}
		noReader := &assistantConversationService{orgStore: assistantNoFieldConfigStore{inner: newOrgUnitMemoryStore()}}
		if noReader.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected no reader false")
		}
		errReader := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigsErr: errors.New("boom")}}
		if errReader.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected reader error false")
		}
		wrongKey := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigs: []orgUnitTenantFieldConfig{{FieldKey: "x_other", ValueType: "text", DataSourceType: "DICT"}}}}
		if wrongKey.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected wrong key false")
		}
		wrongType := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigs: []orgUnitTenantFieldConfig{{FieldKey: orgUnitCreateFieldOrgType, ValueType: "int", DataSourceType: "DICT"}}}}
		if wrongType.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected wrong value type false")
		}
		wrongSource := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigs: []orgUnitTenantFieldConfig{{FieldKey: orgUnitCreateFieldOrgType, ValueType: "text", DataSourceType: "PLAIN"}}}}
		if wrongSource.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected wrong source false")
		}
		badDictConfig := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigs: []orgUnitTenantFieldConfig{{FieldKey: orgUnitCreateFieldOrgType, ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":1}`)}}}}
		if badDictConfig.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected bad dict config false")
		}
		good := &assistantConversationService{orgStore: assistantCreatePolicyStore{orgUnitMemoryStore: newOrgUnitMemoryStore(), fieldConfigs: []orgUnitTenantFieldConfig{{FieldKey: orgUnitCreateFieldOrgType, ValueType: "text", DataSourceType: "DICT", DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`)}}}}
		if !good.isCreateOrgTypeFieldEnabled(context.Background(), "t1", "2026-01-01") {
			t.Fatal("expected enabled true")
		}
	})

	t.Run("enrichCreateOrgUnitDryRunWithPolicy early returns and success", func(t *testing.T) {
		dry := assistantDryRunResult{Explain: "keep"}
		if got := (*assistantConversationService)(nil).enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected unchanged explain, got=%q", got.Explain)
		}
		svc := &assistantConversationService{orgStore: assistantOrgStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore(), resolveErr: errors.New("boom")}}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentPlanOnly}, nil, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected other action unchanged, got=%q", got.Explain)
		}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, nil, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected invalid intent unchanged, got=%q", got.Explain)
		}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部", EffectiveDate: "2026-01-01"}, nil, "", dry); got.Explain != "keep" {
			t.Fatalf("expected missing candidate unchanged, got=%q", got.Explain)
		}
		if got := svc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1"}}, "c1", dry); got.Explain != "keep" {
			t.Fatalf("expected resolve failure unchanged, got=%q", got.Explain)
		}

		previous := defaultSetIDStrategyRegistryStore
		defer func() { defaultSetIDStrategyRegistryStore = previous }()
		defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
			resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (setIDFieldDecision, error) {
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
		result := goodSvc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}, "c1", assistantDryRunResult{})
		if len(result.ValidationErrors) != 0 || result.Explain != "计划已生成，等待确认后可提交" {
			t.Fatalf("expected success dry run, got=%+v", result)
		}

		defaultSetIDStrategyRegistryStore = setIDStrategyRegistryStoreStub{
			resolveFieldDecisionFn: func(_ context.Context, _ string, _ string, fieldKey string, _ string, _ string) (setIDFieldDecision, error) {
				switch fieldKey {
				case orgUnitCreateFieldOrgCode:
					return setIDFieldDecision{FieldKey: fieldKey, Required: false}, nil
				case orgUnitCreateFieldOrgType:
					return setIDFieldDecision{FieldKey: fieldKey, Required: true}, nil
				default:
					return setIDFieldDecision{}, nil
				}
			},
		}
		missingOrgType := goodSvc.enrichCreateOrgUnitDryRunWithPolicy(context.Background(), "t1", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}, []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}, "c1", assistantDryRunResult{})
		if !containsString(missingOrgType.ValidationErrors, "FIELD_REQUIRED_VALUE_MISSING") {
			t.Fatalf("expected org type missing default blocker, got=%v", missingOrgType.ValidationErrors)
		}
	})
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
