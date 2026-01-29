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
  IF v !~ '^[A-Z0-9]{5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_FORMAT',
      DETAIL = format('setid=%s', v);
  END IF;

  RETURN v;
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

CREATE OR REPLACE FUNCTION orgunit.assert_actor_scope_saas()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_scope text;
BEGIN
  v_scope := current_setting('app.current_actor_scope', true);
  IF v_scope IS NULL OR btrim(v_scope) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = 'app.current_actor_scope is required';
  END IF;
  IF v_scope <> 'saas' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = format('app.current_actor_scope=%s', v_scope);
  END IF;
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
  v_root_org_id uuid;
  v_root_valid_from date;
  v_scope_code text;
  v_scope_share_mode text;
  v_package_id uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_id = p_tenant_id AND setid = 'DEFLT'
  ) THEN
    v_evt_id := gen_random_uuid();
    INSERT INTO orgunit.setid_events (event_id, tenant_id, event_type, setid, payload, request_id, initiator_id)
    VALUES (v_evt_id, p_tenant_id, 'BOOTSTRAP', 'DEFLT', jsonb_build_object('name', 'Default'), 'bootstrap:deflt', p_initiator_id)
    ON CONFLICT (tenant_id, request_id) DO NOTHING;

    SELECT id INTO v_evt_db_id
    FROM orgunit.setid_events
    WHERE tenant_id = p_tenant_id AND request_id = 'bootstrap:deflt'
    ORDER BY id DESC
    LIMIT 1;

    INSERT INTO orgunit.setids (tenant_id, setid, name, status, last_event_id)
    VALUES (p_tenant_id, 'DEFLT', 'Default', 'active', v_evt_db_id)
    ON CONFLICT (tenant_id, setid) DO NOTHING;
  END IF;

  FOR v_scope_code, v_scope_share_mode IN
    SELECT scope_code, share_mode
    FROM orgunit.scope_code_registry()
    WHERE is_stable = true
  LOOP
    IF v_scope_share_mode = 'shared-only' THEN
      CONTINUE;
    END IF;

    SELECT p.package_id INTO v_package_id
    FROM orgunit.setid_scope_packages p
    WHERE p.tenant_id = p_tenant_id
      AND p.scope_code = v_scope_code
      AND p.package_code = 'DEFLT';

    IF v_package_id IS NULL THEN
      v_package_id := gen_random_uuid();
      PERFORM orgunit.submit_scope_package_event(
        gen_random_uuid(),
        p_tenant_id,
        v_scope_code,
        v_package_id,
        'BOOTSTRAP',
        current_date,
        jsonb_build_object('package_code', 'DEFLT', 'name', 'Default'),
        format('bootstrap:scope-package:deflt:%s', v_scope_code),
        p_initiator_id
      );

      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_id = p_tenant_id
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';
    END IF;

    IF v_package_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
        DETAIL = format('scope_code=%s', v_scope_code);
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.setid_scope_subscriptions s
      WHERE s.tenant_id = p_tenant_id
        AND s.setid = 'DEFLT'
        AND s.scope_code = v_scope_code
        AND s.validity @> current_date
    ) THEN
      PERFORM orgunit.submit_scope_subscription_event(
        gen_random_uuid(),
        p_tenant_id,
        'DEFLT',
        v_scope_code,
        v_package_id,
        p_tenant_id,
        'BOOTSTRAP',
        current_date,
        format('bootstrap:scope-subscription:deflt:%s', v_scope_code),
        p_initiator_id
      );
    END IF;
  END LOOP;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_id = p_tenant_id AND t.hierarchy_type = 'OrgUnit'
  FOR UPDATE;

  IF v_root_org_id IS NULL THEN
    RETURN;
  END IF;

  SELECT lower(v.validity)::date INTO v_root_valid_from
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = 'OrgUnit'
    AND v.org_id = v_root_org_id
    AND v.status = 'active'
    AND v.is_business_unit = true
    AND v.validity @> current_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_root_valid_from IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_BUSINESS_UNIT_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', v_root_org_id, current_date);
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.setid_binding_versions
    WHERE tenant_id = p_tenant_id
      AND org_id = v_root_org_id
      AND validity @> v_root_valid_from
  ) THEN
    PERFORM orgunit.submit_setid_binding_event(
      gen_random_uuid(),
      p_tenant_id,
      v_root_org_id,
      v_root_valid_from,
      'DEFLT',
      'bootstrap:binding:deflt',
      p_initiator_id
    );
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
  v_scope_code text;
  v_scope_share_mode text;
  v_package_id uuid;
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
  IF v_setid = 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
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

    FOR v_scope_code, v_scope_share_mode IN
      SELECT scope_code, share_mode
      FROM orgunit.scope_code_registry()
      WHERE is_stable = true
    LOOP
      IF v_scope_share_mode = 'shared-only' THEN
        CONTINUE;
      END IF;

      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_id = p_tenant_id
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';

      IF v_package_id IS NULL THEN
        v_package_id := gen_random_uuid();
        PERFORM orgunit.submit_scope_package_event(
          gen_random_uuid(),
          p_tenant_id,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          current_date,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default'),
          format('bootstrap:scope-package:deflt:%s', v_scope_code),
          p_initiator_id
        );

        SELECT p.package_id INTO v_package_id
        FROM orgunit.setid_scope_packages p
        WHERE p.tenant_id = p_tenant_id
          AND p.scope_code = v_scope_code
          AND p.package_code = 'DEFLT';
      END IF;

      IF v_package_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
          DETAIL = format('setid=%s scope_code=%s', v_setid, v_scope_code);
      END IF;

      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.setid_scope_subscriptions s
        WHERE s.tenant_id = p_tenant_id
          AND s.setid = v_setid
          AND s.scope_code = v_scope_code
          AND s.validity @> current_date
      ) THEN
        PERFORM orgunit.submit_scope_subscription_event(
          gen_random_uuid(),
          p_tenant_id,
          v_setid,
          v_scope_code,
          v_package_id,
          p_tenant_id,
          'BOOTSTRAP',
          current_date,
          format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
          p_initiator_id
        );
      END IF;
    END LOOP;
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
    IF v_setid = 'DEFLT' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_RESERVED_WORD',
        DETAIL = 'DEFLT is reserved';
    END IF;
    IF EXISTS (
      SELECT 1 FROM orgunit.setid_binding_versions
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

CREATE OR REPLACE FUNCTION orgunit.submit_global_setid_event(
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
  IF p_tenant_id <> orgunit.global_tenant_id() THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = format('tenant_id=%s', p_tenant_id);
  END IF;

  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.assert_actor_scope_saas();

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
  IF v_setid <> 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'only SHARE is allowed';
  END IF;

  INSERT INTO orgunit.global_setid_events (event_id, tenant_id, event_type, setid, payload, request_id, initiator_id)
  VALUES (p_event_id, p_tenant_id, p_event_type, v_setid, COALESCE(p_payload, '{}'::jsonb), p_request_id, p_initiator_id)
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.global_setid_events
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

    INSERT INTO orgunit.global_setids (tenant_id, setid, name, status, last_event_id)
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
    UPDATE orgunit.global_setids
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
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'SHARE cannot be disabled';
  ELSE
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_setid_binding_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_org_id uuid,
  p_effective_date date,
  p_setid text,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_evt_db_id bigint;
  v_org_status text;
  v_org_is_bu boolean;
  v_existing orgunit.setid_binding_versions%ROWTYPE;
  v_next_start date;
  v_current_end date;
  v_root_org_id uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_id is required';
  END IF;
  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'event_id is required';
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid = 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_SHARE_FORBIDDEN',
      DETAIL = 'SHARE is reserved';
  END IF;

  SELECT status INTO v_org_status
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = 'OrgUnit'
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_org_status IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;
  IF v_org_status <> 'active' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  SELECT is_business_unit INTO v_org_is_bu
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = 'OrgUnit'
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_org_is_bu IS DISTINCT FROM true THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_BUSINESS_UNIT_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_id = p_tenant_id AND setid = v_setid
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_NOT_FOUND',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  IF EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_id = p_tenant_id AND setid = v_setid AND status <> 'active'
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_DISABLED',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_id = p_tenant_id AND t.hierarchy_type = 'OrgUnit';

  IF v_root_org_id IS NOT NULL AND v_root_org_id = p_org_id AND v_setid <> 'DEFLT' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_ROOT_BINDING_FORBIDDEN',
      DETAIL = format('org_id=%s setid=%s', p_org_id, v_setid);
  END IF;

  INSERT INTO orgunit.setid_binding_events (
    event_id,
    tenant_id,
    org_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_org_id,
    'BIND',
    p_effective_date,
    jsonb_build_object('setid', v_setid),
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_binding_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  SELECT min(lower(validity)) INTO v_next_start
  FROM orgunit.setid_binding_versions
  WHERE tenant_id = p_tenant_id
    AND org_id = p_org_id
    AND lower(validity) > p_effective_date;

  SELECT * INTO v_existing
  FROM orgunit.setid_binding_versions
  WHERE tenant_id = p_tenant_id
    AND org_id = p_org_id
    AND validity @> p_effective_date
  ORDER BY lower(validity) DESC
  LIMIT 1
  FOR UPDATE;

  BEGIN
    IF FOUND THEN
      v_current_end := upper(v_existing.validity);
      IF lower(v_existing.validity) = p_effective_date THEN
        UPDATE orgunit.setid_binding_versions
        SET setid = v_setid,
            last_event_id = v_evt_db_id,
            updated_at = now()
        WHERE id = v_existing.id;
      ELSE
        UPDATE orgunit.setid_binding_versions
        SET validity = daterange(lower(v_existing.validity), p_effective_date, '[)'),
            updated_at = now()
        WHERE id = v_existing.id;

        INSERT INTO orgunit.setid_binding_versions (
          tenant_id,
          org_id,
          setid,
          validity,
          last_event_id
        )
        VALUES (
          p_tenant_id,
          p_org_id,
          v_setid,
          daterange(p_effective_date, v_current_end, '[)'),
          v_evt_db_id
        );
      END IF;
    ELSE
      INSERT INTO orgunit.setid_binding_versions (
        tenant_id,
        org_id,
        setid,
        validity,
        last_event_id
      )
      VALUES (
        p_tenant_id,
        p_org_id,
        v_setid,
        daterange(p_effective_date, v_next_start, '[)'),
        v_evt_db_id
      );
    END IF;
  EXCEPTION
    WHEN exclusion_violation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_BINDING_OVERLAP',
        DETAIL = format('org_id=%s effective_date=%s', p_org_id, p_effective_date);
  END;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.resolve_setid(
  p_tenant_id uuid,
  p_org_id uuid,
  p_as_of_date date
)
RETURNS text
LANGUAGE plpgsql
AS $$
DECLARE
  v_node_path ltree;
  v_org_status text;
  v_setid text;
  v_setid_status text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'org_id is required';
  END IF;
  IF p_as_of_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'as_of_date is required';
  END IF;

  SELECT v.status, v.node_path INTO v_org_status, v_node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = 'OrgUnit'
    AND v.org_id = p_org_id
    AND v.validity @> p_as_of_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_org_status IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_as_of_date);
  END IF;
  IF v_org_status <> 'active' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_as_of_date);
  END IF;

  SELECT b.setid INTO v_setid
  FROM orgunit.setid_binding_versions b
  JOIN orgunit.org_unit_versions o
    ON o.tenant_id = b.tenant_id
   AND o.hierarchy_type = 'OrgUnit'
   AND o.org_id = b.org_id
  WHERE b.tenant_id = p_tenant_id
    AND b.validity @> p_as_of_date
    AND o.validity @> p_as_of_date
    AND o.status = 'active'
    AND o.is_business_unit = true
    AND o.node_path @> v_node_path
  ORDER BY nlevel(o.node_path) DESC
  LIMIT 1;

  IF v_setid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_BINDING_MISSING',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_as_of_date);
  END IF;

  SELECT status INTO v_setid_status
  FROM orgunit.setids
  WHERE tenant_id = p_tenant_id AND setid = v_setid;

  IF v_setid_status IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_NOT_FOUND',
      DETAIL = format('setid=%s', v_setid);
  END IF;
  IF v_setid_status <> 'active' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_DISABLED',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  RETURN v_setid;
END;
$$;
