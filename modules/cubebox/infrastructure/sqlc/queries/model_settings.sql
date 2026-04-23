-- name: ListModelProviders :many
SELECT
  tenant_uuid,
  provider_id,
  provider_type,
  display_name,
  base_url,
  enabled,
  created_by_principal_id,
  updated_by_principal_id,
  created_at,
  updated_at,
  disabled_at
FROM iam.cubebox_model_providers
WHERE tenant_uuid = $1::uuid
ORDER BY updated_at DESC, provider_id DESC;

-- name: GetModelProvider :one
SELECT
  tenant_uuid,
  provider_id,
  provider_type,
  display_name,
  base_url,
  enabled,
  created_by_principal_id,
  updated_by_principal_id,
  created_at,
  updated_at,
  disabled_at
FROM iam.cubebox_model_providers
WHERE tenant_uuid = $1::uuid
  AND provider_id = $2;

-- name: UpsertModelProvider :one
INSERT INTO iam.cubebox_model_providers (
  tenant_uuid,
  provider_id,
  provider_type,
  display_name,
  base_url,
  enabled,
  created_by_principal_id,
  updated_by_principal_id,
  created_at,
  updated_at,
  disabled_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7::uuid,
  $8::uuid,
  $9,
  $10,
  $11
)
ON CONFLICT (tenant_uuid, provider_id) DO UPDATE
SET
  provider_type = EXCLUDED.provider_type,
  display_name = EXCLUDED.display_name,
  base_url = EXCLUDED.base_url,
  enabled = EXCLUDED.enabled,
  updated_by_principal_id = EXCLUDED.updated_by_principal_id,
  updated_at = EXCLUDED.updated_at,
  disabled_at = EXCLUDED.disabled_at
RETURNING
  tenant_uuid,
  provider_id,
  provider_type,
  display_name,
  base_url,
  enabled,
  created_by_principal_id,
  updated_by_principal_id,
  created_at,
  updated_at,
  disabled_at;

-- name: InsertModelCredential :one
INSERT INTO iam.cubebox_model_credentials (
  tenant_uuid,
  credential_id,
  provider_id,
  secret_ref,
  masked_secret,
  version,
  active,
  created_by_principal_id,
  created_at,
  disabled_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8::uuid,
  $9,
  $10
)
RETURNING
  tenant_uuid,
  credential_id,
  provider_id,
  secret_ref,
  masked_secret,
  version,
  active,
  created_by_principal_id,
  created_at,
  disabled_at;

-- name: DeactivateProviderCredentials :exec
UPDATE iam.cubebox_model_credentials
SET
  active = false,
  disabled_at = $3
WHERE tenant_uuid = $1::uuid
  AND provider_id = $2
  AND active = true;

-- name: ListModelCredentials :many
SELECT
  tenant_uuid,
  credential_id,
  provider_id,
  secret_ref,
  masked_secret,
  version,
  active,
  created_by_principal_id,
  created_at,
  disabled_at
FROM iam.cubebox_model_credentials
WHERE tenant_uuid = $1::uuid
ORDER BY created_at DESC, credential_id DESC;

-- name: ListModelCredentialsByProvider :many
SELECT
  tenant_uuid,
  credential_id,
  provider_id,
  secret_ref,
  masked_secret,
  version,
  active,
  created_by_principal_id,
  created_at,
  disabled_at
FROM iam.cubebox_model_credentials
WHERE tenant_uuid = $1::uuid
  AND provider_id = $2
ORDER BY created_at DESC, credential_id DESC;

-- name: DeactivateCredential :one
UPDATE iam.cubebox_model_credentials
SET
  active = false,
  disabled_at = $3
WHERE tenant_uuid = $1::uuid
  AND credential_id = $2
RETURNING
  tenant_uuid,
  credential_id,
  provider_id,
  secret_ref,
  masked_secret,
  version,
  active,
  created_by_principal_id,
  created_at,
  disabled_at;

-- name: UpsertModelSelection :one
INSERT INTO iam.cubebox_model_selections (
  tenant_uuid,
  selection_id,
  provider_id,
  model_slug,
  capability_summary,
  selected_by_principal_id,
  created_at,
  updated_at
) VALUES (
  $1::uuid,
  'active',
  $2,
  $3,
  $4,
  $5::uuid,
  $6,
  $7
)
ON CONFLICT (tenant_uuid, selection_id) DO UPDATE
SET
  provider_id = EXCLUDED.provider_id,
  model_slug = EXCLUDED.model_slug,
  capability_summary = EXCLUDED.capability_summary,
  selected_by_principal_id = EXCLUDED.selected_by_principal_id,
  updated_at = EXCLUDED.updated_at
RETURNING
  tenant_uuid,
  selection_id,
  provider_id,
  model_slug,
  capability_summary,
  selected_by_principal_id,
  created_at,
  updated_at;

-- name: GetActiveModelSelection :one
SELECT
  tenant_uuid,
  selection_id,
  provider_id,
  model_slug,
  capability_summary,
  selected_by_principal_id,
  created_at,
  updated_at
FROM iam.cubebox_model_selections
WHERE tenant_uuid = $1::uuid
  AND selection_id = 'active';

-- name: InsertModelHealthCheck :one
INSERT INTO iam.cubebox_model_health_checks (
  tenant_uuid,
  health_check_id,
  provider_id,
  model_slug,
  status,
  latency_ms,
  error_summary,
  validated_by_principal_id,
  validated_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8::uuid,
  $9
)
RETURNING
  tenant_uuid,
  health_check_id,
  provider_id,
  model_slug,
  status,
  latency_ms,
  error_summary,
  validated_by_principal_id,
  validated_at;

-- name: ListModelHealthChecks :many
SELECT
  tenant_uuid,
  health_check_id,
  provider_id,
  model_slug,
  status,
  latency_ms,
  error_summary,
  validated_by_principal_id,
  validated_at
FROM iam.cubebox_model_health_checks
WHERE tenant_uuid = $1::uuid
ORDER BY validated_at DESC, health_check_id DESC;

-- name: GetLatestModelHealthCheckByProviderAndModel :one
SELECT
  tenant_uuid,
  health_check_id,
  provider_id,
  model_slug,
  status,
  latency_ms,
  error_summary,
  validated_by_principal_id,
  validated_at
FROM iam.cubebox_model_health_checks
WHERE tenant_uuid = $1::uuid
  AND provider_id = $2
  AND model_slug = $3
ORDER BY validated_at DESC, health_check_id DESC
LIMIT 1;
