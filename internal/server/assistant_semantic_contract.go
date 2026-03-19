package server

import "strings"

const (
	assistantSemanticRetrievalKindCandidateLookup = "candidate_lookup"

	assistantSemanticRetrievalStateNotRequested       = "not_requested"
	assistantSemanticRetrievalStateDeferredByBoundary = "deferred_by_boundary"
	assistantSemanticRetrievalStateNoMatch            = "no_match"
	assistantSemanticRetrievalStateMultipleMatches    = "multiple_matches"
	assistantSemanticRetrievalStateSingleMatch        = "single_match"

	assistantReplySourceProjection = "projection"
)

type assistantSemanticRetrievalRequest struct {
	Kind    string `json:"kind,omitempty"`
	Slot    string `json:"slot,omitempty"`
	RefText string `json:"ref_text,omitempty"`
	AsOf    string `json:"as_of,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type assistantSemanticRetrievalResult struct {
	Kind                string   `json:"kind,omitempty"`
	Slot                string   `json:"slot,omitempty"`
	State               string   `json:"state,omitempty"`
	RefText             string   `json:"ref_text,omitempty"`
	AsOf                string   `json:"as_of,omitempty"`
	CandidateCount      int      `json:"candidate_count,omitempty"`
	CandidateIDs        []string `json:"candidate_ids,omitempty"`
	SelectedCandidateID string   `json:"selected_candidate_id,omitempty"`
}

type assistantConversationSemanticState struct {
	GoalSummary         string                              `json:"goal_summary,omitempty"`
	Action              string                              `json:"action,omitempty"`
	IntentID            string                              `json:"intent_id,omitempty"`
	RouteKind           string                              `json:"route_kind,omitempty"`
	RouteCatalogVersion string                              `json:"route_catalog_version,omitempty"`
	Slots               assistantIntentSpec                 `json:"slots,omitempty"`
	RetrievalNeeded     bool                                `json:"retrieval_needed,omitempty"`
	RetrievalRequests   []assistantSemanticRetrievalRequest `json:"retrieval_requests,omitempty"`
	RetrievalResults    []assistantSemanticRetrievalResult  `json:"retrieval_results,omitempty"`
	NextQuestion        string                              `json:"next_question,omitempty"`
	UserVisibleReply    string                              `json:"user_visible_reply,omitempty"`
	Readiness           string                              `json:"readiness,omitempty"`
	ConfidenceNote      string                              `json:"confidence_note,omitempty"`
	SelectedCandidateID string                              `json:"selected_candidate_id,omitempty"`
}

func assistantSemanticReadinessKnown(value string) bool {
	switch strings.TrimSpace(value) {
	case "", assistantSemanticReadinessNeedMoreInfo, assistantSemanticReadinessReadyForDryRun, assistantSemanticReadinessReadyForConfirm, assistantSemanticReadinessNonBusiness:
		return true
	default:
		return false
	}
}

func assistantSemanticRetrievalStateKnown(value string) bool {
	switch strings.TrimSpace(value) {
	case "", assistantSemanticRetrievalStateNotRequested, assistantSemanticRetrievalStateDeferredByBoundary, assistantSemanticRetrievalStateNoMatch, assistantSemanticRetrievalStateMultipleMatches, assistantSemanticRetrievalStateSingleMatch:
		return true
	default:
		return false
	}
}

func assistantSemanticRetrievalResultPresent(result assistantSemanticRetrievalResult) bool {
	return strings.TrimSpace(result.Kind) != "" ||
		strings.TrimSpace(result.Slot) != "" ||
		strings.TrimSpace(result.State) != "" ||
		strings.TrimSpace(result.RefText) != "" ||
		strings.TrimSpace(result.AsOf) != "" ||
		result.CandidateCount > 0 ||
		len(result.CandidateIDs) > 0 ||
		strings.TrimSpace(result.SelectedCandidateID) != ""
}

func assistantSemanticStatePresent(state assistantConversationSemanticState) bool {
	return strings.TrimSpace(state.Action) != "" ||
		strings.TrimSpace(state.IntentID) != "" ||
		strings.TrimSpace(state.RouteKind) != "" ||
		strings.TrimSpace(state.GoalSummary) != "" ||
		strings.TrimSpace(state.UserVisibleReply) != "" ||
		strings.TrimSpace(state.NextQuestion) != "" ||
		strings.TrimSpace(state.Readiness) != "" ||
		len(state.RetrievalRequests) > 0 ||
		len(state.RetrievalResults) > 0 ||
		strings.TrimSpace(state.SelectedCandidateID) != ""
}

func assistantNormalizeSemanticRetrievalRequests(requests []assistantSemanticRetrievalRequest) []assistantSemanticRetrievalRequest {
	if len(requests) == 0 {
		return nil
	}
	out := make([]assistantSemanticRetrievalRequest, 0, len(requests))
	seen := map[string]struct{}{}
	for _, item := range requests {
		normalized := assistantSemanticRetrievalRequest{
			Kind:    strings.TrimSpace(item.Kind),
			Slot:    strings.TrimSpace(item.Slot),
			RefText: strings.TrimSpace(item.RefText),
			AsOf:    strings.TrimSpace(item.AsOf),
			Limit:   item.Limit,
		}
		if normalized.Kind == "" {
			continue
		}
		if normalized.Limit <= 0 {
			normalized.Limit = 10
		}
		key := normalized.Kind + "|" + normalized.Slot + "|" + normalized.RefText + "|" + normalized.AsOf
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func assistantNormalizeSemanticRetrievalResults(results []assistantSemanticRetrievalResult) []assistantSemanticRetrievalResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]assistantSemanticRetrievalResult, 0, len(results))
	seen := map[string]struct{}{}
	for _, item := range results {
		normalized := assistantSemanticRetrievalResult{
			Kind:                strings.TrimSpace(item.Kind),
			Slot:                strings.TrimSpace(item.Slot),
			State:               strings.TrimSpace(item.State),
			RefText:             strings.TrimSpace(item.RefText),
			AsOf:                strings.TrimSpace(item.AsOf),
			CandidateCount:      item.CandidateCount,
			CandidateIDs:        assistantNormalizeRouteStringSlice(item.CandidateIDs),
			SelectedCandidateID: strings.TrimSpace(item.SelectedCandidateID),
		}
		if normalized.Kind == "" || !assistantSemanticRetrievalStateKnown(normalized.State) {
			continue
		}
		if normalized.CandidateCount < 0 {
			normalized.CandidateCount = 0
		}
		key := normalized.Kind + "|" + normalized.Slot + "|" + normalized.State + "|" + normalized.RefText + "|" + normalized.AsOf
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (state assistantConversationSemanticState) intentSpec() assistantIntentSpec {
	intent := state.Slots
	intent.Action = strings.TrimSpace(firstNonEmpty(state.Action, intent.Action))
	intent.IntentID = strings.TrimSpace(firstNonEmpty(state.IntentID, intent.IntentID))
	intent.RouteKind = strings.TrimSpace(firstNonEmpty(state.RouteKind, intent.RouteKind))
	intent.RouteCatalogVersion = strings.TrimSpace(firstNonEmpty(state.RouteCatalogVersion, intent.RouteCatalogVersion))
	return intent
}

func assistantSemanticStateFromResolved(resolved assistantResolveIntentResult) assistantConversationSemanticState {
	if assistantSemanticStatePresent(resolved.SemanticState) {
		state := resolved.SemanticState
		state.Slots = state.intentSpec()
		state.Action = strings.TrimSpace(state.Slots.Action)
		state.IntentID = strings.TrimSpace(state.Slots.IntentID)
		state.RouteKind = strings.TrimSpace(state.Slots.RouteKind)
		state.RouteCatalogVersion = strings.TrimSpace(state.Slots.RouteCatalogVersion)
		state.RetrievalRequests = assistantNormalizeSemanticRetrievalRequests(state.RetrievalRequests)
		state.RetrievalResults = assistantNormalizeSemanticRetrievalResults(state.RetrievalResults)
		return state
	}
	return assistantConversationSemanticState{
		GoalSummary:         strings.TrimSpace(resolved.GoalSummary),
		Action:              strings.TrimSpace(resolved.Intent.Action),
		IntentID:            strings.TrimSpace(resolved.Intent.IntentID),
		RouteKind:           strings.TrimSpace(resolved.Intent.RouteKind),
		RouteCatalogVersion: strings.TrimSpace(resolved.Intent.RouteCatalogVersion),
		Slots:               resolved.Intent,
		NextQuestion:        strings.TrimSpace(resolved.NextQuestion),
		UserVisibleReply:    strings.TrimSpace(resolved.UserVisibleReply),
		Readiness:           strings.TrimSpace(resolved.Readiness),
		SelectedCandidateID: strings.TrimSpace(resolved.SelectedCandidateID),
	}
}

func assistantSyncResolvedSemanticResult(resolved *assistantResolveIntentResult) {
	if resolved == nil {
		return
	}
	state := assistantSemanticStateFromResolved(*resolved)
	resolved.SemanticState = state
	resolved.Intent = state.intentSpec()
	resolved.GoalSummary = strings.TrimSpace(state.GoalSummary)
	resolved.UserVisibleReply = strings.TrimSpace(state.UserVisibleReply)
	resolved.NextQuestion = strings.TrimSpace(state.NextQuestion)
	resolved.Readiness = strings.TrimSpace(state.Readiness)
	resolved.SelectedCandidateID = strings.TrimSpace(state.SelectedCandidateID)
}

func assistantSemanticStateNeedsRetrieval(state assistantConversationSemanticState) bool {
	if len(state.RetrievalRequests) > 0 {
		return true
	}
	intent := state.intentSpec()
	refText := assistantIntentCandidateRefText(intent)
	return refText != "" && strings.TrimSpace(intent.RouteKind) == assistantRouteKindBusinessAction
}

func assistantSemanticCandidateLookupRequest(intent assistantIntentSpec) (assistantSemanticRetrievalRequest, bool) {
	refText := assistantIntentCandidateRefText(intent)
	if refText == "" {
		return assistantSemanticRetrievalRequest{}, false
	}
	asOf := assistantIntentCandidateAsOf(intent)
	slot := "parent_ref_text"
	switch strings.TrimSpace(intent.Action) {
	case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion, assistantIntentCorrectOrgUnit, assistantIntentMoveOrgUnit:
		slot = "new_parent_ref_text"
	}
	return assistantSemanticRetrievalRequest{
		Kind:    assistantSemanticRetrievalKindCandidateLookup,
		Slot:    slot,
		RefText: refText,
		AsOf:    asOf,
		Limit:   10,
	}, true
}

func assistantModelSemanticStateInvalid(resolved assistantResolveIntentResult) bool {
	state := assistantSemanticStateFromResolved(resolved)
	intent := state.intentSpec()
	if assistantModelIntentInvalid(intent) {
		return true
	}
	if !assistantSemanticReadinessKnown(state.Readiness) {
		return true
	}
	return false
}
