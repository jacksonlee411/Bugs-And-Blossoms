package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

const (
	orgUnitCreateFieldOrgCode = "org_code"
	orgUnitCreateFieldOrgType = "d_org_type"
)

type orgUnitCreateFieldDecisionsAPIResponse struct {
	CapabilityKey         string               `json:"capability_key"`
	BaselineCapabilityKey string               `json:"baseline_capability_key,omitempty"`
	BusinessUnitOrgCode   string               `json:"business_unit_org_code"`
	AsOf                  string               `json:"as_of"`
	PolicyVersion         string               `json:"policy_version"`
	PolicyVersionAlg      string               `json:"policy_version_alg,omitempty"`
	IntentPolicyVersion   string               `json:"intent_policy_version,omitempty"`
	BaselinePolicyVersion string               `json:"baseline_policy_version,omitempty"`
	FieldDecisions        []setIDFieldDecision `json:"field_decisions"`
}

func handleOrgUnitCreateFieldDecisionsAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if store == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}

	effectiveDate := strings.TrimSpace(r.URL.Query().Get("effective_date"))
	if effectiveDate == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "effective_date required")
		return
	}
	if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_effective_date", "invalid effective_date")
		return
	}

	parentOrgCode := strings.TrimSpace(r.URL.Query().Get("parent_org_code"))
	businessUnitRef, err := resolveCreateFieldDecisionBusinessUnitRef(r.Context(), store, tenant.ID, parentOrgCode)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_create_field_decisions_context_failed")
		}
		return
	}

	capCtx, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       orgUnitCreateFieldPolicyCapabilityKey,
		BusinessUnitOrgCode: businessUnitRef.OrgCode,
		AsOf:                effectiveDate,
		RequireBusinessUnit: businessUnitRef.OrgCode != "",
	})
	if capErr != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
		return
	}

	_, areaReasonCode, areaAllowed := evaluateFunctionalAreaGate(tenant.ID, capCtx.CapabilityKey)
	if !areaAllowed {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusForbidden, areaReasonCode, functionalAreaErrorMessage(areaReasonCode))
		return
	}

	decisions := make([]setIDFieldDecision, 0, 2)
	for _, fieldKey := range []string{orgUnitCreateFieldOrgCode, orgUnitCreateFieldOrgType} {
		decision, resolveErr := defaultSetIDStrategyRegistryStore.resolveFieldDecision(
			r.Context(),
			tenant.ID,
			capCtx.CapabilityKey,
			fieldKey,
			businessUnitRef.OrgNodeKey,
			capCtx.AsOf,
		)
		if resolveErr != nil {
			status, code := statusCodeForFieldDecisionError(resolveErr)
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, code)
			return
		}
		decisions = append(decisions, decision)
	}

	effectivePolicyVersion, policyParts := resolveOrgUnitEffectivePolicyVersion(tenant.ID, capCtx.CapabilityKey)

	response := orgUnitCreateFieldDecisionsAPIResponse{
		CapabilityKey:         capCtx.CapabilityKey,
		BaselineCapabilityKey: policyParts.BaselineCapabilityKey,
		BusinessUnitOrgCode:   capCtx.BusinessUnitOrgCode,
		AsOf:                  capCtx.AsOf,
		PolicyVersion:         effectivePolicyVersion,
		PolicyVersionAlg:      orgUnitEffectivePolicyVersionAlgorithm,
		IntentPolicyVersion:   policyParts.IntentPolicyVersion,
		BaselinePolicyVersion: policyParts.BaselinePolicyVersion,
		FieldDecisions:        decisions,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func resolveCreateFieldDecisionBusinessUnitRef(ctx context.Context, store OrgUnitStore, tenantID string, parentOrgCode string) (setIDResolvedOrgRef, error) {
	parentOrgCode = strings.TrimSpace(parentOrgCode)
	if parentOrgCode == "" {
		return setIDResolvedOrgRef{}, nil
	}
	return resolveSetIDOrgCodeRef(ctx, tenantID, parentOrgCode, store)
}
