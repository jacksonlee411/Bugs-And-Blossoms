-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.submit_org_event_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_patch jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing orgunit.org_event_corrections_history%ROWTYPE;
  v_request_hash text;
  v_payload jsonb;
  v_effective_payload jsonb;
  v_target_effective date;
  v_prev_effective date;
  v_new_effective date;
  v_next_effective date;
  v_parent_id int;
  v_correction_uuid uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_patch IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_payload := p_patch;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch must be an object';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_request_hash := encode(
    digest(format('%s|%s|%s', p_org_id, p_target_effective_date, v_payload::text), 'sha256'),
    'hex'
  );

  SELECT * INTO v_existing
  FROM orgunit.org_event_corrections_history
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.request_hash <> v_request_hash THEN
      RAISE EXCEPTION USING
        MESSAGE = 'REQUEST_DUPLICATE',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing.correction_uuid;
  END IF;

  SELECT * INTO v_target
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    SELECT e.* INTO v_target
    FROM orgunit.org_events e
    JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
      AND ee.effective_date = p_target_effective_date
    LIMIT 1;
  END IF;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  SELECT ee.effective_date, ee.payload
  INTO v_target_effective, v_effective_payload
  FROM orgunit.org_events_effective ee
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.event_uuid = v_target.event_uuid
  LIMIT 1;

  IF v_target_effective IS NULL THEN
    v_target_effective := v_target.effective_date;
    v_effective_payload := v_target.payload;
  END IF;

  v_payload := COALESCE(v_effective_payload, '{}'::jsonb) || v_payload;
  v_new_effective := v_target_effective;

  IF v_payload ? 'effective_date' THEN
    BEGIN
      v_new_effective := NULLIF(btrim(v_payload->>'effective_date'), '')::date;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'EFFECTIVE_DATE_INVALID',
          DETAIL = format('effective_date=%s', v_payload->>'effective_date');
    END;
    IF v_new_effective IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'EFFECTIVE_DATE_INVALID',
        DETAIL = 'effective_date is required';
    END IF;
    v_payload := v_payload - 'effective_date';
  END IF;

  SELECT MAX(e.effective_date) INTO v_prev_effective
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date < v_target_effective;

  SELECT MIN(e.effective_date) INTO v_next_effective
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date > v_target_effective;

  IF v_prev_effective IS NOT NULL AND v_new_effective <= v_prev_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('prev=%s new=%s', v_prev_effective, v_new_effective);
  END IF;
  IF v_next_effective IS NOT NULL AND v_new_effective >= v_next_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('next=%s new=%s', v_next_effective, v_new_effective);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
      AND e.effective_date = v_new_effective
      AND e.event_uuid <> v_target.event_uuid
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EVENT_DATE_CONFLICT',
      DETAIL = format('org_id=%s effective_date=%s', p_org_id, v_new_effective);
  END IF;

  IF v_target.event_type = 'CREATE' THEN
    v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
  ELSIF v_target.event_type = 'MOVE' THEN
    v_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
  ELSE
    SELECT v.parent_id INTO v_parent_id
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
      AND v.validity @> v_new_effective
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
  END IF;

  IF v_parent_id IS NOT NULL THEN
    PERFORM 1
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_parent_id
      AND v.status = 'active'
      AND v.validity @> v_new_effective
    LIMIT 1;

    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_PARENT_NOT_FOUND_AS_OF',
        DETAIL = format('parent_id=%s as_of=%s', v_parent_id, v_new_effective);
    END IF;
  END IF;

  v_correction_uuid := gen_random_uuid();

  INSERT INTO orgunit.org_event_corrections_history (
    correction_uuid,
    event_uuid,
    tenant_uuid,
    org_id,
    target_effective_date,
    corrected_effective_date,
    original_event,
    replacement_payload,
    initiator_uuid,
    request_id,
    request_hash
  )
  VALUES (
    v_correction_uuid,
    v_target.event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_target_effective_date,
    v_new_effective,
    to_jsonb(v_target),
    v_payload,
    p_initiator_uuid,
    p_request_id,
    v_request_hash
  );

  INSERT INTO orgunit.org_event_corrections_current (
    event_uuid,
    tenant_uuid,
    org_id,
    target_effective_date,
    corrected_effective_date,
    original_event,
    replacement_payload,
    initiator_uuid,
    request_id,
    request_hash
  )
  VALUES (
    v_target.event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_target_effective_date,
    v_new_effective,
    to_jsonb(v_target),
    v_payload,
    p_initiator_uuid,
    p_request_id,
    v_request_hash
  )
  ON CONFLICT (event_uuid) DO UPDATE SET
    tenant_uuid = EXCLUDED.tenant_uuid,
    org_id = EXCLUDED.org_id,
    target_effective_date = EXCLUDED.target_effective_date,
    corrected_effective_date = EXCLUDED.corrected_effective_date,
    original_event = EXCLUDED.original_event,
    replacement_payload = EXCLUDED.replacement_payload,
    initiator_uuid = EXCLUDED.initiator_uuid,
    request_id = EXCLUDED.request_id,
    request_hash = EXCLUDED.request_hash,
    corrected_at = EXCLUDED.corrected_at;

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_correction_uuid;
END;
$$;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.submit_org_event_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_patch jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing orgunit.org_event_corrections_history%ROWTYPE;
  v_request_hash text;
  v_payload jsonb;
  v_new_effective date;
  v_next_effective date;
  v_correction_uuid uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_patch IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_payload := p_patch;
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch must be an object';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_request_hash := encode(
    digest(format('%s|%s|%s', p_org_id, p_target_effective_date, v_payload::text), 'sha256'),
    'hex'
  );

  SELECT * INTO v_existing
  FROM orgunit.org_event_corrections_history
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.request_hash <> v_request_hash THEN
      RAISE EXCEPTION USING
        MESSAGE = 'REQUEST_DUPLICATE',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing.correction_uuid;
  END IF;

  SELECT * INTO v_target
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  v_payload := COALESCE(v_target.payload, '{}'::jsonb) || v_payload;
  v_new_effective := v_target.effective_date;

  IF v_payload ? 'effective_date' THEN
    BEGIN
      v_new_effective := NULLIF(btrim(v_payload->>'effective_date'), '')::date;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'EFFECTIVE_DATE_INVALID',
          DETAIL = format('effective_date=%s', v_payload->>'effective_date');
    END;
    IF v_new_effective IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'EFFECTIVE_DATE_INVALID',
        DETAIL = 'effective_date is required';
    END IF;
    v_payload := v_payload - 'effective_date';
  END IF;

  SELECT MIN(e.effective_date) INTO v_next_effective
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date > v_target.effective_date;

  IF v_new_effective < v_target.effective_date THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('target=%s new=%s', v_target.effective_date, v_new_effective);
  END IF;
  IF v_next_effective IS NOT NULL AND v_new_effective >= v_next_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('next=%s new=%s', v_next_effective, v_new_effective);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
      AND e.effective_date = v_new_effective
      AND e.event_uuid <> v_target.event_uuid
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EVENT_DATE_CONFLICT',
      DETAIL = format('org_id=%s effective_date=%s', p_org_id, v_new_effective);
  END IF;

  v_correction_uuid := gen_random_uuid();

  INSERT INTO orgunit.org_event_corrections_history (
    correction_uuid,
    event_uuid,
    tenant_uuid,
    org_id,
    target_effective_date,
    corrected_effective_date,
    original_event,
    replacement_payload,
    initiator_uuid,
    request_id,
    request_hash
  )
  VALUES (
    v_correction_uuid,
    v_target.event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_target_effective_date,
    v_new_effective,
    to_jsonb(v_target),
    v_payload,
    p_initiator_uuid,
    p_request_id,
    v_request_hash
  );

  INSERT INTO orgunit.org_event_corrections_current (
    event_uuid,
    tenant_uuid,
    org_id,
    target_effective_date,
    corrected_effective_date,
    original_event,
    replacement_payload,
    initiator_uuid,
    request_id,
    request_hash
  )
  VALUES (
    v_target.event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_target_effective_date,
    v_new_effective,
    to_jsonb(v_target),
    v_payload,
    p_initiator_uuid,
    p_request_id,
    v_request_hash
  )
  ON CONFLICT (event_uuid) DO UPDATE SET
    tenant_uuid = EXCLUDED.tenant_uuid,
    org_id = EXCLUDED.org_id,
    target_effective_date = EXCLUDED.target_effective_date,
    corrected_effective_date = EXCLUDED.corrected_effective_date,
    original_event = EXCLUDED.original_event,
    replacement_payload = EXCLUDED.replacement_payload,
    initiator_uuid = EXCLUDED.initiator_uuid,
    request_id = EXCLUDED.request_id,
    request_hash = EXCLUDED.request_hash,
    corrected_at = EXCLUDED.corrected_at;

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_correction_uuid;
END;
$$;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd
