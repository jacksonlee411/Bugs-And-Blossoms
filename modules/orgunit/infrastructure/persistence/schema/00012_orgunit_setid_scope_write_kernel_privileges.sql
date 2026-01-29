ALTER FUNCTION orgunit.submit_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.submit_global_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_global_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_global_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.submit_scope_subscription_event(uuid, uuid, text, text, uuid, uuid, text, date, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_scope_subscription_event(uuid, uuid, text, text, uuid, uuid, text, date, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_scope_subscription_event(uuid, uuid, text, text, uuid, uuid, text, date, text, uuid)
  SET search_path = pg_catalog, orgunit;

REVOKE EXECUTE ON FUNCTION orgunit.submit_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION orgunit.submit_global_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION orgunit.submit_scope_subscription_event(uuid, uuid, text, text, uuid, uuid, text, date, text, uuid) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.submit_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid) TO app_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.submit_global_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid) TO app_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.submit_scope_subscription_event(uuid, uuid, text, text, uuid, uuid, text, date, text, uuid) TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.submit_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid) TO superadmin_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.submit_global_scope_package_event(uuid, uuid, text, uuid, text, date, jsonb, text, uuid) TO superadmin_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.submit_scope_subscription_event(uuid, uuid, text, text, uuid, uuid, text, date, text, uuid) TO superadmin_runtime';
  END IF;
END $$;
