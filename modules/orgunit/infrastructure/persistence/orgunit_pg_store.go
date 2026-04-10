package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

var marshalCreatePayloadJSON = json.Marshal

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type OrgUnitPGStore struct {
	pool pgBeginner
}

func NewOrgUnitPGStore(pool pgBeginner) ports.OrgUnitWriteStore {
	return &OrgUnitPGStore{pool: pool}
}

func (s *OrgUnitPGStore) SubmitEvent(ctx context.Context, tenantID string, eventUUID string, orgNodeKey *string, eventType string, effectiveDate string, payload json.RawMessage, requestID string, initiatorUUID string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return 0, err
	}

	var orgNodeKeyValue any
	if orgNodeKey != nil {
		orgNodeKeyValue = *orgNodeKey
	}

	var eventID int64
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  $3::char(8),
  $4::text,
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
)
`, eventUUID, tenantID, orgNodeKeyValue, eventType, effectiveDate, payload, requestID, initiatorUUID).Scan(&eventID); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return eventID, nil
}

func (s *OrgUnitPGStore) SubmitCorrection(ctx context.Context, tenantID string, orgNodeKey string, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	var correctionUUID string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event_correction(
  $1::uuid,
  $2::char(8),
  $3::date,
  $4::jsonb,
  $5::text,
  $6::uuid
)
`, tenantID, orgNodeKey, targetEffectiveDate, patch, requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correctionUUID, nil
}

func (s *OrgUnitPGStore) SubmitStatusCorrection(ctx context.Context, tenantID string, orgNodeKey string, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	var correctionUUID string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_status_correction(
  $1::uuid,
  $2::char(8),
  $3::date,
  $4::text,
  $5::text,
  $6::uuid
)
`, tenantID, orgNodeKey, targetEffectiveDate, targetStatus, requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correctionUUID, nil
}

func (s *OrgUnitPGStore) SubmitRescindEvent(ctx context.Context, tenantID string, orgNodeKey string, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	var correctionUUID string
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event_rescind(
  $1::uuid,
  $2::char(8),
  $3::date,
  $4::text,
  $5::text,
  $6::uuid
)
`, tenantID, orgNodeKey, targetEffectiveDate, reason, requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correctionUUID, nil
}

func (s *OrgUnitPGStore) SubmitRescindOrg(ctx context.Context, tenantID string, orgNodeKey string, reason string, requestID string, initiatorUUID string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return 0, err
	}

	var rescindedEvents int
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_rescind(
  $1::uuid,
  $2::char(8),
  $3::text,
  $4::text,
  $5::uuid
)
`, tenantID, orgNodeKey, reason, requestID, initiatorUUID).Scan(&rescindedEvents); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return rescindedEvents, nil
}

func (s *OrgUnitPGStore) FindEventByUUID(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.OrgUnitEvent{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.OrgUnitEvent{}, err
	}

	var event types.OrgUnitEvent
	var payload []byte
	if err := tx.QueryRow(ctx, `
SELECT id, event_uuid::text, orgunit.encode_org_node_key(org_id::bigint)::text, event_type, effective_date::text, payload, transaction_time
FROM orgunit.org_events
WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
	`, tenantID, eventUUID).Scan(&event.ID, &event.EventUUID, &event.OrgNodeKey, &event.EventType, &event.EffectiveDate, &payload, &event.TransactionTime); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.OrgUnitEvent{}, ports.ErrOrgEventNotFound
		}
		return types.OrgUnitEvent{}, err
	}

	if payload != nil {
		event.Payload = json.RawMessage(payload)
	}

	if err := tx.Commit(ctx); err != nil {
		return types.OrgUnitEvent{}, err
	}
	return event, nil
}

func (s *OrgUnitPGStore) FindEventByEffectiveDate(ctx context.Context, tenantID string, orgNodeKey string, effectiveDate string) (types.OrgUnitEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.OrgUnitEvent{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.OrgUnitEvent{}, err
	}

	var event types.OrgUnitEvent
	var payload []byte
	if err := tx.QueryRow(ctx, `
