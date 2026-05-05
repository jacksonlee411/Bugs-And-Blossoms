package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type cubeboxAPIToolRunner interface {
	Tools() []cubebox.APITool
	ExecutePlan(ctx context.Context, request cubebox.ExecuteRequest, plan cubebox.APICallPlan) ([]cubebox.ExecuteResult, error)
}

type cubeboxOrgUnitAPIToolRunner struct {
	store   OrgUnitStore
	runtime authzRuntimeStore
	tools   []cubebox.APITool
	toolMap map[string]cubebox.APITool
}

func newCubeboxOrgUnitAPIToolRunner(store OrgUnitStore, runtime authzRuntimeStore) (*cubeboxOrgUnitAPIToolRunner, error) {
	if store == nil || runtime == nil {
		return nil, nil
	}
	facts, err := CollectAuthzAPICatalogRuntimeFacts()
	if err != nil {
		return nil, err
	}
	tools, err := BuildCubeBoxRuntimeAPITools(facts)
	if err != nil {
		return nil, err
	}
	toolMap := make(map[string]cubebox.APITool, len(tools))
	for _, tool := range tools {
		tool = tool.Normalized()
		toolMap[cubebox.APIToolRouteID(tool.Method, tool.Path)] = tool
	}
	return &cubeboxOrgUnitAPIToolRunner{
		store:   store,
		runtime: runtime,
		tools:   tools,
		toolMap: toolMap,
	}, nil
}

func (r *cubeboxOrgUnitAPIToolRunner) Tools() []cubebox.APITool {
	if r == nil {
		return nil
	}
	out := make([]cubebox.APITool, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, tool.Normalized())
	}
	return out
}

func (r *cubeboxOrgUnitAPIToolRunner) ExecutePlan(ctx context.Context, request cubebox.ExecuteRequest, plan cubebox.APICallPlan) ([]cubebox.ExecuteResult, error) {
	if r == nil || r.store == nil || r.runtime == nil || len(r.toolMap) == 0 {
		return nil, cubebox.ErrAPICatalogDriftOrExecutorMissing
	}
	if err := cubebox.ValidateAPICallPlan(plan); err != nil {
		return nil, err
	}
	results := make([]cubebox.ExecuteResult, 0, len(plan.Calls))
	for _, call := range plan.Calls {
		tool, ok := r.toolMap[cubebox.APIToolRouteID(call.Method, call.Path)]
		if !ok {
			return nil, cubebox.ErrAPICatalogDriftOrExecutorMissing
		}
		if err := validateAPICallParams(tool, call.Params); err != nil {
			return nil, err
		}
		result, err := r.executeCall(ctx, request, tool, call)
		if err != nil {
			return nil, err
		}
		result.StepID = strings.TrimSpace(call.ID)
		result.Method = strings.ToUpper(strings.TrimSpace(call.Method))
		result.Path = normalizeServerAPIToolPath(call.Path)
		result.OperationID = tool.OperationID
		if result.ResultFocus == nil {
			result.ResultFocus = append([]string(nil), call.ResultFocus...)
		}
		results = append(results, result)
	}
	return results, nil
}

func (r *cubeboxOrgUnitAPIToolRunner) executeCall(ctx context.Context, request cubebox.ExecuteRequest, tool cubebox.APITool, call cubebox.APICallStep) (cubebox.ExecuteResult, error) {
	tenantID := strings.TrimSpace(request.TenantID)
	principalID := strings.TrimSpace(request.PrincipalID)
	if tenantID == "" || principalID == "" {
		return cubebox.ExecuteResult{}, cubebox.ErrAPICallPlanBoundaryViolation
	}
	if tool.AuthzCapabilityKey == "" || tool.ResourceObject == "" || tool.Action == "" {
		return cubebox.ExecuteResult{}, cubebox.ErrAPICatalogDriftOrExecutorMissing
	}
	if tool.AuthzCapabilityKey != authz.AuthzCapabilityKey(tool.ResourceObject, tool.Action) {
		return cubebox.ExecuteResult{}, cubebox.ErrAPICatalogDriftOrExecutorMissing
	}
	if req, ok := findRouteRequirement(tool.Method, tool.Path); !ok ||
		req.Surface != authz.CapabilitySurfaceTenantAPI ||
		req.Object != tool.ResourceObject ||
		req.Action != tool.Action {
		return cubebox.ExecuteResult{}, cubebox.ErrAPICatalogDriftOrExecutorMissing
	}
	allowed, err := r.runtime.AuthorizePrincipal(ctx, tenantID, principalID, tool.ResourceObject, tool.Action)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	if !allowed {
		return cubebox.ExecuteResult{}, &orgUnitAuthzScopeError{err: errAuthzScopeForbidden}
	}

	httpReq, err := buildAPIToolHTTPRequest(ctx, request, tool, call)
	if err != nil {
		return cubebox.ExecuteResult{}, err
	}
	rec := httptest.NewRecorder()
	switch tool.Path {
	case "/org/api/org-units":
		handleOrgUnitsAPI(rec, httpReq, r.store, nil, r.runtime)
	case "/org/api/org-units/details":
		handleOrgUnitsDetailsAPI(rec, httpReq, r.store, r.runtime)
	case "/org/api/org-units/search":
		handleOrgUnitsSearchAPI(rec, httpReq, r.store, r.runtime)
	case "/org/api/org-units/audit":
		handleOrgUnitsAuditAPI(rec, httpReq, r.store, r.runtime)
	default:
		return cubebox.ExecuteResult{}, cubebox.ErrAPICatalogDriftOrExecutorMissing
	}
	if rec.Code < 200 || rec.Code >= 300 {
		return cubebox.ExecuteResult{}, apiToolHTTPError(rec.Code, rec.Body.String())
	}
	payload := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		return cubebox.ExecuteResult{}, err
	}
	return projectAPIToolResult(tool, payload), nil
}

