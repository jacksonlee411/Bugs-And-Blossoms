CREATE OR REPLACE FUNCTION staffing.submit_iit_special_additional_deduction_claim_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_person_uuid uuid,
  p_tax_year integer,
  p_tax_month smallint,
  p_amount numeric,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_event_db_id bigint;
  v_existing staffing.iit_special_additional_deduction_claim_events%ROWTYPE;
  v_now timestamptz;
  v_lock_key text;
  v_period_start date;
  v_period_end_excl date;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_tax_year IS NULL OR p_tax_year < 2000 OR p_tax_year > 9999 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'tax_year out of range';
  END IF;
  IF p_tax_month IS NULL OR p_tax_month < 1 OR p_tax_month > 12 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'tax_month out of range';
  END IF;
  IF p_amount IS NULL OR p_amount < 0 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'amount must be >= 0';
  END IF;
  IF btrim(coalesce(p_request_id, '')) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:iit:sad:%s:%s:%s:%s', p_tenant_id, p_person_uuid, p_tax_year, p_tax_month);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_period_start := make_date(p_tax_year, p_tax_month, 1);
  v_period_end_excl := (v_period_start + interval '1 month')::date;

  IF EXISTS (
    SELECT 1
    FROM staffing.payroll_runs r
    JOIN staffing.pay_periods pp
      ON pp.tenant_id = r.tenant_id AND pp.id = r.pay_period_id
    WHERE r.tenant_id = p_tenant_id
      AND r.run_state = 'finalized'
      AND lower(pp.period) = v_period_start
      AND upper(pp.period) = v_period_end_excl
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_IIT_SAD_CLAIM_MONTH_FINALIZED',
      DETAIL = format('tax_year=%s tax_month=%s', p_tax_year, p_tax_month);
  END IF;

  INSERT INTO staffing.iit_special_additional_deduction_claim_events (
    event_id,
    tenant_id,
    person_uuid,
    tax_year,
    tax_month,
    amount,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_person_uuid,
    p_tax_year,
    p_tax_month,
    p_amount,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.iit_special_additional_deduction_claim_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.tax_year <> p_tax_year
      OR v_existing.tax_month <> p_tax_month
      OR v_existing.amount <> p_amount
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

  INSERT INTO staffing.iit_special_additional_deduction_claims (
    tenant_id,
    person_uuid,
    tax_year,
    tax_month,
    amount,
    last_event_id,
    created_at,
    updated_at
  )
  VALUES (
    p_tenant_id,
    p_person_uuid,
    p_tax_year,
    p_tax_month,
    p_amount,
    v_event_db_id,
    v_now,
    v_now
  )
  ON CONFLICT (tenant_id, person_uuid, tax_year, tax_month)
  DO UPDATE SET
    amount = EXCLUDED.amount,
    last_event_id = EXCLUDED.last_event_id,
    updated_at = EXCLUDED.updated_at;

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.iit_compute_cumulative_withholding(
  p_ytd_income numeric,
  p_ytd_tax_exempt_income numeric,
  p_ytd_standard_deduction numeric,
  p_ytd_special_deduction numeric,
  p_ytd_special_additional_deduction numeric,
  p_effective_withheld numeric
)
RETURNS TABLE (
  taxable_income numeric,
  tax_liability numeric,
  delta numeric,
  withhold_this_month numeric,
  credit numeric,
  rate numeric,
  quick_deduction numeric
)
LANGUAGE plpgsql
AS $$
DECLARE
  v_taxable numeric;
  v_rate numeric;
  v_quick numeric;
  v_tax_liability numeric;
  v_delta numeric;
BEGIN
  v_taxable := greatest(
    0,
    coalesce(p_ytd_income, 0)
      - coalesce(p_ytd_tax_exempt_income, 0)
      - coalesce(p_ytd_standard_deduction, 0)
      - coalesce(p_ytd_special_deduction, 0)
      - coalesce(p_ytd_special_additional_deduction, 0)
  );

  IF v_taxable <= 36000 THEN
    v_rate := 0.03;
    v_quick := 0;
  ELSIF v_taxable <= 144000 THEN
    v_rate := 0.10;
    v_quick := 2520;
  ELSIF v_taxable <= 300000 THEN
    v_rate := 0.20;
    v_quick := 16920;
  ELSIF v_taxable <= 420000 THEN
    v_rate := 0.25;
    v_quick := 31920;
  ELSIF v_taxable <= 660000 THEN
    v_rate := 0.30;
    v_quick := 52920;
  ELSIF v_taxable <= 960000 THEN
    v_rate := 0.35;
    v_quick := 85920;
  ELSE
    v_rate := 0.45;
    v_quick := 181920;
  END IF;

  v_tax_liability := round(v_taxable * v_rate - v_quick, 2);
  IF v_tax_liability < 0 THEN
    v_tax_liability := 0;
  END IF;

  v_delta := v_tax_liability - coalesce(p_effective_withheld, 0);

  taxable_income := round(v_taxable, 2);
  tax_liability := v_tax_liability;
  delta := round(v_delta, 2);
  rate := v_rate;
  quick_deduction := v_quick;

  IF v_delta > 0 THEN
    withhold_this_month := round(v_delta, 2);
    credit := 0;
  ELSE
    withhold_this_month := 0;
    credit := round(-v_delta, 2);
  END IF;

  RETURN NEXT;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.payroll_apply_iit(
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
  p_run_event_db_id bigint,
  p_now timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_period daterange;
  v_period_start date;
  v_period_end_excl date;
  v_tax_year integer;
  v_tax_month smallint;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_run_event_db_id IS NULL OR p_run_event_db_id <= 0 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_event_db_id is required';
  END IF;
  IF p_now IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'now is required';
  END IF;

  SELECT period INTO v_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id AND id = p_pay_period_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', p_pay_period_id);
  END IF;

  v_period_start := lower(v_period);
  v_period_end_excl := upper(v_period);
  IF v_period_start IS NULL OR v_period_end_excl IS NULL
    OR date_trunc('month', v_period_start)::date <> v_period_start
    OR (v_period_start + interval '1 month')::date <> v_period_end_excl
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_IIT_PERIOD_NOT_MONTHLY',
      DETAIL = format('period=%s', v_period);
  END IF;

  v_tax_year := extract(year from v_period_start)::integer;
  v_tax_month := extract(month from v_period_start)::smallint;

  IF EXISTS (
    SELECT 1
    FROM staffing.payslips p
    JOIN staffing.payroll_balances b
      ON b.tenant_id = p.tenant_id
      AND b.tax_entity_id = p.tenant_id
      AND b.person_uuid = p.person_uuid
      AND b.tax_year = v_tax_year
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND v_tax_month <= b.last_tax_month
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_IIT_BALANCES_MONTH_NOT_ADVANCING',
      DETAIL = format('tax_year=%s tax_month=%s', v_tax_year, v_tax_month);
  END IF;

  WITH
  si AS (
    SELECT
      i.payslip_id,
      COALESCE(sum(i.employee_amount), 0) AS employee_amount
    FROM staffing.payslip_social_insurance_items i
    WHERE i.tenant_id = p_tenant_id AND i.run_id = p_run_id
    GROUP BY i.payslip_id
  ),
  sad AS (
    SELECT
      c.person_uuid,
      c.amount
    FROM staffing.iit_special_additional_deduction_claims c
    WHERE c.tenant_id = p_tenant_id
      AND c.tax_year = v_tax_year
      AND c.tax_month = v_tax_month
  ),
  calc AS (
    SELECT
      p.id AS payslip_id,
      p.person_uuid,
      p.gross_pay AS income_this_month,
      COALESCE(si.employee_amount, 0) AS si_employee_amount,
      COALESCE(sad.amount, 0) AS sad_amount_this_month,
      COALESCE(b.first_tax_month, v_tax_month) AS first_tax_month,
      COALESCE(b.ytd_income, 0) AS prev_ytd_income,
      COALESCE(b.ytd_tax_exempt_income, 0) AS prev_ytd_tax_exempt_income,
      COALESCE(b.ytd_special_deduction, 0) AS prev_ytd_special_deduction,
      COALESCE(b.ytd_special_additional_deduction, 0) AS prev_ytd_special_additional_deduction,
      COALESCE(b.ytd_iit_withheld, 0) AS prev_ytd_iit_withheld,
      COALESCE(b.ytd_iit_credit, 0) AS prev_ytd_iit_credit
    FROM staffing.payslips p
    LEFT JOIN staffing.payroll_balances b
      ON b.tenant_id = p.tenant_id
      AND b.tax_entity_id = p.tenant_id
      AND b.person_uuid = p.person_uuid
      AND b.tax_year = v_tax_year
    LEFT JOIN si ON si.payslip_id = p.id
    LEFT JOIN sad ON sad.person_uuid = p.person_uuid
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
  ),
  ytd AS (
    SELECT
      c.*,
      (c.prev_ytd_income + c.income_this_month) AS ytd_income,
      c.prev_ytd_tax_exempt_income AS ytd_tax_exempt_income,
      (c.prev_ytd_special_deduction + c.si_employee_amount) AS ytd_special_deduction,
      (c.prev_ytd_special_additional_deduction + c.sad_amount_this_month) AS ytd_special_additional_deduction,
      (5000::numeric * (v_tax_month - c.first_tax_month + 1)::numeric) AS ytd_standard_deduction,
      (c.prev_ytd_iit_withheld + c.prev_ytd_iit_credit) AS effective_withheld
    FROM calc c
  ),
  iit AS (
    SELECT
      y.*,
      t.taxable_income,
      t.tax_liability,
      t.delta,
      t.withhold_this_month,
      t.credit,
      t.rate,
      t.quick_deduction
    FROM ytd y
    CROSS JOIN LATERAL staffing.iit_compute_cumulative_withholding(
      y.ytd_income,
      y.ytd_tax_exempt_income,
      y.ytd_standard_deduction,
      y.ytd_special_deduction,
      y.ytd_special_additional_deduction,
      y.effective_withheld
    ) t
  )
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
    iit.payslip_id,
    'DEDUCTION_IIT_WITHHOLDING',
    'deduction',
    iit.withhold_this_month,
    jsonb_build_object(
      'tax_year', v_tax_year::text,
      'tax_month', v_tax_month::text,
      'first_tax_month', iit.first_tax_month::text,
      'income_this_month', iit.income_this_month::text,
      'si_employee_amount', iit.si_employee_amount::text,
      'sad_amount_this_month', iit.sad_amount_this_month::text,
      'ytd_income', iit.ytd_income::text,
      'ytd_tax_exempt_income', iit.ytd_tax_exempt_income::text,
      'ytd_standard_deduction', iit.ytd_standard_deduction::text,
      'ytd_special_deduction', iit.ytd_special_deduction::text,
      'ytd_special_additional_deduction', iit.ytd_special_additional_deduction::text,
      'taxable_income', iit.taxable_income::text,
      'rate', iit.rate::text,
      'quick_deduction', iit.quick_deduction::text,
      'tax_liability', iit.tax_liability::text,
      'effective_withheld', iit.effective_withheld::text,
      'delta', iit.delta::text,
      'withhold_this_month', iit.withhold_this_month::text,
      'credit', iit.credit::text
    ),
    p_run_event_db_id
  FROM iit;

  UPDATE staffing.payslips p
  SET
    net_pay = p.net_pay - iit.withhold_this_month,
    last_run_event_id = p_run_event_db_id,
    updated_at = p_now
  FROM (
    SELECT
      p.id AS payslip_id,
      i.amount AS withhold_this_month
    FROM staffing.payslips p
    JOIN staffing.payslip_items i
      ON i.tenant_id = p.tenant_id
      AND i.payslip_id = p.id
      AND i.item_code = 'DEDUCTION_IIT_WITHHOLDING'
      AND i.last_run_event_id = p_run_event_db_id
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
  ) AS iit
  WHERE p.tenant_id = p_tenant_id AND p.id = iit.payslip_id;
END;
$$;

CREATE OR REPLACE FUNCTION staffing.payroll_post_iit_balances(
  p_tenant_id uuid,
  p_run_id uuid,
  p_pay_period_id uuid,
  p_run_event_db_id bigint,
  p_now timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_period daterange;
  v_period_start date;
  v_period_end_excl date;
  v_tax_year integer;
  v_tax_month smallint;

  v_first_tax_month smallint;
  v_prev_last_tax_month smallint;
  v_prev_ytd_income numeric;
  v_prev_ytd_tax_exempt_income numeric;
  v_prev_ytd_special_deduction numeric;
  v_prev_ytd_special_additional_deduction numeric;
  v_prev_ytd_iit_withheld numeric;
  v_prev_ytd_iit_credit numeric;

  v_income_this_month numeric;
  v_si_employee_amount numeric;
  v_sad_amount_this_month numeric;

  v_ytd_income numeric;
  v_ytd_tax_exempt_income numeric;
  v_ytd_standard_deduction numeric;
  v_ytd_special_deduction numeric;
  v_ytd_special_additional_deduction numeric;

  v_taxable_income numeric;
  v_tax_liability numeric;
  v_delta numeric;
  v_withhold_this_month numeric;
  v_credit numeric;
  v_rate numeric;
  v_quick numeric;

  v_effective_withheld numeric;
  v_payslip_item_amount numeric;

  rec record;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_pay_period_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'pay_period_id is required';
  END IF;
  IF p_run_event_db_id IS NULL OR p_run_event_db_id <= 0 THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'run_event_db_id is required';
  END IF;
  IF p_now IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'now is required';
  END IF;

  SELECT period INTO v_period
  FROM staffing.pay_periods
  WHERE tenant_id = p_tenant_id AND id = p_pay_period_id
  FOR UPDATE;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_PAY_PERIOD_NOT_FOUND',
      DETAIL = format('pay_period_id=%s', p_pay_period_id);
  END IF;

  v_period_start := lower(v_period);
  v_period_end_excl := upper(v_period);
  IF v_period_start IS NULL OR v_period_end_excl IS NULL
    OR date_trunc('month', v_period_start)::date <> v_period_start
    OR (v_period_start + interval '1 month')::date <> v_period_end_excl
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_IIT_PERIOD_NOT_MONTHLY',
      DETAIL = format('period=%s', v_period);
  END IF;

  v_tax_year := extract(year from v_period_start)::integer;
  v_tax_month := extract(month from v_period_start)::smallint;

  FOR rec IN
    SELECT
      p.id AS payslip_id,
      p.person_uuid,
      p.gross_pay AS income_this_month,
      COALESCE((
        SELECT sum(i.employee_amount)
        FROM staffing.payslip_social_insurance_items i
        WHERE i.tenant_id = p.tenant_id AND i.payslip_id = p.id AND i.run_id = p_run_id
      ), 0) AS si_employee_amount,
      COALESCE((
        SELECT c.amount
        FROM staffing.iit_special_additional_deduction_claims c
        WHERE c.tenant_id = p.tenant_id
          AND c.person_uuid = p.person_uuid
          AND c.tax_year = v_tax_year
          AND c.tax_month = v_tax_month
      ), 0) AS sad_amount_this_month
    FROM staffing.payslips p
    WHERE p.tenant_id = p_tenant_id AND p.run_id = p_run_id
  LOOP
    SELECT amount INTO v_payslip_item_amount
    FROM staffing.payslip_items
    WHERE tenant_id = p_tenant_id
      AND payslip_id = rec.payslip_id
      AND item_code = 'DEDUCTION_IIT_WITHHOLDING'
    ORDER BY id DESC
    LIMIT 1;
    IF v_payslip_item_amount IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IIT_PAYSLIP_ITEM_MISSING',
        DETAIL = format('payslip_id=%s', rec.payslip_id);
    END IF;

    SELECT
      first_tax_month,
      last_tax_month,
      ytd_income,
      ytd_tax_exempt_income,
      ytd_special_deduction,
      ytd_special_additional_deduction,
      ytd_iit_withheld,
      ytd_iit_credit
    INTO
      v_first_tax_month,
      v_prev_last_tax_month,
      v_prev_ytd_income,
      v_prev_ytd_tax_exempt_income,
      v_prev_ytd_special_deduction,
      v_prev_ytd_special_additional_deduction,
      v_prev_ytd_iit_withheld,
      v_prev_ytd_iit_credit
    FROM staffing.payroll_balances
    WHERE tenant_id = p_tenant_id
      AND tax_entity_id = p_tenant_id
      AND person_uuid = rec.person_uuid
      AND tax_year = v_tax_year
    FOR UPDATE;

    IF NOT FOUND THEN
      v_first_tax_month := v_tax_month;
      v_prev_last_tax_month := 0;
      v_prev_ytd_income := 0;
      v_prev_ytd_tax_exempt_income := 0;
      v_prev_ytd_special_deduction := 0;
      v_prev_ytd_special_additional_deduction := 0;
      v_prev_ytd_iit_withheld := 0;
      v_prev_ytd_iit_credit := 0;
    ELSE
      IF v_tax_month <= v_prev_last_tax_month THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_IIT_BALANCES_MONTH_NOT_ADVANCING',
          DETAIL = format('tax_year=%s tax_month=%s last_tax_month=%s', v_tax_year, v_tax_month, v_prev_last_tax_month);
      END IF;
    END IF;

    v_income_this_month := rec.income_this_month;
    v_si_employee_amount := rec.si_employee_amount;
    v_sad_amount_this_month := rec.sad_amount_this_month;

    v_ytd_income := v_prev_ytd_income + v_income_this_month;
    v_ytd_tax_exempt_income := v_prev_ytd_tax_exempt_income;
    v_ytd_special_deduction := v_prev_ytd_special_deduction + v_si_employee_amount;
    v_ytd_special_additional_deduction := v_prev_ytd_special_additional_deduction + v_sad_amount_this_month;
    v_ytd_standard_deduction := 5000::numeric * (v_tax_month - v_first_tax_month + 1)::numeric;

    v_effective_withheld := v_prev_ytd_iit_withheld + v_prev_ytd_iit_credit;

    SELECT
      taxable_income,
      tax_liability,
      delta,
      withhold_this_month,
      credit,
      rate,
      quick_deduction
    INTO
      v_taxable_income,
      v_tax_liability,
      v_delta,
      v_withhold_this_month,
      v_credit,
      v_rate,
      v_quick
    FROM staffing.iit_compute_cumulative_withholding(
      v_ytd_income,
      v_ytd_tax_exempt_income,
      v_ytd_standard_deduction,
      v_ytd_special_deduction,
      v_ytd_special_additional_deduction,
      v_effective_withheld
    );

    IF v_payslip_item_amount <> v_withhold_this_month THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IIT_WITHHOLDING_MISMATCH_RECALC_REQUIRED',
        DETAIL = format('payslip_id=%s item=%s expected=%s', rec.payslip_id, v_payslip_item_amount::text, v_withhold_this_month::text);
    END IF;

    INSERT INTO staffing.payroll_balances (
      tenant_id,
      tax_entity_id,
      person_uuid,
      tax_year,
      first_tax_month,
      last_tax_month,
      ytd_income,
      ytd_tax_exempt_income,
      ytd_standard_deduction,
      ytd_special_deduction,
      ytd_special_additional_deduction,
      ytd_taxable_income,
      ytd_iit_tax_liability,
      ytd_iit_withheld,
      ytd_iit_credit,
      last_pay_period_id,
      last_run_id,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_id,
      p_tenant_id,
      rec.person_uuid,
      v_tax_year,
      v_first_tax_month,
      v_tax_month,
      v_ytd_income,
      v_ytd_tax_exempt_income,
      v_ytd_standard_deduction,
      v_ytd_special_deduction,
      v_ytd_special_additional_deduction,
      v_taxable_income,
      v_tax_liability,
      v_prev_ytd_iit_withheld + v_withhold_this_month,
      v_credit,
      p_pay_period_id,
      p_run_id,
      p_now,
      p_now
    )
    ON CONFLICT (tenant_id, tax_entity_id, person_uuid, tax_year)
    DO UPDATE SET
      last_tax_month = EXCLUDED.last_tax_month,
      ytd_income = EXCLUDED.ytd_income,
      ytd_tax_exempt_income = EXCLUDED.ytd_tax_exempt_income,
      ytd_standard_deduction = EXCLUDED.ytd_standard_deduction,
      ytd_special_deduction = EXCLUDED.ytd_special_deduction,
      ytd_special_additional_deduction = EXCLUDED.ytd_special_additional_deduction,
      ytd_taxable_income = EXCLUDED.ytd_taxable_income,
      ytd_iit_tax_liability = EXCLUDED.ytd_iit_tax_liability,
      ytd_iit_withheld = EXCLUDED.ytd_iit_withheld,
      ytd_iit_credit = EXCLUDED.ytd_iit_credit,
      last_pay_period_id = EXCLUDED.last_pay_period_id,
      last_run_id = EXCLUDED.last_run_id,
      updated_at = EXCLUDED.updated_at;
  END LOOP;
END;
$$;
