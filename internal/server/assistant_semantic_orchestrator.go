package server

import (
	"context"
	"strings"
)

type assistantSemanticTurnResolution struct {
	Resolved            assistantResolveIntentResult
	Runtime             *assistantKnowledgeRuntime
	Candidates          []assistantCandidate
	ResolvedCandidateID string
	SelectedCandidateID string
	AmbiguityCount      int
	Confidence          float64
	ResolutionSource    string
	Retrieval           assistantSemanticRetrievalResult
}

func (s *assistantConversationService) resolveSemanticPrompt(ctx context.Context, tenantID string, conversationID string, prompt string) (assistantResolveIntentResult, error) {
	if s == nil {
		return assistantResolveIntentResult{}, errAssistantServiceMissing
	}
	if s.gatewayErr != nil {
		return assistantResolveIntentResult{}, s.gatewayErr
	}
	if s.modelGateway == nil {
		return assistantResolveIntentResult{}, errAssistantModelProviderUnavailable
	}
	resolved, err := s.modelGateway.ResolveIntent(ctx, assistantResolveIntentRequest{
		Prompt:         strings.TrimSpace(prompt),
		ConversationID: conversationID,
		TenantID:       tenantID,
	})
	if err != nil {
		return assistantResolveIntentResult{}, err
	}
	assistantSyncResolvedSemanticResult(&resolved)
	if assistantModelSemanticStateInvalid(resolved) {
		return assistantResolveIntentResult{}, errAssistantPlanSchemaConstrainedDecodeFailed
	}
	return resolved, nil
}

func assistantSanitizeSemanticState(state assistantConversationSemanticState, temporalHints assistantExplicitTemporalHints, pendingTurn *assistantTurn) assistantConversationSemanticState {
	intent := assistantSanitizeResolvedIntentFacts(state.intentSpec(), temporalHints, pendingTurn)
	state.Slots = intent
	state.Action = strings.TrimSpace(intent.Action)
	state.IntentID = strings.TrimSpace(intent.IntentID)
	state.RouteKind = strings.TrimSpace(intent.RouteKind)
	state.RouteCatalogVersion = strings.TrimSpace(intent.RouteCatalogVersion)
	return state
}

func assistantSemanticConfidence(state assistantConversationSemanticState, retrieval assistantSemanticRetrievalResult) float64 {
	switch strings.TrimSpace(retrieval.State) {
	case assistantSemanticRetrievalStateSingleMatch:
		return 0.95
	case assistantSemanticRetrievalStateMultipleMatches:
		return 0.55
	case assistantSemanticRetrievalStateNoMatch:
		return 0.30
	case assistantSemanticRetrievalStateDeferredByBoundary:
		return 0.65
	}
	switch strings.TrimSpace(state.Readiness) {
	case assistantSemanticReadinessReadyForConfirm:
		return 0.95
	case assistantSemanticReadinessReadyForDryRun:
		return 0.80
	case assistantSemanticReadinessNeedMoreInfo:
		return 0.65
	default:
		if strings.TrimSpace(state.RouteKind) == assistantRouteKindBusinessAction {
			return 0.65
		}
		return 0.30
	}
}

func assistantSemanticCandidateLookupFromState(state assistantConversationSemanticState) (assistantSemanticRetrievalRequest, bool, bool) {
	for _, item := range state.RetrievalRequests {
		if strings.TrimSpace(item.Kind) != assistantSemanticRetrievalKindCandidateLookup {
			continue
		}
		return item, true, true
	}
	request, ok := assistantSemanticCandidateLookupRequest(state.intentSpec())
	return request, false, ok
}

func assistantBuildRetrievalCandidateIDs(candidates []assistantCandidate) []string {
	if len(candidates) == 0 {
		return nil
	}
	ids := make([]string, 0, len(candidates))
	for _, item := range candidates {
		if id := strings.TrimSpace(item.CandidateID); id != "" {
			ids = append(ids, id)
		}
	}
	return assistantNormalizeRouteStringSlice(ids)
}

