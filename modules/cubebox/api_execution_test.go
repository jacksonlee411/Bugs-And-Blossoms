package cubebox

import "testing"

func TestProjectNarrationResultsUsesRawPayloadCopy(t *testing.T) {
	payload := map[string]any{
		"org_unit": map[string]any{
			"org_code": "1001",
			"name":     "总部",
		},
	}
	results := ProjectNarrationResults([]ExecuteResult{
		{
			Method:      "GET",
			Path:        "/org/api/org-units/details",
			OperationID: "orgunit.details",
			Payload:     payload,
			ConfirmedEntity: &QueryEntity{
				Domain:    "orgunit",
				EntityKey: "1001",
			},
		},
	})
	if len(results) != 1 || results[0].Domain != "orgunit" {
		t.Fatalf("unexpected narration results=%#v", results)
	}
	if results[0].Data["org_unit"] == nil {
		t.Fatalf("expected raw payload copied, got %#v", results[0].Data)
	}
	payload["mutated"] = true
	if _, ok := results[0].Data["mutated"]; ok {
		t.Fatalf("expected top-level payload snapshot, got %#v", results[0].Data)
	}
	for _, forbidden := range []string{"executor_key", "step_id", "result_focus", "confirmed_entity", "data_present"} {
		if _, ok := results[0].Data[forbidden]; ok {
			t.Fatalf("narration result leaked internal field %q: %#v", forbidden, results[0].Data)
		}
	}
}
