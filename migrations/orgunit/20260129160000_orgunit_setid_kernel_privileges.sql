-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE ON TABLE
  orgunit.setid_events,
  orgunit.setids,
  orgunit.setid_binding_events,
  orgunit.setid_binding_versions,
  orgunit.global_setid_events,
  orgunit.global_setids,
  orgunit.org_unit_versions,
  orgunit.org_trees
TO orgunit_kernel;

GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO orgunit_kernel;

ALTER FUNCTION orgunit.submit_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.submit_global_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_global_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_global_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, int, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, int, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, int, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.ensure_setid_bootstrap(uuid, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.ensure_setid_bootstrap(uuid, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.ensure_setid_bootstrap(uuid, uuid)
  SET search_path = pg_catalog, orgunit;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_events, ' ||
      'orgunit.setids, ' ||
      'orgunit.setid_binding_events, ' ||
      'orgunit.setid_binding_versions, ' ||
      'orgunit.global_setid_events, ' ||
      'orgunit.global_setids ' ||
      'FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_events, ' ||
      'orgunit.setids, ' ||
      'orgunit.setid_binding_events, ' ||
      'orgunit.setid_binding_versions, ' ||
      'orgunit.global_setid_events, ' ||
      'orgunit.global_setids ' ||
      'FROM app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_events, ' ||
      'orgunit.setids, ' ||
      'orgunit.setid_binding_events, ' ||
      'orgunit.setid_binding_versions, ' ||
      'orgunit.global_setid_events, ' ||
      'orgunit.global_setids ' ||
      'FROM superadmin_runtime';
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER FUNCTION orgunit.submit_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  RESET search_path;
ALTER FUNCTION orgunit.submit_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  OWNER TO app;

ALTER FUNCTION orgunit.submit_global_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_global_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  RESET search_path;
ALTER FUNCTION orgunit.submit_global_setid_event(uuid, uuid, text, text, jsonb, text, uuid)
  OWNER TO app;

ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, int, date, text, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, int, date, text, text, uuid)
  RESET search_path;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, int, date, text, text, uuid)
  OWNER TO app;

ALTER FUNCTION orgunit.ensure_setid_bootstrap(uuid, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.ensure_setid_bootstrap(uuid, uuid)
  RESET search_path;
ALTER FUNCTION orgunit.ensure_setid_bootstrap(uuid, uuid)
  OWNER TO app;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_events, ' ||
      'orgunit.setids, ' ||
      'orgunit.setid_binding_events, ' ||
      'orgunit.setid_binding_versions, ' ||
      'orgunit.global_setid_events, ' ||
      'orgunit.global_setids ' ||
      'TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'GRANT INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_events, ' ||
      'orgunit.setids, ' ||
      'orgunit.setid_binding_events, ' ||
      'orgunit.setid_binding_versions, ' ||
      'orgunit.global_setid_events, ' ||
      'orgunit.global_setids ' ||
      'TO app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_events, ' ||
      'orgunit.setids, ' ||
      'orgunit.setid_binding_events, ' ||
      'orgunit.setid_binding_versions, ' ||
      'orgunit.global_setid_events, ' ||
      'orgunit.global_setids ' ||
      'TO superadmin_runtime';
  END IF;
END $$;

REVOKE ALL PRIVILEGES ON TABLE
  orgunit.setid_events,
  orgunit.setids,
  orgunit.setid_binding_events,
  orgunit.setid_binding_versions,
  orgunit.global_setid_events,
  orgunit.global_setids,
  orgunit.org_unit_versions,
  orgunit.org_trees
FROM orgunit_kernel;
REVOKE USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit FROM orgunit_kernel;
REVOKE USAGE ON SCHEMA orgunit FROM orgunit_kernel;
DROP ROLE IF EXISTS orgunit_kernel;
-- +goose StatementEnd
