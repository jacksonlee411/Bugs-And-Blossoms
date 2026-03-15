package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const (
	assistantRouteDecisionSourceSemanticModelV1 = "semantic_model_v1"
	assistantReplySourceSemanticModel           = "semantic_model"

	assistantSemanticReadinessNeedMoreInfo    = "need_more_info"
	assistantSemanticReadinessReadyForDryRun  = "ready_for_dry_run"
	assistantSemanticReadinessReadyForConfirm = "ready_for_confirm"
	assistantSemanticReadinessNonBusiness     = "non_business"
)

type assistantSemanticPromptAction struct {
	ActionID      string   `json:"action_id"`
	PlanSummary   string   `json:"plan_summary"`
	RequiredSlots []string `json:"required_slots,omitempty"`
}

type assistantSemanticPromptCandidate struct {
	CandidateID   string `json:"candidate_id"`
	CandidateCode string `json:"candidate_code"`
	Name          string `json:"name"`
	Path          string `json:"path,omitempty"`
}

type assistantSemanticPromptPending struct {
	Action              string                             `json:"action,omitempty"`
	MissingFields       []string                           `json:"missing_fields,omitempty"`
	PendingDraftSummary string                             `json:"pending_draft_summary,omitempty"`
	SelectedCandidateID string                             `json:"selected_candidate_id,omitempty"`
	Candidates          []assistantSemanticPromptCandidate `json:"candidates,omitempty"`
}

type assistantSemanticPromptEnvelope struct {
	CurrentUserInput string                          `json:"current_user_input"`
	AllowedActions   []assistantSemanticPromptAction `json:"allowed_actions"`
	PendingTurn      *assistantSemanticPromptPending `json:"pending_turn,omitempty"`
}

func assistantBuildSemanticPrompt(userInput string, pendingTurn *assistantTurn) string {
	envelope := assistantSemanticPromptEnvelope{
		CurrentUserInput: strings.TrimSpace(userInput),
		AllowedActions:   assistantSemanticPromptActions(),
	}
	if pending := assistantSemanticPromptPendingTurn(pendingTurn); pending != nil {
		envelope.PendingTurn = pending
	}
	payload, _ := json.Marshal(envelope)
	return string(payload)
}

func assistantSemanticPromptActions() []assistantSemanticPromptAction {
	actions := []string{
		assistantIntentCreateOrgUnit,
		assistantIntentAddOrgUnitVersion,
		assistantIntentInsertOrgUnitVersion,
		assistantIntentCorrectOrgUnit,
		assistantIntentMoveOrgUnit,
		assistantIntentRenameOrgUnit,
		assistantIntentDisableOrgUnit,
		assistantIntentEnableOrgUnit,
		assistantIntentPlanOnly,
	}
	out := make([]assistantSemanticPromptAction, 0, len(actions))
	for _, actionID := range actions {
		spec, _ := assistantLookupDefaultActionSpec(actionID)
		item := assistantSemanticPromptAction{
			ActionID:    strings.TrimSpace(spec.ID),
			PlanSummary: strings.TrimSpace(spec.PlanSummary),
		}
		switch actionID {
		case assistantIntentCreateOrgUnit:
			item.RequiredSlots = []string{"parent_ref_text", "entity_name", "effective_date"}
		case assistantIntentAddOrgUnitVersion, assistantIntentInsertOrgUnitVersion:
			item.RequiredSlots = []string{"org_code", "effective_date"}
		case assistantIntentCorrectOrgUnit:
			item.RequiredSlots = []string{"org_code", "target_effective_date"}
		case assistantIntentMoveOrgUnit:
			item.RequiredSlots = []string{"org_code", "effective_date", "new_parent_ref_text"}
		case assistantIntentRenameOrgUnit:
			item.RequiredSlots = []string{"org_code", "effective_date", "new_name"}
		case assistantIntentDisableOrgUnit, assistantIntentEnableOrgUnit:
			item.RequiredSlots = []string{"org_code", "effective_date"}
		}
		out = append(out, item)
	}
	return out
}

