-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE SCHEMA IF NOT EXISTS orgunit;

CREATE OR REPLACE FUNCTION orgunit.global_tenant_id()
RETURNS uuid
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT '00000000-0000-0000-0000-000000000000'::uuid;
$$;

CREATE OR REPLACE FUNCTION orgunit.org_ltree_label(p_id int)
RETURNS text
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT p_id::text;
$$;

CREATE OR REPLACE FUNCTION orgunit.org_path_ids(p_path ltree)
RETURNS int[]
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT array_agg((t.part)::int ORDER BY t.ord)
  FROM unnest(string_to_array(p_path::text, '.')) WITH ORDINALITY AS t(part, ord);
$$;

CREATE SEQUENCE IF NOT EXISTS orgunit.org_id_seq
  START WITH 10000000
  INCREMENT BY 1
  MINVALUE 10000000
  MAXVALUE 99999999
  NO CYCLE;

CREATE TABLE IF NOT EXISTS orgunit.org_trees (
  tenant_uuid uuid NOT NULL,
  hierarchy_type text NOT NULL DEFAULT 'OrgUnit',
  root_org_id int NOT NULL CHECK (root_org_id BETWEEN 10000000 AND 99999999),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, hierarchy_type),
  CONSTRAINT org_trees_hierarchy_type_check CHECK (hierarchy_type IN ('OrgUnit'))
);

CREATE TABLE IF NOT EXISTS orgunit.org_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  hierarchy_type text NOT NULL DEFAULT 'OrgUnit',
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_code text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT org_events_hierarchy_type_check CHECK (hierarchy_type IN ('OrgUnit')),
  CONSTRAINT org_events_event_type_check CHECK (event_type IN ('CREATE','MOVE','RENAME','DISABLE','SET_BUSINESS_UNIT')),
  CONSTRAINT org_events_one_per_day_unique UNIQUE (tenant_uuid, hierarchy_type, org_id, effective_date)
);

CREATE UNIQUE INDEX IF NOT EXISTS org_events_event_uuid_unique ON orgunit.org_events (event_uuid);
CREATE INDEX IF NOT EXISTS org_events_tenant_org_effective_idx ON orgunit.org_events (tenant_uuid, org_id, effective_date, id);
CREATE INDEX IF NOT EXISTS org_events_tenant_type_effective_idx ON orgunit.org_events (tenant_uuid, hierarchy_type, effective_date, id);

CREATE TABLE IF NOT EXISTS orgunit.org_unit_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  hierarchy_type text NOT NULL DEFAULT 'OrgUnit',
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  parent_id int NULL CHECK (parent_id BETWEEN 10000000 AND 99999999),
  node_path ltree NOT NULL,
  validity daterange NOT NULL,
  path_ids int[] GENERATED ALWAYS AS (orgunit.org_path_ids(node_path)) STORED,
  name varchar(255) NOT NULL,
  full_name_path text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  is_business_unit boolean NOT NULL DEFAULT false,
  manager_uuid uuid NULL,
  last_event_id bigint NOT NULL REFERENCES orgunit.org_events(id),
  CONSTRAINT org_unit_versions_hierarchy_type_check CHECK (hierarchy_type IN ('OrgUnit')),
  CONSTRAINT org_unit_versions_status_check CHECK (status IN ('active','disabled')),
  CONSTRAINT org_unit_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT org_unit_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT org_unit_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      hierarchy_type gist_text_ops WITH =,
      org_id gist_int4_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS org_unit_versions_search_gist
  ON orgunit.org_unit_versions
  USING gist (tenant_uuid gist_uuid_ops, hierarchy_type gist_text_ops, node_path, validity);

CREATE INDEX IF NOT EXISTS org_unit_versions_active_day_gist
  ON orgunit.org_unit_versions
  USING gist (tenant_uuid gist_uuid_ops, hierarchy_type gist_text_ops, validity)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS org_unit_versions_lookup_btree
  ON orgunit.org_unit_versions (tenant_uuid, hierarchy_type, org_id, lower(validity));

CREATE INDEX IF NOT EXISTS org_unit_versions_path_ids_gin
  ON orgunit.org_unit_versions
  USING gin (path_ids);

ALTER TABLE orgunit.org_trees ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_trees FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_trees;
CREATE POLICY tenant_isolation ON orgunit.org_trees
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.org_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_events;
CREATE POLICY tenant_isolation ON orgunit.org_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.org_unit_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_unit_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_unit_versions;
CREATE POLICY tenant_isolation ON orgunit.org_unit_versions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_unit_versions;
ALTER TABLE IF EXISTS orgunit.org_unit_versions DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.org_unit_versions;

DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_events;
ALTER TABLE IF EXISTS orgunit.org_events DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.org_events;

DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_trees;
ALTER TABLE IF EXISTS orgunit.org_trees DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS orgunit.org_trees;

DROP FUNCTION IF EXISTS orgunit.org_path_ids(ltree);
DROP SEQUENCE IF EXISTS orgunit.org_id_seq;
DROP FUNCTION IF EXISTS orgunit.org_ltree_label(int);
DROP FUNCTION IF EXISTS orgunit.global_tenant_id();
-- +goose StatementEnd
