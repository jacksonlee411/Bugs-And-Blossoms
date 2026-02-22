CREATE OR REPLACE FUNCTION orgunit.scope_code_registry()
RETURNS TABLE(scope_code text, owner_module text, share_mode text, is_stable boolean) AS $$
  VALUES
    ('jobcatalog', 'jobcatalog', 'tenant-only', true),
    ('orgunit_geo_admin', 'orgunit', 'shared-only', true),
    ('orgunit_location', 'orgunit', 'shared-only', true),
    ('person_school', 'person', 'shared-only', true),
    ('person_education_type', 'person', 'shared-only', true),
    ('person_credential_type', 'person', 'shared-only', true)
$$ LANGUAGE SQL IMMUTABLE;

CREATE OR REPLACE FUNCTION orgunit.scope_code_is_valid(p_scope_code text)
RETURNS boolean AS $$
  SELECT EXISTS (
    SELECT 1 FROM orgunit.scope_code_registry() WHERE scope_code = p_scope_code
  );
$$ LANGUAGE SQL IMMUTABLE;

CREATE OR REPLACE FUNCTION orgunit.scope_code_share_mode(p_scope_code text)
RETURNS text AS $$
  SELECT share_mode FROM orgunit.scope_code_registry() WHERE scope_code = p_scope_code
$$ LANGUAGE SQL IMMUTABLE;

CREATE TABLE IF NOT EXISTS orgunit.setid_scope_packages (
  tenant_uuid uuid NOT NULL,
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  package_code text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_scope_packages_pk PRIMARY KEY (tenant_uuid, package_id),
  CONSTRAINT setid_scope_packages_code_unique UNIQUE (tenant_uuid, scope_code, package_code),
  CONSTRAINT setid_scope_packages_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT setid_scope_packages_code_check CHECK (package_code ~ '^[A-Z0-9_]{1,16}$'),
  CONSTRAINT setid_scope_packages_status_check CHECK (status IN ('active', 'disabled')),
  CONSTRAINT setid_scope_packages_deflt_active_check CHECK (package_code <> 'DEFLT' OR status = 'active')
);

CREATE TABLE IF NOT EXISTS orgunit.global_setid_scope_packages (
  tenant_uuid uuid NOT NULL DEFAULT orgunit.global_tenant_id(),
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  package_code text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT global_scope_packages_pk PRIMARY KEY (tenant_uuid, package_id),
  CONSTRAINT global_scope_packages_tenant_check CHECK (tenant_uuid = orgunit.global_tenant_id()),
  CONSTRAINT global_scope_packages_code_unique UNIQUE (tenant_uuid, scope_code, package_code),
  CONSTRAINT global_scope_packages_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT global_scope_packages_code_check CHECK (package_code ~ '^[A-Z0-9_]{1,16}$'),
  CONSTRAINT global_scope_packages_status_check CHECK (status IN ('active', 'disabled')),
  CONSTRAINT global_scope_packages_deflt_active_check CHECK (package_code <> 'DEFLT' OR status = 'active')
);

