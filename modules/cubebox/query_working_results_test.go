package cubebox_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestDefaultQueryLoopBudgetUsesEarlyStageExpandedLimits(t *testing.T) {
	budget := cubebox.DefaultQueryLoopBudget()
	if budget.MaxPlanningRounds != 80 {
		t.Fatalf("expected planning rounds 80, got %d", budget.MaxPlanningRounds)
	}
	if budget.MaxExecutedSteps != 160 {
		t.Fatalf("expected executed steps 160, got %d", budget.MaxExecutedSteps)
	}
	if budget.MaxWorkingResultItems != 1000 {
		t.Fatalf("expected working result items 1000, got %d", budget.MaxWorkingResultItems)
	}
	if budget.MaxRepeatedPlan != 2 {
		t.Fatalf("expected repeated plan budget 2, got %d", budget.MaxRepeatedPlan)
	}
}

func TestStepFingerprintSortsParamKeysAndNormalizesValues(t *testing.T) {
	left := cubebox.StepFingerprint(cubebox.ReadPlanStep{
		ExecutorKey: "orgunit.list",
		Params: map[string]any{
			"parent_org_code":  " 100000 ",
			"as_of":            "2026-04-25",
			"include_disabled": false,
		},
	})
	right := cubebox.StepFingerprint(cubebox.ReadPlanStep{
		ExecutorKey: "orgunit.list",
		Params: map[string]any{
			"include_disabled": false,
			"as_of":            "2026-04-25",
			"parent_org_code":  "100000",
		},
	})

	if left != right {
		t.Fatalf("expected stable fingerprint, left=%q right=%q", left, right)
	}
	if !strings.Contains(left, `as_of="2026-04-25"|include_disabled=false|parent_org_code="100000"`) {
		t.Fatalf("unexpected fingerprint=%q", left)
	}
}

func TestQueryWorkingResultsAppendPlanBuildsObservationLedger(t *testing.T) {
	state := cubebox.NewQueryWorkingResultsState("查组织树", cubebox.QueryLoopBudget{
		MaxPlanningRounds:     4,
		MaxExecutedSteps:      8,
		MaxWorkingResultItems: 1,
		MaxRepeatedPlan:       1,
	})
	state.NotePlanningRound()
	plan := cubebox.ReadPlan{
		Intent: "orgunit.list",
		Steps: []cubebox.ReadPlanStep{{
			ID:          "step-1",
			ExecutorKey: "orgunit.list",
			Params:      map[string]any{"as_of": "2026-04-25"},
		}},
	}
	state.AppendPlan(1, plan, []cubebox.ExecuteResult{{
		Payload: map[string]any{
			"as_of": "2026-04-25",
			"org_units": []map[string]any{
				{"org_code": "100000", "name": "总部", "has_children": true},
				{"org_code": "200000", "name": "华东", "has_children": false},
			},
		},
	}})

	snapshot := state.Snapshot()
	if snapshot.RoundIndex != 1 {
		t.Fatalf("unexpected round index=%d", snapshot.RoundIndex)
	}
	if snapshot.Budget.RemainingPlanningRounds != 3 || snapshot.Budget.RemainingExecutedSteps != 7 {
		t.Fatalf("unexpected budget=%#v", snapshot.Budget)
	}
	if len(snapshot.CompletedPlans) != 1 || len(snapshot.CompletedPlans[0].Steps) != 1 {
		t.Fatalf("unexpected completed plans=%#v", snapshot.CompletedPlans)
	}
	if snapshot.LatestObservation == nil || snapshot.LatestObservation.ItemCount != 2 || !snapshot.LatestObservation.Truncated {
		t.Fatalf("unexpected latest observation=%#v", snapshot.LatestObservation)
	}
	if got := len(snapshot.LatestObservation.Items); got != 1 {
		t.Fatalf("expected truncated latest items length 1, got %d", got)
	}
	if !state.HasExecution() || len(snapshot.ExecutedFingerprints) != 1 {
		t.Fatalf("expected executed fingerprint, snapshot=%#v", snapshot)
	}
}

func TestQueryWorkingResultsRepeatDetectionUsesBudget(t *testing.T) {
	state := cubebox.NewQueryWorkingResultsState("查组织树", cubebox.QueryLoopBudget{
		MaxPlanningRounds:     4,
		MaxExecutedSteps:      8,
		MaxWorkingResultItems: 50,
		MaxRepeatedPlan:       1,
	})
	step := cubebox.ReadPlanStep{ID: "step-1", ExecutorKey: "orgunit.list", Params: map[string]any{"as_of": "2026-04-25"}}
	fingerprint := cubebox.StepFingerprint(step)
	state.AppendPlan(1, cubebox.ReadPlan{Intent: "orgunit.list", Steps: []cubebox.ReadPlanStep{step}}, []cubebox.ExecuteResult{{Payload: map[string]any{"org_units": []any{}}}})

	if !state.HasExecuted(fingerprint) {
		t.Fatalf("expected fingerprint recorded")
	}
	if exceeded := state.NoteRepeat(fingerprint); exceeded {
		t.Fatalf("first repeat should be planner-visible, not terminal")
	}
	if exceeded := state.NoteRepeat(fingerprint); !exceeded {
		t.Fatalf("second repeat should exceed repeat budget")
	}
	if got := len(state.Snapshot().RepeatObservations); got != 2 {
		t.Fatalf("unexpected repeat observations=%d", got)
	}
}

