package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/fieldpolicy"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

const (
	personalizationModeTenantOnly = "tenant_only"
	personalizationModeSetID      = "setid"
	orgApplicabilityTenant        = "tenant"
	orgApplicabilityBusinessUnit  = "business_unit"
	priorityModeBlendCustomFirst  = "blend_custom_first"
	priorityModeBlendDefltFirst   = "blend_deflt_first"
	priorityModeDefltUnsubscribed = "deflt_unsubscribed"
	localOverrideModeAllow        = "allow"
	localOverrideModeNoOverride   = "no_override"
	localOverrideModeNoLocal      = "no_local"
	fieldPolicyConflictCode       = "FIELD_POLICY_CONFLICT"
	fieldPolicyMissingCode        = "FIELD_POLICY_MISSING"
	explainRequiredCode           = "EXPLAIN_REQUIRED"
	fieldVisibleInContextCode     = "FIELD_VISIBLE_IN_CONTEXT"
	fieldRequiredInContextCode    = "FIELD_REQUIRED_IN_CONTEXT"
	fieldHiddenInContextCode      = "FIELD_HIDDEN_IN_CONTEXT"
	fieldMaskedInContextCode      = "FIELD_MASKED_IN_CONTEXT"
	fieldDefaultRuleMissingCode   = "FIELD_DEFAULT_RULE_MISSING"
	fieldPolicyDisableDeniedCode  = "FIELD_POLICY_DISABLE_NOT_ALLOWED"
	fieldPolicyRedundantOverride  = "FIELD_POLICY_REDUNDANT_OVERRIDE"

	fieldVisibilityVisible = "visible"
	fieldVisibilityHidden  = "hidden"
	fieldVisibilityMasked  = "masked"

	fieldMaskStrategyRedact         = "redact"
	fieldMaskedDefaultValueFallback = "***"

	strategySourceBaseline       = "baseline"
	strategySourceIntentOverride = "intent_override"
	fieldPolicyPriorityModeCode  = "FIELD_POLICY_PRIORITY_MODE_INVALID"
	fieldPolicyLocalModeCode     = "FIELD_POLICY_LOCAL_OVERRIDE_MODE_INVALID"
	fieldPolicyModeComboCode     = "FIELD_POLICY_MODE_COMBINATION_INVALID"
)

var (
	capabilityKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	fieldKeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	resolvedSetIDPattern = regexp.MustCompile(`^[A-Z0-9]{5}$`)
	errStrategyNotFound  = errors.New("SETID_STRATEGY_REGISTRY_NOT_FOUND")
	errDisableNotAllowed = errors.New(fieldPolicyDisableDeniedCode)
)

type setIDStrategyRegistryItem struct {
	CapabilityKey       string   `json:"capability_key"`
	OwnerModule         string   `json:"owner_module"`
	SourceType          string   `json:"source_type,omitempty"`
	FieldKey            string   `json:"field_key"`
	PersonalizationMode string   `json:"personalization_mode"`
	OrgApplicability    string   `json:"org_applicability"`
	BusinessUnitNodeKey string   `json:"business_unit_node_key,omitempty"`
	ResolvedSetID       string   `json:"resolved_setid,omitempty"`
	Required            bool     `json:"required"`
	Visible             bool     `json:"visible"`
	Maintainable        bool     `json:"maintainable"`
	DefaultRuleRef      string   `json:"default_rule_ref,omitempty"`
	DefaultValue        string   `json:"default_value,omitempty"`
	AllowedValueCodes   []string `json:"allowed_value_codes,omitempty"`
	Priority            int      `json:"priority"`
	PriorityMode        string   `json:"priority_mode"`
	LocalOverrideMode   string   `json:"local_override_mode"`
	ExplainRequired     bool     `json:"explain_required"`
	IsStable            bool     `json:"is_stable"`
	ChangePolicy        string   `json:"change_policy"`
	EffectiveDate       string   `json:"effective_date"`
	EndDate             string   `json:"end_date,omitempty"`
	UpdatedAt           string   `json:"updated_at"`
}

type setIDStrategyRegistryUpsertAPIRequest struct {
	CapabilityKey       string   `json:"capability_key"`
	OwnerModule         string   `json:"owner_module"`
	FieldKey            string   `json:"field_key"`
	PersonalizationMode string   `json:"personalization_mode"`
	OrgApplicability    string   `json:"org_applicability"`
	BusinessUnitOrgCode string   `json:"business_unit_org_code"`
	Required            bool     `json:"required"`
	Visible             bool     `json:"visible"`
	Maintainable        *bool    `json:"maintainable"`
	DefaultRuleRef      string   `json:"default_rule_ref"`
	DefaultValue        string   `json:"default_value"`
	AllowedValueCodes   []string `json:"allowed_value_codes"`
	Priority            int      `json:"priority"`
	PriorityMode        string   `json:"priority_mode"`
	LocalOverrideMode   string   `json:"local_override_mode"`
	ExplainRequired     bool     `json:"explain_required"`
	IsStable            bool     `json:"is_stable"`
	ChangePolicy        string   `json:"change_policy"`
	EffectiveDate       string   `json:"effective_date"`
	EndDate             string   `json:"end_date"`
	RequestID           string   `json:"request_id"`
}

type setIDStrategyRegistryDisableAPIRequest struct {
	CapabilityKey       string `json:"capability_key"`
	FieldKey            string `json:"field_key"`
	OrgApplicability    string `json:"org_applicability"`
	BusinessUnitOrgCode string `json:"business_unit_org_code"`
	EffectiveDate       string `json:"effective_date"`
	DisableAsOf         string `json:"disable_as_of"`
	RequestID           string `json:"request_id"`
}

type setIDStrategyRegistryAPIItem struct {
	CapabilityKey       string   `json:"capability_key"`
	OwnerModule         string   `json:"owner_module"`
	SourceType          string   `json:"source_type,omitempty"`
	FieldKey            string   `json:"field_key"`
	PersonalizationMode string   `json:"personalization_mode"`
	OrgApplicability    string   `json:"org_applicability"`
	BusinessUnitOrgCode string   `json:"business_unit_org_code,omitempty"`
	Required            bool     `json:"required"`
	Visible             bool     `json:"visible"`
	Maintainable        bool     `json:"maintainable"`
	DefaultRuleRef      string   `json:"default_rule_ref,omitempty"`
	DefaultValue        string   `json:"default_value,omitempty"`
	AllowedValueCodes   []string `json:"allowed_value_codes,omitempty"`
	Priority            int      `json:"priority"`
	PriorityMode        string   `json:"priority_mode"`
	LocalOverrideMode   string   `json:"local_override_mode"`
	ExplainRequired     bool     `json:"explain_required"`
	IsStable            bool     `json:"is_stable"`
	ChangePolicy        string   `json:"change_policy"`
	EffectiveDate       string   `json:"effective_date"`
	EndDate             string   `json:"end_date,omitempty"`
	UpdatedAt           string   `json:"updated_at"`
}

type setIDStrategyRegistryDisableRequest struct {
	CapabilityKey       string
	FieldKey            string
	OrgApplicability    string
	BusinessUnitNodeKey string
	ResolvedSetID       string
	EffectiveDate       string
	DisableAsOf         string
}

type setIDFieldDecision struct {
	CapabilityKey      string   `json:"capability_key"`
	SourceType         string   `json:"source_type,omitempty"`
	FieldKey           string   `json:"field_key"`
	Required           bool     `json:"required"`
	Visible            bool     `json:"visible"`
	Maintainable       bool     `json:"maintainable"`
	Visibility         string   `json:"visibility,omitempty"`
	MaskStrategy       string   `json:"mask_strategy,omitempty"`
	DefaultRuleRef     string   `json:"default_rule_ref,omitempty"`
	ResolvedDefaultVal string   `json:"resolved_default_value,omitempty"`
	MaskedDefaultVal   string   `json:"masked_default_value,omitempty"`
	AllowedValueCodes  []string `json:"allowed_value_codes,omitempty"`
	PriorityMode       string   `json:"priority_mode,omitempty"`
	LocalOverrideMode  string   `json:"local_override_mode,omitempty"`
	Decision           string   `json:"decision"`
	ReasonCode         string   `json:"reason_code,omitempty"`
}

