package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"net/http"
	"strings"
)

const (
	internalRuleDecisionAllow = "allow"
	internalRuleDecisionDeny  = "deny"
)

type internalRulesEvaluateRequest struct {
	CapabilityKey       string `json:"capability_key"`
	FieldKey            string `json:"field_key"`
	BusinessUnitOrgCode string `json:"business_unit_org_code"`
	OrgCode             string `json:"org_code,omitempty"`
	AsOf                string `json:"as_of"`
	RequestID           string `json:"request_id"`
}

type internalEvaluationContext struct {
	TenantID            string `json:"tenant_id"`
	ActorID             string `json:"actor_id,omitempty"`
	ActorRole           string `json:"actor_role,omitempty"`
	CapabilityKey       string `json:"capability_key"`
	FieldKey            string `json:"field_key"`
	SetID               string `json:"setid"`
	SetIDSource         string `json:"setid_source,omitempty"`
	BusinessUnitOrgCode string `json:"business_unit_org_code"`
	OrgCode             string `json:"org_code"`
	AsOf                string `json:"as_of"`
	RequestID           string `json:"request_id"`
	TraceID             string `json:"trace_id"`

	businessUnitNodeKey string
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
	TraceID                string                    `json:"trace_id"`
	RequestID              string                    `json:"request_id"`
	CapabilityKey          string                    `json:"capability_key"`
	FunctionalAreaKey      string                    `json:"functional_area_key"`
	FieldKey               string                    `json:"field_key"`
	SetID                  string                    `json:"setid"`
	PolicyVersion          string                    `json:"policy_version"`
	EffectivePolicyVersion string                    `json:"effective_policy_version"`
	Decision               string                    `json:"decision"`
	ReasonCode             string                    `json:"reason_code"`
	SelectedRuleID         string                    `json:"selected_rule_id,omitempty"`
	SelectedRule           *internalRuleCandidate    `json:"selected_rule,omitempty"`
	BriefExplain           string                    `json:"brief_explain"`
	Context                internalEvaluationContext `json:"context"`
	CandidatesEvaluated    int                       `json:"candidates_evaluated"`
	EligibilityMatched     int                       `json:"eligibility_matched"`
}

var canViewInternalRulesEvaluate = canViewSetIDFullExplain

