-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS jobcatalog.job_profiles (
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_profiles_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_profiles_tenant_setid_code_key UNIQUE (tenant_id, setid, code),
  CONSTRAINT job_profiles_tenant_setid_id_unique UNIQUE (tenant_id, setid, id)
);

CREATE TABLE IF NOT EXISTS jobcatalog.job_profile_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_profile_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_profile_events_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_profile_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE','DISABLE')),
  CONSTRAINT job_profile_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT job_profile_events_one_per_day_unique UNIQUE (tenant_id, setid, job_profile_id, effective_date),
  CONSTRAINT job_profile_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT job_profile_events_profile_fk
    FOREIGN KEY (tenant_id, setid, job_profile_id) REFERENCES jobcatalog.job_profiles(tenant_id, setid, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS job_profile_events_tenant_effective_idx
  ON jobcatalog.job_profile_events (tenant_id, setid, job_profile_id, effective_date, id);

CREATE TABLE IF NOT EXISTS jobcatalog.job_profile_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_profile_id uuid NOT NULL,
  validity daterange NOT NULL,
  name text NOT NULL,
  description text NULL,
  is_active boolean NOT NULL DEFAULT true,
  external_refs jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_event_id bigint NOT NULL REFERENCES jobcatalog.job_profile_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_profile_versions_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_profile_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT job_profile_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT job_profile_versions_profile_fk
    FOREIGN KEY (tenant_id, setid, job_profile_id) REFERENCES jobcatalog.job_profiles(tenant_id, setid, id) ON DELETE RESTRICT,
  CONSTRAINT job_profile_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      setid gist_text_ops WITH =,
      job_profile_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS job_profile_versions_active_day_gist
  ON jobcatalog.job_profile_versions
  USING gist (tenant_id gist_uuid_ops, setid gist_text_ops, validity)
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS job_profile_versions_lookup_btree
  ON jobcatalog.job_profile_versions (tenant_id, setid, job_profile_id, lower(validity));

CREATE TABLE IF NOT EXISTS jobcatalog.job_profile_version_job_families (
  tenant_id uuid NOT NULL,
  setid text NOT NULL,
  job_profile_version_id bigint NOT NULL,
  job_family_id uuid NOT NULL,
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT job_profile_version_job_families_setid_format_check CHECK (setid ~ '^[A-Z0-9]{1,5}$'),
  CONSTRAINT job_profile_version_job_families_version_fk
    FOREIGN KEY (job_profile_version_id) REFERENCES jobcatalog.job_profile_versions(id) ON DELETE CASCADE,
  CONSTRAINT job_profile_version_job_families_family_fk
    FOREIGN KEY (tenant_id, setid, job_family_id) REFERENCES jobcatalog.job_families(tenant_id, setid, id) ON DELETE RESTRICT,
  CONSTRAINT job_profile_version_job_families_unique UNIQUE (tenant_id, setid, job_profile_version_id, job_family_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS job_profile_version_job_families_one_primary_unique
  ON jobcatalog.job_profile_version_job_families (tenant_id, setid, job_profile_version_id)
  WHERE is_primary = true;

CREATE INDEX IF NOT EXISTS job_profile_version_job_families_family_lookup_btree
  ON jobcatalog.job_profile_version_job_families (tenant_id, setid, job_family_id);

ALTER TABLE jobcatalog.job_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_profiles FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_profiles;
CREATE POLICY tenant_isolation ON jobcatalog.job_profiles
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_profile_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_profile_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_profile_events;
CREATE POLICY tenant_isolation ON jobcatalog.job_profile_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_profile_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_profile_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_profile_versions;
CREATE POLICY tenant_isolation ON jobcatalog.job_profile_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE jobcatalog.job_profile_version_job_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobcatalog.job_profile_version_job_families FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON jobcatalog.job_profile_version_job_families;
CREATE POLICY tenant_isolation ON jobcatalog.job_profile_version_job_families
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS jobcatalog.job_profile_version_job_families;
DROP TABLE IF EXISTS jobcatalog.job_profile_versions;
DROP TABLE IF EXISTS jobcatalog.job_profile_events;
DROP TABLE IF EXISTS jobcatalog.job_profiles;
-- +goose StatementEnd

