-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.recompute_daily_attendance_result(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_work_date date
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_tz text := 'Asia/Shanghai';
  v_ruleset_version text := 'TIME_PROFILE_V1';

  v_shift_start_local time := NULL;
  v_shift_end_local time := NULL;
  v_late_tolerance_min int := 0;
  v_early_tolerance_min int := 0;

  v_overtime_min_minutes int := 0;
  v_overtime_rounding_mode text := 'NONE';
  v_overtime_rounding_unit_minutes int := 0;

  v_window_before interval := interval '6 hours';
  v_window_after interval := interval '12 hours';

  v_shift_start timestamptz;
  v_shift_end timestamptz;
  v_window_start timestamptz;
  v_window_end timestamptz;

  v_punch_count int := 0;
  v_input_max_id bigint := NULL;
  v_input_max_punch_time timestamptz := NULL;

  v_expect text := 'IN';
  v_open_in_time timestamptz := NULL;

  v_first_in_time timestamptz := NULL;
  v_last_out_time timestamptz := NULL;

  v_day_type text := NULL;
  v_holiday_day_last_event_id bigint := NULL;

  v_scheduled_minutes int := 0;
  v_worked_minutes int := 0;
  v_overtime_minutes_150 int := 0;
  v_overtime_minutes_200 int := 0;
  v_overtime_minutes_300 int := 0;
  v_late_minutes int := 0;
  v_early_leave_minutes int := 0;

  v_time_profile_last_event_id bigint := NULL;

  v_status text := 'ABSENT';
  v_flags text[] := '{}'::text[];

  r record;
  v_delta_min int;
  v_raw_ot int := 0;
  v_rounded_ot int := 0;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_work_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'work_date is required';
  END IF;

  PERFORM pg_advisory_xact_lock(
    hashtext(p_tenant_id::text),
    hashtext(p_person_uuid::text || ':' || p_work_date::text)
  );

  SELECT
    shift_start_local,
    shift_end_local,
    late_tolerance_minutes,
    early_leave_tolerance_minutes,
    overtime_min_minutes,
    overtime_rounding_mode,
    overtime_rounding_unit_minutes,
    last_event_id
  INTO
    v_shift_start_local,
    v_shift_end_local,
    v_late_tolerance_min,
    v_early_tolerance_min,
    v_overtime_min_minutes,
    v_overtime_rounding_mode,
    v_overtime_rounding_unit_minutes,
    v_time_profile_last_event_id
  FROM staffing.time_profile_versions
  WHERE tenant_id = p_tenant_id
    AND lifecycle_status = 'active'
    AND validity @> p_work_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_TIME_PROFILE_NOT_CONFIGURED_AS_OF',
      DETAIL = format('tenant_id=%s as_of=%s', p_tenant_id, p_work_date);
  END IF;

  v_scheduled_minutes := floor(extract(epoch FROM (v_shift_end_local - v_shift_start_local)) / 60.0)::int;
  IF v_scheduled_minutes < 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'scheduled_minutes must be non-negative';
  END IF;

  SELECT day_type, last_event_id
  INTO v_day_type, v_holiday_day_last_event_id
  FROM staffing.holiday_days
  WHERE tenant_id = p_tenant_id
    AND day_date = p_work_date;

  IF NOT FOUND THEN
    IF extract(isodow FROM p_work_date) IN (6, 7) THEN
      v_day_type := 'RESTDAY';
    ELSE
      v_day_type := 'WORKDAY';
    END IF;
    v_holiday_day_last_event_id := NULL;
  END IF;

  v_shift_start := (p_work_date + v_shift_start_local) AT TIME ZONE v_tz;
  v_shift_end := (p_work_date + v_shift_end_local) AT TIME ZONE v_tz;
  v_window_start := v_shift_start - v_window_before;
  v_window_end := v_shift_end + v_window_after;

  FOR r IN
    SELECT e.id, e.punch_time, e.punch_type
    FROM staffing.time_punch_events e
    WHERE e.tenant_id = p_tenant_id
      AND e.person_uuid = p_person_uuid
      AND e.punch_time >= v_window_start
      AND e.punch_time < v_window_end
      AND NOT EXISTS (
        SELECT 1
        FROM staffing.time_punch_void_events v
        WHERE v.tenant_id = e.tenant_id
          AND v.target_punch_event_db_id = e.id
      )
    ORDER BY e.punch_time ASC, e.id ASC
  LOOP
    v_punch_count := v_punch_count + 1;
    v_input_max_id := COALESCE(v_input_max_id, r.id);
    v_input_max_id := GREATEST(v_input_max_id, r.id);
    v_input_max_punch_time := COALESCE(v_input_max_punch_time, r.punch_time);
    v_input_max_punch_time := GREATEST(v_input_max_punch_time, r.punch_time);

    IF r.punch_type = 'IN' THEN
      IF v_expect = 'IN' THEN
        v_open_in_time := r.punch_time;
        v_expect := 'OUT';
        IF v_first_in_time IS NULL THEN
          v_first_in_time := r.punch_time;
        END IF;
      ELSE
        v_flags := array_append(v_flags, 'MISSING_OUT');
        v_open_in_time := r.punch_time;
        v_expect := 'OUT';
        IF v_first_in_time IS NULL THEN
          v_first_in_time := r.punch_time;
        END IF;
      END IF;
    ELSIF r.punch_type = 'OUT' THEN
      IF v_expect = 'OUT' AND v_open_in_time IS NOT NULL THEN
        v_delta_min := floor(extract(epoch FROM (r.punch_time - v_open_in_time)) / 60.0)::int;
        IF v_delta_min > 0 THEN
          v_worked_minutes := v_worked_minutes + v_delta_min;
        END IF;
        v_last_out_time := r.punch_time;
        v_open_in_time := NULL;
        v_expect := 'IN';
      ELSE
        v_flags := array_append(v_flags, 'MISSING_IN');
      END IF;
    ELSIF r.punch_type = 'RAW' THEN
      IF v_expect = 'IN' THEN
        v_open_in_time := r.punch_time;
        v_expect := 'OUT';
        IF v_first_in_time IS NULL THEN
          v_first_in_time := r.punch_time;
        END IF;
      ELSE
        IF v_expect = 'OUT' AND v_open_in_time IS NOT NULL THEN
          v_delta_min := floor(extract(epoch FROM (r.punch_time - v_open_in_time)) / 60.0)::int;
          IF v_delta_min > 0 THEN
            v_worked_minutes := v_worked_minutes + v_delta_min;
          END IF;
          v_last_out_time := r.punch_time;
          v_open_in_time := NULL;
          v_expect := 'IN';
        ELSE
          v_flags := array_append(v_flags, 'MISSING_IN');
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported punch_type in recompute: %s', r.punch_type);
    END IF;
  END LOOP;

  IF v_punch_count = 0 THEN
    IF v_day_type = 'WORKDAY' THEN
      v_status := 'ABSENT';
      v_flags := array_append(v_flags, 'ABSENT');
    ELSE
      v_status := 'OFF';
      v_flags := '{}'::text[];
    END IF;
  ELSE
    IF v_first_in_time IS NULL THEN
      v_flags := array_append(v_flags, 'MISSING_IN');
    END IF;
    IF v_expect = 'OUT' THEN
      v_flags := array_append(v_flags, 'MISSING_OUT');
    END IF;

    IF v_first_in_time IS NOT NULL THEN
      v_delta_min := floor(extract(epoch FROM (v_first_in_time - v_shift_start)) / 60.0)::int;
      IF v_delta_min > v_late_tolerance_min THEN
        v_late_minutes := v_delta_min - v_late_tolerance_min;
        v_flags := array_append(v_flags, 'LATE');
      END IF;
    END IF;

    IF v_last_out_time IS NOT NULL THEN
      v_delta_min := floor(extract(epoch FROM (v_shift_end - v_last_out_time)) / 60.0)::int;
      IF v_delta_min > v_early_tolerance_min THEN
        v_early_leave_minutes := v_delta_min - v_early_tolerance_min;
        v_flags := array_append(v_flags, 'EARLY_LEAVE');
      END IF;
    END IF;

    IF array_length(v_flags, 1) IS NULL THEN
      v_status := 'PRESENT';
    ELSE
      SELECT COALESCE(array_agg(DISTINCT f ORDER BY f), '{}'::text[]) INTO v_flags
      FROM unnest(v_flags) AS f;
      v_status := 'EXCEPTION';
    END IF;
  END IF;

  IF v_day_type = 'WORKDAY' THEN
    v_raw_ot := GREATEST(0, v_worked_minutes - v_scheduled_minutes);
  ELSE
    v_raw_ot := v_worked_minutes;
  END IF;

  IF v_raw_ot < v_overtime_min_minutes THEN
    v_raw_ot := 0;
  END IF;

  v_rounded_ot := v_raw_ot;
  IF v_rounded_ot > 0 AND v_overtime_rounding_unit_minutes > 0 AND v_overtime_rounding_mode <> 'NONE' THEN
    IF v_overtime_rounding_mode = 'FLOOR' THEN
      v_rounded_ot := floor(v_rounded_ot::numeric / v_overtime_rounding_unit_minutes::numeric) * v_overtime_rounding_unit_minutes;
    ELSIF v_overtime_rounding_mode = 'CEIL' THEN
      v_rounded_ot := ceiling(v_rounded_ot::numeric / v_overtime_rounding_unit_minutes::numeric) * v_overtime_rounding_unit_minutes;
    ELSIF v_overtime_rounding_mode = 'NEAREST' THEN
      v_rounded_ot := round(v_rounded_ot::numeric / v_overtime_rounding_unit_minutes::numeric) * v_overtime_rounding_unit_minutes;
    END IF;
  END IF;

  v_overtime_minutes_150 := 0;
  v_overtime_minutes_200 := 0;
  v_overtime_minutes_300 := 0;
  IF v_rounded_ot > 0 THEN
    IF v_day_type = 'WORKDAY' THEN
      v_overtime_minutes_150 := v_rounded_ot;
    ELSIF v_day_type = 'RESTDAY' THEN
      v_overtime_minutes_200 := v_rounded_ot;
    ELSIF v_day_type = 'LEGAL_HOLIDAY' THEN
      v_overtime_minutes_300 := v_rounded_ot;
    END IF;
  END IF;

  INSERT INTO staffing.daily_attendance_results (
    tenant_id,
    person_uuid,
    work_date,
    ruleset_version,
    day_type,
    status,
    flags,
    first_in_time,
    last_out_time,
    scheduled_minutes,
    worked_minutes,
    overtime_minutes_150,
    overtime_minutes_200,
    overtime_minutes_300,
    late_minutes,
    early_leave_minutes,
    input_punch_count,
    input_max_punch_event_db_id,
    input_max_punch_time,
    time_profile_last_event_id,
    holiday_day_last_event_id,
    computed_at,
    created_at,
    updated_at
  )
  VALUES (
    p_tenant_id,
    p_person_uuid,
    p_work_date,
    v_ruleset_version,
    v_day_type,
    v_status,
    v_flags,
    v_first_in_time,
    v_last_out_time,
    v_scheduled_minutes,
    v_worked_minutes,
    v_overtime_minutes_150,
    v_overtime_minutes_200,
    v_overtime_minutes_300,
    v_late_minutes,
    v_early_leave_minutes,
    v_punch_count,
    v_input_max_id,
    v_input_max_punch_time,
    v_time_profile_last_event_id,
    v_holiday_day_last_event_id,
    now(),
    now(),
    now()
  )
  ON CONFLICT (tenant_id, person_uuid, work_date)
  DO UPDATE SET
    ruleset_version = EXCLUDED.ruleset_version,
    day_type = EXCLUDED.day_type,
    status = EXCLUDED.status,
    flags = EXCLUDED.flags,
    first_in_time = EXCLUDED.first_in_time,
    last_out_time = EXCLUDED.last_out_time,
    scheduled_minutes = EXCLUDED.scheduled_minutes,
    worked_minutes = EXCLUDED.worked_minutes,
    overtime_minutes_150 = EXCLUDED.overtime_minutes_150,
    overtime_minutes_200 = EXCLUDED.overtime_minutes_200,
    overtime_minutes_300 = EXCLUDED.overtime_minutes_300,
    late_minutes = EXCLUDED.late_minutes,
    early_leave_minutes = EXCLUDED.early_leave_minutes,
    input_punch_count = EXCLUDED.input_punch_count,
    input_max_punch_event_db_id = EXCLUDED.input_max_punch_event_db_id,
    input_max_punch_time = EXCLUDED.input_max_punch_time,
    time_profile_last_event_id = EXCLUDED.time_profile_last_event_id,
    holiday_day_last_event_id = EXCLUDED.holiday_day_last_event_id,
    computed_at = EXCLUDED.computed_at,
    updated_at = EXCLUDED.updated_at;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_time_punch_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_punch_time timestamptz,
  p_punch_type text,
  p_source_provider text,
  p_payload jsonb,
  p_source_raw_payload jsonb,
  p_device_info jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_event_db_id bigint;
  v_existing staffing.time_punch_events%ROWTYPE;
  v_payload jsonb;
  v_source_raw jsonb;
  v_device jsonb;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_punch_time IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'punch_time is required';
  END IF;
  IF p_punch_type NOT IN ('IN','OUT','RAW') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported punch_type: %s', p_punch_type);
  END IF;
  IF p_source_provider NOT IN ('MANUAL','IMPORT','DINGTALK','WECOM') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported source_provider: %s', p_source_provider);
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  v_source_raw := COALESCE(p_source_raw_payload, '{}'::jsonb);
  v_device := COALESCE(p_device_info, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;
  IF jsonb_typeof(v_source_raw) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'source_raw_payload must be an object';
  END IF;
  IF jsonb_typeof(v_device) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'device_info must be an object';
  END IF;

  INSERT INTO staffing.time_punch_events (
    event_id,
    tenant_id,
    person_uuid,
    punch_time,
    punch_type,
    source_provider,
    payload,
    source_raw_payload,
    device_info,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_person_uuid,
    p_punch_time,
    p_punch_type,
    p_source_provider,
    v_payload,
    v_source_raw,
    v_device,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.time_punch_events
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing.tenant_id <> p_tenant_id
        OR v_existing.person_uuid <> p_person_uuid
        OR v_existing.punch_time <> p_punch_time
        OR v_existing.punch_type <> p_punch_type
        OR v_existing.source_provider <> p_source_provider
        OR v_existing.payload <> v_payload
        OR v_existing.source_raw_payload <> v_source_raw
        OR v_existing.device_info <> v_device
        OR v_existing.request_id <> p_request_id
        OR v_existing.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
      END IF;

      RETURN v_existing.id;
    END IF;

    SELECT * INTO v_existing
    FROM staffing.time_punch_events
    WHERE tenant_id = p_tenant_id
      AND request_id = p_request_id;

    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('request_id_conflict_not_found request_id=%s event_id=%s', p_request_id, p_event_id);
    END IF;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.punch_time <> p_punch_time
      OR v_existing.punch_type <> p_punch_type
      OR v_existing.source_provider <> p_source_provider
      OR v_existing.payload <> v_payload
      OR v_existing.source_raw_payload <> v_source_raw
      OR v_existing.device_info <> v_device
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('request_id=%s existing_id=%s', p_request_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM staffing.recompute_daily_attendance_results_for_punch(p_tenant_id, p_person_uuid, p_punch_time);

  RETURN v_event_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