CREATE TABLE IF NOT EXISTS orgunit.setid_scope_package_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_scope_package_events_event_id_unique UNIQUE (event_uuid),
  CONSTRAINT setid_scope_package_events_request_id_unique UNIQUE (tenant_uuid, request_id),
  CONSTRAINT setid_scope_package_events_event_type_check CHECK (event_type IN ('BOOTSTRAP', 'CREATE', 'RENAME', 'DISABLE')),
  CONSTRAINT setid_scope_package_events_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT setid_scope_package_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS setid_scope_package_events_tenant_time_idx
  ON orgunit.setid_scope_package_events (tenant_uuid, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.global_setid_scope_package_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL DEFAULT orgunit.global_tenant_id(),
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT global_scope_package_events_event_id_unique UNIQUE (event_uuid),
  CONSTRAINT global_scope_package_events_request_id_unique UNIQUE (tenant_uuid, request_id),
  CONSTRAINT global_scope_package_events_event_type_check CHECK (event_type IN ('BOOTSTRAP', 'CREATE', 'RENAME', 'DISABLE')),
  CONSTRAINT global_scope_package_events_tenant_check CHECK (tenant_uuid = orgunit.global_tenant_id()),
  CONSTRAINT global_scope_package_events_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT global_scope_package_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS global_scope_package_events_tenant_time_idx
  ON orgunit.global_setid_scope_package_events (tenant_uuid, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.setid_scope_package_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  package_code text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES orgunit.setid_scope_package_events(id),
  CONSTRAINT setid_scope_package_versions_pkg_fk FOREIGN KEY (tenant_uuid, package_id)
    REFERENCES orgunit.setid_scope_packages (tenant_uuid, package_id),
  CONSTRAINT setid_scope_package_versions_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT setid_scope_package_versions_code_check CHECK (package_code ~ '^[A-Z0-9_]{1,16}$'),
  CONSTRAINT setid_scope_package_versions_status_check CHECK (status IN ('active', 'disabled')),
  CONSTRAINT setid_scope_package_versions_deflt_active_check CHECK (package_code <> 'DEFLT' OR status = 'active'),
  CONSTRAINT setid_scope_package_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT setid_scope_package_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT setid_scope_package_versions_no_overlap EXCLUDE USING gist (
    tenant_uuid WITH =,
    package_id WITH =,
    validity WITH &&
  )
);

CREATE INDEX IF NOT EXISTS setid_scope_package_versions_lookup_idx
  ON orgunit.setid_scope_package_versions (tenant_uuid, package_id, lower(validity));

CREATE TABLE IF NOT EXISTS orgunit.global_setid_scope_package_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL DEFAULT orgunit.global_tenant_id(),
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  package_code text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES orgunit.global_setid_scope_package_events(id),
  CONSTRAINT global_scope_package_versions_pkg_fk FOREIGN KEY (tenant_uuid, package_id)
    REFERENCES orgunit.global_setid_scope_packages (tenant_uuid, package_id),
  CONSTRAINT global_scope_package_versions_tenant_check CHECK (tenant_uuid = orgunit.global_tenant_id()),
  CONSTRAINT global_scope_package_versions_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT global_scope_package_versions_code_check CHECK (package_code ~ '^[A-Z0-9_]{1,16}$'),
  CONSTRAINT global_scope_package_versions_status_check CHECK (status IN ('active', 'disabled')),
  CONSTRAINT global_scope_package_versions_deflt_active_check CHECK (package_code <> 'DEFLT' OR status = 'active'),
  CONSTRAINT global_scope_package_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT global_scope_package_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT global_scope_package_versions_no_overlap EXCLUDE USING gist (
    tenant_uuid WITH =,
    package_id WITH =,
    validity WITH &&
  )
);

CREATE INDEX IF NOT EXISTS global_scope_package_versions_lookup_idx
  ON orgunit.global_setid_scope_package_versions (tenant_uuid, package_id, lower(validity));

CREATE TABLE IF NOT EXISTS orgunit.setid_scope_subscription_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  setid text NOT NULL,
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  package_owner_tenant_uuid uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_scope_subscription_events_event_id_unique UNIQUE (event_uuid),
  CONSTRAINT setid_scope_subscription_events_request_id_unique UNIQUE (tenant_uuid, request_id),
  CONSTRAINT setid_scope_subscription_events_event_type_check CHECK (event_type IN ('BOOTSTRAP', 'SUBSCRIBE')),
  CONSTRAINT setid_scope_subscription_events_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT setid_scope_subscription_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$'),
  CONSTRAINT setid_scope_subscription_events_owner_check CHECK (
    package_owner_tenant_uuid = tenant_uuid OR package_owner_tenant_uuid = orgunit.global_tenant_id()
  ),
  CONSTRAINT setid_scope_subscription_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS setid_scope_subscription_events_tenant_idx
  ON orgunit.setid_scope_subscription_events (tenant_uuid, setid, scope_code, effective_date, id);

