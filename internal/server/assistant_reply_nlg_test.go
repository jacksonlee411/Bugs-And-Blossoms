package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func seedAssistantReplyConversation(t *testing.T, svc *assistantConversationService) (Principal, *assistantConversation, *assistantTurn) {
	t.Helper()
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	created := svc.createConversation("tenant_1", principal)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	stored, ok := svc.byID[created.ConversationID]
	if !ok || stored == nil {
		t.Fatalf("conversation not stored")
	}
	turn := &assistantTurn{
		TurnID:    "turn_reply_1",
		State:     assistantStateValidated,
		RequestID: "assistant_req_reply_1",
		TraceID:   "trace_reply_1",
		Intent: assistantIntentSpec{
			Action:        assistantIntentCreateOrgUnit,
			ParentRefText: "鲜花组织",
			EntityName:    "运营部",
			EffectiveDate: "2026-01-01",
		},
		DryRun: assistantDryRunResult{
			Explain:          "计划已生成，等待确认后可提交",
			ValidationErrors: []string{},
		},
		Candidates: []assistantCandidate{
			{
				CandidateID:   "flowers-root",
				CandidateCode: "FLOWERS-ROOT",
				Name:          "鲜花组织",
				Path:          "/集团/鲜花组织",
				AsOf:          "2026-01-01",
				IsActive:      true,
				MatchScore:    0.95,
			},
		},
		ResolvedCandidateID: "flowers-root",
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	stored.Turns = append(stored.Turns, turn)
	stored.State = turn.State
	stored.UpdatedAt = turn.UpdatedAt
	return principal, created, turn
}

func TestAssistantReplyNLGPipeline(t *testing.T) {
	svc := newAssistantConversationService(nil, nil)
	principal, conversation, turn := seedAssistantReplyConversation(t, svc)

	original := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = original }()

	captured := assistantReplyRenderPrompt{}
	assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		captured = prompt
		return assistantReplyModelResult{
			Text:           "已生成草案，请确认后继续。",
			Kind:           "info",
			Stage:          "draft",
			ReplyModelName: assistantReplyTargetModelName,
		}, nil
	}

	reply, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, turn.TurnID, assistantRenderReplyRequest{
		Stage:        "draft",
		Kind:         "info",
		Outcome:      "success",
		FallbackText: "fallback",
		Locale:       "zh",
	})
	if err != nil {
		t.Fatalf("renderTurnReply err=%v", err)
	}
	if reply.Text != "已生成草案，请确认后继续。" {
		t.Fatalf("unexpected reply text=%q", reply.Text)
	}
	if reply.ReplyModelName != assistantReplyTargetModelName {
		t.Fatalf("unexpected reply model=%q", reply.ReplyModelName)
	}
	if captured.ConversationID != conversation.ConversationID || captured.TurnID != turn.TurnID {
		t.Fatalf("unexpected prompt identity: conv=%q turn=%q", captured.ConversationID, captured.TurnID)
	}
	if captured.Machine.IntentAction != assistantIntentCreateOrgUnit {
		t.Fatalf("expected intent action create_orgunit, got=%q", captured.Machine.IntentAction)
	}
	if captured.Machine.CandidateCount != 1 {
		t.Fatalf("expected candidate_count=1, got=%d", captured.Machine.CandidateCount)
	}
}

func TestAssistantReplyModelTargetGate(t *testing.T) {
	svc := newAssistantConversationService(nil, nil)
	principal, conversation, turn := seedAssistantReplyConversation(t, svc)

	original := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = original }()

	assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, _ assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		return assistantReplyModelResult{
			Text:           "模型命中失败",
			Kind:           "error",
			Stage:          "commit_failed",
			ReplyModelName: "gpt-4.1",
		}, nil
	}

	_, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, turn.TurnID, assistantRenderReplyRequest{
		Stage:   "commit_failed",
		Kind:    "error",
		Outcome: "failure",
	})
	if !errors.Is(err, errAssistantReplyModelTargetMismatch) {
		t.Fatalf("expected errAssistantReplyModelTargetMismatch, got=%v", err)
	}
}