func assistantSemanticPromptPendingTurn(turn *assistantTurn) *assistantSemanticPromptPending {
	if turn == nil {
		return nil
	}
	out := &assistantSemanticPromptPending{
		Action:              strings.TrimSpace(turn.Intent.Action),
		MissingFields:       append([]string(nil), assistantTurnMissingFields(turn)...),
		PendingDraftSummary: strings.TrimSpace(turn.PendingDraftSummary),
		SelectedCandidateID: strings.TrimSpace(firstNonEmpty(turn.SelectedCandidateID, turn.ResolvedCandidateID)),
	}
	if len(turn.Candidates) > 0 {
		out.Candidates = make([]assistantSemanticPromptCandidate, 0, len(turn.Candidates))
		for _, candidate := range turn.Candidates {
			out.Candidates = append(out.Candidates, assistantSemanticPromptCandidate{
				CandidateID:   strings.TrimSpace(candidate.CandidateID),
				CandidateCode: strings.TrimSpace(candidate.CandidateCode),
				Name:          strings.TrimSpace(candidate.Name),
				Path:          strings.TrimSpace(candidate.Path),
			})
		}
	}
	if strings.TrimSpace(out.Action) == "" && len(out.MissingFields) == 0 && len(out.Candidates) == 0 && strings.TrimSpace(out.PendingDraftSummary) == "" {
		return nil
	}
	return out
}

func assistantSemanticCurrentUserInput(prompt string) string {
	var envelope assistantSemanticPromptEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(prompt)), &envelope); err == nil {
		if text := strings.TrimSpace(envelope.CurrentUserInput); text != "" {
			return text
		}
	}
	return strings.TrimSpace(prompt)
}

func assistantSeedTurnReplyFromSemantic(turn *assistantTurn, resolved assistantResolveIntentResult) {
	if turn == nil {
		return
	}
	text := strings.TrimSpace(resolved.UserVisibleReply)
	if text == "" {
		text = strings.TrimSpace(resolved.NextQuestion)
	}
	if text == "" {
		switch {
		case strings.TrimSpace(turn.PendingDraftSummary) != "":
			text = strings.TrimSpace(turn.PendingDraftSummary)
		case strings.TrimSpace(turn.DryRun.Explain) != "":
			text = strings.TrimSpace(turn.DryRun.Explain)
		case len(turn.MissingFields) > 0:
			text = "请继续补充缺失信息后再继续。"
		default:
			return
		}
	}
	reply := &assistantRenderReplyResponse{
		Text:               text,
		Kind:               "message",
		Stage:              assistantSemanticReplyStage(turn),
		ReplyModelName:     strings.TrimSpace(firstNonEmpty(resolved.ModelName, turn.Plan.ModelName)),
		ReplyPromptVersion: assistantReplyPromptVersionV1,
		ReplySource:        assistantReplySourceSemanticModel,
		UsedFallback:       false,
	}
	assistantSetReplySnapshot(turn, reply, "")
}

func assistantSemanticReplyStage(turn *assistantTurn) string {
	if turn == nil {
		return "draft"
	}
	switch strings.TrimSpace(turn.Phase) {
	case assistantPhaseAwaitCommitConfirm:
		return "confirm_summary"
	case assistantPhaseAwaitCandidatePick, assistantPhaseAwaitCandidateConfirm, assistantPhaseAwaitClarification, assistantPhaseAwaitMissingFields:
		return "clarification_required"
	case assistantPhaseFailed:
		return "commit_failed"
	default:
		return "draft"
	}
}

func assistantSemanticReplyFromTurn(turn *assistantTurn, conversationID string, turnID string) *assistantRenderReplyResponse {
	if turn == nil {
		return nil
	}
	if turn.ReplyNLG != nil && strings.TrimSpace(turn.ReplyNLG.Text) != "" {
		copyReply := *turn.ReplyNLG
		copyReply.ConversationID = strings.TrimSpace(conversationID)
		copyReply.TurnID = strings.TrimSpace(turnID)
		return &copyReply
	}
	if turn.CommitReply == nil || strings.TrimSpace(turn.CommitReply.Message) == "" {
		return nil
	}
	return &assistantRenderReplyResponse{
		Text:               strings.TrimSpace(turn.CommitReply.Message),
		Kind:               "message",
		Stage:              assistantSemanticReplyStage(turn),
		ReplyModelName:     strings.TrimSpace(turn.Plan.ModelName),
		ReplyPromptVersion: assistantReplyPromptVersionV1,
		ReplySource:        assistantReplySourceSemanticModel,
		UsedFallback:       false,
		ConversationID:     strings.TrimSpace(conversationID),
		TurnID:             strings.TrimSpace(turnID),
	}
}

