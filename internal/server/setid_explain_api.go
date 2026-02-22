package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

const (
	explainLevelBrief = "brief"
	explainLevelFull  = "full"
)

type setIDExplainResponse struct {
	TraceID           string               `json:"trace_id"`
	RequestID         string               `json:"request_id"`
	CapabilityKey     string               `json:"capability_key"`
	TenantID          string               `json:"tenant_id"`
	BusinessUnitID    string               `json:"business_unit_id"`
	AsOf              string               `json:"as_of"`
	OrgUnitID         string               `json:"org_unit_id,omitempty"`
	ScopeCode         string               `json:"scope_code"`
	ResolvedSetID     string               `json:"resolved_setid"`
	ResolvedPackageID string               `json:"resolved_package_id"`
	PackageOwner      string               `json:"package_owner"`
	Decision          string               `json:"decision"`
	ReasonCode        string               `json:"reason_code,omitempty"`
	Level             string               `json:"level"`
	FieldDecisions    []setIDFieldDecision `json:"field_decisions"`
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
	scopeCode := strings.TrimSpace(r.URL.Query().Get("scope_code"))
	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if capabilityKey == "" || fieldKey == "" || businessUnitID == "" || scopeCode == "" || asOf == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "capability_key/field_key/business_unit_id/scope_code/as_of required")
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

	resolvedSetID, err := store.ResolveSetID(r.Context(), tenant.ID, orgUnitID, asOf)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, scopeReasonOwnerContextForbidden, "business unit context forbidden")
		return
	}
	targetSetID := strings.TrimSpace(r.URL.Query().Get("setid"))
	if targetSetID == "" {
		targetSetID = resolvedSetID
	}
	sub, err := store.GetScopeSubscription(r.Context(), tenant.ID, targetSetID, scopeCode, asOf)
	if err != nil {
		writeScopeAPIError(w, r, err, "scope_subscription_get_failed")
		return
	}

	fieldDecision, err := defaultSetIDStrategyRegistryRuntime.resolveFieldDecision(tenant.ID, capabilityKey, fieldKey, businessUnitID, asOf)
	if err != nil {
		status, code := statusCodeForFieldDecisionError(err)
		routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, code)
		return
	}

	fieldDecision.Decision = "allow"
	if fieldDecision.Required {
		fieldDecision.ReasonCode = fieldRequiredInContextCode
	}
	responseDecision := "allow"
	responseReasonCode := ""
	if !fieldDecision.Visible {
		fieldDecision.Decision = "deny"
		fieldDecision.ReasonCode = fieldHiddenInContextCode
		responseDecision = "deny"
		responseReasonCode = fieldHiddenInContextCode
	}

	response := setIDExplainResponse{
		TraceID:           traceIDFromRequestHeader(r),
		RequestID:         requestID,
		CapabilityKey:     capabilityKey,
		TenantID:          tenant.ID,
		BusinessUnitID:    businessUnitID,
		AsOf:              asOf,
		OrgUnitID:         orgUnitID,
		ScopeCode:         scopeCode,
		ResolvedSetID:     strings.ToUpper(strings.TrimSpace(resolvedSetID)),
		ResolvedPackageID: sub.PackageID,
		PackageOwner:      sub.PackageOwner,
		Decision:          responseDecision,
		ReasonCode:        responseReasonCode,
		Level:             level,
		FieldDecisions:    []setIDFieldDecision{fieldDecision},
	}
	if level == explainLevelBrief {
		response.TenantID = ""
		response.OrgUnitID = ""
	}
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
