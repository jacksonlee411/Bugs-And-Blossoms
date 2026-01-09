-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd

