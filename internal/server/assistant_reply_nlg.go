package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	assistantReplyPromptVersionV1    = "assistant.reply.v1"
	assistantReplySourceModel        = "model"
	assistantReplySourceGuidancePack = "reply_guidance_pack"
	assistantReplySourceFallback     = "fallback"

	assistantReplyGuidanceKindClarificationRequired = "clarification_required"
	assistantReplyGuidanceKindMissingFields         = "missing_fields"
	assistantReplyGuidanceKindCandidateList         = "candidate_list"
	assistantReplyGuidanceKindCandidateConfirm      = "candidate_confirm"
	assistantReplyGuidanceKindConfirmSummary        = "confirm_summary"
	assistantReplyGuidanceKindCommitSuccess         = "commit_success"
	assistantReplyGuidanceKindCommitFailed          = "commit_failed"
	assistantReplyGuidanceKindTaskWaiting           = "task_waiting"
	assistantReplyGuidanceKindManualTakeover        = "manual_takeover"
	assistantReplyGuidanceKindNonBusinessRoute      = "non_business_route"
)

var assistantReplyTemplateVarWhitelist = map[string]struct{}{
	"missing_fields":     {},
	"candidate_list":     {},
	"candidate_count":    {},
	"selected_candidate": {},
	"summary":            {},
	"effective_date":     {},
	"entity_name":        {},
	"parent_ref_text":    {},
	"error_explanation":  {},
	"next_action":        {},
	"task_status":        {},
}

type assistantRenderReplyResponse struct {
	Text               string `json:"text"`
	Kind               string `json:"kind"`
	Stage              string `json:"stage"`
	ReplyModelName     string `json:"reply_model_name"`
	ReplyPromptVersion string `json:"reply_prompt_version"`
	ReplySource        string `json:"reply_source"`
	UsedFallback       bool   `json:"used_fallback"`
	ConversationID     string `json:"conversation_id"`
	TurnID             string `json:"turn_id"`
}

type assistantReplyRenderPrompt struct {
	ConversationID          string                     `json:"conversation_id"`
	TurnID                  string                     `json:"turn_id,omitempty"`
	Stage                   string                     `json:"stage"`
	Kind                    string                     `json:"kind"`
	ReplyKind               string                     `json:"reply_kind,omitempty"`
	Outcome                 string                     `json:"outcome"`
	ErrorCode               string                     `json:"error_code,omitempty"`
	ErrorMessage            string                     `json:"error_message,omitempty"`
	NextAction              string                     `json:"next_action,omitempty"`
	Locale                  string                     `json:"locale"`
	FallbackText            string                     `json:"fallback_text,omitempty"`
	TemplateID              string                     `json:"template_id,omitempty"`
	ReplyGuidanceVersion    string                     `json:"reply_guidance_version,omitempty"`
	KnowledgeSnapshotDigest string                     `json:"knowledge_snapshot_digest,omitempty"`
	ResolverContractVersion string                     `json:"resolver_contract_version,omitempty"`
	Machine                 assistantReplyMachineState `json:"machine"`
}

type assistantReplyMachineState struct {
	TurnState                     string                 `json:"turn_state,omitempty"`
	TurnPhase                     string                 `json:"turn_phase,omitempty"`
	IntentAction                  string                 `json:"intent_action,omitempty"`
	IntentID                      string                 `json:"intent_id,omitempty"`
	RouteKind                     string                 `json:"route_kind,omitempty"`
	RouteReasonCodes              []string               `json:"route_reason_codes,omitempty"`
	ParentRefText                 string                 `json:"parent_ref_text,omitempty"`
	EntityName                    string                 `json:"entity_name,omitempty"`
	EffectiveDate                 string                 `json:"effective_date,omitempty"`
	ValidationErrors              []string               `json:"validation_errors,omitempty"`
	MissingFields                 []string               `json:"missing_fields,omitempty"`
	DryRunExplain                 string                 `json:"dry_run_explain,omitempty"`
	PendingDraftSummary           string                 `json:"pending_draft_summary,omitempty"`
	CandidateCount                int                    `json:"candidate_count,omitempty"`
	Candidates                    []assistantCandidate   `json:"candidates,omitempty"`
	ResolvedCandidate             string                 `json:"resolved_candidate_id,omitempty"`
	SelectedCandidate             string                 `json:"selected_candidate_id,omitempty"`
	ClarificationKind             string                 `json:"clarification_kind,omitempty"`
	ClarificationPromptTemplateID string                 `json:"clarification_prompt_template_id,omitempty"`
	ClarificationCurrentRound     int                    `json:"clarification_current_round,omitempty"`
	ClarificationReasonCodes      []string               `json:"clarification_reason_codes,omitempty"`
	TaskStatus                    string                 `json:"task_status,omitempty"`
	TaskLastErrorCode             string                 `json:"task_last_error_code,omitempty"`
	CommitReply                   *assistantCommitReply  `json:"commit_reply,omitempty"`
	CommitResult                  *assistantCommitResult `json:"commit_result,omitempty"`
}

