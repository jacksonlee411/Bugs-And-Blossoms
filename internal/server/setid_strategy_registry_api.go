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
)

const (
	personalizationModeTenantOnly = "tenant_only"
	personalizationModeSetID      = "setid"
	orgApplicabilityTenant        = "tenant"
	orgApplicabilityBusinessUnit  = "business_unit"
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
)

var (
	capabilityKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	fieldKeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
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
	BusinessUnitID      string   `json:"business_unit_id,omitempty"`
	Required            bool     `json:"required"`
	Visible             bool     `json:"visible"`
	Maintainable        bool     `json:"maintainable"`
	DefaultRuleRef      string   `json:"default_rule_ref,omitempty"`
	DefaultValue        string   `json:"default_value,omitempty"`
	AllowedValueCodes   []string `json:"allowed_value_codes,omitempty"`
	Priority            int      `json:"priority"`
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
	BusinessUnitID      string   `json:"business_unit_id"`
	Required            bool     `json:"required"`
	Visible             bool     `json:"visible"`
	Maintainable        *bool    `json:"maintainable"`
	DefaultRuleRef      string   `json:"default_rule_ref"`
	DefaultValue        string   `json:"default_value"`
	AllowedValueCodes   []string `json:"allowed_value_codes"`
	Priority            int      `json:"priority"`
	ExplainRequired     bool     `json:"explain_required"`
	IsStable            bool     `json:"is_stable"`
	ChangePolicy        string   `json:"change_policy"`
	EffectiveDate       string   `json:"effective_date"`
	EndDate             string   `json:"end_date"`
	RequestID           string   `json:"request_id"`
}

type setIDStrategyRegistryDisableAPIRequest struct {
	CapabilityKey    string `json:"capability_key"`
	FieldKey         string `json:"field_key"`
	OrgApplicability string `json:"org_applicability"`
	BusinessUnitID   string `json:"business_unit_id"`
	EffectiveDate    string `json:"effective_date"`
	DisableAsOf      string `json:"disable_as_of"`
	RequestID        string `json:"request_id"`
}

