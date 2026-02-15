package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type OrgUnitPGStore struct {
	pool pgBeginner
}

func NewOrgUnitPGStore(pool pgBeginner) ports.OrgUnitWriteStore {
	return &OrgUnitPGStore{pool: pool}
}

func (s *OrgUnitPGStore) SubmitEvent(ctx context.Context, tenantID string, eventUUID string, orgID *int, eventType string, effectiveDate string, payload json.RawMessage, requestCode string, initiatorUUID string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return 0, err
	}

	var orgIDValue any
	if orgID != nil {
		orgIDValue = *orgID
	}

	var eventID int64
	if err := tx.QueryRow(ctx, `
SELECT orgunit.submit_org_event(
  $1::uuid,
  $2::uuid,
  $3::int,
  $4::text,
  $5::date,
  $6::jsonb,
  $7::text,
  $8::uuid
)
`, eventUUID, tenantID, orgIDValue, eventType, effectiveDate, payload, requestCode, initiatorUUID).Scan(&eventID); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return eventID, nil
}

func (s *OrgUnitPGStore) SubmitCorrection(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error) {
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
  $2::int,
  $3::date,
  $4::jsonb,
  $5::text,
  $6::uuid
)
`, tenantID, orgID, targetEffectiveDate, patch, requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correctionUUID, nil
}

func (s *OrgUnitPGStore) SubmitStatusCorrection(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error) {
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
  $2::int,
  $3::date,
  $4::text,
  $5::text,
  $6::uuid
)
`, tenantID, orgID, targetEffectiveDate, targetStatus, requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correctionUUID, nil
}

func (s *OrgUnitPGStore) SubmitRescindEvent(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error) {
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
  $2::int,
  $3::date,
  $4::text,
  $5::text,
  $6::uuid
)
`, tenantID, orgID, targetEffectiveDate, reason, requestID, initiatorUUID).Scan(&correctionUUID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return correctionUUID, nil
}

func (s *OrgUnitPGStore) SubmitRescindOrg(ctx context.Context, tenantID string, orgID int, reason string, requestID string, initiatorUUID string) (int, error) {
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
  $2::int,
  $3::text,
  $4::text,
  $5::uuid
)
`, tenantID, orgID, reason, requestID, initiatorUUID).Scan(&rescindedEvents); err != nil {
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
SELECT id, event_uuid::text, org_id, event_type, effective_date::text, payload, transaction_time
FROM orgunit.org_events
WHERE tenant_uuid = $1::uuid AND event_uuid = $2::uuid
`, tenantID, eventUUID).Scan(&event.ID, &event.EventUUID, &event.OrgID, &event.EventType, &event.EffectiveDate, &payload, &event.TransactionTime); err != nil {
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

func (s *OrgUnitPGStore) FindEventByEffectiveDate(ctx context.Context, tenantID string, orgID int, effectiveDate string) (types.OrgUnitEvent, error) {
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
SELECT id, event_uuid::text, org_id, event_type, effective_date::text, payload, transaction_time
FROM orgunit.org_events_effective
WHERE tenant_uuid = $1::uuid AND org_id = $2::int AND effective_date = $3::date
`, tenantID, orgID, effectiveDate).Scan(&event.ID, &event.EventUUID, &event.OrgID, &event.EventType, &event.EffectiveDate, &payload, &event.TransactionTime); err != nil {
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

func (s *OrgUnitPGStore) ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return 0, err
	}

	orgID, err := orgunitpkg.ResolveOrgID(ctx, tx, tenantID, orgCode)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return orgID, nil
}

func (s *OrgUnitPGStore) ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return "", err
	}

	orgCode, err := orgunitpkg.ResolveOrgCode(ctx, tx, tenantID, orgID)
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
