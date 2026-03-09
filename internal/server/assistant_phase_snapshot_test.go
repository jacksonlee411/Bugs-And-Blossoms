package server

import (
	"strings"
	"testing"
	"time"
)

func TestAssistantPhaseSnapshot_TurnDerivedFields(t *testing.T) {
	t.Run("missing fields", func(t *testing.T) {
		turn := &assistantTurn{
			State:  assistantStateValidated,
			Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			DryRun: assistantDryRunResult{ValidationErrors: []string{"missing_entity_name", "missing_effective_date"}},
		}
		assistantRefreshTurnDerivedFields(turn)
		if turn.Phase != assistantPhaseAwaitMissingFields {
			t.Fatalf("expected await_missing_fields, got=%q", turn.Phase)
		}
		if len(turn.MissingFields) != 2 || turn.PendingDraftSummary != "" {
			t.Fatalf("unexpected derived fields: missing=%v summary=%q", turn.MissingFields, turn.PendingDraftSummary)
		}
	})

	t.Run("candidate pick", func(t *testing.T) {
		turn := &assistantTurn{
			State:      assistantStateValidated,
			Intent:     assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			DryRun:     assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
			Candidates: []assistantCandidate{{CandidateID: "c1", Name: "鲜花组织"}, {CandidateID: "c2", Name: "花店组织"}},
		}
		assistantRefreshTurnDerivedFields(turn)
		if turn.Phase != assistantPhaseAwaitCandidatePick {
			t.Fatalf("expected await_candidate_pick, got=%q", turn.Phase)
		}
		if turn.SelectedCandidateID != "" {
			t.Fatalf("expected no selected candidate, got=%q", turn.SelectedCandidateID)
		}
	})

	t.Run("candidate confirm", func(t *testing.T) {
		turn := &assistantTurn{
			State:               assistantStateValidated,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			DryRun:              assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
			Candidates:          []assistantCandidate{{CandidateID: "c1", Name: "鲜花组织"}},
			ResolvedCandidateID: "c1",
		}
		assistantRefreshTurnDerivedFields(turn)
		if turn.Phase != assistantPhaseAwaitCandidateConfirm {
			t.Fatalf("expected await_candidate_confirm, got=%q", turn.Phase)
		}
		if turn.SelectedCandidateID != "c1" {
			t.Fatalf("expected selected candidate c1, got=%q", turn.SelectedCandidateID)
		}
	})

	t.Run("await commit confirm", func(t *testing.T) {
		turn := &assistantTurn{
			State:               assistantStateValidated,
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
			Candidates:          []assistantCandidate{{CandidateID: "c1", Name: "鲜花组织"}},
			ResolvedCandidateID: "c1",
		}
		assistantRefreshTurnDerivedFields(turn)
		if turn.Phase != assistantPhaseAwaitCommitConfirm {
			t.Fatalf("expected await_commit_confirm, got=%q", turn.Phase)
		}
		if !strings.Contains(turn.PendingDraftSummary, "新建组织：运营部") {
			t.Fatalf("expected pending draft summary populated, got=%q", turn.PendingDraftSummary)
		}
	})

	t.Run("committed reply", func(t *testing.T) {
		turn := &assistantTurn{
			State:        assistantStateCommitted,
			CommitResult: &assistantCommitResult{OrgCode: "ORG-1"},
		}
		assistantRefreshTurnDerivedFields(turn)
		if turn.Phase != assistantPhaseCommitted {
			t.Fatalf("expected committed, got=%q", turn.Phase)
		}
		if turn.CommitReply == nil || turn.CommitReply.Outcome != "success" {
			t.Fatalf("expected success commit reply, got=%+v", turn.CommitReply)
		}
	})
}

