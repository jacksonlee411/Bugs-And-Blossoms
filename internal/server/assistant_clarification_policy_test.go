package server

import (
	"errors"
	"testing"
	"time"
)

func assistantClarificationRuntimeFixture() *assistantKnowledgeRuntime {
	return &assistantKnowledgeRuntime{
		routeCatalog: assistantIntentRouteCatalog{Entries: []assistantIntentRouteEntry{
			{
				IntentID:                "org.orgunit_create",
				ActionID:                assistantIntentCreateOrgUnit,
				RouteKind:               assistantRouteKindBusinessAction,
				RequiredSlots:           []string{"parent_ref_text", "entity_name", "effective_date"},
				ClarificationTemplateID: "clarify.org.orgunit_create.v1",
			},
		}},
	}
}

func assistantClarificationRouteDecisionFixture() assistantIntentRouteDecision {
	return assistantIntentRouteDecision{
		RouteKind:               assistantRouteKindBusinessAction,
		IntentID:                "org.orgunit_create",
		CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
		ConfidenceBand:          assistantRouteConfidenceHigh,
		RouteCatalogVersion:     "2026-03-11.v1",
		KnowledgeSnapshotDigest: "sha256:test",
		ResolverContractVersion: "resolver.v1",
		DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
	}
}

func TestAssistantClarificationDecision_PriorityOrder(t *testing.T) {
	runtime := assistantClarificationRuntimeFixture()
	baseRoute := assistantClarificationRouteDecisionFixture()
	baseIntent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "运营部", EffectiveDate: "2026-01-01"}

	disambiguationRoute := baseRoute
	disambiguationRoute.RouteKind = assistantRouteKindUncertain
	disambiguationRoute.ClarificationRequired = true
	disambiguationRoute.CandidateActionIDs = []string{assistantIntentCreateOrgUnit, assistantIntentMoveOrgUnit}
	disambiguation := assistantBuildClarificationDecision(assistantClarificationBuildInput{
		UserInput:     "请帮我处理组织",
		Intent:        assistantIntentSpec{Action: assistantIntentPlanOnly},
		RouteDecision: disambiguationRoute,
		DryRun:        assistantDryRunResult{},
		Runtime:       runtime,
	})
	if disambiguation == nil || disambiguation.ClarificationKind != assistantClarificationKindIntentDisambiguate {
		t.Fatalf("expected intent disambiguation, got=%+v", disambiguation)
	}

	candidatePick := assistantBuildClarificationDecision(assistantClarificationBuildInput{
		Intent:               baseIntent,
		RouteDecision:        baseRoute,
		DryRun:               assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
		Candidates:           []assistantCandidate{{CandidateID: "FLOWER-A"}, {CandidateID: "FLOWER-B"}},
		ResolvedCandidateID:  "",
		SelectedCandidateID:  "",
		Runtime:              runtime,
		PendingClarification: nil,
	})
	if candidatePick == nil || candidatePick.ClarificationKind != assistantClarificationKindCandidatePick {
		t.Fatalf("expected candidate_pick, got=%+v", candidatePick)
	}

	candidateConfirm := assistantBuildClarificationDecision(assistantClarificationBuildInput{
		Intent:              baseIntent,
		RouteDecision:       baseRoute,
		DryRun:              assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
		Candidates:          []assistantCandidate{{CandidateID: "FLOWER-A"}, {CandidateID: "FLOWER-B"}},
		SelectedCandidateID: "FLOWER-A",
		Runtime:             runtime,
	})
	if candidateConfirm == nil || candidateConfirm.ClarificationKind != assistantClarificationKindCandidateConfirm {
		t.Fatalf("expected candidate_confirm, got=%+v", candidateConfirm)
	}

	formatIntent := baseIntent
	formatIntent.EffectiveDate = "2026/01/01"
	formatIntent.EntityName = ""
	formatDecision := assistantBuildClarificationDecision(assistantClarificationBuildInput{
		Intent:        formatIntent,
		RouteDecision: baseRoute,
		DryRun: assistantDryRunResult{ValidationErrors: []string{
			"invalid_effective_date_format",
			"missing_entity_name",
		}},
		Runtime: runtime,
	})
	if formatDecision == nil || formatDecision.ClarificationKind != assistantClarificationKindFormatConfirm {
		t.Fatalf("expected format_confirmation, got=%+v", formatDecision)
	}

	missingDecision := assistantBuildClarificationDecision(assistantClarificationBuildInput{
		Intent:        baseIntent,
		RouteDecision: baseRoute,
		DryRun:        assistantDryRunResult{ValidationErrors: []string{"missing_effective_date"}},
		Runtime:       runtime,
	})
	if missingDecision == nil || missingDecision.ClarificationKind != assistantClarificationKindMissingSlots {
		t.Fatalf("expected missing_slots, got=%+v", missingDecision)
	}
}

