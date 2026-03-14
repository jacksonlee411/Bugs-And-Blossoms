package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAssistant268SemanticPromptHelpers(t *testing.T) {
	if got := assistantSemanticPromptPendingTurn(nil); got != nil {
		t.Fatalf("expected nil pending turn, got=%+v", got)
	}
	empty := &assistantTurn{}
	if got := assistantSemanticPromptPendingTurn(empty); got != nil {
		t.Fatalf("expected nil empty pending prompt, got=%+v", got)
	}

	turn := &assistantTurn{
		Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
		DryRun:              assistantDryRunResult{ValidationErrors: []string{"missing_entity_name"}},
		PendingDraftSummary: "待确认草案",
		ResolvedCandidateID: "FLOWER-A",
		Candidates: []assistantCandidate{{
			CandidateID:   "FLOWER-A",
			CandidateCode: "FLOWER-A",
			Name:          "鲜花组织",
			Path:          "/集团/鲜花组织",
		}},
	}
	prompt := assistantBuildSemanticPrompt("  补充一个名字  ", turn)
	var envelope assistantSemanticPromptEnvelope
	if err := json.Unmarshal([]byte(prompt), &envelope); err != nil {
		t.Fatalf("unmarshal prompt err=%v prompt=%s", err, prompt)
	}
	if envelope.CurrentUserInput != "补充一个名字" {
		t.Fatalf("unexpected current_user_input=%q", envelope.CurrentUserInput)
	}
	if envelope.PendingTurn == nil || envelope.PendingTurn.Action != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected pending turn=%+v", envelope.PendingTurn)
	}
	if len(envelope.PendingTurn.MissingFields) != 1 || envelope.PendingTurn.MissingFields[0] != "entity_name" {
		t.Fatalf("unexpected missing fields=%v", envelope.PendingTurn.MissingFields)
	}
	if envelope.PendingTurn.SelectedCandidateID != "FLOWER-A" || len(envelope.PendingTurn.Candidates) != 1 {
		t.Fatalf("unexpected pending candidates=%+v", envelope.PendingTurn)
	}
	required := map[string][]string{}
	for _, action := range envelope.AllowedActions {
		required[action.ActionID] = action.RequiredSlots
	}
	if got := strings.Join(required[assistantIntentCorrectOrgUnit], ","); got != "org_code,target_effective_date" {
		t.Fatalf("unexpected correct required slots=%q", got)
	}
	if got := assistantSemanticCurrentUserInput(prompt); got != "补充一个名字" {
		t.Fatalf("unexpected extracted user input=%q", got)
	}
	if got := assistantSemanticCurrentUserInput("  原始文本  "); got != "原始文本" {
		t.Fatalf("unexpected raw user input=%q", got)
	}

	noPendingPrompt := assistantBuildSemanticPrompt("直接输入", nil)
	var noPending assistantSemanticPromptEnvelope
	if err := json.Unmarshal([]byte(noPendingPrompt), &noPending); err != nil {
		t.Fatalf("unmarshal no-pending prompt err=%v", err)
	}
	if noPending.PendingTurn != nil {
		t.Fatalf("expected nil pending turn in prompt, got=%+v", noPending.PendingTurn)
	}
}

