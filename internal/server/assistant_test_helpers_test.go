package server

import (
	"context"
	"strings"
)

type assistantCreatePolicyRegistryStore struct {
	fallback setIDStrategyRegistryStore
}

var assistantCreatePolicyRegistryBaseStore setIDStrategyRegistryStore

func (s assistantCreatePolicyRegistryStore) upsert(ctx context.Context, tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error) {
	return s.fallback.upsert(ctx, tenantID, item)
}

func (s assistantCreatePolicyRegistryStore) disable(ctx context.Context, tenantID string, req setIDStrategyRegistryDisableRequest) (setIDStrategyRegistryItem, bool, error) {
	return s.fallback.disable(ctx, tenantID, req)
}

func (s assistantCreatePolicyRegistryStore) list(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error) {
	return s.fallback.list(ctx, tenantID, capabilityKey, fieldKey, asOf)
}

func (s assistantCreatePolicyRegistryStore) resolveFieldDecision(
	ctx context.Context,
	tenantID string,
	capabilityKey string,
	fieldKey string,
	resolvedSetID string,
	businessUnitNodeKey string,
	asOf string,
) (setIDFieldDecision, error) {
	if strings.TrimSpace(capabilityKey) == orgUnitCreateFieldPolicyCapabilityKey {
		switch strings.TrimSpace(fieldKey) {
		case orgUnitCreateFieldOrgCode:
			return setIDFieldDecision{
				CapabilityKey:  capabilityKey,
				FieldKey:       fieldKey,
				Required:       true,
				Visible:        true,
				Maintainable:   false,
				DefaultRuleRef: `next_org_code("F", 8)`,
			}, nil
		case orgUnitCreateFieldOrgType:
			return setIDFieldDecision{
				CapabilityKey:      capabilityKey,
				FieldKey:           fieldKey,
				Required:           true,
				Visible:            true,
				Maintainable:       true,
				ResolvedDefaultVal: "10",
				AllowedValueCodes:  []string{"10"},
			}, nil
		}
	}
	if baselineCapabilityKey, ok := orgUnitBaselineCapabilityKeyForIntentCapability(capabilityKey); ok {
		switch strings.TrimSpace(baselineCapabilityKey) {
		case orgUnitWriteFieldPolicyCapabilityKey:
			switch strings.TrimSpace(fieldKey) {
			case "name", "parent_org_code", "status", "is_business_unit", "manager_pernr":
				return setIDFieldDecision{
					CapabilityKey: capabilityKey,
					FieldKey:      fieldKey,
					Visible:       true,
					Maintainable:  true,
				}, nil
			}
		}
	}
	return s.fallback.resolveFieldDecision(ctx, tenantID, capabilityKey, fieldKey, resolvedSetID, businessUnitNodeKey, asOf)
}

func init() {
	assistantCreatePolicyRegistryBaseStore = defaultSetIDStrategyRegistryStore
	assistantResetCreatePolicyRegistryStoreForTest()
}

func assistantResetCreatePolicyRegistryStoreForTest() {
	defaultSetIDStrategyRegistryStore = assistantCreatePolicyRegistryStore{
		fallback: assistantCreatePolicyRegistryBaseStore,
	}
}

const (
	assistantTestRouteCatalogVersion     = "2026-03-11.v1"
	assistantTestKnowledgeSnapshotDigest = "sha256:test"
	assistantTestReplyGuidanceVersion    = "2026-03-11.v1"
)

func assistantTestBusinessIntentID(actionID string) string {
	switch strings.TrimSpace(actionID) {
	case assistantIntentCreateOrgUnit:
		return "org.orgunit_create"
	case assistantIntentAddOrgUnitVersion:
		return "org.orgunit_add_version"
	case assistantIntentInsertOrgUnitVersion:
		return "org.orgunit_insert_version"
	case assistantIntentCorrectOrgUnit:
		return "org.orgunit_correct"
	case assistantIntentRenameOrgUnit:
		return "org.orgunit_rename"
	case assistantIntentMoveOrgUnit:
		return "org.orgunit_move"
	case assistantIntentDisableOrgUnit:
		return "org.orgunit_disable"
	case assistantIntentEnableOrgUnit:
		return "org.orgunit_enable"
	default:
		return "action." + strings.TrimSpace(actionID)
	}
}

func assistantTestBusinessRouteDecision(actionID string) assistantIntentRouteDecision {
	actionID = strings.TrimSpace(actionID)
	return assistantIntentRouteDecision{
		RouteKind:               assistantRouteKindBusinessAction,
		IntentID:                assistantTestBusinessIntentID(actionID),
		CandidateActionIDs:      []string{actionID},
		ConfidenceBand:          assistantRouteConfidenceHigh,
		RouteCatalogVersion:     assistantTestRouteCatalogVersion,
		KnowledgeSnapshotDigest: assistantTestKnowledgeSnapshotDigest,
		ResolverContractVersion: assistantResolverContractVersionV1,
		DecisionSource:          assistantRouteDecisionSourceSemanticModelV1,
	}
}