func TestAssistantClarificationDecision_RoundProgressAndExhausted(t *testing.T) {
	runtime := assistantClarificationRuntimeFixture()
	route := assistantClarificationRouteDecisionFixture()
	intent := assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织", EntityName: "", EffectiveDate: "2026-01-01"}
	input := assistantClarificationBuildInput{
		Intent:        intent,
		RouteDecision: route,
		DryRun:        assistantDryRunResult{ValidationErrors: []string{"missing_entity_name"}},
		Runtime:       runtime,
	}

	prev := &assistantClarificationDecision{
		ClarificationKind: assistantClarificationKindMissingSlots,
		Status:            assistantClarificationStatusOpen,
		CurrentRound:      1,
		MaxRounds:         3,
		AwaitPhase:        assistantPhaseAwaitMissingFields,
		ExitTo:            assistantClarificationExitBusinessResume,
		ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot},
	}
	input.PendingClarification = prev
	next := assistantBuildClarificationDecision(input)
	if next == nil || next.CurrentRound != 2 || next.Status != assistantClarificationStatusOpen || !assistantSliceHas(next.ReasonCodes, assistantClarificationReasonNoProgress) {
		t.Fatalf("expected round + no_progress, got=%+v", next)
	}

	prev = &assistantClarificationDecision{
		ClarificationKind: assistantClarificationKindMissingSlots,
		Status:            assistantClarificationStatusOpen,
		CurrentRound:      2,
		MaxRounds:         3,
		AwaitPhase:        assistantPhaseAwaitMissingFields,
		ExitTo:            assistantClarificationExitBusinessResume,
		ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot, assistantClarificationReasonNoProgress},
	}
	input.PendingClarification = prev
	aborted := assistantBuildClarificationDecision(input)
	if aborted == nil || aborted.Status != assistantClarificationStatusAborted || aborted.ExitTo != assistantClarificationExitManualHint {
		t.Fatalf("expected aborted manual hint, got=%+v", aborted)
	}

	prev = &assistantClarificationDecision{
		ClarificationKind: assistantClarificationKindMissingSlots,
		Status:            assistantClarificationStatusOpen,
		CurrentRound:      3,
		MaxRounds:         3,
		AwaitPhase:        assistantPhaseAwaitMissingFields,
		ExitTo:            assistantClarificationExitBusinessResume,
		ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot},
	}
	input.PendingClarification = prev
	exhausted := assistantBuildClarificationDecision(input)
	if exhausted == nil || exhausted.Status != assistantClarificationStatusExhausted || !assistantSliceHas(exhausted.ReasonCodes, assistantClarificationReasonRoundsExhausted) {
		t.Fatalf("expected exhausted clarification, got=%+v", exhausted)
	}
}

func TestAssistantClarificationGate_BlocksConfirmAndCommit(t *testing.T) {
	original := assistantLoadAuthorizerFn
	assistantLoadAuthorizerFn = func() (authorizer, error) {
		return assistantGateAuthorizerStub{allowed: true, enforced: true}, nil
	}
	defer func() { assistantLoadAuthorizerFn = original }()

	spec, ok := assistantLookupDefaultActionSpec(assistantIntentCreateOrgUnit)
	if !ok {
		t.Fatal("missing create action spec")
	}
	turn := &assistantTurn{
		Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
		RouteDecision: assistantIntentRouteDecision{
			RouteKind:               assistantRouteKindBusinessAction,
			IntentID:                "org.orgunit_create",
			CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
			ConfidenceBand:          assistantRouteConfidenceMedium,
			RouteCatalogVersion:     "2026-03-11.v1",
			KnowledgeSnapshotDigest: "sha256:test",
			ResolverContractVersion: "resolver.v1",
			DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
		},
		Clarification: &assistantClarificationDecision{
			ClarificationKind:       assistantClarificationKindMissingSlots,
			Status:                  assistantClarificationStatusOpen,
			MissingSlots:            []string{"entity_name"},
			ReasonCodes:             []string{assistantClarificationReasonMissingRequiredSlot},
			MaxRounds:               3,
			CurrentRound:            1,
			ExitTo:                  assistantClarificationExitBusinessResume,
			AwaitPhase:              assistantPhaseAwaitMissingFields,
			KnowledgeSnapshotDigest: "sha256:test",
			RouteCatalogVersion:     "2026-03-11.v1",
		},
		Phase: assistantPhaseAwaitMissingFields,
	}

	confirm := assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageConfirm, Action: spec, Turn: turn})
	if confirm.Allowed || !errors.Is(confirm.Error, errAssistantClarificationRequired) {
		t.Fatalf("expected clarification required on confirm, got=%+v", confirm)
	}
	commit := assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, Turn: turn})
	if commit.Allowed || !errors.Is(commit.Error, errAssistantClarificationRequired) {
		t.Fatalf("expected clarification required on commit, got=%+v", commit)
	}

	turn.Clarification.Status = assistantClarificationStatusExhausted
	turn.Phase = assistantPhaseFailed
	commit = assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, Turn: turn})
	if commit.Allowed || !errors.Is(commit.Error, errAssistantClarificationRoundsExhausted) {
		t.Fatalf("expected rounds exhausted, got=%+v", commit)
	}

	turn.Clarification.Status = assistantClarificationStatusAborted
	turn.Clarification.ExitTo = assistantClarificationExitManualHint
	commit = assistantEvaluateActionGate(assistantActionGateInput{Stage: assistantActionStageCommit, Action: spec, Turn: turn})
	if commit.Allowed || !errors.Is(commit.Error, errAssistantManualHintRequired) {
		t.Fatalf("expected manual hint required, got=%+v", commit)
	}
}