func TestAssistantPhaseSnapshot_ConversationAndReplyDerived(t *testing.T) {
	turn := &assistantTurn{
		TurnID:              "turn_1",
		State:               assistantStateValidated,
		RequestID:           "req_1",
		TraceID:             "trace_1",
		Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"},
		Candidates:          []assistantCandidate{{CandidateID: "c1", Name: "鲜花组织"}},
		ResolvedCandidateID: "c1",
		UpdatedAt:           time.Now().UTC(),
	}
	conversation := &assistantConversation{
		ConversationID: "conv_1",
		State:          assistantStateValidated,
		Turns:          []*assistantTurn{turn},
		Transitions: []assistantStateTransition{{
			TurnID:     "turn_1",
			TurnAction: "confirm",
			RequestID:  "req_1",
			TraceID:    "trace_1",
			FromState:  assistantStateValidated,
			ToState:    assistantStateConfirmed,
			ReasonCode: "confirmed",
			ActorID:    "actor_1",
			ChangedAt:  time.Now().UTC(),
		}},
	}
	assistantRefreshConversationDerivedFields(conversation)
	if conversation.CurrentPhase != assistantPhaseAwaitCommitConfirm {
		t.Fatalf("expected current phase await_commit_confirm, got=%q", conversation.CurrentPhase)
	}
	if conversation.Transitions[0].FromPhase != assistantPhaseAwaitCommitConfirm || conversation.Transitions[0].ToPhase != assistantPhaseAwaitCommitConfirm {
		t.Fatalf("unexpected transition phases: %+v", conversation.Transitions[0])
	}

	reply := &assistantRenderReplyResponse{Text: "已提交成功", Stage: "commit_result"}
	assistantSetReplySnapshot(turn, reply, "")
	if turn.CommitReply == nil || turn.CommitReply.Message != "已提交成功" {
		t.Fatalf("expected reply snapshot persisted, got=%+v", turn.CommitReply)
	}
}