func validateAPICallParams(tool cubebox.APITool, params map[string]any) error {
	if params == nil {
		return cubebox.ErrAPICallPlanBoundaryViolation
	}
	for name := range params {
		if !tool.AllowsParam(name) {
			return fmt.Errorf("%w: unexpected param for %s %s: %s", cubebox.ErrAPICallPlanBoundaryViolation, tool.Method, tool.Path, strings.TrimSpace(name))
		}
	}
	for _, name := range tool.RequestSchema.Required {
		value, ok := params[name]
		if !ok || value == nil {
			return fmt.Errorf("%w: required param missing for %s %s: %s", cubebox.ErrAPICallPlanBoundaryViolation, tool.Method, tool.Path, name)
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return fmt.Errorf("%w: required param empty for %s %s: %s", cubebox.ErrAPICallPlanBoundaryViolation, tool.Method, tool.Path, name)
		}
	}
	return nil
}

func buildAPIToolHTTPRequest(ctx context.Context, request cubebox.ExecuteRequest, tool cubebox.APITool, call cubebox.APICallStep) (*http.Request, error) {
	values := url.Values{}
	for name, value := range call.Params {
		if err := appendAPIToolQueryParam(values, name, value); err != nil {
			return nil, err
		}
	}
	if tool.Path == "/org/api/org-units" {
		normalizeOrgUnitListAPIToolPagination(values)
	}
	target := tool.Path
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	httpReq := httptest.NewRequest(tool.Method, target, nil).WithContext(ctx)
	httpReq = httpReq.WithContext(withTenant(httpReq.Context(), Tenant{ID: strings.TrimSpace(request.TenantID)}))
	httpReq = httpReq.WithContext(withPrincipal(httpReq.Context(), Principal{ID: strings.TrimSpace(request.PrincipalID), TenantID: strings.TrimSpace(request.TenantID)}))
	return httpReq, nil
}

func appendAPIToolQueryParam(values url.Values, name string, value any) error {
	name = strings.TrimSpace(name)
	if name == "" || value == nil {
		return nil
	}
	if name == "page_size" {
		name = "size"
	}
	if name == "keyword" {
		name = "q"
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			values.Set(name, strings.TrimSpace(v))
		}
	case bool:
		values.Set(name, strconv.FormatBool(v))
	case int:
		values.Set(name, strconv.Itoa(v))
	case int64:
		values.Set(name, strconv.FormatInt(v, 10))
	case float64:
		if v != float64(int(v)) {
			return fmt.Errorf("%w: numeric param must be integer: %s", cubebox.ErrAPICallPlanBoundaryViolation, name)
		}
		values.Set(name, strconv.Itoa(int(v)))
	default:
		return fmt.Errorf("%w: unsupported param type for %s", cubebox.ErrAPICallPlanBoundaryViolation, name)
	}
	return nil
}

func normalizeOrgUnitListAPIToolPagination(values url.Values) {
	rawPage := strings.TrimSpace(values.Get("page"))
	rawSize := strings.TrimSpace(values.Get("size"))
	if rawPage == "" && rawSize == "" {
		values.Set("page", "0")
		values.Set("size", "100")
		return
	}
	if rawPage != "" {
		if page, err := strconv.Atoi(rawPage); err == nil && page > 0 {
			values.Set("page", strconv.Itoa(page-1))
		}
	}
	if rawSize == "" {
		values.Set("size", "100")
	}
}

func projectAPIToolResult(tool cubebox.APITool, payload map[string]any) cubebox.ExecuteResult {
	result := cubebox.ExecuteResult{
		Method:      strings.ToUpper(strings.TrimSpace(tool.Method)),
		Path:        normalizeServerAPIToolPath(tool.Path),
		OperationID: strings.TrimSpace(tool.OperationID),
		Payload:     payload,
	}
	if entity := apiToolConfirmedEntity(tool, payload); entity != nil {
		result.ConfirmedEntity = entity
	}
	if candidates := apiToolPresentedCandidates(tool, payload); len(candidates) > 0 {
		result.PresentedCandidates = candidates
	}
	return result
}

