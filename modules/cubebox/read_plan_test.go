package cubebox

import (
	"errors"
	"testing"
)

func TestDecodeReadPlanExecutable(t *testing.T) {
	raw := []byte(`{
	  "intent": "orgunit.details",
	  "confidence": 0.95,
	  "missing_params": [],
	  "steps": [
	    {
	      "id": "step-1",
	      "api_key": "orgunit.details",
	      "params": {"org_code":"1001","as_of":"2026-04-23"},
	      "result_focus": ["org_unit.name"],
	      "depends_on": []
	    }
	  ],
	  "explain_focus": ["组织基本信息"]
	}`)

	plan, err := DecodeReadPlan(raw)
	if err != nil {
		t.Fatalf("DecodeReadPlan err=%v", err)
	}
	if plan.Intent != "orgunit.details" {
		t.Fatalf("intent=%q", plan.Intent)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].APIKey != "orgunit.details" {
		t.Fatalf("unexpected steps=%+v", plan.Steps)
	}
}

func TestDecodeReadPlanClarifyingQuestion(t *testing.T) {
	raw := []byte(`{
	  "intent": "orgunit.details",
	  "confidence": 0.41,
	  "missing_params": ["org_code", "as_of"],
	  "clarifying_question": "请提供组织编码和查询日期。"
	}`)

	plan, err := DecodeReadPlan(raw)
	if err != nil {
		t.Fatalf("DecodeReadPlan err=%v", err)
	}
	if len(plan.MissingParams) != 2 {
		t.Fatalf("missing_params=%v", plan.MissingParams)
	}
	if plan.ClarifyingQuestion == "" {
		t.Fatal("clarifying_question empty")
	}
}

func TestDecodeReadPlanRejectsInvalidJSON(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{`))
	if !errors.Is(err, ErrReadPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected ErrReadPlanSchemaConstrainedDecodeFailed, got %v", err)
	}
}

func TestDecodeReadPlanRejectsMissingSteps(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{
	  "intent": "orgunit.details",
	  "confidence": 0.95,
	  "missing_params": [],
	  "explain_focus": []
	}`))
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}

func TestDecodeReadPlanRejectsUnknownTopLevelField(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{
	  "intent": "orgunit.details",
	  "confidence": 0.95,
	  "missing_params": [],
	  "steps": [
	    {
	      "id": "step-1",
	      "api_key": "orgunit.details",
	      "params": {"org_code":"1001","as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": []
	    }
	  ],
	  "explain_focus": [],
	  "unexpected": true
	}`))
	if !errors.Is(err, ErrReadPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected ErrReadPlanSchemaConstrainedDecodeFailed, got %v", err)
	}
}

func TestDecodeReadPlanRejectsUnknownStepField(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{
	  "intent": "orgunit.details",
	  "confidence": 0.95,
	  "missing_params": [],
	  "steps": [
	    {
	      "id": "step-1",
	      "api_key": "orgunit.details",
	      "params": {"org_code":"1001","as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": [],
	      "unexpected": true
	    }
	  ],
	  "explain_focus": []
	}`))
	if !errors.Is(err, ErrReadPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected ErrReadPlanSchemaConstrainedDecodeFailed, got %v", err)
	}
}

func TestDecodeReadPlanRejectsTrailingJSONValue(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{"intent":"orgunit.details","confidence":0.1,"missing_params":["org_code"],"clarifying_question":"x"} {"extra":true}`))
	if !errors.Is(err, ErrReadPlanSchemaConstrainedDecodeFailed) {
		t.Fatalf("expected ErrReadPlanSchemaConstrainedDecodeFailed, got %v", err)
	}
}

func TestDecodeReadPlanRejectsSecondStepWithoutDependsOn(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{
	  "intent": "orgunit.search_then_details",
	  "confidence": 0.78,
	  "missing_params": [],
	  "steps": [
	    {
	      "id": "step-1",
	      "api_key": "orgunit.search",
	      "params": {"query":"华东","as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": []
	    },
	    {
	      "id": "step-2",
	      "api_key": "orgunit.details",
	      "params": {"org_code_from":"step-1.target_org_code","as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": []
	    }
	  ],
	  "explain_focus": []
	}`))
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}

func TestDecodeReadPlanRejectsNonLinearDependsOn(t *testing.T) {
	_, err := DecodeReadPlan([]byte(`{
	  "intent": "orgunit.search_then_details",
	  "confidence": 0.78,
	  "missing_params": [],
	  "steps": [
	    {
	      "id": "step-1",
	      "api_key": "orgunit.search",
	      "params": {"query":"华东","as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": []
	    },
	    {
	      "id": "step-2",
	      "api_key": "orgunit.list",
	      "params": {"as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": []
	    },
	    {
	      "id": "step-3",
	      "api_key": "orgunit.details",
	      "params": {"org_code_from":"step-1.target_org_code","as_of":"2026-04-23"},
	      "result_focus": [],
	      "depends_on": ["step-1"]
	    }
	  ],
	  "explain_focus": []
	}`))
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}
