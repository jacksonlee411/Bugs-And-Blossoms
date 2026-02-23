package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	internalRuleDecisionAllow = "allow"
	internalRuleDecisionDeny  = "deny"
)

type internalRulesEvaluateRequest struct {
	CapabilityKey   string `json:"capability_key"`
	FieldKey        string `json:"field_key"`
	BusinessUnitID  string `json:"business_unit_id"`
	AsOf            string `json:"as_of"`
	RequestID       string `json:"request_id"`
	TargetOrgUnitID string `json:"target_org_unit_id,omitempty"`
}

type internalEvaluationContext struct {
	TenantID       string `json:"tenant_id"`
	ActorID        string `json:"actor_id,omitempty"`
	ActorRole      string `json:"actor_role,omitempty"`
	CapabilityKey  string `json:"capability_key"`
	FieldKey       string `json:"field_key"`
	SetID          string `json:"setid"`
	BusinessUnitID string `json:"business_unit_id"`
	AsOf           string `json:"as_of"`
	RequestID      string `json:"request_id"`
	TraceID        string `json:"trace_id"`
}

type internalRuleCandidate struct {
	RuleID          string `json:"rule_id"`
	Priority        int    `json:"priority"`
	EffectiveDate   string `json:"effective_date"`
	EndDate         string `json:"end_date,omitempty"`
	EligibilityExpr string `json:"eligibility_expr"`
	DecisionExpr    string `json:"decision_expr"`
	ReasonCode      string `json:"reason_code"`
}

type internalRulesEvaluateResponse struct {
	TraceID             string                    `json:"trace_id"`
	RequestID           string                    `json:"request_id"`
	CapabilityKey       string                    `json:"capability_key"`
	FunctionalAreaKey   string                    `json:"functional_area_key"`
	FieldKey            string                    `json:"field_key"`
	SetID               string                    `json:"setid"`
	PolicyVersion       string                    `json:"policy_version"`
	Decision            string                    `json:"decision"`
	ReasonCode          string                    `json:"reason_code"`
	SelectedRuleID      string                    `json:"selected_rule_id,omitempty"`
	SelectedRule        *internalRuleCandidate    `json:"selected_rule,omitempty"`
	BriefExplain        string                    `json:"brief_explain"`
	Context             internalEvaluationContext `json:"context"`
	CandidatesEvaluated int                       `json:"candidates_evaluated"`
	EligibilityMatched  int                       `json:"eligibility_matched"`
}

var newInternalRulesCELEnv = func() (*cel.Env, error) {
	return cel.NewEnv(cel.Variable("ctx", cel.MapType(cel.StringType, cel.StringType)))
}

var newInternalRulesCELProgram = func(env *cel.Env, ast *cel.Ast) (cel.Program, error) {
	return env.Program(ast)
}

var canViewInternalRulesEvaluate = canViewSetIDFullExplain

var internalRuleEligibilityProgramCache sync.Map
var internalRuleDecisionProgramCache sync.Map