func (s *assistantConversationService) prepareTurnDraft(
	ctx context.Context,
	tenantID string,
	principal Principal,
	conversationID string,
	userInput string,
	pendingTurn *assistantTurn,
) (*assistantTurn, error) {
	resolvedIntent, err := s.resolveIntentWithPendingTurn(ctx, tenantID, conversationID, userInput, pendingTurn)
	if err != nil {
		return nil, err
	}
	knowledgeRuntime, err := s.ensureKnowledgeRuntime()
	if err != nil {
		return nil, err
	}

	intent := resolvedIntent.Intent
	resume := assistantClarificationResumeResult{Intent: intent}
	if pendingTurn != nil {
		resume = assistantResumeFromClarificationFn(pendingTurn, userInput, intent)
		intent = resume.Intent
	}

	routeDecision, err := assistantBuildIntentRouteDecisionFn(userInput, resolvedIntent, intent, knowledgeRuntime)
	if err != nil {
		return nil, err
	}
	intent = assistantProjectIntentRouteDecision(intent, routeDecision)

	candidates := make([]assistantCandidate, 0)
	resolvedCandidateID := ""
	selectedCandidateID := strings.TrimSpace(firstNonEmpty(resolvedIntent.SelectedCandidateID, resume.SelectedCandidateID))
	resolutionSource := ""
	ambiguityCount := 0
	confidence := 0.65

	candidateRefText := assistantIntentCandidateRefText(intent)
	candidateAsOf := assistantIntentCandidateAsOf(intent)
	if candidateRefText != "" && candidateAsOf != "" {
		resolved, resolveErr := s.resolveCandidates(ctx, tenantID, candidateRefText, candidateAsOf)
		if resolveErr != nil {
			if !errorsIsAny(resolveErr, errOrgUnitNotFound) {
				return nil, resolveErr
			}
			resolved = make([]assistantCandidate, 0)
		}
		candidates = resolved
		ambiguityCount = len(candidates)
		switch len(candidates) {
		case 0:
			confidence = 0.3
		case 1:
			resolvedCandidateID = candidates[0].CandidateID
			resolutionSource = assistantResolutionAuto
			confidence = 0.95
		default:
			confidence = 0.55
		}
	} else if candidateRefText != "" {
		resolutionSource = "deferred_candidate_lookup"
	}

	if selectedCandidateID != "" && assistantCandidateExists(candidates, selectedCandidateID) {
		resolvedCandidateID = selectedCandidateID
		resolutionSource = assistantResolutionUserConfirmed
		confidence = 0.95
	}
	if resumeCandidateID := strings.TrimSpace(resume.ResolvedCandidateID); resumeCandidateID != "" && assistantCandidateExists(candidates, resumeCandidateID) {
		resolvedCandidateID = resumeCandidateID
		selectedCandidateID = resumeCandidateID
		resolutionSource = assistantResolutionUserConfirmed
		confidence = 0.95
	}

	dryRun := assistantBuildDryRunFn(intent, candidates, resolvedCandidateID)
	dryRun = s.enrichCreateOrgUnitDryRunWithPolicy(ctx, tenantID, intent, candidates, resolvedCandidateID, dryRun)

	var pendingClarification *assistantClarificationDecision
	if pendingTurn != nil {
		pendingClarification = pendingTurn.Clarification
	}
	clarification := assistantBuildClarificationDecisionFn(assistantClarificationBuildInput{
		UserInput:            userInput,
		Intent:               intent,
		RouteDecision:        routeDecision,
		DryRun:               dryRun,
		Candidates:           candidates,
		ResolvedCandidateID:  resolvedCandidateID,
		SelectedCandidateID:  selectedCandidateID,
		Runtime:              knowledgeRuntime,
		PendingClarification: pendingClarification,
		ResumeProgress:       resume.Progress,
	})
	spec, specOK := s.lookupActionSpec(intent.Action)
	requiresActionSpec := assistantIntentNeedsActionSpec(intent, routeDecision)
	if requiresActionSpec && !specOK {
		return nil, errAssistantUnsupportedIntent
	}

	plan := assistantBuildPlan(intent)
	turnCreatedAt := time.Now().UTC()
	plan = assistantFreezeConfirmWindow(plan, turnCreatedAt)
	plan.ModelProvider = resolvedIntent.ProviderName
	plan.ModelName = resolvedIntent.ModelName
	plan.ModelRevision = resolvedIntent.ModelRevision

	if requiresActionSpec && specOK {
		skillExecutionPlan, configDeltaPlan := assistantCompileIntentToPlansWithSpec(intent, resolvedCandidateID, spec)
		plan.SkillExecutionPlan = skillExecutionPlan
		plan.ConfigDeltaPlan = configDeltaPlan
		decision := assistantEvaluateActionGate(assistantActionGateInput{
			Stage:         assistantActionStagePlan,
			TenantID:      tenantID,
			Principal:     principal,
			Action:        spec,
			Intent:        intent,
			RouteDecision: routeDecision,
			Candidates:    candidates,
			ResolvedID:    resolvedCandidateID,
			UserInput:     userInput,
		})
		if !decision.Allowed {
			if errorsIsAny(decision.Error, errAssistantActionCapabilityUnregistered) {
				return nil, errAssistantPlanBoundaryViolation
			}
			return nil, decision.Error
		}
		tempTurn := &assistantTurn{
			Intent:              intent,
			RouteDecision:       routeDecision,
			Clarification:       clarification,
			Plan:                plan,
			Candidates:          candidates,
			ResolvedCandidateID: resolvedCandidateID,
			SelectedCandidateID: selectedCandidateID,
			DryRun:              dryRun,
		}
		if err := s.refreshTurnVersionTuple(ctx, tenantID, tempTurn); err != nil {
			return nil, err
		}
		plan = tempTurn.Plan
		dryRun = tempTurn.DryRun
	}
	assistantApplyPlanKnowledgeSnapshot(&plan, routeDecision, knowledgeRuntime)
	tempTurn := &assistantTurn{
		Intent:              intent,
		RouteDecision:       routeDecision,
		Clarification:       clarification,
		Plan:                plan,
		Candidates:          candidates,
		ResolvedCandidateID: resolvedCandidateID,
		SelectedCandidateID: selectedCandidateID,
		DryRun:              dryRun,
	}
	planContext, err := knowledgeRuntime.buildPlanContextV1(tenantID, knowledgeRuntime.planContextLocale(), intent, spec, tempTurn)
	if err != nil {
		return nil, err
	}
	assistantApplyPlanContextV1(&plan, &dryRun, intent, planContext)
	tempTurn.Plan = plan
	tempTurn.DryRun = dryRun
	if !assistantTurnRouteAuditVersionsConsistent(tempTurn) {
		return nil, errAssistantPlanContractVersionMismatch
	}
	if err := assistantAnnotateIntentPlanFn(tenantID, conversationID, userInput, &intent, &plan, &dryRun); err != nil {
		return nil, err
	}
	policyVersion, compositionVersion, mappingVersion := assistantTurnVersionSnapshot(plan.CapabilityKey)
	turn := &assistantTurn{
		TurnID:              "turn_" + strings.ReplaceAll(newUUIDString(), "-", ""),
		UserInput:           userInput,
		State:               assistantStateValidated,
		RiskTier:            "low",
		RequestID:           "assistant_" + strings.ReplaceAll(newUUIDString(), "-", ""),
		TraceID:             strings.ReplaceAll(newUUIDString(), "-", ""),
		PolicyVersion:       policyVersion,
		CompositionVersion:  compositionVersion,
		MappingVersion:      mappingVersion,
		Intent:              intent,
		RouteDecision:       routeDecision,
		Clarification:       clarification,
		Plan:                plan,
		Candidates:          candidates,
		ResolvedCandidateID: resolvedCandidateID,
		SelectedCandidateID: selectedCandidateID,
		AmbiguityCount:      ambiguityCount,
		Confidence:          confidence,
		ResolutionSource:    resolutionSource,
		DryRun:              dryRun,
		CreatedAt:           turnCreatedAt,
		UpdatedAt:           turnCreatedAt,
	}
	if specOK {
		turn.RiskTier = strings.TrimSpace(spec.Security.RiskTier)
		if turn.RiskTier == "" {
			turn.RiskTier = "low"
		}
	}
	assistantRefreshTurnDerivedFields(turn)
	assistantSeedTurnReplyFromSemantic(turn, resolvedIntent)
	return turn, nil
}
