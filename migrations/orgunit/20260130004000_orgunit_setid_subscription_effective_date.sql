-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.ensure_setid_bootstrap(
  p_tenant_uuid uuid,
  p_initiator_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_evt_id uuid;
  v_evt_db_id bigint;
  v_root_org_id int;
  v_root_valid_from date;
  v_scope_code text;
  v_scope_share_mode text;
  v_package_id uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  PERFORM orgunit.lock_setid_governance(p_tenant_uuid);

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_uuid = p_tenant_uuid AND setid = 'DEFLT'
  ) THEN
    v_evt_id := gen_random_uuid();
    INSERT INTO orgunit.setid_events (event_uuid, tenant_uuid, event_type, setid, payload, request_code, initiator_uuid)
    VALUES (v_evt_id, p_tenant_uuid, 'BOOTSTRAP', 'DEFLT', jsonb_build_object('name', 'Default'), 'bootstrap:deflt', p_initiator_uuid)
    ON CONFLICT (tenant_uuid, request_code) DO NOTHING;

    SELECT id INTO v_evt_db_id
    FROM orgunit.setid_events
    WHERE tenant_uuid = p_tenant_uuid AND request_code = 'bootstrap:deflt'
    ORDER BY id DESC
    LIMIT 1;

    INSERT INTO orgunit.setids (tenant_uuid, setid, name, status, last_event_id)
    VALUES (p_tenant_uuid, 'DEFLT', 'Default', 'active', v_evt_db_id)
    ON CONFLICT (tenant_uuid, setid) DO NOTHING;
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid AND t.hierarchy_type = 'OrgUnit'
  FOR UPDATE;

  IF v_root_org_id IS NULL THEN
    RETURN;
  END IF;

  SELECT lower(v.validity)::date INTO v_root_valid_from
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
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
      CONTINUE;
    END IF;

    SELECT p.package_id INTO v_package_id
    FROM orgunit.setid_scope_packages p
    WHERE p.tenant_uuid = p_tenant_uuid
      AND p.scope_code = v_scope_code
      AND p.package_code = 'DEFLT';

    IF v_package_id IS NULL THEN
      v_package_id := gen_random_uuid();
      PERFORM orgunit.submit_scope_package_event(
        gen_random_uuid(),
        p_tenant_uuid,
        v_scope_code,
        v_package_id,
        'BOOTSTRAP',
        v_root_valid_from,
        jsonb_build_object('package_code', 'DEFLT', 'name', 'Default'),
        format('bootstrap:scope-package:deflt:%s', v_scope_code),
        p_initiator_uuid
      );

      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_uuid = p_tenant_uuid
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
      WHERE s.tenant_uuid = p_tenant_uuid
        AND s.setid = 'DEFLT'
        AND s.scope_code = v_scope_code
        AND s.validity @> v_root_valid_from
    ) THEN
      PERFORM orgunit.submit_scope_subscription_event(
        gen_random_uuid(),
        p_tenant_uuid,
        'DEFLT',
        v_scope_code,
        v_package_id,
        p_tenant_uuid,
        'BOOTSTRAP',
        v_root_valid_from,
        format('bootstrap:scope-subscription:deflt:%s', v_scope_code),
        p_initiator_uuid
      );
    END IF;
  END LOOP;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.setid_binding_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = v_root_org_id
      AND validity @> v_root_valid_from
  ) THEN
    PERFORM orgunit.submit_setid_binding_event(
      gen_random_uuid(),
      p_tenant_uuid,
      v_root_org_id,
      v_root_valid_from,
      'DEFLT',
      'bootstrap:binding:deflt',
      p_initiator_uuid
    );
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_setid_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_event_type text,
  p_setid text,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
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
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  PERFORM orgunit.lock_setid_governance(p_tenant_uuid);

  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_code is required';
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

  INSERT INTO orgunit.setid_events (event_uuid, tenant_uuid, event_type, setid, payload, request_code, initiator_uuid)
  VALUES (p_event_uuid, p_tenant_uuid, p_event_type, v_setid, COALESCE(p_payload, '{}'::jsonb), p_request_code, p_initiator_uuid)
  ON CONFLICT (tenant_uuid, request_code) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_events
  WHERE tenant_uuid = p_tenant_uuid AND request_code = p_request_code
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
      SELECT 1 FROM orgunit.setids WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_ALREADY_EXISTS',
        DETAIL = format('setid=%s', v_setid);
    END IF;

    INSERT INTO orgunit.setids (tenant_uuid, setid, name, status, last_event_id)
    VALUES (p_tenant_uuid, v_setid, v_name, 'active', v_evt_db_id)
    ON CONFLICT (tenant_uuid, setid) DO UPDATE
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
        CONTINUE;
      END IF;

      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_uuid = p_tenant_uuid
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';

      IF v_package_id IS NULL THEN
        v_package_id := gen_random_uuid();
        PERFORM orgunit.submit_scope_package_event(
          gen_random_uuid(),
          p_tenant_uuid,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          v_effective_date,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default'),
          format('bootstrap:scope-package:deflt:%s', v_scope_code),
          p_initiator_uuid
        );

        SELECT p.package_id INTO v_package_id
        FROM orgunit.setid_scope_packages p
        WHERE p.tenant_uuid = p_tenant_uuid
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
        WHERE s.tenant_uuid = p_tenant_uuid
          AND s.setid = v_setid
          AND s.scope_code = v_scope_code
          AND s.validity @> v_effective_date
      ) THEN
        PERFORM orgunit.submit_scope_subscription_event(
          gen_random_uuid(),
          p_tenant_uuid,
          v_setid,
          v_scope_code,
          v_package_id,
          p_tenant_uuid,
          'BOOTSTRAP',
          v_effective_date,
          format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
          p_initiator_uuid
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
    WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid;
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
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid
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
    WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid;
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op
-- +goose StatementEnd
