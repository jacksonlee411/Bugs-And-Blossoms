-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org(
  p_tenant_uuid uuid,
  p_org_id int
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_event orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
  v_root_org_id int;
  v_has_create boolean;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  SELECT EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
      AND e.event_type = 'CREATE'
  ) INTO v_has_create;

  IF v_has_create AND v_root_org_id = p_org_id THEN
    DELETE FROM orgunit.org_trees
    WHERE tenant_uuid = p_tenant_uuid;
  ELSIF v_root_org_id = p_org_id AND NOT v_has_create THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('root org missing create event: org_id=%s', p_org_id);
  END IF;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id;

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events_effective
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
    ORDER BY effective_date, id
  LOOP
    v_payload := COALESCE(v_event.payload, '{}'::jsonb);

    IF v_event.event_type = 'CREATE' THEN
      v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
      v_name := NULLIF(btrim(v_payload->>'name'), '');
      v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
      v_org_code := NULLIF(v_payload->>'org_code', '');
      v_is_business_unit := NULL;
      IF v_payload ? 'is_business_unit' THEN
        BEGIN
          v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_INVALID_ARGUMENT',
              DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
        END;
      END IF;
      PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_event.org_id, v_org_code, v_parent_id, v_event.effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'ENABLE' THEN
      PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'SET_BUSINESS_UNIT' THEN
      IF NOT (v_payload ? 'is_business_unit') THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = 'is_business_unit is required';
      END IF;
      BEGIN
        v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_INVALID_ARGUMENT',
            DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
      END;
      PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_is_business_unit, v_event.id);
    ELSE
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_event.event_type);
    END IF;
  END LOOP;

  SELECT v.node_path INTO v_root_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_root_path IS NULL THEN
    RETURN;
  END IF;

  SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.node_path <@ v_root_path;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
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

  PERFORM orgunit.rebuild_org_unit_versions_for_org(p_tenant_uuid, p_org_id);

  RETURN v_correction_uuid;
END;
$$;
-- +goose StatementEnd

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

  PERFORM orgunit.rebuild_org_unit_versions_for_org(p_tenant_uuid, p_org_id);

  RETURN v_event_count;
END;
$$;
-- +goose StatementEnd

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
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  v_new_effective := p_target_effective_date;

  IF v_payload ? 'effective_date' THEN
    BEGIN
      v_new_effective := (v_payload->>'effective_date')::date;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = format('effective_date=%s', v_payload->>'effective_date');
    END;
    IF v_new_effective < p_target_effective_date THEN
      SELECT MIN(e.effective_date) INTO v_next_effective
      FROM orgunit.org_events e
      WHERE e.tenant_uuid = p_tenant_uuid
        AND e.org_id = p_org_id
        AND e.effective_date > v_new_effective;

      IF v_next_effective IS NOT NULL AND v_next_effective <= p_target_effective_date THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_EVENT_DATE_CONFLICT',
          DETAIL = format('target=%s next=%s', v_new_effective, v_next_effective);
      END IF;
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

  PERFORM orgunit.rebuild_org_unit_versions_for_org(p_tenant_uuid, p_org_id);

  RETURN v_correction_uuid;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.submit_org_status_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_target_status text,
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
  v_request_hash text;
  v_target_status text;
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
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_target_status := lower(btrim(COALESCE(p_target_status, '')));
  IF v_target_status IN ('enabled', '有效') THEN
    v_target_status := 'active';
  ELSIF v_target_status IN ('inactive', '无效') THEN
    v_target_status := 'disabled';
  END IF;
  IF v_target_status NOT IN ('active', 'disabled') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_status invalid';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_request_hash := encode(
    digest(format('%s|%s|%s|CORRECT_STATUS', p_org_id, p_target_effective_date, v_target_status), 'sha256'),
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

  IF v_target.event_type NOT IN ('ENABLE', 'DISABLE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET',
      DETAIL = format('event_type=%s', v_target.event_type);
  END IF;

  SELECT * INTO v_existing_current
  FROM orgunit.org_event_corrections_current c
  WHERE c.tenant_uuid = p_tenant_uuid
    AND c.org_id = p_org_id
    AND c.event_uuid = v_target.event_uuid
  LIMIT 1;

  IF FOUND AND COALESCE(v_existing_current.replacement_payload->>'op', '') IN ('RESCIND_EVENT', 'RESCIND_ORG') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', v_target.event_uuid);
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
    'op', 'CORRECT_STATUS',
    'target_status', v_target_status,
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

  PERFORM orgunit.rebuild_org_unit_versions_for_org(p_tenant_uuid, p_org_id);

  RETURN v_correction_uuid;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.replay_org_unit_versions(uuid);
-- +goose StatementEnd

-- +goose StatementBegin
REVOKE EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) FROM PUBLIC;
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) FROM app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) FROM app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) FROM superadmin_runtime';
  END IF;
