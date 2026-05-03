package cubebox_test

import (
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestDecodePlannerOutcomeEnvelopeAPICalls(t *testing.T) {
	raw := `{"outcome":"API_CALLS","calls":[{"id":"step-1","method":"get","path":"org/api/org-units","params":{"as_of":"2026-04-25"},"depends_on":[]}]}`

	outcome, err := cubebox.DecodePlannerOutcome([]byte(raw))
	if err != nil {
		t.Fatalf("DecodePlannerOutcome err=%v", err)
	}
	if outcome.Type != cubebox.PlannerOutcomeAPICalls {
		t.Fatalf("unexpected outcome=%#v", outcome)
	}
	call := outcome.Calls.Calls[0]
	if call.Method != "GET" || call.Path != "/org/api/org-units" {
		t.Fatalf("unexpected call route=%#v", call)
	}
	if got := call.Params["as_of"]; got != "2026-04-25" {
		t.Fatalf("unexpected call params=%#v", call.Params)
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

func TestDecodePlannerOutcomeRejectsLegacyReadPlanShapes(t *testing.T) {
	for _, raw := range []string{
		`NO_QUERY`,
		`{"intent":"orgunit.list","confidence":0.9,"missing_params":[],"steps":[{"id":"step-1","executor_key":"orgunit.list","params":{"as_of":"2026-04-25"},"depends_on":[]}],"explain_focus":[]}`,
		`{"outcome":"READ_PLAN","plan":{"intent":"orgunit.list","confidence":0.9,"missing_params":[],"steps":[{"id":"step-1","executor_key":"orgunit.list","params":{"as_of":"2026-04-25"},"depends_on":[]}],"explain_focus":[]}}`,
		`{"outcome":"API_CALLS","plan":{"intent":"orgunit.list","steps":[]},"calls":[{"id":"step-1","method":"GET","path":"/org/api/org-units","params":{"as_of":"2026-04-25"},"depends_on":[]}]}`,
		`{"outcome":"API_CALLS","calls":[{"id":"step-1","method":"GET","path":"/org/api/org-units","executor_key":"orgunit.list","params":{"as_of":"2026-04-25"},"depends_on":[]}]}`,
	} {
		_, err := cubebox.DecodePlannerOutcome([]byte(raw))
		if err == nil {
			t.Fatalf("expected legacy payload rejected: %s", raw)
		}
	}
}

func TestDecodePlannerOutcomeRejectsInvalidPayloads(t *testing.T) {
	for _, raw := range []string{
		`DONE`,
		`plain text`,
		`{"outcome":"MAYBE"}`,
		`{"outcome":"DONE","calls":[]}`,
		`{"outcome":"NO_QUERY","clarifying_question":"够了"}`,
		`{"outcome":"API_CALLS","missing_params":["as_of"],"calls":[{"id":"step-1","method":"GET","path":"/org/api/org-units","params":{"as_of":"2026-04-25"},"depends_on":[]}]}`,
		`{"outcome":"CLARIFY","missing_params":["as_of"],"clarifying_question":"请提供日期。","calls":[]}`,
	} {
		_, err := cubebox.DecodePlannerOutcome([]byte(raw))
		if !errors.Is(err, cubebox.ErrPlannerOutcomeInvalid) {
			t.Fatalf("expected ErrPlannerOutcomeInvalid for %q, got %v", raw, err)
		}
	}
}
