package server

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAssistantReplyRealizer_ResolveKindAndMappings(t *testing.T) {
	cases := []struct {
		name  string
		input assistantReplyRealizerInput
		want  string
	}{
		{name: "manual takeover", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{TaskStatus: assistantTaskStatusManualTakeoverNeeded}}, want: assistantReplyGuidanceKindManualTakeover},
		{name: "task waiting queued", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{TaskStatus: assistantTaskStatusQueued}}, want: assistantReplyGuidanceKindTaskWaiting},
		{name: "non business route", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{RouteKind: assistantRouteKindKnowledgeQA}}, want: assistantReplyGuidanceKindNonBusinessRoute},
		{name: "clarification missing slots", input: assistantReplyRealizerInput{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots}}, want: assistantReplyGuidanceKindMissingFields},
		{name: "clarification candidate pick", input: assistantReplyRealizerInput{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindCandidatePick}}, want: assistantReplyGuidanceKindCandidateList},
		{name: "clarification candidate confirm", input: assistantReplyRealizerInput{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindCandidateConfirm}}, want: assistantReplyGuidanceKindCandidateConfirm},
		{name: "clarification generic", input: assistantReplyRealizerInput{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindIntentDisambiguate}}, want: assistantReplyGuidanceKindClarificationRequired},
		{name: "route clarification required", input: assistantReplyRealizerInput{RouteDecision: assistantIntentRouteDecision{ClarificationRequired: true}}, want: assistantReplyGuidanceKindClarificationRequired},
		{name: "commit failed by error", input: assistantReplyRealizerInput{OutcomeHint: "failure", ErrorCode: "x"}, want: assistantReplyGuidanceKindCommitFailed},
		{name: "commit success by commit result", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{CommitResult: &assistantCommitResult{OrgCode: "ORG1"}}}, want: assistantReplyGuidanceKindCommitSuccess},
		{name: "confirm summary by phase", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{TurnPhase: assistantPhaseAwaitCommitConfirm}}, want: assistantReplyGuidanceKindConfirmSummary},
		{name: "candidate list by phase", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{TurnPhase: assistantPhaseAwaitCandidateConfirm}}, want: assistantReplyGuidanceKindCandidateList},
		{name: "candidate confirm by selected", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{TurnPhase: assistantPhaseAwaitCandidateConfirm, SelectedCandidate: "cid-1"}}, want: assistantReplyGuidanceKindCandidateConfirm},
		{name: "missing fields by phase", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{TurnPhase: assistantPhaseAwaitMissingFields}}, want: assistantReplyGuidanceKindMissingFields},
		{name: "missing fields by list", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{MissingFields: []string{"effective_date"}}}, want: assistantReplyGuidanceKindMissingFields},
		{name: "candidate list by count", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{CandidateCount: 2}}, want: assistantReplyGuidanceKindCandidateList},
		{name: "candidate confirm by count and selected", input: assistantReplyRealizerInput{Machine: assistantReplyMachineState{CandidateCount: 2, SelectedCandidate: "cid-1"}}, want: assistantReplyGuidanceKindCandidateConfirm},
		{name: "from stage hint", input: assistantReplyRealizerInput{StageHint: assistantTaskStatusRunning}, want: assistantReplyGuidanceKindTaskWaiting},
		{name: "default confirm summary", input: assistantReplyRealizerInput{}, want: assistantReplyGuidanceKindConfirmSummary},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := assistantResolveReplyGuidanceKind(tc.input); got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}

	if got := assistantReplyTaskStatusFromStageHint("unknown"); got != "" {
		t.Fatalf("unexpected task status=%q", got)
	}
	if got := assistantReplyTaskStatusFromStageHint(assistantTaskStatusRunning); got != assistantTaskStatusRunning {
		t.Fatalf("unexpected running status=%q", got)
	}

	for _, item := range []struct {
		stage string
		want  string
	}{
		{"await_clarification", assistantReplyGuidanceKindClarificationRequired},
		{assistantPhaseAwaitMissingFields, assistantReplyGuidanceKindMissingFields},
		{"candidate_list", assistantReplyGuidanceKindCandidateList},
		{"candidate_confirm", assistantReplyGuidanceKindCandidateConfirm},
		{assistantPhaseAwaitCommitConfirm, assistantReplyGuidanceKindConfirmSummary},
		{"commit_result", assistantReplyGuidanceKindCommitSuccess},
		{"commit_failed", assistantReplyGuidanceKindCommitFailed},
		{assistantTaskStatusQueued, assistantReplyGuidanceKindTaskWaiting},
		{assistantTaskStatusManualTakeoverNeeded, assistantReplyGuidanceKindManualTakeover},
		{"non_business_route", assistantReplyGuidanceKindNonBusinessRoute},
	} {
		if got := assistantReplyGuidanceKindFromStageHint(item.stage, assistantReplyMachineState{}); got != item.want {
			t.Fatalf("stage=%q got=%q want=%q", item.stage, got, item.want)
		}
	}
	if got := assistantReplyGuidanceKindFromStageHint(assistantPhaseAwaitCandidateConfirm, assistantReplyMachineState{SelectedCandidate: "cid-1"}); got != assistantReplyGuidanceKindCandidateConfirm {
		t.Fatalf("unexpected kind for await_candidate_confirm with selected candidate: %q", got)
	}
	if got := assistantReplyGuidanceKindFromStageHint(assistantPhaseAwaitCandidateConfirm, assistantReplyMachineState{}); got != assistantReplyGuidanceKindCandidateList {
		t.Fatalf("unexpected kind for await_candidate_confirm without selected candidate: %q", got)
	}
	if got := assistantReplyGuidanceKindFromStageHint("unknown", assistantReplyMachineState{}); got != "" {
		t.Fatalf("unexpected kind for unknown stage=%q", got)
	}

	for _, item := range []struct {
		replyKind string
		wantStage string
	}{
		{assistantReplyGuidanceKindCandidateList, "candidate_list"},
		{assistantReplyGuidanceKindCandidateConfirm, "candidate_confirm"},
		{assistantReplyGuidanceKindTaskWaiting, assistantReplyGuidanceKindTaskWaiting},
		{assistantReplyGuidanceKindManualTakeover, assistantReplyGuidanceKindManualTakeover},
		{assistantReplyGuidanceKindNonBusinessRoute, assistantReplyGuidanceKindNonBusinessRoute},
	} {
		if got := assistantReplyStageFromGuidanceKind(item.replyKind); got != item.wantStage {
			t.Fatalf("reply kind=%q got stage=%q want=%q", item.replyKind, got, item.wantStage)
		}
	}
	if got := assistantReplyStageFromGuidanceKind("unknown"); got != "draft" {
		t.Fatalf("unexpected stage fallback=%q", got)
	}
	if got := assistantReplyRenderKindFromGuidanceKind("unknown"); got != "info" {
		t.Fatalf("unexpected render kind fallback=%q", got)
	}
	if got := assistantNormalizeReplyRenderStage("", "unknown"); got != "draft" {
		t.Fatalf("unexpected normalized stage=%q", got)
	}
}

