package attendanceintegrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type PGStore struct {
	pool pgBeginner
}

func NewPGStore(pool pgBeginner) *PGStore {
	return &PGStore{pool: pool}
}

func normalizeProvider(p Provider) (Provider, error) {
	v := strings.ToUpper(strings.TrimSpace(string(p)))
	switch v {
	case string(ProviderDingTalk):
		return ProviderDingTalk, nil
	case string(ProviderWeCom):
		return ProviderWeCom, nil
	default:
		return "", errors.New("provider must be DINGTALK|WECOM")
	}
}

func normalizeExternalUserID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("external_user_id is required")
	}
	return raw, nil
}

func normalizeJSONObj(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(raw) {
		return nil, errors.New("json must be valid")
	}
	trimmed := bytes.TrimSpace(raw)
	if trimmed[0] != '{' {
		return nil, errors.New("json must be an object")
	}
	return raw, nil
}

func (s *PGStore) TouchExternalIdentityLink(ctx context.Context, tenantID string, provider Provider, externalUserID string, lastSeenPayload []byte) (IdentityResolution, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return IdentityResolution{}, errors.New("tenant_id is required")
	}
	var err error
	provider, err = normalizeProvider(provider)
	if err != nil {
		return IdentityResolution{}, err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return IdentityResolution{}, err
	}

	payload, err := normalizeJSONObj(json.RawMessage(lastSeenPayload))
	if err != nil {
		return IdentityResolution{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return IdentityResolution{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return IdentityResolution{}, err
	}

	var status string
	var personUUID string
	if err := tx.QueryRow(ctx, `
INSERT INTO person.external_identity_links (
  tenant_id,
  provider,
  external_user_id,
  status,
  person_uuid,
  first_seen_at,
  last_seen_at,
  seen_count,
  last_seen_payload,
  created_at,
  updated_at
)
VALUES (
  $1::uuid,
  $2::text,
  $3::text,
  'pending',
  NULL,
  now(),
  now(),
  1,
  $4::jsonb,
  now(),
  now()
)
ON CONFLICT (tenant_id, provider, external_user_id)
DO UPDATE SET
  last_seen_at = now(),
  seen_count = person.external_identity_links.seen_count + 1,
  last_seen_payload = EXCLUDED.last_seen_payload,
  updated_at = now()
RETURNING status, COALESCE(person_uuid::text, '')
`, tenantID, provider, externalUserID, []byte(payload)).Scan(&status, &personUUID); err != nil {
		return IdentityResolution{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return IdentityResolution{}, err
	}

	out := IdentityResolution{Status: IdentityStatus(status)}
	if strings.TrimSpace(personUUID) != "" {
		out.PersonUUID = &personUUID
	}
	return out, nil
}

func (s *PGStore) SubmitTimePunch(ctx context.Context, params SubmitTimePunchParams) (int64, error) {
	params.TenantID = strings.TrimSpace(params.TenantID)
	if params.TenantID == "" {
		return 0, errors.New("tenant_id is required")
	}
	params.PersonUUID = strings.TrimSpace(params.PersonUUID)
	if params.PersonUUID == "" {
		return 0, errors.New("person_uuid is required")
	}
	params.InitiatorID = strings.TrimSpace(params.InitiatorID)
	if params.InitiatorID == "" {
		return 0, errors.New("initiator_id is required")
	}
	params.RequestID = strings.TrimSpace(params.RequestID)
	if params.RequestID == "" {
		return 0, errors.New("request_id is required")
	}
	provider, err := normalizeProvider(params.SourceProvider)
	if err != nil {
		return 0, err
	}
	punchType := strings.ToUpper(strings.TrimSpace(params.PunchType))
	if punchType == "" {
		return 0, errors.New("punch_type is required")
	}

	payload, err := normalizeJSONObj(params.Payload)
	if err != nil {
		return 0, err
	}
	sourceRaw, err := normalizeJSONObj(params.SourceRawPayload)
	if err != nil {
		return 0, err
	}
	deviceInfo, err := normalizeJSONObj(params.DeviceInfo)
	if err != nil {
		return 0, err
	}

	punchTime := params.PunchTime.UTC()
	if punchTime.IsZero() {
		return 0, errors.New("punch_time is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, params.TenantID); err != nil {
		return 0, err
	}

	var eventDBID int64
	if err := tx.QueryRow(ctx, `
SELECT staffing.submit_time_punch_event(
  gen_random_uuid(),
  $1::uuid,
  $2::uuid,
  $3::timestamptz,
  $4::text,
  $5::text,
  $6::jsonb,
  $7::jsonb,
  $8::jsonb,
  $9::text,
  $10::uuid
)
`, params.TenantID, params.PersonUUID, punchTime, punchType, provider, []byte(payload), []byte(sourceRaw), []byte(deviceInfo), params.RequestID, params.InitiatorID).Scan(&eventDBID); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return eventDBID, nil
}
