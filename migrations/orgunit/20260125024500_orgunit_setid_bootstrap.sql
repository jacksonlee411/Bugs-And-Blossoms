-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
  v_global_tenant uuid;
  v_tenant_id uuid;
BEGIN
  SELECT orgunit.global_tenant_id() INTO v_global_tenant;

  PERFORM set_config('app.current_tenant', v_global_tenant::text, true);
  PERFORM set_config('app.allow_share_read', 'on', true);

  IF EXISTS (
    SELECT 1 FROM orgunit.global_setid_events WHERE setid <> 'SHARE'
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_FORMAT_INVALID',
      DETAIL = 'tenant_uuid=global table=orgunit.global_setid_events';
  END IF;

  IF EXISTS (
    SELECT 1 FROM orgunit.global_setids WHERE setid <> 'SHARE'
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_FORMAT_INVALID',
      DETAIL = 'tenant_uuid=global table=orgunit.global_setids';
  END IF;

  PERFORM set_config('app.current_actor_scope', 'saas', true);
  PERFORM orgunit.submit_global_setid_event(
    gen_random_uuid(),
    v_global_tenant,
    'BOOTSTRAP',
    'SHARE',
    jsonb_build_object('name', 'Shared'),
    'bootstrap:share',
    v_global_tenant
  );

  IF to_regclass('iam.tenants') IS NOT NULL THEN
    FOR v_tenant_id IN
      SELECT id FROM iam.tenants WHERE id <> v_global_tenant
    LOOP
      PERFORM set_config('app.current_tenant', v_tenant_id::text, true);

      IF EXISTS (
        SELECT 1 FROM orgunit.setid_events WHERE setid !~ '^[A-Z0-9]{5}$'
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SETID_FORMAT_INVALID',
          DETAIL = format('tenant_uuid=%s table=orgunit.setid_events', v_tenant_id);
      END IF;

      IF EXISTS (
        SELECT 1 FROM orgunit.setids WHERE setid !~ '^[A-Z0-9]{5}$'
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SETID_FORMAT_INVALID',
          DETAIL = format('tenant_uuid=%s table=orgunit.setids', v_tenant_id);
      END IF;

      IF EXISTS (
        SELECT 1 FROM orgunit.setid_binding_versions WHERE setid !~ '^[A-Z0-9]{5}$'
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SETID_FORMAT_INVALID',
          DETAIL = format('tenant_uuid=%s table=orgunit.setid_binding_versions', v_tenant_id);
      END IF;

      IF EXISTS (
        SELECT 1
        FROM orgunit.setid_binding_events
        WHERE (payload ? 'setid')
          AND (payload->>'setid') !~ '^[A-Z0-9]{5}$'
      ) THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SETID_FORMAT_INVALID',
          DETAIL = format('tenant_uuid=%s table=orgunit.setid_binding_events', v_tenant_id);
      END IF;

      PERFORM orgunit.ensure_setid_bootstrap(v_tenant_id, v_tenant_id);
    END LOOP;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM orgunit.setid_binding_versions v
USING orgunit.setid_binding_events e
WHERE v.last_event_id = e.id
  AND e.request_code = 'bootstrap:binding:deflt';

DELETE FROM orgunit.setid_binding_events
WHERE request_code = 'bootstrap:binding:deflt';

DELETE FROM orgunit.setids s
WHERE s.setid = 'DEFLT'
  AND s.last_event_id IN (
    SELECT id FROM orgunit.setid_events WHERE request_code = 'bootstrap:deflt'
  )
  AND NOT EXISTS (
    SELECT 1 FROM orgunit.setid_events e
    WHERE e.tenant_uuid = s.tenant_uuid
      AND e.setid = s.setid
      AND e.request_code <> 'bootstrap:deflt'
  );

DELETE FROM orgunit.setid_events e
WHERE e.request_code = 'bootstrap:deflt'
  AND NOT EXISTS (
    SELECT 1 FROM orgunit.setids s WHERE s.last_event_id = e.id
  );

DELETE FROM orgunit.global_setids s
WHERE s.setid = 'SHARE'
  AND s.last_event_id IN (
    SELECT id FROM orgunit.global_setid_events WHERE request_code = 'bootstrap:share'
  );

DELETE FROM orgunit.global_setid_events
WHERE request_code = 'bootstrap:share';
-- +goose StatementEnd
