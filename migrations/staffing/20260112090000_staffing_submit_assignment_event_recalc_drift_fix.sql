-- +goose Up
-- Fix dev DB drift: ensure submit_assignment_event triggers payroll recalc requests after assignment changes (P0-5).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION staffing.submit_assignment_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_assignment_id uuid,
  p_person_uuid uuid,
  p_assignment_type text,
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
  v_existing staffing.assignment_events%ROWTYPE;
  v_payload jsonb;
  v_existing_assignment_id uuid;
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
  IF p_assignment_type IS NULL OR btrim(p_assignment_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = 'assignment_type is required';
  END IF;
  IF p_assignment_type <> 'primary' THEN
    RAISE EXCEPTION USING MESSAGE = 'STAFFING_INVALID_ARGUMENT', DETAIL = format('unsupported assignment_type: %s', p_assignment_type);
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
  IF p_event_type NOT IN ('CREATE','UPDATE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('staffing:assignment:%s:%s', p_tenant_id, p_assignment_id);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  INSERT INTO staffing.assignments (tenant_id, id, person_uuid, assignment_type)
  VALUES (p_tenant_id, p_assignment_id, p_person_uuid, p_assignment_type)
  ON CONFLICT (tenant_id, person_uuid, assignment_type) DO NOTHING;

  SELECT id INTO v_existing_assignment_id
  FROM staffing.assignments
  WHERE tenant_id = p_tenant_id AND person_uuid = p_person_uuid AND assignment_type = p_assignment_type;

  IF v_existing_assignment_id IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_INVALID_ARGUMENT',
      DETAIL = 'assignment identity missing';
  END IF;
  IF v_existing_assignment_id <> p_assignment_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'STAFFING_ASSIGNMENT_ID_MISMATCH',
      DETAIL = format('assignment_id=%s existing_id=%s', p_assignment_id, v_existing_assignment_id);
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  INSERT INTO staffing.assignment_events (
    event_id,
    tenant_id,
    assignment_id,
    person_uuid,
    assignment_type,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_assignment_id,
    p_person_uuid,
    p_assignment_type,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM staffing.assignment_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.assignment_id <> p_assignment_id
      OR v_existing.person_uuid <> p_person_uuid
      OR v_existing.assignment_type <> p_assignment_type
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

  PERFORM staffing.replay_assignment_versions(p_tenant_id, p_assignment_id);

  -- NOTE: use dynamic SQL to avoid schema file ordering issues (P0-5 adds staffing.maybe_create_payroll_recalc_request_from_assignment_event later).
  EXECUTE 'SELECT staffing.maybe_create_payroll_recalc_request_from_assignment_event($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::date,$7::jsonb,$8::text,$9::uuid);'
  USING p_event_id, p_tenant_id, p_assignment_id, p_person_uuid, p_event_type, p_effective_date, v_payload, p_request_id, p_initiator_id;

  RETURN v_event_db_id;
END;
$$;
-- +goose StatementEnd