func TestAssistantPhaseSnapshot_HelperCoverage(t *testing.T) {
	assistantRefreshConversationDerivedFields(nil)
	assistantRefreshTurnDerivedFields(nil)
	assistantRefreshTransitionDerivedFields(nil, nil)
	assistantSetReplySnapshot(nil, &assistantRenderReplyResponse{Text: "noop"}, "")
	assistantSetReplySnapshot(&assistantTurn{}, nil, "")

	if got := assistantConversationPhaseFromLegacyState(assistantStateCommitted); got != assistantPhaseCommitted {
		t.Fatalf("committed derived phase=%q", got)
	}
	if got := assistantConversationPhaseFromLegacyState(assistantStateCanceled); got != assistantPhaseCanceled {
		t.Fatalf("canceled derived phase=%q", got)
	}
	if got := assistantConversationPhaseFromLegacyState(assistantStateExpired); got != assistantPhaseExpired {
		t.Fatalf("expired derived phase=%q", got)
	}
	if got := assistantTurnPhase(nil); got != assistantPhaseIdle {
		t.Fatalf("nil turn phase=%q", got)
	}
	if got := assistantTurnPhase(&assistantTurn{State: assistantStateCanceled}); got != assistantPhaseCanceled {
		t.Fatalf("canceled turn phase=%q", got)
	}
	if got := assistantTurnPhase(&assistantTurn{State: assistantStateExpired}); got != assistantPhaseExpired {
		t.Fatalf("expired turn phase=%q", got)
	}
	if got := assistantTurnPhase(&assistantTurn{State: assistantStateValidated, ErrorCode: "x"}); got != assistantPhaseFailed {
		t.Fatalf("failed turn phase=%q", got)
	}
	if fields := assistantTurnMissingFields(nil); fields != nil {
		t.Fatalf("expected nil missing fields, got=%v", fields)
	}
	dupFields := assistantTurnMissingFields(&assistantTurn{DryRun: assistantDryRunResult{ValidationErrors: []string{"missing_entity_name", "missing_entity_name"}}})
	if len(dupFields) != 1 || dupFields[0] != "entity_name" {
		t.Fatalf("unexpected dedup fields=%v", dupFields)
	}
	effectiveFields := assistantTurnMissingFields(&assistantTurn{DryRun: assistantDryRunResult{ValidationErrors: []string{"missing_effective_date", "invalid_effective_date_format"}}})
	if len(effectiveFields) != 1 || effectiveFields[0] != "effective_date" {
		t.Fatalf("unexpected effective-date dedup=%v", effectiveFields)
	}
	if assistantTurnHasValidationCode(nil, "x") {
		t.Fatal("nil turn should not have validation code")
	}
	if got := assistantTurnSelectedCandidateID(nil); got != "" {
		t.Fatalf("nil selected candidate=%q", got)
	}
	if got := assistantTurnSelectedCandidateID(&assistantTurn{SelectedCandidateID: "selected", ResolvedCandidateID: "resolved"}); got != "selected" {
		t.Fatalf("selected candidate=%q", got)
	}
	if got := assistantTurnPendingDraftSummary(nil); got != "" {
		t.Fatalf("nil pending summary=%q", got)
	}
	if got := assistantTurnPendingDraftSummary(&assistantTurn{Intent: assistantIntentSpec{Action: "other"}, DryRun: assistantDryRunResult{Explain: "说明"}}); got != "说明" {
		t.Fatalf("non-create summary=%q", got)
	}
	if got := assistantTurnPendingDraftSummary(&assistantTurn{Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit}}); got != "" {
		t.Fatalf("empty create summary=%q", got)
	}
	if got := assistantTurnParentLabel(nil); got != "" {
		t.Fatalf("nil parent label=%q", got)
	}
	if got := assistantTurnParentLabel(&assistantTurn{SelectedCandidateID: "c1", Candidates: []assistantCandidate{{CandidateID: "c1", CandidateCode: "FLOWER-A"}}}); got != "FLOWER-A" {
		t.Fatalf("candidate code label=%q", got)
	}
	if got := assistantTurnParentLabel(&assistantTurn{Intent: assistantIntentSpec{ParentRefText: "鲜花组织"}}); got != "鲜花组织" {
		t.Fatalf("intent parent label=%q", got)
	}
	if reply := assistantTurnCommitReply(nil); reply != nil {
		t.Fatalf("nil commit reply=%+v", reply)
	}
	if reply := assistantTurnCommitReply(&assistantTurn{ReplyNLG: &assistantRenderReplyResponse{Stage: "commit_result", Text: "提交完成"}}); reply == nil || reply.Message != "提交完成" || reply.Outcome != "success" {
		t.Fatalf("reply_nlg success=%+v", reply)
	}
	if reply := assistantTurnCommitReply(&assistantTurn{ReplyNLG: &assistantRenderReplyResponse{Stage: "commit_failed", Text: "提交失败"}, ErrorCode: "commit_failed"}); reply == nil || reply.Outcome != "failure" {
		t.Fatalf("reply_nlg failure=%+v", reply)
	}
	if reply := assistantTurnCommitReply(&assistantTurn{CommitResult: &assistantCommitResult{EventUUID: "evt-1"}}); reply == nil || !strings.Contains(reply.Message, "evt-1") {
		t.Fatalf("commit_result fallback=%+v", reply)
	}
	if reply := assistantTurnCommitReply(&assistantTurn{CommitResult: &assistantCommitResult{}}); reply == nil || reply.Message != "提交成功" {
		t.Fatalf("commit_result empty=%+v", reply)
	}
	if reply := assistantTurnCommitReply(&assistantTurn{ErrorCode: "boom"}); reply == nil || reply.ErrorCode != "boom" {
		t.Fatalf("error fallback=%+v", reply)
	}
	transitionCases := []struct {
		state     string
		reason    string
		turnPhase string
		from      bool
		want      string
	}{
		{reason: "conversation_created", from: true, want: "init"},
		{reason: "conversation_created", from: false, want: assistantPhaseIdle},
		{reason: "turn_created", from: false, want: assistantPhaseAwaitCommitConfirm},
		{reason: "confirmed", from: true, turnPhase: assistantPhaseAwaitCandidateConfirm, want: assistantPhaseAwaitCandidateConfirm},
		{reason: "contract_version_mismatch", want: assistantPhaseAwaitCommitConfirm},
		{state: assistantStateExpired, reason: "confirm_ttl_expired", want: assistantPhaseExpired},
		{reason: "committed", from: false, want: assistantPhaseCommitted},
		{state: "init", want: "init"},
		{state: assistantStateCommitted, want: assistantPhaseCommitted},
		{state: assistantStateCanceled, want: assistantPhaseCanceled},
		{state: assistantStateExpired, want: assistantPhaseExpired},
		{state: assistantStateValidated, turnPhase: assistantPhaseAwaitCandidateConfirm, want: assistantPhaseAwaitCandidateConfirm},
	}
	for _, tc := range transitionCases {
		if got := assistantTransitionPhaseValue(tc.state, tc.reason, tc.turnPhase, tc.from); got != tc.want {
			t.Fatalf("transition phase mismatch state=%q reason=%q from=%v got=%q want=%q", tc.state, tc.reason, tc.from, got, tc.want)
		}
	}
	if got := assistantCommitReplyJSON(nil); got != nil {
		t.Fatalf("nil commit reply json=%v", got)
	}
}
