-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.replay_time_profile_versions(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_name text := NULL;
  v_lifecycle_status text := 'active';

  v_shift_start_local time := NULL;
  v_shift_end_local time := NULL;
  v_late_tolerance_minutes int := 0;
  v_early_leave_tolerance_minutes int := 0;

  v_overtime_min_minutes int := 0;
  v_overtime_rounding_mode text := 'NONE';
  v_overtime_rounding_unit_minutes int := 0;

  v_prev_effective date := NULL;
  v_validity daterange;
  v_last_validity daterange;

  v_tmp_text text;
  v_has_any boolean := false;
  v_lock_key text;

  v_row record;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  v_lock_key := format('staffing:time-profile:%s', p_tenant_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.time_profile_versions
  WHERE tenant_id = p_tenant_id;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.payload,
      lead(e.effective_date) OVER (ORDER BY e.effective_date ASC, e.id ASC) AS next_effective
    FROM staffing.time_profile_events e
    WHERE e.tenant_id = p_tenant_id
    ORDER BY e.effective_date ASC, e.id ASC
  LOOP
    v_has_any := true;

    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_name := NULL;
      v_lifecycle_status := 'active';
      v_late_tolerance_minutes := 0;
      v_early_leave_tolerance_minutes := 0;
      v_overtime_min_minutes := 0;
      v_overtime_rounding_mode := 'NONE';
      v_overtime_rounding_unit_minutes := 0;

      v_tmp_text := NULLIF(btrim(v_row.payload->>'shift_start_local'), '');
      IF v_tmp_text IS NULL THEN
        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_start_local is required';
      END IF;
      BEGIN
        v_shift_start_local := v_tmp_text::time;
      EXCEPTION
        WHEN others THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid shift_start_local: %s', v_row.payload->>'shift_start_local');
      END;

      v_tmp_text := NULLIF(btrim(v_row.payload->>'shift_end_local'), '');
      IF v_tmp_text IS NULL THEN
        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_end_local is required';
      END IF;
      BEGIN
        v_shift_end_local := v_tmp_text::time;
      EXCEPTION
        WHEN others THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid shift_end_local: %s', v_row.payload->>'shift_end_local');
      END;

      IF v_shift_end_local <= v_shift_start_local THEN
        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_end_local must be greater than shift_start_local';
      END IF;

      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_lifecycle_status := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_lifecycle_status IS NULL OR v_lifecycle_status NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid lifecycle_status: %s', v_row.payload->>'lifecycle_status');
        END IF;
      END IF;

      IF v_row.payload ? 'late_tolerance_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'late_tolerance_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_late_tolerance_minutes := 0;
        ELSE
          BEGIN
            v_late_tolerance_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid late_tolerance_minutes: %s', v_row.payload->>'late_tolerance_minutes');
          END;
          IF v_late_tolerance_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'late_tolerance_minutes must be non-negative';
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'early_leave_tolerance_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'early_leave_tolerance_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_early_leave_tolerance_minutes := 0;
        ELSE
          BEGIN
            v_early_leave_tolerance_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid early_leave_tolerance_minutes: %s', v_row.payload->>'early_leave_tolerance_minutes');
          END;
          IF v_early_leave_tolerance_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'early_leave_tolerance_minutes must be non-negative';
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'overtime_min_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'overtime_min_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_overtime_min_minutes := 0;
        ELSE
          BEGIN
            v_overtime_min_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid overtime_min_minutes: %s', v_row.payload->>'overtime_min_minutes');
          END;
          IF v_overtime_min_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'overtime_min_minutes must be non-negative';
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'overtime_rounding_mode' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'overtime_rounding_mode'), '');
        IF v_tmp_text IS NULL THEN
          v_overtime_rounding_mode := 'NONE';
        ELSE
          v_overtime_rounding_mode := upper(v_tmp_text);
        END IF;
        IF v_overtime_rounding_mode NOT IN ('NONE','FLOOR','CEIL','NEAREST') THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid overtime_rounding_mode: %s', v_row.payload->>'overtime_rounding_mode');
        END IF;
      END IF;

      IF v_row.payload ? 'overtime_rounding_unit_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'overtime_rounding_unit_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_overtime_rounding_unit_minutes := 0;
        ELSE
          BEGIN
            v_overtime_rounding_unit_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid overtime_rounding_unit_minutes: %s', v_row.payload->>'overtime_rounding_unit_minutes');
          END;
          IF v_overtime_rounding_unit_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'overtime_rounding_unit_minutes must be non-negative';
          END IF;
        END IF;
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'shift_start_local' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'shift_start_local'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_start_local is required';
        END IF;
        BEGIN
          v_shift_start_local := v_tmp_text::time;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid shift_start_local: %s', v_row.payload->>'shift_start_local');
        END;
      END IF;

      IF v_row.payload ? 'shift_end_local' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'shift_end_local'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_end_local is required';
        END IF;
        BEGIN
          v_shift_end_local := v_tmp_text::time;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid shift_end_local: %s', v_row.payload->>'shift_end_local');
        END;
      END IF;

      IF v_shift_end_local <= v_shift_start_local THEN
        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_end_local must be greater than shift_start_local';
      END IF;

      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;

      IF v_row.payload ? 'lifecycle_status' THEN
        v_lifecycle_status := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_lifecycle_status IS NULL OR v_lifecycle_status NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid lifecycle_status: %s', v_row.payload->>'lifecycle_status');
        END IF;
      END IF;

      IF v_row.payload ? 'late_tolerance_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'late_tolerance_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_late_tolerance_minutes := 0;
        ELSE
          BEGIN
            v_late_tolerance_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid late_tolerance_minutes: %s', v_row.payload->>'late_tolerance_minutes');
          END;
          IF v_late_tolerance_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'late_tolerance_minutes must be non-negative';
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'early_leave_tolerance_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'early_leave_tolerance_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_early_leave_tolerance_minutes := 0;
        ELSE
          BEGIN
            v_early_leave_tolerance_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid early_leave_tolerance_minutes: %s', v_row.payload->>'early_leave_tolerance_minutes');
          END;
          IF v_early_leave_tolerance_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'early_leave_tolerance_minutes must be non-negative';
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'overtime_min_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'overtime_min_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_overtime_min_minutes := 0;
        ELSE
          BEGIN
            v_overtime_min_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid overtime_min_minutes: %s', v_row.payload->>'overtime_min_minutes');
          END;
          IF v_overtime_min_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'overtime_min_minutes must be non-negative';
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'overtime_rounding_mode' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'overtime_rounding_mode'), '');
        IF v_tmp_text IS NULL THEN
          v_overtime_rounding_mode := 'NONE';
        ELSE
          v_overtime_rounding_mode := upper(v_tmp_text);
        END IF;
        IF v_overtime_rounding_mode NOT IN ('NONE','FLOOR','CEIL','NEAREST') THEN
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid overtime_rounding_mode: %s', v_row.payload->>'overtime_rounding_mode');
        END IF;
      END IF;

      IF v_row.payload ? 'overtime_rounding_unit_minutes' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'overtime_rounding_unit_minutes'), '');
        IF v_tmp_text IS NULL THEN
          v_overtime_rounding_unit_minutes := 0;
        ELSE
          BEGIN
            v_overtime_rounding_unit_minutes := v_tmp_text::int;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid overtime_rounding_unit_minutes: %s', v_row.payload->>'overtime_rounding_unit_minutes');
          END;
          IF v_overtime_rounding_unit_minutes < 0 THEN
            RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'overtime_rounding_unit_minutes must be non-negative';
          END IF;
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    INSERT INTO staffing.time_profile_versions (
      tenant_id,
      name,
      lifecycle_status,
      shift_start_local,
      shift_end_local,
      late_tolerance_minutes,
      early_leave_tolerance_minutes,
      overtime_min_minutes,
      overtime_rounding_mode,
      overtime_rounding_unit_minutes,
      validity,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      v_name,
      v_lifecycle_status,
      v_shift_start_local,
      v_shift_end_local,
      v_late_tolerance_minutes,
      v_early_leave_tolerance_minutes,
      v_overtime_min_minutes,
      v_overtime_rounding_mode,
      v_overtime_rounding_unit_minutes,
      v_validity,
      v_row.event_db_id
    );

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF NOT v_has_any THEN
    RETURN;
  END IF;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.time_profile_versions
      WHERE tenant_id = p_tenant_id
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_GAP',
      DETAIL = 'time_profile_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.time_profile_versions
  WHERE tenant_id = p_tenant_id
  ORDER BY lower(validity) DESC
  LIMIT 1;

  IF v_last_validity IS NOT NULL AND NOT upper_inf(v_last_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'last time_profile version validity must be unbounded (infinity)';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_time_profile_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.time_profile_events%ROWTYPE;
  v_payload jsonb;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:time-profile:%s', p_tenant_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;
  IF p_event_type = 'CREATE' THEN
    IF NULLIF(btrim(v_payload->>'shift_start_local'), '') IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_start_local is required';
    END IF;
    IF NULLIF(btrim(v_payload->>'shift_end_local'), '') IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'shift_end_local is required';
    END IF;
  END IF;

  INSERT INTO staffing.time_profile_events (
    event_id,
    tenant_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.time_profile_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM staffing.replay_time_profile_versions(p_tenant_id);

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_holiday_day_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_day_date date,
  p_event_type text,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.holiday_day_events%ROWTYPE;
  v_payload jsonb;
  v_day_type text;
  v_holiday_code text;
  v_note text;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_day_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'day_date is required';
  END IF;
  IF p_event_type NOT IN ('SET','CLEAR') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:holiday-day:%s:%s', p_tenant_id, p_day_date);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  IF p_event_type = 'SET' THEN
    v_day_type := NULLIF(btrim(v_payload->>'day_type'), '');
    IF v_day_type IS NULL OR v_day_type NOT IN ('WORKDAY','RESTDAY','LEGAL_HOLIDAY') THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('invalid day_type: %s', v_payload->>'day_type');
    END IF;
    v_holiday_code := NULLIF(btrim(v_payload->>'holiday_code'), '');
    v_note := NULLIF(btrim(v_payload->>'note'), '');
  END IF;

  INSERT INTO staffing.holiday_day_events (
    event_id,
    tenant_id,
    day_date,
    event_type,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_day_date,
    p_event_type,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.holiday_day_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.day_date <> p_day_date
      OR v_existing.event_type <> p_event_type
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  IF p_event_type = 'SET' THEN
    INSERT INTO staffing.holiday_days (
      tenant_id,
      day_date,
      day_type,
      holiday_code,
      note,
      last_event_id,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_id,
      p_day_date,
      v_day_type,
      v_holiday_code,
      v_note,
      v_event_db_id,
      now(),
      now()
    )
    ON CONFLICT (tenant_id, day_date)
    DO UPDATE SET
      day_type = EXCLUDED.day_type,
      holiday_code = EXCLUDED.holiday_code,
      note = EXCLUDED.note,
      last_event_id = EXCLUDED.last_event_id,
      updated_at = EXCLUDED.updated_at;
  ELSE
    DELETE FROM staffing.holiday_days
    WHERE tenant_id = p_tenant_id AND day_date = p_day_date;
  END IF;

  RETURN v_event_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS staffing.submit_holiday_day_event(uuid, uuid, date, text, jsonb, text, uuid);
DROP FUNCTION IF EXISTS staffing.submit_time_profile_event(uuid, uuid, text, date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS staffing.replay_time_profile_versions(uuid);

