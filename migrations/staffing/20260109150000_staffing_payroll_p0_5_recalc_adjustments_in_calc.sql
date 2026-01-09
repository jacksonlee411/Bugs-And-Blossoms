-- +goose Up
-- +goose StatementBegin
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

  v_lock_key := format('staffing:payroll-run:%s:%s', p_tenant_id, p_run_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_now := now();

  BEGIN
    INSERT INTO staffing.payroll_run_events (
      event_id,
      tenant_id,
      run_id,
      pay_period_id,
      event_type,
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
      v_payload,
      p_request_id,
      p_initiator_id
    )
    ON CONFLICT (event_id) DO NOTHING
    RETURNING id INTO v_event_db_id;
  EXCEPTION
    WHEN unique_violation THEN
      RAISE;
  END;

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

  SELECT * INTO v_existing_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id AND id = p_run_id;

  IF p_event_type = 'CREATE' THEN
    IF FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_EXISTS',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    SELECT status, pay_group, period INTO v_period_status, v_pay_group, v_period
    FROM staffing.pay_periods
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;

    IF v_period_status IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
        DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END IF;
    IF v_period_status <> 'open' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_CLOSED',
        DETAIL = format('pay_period_id=%s status=%s', p_pay_period_id, v_period_status);
    END IF;

    INSERT INTO staffing.payroll_runs (
      tenant_id,
      id,
      pay_period_id,
      run_state,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_run_id,
      p_pay_period_id,
      'draft',
      v_event_db_id
    );

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
      last_run_event_id
    )
    SELECT
      p_tenant_id,
      gen_random_uuid(),
      p_run_id,
      p_pay_period_id,
      av.person_uuid,
      av.assignment_id,
      av.currency,
      0,
      0,
      0,
      v_event_db_id
    FROM staffing.assignment_versions av
    WHERE av.tenant_id = p_tenant_id
      AND av.assignment_type = 'primary'
      AND av.status = 'active'
      AND av.validity @> lower(v_period)
      AND av.base_salary IS NOT NULL;

    RETURN v_event_db_id;
  END IF;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND',
      DETAIL = format('run_id=%s', p_run_id);
  END IF;

  IF v_existing_run.pay_period_id <> p_pay_period_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_PAY_PERIOD_MISMATCH',
      DETAIL = format('run_id=%s pay_period_id=%s existing_pay_period_id=%s', p_run_id, p_pay_period_id, v_existing_run.pay_period_id);
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
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'calculating';
  ELSIF p_event_type = 'CALC_FINISH' THEN
    IF v_existing_run.run_state <> 'calculating' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'calculated';
  ELSIF p_event_type = 'CALC_FAIL' THEN
    IF v_existing_run.run_state <> 'calculating' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'failed';
  ELSIF p_event_type = 'FINALIZE' THEN
    IF v_existing_run.run_state <> 'calculated' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'finalized';
  ELSE
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unexpected event_type: %s', p_event_type);
  END IF;

  SELECT status, pay_group, period INTO v_period_status, v_pay_group, v_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;

  IF v_period_status IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', p_pay_period_id);
  END IF;

  v_period_start := lower(v_period);
  v_period_end_excl := upper(v_period);
  v_period_days := (v_period_end_excl - v_period_start);

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
    IF v_pay_group <> 'monthly' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_GROUP_NOT_SUPPORTED',
        DETAIL = format('pay_group=%s', v_pay_group);
    END IF;
    IF v_period_start <> date_trunc('month', v_period_start)::date OR v_period_end_excl <> (date_trunc('month', v_period_start)::date + INTERVAL '1 month')::date THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PERIOD_NOT_NATURAL_MONTH',
        DETAIL = format('period_start=%s period_end_exclusive=%s', v_period_start, v_period_end_excl);
    END IF;

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
	      a.item_code,
	      a.item_kind,
	      a.amount,
	      a.meta || jsonb_build_object(
	        'recalc_request_id', a.recalc_request_id::text,
	        'application_id', a.application_id::text,
	        'origin_pay_period_id', a.origin_pay_period_id::text,
	        'origin_run_id', a.origin_run_id::text,
	        'origin_payslip_id', a.origin_payslip_id
	      ),
	      v_event_db_id
	    FROM staffing.payslips p
	    JOIN staffing.payroll_adjustments a
	      ON a.tenant_id = p.tenant_id
	      AND a.target_run_id = p.run_id
	      AND a.person_uuid = p.person_uuid
	      AND a.assignment_id = p.assignment_id
	    WHERE p.tenant_id = p_tenant_id
	      AND p.run_id = p_run_id;

	    WITH sums AS (
	      SELECT
	        p.id AS payslip_id,
	        COALESCE(sum(i.amount) FILTER (WHERE i.item_kind = 'earning'), 0) AS gross,
	        COALESCE(sum(i.amount) FILTER (WHERE i.item_kind = 'deduction'), 0) AS deductions,
	        COALESCE(sum(i.amount) FILTER (WHERE i.item_kind = 'employer_cost'), 0) AS employer_cost
	      FROM staffing.payslips p
	      LEFT JOIN staffing.payslip_items i
	        ON i.tenant_id = p.tenant_id AND i.payslip_id = p.id
	      WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
      GROUP BY p.id
    )
	    UPDATE staffing.payslips p
	    SET
	      gross_pay = sums.gross,
	      net_pay = sums.gross - sums.deductions,
	      employer_total = sums.employer_cost,
	      last_run_event_id = v_event_db_id,
	      updated_at = v_now
	    FROM sums
	    WHERE p.tenant_id = p_tenant_id AND p.id = sums.payslip_id;

    -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-3 adds staffing.payroll_apply_social_insurance later).
    EXECUTE 'SELECT staffing.payroll_apply_social_insurance($1::uuid,$2::uuid,$3::uuid,$4::date,$5::date,$6::bigint,$7::timestamptz);'
    USING p_tenant_id, p_run_id, p_pay_period_id, v_period_start, v_period_end_excl, v_event_db_id, v_now;

    -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-4 adds staffing.payroll_apply_iit later).
    EXECUTE 'SELECT staffing.payroll_apply_iit($1::uuid,$2::uuid,$3::uuid,$4::bigint,$5::timestamptz);'
    USING p_tenant_id, p_run_id, p_pay_period_id, v_event_db_id, v_now;

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
    -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-4 adds staffing.payroll_post_iit_balances later).
    EXECUTE 'SELECT staffing.payroll_post_iit_balances($1::uuid,$2::uuid,$3::uuid,$4::bigint,$5::timestamptz);'
    USING p_tenant_id, p_run_id, p_pay_period_id, v_event_db_id, v_now;

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

