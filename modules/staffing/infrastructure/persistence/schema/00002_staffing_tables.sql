CREATE TABLE IF NOT EXISTS staffing.positions (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS staffing.position_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  position_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT position_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT position_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT position_events_payload_allowed_keys_check CHECK (
    (
      payload
      - 'org_unit_id'
      - 'name'
      - 'reports_to_position_id'
      - 'business_unit_id'
      - 'job_profile_id'
      - 'lifecycle_status'
      - 'capacity_fte'
    ) = '{}'::jsonb
  ),
  CONSTRAINT position_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT position_events_one_per_day_unique UNIQUE (tenant_id, position_id, effective_date),
  CONSTRAINT position_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT position_events_position_fk FOREIGN KEY (tenant_id, position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS position_events_tenant_position_effective_idx
  ON staffing.position_events (tenant_id, position_id, effective_date, id);

CREATE TABLE IF NOT EXISTS staffing.position_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  position_id uuid NOT NULL,
  org_unit_id uuid NOT NULL,
  reports_to_position_id uuid NULL,
  name text NULL,
  lifecycle_status text NOT NULL DEFAULT 'active',
  capacity_fte numeric(9,2) NOT NULL DEFAULT 1.0,
  profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.position_events(id),
  CONSTRAINT position_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT position_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT position_versions_capacity_fte_check CHECK (capacity_fte > 0),
  CONSTRAINT position_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object'),
  CONSTRAINT position_versions_lifecycle_status_check CHECK (lifecycle_status IN ('active','disabled')),
  CONSTRAINT position_versions_position_fk FOREIGN KEY (tenant_id, position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT position_versions_reports_to_fk FOREIGN KEY (tenant_id, reports_to_position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT position_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      position_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS position_versions_lookup_btree
  ON staffing.position_versions (tenant_id, position_id, lower(validity));

ALTER TABLE staffing.position_versions
  ADD COLUMN IF NOT EXISTS business_unit_id text NOT NULL,
  ADD COLUMN IF NOT EXISTS jobcatalog_setid text NULL,
  ADD COLUMN IF NOT EXISTS job_profile_id uuid NULL;

ALTER TABLE staffing.position_versions
  DROP CONSTRAINT IF EXISTS position_versions_business_unit_id_format_check,
  DROP CONSTRAINT IF EXISTS position_versions_jobcatalog_setid_format_check,
  DROP CONSTRAINT IF EXISTS position_versions_jobcatalog_setid_requires_bu_check,
  DROP CONSTRAINT IF EXISTS position_versions_job_profile_requires_setid_check,
  DROP CONSTRAINT IF EXISTS position_versions_job_profile_fk;

ALTER TABLE staffing.position_versions
  ADD CONSTRAINT position_versions_business_unit_id_format_check CHECK (business_unit_id ~ '^[A-Z0-9]{1,5}$'),
  ADD CONSTRAINT position_versions_jobcatalog_setid_format_check CHECK (jobcatalog_setid IS NULL OR jobcatalog_setid ~ '^[A-Z0-9]{1,5}$'),
  ADD CONSTRAINT position_versions_jobcatalog_setid_requires_bu_check CHECK (jobcatalog_setid IS NULL OR business_unit_id IS NOT NULL),
  ADD CONSTRAINT position_versions_job_profile_requires_setid_check CHECK (job_profile_id IS NULL OR jobcatalog_setid IS NOT NULL);

CREATE TABLE IF NOT EXISTS staffing.assignments (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  person_uuid uuid NOT NULL,
  assignment_type text NOT NULL DEFAULT 'primary',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT assignments_assignment_type_check CHECK (assignment_type IN ('primary')),
  CONSTRAINT assignments_tenant_person_type_unique UNIQUE (tenant_id, person_uuid, assignment_type)
);

CREATE TABLE IF NOT EXISTS staffing.assignment_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_type text NOT NULL DEFAULT 'primary',
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assignment_events_assignment_type_check CHECK (assignment_type IN ('primary')),
  CONSTRAINT assignment_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT assignment_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT assignment_events_payload_allowed_keys_check CHECK (
    (
      payload
      - 'position_id'
      - 'status'
      - 'allocated_fte'
      - 'profile'
    ) = '{}'::jsonb
  ),
  CONSTRAINT assignment_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT assignment_events_one_per_day_unique UNIQUE (tenant_id, assignment_id, effective_date),
  CONSTRAINT assignment_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT assignment_events_assignment_fk FOREIGN KEY (tenant_id, assignment_id) REFERENCES staffing.assignments(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS assignment_events_tenant_assignment_effective_idx
  ON staffing.assignment_events (tenant_id, assignment_id, effective_date, id);

CREATE TABLE IF NOT EXISTS staffing.assignment_event_corrections (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  target_effective_date date NOT NULL,
  replacement_payload jsonb NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assignment_event_corrections_replacement_payload_obj_check CHECK (jsonb_typeof(replacement_payload) = 'object'),
  CONSTRAINT assignment_event_corrections_event_id_unique UNIQUE (event_id),
  CONSTRAINT assignment_event_corrections_target_unique UNIQUE (tenant_id, assignment_id, target_effective_date),
  CONSTRAINT assignment_event_corrections_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE TABLE IF NOT EXISTS staffing.assignment_event_rescinds (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  target_effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assignment_event_rescinds_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT assignment_event_rescinds_event_id_unique UNIQUE (event_id),
  CONSTRAINT assignment_event_rescinds_target_unique UNIQUE (tenant_id, assignment_id, target_effective_date),
  CONSTRAINT assignment_event_rescinds_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE TABLE IF NOT EXISTS staffing.assignment_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  position_id uuid NOT NULL,
  assignment_type text NOT NULL DEFAULT 'primary',
  status text NOT NULL DEFAULT 'active',
  allocated_fte numeric(9,2) NOT NULL DEFAULT 1.0,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.assignment_events(id),
  CONSTRAINT assignment_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT assignment_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT assignment_versions_allocated_fte_check CHECK (allocated_fte > 0),
  CONSTRAINT assignment_versions_status_check CHECK (status IN ('active','inactive')),
  CONSTRAINT assignment_versions_assignment_type_check CHECK (assignment_type IN ('primary')),
  CONSTRAINT assignment_versions_assignment_fk FOREIGN KEY (tenant_id, assignment_id) REFERENCES staffing.assignments(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT assignment_versions_position_fk FOREIGN KEY (tenant_id, position_id) REFERENCES staffing.positions(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT assignment_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      assignment_id gist_uuid_ops WITH =,
      validity WITH &&
    ),
  CONSTRAINT assignment_versions_position_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      position_id gist_uuid_ops WITH =,
      validity WITH &&
    )
    WHERE (status = 'active')
);

CREATE INDEX IF NOT EXISTS assignment_versions_person_lookup_btree
  ON staffing.assignment_versions (tenant_id, person_uuid, lower(validity));

ALTER TABLE staffing.assignment_versions
  ADD COLUMN IF NOT EXISTS profile jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE staffing.assignment_versions
  DROP CONSTRAINT IF EXISTS assignment_versions_profile_is_object_check;

ALTER TABLE staffing.assignment_versions
  ADD CONSTRAINT assignment_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object');

ALTER TABLE staffing.positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.positions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.positions;
CREATE POLICY tenant_isolation ON staffing.positions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.position_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.position_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.position_events;
CREATE POLICY tenant_isolation ON staffing.position_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.position_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.position_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.position_versions;
CREATE POLICY tenant_isolation ON staffing.position_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.assignments FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.assignments;
CREATE POLICY tenant_isolation ON staffing.assignments
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.assignment_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.assignment_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_events;
CREATE POLICY tenant_isolation ON staffing.assignment_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.assignment_event_corrections ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.assignment_event_corrections FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_event_corrections;
CREATE POLICY tenant_isolation ON staffing.assignment_event_corrections
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.assignment_event_rescinds ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.assignment_event_rescinds FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_event_rescinds;
CREATE POLICY tenant_isolation ON staffing.assignment_event_rescinds
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.assignment_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.assignment_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_versions;
CREATE POLICY tenant_isolation ON staffing.assignment_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
