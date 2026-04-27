package cubebox

import (
	"context"
	"errors"
	"testing"
)

type readExecutorStub struct {
	validateFn func(map[string]any) (map[string]any, error)
	executeFn  func(context.Context, ExecuteRequest, map[string]any) (ExecuteResult, error)
}

func (s readExecutorStub) ValidateParams(raw map[string]any) (map[string]any, error) {
	if s.validateFn != nil {
		return s.validateFn(raw)
	}
	return raw, nil
}

func (s readExecutorStub) Execute(ctx context.Context, request ExecuteRequest, params map[string]any) (ExecuteResult, error) {
	if s.executeFn != nil {
		return s.executeFn(ctx, request, params)
	}
	return ExecuteResult{Payload: map[string]any{}}, nil
}

func TestNewExecutionRegistryRejectsDuplicateAPIKey(t *testing.T) {
	_, err := NewExecutionRegistry(
		RegisteredExecutor{ExecutorKey: "orgunit.details", Executor: readExecutorStub{}},
		RegisteredExecutor{ExecutorKey: "orgunit.details", Executor: readExecutorStub{}},
	)
	if err == nil {
		t.Fatal("expected duplicate api_key error")
	}
}

func TestExecutionRegistryRegisteredExecutorsReturnsSortedSnapshot(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{ExecutorKey: "orgunit.list_children", Executor: readExecutorStub{}},
		RegisteredExecutor{ExecutorKey: "orgunit.details", Executor: readExecutorStub{}},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	items := registry.RegisteredExecutors()
	if len(items) != 2 {
		t.Fatalf("registered executors=%d", len(items))
	}
	if items[0].ExecutorKey != "orgunit.details" || items[1].ExecutorKey != "orgunit.list_children" {
		t.Fatalf("unexpected api key order: %#v", items)
	}
}

func TestExecutionRegistryProjectNarrationResultsUsesRawPayloadCopy(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{ExecutorKey: "orgunit.details", Executor: readExecutorStub{}},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	source := map[string]any{
		"org_unit": map[string]any{
			"org_code": "100000",
			"name":     "飞虫与鲜花",
		},
		"as_of": "2026-04-25",
	}
	results := registry.ProjectNarrationResults([]ExecuteResult{
		{
			ExecutorKey: "orgunit.details",
			StepID:      "step-1",
			Payload:     source,
			ResultFocus: []string{"org_unit.name"},
			ConfirmedEntity: &QueryEntity{
				Domain:    "orgunit",
				EntityKey: "100000",
			},
		},
	})
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	if results[0].Domain != "orgunit" {
		t.Fatalf("domain=%q", results[0].Domain)
	}
	if results[0].Data == nil {
		t.Fatal("expected narration data")
	}
	if _, ok := results[0].Data["org_unit"]; !ok {
		t.Fatalf("expected raw payload field, got %+v", results[0].Data)
	}
	if got := results[0].Data["as_of"]; got != "2026-04-25" {
		t.Fatalf("as_of=%v", got)
	}
	for _, forbidden := range []string{"executor_key", "step_id", "result_focus", "confirmed_entity", "data_present"} {
		if _, ok := results[0].Data[forbidden]; ok {
			t.Fatalf("unexpected execution envelope field %q in narration data: %+v", forbidden, results[0].Data)
		}
	}
	results[0].Data["as_of"] = "mutated"
	if got := source["as_of"]; got != "2026-04-25" {
		t.Fatalf("expected top-level narration data map to be copied, source as_of=%v", got)
	}
	source["org_unit"].(map[string]any)["name"] = "已更新"
	if got := results[0].Data["org_unit"].(map[string]any)["name"]; got != "已更新" {
		t.Fatalf("expected shallow copy to retain nested payload reference, got %v", got)
	}
	source["added_later"] = "should-not-appear"
	if _, ok := results[0].Data["added_later"]; ok {
		t.Fatalf("unexpected mutation leakage: %+v", results[0].Data)
	}
}

func TestExecutionRegistryExecutePlan(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.search",
			RequiredParams: []string{"query", "as_of"},
			Executor: readExecutorStub{
				validateFn: func(raw map[string]any) (map[string]any, error) {
					return raw, nil
				},
				executeFn: func(_ context.Context, request ExecuteRequest, params map[string]any) (ExecuteResult, error) {
					if request.StepID != "step-1" {
						t.Fatalf("step_id=%q", request.StepID)
					}
					return ExecuteResult{
						Payload: map[string]any{
							"target_org_code": "1001",
							"query":           params["query"],
						},
					}, nil
				},
			},
		},
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			Executor: readExecutorStub{
				validateFn: func(raw map[string]any) (map[string]any, error) {
					return raw, nil
				},
				executeFn: func(_ context.Context, request ExecuteRequest, params map[string]any) (ExecuteResult, error) {
					if request.StepID != "step-2" {
						t.Fatalf("step_id=%q", request.StepID)
					}
					prev, ok := request.PreviousResults["step-1"]
					if !ok {
						t.Fatal("missing previous result for step-1")
					}
					if params["org_code"] != "1001" {
						t.Fatalf("expected resolved org_code, got %v", params["org_code"])
					}
					return ExecuteResult{
						Payload: map[string]any{
							"org_code":      params["org_code"],
							"resolved_from": prev.Payload["target_org_code"],
						},
					}, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	plan := ReadPlan{
		Intent:        "orgunit.search_then_details",
		Confidence:    0.8,
		MissingParams: []string{},
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.search",
				Params:      map[string]any{"query": "华东", "as_of": "2026-04-23"},
				ResultFocus: []string{"target_org_code"},
				DependsOn:   []string{},
			},
			{
				ID:          "step-2",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "@step-1.target_org_code", "as_of": "2026-04-23"},
				ResultFocus: []string{"org_unit.name"},
				DependsOn:   []string{"step-1"},
			},
		},
		ExplainFocus: []string{"先说明命中的组织", "再说明详情"},
	}

	results, err := registry.ExecutePlan(context.Background(), ExecuteRequest{
		TenantID:       "tenant-1",
		PrincipalID:    "principal-1",
		ConversationID: "conv-1",
	}, plan)
	if err != nil {
		t.Fatalf("ExecutePlan err=%v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results=%d", len(results))
	}
	if results[0].ExecutorKey != "orgunit.search" || results[1].ExecutorKey != "orgunit.details" {
		t.Fatalf("results=%+v", results)
	}
	if results[1].Payload["resolved_from"] != "1001" {
		t.Fatalf("resolved_from=%v", results[1].Payload["resolved_from"])
	}
}

