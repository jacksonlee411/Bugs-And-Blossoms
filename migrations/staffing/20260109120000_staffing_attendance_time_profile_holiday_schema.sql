-- +goose Up
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

ALTER TABLE staffing.daily_attendance_results
  ADD COLUMN IF NOT EXISTS day_type text NULL,
  ADD COLUMN IF NOT EXISTS scheduled_minutes int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS overtime_minutes_150 int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS overtime_minutes_200 int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS overtime_minutes_300 int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS time_profile_last_event_id bigint NULL,
  ADD COLUMN IF NOT EXISTS holiday_day_last_event_id bigint NULL;

ALTER TABLE staffing.daily_attendance_results
  DROP CONSTRAINT IF EXISTS daily_attendance_results_status_check,
  DROP CONSTRAINT IF EXISTS daily_attendance_results_day_type_check,
  DROP CONSTRAINT IF EXISTS daily_attendance_results_overtime_nonneg_check;

ALTER TABLE staffing.daily_attendance_results
  ADD CONSTRAINT daily_attendance_results_status_check
    CHECK (status IN ('PRESENT','ABSENT','EXCEPTION','OFF')),
  ADD CONSTRAINT daily_attendance_results_day_type_check
    CHECK (day_type IS NULL OR day_type IN ('WORKDAY','RESTDAY','LEGAL_HOLIDAY')),
  ADD CONSTRAINT daily_attendance_results_overtime_nonneg_check
    CHECK (scheduled_minutes >= 0 AND overtime_minutes_150 >= 0 AND overtime_minutes_200 >= 0 AND overtime_minutes_300 >= 0);

-- +goose Down
ALTER TABLE staffing.daily_attendance_results
  DROP CONSTRAINT IF EXISTS daily_attendance_results_status_check,
  DROP CONSTRAINT IF EXISTS daily_attendance_results_day_type_check,
  DROP CONSTRAINT IF EXISTS daily_attendance_results_overtime_nonneg_check;

ALTER TABLE staffing.daily_attendance_results
  ADD CONSTRAINT daily_attendance_results_status_check
    CHECK (status IN ('PRESENT','ABSENT','EXCEPTION'));

ALTER TABLE staffing.daily_attendance_results
  DROP COLUMN IF EXISTS holiday_day_last_event_id,
  DROP COLUMN IF EXISTS time_profile_last_event_id,
  DROP COLUMN IF EXISTS overtime_minutes_300,
  DROP COLUMN IF EXISTS overtime_minutes_200,
  DROP COLUMN IF EXISTS overtime_minutes_150,
  DROP COLUMN IF EXISTS scheduled_minutes,
  DROP COLUMN IF EXISTS day_type;

DROP TABLE IF EXISTS staffing.holiday_days;
DROP TABLE IF EXISTS staffing.holiday_day_events;
DROP TABLE IF EXISTS staffing.time_profile_versions;
DROP TABLE IF EXISTS staffing.time_profile_events;

