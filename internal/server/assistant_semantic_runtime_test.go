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
	if synthetic.Action != assistantIntentPlanOnly || synthetic.RouteKind != assistantRouteKindUncertain || synthetic.IntentID != "route.uncertain" {
		t.Fatalf("unexpected synthetic payload=%+v", synthetic)
	}
	if synthetic.ParentRefText != "" || synthetic.EntityName != "" || synthetic.EffectiveDate != "" {
		t.Fatalf("synthetic provider should not locally extract slots, got=%+v", synthetic)
	}
	confirmPayload := assistantSyntheticSemanticPayloadForPrompt(assistantBuildSemanticPrompt("确认", turn))
	if confirmPayload.Action != assistantIntentPlanOnly || confirmPayload.RouteKind != assistantRouteKindUncertain || confirmPayload.IntentID != "route.uncertain" {
		t.Fatalf("unexpected synthetic confirm payload=%+v", confirmPayload)
	}
}

func TestAssistant268SemanticPromptActions_Branches(t *testing.T) {
	hooks := captureAssistantKnowledgeHooks()
	defer hooks.restore()

	t.Run("falls back without runtime", func(t *testing.T) {
		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return nil, errors.New("runtime unavailable")
		}

		actions := assistantSemanticPromptActions()
		byID := map[string]assistantSemanticPromptAction{}
		for _, item := range actions {
			byID[item.ActionID] = item
		}
		if len(byID[assistantIntentCreateOrgUnit].RequiredSlots) == 0 {
			t.Fatal("expected business action required slots fallback without runtime")
		}
		if byID[assistantIntentPlanOnly].PlanSummary != "" {
			t.Fatalf("expected empty non-business summary without runtime, got=%q", byID[assistantIntentPlanOnly].PlanSummary)
		}
	})

	t.Run("uses runtime route metadata and markdown summaries", func(t *testing.T) {
		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return &assistantKnowledgeRuntime{
				routeByAction: map[string]assistantIntentRouteEntry{
					assistantIntentCreateOrgUnit: {
						IntentID:      "org.orgunit_create",
						RouteKind:     assistantRouteKindBusinessAction,
						RequiredSlots: []string{"parent_ref_text"},
					},
				},
				routeCatalog: assistantIntentRouteCatalog{
					Entries: []assistantIntentRouteEntry{
						{IntentID: assistantInterpretationDefaultPackID, RouteKind: assistantRouteKindKnowledgeQA},
					},
				},
				actionDocsByAction: map[string]map[string]assistantKnowledgeMarkdownDocument{
					assistantIntentCreateOrgUnit: {
						"zh": {ID: "action.org.orgunit_create", Locale: "zh", Title: "创建组织动作说明"},
					},
				},
				actionView: map[string]map[string]assistantActionViewPack{
					assistantIntentCreateOrgUnit: {
						"zh": {Summary: "创建组织摘要"},
					},
				},
				intentDocs: map[string]map[string]assistantKnowledgeMarkdownDocument{
					assistantInterpretationDefaultPackID: {
						"zh": {ID: assistantInterpretationDefaultPackID, Locale: "zh", Title: "知识问答", Summary: "知识问答摘要"},
					},
				},
			}, nil
		}

		actions := assistantSemanticPromptActions()
		byID := map[string]assistantSemanticPromptAction{}
		for _, item := range actions {
			byID[item.ActionID] = item
		}
		create := byID[assistantIntentCreateOrgUnit]
		if create.PlanSummary != "创建组织摘要" {
			t.Fatalf("unexpected create summary=%q", create.PlanSummary)
		}
		if strings.Join(create.RequiredSlots, ",") != "parent_ref_text" {
			t.Fatalf("unexpected create required slots=%v", create.RequiredSlots)
		}
		planOnly := byID[assistantIntentPlanOnly]
		if planOnly.PlanSummary != "知识问答摘要" {
			t.Fatalf("unexpected plan_only summary=%q", planOnly.PlanSummary)
		}
		if len(planOnly.RequiredSlots) != 0 {
			t.Fatalf("plan_only should not carry required slots, got=%v", planOnly.RequiredSlots)
		}
	})
}

