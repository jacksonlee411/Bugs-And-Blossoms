-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS orgunit;

CREATE TABLE IF NOT EXISTS orgunit.events (
  tenant_id uuid NOT NULL,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  event_type text NOT NULL,
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, event_id)
);

CREATE TABLE IF NOT EXISTS orgunit.nodes (
  tenant_id uuid NOT NULL,
  node_id uuid NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, node_id)
);

ALTER TABLE orgunit.events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orgunit.events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.nodes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON orgunit.nodes
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE OR REPLACE FUNCTION orgunit.submit_orgunit_event(
  p_tenant_id uuid,
  p_event_type text,
  p_payload jsonb
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_tenant uuid;
  v_node_id uuid;
  v_name text;
BEGIN
  v_ctx_tenant := current_setting('app.current_tenant')::uuid;
  IF v_ctx_tenant <> p_tenant_id THEN
    RAISE EXCEPTION 'RLS_TENANT_MISMATCH'
      USING
        ERRCODE = 'P0001',
        DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_id, v_ctx_tenant);
  END IF;

  IF p_event_type = 'node_created' THEN
    v_name := coalesce(p_payload->>'name', '');
    IF v_name = '' THEN
      RAISE EXCEPTION 'ORGUNIT_NAME_EMPTY' USING ERRCODE = 'P0001';
    END IF;
    v_node_id := gen_random_uuid();

    INSERT INTO orgunit.nodes (tenant_id, node_id, name)
    VALUES (p_tenant_id, v_node_id, v_name);

    p_payload := jsonb_set(p_payload, '{node_id}', to_jsonb(v_node_id::text), true);
  ELSE
    RAISE EXCEPTION 'ORGUNIT_EVENT_UNKNOWN' USING ERRCODE = 'P0001';
  END IF;

  INSERT INTO orgunit.events (tenant_id, event_type, payload)
  VALUES (p_tenant_id, p_event_type, p_payload);

  RETURN v_node_id;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.submit_orgunit_event(uuid, text, jsonb);
DROP POLICY IF EXISTS tenant_isolation ON orgunit.nodes;
ALTER TABLE IF EXISTS orgunit.nodes DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.nodes;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.events;
ALTER TABLE IF EXISTS orgunit.events DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.events;
DROP SCHEMA IF EXISTS orgunit;
DROP EXTENSION IF EXISTS pgcrypto;
-- +goose StatementEnd

