package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	personalizationModeTenantOnly   = "tenant_only"
	personalizationModeSetID        = "setid"
	personalizationModeScopePackage = "scope_package"
	orgLevelTenant                  = "tenant"
	orgLevelBusinessUnit            = "business_unit"
	fieldPolicyConflictCode         = "FIELD_POLICY_CONFLICT"
	fieldPolicyMissingCode          = "FIELD_POLICY_MISSING"
	explainRequiredCode             = "EXPLAIN_REQUIRED"
	fieldRequiredInContextCode      = "FIELD_REQUIRED_IN_CONTEXT"
	fieldHiddenInContextCode        = "FIELD_HIDDEN_IN_CONTEXT"
	fieldDefaultRuleMissingCode     = "FIELD_DEFAULT_RULE_MISSING"
)

var (
	capabilityKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	fieldKeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

type setIDStrategyRegistryItem struct {
	CapabilityKey       string `json:"capability_key"`
	OwnerModule         string `json:"owner_module"`
	FieldKey            string `json:"field_key"`
	PersonalizationMode string `json:"personalization_mode"`
	ScopeCode           string `json:"scope_code,omitempty"`
	OrgLevel            string `json:"org_level"`
	BusinessUnitID      string `json:"business_unit_id,omitempty"`
	Required            bool   `json:"required"`
	Visible             bool   `json:"visible"`
	DefaultRuleRef      string `json:"default_rule_ref,omitempty"`
	DefaultValue        string `json:"default_value,omitempty"`
	Priority            int    `json:"priority"`
	ExplainRequired     bool   `json:"explain_required"`
	IsStable            bool   `json:"is_stable"`
	ChangePolicy        string `json:"change_policy"`
	EffectiveDate       string `json:"effective_date"`
	EndDate             string `json:"end_date,omitempty"`
	UpdatedAt           string `json:"updated_at"`
}

type setIDStrategyRegistryUpsertAPIRequest struct {
	CapabilityKey       string `json:"capability_key"`
	OwnerModule         string `json:"owner_module"`
	FieldKey            string `json:"field_key"`
	PersonalizationMode string `json:"personalization_mode"`
	ScopeCode           string `json:"scope_code"`
	OrgLevel            string `json:"org_level"`
	BusinessUnitID      string `json:"business_unit_id"`
	Required            bool   `json:"required"`
	Visible             bool   `json:"visible"`
	DefaultRuleRef      string `json:"default_rule_ref"`
	DefaultValue        string `json:"default_value"`
	Priority            int    `json:"priority"`
	ExplainRequired     bool   `json:"explain_required"`
	IsStable            bool   `json:"is_stable"`
	ChangePolicy        string `json:"change_policy"`
	EffectiveDate       string `json:"effective_date"`
	EndDate             string `json:"end_date"`
	RequestID           string `json:"request_id"`
}

type setIDFieldDecision struct {
	CapabilityKey      string `json:"capability_key"`
	FieldKey           string `json:"field_key"`
	Required           bool   `json:"required"`
	Visible            bool   `json:"visible"`
	DefaultRuleRef     string `json:"default_rule_ref,omitempty"`
	ResolvedDefaultVal string `json:"resolved_default_value,omitempty"`
	Decision           string `json:"decision"`
	ReasonCode         string `json:"reason_code,omitempty"`
}

type setIDStrategyRegistryRuntime struct {
	mu       sync.RWMutex
	byTenant map[string][]setIDStrategyRegistryItem
}

func newSetIDStrategyRegistryRuntime() *setIDStrategyRegistryRuntime {
	return &setIDStrategyRegistryRuntime{
		byTenant: make(map[string][]setIDStrategyRegistryItem),
	}
}

var defaultSetIDStrategyRegistryRuntime = newSetIDStrategyRegistryRuntime()

func resetSetIDStrategyRegistryRuntimeForTest() {
	defaultSetIDStrategyRegistryRuntime = newSetIDStrategyRegistryRuntime()
}

func strategyRegistrySortKey(item setIDStrategyRegistryItem) string {
	return strings.Join([]string{
		item.CapabilityKey,
		item.FieldKey,
		item.OrgLevel,
		item.BusinessUnitID,
		item.EffectiveDate,
	}, "|")
}

func normalizeStrategyRegistryItem(req setIDStrategyRegistryUpsertAPIRequest) setIDStrategyRegistryItem {
	item := setIDStrategyRegistryItem{
		CapabilityKey:       strings.ToLower(strings.TrimSpace(req.CapabilityKey)),
		OwnerModule:         strings.ToLower(strings.TrimSpace(req.OwnerModule)),
		FieldKey:            strings.ToLower(strings.TrimSpace(req.FieldKey)),
		PersonalizationMode: strings.ToLower(strings.TrimSpace(req.PersonalizationMode)),
		ScopeCode:           strings.ToLower(strings.TrimSpace(req.ScopeCode)),
		OrgLevel:            strings.ToLower(strings.TrimSpace(req.OrgLevel)),
		BusinessUnitID:      strings.TrimSpace(req.BusinessUnitID),
		Required:            req.Required,
		Visible:             req.Visible,
		DefaultRuleRef:      strings.TrimSpace(req.DefaultRuleRef),
		DefaultValue:        strings.TrimSpace(req.DefaultValue),
		Priority:            req.Priority,
		ExplainRequired:     req.ExplainRequired,
		IsStable:            req.IsStable,
		ChangePolicy:        strings.ToLower(strings.TrimSpace(req.ChangePolicy)),
		EffectiveDate:       strings.TrimSpace(req.EffectiveDate),
		EndDate:             strings.TrimSpace(req.EndDate),
	}
	if item.Priority <= 0 {
		item.Priority = 100
	}
	if item.ChangePolicy == "" {
		item.ChangePolicy = "plan_required"
	}
	if item.OrgLevel == orgLevelTenant {
		item.BusinessUnitID = ""
	}
	item.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return item
}

func validateStrategyRegistryItem(item setIDStrategyRegistryItem) (int, string, string) {
	if item.CapabilityKey == "" || item.OwnerModule == "" || item.FieldKey == "" || item.PersonalizationMode == "" || item.OrgLevel == "" || item.EffectiveDate == "" {
		return http.StatusBadRequest, "invalid_request", "capability_key/owner_module/field_key/personalization_mode/org_level/effective_date required"
	}
	if !capabilityKeyPattern.MatchString(item.CapabilityKey) {
		return http.StatusBadRequest, "invalid_capability_key", "invalid capability_key"
	}
	if !fieldKeyPattern.MatchString(item.FieldKey) {
		return http.StatusBadRequest, "invalid_field_key", "invalid field_key"
	}
	switch item.PersonalizationMode {
	case personalizationModeTenantOnly, personalizationModeSetID, personalizationModeScopePackage:
	default:
		return http.StatusUnprocessableEntity, "personalization_mode_invalid", "personalization_mode invalid"
	}
	switch item.OrgLevel {
	case orgLevelTenant:
	case orgLevelBusinessUnit:
		if item.BusinessUnitID == "" {
			return http.StatusBadRequest, "invalid_business_unit_id", "business_unit_id required"
		}
		if _, err := parseOrgID8(item.BusinessUnitID); err != nil {
			return http.StatusBadRequest, "invalid_business_unit_id", "invalid business_unit_id"
		}
	default:
		return http.StatusUnprocessableEntity, "org_level_invalid", "org_level invalid"
	}
	if item.PersonalizationMode == personalizationModeScopePackage && item.ScopeCode == "" {
		return http.StatusBadRequest, "invalid_scope_code", "scope_code required for scope_package mode"
	}
	if item.PersonalizationMode != personalizationModeTenantOnly && !item.ExplainRequired {
		return http.StatusUnprocessableEntity, explainRequiredCode, "explain_required must be true when personalization_mode is not tenant_only"
	}
	if !item.Visible && item.Required {
		return http.StatusUnprocessableEntity, fieldPolicyConflictCode, fieldPolicyConflictCode
	}
	effectiveDate, err := time.Parse("2006-01-02", item.EffectiveDate)
	if err != nil {
		return http.StatusBadRequest, "invalid_effective_date", "invalid effective_date"
	}
	if item.EndDate != "" {
		endDate, endErr := time.Parse("2006-01-02", item.EndDate)
		if endErr != nil {
			return http.StatusBadRequest, "invalid_end_date", "invalid end_date"
		}
		if !endDate.After(effectiveDate) {
			return http.StatusUnprocessableEntity, fieldPolicyConflictCode, "end_date must be greater than effective_date"
		}
	}
	return 0, "", ""
}

func (s *setIDStrategyRegistryRuntime) upsert(tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.byTenant[tenantID]
	itemKey := strategyRegistrySortKey(item)
	updated := false
	for i := range items {
		if strategyRegistrySortKey(items[i]) == itemKey {
			items[i] = item
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b setIDStrategyRegistryItem) int {
		return strings.Compare(strategyRegistrySortKey(a), strategyRegistrySortKey(b))
	})
	s.byTenant[tenantID] = items
	return item, updated
}

func (s *setIDStrategyRegistryRuntime) list(tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error) {
	asOfDate, err := time.Parse("2006-01-02", strings.TrimSpace(asOf))
	if err != nil {
		return nil, errors.New("invalid as_of")
	}
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	fieldKey = strings.ToLower(strings.TrimSpace(fieldKey))

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := s.byTenant[tenantID]
	out := make([]setIDStrategyRegistryItem, 0, len(items))
	for _, item := range items {
		if capabilityKey != "" && item.CapabilityKey != capabilityKey {
			continue
		}
		if fieldKey != "" && item.FieldKey != fieldKey {
			continue
		}
		effectiveDate, parseErr := time.Parse("2006-01-02", item.EffectiveDate)
		if parseErr != nil || effectiveDate.After(asOfDate) {
			continue
		}
		if item.EndDate != "" {
			endDate, endErr := time.Parse("2006-01-02", item.EndDate)
			if endErr == nil && !endDate.After(asOfDate) {
				continue
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *setIDStrategyRegistryRuntime) resolveFieldDecision(
	tenantID string,
	capabilityKey string,
	fieldKey string,
	businessUnitID string,
	asOf string,
) (setIDFieldDecision, error) {
	items, err := s.list(tenantID, capabilityKey, fieldKey, asOf)
	if err != nil {
		return setIDFieldDecision{}, err
	}
	businessUnitID = strings.TrimSpace(businessUnitID)
	type candidate struct {
		item        setIDStrategyRegistryItem
		specificity int
	}
	var chosen *candidate
	for _, item := range items {
		current := candidate{item: item}
		switch item.OrgLevel {
		case orgLevelBusinessUnit:
			if !strings.EqualFold(item.BusinessUnitID, businessUnitID) {
				continue
			}
			current.specificity = 2
		case orgLevelTenant:
			current.specificity = 1
		default:
			continue
		}
		if chosen == nil || current.specificity > chosen.specificity || (current.specificity == chosen.specificity && current.item.Priority > chosen.item.Priority) || (current.specificity == chosen.specificity && current.item.Priority == chosen.item.Priority && current.item.EffectiveDate > chosen.item.EffectiveDate) {
			chosen = &current
		}
	}
	if chosen == nil {
		return setIDFieldDecision{}, errors.New(fieldPolicyMissingCode)
	}
	if chosen.item.Required && !chosen.item.Visible {
		return setIDFieldDecision{}, errors.New(fieldPolicyConflictCode)
	}
	if chosen.item.DefaultRuleRef == "" && chosen.item.DefaultValue == "" {
		return setIDFieldDecision{}, errors.New(fieldDefaultRuleMissingCode)
	}
	return setIDFieldDecision{
		CapabilityKey:      chosen.item.CapabilityKey,
		FieldKey:           chosen.item.FieldKey,
		Required:           chosen.item.Required,
		Visible:            chosen.item.Visible,
		DefaultRuleRef:     chosen.item.DefaultRuleRef,
		ResolvedDefaultVal: chosen.item.DefaultValue,
		Decision:           "allow",
	}, nil
}

func handleSetIDStrategyRegistryAPI(w http.ResponseWriter, r *http.Request) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	switch r.Method {
	case http.MethodGet:
		asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
		if asOf == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "as_of required")
			return
		}
		items, err := defaultSetIDStrategyRegistryRuntime.list(tenant.ID, r.URL.Query().Get("capability_key"), r.URL.Query().Get("field_key"), asOf)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant_id": tenant.ID,
			"as_of":     asOf,
			"items":     items,
		})
		return
	case http.MethodPost:
		var req setIDStrategyRegistryUpsertAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		req.RequestID = strings.TrimSpace(req.RequestID)
		if req.RequestID == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "request_id required")
			return
		}
		item := normalizeStrategyRegistryItem(req)
		if status, code, message := validateStrategyRegistryItem(item); status != 0 {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
			return
		}
		saved, updated := defaultSetIDStrategyRegistryRuntime.upsert(tenant.ID, item)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if updated {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(saved)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}