func TestAssistant268SyntheticSemanticHelperCoverage(t *testing.T) {
	actionCases := map[string]string{
		"请新建组织":  assistantIntentCreateOrgUnit,
		"请创建部门":  assistantIntentCreateOrgUnit,
		"请新增版本":  assistantIntentAddOrgUnitVersion,
		"请插入版本":  assistantIntentInsertOrgUnitVersion,
		"请更正组织":  assistantIntentCorrectOrgUnit,
		"请移动组织":  assistantIntentMoveOrgUnit,
		"请重命名组织": assistantIntentRenameOrgUnit,
		"请停用组织":  assistantIntentDisableOrgUnit,
		"请启用组织":  assistantIntentEnableOrgUnit,
	}
	for input, want := range actionCases {
		if got := assistantSyntheticSemanticAction(input); got != want {
			t.Fatalf("input=%q action=%q want=%q", input, got, want)
		}
	}
	if got := assistantSyntheticSemanticAction("系统有哪些功能"); got != "" {
		t.Fatalf("non-business input should not map local action, got=%q", got)
	}

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

	t.Run("non business route helpers and system prompt", func(t *testing.T) {
		hooks := captureAssistantKnowledgeHooks()
		defer hooks.restore()

		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return nil, errors.New("runtime unavailable")
		}
		if intentID, routeKind := assistantSemanticIntentRouteForNonBusiness(assistantRouteKindUncertain); intentID != assistantRouteFallbackUncertainID || routeKind != assistantRouteKindUncertain {
			t.Fatalf("unexpected uncertain fallback intent=%q route=%q", intentID, routeKind)
		}
		if prompt := assistantOpenAISystemPrompt(); !strings.Contains(prompt, "你必须只输出严格 JSON") {
			t.Fatalf("fallback system prompt=%q", prompt)
		}

		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return &assistantKnowledgeRuntime{
				routeByAction: map[string]assistantIntentRouteEntry{
					assistantIntentCreateOrgUnit:  {IntentID: "org.orgunit_create", RouteKind: assistantRouteKindBusinessAction},
					assistantIntentDisableOrgUnit: {IntentID: "org.orgunit_disable", RouteKind: assistantRouteKindBusinessAction},
				},
				routeCatalog: assistantIntentRouteCatalog{
					Entries: []assistantIntentRouteEntry{
						{IntentID: "knowledge.general_qa", RouteKind: assistantRouteKindKnowledgeQA},
						{IntentID: "chat.greeting", RouteKind: assistantRouteKindChitchat},
						{IntentID: "route.uncertain", RouteKind: assistantRouteKindUncertain},
					},
				},
				intentDocs: map[string]map[string]assistantKnowledgeMarkdownDocument{
					"knowledge.general_qa": {
						"zh": {ID: "knowledge.general_qa", Locale: "zh", Title: "知识问答"},
					},
					"chat.greeting": {
						"zh": {ID: "chat.greeting", Locale: "zh", Title: "问候"},
					},
					"route.uncertain": {
						"zh": {ID: "route.uncertain", Locale: "zh", Title: "待澄清"},
					},
				},
			}, nil
		}
		if intentID, routeKind := assistantSemanticIntentRouteForNonBusiness(assistantRouteKindKnowledgeQA); intentID != "knowledge.general_qa" || routeKind != assistantRouteKindKnowledgeQA {
			t.Fatalf("unexpected knowledge route intent=%q route=%q", intentID, routeKind)
		}
		if intentID, routeKind := assistantSemanticIntentRouteForNonBusiness("other"); intentID != "" || routeKind != "other" {
			t.Fatalf("unexpected unknown route intent=%q route=%q", intentID, routeKind)
		}
		prompt := assistantOpenAISystemPrompt()
		for _, want := range []string{
			"create_orgunit=org.orgunit_create",
			"disable_orgunit=org.orgunit_disable",
			"知识问答 输出 action=plan_only、route_kind=knowledge_qa、intent_id=knowledge.general_qa",
			"问候 输出 action=plan_only、route_kind=chitchat、intent_id=chat.greeting",
			"待澄清 输出 action=plan_only、route_kind=uncertain、intent_id=route.uncertain",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("system prompt missing %q in %q", want, prompt)
			}
		}

		assistantLoadKnowledgeRuntimeFn = func() (*assistantKnowledgeRuntime, error) {
			return &assistantKnowledgeRuntime{
				routeByAction: map[string]assistantIntentRouteEntry{
					assistantIntentCreateOrgUnit: {IntentID: ""},
				},
				routeCatalog: assistantIntentRouteCatalog{
					Entries: []assistantIntentRouteEntry{
						{IntentID: "knowledge.general_qa", RouteKind: ""},
						{IntentID: "", RouteKind: assistantRouteKindChitchat},
					},
				},
				intentDocs: map[string]map[string]assistantKnowledgeMarkdownDocument{
					"knowledge.general_qa": {
						"zh": {ID: "knowledge.general_qa", Locale: "zh", Title: "知识问答"},
					},
				},
			}, nil
		}
		if got := assistantSemanticIntentIDForAction(""); got != "" {
			t.Fatalf("blank action should return empty intent id, got=%q", got)
		}
		if got := assistantSemanticIntentIDForAction(assistantIntentCreateOrgUnit); got != "action."+assistantIntentCreateOrgUnit {
			t.Fatalf("blank mapped intent id should fall back, got=%q", got)
		}
		if intentID, routeKind := assistantSemanticIntentRouteForNonBusiness(assistantRouteKindKnowledgeQA); intentID != "" || routeKind != assistantRouteKindKnowledgeQA {
			t.Fatalf("blank runtime route kind should fall through, got intent=%q route=%q", intentID, routeKind)
		}
		if intentID, routeKind := assistantSemanticIntentRouteForNonBusiness(assistantRouteKindChitchat); intentID != "" || routeKind != assistantRouteKindChitchat {
			t.Fatalf("blank runtime intent id should fall through, got intent=%q route=%q", intentID, routeKind)
		}
		prompt = assistantOpenAISystemPrompt()
		if !strings.Contains(prompt, "create_orgunit=action.create_orgunit") {
			t.Fatalf("blank business intent id should fall back to action.* mapping, got=%q", prompt)
		}
		if strings.Contains(prompt, "问候 输出 action=plan_only") {
			t.Fatalf("blank non-business intent id should be skipped from prompt, got=%q", prompt)
		}
	})

	business := assistantSyntheticSemanticPayload("在鲜花组织之下新建一个部门，成立日期是2026-01-01")
	if business.RouteKind != assistantRouteKindBusinessAction || business.IntentID != "org.orgunit_create" {
		t.Fatalf("unexpected business payload=%+v", business)
	}
	if business.ParentRefText != "" || business.EntityName != "" || business.EffectiveDate != "" {
		t.Fatalf("business synthetic payload should not contain locally extracted slots=%+v", business)
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
	pendingConfirmPrompt := `{"current_user_input":"确认","allowed_actions":[],"pending_turn":{"action":"create_orgunit"}}`
	if payload := assistantSyntheticSemanticPayloadForPrompt(pendingConfirmPrompt); payload.Action != assistantIntentPlanOnly || payload.RouteKind != assistantRouteKindUncertain {
		t.Fatalf("pending confirm should wait for model-owned continuation=%+v", payload)
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
		{name: "non-business empty action valid", intent: assistantIntentSpec{RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa"}, want: false},
		{name: "plan only valid", intent: assistantIntentSpec{Action: assistantIntentPlanOnly, RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa"}, want: false},
		{name: "business plan only invalid", intent: assistantIntentSpec{Action: assistantIntentPlanOnly, RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create"}, want: true},
		{name: "non-business business-action mismatch invalid", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: assistantRouteKindKnowledgeQA, IntentID: "knowledge.general_qa"}, want: true},
		{name: "unknown action invalid", intent: assistantIntentSpec{Action: "unsupported"}, want: true},
		{name: "missing route kind invalid", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, IntentID: "org.orgunit_create"}, want: true},
		{name: "missing intent id invalid", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: assistantRouteKindBusinessAction}, want: true},
		{name: "invalid route kind invalid", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: "bad_kind", IntentID: "org.orgunit_create"}, want: true},
		{name: "invalid effective date", intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_create", EffectiveDate: "2026/01/01"}, want: true},
		{name: "invalid target date", intent: assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_correct", TargetEffectiveDate: "2026/01/01"}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := assistantModelIntentInvalid(tc.intent); got != tc.want {
				t.Fatalf("got=%v want=%v intent=%+v", got, tc.want, tc.intent)
			}
		})
	}

	t.Run("extract explicit temporal hints", func(t *testing.T) {
		iso := assistantExtractExplicitTemporalHints("成立日期是2026-01-01")
		if iso.EffectiveDate != "2026-01-01" || iso.TargetEffectiveDate != "2026-01-01" {
			t.Fatalf("unexpected iso hints=%+v", iso)
		}
		cn := assistantExtractExplicitTemporalHints("成立日期是2026年1月2日")
		if cn.EffectiveDate != "2026-01-02" || cn.TargetEffectiveDate != "2026-01-02" {
			t.Fatalf("unexpected cn hints=%+v", cn)
		}
		empty := assistantExtractExplicitTemporalHints("补充部门名称")
		if empty.EffectiveDate != "" || empty.TargetEffectiveDate != "" {
			t.Fatalf("unexpected empty hints=%+v", empty)
		}
	})

	t.Run("build plan summaries for projection-only routes", func(t *testing.T) {
		qaPlan := assistantBuildPlan(assistantIntentSpec{RouteKind: assistantRouteKindKnowledgeQA})
		if qaPlan.Title != "知识问答意图" || qaPlan.Summary != "当前轮属于知识问答，只返回说明，不触发业务提交。" {
			t.Fatalf("unexpected qa plan=%+v", qaPlan)
		}
		chitchatPlan := assistantBuildPlan(assistantIntentSpec{RouteKind: assistantRouteKindChitchat})
		if chitchatPlan.Title != "闲聊意图" || chitchatPlan.Summary != "当前轮属于闲聊响应，不触发业务提交。" {
			t.Fatalf("unexpected chitchat plan=%+v", chitchatPlan)
		}
		uncertainPlan := assistantBuildPlan(assistantIntentSpec{RouteKind: assistantRouteKindUncertain})
		if uncertainPlan.Title != "未确定意图" || uncertainPlan.Summary != "当前轮语义仍不确定，仅保留澄清投影，不触发业务提交。" {
			t.Fatalf("unexpected uncertain plan=%+v", uncertainPlan)
		}
	})

	t.Run("prepare turn draft covers deferred and empty candidate branches", func(t *testing.T) {
		originalAuthorizer := assistantLoadAuthorizerFn
		originalDefinitions := capabilityDefinitionByKey
		defer func() {
			assistantLoadAuthorizerFn = originalAuthorizer
			capabilityDefinitionByKey = originalDefinitions
		}()
		assistantLoadAuthorizerFn = func() (authorizer, error) {
			return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
		}

		spec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
		if !ok {
			t.Fatal("missing create spec")
		}
		capabilityDefinitionByKey = map[string]capabilityDefinition{
			spec.CapabilityKey: {CapabilityKey: spec.CapabilityKey, Status: routeCapabilityStatusActive, ActivationState: routeCapabilityStatusActive},
		}
		principal := Principal{ID: "actor_1", RoleSlug: "tenant-admin"}

		notFoundStore := assistantOrgStoreStub{orgUnitMemoryStore: newOrgUnitMemoryStore(), searchErr: errOrgUnitNotFound}
		notFoundSvc := newAssistantConversationService(notFoundStore, assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		notFoundSvc.modelGateway = assistantTestStaticSemanticGateway(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01"}`)
		notFoundConv := notFoundSvc.createConversation("tenant_1", principal)
		notFoundConversation, err := notFoundSvc.createTurn(context.Background(), "tenant_1", principal, notFoundConv.ConversationID, "在鲜花组织之下新建运营部，成立日期是2026-01-01")
		if err != nil {
			t.Fatalf("createTurn not-found err=%v", err)
		}
		notFoundTurn := latestTurn(notFoundConversation)
		if len(notFoundTurn.Candidates) != 0 || notFoundTurn.AmbiguityCount != 0 || notFoundTurn.Confidence != 0.3 {
			t.Fatalf("unexpected not-found turn=%+v", notFoundTurn)
		}

		deferredSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		deferredSvc.modelGateway = assistantTestStaticSemanticGateway(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部"}`)
		deferredConv := deferredSvc.createConversation("tenant_1", principal)
		deferredConversation, err := deferredSvc.createTurn(context.Background(), "tenant_1", principal, deferredConv.ConversationID, "在鲜花组织之下新建运营部")
		if err != nil {
			t.Fatalf("createTurn deferred err=%v", err)
		}
		if got := latestTurn(deferredConversation).ResolutionSource; got != "deferred_candidate_lookup" {
			t.Fatalf("unexpected deferred resolution source=%q", got)
		}

		blankRiskSpec := spec
		blankRiskSpec.Security.RiskTier = ""
		blankRiskSvc := newAssistantConversationService(newOrgUnitMemoryStore(), assistantWriteServiceStub{store: newOrgUnitMemoryStore()})
		blankRiskSvc.actionRegistry = assistantActionRegistryMap{specs: map[string]assistantActionSpec{assistantIntentCreateOrgUnit: blankRiskSpec}}
		blankRiskSvc.modelGateway = assistantTestStaticSemanticGateway(`{"action":"create_orgunit","route_kind":"business_action","intent_id":"org.orgunit_create","parent_ref_text":"鲜花组织","entity_name":"运营部","effective_date":"2026-01-01"}`)
		blankRiskConv := blankRiskSvc.createConversation("tenant_1", principal)
		blankRiskConversation, err := blankRiskSvc.createTurn(context.Background(), "tenant_1", principal, blankRiskConv.ConversationID, "在鲜花组织之下新建运营部，成立日期是2026-01-01")
		if err != nil {
			t.Fatalf("createTurn blank-risk err=%v", err)
		}
		if got := latestTurn(blankRiskConversation).RiskTier; got != "low" {
			t.Fatalf("expected blank risk tier to fall back low, got=%q", got)
		}
	})

	t.Run("sanitize create and effective-date actions", func(t *testing.T) {
		create := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-03-09"},
			assistantExplicitTemporalHints{},
			nil,
		)
		if create.EffectiveDate != "" {
			t.Fatalf("expected hallucinated create date cleared, got=%+v", create)
		}

		rename := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentRenameOrgUnit, EffectiveDate: "2026-03-09"},
			assistantExplicitTemporalHints{EffectiveDate: "2026-04-01"},
			nil,
		)
		if rename.EffectiveDate != "2026-04-01" {
			t.Fatalf("expected explicit effective date preserved, got=%+v", rename)
		}
	})

	t.Run("sanitize correct target date from local or pending context", func(t *testing.T) {
		correct := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-09"},
			assistantExplicitTemporalHints{TargetEffectiveDate: "2026-01-01"},
			nil,
		)
		if correct.TargetEffectiveDate != "2026-01-01" {
			t.Fatalf("expected explicit target date from user input, got=%+v", correct)
		}

		preserved := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-09"},
			assistantExplicitTemporalHints{},
			&assistantTurn{Intent: assistantIntentSpec{TargetEffectiveDate: "2026-02-01"}},
		)
		if preserved.TargetEffectiveDate != "2026-03-09" {
			t.Fatalf("expected pending-turn context to avoid clearing target date, got=%+v", preserved)
		}

		cleared := assistantSanitizeResolvedIntentFacts(
			assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-09"},
			assistantExplicitTemporalHints{},
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