type assistantReplyModelResult struct {
	Text           string
	Kind           string
	Stage          string
	ReplyModelName string
	ReplySource    string
	UsedFallback   bool
}

type assistantReplyRealizerInput struct {
	StageHint               string
	ResolvedReplyKind       string
	Locale                  string
	OutcomeHint             string
	ErrorCode               string
	ErrorExplanation        string
	NextAction              string
	RouteDecision           assistantIntentRouteDecision
	Clarification           *assistantClarificationDecision
	Machine                 assistantReplyMachineState
	ReplyGuidanceVersion    string
	KnowledgeSnapshotDigest string
	ResolverContractVersion string
}

type assistantReplyGuidanceSelection struct {
	ReplyKind        string
	TemplateID       string
	TemplateText     string
	Locale           string
	KnowledgeVersion string
}

type assistantReplyRealizerOutput struct {
	Text                 string
	ReplyKind            string
	Kind                 string
	Stage                string
	ReplySource          string
	UsedFallback         bool
	TemplateID           string
	ReplyGuidanceVersion string
}

var assistantRenderReplyWithModelFn = assistantRenderReplyWithModel

func (s *assistantConversationService) renderTurnReply(ctx context.Context, tenantID string, principal Principal, conversationID string, turnID string, req assistantRenderReplyRequest) (*assistantRenderReplyResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	conversation, err := s.getConversation(tenantID, principal.ID, conversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(principal.RoleSlug) != "" && strings.TrimSpace(conversation.ActorRole) != "" && strings.TrimSpace(principal.RoleSlug) != strings.TrimSpace(conversation.ActorRole) {
		return nil, errAssistantConversationForbidden
	}

	resolvedTurnID := strings.TrimSpace(turnID)
	turn := assistantFindTurnForReply(conversation, resolvedTurnID)
	if turn == nil && resolvedTurnID == "" {
		turn = latestTurn(conversation)
		if turn != nil {
			resolvedTurnID = strings.TrimSpace(turn.TurnID)
		}
	}
	if turn == nil && !req.AllowMissingTurn {
		return nil, errAssistantTurnNotFound
	}

	locale := assistantReplyLocale(req.Locale)
	runtime, runtimeErr := s.ensureKnowledgeRuntime()
	if runtimeErr != nil {
		runtime = nil
	}

	realizerInput := assistantBuildReplyRealizerInput(req, turn, locale)
	if runtime != nil {
		if strings.TrimSpace(realizerInput.ReplyGuidanceVersion) == "" {
			realizerInput.ReplyGuidanceVersion = strings.TrimSpace(runtime.ReplyGuidanceVersion)
		}
		if strings.TrimSpace(realizerInput.ResolverContractVersion) == "" {
			realizerInput.ResolverContractVersion = strings.TrimSpace(runtime.ResolverContractVersion)
		}
	}
	realizerOutput := assistantRealizeReply(realizerInput, runtime, req, turn)

	prompt := assistantReplyRenderPrompt{
		ConversationID:          strings.TrimSpace(conversation.ConversationID),
		TurnID:                  resolvedTurnID,
		Stage:                   strings.TrimSpace(realizerOutput.Stage),
		Kind:                    strings.TrimSpace(realizerOutput.Kind),
		ReplyKind:               strings.TrimSpace(realizerOutput.ReplyKind),
		Outcome:                 strings.TrimSpace(realizerInput.OutcomeHint),
		ErrorCode:               strings.TrimSpace(realizerInput.ErrorCode),
		ErrorMessage:            strings.TrimSpace(realizerInput.ErrorExplanation),
		NextAction:              strings.TrimSpace(realizerInput.NextAction),
		Locale:                  locale,
		FallbackText:            strings.TrimSpace(realizerOutput.Text),
		TemplateID:              strings.TrimSpace(realizerOutput.TemplateID),
		ReplyGuidanceVersion:    strings.TrimSpace(realizerOutput.ReplyGuidanceVersion),
		KnowledgeSnapshotDigest: strings.TrimSpace(realizerInput.KnowledgeSnapshotDigest),
		ResolverContractVersion: strings.TrimSpace(realizerInput.ResolverContractVersion),
		Machine:                 realizerInput.Machine,
	}

	modelResult, err := assistantRenderReplyWithModelFn(ctx, s, prompt)
	if err != nil {
		return nil, err
	}
	replyModelName := strings.TrimSpace(modelResult.ReplyModelName)
	if replyModelName != assistantReplyTargetModelName {
		return nil, errAssistantReplyModelTargetMismatch
	}
	text := strings.TrimSpace(modelResult.Text)
	if text == "" {
		return nil, errAssistantReplyRenderFailed
	}

	replySource := strings.TrimSpace(modelResult.ReplySource)
	if replySource == "" {
		replySource = assistantReplySourceModel
	}
	reply := &assistantRenderReplyResponse{
		Text:               text,
		Kind:               assistantNormalizeReplyRenderKind(modelResult.Kind, realizerOutput.Kind),
		Stage:              assistantNormalizeReplyRenderStage(modelResult.Stage, realizerOutput.Stage),
		ReplyModelName:     replyModelName,
		ReplyPromptVersion: assistantReplyPromptVersionV1,
		ReplySource:        replySource,
		UsedFallback:       realizerOutput.UsedFallback || modelResult.UsedFallback,
		ConversationID:     strings.TrimSpace(conversation.ConversationID),
		TurnID:             resolvedTurnID,
	}
	if turn != nil {
		assistantSetReplySnapshot(turn, reply, req.ErrorCode)
		s.persistRenderedReplySnapshot(ctx, tenantID, conversationID, turn)
	}
	s.persistRenderedReply(tenantID, principal.ID, conversationID, resolvedTurnID, reply)
	return reply, nil
}

func assistantRenderReplyWithModel(ctx context.Context, svc *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
	if svc == nil {
		return assistantReplyModelResult{}, errAssistantServiceMissing
	}
	if svc.gatewayErr != nil {
		return assistantReplyModelResult{}, svc.gatewayErr
	}
	if svc.modelGateway == nil {
		return assistantReplyModelResult{}, errAssistantModelProviderUnavailable
	}
	return svc.modelGateway.RenderReply(ctx, prompt)
}

func (s *assistantConversationService) persistRenderedReplySnapshot(ctx context.Context, tenantID string, conversationID string, turn *assistantTurn) {
	if s == nil || s.pool == nil || turn == nil {
		return
	}
	tx, err := s.beginAssistantTx(ctx, tenantID)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	assistantRefreshTurnDerivedFields(turn)
	_, err = tx.Exec(ctx, `
UPDATE iam.assistant_turns
SET phase = $4,
    pending_draft_summary = NULLIF($5, ''),
    missing_fields = $6::jsonb,
    candidate_options = $7::jsonb,
    selected_candidate_id = NULLIF($8, ''),
    commit_reply = $9::jsonb,
    error_code = NULLIF($10, ''),
    updated_at = $11
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND turn_id = $3
`, tenantID, conversationID, turn.TurnID, turn.Phase, turn.PendingDraftSummary, assistantMissingFieldsJSON(turn), assistantCandidateOptionsJSON(turn), turn.SelectedCandidateID, assistantCommitReplyJSON(turn), turn.ErrorCode, turn.UpdatedAt)
	if err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func (s *assistantConversationService) persistRenderedReply(tenantID string, actorID string, conversationID string, turnID string, reply *assistantRenderReplyResponse) {
	if s == nil || reply == nil {
		return
	}
	conversationID = strings.TrimSpace(conversationID)
	turnID = strings.TrimSpace(turnID)
	if conversationID == "" || turnID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.byID[conversationID]
	if !ok || conversation == nil {
		return
	}
	if conversation.TenantID != tenantID || conversation.ActorID != actorID {
		return
	}
	turn := assistantFindTurnForReply(conversation, turnID)
	if turn == nil {
		return
	}
	assistantSetReplySnapshot(turn, reply, turn.ErrorCode)
	assistantRefreshConversationDerivedFields(conversation)
}

func assistantFindTurnForReply(conversation *assistantConversation, turnID string) *assistantTurn {
	if conversation == nil {
		return nil
	}
	for _, turn := range conversation.Turns {
		if turn != nil && strings.TrimSpace(turn.TurnID) == strings.TrimSpace(turnID) {
			return turn
		}
	}
	return nil
}

func assistantReplyMachineFromTurn(turn *assistantTurn) assistantReplyMachineState {
	if turn == nil {
		return assistantReplyMachineState{}
	}
	selectedCandidate := strings.TrimSpace(turn.SelectedCandidateID)
	if selectedCandidate == "" {
		selectedCandidate = strings.TrimSpace(turn.ResolvedCandidateID)
	}
	machine := assistantReplyMachineState{
		TurnState:           strings.TrimSpace(turn.State),
		TurnPhase:           strings.TrimSpace(turn.Phase),
		IntentAction:        strings.TrimSpace(turn.Intent.Action),
		IntentID:            strings.TrimSpace(turn.RouteDecision.IntentID),
		RouteKind:           strings.TrimSpace(turn.RouteDecision.RouteKind),
		RouteReasonCodes:    append([]string(nil), assistantNormalizeRouteStringSlice(turn.RouteDecision.ReasonCodes)...),
		ParentRefText:       strings.TrimSpace(turn.Intent.ParentRefText),
		EntityName:          strings.TrimSpace(turn.Intent.EntityName),
		EffectiveDate:       strings.TrimSpace(turn.Intent.EffectiveDate),
		ValidationErrors:    append([]string(nil), assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors)...),
		MissingFields:       append([]string(nil), assistantTurnMissingFields(turn)...),
		DryRunExplain:       strings.TrimSpace(turn.DryRun.Explain),
		PendingDraftSummary: strings.TrimSpace(turn.PendingDraftSummary),
		CandidateCount:      len(turn.Candidates),
		Candidates:          append([]assistantCandidate(nil), turn.Candidates...),
		ResolvedCandidate:   strings.TrimSpace(turn.ResolvedCandidateID),
		SelectedCandidate:   selectedCandidate,
		CommitResult:        turn.CommitResult,
		CommitReply:         turn.CommitReply,
	}
	if clarification := turn.Clarification; clarification != nil {
		machine.ClarificationKind = strings.TrimSpace(clarification.ClarificationKind)
		machine.ClarificationPromptTemplateID = strings.TrimSpace(clarification.PromptTemplateID)
		machine.ClarificationCurrentRound = clarification.CurrentRound
		machine.ClarificationReasonCodes = append([]string(nil), assistantNormalizeRouteStringSlice(clarification.ReasonCodes)...)
	}
	return machine
}

func assistantReplyTaskStatusFromStageHint(stageHint string) string {
	switch strings.TrimSpace(stageHint) {
	case assistantTaskStatusQueued, assistantTaskStatusRunning, assistantTaskStatusManualTakeoverNeeded:
		return strings.TrimSpace(stageHint)
	default:
		return ""
	}
}

func assistantBuildReplyRealizerInput(req assistantRenderReplyRequest, turn *assistantTurn, locale string) assistantReplyRealizerInput {
	machine := assistantReplyMachineFromTurn(turn)
	if machine.TaskStatus == "" {
		machine.TaskStatus = assistantReplyTaskStatusFromStageHint(req.Stage)
	}
	if machine.TaskLastErrorCode == "" {
		machine.TaskLastErrorCode = strings.TrimSpace(req.ErrorCode)
	}
	input := assistantReplyRealizerInput{
		StageHint:         strings.TrimSpace(req.Stage),
		Locale:            assistantReplyLocale(locale),
		OutcomeHint:       assistantReplyOutcome(req.Outcome, req.ErrorCode),
		ErrorCode:         strings.TrimSpace(req.ErrorCode),
		ErrorExplanation:  strings.TrimSpace(req.ErrorMessage),
		NextAction:        strings.TrimSpace(req.NextAction),
		Machine:           machine,
		ResolvedReplyKind: "",
	}
	if turn != nil {
		input.RouteDecision = turn.RouteDecision
		if turn.Clarification != nil {
			copyClarification := *turn.Clarification
			input.Clarification = &copyClarification
		}
		input.ReplyGuidanceVersion = strings.TrimSpace(turn.Plan.ReplyGuidanceVersion)
		input.KnowledgeSnapshotDigest = strings.TrimSpace(turn.Plan.KnowledgeSnapshotDigest)
		input.ResolverContractVersion = strings.TrimSpace(turn.Plan.ResolverContractVersion)
	}
	input.ResolvedReplyKind = assistantResolveReplyGuidanceKind(input)
	return input
}

func assistantResolveReplyGuidanceKind(input assistantReplyRealizerInput) string {
	if strings.TrimSpace(input.Machine.TaskStatus) == assistantTaskStatusManualTakeoverNeeded {
		return assistantReplyGuidanceKindManualTakeover
	}
	if strings.TrimSpace(input.Machine.TaskStatus) == assistantTaskStatusQueued || strings.TrimSpace(input.Machine.TaskStatus) == assistantTaskStatusRunning {
		return assistantReplyGuidanceKindTaskWaiting
	}
	if strings.TrimSpace(input.Machine.RouteKind) != "" && strings.TrimSpace(input.Machine.RouteKind) != assistantRouteKindBusinessAction {
		return assistantReplyGuidanceKindNonBusinessRoute
	}
	if input.Clarification != nil && strings.TrimSpace(input.Clarification.Status) == assistantClarificationStatusOpen {
		switch strings.TrimSpace(input.Clarification.ClarificationKind) {
		case assistantClarificationKindMissingSlots:
			return assistantReplyGuidanceKindMissingFields
		case assistantClarificationKindCandidatePick:
			return assistantReplyGuidanceKindCandidateList
		case assistantClarificationKindCandidateConfirm:
			return assistantReplyGuidanceKindCandidateConfirm
		default:
			return assistantReplyGuidanceKindClarificationRequired
		}
	}
	if input.RouteDecision.ClarificationRequired {
		return assistantReplyGuidanceKindClarificationRequired
	}
	if strings.TrimSpace(input.ErrorCode) != "" || strings.TrimSpace(input.OutcomeHint) == "failure" || strings.TrimSpace(input.Machine.TaskLastErrorCode) != "" {
		return assistantReplyGuidanceKindCommitFailed
	}
	if input.Machine.CommitResult != nil || (input.Machine.CommitReply != nil && strings.TrimSpace(input.Machine.CommitReply.Outcome) == "success") {
		return assistantReplyGuidanceKindCommitSuccess
	}
	if strings.TrimSpace(input.Machine.TurnPhase) == assistantPhaseAwaitCommitConfirm {
		return assistantReplyGuidanceKindConfirmSummary
	}
	if strings.TrimSpace(input.Machine.TurnPhase) == assistantPhaseAwaitCandidateConfirm {
		if strings.TrimSpace(input.Machine.SelectedCandidate) == "" {
			return assistantReplyGuidanceKindCandidateList
		}
		return assistantReplyGuidanceKindCandidateConfirm
	}
	if strings.TrimSpace(input.Machine.TurnPhase) == assistantPhaseAwaitMissingFields {
		return assistantReplyGuidanceKindMissingFields
	}
	if len(input.Machine.MissingFields) > 0 || len(input.Machine.ValidationErrors) > 0 {
		return assistantReplyGuidanceKindMissingFields
	}
	if input.Machine.CandidateCount > 1 {
		if strings.TrimSpace(input.Machine.SelectedCandidate) == "" {
			return assistantReplyGuidanceKindCandidateList
		}
		return assistantReplyGuidanceKindCandidateConfirm
	}
	if hint := assistantReplyGuidanceKindFromStageHint(input.StageHint, input.Machine); hint != "" {
		return hint
	}
	return assistantReplyGuidanceKindConfirmSummary
}

func assistantReplyGuidanceKindFromStageHint(stageHint string, machine assistantReplyMachineState) string {
	switch strings.TrimSpace(stageHint) {
	case "await_clarification":
		return assistantReplyGuidanceKindClarificationRequired
	case assistantPhaseAwaitMissingFields, "missing_fields":
		return assistantReplyGuidanceKindMissingFields
	case "candidate_list", assistantPhaseAwaitCandidatePick:
		return assistantReplyGuidanceKindCandidateList
	case "candidate_confirm":
		return assistantReplyGuidanceKindCandidateConfirm
	case assistantPhaseAwaitCandidateConfirm:
		if strings.TrimSpace(machine.SelectedCandidate) == "" {
			return assistantReplyGuidanceKindCandidateList
		}
		return assistantReplyGuidanceKindCandidateConfirm
	case assistantPhaseAwaitCommitConfirm:
		return assistantReplyGuidanceKindConfirmSummary
	case "commit_result":
		return assistantReplyGuidanceKindCommitSuccess
	case "commit_failed":
		return assistantReplyGuidanceKindCommitFailed
	case assistantTaskStatusQueued, assistantTaskStatusRunning:
		return assistantReplyGuidanceKindTaskWaiting
	case assistantTaskStatusManualTakeoverNeeded:
		return assistantReplyGuidanceKindManualTakeover
	case "non_business_route":
		return assistantReplyGuidanceKindNonBusinessRoute
	default:
		return ""
	}
}

func assistantReplyStageFromGuidanceKind(replyKind string) string {
	switch strings.TrimSpace(replyKind) {
	case assistantReplyGuidanceKindClarificationRequired:
		return "await_clarification"
	case assistantReplyGuidanceKindMissingFields:
		return "missing_fields"
	case assistantReplyGuidanceKindCandidateList:
		return "candidate_list"
	case assistantReplyGuidanceKindCandidateConfirm:
		return "candidate_confirm"
	case assistantReplyGuidanceKindConfirmSummary:
		return "draft"
	case assistantReplyGuidanceKindCommitSuccess:
		return "commit_result"
	case assistantReplyGuidanceKindCommitFailed:
		return "commit_failed"
	case assistantReplyGuidanceKindTaskWaiting:
		return assistantReplyGuidanceKindTaskWaiting
	case assistantReplyGuidanceKindManualTakeover:
		return assistantReplyGuidanceKindManualTakeover
	case assistantReplyGuidanceKindNonBusinessRoute:
		return assistantReplyGuidanceKindNonBusinessRoute
	default:
		return "draft"
	}
}

func assistantReplyRenderKindFromGuidanceKind(replyKind string) string {
	switch strings.TrimSpace(replyKind) {
	case assistantReplyGuidanceKindCommitSuccess:
		return "success"
	case assistantReplyGuidanceKindCommitFailed:
		return "error"
	case assistantReplyGuidanceKindMissingFields, assistantReplyGuidanceKindCandidateList, assistantReplyGuidanceKindCandidateConfirm, assistantReplyGuidanceKindClarificationRequired, assistantReplyGuidanceKindTaskWaiting, assistantReplyGuidanceKindManualTakeover:
		return "warning"
	default:
		return "info"
	}
}

func assistantSelectReplyGuidance(input assistantReplyRealizerInput, runtime *assistantKnowledgeRuntime) (assistantReplyGuidanceSelection, bool) {
	if runtime == nil {
		return assistantReplyGuidanceSelection{}, false
	}
	pack, ok := runtime.findReplyGuidance(input.ResolvedReplyKind, input.Locale, input.ErrorCode)
	if !ok || len(pack.GuidanceTemplates) == 0 {
		return assistantReplyGuidanceSelection{}, false
	}
	template := pack.GuidanceTemplates[0]
	return assistantReplyGuidanceSelection{
		ReplyKind:        strings.TrimSpace(input.ResolvedReplyKind),
		TemplateID:       strings.TrimSpace(template.TemplateID),
		TemplateText:     strings.TrimSpace(template.Text),
		Locale:           strings.TrimSpace(pack.Locale),
		KnowledgeVersion: strings.TrimSpace(pack.KnowledgeVersion),
	}, true
}

func assistantReplyGenericFailureText(locale string) string {
	if strings.TrimSpace(strings.ToLower(locale)) == "en" {
		return "The request could not be completed. Please adjust the input and try again."
	}
	return "本次请求未能完成，请根据提示调整后重试。"
}

func assistantReplyCandidateLabel(machine assistantReplyMachineState) string {
	targetID := strings.TrimSpace(machine.SelectedCandidate)
	if targetID == "" {
		targetID = strings.TrimSpace(machine.ResolvedCandidate)
	}
	if targetID == "" {
		return ""
	}
	for _, candidate := range machine.Candidates {
		if strings.TrimSpace(candidate.CandidateID) != targetID {
			continue
		}
		name := strings.TrimSpace(candidate.Name)
		if name == "" {
			name = targetID
		}
		code := strings.TrimSpace(candidate.CandidateCode)
		if code == "" {
			return name
		}
		return name + " / " + code
	}
	return targetID
}

func assistantReplyCandidateListText(machine assistantReplyMachineState) string {
	if len(machine.Candidates) == 0 {
		return ""
	}
	lines := make([]string, 0, len(machine.Candidates))
	for idx, candidate := range machine.Candidates {
		name := strings.TrimSpace(candidate.Name)
		if name == "" {
			name = strings.TrimSpace(candidate.CandidateID)
		}
		code := strings.TrimSpace(candidate.CandidateCode)
		path := strings.TrimSpace(candidate.Path)
		label := name
		if code != "" {
			label += " / " + code
		}
		if path != "" {
			label += " (" + path + ")"
		}
		lines = append(lines, fmt.Sprintf("%d. %s", idx+1, label))
	}
	return strings.Join(lines, "\n")
}

func assistantReplyMissingFieldsText(machine assistantReplyMachineState, locale string) string {
	if len(machine.MissingFields) > 0 {
		separator := "、"
		if strings.TrimSpace(strings.ToLower(locale)) == "en" {
			separator = ", "
		}
		return strings.Join(machine.MissingFields, separator)
	}
	if len(machine.ValidationErrors) > 0 {
		return strings.TrimSpace(assistantDryRunValidationExplain(machine.ValidationErrors))
	}
	return ""
}

func assistantBuildReplyTemplateVariables(input assistantReplyRealizerInput) map[string]string {
	machine := input.Machine
	errorExplanation := assistantSanitizeUserFacingReplyText(strings.TrimSpace(input.ErrorExplanation), input.Locale)
	if errorExplanation == "" && strings.TrimSpace(input.ErrorCode) != "" {
		errorExplanation = assistantReplyGenericFailureText(input.Locale)
	}
	summary := strings.TrimSpace(machine.PendingDraftSummary)
	if summary == "" {
		summary = strings.TrimSpace(machine.DryRunExplain)
	}
	return map[string]string{
		"missing_fields":     strings.TrimSpace(assistantReplyMissingFieldsText(machine, input.Locale)),
		"candidate_list":     strings.TrimSpace(assistantReplyCandidateListText(machine)),
		"candidate_count":    strconv.Itoa(machine.CandidateCount),
		"selected_candidate": strings.TrimSpace(assistantReplyCandidateLabel(machine)),
		"summary":            summary,
		"effective_date":     strings.TrimSpace(machine.EffectiveDate),
		"entity_name":        strings.TrimSpace(machine.EntityName),
		"parent_ref_text":    strings.TrimSpace(machine.ParentRefText),
		"error_explanation":  errorExplanation,
		"next_action":        strings.TrimSpace(input.NextAction),
		"task_status":        strings.TrimSpace(machine.TaskStatus),
	}
}

func assistantRenderReplyGuidanceTemplate(templateText string, variables map[string]string) (string, error) {
	templateText = strings.TrimSpace(templateText)
	if templateText == "" {
		return "", fmt.Errorf("reply guidance template text required")
	}
	if variables == nil {
		variables = map[string]string{}
	}
	var builder strings.Builder
	for idx := 0; idx < len(templateText); {
		if templateText[idx] != '{' {
			builder.WriteByte(templateText[idx])
			idx++
			continue
		}
		end := strings.IndexByte(templateText[idx+1:], '}')
		if end < 0 {
			builder.WriteByte(templateText[idx])
			idx++
			continue
		}
		end += idx + 1
		key := strings.TrimSpace(templateText[idx+1 : end])
		if _, allowed := assistantReplyTemplateVarWhitelist[key]; !allowed {
			return "", fmt.Errorf("template variable not allowed %s", key)
		}
		value := strings.TrimSpace(variables[key])
		if value == "" {
			return "", fmt.Errorf("template variable missing %s", key)
		}
		builder.WriteString(value)
		idx = end + 1
	}
	text := strings.TrimSpace(builder.String())
	return text, nil
}

func assistantRealizeReply(input assistantReplyRealizerInput, runtime *assistantKnowledgeRuntime, req assistantRenderReplyRequest, turn *assistantTurn) assistantReplyRealizerOutput {
	replyKind := strings.TrimSpace(input.ResolvedReplyKind)
	if replyKind == "" {
		replyKind = assistantReplyGuidanceKindFromStageHint(input.StageHint, input.Machine)
	}
	if replyKind == "" {
		replyKind = assistantReplyGuidanceKindConfirmSummary
	}
	output := assistantReplyRealizerOutput{
		ReplyKind:            replyKind,
		Kind:                 assistantReplyRenderKindFromGuidanceKind(replyKind),
		Stage:                assistantReplyStageFromGuidanceKind(replyKind),
		ReplySource:          assistantReplySourceGuidancePack,
		ReplyGuidanceVersion: strings.TrimSpace(input.ReplyGuidanceVersion),
	}

	selection, selected := assistantSelectReplyGuidance(input, runtime)
	if selected {
		text, err := assistantRenderReplyGuidanceTemplate(selection.TemplateText, assistantBuildReplyTemplateVariables(input))
		if err == nil {
			output.Text = assistantSanitizeUserFacingReplyText(text, input.Locale)
			output.TemplateID = selection.TemplateID
			if output.Text != "" {
				return output
			}
		}
	}

	fallbackText := assistantReplyFallbackText(req, output.Stage, turn, input.Locale)
	output.Text = strings.TrimSpace(fallbackText)
	output.ReplySource = assistantReplySourceFallback
	output.UsedFallback = true
	return output
}

func assistantNormalizeReplyRenderKind(raw string, fallback string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	switch normalized {
	case "info", "warning", "success", "error":
		return normalized
	default:
		return assistantReplyKind(fallback, "draft", "success")
	}
}

func assistantValidReplyStageValue(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "draft", "missing_fields", "candidate_list", "candidate_confirm", "commit_result", "commit_failed", "await_clarification", assistantReplyGuidanceKindTaskWaiting, assistantReplyGuidanceKindManualTakeover, assistantReplyGuidanceKindNonBusinessRoute:
		return true
	default:
		return false
	}
}

