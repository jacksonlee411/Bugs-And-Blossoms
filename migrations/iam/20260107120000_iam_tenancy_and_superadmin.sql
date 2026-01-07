-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS iam.tenants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tenants_name_nonempty_check CHECK (btrim(name) <> '')
);

CREATE TABLE IF NOT EXISTS iam.tenant_domains (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  hostname text NOT NULL,
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tenant_domains_hostname_nonempty_check CHECK (hostname <> ''),
  CONSTRAINT tenant_domains_hostname_lower_check CHECK (hostname = lower(hostname)),
  CONSTRAINT tenant_domains_hostname_trim_check CHECK (hostname = btrim(hostname)),
  CONSTRAINT tenant_domains_hostname_no_port_check CHECK (position(':' in hostname) = 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS tenant_domains_hostname_unique ON iam.tenant_domains (hostname);
CREATE INDEX IF NOT EXISTS tenant_domains_tenant_idx ON iam.tenant_domains (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS tenant_domains_primary_unique ON iam.tenant_domains (tenant_id) WHERE is_primary = true;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT USAGE ON SCHEMA iam TO superadmin_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.tenants TO superadmin_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.tenant_domains TO superadmin_runtime';
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS iam.superadmin_audit_logs (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  actor text NOT NULL,
  action text NOT NULL,
  target_tenant_id uuid NULL REFERENCES iam.tenants(id) ON DELETE SET NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT superadmin_audit_logs_actor_nonempty_check CHECK (btrim(actor) <> ''),
  CONSTRAINT superadmin_audit_logs_action_nonempty_check CHECK (btrim(action) <> ''),
  CONSTRAINT superadmin_audit_logs_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS superadmin_audit_logs_event_id_unique ON iam.superadmin_audit_logs (event_id);
CREATE INDEX IF NOT EXISTS superadmin_audit_logs_target_tenant_idx ON iam.superadmin_audit_logs (target_tenant_id, id);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT INSERT, SELECT ON iam.superadmin_audit_logs TO superadmin_runtime';
    EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE iam.superadmin_audit_logs_id_seq TO superadmin_runtime';
  END IF;
END
$$;

INSERT INTO iam.tenants (id, name, is_active)
VALUES ('00000000-0000-0000-0000-000000000001', 'Local Tenant', true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO iam.tenant_domains (tenant_id, hostname, is_primary)
VALUES ('00000000-0000-0000-0000-000000000001', 'localhost', true)
ON CONFLICT (hostname) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.superadmin_audit_logs;
DROP TABLE IF EXISTS iam.tenant_domains;
DROP TABLE IF EXISTS iam.tenants;
-- +goose StatementEnd
