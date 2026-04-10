package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

const (
	explainLevelBrief = "brief"
	explainLevelFull  = "full"

	capabilityPolicyVersionBaseline = "2026-02-23"
)

type setIDExplainResponse struct {
	TraceID               string               `json:"trace_id"`
	RequestID             string               `json:"request_id"`
	CapabilityKey         string               `json:"capability_key"`
	SetID                 string               `json:"setid"`
	FunctionalAreaKey     string               `json:"functional_area_key"`
	PolicyVersion         string               `json:"policy_version"`
	TenantID              string               `json:"tenant_id"`
	BusinessUnitOrgCode   string               `json:"business_unit_org_code"`
	OrgCode               string               `json:"org_code,omitempty"`
	AsOf                  string               `json:"as_of"`
	ResolvedSetID         string               `json:"resolved_setid"`
	ResolvedConfigVersion string               `json:"resolved_config_version,omitempty"`
	Decision              string               `json:"decision"`
	ReasonCode            string               `json:"reason_code"`
	Level                 string               `json:"level"`
	FieldDecisions        []setIDFieldDecision `json:"field_decisions"`
}

type setIDResolvedOrgRef struct {
	OrgCode      string
	OrgNodeKey   string
	LegacyOrgID8 string
}

func handleSetIDExplainAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore, orgResolver OrgUnitCodeResolver) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	capabilityKey := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("capability_key")))
	fieldKey := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("field_key")))
	businessUnitOrgCode := strings.TrimSpace(r.URL.Query().Get("business_unit_org_code"))
	requestID := normalizeSetIDExplainRequestID(r)
	if capabilityKey == "" || fieldKey == "" || businessUnitOrgCode == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "capability_key/field_key/business_unit_org_code required")
		return
	}
	asOf, err := parseRequiredQueryDay(r, "as_of")
	if err != nil {
		writeInternalDayFieldError(w, r, err)
		return
	}
	businessUnitOrgCode, _, businessUnitID, err := resolveSetIDExplainOrgCode(
		r.Context(),
		tenant.ID,
		businessUnitOrgCode,
		orgResolver,
	)
	if err != nil {
		writeSetIDExplainOrgCodeError(w, r, "business_unit_org_code", err)
		return
	}
	capCtx, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       capabilityKey,
		BusinessUnitOrgCode: businessUnitOrgCode,
		AsOf:                asOf,
		RequireBusinessUnit: true,
	})
	if capErr != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
		return
	}
	capabilityKey = capCtx.CapabilityKey
	businessUnitOrgCode = capCtx.BusinessUnitOrgCode
	asOf = capCtx.AsOf
	functionalAreaKey, areaReasonCode, areaAllowed := evaluateFunctionalAreaGate(tenant.ID, capabilityKey)
	if !areaAllowed {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, areaReasonCode, functionalAreaErrorMessage(areaReasonCode))
		return
	}

	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = explainLevelBrief
	}
	if level != explainLevelBrief && level != explainLevelFull {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_explain_level", "invalid explain level")
		return
	}
	if level == explainLevelFull && !canViewSetIDFullExplain(r.Context()) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, scopeReasonActorScopeForbidden, "actor scope forbidden")
		return
	}

	orgCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	if orgCode == "" {
		orgCode = businessUnitOrgCode
	}
	orgCode, orgNodeKey, _, err := resolveSetIDExplainOrgCode(r.Context(), tenant.ID, orgCode, orgResolver)
	if err != nil {
		writeSetIDExplainOrgCodeError(w, r, "org_code", err)
		return
	}
	dynamicRelations := preloadCapabilityDynamicRelations(r.Context(), businessUnitOrgCode)
	if !dynamicRelations.actorManages(orgCode, asOf) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}

	resolvedSetID, err := store.ResolveSetID(r.Context(), tenant.ID, orgNodeKey, asOf)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}
	targetSetID := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("setid")))
	resolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	if targetSetID != "" && targetSetID != resolvedSetID {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}

	fieldDecision, err := defaultSetIDStrategyRegistryStore.resolveFieldDecision(r.Context(), tenant.ID, capabilityKey, fieldKey, businessUnitID, asOf)
	if err != nil {
		status, code := statusCodeForFieldDecisionError(err)
		routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, code)
		return
	}
	fieldDecision, responseDecision, responseReasonCode := applySetIDFieldVisibility(fieldDecision)
	fieldDecision.Decision = responseDecision
	fieldDecision.ReasonCode = responseReasonCode

	traceID := traceIDFromRequestHeader(r)
	if traceID == "" {
		traceID = fallbackSetIDExplainTraceID(requestID, capabilityKey, businessUnitOrgCode, asOf)
	}
	policyVersion := defaultPolicyActivationRuntime.activePolicyVersion(tenant.ID, capabilityKey)

	response := setIDExplainResponse{
		TraceID:               traceID,
		RequestID:             requestID,
		CapabilityKey:         capabilityKey,
		SetID:                 resolvedSetID,
		FunctionalAreaKey:     functionalAreaKey,
		PolicyVersion:         policyVersion,
		TenantID:              tenant.ID,
		BusinessUnitOrgCode:   businessUnitOrgCode,
		AsOf:                  asOf,
		ResolvedSetID:         resolvedSetID,
		ResolvedConfigVersion: policyVersion,
		Decision:              responseDecision,
		ReasonCode:            responseReasonCode,
		Level:                 level,
		FieldDecisions:        []setIDFieldDecision{fieldDecision},
	}
	if !strings.EqualFold(orgCode, businessUnitOrgCode) {
		response.OrgCode = orgCode
	}
	if level == explainLevelBrief {
		response.TenantID = ""
		response.ResolvedConfigVersion = ""
	}
	logSetIDExplainAudit(response)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}

