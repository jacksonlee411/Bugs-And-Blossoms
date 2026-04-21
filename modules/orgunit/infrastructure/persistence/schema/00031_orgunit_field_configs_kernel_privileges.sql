-- OrgUnit tenant field configs (metadata) kernel privileges (Phase 1)
-- SSOT: docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;

ALTER TABLE IF EXISTS orgunit.tenant_field_config_events OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.tenant_field_configs OWNER TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  orgunit.tenant_field_config_events,
  orgunit.tenant_field_configs
TO orgunit_kernel;

GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO orgunit_kernel;

ALTER FUNCTION orgunit.guard_tenant_field_configs_write()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.guard_tenant_field_configs_write()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.rename_jsonb_object_key_strict(jsonb, text, text)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rename_jsonb_object_key_strict(jsonb, text, text)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.rename_jsonb_object_key_strict(jsonb, text, text)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM app';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM app_runtime';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM app_nobypassrls';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM superadmin_runtime';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO superadmin_runtime';
  END IF;
END $$;