func TestExecutionRegistryExecutePlanResolvesPayloadPathReference(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.search",
			RequiredParams: []string{"query", "as_of"},
			Executor: readExecutorStub{
				executeFn: func(_ context.Context, _ ExecuteRequest, _ map[string]any) (ExecuteResult, error) {
					return ExecuteResult{
						Payload: map[string]any{
							"payload":         map[string]any{"target_org_code": "nested-value"},
							"target_org_code": "top-value",
						},
					}, nil
				},
			},
		},
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			Executor: readExecutorStub{
				executeFn: func(_ context.Context, _ ExecuteRequest, params map[string]any) (ExecuteResult, error) {
					if params["org_code"] != "top-value" {
						t.Fatalf("expected payload reference resolved, got %v", params["org_code"])
					}
					return ExecuteResult{Payload: map[string]any{}}, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:     "orgunit.search_then_details",
		Confidence: 0.8,
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.search",
				Params:      map[string]any{"query": "华东", "as_of": "2026-04-23"},
				DependsOn:   []string{},
			},
			{
				ID:          "step-2",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "@step-1.payload.target_org_code", "as_of": "2026-04-23"},
				DependsOn:   []string{"step-1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExecutePlan err=%v", err)
	}
}

func TestExecutionRegistryExecutePlanRejectsMissingExecutor(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{ExecutorKey: "orgunit.search", Executor: readExecutorStub{}},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:        "orgunit.details",
		Confidence:    0.9,
		MissingParams: []string{},
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "1001", "as_of": "2026-04-23"},
				ResultFocus: []string{},
				DependsOn:   []string{},
			},
		},
		ExplainFocus: []string{},
	})
	if !errors.Is(err, ErrAPICatalogDriftOrExecutorMissing) {
		t.Fatalf("expected ErrAPICatalogDriftOrExecutorMissing, got %v", err)
	}
}

func TestExecutionRegistryExecutePlanRejectsClarifyingPlan(t *testing.T) {
	registry, err := NewExecutionRegistry()
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:             "orgunit.details",
		Confidence:         0.4,
		MissingParams:      []string{"org_code"},
		ClarifyingQuestion: "请提供组织编码",
	})
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}

func TestExecutionRegistryExecutePlanRejectsMissingRequiredParam(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			Executor:       readExecutorStub{},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:        "orgunit.details",
		Confidence:    0.9,
		MissingParams: []string{},
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "1001"},
				ResultFocus: []string{},
				DependsOn:   []string{},
			},
		},
		ExplainFocus: []string{},
	})
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}

func TestExecutionRegistryExecutePlanRejectsUnexpectedParam(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			OptionalParams: []string{"include_disabled"},
			Executor:       readExecutorStub{},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:        "orgunit.details",
		Confidence:    0.9,
		MissingParams: []string{},
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "1001", "as_of": "2026-04-23", "org_code_from": "step-0.target_org_code"},
				ResultFocus: []string{},
				DependsOn:   []string{},
			},
		},
		ExplainFocus: []string{},
	})
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}

func TestExecutionRegistryExecutePlanRejectsInvalidReferenceSyntax(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			Executor:       readExecutorStub{},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:     "orgunit.details",
		Confidence: 0.9,
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "@bad", "as_of": "2026-04-23"},
				DependsOn:   []string{},
			},
		},
	})
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}

func TestExecutionRegistryExecutePlanRejectsReferenceToMissingStep(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			ExecutorKey:    "orgunit.search",
			RequiredParams: []string{"query", "as_of"},
			Executor:       readExecutorStub{},
		},
		RegisteredExecutor{
			ExecutorKey:    "orgunit.details",
			RequiredParams: []string{"org_code", "as_of"},
			Executor:       readExecutorStub{},
		},
	)
	if err != nil {
		t.Fatalf("NewExecutionRegistry err=%v", err)
	}

	_, err = registry.ExecutePlan(context.Background(), ExecuteRequest{}, ReadPlan{
		Intent:     "orgunit.search_then_details",
		Confidence: 0.9,
		Steps: []ReadPlanStep{
			{
				ID:          "step-1",
				ExecutorKey: "orgunit.search",
				Params:      map[string]any{"query": "华东", "as_of": "2026-04-23"},
				DependsOn:   []string{},
			},
			{
				ID:          "step-2",
				ExecutorKey: "orgunit.details",
				Params:      map[string]any{"org_code": "@step-0.target_org_code", "as_of": "2026-04-23"},
				DependsOn:   []string{"step-1"},
			},
		},
	})
	if !errors.Is(err, ErrReadPlanBoundaryViolation) {
		t.Fatalf("expected ErrReadPlanBoundaryViolation, got %v", err)
	}
}
