package server

import (
	"errors"
	"sort"
	"strings"
)

const (
	assistantRouteDecisionSourceKnowledgeRuntimeV1 = "knowledge_runtime_v1"
	assistantRouteConfidenceHigh                   = "high"
	assistantRouteConfidenceMedium                 = "medium"
	assistantRouteConfidenceLow                    = "low"

	assistantRouteReasonBusinessActionRegistered = "route_business_action_registered"
	assistantRouteReasonNonBusinessCatalogMatch  = "route_non_business_catalog_match"
	assistantRouteReasonUncertainNoMatch         = "route_uncertain_no_match"
	assistantRouteReasonLocalIntentUpgrade       = "route_local_intent_upgrade"
	assistantRouteReasonModelPlanOnly            = "route_model_plan_only"
	assistantRouteReasonConfidenceBelowMin       = "route_confidence_below_min"
	assistantRouteReasonActionUnregistered       = "route_action_unregistered"
	assistantRouteReasonNonBusinessBlocked       = "route_non_business_blocked"
	assistantRouteReasonClarificationRequired    = "route_clarification_required"
	assistantRouteReasonCatalogVersionMissing    = "route_catalog_version_missing"
	assistantRouteReasonDecisionMissing          = "route_decision_missing"
	assistantRouteReasonCandidateActionConflict  = "route_candidate_action_conflict"
)

type assistantIntentRouteDecision struct {
	RouteKind               string   `json:"route_kind,omitempty"`
	IntentID                string   `json:"intent_id,omitempty"`
	CandidateActionIDs      []string `json:"candidate_action_ids,omitempty"`
	ConfidenceBand          string   `json:"confidence_band,omitempty"`
	ClarificationRequired   bool     `json:"clarification_required,omitempty"`
	ReasonCodes             []string `json:"reason_codes,omitempty"`
	RouteCatalogVersion     string   `json:"route_catalog_version,omitempty"`
	KnowledgeSnapshotDigest string   `json:"knowledge_snapshot_digest,omitempty"`
	ResolverContractVersion string   `json:"resolver_contract_version,omitempty"`
	DecisionSource          string   `json:"decision_source,omitempty"`
}

var assistantBuildIntentRouteDecisionFn = assistantBuildIntentRouteDecision

func assistantIntentRouteDecisionPresent(decision assistantIntentRouteDecision) bool {
	return strings.TrimSpace(decision.RouteKind) != "" ||
		strings.TrimSpace(decision.IntentID) != "" ||
		len(decision.CandidateActionIDs) > 0 ||
		strings.TrimSpace(decision.ConfidenceBand) != "" ||
		decision.ClarificationRequired ||
		len(decision.ReasonCodes) > 0 ||
		strings.TrimSpace(decision.RouteCatalogVersion) != "" ||
		strings.TrimSpace(decision.KnowledgeSnapshotDigest) != "" ||
		strings.TrimSpace(decision.ResolverContractVersion) != "" ||
		strings.TrimSpace(decision.DecisionSource) != ""
}

func assistantValidRouteConfidenceBand(band string) bool {
	switch strings.TrimSpace(band) {
	case assistantRouteConfidenceHigh, assistantRouteConfidenceMedium, assistantRouteConfidenceLow:
		return true
	default:
		return false
	}
}

func assistantNormalizeRouteStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func assistantValidateIntentRouteDecision(decision assistantIntentRouteDecision) error {
	if !assistantIntentRouteDecisionPresent(decision) {
		return errAssistantRouteDecisionMissing
	}
	if !assistantValidRouteKind(decision.RouteKind) {
		return errAssistantRouteRuntimeInvalid
	}
	if strings.TrimSpace(decision.IntentID) == "" {
		return errAssistantRouteRuntimeInvalid
	}
	if !assistantValidRouteConfidenceBand(decision.ConfidenceBand) {
		return errAssistantRouteRuntimeInvalid
	}
	if strings.TrimSpace(decision.RouteCatalogVersion) == "" {
		return errAssistantRouteRuntimeInvalid
	}
	if strings.TrimSpace(decision.KnowledgeSnapshotDigest) == "" {
		return errAssistantRouteRuntimeInvalid
	}
	if strings.TrimSpace(decision.ResolverContractVersion) == "" {
		return errAssistantRouteRuntimeInvalid
	}
	if strings.TrimSpace(decision.DecisionSource) == "" {
		return errAssistantRouteRuntimeInvalid
	}
	if strings.TrimSpace(decision.RouteKind) == assistantRouteKindBusinessAction {
		if len(decision.CandidateActionIDs) == 0 {
			return errAssistantRouteRuntimeInvalid
		}
		for _, actionID := range decision.CandidateActionIDs {
			if strings.TrimSpace(actionID) == "" {
				return errAssistantRouteRuntimeInvalid
			}
		}
	} else if len(decision.CandidateActionIDs) != 0 {
		return errAssistantRouteActionConflict
	}
	return nil
}

