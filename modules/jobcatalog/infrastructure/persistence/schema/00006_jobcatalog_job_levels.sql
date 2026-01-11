CREATE TABLE IF NOT EXISTS jobcatalog.job_levels (
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_levels_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_levels_tenant_setid_code_key UNIQUE (tenant_id, setid, code),
  CONSTRAINT job_levels_tenant_setid_id_unique UNIQUE (tenant_id, setid, id)
);

CREATE TABLE IF NOT EXISTS jobcatalog.job_level_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_level_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_level_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_level_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE','DISABLE')),
  CONSTRAINT job_level_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT job_level_events_one_per_day_unique UNIQUE (tenant_id, setid, job_level_id, effective_date),
  CONSTRAINT job_level_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT job_level_events_level_fk
    FOREIGN KEY (tenant_id, setid, job_level_id) REFERENCES jobcatalog.job_levels(tenant_id, setid, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS job_level_events_tenant_effective_idx
  ON jobcatalog.job_level_events (tenant_id, setid, job_level_id, effective_date, id);

CREATE TABLE IF NOT EXISTS jobcatalog.job_level_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_level_id uuid NOT NULL,
  validity daterange NOT NULL,
  name text NOT NULL,
  description text NULL,
  is_active boolean NOT NULL DEFAULT true,
  external_refs jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_event_id bigint NOT NULL REFERENCES jobcatalog.job_level_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_level_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_level_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT job_level_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT job_level_versions_level_fk
    FOREIGN KEY (tenant_id, setid, job_level_id) REFERENCES jobcatalog.job_levels(tenant_id, setid, id) ON DELETE RESTRICT,
  CONSTRAINT job_level_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      setid gist_text_ops WITH =,
      job_level_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS job_level_versions_active_day_gist
  ON jobcatalog.job_level_versions
  USING gist (tenant_id gist_uuid_ops, setid gist_text_ops, validity)
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS job_level_versions_lookup_btree
  ON jobcatalog.job_level_versions (tenant_id, setid, job_level_id, lower(validity));

ALTER TABLE jobcatalog.job_levels ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_levels FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_levels;
CREATE POLICY tenant_isolation ON jobcatalog.job_levels
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_level_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_level_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_level_events;
CREATE POLICY tenant_isolation ON jobcatalog.job_level_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_level_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_level_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_level_versions;
CREATE POLICY tenant_isolation ON jobcatalog.job_level_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

