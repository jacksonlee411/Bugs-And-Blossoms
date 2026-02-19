-- OrgUnit tenant field policies kernel privileges
-- SSOT: docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;

ALTER TABLE IF EXISTS orgunit.tenant_field_policies OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.tenant_field_policy_events OWNER TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  orgunit.tenant_field_policies,
  orgunit.tenant_field_policy_events
TO orgunit_kernel;

GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO orgunit_kernel;

ALTER FUNCTION orgunit.guard_tenant_field_policies_write()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.guard_tenant_field_policies_write()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.assert_tenant_field_policies_non_overlapping()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_tenant_field_policies_non_overlapping()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.upsert_tenant_field_policy(uuid, text, text, text, boolean, text, text, date, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.upsert_tenant_field_policy(uuid, text, text, text, boolean, text, text, date, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.upsert_tenant_field_policy(uuid, text, text, text, boolean, text, text, date, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.disable_tenant_field_policy(uuid, text, text, text, date, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.disable_tenant_field_policy(uuid, text, text, text, date, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.disable_tenant_field_policy(uuid, text, text, text, date, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'FROM app';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'TO app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'FROM app_runtime';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'FROM app_nobypassrls';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'TO app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'FROM superadmin_runtime';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_policies, ' ||
      'orgunit.tenant_field_policy_events ' ||
      'TO superadmin_runtime';
  END IF;
END $$;