type setIDStrategyRegistryRuntime struct {
	mu       sync.RWMutex
	byTenant map[string][]setIDStrategyRegistryItem
}

type setIDStrategyRegistryStore interface {
	upsert(ctx context.Context, tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error)
	disable(ctx context.Context, tenantID string, req setIDStrategyRegistryDisableRequest) (setIDStrategyRegistryItem, bool, error)
	list(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error)
	resolveFieldDecision(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, resolvedSetID string, businessUnitNodeKey string, asOf string) (setIDFieldDecision, error)
}

type setIDStrategyRegistryRuntimeStore struct {
	runtime *setIDStrategyRegistryRuntime
}

type setIDStrategyRegistryPGStore struct {
	pool pgBeginner
}

func newSetIDStrategyRegistryRuntime() *setIDStrategyRegistryRuntime {
	return &setIDStrategyRegistryRuntime{
		byTenant: make(map[string][]setIDStrategyRegistryItem),
	}
}

var defaultSetIDStrategyRegistryRuntime = newSetIDStrategyRegistryRuntime()
var defaultSetIDStrategyRegistryStore setIDStrategyRegistryStore = &setIDStrategyRegistryRuntimeStore{
	runtime: defaultSetIDStrategyRegistryRuntime,
}

func resetSetIDStrategyRegistryRuntimeForTest() {
	defaultSetIDStrategyRegistryRuntime = newSetIDStrategyRegistryRuntime()
	defaultSetIDStrategyRegistryStore = &setIDStrategyRegistryRuntimeStore{
		runtime: defaultSetIDStrategyRegistryRuntime,
	}
}

func useSetIDStrategyRegistryStore(store setIDStrategyRegistryStore) {
	if store == nil {
		defaultSetIDStrategyRegistryStore = &setIDStrategyRegistryRuntimeStore{
			runtime: defaultSetIDStrategyRegistryRuntime,
		}
		return
	}
	defaultSetIDStrategyRegistryStore = store
}

func newSetIDStrategyRegistryPGStore(pool pgBeginner) setIDStrategyRegistryStore {
	if pool == nil {
		return nil
	}
	return &setIDStrategyRegistryPGStore{pool: pool}
}

func (s *setIDStrategyRegistryRuntimeStore) upsert(_ context.Context, tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error) {
	saved, updated := s.runtime.upsert(tenantID, item)
	return saved, updated, nil
}

func (s *setIDStrategyRegistryRuntimeStore) list(_ context.Context, tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error) {
	return s.runtime.list(tenantID, capabilityKey, fieldKey, asOf)
}

func (s *setIDStrategyRegistryRuntimeStore) disable(_ context.Context, tenantID string, req setIDStrategyRegistryDisableRequest) (setIDStrategyRegistryItem, bool, error) {
	return s.runtime.disable(tenantID, req)
}

func (s *setIDStrategyRegistryRuntimeStore) resolveFieldDecision(_ context.Context, tenantID string, capabilityKey string, fieldKey string, resolvedSetID string, businessUnitNodeKey string, asOf string) (setIDFieldDecision, error) {
	return s.runtime.resolveFieldDecision(tenantID, capabilityKey, fieldKey, resolvedSetID, businessUnitNodeKey, asOf)
}

func (s *setIDStrategyRegistryPGStore) withTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func parseAsOfDate(asOf string) (string, error) {
	asOf = strings.TrimSpace(asOf)
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		return "", errors.New("invalid as_of")
	}
	return asOf, nil
}

var marshalStrategyRegistryAllowedValueCodes = json.Marshal

func scanSetIDStrategyRegistryRows(rows pgx.Rows) ([]setIDStrategyRegistryItem, error) {
	out := make([]setIDStrategyRegistryItem, 0, 8)
	for rows.Next() {
		var item setIDStrategyRegistryItem
		var allowedValueCodesRaw string
		if err := rows.Scan(
			&item.CapabilityKey,
			&item.OwnerModule,
			&item.FieldKey,
			&item.PersonalizationMode,
			&item.OrgApplicability,
			&item.BusinessUnitNodeKey,
			&item.ResolvedSetID,
			&item.Required,
			&item.Visible,
			&item.Maintainable,
			&item.DefaultRuleRef,
			&item.DefaultValue,
			&allowedValueCodesRaw,
			&item.Priority,
			&item.ExplainRequired,
			&item.IsStable,
			&item.ChangePolicy,
			&item.EffectiveDate,
			&item.EndDate,
			&item.UpdatedAt,
			&item.PriorityMode,
			&item.LocalOverrideMode,
		); err != nil {
			return nil, err
		}
		item.ResolvedSetID = normalizeStrategyRegistryResolvedSetID(item.ResolvedSetID)
		if strings.TrimSpace(allowedValueCodesRaw) != "" {
			if err := json.Unmarshal([]byte(allowedValueCodesRaw), &item.AllowedValueCodes); err != nil {
				return nil, err
			}
		}
		item.PriorityMode, item.LocalOverrideMode = normalizeStrategyModes(item.PriorityMode, item.LocalOverrideMode)
		item.AllowedValueCodes = normalizeAllowedValueCodes(item.AllowedValueCodes)
		item.SourceType = strategySourceTypeForCapabilityKey(item.CapabilityKey)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *setIDStrategyRegistryPGStore) upsert(ctx context.Context, tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error) {
	var updated bool
	endDate := any(nil)
	if strings.TrimSpace(item.EndDate) != "" {
		endDate = strings.TrimSpace(item.EndDate)
	}
	allowedValueCodesJSON := any(nil)
	if len(item.AllowedValueCodes) > 0 {
		raw, err := marshalStrategyRegistryAllowedValueCodes(item.AllowedValueCodes)
		if err != nil {
			return setIDStrategyRegistryItem{}, false, err
		}
		allowedValueCodesJSON = string(raw)
	}
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM orgunit.setid_strategy_registry
  WHERE tenant_uuid = $1::uuid
    AND capability_key = $2::text
    AND field_key = $3::text
    AND org_applicability = $4::text
    AND business_unit_node_key = $5::text
    AND resolved_setid = $6::text
    AND effective_date = $7::date
)
`, tenantID, item.CapabilityKey, item.FieldKey, item.OrgApplicability, item.BusinessUnitNodeKey, item.ResolvedSetID, item.EffectiveDate).Scan(&updated); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
INSERT INTO orgunit.setid_strategy_registry (
  tenant_uuid,
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  business_unit_node_key,
  resolved_setid,
  required,
  visible,
  maintainable,
  default_rule_ref,
  default_value,
  allowed_value_codes,
  priority,
  priority_mode,
  local_override_mode,
  explain_required,
  is_stable,
  change_policy,
  effective_date,
  end_date,
  updated_at
) VALUES (
  $1::uuid,
  $2::text,
  $3::text,
  $4::text,
  $5::text,
  $6::text,
  $7::text,
  $8::text,
  $9::boolean,
  $10::boolean,
  $11::boolean,
  NULLIF($12::text, ''),
  NULLIF($13::text, ''),
  $14::jsonb,
  $15::integer,
  $16::text,
  $17::text,
  $18::boolean,
  $19::boolean,
  $20::text,
  $21::date,
  $22::date,
  $23::timestamptz
)
ON CONFLICT (tenant_uuid, capability_key, field_key, org_applicability, resolved_setid, business_unit_node_key, effective_date)
DO UPDATE SET
  owner_module = EXCLUDED.owner_module,
  personalization_mode = EXCLUDED.personalization_mode,
  required = EXCLUDED.required,
  visible = EXCLUDED.visible,
  maintainable = EXCLUDED.maintainable,
  default_rule_ref = EXCLUDED.default_rule_ref,
  default_value = EXCLUDED.default_value,
  allowed_value_codes = EXCLUDED.allowed_value_codes,
  priority = EXCLUDED.priority,
  priority_mode = EXCLUDED.priority_mode,
  local_override_mode = EXCLUDED.local_override_mode,
  explain_required = EXCLUDED.explain_required,
  is_stable = EXCLUDED.is_stable,
  change_policy = EXCLUDED.change_policy,
  end_date = EXCLUDED.end_date,
  updated_at = EXCLUDED.updated_at
`, tenantID, item.CapabilityKey, item.OwnerModule, item.FieldKey, item.PersonalizationMode, item.OrgApplicability, item.BusinessUnitNodeKey, item.ResolvedSetID, item.Required, item.Visible, item.Maintainable, item.DefaultRuleRef, item.DefaultValue, allowedValueCodesJSON, item.Priority, item.PriorityMode, item.LocalOverrideMode, item.ExplainRequired, item.IsStable, item.ChangePolicy, item.EffectiveDate, endDate, item.UpdatedAt)
		return err
	})
	return item, updated, err
}