func resolveSetIDOrgCodeRef(ctx context.Context, tenantID string, orgCode string, orgResolver OrgUnitCodeResolver) (setIDResolvedOrgRef, error) {
	if orgResolver == nil {
		return setIDResolvedOrgRef{}, errors.New("org code resolver missing")
	}
	normalizedOrgCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	orgNodeKey, err := orgResolver.ResolveOrgNodeKeyByCode(ctx, tenantID, normalizedOrgCode)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	orgID, err := decodeOrgNodeKeyToID(normalizedOrgNodeKey)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	return setIDResolvedOrgRef{
		OrgCode:      normalizedOrgCode,
		OrgNodeKey:   normalizedOrgNodeKey,
		LegacyOrgID8: strconv.Itoa(orgID),
	}, nil
}

func resolveSetIDExplainOrgCode(ctx context.Context, tenantID string, orgCode string, orgResolver OrgUnitCodeResolver) (string, string, string, error) {
	ref, err := resolveSetIDOrgCodeRef(ctx, tenantID, orgCode, orgResolver)
	if err != nil {
		return "", "", "", err
	}
	return ref.OrgCode, ref.OrgNodeKey, ref.LegacyOrgID8, nil
}

func writeSetIDExplainOrgCodeError(w http.ResponseWriter, r *http.Request, field string, err error) {
	switch {
	case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, field+"_invalid", field+" invalid")
	case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, field+"_not_found", field+" not found")
	default:
		writeInternalAPIError(w, r, err, "setid_explain_"+field+"_resolve_failed")
	}
}

func statusCodeForFieldDecisionError(err error) (int, string) {
	switch strings.TrimSpace(err.Error()) {
	case fieldPolicyMissingCode, fieldDefaultRuleMissingCode, fieldPolicyConflictCode:
		return http.StatusUnprocessableEntity, strings.TrimSpace(err.Error())
	default:
		return http.StatusInternalServerError, "FIELD_EXPLAIN_MISSING"
	}
}

func canViewSetIDFullExplain(ctx context.Context) bool {
	p, ok := currentPrincipal(ctx)
	if !ok {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(p.RoleSlug))
	return role == authz.RoleTenantAdmin || role == authz.RoleSuperadmin
}

