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
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
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
CREATE INDEX IF NOT EXISTS tenant_domains_tenant_idx ON iam.tenant_domains (tenant_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS tenant_domains_primary_unique ON iam.tenant_domains (tenant_uuid) WHERE is_primary = true;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT USAGE ON SCHEMA iam TO superadmin_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.tenants TO superadmin_runtime';
    EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON iam.tenant_domains TO superadmin_runtime';
  END IF;
END
$$;