func ensureStrategyResolvableAfterDisable(items []setIDStrategyRegistryItem, req setIDStrategyRegistryDisableRequest) error {
	asOf, err := time.Parse("2006-01-02", req.DisableAsOf)
	if err != nil {
		return err
	}
	capabilityCandidates := map[string]struct{}{
		req.CapabilityKey: {},
	}
	if baselineCapabilityKey, ok := orgUnitBaselineCapabilityKeyForIntentCapability(req.CapabilityKey); ok {
		capabilityCandidates[baselineCapabilityKey] = struct{}{}
	}
	active := make([]setIDStrategyRegistryItem, 0, len(items))
	for _, item := range items {
		if _, ok := capabilityCandidates[item.CapabilityKey]; !ok || item.FieldKey != req.FieldKey {
			continue
		}
		effectiveDate, effectiveErr := time.Parse("2006-01-02", item.EffectiveDate)
		if effectiveErr != nil || effectiveDate.After(asOf) {
			continue
		}
		if item.EndDate != "" {
			endDate, endErr := time.Parse("2006-01-02", item.EndDate)
			if endErr == nil && !endDate.After(asOf) {
				continue
			}
		}
		active = append(active, item)
	}
	businessUnitNodeKey := ""
	if req.OrgApplicability == orgApplicabilityBusinessUnit {
		businessUnitNodeKey = req.BusinessUnitNodeKey
	}
	if _, err := resolveFieldDecisionFromItems(active, req.CapabilityKey, req.FieldKey, req.ResolvedSetID, businessUnitNodeKey); err != nil {
		return errDisableNotAllowed
	}
	return nil
}

func (s *setIDStrategyRegistryPGStore) disable(ctx context.Context, tenantID string, req setIDStrategyRegistryDisableRequest) (setIDStrategyRegistryItem, bool, error) {
	var target setIDStrategyRegistryItem
	updated := false
	nowUTC := time.Now().UTC()
	endDate := strings.TrimSpace(req.DisableAsOf)
	capabilityCandidates := []string{req.CapabilityKey}
	if baselineCapabilityKey, ok := orgUnitBaselineCapabilityKeyForIntentCapability(req.CapabilityKey); ok && baselineCapabilityKey != req.CapabilityKey {
		capabilityCandidates = append(capabilityCandidates, baselineCapabilityKey)
	}
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
SELECT
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  business_unit_node_key,
  resolved_setid,
  required,
  visible,
  maintainable,
  COALESCE(default_rule_ref, ''),
  COALESCE(default_value, ''),
  COALESCE(allowed_value_codes, '[]'::jsonb)::text,
  priority,
  explain_required,
  is_stable,
  change_policy,
  effective_date::text,
  COALESCE(end_date::text, ''),
  to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
  priority_mode,
  local_override_mode
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND capability_key = $2::text
  AND field_key = $3::text
  AND org_applicability = $4::text
  AND business_unit_node_key = $5::text
  AND resolved_setid = $6::text
  AND effective_date = $7::date
FOR UPDATE
`, tenantID, req.CapabilityKey, req.FieldKey, req.OrgApplicability, req.BusinessUnitNodeKey, req.ResolvedSetID, req.EffectiveDate)
		var allowedValueCodesRaw string
		if err := row.Scan(
			&target.CapabilityKey,
			&target.OwnerModule,
			&target.FieldKey,
			&target.PersonalizationMode,
			&target.OrgApplicability,
			&target.BusinessUnitNodeKey,
			&target.ResolvedSetID,
			&target.Required,
			&target.Visible,
			&target.Maintainable,
			&target.DefaultRuleRef,
			&target.DefaultValue,
			&allowedValueCodesRaw,
			&target.Priority,
			&target.ExplainRequired,
			&target.IsStable,
			&target.ChangePolicy,
			&target.EffectiveDate,
			&target.EndDate,
			&target.UpdatedAt,
			&target.PriorityMode,
			&target.LocalOverrideMode,
		); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errStrategyNotFound
			}
			return err
		}
		if strings.TrimSpace(allowedValueCodesRaw) != "" {
			if err := json.Unmarshal([]byte(allowedValueCodesRaw), &target.AllowedValueCodes); err != nil {
				return err
			}
		}
		target.PriorityMode, target.LocalOverrideMode = normalizeStrategyModes(target.PriorityMode, target.LocalOverrideMode)
		target.AllowedValueCodes = normalizeAllowedValueCodes(target.AllowedValueCodes)
		target.ResolvedSetID = normalizeStrategyRegistryResolvedSetID(target.ResolvedSetID)
		if target.EndDate == endDate {
			return nil
		}
		if target.EndDate != "" && target.EndDate < endDate {
			return errors.New("invalid_disable_date")
		}
		if _, err := tx.Exec(ctx, `
UPDATE orgunit.setid_strategy_registry
SET end_date = $8::date,
    updated_at = $9::timestamptz
WHERE tenant_uuid = $1::uuid
  AND capability_key = $2::text
  AND field_key = $3::text
  AND org_applicability = $4::text
  AND business_unit_node_key = $5::text
  AND resolved_setid = $6::text
  AND effective_date = $7::date
`, tenantID, req.CapabilityKey, req.FieldKey, req.OrgApplicability, req.BusinessUnitNodeKey, req.ResolvedSetID, req.EffectiveDate, endDate, nowUTC); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
SELECT
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  business_unit_node_key,
  resolved_setid,
  required,
  visible,
  maintainable,
  COALESCE(default_rule_ref, ''),
  COALESCE(default_value, ''),
  COALESCE(allowed_value_codes, '[]'::jsonb)::text,
  priority,
  explain_required,
  is_stable,
  change_policy,
  effective_date::text,
  COALESCE(end_date::text, ''),
  to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
  priority_mode,
  local_override_mode
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND capability_key = ANY($2::text[])
  AND field_key = $3::text
  AND effective_date <= $4::date
  AND (end_date IS NULL OR end_date > $4::date)
ORDER BY capability_key ASC, field_key ASC, org_applicability ASC, resolved_setid ASC, business_unit_node_key ASC, effective_date ASC
`, tenantID, capabilityCandidates, req.FieldKey, endDate)
		if err != nil {
			return err
		}
		defer rows.Close()
		items, err := scanSetIDStrategyRegistryRows(rows)
		if err != nil {
			return err
		}
		if err := ensureStrategyResolvableAfterDisable(items, req); err != nil {
			return err
		}
		target.EndDate = endDate
		target.UpdatedAt = nowUTC.Format("2006-01-02T15:04:05Z")
		updated = true
		return nil
	})
	return target, updated, err
}

