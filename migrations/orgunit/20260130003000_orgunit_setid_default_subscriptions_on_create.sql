-- +goose Up
-- +goose StatementBegin
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op
-- +goose StatementEnd
