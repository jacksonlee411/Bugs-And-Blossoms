-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.submit_org_rescind(
  p_tenant_uuid uuid,
  p_org_id int,
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_reason text;
  v_root_org_id int;
  v_node_path ltree;
  v_event_count int;
  v_existing_batch_count int;
  v_need_apply boolean;
  v_request_id_seq text;
  v_request_hash text;
  v_payload jsonb;
  v_existing orgunit.org_event_corrections_history%ROWTYPE;
  rec record;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
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

  SELECT * INTO v_existing
  FROM orgunit.org_event_corrections_history
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
      DETAIL = format('request_id=%s', p_request_id);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid
  LIMIT 1;

  IF v_root_org_id = p_org_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_DELETE_FORBIDDEN',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT v.node_path INTO v_node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_node_path IS NOT NULL AND EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions c
    WHERE c.tenant_uuid = p_tenant_uuid
      AND c.node_path <@ v_node_path
      AND c.org_id <> p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_HAS_CHILDREN_CANNOT_DELETE',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_binding_versions b
    WHERE b.tenant_uuid = p_tenant_uuid
      AND b.org_id = p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_HAS_DEPENDENCIES_CANNOT_DELETE',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT COUNT(*) INTO v_event_count
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id;

  IF v_event_count = 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT COUNT(*) INTO v_existing_batch_count
  FROM orgunit.org_event_corrections_history h
  WHERE h.tenant_uuid = p_tenant_uuid
    AND h.request_id LIKE p_request_id || '#%';

  IF v_existing_batch_count > 0 AND v_existing_batch_count <> v_event_count THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
      DETAIL = format('request_id=%s', p_request_id);
  END IF;

  v_need_apply := false;

  FOR rec IN
    SELECT
      row_number() OVER (ORDER BY e.effective_date, e.id) AS seq,
      e.event_uuid,
      COALESCE(ee.effective_date, e.effective_date) AS target_effective_date,
      to_jsonb(e) AS original_event
    FROM orgunit.org_events e
    LEFT JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
    ORDER BY e.effective_date, e.id
  LOOP
    v_request_id_seq := format('%s#%s', p_request_id, lpad(rec.seq::text, 4, '0'));
    v_request_hash := encode(
      digest(format('%s|%s|%s|%s', p_org_id, rec.event_uuid, rec.target_effective_date, v_reason), 'sha256'),
      'hex'
    );

    SELECT * INTO v_existing
    FROM orgunit.org_event_corrections_history h
    WHERE h.tenant_uuid = p_tenant_uuid
      AND h.request_id = v_request_id_seq
    LIMIT 1;

    IF FOUND THEN
      IF v_existing.request_hash <> v_request_hash THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
          DETAIL = format('request_id=%s', p_request_id);
      END IF;
      CONTINUE;
    END IF;

    v_need_apply := true;
  END LOOP;

  IF NOT v_need_apply THEN
    RETURN v_event_count;
  END IF;

  FOR rec IN
    SELECT
      row_number() OVER (ORDER BY e.effective_date, e.id) AS seq,
      e.event_uuid,
      COALESCE(ee.effective_date, e.effective_date) AS target_effective_date,
      to_jsonb(e) AS original_event
    FROM orgunit.org_events e
    LEFT JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
    ORDER BY e.effective_date, e.id
  LOOP
    v_request_id_seq := format('%s#%s', p_request_id, lpad(rec.seq::text, 4, '0'));
    v_request_hash := encode(
      digest(format('%s|%s|%s|%s', p_org_id, rec.event_uuid, rec.target_effective_date, v_reason), 'sha256'),
      'hex'
    );

    SELECT * INTO v_existing
    FROM orgunit.org_event_corrections_history h
    WHERE h.tenant_uuid = p_tenant_uuid
      AND h.request_id = v_request_id_seq
    LIMIT 1;

    IF FOUND THEN
      CONTINUE;
    END IF;

    v_payload := jsonb_build_object(
      'op', 'RESCIND_ORG',
      'reason', v_reason,
      'batch_request_id', p_request_id,
      'target_event_uuid', rec.event_uuid,
      'target_effective_date', rec.target_effective_date
    );

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
      gen_random_uuid(),
      rec.event_uuid,
      p_tenant_uuid,
      p_org_id,
      rec.target_effective_date,
      rec.target_effective_date,
      rec.original_event,
      v_payload,
      p_initiator_uuid,
      v_request_id_seq,
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
      rec.event_uuid,
      p_tenant_uuid,
      p_org_id,
      rec.target_effective_date,
      rec.target_effective_date,
      rec.original_event,
      v_payload,
      p_initiator_uuid,
      v_request_id_seq,
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
  END LOOP;

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_event_count;
END;
$$;

ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.submit_org_rescind(uuid, int, text, text, uuid);
-- +goose StatementEnd