func apiToolConfirmedEntity(tool cubebox.APITool, payload map[string]any) *cubebox.QueryEntity {
	keyField := strings.TrimSpace(tool.ObservationProjection.EntityKeyField)
	if keyField == "" || len(payload) == 0 {
		return nil
	}
	source := payload
	if root := strings.TrimSpace(tool.ObservationProjection.RootField); root != "" {
		if nested, ok := payload[root].(map[string]any); ok {
			source = nested
		} else {
			return nil
		}
	}
	key := stringFromPayload(source[keyField])
	if key == "" {
		return nil
	}
	entity := cubebox.QueryEntity{
		Intent:    strings.TrimSpace(tool.OperationID),
		Domain:    "orgunit",
		EntityKey: key,
		AsOf:      stringFromPayload(payload["as_of"]),
	}
	if entity.AsOf == "" {
		entity.AsOf = stringFromPayload(source["tree_as_of"])
	}
	return cubebox.NormalizeQueryEntity(entity)
}

func stringFromPayload(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func apiToolPresentedCandidates(tool cubebox.APITool, payload map[string]any) []cubebox.QueryCandidate {
	switch strings.TrimSpace(tool.OperationID) {
	case "orgunit.list":
		return apiToolCandidatesFromListPayload(payload)
	case "orgunit.search":
		return apiToolCandidatesFromSearchPayload(payload)
	default:
		return nil
	}
}

func apiToolCandidatesFromListPayload(payload map[string]any) []cubebox.QueryCandidate {
	rawItems, ok := payload["org_units"].([]any)
	if !ok || len(rawItems) == 0 {
		return nil
	}
	asOf := stringFromPayload(payload["as_of"])
	items := make([]cubebox.QueryCandidate, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		candidate := cubebox.NormalizeQueryCandidate(cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: stringFromPayload(item["org_code"]),
			Name:      stringFromPayload(item["name"]),
			AsOf:      asOf,
			Status:    stringFromPayload(item["status"]),
		})
		if candidate == nil {
			continue
		}
		items = append(items, *candidate)
	}
	return items
}

func apiToolCandidatesFromSearchPayload(payload map[string]any) []cubebox.QueryCandidate {
	rawItems, ok := payload["candidates"].([]any)
	if !ok || len(rawItems) == 0 {
		return nil
	}
	asOf := stringFromPayload(payload["tree_as_of"])
	items := make([]cubebox.QueryCandidate, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		candidateAsOf := stringFromPayload(item["as_of"])
		if candidateAsOf == "" {
			candidateAsOf = asOf
		}
		candidate := cubebox.NormalizeQueryCandidate(cubebox.QueryCandidate{
			Domain:    "orgunit",
			EntityKey: stringFromPayload(item["org_code"]),
			Name:      stringFromPayload(item["name"]),
			AsOf:      candidateAsOf,
			Status:    stringFromPayload(item["status"]),
		})
		if candidate == nil {
			continue
		}
		items = append(items, *candidate)
	}
	return items
}

type apiToolHTTPStatusError struct {
	status int
	body   string
}

func (e apiToolHTTPStatusError) Error() string {
	return fmt.Sprintf("api tool http status %d", e.status)
}

func apiToolHTTPError(status int, body string) error {
	body = strings.TrimSpace(body)
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return newBadRequestError(body)
	case http.StatusConflict:
		if ambiguous := apiToolSearchAmbiguousError(body); ambiguous != nil {
			return ambiguous
		}
		return apiToolHTTPStatusError{status: status, body: body}
	case http.StatusForbidden, http.StatusUnauthorized:
		return &orgUnitAuthzScopeError{err: errAuthzScopeForbidden}
	case http.StatusNotFound:
		return &orgUnitNotFoundError{}
	default:
		return apiToolHTTPStatusError{status: status, body: body}
	}
}

func apiToolSearchAmbiguousError(body string) error {
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}
	if stringFromPayload(payload["error_code"]) != "org_unit_search_ambiguous" {
		return nil
	}
	candidates := make([]OrgUnitSearchCandidate, 0)
	rawItems, _ := payload["candidates"].([]any)
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		orgCode := stringFromPayload(item["org_code"])
		if orgCode == "" {
			continue
		}
		candidates = append(candidates, OrgUnitSearchCandidate{
			OrgCode: orgCode,
			Name:    stringFromPayload(item["name"]),
			Status:  stringFromPayload(item["status"]),
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	return &orgUnitSearchAmbiguousError{
		Candidates: candidates,
		AsOf:       stringFromPayload(payload["tree_as_of"]),
	}
}
