-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS jobcatalog.job_families (
  tenant_uuid uuid NOT NULL,
  setid text NOT NULL,
  job_family_uuid uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_family_code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_families_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_families_tenant_setid_code_key UNIQUE (tenant_uuid, job_family_code),
  CONSTRAINT job_families_tenant_setid_id_unique UNIQUE (tenant_uuid, setid, job_family_uuid)
);

CREATE TABLE IF NOT EXISTS jobcatalog.job_family_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_uuid uuid NOT NULL,
  setid text NOT NULL,
  job_family_uuid uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_code text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_family_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE','DISABLE')),
  CONSTRAINT job_family_events_event_uuid_unique UNIQUE (event_uuid),
  CONSTRAINT job_family_events_one_per_day_unique UNIQUE (tenant_uuid, setid, job_family_uuid, effective_date),
  CONSTRAINT job_family_events_request_code_unique UNIQUE (tenant_uuid, request_code),
  CONSTRAINT job_family_events_family_fk
    FOREIGN KEY (tenant_uuid, setid, job_family_uuid) REFERENCES jobcatalog.job_families(tenant_uuid, setid, job_family_uuid) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS job_family_events_tenant_effective_idx
  ON jobcatalog.job_family_events (tenant_uuid, setid, job_family_uuid, effective_date, id);

CREATE TABLE IF NOT EXISTS jobcatalog.job_family_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  setid text NOT NULL,
  job_family_uuid uuid NOT NULL,
  validity daterange NOT NULL,
  name text NOT NULL,
  description text NULL,
  is_active boolean NOT NULL DEFAULT true,
  external_refs jsonb NOT NULL DEFAULT '{}'::jsonb,
  job_family_group_uuid uuid NOT NULL,
  last_event_id bigint NOT NULL REFERENCES jobcatalog.job_family_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_family_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT job_family_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT job_family_versions_family_fk
    FOREIGN KEY (tenant_uuid, setid, job_family_uuid) REFERENCES jobcatalog.job_families(tenant_uuid, setid, job_family_uuid) ON DELETE RESTRICT,
  CONSTRAINT job_family_versions_group_fk
    FOREIGN KEY (tenant_uuid, setid, job_family_group_uuid) REFERENCES jobcatalog.job_family_groups(tenant_uuid, setid, job_family_group_uuid) ON DELETE RESTRICT,
  CONSTRAINT job_family_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      setid gist_text_ops WITH =,
      job_family_uuid gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS job_family_versions_active_day_gist
  ON jobcatalog.job_family_versions
  USING gist (tenant_uuid gist_uuid_ops, setid gist_text_ops, validity)
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS job_family_versions_lookup_btree
  ON jobcatalog.job_family_versions (tenant_uuid, setid, job_family_uuid, lower(validity));

ALTER TABLE jobcatalog.job_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_families FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_families;
CREATE POLICY tenant_isolation ON jobcatalog.job_families
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_family_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_family_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_family_events;
CREATE POLICY tenant_isolation ON jobcatalog.job_family_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_family_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_family_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_family_versions;
CREATE POLICY tenant_isolation ON jobcatalog.job_family_versions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS jobcatalog.job_family_versions;
DROP TABLE IF EXISTS jobcatalog.job_family_events;
DROP TABLE IF EXISTS jobcatalog.job_families;
-- +goose StatementEnd
