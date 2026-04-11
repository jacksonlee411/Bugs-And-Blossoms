DROP VIEW IF EXISTS orgunit.org_events_effective CASCADE;

DROP TABLE IF EXISTS orgunit.org_id_allocators CASCADE;
DROP TABLE IF EXISTS orgunit.org_unit_codes CASCADE;
DROP TABLE IF EXISTS orgunit.org_unit_versions CASCADE;
DROP TABLE IF EXISTS orgunit.org_events CASCADE;
DROP TABLE IF EXISTS orgunit.org_trees CASCADE;
DROP TABLE IF EXISTS orgunit.org_node_key_registry CASCADE;
DROP SEQUENCE IF EXISTS orgunit.org_node_key_seq;

DROP FUNCTION IF EXISTS orgunit.org_path_ids(ltree);
DROP FUNCTION IF EXISTS orgunit.org_ltree_label(int);
DROP FUNCTION IF EXISTS orgunit.allocate_org_id(uuid);

CREATE OR REPLACE FUNCTION orgunit.is_valid_org_node_key(p_value text)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT btrim(COALESCE(p_value, '')) ~ '^[ABCDEFGHJKLMNPQRSTUVWXYZ][ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{7}$';
$$;

CREATE OR REPLACE FUNCTION orgunit.org_ltree_label(p_org_node_key text)
RETURNS text
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT btrim(p_org_node_key);
$$;

CREATE OR REPLACE FUNCTION orgunit.org_path_node_keys(p_path ltree)
RETURNS text[]
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT array_agg(t.part ORDER BY t.ord)
  FROM unnest(string_to_array(p_path::text, '.')) WITH ORDINALITY AS t(part, ord);
$$;

