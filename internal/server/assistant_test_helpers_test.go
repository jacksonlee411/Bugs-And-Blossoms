package server

import "strings"

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
