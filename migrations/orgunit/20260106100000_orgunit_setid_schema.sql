-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orgunit.setid_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  event_type text NOT NULL,
  setid text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT setid_events_event_type_check CHECK (event_type IN ('BOOTSTRAP','CREATE','RENAME','DISABLE')),
  CONSTRAINT setid_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT setid_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS setid_events_event_id_unique ON orgunit.setid_events (event_id);
CREATE INDEX IF NOT EXISTS setid_events_tenant_time_idx ON orgunit.setid_events (tenant_id, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.setids (
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  last_event_id bigint NOT NULL REFERENCES orgunit.setid_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, setid),
  CONSTRAINT setids_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT setids_status_check CHECK (status IN ('active','disabled')),
  CONSTRAINT setids_share_must_be_active CHECK (setid <> 'SHARE' OR status = 'active')
);

CREATE TABLE IF NOT EXISTS orgunit.business_unit_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  event_type text NOT NULL,
  business_unit_id text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT business_unit_events_event_type_check CHECK (event_type IN ('BOOTSTRAP','CREATE','RENAME','DISABLE')),
  CONSTRAINT business_unit_events_id_format_check CHECK (business_unit_id ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT business_unit_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS business_unit_events_event_id_unique ON orgunit.business_unit_events (event_id);
CREATE INDEX IF NOT EXISTS business_unit_events_tenant_time_idx ON orgunit.business_unit_events (tenant_id, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.business_units (
  tenant_id uuid NOT NULL,
  business_unit_id text NOT NULL,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  last_event_id bigint NOT NULL REFERENCES orgunit.business_unit_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, business_unit_id),
  CONSTRAINT business_units_id_format_check CHECK (business_unit_id ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT business_units_status_check CHECK (status IN ('active','disabled'))
);

CREATE TABLE IF NOT EXISTS orgunit.set_control_mapping_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  event_type text NOT NULL,
  record_group text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT set_control_mapping_events_event_type_check CHECK (event_type IN ('BOOTSTRAP','PUT')),
  CONSTRAINT set_control_mapping_events_record_group_check CHECK (record_group IN ('jobcatalog')),
  CONSTRAINT set_control_mapping_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS set_control_mapping_events_event_id_unique ON orgunit.set_control_mapping_events (event_id);
CREATE INDEX IF NOT EXISTS set_control_mapping_events_tenant_time_idx ON orgunit.set_control_mapping_events (tenant_id, transaction_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.set_control_mappings (
  tenant_id uuid NOT NULL,
  business_unit_id text NOT NULL,
  record_group text NOT NULL,
  setid text NOT NULL,
  last_event_id bigint NOT NULL REFERENCES orgunit.set_control_mapping_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, business_unit_id, record_group),
  CONSTRAINT set_control_mappings_record_group_check CHECK (record_group IN ('jobcatalog')),
  CONSTRAINT set_control_mappings_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT set_control_mappings_bu_format_check CHECK (business_unit_id ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT set_control_mappings_setid_fk FOREIGN KEY (tenant_id, setid) REFERENCES orgunit.setids (tenant_id, setid),
  CONSTRAINT set_control_mappings_bu_fk FOREIGN KEY (tenant_id, business_unit_id) REFERENCES orgunit.business_units (tenant_id, business_unit_id)
);

ALTER TABLE orgunit.setid_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setid_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setid_events;
CREATE POLICY tenant_isolation ON orgunit.setid_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.setids ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.setids FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.setids;
CREATE POLICY tenant_isolation ON orgunit.setids
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.business_unit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.business_unit_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.business_unit_events;
CREATE POLICY tenant_isolation ON orgunit.business_unit_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.business_units ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.business_units FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.business_units;
CREATE POLICY tenant_isolation ON orgunit.business_units
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.set_control_mapping_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.set_control_mapping_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.set_control_mapping_events;
CREATE POLICY tenant_isolation ON orgunit.set_control_mapping_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.set_control_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.set_control_mappings FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.set_control_mappings;
CREATE POLICY tenant_isolation ON orgunit.set_control_mappings
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS orgunit.set_control_mappings;
DROP TABLE IF EXISTS orgunit.set_control_mapping_events;
DROP TABLE IF EXISTS orgunit.business_units;
DROP TABLE IF EXISTS orgunit.business_unit_events;
DROP TABLE IF EXISTS orgunit.setids;
DROP TABLE IF EXISTS orgunit.setid_events;
-- +goose StatementEnd

