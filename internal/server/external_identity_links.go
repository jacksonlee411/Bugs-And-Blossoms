package server

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

func normalizeExternalIdentityProvider(raw string) (string, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" {
		return "", errors.New("provider is required")
	}
	switch raw {
	case "DINGTALK", "WECOM":
		return raw, nil
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

func (s *personPGStore) ListExternalIdentityLinks(ctx context.Context, tenantID string, limit int) ([]ExternalIdentityLink, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	rows, err := tx.Query(ctx, `
SELECT
  provider,
  external_user_id,
  status,
  COALESCE(person_uuid::text, ''),
  first_seen_at,
  last_seen_at,
  seen_count,
  last_seen_payload,
  created_at,
  updated_at
FROM person.external_identity_links
WHERE tenant_id = $1::uuid
ORDER BY last_seen_at DESC, provider ASC, external_user_id ASC
LIMIT $2::int
`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExternalIdentityLink
	for rows.Next() {
		var l ExternalIdentityLink
		var personUUID string
		var payload []byte
		if err := rows.Scan(
			&l.Provider,
			&l.ExternalUserID,
			&l.Status,
			&personUUID,
			&l.FirstSeenAt,
			&l.LastSeenAt,
			&l.SeenCount,
			&payload,
			&l.CreatedAt,
			&l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		l.TenantID = tenantID
		if personUUID != "" {
			l.PersonUUID = &personUUID
		}
		l.FirstSeenAt = l.FirstSeenAt.UTC()
		l.LastSeenAt = l.LastSeenAt.UTC()
		l.CreatedAt = l.CreatedAt.UTC()
		l.UpdatedAt = l.UpdatedAt.UTC()
		l.LastSeenPayload = json.RawMessage(payload)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *personPGStore) LinkExternalIdentity(ctx context.Context, tenantID string, provider string, externalUserID string, personUUID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	provider, err = normalizeExternalIdentityProvider(provider)
	if err != nil {
		return err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return err
	}
	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return errors.New("person_uuid is required")
	}

	if _, err := tx.Exec(ctx, `
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
  'active',
  $4::uuid,
  now(),
  now(),
  1,
  '{}'::jsonb,
  now(),
  now()
)
ON CONFLICT (tenant_id, provider, external_user_id)
DO UPDATE SET
  status = 'active',
  person_uuid = EXCLUDED.person_uuid,
  updated_at = now()
`, tenantID, provider, externalUserID, personUUID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *personPGStore) DisableExternalIdentity(ctx context.Context, tenantID string, provider string, externalUserID string) error {
	return s.updateExternalIdentityStatus(ctx, tenantID, provider, externalUserID, "disabled")
}

func (s *personPGStore) EnableExternalIdentity(ctx context.Context, tenantID string, provider string, externalUserID string) error {
	return s.updateExternalIdentityStatus(ctx, tenantID, provider, externalUserID, "active")
}

func (s *personPGStore) IgnoreExternalIdentity(ctx context.Context, tenantID string, provider string, externalUserID string) error {
	return s.updateExternalIdentityStatus(ctx, tenantID, provider, externalUserID, "ignored")
}

func (s *personPGStore) UnignoreExternalIdentity(ctx context.Context, tenantID string, provider string, externalUserID string) error {
	return s.updateExternalIdentityStatus(ctx, tenantID, provider, externalUserID, "pending")
}

func (s *personPGStore) UnlinkExternalIdentity(ctx context.Context, tenantID string, provider string, externalUserID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	provider, err = normalizeExternalIdentityProvider(provider)
	if err != nil {
		return err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
UPDATE person.external_identity_links
SET status = 'pending', person_uuid = NULL, updated_at = now()
WHERE tenant_id = $1::uuid
  AND provider = $2::text
  AND external_user_id = $3::text
  AND status IN ('active', 'disabled')
`, tenantID, provider, externalUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("identity link not found (or not linkable)")
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *personPGStore) updateExternalIdentityStatus(ctx context.Context, tenantID string, provider string, externalUserID string, status string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	provider, err = normalizeExternalIdentityProvider(provider)
	if err != nil {
		return err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return err
	}

	var allowFrom []string
	var setStatus string
	switch status {
	case "disabled":
		allowFrom = []string{"active", "disabled"}
		setStatus = "disabled"
	case "active":
		allowFrom = []string{"active", "disabled"}
		setStatus = "active"
	case "ignored":
		allowFrom = []string{"pending", "ignored"}
		setStatus = "ignored"
	case "pending":
		allowFrom = []string{"pending", "ignored"}
		setStatus = "pending"
	default:
		return errors.New("invalid status")
	}

	tag, err := tx.Exec(ctx, `
UPDATE person.external_identity_links
SET status = $4::text, updated_at = now()
WHERE tenant_id = $1::uuid
  AND provider = $2::text
  AND external_user_id = $3::text
  AND status = ANY ($5::text[])
`, tenantID, provider, externalUserID, setStatus, allowFrom)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("identity link not found (or invalid state)")
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *personMemoryStore) externalIdentityKey(tenantID string, provider string, externalUserID string) string {
	return tenantID + "|" + provider + "|" + externalUserID
}

func (s *personMemoryStore) ListExternalIdentityLinks(_ context.Context, tenantID string, limit int) ([]ExternalIdentityLink, error) {
	var out []ExternalIdentityLink
	for _, v := range s.links {
		if v.TenantID == tenantID {
			out = append(out, v)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].LastSeenAt.Equal(out[j].LastSeenAt) {
			if out[i].Provider == out[j].Provider {
				return out[i].ExternalUserID < out[j].ExternalUserID
			}
			return out[i].Provider < out[j].Provider
		}
		return out[i].LastSeenAt.After(out[j].LastSeenAt)
	})

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *personMemoryStore) LinkExternalIdentity(_ context.Context, tenantID string, provider string, externalUserID string, personUUID string) error {
	provider, err := normalizeExternalIdentityProvider(provider)
	if err != nil {
		return err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return err
	}
	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return errors.New("person_uuid is required")
	}

	now := time.Now().UTC()
	k := s.externalIdentityKey(tenantID, provider, externalUserID)
	existing, ok := s.links[k]
	if !ok {
		existing = ExternalIdentityLink{
			TenantID:        tenantID,
			Provider:        provider,
			ExternalUserID:  externalUserID,
			FirstSeenAt:     now,
			CreatedAt:       now,
			LastSeenPayload: json.RawMessage(`{}`),
		}
	}
	existing.Status = "active"
	existing.PersonUUID = &personUUID
	existing.LastSeenAt = now
	existing.UpdatedAt = now
	existing.SeenCount++
	s.links[k] = existing
	return nil
}

func (s *personMemoryStore) DisableExternalIdentity(_ context.Context, tenantID string, provider string, externalUserID string) error {
	return s.setExternalIdentityStatus(tenantID, provider, externalUserID, "disabled", []string{"active", "disabled"})
}

func (s *personMemoryStore) EnableExternalIdentity(_ context.Context, tenantID string, provider string, externalUserID string) error {
	return s.setExternalIdentityStatus(tenantID, provider, externalUserID, "active", []string{"active", "disabled"})
}

func (s *personMemoryStore) IgnoreExternalIdentity(_ context.Context, tenantID string, provider string, externalUserID string) error {
	return s.setExternalIdentityStatus(tenantID, provider, externalUserID, "ignored", []string{"pending", "ignored"})
}

func (s *personMemoryStore) UnignoreExternalIdentity(_ context.Context, tenantID string, provider string, externalUserID string) error {
	return s.setExternalIdentityStatus(tenantID, provider, externalUserID, "pending", []string{"pending", "ignored"})
}

func (s *personMemoryStore) UnlinkExternalIdentity(_ context.Context, tenantID string, provider string, externalUserID string) error {
	provider, err := normalizeExternalIdentityProvider(provider)
	if err != nil {
		return err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return err
	}

	k := s.externalIdentityKey(tenantID, provider, externalUserID)
	existing, ok := s.links[k]
	if !ok {
		return errors.New("identity link not found")
	}
	if existing.Status != "active" && existing.Status != "disabled" {
		return errors.New("identity link not found (or not linkable)")
	}
	now := time.Now().UTC()
	existing.Status = "pending"
	existing.PersonUUID = nil
	existing.UpdatedAt = now
	s.links[k] = existing
	return nil
}

func (s *personMemoryStore) setExternalIdentityStatus(tenantID string, provider string, externalUserID string, status string, allowFrom []string) error {
	provider, err := normalizeExternalIdentityProvider(provider)
	if err != nil {
		return err
	}
	externalUserID, err = normalizeExternalUserID(externalUserID)
	if err != nil {
		return err
	}

	k := s.externalIdentityKey(tenantID, provider, externalUserID)
	existing, ok := s.links[k]
	if !ok {
		return errors.New("identity link not found")
	}
	allowed := false
	for _, st := range allowFrom {
		if existing.Status == st {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("identity link not found (or invalid state)")
	}

	now := time.Now().UTC()
	existing.Status = status
	existing.UpdatedAt = now
	s.links[k] = existing
	return nil
}
