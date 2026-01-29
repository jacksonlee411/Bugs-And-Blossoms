CREATE OR REPLACE FUNCTION orgunit.submit_scope_package_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_scope_code text,
  p_package_id uuid,
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
  v_evt_db_id bigint;
  v_payload jsonb;
  v_scope_mode text;
  v_package_code text;
  v_name text;
  v_status text;
  v_existing_pkg orgunit.setid_scope_packages%ROWTYPE;
  v_existing_version orgunit.setid_scope_package_versions%ROWTYPE;
  v_next_start date;
  v_current_end date;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'event_id is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'REQUEST_ID_REQUIRED';
  END IF;
  IF p_scope_code IS NULL OR NOT orgunit.scope_code_is_valid(p_scope_code) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_CODE_INVALID';
  END IF;
  IF p_package_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_NOT_FOUND';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('BOOTSTRAP', 'CREATE', 'RENAME', 'DISABLE') THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  v_scope_mode := orgunit.scope_code_share_mode(p_scope_code);
  IF v_scope_mode = 'shared-only' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_SCOPE_MISMATCH';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'payload must be an object';
  END IF;

  INSERT INTO orgunit.setid_scope_package_events (
    event_id,
    tenant_id,
    scope_code,
    package_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_scope_code,
    p_package_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_scope_package_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_scope_package_versions
    WHERE last_event_id = v_evt_db_id
  ) THEN
    RETURN v_evt_db_id;
  END IF;

  IF p_event_type IN ('BOOTSTRAP', 'CREATE') THEN
    v_package_code := upper(btrim(COALESCE(v_payload->>'package_code', '')));
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');

    IF v_package_code = '' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_INVALID';
    END IF;
    IF v_package_code !~ '^[A-Z0-9_]{1,16}$' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_INVALID';
    END IF;
    IF p_event_type = 'CREATE' AND v_package_code = 'DEFLT' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_RESERVED';
    END IF;
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    IF EXISTS (
      SELECT 1
      FROM orgunit.setid_scope_packages
      WHERE tenant_id = p_tenant_id
        AND scope_code = p_scope_code
        AND package_code = v_package_code
        AND package_id <> p_package_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_DUPLICATE';
    END IF;

    v_status := 'active';

    INSERT INTO orgunit.setid_scope_packages (
      tenant_id,
      scope_code,
      package_id,
      package_code,
      name,
      status
    )
    VALUES (
      p_tenant_id,
      p_scope_code,
      p_package_id,
      v_package_code,
      v_name,
      v_status
    )
    ON CONFLICT (tenant_id, package_id) DO UPDATE
    SET scope_code = EXCLUDED.scope_code,
        package_code = EXCLUDED.package_code,
        name = EXCLUDED.name,
        status = EXCLUDED.status,
        updated_at = now();
  ELSIF p_event_type = 'RENAME' THEN
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    SELECT * INTO v_existing_pkg
    FROM orgunit.setid_scope_packages
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id
    FOR UPDATE;

    IF v_existing_pkg.package_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_NOT_FOUND';
    END IF;

    v_package_code := v_existing_pkg.package_code;
    v_status := v_existing_pkg.status;

    UPDATE orgunit.setid_scope_packages
    SET name = v_name,
        updated_at = now()
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id;
  ELSIF p_event_type = 'DISABLE' THEN
    SELECT * INTO v_existing_pkg
    FROM orgunit.setid_scope_packages
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id
    FOR UPDATE;

    IF v_existing_pkg.package_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_NOT_FOUND';
    END IF;
    IF v_existing_pkg.package_code = 'DEFLT' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_DEFLT_FORBIDDEN';
    END IF;

    v_package_code := v_existing_pkg.package_code;
    v_name := v_existing_pkg.name;
    v_status := 'disabled';

    UPDATE orgunit.setid_scope_packages
    SET status = 'disabled',
        updated_at = now()
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id;
  END IF;

  SELECT min(lower(validity)) INTO v_next_start
  FROM orgunit.setid_scope_package_versions
  WHERE tenant_id = p_tenant_id
    AND package_id = p_package_id
    AND lower(validity) > p_effective_date;

  SELECT * INTO v_existing_version
  FROM orgunit.setid_scope_package_versions
  WHERE tenant_id = p_tenant_id
    AND package_id = p_package_id
    AND validity @> p_effective_date
  ORDER BY lower(validity) DESC
  LIMIT 1
  FOR UPDATE;

  BEGIN
    IF FOUND THEN
      v_current_end := upper(v_existing_version.validity);
      IF lower(v_existing_version.validity) = p_effective_date THEN
        UPDATE orgunit.setid_scope_package_versions
        SET scope_code = p_scope_code,
            package_code = v_package_code,
            name = v_name,
            status = v_status,
            last_event_id = v_evt_db_id
        WHERE id = v_existing_version.id;
      ELSE
        UPDATE orgunit.setid_scope_package_versions
        SET validity = daterange(lower(v_existing_version.validity), p_effective_date, '[)')
        WHERE id = v_existing_version.id;

        INSERT INTO orgunit.setid_scope_package_versions (
          tenant_id,
          scope_code,
          package_id,
          package_code,
          name,
          status,
          validity,
          last_event_id
        )
        VALUES (
          p_tenant_id,
          p_scope_code,
          p_package_id,
          v_package_code,
          v_name,
          v_status,
          daterange(p_effective_date, v_current_end, '[)'),
          v_evt_db_id
        );
      END IF;
    ELSE
      INSERT INTO orgunit.setid_scope_package_versions (
        tenant_id,
        scope_code,
        package_id,
        package_code,
        name,
        status,
        validity,
        last_event_id
      )
      VALUES (
        p_tenant_id,
        p_scope_code,
        p_package_id,
        v_package_code,
        v_name,
        v_status,
        daterange(p_effective_date, v_next_start, '[)'),
        v_evt_db_id
      );
    END IF;
  EXCEPTION
    WHEN exclusion_violation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_VERSION_OVERLAP',
        DETAIL = format('package_id=%s effective_date=%s', p_package_id, p_effective_date);
  END;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_global_scope_package_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_scope_code text,
  p_package_id uuid,
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
  v_evt_db_id bigint;
  v_payload jsonb;
  v_scope_mode text;
  v_package_code text;
  v_name text;
  v_status text;
  v_existing_pkg orgunit.global_setid_scope_packages%ROWTYPE;
  v_existing_version orgunit.global_setid_scope_package_versions%ROWTYPE;
  v_next_start date;
  v_current_end date;
BEGIN
  IF p_tenant_id <> orgunit.global_tenant_id() THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = format('tenant_id=%s', p_tenant_id);
  END IF;

  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.assert_actor_scope_saas();

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'event_id is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'REQUEST_ID_REQUIRED';
  END IF;
  IF p_scope_code IS NULL OR NOT orgunit.scope_code_is_valid(p_scope_code) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_CODE_INVALID';
  END IF;
  IF p_package_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_NOT_FOUND';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('BOOTSTRAP', 'CREATE', 'RENAME', 'DISABLE') THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  v_scope_mode := orgunit.scope_code_share_mode(p_scope_code);
  IF v_scope_mode = 'tenant-only' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_SCOPE_MISMATCH';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
      DETAIL = 'payload must be an object';
  END IF;

  INSERT INTO orgunit.global_setid_scope_package_events (
    event_id,
    tenant_id,
    scope_code,
    package_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_scope_code,
    p_package_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.global_setid_scope_package_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  IF EXISTS (
    SELECT 1
    FROM orgunit.global_setid_scope_package_versions
    WHERE last_event_id = v_evt_db_id
  ) THEN
    RETURN v_evt_db_id;
  END IF;

  IF p_event_type IN ('BOOTSTRAP', 'CREATE') THEN
    v_package_code := upper(btrim(COALESCE(v_payload->>'package_code', '')));
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');

    IF v_package_code = '' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_INVALID';
    END IF;
    IF v_package_code !~ '^[A-Z0-9_]{1,16}$' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_INVALID';
    END IF;
    IF p_event_type = 'CREATE' AND v_package_code = 'DEFLT' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_RESERVED';
    END IF;
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    IF EXISTS (
      SELECT 1
      FROM orgunit.global_setid_scope_packages
      WHERE tenant_id = p_tenant_id
        AND scope_code = p_scope_code
        AND package_code = v_package_code
        AND package_id <> p_package_id
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_CODE_DUPLICATE';
    END IF;

    v_status := 'active';

    INSERT INTO orgunit.global_setid_scope_packages (
      tenant_id,
      scope_code,
      package_id,
      package_code,
      name,
      status
    )
    VALUES (
      p_tenant_id,
      p_scope_code,
      p_package_id,
      v_package_code,
      v_name,
      v_status
    )
    ON CONFLICT (tenant_id, package_id) DO UPDATE
    SET scope_code = EXCLUDED.scope_code,
        package_code = EXCLUDED.package_code,
        name = EXCLUDED.name,
        status = EXCLUDED.status,
        updated_at = now();
  ELSIF p_event_type = 'RENAME' THEN
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    SELECT * INTO v_existing_pkg
    FROM orgunit.global_setid_scope_packages
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id
    FOR UPDATE;

    IF v_existing_pkg.package_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_NOT_FOUND';
    END IF;

    v_package_code := v_existing_pkg.package_code;
    v_status := v_existing_pkg.status;

    UPDATE orgunit.global_setid_scope_packages
    SET name = v_name,
        updated_at = now()
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id;
  ELSIF p_event_type = 'DISABLE' THEN
    SELECT * INTO v_existing_pkg
    FROM orgunit.global_setid_scope_packages
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id
    FOR UPDATE;

    IF v_existing_pkg.package_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_NOT_FOUND';
    END IF;
    IF v_existing_pkg.package_code = 'DEFLT' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_DEFLT_FORBIDDEN';
    END IF;

    v_package_code := v_existing_pkg.package_code;
    v_name := v_existing_pkg.name;
    v_status := 'disabled';

    UPDATE orgunit.global_setid_scope_packages
    SET status = 'disabled',
        updated_at = now()
    WHERE tenant_id = p_tenant_id
      AND package_id = p_package_id;
  END IF;

  SELECT min(lower(validity)) INTO v_next_start
  FROM orgunit.global_setid_scope_package_versions
  WHERE tenant_id = p_tenant_id
    AND package_id = p_package_id
    AND lower(validity) > p_effective_date;

  SELECT * INTO v_existing_version
  FROM orgunit.global_setid_scope_package_versions
  WHERE tenant_id = p_tenant_id
    AND package_id = p_package_id
    AND validity @> p_effective_date
  ORDER BY lower(validity) DESC
  LIMIT 1
  FOR UPDATE;

  BEGIN
    IF FOUND THEN
      v_current_end := upper(v_existing_version.validity);
      IF lower(v_existing_version.validity) = p_effective_date THEN
        UPDATE orgunit.global_setid_scope_package_versions
        SET scope_code = p_scope_code,
            package_code = v_package_code,
            name = v_name,
            status = v_status,
            last_event_id = v_evt_db_id
        WHERE id = v_existing_version.id;
      ELSE
        UPDATE orgunit.global_setid_scope_package_versions
        SET validity = daterange(lower(v_existing_version.validity), p_effective_date, '[)')
        WHERE id = v_existing_version.id;

        INSERT INTO orgunit.global_setid_scope_package_versions (
          tenant_id,
          scope_code,
          package_id,
          package_code,
          name,
          status,
          validity,
          last_event_id
        )
        VALUES (
          p_tenant_id,
          p_scope_code,
          p_package_id,
          v_package_code,
          v_name,
          v_status,
          daterange(p_effective_date, v_current_end, '[)'),
          v_evt_db_id
        );
      END IF;
    ELSE
      INSERT INTO orgunit.global_setid_scope_package_versions (
        tenant_id,
        scope_code,
        package_id,
        package_code,
        name,
        status,
        validity,
        last_event_id
      )
      VALUES (
        p_tenant_id,
        p_scope_code,
        p_package_id,
        v_package_code,
        v_name,
        v_status,
        daterange(p_effective_date, v_next_start, '[)'),
        v_evt_db_id
      );
    END IF;
  EXCEPTION
    WHEN exclusion_violation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'PACKAGE_VERSION_OVERLAP',
        DETAIL = format('package_id=%s effective_date=%s', p_package_id, p_effective_date);
  END;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_scope_subscription_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_setid text,
  p_scope_code text,
  p_package_id uuid,
  p_package_owner_tenant_id uuid,
  p_event_type text,
  p_effective_date date,
  p_request_id text,
  p_initiator_id uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_evt_db_id bigint;
  v_scope_mode text;
  v_existing orgunit.setid_scope_subscriptions%ROWTYPE;
  v_next_start date;
  v_current_end date;
  v_setid_status text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_SUBSCRIPTION_INVALID_ARGUMENT',
      DETAIL = 'event_id is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'REQUEST_ID_REQUIRED';
  END IF;
  IF p_scope_code IS NULL OR NOT orgunit.scope_code_is_valid(p_scope_code) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_CODE_INVALID';
  END IF;
  IF p_package_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_NOT_FOUND';
  END IF;
  IF p_package_owner_tenant_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_OWNER_INVALID';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_SUBSCRIPTION_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_SUBSCRIPTION_INVALID_ARGUMENT',
      DETAIL = 'initiator_id is required';
  END IF;
  IF p_event_type NOT IN ('BOOTSTRAP', 'SUBSCRIBE') THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SCOPE_SUBSCRIPTION_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid = 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'SHARE is reserved';
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

  IF p_package_owner_tenant_id <> p_tenant_id
     AND p_package_owner_tenant_id <> orgunit.global_tenant_id() THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'PACKAGE_OWNER_INVALID';
  END IF;

  v_scope_mode := orgunit.scope_code_share_mode(p_scope_code);
  IF v_scope_mode = 'shared-only' THEN
    PERFORM orgunit.assert_actor_scope_saas();
  END IF;

  PERFORM orgunit.assert_scope_package_active_as_of(
    p_tenant_id,
    p_scope_code,
    p_package_id,
    p_package_owner_tenant_id,
    p_effective_date
  );

  INSERT INTO orgunit.setid_scope_subscription_events (
    event_id,
    tenant_id,
    setid,
    scope_code,
    package_id,
    package_owner_tenant_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    v_setid,
    p_scope_code,
    p_package_id,
    p_package_owner_tenant_id,
    p_event_type,
    p_effective_date,
    '{}'::jsonb,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (tenant_id, request_id) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_scope_subscription_events
  WHERE tenant_id = p_tenant_id AND request_id = p_request_id
  ORDER BY id DESC
  LIMIT 1;

  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_scope_subscriptions
    WHERE last_event_id = v_evt_db_id
  ) THEN
    RETURN v_evt_db_id;
  END IF;

  SELECT min(lower(validity)) INTO v_next_start
  FROM orgunit.setid_scope_subscriptions
  WHERE tenant_id = p_tenant_id
    AND setid = v_setid
    AND scope_code = p_scope_code
    AND lower(validity) > p_effective_date;

  SELECT * INTO v_existing
  FROM orgunit.setid_scope_subscriptions
  WHERE tenant_id = p_tenant_id
    AND setid = v_setid
    AND scope_code = p_scope_code
    AND validity @> p_effective_date
  ORDER BY lower(validity) DESC
  LIMIT 1
  FOR UPDATE;

  BEGIN
    IF FOUND THEN
      v_current_end := upper(v_existing.validity);
      IF lower(v_existing.validity) = p_effective_date THEN
        UPDATE orgunit.setid_scope_subscriptions
        SET package_id = p_package_id,
            package_owner_tenant_id = p_package_owner_tenant_id,
            last_event_id = v_evt_db_id,
            updated_at = now()
        WHERE id = v_existing.id;
      ELSE
        UPDATE orgunit.setid_scope_subscriptions
        SET validity = daterange(lower(v_existing.validity), p_effective_date, '[)'),
            updated_at = now()
        WHERE id = v_existing.id;

        INSERT INTO orgunit.setid_scope_subscriptions (
          tenant_id,
          setid,
          scope_code,
          package_id,
          package_owner_tenant_id,
          validity,
          last_event_id
        )
        VALUES (
          p_tenant_id,
          v_setid,
          p_scope_code,
          p_package_id,
          p_package_owner_tenant_id,
          daterange(p_effective_date, v_current_end, '[)'),
          v_evt_db_id
        );
      END IF;
    ELSE
      INSERT INTO orgunit.setid_scope_subscriptions (
        tenant_id,
        setid,
        scope_code,
        package_id,
        package_owner_tenant_id,
        validity,
        last_event_id
      )
      VALUES (
        p_tenant_id,
        v_setid,
        p_scope_code,
        p_package_id,
        p_package_owner_tenant_id,
        daterange(p_effective_date, v_next_start, '[)'),
        v_evt_db_id
      );
    END IF;
  EXCEPTION
    WHEN exclusion_violation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SUBSCRIPTION_OVERLAP',
        DETAIL = format('setid=%s scope_code=%s effective_date=%s', v_setid, p_scope_code, p_effective_date);
  END;

  RETURN v_evt_db_id;
END;
$$;
