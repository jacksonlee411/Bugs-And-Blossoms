package cubebox_test

import (
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestDecodePlannerOutcomeEnvelopeReadPlan(t *testing.T) {
	raw := `{"outcome":"READ_PLAN","plan":{"intent":"orgunit.list","confidence":0.9,"missing_params":[],"steps":[{"id":"step-1","api_key":"orgunit.list","params":{"as_of":"2026-04-25"},"depends_on":[]}],"explain_focus":[]}}`

	outcome, err := cubebox.DecodePlannerOutcome([]byte(raw))
	if err != nil {
		t.Fatalf("DecodePlannerOutcome err=%v", err)
	}
	if outcome.Type != cubebox.PlannerOutcomeReadPlan {
		t.Fatalf("unexpected outcome=%#v", outcome)
	}
	if got := outcome.Plan.Steps[0].Params["as_of"]; got != "2026-04-25" {
		t.Fatalf("unexpected plan params=%#v", outcome.Plan.Steps[0].Params)
	}
}

func TestDecodePlannerOutcomeEnvelopeClarify(t *testing.T) {
	raw := `{"outcome":"CLARIFY","missing_params":["as_of","as_of"," parent_org_code "],"clarifying_question":"请提供查询日期和上级组织编码。"}`

	outcome, err := cubebox.DecodePlannerOutcome([]byte(raw))
	if err != nil {
		t.Fatalf("DecodePlannerOutcome err=%v", err)
	}
	if outcome.Type != cubebox.PlannerOutcomeClarify {
		t.Fatalf("unexpected outcome=%#v", outcome)
	}
	if got, want := len(outcome.MissingParams), 2; got != want {
		t.Fatalf("expected deduped missing params length=%d got %#v", want, outcome.MissingParams)
	}
	if outcome.ClarifyingQuestion != "请提供查询日期和上级组织编码。" {
		t.Fatalf("unexpected question=%q", outcome.ClarifyingQuestion)
	}
}

func TestDecodePlannerOutcomeEnvelopeDoneAndNoQuery(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want cubebox.PlannerOutcomeType
	}{
		{name: "done", raw: `{"outcome":"DONE"}`, want: cubebox.PlannerOutcomeDone},
		{name: "no query", raw: `{"outcome":"NO_QUERY"}`, want: cubebox.PlannerOutcomeNoQuery},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			outcome, err := cubebox.DecodePlannerOutcome([]byte(tt.raw))
			if err != nil {
				t.Fatalf("DecodePlannerOutcome err=%v", err)
			}
			if outcome.Type != tt.want {
				t.Fatalf("unexpected outcome=%#v want=%s", outcome, tt.want)
			}
		})
	}
}

func TestDecodePlannerOutcomeCompatibilityBareReadPlanAndNoQuery(t *testing.T) {
	readPlan := `{"intent":"orgunit.list","confidence":0.9,"missing_params":[],"steps":[{"id":"step-1","api_key":"orgunit.list","params":{"as_of":"2026-04-25"},"depends_on":[]}],"explain_focus":[]}`
	outcome, err := cubebox.DecodePlannerOutcome([]byte(readPlan))
	if err != nil {
		t.Fatalf("DecodePlannerOutcome bare read plan err=%v", err)
	}
	if outcome.Type != cubebox.PlannerOutcomeReadPlan || outcome.CompatibilitySource != "bare_read_plan" {
		t.Fatalf("unexpected bare read plan outcome=%#v", outcome)
	}

	outcome, err = cubebox.DecodePlannerOutcome([]byte("NO_QUERY"))
	if err != nil {
		t.Fatalf("DecodePlannerOutcome bare NO_QUERY err=%v", err)
	}
	if outcome.Type != cubebox.PlannerOutcomeNoQuery || outcome.CompatibilitySource != "bare_no_query" {
		t.Fatalf("unexpected bare no query outcome=%#v", outcome)
	}
}

func TestDecodePlannerOutcomeCompatibilityBareClarifyingReadPlan(t *testing.T) {
	readPlan := `{"intent":"orgunit.list","confidence":0.4,"missing_params":["as_of"],"steps":[],"explain_focus":[],"clarifying_question":"请提供查询日期。"}`

	outcome, err := cubebox.DecodePlannerOutcome([]byte(readPlan))
	if err != nil {
		t.Fatalf("DecodePlannerOutcome bare clarifying read plan err=%v", err)
	}
	if outcome.Type != cubebox.PlannerOutcomeClarify {
		t.Fatalf("unexpected outcome=%#v", outcome)
	}
	if outcome.ClarifyingQuestion != "请提供查询日期。" {
		t.Fatalf("unexpected question=%q", outcome.ClarifyingQuestion)
	}
}

func TestDecodePlannerOutcomeRejectsInvalidPayloads(t *testing.T) {
	for _, raw := range []string{
		`DONE`,
		`plain text`,
		`{"outcome":"MAYBE"}`,
		`{"outcome":"DONE","plan":{"intent":"orgunit.list"}}`,
		`{"outcome":"NO_QUERY","clarifying_question":"够了"}`,
		`{"outcome":"READ_PLAN","missing_params":["as_of"],"plan":{"intent":"orgunit.list","confidence":0.9,"missing_params":[],"steps":[{"id":"step-1","api_key":"orgunit.list","params":{"as_of":"2026-04-25"},"depends_on":[]}],"explain_focus":[]}}`,
		`{"outcome":"CLARIFY","missing_params":["as_of"],"clarifying_question":"请提供日期。","plan":{"intent":"orgunit.list"}}`,
	} {
		_, err := cubebox.DecodePlannerOutcome([]byte(raw))
		if !errors.Is(err, cubebox.ErrPlannerOutcomeInvalid) {
			t.Fatalf("expected ErrPlannerOutcomeInvalid for %q, got %v", raw, err)
		}
	}
}