func TestAssistant268SemanticReplyHelpers(t *testing.T) {
	if got := assistantSemanticReplyStage(nil); got != "draft" {
		t.Fatalf("nil stage=%q", got)
	}
	assistantSeedTurnReplyFromSemantic(nil, assistantResolveIntentResult{UserVisibleReply: "noop"})

	t.Run("seed from semantic reply and next question", func(t *testing.T) {
		turn := &assistantTurn{Phase: assistantPhaseAwaitMissingFields, Plan: assistantPlanSummary{ModelName: "gpt-5-codex"}}
		assistantSeedTurnReplyFromSemantic(turn, assistantResolveIntentResult{UserVisibleReply: "请补充名称。"})
		if turn.ReplyNLG == nil || turn.ReplyNLG.Text != "请补充名称。" || turn.ReplyNLG.ReplySource != assistantReplySourceSemanticModel {
			t.Fatalf("unexpected seeded reply=%+v", turn.ReplyNLG)
		}
		if turn.CommitReply == nil || turn.CommitReply.Outcome != "pending" {
			t.Fatalf("unexpected commit reply=%+v", turn.CommitReply)
		}

		turn2 := &assistantTurn{Phase: assistantPhaseAwaitMissingFields}
		assistantSeedTurnReplyFromSemantic(turn2, assistantResolveIntentResult{NextQuestion: "请告诉我部门名称。"})
		if turn2.ReplyNLG == nil || turn2.ReplyNLG.Text != "请告诉我部门名称。" {
			t.Fatalf("unexpected next-question reply=%+v", turn2.ReplyNLG)
		}
	})

	t.Run("seed from draft summary explain and missing fields fallback", func(t *testing.T) {
		summaryTurn := &assistantTurn{Phase: assistantPhaseAwaitCommitConfirm, PendingDraftSummary: "上级组织：鲜花组织"}
		assistantSeedTurnReplyFromSemantic(summaryTurn, assistantResolveIntentResult{})
		if summaryTurn.ReplyNLG == nil || summaryTurn.ReplyNLG.Stage != "confirm_summary" {
			t.Fatalf("unexpected summary reply=%+v", summaryTurn.ReplyNLG)
		}

		explainTurn := &assistantTurn{Phase: assistantPhaseAwaitCommitConfirm, DryRun: assistantDryRunResult{Explain: "计划已生成"}}
		assistantSeedTurnReplyFromSemantic(explainTurn, assistantResolveIntentResult{})
		if explainTurn.ReplyNLG == nil || explainTurn.ReplyNLG.Text != "计划已生成" {
			t.Fatalf("unexpected explain reply=%+v", explainTurn.ReplyNLG)
		}

		missingTurn := &assistantTurn{Phase: assistantPhaseAwaitMissingFields, MissingFields: []string{"entity_name"}}
		assistantSeedTurnReplyFromSemantic(missingTurn, assistantResolveIntentResult{})
		if missingTurn.ReplyNLG == nil || !strings.Contains(missingTurn.ReplyNLG.Text, "补充缺失信息") {
			t.Fatalf("unexpected missing-fields reply=%+v", missingTurn.ReplyNLG)
		}

		noReplyTurn := &assistantTurn{}
		assistantSeedTurnReplyFromSemantic(noReplyTurn, assistantResolveIntentResult{})
		if noReplyTurn.ReplyNLG != nil {
			t.Fatalf("expected no reply snapshot, got=%+v", noReplyTurn.ReplyNLG)
		}
	})

	t.Run("reply from turn snapshot or commit reply", func(t *testing.T) {
		turn := &assistantTurn{
			Phase: assistantPhaseAwaitCommitConfirm,
			ReplyNLG: &assistantRenderReplyResponse{
				Text:           "语义回复",
				Stage:          "confirm_summary",
				ReplyModelName: "gpt-5-codex",
			},
		}
		reply := assistantSemanticReplyFromTurn(turn, "conv_1", "turn_1")
		if reply == nil || reply.ConversationID != "conv_1" || reply.TurnID != "turn_1" {
			t.Fatalf("unexpected stored semantic reply=%+v", reply)
		}

		commitTurn := &assistantTurn{
			Phase:       assistantPhaseFailed,
			CommitReply: &assistantCommitReply{Message: "失败了"},
			Plan:        assistantPlanSummary{ModelName: "gpt-5-codex"},
		}
		commitReply := assistantSemanticReplyFromTurn(commitTurn, "conv_2", "turn_2")
		if commitReply == nil || commitReply.Stage != "commit_failed" || commitReply.Text != "失败了" {
			t.Fatalf("unexpected commit reply=%+v", commitReply)
		}

		if got := assistantSemanticReplyFromTurn(&assistantTurn{}, "conv", "turn"); got != nil {
			t.Fatalf("expected nil reply, got=%+v", got)
		}
		if got := assistantSemanticReplyFromTurn(nil, "conv", "turn"); got != nil {
			t.Fatalf("expected nil reply for nil turn, got=%+v", got)
		}
	})
}

func TestAssistant268IntentPipelineHelpers(t *testing.T) {
	cases := []struct {
		name   string
		intent assistantIntentSpec
		want   bool
	}{
		{name: "empty action invalid", intent: assistantIntentSpec{}, want: true},
		{name: "plan only valid", intent: assistantIntentSpec{Action: assistantIntentPlanOnly}, want: false},
		{name: "unknown action invalid", intent: assistantIntentSpec{Action: "unsupported"}, want: true},
		{name: "invalid effective date", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026/01/01"}, want: true},
		{name: "invalid target date", intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026/01/01"}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := assistantModelIntentInvalid(tc.intent); got != tc.want {
				t.Fatalf("got=%v want=%v intent=%+v", got, tc.want, tc.intent)
			}
		})
	}

	t.Run("sanitize create and effective-date actions", func(t *testing.T) {
		create := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-03-09"},
			assistantIntentSpec{},
			nil,
		)
		if create.EffectiveDate != "" {
			t.Fatalf("expected hallucinated create date cleared, got=%+v", create)
		}

		rename := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentRenameOrgUnit, EffectiveDate: "2026-03-09"},
			assistantIntentSpec{EffectiveDate: "2026-04-01"},
			nil,
		)
		if rename.EffectiveDate != "2026-04-01" {
			t.Fatalf("expected explicit effective date preserved, got=%+v", rename)
		}
	})

	t.Run("sanitize correct target date from local or pending context", func(t *testing.T) {
		correct := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-09"},
			assistantIntentSpec{EffectiveDate: "2026-01-01"},
			nil,
		)
		if correct.TargetEffectiveDate != "2026-01-01" {
			t.Fatalf("expected explicit target date from user input, got=%+v", correct)
		}

		preserved := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-09"},
			assistantIntentSpec{},
			&assistantTurn{Intent: assistantIntentSpec{TargetEffectiveDate: "2026-02-01"}},
		)
		if preserved.TargetEffectiveDate != "2026-03-09" {
			t.Fatalf("expected pending-turn context to avoid clearing target date, got=%+v", preserved)
		}

		cleared := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-09"},
			assistantIntentSpec{},
			nil,
		)
		if cleared.TargetEffectiveDate != "" {
			t.Fatalf("expected hallucinated target date cleared, got=%+v", cleared)
		}
	})
}

