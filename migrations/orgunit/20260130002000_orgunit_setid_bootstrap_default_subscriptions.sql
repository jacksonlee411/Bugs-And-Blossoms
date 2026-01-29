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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- no-op
-- +goose StatementEnd