CREATE OR REPLACE FUNCTION staffing.payroll_apply_social_insurance(
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
  p_period_start date,
  p_period_end_excl date,
  p_run_event_db_id bigint,
  p_now timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_types int;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_period_start IS NULL OR p_period_end_excl IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'period bounds are required';
  END IF;
  IF p_period_end_excl <= p_period_start THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'invalid period bounds';
  END IF;
  IF p_run_event_db_id IS NULL OR p_run_event_db_id <= 0 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_event_db_id is required';
  END IF;
  IF p_now IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'now is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM staffing.social_insurance_policies
    WHERE tenant_id = p_tenant_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_MISSING',
      DETAIL = format('tenant_id=%s', p_tenant_id);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM staffing.social_insurance_policy_events e
    WHERE e.tenant_id = p_tenant_id
      AND e.effective_date > p_period_start
      AND e.effective_date < p_period_end_excl
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD',
      DETAIL = format('period_start=%s period_end_exclusive=%s', p_period_start, p_period_end_excl);
  END IF;

  SELECT count(DISTINCT insurance_type) INTO v_types
  FROM staffing.social_insurance_policy_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.validity @> p_period_start;

  IF v_types <> 6 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF',
      DETAIL = format('as_of=%s types_found=%s', p_period_start, v_types);
  END IF;

  DELETE FROM staffing.payslip_social_insurance_items i
  USING staffing.payslips p
  WHERE p.tenant_id = p_tenant_id
    AND p.run_id = p_run_id
    AND i.tenant_id = p_tenant_id
    AND i.payslip_id = p.id;

  WITH policy_as_of AS (
    SELECT
      v.policy_id,
      v.city_code,
      v.hukou_type,
      v.insurance_type,
      v.employer_rate,
      v.employee_rate,
      v.base_floor,
      v.base_ceiling,
      v.rounding_rule,
      v.precision,
      v.rules_config,
      v.validity,
      v.last_event_id
    FROM staffing.social_insurance_policy_versions v
    WHERE v.tenant_id = p_tenant_id
      AND v.validity @> p_period_start
  )
  INSERT INTO staffing.payslip_social_insurance_items (
    tenant_id,
    payslip_id,
    run_id,
    pay_period_id,
    person_uuid,
    assignment_id,
    city_code,
    hukou_type,
    insurance_type,
    base_amount,
    employee_amount,
    employer_amount,
    currency,
    policy_id,
    policy_last_event_id,
    last_run_event_id,
    meta,
    created_at,
    updated_at
  )
  SELECT
    p_tenant_id,
    p.id,
    p_run_id,
    p_pay_period_id,
    p.person_uuid,
    p.assignment_id,
    pol.city_code,
    pol.hukou_type,
    pol.insurance_type,
    GREATEST(pol.base_floor, LEAST(p.gross_pay, pol.base_ceiling)) AS base_amount,
    staffing.round_by_rule(
      GREATEST(pol.base_floor, LEAST(p.gross_pay, pol.base_ceiling)) * pol.employee_rate,
      pol.rounding_rule,
      pol.precision
    ) AS employee_amount,
    staffing.round_by_rule(
      GREATEST(pol.base_floor, LEAST(p.gross_pay, pol.base_ceiling)) * pol.employer_rate,
      pol.rounding_rule,
      pol.precision
    ) AS employer_amount,
    p.currency,
    pol.policy_id,
    pol.last_event_id,
    p_run_event_db_id,
    jsonb_build_object(
      'as_of', p_period_start::text,
      'policy_effective_date', lower(pol.validity)::text,
      'employer_rate', pol.employer_rate::text,
      'employee_rate', pol.employee_rate::text,
      'base_floor', pol.base_floor::text,
      'base_ceiling', pol.base_ceiling::text,
      'rounding_rule', pol.rounding_rule,
      'precision', pol.precision::text,
      'gross_pay', p.gross_pay::text
    ),
    p_now,
    p_now
  FROM staffing.payslips p
  CROSS JOIN policy_as_of pol
  WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id;

  WITH sums AS (
    SELECT
      p.id AS payslip_id,
      COALESCE(sum(i.employee_amount), 0) AS employee_total,
      COALESCE(sum(i.employer_amount), 0) AS employer_total
    FROM staffing.payslips p
    LEFT JOIN staffing.payslip_social_insurance_items i
      ON i.tenant_id = p.tenant_id AND i.payslip_id = p.id
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
    GROUP BY p.id
  )
  UPDATE staffing.payslips p
  SET
    net_pay = p.net_pay - sums.employee_total,
    employer_total = p.employer_total + sums.employer_total,
    last_run_event_id = p_run_event_db_id,
    updated_at = p_now
  FROM sums
  WHERE p.tenant_id = p_tenant_id AND p.id = sums.payslip_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
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

  v_lock_key := format('staffing:payroll-run:%s:%s', p_tenant_id, p_run_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_now := now();

  BEGIN
    INSERT INTO staffing.payroll_run_events (
      event_id,
      tenant_id,
      run_id,
      pay_period_id,
      event_type,
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
      v_payload,
      p_request_id,
      p_initiator_id
    )
    ON CONFLICT (event_id) DO NOTHING
    RETURNING id INTO v_event_db_id;
  EXCEPTION
    WHEN unique_violation THEN
      RAISE;
  END;

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

  SELECT * INTO v_existing_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id AND id = p_run_id;

  IF p_event_type = 'CREATE' THEN
    IF FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_EXISTS',
        DETAIL = format('run_id=%s', p_run_id);
    END IF;

    SELECT status, pay_group, period INTO v_period_status, v_pay_group, v_period
    FROM staffing.pay_periods
    WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;

    IF v_period_status IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
        DETAIL = format('pay_period_id=%s', p_pay_period_id);
    END IF;
    IF v_period_status <> 'open' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_CLOSED',
        DETAIL = format('pay_period_id=%s status=%s', p_pay_period_id, v_period_status);
    END IF;

    INSERT INTO staffing.payroll_runs (
      tenant_id,
      id,
      pay_period_id,
      run_state,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_run_id,
      p_pay_period_id,
      'draft',
      v_event_db_id
    );

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
      last_run_event_id
    )
    SELECT
      p_tenant_id,
      gen_random_uuid(),
      p_run_id,
      p_pay_period_id,
      av.person_uuid,
      av.assignment_id,
      av.currency,
      0,
      0,
      0,
      v_event_db_id
    FROM staffing.assignment_versions av
    WHERE av.tenant_id = p_tenant_id
      AND av.assignment_type = 'primary'
      AND av.status = 'active'
      AND av.validity @> lower(v_period)
      AND av.base_salary IS NOT NULL;

    RETURN v_event_db_id;
  END IF;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND',
      DETAIL = format('run_id=%s', p_run_id);
  END IF;

  IF v_existing_run.pay_period_id <> p_pay_period_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_PAY_PERIOD_MISMATCH',
      DETAIL = format('run_id=%s pay_period_id=%s existing_pay_period_id=%s', p_run_id, p_pay_period_id, v_existing_run.pay_period_id);
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
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'calculating';
  ELSIF p_event_type = 'CALC_FINISH' THEN
    IF v_existing_run.run_state <> 'calculating' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'calculated';
  ELSIF p_event_type = 'CALC_FAIL' THEN
    IF v_existing_run.run_state <> 'calculating' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'failed';
  ELSIF p_event_type = 'FINALIZE' THEN
    IF v_existing_run.run_state <> 'calculated' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RUN_INVALID_TRANSITION',
        DETAIL = format('run_id=%s run_state=%s event_type=%s', p_run_id, v_existing_run.run_state, p_event_type);
    END IF;
    v_next_state := 'finalized';
  ELSE
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unexpected event_type: %s', p_event_type);
  END IF;

  SELECT status, pay_group, period INTO v_period_status, v_pay_group, v_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;

  IF v_period_status IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', p_pay_period_id);
  END IF;

  v_period_start := lower(v_period);
  v_period_end_excl := upper(v_period);
  v_period_days := (v_period_end_excl - v_period_start);

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
    IF v_pay_group <> 'monthly' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PAY_GROUP_NOT_SUPPORTED',
        DETAIL = format('pay_group=%s', v_pay_group);
    END IF;
    IF v_period_start <> date_trunc('month', v_period_start)::date OR v_period_end_excl <> (date_trunc('month', v_period_start)::date + INTERVAL '1 month')::date THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_PERIOD_NOT_NATURAL_MONTH',
        DETAIL = format('period_start=%s period_end_exclusive=%s', v_period_start, v_period_end_excl);
    END IF;

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

    -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-4 adds staffing.payroll_apply_iit later).
    EXECUTE 'SELECT staffing.payroll_apply_iit($1::uuid,$2::uuid,$3::uuid,$4::bigint,$5::timestamptz);'
    USING p_tenant_id, p_run_id, p_pay_period_id, v_event_db_id, v_now;

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
    -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-4 adds staffing.payroll_post_iit_balances later).
    EXECUTE 'SELECT staffing.payroll_post_iit_balances($1::uuid,$2::uuid,$3::uuid,$4::bigint,$5::timestamptz);'
    USING p_tenant_id, p_run_id, p_pay_period_id, v_event_db_id, v_now;

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