func handleInternalRulesEvaluateAPI(w http.ResponseWriter, r *http.Request, setidStore SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodPost {
		routingWriteErrorInternal(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if setidStore == nil {
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "setid_resolver_missing", "setid resolver missing")
		return
	}
	if !canViewInternalRulesEvaluate(r.Context()) {
		routingWriteErrorInternal(w, r, http.StatusForbidden, scopeReasonActorScopeForbidden, "actor scope forbidden")
		return
	}

	var req internalRulesEvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "bad_json", "bad json")
		return
	}

	req.CapabilityKey = strings.ToLower(strings.TrimSpace(req.CapabilityKey))
	req.FieldKey = strings.ToLower(strings.TrimSpace(req.FieldKey))
	req.BusinessUnitID = strings.TrimSpace(req.BusinessUnitID)
	req.TargetOrgUnitID = strings.TrimSpace(req.TargetOrgUnitID)
	req.AsOf = strings.TrimSpace(req.AsOf)
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.CapabilityKey == "" || req.FieldKey == "" || req.BusinessUnitID == "" || req.AsOf == "" {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_request", "capability_key/field_key/business_unit_id/as_of required")
		return
	}
	if !fieldKeyPattern.MatchString(req.FieldKey) {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_request", "invalid field_key")
		return
	}
	if _, err := time.Parse("2006-01-02", req.AsOf); err != nil {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}
	if _, err := parseOrgID8(req.BusinessUnitID); err != nil {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_business_unit_id", "invalid business_unit_id")
		return
	}
	if req.TargetOrgUnitID == "" {
		req.TargetOrgUnitID = req.BusinessUnitID
	}
	if _, err := parseOrgID8(req.TargetOrgUnitID); err != nil {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_org_unit_id", "invalid target_org_unit_id")
		return
	}

	capCtx, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       req.CapabilityKey,
		BusinessUnitID:      req.BusinessUnitID,
		AsOf:                req.AsOf,
		RequireBusinessUnit: true,
	})
	if capErr != nil {
		routingWriteErrorInternal(w, r, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
		return
	}
	req.CapabilityKey = capCtx.CapabilityKey
	req.BusinessUnitID = capCtx.BusinessUnitID
	req.AsOf = capCtx.AsOf
	functionalAreaKey, areaReasonCode, areaAllowed := evaluateFunctionalAreaGate(tenant.ID, req.CapabilityKey)
	if !areaAllowed {
		routingWriteErrorInternal(w, r, http.StatusForbidden, areaReasonCode, functionalAreaErrorMessage(areaReasonCode))
		return
	}

	dynamicRelations := preloadCapabilityDynamicRelations(r.Context(), req.BusinessUnitID)
	if !dynamicRelations.actorManages(req.TargetOrgUnitID, req.AsOf) {
		routingWriteErrorInternal(w, r, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}

	resolvedSetID, err := setidStore.ResolveSetID(r.Context(), tenant.ID, req.TargetOrgUnitID, req.AsOf)
	if err != nil {
		routingWriteErrorInternal(w, r, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}
	resolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	if resolvedSetID == "" {
		routingWriteErrorInternal(w, r, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}

	items, err := defaultSetIDStrategyRegistryStore.list(r.Context(), tenant.ID, req.CapabilityKey, req.FieldKey, req.AsOf)
	if err != nil {
		if strings.Contains(err.Error(), "invalid as_of") {
			routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
			return
		}
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	requestID := req.RequestID
	if requestID == "" {
		requestID = normalizeSetIDExplainRequestID(r)
	}
	traceID := traceIDFromRequestHeader(r)
	if traceID == "" {
		traceID = fallbackSetIDExplainTraceID(requestID, req.CapabilityKey, req.BusinessUnitID, req.AsOf)
	}

	evalCtx := buildInternalEvaluationContext(r.Context(), tenant.ID, req, resolvedSetID, requestID, traceID)
	candidates := buildInternalRuleCandidates(items)
	ctxMap := evalCtx.celContextMap()

	decision, reasonCode, selectedRule, matched, evalErr := evaluateInternalRuleCandidates(ctxMap, candidates)
	if evalErr != nil {
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	response := internalRulesEvaluateResponse{
		TraceID:             traceID,
		RequestID:           requestID,
		CapabilityKey:       req.CapabilityKey,
		FunctionalAreaKey:   functionalAreaKey,
		FieldKey:            req.FieldKey,
		SetID:               resolvedSetID,
		PolicyVersion:       capabilityPolicyVersionBaseline,
		Decision:            decision,
		ReasonCode:          reasonCode,
		BriefExplain:        internalRuleBriefExplain(selectedRule, matched),
		Context:             evalCtx,
		CandidatesEvaluated: len(candidates),
		EligibilityMatched:  matched,
	}
	if selectedRule != nil {
		response.SelectedRuleID = selectedRule.RuleID
		response.SelectedRule = selectedRule
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}

func routingWriteErrorInternal(w http.ResponseWriter, r *http.Request, status int, code string, msg string) {
	routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, msg)
}

func buildInternalEvaluationContext(ctx context.Context, tenantID string, req internalRulesEvaluateRequest, resolvedSetID string, requestID string, traceID string) internalEvaluationContext {
	evalCtx := internalEvaluationContext{
		TenantID:       tenantID,
		CapabilityKey:  req.CapabilityKey,
		FieldKey:       req.FieldKey,
		SetID:          resolvedSetID,
		BusinessUnitID: req.BusinessUnitID,
		AsOf:           req.AsOf,
		RequestID:      requestID,
		TraceID:        traceID,
	}
	if principal, ok := currentPrincipal(ctx); ok {
		evalCtx.ActorID = strings.TrimSpace(principal.ID)
		evalCtx.ActorRole = strings.ToLower(strings.TrimSpace(principal.RoleSlug))
	}
	return evalCtx
}

func (c internalEvaluationContext) celContextMap() map[string]string {
	return map[string]string{
		"tenant_id":        c.TenantID,
		"actor_id":         c.ActorID,
		"actor_role":       c.ActorRole,
		"capability_key":   c.CapabilityKey,
		"field_key":        c.FieldKey,
		"setid":            c.SetID,
		"business_unit_id": c.BusinessUnitID,
		"as_of":            c.AsOf,
		"request_id":       c.RequestID,
		"trace_id":         c.TraceID,
	}
}

func buildInternalRuleCandidates(items []setIDStrategyRegistryItem) []internalRuleCandidate {
	out := make([]internalRuleCandidate, 0, len(items))
	for _, item := range items {
		decisionExpr, reasonCode := buildInternalDecisionAndReason(item)
		out = append(out, internalRuleCandidate{
			RuleID:          strategyRegistrySortKey(item),
			Priority:        item.Priority,
			EffectiveDate:   item.EffectiveDate,
			EndDate:         item.EndDate,
			EligibilityExpr: buildInternalEligibilityExpr(item),
			DecisionExpr:    decisionExpr,
			ReasonCode:      reasonCode,
		})
	}
	return out
}

func buildInternalEligibilityExpr(item setIDStrategyRegistryItem) string {
	if item.OrgLevel == orgLevelBusinessUnit {
		return fmt.Sprintf("ctx[\"business_unit_id\"] == %q", strings.TrimSpace(item.BusinessUnitID))
	}
	return "true"
}

func buildInternalDecisionAndReason(item setIDStrategyRegistryItem) (string, string) {
	if !item.Visible {
		return "\"deny\"", fieldHiddenInContextCode
	}
	if item.Required {
		return "\"allow\"", fieldRequiredInContextCode
	}
	return "\"allow\"", fieldVisibleInContextCode
}

func evaluateInternalRuleCandidates(ctxMap map[string]string, candidates []internalRuleCandidate) (string, string, *internalRuleCandidate, int, error) {
	matched := 0
	var selected *internalRuleCandidate
	for i := range candidates {
		candidate := candidates[i]
		eligible, err := evalInternalEligibilityExpr(candidate.EligibilityExpr, ctxMap)
		if err != nil {
			return "", "", nil, matched, err
		}
		if !eligible {
			continue
		}
		matched++
		if selected == nil || candidate.Priority > selected.Priority ||
			(candidate.Priority == selected.Priority && candidate.EffectiveDate > selected.EffectiveDate) {
			copyCandidate := candidate
			selected = &copyCandidate
		}
	}
	if selected == nil {
		return internalRuleDecisionDeny, fieldPolicyMissingCode, nil, matched, nil
	}
	decision, err := evalInternalDecisionExpr(selected.DecisionExpr, ctxMap)
	if err != nil {
		return "", "", nil, matched, err
	}
	switch decision {
	case internalRuleDecisionAllow, internalRuleDecisionDeny:
	default:
		decision = internalRuleDecisionDeny
	}
	reasonCode := strings.TrimSpace(selected.ReasonCode)
	if reasonCode == "" {
		if decision == internalRuleDecisionDeny {
			reasonCode = fieldPolicyMissingCode
		} else {
			reasonCode = fieldVisibleInContextCode
		}
	}
	return decision, reasonCode, selected, matched, nil
}

func evalInternalEligibilityExpr(expr string, ctxMap map[string]string) (bool, error) {
	program, err := loadOrCompileInternalProgram(expr, cel.BoolType, &internalRuleEligibilityProgramCache)
	if err != nil {
		return false, err
	}
	out, _, err := program.Eval(map[string]any{"ctx": ctxMap})
	if err != nil {
		return false, err
	}
	v := out.Value().(bool)
	return v, nil
}

func evalInternalDecisionExpr(expr string, ctxMap map[string]string) (string, error) {
	program, err := loadOrCompileInternalProgram(expr, cel.StringType, &internalRuleDecisionProgramCache)
	if err != nil {
		return "", err
	}
	out, _, err := program.Eval(map[string]any{"ctx": ctxMap})
	if err != nil {
		return "", err
	}
	v := out.Value().(string)
	return strings.ToLower(strings.TrimSpace(v)), nil
}

func loadOrCompileInternalProgram(expr string, outputType *cel.Type, cache *sync.Map) (cel.Program, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, errors.New("expression required")
	}
	if cached, ok := cache.Load(expr); ok {
		return cached.(cel.Program), nil
	}
	env, err := newInternalRulesCELEnv()
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	if ast.OutputType() != outputType {
		return nil, errors.New("expression output type mismatch")
	}
	program, err := newInternalRulesCELProgram(env, ast)
	if err != nil {
		return nil, err
	}
	cache.Store(expr, program)
	return program, nil
}

func internalRuleBriefExplain(selectedRule *internalRuleCandidate, matched int) string {
	if selectedRule == nil {
		return "no eligible rule candidate"
	}
	return fmt.Sprintf("selected %s (priority=%d, matched=%d)", selectedRule.RuleID, selectedRule.Priority, matched)
}
