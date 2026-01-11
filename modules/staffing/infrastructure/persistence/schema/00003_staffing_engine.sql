CREATE OR REPLACE FUNCTION staffing.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'tenant_id is required';
  END IF;

  v_ctx_raw := current_setting('app.current_tenant', true);
  IF v_ctx_raw IS NULL OR btrim(v_ctx_raw) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_CONTEXT_MISSING',
      DETAIL = 'app.current_tenant is required';
  END IF;

  BEGIN
    v_ctx_tenant := v_ctx_raw::uuid;
  EXCEPTION
    WHEN invalid_text_representation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'RLS_TENANT_CONTEXT_INVALID',
        DETAIL = format('app.current_tenant=%s', v_ctx_raw);
  END;

  IF v_ctx_tenant <> p_tenant_id THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_MISMATCH',
      DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_id, v_ctx_tenant);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_time_punch_void_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_target_punch_event_id uuid,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_target staffing.time_punch_events%ROWTYPE;
  v_existing_by_event staffing.time_punch_void_events%ROWTYPE;
  v_existing_by_target staffing.time_punch_void_events%ROWTYPE;
  v_payload jsonb;
  v_void_db_id bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_target_punch_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_punch_event_id is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  SELECT * INTO v_target
  FROM staffing.time_punch_events
  WHERE tenant_id = p_tenant_id
    AND event_id = p_target_punch_event_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_TIME_PUNCH_EVENT_NOT_FOUND',
      DETAIL = format('tenant_id=%s target_event_id=%s', p_tenant_id, p_target_punch_event_id);
  END IF;

  INSERT INTO staffing.time_punch_void_events (
    event_id,
    tenant_id,
    person_uuid,
    target_punch_event_db_id,
    target_punch_event_id,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    v_target.person_uuid,
    v_target.id,
    v_target.event_id,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_void_db_id;

  IF v_void_db_id IS NULL THEN
    SELECT * INTO v_existing_by_event
    FROM staffing.time_punch_void_events
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing_by_event.tenant_id <> p_tenant_id
        OR v_existing_by_event.person_uuid <> v_target.person_uuid
        OR v_existing_by_event.target_punch_event_db_id <> v_target.id
        OR v_existing_by_event.target_punch_event_id <> v_target.event_id
        OR v_existing_by_event.payload <> v_payload
        OR v_existing_by_event.request_id <> p_request_id
        OR v_existing_by_event.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_by_event.id);
      END IF;
      RETURN v_existing_by_event.id;
    END IF;

    SELECT * INTO v_existing_by_target
    FROM staffing.time_punch_void_events
    WHERE tenant_id = p_tenant_id
      AND target_punch_event_db_id = v_target.id
    LIMIT 1;

    IF FOUND THEN
      RETURN v_existing_by_target.id;
    END IF;

    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'void insert failed';
  END IF;

  PERFORM staffing.recompute_daily_attendance_results_for_punch(p_tenant_id, v_target.person_uuid, v_target.punch_time);

  RETURN v_void_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_attendance_recalc_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_from_date date,
  p_to_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_existing staffing.attendance_recalc_events%ROWTYPE;
  v_payload jsonb;
  v_recalc_db_id bigint;
  v_d date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_from_date IS NULL OR p_to_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'from_date/to_date is required';
  END IF;
  IF p_to_date < p_from_date THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'to_date must be >= from_date';
  END IF;
  IF (p_to_date - p_from_date) > 30 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'date range too large (max 31 days)';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  INSERT INTO staffing.attendance_recalc_events (
    event_id,
    tenant_id,
    person_uuid,
    from_date,
    to_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_person_uuid,
    p_from_date,
    p_to_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_recalc_db_id;

  IF v_recalc_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.attendance_recalc_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.from_date <> p_from_date
      OR v_existing.to_date <> p_to_date
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

  v_d := p_from_date;
  WHILE v_d <= p_to_date LOOP
    PERFORM staffing.recompute_daily_attendance_result(p_tenant_id, p_person_uuid, v_d);
    v_d := v_d + 1;
  END LOOP;

  RETURN v_recalc_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.assert_position_capacity(
  p_tenant_id uuid,
  p_position_id uuid,
  p_validity daterange
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_as_of date;
  v_capacity_fte numeric(9,2);
  v_allocated_sum numeric(9,2);
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_id is required';
  END IF;
  IF p_validity IS NULL OR isempty(p_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'validity is required';
  END IF;

  FOR v_as_of IN
    SELECT d::date AS as_of
    FROM (
      SELECT lower(p_validity) AS d
      UNION
      SELECT lower(av.validity) AS d
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.position_id = p_position_id
        AND av.status = 'active'
        AND av.validity && p_validity
      UNION
      SELECT lower(pv.validity) AS d
      FROM staffing.position_versions pv
      WHERE pv.tenant_id = p_tenant_id
        AND pv.position_id = p_position_id
        AND pv.validity && p_validity
    ) dates
    WHERE d IS NOT NULL
      AND d >= lower(p_validity)
      AND (upper_inf(p_validity) OR d < upper(p_validity))
    ORDER BY d
  LOOP
    SELECT pv.capacity_fte INTO v_capacity_fte
    FROM staffing.position_versions pv
    WHERE pv.tenant_id = p_tenant_id
      AND pv.position_id = p_position_id
      AND pv.validity @> v_as_of
    LIMIT 1;

    IF v_capacity_fte IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
        DETAIL = format('position_id=%s as_of=%s', p_position_id, v_as_of);
    END IF;

    SELECT COALESCE(sum(av.allocated_fte), 0)::numeric(9,2) INTO v_allocated_sum
    FROM staffing.assignment_versions av
    WHERE av.tenant_id = p_tenant_id
      AND av.position_id = p_position_id
      AND av.status = 'active'
      AND av.validity @> v_as_of;

    IF v_allocated_sum > v_capacity_fte THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_POSITION_CAPACITY_EXCEEDED',
        DETAIL = format('position_id=%s as_of=%s allocated_sum=%s capacity_fte=%s', p_position_id, v_as_of, v_allocated_sum, v_capacity_fte);
    END IF;
  END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.replay_position_versions(
  p_tenant_id uuid,
  p_position_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_org_unit_id uuid;
  v_reports_to_position_id uuid;
  v_business_unit_id text;
  v_jobcatalog_setid text;
  v_job_profile_id uuid;
  v_capacity_fte numeric(9,2);
  v_name text;
  v_lifecycle_status text;
  v_reports_to_status text;
  v_tmp_text text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'position_id is required';
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_id, p_position_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.position_versions
  WHERE tenant_id = p_tenant_id AND position_id = p_position_id;

  v_org_unit_id := NULL;
  v_reports_to_position_id := NULL;
  v_business_unit_id := NULL;
  v_jobcatalog_setid := NULL;
  v_job_profile_id := NULL;
  v_capacity_fte := 1.0;
  v_name := NULL;
  v_lifecycle_status := 'active';
  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.payload,
      lead(e.effective_date) OVER (ORDER BY e.effective_date ASC, e.id ASC) AS next_effective
    FROM staffing.position_events e
    WHERE e.tenant_id = p_tenant_id AND e.position_id = p_position_id
    ORDER BY e.effective_date ASC, e.id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_org_unit_id := NULLIF(v_row.payload->>'org_unit_id', '')::uuid;
      IF v_org_unit_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'org_unit_id is required';
      END IF;

      v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      v_reports_to_position_id := NULL;
      v_business_unit_id := NULLIF(btrim(v_row.payload->>'business_unit_id'), '');
      v_job_profile_id := NULL;
      IF v_row.payload ? 'job_profile_id' THEN
        v_job_profile_id := NULLIF(v_row.payload->>'job_profile_id', '')::uuid;
      END IF;
      v_capacity_fte := 1.0;
      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              ERRCODE = 'P0001',
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;
      v_lifecycle_status := 'active';
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'org_unit_id' THEN
        v_org_unit_id := NULLIF(v_row.payload->>'org_unit_id', '')::uuid;
        IF v_org_unit_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'org_unit_id is required';
        END IF;
      END IF;
      IF v_row.payload ? 'name' THEN
        v_name := NULLIF(btrim(v_row.payload->>'name'), '');
      END IF;
      IF v_row.payload ? 'reports_to_position_id' THEN
        v_reports_to_position_id := NULLIF(v_row.payload->>'reports_to_position_id', '')::uuid;
      END IF;
      IF v_row.payload ? 'business_unit_id' THEN
        v_business_unit_id := NULLIF(btrim(v_row.payload->>'business_unit_id'), '');
      END IF;
      IF v_row.payload ? 'job_profile_id' THEN
        v_job_profile_id := NULLIF(v_row.payload->>'job_profile_id', '')::uuid;
      END IF;
      IF v_row.payload ? 'capacity_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'capacity_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'capacity_fte is required';
        END IF;
        BEGIN
          v_capacity_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              ERRCODE = 'P0001',
              MESSAGE = 'STAFFING_INVALID_ARGUMENT',
              DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END;
        IF v_capacity_fte <= 0 THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid capacity_fte: %s', v_row.payload->>'capacity_fte');
        END IF;
      END IF;
      IF v_row.payload ? 'lifecycle_status' THEN
        v_lifecycle_status := NULLIF(btrim(v_row.payload->>'lifecycle_status'), '');
        IF v_lifecycle_status IS NULL OR v_lifecycle_status NOT IN ('active','disabled') THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid lifecycle_status: %s', v_row.payload->>'lifecycle_status');
        END IF;
      END IF;
    ELSE
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_id = p_tenant_id
        AND v.hierarchy_type = 'OrgUnit'
        AND v.org_id = v_org_unit_id
        AND v.status = 'active'
        AND v.validity @> v_row.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_ORG_UNIT_NOT_FOUND_AS_OF',
        DETAIL = format('org_unit_id=%s as_of=%s', v_org_unit_id, v_row.effective_date);
    END IF;

    v_jobcatalog_setid := NULL;
    IF v_business_unit_id IS NOT NULL THEN
      v_jobcatalog_setid := orgunit.resolve_setid(p_tenant_id, v_business_unit_id, 'jobcatalog');
    END IF;
    IF v_job_profile_id IS NOT NULL THEN
      IF v_jobcatalog_setid IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'business_unit_id is required when binding job_profile_id';
      END IF;
      IF NOT EXISTS (
        SELECT 1
        FROM jobcatalog.job_profiles jp
        WHERE jp.tenant_id = p_tenant_id
          AND jp.setid = v_jobcatalog_setid
          AND jp.id = v_job_profile_id
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'JOBCATALOG_REFERENCE_NOT_FOUND',
          DETAIL = format('job_profile_id=%s setid=%s', v_job_profile_id, v_jobcatalog_setid);
      END IF;
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    IF v_reports_to_position_id IS NOT NULL THEN
      IF v_reports_to_position_id = p_position_id THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_SELF',
          DETAIL = format('position_id=%s as_of=%s', p_position_id, v_row.effective_date);
      END IF;

      SELECT pv.lifecycle_status INTO v_reports_to_status
      FROM staffing.position_versions pv
      WHERE pv.tenant_id = p_tenant_id
        AND pv.position_id = v_reports_to_position_id
        AND pv.validity @> v_row.effective_date
      LIMIT 1;
      IF NOT FOUND THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_reports_to_position_id, v_row.effective_date);
      END IF;
      IF v_reports_to_status <> 'active' THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_reports_to_position_id, v_row.effective_date);
      END IF;

      IF EXISTS (
        WITH RECURSIVE chain AS (
          SELECT
            pv.position_id,
            pv.reports_to_position_id,
            ARRAY[pv.position_id]::uuid[] AS path
          FROM staffing.position_versions pv
          WHERE pv.tenant_id = p_tenant_id
            AND pv.position_id = v_reports_to_position_id
            AND pv.validity @> v_row.effective_date
          UNION ALL
          SELECT
            pv.position_id,
            pv.reports_to_position_id,
            c.path || pv.position_id
          FROM chain c
          JOIN staffing.position_versions pv
            ON pv.tenant_id = p_tenant_id
           AND pv.position_id = c.reports_to_position_id
           AND pv.validity @> v_row.effective_date
          WHERE c.reports_to_position_id IS NOT NULL
            AND NOT (pv.position_id = ANY(c.path))
        )
        SELECT 1
        FROM chain
        WHERE reports_to_position_id = p_position_id
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_REPORTS_TO_CYCLE',
          DETAIL = format('position_id=%s reports_to_position_id=%s as_of=%s', p_position_id, v_reports_to_position_id, v_row.effective_date);
      END IF;
    END IF;

    IF v_lifecycle_status = 'disabled' AND EXISTS (
      SELECT 1
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.position_id = p_position_id
        AND av.status = 'active'
        AND av.validity && v_validity
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'STAFFING_POSITION_HAS_ACTIVE_ASSIGNMENT_AS_OF',
        DETAIL = format('position_id=%s as_of=%s', p_position_id, lower(v_validity));
    END IF;

    INSERT INTO staffing.position_versions (
      tenant_id,
      position_id,
      org_unit_id,
      reports_to_position_id,
      business_unit_id,
      jobcatalog_setid,
      job_profile_id,
      name,
      lifecycle_status,
      capacity_fte,
      profile,
      validity,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_position_id,
      v_org_unit_id,
      v_reports_to_position_id,
      v_business_unit_id,
      v_jobcatalog_setid,
      v_job_profile_id,
      v_name,
      v_lifecycle_status,
      v_capacity_fte,
      '{}'::jsonb,
      v_validity,
      v_row.event_db_id
    );

    PERFORM staffing.assert_position_capacity(p_tenant_id, p_position_id, v_validity);

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.position_versions
      WHERE tenant_id = p_tenant_id AND position_id = p_position_id
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
      DETAIL = 'position_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.position_versions
  WHERE tenant_id = p_tenant_id AND position_id = p_position_id
  ORDER BY lower(validity) DESC
  LIMIT 1;

  IF v_last_validity IS NOT NULL AND NOT upper_inf(v_last_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'last position version validity must be unbounded (infinity)';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_position_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_position_id uuid,
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
  v_reports_to_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.position_events%ROWTYPE;
  v_payload jsonb;
  v_prev_effective_max date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_position_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'position_id is required';
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
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  IF v_payload ? 'reports_to_position_id' THEN
    v_reports_to_lock_key := format('staffing:position-reports-to:%s', p_tenant_id);
    PERFORM pg_advisory_xact_lock(hashtextextended(v_reports_to_lock_key, 0));
  END IF;

  v_lock_key := format('staffing:position:%s:%s', p_tenant_id, p_position_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.positions (tenant_id, id)
  VALUES (p_tenant_id, p_position_id)
  ON CONFLICT DO NOTHING;

  INSERT INTO staffing.position_events (
    event_id,
    tenant_id,
    position_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_position_id,
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
    FROM staffing.position_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.position_id <> p_position_id
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

  IF p_event_type = 'UPDATE' AND v_payload ? 'reports_to_position_id' THEN
    SELECT max(effective_date) INTO v_prev_effective_max
    FROM staffing.position_events
    WHERE tenant_id = p_tenant_id
      AND position_id = p_position_id
      AND id <> v_event_db_id;

    IF v_prev_effective_max IS NOT NULL AND p_effective_date <= v_prev_effective_max THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('reports_to_position_id updates must be forward-only: effective_date=%s last_effective_date=%s', p_effective_date, v_prev_effective_max);
    END IF;
  END IF;

  PERFORM staffing.replay_position_versions(p_tenant_id, p_position_id);

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.replay_assignment_versions(
  p_tenant_id uuid,
  p_assignment_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_person_uuid uuid;
  v_assignment_type text;
  v_position_id uuid;
  v_status text;
  v_allocated_fte numeric(9,2);
  v_base_salary numeric(15,2);
  v_currency text;
  v_profile jsonb;
  v_tmp_text text;
  v_row RECORD;
  v_validity daterange;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'assignment_id is required';
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.assignment_versions
  WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id;

  v_person_uuid := NULL;
  v_assignment_type := NULL;
  v_position_id := NULL;
  v_status := 'active';
  v_allocated_fte := 1.0;
  v_base_salary := NULL;
  v_currency := 'CNY';
  v_profile := '{}'::jsonb;
  v_prev_effective := NULL;

  FOR v_row IN
    WITH base AS (
      SELECT
        e.id AS event_db_id,
        e.event_type,
        e.effective_date,
        e.person_uuid,
        e.assignment_type,
        COALESCE(c.replacement_payload, e.payload) AS payload,
        (r.id IS NOT NULL) AS is_rescinded
      FROM staffing.assignment_events e
      LEFT JOIN staffing.assignment_event_corrections c
        ON c.tenant_id = e.tenant_id
       AND c.assignment_id = e.assignment_id
       AND c.target_effective_date = e.effective_date
      LEFT JOIN staffing.assignment_event_rescinds r
        ON r.tenant_id = e.tenant_id
       AND r.assignment_id = e.assignment_id
       AND r.target_effective_date = e.effective_date
      WHERE e.tenant_id = p_tenant_id
        AND e.assignment_id = p_assignment_id
    ),
    filtered AS (
      SELECT *
      FROM base
      WHERE NOT is_rescinded
    ),
    ordered AS (
      SELECT
        event_db_id,
        event_type,
        effective_date,
        person_uuid,
        assignment_type,
        payload,
        lead(effective_date) OVER (ORDER BY effective_date ASC, event_db_id ASC) AS next_effective
      FROM filtered
    )
    SELECT *
    FROM ordered
    ORDER BY effective_date ASC, event_db_id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'CREATE must be the first event';
      END IF;

      v_person_uuid := v_row.person_uuid;
      v_assignment_type := v_row.assignment_type;

      v_position_id := NULLIF(v_row.payload->>'position_id', '')::uuid;
      IF v_position_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_ARGUMENT',
          DETAIL = 'position_id is required';
      END IF;
      v_status := 'active';

      IF v_row.payload ? 'base_salary' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'base_salary'), '');
        IF v_tmp_text IS NULL THEN
          v_base_salary := NULL;
        ELSE
          BEGIN
            v_base_salary := v_tmp_text::numeric;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_ASSIGNMENT_BASE_SALARY_INVALID',
                DETAIL = format('base_salary=%s', v_row.payload->>'base_salary');
          END;
          IF v_base_salary < 0 THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_BASE_SALARY_INVALID',
              DETAIL = format('base_salary=%s', v_row.payload->>'base_salary');
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'allocated_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'allocated_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = 'allocated_fte is required';
        END IF;
        BEGIN
          v_allocated_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
              DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END;
        IF v_allocated_fte <= 0 OR v_allocated_fte > 1 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'currency' THEN
        v_tmp_text := upper(btrim(v_row.payload->>'currency'));
        IF v_tmp_text IS NULL OR v_tmp_text = '' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED',
            DETAIL = 'currency is required';
        END IF;
        IF v_tmp_text <> 'CNY' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED',
            DETAIL = format('currency=%s', v_row.payload->>'currency');
        END IF;
        v_currency := v_tmp_text;
      END IF;

      IF v_row.payload ? 'profile' THEN
        IF jsonb_typeof(v_row.payload->'profile') <> 'object' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_PROFILE_INVALID',
            DETAIL = 'profile must be an object';
        END IF;
        v_profile := v_row.payload->'profile';
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_INVALID_EVENT',
          DETAIL = 'UPDATE requires prior state';
      END IF;

      IF v_row.payload ? 'position_id' THEN
        v_position_id := NULLIF(v_row.payload->>'position_id', '')::uuid;
        IF v_position_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = 'position_id is required';
	      END IF;
      END IF;

      IF v_row.payload ? 'status' THEN
        v_status := NULLIF(btrim(v_row.payload->>'status'), '');
        IF v_status IS NULL OR v_status NOT IN ('active','inactive') THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_INVALID_ARGUMENT',
            DETAIL = format('invalid status: %s', v_row.payload->>'status');
	        END IF;
      END IF;

      IF v_row.payload ? 'base_salary' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'base_salary'), '');
        IF v_tmp_text IS NULL THEN
          v_base_salary := NULL;
        ELSE
          BEGIN
            v_base_salary := v_tmp_text::numeric;
          EXCEPTION
            WHEN others THEN
              RAISE EXCEPTION USING
                MESSAGE = 'STAFFING_ASSIGNMENT_BASE_SALARY_INVALID',
                DETAIL = format('base_salary=%s', v_row.payload->>'base_salary');
          END;
          IF v_base_salary < 0 THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_BASE_SALARY_INVALID',
              DETAIL = format('base_salary=%s', v_row.payload->>'base_salary');
          END IF;
        END IF;
      END IF;

      IF v_row.payload ? 'allocated_fte' THEN
        v_tmp_text := NULLIF(btrim(v_row.payload->>'allocated_fte'), '');
        IF v_tmp_text IS NULL THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = 'allocated_fte is required';
        END IF;
        BEGIN
          v_allocated_fte := v_tmp_text::numeric;
        EXCEPTION
          WHEN others THEN
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
              DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END;
        IF v_allocated_fte <= 0 OR v_allocated_fte > 1 THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_ALLOCATED_FTE_INVALID',
            DETAIL = format('allocated_fte=%s', v_row.payload->>'allocated_fte');
        END IF;
      END IF;

      IF v_row.payload ? 'currency' THEN
        v_tmp_text := upper(btrim(v_row.payload->>'currency'));
        IF v_tmp_text IS NULL OR v_tmp_text = '' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED',
            DETAIL = 'currency is required';
        END IF;
        IF v_tmp_text <> 'CNY' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_CURRENCY_UNSUPPORTED',
            DETAIL = format('currency=%s', v_row.payload->>'currency');
        END IF;
        v_currency := v_tmp_text;
      END IF;

      IF v_row.payload ? 'profile' THEN
        IF jsonb_typeof(v_row.payload->'profile') <> 'object' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_ASSIGNMENT_PROFILE_INVALID',
            DETAIL = 'profile must be an object';
        END IF;
        v_profile := v_row.payload->'profile';
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

    IF v_status = 'active' THEN
      IF NOT EXISTS (
        SELECT 1
        FROM staffing.position_versions pv
        WHERE pv.tenant_id = p_tenant_id
          AND pv.position_id = v_position_id
          AND pv.lifecycle_status = 'active'
          AND pv.validity @> v_row.effective_date
        LIMIT 1
      ) THEN
        IF EXISTS (
          SELECT 1
          FROM staffing.position_versions pv
          WHERE pv.tenant_id = p_tenant_id
            AND pv.position_id = v_position_id
            AND pv.validity @> v_row.effective_date
          LIMIT 1
        ) THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'STAFFING_POSITION_DISABLED_AS_OF',
            DETAIL = format('position_id=%s as_of=%s', v_position_id, v_row.effective_date);
        END IF;

        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'STAFFING_POSITION_NOT_FOUND_AS_OF',
          DETAIL = format('position_id=%s as_of=%s', v_position_id, v_row.effective_date);
      END IF;
    END IF;

    INSERT INTO staffing.assignment_versions (
      tenant_id,
      assignment_id,
      person_uuid,
      position_id,
      assignment_type,
      status,
      allocated_fte,
      base_salary,
      currency,
      profile,
      validity,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_assignment_id,
      v_person_uuid,
      v_position_id,
      v_assignment_type,
      v_status,
      v_allocated_fte,
      v_base_salary,
      v_currency,
      v_profile,
      v_validity,
      v_row.event_db_id
    );

    IF v_status = 'active' THEN
      PERFORM staffing.assert_position_capacity(p_tenant_id, v_position_id, v_validity);
    END IF;

    v_prev_effective := v_row.effective_date;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        validity,
        lag(validity) OVER (ORDER BY lower(validity)) AS prev_validity
      FROM staffing.assignment_versions
      WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id
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
      DETAIL = 'assignment_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.assignment_versions
  WHERE tenant_id = p_tenant_id AND assignment_id = p_assignment_id
  ORDER BY lower(validity) DESC
  LIMIT 1;

  IF v_last_validity IS NOT NULL AND NOT upper_inf(v_last_validity) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'last assignment version validity must be unbounded (infinity)';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_assignment_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_person_uuid uuid,
  p_assignment_type text,
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
  v_existing staffing.assignment_events%ROWTYPE;
  v_payload jsonb;
  v_existing_assignment_id uuid;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_assignment_type IS NULL OR btrim(p_assignment_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_type is required';
  END IF;
  IF p_assignment_type <> 'primary' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported assignment_type: %s', p_assignment_type);
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
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.assignments (tenant_id, id, person_uuid, assignment_type)
  VALUES (p_tenant_id, p_assignment_id, p_person_uuid, p_assignment_type)
  ON CONFLICT (tenant_id, person_uuid, assignment_type) DO NOTHING;

  SELECT id INTO v_existing_assignment_id
  FROM staffing.assignments
  WHERE tenant_id = p_tenant_id AND person_uuid = p_person_uuid AND assignment_type = p_assignment_type;

  IF v_existing_assignment_id IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'assignment identity missing';
  END IF;
  IF v_existing_assignment_id <> p_assignment_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_ID_MISMATCH',
      DETAIL = format('assignment_id=%s existing_id=%s', p_assignment_id, v_existing_assignment_id);
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  INSERT INTO staffing.assignment_events (
    event_id,
    tenant_id,
    assignment_id,
    person_uuid,
    assignment_type,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_person_uuid,
    p_assignment_type,
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
    FROM staffing.assignment_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.assignment_id <> p_assignment_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.assignment_type <> p_assignment_type
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

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-5 adds staffing.maybe_create_payroll_recalc_request_from_assignment_event later).
  EXECUTE 'SELECT staffing.maybe_create_payroll_recalc_request_from_assignment_event($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::date,$7::jsonb,$8::text,$9::uuid);'
  USING p_event_id, p_tenant_id, p_assignment_id, p_person_uuid, p_event_type, p_effective_date, v_payload, p_request_id, p_initiator_id;

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_assignment_event_correction(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_target_effective_date date,
  p_replacement_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target staffing.assignment_events%ROWTYPE;
  v_existing_by_event staffing.assignment_event_corrections%ROWTYPE;
  v_existing_by_request staffing.assignment_event_corrections%ROWTYPE;
  v_existing_by_target staffing.assignment_event_corrections%ROWTYPE;
  v_payload jsonb;
  v_correction_db_id bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_replacement_payload IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'replacement_payload is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := p_replacement_payload;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'replacement_payload must be an object';
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_target
  FROM staffing.assignment_events
  WHERE tenant_id = p_tenant_id
    AND assignment_id = p_assignment_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_NOT_FOUND',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM staffing.assignment_event_rescinds r
    WHERE r.tenant_id = p_tenant_id
      AND r.assignment_id = p_assignment_id
      AND r.target_effective_date = p_target_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_ALREADY_RESCINDED',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  INSERT INTO staffing.assignment_event_corrections (
    event_id,
    tenant_id,
    assignment_id,
    target_effective_date,
    replacement_payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_correction_db_id;

  IF v_correction_db_id IS NULL THEN
    SELECT * INTO v_existing_by_event
    FROM staffing.assignment_event_corrections
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing_by_event.tenant_id <> p_tenant_id
        OR v_existing_by_event.assignment_id <> p_assignment_id
        OR v_existing_by_event.target_effective_date <> p_target_effective_date
        OR v_existing_by_event.replacement_payload <> v_payload
        OR v_existing_by_event.request_id <> p_request_id
        OR v_existing_by_event.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_by_event.id);
      END IF;
      v_correction_db_id := v_existing_by_event.id;
    ELSE
      SELECT * INTO v_existing_by_request
      FROM staffing.assignment_event_corrections
      WHERE tenant_id = p_tenant_id
        AND request_id = p_request_id
      LIMIT 1;

      IF FOUND THEN
        IF v_existing_by_request.tenant_id <> p_tenant_id
          OR v_existing_by_request.assignment_id <> p_assignment_id
          OR v_existing_by_request.target_effective_date <> p_target_effective_date
          OR v_existing_by_request.replacement_payload <> v_payload
          OR v_existing_by_request.request_id <> p_request_id
          OR v_existing_by_request.initiator_id <> p_initiator_id
        THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
            DETAIL = format('request_id=%s existing_id=%s', p_request_id, v_existing_by_request.id);
        END IF;
        v_correction_db_id := v_existing_by_request.id;
      ELSE
        SELECT * INTO v_existing_by_target
        FROM staffing.assignment_event_corrections
        WHERE tenant_id = p_tenant_id
          AND assignment_id = p_assignment_id
          AND target_effective_date = p_target_effective_date
        LIMIT 1;

        IF FOUND THEN
          IF v_existing_by_target.replacement_payload = v_payload THEN
            v_correction_db_id := v_existing_by_target.id;
          ELSE
            RAISE EXCEPTION USING
              MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_ALREADY_CORRECTED',
              DETAIL = format('assignment_id=%s target_effective_date=%s existing_id=%s', p_assignment_id, p_target_effective_date, v_existing_by_target.id);
          END IF;
        ELSE
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'correction insert failed';
        END IF;
      END IF;
    END IF;
  END IF;

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  EXECUTE 'SELECT staffing.maybe_create_payroll_recalc_request_from_assignment_event($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::date,$7::jsonb,$8::text,$9::uuid);'
  USING p_event_id, p_tenant_id, p_assignment_id, v_target.person_uuid, 'UPDATE', p_target_effective_date, v_payload, p_request_id, p_initiator_id;

  RETURN v_correction_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_assignment_event_rescind(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_target_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target staffing.assignment_events%ROWTYPE;
  v_existing_by_event staffing.assignment_event_rescinds%ROWTYPE;
  v_existing_by_request staffing.assignment_event_rescinds%ROWTYPE;
  v_existing_by_target staffing.assignment_event_rescinds%ROWTYPE;
  v_payload jsonb;
  v_rescind_db_id bigint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_target
  FROM staffing.assignment_events
  WHERE tenant_id = p_tenant_id
    AND assignment_id = p_assignment_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_EVENT_NOT_FOUND',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  IF v_target.event_type = 'CREATE' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_CREATE_CANNOT_RESCIND',
      DETAIL = format('assignment_id=%s target_effective_date=%s', p_assignment_id, p_target_effective_date);
  END IF;

  INSERT INTO staffing.assignment_event_rescinds (
    event_id,
    tenant_id,
    assignment_id,
    target_effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT DO NOTHING
  RETURNING id INTO v_rescind_db_id;

  IF v_rescind_db_id IS NULL THEN
    SELECT * INTO v_existing_by_event
    FROM staffing.assignment_event_rescinds
    WHERE event_id = p_event_id;

    IF FOUND THEN
      IF v_existing_by_event.tenant_id <> p_tenant_id
        OR v_existing_by_event.assignment_id <> p_assignment_id
        OR v_existing_by_event.target_effective_date <> p_target_effective_date
        OR v_existing_by_event.payload <> v_payload
        OR v_existing_by_event.request_id <> p_request_id
        OR v_existing_by_event.initiator_id <> p_initiator_id
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
          DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_by_event.id);
      END IF;
      v_rescind_db_id := v_existing_by_event.id;
    ELSE
      SELECT * INTO v_existing_by_request
      FROM staffing.assignment_event_rescinds
      WHERE tenant_id = p_tenant_id
        AND request_id = p_request_id
      LIMIT 1;

      IF FOUND THEN
        IF v_existing_by_request.tenant_id <> p_tenant_id
          OR v_existing_by_request.assignment_id <> p_assignment_id
          OR v_existing_by_request.target_effective_date <> p_target_effective_date
          OR v_existing_by_request.payload <> v_payload
          OR v_existing_by_request.request_id <> p_request_id
          OR v_existing_by_request.initiator_id <> p_initiator_id
        THEN
          RAISE EXCEPTION USING
            MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
            DETAIL = format('request_id=%s existing_id=%s', p_request_id, v_existing_by_request.id);
        END IF;
        v_rescind_db_id := v_existing_by_request.id;
      ELSE
        SELECT * INTO v_existing_by_target
        FROM staffing.assignment_event_rescinds
        WHERE tenant_id = p_tenant_id
          AND assignment_id = p_assignment_id
          AND target_effective_date = p_target_effective_date
        LIMIT 1;

        IF FOUND THEN
          v_rescind_db_id := v_existing_by_target.id;
        ELSE
          RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'rescind insert failed';
        END IF;
      END IF;
    END IF;
  END IF;

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  EXECUTE 'SELECT staffing.maybe_create_payroll_recalc_request_from_assignment_event($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::date,$7::jsonb,$8::text,$9::uuid);'
  USING p_event_id, p_tenant_id, p_assignment_id, v_target.person_uuid, 'UPDATE', p_target_effective_date, v_target.payload, p_request_id, p_initiator_id;

  RETURN v_rescind_db_id;
END;
$$;

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
  v_tmp_int int;
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

CREATE OR REPLACE FUNCTION staffing.recompute_time_bank_cycle(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_work_date date
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_cycle_type text := 'MONTH';
  v_cycle_start date;
  v_cycle_end date;

  v_ruleset_version text := 'TIME_BANK_V1';

  v_worked_total int := 0;
  v_ot_150 int := 0;
  v_ot_200 int := 0;
  v_ot_300 int := 0;
  v_comp_earned int := 0;
  v_comp_used int := 0;

  v_input_max_id bigint := NULL;
  v_input_max_punch_time timestamptz := NULL;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_work_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'work_date is required';
  END IF;

  v_cycle_start := date_trunc('month', p_work_date)::date;
  v_cycle_end := ((date_trunc('month', p_work_date) + interval '1 month')::date - 1);

  PERFORM pg_advisory_xact_lock(
    hashtext(p_tenant_id::text),
    hashtext(p_person_uuid::text || ':' || v_cycle_type || ':' || v_cycle_start::text)
  );

  SELECT
    COALESCE(sum(worked_minutes), 0)::int,
    COALESCE(sum(overtime_minutes_150), 0)::int,
    COALESCE(sum(overtime_minutes_200), 0)::int,
    COALESCE(sum(overtime_minutes_300), 0)::int,
    COALESCE(sum(CASE WHEN day_type = 'RESTDAY' THEN overtime_minutes_200 ELSE 0 END), 0)::int,
    COALESCE(max(input_max_punch_event_db_id), NULL),
    COALESCE(max(input_max_punch_time), NULL)
  INTO
    v_worked_total,
    v_ot_150,
    v_ot_200,
    v_ot_300,
    v_comp_earned,
    v_input_max_id,
    v_input_max_punch_time
  FROM staffing.daily_attendance_results
  WHERE tenant_id = p_tenant_id
    AND person_uuid = p_person_uuid
    AND work_date >= v_cycle_start
    AND work_date <= v_cycle_end;

  INSERT INTO staffing.time_bank_cycles (
    tenant_id,
    person_uuid,
    cycle_type,
    cycle_start_date,
    cycle_end_date,
    ruleset_version,
    worked_minutes_total,
    overtime_minutes_150,
    overtime_minutes_200,
    overtime_minutes_300,
    comp_earned_minutes,
    comp_used_minutes,
    input_max_punch_event_db_id,
    input_max_punch_time,
    computed_at,
    created_at,
    updated_at
  )
  VALUES (
    p_tenant_id,
    p_person_uuid,
    v_cycle_type,
    v_cycle_start,
    v_cycle_end,
    v_ruleset_version,
    v_worked_total,
    v_ot_150,
    v_ot_200,
    v_ot_300,
    v_comp_earned,
    v_comp_used,
    v_input_max_id,
    v_input_max_punch_time,
    now(),
    now(),
    now()
  )
  ON CONFLICT (tenant_id, person_uuid, cycle_type, cycle_start_date)
  DO UPDATE SET
    cycle_end_date = EXCLUDED.cycle_end_date,
    ruleset_version = EXCLUDED.ruleset_version,
    worked_minutes_total = EXCLUDED.worked_minutes_total,
    overtime_minutes_150 = EXCLUDED.overtime_minutes_150,
    overtime_minutes_200 = EXCLUDED.overtime_minutes_200,
    overtime_minutes_300 = EXCLUDED.overtime_minutes_300,
    comp_earned_minutes = EXCLUDED.comp_earned_minutes,
    comp_used_minutes = EXCLUDED.comp_used_minutes,
    input_max_punch_event_db_id = EXCLUDED.input_max_punch_event_db_id,
    input_max_punch_time = EXCLUDED.input_max_punch_time,
    computed_at = EXCLUDED.computed_at,
    updated_at = EXCLUDED.updated_at;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.get_time_profile_for_work_date(
  p_tenant_id uuid,
  p_work_date date
)
RETURNS TABLE (
  shift_start_local time,
  shift_end_local time,
  late_tolerance_minutes int,
  early_leave_tolerance_minutes int,
  overtime_min_minutes int,
  overtime_rounding_mode text,
  overtime_rounding_unit_minutes int,
  time_profile_last_event_id bigint,
  shift_start timestamptz,
  shift_end timestamptz,
  window_start timestamptz,
  window_end timestamptz
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_tz text := 'Asia/Shanghai';
  v_window_before interval := interval '6 hours';
  v_window_after interval := interval '12 hours';
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_work_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'work_date is required';
  END IF;

  SELECT
    tp.shift_start_local,
    tp.shift_end_local,
    tp.late_tolerance_minutes,
    tp.early_leave_tolerance_minutes,
    tp.overtime_min_minutes,
    tp.overtime_rounding_mode,
    tp.overtime_rounding_unit_minutes,
    tp.last_event_id
  INTO
    shift_start_local,
    shift_end_local,
    late_tolerance_minutes,
    early_leave_tolerance_minutes,
    overtime_min_minutes,
    overtime_rounding_mode,
    overtime_rounding_unit_minutes,
    time_profile_last_event_id
  FROM staffing.time_profile_versions tp
  WHERE tp.tenant_id = p_tenant_id
    AND tp.lifecycle_status = 'active'
    AND tp.validity @> p_work_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_TIME_PROFILE_NOT_CONFIGURED_AS_OF',
      DETAIL = format('tenant_id=%s as_of=%s', p_tenant_id, p_work_date);
  END IF;

  shift_start := (p_work_date + shift_start_local) AT TIME ZONE v_tz;
  shift_end := (p_work_date + shift_end_local) AT TIME ZONE v_tz;
  window_start := shift_start - v_window_before;
  window_end := shift_end + v_window_after;

  RETURN NEXT;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.recompute_daily_attendance_result(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_work_date date
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ruleset_version text := 'TIME_PROFILE_V1';

  v_shift_start_local time := NULL;
  v_shift_end_local time := NULL;
  v_late_tolerance_min int := 0;
  v_early_tolerance_min int := 0;

  v_overtime_min_minutes int := 0;
  v_overtime_rounding_mode text := 'NONE';
  v_overtime_rounding_unit_minutes int := 0;

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
    time_profile_last_event_id,
    shift_start,
    shift_end,
    window_start,
    window_end
  INTO
    v_shift_start_local,
    v_shift_end_local,
    v_late_tolerance_min,
    v_early_tolerance_min,
    v_overtime_min_minutes,
    v_overtime_rounding_mode,
    v_overtime_rounding_unit_minutes,
    v_time_profile_last_event_id,
    v_shift_start,
    v_shift_end,
    v_window_start,
    v_window_end
  FROM staffing.get_time_profile_for_work_date(p_tenant_id, p_work_date);

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

  PERFORM staffing.recompute_time_bank_cycle(p_tenant_id, p_person_uuid, p_work_date);
END;
$$;

CREATE OR REPLACE FUNCTION staffing.recompute_daily_attendance_results_for_punch(
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_punch_time timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_tz text := 'Asia/Shanghai';
  v_local_date date;
  v_d1 date;
  v_d2 date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_punch_time IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'punch_time is required';
  END IF;

  v_local_date := (p_punch_time AT TIME ZONE v_tz)::date;
  v_d1 := v_local_date - 1;
  v_d2 := v_local_date;

  PERFORM staffing.recompute_daily_attendance_result(p_tenant_id, p_person_uuid, v_d1);
  PERFORM staffing.recompute_daily_attendance_result(p_tenant_id, p_person_uuid, v_d2);
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