func TestAssistant268ModelGatewaySemanticHelpers(t *testing.T) {
	readinessCases := map[string]string{
		"ready":                  assistantSemanticReadinessReadyForConfirm,
		"ready_for_dry_run":      assistantSemanticReadinessReadyForDryRun,
		"qa":                     assistantSemanticReadinessNonBusiness,
		"clarification_required": assistantSemanticReadinessNeedMoreInfo,
		"unknown":                "",
	}
	for input, want := range readinessCases {
		if got := assistantNormalizeOpenAIReadiness(input); got != want {
			t.Fatalf("input=%q got=%q want=%q", input, got, want)
		}
	}

	normalized := assistantNormalizeOpenAIIntentPayload(`{"action":"create_orgunit","goalSummary":"创建运营部","reply":"已生成草案","status":"ready","candidateId":"FLOWER-A"}`)
	payload, err := assistantStrictDecodeSemanticIntent(normalized)
	if err != nil {
		t.Fatalf("strict decode semantic err=%v payload=%s", err, string(normalized))
	}
	if payload.GoalSummary != "创建运营部" || payload.UserVisibleReply != "已生成草案" || payload.Readiness != assistantSemanticReadinessReadyForConfirm || payload.SelectedCandidateID != "FLOWER-A" {
		t.Fatalf("unexpected semantic payload=%+v", payload)
	}
	if payload.intentSpec().Action != assistantIntentCreateOrgUnit {
		t.Fatalf("unexpected intent spec=%+v", payload.intentSpec())
	}
	normalizedNext := assistantNormalizeOpenAIIntentPayload(`{"action":"create_orgunit","nextQuestion":"请补充名称","state":"missing_info"}`)
	nextPayload, err := assistantStrictDecodeSemanticIntent(normalizedNext)
	if err != nil {
		t.Fatalf("strict decode next-question payload err=%v payload=%s", err, string(normalizedNext))
	}
	if nextPayload.NextQuestion != "请补充名称" || nextPayload.Readiness != assistantSemanticReadinessNeedMoreInfo {
		t.Fatalf("unexpected next-question semantic payload=%+v", nextPayload)
	}

	if _, err := assistantStrictDecodeSemanticIntent([]byte(`{"action":"create_orgunit","extra":"x"}`)); err == nil {
		t.Fatal("expected unknown semantic field to fail")
	}
	if _, err := assistantStrictDecodeSemanticIntent([]byte(`{"action":"create_orgunit"}{}`)); !errors.Is(err, errAssistantPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected trailing payload failure, got=%v", err)
	}
}

