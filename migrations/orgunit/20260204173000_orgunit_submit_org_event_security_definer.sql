-- +goose Up
-- +goose StatementBegin
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  RESET search_path;
-- +goose StatementEnd