CREATE OR REPLACE FUNCTION orgunit.is_org_event_snapshot_presence_valid(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb,
  p_rescind_outcome text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT CASE
    WHEN p_event_type = 'CREATE'
      THEN p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS')
      THEN p_before_snapshot IS NOT NULL AND p_after_snapshot IS NOT NULL

    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN p_before_snapshot IS NOT NULL
           AND (
             (p_rescind_outcome = 'ABSENT' AND p_after_snapshot IS NULL)
             OR (p_rescind_outcome = 'PRESENT' AND p_after_snapshot IS NOT NULL)
           )

    ELSE true
  END;
$$;

CREATE OR REPLACE FUNCTION orgunit.is_orgunit_snapshot_complete(p_snapshot jsonb)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT p_snapshot IS NOT NULL
    AND jsonb_typeof(p_snapshot) = 'object'
    AND p_snapshot ?& ARRAY[
      'org_node_key',
      'name',
      'status',
      'parent_org_node_key',
      'node_path',
      'validity',
      'full_name_path',
      'is_business_unit'
    ];
$$;

CREATE OR REPLACE FUNCTION orgunit.is_org_event_snapshot_content_valid(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb,
  p_rescind_outcome text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT CASE
    WHEN p_event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      THEN orgunit.is_orgunit_snapshot_complete(p_before_snapshot)
           AND (
             p_rescind_outcome = 'ABSENT'
             OR (
               p_rescind_outcome = 'PRESENT'
               AND orgunit.is_orgunit_snapshot_complete(p_after_snapshot)
             )
           )
    ELSE true
  END;
$$;

CREATE SEQUENCE IF NOT EXISTS orgunit.org_node_key_seq
  AS bigint
  START WITH 1
  MINVALUE 1
  NO MAXVALUE
  CACHE 1;

CREATE TABLE IF NOT EXISTS orgunit.org_node_key_registry (
  org_node_key char(8) PRIMARY KEY,
  seq bigint NOT NULL UNIQUE,
  tenant_uuid uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT org_node_key_registry_key_format_check CHECK (
    orgunit.is_valid_org_node_key(btrim(org_node_key::text))
  )
);

CREATE TABLE IF NOT EXISTS orgunit.org_trees (
  tenant_uuid uuid NOT NULL,
  root_org_node_key char(8) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid),
  CONSTRAINT org_trees_root_org_node_key_format_check CHECK (
    orgunit.is_valid_org_node_key(btrim(root_org_node_key::text))
  )
);

CREATE TABLE IF NOT EXISTS orgunit.org_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  org_node_key char(8) NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_uuid uuid NOT NULL,
  initiator_name text NULL,
  initiator_employee_id text NULL,
  reason text NULL,
  before_snapshot jsonb NULL,
  after_snapshot jsonb NULL,
  rescind_outcome text NULL,
  tx_time timestamptz NOT NULL DEFAULT now(),
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT org_events_org_node_key_format_check CHECK (
    orgunit.is_valid_org_node_key(btrim(org_node_key::text))
  ),
  CONSTRAINT org_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')),
  CONSTRAINT org_events_target_event_uuid_required CHECK (
    event_type NOT IN ('CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')
    OR (
      payload ? 'target_event_uuid'
      AND NULLIF(btrim(payload->>'target_event_uuid'), '') IS NOT NULL
    )
  ),
  CONSTRAINT org_events_rescind_payload_required CHECK (
    event_type NOT IN ('RESCIND_EVENT','RESCIND_ORG')
    OR (
      COALESCE(NULLIF(btrim(payload->>'op'), ''), '') = event_type
      AND COALESCE(NULLIF(btrim(payload->>'reason'), ''), '') <> ''
      AND COALESCE(NULLIF(btrim(payload->>'target_event_uuid'), ''), '') <> ''
      AND COALESCE(NULLIF(btrim(payload->>'target_effective_date'), ''), '') = to_char(effective_date, 'YYYY-MM-DD')
    )
  ),
  CONSTRAINT org_events_snapshot_shape_check CHECK (
    (before_snapshot IS NULL OR jsonb_typeof(before_snapshot) = 'object')
    AND (after_snapshot IS NULL OR jsonb_typeof(after_snapshot) = 'object')
  ),
  CONSTRAINT org_events_rescind_outcome_check CHECK (
    (
      event_type NOT IN ('RESCIND_EVENT','RESCIND_ORG')
      AND rescind_outcome IS NULL
    )
    OR (
      event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      AND rescind_outcome IN ('PRESENT','ABSENT')
    )
  ),
  CONSTRAINT org_events_snapshot_presence_check CHECK (
    orgunit.is_org_event_snapshot_presence_valid(
      event_type,
      before_snapshot,
      after_snapshot,
      rescind_outcome
    )
  ),
  CONSTRAINT org_events_snapshot_content_check CHECK (
    orgunit.is_org_event_snapshot_content_valid(
      event_type,
      before_snapshot,
      after_snapshot,
      rescind_outcome
    )
  ),
  CONSTRAINT org_events_request_id_unique UNIQUE (tenant_uuid, request_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS org_events_event_uuid_unique ON orgunit.org_events (event_uuid);
CREATE INDEX IF NOT EXISTS org_events_tenant_org_effective_idx ON orgunit.org_events (tenant_uuid, org_node_key, effective_date, id);
CREATE INDEX IF NOT EXISTS org_events_tenant_effective_idx ON orgunit.org_events (tenant_uuid, effective_date, id);
CREATE INDEX IF NOT EXISTS org_events_tenant_tx_time_idx ON orgunit.org_events (tenant_uuid, tx_time DESC, id DESC);

CREATE TABLE IF NOT EXISTS orgunit.org_unit_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  org_node_key char(8) NOT NULL,
  parent_org_node_key char(8) NULL,
  node_path ltree NOT NULL,
  validity daterange NOT NULL,
  path_node_keys text[] GENERATED ALWAYS AS (orgunit.org_path_node_keys(node_path)) STORED,
  name varchar(255) NOT NULL,
  full_name_path text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  is_business_unit boolean NOT NULL DEFAULT false,
  manager_uuid uuid NULL,
  last_event_id bigint NOT NULL,
  CONSTRAINT org_unit_versions_org_node_key_format_check CHECK (
    orgunit.is_valid_org_node_key(btrim(org_node_key::text))
  ),
  CONSTRAINT org_unit_versions_parent_org_node_key_format_check CHECK (
    parent_org_node_key IS NULL
    OR orgunit.is_valid_org_node_key(btrim(parent_org_node_key::text))
  ),
  CONSTRAINT org_unit_versions_last_event_id_fkey
    FOREIGN KEY (last_event_id)
    REFERENCES orgunit.org_events(id)
    DEFERRABLE INITIALLY DEFERRED,
  CONSTRAINT org_unit_versions_status_check CHECK (status IN ('active','disabled')),
  CONSTRAINT org_unit_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT org_unit_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT org_unit_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      org_node_key gist_bpchar_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS org_unit_versions_search_gist
  ON orgunit.org_unit_versions
  USING gist (tenant_uuid gist_uuid_ops, node_path, validity);

CREATE INDEX IF NOT EXISTS org_unit_versions_active_day_gist
  ON orgunit.org_unit_versions
  USING gist (tenant_uuid gist_uuid_ops, validity)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS org_unit_versions_lookup_btree
  ON orgunit.org_unit_versions (tenant_uuid, org_node_key, lower(validity));

CREATE INDEX IF NOT EXISTS org_unit_versions_path_node_keys_gin
  ON orgunit.org_unit_versions
  USING gin (path_node_keys);

CREATE TABLE IF NOT EXISTS orgunit.org_unit_codes (
  tenant_uuid uuid NOT NULL,
  org_node_key char(8) NOT NULL,
  org_code text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, org_node_key),
  CONSTRAINT org_unit_codes_org_node_key_format_check CHECK (
    orgunit.is_valid_org_node_key(btrim(org_node_key::text))
  ),
  CONSTRAINT org_unit_codes_org_code_format CHECK (
    length(org_code) BETWEEN 1 AND 64
    AND org_code = upper(org_code)
    AND org_code ~ E'^[\t\x20-\x7E\u3000-\u303F\uFF01-\uFF60\uFFE0-\uFFEE]{1,64}$'
    AND org_code !~ E'^[\t\x20\u3000]+$'
  ),
  CONSTRAINT org_unit_codes_org_code_unique UNIQUE (tenant_uuid, org_code)
);

ALTER TABLE orgunit.org_node_key_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_node_key_registry FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_node_key_registry;
CREATE POLICY tenant_isolation ON orgunit.org_node_key_registry
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

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

ALTER TABLE orgunit.org_unit_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_unit_codes FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_unit_codes;
CREATE POLICY tenant_isolation ON orgunit.org_unit_codes
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
