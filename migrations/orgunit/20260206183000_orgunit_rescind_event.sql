-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE VIEW orgunit.org_events_effective AS
SELECT
  e.id,
  e.event_uuid,
  e.tenant_uuid,
  e.org_id,
  e.event_type,
  COALESCE(c.corrected_effective_date, e.effective_date) AS effective_date,
  COALESCE(c.replacement_payload, e.payload) AS payload,
  e.request_code,
  e.initiator_uuid,
  e.transaction_time,
  e.created_at
FROM orgunit.org_events e
LEFT JOIN orgunit.org_event_corrections_current c
  ON c.event_uuid = e.event_uuid
 AND c.tenant_uuid = e.tenant_uuid
 AND c.org_id = e.org_id
WHERE COALESCE(c.replacement_payload->>'op', '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG');

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_rescind(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_reason text,
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
  v_existing_current orgunit.org_event_corrections_current%ROWTYPE;
  v_existing_correction_uuid uuid;
  v_request_hash text;
  v_reason text;
  v_target_effective date;
  v_effective_payload jsonb;
  v_correction_uuid uuid;
  v_payload jsonb;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  v_reason := btrim(COALESCE(p_reason, ''));
  IF v_reason = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'reason is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_request_hash := encode(
    digest(format('%s|%s|%s', p_org_id, p_target_effective_date, v_reason), 'sha256'),
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
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
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

  SELECT * INTO v_existing_current
  FROM orgunit.org_event_corrections_current c
  WHERE c.tenant_uuid = p_tenant_uuid
    AND c.org_id = p_org_id
    AND c.event_uuid = v_target.event_uuid
  LIMIT 1;

  IF FOUND AND COALESCE(v_existing_current.replacement_payload->>'op', '') IN ('RESCIND_EVENT', 'RESCIND_ORG') THEN
    SELECT h.correction_uuid INTO v_existing_correction_uuid
    FROM orgunit.org_event_corrections_history h
    WHERE h.tenant_uuid = p_tenant_uuid
      AND h.request_id = v_existing_current.request_id
    LIMIT 1;

    IF v_existing_correction_uuid IS NOT NULL THEN
      RETURN v_existing_correction_uuid;
    END IF;
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

  v_payload := jsonb_build_object(
    'op', 'RESCIND_EVENT',
    'reason', v_reason,
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  );

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
    v_target_effective,
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
    v_target_effective,
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

  BEGIN
    PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);
  EXCEPTION
    WHEN OTHERS THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REPLAY_FAILED',
        DETAIL = format('tenant_uuid=%s org_id=%s cause=%s', p_tenant_uuid, p_org_id, SQLERRM);
  END;

  RETURN v_correction_uuid;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.guard_event_rescind_priority()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF COALESCE(OLD.replacement_payload->>'op', '') IN ('RESCIND_EVENT', 'RESCIND_ORG')
    AND COALESCE(NEW.replacement_payload->>'op', '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG')
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', OLD.event_uuid);
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS guard_event_rescind_priority ON orgunit.org_event_corrections_current;
CREATE TRIGGER guard_event_rescind_priority
BEFORE UPDATE ON orgunit.org_event_corrections_current
FOR EACH ROW EXECUTE FUNCTION orgunit.guard_event_rescind_priority();

ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.guard_event_rescind_priority()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.guard_event_rescind_priority()
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE VIEW orgunit.org_events_effective AS
SELECT
  e.id,
  e.event_uuid,
  e.tenant_uuid,
  e.org_id,
  e.event_type,
  COALESCE(c.corrected_effective_date, e.effective_date) AS effective_date,
  COALESCE(c.replacement_payload, e.payload) AS payload,
  e.request_code,
  e.initiator_uuid,
  e.transaction_time,
  e.created_at
FROM orgunit.org_events e
LEFT JOIN orgunit.org_event_corrections_current c
  ON c.event_uuid = e.event_uuid
 AND c.tenant_uuid = e.tenant_uuid
 AND c.org_id = e.org_id;


DROP TRIGGER IF EXISTS guard_event_rescind_priority ON orgunit.org_event_corrections_current;
DROP FUNCTION IF EXISTS orgunit.guard_event_rescind_priority();
DROP FUNCTION IF EXISTS orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid);
-- +goose StatementEnd
