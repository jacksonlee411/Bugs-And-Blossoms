-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.iit_withhold_this_month_cents(
  p_ytd_income_cents bigint,
  p_ytd_tax_exempt_income_cents bigint,
  p_ytd_standard_deduction_cents bigint,
  p_ytd_special_deduction_cents bigint,
  p_ytd_special_additional_deduction_cents bigint,
  p_effective_withheld_cents bigint
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_withhold_this_month numeric;
BEGIN
  SELECT t.withhold_this_month INTO v_withhold_this_month
  FROM staffing.iit_compute_cumulative_withholding(
    round(coalesce(p_ytd_income_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_tax_exempt_income_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_standard_deduction_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_special_deduction_cents, 0)::numeric / 100, 2),
    round(coalesce(p_ytd_special_additional_deduction_cents, 0)::numeric / 100, 2),
    round(coalesce(p_effective_withheld_cents, 0)::numeric / 100, 2)
  ) t;

  RETURN round(coalesce(v_withhold_this_month, 0) * 100, 0)::bigint;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
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

  v_int64_max constant bigint := 9223372036854775807;
  v_net_guaranteed_payslip record;

  v_base_income_cents bigint;
  v_si_employee_cents bigint;
  v_sad_amount_cents bigint;
  v_first_tax_month smallint;

  v_prev_ytd_income_cents bigint;
  v_prev_ytd_tax_exempt_income_cents bigint;
  v_prev_ytd_special_deduction_cents bigint;
  v_prev_ytd_special_additional_deduction_cents bigint;
  v_prev_ytd_iit_withheld_cents bigint;
  v_prev_ytd_iit_credit_cents bigint;

  v_ytd_income_base_cents bigint;
  v_ytd_tax_exempt_income_cents bigint;
  v_ytd_standard_deduction_cents bigint;
  v_ytd_special_deduction_cents bigint;
  v_ytd_special_additional_deduction_cents bigint;
  v_effective_withheld_cents bigint;

  v_group_target_net_cents bigint;
  v_base_iit_withhold_cents bigint;
  v_test_iit_withhold_cents bigint;
  v_delta_iit_cents bigint;
  v_test_net_cents bigint;

  v_lo_cents bigint;
  v_hi_cents bigint;
  v_mid_cents bigint;
  v_expand int;
  v_iters int;
  v_solved_gross_cents bigint;
  v_group_delta_iit_cents bigint;
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

  FOR v_net_guaranteed_payslip IN
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
    )
    SELECT
      p.id AS payslip_id,
      p.person_uuid,
      p.assignment_id,
      round(p.gross_pay * 100, 0)::bigint AS base_income_cents,
      round(COALESCE(si.employee_amount, 0) * 100, 0)::bigint AS si_employee_cents,
      round(COALESCE(sad.amount, 0) * 100, 0)::bigint AS sad_amount_cents,
      COALESCE(b.first_tax_month, v_tax_month) AS first_tax_month,
      round(COALESCE(b.ytd_income, 0) * 100, 0)::bigint AS prev_ytd_income_cents,
      round(COALESCE(b.ytd_tax_exempt_income, 0) * 100, 0)::bigint AS prev_ytd_tax_exempt_income_cents,
      round(COALESCE(b.ytd_special_deduction, 0) * 100, 0)::bigint AS prev_ytd_special_deduction_cents,
      round(COALESCE(b.ytd_special_additional_deduction, 0) * 100, 0)::bigint AS prev_ytd_special_additional_deduction_cents,
      round(COALESCE(b.ytd_iit_withheld, 0) * 100, 0)::bigint AS prev_ytd_iit_withheld_cents,
      round(COALESCE(b.ytd_iit_credit, 0) * 100, 0)::bigint AS prev_ytd_iit_credit_cents
    FROM staffing.payslips p
    LEFT JOIN staffing.payroll_balances b
      ON b.tenant_id = p.tenant_id
      AND b.tax_entity_id = p.tenant_id
      AND b.person_uuid = p.person_uuid
      AND b.tax_year = v_tax_year
    LEFT JOIN si ON si.payslip_id = p.id
    LEFT JOIN sad ON sad.person_uuid = p.person_uuid
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND EXISTS (
        SELECT 1
        FROM staffing.payslip_item_inputs i
        WHERE i.tenant_id = p.tenant_id
          AND i.run_id = p.run_id
          AND i.person_uuid = p.person_uuid
          AND i.assignment_id = p.assignment_id
          AND i.calc_mode = 'net_guaranteed_iit'
      )
  LOOP
    v_base_income_cents := v_net_guaranteed_payslip.base_income_cents;
    v_si_employee_cents := v_net_guaranteed_payslip.si_employee_cents;
    v_sad_amount_cents := v_net_guaranteed_payslip.sad_amount_cents;
    v_first_tax_month := v_net_guaranteed_payslip.first_tax_month;

    v_prev_ytd_income_cents := v_net_guaranteed_payslip.prev_ytd_income_cents;
    v_prev_ytd_tax_exempt_income_cents := v_net_guaranteed_payslip.prev_ytd_tax_exempt_income_cents;
    v_prev_ytd_special_deduction_cents := v_net_guaranteed_payslip.prev_ytd_special_deduction_cents;
    v_prev_ytd_special_additional_deduction_cents := v_net_guaranteed_payslip.prev_ytd_special_additional_deduction_cents;
    v_prev_ytd_iit_withheld_cents := v_net_guaranteed_payslip.prev_ytd_iit_withheld_cents;
    v_prev_ytd_iit_credit_cents := v_net_guaranteed_payslip.prev_ytd_iit_credit_cents;

    SELECT round(sum(i.amount) * 100, 0)::bigint INTO v_group_target_net_cents
    FROM staffing.payslip_item_inputs i
    WHERE i.tenant_id = p_tenant_id
      AND i.run_id = p_run_id
      AND i.person_uuid = v_net_guaranteed_payslip.person_uuid
      AND i.assignment_id = v_net_guaranteed_payslip.assignment_id
      AND i.calc_mode = 'net_guaranteed_iit';

    IF v_group_target_net_cents IS NULL OR v_group_target_net_cents <= 0 THEN
      CONTINUE;
    END IF;

    v_ytd_income_base_cents := v_prev_ytd_income_cents + v_base_income_cents;
    v_ytd_tax_exempt_income_cents := v_prev_ytd_tax_exempt_income_cents;
    v_ytd_special_deduction_cents := v_prev_ytd_special_deduction_cents + v_si_employee_cents;
    v_ytd_special_additional_deduction_cents := v_prev_ytd_special_additional_deduction_cents + v_sad_amount_cents;
    v_ytd_standard_deduction_cents := 5000 * 100 * (v_tax_month - v_first_tax_month + 1);
    v_effective_withheld_cents := v_prev_ytd_iit_withheld_cents + v_prev_ytd_iit_credit_cents;

    v_base_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
      v_ytd_income_base_cents,
      v_ytd_tax_exempt_income_cents,
      v_ytd_standard_deduction_cents,
      v_ytd_special_deduction_cents,
      v_ytd_special_additional_deduction_cents,
      v_effective_withheld_cents
    );

    v_lo_cents := v_group_target_net_cents;
    v_hi_cents := v_group_target_net_cents;

    FOR v_expand IN 1..32 LOOP
      v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
        v_ytd_income_base_cents + v_hi_cents,
        v_ytd_tax_exempt_income_cents,
        v_ytd_standard_deduction_cents,
        v_ytd_special_deduction_cents,
        v_ytd_special_additional_deduction_cents,
        v_effective_withheld_cents
      );
      v_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;
      v_test_net_cents := v_hi_cents - v_delta_iit_cents;

      IF v_test_net_cents >= v_group_target_net_cents THEN
        EXIT;
      END IF;

      IF v_hi_cents > v_int64_max / 2 OR v_hi_cents > v_int64_max - v_ytd_income_base_cents THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_UPPER_BOUND_EXHAUSTED',
          DETAIL = format(
            'payslip_id=%s person_uuid=%s assignment_id=%s base_income=%s target_net=%s',
            v_net_guaranteed_payslip.payslip_id,
            v_net_guaranteed_payslip.person_uuid,
            v_net_guaranteed_payslip.assignment_id,
            (v_base_income_cents::numeric / 100)::text,
            (v_group_target_net_cents::numeric / 100)::text
          );
      END IF;

      v_hi_cents := v_hi_cents * 2;
    END LOOP;

    v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
      v_ytd_income_base_cents + v_hi_cents,
      v_ytd_tax_exempt_income_cents,
      v_ytd_standard_deduction_cents,
      v_ytd_special_deduction_cents,
      v_ytd_special_additional_deduction_cents,
      v_effective_withheld_cents
    );
    v_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;
    v_test_net_cents := v_hi_cents - v_delta_iit_cents;
    IF v_test_net_cents < v_group_target_net_cents THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_UPPER_BOUND_EXHAUSTED',
        DETAIL = format(
          'payslip_id=%s person_uuid=%s assignment_id=%s base_income=%s target_net=%s hi=%s hi_net=%s',
          v_net_guaranteed_payslip.payslip_id,
          v_net_guaranteed_payslip.person_uuid,
          v_net_guaranteed_payslip.assignment_id,
          (v_base_income_cents::numeric / 100)::text,
          (v_group_target_net_cents::numeric / 100)::text,
          (v_hi_cents::numeric / 100)::text,
          (v_test_net_cents::numeric / 100)::text
        );
    END IF;

    v_iters := 0;
    WHILE v_lo_cents < v_hi_cents LOOP
      v_iters := v_iters + 1;
      v_mid_cents := (v_lo_cents + v_hi_cents) / 2;

      v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
        v_ytd_income_base_cents + v_mid_cents,
        v_ytd_tax_exempt_income_cents,
        v_ytd_standard_deduction_cents,
        v_ytd_special_deduction_cents,
        v_ytd_special_additional_deduction_cents,
        v_effective_withheld_cents
      );
      v_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;
      v_test_net_cents := v_mid_cents - v_delta_iit_cents;

      IF v_test_net_cents >= v_group_target_net_cents THEN
        v_hi_cents := v_mid_cents;
      ELSE
        v_lo_cents := v_mid_cents + 1;
      END IF;
    END LOOP;

    v_solved_gross_cents := v_lo_cents;
    v_test_iit_withhold_cents := staffing.iit_withhold_this_month_cents(
      v_ytd_income_base_cents + v_solved_gross_cents,
      v_ytd_tax_exempt_income_cents,
      v_ytd_standard_deduction_cents,
      v_ytd_special_deduction_cents,
      v_ytd_special_additional_deduction_cents,
      v_effective_withheld_cents
    );
    v_group_delta_iit_cents := v_test_iit_withhold_cents - v_base_iit_withhold_cents;

    IF v_solved_gross_cents - v_group_delta_iit_cents <> v_group_target_net_cents THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_SOLVER_CONTRACT_VIOLATION',
        DETAIL = format(
          'payslip_id=%s person_uuid=%s assignment_id=%s target_net=%s solved_gross=%s delta_iit=%s',
          v_net_guaranteed_payslip.payslip_id,
          v_net_guaranteed_payslip.person_uuid,
          v_net_guaranteed_payslip.assignment_id,
          (v_group_target_net_cents::numeric / 100)::text,
          (v_solved_gross_cents::numeric / 100)::text,
          (v_group_delta_iit_cents::numeric / 100)::text
        );
    END IF;

    WITH
    inputs AS (
      SELECT
        i.id AS input_id,
        i.item_code,
        round(i.amount * 100, 0)::bigint AS target_net_cents,
        i.last_event_id AS input_last_event_id
      FROM staffing.payslip_item_inputs i
      WHERE i.tenant_id = p_tenant_id
        AND i.run_id = p_run_id
        AND i.person_uuid = v_net_guaranteed_payslip.person_uuid
        AND i.assignment_id = v_net_guaranteed_payslip.assignment_id
        AND i.calc_mode = 'net_guaranteed_iit'
    ),
    alloc_base AS (
      SELECT
        i.*,
        (v_group_delta_iit_cents::numeric * i.target_net_cents::numeric) AS mul,
        floor((v_group_delta_iit_cents::numeric * i.target_net_cents::numeric) / v_group_target_net_cents::numeric)::bigint AS q
      FROM inputs i
    ),
    alloc AS (
      SELECT
        a.*,
        (a.mul - (a.q::numeric * v_group_target_net_cents::numeric))::bigint AS r
      FROM alloc_base a
    ),
    residual AS (
      SELECT
        v_group_delta_iit_cents - sum(a.q) AS residual
      FROM alloc a
    ),
    ranked AS (
      SELECT
        a.*,
        row_number() OVER (ORDER BY a.r DESC, a.item_code ASC) AS rn,
        (SELECT residual FROM residual) AS residual
      FROM alloc a
    )
    INSERT INTO staffing.payslip_items (
      tenant_id,
      payslip_id,
      item_code,
      item_kind,
      amount,
      meta,
      last_run_event_id,
      calc_mode,
      tax_bearer,
      target_net,
      iit_delta
    )
    SELECT
      p_tenant_id,
      v_net_guaranteed_payslip.payslip_id,
      r.item_code,
      'earning',
      round(((r.target_net_cents + (r.q + CASE WHEN r.rn <= r.residual THEN 1 ELSE 0 END))::numeric) / 100, 2),
      jsonb_build_object(
        'input_id', r.input_id::text,
        'input_last_event_id', r.input_last_event_id::text,
        'tax_year', v_tax_year::text,
        'tax_month', v_tax_month::text,
        'group_target_net', (v_group_target_net_cents::numeric / 100)::text,
        'group_solved_gross', (v_solved_gross_cents::numeric / 100)::text,
        'group_delta_iit', (v_group_delta_iit_cents::numeric / 100)::text,
        'base_income', (v_base_income_cents::numeric / 100)::text,
        'base_iit_withhold', (v_base_iit_withhold_cents::numeric / 100)::text,
        'iterations', v_iters::text
      ),
      p_run_event_db_id,
      'net_guaranteed_iit',
      'employer',
      round((r.target_net_cents::numeric) / 100, 2),
      round(((r.q + CASE WHEN r.rn <= r.residual THEN 1 ELSE 0 END)::numeric) / 100, 2)
    FROM ranked r;

    UPDATE staffing.payslips p
    SET
      gross_pay = p.gross_pay + round(v_solved_gross_cents::numeric / 100, 2),
      net_pay = p.net_pay + round(v_solved_gross_cents::numeric / 100, 2),
      last_run_event_id = p_run_event_db_id,
      updated_at = p_now
    WHERE p.tenant_id = p_tenant_id AND p.id = v_net_guaranteed_payslip.payslip_id;
  END LOOP;

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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose StatementBegin
DROP FUNCTION IF EXISTS staffing.iit_withhold_this_month_cents(
  bigint,
  bigint,
  bigint,
  bigint,
  bigint,
  bigint
);
-- +goose StatementEnd

