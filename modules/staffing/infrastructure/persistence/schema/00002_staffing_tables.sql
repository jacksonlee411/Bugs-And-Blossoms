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
  CONSTRAINT assignment_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT assignment_events_one_per_day_unique UNIQUE (tenant_id, assignment_id, effective_date),
  CONSTRAINT assignment_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT assignment_events_assignment_fk FOREIGN KEY (tenant_id, assignment_id) REFERENCES staffing.assignments(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS assignment_events_tenant_assignment_effective_idx
  ON staffing.assignment_events (tenant_id, assignment_id, effective_date, id);

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
  ADD COLUMN IF NOT EXISTS base_salary numeric(15,2) NULL,
  ADD COLUMN IF NOT EXISTS currency char(3) NOT NULL DEFAULT 'CNY',
  ADD COLUMN IF NOT EXISTS profile jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE staffing.assignment_versions
  DROP CONSTRAINT IF EXISTS assignment_versions_currency_check,
  DROP CONSTRAINT IF EXISTS assignment_versions_base_salary_check,
  DROP CONSTRAINT IF EXISTS assignment_versions_profile_is_object_check;

ALTER TABLE staffing.assignment_versions
  ADD CONSTRAINT assignment_versions_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  ADD CONSTRAINT assignment_versions_base_salary_check CHECK (base_salary IS NULL OR base_salary >= 0),
  ADD CONSTRAINT assignment_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object');

CREATE TABLE IF NOT EXISTS staffing.time_punch_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  punch_time timestamptz NOT NULL,
  punch_type text NOT NULL,
  source_provider text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  source_raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  device_info jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT time_punch_events_punch_type_check CHECK (punch_type IN ('IN','OUT','RAW')),
  CONSTRAINT time_punch_events_source_provider_check CHECK (source_provider IN ('MANUAL','IMPORT','DINGTALK','WECOM')),
  CONSTRAINT time_punch_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT time_punch_events_source_raw_is_object_check CHECK (jsonb_typeof(source_raw_payload) = 'object'),
  CONSTRAINT time_punch_events_device_info_is_object_check CHECK (jsonb_typeof(device_info) = 'object'),
  CONSTRAINT time_punch_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT time_punch_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS time_punch_events_lookup_idx
  ON staffing.time_punch_events (tenant_id, person_uuid, punch_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS staffing.daily_attendance_results (
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  work_date date NOT NULL,

  ruleset_version text NOT NULL,
  day_type text NULL,
  status text NOT NULL,
  flags text[] NOT NULL DEFAULT '{}'::text[],

  first_in_time timestamptz NULL,
  last_out_time timestamptz NULL,
  scheduled_minutes int NOT NULL DEFAULT 0,
  worked_minutes int NOT NULL DEFAULT 0,
  overtime_minutes_150 int NOT NULL DEFAULT 0,
  overtime_minutes_200 int NOT NULL DEFAULT 0,
  overtime_minutes_300 int NOT NULL DEFAULT 0,
  late_minutes int NOT NULL DEFAULT 0,
  early_leave_minutes int NOT NULL DEFAULT 0,

  input_punch_count int NOT NULL DEFAULT 0,
  input_max_punch_event_db_id bigint NULL,
  input_max_punch_time timestamptz NULL,

  time_profile_last_event_id bigint NULL,
  holiday_day_last_event_id bigint NULL,

  computed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, person_uuid, work_date),

  CONSTRAINT daily_attendance_results_status_check
    CHECK (status IN ('PRESENT','ABSENT','EXCEPTION','OFF')),
  CONSTRAINT daily_attendance_results_day_type_check
    CHECK (day_type IS NULL OR day_type IN ('WORKDAY','RESTDAY','LEGAL_HOLIDAY')),
  CONSTRAINT daily_attendance_results_minutes_nonneg_check
    CHECK (scheduled_minutes >= 0 AND worked_minutes >= 0 AND late_minutes >= 0 AND early_leave_minutes >= 0),
  CONSTRAINT daily_attendance_results_overtime_nonneg_check
    CHECK (overtime_minutes_150 >= 0 AND overtime_minutes_200 >= 0 AND overtime_minutes_300 >= 0),
  CONSTRAINT daily_attendance_results_flags_allowlist_check
    CHECK (flags <@ ARRAY['ABSENT','MISSING_IN','MISSING_OUT','LATE','EARLY_LEAVE']::text[])
);

CREATE INDEX IF NOT EXISTS daily_attendance_results_lookup_idx
  ON staffing.daily_attendance_results (tenant_id, person_uuid, work_date DESC);

CREATE TABLE IF NOT EXISTS staffing.time_profile_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT time_profile_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT time_profile_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT time_profile_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT time_profile_events_one_per_day_unique UNIQUE (tenant_id, effective_date),
  CONSTRAINT time_profile_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS time_profile_events_lookup_idx
  ON staffing.time_profile_events (tenant_id, effective_date, id);

CREATE TABLE IF NOT EXISTS staffing.time_profile_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  name text NULL,
  lifecycle_status text NOT NULL DEFAULT 'active',

  shift_start_local time NOT NULL,
  shift_end_local time NOT NULL,
  late_tolerance_minutes int NOT NULL DEFAULT 0,
  early_leave_tolerance_minutes int NOT NULL DEFAULT 0,

  overtime_min_minutes int NOT NULL DEFAULT 0,
  overtime_rounding_mode text NOT NULL DEFAULT 'NONE',
  overtime_rounding_unit_minutes int NOT NULL DEFAULT 0,

  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.time_profile_events(id),

  CONSTRAINT time_profile_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT time_profile_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT time_profile_versions_lifecycle_status_check CHECK (lifecycle_status IN ('active','disabled')),
  CONSTRAINT time_profile_versions_shift_time_order_check CHECK (shift_end_local > shift_start_local),
  CONSTRAINT time_profile_versions_tolerance_minutes_check CHECK (late_tolerance_minutes >= 0 AND early_leave_tolerance_minutes >= 0),
  CONSTRAINT time_profile_versions_overtime_min_check CHECK (overtime_min_minutes >= 0),
  CONSTRAINT time_profile_versions_overtime_rounding_mode_check CHECK (overtime_rounding_mode IN ('NONE','FLOOR','CEIL','NEAREST')),
  CONSTRAINT time_profile_versions_overtime_rounding_unit_check CHECK (overtime_rounding_unit_minutes >= 0),
  CONSTRAINT time_profile_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS time_profile_versions_lookup_idx
  ON staffing.time_profile_versions (tenant_id, lower(validity));

CREATE TABLE IF NOT EXISTS staffing.holiday_day_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  day_date date NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT holiday_day_events_event_type_check CHECK (event_type IN ('SET','CLEAR')),
  CONSTRAINT holiday_day_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT holiday_day_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT holiday_day_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS holiday_day_events_lookup_idx
  ON staffing.holiday_day_events (tenant_id, day_date, id);

CREATE TABLE IF NOT EXISTS staffing.holiday_days (
  tenant_id uuid NOT NULL,
  day_date date NOT NULL,
  day_type text NOT NULL,
  holiday_code text NULL,
  note text NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.holiday_day_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, day_date),
  CONSTRAINT holiday_days_day_type_check CHECK (day_type IN ('WORKDAY','RESTDAY','LEGAL_HOLIDAY'))
);

CREATE INDEX IF NOT EXISTS holiday_days_lookup_idx
  ON staffing.holiday_days (tenant_id, day_date DESC);

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

ALTER TABLE staffing.assignment_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.assignment_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.assignment_versions;
CREATE POLICY tenant_isolation ON staffing.assignment_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.time_punch_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.time_punch_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.time_punch_events;
CREATE POLICY tenant_isolation ON staffing.time_punch_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.daily_attendance_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.daily_attendance_results FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.daily_attendance_results;
CREATE POLICY tenant_isolation ON staffing.daily_attendance_results
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.time_profile_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.time_profile_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.time_profile_events;
CREATE POLICY tenant_isolation ON staffing.time_profile_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.time_profile_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.time_profile_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.time_profile_versions;
CREATE POLICY tenant_isolation ON staffing.time_profile_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.holiday_day_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.holiday_day_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.holiday_day_events;
CREATE POLICY tenant_isolation ON staffing.holiday_day_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.holiday_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.holiday_days FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.holiday_days;
CREATE POLICY tenant_isolation ON staffing.holiday_days
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
