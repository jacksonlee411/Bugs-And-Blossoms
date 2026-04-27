package cubebox

import "testing"

func TestBuildQueryEvidenceWindowProjectsResultListsSeparately(t *testing.T) {
	context := QueryContext{
		RecentCandidateGroups: []QueryCandidateGroup{
			{
				GroupID:         "resultgrp_finance",
				CandidateSource: "results",
				CandidateCount:  4,
				Candidates: []QueryCandidate{
					{Domain: "orgunit", EntityKey: "200001", Name: "财务部", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200002", Name: "财务一组", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200003", Name: "财务三组", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200004", Name: "财务四组", AsOf: "2026-04-27"},
				},
			},
		},
	}

	window := BuildQueryEvidenceWindow(context, "增加列出他们的组织路径", QueryEvidenceWindowBudget{
		MaxEntityObservations: 5,
		MaxOptionGroups:       5,
		MaxOptionsPerGroup:    3,
		MaxDialogueTurns:      5,
	})

	if got, want := len(window.Observations), 1; got != want {
		t.Fatalf("expected %d observation, got %#v", want, window.Observations)
	}
	group := window.Observations[0]
	if group.Kind != "result_list" {
		t.Fatalf("expected result_list observation, got %#v", group)
	}
	if got := group.ResultSummary["group_id"]; got != "resultgrp_finance" {
		t.Fatalf("unexpected group id=%#v", group.ResultSummary)
	}
	if got := group.ResultSummary["item_count"]; got != 4 {
		t.Fatalf("unexpected item count=%#v", group.ResultSummary)
	}
	items, ok := group.ResultSummary["items"].([]map[string]any)
	if !ok || len(items) != 3 {
		t.Fatalf("expected result list truncated to 3 items, got %#v", group.ResultSummary["items"])
	}
	if _, exists := group.ResultSummary["requires_explicit_user_choice"]; exists {
		t.Fatalf("result list must not require explicit user choice: %#v", group.ResultSummary)
	}
}