func TestWorkingResultsPromptBlockStaysBusinessAgnostic(t *testing.T) {
	state := cubebox.NewQueryWorkingResultsState("查组织树", cubebox.DefaultQueryLoopBudget())
	state.NotePlanningRound()
	state.AppendPlan(1, cubebox.ReadPlan{
		Intent: "orgunit.list",
		Steps: []cubebox.ReadPlanStep{{
			ID:          "step-1",
			ExecutorKey: "orgunit.list",
			Params:      map[string]any{"as_of": "2026-04-25"},
		}},
	}, []cubebox.ExecuteResult{{
		Payload: map[string]any{"org_units": []map[string]any{{"org_code": "100000", "has_children": true}}},
	}})

	body := cubebox.WorkingResultsPromptBlock(state.Snapshot())
	if !json.Valid([]byte(body)) {
		t.Fatalf("expected valid json, got %s", body)
	}
	for _, forbidden := range []string{"remaining_parent_org_codes", "aggregated_facts", "winner", "resolved_entity"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("working_results leaked capability-specific field %q in %s", forbidden, body)
		}
	}
	for _, required := range []string{"working_results", "latest_observation", "executed_fingerprints", "has_children"} {
		if !strings.Contains(body, required) {
			t.Fatalf("expected %q in working results block=%s", required, body)
		}
	}
}

func TestWorkingResultsPromptBlockProjectsRecentHistoryWindow(t *testing.T) {
	snapshot := cubebox.QueryWorkingResults{
		RoundIndex:       80,
		OriginalUserGoal: "查整棵组织树",
		Budget: cubebox.QueryWorkingResultsBudget{
			MaxPlanningRounds:       80,
			RemainingPlanningRounds: 0,
			MaxExecutedSteps:        160,
			RemainingExecutedSteps:  0,
			MaxWorkingResultItems:   1000,
		},
		CompletedPlans:       make([]cubebox.QueryCompletedPlan, 0, 25),
		ExecutedFingerprints: make([]string, 0, 60),
		RepeatObservations:   make([]cubebox.QueryRepeatObservation, 0, 12),
		LatestObservation: &cubebox.QueryWorkingObservation{
			Round:             80,
			StepID:            "step-80",
			ExecutorKey:       "orgunit.list",
			ParamsFingerprint: `orgunit.list|as_of="2026-04-25"|parent_org_code="999999"`,
			Items:             make([]any, 0, 250),
			ItemCount:         250,
			Truncated:         false,
		},
	}
	for i := 1; i <= 250; i++ {
		snapshot.LatestObservation.Items = append(snapshot.LatestObservation.Items, map[string]any{
			"org_code":     "9" + strings.Repeat("0", i%5),
			"has_children": i%2 == 0,
		})
	}
	for i := 1; i <= 25; i++ {
		snapshot.CompletedPlans = append(snapshot.CompletedPlans, cubebox.QueryCompletedPlan{
			Round:  i,
			Intent: "orgunit.list",
			Steps: []cubebox.QueryCompletedPlanStep{{
				StepID:            "step-" + string(rune('A'+i-1)),
				ExecutorKey:       "orgunit.list",
				ParamsFingerprint: "fp-plan-" + strings.Repeat("x", i%3) + string(rune('A'+i-1)),
				ItemCount:         1,
			}},
		})
	}
	for i := 1; i <= 60; i++ {
		snapshot.ExecutedFingerprints = append(snapshot.ExecutedFingerprints, "fp-"+strings.Repeat("0", i/10)+string(rune('A'+(i-1)%26)))
	}
	for i := 1; i <= 12; i++ {
		snapshot.RepeatObservations = append(snapshot.RepeatObservations, cubebox.QueryRepeatObservation{
			Round:             i,
			ParamsFingerprint: "repeat-" + string(rune('A'+i-1)),
			Message:           "该查询步骤已执行过，请选择下一步或返回 DONE。",
		})
	}

	body := cubebox.WorkingResultsPromptBlock(snapshot)
	if !json.Valid([]byte(body)) {
		t.Fatalf("expected valid json, got %s", body)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("unmarshal prompt block: %v", err)
	}
	working, ok := envelope["working_results"].(map[string]any)
	if !ok {
		t.Fatalf("expected working_results object, got %#v", envelope["working_results"])
	}
	if got := int(working["executed_fingerprint_count"].(float64)); got != 60 {
		t.Fatalf("expected executed fingerprint count 60, got %d", got)
	}
	if got := int(working["repeat_observation_count"].(float64)); got != 12 {
		t.Fatalf("expected repeat observation count 12, got %d", got)
	}
	completed, ok := working["completed_plans"].([]any)
	if !ok || len(completed) != 20 {
		t.Fatalf("expected 20 completed plans in prompt projection, got %#v", working["completed_plans"])
	}
	fingerprints, ok := working["executed_fingerprints"].([]any)
	if !ok || len(fingerprints) != 60 {
		t.Fatalf("expected all 60 executed fingerprints in prompt projection, got %#v", working["executed_fingerprints"])
	}
	repeats, ok := working["repeat_observations"].([]any)
	if !ok || len(repeats) != 8 {
		t.Fatalf("expected 8 repeat observations in prompt projection, got %#v", working["repeat_observations"])
	}
	latest, ok := working["latest_observation"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest_observation object, got %#v", working["latest_observation"])
	}
	items, ok := latest["items"].([]any)
	if !ok || len(items) != 200 {
		t.Fatalf("expected latest observation items truncated to 200, got %#v", latest["items"])
	}
	if truncated, ok := latest["truncated"].(bool); !ok || !truncated {
		t.Fatalf("expected latest observation truncated=true, got %#v", latest["truncated"])
	}
}