func TestAssistantClarificationPolicy_HelperCoverage(t *testing.T) {
	t.Run("kind helpers", func(t *testing.T) {
		if got := assistantClarificationKindAwaitPhase(assistantClarificationKindMissingSlots); got != assistantPhaseAwaitMissingFields {
			t.Fatalf("unexpected await phase=%q", got)
		}
		if got := assistantClarificationKindAwaitPhase(assistantClarificationKindCandidatePick); got != assistantPhaseAwaitCandidatePick {
			t.Fatalf("unexpected await phase=%q", got)
		}
		if got := assistantClarificationKindAwaitPhase(assistantClarificationKindCandidateConfirm); got != assistantPhaseAwaitCandidateConfirm {
			t.Fatalf("unexpected await phase=%q", got)
		}
		if got := assistantClarificationKindAwaitPhase(assistantClarificationKindIntentDisambiguate); got != assistantPhaseAwaitClarification {
			t.Fatalf("unexpected await phase=%q", got)
		}
		if got := assistantClarificationKindAwaitPhase(assistantClarificationKindFormatConfirm); got != assistantPhaseAwaitClarification {
			t.Fatalf("unexpected await phase=%q", got)
		}
		if got := assistantClarificationKindAwaitPhase("unknown"); got != "" {
			t.Fatalf("unexpected await phase for unknown=%q", got)
		}

		if got := assistantClarificationKindMaxRounds(assistantClarificationKindIntentDisambiguate); got != assistantClarificationMaxRoundsIntentDisambiguate {
			t.Fatalf("unexpected max rounds=%d", got)
		}
		if got := assistantClarificationKindMaxRounds(assistantClarificationKindCandidatePick); got != assistantClarificationMaxRoundsCandidatePick {
			t.Fatalf("unexpected max rounds=%d", got)
		}
		if got := assistantClarificationKindMaxRounds(assistantClarificationKindCandidateConfirm); got != assistantClarificationMaxRoundsCandidateConfirm {
			t.Fatalf("unexpected max rounds=%d", got)
		}
		if got := assistantClarificationKindMaxRounds(assistantClarificationKindFormatConfirm); got != assistantClarificationMaxRoundsFormatConfirm {
			t.Fatalf("unexpected max rounds=%d", got)
		}
		if got := assistantClarificationKindMaxRounds(assistantClarificationKindMissingSlots); got != assistantClarificationMaxRoundsMissingSlots {
			t.Fatalf("unexpected max rounds=%d", got)
		}
		if got := assistantClarificationKindMaxRounds("unknown"); got != 0 {
			t.Fatalf("unexpected max rounds unknown=%d", got)
		}
	})

	t.Run("decision and turn helpers", func(t *testing.T) {
		if assistantClarificationDecisionPresent(nil) {
			t.Fatal("nil decision should be absent")
		}
		if !assistantClarificationDecisionPresent(&assistantClarificationDecision{Status: assistantClarificationStatusOpen}) {
			t.Fatal("status should mark decision present")
		}
		if open := assistantTurnOpenClarification(nil); open != nil {
			t.Fatalf("nil turn open=%+v", open)
		}
		if open := assistantTurnOpenClarification(&assistantTurn{}); open != nil {
			t.Fatalf("turn without clarification open=%+v", open)
		}
		closed := &assistantTurn{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusResolved}}
		if open := assistantTurnOpenClarification(closed); open != nil {
			t.Fatalf("resolved clarification should not be open=%+v", open)
		}
		openTurn := &assistantTurn{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen}}
		if open := assistantTurnOpenClarification(openTurn); open == nil {
			t.Fatal("open clarification expected")
		}
		if !assistantTurnHasOpenClarification(openTurn) {
			t.Fatal("expected open clarification")
		}
	})

	t.Run("runtime derived helpers", func(t *testing.T) {
		runtime := &assistantKnowledgeRuntime{
			routeCatalog: assistantIntentRouteCatalog{
				Entries: []assistantIntentRouteEntry{
					{
						IntentID:                "org.orgunit_create",
						ActionID:                assistantIntentCreateOrgUnit,
						RequiredSlots:           []string{"parent_ref_text", " entity_name ", ""},
						ClarificationTemplateID: "tpl.create",
					},
					{
						IntentID:                "org.orgunit_move",
						ActionID:                assistantIntentMoveOrgUnit,
						RequiredSlots:           []string{"org_code", "new_parent_ref_text", "effective_date"},
						ClarificationTemplateID: "tpl.move",
					},
				},
			},
		}
		if slots := assistantClarificationRequiredSlots(nil, assistantIntentRouteDecision{}, assistantIntentSpec{}); slots != nil {
			t.Fatalf("nil runtime slots=%v", slots)
		}
		slots := assistantClarificationRequiredSlots(runtime, assistantIntentRouteDecision{IntentID: "org.orgunit_create"}, assistantIntentSpec{Action: assistantIntentCreateOrgUnit})
		if len(slots) != 2 || slots[0] != "entity_name" || slots[1] != "parent_ref_text" {
			t.Fatalf("unexpected required slots=%v", slots)
		}
		intentFallback := assistantClarificationRequiredSlots(runtime, assistantIntentRouteDecision{}, assistantIntentSpec{IntentID: "org.orgunit_move", Action: assistantIntentPlanOnly})
		if len(intentFallback) != 3 {
			t.Fatalf("expected fallback intent slots, got=%v", intentFallback)
		}
		if slots := assistantClarificationRequiredSlots(runtime, assistantIntentRouteDecision{IntentID: "org.orgunit_create"}, assistantIntentSpec{Action: assistantIntentMoveOrgUnit}); slots != nil {
			t.Fatalf("action mismatch should skip matched intent entry, got=%v", slots)
		}
		if slots := assistantClarificationRequiredSlots(runtime, assistantIntentRouteDecision{IntentID: "missing"}, assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); slots != nil {
			t.Fatalf("unmatched slots=%v", slots)
		}

		if tpl := assistantClarificationPromptTemplate(nil, assistantIntentRouteDecision{}, assistantIntentSpec{}); tpl != "" {
			t.Fatalf("nil runtime template=%q", tpl)
		}
		if tpl := assistantClarificationPromptTemplate(runtime, assistantIntentRouteDecision{IntentID: "org.orgunit_create"}, assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); tpl != "tpl.create" {
			t.Fatalf("template mismatch=%q", tpl)
		}
		if tpl := assistantClarificationPromptTemplate(runtime, assistantIntentRouteDecision{}, assistantIntentSpec{IntentID: "org.orgunit_move", Action: assistantIntentPlanOnly}); tpl != "tpl.move" {
			t.Fatalf("fallback template mismatch=%q", tpl)
		}
		if tpl := assistantClarificationPromptTemplate(runtime, assistantIntentRouteDecision{IntentID: "org.orgunit_create"}, assistantIntentSpec{Action: assistantIntentMoveOrgUnit}); tpl != "" {
			t.Fatalf("action mismatch should skip template, got=%q", tpl)
		}
		if tpl := assistantClarificationPromptTemplate(runtime, assistantIntentRouteDecision{IntentID: "missing"}, assistantIntentSpec{}); tpl != "" {
			t.Fatalf("unexpected template=%q", tpl)
		}
	})

	t.Run("action helpers", func(t *testing.T) {
		candidates := assistantClarificationActionCandidates(
			"请创建并移动组织",
			assistantIntentRouteDecision{CandidateActionIDs: []string{assistantIntentMoveOrgUnit}},
			assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
		)
		if len(candidates) != 2 {
			t.Fatalf("unexpected action candidates=%v", candidates)
		}
		if assistantClarificationActionStable(assistantIntentSpec{Action: assistantIntentPlanOnly}, assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction}) {
			t.Fatal("plan_only should not be stable")
		}
		if assistantClarificationActionStable(assistantIntentSpec{Action: assistantIntentCreateOrgUnit, RouteKind: assistantRouteKindKnowledgeQA}, assistantIntentRouteDecision{}) {
			t.Fatal("non-business route should not be stable")
		}
		if assistantClarificationActionStable(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, CandidateActionIDs: []string{assistantIntentCreateOrgUnit, assistantIntentMoveOrgUnit}}) {
			t.Fatal("multiple candidate actions should be unstable")
		}
		if !assistantClarificationActionStable(assistantIntentSpec{Action: assistantIntentCreateOrgUnit}, assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, CandidateActionIDs: []string{assistantIntentCreateOrgUnit}}) {
			t.Fatal("single business action should be stable")
		}
	})

	t.Run("slot and progress helpers", func(t *testing.T) {
		if slot := assistantClarificationCurrentSlot([]string{"entity_name"}, nil); slot != nil {
			t.Fatalf("unexpected current slot=%v", slot)
		}
		if slot := assistantClarificationCurrentSlot([]string{"parent_ref_text", "entity_name"}, []string{"entity_name", "effective_date"}); len(slot) != 1 || slot[0] != "entity_name" {
			t.Fatalf("unexpected prioritized slot=%v", slot)
		}
		if slot := assistantClarificationCurrentSlot([]string{"", "entity_name"}, []string{"", "entity_name"}); len(slot) != 1 || slot[0] != "entity_name" {
			t.Fatalf("unexpected slot with empty entries=%v", slot)
		}
		if slot := assistantClarificationCurrentSlot([]string{"parent_ref_text"}, []string{" unknown "}); len(slot) != 1 || slot[0] != "unknown" {
			t.Fatalf("unexpected fallback slot=%v", slot)
		}

		prev := &assistantClarificationDecision{MissingSlots: []string{"a", "b"}}
		if assistantClarificationDecisionProgressed(nil, &assistantClarificationDecision{}) {
			t.Fatal("nil prev should not progress")
		}
		if !assistantClarificationDecisionProgressed(prev, nil) {
			t.Fatal("nil next should count as progress")
		}
		if !assistantClarificationDecisionProgressed(prev, &assistantClarificationDecision{MissingSlots: []string{"a"}}) {
			t.Fatal("reduced missing slots should progress")
		}
		if !assistantClarificationDecisionProgressed(&assistantClarificationDecision{CandidateIDs: []string{"1", "2"}}, &assistantClarificationDecision{CandidateIDs: []string{"1"}}) {
			t.Fatal("candidate narrowing should progress")
		}
		if !assistantClarificationDecisionProgressed(&assistantClarificationDecision{CandidateActionIDs: []string{"a", "b"}}, &assistantClarificationDecision{CandidateActionIDs: []string{"a"}}) {
			t.Fatal("action narrowing should progress")
		}
		if assistantClarificationDecisionProgressed(&assistantClarificationDecision{MissingSlots: []string{"a"}}, &assistantClarificationDecision{MissingSlots: []string{"a"}}) {
			t.Fatal("unchanged slots should not progress")
		}

		if got := assistantClarificationExitForKind(assistantClarificationKindMissingSlots); got != assistantClarificationExitBusinessResume {
			t.Fatalf("unexpected exit=%q", got)
		}
		if got := assistantClarificationExitForKind("unknown"); got != assistantClarificationExitUncertain {
			t.Fatalf("unexpected unknown exit=%q", got)
		}
	})

	t.Run("finalize decision", func(t *testing.T) {
		runtime := assistantClarificationRuntimeFixture()
		route := assistantClarificationRouteDecisionFixture()
		input := assistantClarificationBuildInput{
			Runtime:       runtime,
			RouteDecision: route,
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
		}
		if out := assistantFinalizeClarificationDecision(nil, input); out != nil {
			t.Fatalf("nil decision finalize=%+v", out)
		}

		base := &assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			RequiredSlots:     []string{" parent_ref_text ", "parent_ref_text"},
			MissingSlots:      []string{" entity_name "},
			CandidateIDs:      []string{" A ", "A"},
			ReasonCodes:       []string{" reason ", "reason"},
		}
		final := assistantFinalizeClarificationDecision(base, input)
		if final.Status != assistantClarificationStatusOpen || final.CurrentRound != 1 || final.MaxRounds != assistantClarificationMaxRoundsMissingSlots || final.ExitTo == "" || final.PromptTemplateID == "" {
			t.Fatalf("unexpected finalized decision=%+v", final)
		}
		if final.KnowledgeSnapshotDigest != route.KnowledgeSnapshotDigest || final.RouteCatalogVersion != route.RouteCatalogVersion {
			t.Fatalf("knowledge snapshot not copied=%+v", final)
		}

		prev := &assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			Status:            assistantClarificationStatusOpen,
			CurrentRound:      1,
			MaxRounds:         3,
			ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot},
		}
		next := assistantFinalizeClarificationDecision(&assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			MissingSlots:      []string{"entity_name"},
			ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot},
		}, assistantClarificationBuildInput{Runtime: runtime, RouteDecision: route, PendingClarification: prev})
		if next.CurrentRound != 2 || !assistantSliceHas(next.ReasonCodes, assistantClarificationReasonNoProgress) {
			t.Fatalf("expected no-progress round bump, got=%+v", next)
		}

		aborted := assistantFinalizeClarificationDecision(&assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			MissingSlots:      []string{"entity_name"},
			ReasonCodes:       []string{assistantClarificationReasonMissingRequiredSlot},
		}, assistantClarificationBuildInput{
			Runtime:       runtime,
			RouteDecision: route,
			PendingClarification: &assistantClarificationDecision{
				ClarificationKind: assistantClarificationKindMissingSlots,
				Status:            assistantClarificationStatusOpen,
				CurrentRound:      2,
				MaxRounds:         3,
				ReasonCodes:       []string{assistantClarificationReasonNoProgress},
			},
		})
		if aborted.Status != assistantClarificationStatusAborted || aborted.ExitTo != assistantClarificationExitManualHint {
			t.Fatalf("expected aborted decision, got=%+v", aborted)
		}

		progressed := assistantFinalizeClarificationDecision(&assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			MissingSlots:      []string{"entity_name"},
		}, assistantClarificationBuildInput{
			Runtime:              runtime,
			RouteDecision:        route,
			ResumeProgress:       true,
			PendingClarification: &assistantClarificationDecision{ClarificationKind: assistantClarificationKindMissingSlots, Status: assistantClarificationStatusOpen, CurrentRound: 2},
		})
		if progressed.CurrentRound != 2 || assistantSliceHas(progressed.ReasonCodes, assistantClarificationReasonNoProgress) {
			t.Fatalf("resume progress should keep round, got=%+v", progressed)
		}

		exhausted := assistantFinalizeClarificationDecision(&assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindCandidateConfirm,
		}, assistantClarificationBuildInput{
			Runtime:       runtime,
			RouteDecision: route,
			PendingClarification: &assistantClarificationDecision{
				ClarificationKind: assistantClarificationKindCandidateConfirm,
				Status:            assistantClarificationStatusOpen,
				CurrentRound:      1,
			},
		})
		if exhausted.Status != assistantClarificationStatusExhausted || exhausted.ExitTo != assistantClarificationExitUncertain || !assistantSliceHas(exhausted.ReasonCodes, assistantClarificationReasonRoundsExhausted) {
			t.Fatalf("expected exhausted decision, got=%+v", exhausted)
		}

		unknownKind := assistantFinalizeClarificationDecision(&assistantClarificationDecision{
			ClarificationKind: "unknown_kind",
		}, input)
		if unknownKind.MaxRounds != 1 {
			t.Fatalf("unknown kind should fallback max_rounds=1, got=%+v", unknownKind)
		}

		baseRoundFallback := assistantFinalizeClarificationDecision(&assistantClarificationDecision{
			ClarificationKind: assistantClarificationKindMissingSlots,
			MissingSlots:      []string{"entity_name"},
		}, assistantClarificationBuildInput{
			Runtime:       runtime,
			RouteDecision: route,
			PendingClarification: &assistantClarificationDecision{
				ClarificationKind: assistantClarificationKindMissingSlots,
				Status:            assistantClarificationStatusOpen,
				CurrentRound:      0,
			},
		})
		if baseRoundFallback.CurrentRound != 2 {
			t.Fatalf("base round fallback expected 2, got=%+v", baseRoundFallback)
		}
	})

	t.Run("build decision branches", func(t *testing.T) {
		runtime := assistantClarificationRuntimeFixture()
		route := assistantClarificationRouteDecisionFixture()

		disambiguate := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			UserInput: "创建并移动组织",
			Intent: assistantIntentSpec{
				Action: assistantIntentPlanOnly,
			},
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindUncertain,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit, assistantIntentMoveOrgUnit, assistantIntentRenameOrgUnit},
				ClarificationRequired:   true,
				ConfidenceBand:          assistantRouteConfidenceLow,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
			},
			Runtime: runtime,
		})
		if disambiguate == nil || disambiguate.ClarificationKind != assistantClarificationKindIntentDisambiguate || len(disambiguate.CandidateActionIDs) != 2 {
			t.Fatalf("unexpected disambiguate=%+v", disambiguate)
		}

		pick := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			RouteDecision: route,
			DryRun:        assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
			Candidates:    []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}},
			Runtime:       runtime,
		})
		if pick == nil || pick.ClarificationKind != assistantClarificationKindCandidatePick {
			t.Fatalf("unexpected candidate pick=%+v", pick)
		}

		confirm := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			Intent:              assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			RouteDecision:       route,
			DryRun:              assistantDryRunResult{ValidationErrors: []string{"candidate_confirmation_required"}},
			Candidates:          []assistantCandidate{{CandidateID: "A"}, {CandidateID: "B"}},
			SelectedCandidateID: "A",
			Runtime:             runtime,
		})
		if confirm == nil || confirm.ClarificationKind != assistantClarificationKindCandidateConfirm {
			t.Fatalf("unexpected candidate confirm=%+v", confirm)
		}

		formatByValidation := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			Intent:        assistantIntentSpec{Action: assistantIntentMoveOrgUnit},
			RouteDecision: assistantIntentRouteDecision{RouteKind: assistantRouteKindBusinessAction, IntentID: "org.orgunit_move", CandidateActionIDs: []string{assistantIntentMoveOrgUnit}},
			DryRun:        assistantDryRunResult{ValidationErrors: []string{"invalid_target_effective_date_format"}},
			Runtime: &assistantKnowledgeRuntime{
				routeCatalog: assistantIntentRouteCatalog{Entries: []assistantIntentRouteEntry{
					{IntentID: "org.orgunit_move", ActionID: assistantIntentMoveOrgUnit, RequiredSlots: []string{"target_effective_date"}, ClarificationTemplateID: "tpl.move"},
				}},
			},
		})
		if formatByValidation == nil || formatByValidation.ClarificationKind != assistantClarificationKindFormatConfirm || len(formatByValidation.MissingSlots) != 1 || formatByValidation.MissingSlots[0] != "target_effective_date" {
			t.Fatalf("unexpected format confirmation=%+v", formatByValidation)
		}

		formatByRelative := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			UserInput:     "明天生效",
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: ""},
			RouteDecision: route,
			DryRun:        assistantDryRunResult{},
			Runtime:       runtime,
		})
		if formatByRelative == nil || formatByRelative.ClarificationKind != assistantClarificationKindFormatConfirm {
			t.Fatalf("unexpected relative-date format=%+v", formatByRelative)
		}

		missing := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			Intent:        assistantIntentSpec{Action: assistantIntentCreateOrgUnit},
			RouteDecision: route,
			DryRun:        assistantDryRunResult{ValidationErrors: []string{"missing_entity_name"}},
			Runtime:       runtime,
		})
		if missing == nil || missing.ClarificationKind != assistantClarificationKindMissingSlots {
			t.Fatalf("unexpected missing slots=%+v", missing)
		}

		none := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			Intent: assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "p", EntityName: "n", EffectiveDate: "2026-01-01"},
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               assistantRouteKindBusinessAction,
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ConfidenceBand:          assistantRouteConfidenceHigh,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
			},
			Runtime: runtime,
		})
		if none != nil {
			t.Fatalf("expected no clarification, got=%+v", none)
		}

		routeKindFallback := assistantBuildClarificationDecision(assistantClarificationBuildInput{
			UserInput: "请处理组织",
			Intent: assistantIntentSpec{
				Action:    assistantIntentPlanOnly,
				RouteKind: assistantRouteKindUncertain,
			},
			RouteDecision: assistantIntentRouteDecision{
				RouteKind:               "",
				IntentID:                "org.orgunit_create",
				CandidateActionIDs:      []string{assistantIntentCreateOrgUnit},
				ClarificationRequired:   true,
				ConfidenceBand:          assistantRouteConfidenceLow,
				RouteCatalogVersion:     "v1",
				KnowledgeSnapshotDigest: "d",
				ResolverContractVersion: "r",
				DecisionSource:          assistantRouteDecisionSourceKnowledgeRuntimeV1,
			},
			Runtime: runtime,
		})
		if routeKindFallback == nil || routeKindFallback.ClarificationKind != assistantClarificationKindIntentDisambiguate {
			t.Fatalf("route-kind fallback should trigger disambiguation, got=%+v", routeKindFallback)
		}
	})
}

