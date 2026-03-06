package server

import (
	"context"
	"errors"
	"strings"
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
	if reply.ReplySource != assistantReplySourceModel {
		t.Fatalf("unexpected reply source=%q", reply.ReplySource)
	}
	if reply.UsedFallback {
		t.Fatal("reply should not use fallback")
	}
	if captured.ConversationID != conversation.ConversationID || captured.TurnID != turn.TurnID {
		t.Fatalf("unexpected prompt identity: conv=%q turn=%q", captured.ConversationID, captured.TurnID)
	}
	if captured.Machine.IntentAction != assistantIntentCreateOrgUnit {
		t.Fatalf("expected intent action create_orgunit, got=%q", captured.Machine.IntentAction)
	}
	storedConversation, err := svc.getConversation("tenant_1", principal.ID, conversation.ConversationID)
	if err != nil {
		t.Fatalf("getConversation err=%v", err)
	}
	storedTurn := assistantFindTurnForReply(storedConversation, turn.TurnID)
	if storedTurn == nil || storedTurn.ReplyNLG == nil {
		t.Fatal("expected reply_nlg persisted on turn")
	}
	if storedTurn.ReplyNLG.ReplySource != assistantReplySourceModel {
		t.Fatalf("unexpected persisted reply source=%q", storedTurn.ReplyNLG.ReplySource)
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

func TestAssistantReplyFallbackText_HidesTechnicalSignals(t *testing.T) {
	text := assistantReplyFallbackText(assistantRenderReplyRequest{
		Stage:        "commit_failed",
		Kind:         "error",
		Outcome:      "failure",
		ErrorCode:    "ai_plan_schema_constrained_decode_failed",
		ErrorMessage: "ai_plan_schema_constrained_decode_failed",
	}, "commit_failed", nil, "zh")
	if strings.Contains(text, "ai_plan_schema_constrained_decode_failed") {
		t.Fatalf("expected technical signal hidden, got=%q", text)
	}
	if strings.TrimSpace(text) == "" {
		t.Fatalf("expected non-empty fallback text")
	}
}

func TestAssistantDecodeOpenAIReplyResult_HidesTechnicalSignals(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":"ai_plan_schema_constrained_decode_failed"}}]}`)
	result, err := assistantDecodeOpenAIReplyResult(raw, assistantReplyRenderPrompt{
		ConversationID: "conv_1",
		TurnID:         "turn_1",
		Stage:          "commit_failed",
		Kind:           "error",
		Outcome:        "failure",
		Locale:         "zh",
		FallbackText:   "ai_plan_schema_constrained_decode_failed",
	})
	if err != nil {
		t.Fatalf("assistantDecodeOpenAIReplyResult err=%v", err)
	}
	if strings.Contains(result.Text, "ai_plan_schema_constrained_decode_failed") {
		t.Fatalf("expected technical signal hidden, got=%q", result.Text)
	}
}

func TestAssistantDecodeOpenAIReplyResult_RejectsFallbackMasquerade(t *testing.T) {
	_, err := assistantDecodeOpenAIReplyResult([]byte(`not-json`), assistantReplyRenderPrompt{
		ConversationID: "conv_1",
		TurnID:         "turn_1",
		Stage:          "draft",
		Kind:           "info",
		Outcome:        "success",
		Locale:         "zh",
		FallbackText:   "本地兜底文案",
	})
	if !errors.Is(err, errAssistantReplyRenderFailed) {
		t.Fatalf("expected errAssistantReplyRenderFailed, got=%v", err)
	}
}

func TestAssistantRenderReply_AllowsMissingTurnWhenExplicitlyRequested(t *testing.T) {
	svc := newAssistantConversationService(nil, nil)
	principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}
	conversation := svc.createConversation("tenant_1", principal)

	original := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = original }()

	captured := assistantReplyRenderPrompt{}
	assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		captured = prompt
		return assistantReplyModelResult{
			Text:           "请补充成立日期后继续。",
			Kind:           "warning",
			Stage:          "missing_fields",
			ReplyModelName: assistantReplyTargetModelName,
			ReplySource:    assistantReplySourceModel,
		}, nil
	}

	reply, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, "missing-turn-context", assistantRenderReplyRequest{
		Stage:            "missing_fields",
		Kind:             "warning",
		Outcome:          "failure",
		ErrorCode:        "ai_plan_schema_constrained_decode_failed",
		ErrorMessage:     "ai plan schema constrained decode failed",
		FallbackText:     "请补充生效日期（YYYY-MM-DD）",
		AllowMissingTurn: true,
	})
	if err != nil {
		t.Fatalf("renderTurnReply err=%v", err)
	}
	if reply == nil || strings.TrimSpace(reply.Text) == "" {
		t.Fatalf("expected rendered reply, got=%+v", reply)
	}
	if captured.ConversationID != conversation.ConversationID {
		t.Fatalf("unexpected conversation id=%q", captured.ConversationID)
	}
	if captured.TurnID != "missing-turn-context" {
		t.Fatalf("expected missing-turn sentinel id, got=%q", captured.TurnID)
	}
	if captured.Machine.TurnState != "" {
		t.Fatalf("expected empty machine state without turn, got=%+v", captured.Machine)
	}
}

func TestAssistantReplyNLGFailurePipeline(t *testing.T) {
	svc := newAssistantConversationService(nil, nil)
	principal, conversation, turn := seedAssistantReplyConversation(t, svc)

	original := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = original }()

	captured := assistantReplyRenderPrompt{}
	assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		captured = prompt
		return assistantReplyModelResult{
			Text:           "这次提交暂未成功，请检查条件后重试。",
			Kind:           "error",
			Stage:          "commit_failed",
			ReplyModelName: assistantReplyTargetModelName,
			ReplySource:    assistantReplySourceModel,
		}, nil
	}

	reply, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, turn.TurnID, assistantRenderReplyRequest{
		Stage:        "commit_failed",
		Kind:         "error",
		Outcome:      "failure",
		ErrorCode:    "conversation_state_invalid",
		ErrorMessage: "提交失败，请按最新提示继续。",
		Locale:       "zh",
	})
	if err != nil {
		t.Fatalf("renderTurnReply err=%v", err)
	}
	if reply.ReplySource != assistantReplySourceModel {
		t.Fatalf("unexpected reply source=%q", reply.ReplySource)
	}
	if captured.Outcome != "failure" {
		t.Fatalf("expected failure outcome, got=%q", captured.Outcome)
	}
	if captured.ErrorCode != "conversation_state_invalid" {
		t.Fatalf("expected error code passed through, got=%q", captured.ErrorCode)
	}
	if captured.TurnID != turn.TurnID {
		t.Fatalf("expected real turn id, got=%q", captured.TurnID)
	}
}
