package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

var resolveOrgUnitMutationPolicyForAppend = orgunitservices.ResolvePolicy

type orgUnitAppendFacts struct {
	TreeInitialized  bool
	TargetExistsAsOf bool
	TargetStatusAsOf string
	IsRoot           bool
}

type orgUnitAppendCapabilitiesStore interface {
	ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error)
	ResolveAppendFacts(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitAppendFacts, error)
}

type orgUnitAppendCapability struct {
	Enabled          bool              `json:"enabled"`
	AllowedFields    []string          `json:"allowed_fields"`
	FieldPayloadKeys map[string]string `json:"field_payload_keys"`
	DenyReasons      []string          `json:"deny_reasons"`
}

type orgUnitAppendCapabilitiesAPIResponse struct {
	OrgCode       string `json:"org_code"`
	EffectiveDate string `json:"effective_date"`
	Capabilities  struct {
		Create      orgUnitAppendCapability            `json:"create"`
		EventUpdate map[string]orgUnitAppendCapability `json:"event_update"`
	} `json:"capabilities"`
}

func handleOrgUnitAppendCapabilitiesAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	appendStore, ok := store.(orgUnitAppendCapabilitiesStore)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_store_missing", "orgunit store missing")
		return
	}

	rawCode := strings.TrimSpace(r.URL.Query().Get("org_code"))
	effectiveDate := strings.TrimSpace(r.URL.Query().Get("effective_date"))
	if rawCode == "" || effectiveDate == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "org_code/effective_date required")
		return
	}
	normalizedCode, err := orgunitpkg.NormalizeOrgCode(rawCode)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		return
	}
	if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "effective_date invalid")
		return
	}

	treeInitialized, err := appendStore.IsOrgTreeInitialized(r.Context(), tenant.ID)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_append_tree_resolve_failed")
		return
	}

	orgID, err := appendStore.ResolveOrgID(r.Context(), tenant.ID, normalizedCode)
	orgExists := err == nil
	if err != nil && !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}

	extConfigs, err := appendStore.ListEnabledTenantFieldConfigsAsOf(r.Context(), tenant.ID, effectiveDate)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_append_ext_fields_failed")
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

	canAdmin := canEditOrgNodes(r.Context())

	createDecision, err := resolveOrgUnitMutationPolicyForAppend(orgunitservices.OrgUnitMutationPolicyKey{
		ActionKind:       orgunitservices.OrgUnitActionCreate,
		EmittedEventType: orgunitservices.OrgUnitEmittedCreate,
	}, orgunitservices.OrgUnitMutationPolicyFacts{
		CanAdmin:            canAdmin,
		TreeInitialized:     treeInitialized,
		OrgAlreadyExists:    orgExists,
		CreateAsRoot:        strings.EqualFold(normalizedCode, "ROOT"),
		EnabledExtFieldKeys: extFieldKeys,
	})
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_append_policy_failed")
		return
	}

	eventFacts := orgUnitAppendFacts{
		TreeInitialized: treeInitialized,
	}
	if orgExists {
		resolvedFacts, err := appendStore.ResolveAppendFacts(r.Context(), tenant.ID, orgID, effectiveDate)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_append_facts_failed")
			return
		}
		eventFacts = resolvedFacts
	}

	buildEventCapability := func(emitted orgunitservices.OrgUnitEmittedEventType) (orgUnitAppendCapability, error) {
		decision, err := resolveOrgUnitMutationPolicyForAppend(orgunitservices.OrgUnitMutationPolicyKey{
			ActionKind:       orgunitservices.OrgUnitActionEventUpdate,
			EmittedEventType: emitted,
		}, orgunitservices.OrgUnitMutationPolicyFacts{
			CanAdmin:            canAdmin,
			TreeInitialized:     eventFacts.TreeInitialized,
			TargetExistsAsOf:    eventFacts.TargetExistsAsOf,
			TargetStatusAsOf:    eventFacts.TargetStatusAsOf,
			IsRoot:              eventFacts.IsRoot,
			EnabledExtFieldKeys: extFieldKeys,
		})
		if err != nil {
			return orgUnitAppendCapability{}, err
		}
		return orgUnitAppendCapability{
			Enabled:          decision.Enabled,
			AllowedFields:    decision.AllowedFields,
			FieldPayloadKeys: decision.FieldPayloadKeys,
			DenyReasons:      decision.DenyReasons,
		}, nil
	}

	eventUpdate := make(map[string]orgUnitAppendCapability, 5)
	for _, emitted := range []orgunitservices.OrgUnitEmittedEventType{
		orgunitservices.OrgUnitEmittedRename,
		orgunitservices.OrgUnitEmittedMove,
		orgunitservices.OrgUnitEmittedDisable,
		orgunitservices.OrgUnitEmittedEnable,
		orgunitservices.OrgUnitEmittedSetBusinessUnit,
	} {
		capability, err := buildEventCapability(emitted)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_append_policy_failed")
			return
		}
		eventUpdate[string(emitted)] = capability
	}

	resp := orgUnitAppendCapabilitiesAPIResponse{
		OrgCode:       normalizedCode,
		EffectiveDate: effectiveDate,
	}
	resp.Capabilities.Create = orgUnitAppendCapability{
		Enabled:          createDecision.Enabled,
		AllowedFields:    createDecision.AllowedFields,
		FieldPayloadKeys: createDecision.FieldPayloadKeys,
		DenyReasons:      createDecision.DenyReasons,
	}
	resp.Capabilities.EventUpdate = eventUpdate

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
