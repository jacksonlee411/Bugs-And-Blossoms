-- DEV-PLAN-320 P3: SetID org_node_key privileges overlay
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

ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid)
  SET search_path = pg_catalog, orgunit;

ALTER FUNCTION orgunit.resolve_setid(uuid, char(8), date)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.resolve_setid(uuid, char(8), date)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.resolve_setid(uuid, char(8), date)
  SET search_path = pg_catalog, orgunit;
