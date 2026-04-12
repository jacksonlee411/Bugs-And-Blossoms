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

type assistantAuthoritativeDecision struct {
	Accepted            bool
	Intent              assistantIntentSpec
	ActionSpec          assistantActionSpec
	RouteDecision       assistantIntentRouteDecision
	Candidates          []assistantCandidate
	ResolvedCandidate   *assistantCandidate
	ResolvedCandidateID string
	SelectedCandidateID string
	Clarification       *assistantClarificationDecision
	FailClosedCode      string
}

func assistantBuildSemanticPrompt(userInput string, pendingTurn *assistantTurn) string {
	envelope := assistantAssembleSemanticContext(assistantSemanticContextAssemblerInput{
		UserInput:   userInput,
		PendingTurn: pendingTurn,
	})
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

func assistantValidateProposalWriteDates(proposal assistantRuntimeProposal, retrieval assistantSemanticRetrievalResult) error {
	proposal = assistantNormalizeRuntimeProposal(proposal)
	if proposal.EffectiveDate != "" && !assistantDateISOYMD(proposal.EffectiveDate) {
		return errAssistantPlanSchemaConstrainedDecodeFailed
	}
	if proposal.TargetEffectiveDate != "" && !assistantDateISOYMD(proposal.TargetEffectiveDate) {
		return errAssistantPlanSchemaConstrainedDecodeFailed
	}
	// 读链路 as_of 只允许作为 retrieval 条件存在，不能在 proposal 中反向提升为写侧日期默认值。
	if strings.TrimSpace(retrieval.AsOf) != "" {
		if proposal.EffectiveDate == "" && proposal.TargetEffectiveDate == "" {
			return nil
		}
	}
	return nil
}

func assistantBuildAuthoritativeDryRun(
	svc *assistantConversationService,
	ctx context.Context,
	tenantID string,
	intent assistantIntentSpec,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	retrieval assistantSemanticRetrievalResult,
) assistantDryRunResult {
	dryRun := assistantBuildDryRunWithRetrieval(intent, candidates, resolvedCandidateID, retrieval)
	if svc != nil {
		dryRun = svc.enrichAuthoritativeOrgUnitDryRunWithPolicy(ctx, tenantID, intent, candidates, resolvedCandidateID, dryRun)
	}
	return dryRun
}

func (s *assistantConversationService) assistantAcceptProposal(
	ctx context.Context,
	tenantID string,
	principal Principal,
	userInput string,
	proposal assistantRuntimeProposal,
	routeDecision assistantIntentRouteDecision,
	candidates []assistantCandidate,
	resolvedCandidateID string,
	selectedCandidateID string,
	knowledgeRuntime *assistantKnowledgeRuntime,
	retrieval assistantSemanticRetrievalResult,
	pendingClarification *assistantClarificationDecision,
) (assistantAuthoritativeDecision, error) {
	_ = principal
	proposal = assistantNormalizeRuntimeProposal(proposal)
	if err := assistantValidateProposalWriteDates(proposal, retrieval); err != nil {
		return assistantAuthoritativeDecision{}, err
	}
	intent := assistantProjectIntentRouteDecision(proposal.intentSpec(), routeDecision)
	decision := assistantAuthoritativeDecision{
		Accepted:            true,
		Intent:              intent,
		RouteDecision:       routeDecision,
		Candidates:          append([]assistantCandidate(nil), candidates...),
		ResolvedCandidateID: strings.TrimSpace(resolvedCandidateID),
		SelectedCandidateID: strings.TrimSpace(selectedCandidateID),
	}
	requiresActionSpec := assistantIntentNeedsActionSpec(intent, routeDecision)
	if requiresActionSpec {
		spec, ok := s.lookupActionSpec(intent.Action)
		if !ok {
			decision.FailClosedCode = errAssistantUnsupportedIntent.Error()
			return decision, errAssistantUnsupportedIntent
		}
		if strings.TrimSpace(spec.ID) == "" {
			decision.FailClosedCode = errAssistantActionSpecMissing.Error()
			return decision, errAssistantActionSpecMissing
		}
		decision.ActionSpec = spec
	}
	if decision.SelectedCandidateID != "" && !assistantCandidateExists(decision.Candidates, decision.SelectedCandidateID) {
		decision.FailClosedCode = errAssistantCandidateNotFound.Error()
		return decision, errAssistantCandidateNotFound
	}
	if decision.ResolvedCandidateID != "" {
		resolvedCandidate, ok := assistantFindCandidate(decision.Candidates, decision.ResolvedCandidateID)
		if !ok {
			decision.FailClosedCode = errAssistantCandidateNotFound.Error()
			return decision, errAssistantCandidateNotFound
		}
		decision.ResolvedCandidate = &resolvedCandidate
	}
	dryRun := assistantBuildAuthoritativeDryRun(s, ctx, tenantID, intent, decision.Candidates, decision.ResolvedCandidateID, retrieval)
	decision.Clarification = assistantBuildClarificationDecisionFn(assistantClarificationBuildInput{
		UserInput:            userInput,
		Intent:               intent,
		RouteDecision:        routeDecision,
		DryRun:               dryRun,
		Candidates:           decision.Candidates,
		ResolvedCandidateID:  decision.ResolvedCandidateID,
		SelectedCandidateID:  decision.SelectedCandidateID,
		Runtime:              knowledgeRuntime,
		PendingClarification: pendingClarification,
		ResumeProgress:       false,
	})
	return decision, nil
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
	semanticTurn, err := s.orchestrateSemanticTurn(ctx, tenantID, principal, conversationID, userInput, pendingTurn)
	if err != nil {
		return nil, err
	}
	resolvedIntent := semanticTurn.Resolved
	knowledgeRuntime := semanticTurn.Runtime
	proposal := assistantNormalizeRuntimeProposal(resolvedIntent.Proposal)
	intent := proposal.intentSpec()

	routeDecision, err := assistantBuildIntentRouteDecisionFn(userInput, resolvedIntent, intent, knowledgeRuntime)
	if err != nil {
		return nil, err
	}

	candidates := append([]assistantCandidate(nil), semanticTurn.Candidates...)
	resolvedCandidateID := strings.TrimSpace(semanticTurn.ResolvedCandidateID)
	selectedCandidateID := strings.TrimSpace(semanticTurn.SelectedCandidateID)
	resolutionSource := strings.TrimSpace(semanticTurn.ResolutionSource)
	ambiguityCount := semanticTurn.AmbiguityCount
	confidence := semanticTurn.Confidence

	var pendingClarification *assistantClarificationDecision
	if pendingTurn != nil {
		pendingClarification = pendingTurn.Clarification
	}
	authoritativeDecision, err := s.assistantAcceptProposal(
		ctx,
		tenantID,
		principal,
		userInput,
		proposal,
		routeDecision,
		candidates,
		resolvedCandidateID,
		selectedCandidateID,
		knowledgeRuntime,
		semanticTurn.Retrieval,
		pendingClarification,
	)
	if err != nil {
		return nil, err
	}
	intent = authoritativeDecision.Intent
	candidates = authoritativeDecision.Candidates
	resolvedCandidateID = authoritativeDecision.ResolvedCandidateID
	selectedCandidateID = authoritativeDecision.SelectedCandidateID
	spec := authoritativeDecision.ActionSpec
	specOK := strings.TrimSpace(spec.ID) != ""
	clarification := authoritativeDecision.Clarification
	dryRun := assistantBuildAuthoritativeDryRun(s, ctx, tenantID, intent, candidates, resolvedCandidateID, semanticTurn.Retrieval)
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
