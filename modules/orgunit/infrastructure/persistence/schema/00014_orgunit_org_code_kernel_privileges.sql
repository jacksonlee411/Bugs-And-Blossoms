DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;

ALTER TABLE IF EXISTS orgunit.org_unit_codes OWNER TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  orgunit.org_events,
  orgunit.org_event_corrections_current,
  orgunit.org_event_corrections_history,
  orgunit.org_events_effective,
  orgunit.org_unit_versions,
  orgunit.org_trees,
  orgunit.org_unit_codes
TO orgunit_kernel;

ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) FROM superadmin_runtime';
  END IF;
END $$;

GRANT EXECUTE ON FUNCTION orgunit.replay_org_unit_versions(uuid) TO orgunit_kernel;
ALTER FUNCTION orgunit.replay_org_unit_versions(uuid) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.replay_org_unit_versions(uuid) SET search_path = pg_catalog, orgunit, public;

REVOKE ALL ON TABLE orgunit.org_unit_codes FROM PUBLIC;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE orgunit.org_unit_codes FROM app';
    EXECUTE 'GRANT SELECT ON TABLE orgunit.org_unit_codes TO app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE orgunit.org_unit_codes FROM app_runtime';
    EXECUTE 'GRANT SELECT ON TABLE orgunit.org_unit_codes TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE orgunit.org_unit_codes FROM app_nobypassrls';
    EXECUTE 'GRANT SELECT ON TABLE orgunit.org_unit_codes TO app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE orgunit.org_unit_codes FROM superadmin_runtime';
    EXECUTE 'GRANT SELECT ON TABLE orgunit.org_unit_codes TO superadmin_runtime';
  END IF;
END $$;

CREATE OR REPLACE FUNCTION orgunit.guard_org_unit_codes_write()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_user <> 'orgunit_kernel' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORGUNIT_CODES_WRITE_FORBIDDEN',
      DETAIL = format('role=%s', current_user);
  END IF;

  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

ALTER FUNCTION orgunit.guard_org_unit_codes_write() OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.guard_org_unit_codes_write() SET search_path = pg_catalog, orgunit, public;

DROP TRIGGER IF EXISTS guard_org_unit_codes_write ON orgunit.org_unit_codes;
CREATE TRIGGER guard_org_unit_codes_write
BEFORE INSERT OR UPDATE OR DELETE ON orgunit.org_unit_codes
FOR EACH ROW EXECUTE FUNCTION orgunit.guard_org_unit_codes_write();
