-- Payroll (P0-5) retroactive accounting kernel functions

CREATE OR REPLACE FUNCTION staffing.maybe_create_payroll_recalc_request_from_assignment_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_person_uuid uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_hit_pay_period_id uuid;
  v_hit_run_id uuid;
  v_hit_payslip_id uuid;
  v_recalc_request_id uuid;
  v_payload jsonb;
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
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF btrim(coalesce(p_request_id, '')) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  IF p_event_type = 'UPDATE' THEN
    v_payload := COALESCE(p_payload, '{}'::jsonb);
    IF jsonb_typeof(v_payload) <> 'object' THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be object';
    END IF;
    IF NOT (
      v_payload ? 'status'
      OR v_payload ? 'allocated_fte'
      OR v_payload ? 'base_salary'
      OR v_payload ? 'currency'
    ) THEN
      RETURN NULL;
    END IF;
  END IF;

  SELECT id INTO v_hit_pay_period_id
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id
    AND status = 'closed'
    AND upper(period) > p_effective_date
  ORDER BY lower(period) ASC, id ASC
  LIMIT 1;

  IF v_hit_pay_period_id IS NULL THEN
    RETURN NULL;
  END IF;

  SELECT id INTO v_hit_run_id
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id
    AND pay_period_id = v_hit_pay_period_id
    AND run_state = 'finalized'
  ORDER BY created_at DESC, id ASC
  LIMIT 1;

  IF v_hit_run_id IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_HIT_FINALIZED_BUT_MISSING_RUN',
      DETAIL = format('hit_pay_period_id=%s', v_hit_pay_period_id);
  END IF;

  SELECT id INTO v_hit_payslip_id
  FROM staffing.payslips
  WHERE tenant_id = p_tenant_id
    AND run_id = v_hit_run_id
    AND person_uuid = p_person_uuid
    AND assignment_id = p_assignment_id
  LIMIT 1;

  INSERT INTO staffing.payroll_recalc_requests (
    tenant_id,
    trigger_event_id,
    trigger_source,
    person_uuid,
    assignment_id,
    effective_date,
    hit_pay_period_id,
    hit_run_id,
    hit_payslip_id,
    request_id,
    initiator_id
  )
  VALUES (
    p_tenant_id,
    p_event_id,
    'assignment',
    p_person_uuid,
    p_assignment_id,
    p_effective_date,
    v_hit_pay_period_id,
    v_hit_run_id,
    v_hit_payslip_id,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (tenant_id, trigger_event_id) DO NOTHING
  RETURNING recalc_request_id INTO v_recalc_request_id;

  IF v_recalc_request_id IS NULL THEN
    SELECT recalc_request_id INTO v_recalc_request_id
    FROM staffing.payroll_recalc_requests
    WHERE tenant_id = p_tenant_id
      AND trigger_event_id = p_event_id;
  END IF;

  RETURN v_recalc_request_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_payroll_recalc_apply_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_recalc_request_id uuid,
  p_target_run_id uuid,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_now timestamptz;

  v_req staffing.payroll_recalc_requests%ROWTYPE;
  v_existing_app staffing.payroll_recalc_applications%ROWTYPE;
  v_target_run staffing.payroll_runs%ROWTYPE;
  v_target_period staffing.pay_periods%ROWTYPE;
  v_hit_period staffing.pay_periods%ROWTYPE;

  v_target_period_start date;
  v_origin_tax_year integer;
  v_target_tax_year integer;

  v_app_id bigint;
  v_inserted int := 0;

  v_origin record;
  v_period_start date;
  v_period_end_excl date;
  v_period_days int;
  v_origin_payslip_id uuid;

  v_original_base_salary numeric(15,2);
  v_already_forwarded_base_salary numeric(15,2);
  v_recalculated_base_salary numeric(15,2);
  v_delta numeric(15,2);
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_recalc_request_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'recalc_request_id is required';
  END IF;
  IF p_target_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'target_run_id is required';
  END IF;
  IF btrim(coalesce(p_request_id, '')) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:payroll:recalc:apply:%s:%s', p_tenant_id, p_recalc_request_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_req
  FROM staffing.payroll_recalc_requests
  WHERE tenant_id = p_tenant_id
    AND recalc_request_id = p_recalc_request_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_REQUEST_NOT_FOUND',
      DETAIL = format('recalc_request_id=%s', p_recalc_request_id);
  END IF;

  SELECT * INTO v_existing_app
  FROM staffing.payroll_recalc_applications
  WHERE tenant_id = p_tenant_id
    AND recalc_request_id = p_recalc_request_id;

  IF FOUND THEN
    IF v_existing_app.event_id <> p_event_id THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_RECALC_ALREADY_APPLIED',
        DETAIL = format('recalc_request_id=%s', p_recalc_request_id);
    END IF;
    RETURN v_existing_app.id;
  END IF;

  SELECT * INTO v_target_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id
    AND id = p_target_run_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND',
      DETAIL = format('run_id=%s', p_target_run_id);
  END IF;

  IF v_target_run.run_state NOT IN ('draft','failed') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_TARGET_RUN_NOT_EDITABLE',
      DETAIL = format('target_run_id=%s run_state=%s', p_target_run_id, v_target_run.run_state);
  END IF;

  SELECT * INTO v_target_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id
    AND id = v_target_run.pay_period_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', v_target_run.pay_period_id);
  END IF;

  IF v_target_period.status <> 'open' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_TARGET_PERIOD_CLOSED',
      DETAIL = format('target_pay_period_id=%s status=%s', v_target_run.pay_period_id, v_target_period.status);
  END IF;

  SELECT * INTO v_hit_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id
    AND id = v_req.hit_pay_period_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('hit_pay_period_id=%s', v_req.hit_pay_period_id);
  END IF;

  IF v_target_period.pay_group <> v_hit_period.pay_group THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_PAY_GROUP_MISMATCH',
      DETAIL = format('hit_pay_group=%s target_pay_group=%s', v_hit_period.pay_group, v_target_period.pay_group);
  END IF;

  v_origin_tax_year := extract(year from lower(v_hit_period.period))::integer;
  v_target_tax_year := extract(year from lower(v_target_period.period))::integer;
  IF v_origin_tax_year <> v_target_tax_year THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_CROSS_TAX_YEAR_UNSUPPORTED',
      DETAIL = format('origin_tax_year=%s target_tax_year=%s', v_origin_tax_year, v_target_tax_year);
  END IF;

  v_now := now();

  INSERT INTO staffing.payroll_recalc_applications (
    event_id,
    tenant_id,
    recalc_request_id,
    target_run_id,
    target_pay_period_id,
    request_id,
    initiator_id,
    transaction_time,
    created_at
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_recalc_request_id,
    p_target_run_id,
    v_target_run.pay_period_id,
    p_request_id,
    p_initiator_id,
    v_now,
    v_now
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_app_id;

  IF v_app_id IS NULL THEN
    SELECT * INTO v_existing_app
    FROM staffing.payroll_recalc_applications
    WHERE event_id = p_event_id;

    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s', p_event_id);
    END IF;

    IF v_existing_app.tenant_id <> p_tenant_id
      OR v_existing_app.recalc_request_id <> p_recalc_request_id
      OR v_existing_app.target_run_id <> p_target_run_id
      OR v_existing_app.target_pay_period_id <> v_target_run.pay_period_id
      OR v_existing_app.request_id <> p_request_id
      OR v_existing_app.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_app.id);
    END IF;

    RETURN v_existing_app.id;
  END IF;

  v_target_period_start := lower(v_target_period.period);

  FOR v_origin IN
    SELECT
      pp.id AS pay_period_id,
      pp.period AS period,
      r.id AS run_id
    FROM staffing.pay_periods pp
    JOIN staffing.payroll_runs r
      ON r.tenant_id = pp.tenant_id
      AND r.pay_period_id = pp.id
      AND r.run_state = 'finalized'
    WHERE pp.tenant_id = p_tenant_id
      AND pp.status = 'closed'
      AND pp.pay_group = v_hit_period.pay_group
      AND upper(pp.period) > v_req.effective_date
      AND lower(pp.period) < v_target_period_start
    ORDER BY lower(pp.period) ASC, pp.id ASC
  LOOP
    v_period_start := lower(v_origin.period);
    v_period_end_excl := upper(v_origin.period);
    v_period_days := (v_period_end_excl - v_period_start);

    SELECT id INTO v_origin_payslip_id
    FROM staffing.payslips
    WHERE tenant_id = p_tenant_id
      AND run_id = v_origin.run_id
      AND person_uuid = v_req.person_uuid
      AND assignment_id = v_req.assignment_id
    LIMIT 1;

    IF v_origin_payslip_id IS NULL THEN
      v_original_base_salary := 0;
    ELSE
      SELECT COALESCE(sum(amount), 0) INTO v_original_base_salary
      FROM staffing.payslip_items
      WHERE tenant_id = p_tenant_id
        AND payslip_id = v_origin_payslip_id
        AND item_kind = 'earning'
        AND item_code = 'EARNING_BASE_SALARY';
    END IF;

    SELECT COALESCE(sum(amount), 0) INTO v_already_forwarded_base_salary
    FROM staffing.payroll_adjustments
    WHERE tenant_id = p_tenant_id
      AND person_uuid = v_req.person_uuid
      AND assignment_id = v_req.assignment_id
      AND origin_pay_period_id = v_origin.pay_period_id
      AND item_kind = 'earning'
      AND item_code = 'EARNING_BASE_SALARY';

    IF EXISTS (
      SELECT 1
      FROM staffing.assignment_versions av
      WHERE av.tenant_id = p_tenant_id
        AND av.assignment_id = v_req.assignment_id
        AND av.assignment_type = 'primary'
        AND av.status = 'active'
        AND av.validity && v_origin.period
        AND av.base_salary IS NULL
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_MISSING_BASE_SALARY',
        DETAIL = format('assignment_id=%s origin_pay_period_id=%s', v_req.assignment_id, v_origin.pay_period_id);
    END IF;

    SELECT COALESCE(sum(
      round(
        av.base_salary * av.allocated_fte
          * (least(coalesce(upper(av.validity), v_period_end_excl), v_period_end_excl) - greatest(lower(av.validity), v_period_start))::numeric
          / v_period_days::numeric,
        2
      )
    ), 0) INTO v_recalculated_base_salary
    FROM staffing.assignment_versions av
    WHERE av.tenant_id = p_tenant_id
      AND av.assignment_id = v_req.assignment_id
      AND av.person_uuid = v_req.person_uuid
      AND av.assignment_type = 'primary'
      AND av.status = 'active'
      AND av.validity && v_origin.period
      AND av.base_salary IS NOT NULL;

    v_delta := round(v_recalculated_base_salary - (v_original_base_salary + v_already_forwarded_base_salary), 2);

    IF v_delta <> 0 THEN
      INSERT INTO staffing.payroll_adjustments (
        tenant_id,
        application_id,
        recalc_request_id,
        target_run_id,
        target_pay_period_id,
        person_uuid,
        assignment_id,
        origin_pay_period_id,
        origin_run_id,
        origin_payslip_id,
        item_kind,
        item_code,
        amount,
        meta,
        created_at
      )
      VALUES (
        p_tenant_id,
        v_app_id,
        p_recalc_request_id,
        p_target_run_id,
        v_target_run.pay_period_id,
        v_req.person_uuid,
        v_req.assignment_id,
        v_origin.pay_period_id,
        v_origin.run_id,
        v_origin_payslip_id,
        'earning',
        'EARNING_BASE_SALARY',
        v_delta,
        jsonb_build_object(
          'trigger_event_id', v_req.trigger_event_id::text
        ),
        v_now
      );
      v_inserted := v_inserted + 1;
    END IF;
  END LOOP;

  IF v_inserted = 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RECALC_NOTHING_TO_APPLY',
      DETAIL = format('recalc_request_id=%s', p_recalc_request_id);
  END IF;

  RETURN v_app_id;
END;
$$;
