CREATE OR REPLACE FUNCTION staffing.submit_payroll_pay_period_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_pay_period_id uuid,
  p_pay_group text,
  p_period daterange,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing staffing.pay_period_events%ROWTYPE;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_pay_group IS NULL OR btrim(p_pay_group) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_group is required';
  END IF;
  IF p_pay_group <> btrim(p_pay_group) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_group must be trimmed';
  END IF;
  IF p_pay_group <> lower(p_pay_group) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_group must be lower';
  END IF;
  IF p_period IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'period is required';
  END IF;
  IF isempty(p_period) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'period must be non-empty';
  END IF;
  IF NOT lower_inc(p_period) OR upper_inc(p_period) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'period must be [)';
  END IF;
  IF lower_inf(p_period) OR upper_inf(p_period) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'period must be bounded';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:payroll:pay_period:%s:%s', p_tenant_id, p_pay_group);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.pay_period_events (
    event_id,
    tenant_id,
    pay_period_id,
    event_type,
    pay_group,
    period,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_pay_period_id,
    'CREATE',
    p_pay_group,
    p_period,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.pay_period_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.pay_period_id <> p_pay_period_id
      OR v_existing.event_type <> 'CREATE'
      OR v_existing.pay_group <> p_pay_group
      OR v_existing.period <> p_period
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  BEGIN
    INSERT INTO staffing.pay_periods (
      tenant_id,
      id,
      pay_group,
      period,
      status,
      closed_at,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_pay_period_id,
      p_pay_group,
      p_period,
      'open',
      NULL,
      v_event_db_id
    );
  EXCEPTION
    WHEN unique_violation THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_EXISTS',
        DETAIL = format('pay_period_id=%s', p_pay_period_id);
    WHEN exclusion_violation THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_OVERLAP',
        DETAIL = format('pay_group=%s period=%s', p_pay_group, p_period);
  END;

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.payroll_run_events_after_insert_ensure_payslips_on_calc_finish()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v_period daterange;
  v_now timestamptz;
BEGIN
  PERFORM staffing.assert_current_tenant(NEW.tenant_id);

  SELECT period INTO v_period
  FROM staffing.pay_periods
  WHERE tenant_id = NEW.tenant_id AND id = NEW.pay_period_id;

  IF v_period IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', NEW.pay_period_id);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM staffing.assignment_versions av
    WHERE av.tenant_id = NEW.tenant_id
      AND av.assignment_type = 'primary'
      AND av.status = 'active'
      AND av.validity && v_period
      AND av.base_salary IS NULL
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_MISSING_BASE_SALARY',
      DETAIL = format('run_id=%s', NEW.run_id);
  END IF;

  v_now := now();

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
    NEW.tenant_id,
    gen_random_uuid(),
    NEW.run_id,
    NEW.pay_period_id,
    av.person_uuid,
    av.assignment_id,
    av.currency,
    0,
    0,
    0,
    NEW.id,
    v_now,
    v_now
  FROM staffing.assignment_versions av
  WHERE av.tenant_id = NEW.tenant_id
    AND av.assignment_type = 'primary'
    AND av.status = 'active'
    AND av.validity && v_period
  GROUP BY av.person_uuid, av.assignment_id, av.currency
  ON CONFLICT ON CONSTRAINT payslips_run_person_assignment_unique
  DO UPDATE SET
    pay_period_id = EXCLUDED.pay_period_id,
    currency = EXCLUDED.currency,
    last_run_event_id = EXCLUDED.last_run_event_id,
    updated_at = EXCLUDED.updated_at;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS payroll_run_events_calc_finish_ensure_payslips ON staffing.payroll_run_events;
CREATE TRIGGER payroll_run_events_calc_finish_ensure_payslips
AFTER INSERT ON staffing.payroll_run_events
FOR EACH ROW
WHEN (NEW.event_type = 'CALC_FINISH')
EXECUTE FUNCTION staffing.payroll_run_events_after_insert_ensure_payslips_on_calc_finish();

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

    -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-3 adds staffing.payroll_apply_social_insurance later).
    EXECUTE 'SELECT staffing.payroll_apply_social_insurance($1::uuid,$2::uuid,$3::uuid,$4::date,$5::date,$6::bigint,$7::timestamptz);'
    USING p_tenant_id, p_run_id, p_pay_period_id, v_period_start, v_period_end_excl, v_event_db_id, v_now;

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
