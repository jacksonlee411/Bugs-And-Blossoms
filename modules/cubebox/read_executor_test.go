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
		RegisteredExecutor{APIKey: "orgunit.details", Executor: readExecutorStub{}},
		RegisteredExecutor{APIKey: "orgunit.details", Executor: readExecutorStub{}},
	)
	if err == nil {
		t.Fatal("expected duplicate api_key error")
	}
}

func TestExecutionRegistryExecutePlan(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{
			APIKey:         "orgunit.search",
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
			APIKey:         "orgunit.details",
			RequiredParams: []string{"org_code_from", "as_of"},
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
					return ExecuteResult{
						Payload: map[string]any{
							"org_code_from": params["org_code_from"],
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
				APIKey:      "orgunit.search",
				Params:      map[string]any{"query": "华东", "as_of": "2026-04-23"},
				ResultFocus: []string{"target_org_code"},
				DependsOn:   []string{},
			},
			{
				ID:          "step-2",
				APIKey:      "orgunit.details",
				Params:      map[string]any{"org_code_from": "step-1.target_org_code", "as_of": "2026-04-23"},
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
	if results[0].APIKey != "orgunit.search" || results[1].APIKey != "orgunit.details" {
		t.Fatalf("results=%+v", results)
	}
	if results[1].Payload["resolved_from"] != "1001" {
		t.Fatalf("resolved_from=%v", results[1].Payload["resolved_from"])
	}
}

func TestExecutionRegistryExecutePlanRejectsMissingExecutor(t *testing.T) {
	registry, err := NewExecutionRegistry(
		RegisteredExecutor{APIKey: "orgunit.search", Executor: readExecutorStub{}},
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
				APIKey:      "orgunit.details",
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
			APIKey:         "orgunit.details",
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
				APIKey:      "orgunit.details",
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