func (s *assistantConversationService) executeSemanticCandidateLookup(
	ctx context.Context,
	tenantID string,
	state assistantConversationSemanticState,
) (assistantSemanticRetrievalResult, []assistantCandidate, error) {
	request, _, ok := assistantSemanticCandidateLookupFromState(state)
	if !ok {
		return assistantSemanticRetrievalResult{
			Kind:  assistantSemanticRetrievalKindCandidateLookup,
			State: assistantSemanticRetrievalStateNotRequested,
		}, nil, nil
	}
	result := assistantSemanticRetrievalResult{
		Kind:    assistantSemanticRetrievalKindCandidateLookup,
		Slot:    strings.TrimSpace(request.Slot),
		RefText: strings.TrimSpace(request.RefText),
		AsOf:    strings.TrimSpace(request.AsOf),
	}
	if result.RefText == "" {
		return assistantSemanticRetrievalResult{
			Kind:  assistantSemanticRetrievalKindCandidateLookup,
			Slot:  strings.TrimSpace(request.Slot),
			State: assistantSemanticRetrievalStateNotRequested,
		}, nil, nil
	}
	if result.AsOf == "" {
		result.State = assistantSemanticRetrievalStateDeferredByBoundary
		return result, nil, nil
	}
	candidates, err := s.resolveCandidates(ctx, tenantID, result.RefText, result.AsOf)
	if err != nil {
		if !errorsIsAny(err, errOrgUnitNotFound) {
			return assistantSemanticRetrievalResult{}, nil, err
		}
		candidates = nil
	}
	result.CandidateCount = len(candidates)
	result.CandidateIDs = assistantBuildRetrievalCandidateIDs(candidates)
	switch len(candidates) {
	case 0:
		result.State = assistantSemanticRetrievalStateNoMatch
	case 1:
		result.State = assistantSemanticRetrievalStateSingleMatch
		result.SelectedCandidateID = strings.TrimSpace(candidates[0].CandidateID)
	default:
		result.State = assistantSemanticRetrievalStateMultipleMatches
	}
	return result, candidates, nil
}

func assistantSemanticSelectCandidate(state assistantConversationSemanticState, candidates []assistantCandidate, retrieval assistantSemanticRetrievalResult) (resolvedCandidateID string, selectedCandidateID string, resolutionSource string) {
	selectedCandidateID = strings.TrimSpace(state.SelectedCandidateID)
	if selectedCandidateID != "" && assistantCandidateExists(candidates, selectedCandidateID) {
		return selectedCandidateID, selectedCandidateID, assistantResolutionUserConfirmed
	}
	if strings.TrimSpace(retrieval.State) == assistantSemanticRetrievalStateSingleMatch && len(candidates) == 1 {
		return strings.TrimSpace(candidates[0].CandidateID), "", assistantResolutionAuto
	}
	if strings.TrimSpace(retrieval.State) == assistantSemanticRetrievalStateDeferredByBoundary {
		return "", "", "deferred_candidate_lookup"
	}
	return "", "", ""
}

