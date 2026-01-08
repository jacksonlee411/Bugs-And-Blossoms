-- +goose Up
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
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.person_uuid,
      e.assignment_type,
      e.payload,
      lead(e.effective_date) OVER (ORDER BY e.effective_date ASC, e.id ASC) AS next_effective
    FROM staffing.assignment_events e
    WHERE e.tenant_id = p_tenant_id AND e.assignment_id = p_assignment_id
    ORDER BY e.effective_date ASC, e.id ASC
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

CREATE OR REPLACE FUNCTION staffing.submit_payroll_run_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
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
  v_existing staffing.payroll_run_events%ROWTYPE;
  v_existing_run staffing.payroll_runs%ROWTYPE;
  v_payload jsonb;
  v_next_state text;
  v_now timestamptz;
  v_period_status text;
  v_pay_group text;
  v_period daterange;
  v_period_start date;
  v_period_end_excl date;
  v_period_days int;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_type is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','CALC_START','CALC_FINISH','CALC_FAIL','FINALIZE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
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

  v_lock_key := format('staffing:payroll:run:%s:%s', p_tenant_id, p_run_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing
  FROM staffing.payroll_run_events
  WHERE event_id = p_event_id;

  IF FOUND THEN
    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.run_id <> p_run_id
      OR v_existing.pay_period_id <> p_pay_period_id
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

  SELECT * INTO v_existing_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id AND id = p_run_id
  FOR UPDATE;

  IF p_event_type = 'CREATE' THEN
    IF FOUND THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_RUN_EXISTS', DETAIL = format('run_id=%s', p_run_id);
    END IF;

    SELECT status INTO v_period_status
    FROM staffing.pay_periods
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id
    FOR UPDATE;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
        DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END IF;
    IF v_period_status <> 'open' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_CLOSED',
        DETAIL = format('pay_period_id=%s status=%s', p_pay_period_id, v_period_status);
    END IF;

    v_next_state := 'draft';
  ELSE
    IF NOT FOUND THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND', DETAIL = format('run_id=%s', p_run_id);
    END IF;

    IF v_existing_run.pay_period_id <> p_pay_period_id THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('pay_period_id mismatch: run_id=%s run_pay_period_id=%s param_pay_period_id=%s', p_run_id, v_existing_run.pay_period_id, p_pay_period_id);
    END IF;

    IF v_existing_run.run_state = 'finalized' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_FINALIZED_READONLY',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    IF p_event_type = 'CALC_START' THEN
      IF v_existing_run.run_state NOT IN ('draft','failed') THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'calculating';
    ELSIF p_event_type = 'CALC_FINISH' THEN
      IF v_existing_run.run_state <> 'calculating' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'calculated';
    ELSIF p_event_type = 'CALC_FAIL' THEN
      IF v_existing_run.run_state <> 'calculating' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'failed';
    ELSIF p_event_type = 'FINALIZE' THEN
      IF v_existing_run.run_state <> 'calculated' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'finalized';
    ELSE
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', p_event_type);
    END IF;
  END IF;

  INSERT INTO staffing.payroll_run_events (
    event_id,
    tenant_id,
    run_id,
    pay_period_id,
    event_type,
    run_state,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_run_id,
    p_pay_period_id,
    p_event_type,
    v_next_state,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.payroll_run_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.run_id <> p_run_id
      OR v_existing.pay_period_id <> p_pay_period_id
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

  v_now := now();

  IF p_event_type = 'CREATE' THEN
    INSERT INTO staffing.payroll_runs (
      tenant_id,
      id,
      pay_period_id,
      run_state,
      needs_recalc,
      calc_started_at,
      calc_finished_at,
      finalized_at,
      last_event_id,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_id,
      p_run_id,
      p_pay_period_id,
      'draft',
      false,
      NULL,
      NULL,
      NULL,
      v_event_db_id,
      v_now,
      v_now
    );

    RETURN v_event_db_id;
  END IF;

  IF p_event_type = 'CALC_START' THEN
    UPDATE staffing.payroll_runs
    SET
      run_state = v_next_state,
      calc_started_at = v_now,
      calc_finished_at = NULL,
      last_event_id = v_event_db_id,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  ELSIF p_event_type = 'CALC_FINISH' THEN
    SELECT pay_group, period
    INTO v_pay_group, v_period
    FROM staffing.pay_periods
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id
    FOR UPDATE;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
        DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END IF;

    IF v_pay_group <> 'monthly' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_UNSUPPORTED_PAY_GROUP',
        DETAIL = format('pay_group=%s', v_pay_group);
    END IF;

    v_period_start := lower(v_period);
    v_period_end_excl := upper(v_period);
    IF v_period_start IS NULL OR v_period_end_excl IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_UNSUPPORTED_PAY_PERIOD',
        DETAIL = format('period=%s', v_period);
    END IF;

    IF date_trunc('month', v_period_start)::date <> v_period_start
      OR (v_period_start + interval '1 month')::date <> v_period_end_excl
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_UNSUPPORTED_PAY_PERIOD',
        DETAIL = format('period=%s', v_period);
    END IF;

    v_period_days := v_period_end_excl - v_period_start;
    IF v_period_days <= 0 THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_UNSUPPORTED_PAY_PERIOD',
        DETAIL = format('period=%s', v_period);
    END IF;

    IF EXISTS (
      SELECT 1
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.assignment_type = 'primary'
        AND av.status = 'active'
        AND av.validity && v_period
        AND (av.allocated_fte <= 0 OR av.allocated_fte > 1)
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_INVALID_ALLOCATED_FTE',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    IF EXISTS (
      SELECT 1
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.assignment_type = 'primary'
        AND av.status = 'active'
        AND av.validity && v_period
        AND av.base_salary IS NULL
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_MISSING_BASE_SALARY',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    IF EXISTS (
      SELECT 1
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.assignment_type = 'primary'
        AND av.status = 'active'
        AND av.validity && v_period
        AND av.currency <> 'CNY'
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_UNSUPPORTED_CURRENCY',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    INSERT INTO staffing.payslips (
      tenant_id,
      id,
      run_id,
      pay_period_id,
      person_uuid,
      assignment_id,
      currency,
      gross_pay,
      net_pay,
      employer_total,
      last_run_event_id,
      created_at,
      updated_at
    )
    SELECT
      p_tenant_id,
      gen_random_uuid(),
      p_run_id,
      p_pay_period_id,
      av.person_uuid,
      av.assignment_id,
      'CNY',
      0,
      0,
      0,
      v_event_db_id,
      v_now,
      v_now
    FROM staffing.assignment_versions av
    WHERE av.tenant_id = p_tenant_id
      AND av.assignment_type = 'primary'
      AND av.status = 'active'
      AND av.validity && v_period
    GROUP BY av.person_uuid, av.assignment_id
    ON CONFLICT ON CONSTRAINT payslips_run_person_assignment_unique
    DO UPDATE SET
      pay_period_id = EXCLUDED.pay_period_id,
      currency = EXCLUDED.currency,
      last_run_event_id = EXCLUDED.last_run_event_id,
      updated_at = EXCLUDED.updated_at;

    DELETE FROM staffing.payslip_items i
    USING staffing.payslips p
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND i.tenant_id = p_tenant_id
      AND i.payslip_id = p.id;

    DELETE FROM staffing.payslips p
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND NOT EXISTS (
        SELECT 1
        FROM staffing.assignment_versions av
        WHERE av.tenant_id = p_tenant_id
          AND av.assignment_type = 'primary'
          AND av.status = 'active'
          AND av.validity && v_period
          AND av.person_uuid = p.person_uuid
          AND av.assignment_id = p.assignment_id
      );

    INSERT INTO staffing.payslip_items (
      tenant_id,
      payslip_id,
      item_code,
      item_kind,
      amount,
      meta,
      last_run_event_id
    )
    SELECT
      p_tenant_id,
      p.id,
      'EARNING_BASE_SALARY',
      'earning',
      round(
        av.base_salary * av.allocated_fte
          * (least(coalesce(upper(av.validity), v_period_end_excl), v_period_end_excl) - greatest(lower(av.validity), v_period_start))::numeric
          / v_period_days::numeric,
        2
      ) AS amount,
      jsonb_build_object(
        'pay_group', v_pay_group,
        'period_start', v_period_start::text,
        'period_end_exclusive', v_period_end_excl::text,
        'segment_start', greatest(lower(av.validity), v_period_start)::text,
        'segment_end_exclusive', least(coalesce(upper(av.validity), v_period_end_excl), v_period_end_excl)::text,
        'base_salary', av.base_salary::text,
        'allocated_fte', av.allocated_fte::text,
        'overlap_days', (least(coalesce(upper(av.validity), v_period_end_excl), v_period_end_excl) - greatest(lower(av.validity), v_period_start))::text,
        'period_days', v_period_days::text,
        'ratio', ((least(coalesce(upper(av.validity), v_period_end_excl), v_period_end_excl) - greatest(lower(av.validity), v_period_start))::numeric / v_period_days::numeric)::text
      ),
      v_event_db_id
    FROM staffing.assignment_versions av
    JOIN staffing.payslips p
      ON p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND p.person_uuid = av.person_uuid
      AND p.assignment_id = av.assignment_id
    WHERE av.tenant_id = p_tenant_id
      AND av.assignment_type = 'primary'
      AND av.status = 'active'
      AND av.validity && v_period;

    WITH sums AS (
      SELECT
        p.id AS payslip_id,
        COALESCE(sum(i.amount) FILTER (WHERE i.item_kind = 'earning'), 0) AS gross
      FROM staffing.payslips p
      LEFT JOIN staffing.payslip_items i
        ON i.tenant_id = p.tenant_id AND i.payslip_id = p.id
      WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
      GROUP BY p.id
    )
    UPDATE staffing.payslips p
    SET
      gross_pay = sums.gross,
      net_pay = sums.gross,
      employer_total = 0,
      last_run_event_id = v_event_db_id,
      updated_at = v_now
    FROM sums
    WHERE p.tenant_id = p_tenant_id AND p.id = sums.payslip_id;

    UPDATE staffing.payroll_runs
    SET
      run_state = v_next_state,
      calc_finished_at = v_now,
      last_event_id = v_event_db_id,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  ELSIF p_event_type = 'CALC_FAIL' THEN
    UPDATE staffing.payroll_runs
    SET
      run_state = v_next_state,
      last_event_id = v_event_db_id,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  ELSIF p_event_type = 'FINALIZE' THEN
    BEGIN
      UPDATE staffing.payroll_runs
      SET
        run_state = v_next_state,
        finalized_at = v_now,
        last_event_id = v_event_db_id,
        updated_at = v_now
      WHERE tenant_id = p_tenant_id AND id = p_run_id;
    EXCEPTION
      WHEN unique_violation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_ALREADY_FINALIZED',
          DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END;

    UPDATE staffing.pay_periods
    SET
      status = 'closed',
      closed_at = COALESCE(closed_at, v_now),
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;
  ELSE
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unexpected event_type: %s', p_event_type);
  END IF;

  RETURN v_event_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
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
  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.person_uuid,
      e.assignment_type,
      e.payload,
      lead(e.effective_date) OVER (ORDER BY e.effective_date ASC, e.id ASC) AS next_effective
    FROM staffing.assignment_events e
    WHERE e.tenant_id = p_tenant_id AND e.assignment_id = p_assignment_id
    ORDER BY e.effective_date ASC, e.id ASC
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

    INSERT INTO staffing.assignment_versions (
      tenant_id,
      assignment_id,
      person_uuid,
      position_id,
      assignment_type,
      status,
      allocated_fte,
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
      1.0,
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

CREATE OR REPLACE FUNCTION staffing.submit_payroll_run_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
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
  v_existing staffing.payroll_run_events%ROWTYPE;
  v_existing_run staffing.payroll_runs%ROWTYPE;
  v_payload jsonb;
  v_next_state text;
  v_now timestamptz;
  v_period_status text;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_type is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','CALC_START','CALC_FINISH','CALC_FAIL','FINALIZE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
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

  v_lock_key := format('staffing:payroll:run:%s:%s', p_tenant_id, p_run_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing
  FROM staffing.payroll_run_events
  WHERE event_id = p_event_id;

  IF FOUND THEN
    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.run_id <> p_run_id
      OR v_existing.pay_period_id <> p_pay_period_id
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

  SELECT * INTO v_existing_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id AND id = p_run_id
  FOR UPDATE;

  IF p_event_type = 'CREATE' THEN
    IF FOUND THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_RUN_EXISTS', DETAIL = format('run_id=%s', p_run_id);
    END IF;

    SELECT status INTO v_period_status
    FROM staffing.pay_periods
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id
    FOR UPDATE;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
        DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END IF;
    IF v_period_status <> 'open' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_CLOSED',
        DETAIL = format('pay_period_id=%s status=%s', p_pay_period_id, v_period_status);
    END IF;

    v_next_state := 'draft';
  ELSE
    IF NOT FOUND THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND', DETAIL = format('run_id=%s', p_run_id);
    END IF;

    IF v_existing_run.pay_period_id <> p_pay_period_id THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('pay_period_id mismatch: run_id=%s run_pay_period_id=%s param_pay_period_id=%s', p_run_id, v_existing_run.pay_period_id, p_pay_period_id);
    END IF;

    IF v_existing_run.run_state = 'finalized' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_FINALIZED_READONLY',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    IF p_event_type = 'CALC_START' THEN
      IF v_existing_run.run_state NOT IN ('draft','failed') THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'calculating';
    ELSIF p_event_type = 'CALC_FINISH' THEN
      IF v_existing_run.run_state <> 'calculating' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'calculated';
    ELSIF p_event_type = 'CALC_FAIL' THEN
      IF v_existing_run.run_state <> 'calculating' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'failed';
    ELSIF p_event_type = 'FINALIZE' THEN
      IF v_existing_run.run_state <> 'calculated' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
          DETAIL = format('run_id=%s current=%s event=%s', p_run_id, v_existing_run.run_state, p_event_type);
      END IF;
      v_next_state := 'finalized';
    ELSE
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', p_event_type);
    END IF;
  END IF;

  INSERT INTO staffing.payroll_run_events (
    event_id,
    tenant_id,
    run_id,
    pay_period_id,
    event_type,
    run_state,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_run_id,
    p_pay_period_id,
    p_event_type,
    v_next_state,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.payroll_run_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.run_id <> p_run_id
      OR v_existing.pay_period_id <> p_pay_period_id
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

  v_now := now();

  IF p_event_type = 'CREATE' THEN
    INSERT INTO staffing.payroll_runs (
      tenant_id,
      id,
      pay_period_id,
      run_state,
      needs_recalc,
      calc_started_at,
      calc_finished_at,
      finalized_at,
      last_event_id,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_id,
      p_run_id,
      p_pay_period_id,
      'draft',
      false,
      NULL,
      NULL,
      NULL,
      v_event_db_id,
      v_now,
      v_now
    );

    RETURN v_event_db_id;
  END IF;

  IF p_event_type = 'CALC_START' THEN
    UPDATE staffing.payroll_runs
    SET
      run_state = v_next_state,
      calc_started_at = v_now,
      calc_finished_at = NULL,
      last_event_id = v_event_db_id,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  ELSIF p_event_type = 'CALC_FINISH' THEN
    UPDATE staffing.payroll_runs
    SET
      run_state = v_next_state,
      calc_finished_at = v_now,
      last_event_id = v_event_db_id,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  ELSIF p_event_type = 'CALC_FAIL' THEN
    UPDATE staffing.payroll_runs
    SET
      run_state = v_next_state,
      last_event_id = v_event_db_id,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  ELSIF p_event_type = 'FINALIZE' THEN
    BEGIN
      UPDATE staffing.payroll_runs
      SET
        run_state = v_next_state,
        finalized_at = v_now,
        last_event_id = v_event_db_id,
        updated_at = v_now
      WHERE tenant_id = p_tenant_id AND id = p_run_id;
    EXCEPTION
      WHEN unique_violation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_RUN_ALREADY_FINALIZED',
          DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END;

    UPDATE staffing.pay_periods
    SET
      status = 'closed',
      closed_at = COALESCE(closed_at, v_now),
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;
  ELSE
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unexpected event_type: %s', p_event_type);
  END IF;

  RETURN v_event_db_id;
END;
$$;
-- +goose StatementEnd
