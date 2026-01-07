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

