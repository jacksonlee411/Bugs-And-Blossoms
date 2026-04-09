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

// assistantRuntimeProposal 表示模型在 runtime 阶段给出的建议性语义，
// 在 authoritative gate 接受前，不能直接视为 turn 业务真值。
type assistantRuntimeProposal struct {
	ActionHint          string                              `json:"action_hint,omitempty"`
	RouteKindHint       string                              `json:"route_kind_hint,omitempty"`
	IntentIDHint        string                              `json:"intent_id_hint,omitempty"`
	RouteCatalogVersion string                              `json:"route_catalog_version,omitempty"`
	ParentRefText       string                              `json:"parent_ref_text,omitempty"`
	EntityName          string                              `json:"entity_name,omitempty"`
	EffectiveDate       string                              `json:"effective_date,omitempty"`
	OrgCode             string                              `json:"org_code,omitempty"`
	TargetEffectiveDate string                              `json:"target_effective_date,omitempty"`
	NewName             string                              `json:"new_name,omitempty"`
	NewParentRefText    string                              `json:"new_parent_ref_text,omitempty"`
	SelectedCandidateID string                              `json:"selected_candidate_id,omitempty"`
	Readiness           string                              `json:"readiness,omitempty"`
	GoalSummary         string                              `json:"goal_summary,omitempty"`
	UserVisibleReply    string                              `json:"user_visible_reply,omitempty"`
	NextQuestion        string                              `json:"next_question,omitempty"`
	RetrievalNeeded     bool                                `json:"retrieval_needed,omitempty"`
	RetrievalRequests   []assistantSemanticRetrievalRequest `json:"retrieval_requests,omitempty"`
	ConfidenceNote      string                              `json:"confidence_note,omitempty"`
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

func assistantNormalizeRuntimeProposal(proposal assistantRuntimeProposal) assistantRuntimeProposal {
	proposal.ActionHint = strings.TrimSpace(proposal.ActionHint)
	proposal.RouteKindHint = strings.TrimSpace(proposal.RouteKindHint)
	proposal.IntentIDHint = strings.TrimSpace(proposal.IntentIDHint)
	proposal.RouteCatalogVersion = strings.TrimSpace(proposal.RouteCatalogVersion)
	proposal.ParentRefText = strings.TrimSpace(proposal.ParentRefText)
	proposal.EntityName = strings.TrimSpace(proposal.EntityName)
	proposal.EffectiveDate = strings.TrimSpace(proposal.EffectiveDate)
	proposal.OrgCode = strings.TrimSpace(proposal.OrgCode)
	proposal.TargetEffectiveDate = strings.TrimSpace(proposal.TargetEffectiveDate)
	proposal.NewName = strings.TrimSpace(proposal.NewName)
	proposal.NewParentRefText = strings.TrimSpace(proposal.NewParentRefText)
	proposal.SelectedCandidateID = strings.TrimSpace(proposal.SelectedCandidateID)
	proposal.Readiness = strings.TrimSpace(proposal.Readiness)
	proposal.GoalSummary = strings.TrimSpace(proposal.GoalSummary)
	proposal.UserVisibleReply = strings.TrimSpace(proposal.UserVisibleReply)
	proposal.NextQuestion = strings.TrimSpace(proposal.NextQuestion)
	proposal.RetrievalRequests = assistantNormalizeSemanticRetrievalRequests(proposal.RetrievalRequests)
	proposal.ConfidenceNote = strings.TrimSpace(proposal.ConfidenceNote)
	return proposal
}

func assistantRuntimeProposalPresent(proposal assistantRuntimeProposal) bool {
	proposal = assistantNormalizeRuntimeProposal(proposal)
	return proposal.ActionHint != "" ||
		proposal.RouteKindHint != "" ||
		proposal.IntentIDHint != "" ||
		proposal.RouteCatalogVersion != "" ||
		proposal.ParentRefText != "" ||
		proposal.EntityName != "" ||
		proposal.EffectiveDate != "" ||
		proposal.OrgCode != "" ||
		proposal.TargetEffectiveDate != "" ||
		proposal.NewName != "" ||
		proposal.NewParentRefText != "" ||
		proposal.SelectedCandidateID != "" ||
		proposal.Readiness != "" ||
		proposal.GoalSummary != "" ||
		proposal.UserVisibleReply != "" ||
		proposal.NextQuestion != "" ||
		proposal.RetrievalNeeded ||
		len(proposal.RetrievalRequests) > 0 ||
		proposal.ConfidenceNote != ""
}

func (proposal assistantRuntimeProposal) intentSpec() assistantIntentSpec {
	normalized := assistantNormalizeRuntimeProposal(proposal)
	return assistantIntentSpec{
		Action:              normalized.ActionHint,
		IntentID:            normalized.IntentIDHint,
		RouteKind:           normalized.RouteKindHint,
		RouteCatalogVersion: normalized.RouteCatalogVersion,
		ParentRefText:       normalized.ParentRefText,
		EntityName:          normalized.EntityName,
		EffectiveDate:       normalized.EffectiveDate,
		OrgCode:             normalized.OrgCode,
		TargetEffectiveDate: normalized.TargetEffectiveDate,
		NewName:             normalized.NewName,
		NewParentRefText:    normalized.NewParentRefText,
	}
}

func assistantRuntimeProposalFromIntent(intent assistantIntentSpec) assistantRuntimeProposal {
	intent = assistantNormalizeIntentSpec(intent)
	return assistantRuntimeProposal{
		ActionHint:          intent.Action,
		RouteKindHint:       intent.RouteKind,
		IntentIDHint:        intent.IntentID,
		RouteCatalogVersion: intent.RouteCatalogVersion,
		ParentRefText:       intent.ParentRefText,
		EntityName:          intent.EntityName,
		EffectiveDate:       intent.EffectiveDate,
		OrgCode:             intent.OrgCode,
		TargetEffectiveDate: intent.TargetEffectiveDate,
		NewName:             intent.NewName,
		NewParentRefText:    intent.NewParentRefText,
	}
}

func (proposal assistantRuntimeProposal) semanticState() assistantConversationSemanticState {
	intent := proposal.intentSpec()
	normalized := assistantNormalizeRuntimeProposal(proposal)
	return assistantConversationSemanticState{
		GoalSummary:         normalized.GoalSummary,
		Action:              intent.Action,
		IntentID:            intent.IntentID,
		RouteKind:           intent.RouteKind,
		RouteCatalogVersion: intent.RouteCatalogVersion,
		Slots:               intent,
		RetrievalNeeded:     normalized.RetrievalNeeded,
		RetrievalRequests:   normalized.RetrievalRequests,
		NextQuestion:        normalized.NextQuestion,
		UserVisibleReply:    normalized.UserVisibleReply,
		Readiness:           normalized.Readiness,
		ConfidenceNote:      normalized.ConfidenceNote,
		SelectedCandidateID: normalized.SelectedCandidateID,
	}
}

func assistantNormalizeIntentSpec(intent assistantIntentSpec) assistantIntentSpec {
	intent.Action = strings.TrimSpace(intent.Action)
	intent.IntentID = strings.TrimSpace(intent.IntentID)
	intent.RouteKind = strings.TrimSpace(intent.RouteKind)
	intent.RouteCatalogVersion = strings.TrimSpace(intent.RouteCatalogVersion)
	intent.ParentRefText = strings.TrimSpace(intent.ParentRefText)
	intent.EntityName = strings.TrimSpace(intent.EntityName)
	intent.EffectiveDate = strings.TrimSpace(intent.EffectiveDate)
	intent.OrgCode = strings.TrimSpace(intent.OrgCode)
	intent.TargetEffectiveDate = strings.TrimSpace(intent.TargetEffectiveDate)
	intent.NewName = strings.TrimSpace(intent.NewName)
	intent.NewParentRefText = strings.TrimSpace(intent.NewParentRefText)
	intent.IntentSchemaVersion = strings.TrimSpace(intent.IntentSchemaVersion)
	intent.ContextHash = strings.TrimSpace(intent.ContextHash)
	intent.IntentHash = strings.TrimSpace(intent.IntentHash)
	return intent
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
	intent := assistantNormalizeIntentSpec(state.Slots)
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
	proposal := assistantNormalizeRuntimeProposal(resolved.Proposal)
	if assistantRuntimeProposalPresent(proposal) {
		state := proposal.semanticState()
		state.GoalSummary = strings.TrimSpace(firstNonEmpty(state.GoalSummary, resolved.GoalSummary))
		state.NextQuestion = strings.TrimSpace(firstNonEmpty(state.NextQuestion, resolved.NextQuestion))
		state.UserVisibleReply = strings.TrimSpace(firstNonEmpty(state.UserVisibleReply, resolved.UserVisibleReply))
		state.Readiness = strings.TrimSpace(firstNonEmpty(state.Readiness, resolved.Readiness))
		state.SelectedCandidateID = strings.TrimSpace(firstNonEmpty(state.SelectedCandidateID, resolved.SelectedCandidateID))
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
	resolved.Proposal = assistantNormalizeRuntimeProposal(resolved.Proposal)
	if !assistantRuntimeProposalPresent(resolved.Proposal) && resolved.Intent != (assistantIntentSpec{}) {
		resolved.Proposal = assistantRuntimeProposalFromIntent(resolved.Intent)
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
