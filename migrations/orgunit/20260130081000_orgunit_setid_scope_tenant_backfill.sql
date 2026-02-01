-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  v_scope_code text;
  v_tenant_id uuid;
  v_setid text;
  v_package_id uuid;
BEGIN
  FOR v_tenant_id IN
    SELECT DISTINCT tenant_uuid
    FROM orgunit.setids
  LOOP
    PERFORM set_config('app.current_tenant', v_tenant_id::text, true);

    FOR v_scope_code IN
      SELECT scope_code
      FROM orgunit.scope_code_registry()
      WHERE is_stable = true AND share_mode <> 'shared-only'
    LOOP
      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_uuid = v_tenant_id
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';

      IF v_package_id IS NULL THEN
        v_package_id := gen_random_uuid();
        PERFORM orgunit.submit_scope_package_event(
          gen_random_uuid(),
          v_tenant_id,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          current_date,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default'),
          format('bootstrap:scope-package:deflt:%s', v_scope_code),
          v_tenant_id
        );

        SELECT p.package_id INTO v_package_id
        FROM orgunit.setid_scope_packages p
        WHERE p.tenant_uuid = v_tenant_id
          AND p.scope_code = v_scope_code
          AND p.package_code = 'DEFLT';
      END IF;

      IF v_package_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
          DETAIL = format('scope_code=%s', v_scope_code);
      END IF;

      FOR v_setid IN
        SELECT setid
        FROM orgunit.setids
        WHERE tenant_uuid = v_tenant_id
      LOOP
        IF NOT EXISTS (
          SELECT 1
          FROM orgunit.setid_scope_subscriptions s
          WHERE s.tenant_uuid = v_tenant_id
            AND s.setid = v_setid
            AND s.scope_code = v_scope_code
            AND s.validity @> current_date
        ) THEN
          PERFORM orgunit.submit_scope_subscription_event(
            gen_random_uuid(),
            v_tenant_id,
            v_setid,
            v_scope_code,
            v_package_id,
            v_tenant_id,
            'BOOTSTRAP',
            current_date,
            format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
            v_tenant_id
          );
        END IF;
      END LOOP;
    END LOOP;
  END LOOP;
END;
$$;
-- +goose StatementEnd
