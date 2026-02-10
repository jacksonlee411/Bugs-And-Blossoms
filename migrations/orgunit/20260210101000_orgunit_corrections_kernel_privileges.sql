-- +goose Up
-- +goose StatementBegin
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid)
  RESET search_path;

ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_org_status_correction(uuid, int, date, text, text, uuid)
  RESET search_path;

ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_org_event_rescind(uuid, int, date, text, text, uuid)
  RESET search_path;

ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_org_rescind(uuid, int, text, text, uuid)
  RESET search_path;
-- +goose StatementEnd
