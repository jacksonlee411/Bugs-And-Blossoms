-- DEV-PLAN-320 P3: submit_org_event org_node_key allocator overlay
CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_org_node_key char(8);
  v_parent_org_node_key char(8);
  v_new_parent_org_node_key char(8);
  v_name text;
  v_new_name text;
  v_status text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_node_keys char(8)[];
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  IF p_event_type NOT IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF p_event_type = 'SET_BUSINESS_UNIT' THEN
    IF NOT (v_payload ? 'is_business_unit') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = 'is_business_unit is required';
    END IF;
    BEGIN
      PERFORM (v_payload->>'is_business_unit')::boolean;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
    END;
  END IF;

  IF p_event_type = 'CREATE' AND p_org_node_key IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF FOUND THEN
      IF v_existing.tenant_uuid <> p_tenant_uuid
        OR v_existing.event_type <> p_event_type
        OR v_existing.effective_date <> p_effective_date
        OR v_existing.payload <> v_payload
        OR v_existing.request_id <> p_request_id
        OR v_existing.initiator_uuid <> p_initiator_uuid
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
          DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
      END IF;

      RETURN v_existing.id;
    END IF;

    v_org_node_key := orgunit.allocate_org_node_key(p_tenant_uuid);
  ELSE
    IF p_org_node_key IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_node_key is required';
    END IF;
    v_org_node_key := p_org_node_key;
  END IF;

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    -- Idempotency key is request_id: allow server-generated event_uuid to differ across retries.
    IF v_existing_request.org_node_key <> v_org_node_key
      OR v_existing_request.event_type <> p_event_type
      OR v_existing_request.effective_date <> p_effective_date
      OR v_existing_request.payload <> v_payload
      OR v_existing_request.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing_request.id;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_node_key, p_effective_date);

  SELECT * INTO v_existing
  FROM orgunit.org_events
  WHERE event_uuid = p_event_uuid;

  IF FOUND THEN
    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_node_key <> v_org_node_key
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  IF p_event_type = 'CREATE' THEN
    v_parent_org_node_key := NULLIF(v_payload->>'parent_org_node_key', '')::char(8);
    v_name := NULLIF(btrim(v_payload->>'name'), '');
    v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
    v_org_code := NULLIF(v_payload->>'org_code', '');
    v_status := NULLIF(btrim(v_payload->>'status'), '');
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
    PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_org_node_key, v_org_code, v_parent_org_node_key, p_effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event_db_id, v_status);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_node_key = v_org_node_key
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_node_keys := ARRAY[v_org_node_key];
  ELSIF p_event_type = 'UPDATE' THEN
    PERFORM orgunit.apply_update_logic(p_tenant_uuid, v_org_node_key, p_effective_date, v_payload, v_event_db_id);

    IF (v_payload ? 'parent_org_node_key')
      OR (v_payload ? 'new_parent_org_node_key')
      OR (v_payload ? 'name')
      OR (v_payload ? 'new_name')
    THEN
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_node_key = v_org_node_key
        AND v.validity @> p_effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;

      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
      END IF;
    END IF;

    IF (v_payload ? 'parent_org_node_key') OR (v_payload ? 'new_parent_org_node_key') THEN
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_node_key = v_org_node_key
        AND v.validity @> p_effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;

      SELECT array_agg(DISTINCT v.org_node_key) INTO v_org_node_keys
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.node_path <@ v_root_path;
    ELSE
      v_org_node_keys := ARRAY[v_org_node_key];
    END IF;
  ELSIF p_event_type = 'MOVE' THEN
    v_new_parent_org_node_key := NULLIF(v_payload->>'new_parent_org_node_key', '')::char(8);
    PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_org_node_key, v_new_parent_org_node_key, p_effective_date, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_node_key = v_org_node_key
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    SELECT array_agg(DISTINCT v.org_node_key) INTO v_org_node_keys
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.node_path <@ v_root_path;
  ELSIF p_event_type = 'RENAME' THEN
    v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
    PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_org_node_key, p_effective_date, v_new_name, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_node_key = v_org_node_key
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_node_keys := ARRAY[v_org_node_key];
  ELSIF p_event_type = 'DISABLE' THEN
    PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_org_node_key, p_effective_date, v_event_db_id);
    v_org_node_keys := ARRAY[v_org_node_key];
  ELSIF p_event_type = 'ENABLE' THEN
    PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_org_node_key, p_effective_date, v_event_db_id);
    v_org_node_keys := ARRAY[v_org_node_key];
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_org_node_key, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_node_keys := ARRAY[v_org_node_key];
  END IF;

  PERFORM orgunit.apply_org_event_ext_payload(
    p_tenant_uuid,
    v_org_node_key,
    p_effective_date,
    p_event_type,
    v_payload,
    v_event_db_id
  );

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_node_keys);

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_node_key, p_effective_date);
  PERFORM orgunit.assert_org_event_snapshots(p_event_type, v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    event_uuid,
    tenant_uuid,
    org_node_key,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_uuid,
    before_snapshot,
    after_snapshot
  )
  VALUES (
    v_event_db_id,
    p_event_uuid,
    p_tenant_uuid,
    v_org_node_key,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot
  );

  RETURN v_event_db_id;
END;
$$;


ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, char(8), text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, char(8), text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, char(8), text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