func traceIDFromRequestHeader(r *http.Request) string {
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if len(traceID) != 32 || traceID == "00000000000000000000000000000000" {
		return ""
	}
	for _, ch := range traceID {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return traceID
}

func normalizeSetIDExplainRequestID(r *http.Request) string {
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("X-Request-Id"))
	}
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("x-request-id"))
	}
	if requestID != "" {
		return requestID
	}
	if traceID := traceIDFromRequestHeader(r); traceID != "" {
		return "trace-" + traceID
	}
	return "setid-explain-auto"
}

func fallbackSetIDExplainTraceID(requestID string, capabilityKey string, businessUnitID string, asOf string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(requestID),
		strings.ToLower(strings.TrimSpace(capabilityKey)),
		strings.TrimSpace(businessUnitID),
		strings.TrimSpace(asOf),
	}, "|")))
	return hex.EncodeToString(sum[:16])
}

func logSetIDExplainAudit(response setIDExplainResponse) {
	log.Printf(
		"setid_explain decision=%s reason_code=%s capability_key=%s setid=%s policy_version=%s functional_area_key=%s level=%s field_decisions=%s",
		response.Decision,
		response.ReasonCode,
		response.CapabilityKey,
		response.SetID,
		response.PolicyVersion,
		response.FunctionalAreaKey,
		response.Level,
		briefSetIDFieldDecisions(response.FieldDecisions),
	)
}

func applySetIDFieldVisibility(fieldDecision setIDFieldDecision) (setIDFieldDecision, string, string) {
	responseDecision := internalRuleDecisionAllow
	responseReasonCode := fieldVisibleInContextCode
	fieldDecision.Visibility = fieldVisibilityVisible
	fieldDecision.MaskStrategy = ""
	fieldDecision.MaskedDefaultVal = ""
	if fieldDecision.Required {
		responseReasonCode = fieldRequiredInContextCode
	}

	if !fieldDecision.Visible {
		responseDecision = internalRuleDecisionDeny
		responseReasonCode = fieldHiddenInContextCode
		fieldDecision.Visibility = fieldVisibilityHidden
		fieldDecision.MaskStrategy = fieldMaskStrategyRedact
		fieldDecision.MaskedDefaultVal = fieldMaskedDefaultValueFallback
		fieldDecision.ResolvedDefaultVal = ""
	}

	if maskStrategy, masked := setIDMaskStrategyForDecision(fieldDecision); masked {
		fieldDecision.Visibility = fieldVisibilityMasked
		fieldDecision.MaskStrategy = maskStrategy
		fieldDecision.MaskedDefaultVal = fieldMaskedDefaultValueFallback
		fieldDecision.ResolvedDefaultVal = ""
		responseReasonCode = fieldMaskedInContextCode
	}

	fieldDecision.Decision = responseDecision
	fieldDecision.ReasonCode = responseReasonCode
	return fieldDecision, responseDecision, responseReasonCode
}

func setIDMaskStrategyForDecision(fieldDecision setIDFieldDecision) (string, bool) {
	if !fieldDecision.Visible {
		return "", false
	}
	defaultRuleRef := strings.ToLower(strings.TrimSpace(fieldDecision.DefaultRuleRef))
	if !strings.HasPrefix(defaultRuleRef, "mask://") {
		return "", false
	}
	maskStrategy := strings.TrimSpace(strings.TrimPrefix(defaultRuleRef, "mask://"))
	if maskStrategy == "" {
		maskStrategy = fieldMaskStrategyRedact
	}
	return maskStrategy, true
}

func briefSetIDFieldDecisions(fieldDecisions []setIDFieldDecision) string {
	if len(fieldDecisions) == 0 {
		return "-"
	}
	brief := make([]string, 0, len(fieldDecisions))
	for _, item := range fieldDecisions {
		brief = append(brief, strings.Join([]string{
			strings.TrimSpace(item.FieldKey),
			strings.TrimSpace(item.Decision),
			strings.TrimSpace(item.ReasonCode),
			strings.TrimSpace(item.Visibility),
		}, ":"))
	}
	return strings.Join(brief, ",")
}
