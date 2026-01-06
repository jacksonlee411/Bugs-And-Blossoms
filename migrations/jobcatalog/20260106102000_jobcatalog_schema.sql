-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE SCHEMA IF NOT EXISTS jobcatalog;

CREATE TABLE IF NOT EXISTS jobcatalog.job_family_groups (
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_groups_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_family_groups_tenant_setid_code_key UNIQUE (tenant_id, setid, code)
);

CREATE TABLE IF NOT EXISTS jobcatalog.job_family_group_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_family_group_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_group_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_family_group_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE','DISABLE')),
  CONSTRAINT job_family_group_events_one_per_day_unique UNIQUE (tenant_id, setid, job_family_group_id, effective_date),
  CONSTRAINT job_family_group_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS job_family_group_events_event_id_unique ON jobcatalog.job_family_group_events (event_id);
CREATE INDEX IF NOT EXISTS job_family_group_events_tenant_effective_idx
  ON jobcatalog.job_family_group_events (tenant_id, setid, job_family_group_id, effective_date, id);

CREATE TABLE IF NOT EXISTS jobcatalog.job_family_group_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_family_group_id uuid NOT NULL REFERENCES jobcatalog.job_family_groups(id) ON DELETE CASCADE,
  validity daterange NOT NULL,
  name text NOT NULL,
  description text NULL,
  is_active boolean NOT NULL DEFAULT true,
  external_refs jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_event_id bigint NOT NULL REFERENCES jobcatalog.job_family_group_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_family_group_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_family_group_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT job_family_group_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT job_family_group_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      setid gist_text_ops WITH =,
      job_family_group_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS job_family_group_versions_active_day_gist
  ON jobcatalog.job_family_group_versions
  USING gist (tenant_id gist_uuid_ops, setid gist_text_ops, validity)
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS job_family_group_versions_lookup_btree
  ON jobcatalog.job_family_group_versions (tenant_id, setid, job_family_group_id, lower(validity));

ALTER TABLE jobcatalog.job_family_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_family_groups FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_family_groups;
CREATE POLICY tenant_isolation ON jobcatalog.job_family_groups
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_family_group_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_family_group_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_family_group_events;
CREATE POLICY tenant_isolation ON jobcatalog.job_family_group_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_family_group_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_family_group_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_family_group_versions;
CREATE POLICY tenant_isolation ON jobcatalog.job_family_group_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS jobcatalog.job_family_group_versions;
DROP TABLE IF EXISTS jobcatalog.job_family_group_events;
DROP TABLE IF EXISTS jobcatalog.job_family_groups;
DROP SCHEMA IF EXISTS jobcatalog;
-- +goose StatementEnd