func TestAssistantReplyRealizer_TemplateAndVariables(t *testing.T) {
	if got := assistantReplyCandidateLabel(assistantReplyMachineState{}); got != "" {
		t.Fatalf("unexpected empty label=%q", got)
	}
	if got := assistantReplyCandidateLabel(assistantReplyMachineState{SelectedCandidate: "cid-1", Candidates: []assistantCandidate{{CandidateID: "cid-1", Name: "共享中心", CandidateCode: "C1"}}}); got != "共享中心 / C1" {
		t.Fatalf("unexpected candidate label=%q", got)
	}
	if got := assistantReplyCandidateLabel(assistantReplyMachineState{SelectedCandidate: "cid-2", Candidates: []assistantCandidate{{CandidateID: "cid-2", Name: "共享中心"}}}); got != "共享中心" {
		t.Fatalf("unexpected candidate label without code=%q", got)
	}
	if got := assistantReplyCandidateLabel(assistantReplyMachineState{SelectedCandidate: "cid-missing"}); got != "cid-missing" {
		t.Fatalf("unexpected fallback candidate label=%q", got)
	}
	if got := assistantReplyCandidateLabel(assistantReplyMachineState{
		ResolvedCandidate: "cid-2",
		Candidates: []assistantCandidate{
			{CandidateID: "cid-1", Name: "共享中心", CandidateCode: "C1"},
			{CandidateID: "cid-2", CandidateCode: "C2"},
		},
	}); got != "cid-2 / C2" {
		t.Fatalf("unexpected resolved candidate label=%q", got)
	}

	if got := assistantReplyCandidateListText(assistantReplyMachineState{}); got != "" {
		t.Fatalf("unexpected empty candidate list=%q", got)
	}
	listText := assistantReplyCandidateListText(assistantReplyMachineState{Candidates: []assistantCandidate{{CandidateID: "cid-1", CandidateCode: "C1", Name: "共享中心", Path: "集团/共享中心"}}})
	if !strings.Contains(listText, "1. 共享中心 / C1") {
		t.Fatalf("unexpected candidate list text=%q", listText)
	}
	if got := assistantReplyCandidateListText(assistantReplyMachineState{Candidates: []assistantCandidate{{CandidateID: "cid-2", CandidateCode: "C2"}}}); !strings.Contains(got, "cid-2 / C2") {
		t.Fatalf("unexpected candidate list fallback name text=%q", got)
	}

	if got := assistantReplyMissingFieldsText(assistantReplyMachineState{MissingFields: []string{"effective_date", "parent_ref_text"}}, "zh"); !strings.Contains(got, "、") {
		t.Fatalf("expected zh separator, got=%q", got)
	}
	if got := assistantReplyMissingFieldsText(assistantReplyMachineState{MissingFields: []string{"effective_date", "parent_ref_text"}}, "en"); !strings.Contains(got, ", ") {
		t.Fatalf("expected en separator, got=%q", got)
	}
	if got := assistantReplyMissingFieldsText(assistantReplyMachineState{ValidationErrors: []string{"missing_effective_date"}}, "zh"); got == "" {
		t.Fatal("expected validation-based missing fields text")
	}
	if got := assistantReplyMissingFieldsText(assistantReplyMachineState{}, "zh"); got != "" {
		t.Fatalf("unexpected missing fields text=%q", got)
	}

	vars := assistantBuildReplyTemplateVariables(assistantReplyRealizerInput{
		Locale:           "zh",
		ErrorCode:        "assistant_confirmation_required",
		ErrorExplanation: "",
		NextAction:       "请确认",
		Machine: assistantReplyMachineState{
			MissingFields:       []string{"effective_date"},
			CandidateCount:      2,
			Candidates:          []assistantCandidate{{CandidateID: "cid-1", Name: "共享中心", CandidateCode: "C1"}},
			SelectedCandidate:   "cid-1",
			PendingDraftSummary: "摘要",
			EntityName:          "运营部",
			ParentRefText:       "共享中心",
			EffectiveDate:       "2026-01-01",
			TaskStatus:          assistantTaskStatusQueued,
		},
	})
	if vars["error_explanation"] == "" || vars["selected_candidate"] == "" {
		t.Fatalf("unexpected variables=%v", vars)
	}

	if _, err := assistantRenderReplyGuidanceTemplate("", vars); err == nil {
		t.Fatal("expected empty template error")
	}
	if _, err := assistantRenderReplyGuidanceTemplate("{unknown_var}", vars); err == nil {
		t.Fatal("expected unknown variable error")
	}
	if _, err := assistantRenderReplyGuidanceTemplate("{candidate_list}", map[string]string{}); err == nil {
		t.Fatal("expected missing variable error")
	}
	if rendered, err := assistantRenderReplyGuidanceTemplate("前缀{selected_candidate}后缀", vars); err != nil || !strings.Contains(rendered, "共享中心 / C1") {
		t.Fatalf("rendered=%q err=%v", rendered, err)
	}
	if rendered, err := assistantRenderReplyGuidanceTemplate("just {", vars); err != nil || rendered != "just {" {
		t.Fatalf("expected open brace passthrough, got rendered=%q err=%v", rendered, err)
	}
	if rendered, err := assistantRenderReplyGuidanceTemplate("固定文本", nil); err != nil || rendered != "固定文本" {
		t.Fatalf("expected nil variables passthrough, got rendered=%q err=%v", rendered, err)
	}
}

