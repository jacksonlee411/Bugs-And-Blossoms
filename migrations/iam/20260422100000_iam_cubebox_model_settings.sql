-- +goose Up
CREATE TABLE IF NOT EXISTS iam.cubebox_model_providers (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  provider_id text NOT NULL,
  provider_type text NOT NULL,
  display_name text NOT NULL,
  base_url text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  created_by_principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE RESTRICT,
  updated_by_principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz NULL,
  PRIMARY KEY (tenant_uuid, provider_id),
  CONSTRAINT cubebox_model_providers_id_nonempty_check CHECK (btrim(provider_id) <> ''),
  CONSTRAINT cubebox_model_providers_type_nonempty_check CHECK (btrim(provider_type) <> ''),
  CONSTRAINT cubebox_model_providers_display_nonempty_check CHECK (btrim(display_name) <> ''),
  CONSTRAINT cubebox_model_providers_base_url_nonempty_check CHECK (btrim(base_url) <> ''),
  CONSTRAINT cubebox_model_providers_enabled_consistency_check CHECK (
    (enabled = true AND disabled_at IS NULL)
    OR (enabled = false AND disabled_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS cubebox_model_providers_tenant_updated_idx
  ON iam.cubebox_model_providers (tenant_uuid, updated_at DESC, provider_id DESC);

CREATE TABLE IF NOT EXISTS iam.cubebox_model_credentials (
  tenant_uuid uuid NOT NULL,
  credential_id text NOT NULL,
  provider_id text NOT NULL,
  secret_ref text NOT NULL,
  masked_secret text NOT NULL,
  version integer NOT NULL DEFAULT 1,
  active boolean NOT NULL DEFAULT true,
  created_by_principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz NULL,
  PRIMARY KEY (tenant_uuid, credential_id),
  CONSTRAINT cubebox_model_credentials_provider_fk FOREIGN KEY (tenant_uuid, provider_id)
    REFERENCES iam.cubebox_model_providers(tenant_uuid, provider_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_model_credentials_id_nonempty_check CHECK (btrim(credential_id) <> ''),
  CONSTRAINT cubebox_model_credentials_secret_ref_nonempty_check CHECK (btrim(secret_ref) <> ''),
  CONSTRAINT cubebox_model_credentials_masked_secret_nonempty_check CHECK (btrim(masked_secret) <> ''),
  CONSTRAINT cubebox_model_credentials_version_positive_check CHECK (version > 0),
  CONSTRAINT cubebox_model_credentials_active_consistency_check CHECK (
    (active = true AND disabled_at IS NULL)
    OR (active = false AND disabled_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS cubebox_model_credentials_provider_version_unique
  ON iam.cubebox_model_credentials (tenant_uuid, provider_id, version);

CREATE UNIQUE INDEX IF NOT EXISTS cubebox_model_credentials_provider_active_unique
  ON iam.cubebox_model_credentials (tenant_uuid, provider_id)
  WHERE active = true;

CREATE INDEX IF NOT EXISTS cubebox_model_credentials_tenant_provider_created_idx
  ON iam.cubebox_model_credentials (tenant_uuid, provider_id, created_at DESC, credential_id DESC);

CREATE TABLE IF NOT EXISTS iam.cubebox_model_selections (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  selection_id text NOT NULL,
  provider_id text NOT NULL,
  model_slug text NOT NULL,
  capability_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  selected_by_principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, selection_id),
  CONSTRAINT cubebox_model_selections_provider_fk FOREIGN KEY (tenant_uuid, provider_id)
    REFERENCES iam.cubebox_model_providers(tenant_uuid, provider_id) ON DELETE RESTRICT,
  CONSTRAINT cubebox_model_selections_id_singleton_check CHECK (selection_id = 'active'),
  CONSTRAINT cubebox_model_selections_model_slug_nonempty_check CHECK (btrim(model_slug) <> ''),
  CONSTRAINT cubebox_model_selections_capability_summary_object_check CHECK (jsonb_typeof(capability_summary) = 'object')
);

CREATE TABLE IF NOT EXISTS iam.cubebox_model_health_checks (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  health_check_id text NOT NULL,
  provider_id text NOT NULL,
  model_slug text NOT NULL,
  status text NOT NULL,
  latency_ms integer NULL,
  error_summary text NULL,
  validated_by_principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE RESTRICT,
  validated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, health_check_id),
  CONSTRAINT cubebox_model_health_checks_provider_fk FOREIGN KEY (tenant_uuid, provider_id)
    REFERENCES iam.cubebox_model_providers(tenant_uuid, provider_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_model_health_checks_id_nonempty_check CHECK (btrim(health_check_id) <> ''),
  CONSTRAINT cubebox_model_health_checks_model_slug_nonempty_check CHECK (btrim(model_slug) <> ''),
  CONSTRAINT cubebox_model_health_checks_status_check CHECK (status IN ('healthy', 'degraded', 'failed')),
  CONSTRAINT cubebox_model_health_checks_latency_nonnegative_check CHECK (
    latency_ms IS NULL OR latency_ms >= 0
  ),
  CONSTRAINT cubebox_model_health_checks_error_summary_nonempty_or_null_check CHECK (
    error_summary IS NULL OR btrim(error_summary) <> ''
  )
);

CREATE INDEX IF NOT EXISTS cubebox_model_health_checks_tenant_provider_validated_idx
  ON iam.cubebox_model_health_checks (tenant_uuid, provider_id, validated_at DESC, health_check_id DESC);

-- +goose Down
DROP INDEX IF EXISTS iam.cubebox_model_health_checks_tenant_provider_validated_idx;
DROP TABLE IF EXISTS iam.cubebox_model_health_checks;
DROP TABLE IF EXISTS iam.cubebox_model_selections;
DROP INDEX IF EXISTS iam.cubebox_model_credentials_tenant_provider_created_idx;
DROP INDEX IF EXISTS iam.cubebox_model_credentials_provider_active_unique;
DROP INDEX IF EXISTS iam.cubebox_model_credentials_provider_version_unique;
DROP TABLE IF EXISTS iam.cubebox_model_credentials;
DROP INDEX IF EXISTS iam.cubebox_model_providers_tenant_updated_idx;
DROP TABLE IF EXISTS iam.cubebox_model_providers;
