package cubebox

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrAPICatalogDriftOrExecutorMissing = errors.New("CUBEBOX_API_CATALOG_DRIFT_OR_EXECUTOR_MISSING")

type ReadExecutor interface {
	ValidateParams(raw map[string]any) (map[string]any, error)
	Execute(ctx context.Context, request ExecuteRequest, params map[string]any) (ExecuteResult, error)
}

type ExecuteRequest struct {
	TenantID        string
	PrincipalID     string
	ConversationID  string
	PlanIntent      string
	StepID          string
	PreviousResults map[string]ExecuteResult
}

type ExecuteResult struct {
	APIKey          string
	StepID          string
	Payload         map[string]any
	ResultFocus     []string
	ConfirmedEntity *QueryEntity
}

type RegisteredExecutor struct {
	APIKey         string
	RequiredParams []string
	OptionalParams []string
	Executor       ReadExecutor
}

type ExecutionRegistry struct {
	executors map[string]RegisteredExecutor
}

func NewExecutionRegistry(items ...RegisteredExecutor) (*ExecutionRegistry, error) {
	registry := &ExecutionRegistry{
		executors: make(map[string]RegisteredExecutor, len(items)),
	}
	for _, item := range items {
		if err := registry.Register(item); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *ExecutionRegistry) Register(item RegisteredExecutor) error {
	if r == nil {
		return errors.New("execution registry nil")
	}
	apiKey := strings.TrimSpace(item.APIKey)
	if apiKey == "" {
		return errors.New("api_key required")
	}
	if item.Executor == nil {
		return errors.New("executor required")
	}
	if _, exists := r.executors[apiKey]; exists {
		return fmt.Errorf("api_key duplicated: %s", apiKey)
	}
	item.APIKey = apiKey
	item.RequiredParams = normalizeParamNames(item.RequiredParams)
	item.OptionalParams = normalizeParamNames(item.OptionalParams)
	r.executors[apiKey] = item
	return nil
}

func (r *ExecutionRegistry) Resolve(apiKey string) (RegisteredExecutor, bool) {
	if r == nil {
		return RegisteredExecutor{}, false
	}
	item, ok := r.executors[strings.TrimSpace(apiKey)]
	return item, ok
}

func (r *ExecutionRegistry) ExecutePlan(ctx context.Context, request ExecuteRequest, plan ReadPlan) ([]ExecuteResult, error) {
	if r == nil {
		return nil, wrapExecutorMissingError("execution registry missing")
	}
	if err := ValidateReadPlan(plan); err != nil {
		return nil, err
	}
	if len(plan.MissingParams) > 0 {
		return nil, wrapReadPlanBoundaryError("clarifying plan is not executable")
	}

	results := make([]ExecuteResult, 0, len(plan.Steps))
	previousResults := make(map[string]ExecuteResult, len(plan.Steps))
	for _, step := range plan.Steps {
		item, ok := r.Resolve(step.APIKey)
		if !ok {
			return nil, wrapExecutorMissingError(fmt.Sprintf("api_key not registered: %s", strings.TrimSpace(step.APIKey)))
		}
		if err := validateRegisteredParams(item, step.Params); err != nil {
			return nil, err
		}
		validatedParams, err := item.Executor.ValidateParams(step.Params)
		if err != nil {
			return nil, err
		}
		result, err := item.Executor.Execute(ctx, ExecuteRequest{
			TenantID:        strings.TrimSpace(request.TenantID),
			PrincipalID:     strings.TrimSpace(request.PrincipalID),
			ConversationID:  strings.TrimSpace(request.ConversationID),
			PlanIntent:      strings.TrimSpace(plan.Intent),
			StepID:          strings.TrimSpace(step.ID),
			PreviousResults: copyPreviousResults(previousResults),
		}, validatedParams)
		if err != nil {
			return nil, err
		}
		result.APIKey = strings.TrimSpace(step.APIKey)
		result.StepID = strings.TrimSpace(step.ID)
		if result.ResultFocus == nil {
			result.ResultFocus = append([]string(nil), step.ResultFocus...)
		}
		results = append(results, result)
		previousResults[result.StepID] = result
	}

	return results, nil
}

func wrapExecutorMissingError(detail string) error {
	return fmt.Errorf("%w: %s", ErrAPICatalogDriftOrExecutorMissing, strings.TrimSpace(detail))
}

func validateRegisteredParams(item RegisteredExecutor, params map[string]any) error {
	allowed := make(map[string]struct{}, len(item.RequiredParams)+len(item.OptionalParams))
	for _, name := range item.RequiredParams {
		allowed[name] = struct{}{}
	}
	for _, name := range item.OptionalParams {
		allowed[name] = struct{}{}
	}
	for name := range params {
		if _, ok := allowed[name]; ok {
			continue
		}
		return wrapReadPlanBoundaryError(fmt.Sprintf("unexpected param for %s: %s", item.APIKey, strings.TrimSpace(name)))
	}
	for _, name := range item.RequiredParams {
		value, ok := params[name]
		if !ok || value == nil {
			return wrapReadPlanBoundaryError(fmt.Sprintf("required param missing for %s: %s", item.APIKey, name))
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return wrapReadPlanBoundaryError(fmt.Sprintf("required param empty for %s: %s", item.APIKey, name))
		}
	}
	return nil
}

func normalizeParamNames(items []string) []string {
	if items == nil {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func copyPreviousResults(in map[string]ExecuteResult) map[string]ExecuteResult {
	if len(in) == 0 {
		return map[string]ExecuteResult{}
	}
	out := make(map[string]ExecuteResult, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