func assistantNormalizeReplyRenderStage(raw string, fallback string) string {
	stage := strings.TrimSpace(raw)
	if assistantValidReplyStageValue(stage) {
		return stage
	}
	if assistantValidReplyStageValue(fallback) {
		return strings.TrimSpace(fallback)
	}
	return "draft"
}

func assistantReplyOutcome(raw string, errorCode string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "success" || normalized == "failure" {
		return normalized
	}
	if strings.TrimSpace(errorCode) != "" {
		return "failure"
	}
	return "success"
}

func assistantReplyStage(raw string, outcome string, turn *assistantTurn) string {
	normalized := strings.TrimSpace(raw)
	switch normalized {
	case "draft", "missing_fields", "candidate_list", "candidate_confirm", "commit_result", "commit_failed", "await_clarification", assistantReplyGuidanceKindTaskWaiting, assistantReplyGuidanceKindManualTakeover, assistantReplyGuidanceKindNonBusinessRoute:
		return normalized
	}
	if outcome == "failure" {
		return "commit_failed"
	}
	if turn != nil && turn.CommitResult != nil {
		return "commit_result"
	}
	if turn != nil {
		validationErrors := assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors)
		if len(validationErrors) > 0 {
			for _, code := range validationErrors {
				if code == "candidate_confirmation_required" {
					return "candidate_list"
				}
			}
			return "missing_fields"
		}
		if len(turn.Candidates) > 1 && strings.TrimSpace(turn.ResolvedCandidateID) == "" {
			return "candidate_list"
		}
		if len(turn.Candidates) > 1 && strings.TrimSpace(turn.ResolvedCandidateID) != "" {
			return "candidate_confirm"
		}
	}
	return "draft"
}

