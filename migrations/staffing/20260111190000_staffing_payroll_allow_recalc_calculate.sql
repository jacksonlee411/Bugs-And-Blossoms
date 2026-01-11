-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  v_def text;
BEGIN
  SELECT pg_get_functiondef(
    'staffing.submit_payroll_run_event(uuid, uuid, uuid, uuid, text, jsonb, text, uuid)'::regprocedure
  ) INTO v_def;

  IF v_def IS NULL OR v_def = '' THEN
    RAISE EXCEPTION 'missing function: staffing.submit_payroll_run_event';
  END IF;

  v_def := replace(
    v_def,
    'IF v_existing_run.run_state NOT IN (''draft'',''failed'') THEN',
    'IF v_existing_run.run_state NOT IN (''draft'',''failed'') AND NOT (v_existing_run.run_state = ''calculated'' AND v_existing_run.needs_recalc = true) THEN'
  );

  v_def := replace(
    v_def,
    'calc_finished_at = v_now,',
    'calc_finished_at = v_now,' || E'\n      needs_recalc = false,'
  );

  EXECUTE v_def;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
  v_def text;
BEGIN
  SELECT pg_get_functiondef(
    'staffing.submit_payroll_run_event(uuid, uuid, uuid, uuid, text, jsonb, text, uuid)'::regprocedure
  ) INTO v_def;

  IF v_def IS NULL OR v_def = '' THEN
    RAISE EXCEPTION 'missing function: staffing.submit_payroll_run_event';
  END IF;

  v_def := replace(
    v_def,
    'IF v_existing_run.run_state NOT IN (''draft'',''failed'') AND NOT (v_existing_run.run_state = ''calculated'' AND v_existing_run.needs_recalc = true) THEN',
    'IF v_existing_run.run_state NOT IN (''draft'',''failed'') THEN'
  );

  v_def := replace(
    v_def,
    'calc_finished_at = v_now,' || E'\n      needs_recalc = false,',
    'calc_finished_at = v_now,'
  );

  EXECUTE v_def;
END
$$;
-- +goose StatementEnd

