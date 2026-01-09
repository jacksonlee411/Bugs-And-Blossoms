-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.submit_payslip_item_input_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_run_id uuid,
  p_person_uuid uuid,
  p_assignment_id uuid,
  p_event_type text,
  p_item_code text,
  p_item_kind text,
  p_currency char(3),
  p_calc_mode text,
  p_tax_bearer text,
  p_amount numeric(15,2),
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing_event staffing.payslip_item_input_events%ROWTYPE;
  v_run staffing.payroll_runs%ROWTYPE;
  v_existing_input staffing.payslip_item_inputs%ROWTYPE;

  v_now timestamptz;

  v_item_code text;
  v_item_kind text;
  v_currency char(3);
  v_calc_mode text;
  v_tax_bearer text;
  v_amount numeric(15,2);

  v_currency_trim text;
BEGIN
  PERFORM staffing.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_run_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'run_id is required';
  END IF;
  IF p_person_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'person_uuid is required';
  END IF;
  IF p_assignment_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'assignment_id is required';
  END IF;
  IF p_event_type IS NULL OR p_event_type NOT IN ('UPSERT','DELETE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_item_code := COALESCE(p_item_code, '');
  IF btrim(v_item_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code is required';
  END IF;
  IF v_item_code <> btrim(v_item_code) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code must be trimmed';
  END IF;
  IF v_item_code <> upper(v_item_code) THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code must be upper';
  END IF;
  IF v_item_code !~ '^[A-Z0-9_]+$' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'item_code invalid';
  END IF;

  IF btrim(COALESCE(p_request_id, '')) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  v_lock_key := format('staffing:payroll-run:%s:%s', p_tenant_id, p_run_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_run
  FROM staffing.payroll_runs
  WHERE tenant_id = p_tenant_id AND id = p_run_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_NOT_FOUND',
      DETAIL = format('run_id=%s', p_run_id);
  END IF;

  IF v_run.run_state = 'finalized' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_RUN_FINALIZED_READONLY',
      DETAIL = format('run_id=%s', p_run_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM staffing.payslips p
    WHERE p.tenant_id = p_tenant_id
      AND p.run_id = p_run_id
      AND p.person_uuid = p_person_uuid
      AND p.assignment_id = p_assignment_id
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
      DETAIL = format('payslip not found: run_id=%s person_uuid=%s assignment_id=%s', p_run_id, p_person_uuid, p_assignment_id);
  END IF;

  v_now := now();

  IF p_event_type = 'DELETE' THEN
    SELECT * INTO v_existing_input
    FROM staffing.payslip_item_inputs i
    WHERE i.tenant_id = p_tenant_id
      AND i.run_id = p_run_id
      AND i.person_uuid = p_person_uuid
      AND i.assignment_id = p_assignment_id
      AND i.item_code = v_item_code;

    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('input not found: run_id=%s person_uuid=%s assignment_id=%s item_code=%s', p_run_id, p_person_uuid, p_assignment_id, v_item_code);
    END IF;

    v_item_kind := v_existing_input.item_kind;
    v_currency := v_existing_input.currency;
    v_calc_mode := v_existing_input.calc_mode;
    v_tax_bearer := v_existing_input.tax_bearer;
    v_amount := v_existing_input.amount;
  ELSE
    IF p_item_kind IS NULL OR p_item_kind NOT IN ('earning','deduction','employer_cost') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('unsupported item_kind: %s', p_item_kind);
    END IF;

    IF p_calc_mode IS NULL OR p_calc_mode NOT IN ('amount','net_guaranteed_iit') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('unsupported calc_mode: %s', p_calc_mode);
    END IF;

    IF p_tax_bearer IS NULL OR p_tax_bearer NOT IN ('employee','employer') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = format('unsupported tax_bearer: %s', p_tax_bearer);
    END IF;

    IF p_currency IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
        DETAIL = 'currency is required';
    END IF;

    v_currency_trim := btrim(p_currency::text);
    IF v_currency_trim = '' THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'currency is required';
    END IF;
    IF v_currency_trim <> upper(v_currency_trim) THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'currency must be upper';
    END IF;
    IF length(v_currency_trim) <> 3 THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'currency must be 3 letters';
    END IF;

    IF p_amount IS NULL OR p_amount <= 0 THEN
      RAISE EXCEPTION USING MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT', DETAIL = 'amount must be > 0';
    END IF;

    IF p_calc_mode = 'net_guaranteed_iit' THEN
      IF v_currency_trim <> 'CNY' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_CURRENCY_MISMATCH',
          DETAIL = format('currency=%s', v_currency_trim);
      END IF;
      IF p_tax_bearer <> 'employer' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
          DETAIL = format('tax_bearer must be employer, got=%s', p_tax_bearer);
      END IF;
      IF p_item_kind <> 'earning' THEN
        RAISE EXCEPTION USING
          MESSAGE = 'STAFFING_PAYROLL_NET_GUARANTEED_IIT_INVALID_ARGUMENT',
          DETAIL = format('item_kind must be earning, got=%s', p_item_kind);
      END IF;
    END IF;

    v_item_kind := p_item_kind;
    v_currency := p_currency;
    v_calc_mode := p_calc_mode;
    v_tax_bearer := p_tax_bearer;
    v_amount := p_amount;
  END IF;

  INSERT INTO staffing.payslip_item_input_events (
    event_id,
    tenant_id,
    run_id,
    person_uuid,
    assignment_id,
    event_type,
    item_code,
    item_kind,
    currency,
    calc_mode,
    tax_bearer,
    amount,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_run_id,
    p_person_uuid,
    p_assignment_id,
    p_event_type,
    v_item_code,
    v_item_kind,
    v_currency,
    v_calc_mode,
    v_tax_bearer,
    v_amount,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing_event
    FROM staffing.payslip_item_input_events
    WHERE event_id = p_event_id;

    IF v_existing_event.tenant_id <> p_tenant_id
      OR v_existing_event.run_id <> p_run_id
      OR v_existing_event.person_uuid <> p_person_uuid
      OR v_existing_event.assignment_id <> p_assignment_id
      OR v_existing_event.event_type <> p_event_type
      OR v_existing_event.item_code <> v_item_code
      OR v_existing_event.item_kind <> v_item_kind
      OR v_existing_event.currency <> v_currency
      OR v_existing_event.calc_mode <> v_calc_mode
      OR v_existing_event.tax_bearer <> v_tax_bearer
      OR v_existing_event.amount <> v_amount
      OR v_existing_event.request_id <> p_request_id
      OR v_existing_event.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'STAFFING_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing_event.id);
    END IF;

    RETURN v_existing_event.id;
  END IF;

  IF p_event_type = 'UPSERT' THEN
    INSERT INTO staffing.payslip_item_inputs (
      tenant_id,
      run_id,
      person_uuid,
      assignment_id,
      item_code,
      item_kind,
      currency,
      calc_mode,
      tax_bearer,
      amount,
      last_event_id,
      created_at,
      updated_at
    )
    VALUES (
      p_tenant_id,
      p_run_id,
      p_person_uuid,
      p_assignment_id,
      v_item_code,
      v_item_kind,
      v_currency,
      v_calc_mode,
      v_tax_bearer,
      v_amount,
      v_event_db_id,
      v_now,
      v_now
    )
    ON CONFLICT ON CONSTRAINT payslip_item_inputs_natural_unique
    DO UPDATE SET
      item_kind = EXCLUDED.item_kind,
      currency = EXCLUDED.currency,
      calc_mode = EXCLUDED.calc_mode,
      tax_bearer = EXCLUDED.tax_bearer,
      amount = EXCLUDED.amount,
      last_event_id = EXCLUDED.last_event_id,
      updated_at = EXCLUDED.updated_at;
  ELSE
    DELETE FROM staffing.payslip_item_inputs
    WHERE tenant_id = p_tenant_id
      AND run_id = p_run_id
      AND person_uuid = p_person_uuid
      AND assignment_id = p_assignment_id
      AND item_code = v_item_code;
  END IF;

  IF v_run.run_state = 'calculated' THEN
    UPDATE staffing.payroll_runs
    SET
      needs_recalc = true,
      updated_at = v_now
    WHERE tenant_id = p_tenant_id AND id = p_run_id;
  END IF;

  RETURN v_event_db_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS staffing.submit_payslip_item_input_event(
  uuid,
  uuid,
  uuid,
  uuid,
  uuid,
  text,
  text,
  text,
  char(3),
  text,
  text,
  numeric(15,2),
  text,
  uuid
);
-- +goose StatementEnd
