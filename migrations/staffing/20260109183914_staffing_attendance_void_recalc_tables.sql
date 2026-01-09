-- +goose Up
CREATE TABLE IF NOT EXISTS staffing.time_punch_void_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  target_punch_event_db_id bigint NOT NULL,
  target_punch_event_id uuid NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT time_punch_void_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT time_punch_void_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT time_punch_void_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT time_punch_void_events_target_unique UNIQUE (tenant_id, target_punch_event_db_id)
);

CREATE INDEX IF NOT EXISTS time_punch_void_events_person_created_idx
  ON staffing.time_punch_void_events (tenant_id, person_uuid, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS time_punch_void_events_target_idx
  ON staffing.time_punch_void_events (tenant_id, target_punch_event_db_id);

ALTER TABLE staffing.time_punch_void_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.time_punch_void_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.time_punch_void_events;
CREATE POLICY tenant_isolation ON staffing.time_punch_void_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS staffing.attendance_recalc_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  from_date date NOT NULL,
  to_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT attendance_recalc_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT attendance_recalc_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT attendance_recalc_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT attendance_recalc_events_date_range_check CHECK (to_date >= from_date),
  CONSTRAINT attendance_recalc_events_range_size_check CHECK ((to_date - from_date) <= 30)
);

CREATE INDEX IF NOT EXISTS attendance_recalc_events_person_range_idx
  ON staffing.attendance_recalc_events (tenant_id, person_uuid, from_date, to_date, id);

CREATE INDEX IF NOT EXISTS attendance_recalc_events_created_idx
  ON staffing.attendance_recalc_events (tenant_id, created_at DESC, id DESC);

ALTER TABLE staffing.attendance_recalc_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.attendance_recalc_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.attendance_recalc_events;
CREATE POLICY tenant_isolation ON staffing.attendance_recalc_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation ON staffing.attendance_recalc_events;
ALTER TABLE IF EXISTS staffing.attendance_recalc_events DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS staffing.attendance_recalc_events_created_idx;
DROP INDEX IF EXISTS staffing.attendance_recalc_events_person_range_idx;
DROP TABLE IF EXISTS staffing.attendance_recalc_events;

DROP POLICY IF EXISTS tenant_isolation ON staffing.time_punch_void_events;
ALTER TABLE IF EXISTS staffing.time_punch_void_events DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS staffing.time_punch_void_events_target_idx;
DROP INDEX IF EXISTS staffing.time_punch_void_events_person_created_idx;
DROP TABLE IF EXISTS staffing.time_punch_void_events;
