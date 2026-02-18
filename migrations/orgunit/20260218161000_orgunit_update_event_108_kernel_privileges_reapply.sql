-- +goose Up
-- +goose StatementBegin
-- Re-apply kernel execution contract after DEV-PLAN-108 function replacements.

ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.org_events_effective_for_replay(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.org_events_effective_for_replay(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.is_org_event_snapshot_presence_valid(text, jsonb, jsonb, text)
  OWNER TO orgunit_kernel;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY INVOKER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  RESET search_path;

ALTER FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) RESET search_path;

ALTER FUNCTION orgunit.org_events_effective_for_replay(
  uuid,
  int,
  bigint,
  uuid,
  text,
  date,
  jsonb,
  text,
  uuid,
  timestamptz,
  timestamptz,
  timestamptz
) RESET search_path;
-- +goose StatementEnd