func assistantReplyKind(raw string, stage string, outcome string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	switch normalized {
	case "info", "warning", "success", "error":
		return normalized
	}
	if outcome == "failure" {
		return "error"
	}
	switch stage {
	case "missing_fields", "candidate_list", "await_clarification", assistantReplyGuidanceKindTaskWaiting, assistantReplyGuidanceKindManualTakeover:
		return "warning"
	case "commit_result":
		return "success"
	default:
		return "info"
	}
}

func assistantReplyLocale(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "en" {
		return "en"
	}
	return "zh"
}

func assistantReplyFallbackText(req assistantRenderReplyRequest, stage string, turn *assistantTurn, locale string) string {
	if text := assistantSanitizeUserFacingReplyText(strings.TrimSpace(req.FallbackText), locale); text != "" {
		return text
	}
	if stage == "commit_failed" {
		if text := assistantSanitizeUserFacingReplyText(strings.TrimSpace(req.ErrorMessage), locale); text != "" {
			return text
		}
		if strings.TrimSpace(req.ErrorCode) != "" {
			return assistantReplyGenericFailureText(locale)
		}
		return assistantReplyGenericFailureText(locale)
	}
	if stage == assistantReplyGuidanceKindTaskWaiting {
		if locale == "en" {
			return "The task has been queued and is still running. I will continue to track the progress."
		}
		return "任务已进入队列并正在处理中，我会继续跟踪进展。"
	}
	if stage == assistantReplyGuidanceKindManualTakeover {
		if locale == "en" {
			return "Manual intervention is required. Please provide clearer instructions or contact an administrator."
		}
		return "当前需要人工接管。请补充更明确的信息，或联系管理员处理。"
	}
	if stage == assistantReplyGuidanceKindNonBusinessRoute {
		if locale == "en" {
			return "This is a non-business request and will not trigger a business commit."
		}
		return "这是非业务动作请求，不会触发业务提交。"
	}
	if stage == "await_clarification" {
		if locale == "en" {
			return "More clarification is required before proceeding."
		}
		return "继续前需要先澄清关键信息。"
	}
	if turn == nil {
		if locale == "en" {
			return "Your request was received. I will continue processing."
		}
		return "已收到你的请求，我将继续处理。"
	}
	if stage == "commit_result" && turn.CommitResult != nil {
		return fmt.Sprintf(
			"已提交：org_code=%s / parent=%s / effective_date=%s",
			strings.TrimSpace(turn.CommitResult.OrgCode),
			strings.TrimSpace(turn.CommitResult.ParentOrgCode),
			strings.TrimSpace(turn.CommitResult.EffectiveDate),
		)
	}
	if stage == "candidate_list" {
		if len(turn.Candidates) == 0 {
			if locale == "en" {
				return "Multiple parent candidates were detected. Please reply with the candidate index or code before confirming."
			}
			return "检测到多个候选父组织，请回复候选编号或编码后再确认执行。"
		}
		lines := make([]string, 0, len(turn.Candidates))
		for idx, candidate := range turn.Candidates {
			name := strings.TrimSpace(candidate.Name)
			if name == "" {
				name = strings.TrimSpace(candidate.CandidateID)
			}
			code := strings.TrimSpace(candidate.CandidateCode)
			path := strings.TrimSpace(candidate.Path)
			label := name
			if code != "" {
				label += " / " + code
			}
			if path != "" {
				label += " (" + path + ")"
			}
			lines = append(lines, fmt.Sprintf("%d. %s", idx+1, label))
		}
		return "检测到多个上级组织候选，请在对话中回复候选编号或编码：\n" + strings.Join(lines, "\n")
	}
	if stage == "missing_fields" {
		validationErrors := assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors)
		if len(validationErrors) > 0 {
			return assistantDryRunValidationExplain(validationErrors)
		}
	}
	if explain := strings.TrimSpace(turn.DryRun.Explain); explain != "" {
		return assistantSanitizeUserFacingReplyText(explain, locale)
	}
	if locale == "en" {
		return "Your request was received. I will continue processing."
	}
	return "已收到你的请求，我将继续处理。"
}

func assistantSanitizeUserFacingReplyText(text string, locale string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if !assistantReplyContainsTechnicalSignal(trimmed) {
		return trimmed
	}
	if strings.TrimSpace(strings.ToLower(locale)) == "en" {
		return "The request could not be completed. Please adjust the input and try again."
	}
	return "本次请求未能完成，请根据提示调整后重试。"
}

func assistantReplyContainsTechnicalSignal(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if isStableDBCode(strings.ToUpper(trimmed)) {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, token := range []string{
		"ai_",
		"assistant_",
		"schema decode",
		"strict constraints",
		"contract version",
		"determinism",
		"boundary violation",
		"runtime config",
		"model provider",
		"idempotency",
		"trace_id",
		"request_id",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		return r != '_'
	})
	for _, part := range parts {
		if strings.Count(part, "_") >= 2 && len(part) >= 12 {
			return true
		}
	}
	return false
}
