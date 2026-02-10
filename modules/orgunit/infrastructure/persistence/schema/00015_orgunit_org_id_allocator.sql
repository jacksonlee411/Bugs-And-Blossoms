CREATE TABLE IF NOT EXISTS orgunit.org_id_allocators (
  tenant_uuid uuid NOT NULL,
  next_org_id int NOT NULL CHECK (next_org_id BETWEEN 10000000 AND 100000000),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid)
);

ALTER TABLE orgunit.org_id_allocators ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_id_allocators FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_id_allocators;
CREATE POLICY tenant_isolation ON orgunit.org_id_allocators
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_next int;
  v_max int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  SELECT COALESCE(MAX(org_id), 9999999)
  INTO v_max
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid;

  IF v_max >= 99999999 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
  VALUES (p_tenant_uuid, v_max + 2)
  ON CONFLICT (tenant_uuid) DO UPDATE
  SET next_org_id = GREATEST(orgunit.org_id_allocators.next_org_id, v_max + 1) + 1,
      updated_at = now()
  WHERE orgunit.org_id_allocators.next_org_id <= 99999999
  RETURNING next_org_id - 1 INTO v_next;

  IF v_next IS NULL OR v_next > 99999999 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  RETURN v_next;
END;
$$;

ALTER TABLE IF EXISTS orgunit.org_id_allocators OWNER TO orgunit_kernel;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE orgunit.org_id_allocators TO orgunit_kernel;

ALTER FUNCTION orgunit.allocate_org_id(uuid) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SECURITY DEFINER;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SET search_path = pg_catalog, orgunit, public;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_code text,
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
  v_org_id int;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
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
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  IF p_event_type NOT IN ('CREATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN
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

  IF p_event_type = 'CREATE' AND p_org_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF FOUND THEN
      IF v_existing.tenant_uuid <> p_tenant_uuid
        OR v_existing.event_type <> p_event_type
        OR v_existing.effective_date <> p_effective_date
        OR v_existing.payload <> v_payload
        OR v_existing.request_code <> p_request_code
        OR v_existing.initiator_uuid <> p_initiator_uuid
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
          DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
      END IF;

      RETURN v_existing.id;
    END IF;

    v_org_id := orgunit.allocate_org_id(p_tenant_uuid);
  ELSE
    IF p_org_id IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
    END IF;
    v_org_id := p_org_id;
  END IF;

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_uuid <> p_event_uuid
      OR v_existing_request.org_id <> v_org_id
      OR v_existing_request.event_type <> p_event_type
      OR v_existing_request.effective_date <> p_effective_date
      OR v_existing_request.payload <> v_payload
      OR v_existing_request.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_code);
    END IF;

    RETURN v_existing_request.id;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_id, p_effective_date);

  INSERT INTO orgunit.org_events (
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid
  )
  VALUES (
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_code,
    p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_id <> v_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_code <> p_request_code
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  IF p_event_type = 'CREATE' THEN
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
    PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_org_id, v_org_code, v_parent_id, p_effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'MOVE' THEN
    v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
    PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_org_id, v_new_parent_id, p_effective_date, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.node_path <@ v_root_path;
  ELSIF p_event_type = 'RENAME' THEN
    v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
    PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_org_id, p_effective_date, v_new_name, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'DISABLE' THEN
    PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'ENABLE' THEN
    PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_org_id, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  END IF;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_id, p_effective_date);
  PERFORM orgunit.assert_org_event_snapshots(p_event_type, v_before_snapshot, v_after_snapshot);

  UPDATE orgunit.org_events
  SET before_snapshot = v_before_snapshot,
      after_snapshot = v_after_snapshot
  WHERE id = v_event_db_id;

  RETURN v_event_db_id;
END;
$$;

ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
