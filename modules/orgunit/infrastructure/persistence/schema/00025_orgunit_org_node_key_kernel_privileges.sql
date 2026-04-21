DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;
GRANT USAGE ON SCHEMA iam TO orgunit_kernel;

ALTER TABLE IF EXISTS orgunit.org_node_key_registry OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.org_trees OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.org_events OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.org_unit_versions OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.org_unit_codes OWNER TO orgunit_kernel;
ALTER SEQUENCE IF EXISTS orgunit.org_node_key_seq OWNER TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  orgunit.org_node_key_registry,
  orgunit.org_events,
  orgunit.org_unit_versions,
  orgunit.org_trees,
  orgunit.org_unit_codes
TO orgunit_kernel;

GRANT USAGE, SELECT ON SEQUENCE orgunit.org_node_key_seq TO orgunit_kernel;
GRANT SELECT ON TABLE iam.principals TO orgunit_kernel;

ALTER FUNCTION orgunit.is_valid_org_node_key(text)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.org_ltree_label(text)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.org_path_node_keys(ltree)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.encode_org_node_key(bigint)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.decode_org_node_key(char)
  OWNER TO orgunit_kernel;

ALTER FUNCTION orgunit.allocate_org_node_key(uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.allocate_org_node_key(uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.allocate_org_node_key(uuid)
  SET search_path = pg_catalog, orgunit, public;