CREATE TABLE IF NOT EXISTS orgunit.setid_scope_subscriptions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  setid text NOT NULL,
  scope_code text NOT NULL,
  package_id uuid NOT NULL,
  package_owner_tenant_uuid uuid NOT NULL,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES orgunit.setid_scope_subscription_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_scope_subscriptions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$'),
  CONSTRAINT setid_scope_subscriptions_scope_code_check CHECK (orgunit.scope_code_is_valid(scope_code)),
  CONSTRAINT setid_scope_subscriptions_setid_fk FOREIGN KEY (tenant_uuid, setid) REFERENCES orgunit.setids (tenant_uuid, setid),
  CONSTRAINT setid_scope_subscriptions_validity_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT setid_scope_subscriptions_owner_check CHECK (
    package_owner_tenant_uuid = tenant_uuid OR package_owner_tenant_uuid = orgunit.global_tenant_id()
  ),
  CONSTRAINT setid_scope_subscriptions_no_overlap EXCLUDE USING gist (
    tenant_uuid WITH =,
    setid WITH =,
    scope_code WITH =,
    validity WITH &&
  )
);

CREATE INDEX IF NOT EXISTS setid_scope_subscriptions_lookup_idx
  ON orgunit.setid_scope_subscriptions (tenant_uuid, setid, scope_code, lower(validity));

ALTER TABLE orgunit.setid_scope_packages ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_scope_packages FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_scope_packages;
CREATE POLICY tenant_isolation ON orgunit.setid_scope_packages
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setid_scope_package_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_scope_package_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_scope_package_events;
CREATE POLICY tenant_isolation ON orgunit.setid_scope_package_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setid_scope_package_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_scope_package_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_scope_package_versions;
CREATE POLICY tenant_isolation ON orgunit.setid_scope_package_versions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setid_scope_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_scope_subscriptions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_scope_subscriptions;
CREATE POLICY tenant_isolation ON orgunit.setid_scope_subscriptions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setid_scope_subscription_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_scope_subscription_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_scope_subscription_events;
CREATE POLICY tenant_isolation ON orgunit.setid_scope_subscription_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.global_setid_scope_packages ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.global_setid_scope_packages FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS share_scope ON orgunit.global_setid_scope_packages;
CREATE POLICY share_scope ON orgunit.global_setid_scope_packages
USING (
  tenant_uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_tenant')::uuid = orgunit.global_tenant_id()
  AND current_setting('app.allow_share_read', true) = 'on'
)
WITH CHECK (
  tenant_uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_tenant')::uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_actor_scope', true) = 'saas'
);

ALTER TABLE orgunit.global_setid_scope_package_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.global_setid_scope_package_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS share_scope ON orgunit.global_setid_scope_package_events;
CREATE POLICY share_scope ON orgunit.global_setid_scope_package_events
USING (
  tenant_uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_tenant')::uuid = orgunit.global_tenant_id()
  AND current_setting('app.allow_share_read', true) = 'on'
)
WITH CHECK (
  tenant_uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_tenant')::uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_actor_scope', true) = 'saas'
);

ALTER TABLE orgunit.global_setid_scope_package_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.global_setid_scope_package_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS share_scope ON orgunit.global_setid_scope_package_versions;
CREATE POLICY share_scope ON orgunit.global_setid_scope_package_versions
USING (
  tenant_uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_tenant')::uuid = orgunit.global_tenant_id()
  AND current_setting('app.allow_share_read', true) = 'on'
)
WITH CHECK (
  tenant_uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_tenant')::uuid = orgunit.global_tenant_id()
  AND current_setting('app.current_actor_scope', true) = 'saas'
);
