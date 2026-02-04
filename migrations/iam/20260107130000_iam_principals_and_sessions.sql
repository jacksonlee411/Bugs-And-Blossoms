-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS iam.principals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  email text NOT NULL,
  role_slug text NOT NULL,
  display_name text NULL,
  status text NOT NULL,
  kratos_identity_id uuid NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT principals_email_nonempty_check CHECK (btrim(email) <> ''),
  CONSTRAINT principals_email_lower_check CHECK (email = lower(email)),
  CONSTRAINT principals_email_trim_check CHECK (email = btrim(email)),
  CONSTRAINT principals_role_slug_nonempty_check CHECK (btrim(role_slug) <> ''),
  CONSTRAINT principals_role_slug_lower_check CHECK (role_slug = lower(role_slug)),
  CONSTRAINT principals_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS principals_tenant_email_unique ON iam.principals (tenant_uuid, email);
CREATE INDEX IF NOT EXISTS principals_tenant_idx ON iam.principals (tenant_uuid);

CREATE TABLE IF NOT EXISTS iam.sessions (
  token_sha256 bytea PRIMARY KEY,
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz NULL,
  ip text NULL,
  user_agent text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT sessions_token_sha256_len_check CHECK (octet_length(token_sha256) = 32)
);

CREATE INDEX IF NOT EXISTS sessions_tenant_idx ON iam.sessions (tenant_uuid);
CREATE INDEX IF NOT EXISTS sessions_principal_idx ON iam.sessions (principal_id);

CREATE TABLE IF NOT EXISTS iam.superadmin_principals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  display_name text NULL,
  status text NOT NULL,
  kratos_identity_id uuid NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT superadmin_principals_email_nonempty_check CHECK (btrim(email) <> ''),
  CONSTRAINT superadmin_principals_email_lower_check CHECK (email = lower(email)),
  CONSTRAINT superadmin_principals_email_trim_check CHECK (email = btrim(email)),
  CONSTRAINT superadmin_principals_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE TABLE IF NOT EXISTS iam.superadmin_sessions (
  token_sha256 bytea PRIMARY KEY,
  principal_id uuid NOT NULL REFERENCES iam.superadmin_principals(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz NULL,
  ip text NULL,
  user_agent text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT superadmin_sessions_token_sha256_len_check CHECK (octet_length(token_sha256) = 32)
);

CREATE INDEX IF NOT EXISTS superadmin_sessions_principal_idx ON iam.superadmin_sessions (principal_id);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.superadmin_principals TO superadmin_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.superadmin_sessions TO superadmin_runtime';
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.superadmin_sessions;
DROP TABLE IF EXISTS iam.superadmin_principals;
DROP TABLE IF EXISTS iam.sessions;
DROP TABLE IF EXISTS iam.principals;
-- +goose StatementEnd
