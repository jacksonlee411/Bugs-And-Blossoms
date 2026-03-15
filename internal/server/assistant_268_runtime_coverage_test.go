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

	synthetic := assistantSyntheticSemanticPayloadForPrompt(prompt)
	if synthetic.Action != assistantIntentCreateOrgUnit || synthetic.RouteKind != assistantRouteKindBusinessAction || synthetic.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected synthetic payload=%+v", synthetic)
	}
	confirmPayload := assistantSyntheticSemanticPayloadForPrompt(assistantBuildSemanticPrompt("确认", turn))
	if confirmPayload.Action != assistantIntentCreateOrgUnit || confirmPayload.RouteKind != assistantRouteKindBusinessAction || confirmPayload.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected synthetic confirm payload=%+v", confirmPayload)
	}
}

func TestAssistant268SyntheticSemanticHelperCoverage(t *testing.T) {
	mappings := map[string]string{
		assistantIntentCreateOrgUnit:        "org.orgunit_create",
		assistantIntentAddOrgUnitVersion:    "org.orgunit_add_version",
		assistantIntentInsertOrgUnitVersion: "org.orgunit_insert_version",
		assistantIntentCorrectOrgUnit:       "org.orgunit_correct",
		assistantIntentRenameOrgUnit:        "org.orgunit_rename",
		assistantIntentMoveOrgUnit:          "org.orgunit_move",
		assistantIntentDisableOrgUnit:       "org.orgunit_disable",
		assistantIntentEnableOrgUnit:        "org.orgunit_enable",
	}
	for actionID, want := range mappings {
		if got := assistantSemanticIntentIDForAction(actionID); got != want {
			t.Fatalf("action=%s intent_id=%s want=%s", actionID, got, want)
		}
	}
	if got := assistantSemanticIntentIDForAction("custom_action"); got != "action.custom_action" {
		t.Fatalf("unexpected default intent_id=%s", got)
	}

	business := assistantSyntheticSemanticPayload("在鲜花组织之下新建一个部门，成立日期是2026-01-01")
	if business.RouteKind != assistantRouteKindBusinessAction || business.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected business payload=%+v", business)
	}
	qa := assistantSyntheticSemanticPayload("系统有哪些功能")
	if qa.RouteKind != assistantRouteKindKnowledgeQA || qa.IntentID != "knowledge.general_qa" {
		t.Fatalf("unexpected qa payload=%+v", qa)
	}
	chat := assistantSyntheticSemanticPayload("你好")
	if chat.RouteKind != assistantRouteKindChitchat || chat.IntentID != "chat.greeting" {
		t.Fatalf("unexpected chat payload=%+v", chat)
	}
	uncertain := assistantSyntheticSemanticPayload("随便记一下")
	if uncertain.RouteKind != assistantRouteKindUncertain || uncertain.IntentID != "route.uncertain" {
		t.Fatalf("unexpected uncertain payload=%+v", uncertain)
	}

	if assistantSyntheticSemanticLooksLikeKnowledgeQA("   ") {
		t.Fatal("blank text should not be knowledge qa")
	}
	if !assistantSyntheticSemanticLooksLikeKnowledgeQA("help me") {
		t.Fatal("help should be knowledge qa")
	}
	if assistantSyntheticSemanticLooksLikeKnowledgeQA("你好") {
		t.Fatal("greeting should not be knowledge qa")
	}
	if assistantSyntheticSemanticLooksLikeChitchat("   ") {
		t.Fatal("blank text should not be chitchat")
	}
	if !assistantSyntheticSemanticLooksLikeChitchat("hello there") {
		t.Fatal("hello should be chitchat")
	}
	if assistantSyntheticSemanticLooksLikeChitchat("系统有哪些功能") {
		t.Fatal("knowledge qa should not be chitchat")
	}

	pendingPlanOnlyPrompt := `{"current_user_input":"确认","allowed_actions":[],"pending_turn":{"action":"plan_only"}}`
	if payload := assistantSyntheticSemanticPayloadForPrompt(pendingPlanOnlyPrompt); payload.RouteKind != assistantRouteKindUncertain {
		t.Fatalf("pending plan_only should not override route=%+v", payload)
	}
	pendingKnowledgePrompt := `{"current_user_input":"系统有哪些功能","allowed_actions":[],"pending_turn":{"action":"create_orgunit"}}`
	if payload := assistantSyntheticSemanticPayloadForPrompt(pendingKnowledgePrompt); payload.RouteKind != assistantRouteKindKnowledgeQA {
		t.Fatalf("knowledge prompt should keep qa route=%+v", payload)
	}
	pendingBusinessPrompt := `{"current_user_input":"在鲜花组织之下新建部门","allowed_actions":[],"pending_turn":{"action":"move_orgunit"}}`
	if payload := assistantSyntheticSemanticPayloadForPrompt(pendingBusinessPrompt); payload.Action != assistantIntentCreateOrgUnit || payload.IntentID != "org.orgunit_create" {
		t.Fatalf("business prompt should keep explicit action=%+v", payload)
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
}
