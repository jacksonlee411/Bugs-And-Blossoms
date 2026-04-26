package cubebox_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestStepFingerprintSortsParamKeysAndNormalizesValues(t *testing.T) {
	left := cubebox.StepFingerprint(cubebox.ReadPlanStep{
		APIKey: "orgunit.list",
		Params: map[string]any{
			"parent_org_code":  " 100000 ",
			"as_of":            "2026-04-25",
			"include_disabled": false,
		},
	})
	right := cubebox.StepFingerprint(cubebox.ReadPlanStep{
		APIKey: "orgunit.list",
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
			ID:     "step-1",
			APIKey: "orgunit.list",
			Params: map[string]any{"as_of": "2026-04-25"},
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
	step := cubebox.ReadPlanStep{ID: "step-1", APIKey: "orgunit.list", Params: map[string]any{"as_of": "2026-04-25"}}
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
			ID:     "step-1",
			APIKey: "orgunit.list",
			Params: map[string]any{"as_of": "2026-04-25"},
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
