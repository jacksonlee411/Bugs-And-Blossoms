package cubebox

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var ErrAPICatalogDriftOrExecutorMissing = errors.New("CUBEBOX_API_CATALOG_DRIFT_OR_EXECUTOR_MISSING")

var readPlanParamReferencePattern = regexp.MustCompile(`^@([A-Za-z0-9_-]+)\.([A-Za-z0-9_.-]+)$`)

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

type QueryNarrationResult struct {
	Domain string         `json:"domain,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

type RegisteredExecutor struct {
	APIKey             string
	RequiredParams     []string
	OptionalParams     []string
	Executor           ReadExecutor
	NarrationProjector func(ExecuteResult) QueryNarrationResult
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

func (r *ExecutionRegistry) RegisteredExecutors() []RegisteredExecutor {
	if r == nil || len(r.executors) == 0 {
		return nil
	}
	items := make([]RegisteredExecutor, 0, len(r.executors))
	for _, item := range r.executors {
		items = append(items, item)
	}
	slices.SortFunc(items, func(left RegisteredExecutor, right RegisteredExecutor) int {
		return strings.Compare(left.APIKey, right.APIKey)
	})
	return items
}

func (r *ExecutionRegistry) ProjectNarrationResults(results []ExecuteResult) []QueryNarrationResult {
	out := make([]QueryNarrationResult, 0, len(results))
	for _, result := range results {
		item, ok := r.Resolve(result.APIKey)
		if ok && item.NarrationProjector != nil {
			out = append(out, normalizeQueryNarrationResult(item.NarrationProjector(result), result))
			continue
		}
		out = append(out, defaultQueryNarrationResult(result))
	}
	return out
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
		resolvedParams, err := resolveReadPlanStepParams(step.Params, previousResults)
		if err != nil {
			return nil, err
		}
		if err := validateRegisteredParams(item, resolvedParams); err != nil {
			return nil, err
		}
		validatedParams, err := item.Executor.ValidateParams(resolvedParams)
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

func normalizeQueryNarrationResult(view QueryNarrationResult, result ExecuteResult) QueryNarrationResult {
	view.Domain = strings.TrimSpace(view.Domain)
	if view.Domain == "" {
		view.Domain = queryNarrationDomainForResult(result)
	}
	return view
}

func defaultQueryNarrationResult(result ExecuteResult) QueryNarrationResult {
	view := QueryNarrationResult{
		Domain: queryNarrationDomainForResult(result),
	}
	if len(result.Payload) > 0 {
		view.Data = map[string]any{"data_present": true}
	}
	return view
}

func queryNarrationDomainForResult(result ExecuteResult) string {
	if result.ConfirmedEntity != nil {
		if normalized := NormalizeQueryEntity(*result.ConfirmedEntity); normalized != nil {
			return normalized.Domain
		}
	}
	return strings.TrimSpace(stringValue(result.Payload["domain"]))
}

func resolveReadPlanStepParams(params map[string]any, previousResults map[string]ExecuteResult) (map[string]any, error) {
	if len(params) == 0 {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(params))
	for name, value := range params {
		resolved, err := resolveReadPlanParamValue(value, previousResults)
		if err != nil {
			return nil, wrapReadPlanBoundaryError(fmt.Sprintf("param %s reference invalid: %v", strings.TrimSpace(name), err))
		}
		out[name] = resolved
	}
	return out, nil
}

func resolveReadPlanParamValue(value any, previousResults map[string]ExecuteResult) (any, error) {
	text, ok := value.(string)
	if !ok {
		return value, nil
	}
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "@") {
		return value, nil
	}
	matches := readPlanParamReferencePattern.FindStringSubmatch(text)
	if len(matches) != 3 {
		return nil, errors.New("reference syntax invalid")
	}
	stepID := strings.TrimSpace(matches[1])
	fieldPath := strings.TrimSpace(matches[2])
	if stepID == "" || fieldPath == "" {
		return nil, errors.New("reference syntax invalid")
	}
	result, ok := previousResults[stepID]
	if !ok {
		return nil, errors.New("referenced step not found")
	}
	resolved, err := resolveReadPlanResultField(result, fieldPath)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func resolveReadPlanResultField(result ExecuteResult, fieldPath string) (any, error) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return nil, errors.New("reference field required")
	}
	first := strings.TrimSpace(parts[0])
	switch first {
	case "payload":
		if len(parts) < 2 {
			return nil, errors.New("payload child field required")
		}
		return resolveNestedMapField(result.Payload, parts[1:])
	case "api_key":
		return result.APIKey, nil
	case "step_id":
		return result.StepID, nil
	default:
		if len(parts) != 1 {
			return nil, errors.New("only top-level payload alias fields allowed")
		}
		return resolveNestedMapField(result.Payload, parts)
	}
}

func resolveNestedMapField(payload map[string]any, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, errors.New("field path required")
	}
	current := any(payload)
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			return nil, errors.New("field path invalid")
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil, errors.New("field path invalid")
		}
		next, exists := m[key]
		if !exists {
			return nil, errors.New("field not found")
		}
		current = next
	}
	if current == nil {
		return nil, errors.New("field empty")
	}
	return current, nil
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
