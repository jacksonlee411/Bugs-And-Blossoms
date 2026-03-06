package server

import (
	"context"
	"fmt"
	"strings"
)

const (
	assistantReplyPromptVersionV1 = "assistant.reply.v1"
)

type assistantRenderReplyResponse struct {
	Text               string `json:"text"`
	Kind               string `json:"kind"`
	Stage              string `json:"stage"`
	ReplyModelName     string `json:"reply_model_name"`
	ReplyPromptVersion string `json:"reply_prompt_version"`
	ConversationID     string `json:"conversation_id"`
	TurnID             string `json:"turn_id"`
}

type assistantReplyRenderPrompt struct {
	ConversationID string                     `json:"conversation_id"`
	TurnID         string                     `json:"turn_id,omitempty"`
	Stage          string                     `json:"stage"`
	Kind           string                     `json:"kind"`
	Outcome        string                     `json:"outcome"`
	ErrorCode      string                     `json:"error_code,omitempty"`
	ErrorMessage   string                     `json:"error_message,omitempty"`
	NextAction     string                     `json:"next_action,omitempty"`
	Locale         string                     `json:"locale"`
	FallbackText   string                     `json:"fallback_text,omitempty"`
	Machine        assistantReplyMachineState `json:"machine"`
}

type assistantReplyMachineState struct {
	TurnState         string                 `json:"turn_state,omitempty"`
	IntentAction      string                 `json:"intent_action,omitempty"`
	ParentRefText     string                 `json:"parent_ref_text,omitempty"`
	EntityName        string                 `json:"entity_name,omitempty"`
	EffectiveDate     string                 `json:"effective_date,omitempty"`
	ValidationErrors  []string               `json:"validation_errors,omitempty"`
	DryRunExplain     string                 `json:"dry_run_explain,omitempty"`
	CandidateCount    int                    `json:"candidate_count,omitempty"`
	Candidates        []assistantCandidate   `json:"candidates,omitempty"`
	ResolvedCandidate string                 `json:"resolved_candidate_id,omitempty"`
	CommitResult      *assistantCommitResult `json:"commit_result,omitempty"`
}

type assistantReplyModelResult struct {
	Text           string
	Kind           string
	Stage          string
	ReplyModelName string
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

	outcome := assistantReplyOutcome(req.Outcome, req.ErrorCode)
	stage := assistantReplyStage(req.Stage, outcome, turn)
	kind := assistantReplyKind(req.Kind, stage, outcome)
	locale := assistantReplyLocale(req.Locale)
	fallbackText := assistantReplyFallbackText(req, stage, turn)
	prompt := assistantReplyRenderPrompt{
		ConversationID: strings.TrimSpace(conversation.ConversationID),
		TurnID:         resolvedTurnID,
		Stage:          stage,
		Kind:           kind,
		Outcome:        outcome,
		ErrorCode:      strings.TrimSpace(req.ErrorCode),
		ErrorMessage:   strings.TrimSpace(req.ErrorMessage),
		NextAction:     strings.TrimSpace(req.NextAction),
		Locale:         locale,
		FallbackText:   fallbackText,
		Machine:        assistantReplyMachineFromTurn(turn),
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

	return &assistantRenderReplyResponse{
		Text:               text,
		Kind:               assistantReplyKind(modelResult.Kind, kind, outcome),
		Stage:              assistantReplyStage(modelResult.Stage, outcome, turn),
		ReplyModelName:     replyModelName,
		ReplyPromptVersion: assistantReplyPromptVersionV1,
		ConversationID:     strings.TrimSpace(conversation.ConversationID),
		TurnID:             resolvedTurnID,
	}, nil
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
	machine := assistantReplyMachineState{
		TurnState:         strings.TrimSpace(turn.State),
		IntentAction:      strings.TrimSpace(turn.Intent.Action),
		ParentRefText:     strings.TrimSpace(turn.Intent.ParentRefText),
		EntityName:        strings.TrimSpace(turn.Intent.EntityName),
		EffectiveDate:     strings.TrimSpace(turn.Intent.EffectiveDate),
		ValidationErrors:  append([]string(nil), assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors)...),
		DryRunExplain:     strings.TrimSpace(turn.DryRun.Explain),
		CandidateCount:    len(turn.Candidates),
		Candidates:        append([]assistantCandidate(nil), turn.Candidates...),
		ResolvedCandidate: strings.TrimSpace(turn.ResolvedCandidateID),
		CommitResult:      turn.CommitResult,
	}
	return machine
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
	case "draft", "missing_fields", "candidate_list", "candidate_confirm", "commit_result", "commit_failed":
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
	case "missing_fields", "candidate_list":
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

func assistantReplyFallbackText(req assistantRenderReplyRequest, stage string, turn *assistantTurn) string {
	if text := strings.TrimSpace(req.FallbackText); text != "" {
		return text
	}
	if stage == "commit_failed" {
		if text := strings.TrimSpace(req.ErrorMessage); text != "" {
			return text
		}
		if code := strings.TrimSpace(req.ErrorCode); code != "" {
			return fmt.Sprintf("本次执行失败（%s），请根据提示补充信息后重试。", code)
		}
		return "本次执行失败，请根据提示补充信息后重试。"
	}
	if turn == nil {
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
		return explain
	}
	return "已收到你的请求，我将继续处理。"
}
