-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.round_by_rule(
  p_value numeric,
  p_rounding_rule text,
  p_precision smallint
)
RETURNS numeric
LANGUAGE plpgsql
AS $$
DECLARE
  v_scale numeric;
BEGIN
  IF p_rounding_rule IS NULL OR btrim(p_rounding_rule) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'rounding_rule is required';
  END IF;
  IF p_rounding_rule NOT IN ('HALF_UP','CEIL') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported rounding_rule: %s', p_rounding_rule);
  END IF;
  IF p_precision IS NULL OR p_precision < 0 OR p_precision > 2 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('precision out of range: %s', p_precision);
  END IF;

  IF p_rounding_rule = 'HALF_UP' THEN
    RETURN round(p_value, p_precision);
  END IF;

  v_scale := power(10::numeric, p_precision);
  RETURN ceiling(p_value * v_scale) / v_scale;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.replay_social_insurance_policy_versions(
  p_tenant_id uuid,
  p_policy_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_prev_effective date;
  v_last_validity daterange;
  v_row RECORD;
  v_validity daterange;
  v_employer_rate numeric(9,6);
  v_employee_rate numeric(9,6);
  v_base_floor numeric(15,2);
  v_base_ceiling numeric(15,2);
  v_rounding_rule text;
  v_precision smallint;
  v_rules_config jsonb;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_policy_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'policy_id is required';
  END IF;

  v_lock_key := format('staffing:social_insurance_policy:%s:%s', p_tenant_id, p_policy_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM staffing.social_insurance_policy_versions
  WHERE tenant_id = p_tenant_id AND policy_id = p_policy_id;

  v_prev_effective := NULL;

  FOR v_row IN
    SELECT
      e.id AS event_db_id,
      e.event_type,
      e.effective_date,
      e.city_code,
      e.hukou_type,
      e.insurance_type,
      e.payload,
      lead(e.effective_date) OVER (ORDER BY e.effective_date ASC, e.id ASC) AS next_effective
    FROM staffing.social_insurance_policy_events e
    WHERE e.tenant_id = p_tenant_id AND e.policy_id = p_policy_id
    ORDER BY e.effective_date ASC, e.id ASC
  LOOP
    IF v_row.event_type = 'CREATE' THEN
      IF v_prev_effective IS NOT NULL THEN
        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_EVENT', DETAIL = 'CREATE must be the first event';
      END IF;
    ELSIF v_row.event_type = 'UPDATE' THEN
      IF v_prev_effective IS NULL THEN
        RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_EVENT', DETAIL = 'UPDATE requires prior state';
      END IF;
    ELSE
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unexpected event_type: %s', v_row.event_type);
    END IF;

    IF jsonb_typeof(v_row.payload) <> 'object'
      OR NOT (v_row.payload ? 'employer_rate')
      OR NOT (v_row.payload ? 'employee_rate')
      OR NOT (v_row.payload ? 'base_floor')
      OR NOT (v_row.payload ? 'base_ceiling')
      OR NOT (v_row.payload ? 'rounding_rule')
      OR NOT (v_row.payload ? 'precision')
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
        DETAIL = format('policy_id=%s event_db_id=%s', p_policy_id, v_row.event_db_id);
    END IF;

    BEGIN
      v_employer_rate := (v_row.payload->>'employer_rate')::numeric;
      v_employee_rate := (v_row.payload->>'employee_rate')::numeric;
      v_base_floor := (v_row.payload->>'base_floor')::numeric;
      v_base_ceiling := (v_row.payload->>'base_ceiling')::numeric;
      v_rounding_rule := btrim(v_row.payload->>'rounding_rule');
      v_precision := (v_row.payload->>'precision')::smallint;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
          DETAIL = format('policy_id=%s event_db_id=%s', p_policy_id, v_row.event_db_id);
    END;

    IF v_employer_rate < 0 OR v_employer_rate > 1 OR v_employee_rate < 0 OR v_employee_rate > 1 THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
        DETAIL = format('policy_id=%s event_db_id=%s rate out of range', p_policy_id, v_row.event_db_id);
    END IF;
    IF v_base_floor < 0 OR v_base_ceiling < v_base_floor THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
        DETAIL = format('policy_id=%s event_db_id=%s base_floor/base_ceiling invalid', p_policy_id, v_row.event_db_id);
    END IF;
    IF v_rounding_rule IS NULL OR v_rounding_rule = '' OR v_rounding_rule NOT IN ('HALF_UP','CEIL') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
        DETAIL = format('policy_id=%s event_db_id=%s rounding_rule invalid', p_policy_id, v_row.event_db_id);
    END IF;
    IF v_precision < 0 OR v_precision > 2 THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
        DETAIL = format('policy_id=%s event_db_id=%s precision invalid', p_policy_id, v_row.event_db_id);
    END IF;

    IF v_row.payload ? 'rules_config' THEN
      v_rules_config := v_row.payload->'rules_config';
      IF jsonb_typeof(v_rules_config) <> 'object' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
          DETAIL = format('policy_id=%s event_db_id=%s rules_config must be object', p_policy_id, v_row.event_db_id);
      END IF;
    ELSE
      v_rules_config := '{}'::jsonb;
    END IF;

    IF v_row.next_effective IS NULL THEN
      v_validity := daterange(v_row.effective_date, NULL, '[)');
    ELSE
      v_validity := daterange(v_row.effective_date, v_row.next_effective, '[)');
    END IF;

    INSERT INTO staffing.social_insurance_policy_versions (
      tenant_id,
      policy_id,
      city_code,
      hukou_type,
      insurance_type,
      employer_rate,
      employee_rate,
      base_floor,
      base_ceiling,
      rounding_rule,
      precision,
      rules_config,
      validity,
      last_event_id
    )
    VALUES (
      p_tenant_id,
      p_policy_id,
      v_row.city_code,
      v_row.hukou_type,
      v_row.insurance_type,
      v_employer_rate,
      v_employee_rate,
      v_base_floor,
      v_base_ceiling,
      v_rounding_rule,
      v_precision,
      v_rules_config,
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
      FROM staffing.social_insurance_policy_versions
      WHERE tenant_id = p_tenant_id AND policy_id = p_policy_id
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_VALIDITY_GAP',
      DETAIL = 'social_insurance_policy_versions must be gapless';
  END IF;

  SELECT validity INTO v_last_validity
  FROM staffing.social_insurance_policy_versions
  WHERE tenant_id = p_tenant_id AND policy_id = p_policy_id
  ORDER BY lower(validity) DESC
  LIMIT 1;

  IF v_last_validity IS NOT NULL AND NOT upper_inf(v_last_validity) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_VALIDITY_NOT_INFINITE',
      DETAIL = 'last social_insurance_policy_versions validity must be unbounded (infinity)';
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.submit_social_insurance_policy_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_policy_id uuid,
  p_city_code text,
  p_hukou_type text,
  p_insurance_type text,
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
  v_existing staffing.social_insurance_policy_events%ROWTYPE;
  v_payload jsonb;
  v_existing_city_code text;
  v_identity_policy_id uuid;
  v_constraint text;
  v_employer_rate numeric(9,6);
  v_employee_rate numeric(9,6);
  v_base_floor numeric(15,2);
  v_base_ceiling numeric(15,2);
  v_rounding_rule text;
  v_precision smallint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_policy_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'policy_id is required';
  END IF;
  IF p_city_code IS NULL OR btrim(p_city_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'city_code is required';
  END IF;
  IF p_city_code <> btrim(p_city_code) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'city_code must be trimmed';
  END IF;
  IF p_city_code <> upper(p_city_code) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'city_code must be upper';
  END IF;
  IF p_hukou_type IS NULL OR btrim(p_hukou_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'hukou_type is required';
  END IF;
  IF p_hukou_type <> btrim(p_hukou_type) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'hukou_type must be trimmed';
  END IF;
  IF p_hukou_type <> lower(p_hukou_type) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'hukou_type must be lower';
  END IF;
  IF p_hukou_type <> 'default' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_HUKOU_TYPE_NOT_SUPPORTED',
      DETAIL = format('hukou_type=%s', p_hukou_type);
  END IF;
  IF p_insurance_type IS NULL OR btrim(p_insurance_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'insurance_type is required';
  END IF;
  IF p_insurance_type NOT IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND') THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported insurance_type: %s', p_insurance_type);
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_type is required';
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

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;
  IF NOT (v_payload ? 'employer_rate')
    OR NOT (v_payload ? 'employee_rate')
    OR NOT (v_payload ? 'base_floor')
    OR NOT (v_payload ? 'base_ceiling')
    OR NOT (v_payload ? 'rounding_rule')
    OR NOT (v_payload ? 'precision')
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
      DETAIL = format('policy_id=%s', p_policy_id);
  END IF;
  BEGIN
    v_employer_rate := (v_payload->>'employer_rate')::numeric;
    v_employee_rate := (v_payload->>'employee_rate')::numeric;
    v_base_floor := (v_payload->>'base_floor')::numeric;
    v_base_ceiling := (v_payload->>'base_ceiling')::numeric;
    v_rounding_rule := btrim(v_payload->>'rounding_rule');
    v_precision := (v_payload->>'precision')::smallint;
  EXCEPTION
    WHEN invalid_text_representation THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
        DETAIL = format('policy_id=%s', p_policy_id);
  END;
  IF v_employer_rate < 0 OR v_employer_rate > 1 OR v_employee_rate < 0 OR v_employee_rate > 1 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
      DETAIL = format('policy_id=%s rate out of range', p_policy_id);
  END IF;
  IF v_base_floor < 0 OR v_base_ceiling < v_base_floor THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
      DETAIL = format('policy_id=%s base_floor/base_ceiling invalid', p_policy_id);
  END IF;
  IF v_rounding_rule IS NULL OR v_rounding_rule = '' OR v_rounding_rule NOT IN ('HALF_UP','CEIL') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
      DETAIL = format('policy_id=%s rounding_rule invalid', p_policy_id);
  END IF;
  IF v_precision < 0 OR v_precision > 2 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
      DETAIL = format('policy_id=%s precision invalid', p_policy_id);
  END IF;
  IF v_payload ? 'rules_config' AND jsonb_typeof(v_payload->'rules_config') <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_PAYLOAD_REQUIRED',
      DETAIL = format('policy_id=%s rules_config must be object', p_policy_id);
  END IF;

  SELECT city_code INTO v_existing_city_code
  FROM staffing.social_insurance_policies
  WHERE tenant_id = p_tenant_id
  LIMIT 1;
  IF FOUND AND v_existing_city_code <> p_city_code THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_SI_MULTI_CITY_NOT_SUPPORTED',
      DETAIL = format('existing_city_code=%s requested_city_code=%s', v_existing_city_code, p_city_code);
  END IF;

  INSERT INTO staffing.social_insurance_policies (
    tenant_id,
    id,
    city_code,
    hukou_type,
    insurance_type
  )
  VALUES (
    p_tenant_id,
    p_policy_id,
    p_city_code,
    p_hukou_type,
    p_insurance_type
  )
  ON CONFLICT (tenant_id, city_code, hukou_type, insurance_type) DO NOTHING;

  SELECT id INTO v_identity_policy_id
  FROM staffing.social_insurance_policies
  WHERE tenant_id = p_tenant_id
    AND city_code = p_city_code
    AND hukou_type = p_hukou_type
    AND insurance_type = p_insurance_type;

  IF v_identity_policy_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'policy identity missing after upsert';
  END IF;
  IF v_identity_policy_id <> p_policy_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_SOCIAL_INSURANCE_POLICY_ID_MISMATCH',
      DETAIL = format('policy_id=%s existing_policy_id=%s', p_policy_id, v_identity_policy_id);
  END IF;

  v_lock_key := format('staffing:social_insurance_policy:%s:%s', p_tenant_id, p_policy_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  BEGIN
    INSERT INTO staffing.social_insurance_policy_events (
      event_id,
      tenant_id,
      policy_id,
      city_code,
      hukou_type,
      insurance_type,
      event_type,
      effective_date,
      payload,
      request_id,
      initiator_id
    )
    VALUES (
      p_event_id,
      p_tenant_id,
      p_policy_id,
      p_city_code,
      p_hukou_type,
      p_insurance_type,
      p_event_type,
      p_effective_date,
      v_payload,
      p_request_id,
      p_initiator_id
    )
    ON CONFLICT (event_id) DO NOTHING
    RETURNING id INTO v_event_db_id;
  EXCEPTION
    WHEN unique_violation THEN
      GET STACKED DIAGNOSTICS v_constraint = CONSTRAINT_NAME;
      IF v_constraint = 'social_insurance_policy_events_one_per_day_unique' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_SI_POLICY_EVENT_ONE_PER_DAY_CONFLICT',
          DETAIL = format('policy_id=%s effective_date=%s', p_policy_id, p_effective_date);
      END IF;
      RAISE;
  END;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.social_insurance_policy_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.policy_id <> p_policy_id
      OR v_existing.city_code <> p_city_code
      OR v_existing.hukou_type <> p_hukou_type
      OR v_existing.insurance_type <> p_insurance_type
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

  PERFORM staffing.replay_social_insurance_policy_versions(p_tenant_id, p_policy_id);

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
-- restore submit_payroll_run_event (P0-2: no social insurance apply)
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

DROP FUNCTION IF EXISTS staffing.payroll_apply_social_insurance(uuid, uuid, uuid, date, date, bigint, timestamptz);
DROP FUNCTION IF EXISTS staffing.submit_social_insurance_policy_event(uuid, uuid, uuid, text, text, text, text, date, jsonb, text, uuid);
DROP FUNCTION IF EXISTS staffing.replay_social_insurance_policy_versions(uuid, uuid);
DROP FUNCTION IF EXISTS staffing.round_by_rule(numeric, text, smallint);
-- +goose StatementEnd