func TestAssistantReplyRealizer_SelectAndFallback(t *testing.T) {
	runtime := &assistantKnowledgeRuntime{
		replyGuidance: map[string]map[string][]assistantReplyGuidancePack{
			assistantReplyGuidanceKindMissingFields: {
				"zh": {
					{
						ReplyKind:         assistantReplyGuidanceKindMissingFields,
						Locale:            "zh",
						GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.zh.v1", Text: "请补充：{missing_fields}"}},
					},
					{
						ReplyKind:         assistantReplyGuidanceKindMissingFields,
						Locale:            "zh",
						ErrorCodes:        []string{"missing_parent_ref_text"},
						GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.zh.error.v1", Text: "缺少上级组织：{missing_fields}"}},
					},
				},
				"en": {
					{
						ReplyKind:         assistantReplyGuidanceKindMissingFields,
						Locale:            "en",
						GuidanceTemplates: []assistantKnowledgePrompt{{TemplateID: "reply.missing_fields.en.v1", Text: "Missing fields: {missing_fields}"}},
					},
				},
			},
		},
	}
	if _, ok := assistantSelectReplyGuidance(assistantReplyRealizerInput{ResolvedReplyKind: assistantReplyGuidanceKindMissingFields, Locale: "zh"}, nil); ok {
		t.Fatal("expected no selection when runtime missing")
	}
	if _, ok := assistantSelectReplyGuidance(assistantReplyRealizerInput{ResolvedReplyKind: "unknown", Locale: "zh"}, runtime); ok {
		t.Fatal("expected no selection for unknown kind")
	}

	selection, ok := assistantSelectReplyGuidance(assistantReplyRealizerInput{ResolvedReplyKind: assistantReplyGuidanceKindMissingFields, Locale: "zh", ErrorCode: "missing_parent_ref_text"}, runtime)
	if !ok || selection.TemplateID != "reply.missing_fields.zh.error.v1" {
		t.Fatalf("unexpected selection=%+v ok=%v", selection, ok)
	}
	selection, ok = assistantSelectReplyGuidance(assistantReplyRealizerInput{ResolvedReplyKind: assistantReplyGuidanceKindMissingFields, Locale: "fr", ErrorCode: "missing_unknown"}, runtime)
	if !ok || selection.Locale != "zh" || selection.TemplateID != "reply.missing_fields.zh.v1" {
		t.Fatalf("expected zh fallback selection, got=%+v ok=%v", selection, ok)
	}

	output := assistantRealizeReply(
		assistantReplyRealizerInput{
			ResolvedReplyKind: assistantReplyGuidanceKindMissingFields,
			Locale:            "zh",
			ErrorCode:         "missing_parent_ref_text",
			Machine:           assistantReplyMachineState{MissingFields: []string{"parent_ref_text"}},
		},
		runtime,
		assistantRenderReplyRequest{},
		nil,
	)
	if output.UsedFallback || output.ReplySource != assistantReplySourceGuidancePack || strings.TrimSpace(output.Text) == "" {
		t.Fatalf("unexpected realize output=%+v", output)
	}

	fallback := assistantRealizeReply(
		assistantReplyRealizerInput{
			ResolvedReplyKind: assistantReplyGuidanceKindMissingFields,
			Locale:            "zh",
			Machine:           assistantReplyMachineState{MissingFields: nil},
		},
		runtime,
		assistantRenderReplyRequest{ErrorCode: "assistant_confirmation_required"},
		nil,
	)
	if !fallback.UsedFallback || strings.TrimSpace(fallback.Text) == "" {
		t.Fatalf("expected fallback output, got=%+v", fallback)
	}
	if got := assistantRealizeReply(
		assistantReplyRealizerInput{
			ResolvedReplyKind: "",
			StageHint:         assistantTaskStatusQueued,
			Locale:            "zh",
		},
		nil,
		assistantRenderReplyRequest{},
		nil,
	); got.ReplyKind != assistantReplyGuidanceKindTaskWaiting || !got.UsedFallback {
		t.Fatalf("unexpected realize output for empty kind with stage hint=%+v", got)
	}
	if got := assistantRealizeReply(
		assistantReplyRealizerInput{
			ResolvedReplyKind: "",
			StageHint:         "unknown",
			Locale:            "zh",
		},
		nil,
		assistantRenderReplyRequest{},
		nil,
	); got.ReplyKind != assistantReplyGuidanceKindConfirmSummary || !got.UsedFallback {
		t.Fatalf("unexpected realize output for default kind fallback=%+v", got)
	}

	turn := &assistantTurn{DryRun: assistantDryRunResult{Explain: "继续处理"}}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, assistantReplyGuidanceKindTaskWaiting, turn, "en"); !strings.Contains(strings.ToLower(got), "queued") {
		t.Fatalf("unexpected task_waiting en fallback=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, assistantReplyGuidanceKindTaskWaiting, turn, "zh"); !strings.Contains(got, "处理中") {
		t.Fatalf("unexpected task_waiting fallback=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, assistantReplyGuidanceKindManualTakeover, turn, "en"); !strings.Contains(strings.ToLower(got), "manual") {
		t.Fatalf("unexpected manual_takeover fallback=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, assistantReplyGuidanceKindManualTakeover, turn, "zh"); !strings.Contains(got, "人工接管") {
		t.Fatalf("unexpected manual_takeover zh fallback=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, assistantReplyGuidanceKindNonBusinessRoute, turn, "en"); !strings.Contains(strings.ToLower(got), "non-business") {
		t.Fatalf("unexpected non_business_route en fallback=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, assistantReplyGuidanceKindNonBusinessRoute, turn, "zh"); !strings.Contains(got, "非业务动作") {
		t.Fatalf("unexpected non_business_route fallback=%q", got)
	}
	if got := assistantReplyFallbackText(assistantRenderReplyRequest{}, "await_clarification", turn, "en"); !strings.Contains(strings.ToLower(got), "clarification") {
		t.Fatalf("unexpected await_clarification fallback=%q", got)
	}
}

func TestAssistantReplyRealizer_BuildInputAndRenderTurnReply(t *testing.T) {
	if got := assistantReplyTaskStatusFromStageHint("bad"); got != "" {
		t.Fatalf("unexpected status=%q", got)
	}

	input := assistantBuildReplyRealizerInput(assistantRenderReplyRequest{Stage: assistantTaskStatusRunning, ErrorCode: "assistant_confirmation_required"}, nil, "fr")
	if input.Locale != "zh" || input.Machine.TaskStatus != assistantTaskStatusRunning {
		t.Fatalf("unexpected input without turn=%+v", input)
	}

	svc := newAssistantConversationService(nil, nil)
	principal, conversation, turn := seedAssistantReplyConversation(t, svc)
	turn.Clarification = &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindIntentDisambiguate}
	turn.Plan.ReplyGuidanceVersion = "reply.v1"
	turn.Plan.KnowledgeSnapshotDigest = "digest.v1"
	turn.Plan.ResolverContractVersion = "resolver.v1"

	built := assistantBuildReplyRealizerInput(assistantRenderReplyRequest{Stage: assistantTaskStatusManualTakeoverNeeded, ErrorCode: "assistant_confirmation_required", ErrorMessage: "need confirmation", NextAction: "confirm"}, turn, "en")
	if built.Locale != "en" || built.Clarification == nil || built.ReplyGuidanceVersion != "reply.v1" || built.KnowledgeSnapshotDigest != "digest.v1" {
		t.Fatalf("unexpected built input=%+v", built)
	}

	originalRender := assistantRenderReplyWithModelFn
	defer func() { assistantRenderReplyWithModelFn = originalRender }()

	svc.knowledgeRuntime = nil
	svc.knowledgeErr = errAssistantRuntimeConfigInvalid
	captured := assistantReplyRenderPrompt{}
	assistantRenderReplyWithModelFn = func(_ context.Context, _ *assistantConversationService, prompt assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		captured = prompt
		return assistantReplyModelResult{Text: "ok", Kind: "info", Stage: prompt.Stage, ReplyModelName: assistantReplyTargetModelName}, nil
	}
	reply, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, turn.TurnID, assistantRenderReplyRequest{Stage: "draft", Locale: "zh"})
	if err != nil {
		t.Fatalf("renderTurnReply err=%v", err)
	}
	if reply == nil || strings.TrimSpace(captured.ReplyKind) == "" {
		t.Fatalf("expected captured reply kind, reply=%+v prompt=%+v", reply, captured)
	}

	if got := assistantNormalizeReplyRenderKind("bad", "warning"); got != "warning" {
		t.Fatalf("unexpected normalized kind=%q", got)
	}
	if !assistantValidReplyStageValue(assistantReplyGuidanceKindTaskWaiting) {
		t.Fatal("expected task_waiting stage valid")
	}
	if got := assistantNormalizeReplyRenderStage("invalid", assistantReplyGuidanceKindTaskWaiting); got != assistantReplyGuidanceKindTaskWaiting {
		t.Fatalf("unexpected normalized stage=%q", got)
	}

	badModel := func(_ context.Context, _ *assistantConversationService, _ assistantReplyRenderPrompt) (assistantReplyModelResult, error) {
		return assistantReplyModelResult{Text: "ok", ReplyModelName: "gpt-4.1"}, nil
	}
	assistantRenderReplyWithModelFn = badModel
	if _, err := svc.renderTurnReply(context.Background(), "tenant_1", principal, conversation.ConversationID, turn.TurnID, assistantRenderReplyRequest{}); !errors.Is(err, errAssistantReplyModelTargetMismatch) {
		t.Fatalf("expected target mismatch, got=%v", err)
	}
}
