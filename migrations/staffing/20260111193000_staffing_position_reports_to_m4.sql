-- +goose Up
-- +goose StatementBegin
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
  v_name text;
  v_lifecycle_status text;
  v_reports_to_status text;
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
      v_name,
      v_lifecycle_status,
      1.0,
      '{}'::jsonb,
      v_validity,
      v_row.event_db_id
    );

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
-- +goose StatementEnd

-- +goose StatementBegin
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
-- +goose StatementEnd
