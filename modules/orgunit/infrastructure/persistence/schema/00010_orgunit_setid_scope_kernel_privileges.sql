DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE ON TABLE
  orgunit.setid_scope_packages,
  orgunit.setid_scope_package_events,
  orgunit.setid_scope_package_versions,
  orgunit.setid_scope_subscriptions,
  orgunit.setid_scope_subscription_events,
  orgunit.global_setid_scope_packages,
  orgunit.global_setid_scope_package_events,
  orgunit.global_setid_scope_package_versions
TO orgunit_kernel;

GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO orgunit_kernel;

ALTER FUNCTION orgunit.assert_scope_package_active_as_of(uuid, text, uuid, uuid, date)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_scope_package_active_as_of(uuid, text, uuid, uuid, date)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.assert_scope_package_active_as_of(uuid, text, uuid, uuid, date)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.resolve_scope_package(uuid, text, text, date)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.resolve_scope_package(uuid, text, text, date)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.resolve_scope_package(uuid, text, text, date)
  SET search_path = pg_catalog, orgunit;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_scope_packages, ' ||
      'orgunit.setid_scope_package_events, ' ||
      'orgunit.setid_scope_package_versions, ' ||
      'orgunit.setid_scope_subscriptions, ' ||
      'orgunit.setid_scope_subscription_events, ' ||
      'orgunit.global_setid_scope_packages, ' ||
      'orgunit.global_setid_scope_package_events, ' ||
      'orgunit.global_setid_scope_package_versions ' ||
      'FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_scope_packages, ' ||
      'orgunit.setid_scope_package_events, ' ||
      'orgunit.setid_scope_package_versions, ' ||
      'orgunit.setid_scope_subscriptions, ' ||
      'orgunit.setid_scope_subscription_events, ' ||
      'orgunit.global_setid_scope_packages, ' ||
      'orgunit.global_setid_scope_package_events, ' ||
      'orgunit.global_setid_scope_package_versions ' ||
      'FROM app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.setid_scope_packages, ' ||
      'orgunit.setid_scope_package_events, ' ||
      'orgunit.setid_scope_package_versions, ' ||
      'orgunit.setid_scope_subscriptions, ' ||
      'orgunit.setid_scope_subscription_events, ' ||
      'orgunit.global_setid_scope_packages, ' ||
      'orgunit.global_setid_scope_package_events, ' ||
      'orgunit.global_setid_scope_package_versions ' ||
      'FROM superadmin_runtime';
  END IF;
END $$;

REVOKE ALL ON TABLE orgunit.global_setid_scope_packages FROM PUBLIC;
REVOKE ALL ON TABLE orgunit.global_setid_scope_package_versions FROM PUBLIC;
REVOKE ALL ON TABLE orgunit.global_setid_scope_package_events FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE ALL ON TABLE orgunit.global_setid_scope_packages FROM app_runtime';
    EXECUTE 'REVOKE ALL ON TABLE orgunit.global_setid_scope_package_versions FROM app_runtime';
    EXECUTE 'REVOKE ALL ON TABLE orgunit.global_setid_scope_package_events FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT SELECT ON TABLE orgunit.global_setid_scope_packages TO superadmin_runtime';
    EXECUTE 'GRANT SELECT ON TABLE orgunit.global_setid_scope_package_versions TO superadmin_runtime';
    EXECUTE 'GRANT SELECT ON TABLE orgunit.global_setid_scope_package_events TO superadmin_runtime';
  END IF;
END $$;

REVOKE EXECUTE ON FUNCTION orgunit.resolve_scope_package(uuid, text, text, date) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION orgunit.assert_scope_package_active_as_of(uuid, text, uuid, uuid, date) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.resolve_scope_package(uuid, text, text, date) TO app_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.assert_scope_package_active_as_of(uuid, text, uuid, uuid, date) TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.resolve_scope_package(uuid, text, text, date) TO superadmin_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION orgunit.assert_scope_package_active_as_of(uuid, text, uuid, uuid, date) TO superadmin_runtime';
  END IF;
END $$;