func assistantTestAttachBusinessRoute(turn *assistantTurn) *assistantTurn {
	if turn == nil {
		return nil
	}
	actionID := strings.TrimSpace(turn.Intent.Action)
	if actionID == "" || actionID == assistantIntentPlanOnly {
		return turn
	}
	if !assistantIntentRouteDecisionPresent(turn.RouteDecision) {
		turn.RouteDecision = assistantTestBusinessRouteDecision(actionID)
	}
	turn.Intent = assistantProjectIntentRouteDecision(turn.Intent, turn.RouteDecision)
	if strings.TrimSpace(turn.Plan.RouteCatalogVersion) == "" {
		turn.Plan.RouteCatalogVersion = strings.TrimSpace(turn.RouteDecision.RouteCatalogVersion)
	}
	if strings.TrimSpace(turn.Plan.KnowledgeSnapshotDigest) == "" {
		turn.Plan.KnowledgeSnapshotDigest = strings.TrimSpace(turn.RouteDecision.KnowledgeSnapshotDigest)
	}
	if strings.TrimSpace(turn.Plan.ResolverContractVersion) == "" {
		turn.Plan.ResolverContractVersion = strings.TrimSpace(turn.RouteDecision.ResolverContractVersion)
	}
	if strings.TrimSpace(turn.Plan.ContextTemplateVersion) == "" {
		turn.Plan.ContextTemplateVersion = assistantContextTemplateVersionV1
	}
	if strings.TrimSpace(turn.Plan.ReplyGuidanceVersion) == "" {
		turn.Plan.ReplyGuidanceVersion = assistantTestReplyGuidanceVersion
	}
	assistantRefreshTurnDerivedFields(turn)
	return turn
}

func assistantTestAttachCreateOrgUnitProjection(
	turn *assistantTurn,
	snapshot *assistantCreateOrgUnitProjectionSnapshot,
) *assistantTurn {
	if turn == nil {
		return nil
	}
	if strings.TrimSpace(turn.Intent.Action) != assistantIntentCreateOrgUnit {
		return assistantTestAttachBusinessRoute(turn)
	}
	if strings.TrimSpace(turn.Intent.IntentSchemaVersion) == "" {
		turn.Intent.IntentSchemaVersion = assistantIntentSchemaVersionV1
	}
	if strings.TrimSpace(turn.Intent.ContextHash) == "" {
		turn.Intent.ContextHash = "ctx_hash"
	}
	if strings.TrimSpace(turn.Intent.IntentHash) == "" {
		turn.Intent.IntentHash = "intent_hash"
	}
	if strings.TrimSpace(turn.Plan.CompilerContractVersion) == "" {
		turn.Plan.CompilerContractVersion = assistantCompilerContractVersionV1
	}
	if strings.TrimSpace(turn.Plan.CapabilityMapVersion) == "" {
		turn.Plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
	}
	if strings.TrimSpace(turn.Plan.SkillManifestDigest) == "" {
		turn.Plan.SkillManifestDigest = assistantSkillManifestDigest([]string{"org.orgunit_create"})
	}
	if strings.TrimSpace(turn.DryRun.PlanHash) == "" {
		turn.DryRun.PlanHash = "plan_hash"
	}
	if snapshot == nil {
		snapshot = assistantTestCreateOrgUnitProjectionSnapshot()
	}
	turn.DryRun.CreateOrgUnitProjection = assistantCloneCreateOrgUnitProjectionSnapshot(snapshot)
	return assistantTestAttachBusinessRoute(turn)
}

func assistantTestAttachOrgUnitVersionProjection(
	turn *assistantTurn,
	snapshot *assistantOrgUnitVersionProjectionSnapshot,
) *assistantTurn {
	if turn == nil {
		return nil
	}
	action := strings.TrimSpace(turn.Intent.Action)
	if action != assistantIntentAddOrgUnitVersion && action != assistantIntentInsertOrgUnitVersion {
		return assistantTestAttachBusinessRoute(turn)
	}
	if strings.TrimSpace(turn.Intent.IntentSchemaVersion) == "" {
		turn.Intent.IntentSchemaVersion = assistantIntentSchemaVersionV1
	}
	if strings.TrimSpace(turn.Intent.ContextHash) == "" {
		turn.Intent.ContextHash = "ctx_hash"
	}
	if strings.TrimSpace(turn.Intent.IntentHash) == "" {
		turn.Intent.IntentHash = "intent_hash"
	}
	if strings.TrimSpace(turn.Plan.CompilerContractVersion) == "" {
		turn.Plan.CompilerContractVersion = assistantCompilerContractVersionV1
	}
	if strings.TrimSpace(turn.Plan.CapabilityMapVersion) == "" {
		turn.Plan.CapabilityMapVersion = assistantCapabilityMapVersionV1
	}
	if strings.TrimSpace(turn.Plan.SkillManifestDigest) == "" {
		turn.Plan.SkillManifestDigest = assistantSkillManifestDigest([]string{"org.orgunit_add_version"})
	}
	if strings.TrimSpace(turn.DryRun.PlanHash) == "" {
		turn.DryRun.PlanHash = "plan_hash"
	}
	if snapshot == nil {
		snapshot = assistantTestOrgUnitVersionProjectionSnapshot(action)
	}
	turn.DryRun.OrgUnitVersionProjection = assistantCloneOrgUnitVersionProjectionSnapshot(snapshot)
	return assistantTestAttachBusinessRoute(turn)
}

func assistantTestStaticSemanticGateway(payload string) *assistantModelGateway {
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
				return []byte(payload), nil
			}),
		},
	}
}
