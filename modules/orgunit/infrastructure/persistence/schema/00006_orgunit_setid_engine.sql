CREATE OR REPLACE FUNCTION orgunit.normalize_setid(p_setid text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_setid IS NULL OR btrim(p_setid) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_FORMAT',
      DETAIL = 'setid is required';
  END IF;

  v := upper(btrim(p_setid));
  IF v !~ '^[A-Z0-9]{1,5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_FORMAT',
      DETAIL = format('setid=%s', v);
  END IF;

  RETURN v;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.normalize_business_unit_id(p_business_unit_id text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_business_unit_id IS NULL OR btrim(p_business_unit_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_INVALID_ID',
      DETAIL = 'business_unit_id is required';
  END IF;

  v := upper(btrim(p_business_unit_id));
  IF v !~ '^[A-Z0-9]{1,5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_INVALID_ID',
      DETAIL = format('business_unit_id=%s', v);
  END IF;

  RETURN v;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.assert_record_group(p_record_group text)
RETURNS void
LANGUAGE plpgsql
IMMUTABLE
AS $$
BEGIN
  IF p_record_group IS NULL OR btrim(p_record_group) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RECORD_GROUP_UNKNOWN',
      DETAIL = 'record_group is required';
  END IF;
  IF p_record_group <> 'jobcatalog' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RECORD_GROUP_UNKNOWN',
      DETAIL = format('record_group=%s', p_record_group);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.lock_setid_governance(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  k bigint;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  k := hashtextextended('orgunit.setid.governance:' || p_tenant_id::text, 0);
  PERFORM pg_advisory_xact_lock(k);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.ensure_setid_bootstrap(
  p_tenant_id uuid,
  p_initiator_id uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_evt_id uuid;
  v_evt_db_id bigint;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_id = p_tenant_id AND setid = 'SHARE'
  ) THEN
    v_evt_id := gen_random_uuid();
    INSERT INTO orgunit.setid_events (event_id, tenant_id, event_type, setid, payload, request_id, initiator_id)
    VALUES (v_evt_id, p_tenant_id, 'BOOTSTRAP', 'SHARE', jsonb_build_object('name', 'Shared'), 'bootstrap:share', p_initiator_id)
    ON CONFLICT (tenant_id, request_id) DO NOTHING;

    SELECT id INTO v_evt_db_id
    FROM orgunit.setid_events
    WHERE tenant_id = p_tenant_id AND request_id = 'bootstrap:share'
    ORDER BY id DESC
    LIMIT 1;

    INSERT INTO orgunit.setids (tenant_id, setid, name, status, last_event_id)
    VALUES (p_tenant_id, 'SHARE', 'Shared', 'active', v_evt_db_id)
    ON CONFLICT (tenant_id, setid) DO NOTHING;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.business_units WHERE tenant_id = p_tenant_id AND business_unit_id = 'BU000'
  ) THEN
    v_evt_id := gen_random_uuid();
    INSERT INTO orgunit.business_unit_events (event_id, tenant_id, event_type, business_unit_id, payload, request_id, initiator_id)
    VALUES (v_evt_id, p_tenant_id, 'BOOTSTRAP', 'BU000', jsonb_build_object('name', 'Default BU'), 'bootstrap:bu000', p_initiator_id)
    ON CONFLICT (tenant_id, request_id) DO NOTHING;

    SELECT id INTO v_evt_db_id
    FROM orgunit.business_unit_events
    WHERE tenant_id = p_tenant_id AND request_id = 'bootstrap:bu000'
    ORDER BY id DESC
    LIMIT 1;

    INSERT INTO orgunit.business_units (tenant_id, business_unit_id, name, status, last_event_id)
    VALUES (p_tenant_id, 'BU000', 'Default BU', 'active', v_evt_db_id)
    ON CONFLICT (tenant_id, business_unit_id) DO NOTHING;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.set_control_mappings
    WHERE tenant_id = p_tenant_id AND business_unit_id = 'BU000' AND record_group = 'jobcatalog'
  ) THEN
    v_evt_id := gen_random_uuid();
    INSERT INTO orgunit.set_control_mapping_events (event_id, tenant_id, event_type, record_group, payload, request_id, initiator_id)
    VALUES (
      v_evt_id,
      p_tenant_id,
      'BOOTSTRAP',
      'jobcatalog',
      jsonb_build_object('mappings', jsonb_build_array(jsonb_build_object('business_unit_id', 'BU000', 'setid', 'SHARE'))),
      'bootstrap:mappings:jobcatalog',
      p_initiator_id
    )
    ON CONFLICT (tenant_id, request_id) DO NOTHING;

    SELECT id INTO v_evt_db_id
    FROM orgunit.set_control_mapping_events
    WHERE tenant_id = p_tenant_id AND request_id = 'bootstrap:mappings:jobcatalog'
    ORDER BY id DESC
    LIMIT 1;

    INSERT INTO orgunit.set_control_mappings (tenant_id, business_unit_id, record_group, setid, last_event_id)
    VALUES (p_tenant_id, 'BU000', 'jobcatalog', 'SHARE', v_evt_db_id)
    ON CONFLICT (tenant_id, business_unit_id, record_group) DO NOTHING;
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_setid_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_event_type text,
  p_setid text,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_evt_db_id bigint;
  v_name text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_id is required';
  END IF;

  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'event_type is required';
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid = 'SHARE' AND p_event_type <> 'BOOTSTRAP' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED',
      DETAIL = 'SHARE is reserved';
  END IF;

  INSERT INTO orgunit.setid_events (event_id, tenant_id, event_type, setid, payload, request_id, initiator_id)
  VALUES (p_event_id, p_tenant_id, p_event_type, v_setid, COALESCE(p_payload, '{}'::jsonb), p_request_id, p_initiator_id)
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  IF p_event_type IN ('BOOTSTRAP','CREATE') THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    IF p_event_type = 'CREATE' AND EXISTS (
      SELECT 1 FROM orgunit.setids WHERE tenant_id = p_tenant_id AND setid = v_setid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_ALREADY_EXISTS',
        DETAIL = format('setid=%s', v_setid);
    END IF;

    INSERT INTO orgunit.setids (tenant_id, setid, name, status, last_event_id)
    VALUES (p_tenant_id, v_setid, v_name, 'active', v_evt_db_id)
    ON CONFLICT (tenant_id, setid) DO UPDATE
    SET name = EXCLUDED.name,
        status = 'active',
        last_event_id = EXCLUDED.last_event_id,
        updated_at = now();
  ELSIF p_event_type = 'RENAME' THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;
    UPDATE orgunit.setids
    SET name = v_name,
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_id = p_tenant_id AND setid = v_setid;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_setid);
    END IF;
  ELSIF p_event_type = 'DISABLE' THEN
    IF v_setid = 'SHARE' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_RESERVED',
        DETAIL = 'SHARE is reserved';
    END IF;
    IF EXISTS (
      SELECT 1 FROM orgunit.set_control_mappings
      WHERE tenant_id = p_tenant_id AND setid = v_setid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_IN_USE',
        DETAIL = format('setid=%s', v_setid);
    END IF;
    UPDATE orgunit.setids
    SET status = 'disabled',
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_id = p_tenant_id AND setid = v_setid;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_setid);
    END IF;
  ELSE
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_business_unit_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_event_type text,
  p_business_unit_id text,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_bu text;
  v_evt_db_id bigint;
  v_name text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);
  PERFORM orgunit.ensure_setid_bootstrap(p_tenant_id, p_initiator_id);

  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_INVALID_ARGUMENT',
      DETAIL = 'request_id is required';
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_INVALID_ARGUMENT',
      DETAIL = 'event_type is required';
  END IF;

  v_bu := orgunit.normalize_business_unit_id(p_business_unit_id);

  INSERT INTO orgunit.business_unit_events (event_id, tenant_id, event_type, business_unit_id, payload, request_id, initiator_id)
  VALUES (p_event_id, p_tenant_id, p_event_type, v_bu, COALESCE(p_payload, '{}'::jsonb), p_request_id, p_initiator_id)
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.business_unit_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  IF p_event_type IN ('BOOTSTRAP','CREATE') THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    IF p_event_type = 'CREATE' AND EXISTS (
      SELECT 1 FROM orgunit.business_units WHERE tenant_id = p_tenant_id AND business_unit_id = v_bu
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_ALREADY_EXISTS',
        DETAIL = format('business_unit_id=%s', v_bu);
    END IF;

    INSERT INTO orgunit.business_units (tenant_id, business_unit_id, name, status, last_event_id)
    VALUES (p_tenant_id, v_bu, v_name, 'active', v_evt_db_id)
    ON CONFLICT (tenant_id, business_unit_id) DO UPDATE
    SET name = EXCLUDED.name,
        status = 'active',
        last_event_id = EXCLUDED.last_event_id,
        updated_at = now();

    IF NOT EXISTS (
      SELECT 1 FROM orgunit.set_control_mappings
      WHERE tenant_id = p_tenant_id AND business_unit_id = v_bu AND record_group = 'jobcatalog'
    ) THEN
      PERFORM orgunit.put_set_control_mappings(
        gen_random_uuid(),
        p_tenant_id,
        'jobcatalog',
        jsonb_build_array(jsonb_build_object('business_unit_id', v_bu, 'setid', 'SHARE')),
        'bootstrap:mappings:jobcatalog:' || v_bu,
        p_initiator_id
      );
    END IF;
  ELSIF p_event_type = 'RENAME' THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;
    UPDATE orgunit.business_units
    SET name = v_name,
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_id = p_tenant_id AND business_unit_id = v_bu;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_NOT_FOUND',
        DETAIL = format('business_unit_id=%s', v_bu);
    END IF;
  ELSIF p_event_type = 'DISABLE' THEN
    UPDATE orgunit.business_units
    SET status = 'disabled',
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_id = p_tenant_id AND business_unit_id = v_bu;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_NOT_FOUND',
        DETAIL = format('business_unit_id=%s', v_bu);
    END IF;
  ELSE
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.put_set_control_mappings(
  p_event_id uuid,
  p_tenant_id uuid,
  p_record_group text,
  p_mappings jsonb,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_evt_db_id bigint;
  v_row jsonb;
  v_bu text;
  v_setid text;
  v_disabled text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);
  PERFORM orgunit.assert_record_group(p_record_group);
  PERFORM orgunit.ensure_setid_bootstrap(p_tenant_id, p_initiator_id);

  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_id is required';
  END IF;
  IF jsonb_typeof(p_mappings) <> 'array' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'mappings must be an array';
  END IF;

  INSERT INTO orgunit.set_control_mapping_events (event_id, tenant_id, event_type, record_group, payload, request_id, initiator_id)
  VALUES (p_event_id, p_tenant_id, 'PUT', p_record_group, jsonb_build_object('mappings', p_mappings), p_request_id, p_initiator_id)
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.set_control_mapping_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  FOR v_row IN SELECT * FROM jsonb_array_elements(p_mappings)
  LOOP
    v_bu := orgunit.normalize_business_unit_id(v_row->>'business_unit_id');
    v_setid := orgunit.normalize_setid(v_row->>'setid');

    SELECT status INTO v_disabled
    FROM orgunit.business_units
    WHERE tenant_id = p_tenant_id AND business_unit_id = v_bu;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_NOT_FOUND',
        DETAIL = format('business_unit_id=%s', v_bu);
    END IF;
    IF v_disabled = 'disabled' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'BUSINESS_UNIT_DISABLED',
        DETAIL = format('business_unit_id=%s', v_bu);
    END IF;

    SELECT status INTO v_disabled
    FROM orgunit.setids
    WHERE tenant_id = p_tenant_id AND setid = v_setid;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_setid);
    END IF;
    IF v_disabled = 'disabled' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_DISABLED',
        DETAIL = format('setid=%s', v_setid);
    END IF;

    INSERT INTO orgunit.set_control_mappings (tenant_id, business_unit_id, record_group, setid, last_event_id, updated_at)
    VALUES (p_tenant_id, v_bu, p_record_group, v_setid, v_evt_db_id, now())
    ON CONFLICT (tenant_id, business_unit_id, record_group) DO UPDATE
    SET setid = EXCLUDED.setid,
        last_event_id = EXCLUDED.last_event_id,
        updated_at = now();
  END LOOP;

  IF EXISTS (
    SELECT 1
    FROM orgunit.business_units bu
    WHERE bu.tenant_id = p_tenant_id
      AND bu.status = 'active'
      AND NOT EXISTS (
        SELECT 1
        FROM orgunit.set_control_mappings m
        WHERE m.tenant_id = bu.tenant_id
          AND m.business_unit_id = bu.business_unit_id
          AND m.record_group = p_record_group
      )
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_MAPPING_MISSING',
      DETAIL = format('record_group=%s', p_record_group);
  END IF;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.resolve_setid(
  p_tenant_id uuid,
  p_business_unit_id text,
  p_record_group text
)
RETURNS text
LANGUAGE plpgsql
AS $$
DECLARE
  v_bu text;
  v_rg text;
  v_setid text;
  v_status text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  v_bu := orgunit.normalize_business_unit_id(p_business_unit_id);
  v_rg := lower(btrim(p_record_group));
  PERFORM orgunit.assert_record_group(v_rg);

  SELECT status INTO v_status
  FROM orgunit.business_units
  WHERE tenant_id = p_tenant_id AND business_unit_id = v_bu;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_NOT_FOUND',
      DETAIL = format('business_unit_id=%s', v_bu);
  END IF;
  IF v_status = 'disabled' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'BUSINESS_UNIT_DISABLED',
      DETAIL = format('business_unit_id=%s', v_bu);
  END IF;

  SELECT setid INTO v_setid
  FROM orgunit.set_control_mappings
  WHERE tenant_id = p_tenant_id
    AND business_unit_id = v_bu
    AND record_group = v_rg;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_MAPPING_MISSING',
      DETAIL = format('business_unit_id=%s record_group=%s', v_bu, v_rg);
  END IF;

  SELECT status INTO v_status
  FROM orgunit.setids
  WHERE tenant_id = p_tenant_id AND setid = v_setid;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_NOT_FOUND',
      DETAIL = format('setid=%s', v_setid);
  END IF;
  IF v_status = 'disabled' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_DISABLED',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  RETURN v_setid;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.assert_setid_active(
  p_tenant_id uuid,
  p_setid text
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_status text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  v_setid := orgunit.normalize_setid(p_setid);

  SELECT status INTO v_status
  FROM orgunit.setids
  WHERE tenant_id = p_tenant_id AND setid = v_setid;
  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_NOT_FOUND',
      DETAIL = format('setid=%s', v_setid);
  END IF;
  IF v_status = 'disabled' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_DISABLED',
      DETAIL = format('setid=%s', v_setid);
  END IF;
END;
$$;
