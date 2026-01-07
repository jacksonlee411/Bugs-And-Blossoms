CREATE TABLE IF NOT EXISTS iam.principals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
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

CREATE UNIQUE INDEX IF NOT EXISTS principals_tenant_email_unique ON iam.principals (tenant_id, email);
CREATE INDEX IF NOT EXISTS principals_tenant_idx ON iam.principals (tenant_id);

CREATE TABLE IF NOT EXISTS iam.sessions (
  token_sha256 bytea PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  principal_id uuid NOT NULL REFERENCES iam.principals(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz NULL,
  ip text NULL,
  user_agent text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT sessions_token_sha256_len_check CHECK (octet_length(token_sha256) = 32)
);

CREATE INDEX IF NOT EXISTS sessions_tenant_idx ON iam.sessions (tenant_id);
CREATE INDEX IF NOT EXISTS sessions_principal_idx ON iam.sessions (principal_id);

