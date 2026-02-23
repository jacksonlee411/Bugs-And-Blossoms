package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
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
	BusinessUnitID        string               `json:"business_unit_id"`
	AsOf                  string               `json:"as_of"`
	OrgUnitID             string               `json:"org_unit_id,omitempty"`
	ResolvedSetID         string               `json:"resolved_setid"`
	ResolvedConfigVersion string               `json:"resolved_config_version,omitempty"`
	Decision              string               `json:"decision"`
	ReasonCode            string               `json:"reason_code"`
	Level                 string               `json:"level"`
	FieldDecisions        []setIDFieldDecision `json:"field_decisions"`
}

func handleSetIDExplainAPI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
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
	businessUnitID := strings.TrimSpace(r.URL.Query().Get("business_unit_id"))
	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	requestID := normalizeSetIDExplainRequestID(r)
	if capabilityKey == "" || fieldKey == "" || businessUnitID == "" || asOf == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "capability_key/field_key/business_unit_id/as_of required")
		return
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}
	if _, err := parseOrgID8(businessUnitID); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_business_unit_id", "invalid business_unit_id")
		return
	}
	capCtx, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       capabilityKey,
		BusinessUnitID:      businessUnitID,
		AsOf:                asOf,
		RequireBusinessUnit: true,
	})
	if capErr != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
		return
	}
	capabilityKey = capCtx.CapabilityKey
	businessUnitID = capCtx.BusinessUnitID
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

	orgUnitID := strings.TrimSpace(r.URL.Query().Get("org_unit_id"))
	if orgUnitID == "" {
		orgUnitID = businessUnitID
	}
	if _, err := parseOrgID8(orgUnitID); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_org_unit_id", "invalid org_unit_id")
		return
	}
	dynamicRelations := preloadCapabilityDynamicRelations(r.Context(), businessUnitID)
	if !dynamicRelations.actorManages(orgUnitID, asOf) {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, capabilityReasonContextMismatch, "capability context mismatch")
		return
	}

	resolvedSetID, err := store.ResolveSetID(r.Context(), tenant.ID, orgUnitID, asOf)
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

	responseDecision := "allow"
	responseReasonCode := fieldVisibleInContextCode
	if fieldDecision.Required {
		responseReasonCode = fieldRequiredInContextCode
	}
	if !fieldDecision.Visible {
		responseDecision = "deny"
		responseReasonCode = fieldHiddenInContextCode
	}
	fieldDecision.Decision = responseDecision
	fieldDecision.ReasonCode = responseReasonCode

	traceID := traceIDFromRequestHeader(r)
	if traceID == "" {
		traceID = fallbackSetIDExplainTraceID(requestID, capabilityKey, businessUnitID, asOf)
	}

	response := setIDExplainResponse{
		TraceID:               traceID,
		RequestID:             requestID,
		CapabilityKey:         capabilityKey,
		SetID:                 resolvedSetID,
		FunctionalAreaKey:     functionalAreaKey,
		PolicyVersion:         capabilityPolicyVersionBaseline,
		TenantID:              tenant.ID,
		BusinessUnitID:        businessUnitID,
		AsOf:                  asOf,
		OrgUnitID:             orgUnitID,
		ResolvedSetID:         resolvedSetID,
		ResolvedConfigVersion: capabilityPolicyVersionBaseline,
		Decision:              responseDecision,
		ReasonCode:            responseReasonCode,
		Level:                 level,
		FieldDecisions:        []setIDFieldDecision{fieldDecision},
	}
	if level == explainLevelBrief {
		response.TenantID = ""
		response.OrgUnitID = ""
		response.ResolvedConfigVersion = ""
	}
	logSetIDExplainAudit(response)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
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
		"setid_explain decision=%s reason_code=%s capability_key=%s setid=%s policy_version=%s functional_area_key=%s level=%s",
		response.Decision,
		response.ReasonCode,
		response.CapabilityKey,
		response.SetID,
		response.PolicyVersion,
		response.FunctionalAreaKey,
		response.Level,
	)
}
