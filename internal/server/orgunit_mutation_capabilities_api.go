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
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type orgUnitMutationCapabilitiesAPIResponse struct {
	OrgCode                  string                              `json:"org_code"`
	EffectiveDate            string                              `json:"effective_date"`
	EffectiveTargetEventType string                              `json:"effective_target_event_type"`
	RawTargetEventType       string                              `json:"raw_target_event_type"`
	Capabilities             orgUnitMutationCapabilitiesEnvelope `json:"capabilities"`
}

type orgUnitMutationCapabilitiesEnvelope struct {
	CorrectEvent  orgUnitCorrectEventCapability  `json:"correct_event"`
	CorrectStatus orgUnitCorrectStatusCapability `json:"correct_status"`
	RescindEvent  orgUnitBasicCapability         `json:"rescind_event"`
	RescindOrg    orgUnitBasicCapability         `json:"rescind_org"`
}

type orgUnitCorrectEventCapability struct {
	Enabled          bool              `json:"enabled"`
	AllowedFields    []string          `json:"allowed_fields"`
	FieldPayloadKeys map[string]string `json:"field_payload_keys"`
	DenyReasons      []string          `json:"deny_reasons"`
}

type orgUnitCorrectStatusCapability struct {
	Enabled               bool     `json:"enabled"`
	AllowedTargetStatuses []string `json:"allowed_target_statuses"`
	DenyReasons           []string `json:"deny_reasons"`
}

type orgUnitBasicCapability struct {
	Enabled     bool     `json:"enabled"`
	DenyReasons []string `json:"deny_reasons"`
}

type orgUnitMutationCapabilitiesStore interface {
	ResolveMutationTargetEvent(ctx context.Context, tenantID string, orgID int, effectiveDate string) (orgUnitMutationTargetEvent, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error)
	EvaluateRescindOrgDenyReasons(ctx context.Context, tenantID string, orgID int) ([]string, error)
}