SELECT id, event_uuid::text, orgunit.encode_org_node_key(org_id::bigint)::text, event_type, effective_date::text, payload, transaction_time
FROM orgunit.org_events_effective
WHERE tenant_uuid = $1::uuid
  AND org_id = orgunit.decode_org_node_key($2::char(8))::int
  AND effective_date = $3::date
	`, tenantID, orgNodeKey, effectiveDate).Scan(&event.ID, &event.EventUUID, &event.OrgNodeKey, &event.EventType, &event.EffectiveDate, &payload, &event.TransactionTime); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.OrgUnitEvent{}, ports.ErrOrgEventNotFound
		}
		return types.OrgUnitEvent{}, err
	}

	if payload != nil {
		event.Payload = json.RawMessage(payload)
	}

	if err := tx.Commit(ctx); err != nil {
		return types.OrgUnitEvent{}, err
	}
	return event, nil
}

func (s *OrgUnitPGStore) FindEventByRequestID(ctx context.Context, tenantID string, requestID string) (types.OrgUnitEvent, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.OrgUnitEvent{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.OrgUnitEvent{}, false, err
	}

	var event types.OrgUnitEvent
	var payload []byte
	if err := tx.QueryRow(ctx, `
SELECT id, event_uuid::text, orgunit.encode_org_node_key(org_id::bigint)::text, event_type, effective_date::text, payload, transaction_time
FROM orgunit.org_events
WHERE tenant_uuid = $1::uuid
  AND request_id = $2::text
ORDER BY id DESC
LIMIT 1
`, tenantID, requestID).Scan(&event.ID, &event.EventUUID, &event.OrgNodeKey, &event.EventType, &event.EffectiveDate, &payload, &event.TransactionTime); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.OrgUnitEvent{}, false, nil
		}
		return types.OrgUnitEvent{}, false, err
	}
	if payload != nil {
		event.Payload = json.RawMessage(payload)
	}

	if err := tx.Commit(ctx); err != nil {
		return types.OrgUnitEvent{}, false, err
	}
	return event, true, nil
}

func (s *OrgUnitPGStore) ResolveTenantFieldPolicy(
	ctx context.Context,
	tenantID string,
	fieldKey string,
	scopeType string,
	scopeKey string,
	asOf string,
) (types.TenantFieldPolicy, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.TenantFieldPolicy{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.TenantFieldPolicy{}, false, err
	}

	scopeType = strings.ToUpper(strings.TrimSpace(scopeType))
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeType != "FORM" && scopeType != "GLOBAL" {
		return types.TenantFieldPolicy{}, false, nil
	}
	if scopeType == "GLOBAL" {
		scopeKey = "global"
	}

	query := `
SELECT
  field_key,
  scope_type,
  scope_key,
  maintainable,
  default_mode,
  default_rule_expr,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on
FROM orgunit.tenant_field_policies
WHERE tenant_uuid = $1::uuid
  AND field_key = $2::text
  AND enabled_on <= $3::date
  AND ($3::date < COALESCE(disabled_on, 'infinity'::date))
  AND (
    (scope_type = 'FORM' AND scope_key = $4::text)
    OR
    (scope_type = 'GLOBAL' AND scope_key = 'global')
  )
ORDER BY CASE
  WHEN scope_type = 'FORM' AND scope_key = $4::text THEN 0
  ELSE 1
END ASC, enabled_on DESC
LIMIT 1
`
	if scopeType == "GLOBAL" {
		query = `
SELECT
  field_key,
  scope_type,
  scope_key,
  maintainable,
  default_mode,
  default_rule_expr,
  enabled_on::text,
  CASE WHEN disabled_on IS NULL THEN NULL ELSE disabled_on::text END AS disabled_on
FROM orgunit.tenant_field_policies
WHERE tenant_uuid = $1::uuid
  AND field_key = $2::text
  AND enabled_on <= $3::date
  AND ($3::date < COALESCE(disabled_on, 'infinity'::date))
  AND scope_type = 'GLOBAL'
  AND scope_key = 'global'