func handleInternalRulesEvaluateAPI(w http.ResponseWriter, r *http.Request, setidStore SetIDGovernanceStore, orgResolver OrgUnitCodeResolver) {
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
	if orgResolver == nil {
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "orgunit_resolver_missing", "orgunit resolver missing")
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
	req.BusinessUnitOrgCode = strings.TrimSpace(req.BusinessUnitOrgCode)
	req.OrgCode = strings.TrimSpace(req.OrgCode)
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.CapabilityKey == "" || req.FieldKey == "" || req.BusinessUnitOrgCode == "" {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_request", "capability_key/field_key/business_unit_org_code required")
		return
	}
	if !fieldKeyPattern.MatchString(req.FieldKey) {
		routingWriteErrorInternal(w, r, http.StatusBadRequest, "invalid_request", "invalid field_key")
		return
	}
	asOf, err := parseRequiredDay(req.AsOf, "as_of")
	if err != nil {
		if code, message, ok := dayFieldErrorDetails(err); ok {
			routingWriteErrorInternal(w, r, http.StatusBadRequest, code, message)
		}
		return
	}
	req.AsOf = asOf

	contextResolver := newSetIDContextResolver(orgResolver, setidStore)
	policyCtx, err := contextResolver.ResolvePolicyContext(r.Context(), setIDPolicyContextInput{
		TenantID:            tenant.ID,
		CapabilityKey:       req.CapabilityKey,
		FieldKey:            req.FieldKey,
		AsOf:                req.AsOf,
		BusinessUnitOrgCode: req.BusinessUnitOrgCode,
	})
	if err != nil {
		if resolveErr, ok := asSetIDContextResolveError(err); ok {
			switch resolveErr.Code {
			case setIDContextCodeBusinessUnitInvalid:
				writeSetIDExplainOrgCodeError(w, r, "business_unit_org_code", resolveErr.Cause)
			case setIDContextCodeOrgResolverMissing:
				routingWriteErrorInternal(w, r, http.StatusInternalServerError, resolveErr.Code, "orgunit resolver missing")
			case setIDContextCodeSetIDResolverMissing:
				routingWriteErrorInternal(w, r, http.StatusInternalServerError, resolveErr.Code, "setid resolver missing")
			default:
				routingWriteErrorInternal(w, r, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
			}
			return
		}
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	req.BusinessUnitOrgCode = policyCtx.BusinessUnitOrgCode
	if req.OrgCode == "" {
		req.OrgCode = req.BusinessUnitOrgCode
	}

	capCtx, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       policyCtx.CapabilityKey,
		BusinessUnitOrgCode: req.BusinessUnitOrgCode,
		AsOf:                req.AsOf,
		RequireBusinessUnit: true,
	})
	if capErr != nil {
		routingWriteErrorInternal(w, r, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
		return
	}
	req.CapabilityKey = capCtx.CapabilityKey
	req.BusinessUnitOrgCode = capCtx.BusinessUnitOrgCode
	req.AsOf = capCtx.AsOf
	functionalAreaKey, areaReasonCode, areaAllowed := evaluateFunctionalAreaGate(tenant.ID, req.CapabilityKey)
	if !areaAllowed {
		routingWriteErrorInternal(w, r, http.StatusForbidden, areaReasonCode, functionalAreaErrorMessage(areaReasonCode))
		return
	}

	targetCtx := resolvedSetIDContext{
		OrgCode:       policyCtx.BusinessUnitOrgCode,
		OrgNodeKey:    policyCtx.BusinessUnitNodeKey,
		ResolvedSetID: policyCtx.ResolvedSetID,
		SetIDSource:   policyCtx.SetIDSource,
	}
	if !strings.EqualFold(req.OrgCode, capCtx.BusinessUnitOrgCode) {
		targetCtx, err = contextResolver.ResolveOrgContext(r.Context(), tenant.ID, req.OrgCode, req.AsOf, "org_code")
		if err != nil {
			if resolveErr, ok := asSetIDContextResolveError(err); ok {
				switch resolveErr.Code {
				case setIDContextCodeBusinessUnitInvalid:
					writeSetIDExplainOrgCodeError(w, r, "org_code", resolveErr.Cause)
				case setIDContextCodeOrgResolverMissing:
					routingWriteErrorInternal(w, r, http.StatusInternalServerError, resolveErr.Code, "orgunit resolver missing")
				case setIDContextCodeSetIDResolverMissing:
					routingWriteErrorInternal(w, r, http.StatusInternalServerError, resolveErr.Code, "setid resolver missing")
				default:
					routingWriteErrorInternal(w, r, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
				}
				return
			}
			routingWriteErrorInternal(w, r, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
	}
	req.OrgCode = targetCtx.OrgCode

	dynamicRelations := preloadCapabilityDynamicRelations(r.Context(), req.BusinessUnitOrgCode)
	if !dynamicRelations.actorManages(targetCtx.OrgCode, req.AsOf) {
		routingWriteErrorInternal(w, r, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}

	items, err := defaultSetIDStrategyRegistryStore.list(r.Context(), tenant.ID, req.CapabilityKey, req.FieldKey, req.AsOf)
	if err != nil {
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	requestID := req.RequestID
	if requestID == "" {
		requestID = normalizeSetIDExplainRequestID(r)
	}
	traceID := traceIDFromRequestHeader(r)
	if traceID == "" {
		traceID = fallbackSetIDExplainTraceID(requestID, req.CapabilityKey, req.BusinessUnitOrgCode, req.AsOf)
	}

	evalCtx := buildInternalEvaluationContext(r.Context(), tenant.ID, req, policyCtx.BusinessUnitNodeKey, targetCtx.ResolvedSetID, targetCtx.SetIDSource, requestID, traceID)
	evaluation, evalErr := resolveInternalRulesEvaluation(items, req.CapabilityKey, req.FieldKey, targetCtx.ResolvedSetID, policyCtx.BusinessUnitNodeKey)
	if evalErr != nil {
		routingWriteErrorInternal(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	selectedRule := internalRuleCandidateFromResolution(evaluation.Resolution, evaluation.Decision, evaluation.ReasonCode)
	matched := 0
	if evaluation.Resolution != nil {
		matched = len(evaluation.Resolution.MatchedItems)
	}
	effectivePolicyVersion, policyParts := resolveOrgUnitEffectivePolicyVersion(tenant.ID, req.CapabilityKey)

	response := internalRulesEvaluateResponse{
		TraceID:                traceID,
		RequestID:              requestID,
		CapabilityKey:          req.CapabilityKey,
		FunctionalAreaKey:      functionalAreaKey,
		FieldKey:               req.FieldKey,
		SetID:                  targetCtx.ResolvedSetID,
		PolicyVersion:          policyParts.IntentPolicyVersion,
		EffectivePolicyVersion: effectivePolicyVersion,
		Decision:               evaluation.Decision,
		ReasonCode:             evaluation.ReasonCode,
		BriefExplain:           internalRuleBriefExplain(selectedRule, matched),
		Context:                evalCtx,
		CandidatesEvaluated:    len(items),
		EligibilityMatched:     matched,
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

func buildInternalEvaluationContext(ctx context.Context, tenantID string, req internalRulesEvaluateRequest, businessUnitNodeKey string, resolvedSetID string, setIDSource string, requestID string, traceID string) internalEvaluationContext {
	evalCtx := internalEvaluationContext{
		TenantID:            tenantID,
		CapabilityKey:       req.CapabilityKey,
		FieldKey:            req.FieldKey,
		SetID:               resolvedSetID,
		SetIDSource:         strings.TrimSpace(setIDSource),
		BusinessUnitOrgCode: req.BusinessUnitOrgCode,
		OrgCode:             req.OrgCode,
		AsOf:                req.AsOf,
		RequestID:           requestID,
		TraceID:             traceID,
		businessUnitNodeKey: businessUnitNodeKey,
	}
	if principal, ok := currentPrincipal(ctx); ok {
		evalCtx.ActorID = strings.TrimSpace(principal.ID)
		evalCtx.ActorRole = strings.ToLower(strings.TrimSpace(principal.RoleSlug))
	}
	return evalCtx
}

func (c internalEvaluationContext) celContextMap() map[string]string {
	return map[string]string{
		"tenant_id":              c.TenantID,
		"actor_id":               c.ActorID,
		"actor_role":             c.ActorRole,
		"capability_key":         c.CapabilityKey,
		"field_key":              c.FieldKey,
		"setid":                  c.SetID,
		"setid_source":           c.SetIDSource,
		"business_unit_org_code": c.BusinessUnitOrgCode,
		"org_code":               c.OrgCode,
		"business_unit_node_key": c.businessUnitNodeKey,
		"as_of":                  c.AsOf,
		"request_id":             c.RequestID,
		"trace_id":               c.TraceID,
	}
}

type internalRulesEvaluation struct {
	Decision   string
	ReasonCode string
	Resolution *setIDFieldDecisionResolution
}

func resolveInternalRulesEvaluation(
	items []setIDStrategyRegistryItem,
	capabilityKey string,
	fieldKey string,
	resolvedSetID string,
	businessUnitNodeKey string,
) (internalRulesEvaluation, error) {
	resolution, err := resolveFieldDecisionWithTraceFromItems(items, capabilityKey, fieldKey, resolvedSetID, businessUnitNodeKey)
	if err != nil {
		if decision, reasonCode, ok := internalRuleDecisionFromError(err); ok {
			return internalRulesEvaluation{
				Decision:   decision,
				ReasonCode: reasonCode,
			}, nil
		}
		return internalRulesEvaluation{}, err
	}

	fieldDecision, decision, reasonCode := applySetIDFieldVisibility(resolution.Decision)
	fieldDecision.Decision = decision
	fieldDecision.ReasonCode = reasonCode
	return internalRulesEvaluation{
		Decision:   fieldDecision.Decision,
		ReasonCode: fieldDecision.ReasonCode,
		Resolution: &resolution,
	}, nil
}

func internalRuleDecisionFromError(err error) (string, string, bool) {
	code := strings.TrimSpace(stablePgMessage(err))
	switch code {
	case fieldPolicyMissingCode,
		fieldPolicyConflictCode,
		fieldDefaultRuleMissingCode,
		fieldPolicyPriorityModeCode,
		fieldPolicyModeComboCode:
		return internalRuleDecisionDeny, code, true
	default:
		return "", "", false
	}
}

func internalRuleCandidateFromResolution(resolution *setIDFieldDecisionResolution, decision string, reasonCode string) *internalRuleCandidate {
	if resolution == nil || resolution.PrimaryItem == nil {
		return nil
	}
	item := *resolution.PrimaryItem
	return &internalRuleCandidate{
		RuleID:          resolution.PrimaryPolicyID,
		Priority:        item.Priority,
		EffectiveDate:   item.EffectiveDate,
		EndDate:         item.EndDate,
		EligibilityExpr: resolution.MatchedBucket,
		DecisionExpr:    strings.TrimSpace(decision),
		ReasonCode:      strings.TrimSpace(reasonCode),
	}
}

func internalRuleBriefExplain(selectedRule *internalRuleCandidate, matched int) string {
	if selectedRule == nil {
		return "no eligible rule candidate"
	}
	return fmt.Sprintf("selected %s (priority=%d, matched=%d)", selectedRule.RuleID, selectedRule.Priority, matched)
}
