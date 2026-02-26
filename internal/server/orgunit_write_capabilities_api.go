package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitWriteCapabilitiesAPIResponse struct {
	Intent           string            `json:"intent"`
	CapabilityKey    string            `json:"capability_key"`
	PolicyVersion    string            `json:"policy_version"`
	TreeInitialized  bool              `json:"tree_initialized"`
	Enabled          bool              `json:"enabled"`
	DenyReasons      []string          `json:"deny_reasons"`
	AllowedFields    []string          `json:"allowed_fields"`
	FieldPayloadKeys map[string]string `json:"field_payload_keys"`
}

type orgUnitWriteCapabilitiesStore interface {
	ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error)
	ResolveAppendFacts(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitAppendFacts, error)
	ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error)
}

var resolveWriteCapabilitiesInAPI = orgunitservices.ResolveWriteCapabilities

func handleOrgUnitWriteCapabilitiesAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	capStore, ok := store.(orgUnitWriteCapabilitiesStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}

	intent := strings.TrimSpace(r.URL.Query().Get("intent"))
	rawCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	effectiveDate := strings.TrimSpace(r.URL.Query().Get("effective_date"))
	targetEffectiveDate := strings.TrimSpace(r.URL.Query().Get("target_effective_date"))
	if intent == "" || effectiveDate == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "intent/effective_date required")
		return
	}

	capabilityKey, ok := orgUnitFieldPolicyCapabilityKeyForWriteIntent(intent)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "ORG_INTENT_NOT_SUPPORTED", "intent not supported")
		return
	}

	normalizedCode := ""
	if rawCode != "" {
		var err error
		normalizedCode, err = orgunitpkg.NormalizeOrgCode(rawCode)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
			return
		}
	}
	if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "effective_date invalid")
		return
	}
	if targetEffectiveDate != "" {
		if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "target_effective_date invalid")
			return
		}
	}

	extConfigs, err := capStore.ListEnabledTenantFieldConfigsAsOf(r.Context(), tenant.ID, effectiveDate)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_write_capabilities_ext_fields_failed")
		return
	}
	extFieldKeys := make([]string, 0, len(extConfigs))
	for _, cfg := range extConfigs {
		key := strings.TrimSpace(cfg.FieldKey)
		if !isAllowedOrgUnitExtFieldKey(key) {
			continue
		}
		extFieldKeys = append(extFieldKeys, key)
	}
	sort.Strings(extFieldKeys)

	canAdmin := canEditOrgNodes(r.Context())

	treeInitialized, err := capStore.IsOrgTreeInitialized(r.Context(), tenant.ID)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_write_capabilities_tree_failed")
		return
	}

	facts := orgunitservices.OrgUnitWriteCapabilitiesFacts{
		CanAdmin:            canAdmin,
		TreeInitialized:     treeInitialized,
		OrgCode:             normalizedCode,
		EffectiveDate:       effectiveDate,
		TargetEffectiveDate: targetEffectiveDate,
	}

	switch intent {
	case string(orgunitservices.OrgUnitWriteIntentCreateOrg):
		if normalizedCode != "" {
			if _, err := capStore.ResolveOrgID(r.Context(), tenant.ID, normalizedCode); err == nil {
				facts.OrgAlreadyExists = true
			} else if !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				writeInternalAPIError(w, r, err, "orgunit_write_capabilities_resolve_org_failed")
				return
			}
		}

	case string(orgunitservices.OrgUnitWriteIntentAddVersion), string(orgunitservices.OrgUnitWriteIntentInsertVersion):
		if normalizedCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
			return
		}
		orgID, err := capStore.ResolveOrgID(r.Context(), tenant.ID, normalizedCode)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				facts.TargetExistsAsOf = false
				break
			}
			writeInternalAPIError(w, r, err, "orgunit_write_capabilities_resolve_org_failed")
			return
		}
		appendFacts, err := capStore.ResolveAppendFacts(r.Context(), tenant.ID, orgID, effectiveDate)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_write_capabilities_facts_failed")
			return
		}
		facts.TargetExistsAsOf = appendFacts.TargetExistsAsOf

	case string(orgunitservices.OrgUnitWriteIntentCorrect):
		if normalizedCode == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code required")
			return
		}
		if targetEffectiveDate == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "target_effective_date required for correct")
			return
		}
		orgID, err := capStore.ResolveOrgID(r.Context(), tenant.ID, normalizedCode)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				facts.TargetExistsAsOf = false
				break
			}
			writeInternalAPIError(w, r, err, "orgunit_write_capabilities_resolve_org_failed")
			return
		}
		facts.TargetExistsAsOf = true
		target, err := capStore.ResolveMutationTargetEvent(r.Context(), tenant.ID, orgID, targetEffectiveDate)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_write_capabilities_target_failed")
			return
		}
		if !target.HasEffective {
			facts.TargetEventNotFound = !target.HasRaw
			facts.TargetEventRescinded = target.HasRaw
		}

	}

	decision, err := resolveWriteCapabilitiesInAPI(orgunitservices.OrgUnitWriteIntent(intent), extFieldKeys, facts)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_write_capabilities_policy_failed")
		return
	}

	resp := orgUnitWriteCapabilitiesAPIResponse{
		Intent:           intent,
		CapabilityKey:    capabilityKey,
		PolicyVersion:    defaultPolicyActivationRuntime.activePolicyVersion(tenant.ID, capabilityKey),
		TreeInitialized:  treeInitialized,
		Enabled:          decision.Enabled,
		DenyReasons:      decision.DenyReasons,
		AllowedFields:    decision.AllowedFields,
		FieldPayloadKeys: decision.FieldPayloadKeys,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
