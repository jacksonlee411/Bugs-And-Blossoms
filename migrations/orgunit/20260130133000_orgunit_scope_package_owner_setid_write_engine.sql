-- +goose Up
-- +goose StatementBegin
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
  v_global_tenant_id uuid;
  v_prev_actor text;
  v_prev_allow_share text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  v_global_tenant_id := orgunit.global_tenant_id();
  v_prev_actor := current_setting('app.current_actor_scope', true);
  v_prev_allow_share := current_setting('app.allow_share_read', true);

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

  FOR v_scope_code, v_scope_share_mode IN
    SELECT scope_code, share_mode
    FROM orgunit.scope_code_registry()
    WHERE is_stable = true
  LOOP
    IF v_scope_share_mode = 'shared-only' THEN
      PERFORM set_config('app.current_actor_scope', 'saas', true);
      PERFORM set_config('app.current_tenant', v_global_tenant_id::text, true);
      PERFORM set_config('app.allow_share_read', 'on', true);

      SELECT p.package_id INTO v_package_id
      FROM orgunit.global_setid_scope_packages p
      WHERE p.tenant_id = v_global_tenant_id
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';

      IF v_package_id IS NULL THEN
        v_package_id := gen_random_uuid();
        PERFORM orgunit.submit_global_scope_package_event(
          gen_random_uuid(),
          v_global_tenant_id,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          v_root_valid_from,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
          format('bootstrap:global-scope-package:deflt:%s', v_scope_code),
          v_global_tenant_id
        );

        SELECT p.package_id INTO v_package_id
        FROM orgunit.global_setid_scope_packages p
        WHERE p.tenant_id = v_global_tenant_id
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
        FROM orgunit.global_setid_scope_package_versions v
        WHERE v.tenant_id = v_global_tenant_id
          AND v.scope_code = v_scope_code
          AND v.package_id = v_package_id
          AND v.status = 'active'
          AND v.validity @> v_root_valid_from
      ) THEN
        PERFORM orgunit.submit_global_scope_package_event(
          gen_random_uuid(),
          v_global_tenant_id,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          v_root_valid_from,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
          format('bootstrap:global-scope-package:deflt:%s:%s', v_scope_code, v_root_valid_from),
          v_global_tenant_id
        );
      END IF;

      PERFORM set_config('app.current_tenant', p_tenant_id::text, true);
      PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);

      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.setid_scope_subscriptions s
        WHERE s.tenant_id = p_tenant_id
          AND s.setid = 'DEFLT'
          AND s.scope_code = v_scope_code
          AND s.validity @> v_root_valid_from
      ) THEN
        PERFORM orgunit.submit_scope_subscription_event(
          gen_random_uuid(),
          p_tenant_id,
          'DEFLT',
          v_scope_code,
          v_package_id,
          v_global_tenant_id,
          'BOOTSTRAP',
          v_root_valid_from,
          format('bootstrap:scope-subscription:deflt:%s', v_scope_code),
          p_initiator_id
        );
      END IF;

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
        v_root_valid_from,
        jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
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
      FROM orgunit.setid_scope_package_versions v
      WHERE v.tenant_id = p_tenant_id
        AND v.scope_code = v_scope_code
        AND v.package_id = v_package_id
        AND v.status = 'active'
        AND v.validity @> v_root_valid_from
    ) THEN
      PERFORM orgunit.submit_scope_package_event(
        gen_random_uuid(),
        p_tenant_id,
        v_scope_code,
        v_package_id,
        'BOOTSTRAP',
        v_root_valid_from,
        jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
        format('bootstrap:scope-package:deflt:%s:%s', v_scope_code, v_root_valid_from),
        p_initiator_id
      );
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.setid_scope_subscriptions s
      WHERE s.tenant_id = p_tenant_id
        AND s.setid = 'DEFLT'
        AND s.scope_code = v_scope_code
        AND s.validity @> v_root_valid_from
    ) THEN
      PERFORM orgunit.submit_scope_subscription_event(
        gen_random_uuid(),
        p_tenant_id,
        'DEFLT',
        v_scope_code,
        v_package_id,
        p_tenant_id,
        'BOOTSTRAP',
        v_root_valid_from,
        format('bootstrap:scope-subscription:deflt:%s', v_scope_code),
        p_initiator_id
      );
    END IF;
  END LOOP;

  PERFORM set_config('app.current_tenant', p_tenant_id::text, true);
  PERFORM set_config('app.current_actor_scope', COALESCE(v_prev_actor, ''), true);
  PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);

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
  v_effective_date date;
  v_global_tenant_id uuid;
  v_prev_actor text;
  v_prev_allow_share text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);
  PERFORM orgunit.lock_setid_governance(p_tenant_id);

  v_global_tenant_id := orgunit.global_tenant_id();
  v_prev_actor := current_setting('app.current_actor_scope', true);
  v_prev_allow_share := current_setting('app.allow_share_read', true);

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

    v_effective_date := current_date;
    IF p_payload ? 'effective_date' THEN
      v_effective_date := NULLIF(btrim(p_payload->>'effective_date'), '')::date;
    END IF;
    IF v_effective_date IS NULL THEN
      v_effective_date := current_date;
    END IF;

    FOR v_scope_code, v_scope_share_mode IN
      SELECT scope_code, share_mode
      FROM orgunit.scope_code_registry()
      WHERE is_stable = true
    LOOP
      IF v_scope_share_mode = 'shared-only' THEN
        PERFORM set_config('app.current_actor_scope', 'saas', true);
        PERFORM set_config('app.current_tenant', v_global_tenant_id::text, true);
        PERFORM set_config('app.allow_share_read', 'on', true);

        SELECT p.package_id INTO v_package_id
        FROM orgunit.global_setid_scope_packages p
        WHERE p.tenant_id = v_global_tenant_id
          AND p.scope_code = v_scope_code
          AND p.package_code = 'DEFLT';

        IF v_package_id IS NULL THEN
          v_package_id := gen_random_uuid();
          PERFORM orgunit.submit_global_scope_package_event(
            gen_random_uuid(),
            v_global_tenant_id,
            v_scope_code,
            v_package_id,
            'BOOTSTRAP',
            v_effective_date,
            jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
            format('bootstrap:global-scope-package:deflt:%s', v_scope_code),
            v_global_tenant_id
          );

          SELECT p.package_id INTO v_package_id
          FROM orgunit.global_setid_scope_packages p
          WHERE p.tenant_id = v_global_tenant_id
            AND p.scope_code = v_scope_code
            AND p.package_code = 'DEFLT';
        END IF;

        IF v_package_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
            DETAIL = format('setid=%s scope_code=%s', v_setid, v_scope_code);
        END IF;

        PERFORM set_config('app.current_tenant', p_tenant_id::text, true);
        PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);

        IF NOT EXISTS (
          SELECT 1
          FROM orgunit.setid_scope_subscriptions s
          WHERE s.tenant_id = p_tenant_id
            AND s.setid = v_setid
            AND s.scope_code = v_scope_code
            AND s.validity @> v_effective_date
        ) THEN
          PERFORM orgunit.submit_scope_subscription_event(
            gen_random_uuid(),
            p_tenant_id,
            v_setid,
            v_scope_code,
            v_package_id,
            v_global_tenant_id,
            'BOOTSTRAP',
            v_effective_date,
            format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
            p_initiator_id
          );
        END IF;

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
          v_effective_date,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
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
          v_effective_date,
          format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
          p_initiator_id
        );
      END IF;
    END LOOP;

    PERFORM set_config('app.current_tenant', p_tenant_id::text, true);
    PERFORM set_config('app.current_actor_scope', COALESCE(v_prev_actor, ''), true);
    PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);
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
  v_owner_setid text;
  v_name text;
  v_status text;
  v_owner_status text;
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
    v_owner_setid := NULLIF(btrim(COALESCE(v_payload->>'owner_setid', '')), '');
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
    IF v_owner_setid IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SCOPE_PACKAGE_INVALID_ARGUMENT',
        DETAIL = 'owner_setid is required';
    END IF;
    v_owner_setid := orgunit.normalize_setid(v_owner_setid);
    IF v_owner_setid = 'SHARE' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_RESERVED_WORD',
        DETAIL = 'SHARE is reserved';
    END IF;
    SELECT status INTO v_owner_status
    FROM orgunit.setids
    WHERE tenant_id = p_tenant_id AND setid = v_owner_setid;
    IF v_owner_status IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_owner_setid);
    END IF;
    IF v_owner_status <> 'active' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_DISABLED',
        DETAIL = format('setid=%s', v_owner_setid);
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
      owner_setid,
      name,
      status
    )
    VALUES (
      p_tenant_id,
      p_scope_code,
      p_package_id,
      v_package_code,
      v_owner_setid,
      v_name,
      v_status
    )
    ON CONFLICT (tenant_id, package_id) DO UPDATE
    SET scope_code = EXCLUDED.scope_code,
        package_code = EXCLUDED.package_code,
        owner_setid = EXCLUDED.owner_setid,
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
    v_owner_setid := v_existing_pkg.owner_setid;
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
    v_owner_setid := v_existing_pkg.owner_setid;
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
            owner_setid = v_owner_setid,
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
          owner_setid,
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
          v_owner_setid,
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
        owner_setid,
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
        v_owner_setid,
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
-- +goose StatementEnd
