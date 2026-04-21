-- DEV-PLAN-320 P3: org_node_key runtime function privileges
GRANT SELECT ON TABLE orgunit.org_events_effective TO orgunit_kernel;

ALTER FUNCTION orgunit.submit_org_event_correction(uuid, char(8), date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, char(8), date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, char(8), date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_status_correction(uuid, char(8), date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, char(8), date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, char(8), date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, char(8), date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, char(8), date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, char(8), date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_rescind(uuid, char(8), text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, char(8), text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, char(8), text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