END $$;
GRANT EXECUTE ON FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) TO orgunit_kernel;
ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org(uuid, int) SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.replay_org_unit_versions(
  p_tenant_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_trees
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events_effective
    WHERE tenant_uuid = p_tenant_uuid
    ORDER BY effective_date, id
  LOOP
    v_payload := COALESCE(v_event.payload, '{}'::jsonb);

    IF v_event.event_type = 'CREATE' THEN
      v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
      v_name := NULLIF(btrim(v_payload->>'name'), '');
      v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
      v_org_code := NULLIF(v_payload->>'org_code', '');
      v_is_business_unit := NULL;
      IF v_payload ? 'is_business_unit' THEN
        BEGIN
          v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_INVALID_ARGUMENT',
              DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
        END;
      END IF;
      PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_event.org_id, v_org_code, v_parent_id, v_event.effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event.id);
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'ENABLE' THEN
      PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'SET_BUSINESS_UNIT' THEN
      IF NOT (v_payload ? 'is_business_unit') THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = 'is_business_unit is required';
      END IF;
      BEGIN
        v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_INVALID_ARGUMENT',
            DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
      END;
      PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_is_business_unit, v_event.id);
    ELSE
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_event.event_type);
    END IF;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        org_id,
        validity,
        lag(validity) OVER (PARTITION BY org_id ORDER BY lower(validity)) AS prev_validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_GAP',
      DETAIL = 'org_unit_versions must be gapless';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT DISTINCT ON (org_id) org_id, validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
      ORDER BY org_id, lower(validity) DESC
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_NOT_INFINITE',
      DETAIL = 'last version validity must be unbounded (infinity)';
  END IF;

  UPDATE orgunit.org_unit_versions v
  SET full_name_path = (
    SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
    FROM unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
    JOIN orgunit.org_unit_versions a
      ON a.tenant_uuid = v.tenant_uuid
     AND a.org_id = t.uid
     AND a.validity @> lower(v.validity)
  )
  WHERE v.tenant_uuid = p_tenant_uuid
;
END;
$$;

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

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_correction_uuid;
END;
$$;

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
  v_target_path ltree;
  v_descendant_min_create date;
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

  -- Guard high-risk create reordering that would force replay to fail after full-table churn.
  IF v_target.event_type = 'CREATE' AND v_new_effective > v_target_effective THEN
    SELECT v.node_path INTO v_target_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
      AND v.validity @> v_target_effective
    ORDER BY lower(v.validity) DESC
    LIMIT 1;

    IF v_target_path IS NOT NULL THEN
      SELECT MIN(e.effective_date) INTO v_descendant_min_create
      FROM orgunit.org_events_effective e
      WHERE e.tenant_uuid = p_tenant_uuid
        AND e.event_type = 'CREATE'
        AND e.org_id <> p_org_id
        AND EXISTS (
          SELECT 1
          FROM orgunit.org_unit_versions dv
          WHERE dv.tenant_uuid = p_tenant_uuid
            AND dv.org_id = e.org_id
            AND dv.node_path <@ v_target_path
          LIMIT 1
        );

      IF v_descendant_min_create IS NOT NULL AND v_descendant_min_create < v_new_effective THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_HIGH_RISK_REORDER_FORBIDDEN',
          DETAIL = format(
            'org_id=%s target_effective=%s new_effective=%s descendant_create=%s',
            p_org_id,
            v_target_effective,
            v_new_effective,
            v_descendant_min_create
          );
      END IF;
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

CREATE OR REPLACE FUNCTION orgunit.submit_org_status_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_target_status text,
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
  v_request_hash text;
  v_target_status text;
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
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_target_status := lower(btrim(COALESCE(p_target_status, '')));
  IF v_target_status IN ('enabled', '有效') THEN
    v_target_status := 'active';
  ELSIF v_target_status IN ('inactive', '无效') THEN
    v_target_status := 'disabled';
  END IF;
  IF v_target_status NOT IN ('active', 'disabled') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_status invalid';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_request_hash := encode(
    digest(format('%s|%s|%s|CORRECT_STATUS', p_org_id, p_target_effective_date, v_target_status), 'sha256'),
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

  IF v_target.event_type NOT IN ('ENABLE', 'DISABLE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET',
      DETAIL = format('event_type=%s', v_target.event_type);
  END IF;

  SELECT * INTO v_existing_current
  FROM orgunit.org_event_corrections_current c
  WHERE c.tenant_uuid = p_tenant_uuid
    AND c.org_id = p_org_id
    AND c.event_uuid = v_target.event_uuid
  LIMIT 1;

  IF FOUND AND COALESCE(v_existing_current.replacement_payload->>'op', '') IN ('RESCIND_EVENT', 'RESCIND_ORG') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', v_target.event_uuid);
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
    'op', 'CORRECT_STATUS',
    'target_status', v_target_status,
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

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_correction_uuid;
END;
$$;

DROP FUNCTION IF EXISTS orgunit.rebuild_org_unit_versions_for_org(uuid, int);

REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM superadmin_runtime';
  END IF;
END $$;

GRANT EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) TO orgunit_kernel;
ALTER FUNCTION orgunit.replay_org_unit_versions(uuid) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.replay_org_unit_versions(uuid) SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd
