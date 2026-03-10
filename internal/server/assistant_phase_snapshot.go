package server

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	assistantPhaseIdle                  = "idle"
	assistantPhaseAwaitMissingFields    = "await_missing_fields"
	assistantPhaseAwaitCandidatePick    = "await_candidate_pick"
	assistantPhaseAwaitCandidateConfirm = "await_candidate_confirm"
	assistantPhaseAwaitCommitConfirm    = "await_commit_confirm"
	assistantPhaseCommitting            = "committing"
	assistantPhaseCommitted             = "committed"
	assistantPhaseFailed                = "failed"
	assistantPhaseCanceled              = "canceled"
	assistantPhaseExpired               = "expired"
)

type assistantCommitReply struct {
	Outcome   string `json:"outcome"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

func assistantRefreshConversationDerivedFields(conversation *assistantConversation) {
	if conversation == nil {
		return
	}
	for _, turn := range conversation.Turns {
		assistantRefreshTurnDerivedFields(turn)
	}
	for index := range conversation.Transitions {
		assistantRefreshTransitionDerivedFields(conversation, &conversation.Transitions[index])
	}
	currentPhase := strings.TrimSpace(conversation.CurrentPhase)
	if turn := latestTurn(conversation); turn != nil && strings.TrimSpace(turn.Phase) != "" {
		currentPhase = strings.TrimSpace(turn.Phase)
	} else if currentPhase == "" {
		currentPhase = assistantConversationPhaseFromLegacyState(conversation.State)
	}
	conversation.CurrentPhase = currentPhase
}

func assistantRefreshTurnDerivedFields(turn *assistantTurn) {
	if turn == nil {
		return
	}
	turn.MissingFields = assistantTurnMissingFields(turn)
	turn.SelectedCandidateID = assistantTurnSelectedCandidateID(turn)
	turn.PendingDraftSummary = assistantTurnPendingDraftSummary(turn)
	turn.CommitReply = assistantTurnCommitReply(turn)
	if strings.TrimSpace(turn.ErrorCode) == "" && turn.CommitReply != nil {
		turn.ErrorCode = strings.TrimSpace(turn.CommitReply.ErrorCode)
	}
	turn.Phase = assistantTurnPhase(turn)
}

func assistantRefreshTransitionDerivedFields(conversation *assistantConversation, transition *assistantStateTransition) {
	if transition == nil {
		return
	}
	turnPhase := ""
	if conversation != nil && strings.TrimSpace(transition.TurnID) != "" {
		if turn := assistantLookupTurn(conversation, transition.TurnID); turn != nil {
			turnPhase = strings.TrimSpace(turn.Phase)
		}
	}
	if strings.TrimSpace(transition.FromPhase) == "" {
		transition.FromPhase = assistantTransitionPhaseValue(strings.TrimSpace(transition.FromState), strings.TrimSpace(transition.ReasonCode), turnPhase, true)
	}
	if strings.TrimSpace(transition.ToPhase) == "" {
		transition.ToPhase = assistantTransitionPhaseValue(strings.TrimSpace(transition.ToState), strings.TrimSpace(transition.ReasonCode), turnPhase, false)
	}
}

func assistantConversationPhaseFromLegacyState(state string) string {
	switch strings.TrimSpace(state) {
	case assistantStateCommitted:
		return assistantPhaseCommitted
	case assistantStateCanceled:
		return assistantPhaseCanceled
	case assistantStateExpired:
		return assistantPhaseExpired
	default:
		return assistantPhaseIdle
	}
}

func assistantTurnPhase(turn *assistantTurn) string {
	if turn == nil {
		return assistantPhaseIdle
	}
	state := strings.TrimSpace(turn.State)
	switch state {
	case assistantStateCommitted:
		return assistantPhaseCommitted
	case assistantStateCanceled:
		return assistantPhaseCanceled
	case assistantStateExpired:
		return assistantPhaseExpired
	case assistantStateConfirmed:
		return assistantPhaseAwaitCommitConfirm
	}
	if strings.TrimSpace(turn.ErrorCode) != "" {
		return assistantPhaseFailed
	}
	if len(assistantTurnMissingFields(turn)) > 0 {
		return assistantPhaseAwaitMissingFields
	}
	if assistantTurnHasValidationCode(turn, "candidate_confirmation_required") {
		if strings.TrimSpace(assistantTurnSelectedCandidateID(turn)) != "" && state == assistantStateValidated {
			return assistantPhaseAwaitCandidateConfirm
		}
		if strings.TrimSpace(assistantTurnSelectedCandidateID(turn)) == "" {
			return assistantPhaseAwaitCandidatePick
		}
	}
	if strings.TrimSpace(assistantTurnPendingDraftSummary(turn)) != "" {
		return assistantPhaseAwaitCommitConfirm
	}
	return assistantPhaseIdle
}

func assistantTurnMissingFields(turn *assistantTurn) []string {
	if turn == nil {
		return nil
	}
	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, code := range assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors) {
		field := ""
		switch code {
		case "missing_parent_ref_text", "parent_candidate_not_found":
			field = "parent_ref_text"
		case "missing_new_parent_ref_text":
			field = "new_parent_ref_text"
		case "missing_entity_name":
			field = "entity_name"
		case "missing_new_name":
			field = "new_name"
		case "missing_org_code":
			field = "org_code"
		case "missing_effective_date", "invalid_effective_date_format":
			field = "effective_date"
		case "missing_target_effective_date", "invalid_target_effective_date_format":
			field = "target_effective_date"
		case "missing_change_fields":
			field = "change_fields"
		}
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func assistantTurnHasValidationCode(turn *assistantTurn, code string) bool {
	if turn == nil {
		return false
	}
	needle := strings.TrimSpace(code)
	for _, item := range assistantNormalizeValidationErrors(turn.DryRun.ValidationErrors) {
		if item == needle {
			return true
		}
	}
	return false
}

func assistantTurnSelectedCandidateID(turn *assistantTurn) string {
	if turn == nil {
		return ""
	}
	if strings.TrimSpace(turn.SelectedCandidateID) != "" {
		return strings.TrimSpace(turn.SelectedCandidateID)
	}
	return strings.TrimSpace(turn.ResolvedCandidateID)
}

func assistantTurnPendingDraftSummary(turn *assistantTurn) string {
	if turn == nil {
		return ""
	}
	if len(assistantTurnMissingFields(turn)) > 0 {
		return ""
	}
	if assistantTurnHasValidationCode(turn, "candidate_confirmation_required") && strings.TrimSpace(assistantTurnSelectedCandidateID(turn)) == "" {
		return ""
	}
	if strings.TrimSpace(turn.Intent.Action) != assistantIntentCreateOrgUnit {
		return strings.TrimSpace(turn.DryRun.Explain)
	}
	parts := make([]string, 0, 3)
	if parent := strings.TrimSpace(assistantTurnParentLabel(turn)); parent != "" {
		parts = append(parts, "上级组织："+parent)
	}
	if name := strings.TrimSpace(turn.Intent.EntityName); name != "" {
		parts = append(parts, "新建组织："+name)
	}
	if effectiveDate := strings.TrimSpace(turn.Intent.EffectiveDate); effectiveDate != "" {
		parts = append(parts, "生效日期："+effectiveDate)
	}
	if len(parts) == 0 {
		return strings.TrimSpace(turn.DryRun.Explain)
	}
	return strings.Join(parts, "；")
}

func assistantTurnParentLabel(turn *assistantTurn) string {
	if turn == nil {
		return ""
	}
	selectedID := assistantTurnSelectedCandidateID(turn)
	if selectedID != "" {
		if candidate, ok := assistantFindCandidate(turn.Candidates, selectedID); ok {
			if name := strings.TrimSpace(candidate.Name); name != "" {
				return name
			}
			if code := strings.TrimSpace(candidate.CandidateCode); code != "" {
				return code
			}
		}
	}
	return strings.TrimSpace(turn.Intent.ParentRefText)
}

func assistantTurnCommitReply(turn *assistantTurn) *assistantCommitReply {
	if turn == nil {
		return nil
	}
	if turn.CommitReply != nil {
		copyReply := *turn.CommitReply
		return &copyReply
	}
	if turn.ReplyNLG != nil {
		stage := strings.TrimSpace(turn.ReplyNLG.Stage)
		if stage == "commit_result" || stage == "commit_failed" {
			outcome := "success"
			if stage == "commit_failed" || strings.TrimSpace(turn.ErrorCode) != "" {
				outcome = "failure"
			}
			return &assistantCommitReply{
				Outcome:   outcome,
				Message:   strings.TrimSpace(turn.ReplyNLG.Text),
				ErrorCode: strings.TrimSpace(turn.ErrorCode),
			}
		}
	}
	if turn.CommitResult != nil {
		message := strings.TrimSpace(turn.CommitResult.OrgCode)
		if message == "" {
			message = strings.TrimSpace(turn.CommitResult.EventUUID)
		}
		if message != "" {
			message = "提交成功：" + message
		} else {
			message = "提交成功"
		}
		return &assistantCommitReply{Outcome: "success", Message: message}
	}
	if strings.TrimSpace(turn.ErrorCode) != "" {
		return &assistantCommitReply{Outcome: "failure", Message: strings.TrimSpace(turn.ErrorCode), ErrorCode: strings.TrimSpace(turn.ErrorCode)}
	}
	return nil
}

func assistantTransitionPhaseValue(state string, reason string, turnPhase string, from bool) string {
	reason = strings.TrimSpace(reason)
	turnPhase = strings.TrimSpace(turnPhase)
	switch reason {
	case "conversation_created":
		if from {
			return "init"
		}
		return assistantPhaseIdle
	case "turn_created":
		if from {
			return assistantPhaseIdle
		}
		if turnPhase != "" {
			return turnPhase
		}
		return assistantPhaseAwaitCommitConfirm
	case "confirmed":
		if from {
			if turnPhase != "" && turnPhase != assistantPhaseAwaitCommitConfirm {
				return turnPhase
			}
			return assistantPhaseAwaitCommitConfirm
		}
		return assistantPhaseAwaitCommitConfirm
	case "contract_version_mismatch", "version_drift", "version_tuple_stale":
		return assistantPhaseAwaitCommitConfirm
	case "committed":
		if from {
			return assistantPhaseAwaitCommitConfirm
		}
		return assistantPhaseCommitted
	}
	switch strings.TrimSpace(state) {
	case "init":
		return "init"
	case assistantStateCommitted:
		return assistantPhaseCommitted
	case assistantStateCanceled:
		return assistantPhaseCanceled
	case assistantStateExpired:
		return assistantPhaseExpired
	case assistantStateConfirmed:
		return assistantPhaseAwaitCommitConfirm
	case assistantStateValidated:
		if turnPhase != "" {
			return turnPhase
		}
		return assistantPhaseAwaitCommitConfirm
	default:
		return assistantPhaseIdle
	}
}

func assistantCommitReplyJSON(turn *assistantTurn) any {
	reply := assistantTurnCommitReply(turn)
	if reply == nil {
		return nil
	}
	payload, _ := json.Marshal(reply)
	return string(payload)
}

func assistantMissingFieldsJSON(turn *assistantTurn) string {
	payload, _ := json.Marshal(assistantTurnMissingFields(turn))
	return string(payload)
}

func assistantCandidateOptionsJSON(turn *assistantTurn) string {
	candidates := turn.Candidates
	if candidates == nil {
		candidates = make([]assistantCandidate, 0)
	}
	payload, _ := json.Marshal(candidates)
	return string(payload)
}

func assistantSetReplySnapshot(turn *assistantTurn, reply *assistantRenderReplyResponse, errorCode string) {
	if turn == nil || reply == nil {
		return
	}
	copyReply := *reply
	turn.ReplyNLG = &copyReply
	turn.CommitReply = &assistantCommitReply{
		Outcome: func() string {
			if strings.TrimSpace(errorCode) != "" || strings.TrimSpace(reply.Stage) == "commit_failed" {
				return "failure"
			}
			return "success"
		}(),
		Message:   strings.TrimSpace(reply.Text),
		ErrorCode: strings.TrimSpace(errorCode),
	}
	turn.ErrorCode = strings.TrimSpace(errorCode)
	if turn.UpdatedAt.IsZero() {
		turn.UpdatedAt = time.Now().UTC()
	}
	assistantRefreshTurnDerivedFields(turn)
}