ORDER BY enabled_on DESC
LIMIT 1
`
	}

	var policy types.TenantFieldPolicy
	var defaultRule *string
	var disabledOn *string
	if err := tx.QueryRow(ctx, query, tenantID, fieldKey, asOf, scopeKey).Scan(
		&policy.FieldKey,
		&policy.ScopeType,
		&policy.ScopeKey,
		&policy.Maintainable,
		&policy.DefaultMode,
		&defaultRule,
		&policy.EnabledOn,
		&disabledOn,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.TenantFieldPolicy{}, false, nil
		}
		return types.TenantFieldPolicy{}, false, err
	}
	policy.DefaultMode = strings.ToUpper(strings.TrimSpace(policy.DefaultMode))
	policy.DefaultRuleExpr = cloneOptionalString(defaultRule)
	policy.DisabledOn = cloneOptionalString(disabledOn)

	if err := tx.Commit(ctx); err != nil {
		return types.TenantFieldPolicy{}, false, err
	}
	return policy, true, nil
}

func (s *OrgUnitPGStore) ResolveSetIDStrategyFieldDecision(
	ctx context.Context,
	tenantID string,
	capabilityKey string,
	fieldKey string,
	businessUnitID string,
	asOf string,
) (types.SetIDStrategyFieldDecision, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.SetIDStrategyFieldDecision{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.SetIDStrategyFieldDecision{}, false, err
	}

	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	fieldKey = strings.ToLower(strings.TrimSpace(fieldKey))
	businessUnitID = strings.TrimSpace(businessUnitID)
	asOf = strings.TrimSpace(asOf)
	baselineCapabilityKey := orgUnitBaselineCapabilityKeyForSetIDStrategy(capabilityKey)

	rows, err := tx.Query(ctx, `
SELECT
  capability_key,
  field_key,
  org_applicability,
  business_unit_id,
  required,
  visible,
  maintainable,
  COALESCE(default_rule_ref, ''),
  COALESCE(default_value, ''),
  COALESCE(allowed_value_codes, '[]'::jsonb)::text,
  priority,
  effective_date::text
FROM orgunit.setid_strategy_registry
WHERE tenant_uuid = $1::uuid
  AND field_key = $2::text
  AND effective_date <= $3::date
  AND (end_date IS NULL OR end_date > $3::date)
  AND (capability_key = $4::text OR capability_key = $5::text)
  AND (
    (org_applicability = 'business_unit' AND business_unit_id = $6::text)
    OR (org_applicability = 'tenant' AND business_unit_id = '')
  )