func assistantBuildIntentRouteDecision(
	userInput string,
	resolved assistantResolveIntentResult,
	mergedIntent assistantIntentSpec,
	runtime *assistantKnowledgeRuntime,
) (assistantIntentRouteDecision, error) {
	if runtime == nil {
		return assistantIntentRouteDecision{}, errAssistantRouteCatalogMissing
	}
	if semanticDecision, ok, err := assistantBuildSemanticIntentRouteDecision(resolved, mergedIntent, runtime); ok || err != nil {
		return semanticDecision, err
	}
	projected := runtime.routeIntent(userInput, mergedIntent)
	decision := assistantIntentRouteDecision{
		RouteKind:               strings.TrimSpace(projected.RouteKind),
		IntentID:                strings.TrimSpace(projected.IntentID),
		RouteCatalogVersion:     strings.TrimSpace(runtime.RouteCatalogVersion),
		KnowledgeSnapshotDigest: strings.TrimSpace(runtime.SnapshotDigest),
		ResolverContractVersion: strings.TrimSpace(runtime.ResolverContractVersion),
		DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
	}
	if decision.RouteCatalogVersion == "" {
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonCatalogVersionMissing)
		decision.RouteCatalogVersion = "fallback-route-catalog"
	}
	if decision.KnowledgeSnapshotDigest == "" {
		decision.KnowledgeSnapshotDigest = "fallback-knowledge-snapshot"
	}
	if decision.ResolverContractVersion == "" {
		decision.ResolverContractVersion = "fallback-resolver-contract"
	}

	actionID := strings.TrimSpace(projected.Action)
	resolvedAction := strings.TrimSpace(resolved.Intent.Action)
	localUpgrade := actionID != "" && actionID != assistantIntentPlanOnly && (resolvedAction == "" || resolvedAction == assistantIntentPlanOnly)

	switch decision.RouteKind {
	case assistantRouteKindBusinessAction:
		decision.CandidateActionIDs = []string{actionID}
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonBusinessActionRegistered)
		if resolvedAction == "" || resolvedAction == assistantIntentPlanOnly {
			decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonModelPlanOnly)
		}
		if localUpgrade {
			decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonLocalIntentUpgrade)
		}
		decision.ConfidenceBand = assistantRouteConfidenceHigh
		if localUpgrade {
			decision.ConfidenceBand = assistantRouteConfidenceMedium
		}
	case assistantRouteKindKnowledgeQA, assistantRouteKindChitchat:
		decision.ConfidenceBand = assistantRouteConfidenceLow
		decision.ClarificationRequired = false
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonNonBusinessCatalogMatch)
	case assistantRouteKindUncertain:
		decision.ConfidenceBand = assistantRouteConfidenceLow
		decision.ClarificationRequired = true
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonUncertainNoMatch, assistantRouteReasonClarificationRequired)
	default:
		return assistantIntentRouteDecision{}, errAssistantRouteRuntimeInvalid
	}

	decision.CandidateActionIDs = assistantNormalizeRouteStringSlice(decision.CandidateActionIDs)
	decision.ReasonCodes = assistantNormalizeRouteStringSlice(decision.ReasonCodes)
	return decision, nil
}

