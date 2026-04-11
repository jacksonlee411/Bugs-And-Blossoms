-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.submit_setid_binding_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_effective_date date,
  p_setid text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_org_node_key IS NULL OR btrim(p_org_node_key::text) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'org_node_key is required';
  END IF;

  RETURN orgunit.submit_setid_binding_event(
    p_event_uuid,
    p_tenant_uuid,
    orgunit.decode_org_node_key(p_org_node_key)::int,
    p_effective_date,
    p_setid,
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.resolve_setid(
  p_tenant_uuid uuid,
  p_org_node_key char(8),
  p_as_of_date date
)
RETURNS text
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_org_node_key IS NULL OR btrim(p_org_node_key::text) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'org_node_key is required';
  END IF;

  RETURN orgunit.resolve_setid(
    p_tenant_uuid,
    orgunit.decode_org_node_key(p_org_node_key)::int,
    p_as_of_date
  );
END;
$$;

ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.resolve_setid(uuid, char(8), date)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.resolve_setid(uuid, char(8), date)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.resolve_setid(uuid, char(8), date)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.resolve_setid(uuid, char(8), date);
DROP FUNCTION IF EXISTS orgunit.submit_setid_binding_event(uuid, uuid, char(8), date, text, text, uuid);
-- +goose StatementEnd