`, tenantID, fieldKey, asOf, capabilityKey, baselineCapabilityKey, businessUnitID)
	if err != nil {
		return types.SetIDStrategyFieldDecision{}, false, err
	}
	defer rows.Close()

	var chosen *setIDStrategyFieldDecisionCandidate
	chosenBucket := -1
	for rows.Next() {
		var candidate setIDStrategyFieldDecisionCandidate
		if err := rows.Scan(
			&candidate.CapabilityKey,
			&candidate.FieldKey,
			&candidate.OrgApplicability,
			&candidate.BusinessUnitID,
			&candidate.Required,
			&candidate.Visible,
			&candidate.Maintainable,
			&candidate.DefaultRuleRef,
			&candidate.DefaultValue,
			&candidate.AllowedValueCodesRaw,
			&candidate.Priority,
			&candidate.EffectiveDate,
		); err != nil {
			return types.SetIDStrategyFieldDecision{}, false, err
		}
		bucket := orgUnitSetIDStrategyDecisionBucket(candidate, capabilityKey, baselineCapabilityKey, businessUnitID)
		if bucket < 0 {
			continue
		}
		if chosen == nil || bucket > chosenBucket || (bucket == chosenBucket && orgUnitSetIDStrategyCandidateWins(candidate, *chosen)) {
			picked := candidate
			chosen = &picked
			chosenBucket = bucket
		}
	}
	if err := rows.Err(); err != nil {
		return types.SetIDStrategyFieldDecision{}, false, err
	}
	if chosen == nil {
		return types.SetIDStrategyFieldDecision{}, false, nil
	}

	decision := types.SetIDStrategyFieldDecision{
		CapabilityKey:  chosen.CapabilityKey,
		FieldKey:       chosen.FieldKey,
		Required:       chosen.Required,
		Visible:        chosen.Visible,
		Maintainable:   chosen.Maintainable,
		DefaultRuleRef: chosen.DefaultRuleRef,
		DefaultValue:   chosen.DefaultValue,
	}
	if strings.TrimSpace(chosen.AllowedValueCodesRaw) != "" {
		if err := json.Unmarshal([]byte(chosen.AllowedValueCodesRaw), &decision.AllowedValueCodes); err != nil {
			return types.SetIDStrategyFieldDecision{}, false, err
		}
	}
	decision.AllowedValueCodes = normalizeAllowedValueCodes(decision.AllowedValueCodes)

	if err := tx.Commit(ctx); err != nil {
		return types.SetIDStrategyFieldDecision{}, false, err
	}
	return decision, true, nil
}

type setIDStrategyFieldDecisionCandidate struct {
	CapabilityKey        string
	FieldKey             string
	OrgApplicability     string
	BusinessUnitID       string
	Required             bool
	Visible              bool
	Maintainable         bool
	DefaultRuleRef       string
	DefaultValue         string
	AllowedValueCodesRaw string
	Priority             int
	EffectiveDate        string
}

func orgUnitBaselineCapabilityKeyForSetIDStrategy(capabilityKey string) string {
	capabilityKey = strings.ToLower(strings.TrimSpace(capabilityKey))
	switch capabilityKey {
	case "org.orgunit_create.field_policy",
		"org.orgunit_add_version.field_policy",
		"org.orgunit_insert_version.field_policy",
		"org.orgunit_correct.field_policy":
		return "org.orgunit_write.field_policy"
	default:
		return capabilityKey
	}
}

func orgUnitSetIDStrategyDecisionBucket(
	candidate setIDStrategyFieldDecisionCandidate,
	capabilityKey string,
	baselineCapabilityKey string,
	businessUnitID string,
) int {
	candidateCapabilityKey := strings.ToLower(strings.TrimSpace(candidate.CapabilityKey))
	candidateApplicability := strings.ToLower(strings.TrimSpace(candidate.OrgApplicability))
	candidateBusinessUnitID := strings.TrimSpace(candidate.BusinessUnitID)
	switch {
	case candidateCapabilityKey == capabilityKey && candidateApplicability == "business_unit" && candidateBusinessUnitID == businessUnitID && businessUnitID != "":
		return 4
	case candidateCapabilityKey == baselineCapabilityKey && candidateApplicability == "business_unit" && candidateBusinessUnitID == businessUnitID && businessUnitID != "":
		return 3
	case candidateCapabilityKey == capabilityKey && candidateApplicability == "tenant" && candidateBusinessUnitID == "":
		return 2
	case candidateCapabilityKey == baselineCapabilityKey && candidateApplicability == "tenant" && candidateBusinessUnitID == "":
		return 1
	default:
		return -1
	}
}

func orgUnitSetIDStrategyCandidateWins(candidate setIDStrategyFieldDecisionCandidate, chosen setIDStrategyFieldDecisionCandidate) bool {
	if candidate.Priority != chosen.Priority {
		return candidate.Priority > chosen.Priority
	}
	if candidate.EffectiveDate != chosen.EffectiveDate {
		return candidate.EffectiveDate > chosen.EffectiveDate
	}
	if candidate.CapabilityKey != chosen.CapabilityKey {
		return candidate.CapabilityKey > chosen.CapabilityKey
	}
	if candidate.OrgApplicability != chosen.OrgApplicability {
		return candidate.OrgApplicability > chosen.OrgApplicability
	}
	return candidate.BusinessUnitID > chosen.BusinessUnitID
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
		if _, ok := seen[value]; ok {
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

func (s *OrgUnitPGStore) SubmitCreateEventWithGeneratedCode(
	ctx context.Context,
	tenantID string,
	eventUUID string,
	effectiveDate string,
	payload json.RawMessage,
	requestID string,
	initiatorUUID string,
	prefix string,
	width int,
) (int64, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return 0, "", err
	}

	lockKey := fmt.Sprintf("orgunit.next_org_code:%s:%s:%d", tenantID, prefix, width)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0));`, lockKey); err != nil {
		return 0, "", err
	}

	codeLen := len(prefix) + width
	rows, err := tx.Query(ctx, `
SELECT org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid
  AND org_code LIKE ($2::text || '%')
  AND length(org_code) = $3::int
ORDER BY org_code ASC
`, tenantID, prefix, codeLen)
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()

	next := 1
	max := 1
	for range width {
		max *= 10
	}
	max -= 1

	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return 0, "", err
		}
		if !strings.HasPrefix(code, prefix) || len(code) != codeLen {
			continue
		}
		suffix := code[len(prefix):]
		num, err := strconv.Atoi(suffix)
		if err != nil || num <= 0 {
			continue
		}
		if num == next {
			next++
			continue
		}
		if num > next {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}
	if next > max {
		return 0, "", errors.New("ORG_CODE_EXHAUSTED")
	}

	orgCode := fmt.Sprintf("%s%0*d", prefix, width, next)
	payloadObj := map[string]any{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &payloadObj); err != nil {
			return 0, "", err
		}
	}
	payloadObj["org_code"] = orgCode
	payloadWithCode, err := marshalCreatePayloadJSON(payloadObj)
	if err != nil {
		return 0, "", err
	}
	var orgNodeKeyValue any

	var eventID int64
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  $3::char(8),
  $4::text,
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
)
	`, eventUUID, tenantID, orgNodeKeyValue, string(types.OrgUnitEventCreate), effectiveDate, payloadWithCode, requestID, initiatorUUID).Scan(&eventID); err != nil {
		return 0, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, "", err
	}
	return eventID, orgCode, nil
}

func (s *OrgUnitPGStore) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT field_key, value_type, data_source_type, data_source_config
FROM orgunit.tenant_field_configs
WHERE tenant_uuid = $1::uuid
  AND enabled_on <= $2::date
  AND (disabled_on IS NULL OR $2::date < disabled_on)
ORDER BY field_key ASC
`, tenantID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.TenantFieldConfig, 0)
	for rows.Next() {
		var cfg types.TenantFieldConfig
		var raw []byte
		if err := rows.Scan(&cfg.FieldKey, &cfg.ValueType, &cfg.DataSourceType, &raw); err != nil {
			return nil, err
		}
		if raw != nil {
			cfg.DataSourceConfig = json.RawMessage(raw)
		}
		out = append(out, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *OrgUnitPGStore) ResolveOrgNodeKey(ctx context.Context, tenantID string, orgCode string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	orgNodeKey, err := orgunitpkg.ResolveOrgNodeKeyByCode(ctx, tx, tenantID, orgCode)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orgNodeKey, nil
}

func (s *OrgUnitPGStore) ResolveOrgCodeByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	orgCode, err := orgunitpkg.ResolveOrgCodeByNodeKey(ctx, tx, tenantID, orgNodeKey)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orgCode, nil
}

func (s *OrgUnitPGStore) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (types.Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.Person{}, err
	}

	var p types.Person
	if err := tx.QueryRow(ctx, `
SELECT person_uuid::text, pernr, display_name, status
FROM person.persons
WHERE tenant_uuid = $1::uuid AND pernr = $2::text
`, tenantID, pernr).Scan(&p.UUID, &p.Pernr, &p.DisplayName, &p.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Person{}, ports.ErrPersonNotFound
		}
		return types.Person{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return types.Person{}, err
	}
	return p, nil
}

func cloneOptionalString(in *string) *string {
	if in == nil {
		return nil
	}
	value := strings.TrimSpace(*in)
	return &value
}