func assistantBuildSemanticIntentRouteDecision(
	resolved assistantResolveIntentResult,
	mergedIntent assistantIntentSpec,
	runtime *assistantKnowledgeRuntime,
) (assistantIntentRouteDecision, bool, error) {
	routeKind := strings.TrimSpace(resolved.Intent.RouteKind)
	intentID := strings.TrimSpace(resolved.Intent.IntentID)
	if routeKind == "" && intentID == "" {
		return assistantIntentRouteDecision{}, false, nil
	}
	if runtime == nil {
		return assistantIntentRouteDecision{}, true, errAssistantRouteCatalogMissing
	}
	if !assistantValidRouteKind(routeKind) || intentID == "" {
		return assistantIntentRouteDecision{}, true, errAssistantRouteRuntimeInvalid
	}

	decision := assistantIntentRouteDecision{
		RouteKind:               routeKind,
		IntentID:                intentID,
		RouteCatalogVersion:     strings.TrimSpace(firstNonEmpty(resolved.Intent.RouteCatalogVersion, runtime.RouteCatalogVersion)),
		KnowledgeSnapshotDigest: strings.TrimSpace(runtime.SnapshotDigest),
		ResolverContractVersion: strings.TrimSpace(runtime.ResolverContractVersion),
		DecisionSource:          assistantRouteDecisionSourceSemanticModelV1,
	}
	if decision.RouteCatalogVersion == "" {
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonCatalogVersionMissing)
		decision.RouteCatalogVersion = "fallback-route-catalog"
	}
	if decision.KnowledgeSnapshotDigest == "" {
		decision.KnowledgeSnapshotDigest = "fallback-knowledge-snapshot"
	}
	if decision.ResolverContractVersion == "" {
		decision.ResolverContractVersion = "fallback-resolver-contract"
	}

	switch decision.RouteKind {
	case assistantRouteKindBusinessAction:
		actionID := strings.TrimSpace(firstNonEmpty(mergedIntent.Action, resolved.Intent.Action))
		if actionID == "" || actionID == assistantIntentPlanOnly {
			return assistantIntentRouteDecision{}, true, errAssistantRouteRuntimeInvalid
		}
		if _, ok := assistantLookupDefaultActionSpec(actionID); !ok {
			return assistantIntentRouteDecision{}, true, errAssistantRouteRuntimeInvalid
		}
		decision.CandidateActionIDs = []string{actionID}
		decision.ConfidenceBand = assistantRouteConfidenceHigh
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonBusinessActionRegistered)
		if strings.TrimSpace(resolved.Readiness) == assistantSemanticReadinessNeedMoreInfo {
			decision.ConfidenceBand = assistantRouteConfidenceMedium
		}
	case assistantRouteKindKnowledgeQA, assistantRouteKindChitchat:
		decision.ConfidenceBand = assistantRouteConfidenceLow
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonNonBusinessCatalogMatch)
	case assistantRouteKindUncertain:
		decision.ConfidenceBand = assistantRouteConfidenceLow
		decision.ClarificationRequired = true
		decision.ReasonCodes = append(decision.ReasonCodes, assistantRouteReasonUncertainNoMatch, assistantRouteReasonClarificationRequired)
	}

	decision.CandidateActionIDs = assistantNormalizeRouteStringSlice(decision.CandidateActionIDs)
	decision.ReasonCodes = assistantNormalizeRouteStringSlice(decision.ReasonCodes)
	return decision, true, nil
}

func assistantProjectIntentRouteDecision(intent assistantIntentSpec, decision assistantIntentRouteDecision) assistantIntentSpec {
	out := intent
	out.IntentID = strings.TrimSpace(decision.IntentID)
	out.RouteKind = strings.TrimSpace(decision.RouteKind)
	out.RouteCatalogVersion = strings.TrimSpace(decision.RouteCatalogVersion)
	if strings.TrimSpace(decision.RouteKind) != assistantRouteKindBusinessAction {
		out.Action = assistantIntentPlanOnly
		return out
	}
	if len(decision.CandidateActionIDs) == 1 {
		out.Action = strings.TrimSpace(decision.CandidateActionIDs[0])
	}
	return out
}

func assistantTurnRouteKind(turn *assistantTurn) string {
	if turn == nil {
		return ""
	}
	if assistantIntentRouteDecisionPresent(turn.RouteDecision) && assistantValidRouteKind(turn.RouteDecision.RouteKind) {
		return strings.TrimSpace(turn.RouteDecision.RouteKind)
	}
	if assistantValidRouteKind(turn.Intent.RouteKind) {
		return strings.TrimSpace(turn.Intent.RouteKind)
	}
	if actionID := strings.TrimSpace(turn.Intent.Action); actionID != "" && actionID != assistantIntentPlanOnly {
		return assistantRouteKindBusinessAction
	}
	return ""
}

func assistantTurnHasRouteClarificationSignal(turn *assistantTurn) bool {
	if turn == nil {
		return false
	}
	if assistantIntentRouteDecisionPresent(turn.RouteDecision) {
		return turn.RouteDecision.ClarificationRequired
	}
	return false
}

func assistantTurnRouteAuditVersionsConsistent(turn *assistantTurn) bool {
	if turn == nil {
		return true
	}
	if !assistantIntentRouteDecisionPresent(turn.RouteDecision) {
		return true
	}
	decision := turn.RouteDecision
	if strings.TrimSpace(turn.Plan.KnowledgeSnapshotDigest) == "" || strings.TrimSpace(turn.Plan.RouteCatalogVersion) == "" || strings.TrimSpace(turn.Plan.ResolverContractVersion) == "" {
		return false
	}
	if strings.TrimSpace(turn.Plan.ContextTemplateVersion) == "" || strings.TrimSpace(turn.Plan.ReplyGuidanceVersion) == "" {
		return false
	}
	if strings.TrimSpace(turn.Plan.KnowledgeSnapshotDigest) != strings.TrimSpace(decision.KnowledgeSnapshotDigest) {
		return false
	}
	if strings.TrimSpace(turn.Plan.RouteCatalogVersion) != strings.TrimSpace(decision.RouteCatalogVersion) {
		return false
	}
	if strings.TrimSpace(turn.Plan.ResolverContractVersion) != strings.TrimSpace(decision.ResolverContractVersion) {
		return false
	}
	return true
}