func (s *assistantConversationService) orchestrateSemanticTurn(
	ctx context.Context,
	tenantID string,
	principal Principal,
	conversationID string,
	userInput string,
	pendingTurn *assistantTurn,
) (assistantSemanticTurnResolution, error) {
	resolved, err := s.resolveIntentWithPendingTurn(ctx, tenantID, conversationID, userInput, pendingTurn)
	if err != nil {
		return assistantSemanticTurnResolution{}, err
	}
	runtime, err := s.ensureKnowledgeRuntime()
	if err != nil {
		return assistantSemanticTurnResolution{}, err
	}
	assistantSyncResolvedSemanticResult(&resolved)
	contextEnvelope := assistantAssembleSemanticContext(assistantSemanticContextAssemblerInput{
		UserInput:   userInput,
		PendingTurn: pendingTurn,
	})
	temporalHints := assistantExtractExplicitTemporalHints(strings.TrimSpace(userInput))
	state := assistantSanitizeSemanticState(assistantSemanticStateFromResolved(resolved), temporalHints, pendingTurn)
	state = assistantSupplementSemanticStateForCreateOrgUnit(state, pendingTurn, userInput)
	resolved.SemanticState = state
	resolved.Proposal = assistantRuntimeProposalFromIntent(state.intentSpec())
	assistantSyncResolvedSemanticResult(&resolved)

	retrieval, candidates, err := s.executeSemanticCandidateLookup(ctx, tenantID, state)
	if err != nil {
		return assistantSemanticTurnResolution{}, err
	}
	state.RetrievalNeeded = assistantSemanticStateNeedsRetrieval(state)
	if assistantSemanticRetrievalResultPresent(retrieval) {
		state.RetrievalResults = []assistantSemanticRetrievalResult{retrieval}
	}

	request, explicitRequest, hasRequest := assistantSemanticCandidateLookupFromState(state)
	if explicitRequest &&
		hasRequest &&
		strings.TrimSpace(request.RefText) != "" &&
		strings.TrimSpace(retrieval.State) != assistantSemanticRetrievalStateDeferredByBoundary &&
		strings.TrimSpace(retrieval.State) != assistantSemanticRetrievalStateNotRequested {
		followupResolved, followupErr := s.resolveSemanticPrompt(ctx, tenantID, conversationID, assistantBuildSemanticResolutionPrompt(contextEnvelope, state))
		if followupErr != nil {
			return assistantSemanticTurnResolution{}, followupErr
		}
		followupState := assistantSanitizeSemanticState(assistantSemanticStateFromResolved(followupResolved), temporalHints, pendingTurn)
		followupState = assistantSupplementSemanticStateForCreateOrgUnit(followupState, pendingTurn, userInput)
		followupState.RetrievalNeeded = state.RetrievalNeeded
		followupState.RetrievalResults = append([]assistantSemanticRetrievalResult(nil), state.RetrievalResults...)
		if strings.TrimSpace(followupState.SelectedCandidateID) == "" {
			followupState.SelectedCandidateID = strings.TrimSpace(state.SelectedCandidateID)
		}
		followupResolved.SemanticState = followupState
		followupResolved.Proposal = assistantRuntimeProposalFromIntent(followupState.intentSpec())
		assistantSyncResolvedSemanticResult(&followupResolved)
		resolved = followupResolved
		state = assistantSemanticStateFromResolved(resolved)
	}

	resolvedCandidateID, selectedCandidateID, resolutionSource := assistantSemanticSelectCandidate(state, candidates, retrieval)
	if selectedCandidateID == "" {
		selectedCandidateID = strings.TrimSpace(firstNonEmpty(resolved.SelectedCandidateID, state.SelectedCandidateID))
		if selectedCandidateID != "" && !assistantCandidateExists(candidates, selectedCandidateID) {
			selectedCandidateID = ""
		}
	}

	return assistantSemanticTurnResolution{
		Resolved:            resolved,
		Runtime:             runtime,
		Candidates:          candidates,
		ResolvedCandidateID: resolvedCandidateID,
		SelectedCandidateID: selectedCandidateID,
		AmbiguityCount:      len(candidates),
		Confidence:          assistantSemanticConfidence(state, retrieval),
		ResolutionSource:    resolutionSource,
		Retrieval:           retrieval,
	}, nil
}

func assistantSupplementSemanticStateForCreateOrgUnit(state assistantConversationSemanticState, pendingTurn *assistantTurn, userInput string) assistantConversationSemanticState {
	intent := assistantSupplementCreateOrgUnitIntentForDraft(state.intentSpec(), pendingTurn, userInput)
	state.Slots = intent
	state.Action = strings.TrimSpace(intent.Action)
	state.IntentID = strings.TrimSpace(intent.IntentID)
	state.RouteKind = strings.TrimSpace(intent.RouteKind)
	state.RouteCatalogVersion = strings.TrimSpace(intent.RouteCatalogVersion)
	return state
}
