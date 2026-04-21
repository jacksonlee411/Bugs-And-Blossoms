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
	setidresolver "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
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
SELECT
  e.id,
  e.event_uuid::text,
  CASE
    WHEN to_jsonb(e) ? 'org_node_key'
      THEN btrim(COALESCE(to_jsonb(e)->>'org_node_key', ''))
    ELSE orgunit.encode_org_node_key(NULLIF(to_jsonb(e)->>'org_id', '')::bigint)::text
  END AS org_node_key,
  e.event_type,
  e.effective_date::text,
  e.payload,
  e.transaction_time
FROM orgunit.org_events e
WHERE e.tenant_uuid = $1::uuid AND e.event_uuid = $2::uuid
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
SELECT
  e.id,
  e.event_uuid::text,
  CASE
    WHEN to_jsonb(e) ? 'org_node_key'
      THEN btrim(COALESCE(to_jsonb(e)->>'org_node_key', ''))
    ELSE orgunit.encode_org_node_key(NULLIF(to_jsonb(e)->>'org_id', '')::bigint)::text
  END AS org_node_key,
  e.event_type,
  e.effective_date::text,
  e.payload,
  e.transaction_time
FROM orgunit.org_events_effective e
WHERE e.tenant_uuid = $1::uuid
  AND CASE
    WHEN to_jsonb(e) ? 'org_node_key'
      THEN btrim(COALESCE(to_jsonb(e)->>'org_node_key', '')) = $2::text
    ELSE NULLIF(to_jsonb(e)->>'org_id', '')::int = orgunit.decode_org_node_key($2::char(8))::int
  END
  AND e.effective_date = $3::date
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
SELECT
  e.id,
  e.event_uuid::text,
  CASE
    WHEN to_jsonb(e) ? 'org_node_key'
      THEN btrim(COALESCE(to_jsonb(e)->>'org_node_key', ''))
    ELSE orgunit.encode_org_node_key(NULLIF(to_jsonb(e)->>'org_id', '')::bigint)::text
  END AS org_node_key,
  e.event_type,
  e.effective_date::text,
  e.payload,
  e.transaction_time
FROM orgunit.org_events e
WHERE e.tenant_uuid = $1::uuid
  AND e.request_id = $2::text
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

func rootOrgNodeKeyCompatExpr(alias string) string {
	return fmt.Sprintf(
		"CASE WHEN to_jsonb(%[1]s) ? 'root_org_node_key' THEN btrim(COALESCE(to_jsonb(%[1]s)->>'root_org_node_key', '')) ELSE orgunit.encode_org_node_key(NULLIF(to_jsonb(%[1]s)->>'root_org_id', '')::bigint)::text END",
		alias,
	)
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

func (s *OrgUnitPGStore) ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOf string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}
	normalizedOrgNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(strings.TrimSpace(orgNodeKey))
	if err != nil {
		return "", err
	}
	resolvedSetID, err := setidresolver.Resolve(ctx, tx, tenantID, normalizedOrgNodeKey, strings.TrimSpace(asOf))
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return resolvedSetID, nil
}

func (s *OrgUnitPGStore) IsOrgTreeInitialized(ctx context.Context, tenantID string) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return false, err
	}
	var rootOrgNodeKey *string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
SELECT %s AS root_org_node_key
FROM orgunit.org_trees t
WHERE tenant_uuid = $1::uuid
LIMIT 1
	`, rootOrgNodeKeyCompatExpr("t")), tenantID).Scan(&rootOrgNodeKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return rootOrgNodeKey != nil && strings.TrimSpace(*rootOrgNodeKey) != "", nil
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

func cloneOptionalString(in *string) *string {
	if in == nil {
		return nil
	}
	value := strings.TrimSpace(*in)
	return &value
}