type setIDStrategyRegistryDisableRequest struct {
	CapabilityKey    string
	FieldKey         string
	OrgApplicability string
	BusinessUnitID   string
	EffectiveDate    string
	DisableAsOf      string
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
	resolveFieldDecision(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, businessUnitID string, asOf string) (setIDFieldDecision, error)
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

func (s *setIDStrategyRegistryRuntimeStore) resolveFieldDecision(_ context.Context, tenantID string, capabilityKey string, fieldKey string, businessUnitID string, asOf string) (setIDFieldDecision, error) {
	return s.runtime.resolveFieldDecision(tenantID, capabilityKey, fieldKey, businessUnitID, asOf)
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
			&item.BusinessUnitID,
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
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(allowedValueCodesRaw) != "" {
			if err := json.Unmarshal([]byte(allowedValueCodesRaw), &item.AllowedValueCodes); err != nil {
				return nil, err
			}
		}
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
    AND business_unit_id = $5::text
    AND effective_date = $6::date
)
`, tenantID, item.CapabilityKey, item.FieldKey, item.OrgApplicability, item.BusinessUnitID, item.EffectiveDate).Scan(&updated); err != nil {
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
  business_unit_id,
  required,
  visible,
  maintainable,
  default_rule_ref,
  default_value,
  allowed_value_codes,
  priority,
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
  $8::boolean,
  $9::boolean,
  $10::boolean,
  NULLIF($11::text, ''),
  NULLIF($12::text, ''),
  $13::jsonb,
  $14::integer,
  $15::boolean,
  $16::boolean,
  $17::text,
  $18::date,
  $19::date,
  $20::timestamptz
)
ON CONFLICT (tenant_uuid, capability_key, field_key, org_applicability, business_unit_id, effective_date)
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
  explain_required = EXCLUDED.explain_required,
  is_stable = EXCLUDED.is_stable,
  change_policy = EXCLUDED.change_policy,
  end_date = EXCLUDED.end_date,
  updated_at = EXCLUDED.updated_at
`, tenantID, item.CapabilityKey, item.OwnerModule, item.FieldKey, item.PersonalizationMode, item.OrgApplicability, item.BusinessUnitID, item.Required, item.Visible, item.Maintainable, item.DefaultRuleRef, item.DefaultValue, allowedValueCodesJSON, item.Priority, item.ExplainRequired, item.IsStable, item.ChangePolicy, item.EffectiveDate, endDate, item.UpdatedAt)
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
	businessUnitID := ""
	if req.OrgApplicability == orgApplicabilityBusinessUnit {
		businessUnitID = req.BusinessUnitID
	}
	if _, err := resolveFieldDecisionFromItems(active, req.CapabilityKey, req.FieldKey, businessUnitID); err != nil {
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
  business_unit_id,
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
  to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND capability_key = $2::text
  AND field_key = $3::text
  AND org_applicability = $4::text
  AND business_unit_id = $5::text
  AND effective_date = $6::date
FOR UPDATE
`, tenantID, req.CapabilityKey, req.FieldKey, req.OrgApplicability, req.BusinessUnitID, req.EffectiveDate)
		var allowedValueCodesRaw string
		if err := row.Scan(
			&target.CapabilityKey,
			&target.OwnerModule,
			&target.FieldKey,
			&target.PersonalizationMode,
			&target.OrgApplicability,
			&target.BusinessUnitID,
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
		target.AllowedValueCodes = normalizeAllowedValueCodes(target.AllowedValueCodes)
		if target.EndDate == endDate {
			return nil
		}
		if target.EndDate != "" && target.EndDate < endDate {
			return errors.New("invalid_disable_date")
		}
		if _, err := tx.Exec(ctx, `
UPDATE orgunit.setid_strategy_registry
SET end_date = $7::date,
    updated_at = $8::timestamptz
WHERE tenant_uuid = $1::uuid
  AND capability_key = $2::text
  AND field_key = $3::text
  AND org_applicability = $4::text
  AND business_unit_id = $5::text
  AND effective_date = $6::date
`, tenantID, req.CapabilityKey, req.FieldKey, req.OrgApplicability, req.BusinessUnitID, req.EffectiveDate, endDate, nowUTC); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
SELECT
  capability_key,
  owner_module,
  field_key,
  personalization_mode,
  org_applicability,
  business_unit_id,
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
  to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND capability_key = ANY($2::text[])
  AND field_key = $3::text
  AND effective_date <= $4::date
  AND (end_date IS NULL OR end_date > $4::date)
ORDER BY capability_key ASC, field_key ASC, org_applicability ASC, business_unit_id ASC, effective_date ASC
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
  business_unit_id,
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
  to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND ($2::text = '' OR capability_key = $2::text)
  AND ($3::text = '' OR field_key = $3::text)
  AND effective_date <= $4::date
  AND (end_date IS NULL OR end_date > $4::date)
ORDER BY capability_key ASC, field_key ASC, org_applicability ASC, business_unit_id ASC, effective_date ASC
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

func (s *setIDStrategyRegistryPGStore) resolveFieldDecision(ctx context.Context, tenantID string, capabilityKey string, fieldKey string, businessUnitID string, asOf string) (setIDFieldDecision, error) {
	items, err := collectCapabilityResolutionItems(
		func(queryCapabilityKey string) ([]setIDStrategyRegistryItem, error) {
			return s.list(ctx, tenantID, queryCapabilityKey, fieldKey, asOf)
		},
		capabilityKey,
	)
	if err != nil {
		return setIDFieldDecision{}, err
	}
	return resolveFieldDecisionFromItems(items, capabilityKey, fieldKey, businessUnitID)
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
		item.BusinessUnitID,
		item.EffectiveDate,
	}, "|")
}

func normalizeStrategyRegistryItem(req setIDStrategyRegistryUpsertAPIRequest) setIDStrategyRegistryItem {
	item := setIDStrategyRegistryItem{
		CapabilityKey:       strings.ToLower(strings.TrimSpace(req.CapabilityKey)),
		OwnerModule:         strings.ToLower(strings.TrimSpace(req.OwnerModule)),
		SourceType:          "",
		FieldKey:            strings.ToLower(strings.TrimSpace(req.FieldKey)),
		PersonalizationMode: strings.ToLower(strings.TrimSpace(req.PersonalizationMode)),
		OrgApplicability:    strings.ToLower(strings.TrimSpace(req.OrgApplicability)),
		BusinessUnitID:      strings.TrimSpace(req.BusinessUnitID),
		Required:            req.Required,
		Visible:             req.Visible,
		Maintainable:        true,
		DefaultRuleRef:      strings.TrimSpace(req.DefaultRuleRef),
		DefaultValue:        strings.TrimSpace(req.DefaultValue),
		AllowedValueCodes:   normalizeAllowedValueCodes(req.AllowedValueCodes),
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
	if req.Maintainable != nil {
		item.Maintainable = *req.Maintainable
	}
	if item.ChangePolicy == "" {
		item.ChangePolicy = "plan_required"
	}
	if item.OrgApplicability == orgApplicabilityTenant {
		item.BusinessUnitID = ""
	}
	item.SourceType = strategySourceTypeForCapabilityKey(item.CapabilityKey)
	item.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return item
}

func normalizeStrategyRegistryDisableRequest(req setIDStrategyRegistryDisableAPIRequest) setIDStrategyRegistryDisableRequest {
	item := setIDStrategyRegistryDisableRequest{
		CapabilityKey:    strings.ToLower(strings.TrimSpace(req.CapabilityKey)),
		FieldKey:         strings.ToLower(strings.TrimSpace(req.FieldKey)),
		OrgApplicability: strings.ToLower(strings.TrimSpace(req.OrgApplicability)),
		BusinessUnitID:   strings.TrimSpace(req.BusinessUnitID),
		EffectiveDate:    strings.TrimSpace(req.EffectiveDate),
		DisableAsOf:      strings.TrimSpace(req.DisableAsOf),
	}
	if item.OrgApplicability == orgApplicabilityTenant {
		item.BusinessUnitID = ""
	}
	return item
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
	switch item.OrgApplicability {
	case orgApplicabilityTenant:
	case orgApplicabilityBusinessUnit:
		if item.BusinessUnitID == "" {
			return http.StatusBadRequest, "invalid_business_unit_id", "business_unit_id required"
		}
		if _, err := parseOrgID8(item.BusinessUnitID); err != nil {
			return http.StatusBadRequest, "invalid_business_unit_id", "invalid business_unit_id"
		}
	default:
		return http.StatusUnprocessableEntity, "org_applicability_invalid", "org_applicability invalid"
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
		if req.BusinessUnitID == "" {
			return http.StatusBadRequest, "invalid_business_unit_id", "business_unit_id required"
		}
		if _, err := parseOrgID8(req.BusinessUnitID); err != nil {
			return http.StatusBadRequest, "invalid_business_unit_id", "invalid business_unit_id"
		}
	default:
		return http.StatusUnprocessableEntity, "org_applicability_invalid", "org_applicability invalid"
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
		req.BusinessUnitID,
		req.EffectiveDate,
	}, "|")
	targetIndex := -1
	for i := range items {
		candidateKey := strings.Join([]string{
			items[i].CapabilityKey,
			items[i].FieldKey,
			items[i].OrgApplicability,
			items[i].BusinessUnitID,
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
	businessUnitID string,
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
	return resolveFieldDecisionFromItems(items, capabilityKey, fieldKey, businessUnitID)
}

func resolveFieldDecisionFromItems(items []setIDStrategyRegistryItem, capabilityKey string, fieldKey string, businessUnitID string) (setIDFieldDecision, error) {
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	fieldKey = strings.ToLower(strings.TrimSpace(fieldKey))
	businessUnitID = strings.TrimSpace(businessUnitID)

	baselineCapabilityKey, hasBaseline := orgUnitBaselineCapabilityKeyForIntentCapability(capabilityKey)
	if !hasBaseline {
		baselineCapabilityKey = capabilityKey
	}

	lookupChain := []struct {
		capabilityKey    string
		orgApplicability string
		sourceType       string
	}{
		{capabilityKey: capabilityKey, orgApplicability: orgApplicabilityBusinessUnit, sourceType: strategySourceIntentOverride},
		{capabilityKey: baselineCapabilityKey, orgApplicability: orgApplicabilityBusinessUnit, sourceType: strategySourceBaseline},
		{capabilityKey: capabilityKey, orgApplicability: orgApplicabilityTenant, sourceType: strategySourceIntentOverride},
		{capabilityKey: baselineCapabilityKey, orgApplicability: orgApplicabilityTenant, sourceType: strategySourceBaseline},
	}

	for _, step := range lookupChain {
		if step.capabilityKey == "" {
			continue
		}
		if step.capabilityKey == capabilityKey && step.sourceType == strategySourceBaseline {
			continue
		}
		if step.orgApplicability == orgApplicabilityBusinessUnit && businessUnitID == "" {
			continue
		}
		decision, found, err := resolveCapabilityBucketDecision(items, step.capabilityKey, fieldKey, step.orgApplicability, businessUnitID, step.sourceType)
		if err != nil {
			return setIDFieldDecision{}, err
		}
		if found {
			return decision, nil
		}
	}
	return setIDFieldDecision{}, errors.New(fieldPolicyMissingCode)
}

func resolveCapabilityBucketDecision(
	items []setIDStrategyRegistryItem,
	capabilityKey string,
	fieldKey string,
	orgApplicability string,
	businessUnitID string,
	sourceType string,
) (setIDFieldDecision, bool, error) {
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	fieldKey = strings.ToLower(strings.TrimSpace(fieldKey))
	businessUnitID = strings.TrimSpace(businessUnitID)

	var chosen *setIDStrategyRegistryItem
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.CapabilityKey)) != capabilityKey {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.FieldKey)) != fieldKey {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.OrgApplicability)) != orgApplicability {
			continue
		}
		if orgApplicability == orgApplicabilityBusinessUnit && !strings.EqualFold(strings.TrimSpace(item.BusinessUnitID), businessUnitID) {
			continue
		}
		candidate := item
		if chosen == nil ||
			candidate.Priority > chosen.Priority ||
			(candidate.Priority == chosen.Priority && candidate.EffectiveDate > chosen.EffectiveDate) ||
			(candidate.Priority == chosen.Priority && candidate.EffectiveDate == chosen.EffectiveDate && strategyRegistrySortKey(candidate) > strategyRegistrySortKey(*chosen)) {
			chosen = &candidate
		}
	}
	if chosen == nil {
		return setIDFieldDecision{}, false, nil
	}
	if chosen.Required && !chosen.Visible {
		return setIDFieldDecision{}, false, errors.New(fieldPolicyConflictCode)
	}
	if !chosen.Maintainable && strings.TrimSpace(chosen.DefaultRuleRef) == "" && strings.TrimSpace(chosen.DefaultValue) == "" {
		return setIDFieldDecision{}, false, errors.New(fieldDefaultRuleMissingCode)
	}
	return setIDFieldDecision{
		CapabilityKey:      chosen.CapabilityKey,
		SourceType:         sourceType,
		FieldKey:           chosen.FieldKey,
		Required:           chosen.Required,
		Visible:            chosen.Visible,
		Maintainable:       chosen.Maintainable,
		DefaultRuleRef:     chosen.DefaultRuleRef,
		ResolvedDefaultVal: chosen.DefaultValue,
		AllowedValueCodes:  append([]string(nil), chosen.AllowedValueCodes...),
		Decision:           "allow",
	}, true, nil
}

func fieldDecisionSemanticallyEqual(a setIDFieldDecision, b setIDFieldDecision) bool {
	if a.Required != b.Required || a.Visible != b.Visible || a.Maintainable != b.Maintainable {
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
	businessUnitID := ""
	if item.OrgApplicability == orgApplicabilityBusinessUnit {
		businessUnitID = item.BusinessUnitID
	}
	overrideDecision, _, err := resolveCapabilityBucketDecision(
		merged,
		item.CapabilityKey,
		item.FieldKey,
		item.OrgApplicability,
		businessUnitID,
		strategySourceIntentOverride,
	)
	if err != nil {
		return false, err
	}
	baselineDecision, foundBaseline, err := resolveCapabilityBucketDecision(
		merged,
		baselineCapabilityKey,
		item.FieldKey,
		item.OrgApplicability,
		businessUnitID,
		strategySourceBaseline,
	)
	if err != nil {
		return false, err
	}
	if !foundBaseline {
		return false, nil
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
		if candidate.BusinessUnitID != item.BusinessUnitID {
			continue
		}
		if candidate.EffectiveDate != item.EffectiveDate {
			continue
		}
		return candidate, true, nil
	}
	return setIDStrategyRegistryItem{}, false, nil
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
		items, err := defaultSetIDStrategyRegistryStore.list(r.Context(), tenant.ID, r.URL.Query().Get("capability_key"), r.URL.Query().Get("field_key"), asOf)
		if err != nil {
			if strings.Contains(err.Error(), "invalid as_of") {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "setid_strategy_registry_list_failed", "setid strategy registry list failed")
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
		_, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
			CapabilityKey:       item.CapabilityKey,
			BusinessUnitID:      item.BusinessUnitID,
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

func handleSetIDStrategyRegistryDisableAPI(w http.ResponseWriter, r *http.Request) {
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
	item := normalizeStrategyRegistryDisableRequest(req)
	if status, code, message := validateStrategyRegistryDisableRequest(item); status != 0 {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
		return
	}
	_, capErr := resolveCapabilityContext(r.Context(), r, capabilityContextInput{
		CapabilityKey:       item.CapabilityKey,
		BusinessUnitID:      item.BusinessUnitID,
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(saved)
}