func TestAssistantClarificationPolicy_ParsingAndResumeCoverage(t *testing.T) {
	t.Run("date parsing helpers", func(t *testing.T) {
		if !assistantDateISOYMD("2026-03-11") || assistantDateISOYMD("2026/03/11") || assistantDateISOYMD("") {
			t.Fatal("date iso validation mismatch")
		}
		if !assistantContainsRelativeDateToken("下个月一号生效") || assistantContainsRelativeDateToken("普通文本") {
			t.Fatal("relative token detection mismatch")
		}
		now := time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC)
		if got, ok := assistantNormalizeDateFromInput("2026-03-20", now); !ok || got != "2026-03-20" {
			t.Fatalf("iso extract got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("今天生效", now); !ok || got != "2026-03-11" {
			t.Fatalf("today normalize got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("明天生效", now); !ok || got != "2026-03-12" {
			t.Fatalf("tomorrow normalize got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("后天生效", now); !ok || got != "2026-03-13" {
			t.Fatalf("day after normalize got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("下个月1号", now); !ok || got != "2026-04-01" {
			t.Fatalf("next month normalize got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("今天", time.Time{}); !ok || !assistantDateISOYMD(got) {
			t.Fatalf("zero-time normalize got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("无法识别", now); ok || got != "" {
			t.Fatalf("unexpected normalize result got=%q ok=%v", got, ok)
		}
		if got, ok := assistantNormalizeDateFromInput("", now); ok || got != "" {
			t.Fatalf("empty input normalize got=%q ok=%v", got, ok)
		}
	})

	t.Run("candidate resolve and confirm parse", func(t *testing.T) {
		candidates := []assistantCandidate{
			{CandidateID: "ID-A", CandidateCode: "CODE-A", Name: "鲜花组织"},
			{CandidateID: "ID-B", CandidateCode: "CODE-B", Name: "花店组织"},
		}
		if id, ok := assistantResolveCandidateSelection("", candidates); ok || id != "" {
			t.Fatalf("empty input resolved=%q ok=%v", id, ok)
		}
		if id, ok := assistantResolveCandidateSelection("id-a", candidates); !ok || id != "ID-A" {
			t.Fatalf("id resolve=%q ok=%v", id, ok)
		}
		if id, ok := assistantResolveCandidateSelection("code-b", candidates); !ok || id != "ID-B" {
			t.Fatalf("code resolve=%q ok=%v", id, ok)
		}
		if id, ok := assistantResolveCandidateSelection("请选鲜花组织", candidates); !ok || id != "ID-A" {
			t.Fatalf("name resolve=%q ok=%v", id, ok)
		}
		if id, ok := assistantResolveCandidateSelection("鲜花组织和花店组织都可以", candidates); ok || id != "" {
			t.Fatalf("ambiguous resolve=%q ok=%v", id, ok)
		}
		if id, ok := assistantResolveCandidateSelection("未知组织", candidates); ok || id != "" {
			t.Fatalf("unknown resolve=%q ok=%v", id, ok)
		}
		if id, ok := assistantResolveCandidateSelection("鲜花组织", []assistantCandidate{{CandidateID: "X", Name: " "}, {CandidateID: "Y", Name: "鲜花组织"}}); !ok || id != "Y" {
			t.Fatalf("empty-name candidate should be skipped, got=%q ok=%v", id, ok)
		}

		if c, d := assistantParseCandidateConfirmation(""); c || d {
			t.Fatalf("empty parse confirmed=%v denied=%v", c, d)
		}
		if c, d := assistantParseCandidateConfirmation("不是这个"); c || !d {
			t.Fatalf("deny parse confirmed=%v denied=%v", c, d)
		}
		if c, d := assistantParseCandidateConfirmation("好的，确认"); !c || d {
			t.Fatalf("confirm parse confirmed=%v denied=%v", c, d)
		}
		if c, d := assistantParseCandidateConfirmation("继续"); c || d {
			t.Fatalf("neutral parse confirmed=%v denied=%v", c, d)
		}
	})

	t.Run("resume from clarification branches", func(t *testing.T) {
		baseIntent := assistantIntentSpec{Action: assistantIntentPlanOnly}
		if out := assistantResumeFromClarification(nil, "创建", baseIntent); out.Progress {
			t.Fatalf("nil pending should not progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusResolved}}, "创建", baseIntent); out.Progress {
			t.Fatalf("closed clarification should not progress=%+v", out)
		}

		intentDisambiguate := &assistantTurn{Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindIntentDisambiguate}}
		if out := assistantResumeFromClarification(intentDisambiguate, "请创建组织", baseIntent); !out.Progress || out.Intent.Action != assistantIntentCreateOrgUnit {
			t.Fatalf("intent disambiguate result=%+v", out)
		}

		candidatePick := &assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindCandidatePick},
			Candidates:    []assistantCandidate{{CandidateID: "A", CandidateCode: "CODE-A", Name: "鲜花组织"}},
		}
		if out := assistantResumeFromClarification(candidatePick, "code-a", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); !out.Progress || out.SelectedCandidateID != "A" {
			t.Fatalf("candidate pick result=%+v", out)
		}

		candidateConfirm := &assistantTurn{
			Clarification:       &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindCandidateConfirm},
			SelectedCandidateID: "A",
		}
		if out := assistantResumeFromClarification(candidateConfirm, "确认", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); !out.Progress || out.ResolvedCandidateID != "A" {
			t.Fatalf("candidate confirm result=%+v", out)
		}
		if out := assistantResumeFromClarification(candidateConfirm, "不是", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); out.Progress {
			t.Fatalf("candidate deny should not progress=%+v", out)
		}

		formatTarget := &assistantTurn{
			Clarification: &assistantClarificationDecision{
				Status:            assistantClarificationStatusOpen,
				ClarificationKind: assistantClarificationKindFormatConfirm,
				MissingSlots:      []string{"target_effective_date"},
			},
		}
		if out := assistantResumeFromClarification(formatTarget, "明天", assistantIntentSpec{Action: assistantIntentCorrectOrgUnit}); !out.Progress || out.Intent.TargetEffectiveDate == "" {
			t.Fatalf("format target resume=%+v", out)
		}
		formatDefault := &assistantTurn{
			Clarification: &assistantClarificationDecision{
				Status:            assistantClarificationStatusOpen,
				ClarificationKind: assistantClarificationKindFormatConfirm,
			},
		}
		if out := assistantResumeFromClarification(formatDefault, "后天", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); !out.Progress || out.Intent.EffectiveDate == "" {
			t.Fatalf("format default resume=%+v", out)
		}

		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"effective_date"}},
		}, "今天", assistantIntentSpec{Action: assistantIntentCreateOrgUnit}); !out.Progress || out.Intent.EffectiveDate == "" {
			t.Fatalf("missing effective_date normalize=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"effective_date"}},
		}, "任意输入", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EffectiveDate: "2026-03-20"}); !out.Progress {
			t.Fatalf("existing effective_date should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"target_effective_date"}},
		}, "明天", assistantIntentSpec{Action: assistantIntentCorrectOrgUnit}); !out.Progress || out.Intent.TargetEffectiveDate == "" {
			t.Fatalf("missing target normalize=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"target_effective_date"}},
		}, "任意输入", assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, TargetEffectiveDate: "2026-03-30"}); !out.Progress {
			t.Fatalf("existing target_effective_date should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"entity_name"}},
		}, "无关输入", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, EntityName: "运营部"}); !out.Progress {
			t.Fatalf("entity_name should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"parent_ref_text"}},
		}, "无关输入", assistantIntentSpec{Action: assistantIntentCreateOrgUnit, ParentRefText: "鲜花组织"}); !out.Progress {
			t.Fatalf("parent_ref_text should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"new_parent_ref_text"}},
		}, "无关输入", assistantIntentSpec{Action: assistantIntentMoveOrgUnit, NewParentRefText: "新上级"}); !out.Progress {
			t.Fatalf("new_parent_ref_text should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"org_code"}},
		}, "无关输入", assistantIntentSpec{Action: assistantIntentRenameOrgUnit, OrgCode: "ORG-1"}); !out.Progress {
			t.Fatalf("org_code should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"new_name"}},
		}, "无关输入", assistantIntentSpec{Action: assistantIntentRenameOrgUnit, NewName: "新名字"}); !out.Progress {
			t.Fatalf("new_name should progress=%+v", out)
		}
		if out := assistantResumeFromClarification(&assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots, MissingSlots: []string{"change_fields"}},
		}, "无关输入", assistantIntentSpec{Action: assistantIntentCorrectOrgUnit, NewParentRefText: "新上级"}); !out.Progress {
			t.Fatalf("change_fields should progress=%+v", out)
		}
		fallbackMissing := &assistantTurn{
			Clarification: &assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: assistantClarificationKindMissingSlots},
			DryRun:        assistantDryRunResult{ValidationErrors: []string{"missing_org_code"}},
		}
		if out := assistantResumeFromClarification(fallbackMissing, "无关输入", assistantIntentSpec{Action: assistantIntentRenameOrgUnit, OrgCode: "ORG-2"}); !out.Progress {
			t.Fatalf("fallback missing slots should progress=%+v", out)
		}
	})

	t.Run("runtime validation helpers", func(t *testing.T) {
		if err := assistantValidateClarificationDecision(nil); err != nil {
			t.Fatalf("nil clarification should be valid err=%v", err)
		}
		if err := assistantValidateClarificationDecision(&assistantClarificationDecision{Status: "bad", ClarificationKind: assistantClarificationKindMissingSlots}); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("invalid status err=%v", err)
		}
		if err := assistantValidateClarificationDecision(&assistantClarificationDecision{Status: assistantClarificationStatusOpen, ClarificationKind: "bad"}); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("invalid kind err=%v", err)
		}
		if err := assistantValidateClarificationDecision(&assistantClarificationDecision{
			Status:                  assistantClarificationStatusOpen,
			ClarificationKind:       assistantClarificationKindMissingSlots,
			AwaitPhase:              assistantPhaseAwaitClarification,
			MaxRounds:               3,
			CurrentRound:            1,
			ExitTo:                  assistantClarificationExitBusinessResume,
			KnowledgeSnapshotDigest: "d",
			RouteCatalogVersion:     "v1",
		}); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("invalid await phase err=%v", err)
		}
		if err := assistantValidateClarificationDecision(&assistantClarificationDecision{
			Status:                  assistantClarificationStatusOpen,
			ClarificationKind:       assistantClarificationKindMissingSlots,
			AwaitPhase:              assistantPhaseAwaitMissingFields,
			MaxRounds:               0,
			CurrentRound:            1,
			ExitTo:                  assistantClarificationExitBusinessResume,
			KnowledgeSnapshotDigest: "d",
			RouteCatalogVersion:     "v1",
		}); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("invalid rounds err=%v", err)
		}
		if err := assistantValidateClarificationDecision(&assistantClarificationDecision{
			Status:                  assistantClarificationStatusOpen,
			ClarificationKind:       assistantClarificationKindMissingSlots,
			AwaitPhase:              assistantPhaseAwaitMissingFields,
			MaxRounds:               3,
			CurrentRound:            1,
			ExitTo:                  "",
			KnowledgeSnapshotDigest: "d",
			RouteCatalogVersion:     "v1",
		}); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("missing required open fields err=%v", err)
		}
		if err := assistantValidateClarificationDecision(&assistantClarificationDecision{
			Status:            assistantClarificationStatusResolved,
			ClarificationKind: assistantClarificationKindMissingSlots,
		}); err != nil {
			t.Fatalf("resolved clarification should pass err=%v", err)
		}

		if err := assistantValidateTurnClarificationRuntime(nil); err != nil {
			t.Fatalf("nil turn validation err=%v", err)
		}
		invalidDecisionTurn := &assistantTurn{Clarification: &assistantClarificationDecision{Status: "bad", ClarificationKind: assistantClarificationKindMissingSlots}}
		if err := assistantValidateTurnClarificationRuntime(invalidDecisionTurn); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("invalid decision turn err=%v", err)
		}
		if err := assistantValidateTurnClarificationRuntime(&assistantTurn{State: assistantStateValidated}); err != nil {
			t.Fatalf("turn without clarification should pass err=%v", err)
		}
		missingPhaseTurn := &assistantTurn{
			State: assistantStateValidated,
			Clarification: &assistantClarificationDecision{
				Status:                  assistantClarificationStatusOpen,
				ClarificationKind:       "bad",
				AwaitPhase:              assistantPhaseAwaitMissingFields,
				MaxRounds:               1,
				CurrentRound:            1,
				ExitTo:                  assistantClarificationExitBusinessResume,
				KnowledgeSnapshotDigest: "d",
				RouteCatalogVersion:     "v1",
			},
			RouteDecision: assistantClarificationRouteDecisionFixture(),
		}
		if err := assistantValidateTurnClarificationRuntime(missingPhaseTurn); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("invalid kind expected runtime invalid err=%v", err)
		}
		phaseMismatchTurn := &assistantTurn{
			State: assistantStateValidated,
			Phase: assistantPhaseAwaitClarification,
			Clarification: &assistantClarificationDecision{
				Status:                  assistantClarificationStatusOpen,
				ClarificationKind:       assistantClarificationKindMissingSlots,
				AwaitPhase:              assistantPhaseAwaitMissingFields,
				MaxRounds:               2,
				CurrentRound:            1,
				ExitTo:                  assistantClarificationExitBusinessResume,
				KnowledgeSnapshotDigest: "d",
				RouteCatalogVersion:     "v1",
			},
			RouteDecision: assistantClarificationRouteDecisionFixture(),
		}
		if err := assistantValidateTurnClarificationRuntime(phaseMismatchTurn); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("phase mismatch err=%v", err)
		}
		missingRouteTurn := &assistantTurn{
			State: assistantStateValidated,
			Clarification: &assistantClarificationDecision{
				Status:                  assistantClarificationStatusOpen,
				ClarificationKind:       assistantClarificationKindMissingSlots,
				AwaitPhase:              assistantPhaseAwaitMissingFields,
				MaxRounds:               2,
				CurrentRound:            1,
				ExitTo:                  assistantClarificationExitBusinessResume,
				KnowledgeSnapshotDigest: "d",
				RouteCatalogVersion:     "v1",
			},
		}
		if err := assistantValidateTurnClarificationRuntime(missingRouteTurn); !errors.Is(err, errAssistantClarificationRuntimeInvalid) {
			t.Fatalf("missing route decision err=%v", err)
		}
		validTurn := &assistantTurn{
			State: assistantStateValidated,
			Phase: assistantPhaseAwaitMissingFields,
			Clarification: &assistantClarificationDecision{
				Status:                  assistantClarificationStatusOpen,
				ClarificationKind:       assistantClarificationKindMissingSlots,
				AwaitPhase:              assistantPhaseAwaitMissingFields,
				MaxRounds:               2,
				CurrentRound:            1,
				ExitTo:                  assistantClarificationExitBusinessResume,
				KnowledgeSnapshotDigest: "d",
				RouteCatalogVersion:     "v1",
			},
			RouteDecision: assistantClarificationRouteDecisionFixture(),
		}
		if err := assistantValidateTurnClarificationRuntime(validTurn); err != nil {
			t.Fatalf("valid turn runtime err=%v", err)
		}
	})

	t.Run("misc helpers", func(t *testing.T) {
		if assistantSliceHas([]string{"a"}, "") {
			t.Fatal("empty needle should not match")
		}
		if !assistantSliceHas([]string{" a "}, "a") {
			t.Fatal("trimmed match expected")
		}
		if assistantSliceHas([]string{"a"}, "b") {
			t.Fatal("unexpected match")
		}

		assistantApplyPlanKnowledgeSnapshot(nil, assistantIntentRouteDecision{}, nil)
		plan := assistantPlanSummary{}
		route := assistantIntentRouteDecision{
			KnowledgeSnapshotDigest: "d",
			RouteCatalogVersion:     "v1",
			ResolverContractVersion: "r1",
		}
		assistantApplyPlanKnowledgeSnapshot(&plan, route, nil)
		if plan.KnowledgeSnapshotDigest != "d" || plan.RouteCatalogVersion != "v1" || plan.ResolverContractVersion != "r1" {
			t.Fatalf("route snapshot apply failed plan=%+v", plan)
		}
		runtime := &assistantKnowledgeRuntime{ContextTemplateVersion: "ctx.v1", ReplyGuidanceVersion: "reply.v1"}
		assistantApplyPlanKnowledgeSnapshot(&plan, route, runtime)
		if plan.ContextTemplateVersion != "ctx.v1" || plan.ReplyGuidanceVersion != "reply.v1" {
			t.Fatalf("runtime snapshot apply failed plan=%+v", plan)
		}
	})
}