func (s *setIDStrategyRegistryPGStore) list(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, asOf string) ([]setIDStrategyRegistryItem, error) {
	normalizedAsOf, err := parseAsOfDate(asOf)
	if err != nil {
		return nil, err
	}
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	fieldKey = strings.ToLower(strings.TrimSpace(fieldKey))
	out := make([]setIDStrategyRegistryItem, 0, 8)
	err = s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  business_unit_node_key,
  resolved_setid,
  required,
  visible,
  maintainable,
  COALESCE(default_rule_ref, ''),
  COALESCE(default_value, ''),
  COALESCE(allowed_value_codes, '[]'::jsonb)::text,
  priority,
  explain_required,
  is_stable,
  change_policy,
  effective_date::text,
  COALESCE(end_date::text, ''),
  to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
  priority_mode,
  local_override_mode
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND ($2::text = '' OR capability_key = $2::text)
  AND ($3::text = '' OR field_key = $3::text)
  AND effective_date <= $4::date
  AND (end_date IS NULL OR end_date > $4::date)
ORDER BY capability_key ASC, field_key ASC, org_applicability ASC, resolved_setid ASC, business_unit_node_key ASC, effective_date ASC
	`, tenantID, capabilityKey, fieldKey, normalizedAsOf)
		if err != nil {
			return err
		}
		defer rows.Close()
		items, err := scanSetIDStrategyRegistryRows(rows)
		if err != nil {
			return err
		}
		out = append(out, items...)
		return nil
	})
	return out, err
}

func (s *setIDStrategyRegistryPGStore) resolveFieldDecision(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, resolvedSetID string, businessUnitNodeKey string, asOf string) (setIDFieldDecision, error) {
	items, err := collectCapabilityResolutionItems(
		func(queryCapabilityKey string) ([]setIDStrategyRegistryItem, error) {
			return s.list(ctx, tenantID, queryCapabilityKey, fieldKey, asOf)
		},
		capabilityKey,
	)
	if err != nil {
		return setIDFieldDecision{}, err
	}
	return resolveFieldDecisionFromItems(items, capabilityKey, fieldKey, resolvedSetID, businessUnitNodeKey)
}

func collectCapabilityResolutionItems(
	listByCapability func(queryCapabilityKey string) ([]setIDStrategyRegistryItem, error),
	capabilityKey string,
) ([]setIDStrategyRegistryItem, error) {
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	items, err := listByCapability(capabilityKey)
	if err != nil {
		return nil, err
	}
	merged := append([]setIDStrategyRegistryItem(nil), items...)
	baselineCapabilityKey, hasBaseline := orgUnitBaselineCapabilityKeyForIntentCapability(capabilityKey)
	if !hasBaseline || baselineCapabilityKey == capabilityKey {
		return merged, nil
	}
	baselineItems, baselineErr := listByCapability(baselineCapabilityKey)
	if baselineErr != nil {
		return nil, baselineErr
	}
	merged = append(merged, baselineItems...)
	return merged, nil
}

func strategyRegistrySortKey(item setIDStrategyRegistryItem) string {
	return strings.Join([]string{
		item.CapabilityKey,
		item.FieldKey,
		item.OrgApplicability,
		item.ResolvedSetID,
		item.BusinessUnitNodeKey,
		item.EffectiveDate,
	}, "|")
}

func normalizeStrategyRegistryResolvedSetID(resolvedSetID string) string {
	return strings.ToUpper(strings.TrimSpace(resolvedSetID))
}

func normalizeStrategyRegistryItem(req setIDStrategyRegistryUpsertAPIRequest, businessUnitNodeKey string, resolvedSetIDs ...string) setIDStrategyRegistryItem {
	resolvedSetID := ""
	if len(resolvedSetIDs) > 0 {
		resolvedSetID = resolvedSetIDs[0]
	}
	item := setIDStrategyRegistryItem{
		CapabilityKey:       strings.ToLower(strings.TrimSpace(req.CapabilityKey)),
		OwnerModule:         strings.ToLower(strings.TrimSpace(req.OwnerModule)),
		SourceType:          "",
		FieldKey:            strings.ToLower(strings.TrimSpace(req.FieldKey)),
		PersonalizationMode: strings.ToLower(strings.TrimSpace(req.PersonalizationMode)),
		OrgApplicability:    strings.ToLower(strings.TrimSpace(req.OrgApplicability)),
		BusinessUnitNodeKey: strings.TrimSpace(businessUnitNodeKey),
		ResolvedSetID:       normalizeStrategyRegistryResolvedSetID(resolvedSetID),
		Required:            req.Required,
		Visible:             req.Visible,
		Maintainable:        true,
		DefaultRuleRef:      strings.TrimSpace(req.DefaultRuleRef),
		DefaultValue:        strings.TrimSpace(req.DefaultValue),
		AllowedValueCodes:   normalizeAllowedValueCodes(req.AllowedValueCodes),
		Priority:            req.Priority,
		PriorityMode:        strings.ToLower(strings.TrimSpace(req.PriorityMode)),
		LocalOverrideMode:   strings.ToLower(strings.TrimSpace(req.LocalOverrideMode)),
		ExplainRequired:     req.ExplainRequired,
		IsStable:            req.IsStable,
		ChangePolicy:        strings.ToLower(strings.TrimSpace(req.ChangePolicy)),
		EffectiveDate:       strings.TrimSpace(req.EffectiveDate),
		EndDate:             strings.TrimSpace(req.EndDate),
	}
	if item.Priority <= 0 {
		item.Priority = 100
	}
	item.PriorityMode, item.LocalOverrideMode = normalizeStrategyModes(item.PriorityMode, item.LocalOverrideMode)
	if req.Maintainable != nil {
		item.Maintainable = *req.Maintainable
	}
	if item.ChangePolicy == "" {
		item.ChangePolicy = "plan_required"
	}
	if item.OrgApplicability == orgApplicabilityTenant {
		item.BusinessUnitNodeKey = ""
		item.ResolvedSetID = ""
	}
	item.SourceType = strategySourceTypeForCapabilityKey(item.CapabilityKey)
	item.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return item
}

func normalizeStrategyRegistryDisableRequest(req setIDStrategyRegistryDisableAPIRequest, businessUnitNodeKey string, resolvedSetIDs ...string) setIDStrategyRegistryDisableRequest {
	resolvedSetID := ""
	if len(resolvedSetIDs) > 0 {
		resolvedSetID = resolvedSetIDs[0]
	}
	item := setIDStrategyRegistryDisableRequest{
		CapabilityKey:       strings.ToLower(strings.TrimSpace(req.CapabilityKey)),
		FieldKey:            strings.ToLower(strings.TrimSpace(req.FieldKey)),
		OrgApplicability:    strings.ToLower(strings.TrimSpace(req.OrgApplicability)),
		BusinessUnitNodeKey: strings.TrimSpace(businessUnitNodeKey),
		ResolvedSetID:       normalizeStrategyRegistryResolvedSetID(resolvedSetID),
		EffectiveDate:       strings.TrimSpace(req.EffectiveDate),
		DisableAsOf:         strings.TrimSpace(req.DisableAsOf),
	}
	if item.OrgApplicability == orgApplicabilityTenant {
		item.BusinessUnitNodeKey = ""
		item.ResolvedSetID = ""
	}
	return item
}

func writeSetIDStrategyRegistryBusinessUnitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		return
	case err.Error() == "business_unit_org_code_required":
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "business_unit_org_code required")
	case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "business_unit_org_code_invalid", "business_unit_org_code invalid")
	case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "business_unit_org_code_not_found", "business_unit_org_code not found")
	default:
		writeInternalAPIError(w, r, err, "setid_strategy_registry_business_unit_org_code_resolve_failed")
	}
}

func writeSetIDStrategyRegistryContextError(w http.ResponseWriter, r *http.Request, err error) {
	if resolveErr, ok := asSetIDContextResolveError(err); ok {
		switch resolveErr.Code {
		case setIDContextCodeBusinessUnitInvalid:
			writeSetIDStrategyRegistryBusinessUnitError(w, r, resolveErr.Cause)
		case setIDContextCodeOrgResolverMissing:
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "orgunit_resolver_missing", "orgunit resolver missing")
		case setIDContextCodeSetIDResolverMissing:
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_resolver_missing", "setid resolver missing")
		case setIDContextCodeSetIDBindingMissing:
			code := resolveErr.Code
			if resolveErr.Cause != nil {
				code = stablePgMessage(resolveErr.Cause)
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, code, "resolve setid failed")
		case setIDContextCodeSetIDSourceInvalid:
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, resolveErr.Code, "setid source invalid")
		default:
			writeInternalAPIError(w, r, err, "setid_strategy_registry_context_resolve_failed")
		}
		return
	}
	writeInternalAPIError(w, r, err, "setid_strategy_registry_context_resolve_failed")
}

func resolveStrategyRegistryPolicyContext(
	ctx context.Context,
	tenantID string,
	req setIDStrategyRegistryUpsertAPIRequest,
	orgResolver OrgUnitCodeResolver,
	setIDStore SetIDGovernanceStore,
) (setIDPolicyContext, error) {
	contextResolver := newSetIDContextResolver(orgResolver, setIDStore)
	return contextResolver.ResolvePolicyContext(ctx, setIDPolicyContextInput{
		TenantID:            tenantID,
		CapabilityKey:       req.CapabilityKey,
		FieldKey:            req.FieldKey,
		AsOf:                req.EffectiveDate,
		BusinessUnitOrgCode: req.BusinessUnitOrgCode,
	})
}

func resolveStrategyRegistryDisablePolicyContext(
	ctx context.Context,
	tenantID string,
	req setIDStrategyRegistryDisableAPIRequest,
	orgResolver OrgUnitCodeResolver,
	setIDStore SetIDGovernanceStore,
) (setIDPolicyContext, error) {
	contextResolver := newSetIDContextResolver(orgResolver, setIDStore)
	return contextResolver.ResolvePolicyContext(ctx, setIDPolicyContextInput{
		TenantID:            tenantID,
		CapabilityKey:       req.CapabilityKey,
		FieldKey:            req.FieldKey,
		AsOf:                req.EffectiveDate,
		BusinessUnitOrgCode: req.BusinessUnitOrgCode,
	})
}

func strategyRegistryAPIItemFromInternal(ctx context.Context, tenantID string, item setIDStrategyRegistryItem, orgResolver OrgUnitCodeResolver) (setIDStrategyRegistryAPIItem, error) {
	apiItem := setIDStrategyRegistryAPIItem{
		CapabilityKey:       item.CapabilityKey,
		OwnerModule:         item.OwnerModule,
		SourceType:          item.SourceType,
		FieldKey:            item.FieldKey,
		PersonalizationMode: item.PersonalizationMode,
		OrgApplicability:    item.OrgApplicability,
		Required:            item.Required,
		Visible:             item.Visible,
		Maintainable:        item.Maintainable,
		DefaultRuleRef:      item.DefaultRuleRef,
		DefaultValue:        item.DefaultValue,
		AllowedValueCodes:   item.AllowedValueCodes,
		Priority:            item.Priority,
		PriorityMode:        item.PriorityMode,
		LocalOverrideMode:   item.LocalOverrideMode,
		ExplainRequired:     item.ExplainRequired,
		IsStable:            item.IsStable,
		ChangePolicy:        item.ChangePolicy,
		EffectiveDate:       item.EffectiveDate,
		EndDate:             item.EndDate,
		UpdatedAt:           item.UpdatedAt,
	}
	if strings.TrimSpace(item.BusinessUnitNodeKey) == "" {
		return apiItem, nil
	}
	if orgResolver == nil {
		return setIDStrategyRegistryAPIItem{}, errors.New("orgunit resolver missing")
	}
	orgNodeKey, err := normalizeOrgNodeKeyInput(item.BusinessUnitNodeKey)
	if err != nil {
		return setIDStrategyRegistryAPIItem{}, err
	}
	businessUnitOrgCode, err := orgResolver.ResolveOrgCodeByNodeKey(ctx, tenantID, orgNodeKey)
	if err != nil {
		return setIDStrategyRegistryAPIItem{}, err
	}
	apiItem.BusinessUnitOrgCode = businessUnitOrgCode
	return apiItem, nil
}

func normalizeAllowedValueCodes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStrategyModes(priorityMode string, localOverrideMode string) (string, string) {
	priorityMode = strings.ToLower(strings.TrimSpace(priorityMode))
	localOverrideMode = strings.ToLower(strings.TrimSpace(localOverrideMode))
	if priorityMode == "" {
		priorityMode = priorityModeBlendCustomFirst
	}
	if localOverrideMode == "" {
		localOverrideMode = localOverrideModeAllow
	}
	return priorityMode, localOverrideMode
}

func strategySourceTypeForCapabilityKey(capabilityKey string) string {
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	switch capabilityKey {
	case orgUnitWriteFieldPolicyCapabilityKey:
		return strategySourceBaseline
	case orgUnitCreateFieldPolicyCapabilityKey,
		orgUnitAddVersionFieldPolicyCapabilityKey,
		orgUnitInsertVersionFieldPolicyCapabilityKey,
		orgUnitCorrectFieldPolicyCapabilityKey:
		return strategySourceIntentOverride
	default:
		return ""
	}
}

func validateStrategyRegistryItem(item setIDStrategyRegistryItem) (int, string, string) {
	item.PriorityMode, item.LocalOverrideMode = normalizeStrategyModes(item.PriorityMode, item.LocalOverrideMode)
	if item.CapabilityKey == "" || item.OwnerModule == "" || item.FieldKey == "" || item.PersonalizationMode == "" || item.OrgApplicability == "" || item.EffectiveDate == "" {
		return http.StatusBadRequest, "invalid_request", "capability_key/owner_module/field_key/personalization_mode/org_applicability/effective_date required"
	}
	if !capabilityKeyPattern.MatchString(item.CapabilityKey) {
		return http.StatusBadRequest, "invalid_capability_key", "invalid capability_key"
	}
	if containsCapabilityContextToken(item.CapabilityKey) {
		return http.StatusUnprocessableEntity, "invalid_capability_key_context", "capability_key must not include context tokens"
	}
	definition, ok := capabilityDefinitionForKey(item.CapabilityKey)
	if !ok {
		return http.StatusUnprocessableEntity, "invalid_request", "capability_key must be registered"
	}
	if definition.OwnerModule != "" && definition.OwnerModule != item.OwnerModule {
		return http.StatusUnprocessableEntity, "invalid_request", "owner_module must match capability registry"
	}
	catalogEntry, ok := capabilityCatalogEntryForCapabilityKey(item.CapabilityKey)
	if !ok {
		return http.StatusUnprocessableEntity, "invalid_request", "capability_key must be discoverable in capability catalog"
	}
	if catalogEntry.OwnerModule != "" && catalogEntry.OwnerModule != item.OwnerModule {
		return http.StatusUnprocessableEntity, "invalid_request", "owner_module must match capability catalog"
	}
	if !fieldKeyPattern.MatchString(item.FieldKey) {
		return http.StatusBadRequest, "invalid_field_key", "invalid field_key"
	}
	switch item.PersonalizationMode {
	case personalizationModeTenantOnly, personalizationModeSetID:
	default:
		return http.StatusUnprocessableEntity, "personalization_mode_invalid", "personalization_mode invalid"
	}
	switch item.PriorityMode {
	case priorityModeBlendCustomFirst, priorityModeBlendDefltFirst, priorityModeDefltUnsubscribed:
	default:
		return http.StatusUnprocessableEntity, fieldPolicyPriorityModeCode, fieldPolicyPriorityModeCode
	}
	switch item.LocalOverrideMode {
	case localOverrideModeAllow, localOverrideModeNoOverride, localOverrideModeNoLocal:
	default:
		return http.StatusUnprocessableEntity, fieldPolicyLocalModeCode, fieldPolicyLocalModeCode
	}
	if item.PriorityMode == priorityModeDefltUnsubscribed && item.LocalOverrideMode == localOverrideModeNoLocal {
		return http.StatusUnprocessableEntity, fieldPolicyModeComboCode, fieldPolicyModeComboCode
	}
	switch item.OrgApplicability {
	case orgApplicabilityTenant:
	case orgApplicabilityBusinessUnit:
		if item.BusinessUnitNodeKey == "" {
			return http.StatusBadRequest, "business_unit_org_code_invalid", "business_unit_org_code required"
		}
		if _, err := normalizeOrgNodeKeyInput(item.BusinessUnitNodeKey); err != nil {
			return http.StatusBadRequest, "business_unit_org_code_invalid", "business_unit_org_code invalid"
		}
		if item.ResolvedSetID == "" {
			return http.StatusUnprocessableEntity, setIDContextCodeSetIDBindingMissing, setIDContextCodeSetIDBindingMissing
		}
	default:
		return http.StatusUnprocessableEntity, "org_applicability_invalid", "org_applicability invalid"
	}
	if item.ResolvedSetID != "" && !resolvedSetIDPattern.MatchString(item.ResolvedSetID) {
		return http.StatusUnprocessableEntity, setIDContextCodeSetIDSourceInvalid, setIDContextCodeSetIDSourceInvalid
	}
	if item.PersonalizationMode != personalizationModeTenantOnly && !item.ExplainRequired {
		return http.StatusUnprocessableEntity, explainRequiredCode, "explain_required must be true when personalization_mode is not tenant_only"
	}
	if !item.Visible && item.Required {
		return http.StatusUnprocessableEntity, fieldPolicyConflictCode, fieldPolicyConflictCode
	}
	if !item.Maintainable && strings.TrimSpace(item.DefaultRuleRef) == "" && strings.TrimSpace(item.DefaultValue) == "" {
		return http.StatusUnprocessableEntity, fieldDefaultRuleMissingCode, fieldDefaultRuleMissingCode
	}
	if len(item.AllowedValueCodes) > 0 && strings.TrimSpace(item.DefaultValue) != "" && !slices.Contains(item.AllowedValueCodes, strings.TrimSpace(item.DefaultValue)) {
		return http.StatusUnprocessableEntity, "default_value_not_allowed", "default_value must be included in allowed_value_codes"
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

func validateStrategyRegistryDisableRequest(req setIDStrategyRegistryDisableRequest) (int, string, string) {
	if req.CapabilityKey == "" || req.FieldKey == "" || req.OrgApplicability == "" || req.EffectiveDate == "" || req.DisableAsOf == "" {
		return http.StatusBadRequest, "invalid_request", "capability_key/field_key/org_applicability/effective_date/disable_as_of required"
	}
	if !capabilityKeyPattern.MatchString(req.CapabilityKey) {
		return http.StatusBadRequest, "invalid_capability_key", "invalid capability_key"
	}
	if containsCapabilityContextToken(req.CapabilityKey) {
		return http.StatusUnprocessableEntity, "invalid_capability_key_context", "capability_key must not include context tokens"
	}
	if _, ok := capabilityDefinitionForKey(req.CapabilityKey); !ok {
		return http.StatusUnprocessableEntity, "invalid_request", "capability_key must be registered"
	}
	if _, ok := capabilityCatalogEntryForCapabilityKey(req.CapabilityKey); !ok {
		return http.StatusUnprocessableEntity, "invalid_request", "capability_key must be discoverable in capability catalog"
	}
	if !fieldKeyPattern.MatchString(req.FieldKey) {
		return http.StatusBadRequest, "invalid_field_key", "invalid field_key"
	}
	switch req.OrgApplicability {
	case orgApplicabilityTenant:
	case orgApplicabilityBusinessUnit:
		if req.BusinessUnitNodeKey == "" {
			return http.StatusBadRequest, "business_unit_org_code_invalid", "business_unit_org_code required"
		}
		if _, err := normalizeOrgNodeKeyInput(req.BusinessUnitNodeKey); err != nil {
			return http.StatusBadRequest, "business_unit_org_code_invalid", "business_unit_org_code invalid"
		}
		if req.ResolvedSetID == "" {
			return http.StatusUnprocessableEntity, setIDContextCodeSetIDBindingMissing, setIDContextCodeSetIDBindingMissing
		}
	default:
		return http.StatusUnprocessableEntity, "org_applicability_invalid", "org_applicability invalid"
	}
	if req.ResolvedSetID != "" && !resolvedSetIDPattern.MatchString(req.ResolvedSetID) {
		return http.StatusUnprocessableEntity, setIDContextCodeSetIDSourceInvalid, setIDContextCodeSetIDSourceInvalid
	}
	effectiveDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		return http.StatusBadRequest, "invalid_effective_date", "invalid effective_date"
	}
	disableAsOf, disableErr := time.Parse("2006-01-02", req.DisableAsOf)
	if disableErr != nil {
		return http.StatusBadRequest, "invalid_disable_date", "invalid disable_as_of"
	}
	if !disableAsOf.After(effectiveDate) {
		return http.StatusUnprocessableEntity, fieldPolicyConflictCode, "disable_as_of must be greater than effective_date"
	}
	return 0, "", ""
}

func containsCapabilityContextToken(capabilityKey string) bool {
	segments := strings.Split(strings.ToLower(strings.TrimSpace(capabilityKey)), ".")
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		switch segment {
		case "setid", "scope", "tenant", "bu":
			return true
		}
		if strings.HasPrefix(segment, "bu_") || strings.HasPrefix(segment, "setid_") || strings.HasPrefix(segment, "tenant_") || strings.HasPrefix(segment, "scope_") {
			return true
		}
	}
	return false
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

func (s *setIDStrategyRegistryRuntime) disable(tenantID string, req setIDStrategyRegistryDisableRequest) (setIDStrategyRegistryItem, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.byTenant[tenantID]
	targetKey := strings.Join([]string{
		req.CapabilityKey,
		req.FieldKey,
		req.OrgApplicability,
		req.ResolvedSetID,
		req.BusinessUnitNodeKey,
		req.EffectiveDate,
	}, "|")
	targetIndex := -1
	for i := range items {
		candidateKey := strings.Join([]string{
			items[i].CapabilityKey,
			items[i].FieldKey,
			items[i].OrgApplicability,
			items[i].ResolvedSetID,
			items[i].BusinessUnitNodeKey,
			items[i].EffectiveDate,
		}, "|")
		if candidateKey == targetKey {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return setIDStrategyRegistryItem{}, false, errStrategyNotFound
	}
	target := items[targetIndex]
	if target.EndDate == req.DisableAsOf {
		return target, false, nil
	}
	if target.EndDate != "" && target.EndDate < req.DisableAsOf {
		return setIDStrategyRegistryItem{}, false, errors.New("invalid_disable_date")
	}
	target.EndDate = req.DisableAsOf
	target.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	candidateItems := slices.Clone(items)
	candidateItems[targetIndex] = target
	if err := ensureStrategyResolvableAfterDisable(candidateItems, req); err != nil {
		return setIDStrategyRegistryItem{}, false, err
	}
	items[targetIndex] = target
	s.byTenant[tenantID] = items
	return target, true, nil
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
		if strings.TrimSpace(item.SourceType) == "" {
			item.SourceType = strategySourceTypeForCapabilityKey(item.CapabilityKey)
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *setIDStrategyRegistryRuntime) resolveFieldDecision(
	tenantID string,
	capabilityKey string,
	fieldKey string,
	resolvedSetID string,
	businessUnitNodeKey string,
	asOf string,
) (setIDFieldDecision, error) {
	items, err := collectCapabilityResolutionItems(
		func(queryCapabilityKey string) ([]setIDStrategyRegistryItem, error) {
			return s.list(tenantID, queryCapabilityKey, fieldKey, asOf)
		},
		capabilityKey,
	)
	if err != nil {
		return setIDFieldDecision{}, err
	}
	return resolveFieldDecisionFromItems(items, capabilityKey, fieldKey, resolvedSetID, businessUnitNodeKey)
}

type setIDFieldDecisionResolution struct {
	Decision         setIDFieldDecision
	MatchedBucket    string
	PrimaryPolicyID  string
	WinnerPolicyIDs  []string
	MatchedPolicyIDs []string
	ResolutionTrace  []string
	PrimaryItem      *setIDStrategyRegistryItem
	WinnerItems      []setIDStrategyRegistryItem
	MatchedItems     []setIDStrategyRegistryItem
}

func resolveFieldDecisionFromItems(items []setIDStrategyRegistryItem, capabilityKey string, fieldKey string, resolvedSetID string, businessUnitNodeKey string) (setIDFieldDecision, error) {
	resolution, err := resolveFieldDecisionWithTraceFromItems(items, capabilityKey, fieldKey, resolvedSetID, businessUnitNodeKey)
	if err != nil {
		return setIDFieldDecision{}, err
	}
	return resolution.Decision, nil
}

func resolveFieldDecisionWithTraceFromItems(items []setIDStrategyRegistryItem, capabilityKey string, fieldKey string, resolvedSetID string, businessUnitNodeKey string) (setIDFieldDecisionResolution, error) {
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	fieldKey = strings.ToLower(strings.TrimSpace(fieldKey))
	resolvedSetID = normalizeStrategyRegistryResolvedSetID(resolvedSetID)
	businessUnitNodeKey = strings.TrimSpace(businessUnitNodeKey)

	baselineCapabilityKey, hasBaseline := orgUnitBaselineCapabilityKeyForIntentCapability(capabilityKey)
	if !hasBaseline {
		baselineCapabilityKey = ""
	}

	records := make([]fieldpolicy.Record, 0, len(items))
	itemsByPolicyID := make(map[string]setIDStrategyRegistryItem, len(items))
	for _, item := range items {
		policyID := strategyRegistryPolicyID(item)
		records = append(records, fieldpolicy.Record{
			PolicyID:            policyID,
			CapabilityKey:       item.CapabilityKey,
			FieldKey:            item.FieldKey,
			OrgApplicability:    item.OrgApplicability,
			ResolvedSetID:       item.ResolvedSetID,
			BusinessUnitNodeKey: item.BusinessUnitNodeKey,
			Required:            item.Required,
			Visible:             item.Visible,
			Maintainable:        item.Maintainable,
			DefaultRuleRef:      item.DefaultRuleRef,
			DefaultValue:        item.DefaultValue,
			AllowedValueCodes:   append([]string(nil), item.AllowedValueCodes...),
			Priority:            item.Priority,
			PriorityMode:        item.PriorityMode,
			LocalOverrideMode:   item.LocalOverrideMode,
			EffectiveDate:       item.EffectiveDate,
			CreatedAt:           item.UpdatedAt,
		})
		itemsByPolicyID[policyID] = item
	}

	decision, err := fieldpolicy.Resolve(fieldpolicy.PolicyContext{
		CapabilityKey:       capabilityKey,
		FieldKey:            fieldKey,
		ResolvedSetID:       resolvedSetID,
		BusinessUnitNodeKey: businessUnitNodeKey,
	}, baselineCapabilityKey, records)
	if err != nil {
		return setIDFieldDecisionResolution{}, err
	}

	resolution := setIDFieldDecisionResolution{
		Decision: setIDFieldDecision{
			CapabilityKey:      decision.CapabilityKey,
			SourceType:         decision.SourceType,
			FieldKey:           decision.FieldKey,
			Required:           decision.Required,
			Visible:            decision.Visible,
			Maintainable:       decision.Maintainable,
			DefaultRuleRef:     decision.DefaultRuleRef,
			ResolvedDefaultVal: decision.ResolvedDefaultVal,
			AllowedValueCodes:  append([]string(nil), decision.AllowedValueCodes...),
			PriorityMode:       decision.PriorityMode,
			LocalOverrideMode:  decision.LocalOverrideMode,
			Decision:           "allow",
		},
		MatchedBucket:    decision.MatchedBucket,
		PrimaryPolicyID:  decision.PrimaryPolicyID,
		WinnerPolicyIDs:  append([]string(nil), decision.WinnerPolicyIDs...),
		MatchedPolicyIDs: append([]string(nil), decision.MatchedPolicyIDs...),
		ResolutionTrace:  append([]string(nil), decision.ResolutionTrace...),
	}
	if item, ok := itemsByPolicyID[decision.PrimaryPolicyID]; ok {
		copyItem := item
		resolution.PrimaryItem = &copyItem
	}
	resolution.WinnerItems = strategyRegistryItemsByPolicyIDs(itemsByPolicyID, decision.WinnerPolicyIDs)
	resolution.MatchedItems = strategyRegistryItemsByPolicyIDs(itemsByPolicyID, decision.MatchedPolicyIDs)
	return resolution, nil
}

func strategyRegistryPolicyID(item setIDStrategyRegistryItem) string {
	key := strategyRegistrySortKey(item)
	if item.UpdatedAt == "" {
		return key
	}
	return key + "|" + strings.TrimSpace(item.UpdatedAt)
}

func strategyRegistryItemsByPolicyIDs(itemsByPolicyID map[string]setIDStrategyRegistryItem, ids []string) []setIDStrategyRegistryItem {
	if len(ids) == 0 {
		return nil
	}
	out := make([]setIDStrategyRegistryItem, 0, len(ids))
	for _, id := range ids {
		item, ok := itemsByPolicyID[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func fieldDecisionSemanticallyEqual(a setIDFieldDecision, b setIDFieldDecision) bool {
	if a.Required != b.Required || a.Visible != b.Visible || a.Maintainable != b.Maintainable {
		return false
	}
	if strings.TrimSpace(a.PriorityMode) != strings.TrimSpace(b.PriorityMode) {
		return false
	}
	if strings.TrimSpace(a.LocalOverrideMode) != strings.TrimSpace(b.LocalOverrideMode) {
		return false
	}
	if strings.TrimSpace(a.DefaultRuleRef) != strings.TrimSpace(b.DefaultRuleRef) {
		return false
	}
	if strings.TrimSpace(a.ResolvedDefaultVal) != strings.TrimSpace(b.ResolvedDefaultVal) {
		return false
	}
	left := normalizeAllowedValueCodes(a.AllowedValueCodes)
	right := normalizeAllowedValueCodes(b.AllowedValueCodes)
	if len(left) != len(right) {
		return false
	}
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}

func mergeStrategyItemsWithUpsert(items []setIDStrategyRegistryItem, upsertItem setIDStrategyRegistryItem) []setIDStrategyRegistryItem {
	merged := append([]setIDStrategyRegistryItem(nil), items...)
	upsertKey := strategyRegistrySortKey(upsertItem)
	replaced := false
	for i := range merged {
		if strategyRegistrySortKey(merged[i]) == upsertKey {
			merged[i] = upsertItem
			replaced = true
			break
		}
	}
	if !replaced {
		merged = append(merged, upsertItem)
	}
	return merged
}

func isRedundantIntentOverride(ctx context.Context, tenantID string, item setIDStrategyRegistryItem) (bool, error) {
	if strategySourceTypeForCapabilityKey(item.CapabilityKey) != strategySourceIntentOverride {
		return false, nil
	}
	baselineCapabilityKey, _ := orgUnitBaselineCapabilityKeyForIntentCapability(item.CapabilityKey)
	items, err := collectCapabilityResolutionItems(
		func(queryCapabilityKey string) ([]setIDStrategyRegistryItem, error) {
			return defaultSetIDStrategyRegistryStore.list(ctx, tenantID, queryCapabilityKey, item.FieldKey, item.EffectiveDate)
		},
		item.CapabilityKey,
	)
	if err != nil {
		return false, err
	}
	merged := mergeStrategyItemsWithUpsert(items, item)
	businessUnitNodeKey := ""
	if item.OrgApplicability == orgApplicabilityBusinessUnit {
		businessUnitNodeKey = item.BusinessUnitNodeKey
	}
	overrideDecision, err := resolveFieldDecisionFromItems(
		merged,
		item.CapabilityKey,
		item.FieldKey,
		item.ResolvedSetID,
		businessUnitNodeKey,
	)
	if err != nil {
		return false, err
	}
	baselineDecision, err := resolveFieldDecisionFromItems(
		merged,
		baselineCapabilityKey,
		item.FieldKey,
		item.ResolvedSetID,
		businessUnitNodeKey,
	)
	if err != nil {
		if strings.TrimSpace(err.Error()) == fieldPolicyMissingCode {
			return false, nil
		}
		return false, err
	}
	return fieldDecisionSemanticallyEqual(overrideDecision, baselineDecision), nil
}

func findStrategyRegistryItemForUpsert(ctx context.Context, tenantID string, item setIDStrategyRegistryItem) (setIDStrategyRegistryItem, bool, error) {
	items, err := defaultSetIDStrategyRegistryStore.list(ctx, tenantID, item.CapabilityKey, item.FieldKey, item.EffectiveDate)
	if err != nil {
		return setIDStrategyRegistryItem{}, false, err
	}
	for _, candidate := range items {
		if candidate.OrgApplicability != item.OrgApplicability {
			continue
		}
		if candidate.ResolvedSetID != item.ResolvedSetID {
			continue
		}
		if candidate.BusinessUnitNodeKey != item.BusinessUnitNodeKey {
			continue
		}
		if candidate.EffectiveDate != item.EffectiveDate {
			continue
		}
		return candidate, true, nil
	}
	return setIDStrategyRegistryItem{}, false, nil
}

func handleSetIDStrategyRegistryAPI(w http.ResponseWriter, r *http.Request, orgResolver OrgUnitCodeResolver, setIDStores ...SetIDGovernanceStore) {
	var setIDStore SetIDGovernanceStore
	if len(setIDStores) > 0 {
		setIDStore = setIDStores[0]
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	switch r.Method {
	case http.MethodGet:
		asOf, err := parseRequiredQueryDay(r, "as_of")
		if err != nil {
			writeInternalDayFieldError(w, r, err)
			return
		}
		items, err := defaultSetIDStrategyRegistryStore.list(r.Context(), tenant.ID, r.URL.Query().Get("capability_key"), r.URL.Query().Get("field_key"), asOf)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_strategy_registry_list_failed", "setid strategy registry list failed")
			return
		}
		apiItems := make([]setIDStrategyRegistryAPIItem, 0, len(items))
		for _, item := range items {
			apiItem, err := strategyRegistryAPIItemFromInternal(r.Context(), tenant.ID, item, orgResolver)
			if err != nil {
				writeInternalAPIError(w, r, err, "setid_strategy_registry_org_ref_invalid")
				return
			}
			apiItems = append(apiItems, apiItem)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant_id": tenant.ID,
			"as_of":     asOf,
			"items":     apiItems,
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
		businessUnitOrgCode := ""
		businessUnitNodeKey := ""
		resolvedSetID := ""
		if strings.EqualFold(strings.TrimSpace(req.OrgApplicability), orgApplicabilityBusinessUnit) {
			if _, err := time.Parse("2006-01-02", strings.TrimSpace(req.EffectiveDate)); err == nil {
				policyCtx, resolveErr := resolveStrategyRegistryPolicyContext(r.Context(), tenant.ID, req, orgResolver, setIDStore)
				if resolveErr != nil {
					writeSetIDStrategyRegistryContextError(w, r, resolveErr)
					return
				}
				businessUnitOrgCode = policyCtx.BusinessUnitOrgCode
				businessUnitNodeKey = policyCtx.BusinessUnitNodeKey
				resolvedSetID = policyCtx.ResolvedSetID
			}
		}
		item := normalizeStrategyRegistryItem(req, businessUnitNodeKey, resolvedSetID)
		if status, code, message := validateStrategyRegistryItem(item); status != 0 {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
			return
		}
		_, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
			CapabilityKey:       item.CapabilityKey,
			BusinessUnitOrgCode: businessUnitOrgCode,
			AsOf:                item.EffectiveDate,
			RequireBusinessUnit: item.OrgApplicability == orgApplicabilityBusinessUnit,
		})
		if capErr != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
			return
		}
		if item.EndDate == "" {
			existing, found, err := findStrategyRegistryItemForUpsert(r.Context(), tenant.ID, item)
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_strategy_registry_upsert_failed", "setid strategy registry upsert failed")
				return
			}
			if found && existing.EndDate != "" && existing.EndDate <= time.Now().UTC().Format("2006-01-02") {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusConflict, fieldPolicyConflictCode, "cannot restore an already effective disable")
				return
			}
		}
		redundantOverride, err := isRedundantIntentOverride(r.Context(), tenant.ID, item)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_strategy_registry_upsert_failed", "setid strategy registry upsert failed")
			return
		}
		if redundantOverride {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, fieldPolicyRedundantOverride, fieldPolicyRedundantOverride)
			return
		}
		saved, updated, err := defaultSetIDStrategyRegistryStore.upsert(r.Context(), tenant.ID, item)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_strategy_registry_upsert_failed", "setid strategy registry upsert failed")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		apiItem, err := strategyRegistryAPIItemFromInternal(r.Context(), tenant.ID, saved, orgResolver)
		if err != nil {
			writeInternalAPIError(w, r, err, "setid_strategy_registry_org_ref_invalid")
			return
		}
		if updated {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(apiItem)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleSetIDStrategyRegistryDisableAPI(w http.ResponseWriter, r *http.Request, orgResolver OrgUnitCodeResolver, setIDStores ...SetIDGovernanceStore) {
	var setIDStore SetIDGovernanceStore
	if len(setIDStores) > 0 {
		setIDStore = setIDStores[0]
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	if r.Method != http.MethodPost {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req setIDStrategyRegistryDisableAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
		return
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "request_id required")
		return
	}
	businessUnitOrgCode := ""
	businessUnitNodeKey := ""
	resolvedSetID := ""
	if strings.EqualFold(strings.TrimSpace(req.OrgApplicability), orgApplicabilityBusinessUnit) {
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(req.EffectiveDate)); err == nil {
			policyCtx, resolveErr := resolveStrategyRegistryDisablePolicyContext(r.Context(), tenant.ID, req, orgResolver, setIDStore)
			if resolveErr != nil {
				writeSetIDStrategyRegistryContextError(w, r, resolveErr)
				return
			}
			businessUnitOrgCode = policyCtx.BusinessUnitOrgCode
			businessUnitNodeKey = policyCtx.BusinessUnitNodeKey
			resolvedSetID = policyCtx.ResolvedSetID
		}
	}
	item := normalizeStrategyRegistryDisableRequest(req, businessUnitNodeKey, resolvedSetID)
	if status, code, message := validateStrategyRegistryDisableRequest(item); status != 0 {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
		return
	}
	_, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       item.CapabilityKey,
		BusinessUnitOrgCode: businessUnitOrgCode,
		AsOf:                item.DisableAsOf,
		RequireBusinessUnit: item.OrgApplicability == orgApplicabilityBusinessUnit,
	})
	if capErr != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, statusCodeForCapabilityContextError(capErr.Code), capErr.Code, capErr.Message)
		return
	}
	saved, _, err := defaultSetIDStrategyRegistryStore.disable(r.Context(), tenant.ID, item)
	if err != nil {
		switch {
		case errors.Is(err, errStrategyNotFound):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "setid_strategy_registry_not_found", "setid strategy registry not found")
		case err.Error() == "invalid_disable_date":
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_disable_date", "invalid disable_as_of")
		case errors.Is(err, errDisableNotAllowed):
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, fieldPolicyDisableDeniedCode, fieldPolicyDisableDeniedCode)
		default:
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_strategy_registry_disable_failed", "setid strategy registry disable failed")
		}
		return
	}
	apiItem, err := strategyRegistryAPIItemFromInternal(r.Context(), tenant.ID, saved, orgResolver)
	if err != nil {
		writeInternalAPIError(w, r, err, "setid_strategy_registry_org_ref_invalid")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiItem)
}
