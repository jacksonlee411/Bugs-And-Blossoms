CREATE TABLE IF NOT EXISTS orgunit.setid_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  event_type text NOT NULL,
  setid text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_events_event_type_check CHECK (event_type IN ('BOOTSTRAP','CREATE','RENAME','DISABLE')),
  CONSTRAINT setid_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$'),
  CONSTRAINT setid_events_share_forbidden CHECK (setid <> 'SHARE'),
  CONSTRAINT setid_events_request_id_unique UNIQUE (tenant_uuid, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS setid_events_event_id_unique ON orgunit.setid_events (event_uuid);
CREATE INDEX IF NOT EXISTS setid_events_tenant_time_idx ON orgunit.setid_events (tenant_uuid, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.setids (
  tenant_uuid uuid NOT NULL,
  setid text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  last_event_id bigint NOT NULL REFERENCES orgunit.setid_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, setid),
  CONSTRAINT setids_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$'),
  CONSTRAINT setids_share_forbidden CHECK (setid <> 'SHARE'),
  CONSTRAINT setids_status_check CHECK (status IN ('active','disabled')),
  CONSTRAINT setids_deflt_active_check CHECK (setid <> 'DEFLT' OR status = 'active')
);

CREATE TABLE IF NOT EXISTS orgunit.global_setid_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL DEFAULT orgunit.global_tenant_id(),
  event_type text NOT NULL,
  setid text NOT NULL DEFAULT 'SHARE',
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT global_setid_events_event_type_check CHECK (event_type IN ('BOOTSTRAP','CREATE','RENAME','DISABLE')),
  CONSTRAINT global_setid_events_setid_check CHECK (setid = 'SHARE'),
  CONSTRAINT global_setid_events_tenant_check CHECK (tenant_uuid = orgunit.global_tenant_id()),
  CONSTRAINT global_setid_events_request_id_unique UNIQUE (tenant_uuid, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS global_setid_events_event_id_unique ON orgunit.global_setid_events (event_uuid);
CREATE INDEX IF NOT EXISTS global_setid_events_tenant_time_idx ON orgunit.global_setid_events (tenant_uuid, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.global_setids (
  tenant_uuid uuid NOT NULL DEFAULT orgunit.global_tenant_id(),
  setid text NOT NULL DEFAULT 'SHARE',
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  last_event_id bigint NOT NULL REFERENCES orgunit.global_setid_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, setid),
  CONSTRAINT global_setids_share_only CHECK (setid = 'SHARE'),
  CONSTRAINT global_setids_tenant_check CHECK (tenant_uuid = orgunit.global_tenant_id()),
  CONSTRAINT global_setids_status_check CHECK (status = 'active')
);

CREATE TABLE IF NOT EXISTS orgunit.setid_binding_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_binding_events_event_type_check CHECK (event_type IN ('BIND')),
  CONSTRAINT setid_binding_events_event_id_unique UNIQUE (event_uuid),
  CONSTRAINT setid_binding_events_request_id_unique UNIQUE (tenant_uuid, request_id),
  CONSTRAINT setid_binding_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS setid_binding_events_tenant_effective_idx ON orgunit.setid_binding_events (tenant_uuid, org_id, effective_date, id);

CREATE TABLE IF NOT EXISTS orgunit.setid_binding_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  setid text NOT NULL,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES orgunit.setid_binding_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_binding_setid_format_check CHECK (setid ~ '^[A-Z0-9]{5}$'),
  CONSTRAINT setid_binding_setid_fk FOREIGN KEY (tenant_uuid, setid) REFERENCES orgunit.setids (tenant_uuid, setid),
  CONSTRAINT setid_binding_no_share CHECK (setid <> 'SHARE'),
  CONSTRAINT setid_binding_validity_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT setid_binding_no_overlap EXCLUDE USING gist (
    tenant_uuid WITH =,
    org_id gist_int4_ops WITH =,
    validity WITH &&
  )
);

ALTER TABLE orgunit.setid_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_events;
CREATE POLICY tenant_isolation ON orgunit.setid_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setids ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setids FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setids;
CREATE POLICY tenant_isolation ON orgunit.setids
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setid_binding_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_binding_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_binding_events;
CREATE POLICY tenant_isolation ON orgunit.setid_binding_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setid_binding_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_binding_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_binding_versions;
CREATE POLICY tenant_isolation ON orgunit.setid_binding_versions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.global_setid_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.global_setid_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS share_scope ON orgunit.global_setid_events;
CREATE POLICY share_scope ON orgunit.global_setid_events
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

ALTER TABLE orgunit.global_setids ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.global_setids FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS share_scope ON orgunit.global_setids;
CREATE POLICY share_scope ON orgunit.global_setids
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