func handleOrgUnitMutationCapabilitiesAPI(w http.ResponseWriter, r *http.Request, store OrgUnitStore) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	capStore, ok := store.(orgUnitMutationCapabilitiesStore)
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

	orgID, err := store.ResolveOrgID(r.Context(), tenant.ID, normalizedCode)
	if err != nil {
		switch {
		case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "org_code_invalid", "org_code invalid")
		case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "org_code_not_found", "org_code not found")
		default:
			writeInternalAPIError(w, r, err, "orgunit_resolve_org_code_failed")
		}
		return
	}

	target, err := capStore.ResolveMutationTargetEvent(r.Context(), tenant.ID, orgID, effectiveDate)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_mutation_target_resolve_failed")
		return
	}
	if !target.HasEffective {
		if target.HasRaw {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, orgUnitErrEventRescinded, "org event rescinded")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, orgUnitErrEventNotFound, "org event not found")
		return
	}

	effectiveTarget := strings.TrimSpace(target.EffectiveEventType)
	rawTarget := strings.TrimSpace(target.RawEventType)
	if rawTarget == "" {
		rawTarget = effectiveTarget
	}

	ctx := r.Context()
	canAdmin := canEditOrgNodes(ctx)

	extFieldKeys := []string{}
	if effectiveTarget == "CREATE" {
		extConfigs, err := capStore.ListEnabledTenantFieldConfigsAsOf(r.Context(), tenant.ID, effectiveDate)
		if err != nil {
			writeInternalAPIError(w, r, err, "orgunit_mutation_ext_fields_failed")
			return
		}
		extFieldKeys = make([]string, 0, len(extConfigs))
		for _, cfg := range extConfigs {
			key := strings.TrimSpace(cfg.FieldKey)
			if key == "" {
				continue
			}
			extFieldKeys = append(extFieldKeys, key)
		}
		sort.Strings(extFieldKeys)
	}

	coreAllowed := allowedCoreFieldsForTargetEvent(effectiveTarget)
	allowedFields := append([]string(nil), coreAllowed...)
	allowedFields = append(allowedFields, extFieldKeys...)
	sort.Strings(allowedFields)

	fieldPayloadKeys := make(map[string]string, len(allowedFields))
	isExt := make(map[string]struct{}, len(extFieldKeys))
	for _, key := range extFieldKeys {
		isExt[key] = struct{}{}
	}
	for _, field := range allowedFields {
		if _, ok := isExt[field]; ok {
			fieldPayloadKeys[field] = "ext." + field
			continue
		}
		switch field {
		case "effective_date":
			fieldPayloadKeys[field] = "effective_date"
		case "name":
			fieldPayloadKeys[field] = "name"
		case "parent_org_code":
			fieldPayloadKeys[field] = "parent_org_code"
		case "is_business_unit":
			fieldPayloadKeys[field] = "is_business_unit"
		case "manager_pernr":
			fieldPayloadKeys[field] = "manager_pernr"
		}
	}

	correctEventDeny := []string{}
	if !canAdmin {
		correctEventDeny = append(correctEventDeny, "FORBIDDEN")
	}
	correctEvent := orgUnitCorrectEventCapability{
		Enabled:          canAdmin,
		AllowedFields:    allowedFields,
		FieldPayloadKeys: fieldPayloadKeys,
		DenyReasons:      dedupDenyReasons(correctEventDeny),
	}

	statusSupported := effectiveTarget == string(orgunittypes.OrgUnitEventEnable) || effectiveTarget == string(orgunittypes.OrgUnitEventDisable)
	correctStatusDeny := []string{}
	allowedTargetStatuses := []string{}
	if statusSupported {
		allowedTargetStatuses = []string{"active", "disabled"}
	} else {
		correctStatusDeny = append(correctStatusDeny, orgUnitErrStatusCorrectionUnsupported)
	}
	if !canAdmin {
		correctStatusDeny = append(correctStatusDeny, "FORBIDDEN")
	}
	correctStatus := orgUnitCorrectStatusCapability{
		Enabled:               canAdmin && statusSupported,
		AllowedTargetStatuses: allowedTargetStatuses,
		DenyReasons:           dedupDenyReasons(correctStatusDeny),
	}

	rescindEventDeny := []string{}
	if !canAdmin {
		rescindEventDeny = append(rescindEventDeny, "FORBIDDEN")
	}
	rescindEvent := orgUnitBasicCapability{Enabled: canAdmin, DenyReasons: dedupDenyReasons(rescindEventDeny)}

	rescindOrgDeny, err := capStore.EvaluateRescindOrgDenyReasons(r.Context(), tenant.ID, orgID)
	if err != nil {
		writeInternalAPIError(w, r, err, "orgunit_mutation_rescind_org_failed")
		return
	}
	if !canAdmin {
		rescindOrgDeny = append(rescindOrgDeny, "FORBIDDEN")
	}
	rescindOrgDeny = dedupDenyReasons(rescindOrgDeny)
	enabledRescindOrg := canAdmin && len(rescindOrgDeny) == 0
	rescindOrg := orgUnitBasicCapability{Enabled: enabledRescindOrg, DenyReasons: rescindOrgDeny}

	resp := orgUnitMutationCapabilitiesAPIResponse{
		OrgCode:                  normalizedCode,
		EffectiveDate:            effectiveDate,
		EffectiveTargetEventType: effectiveTarget,
		RawTargetEventType:       rawTarget,
		Capabilities: orgUnitMutationCapabilitiesEnvelope{
			CorrectEvent:  correctEvent,
			CorrectStatus: correctStatus,
			RescindEvent:  rescindEvent,
			RescindOrg:    rescindOrg,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func allowedCoreFieldsForTargetEvent(eventType string) []string {
	switch strings.TrimSpace(eventType) {
	case "CREATE":
		return []string{"effective_date", "is_business_unit", "manager_pernr", "name", "parent_org_code"}
	case "RENAME":
		return []string{"effective_date", "name"}
	case "MOVE":
		return []string{"effective_date", "parent_org_code"}
	case "SET_BUSINESS_UNIT":
		return []string{"effective_date", "is_business_unit"}
	case "DISABLE", "ENABLE":
		return []string{"effective_date"}
	default:
		return []string{"effective_date"}
	}
}