func TestAssistant268ReplyRuntimeHelpers(t *testing.T) {
	if !assistantReplyRequestIsPassive(assistantRenderReplyRequest{}) {
		t.Fatal("expected empty request passive")
	}
	if assistantReplyRequestIsPassive(assistantRenderReplyRequest{Stage: "draft"}) {
		t.Fatal("expected stage request to be active")
	}
	if assistantReplyRequestIsPassive(assistantRenderReplyRequest{AllowMissingTurn: true}) {
		t.Fatal("expected allow-missing-turn request to be active")
	}

	svc := newAssistantConversationService(nil, nil)
	principal, created, _ := seedAssistantReplyConversation(t, svc)
	stored, err := svc.getConversation("tenant_1", principal.ID, created.ConversationID)
	if err != nil {
		t.Fatalf("getConversation err=%v", err)
	}

	originalRender := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = originalRender }()

	t.Run("latest turn skips existing semantic reply", func(t *testing.T) {
		called := false
		turn := latestTurn(stored)
		turn.ReplyNLG = &assistantRenderReplyResponse{Text: "已有语义回复", Stage: "draft"}
		assistantRenderReplyWithModelFn = func(context.Context, *assistantConversationService, assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
			called = true
			return assistantReplyModelResult{}, nil
		}
		assistantRenderReplyForLatestTurn(context.Background(), svc, "tenant_1", principal, stored.ConversationID, stored)
		if called {
			t.Fatal("expected latest turn renderer to skip model when semantic reply already exists")
		}
	})

	t.Run("latest turn no-op on render failure", func(t *testing.T) {
		turn := latestTurn(stored)
		turn.ReplyNLG = nil
		assistantRenderReplyWithModelFn = func(context.Context, *assistantConversationService, assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
			return assistantReplyModelResult{}, errAssistantReplyRenderFailed
		}
		assistantRenderReplyForLatestTurn(context.Background(), svc, "tenant_1", principal, stored.ConversationID, stored)
		if turn.ReplyNLG != nil {
			t.Fatalf("expected render failure to keep reply empty, got=%+v", turn.ReplyNLG)
		}
	})

	t.Run("latest turn renders and persists when empty", func(t *testing.T) {
		called := false
		turn := latestTurn(stored)
		turn.ReplyNLG = nil
		assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
			called = true
			return assistantReplyModelResult{
				Text:           "模型回复",
				Kind:           "info",
				Stage:          prompt.Stage,
				ReplyModelName: assistantReplyTargetModelName,
			}, nil
		}
		assistantRenderReplyForLatestTurn(context.Background(), svc, "tenant_1", principal, stored.ConversationID, stored)
		if !called {
			t.Fatal("expected latest turn renderer to invoke model")
		}
		if turn.ReplyNLG == nil || turn.ReplyNLG.Text != "模型回复" {
			t.Fatalf("expected rendered reply persisted on latest turn, got=%+v", turn.ReplyNLG)
		}
	})

	t.Run("passive request returns stored semantic reply", func(t *testing.T) {
		storedTurn := latestTurn(stored)
		storedTurn.ReplyNLG = &assistantRenderReplyResponse{Text: "缓存回复", Stage: "draft", ReplyModelName: "gpt-5-codex", ReplySource: assistantReplySourceSemanticModel}
		assistantRenderReplyWithModelFn = func(context.Context, *assistantConversationService, assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
			t.Fatal("passive render should not call model when semantic reply already exists")
			return assistantReplyModelResult{}, nil
		}
		svc.mu.Lock()
		svc.byID[stored.ConversationID].Turns[len(svc.byID[stored.ConversationID].Turns)-1].ReplyNLG = &assistantRenderReplyResponse{
			Text:           "缓存回复",
			Stage:          "draft",
			ReplyModelName: "gpt-5-codex",
			ReplySource:    assistantReplySourceSemanticModel,
		}
		svc.mu.Unlock()
		reply, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, stored.ConversationID, storedTurn.TurnID, assistantRenderReplyRequest{})
		if err != nil {
			t.Fatalf("renderTurnReply passive err=%v", err)
		}
		if reply == nil || reply.Text != "缓存回复" || reply.ReplySource != assistantReplySourceSemanticModel {
			t.Fatalf("unexpected passive reply=%+v", reply)
		}
	})

	if got := assistantReplyStageFromGuidanceKind(assistantReplyGuidanceKindClarificationRequired); got != "await_clarification" {
		t.Fatalf("unexpected clarification stage=%q", got)
	}
	if got := assistantReplyStageFromGuidanceKind(assistantReplyGuidanceKindCommitSuccess); got != "commit_result" {
		t.Fatalf("unexpected commit success stage=%q", got)
	}
	if got := assistantReplyStageFromGuidanceKind(assistantReplyGuidanceKindTaskWaiting); got != assistantReplyGuidanceKindTaskWaiting {
		t.Fatalf("unexpected task waiting stage=%q", got)
	}
	if got := assistantReplyStageFromGuidanceKind("unknown"); got != "draft" {
		t.Fatalf("unexpected default stage=%q", got)
	}
	if got := assistantReplyRenderKindFromGuidanceKind(assistantReplyGuidanceKindCommitSuccess); got != "success" {
		t.Fatalf("unexpected success render kind=%q", got)
	}
	if got := assistantReplyRenderKindFromGuidanceKind(assistantReplyGuidanceKindCommitFailed); got != "error" {
		t.Fatalf("unexpected failed render kind=%q", got)
	}
	if got := assistantReplyRenderKindFromGuidanceKind(assistantReplyGuidanceKindMissingFields); got != "warning" {
		t.Fatalf("unexpected warning render kind=%q", got)
	}
	if got := assistantReplyRenderKindFromGuidanceKind("unknown"); got != "info" {
		t.Fatalf("unexpected default render kind=%q", got)
	}

	assistantRenderReplyForLatestTurn(context.Background(), nil, "tenant_1", principal, stored.ConversationID, stored)
	assistantRenderReplyForLatestTurn(context.Background(), svc, "tenant_1", principal, stored.ConversationID, nil)
	assistantRenderReplyForLatestTurn(context.Background(), svc, "tenant_1", principal, "conv_empty", &assistantConversation{ConversationID: "conv_empty"})
}