func assistantTurnActionChainAllowed(turn *assistantTurn) bool {
	if turn == nil {
		return false
	}
	if clarification := turn.Clarification; clarification != nil {
		status := strings.TrimSpace(clarification.Status)
		if status == assistantClarificationStatusOpen ||
			status == assistantClarificationStatusExhausted ||
			status == assistantClarificationStatusAborted {
			return false
		}
	}
	if assistantTurnHasRouteClarificationSignal(turn) {
		return false
	}
	return assistantTurnRouteKind(turn) == assistantRouteKindBusinessAction
}

func assistantActionGateRouteDecision(input assistantActionGateInput) (assistantIntentRouteDecision, bool) {
	if assistantIntentRouteDecisionPresent(input.RouteDecision) {
		return input.RouteDecision, true
	}
	if input.Turn != nil && assistantIntentRouteDecisionPresent(input.Turn.RouteDecision) {
		return input.Turn.RouteDecision, true
	}
	return assistantIntentRouteDecision{}, false
}

func assistantRouteGateDenied(err error, reason string) assistantActionGateDecision {
	httpStatus := httpStatusForAssistantRouteError(err)
	if httpStatus == 0 {
		httpStatus = 0
	}
	return assistantActionGateDecision{
		Allowed:    false,
		Error:      err,
		ErrorCode:  err.Error(),
		HTTPStatus: httpStatus,
		ReasonCode: strings.TrimSpace(reason),
	}
}

func assistantCheckRouteDecision(input assistantActionGateInput) assistantActionGateDecision {
	decision, ok := assistantActionGateRouteDecision(input)
	if !ok {
		if input.Stage == assistantActionStagePlan {
			return assistantActionGateDecision{Allowed: true}
		}
		return assistantRouteGateDenied(errAssistantRouteDecisionMissing, assistantRouteReasonDecisionMissing)
	}
	if err := assistantValidateIntentRouteDecision(decision); err != nil {
		if errors.Is(err, errAssistantRouteActionConflict) {
			return assistantRouteGateDenied(err, assistantRouteReasonCandidateActionConflict)
		}
		return assistantRouteGateDenied(err, assistantRouteReasonCatalogVersionMissing)
	}
	if strings.TrimSpace(decision.RouteKind) == assistantRouteKindBusinessAction {
		if len(decision.CandidateActionIDs) != 1 || strings.TrimSpace(input.Action.ID) == "" {
			if input.Stage == assistantActionStagePlan {
				return assistantActionGateDecision{Allowed: true}
			}
			return assistantRouteGateDenied(errAssistantRouteActionConflict, assistantRouteReasonCandidateActionConflict)
		}
		if candidate := strings.TrimSpace(decision.CandidateActionIDs[0]); candidate != strings.TrimSpace(input.Action.ID) {
			return assistantRouteGateDenied(errAssistantRouteActionConflict, assistantRouteReasonCandidateActionConflict)
		}
	}
	if input.Stage == assistantActionStagePlan {
		return assistantActionGateDecision{Allowed: true}
	}
	if strings.TrimSpace(decision.RouteKind) != assistantRouteKindBusinessAction {
		return assistantRouteGateDenied(errAssistantRouteNonBusinessBlocked, assistantRouteReasonNonBusinessBlocked)
	}
	if decision.ClarificationRequired {
		if input.Turn != nil && assistantTurnHasOpenClarification(input.Turn) {
			return assistantRouteGateDenied(errAssistantClarificationRequired, assistantRouteReasonClarificationRequired)
		}
		return assistantRouteGateDenied(errAssistantRouteClarificationRequired, assistantRouteReasonClarificationRequired)
	}
	return assistantActionGateDecision{Allowed: true}
}

func httpStatusForAssistantRouteError(err error) int {
	switch {
	case errors.Is(err, errAssistantRouteRuntimeInvalid):
		return 422
	case errors.Is(err, errAssistantRouteCatalogMissing):
		return 503
	case errors.Is(err, errAssistantRouteActionConflict):
		return 422
	case errors.Is(err, errAssistantRouteDecisionMissing):
		return 409
	case errors.Is(err, errAssistantRouteNonBusinessBlocked):
		return 409
	case errors.Is(err, errAssistantRouteClarificationRequired):
		return 409
	case errors.Is(err, errAssistantClarificationRequired):
		return 409
	case errors.Is(err, errAssistantClarificationRuntimeInvalid):
		return 409
	default:
		return 0
	}
}
