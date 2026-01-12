-- +goose Up
-- Fix dev DB drift: ensure replay_assignment_versions enforces stable fail-closed errors (e.g. missing position).
-- +goose StatementBegin
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
-- +goose StatementEnd