CREATE OR REPLACE FUNCTION staffing.payroll_apply_social_insurance(
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
  p_period_start date,
  p_period_end_excl date,
  p_run_event_db_id bigint,
  p_now timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_types int;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_period_start IS NULL OR p_period_end_excl IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'period bounds are required';
  END IF;
  IF p_period_end_excl <= p_period_start THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'invalid period bounds';
  END IF;
  IF p_run_event_db_id IS NULL OR p_run_event_db_id <= 0 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_event_db_id is required';
  END IF;
  IF p_now IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'now is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM staffing.social_insurance_policies
    WHERE tenant_id = p_tenant_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_MISSING',
      DETAIL = format('tenant_id=%s', p_tenant_id);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM staffing.social_insurance_policy_events e
    WHERE e.tenant_id = p_tenant_id
      AND e.effective_date > p_period_start
      AND e.effective_date < p_period_end_excl
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_CHANGED_WITHIN_PERIOD',
      DETAIL = format('period_start=%s period_end_exclusive=%s', p_period_start, p_period_end_excl);
  END IF;

  SELECT count(DISTINCT insurance_type) INTO v_types
  FROM staffing.social_insurance_policy_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.validity @> p_period_start;

  IF v_types <> 6 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_NOT_FOUND_AS_OF',
      DETAIL = format('as_of=%s types_found=%s', p_period_start, v_types);
  END IF;

  DELETE FROM staffing.payslip_social_insurance_items i
  USING staffing.payslips p
  WHERE p.tenant_id = p_tenant_id
    AND p.run_id = p_run_id
    AND i.tenant_id = p_tenant_id
    AND i.payslip_id = p.id;

  WITH policy_as_of AS (
    SELECT
      v.policy_id,
      v.city_code,
      v.hukou_type,
      v.insurance_type,
      v.employer_rate,
      v.employee_rate,
      v.base_floor,
      v.base_ceiling,
      v.rounding_rule,
      v.precision,
      v.rules_config,
      v.validity,
      v.last_event_id
    FROM staffing.social_insurance_policy_versions v
    WHERE v.tenant_id = p_tenant_id
      AND v.validity @> p_period_start
  )
  INSERT INTO staffing.payslip_social_insurance_items (
    tenant_id,
    payslip_id,
    run_id,
    pay_period_id,
    person_uuid,
    assignment_id,
    city_code,
    hukou_type,
    insurance_type,
    base_amount,
    employee_amount,
    employer_amount,
    currency,
    policy_id,
    policy_last_event_id,
    last_run_event_id,
    meta,
    created_at,
    updated_at
  )
  SELECT
    p_tenant_id,
    p.id,
    p_run_id,
    p_pay_period_id,
    p.person_uuid,
    p.assignment_id,
    pol.city_code,
    pol.hukou_type,
    pol.insurance_type,
    GREATEST(pol.base_floor, LEAST(p.gross_pay, pol.base_ceiling)) AS base_amount,
    staffing.round_by_rule(
      GREATEST(pol.base_floor, LEAST(p.gross_pay, pol.base_ceiling)) * pol.employee_rate,
      pol.rounding_rule,
      pol.precision
    ) AS employee_amount,
    staffing.round_by_rule(
      GREATEST(pol.base_floor, LEAST(p.gross_pay, pol.base_ceiling)) * pol.employer_rate,
      pol.rounding_rule,
      pol.precision
    ) AS employer_amount,
    p.currency,
    pol.policy_id,
    pol.last_event_id,
    p_run_event_db_id,
    jsonb_build_object(
      'as_of', p_period_start::text,
      'policy_effective_date', lower(pol.validity)::text,
      'employer_rate', pol.employer_rate::text,
      'employee_rate', pol.employee_rate::text,
      'base_floor', pol.base_floor::text,
      'base_ceiling', pol.base_ceiling::text,
      'rounding_rule', pol.rounding_rule,
      'precision', pol.precision::text,
      'gross_pay', p.gross_pay::text
    ),
    p_now,
    p_now
  FROM staffing.payslips p
  CROSS JOIN policy_as_of pol
  WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id;

  WITH sums AS (
    SELECT
      p.id AS payslip_id,
      COALESCE(sum(i.employee_amount), 0) AS employee_total,
      COALESCE(sum(i.employer_amount), 0) AS employer_total
    FROM staffing.payslips p
    LEFT JOIN staffing.payslip_social_insurance_items i
      ON i.tenant_id = p.tenant_id AND i.payslip_id = p.id
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
    GROUP BY p.id
  )
  UPDATE staffing.payslips p
  SET
    net_pay = p.gross_pay - sums.employee_total,
    employer_total = sums.employer_total,
    last_run_event_id = p_run_event_db_id,
    updated_at = p_now
  FROM sums
  WHERE p.tenant_id = p_tenant_id AND p.id = sums.payslip_id;
END;
$$;
-- +goose StatementEnd
