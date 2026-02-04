package server

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestOrgunitProjection_IncrementalMatchesReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	adminConn, _, ok := connectTestPostgres(ctx, t)
	if !ok {
		return
	}
	t.Cleanup(func() { _ = adminConn.Close(context.Background()) })

	if err := ensureOrgunitProjectionSchemaForTest(ctx, adminConn); err != nil {
		t.Fatal(err)
	}

	tenant := "00000000-0000-0000-0000-0000000000a1"
	initiator := "00000000-0000-0000-0000-0000000000b1"

	tx, err := adminConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenant); err != nil {
		t.Fatal(err)
	}

	cleanup := []string{
		`DELETE FROM orgunit.org_unit_codes WHERE tenant_uuid = $1::uuid;`,
		`DELETE FROM orgunit.org_unit_versions WHERE tenant_uuid = $1::uuid;`,
		`DELETE FROM orgunit.org_trees WHERE tenant_uuid = $1::uuid;`,
		`DELETE FROM orgunit.org_events WHERE tenant_uuid = $1::uuid;`,
		`DELETE FROM orgunit.org_id_allocators WHERE tenant_uuid = $1::uuid;`,
	}
	for _, stmt := range cleanup {
		if _, err := tx.Exec(ctx, stmt, tenant); err != nil {
			t.Fatal(err)
		}
	}

	submit := func(eventUUID string, orgID int, eventType, effectiveDate, payload, requestCode string) {
		_, err := tx.Exec(ctx, `SELECT orgunit.submit_org_event($1::uuid, $2::uuid, $3::int, $4::text, $5::date, $6::jsonb, $7::text, $8::uuid);`,
			eventUUID, tenant, orgID, eventType, effectiveDate, payload, requestCode, initiator)
		if err != nil {
			t.Fatalf("submit %s: %v", eventType, err)
		}
	}

	submit("00000000-0000-0000-0000-000000000101", 10000001, "CREATE", "2026-01-01", `{"name":"Root","org_code":"R1","is_business_unit":true}`, "r1")
	submit("00000000-0000-0000-0000-000000000102", 10000002, "CREATE", "2026-01-01", `{"parent_id":10000001,"name":"ChildA","org_code":"C1"}`, "r2")
	submit("00000000-0000-0000-0000-000000000103", 10000003, "CREATE", "2026-01-01", `{"parent_id":10000002,"name":"Grand","org_code":"G1"}`, "r3")
	submit("00000000-0000-0000-0000-000000000104", 10000004, "CREATE", "2026-01-01", `{"parent_id":10000001,"name":"Branch","org_code":"B1"}`, "r4")
	submit("00000000-0000-0000-0000-000000000105", 10000002, "RENAME", "2026-02-01", `{"new_name":"ChildA2"}`, "r5")
	submit("00000000-0000-0000-0000-000000000106", 10000004, "SET_BUSINESS_UNIT", "2026-02-10", `{"is_business_unit":true}`, "r6")
	submit("00000000-0000-0000-0000-000000000107", 10000002, "MOVE", "2026-03-01", `{"new_parent_id":10000004}`, "r7")
	submit("00000000-0000-0000-0000-000000000108", 10000003, "DISABLE", "2026-04-01", `{}`, "r8")

	if _, err := tx.Exec(ctx, `
CREATE TEMP TABLE inc_versions AS
SELECT tenant_uuid, org_id, parent_id, node_path, validity, name, full_name_path, status, is_business_unit, manager_uuid, last_event_id
FROM orgunit.org_unit_versions
WHERE tenant_uuid = $1::uuid;`, tenant); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `
CREATE TEMP TABLE inc_trees AS
SELECT tenant_uuid, root_org_id
FROM orgunit.org_trees
WHERE tenant_uuid = $1::uuid;`, tenant); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `
CREATE TEMP TABLE inc_codes AS
SELECT tenant_uuid, org_id, org_code
FROM orgunit.org_unit_codes
WHERE tenant_uuid = $1::uuid;`, tenant); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `SELECT orgunit.replay_org_unit_versions($1::uuid);`, tenant); err != nil {
		t.Fatal(err)
	}

	assertNoDiff := func(name, query string, args ...any) {
		var n int
		if err := tx.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("query %s: %v", name, err)
		}
		if n != 0 {
			t.Fatalf("%s mismatch: %d", name, n)
		}
	}

	assertNoDiff("versions inc->replay", `
SELECT count(*) FROM (
  SELECT * FROM inc_versions
  EXCEPT
  SELECT tenant_uuid, org_id, parent_id, node_path, validity, name, full_name_path, status, is_business_unit, manager_uuid, last_event_id
  FROM orgunit.org_unit_versions
  WHERE tenant_uuid = $1::uuid
) diff;`, tenant)

	assertNoDiff("versions replay->inc", `
SELECT count(*) FROM (
  SELECT tenant_uuid, org_id, parent_id, node_path, validity, name, full_name_path, status, is_business_unit, manager_uuid, last_event_id
  FROM orgunit.org_unit_versions
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT * FROM inc_versions
) diff;`, tenant)

	assertNoDiff("trees inc->replay", `
SELECT count(*) FROM (
  SELECT * FROM inc_trees
  EXCEPT
  SELECT tenant_uuid, root_org_id
  FROM orgunit.org_trees
  WHERE tenant_uuid = $1::uuid
) diff;`, tenant)

	assertNoDiff("trees replay->inc", `
SELECT count(*) FROM (
  SELECT tenant_uuid, root_org_id
  FROM orgunit.org_trees
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT * FROM inc_trees
) diff;`, tenant)

	assertNoDiff("codes inc->replay", `
SELECT count(*) FROM (
  SELECT * FROM inc_codes
  EXCEPT
  SELECT tenant_uuid, org_id, org_code
  FROM orgunit.org_unit_codes
  WHERE tenant_uuid = $1::uuid
) diff;`, tenant)

	assertNoDiff("codes replay->inc", `
SELECT count(*) FROM (
  SELECT tenant_uuid, org_id, org_code
  FROM orgunit.org_unit_codes
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT * FROM inc_codes
) diff;`, tenant)
}

func ensureOrgunitProjectionSchemaForTest(ctx context.Context, conn *pgx.Conn) error {
	ddl := []string{
		`CREATE EXTENSION IF NOT EXISTS ltree;`,
		`CREATE EXTENSION IF NOT EXISTS btree_gist;`,
		`CREATE SCHEMA IF NOT EXISTS orgunit;`,
		`CREATE OR REPLACE FUNCTION orgunit.org_ltree_label(p_id int)
RETURNS text
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT p_id::text;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.org_path_ids(p_path ltree)
RETURNS int[]
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT array_agg((t.part)::int ORDER BY t.ord)
  FROM unnest(string_to_array(p_path::text, '.')) WITH ORDINALITY AS t(part, ord);
$$;`,
		`CREATE TABLE IF NOT EXISTS orgunit.org_trees (
  tenant_uuid uuid NOT NULL,
  root_org_id int NOT NULL CHECK (root_org_id BETWEEN 10000000 AND 99999999),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid)
);`,
		`CREATE TABLE IF NOT EXISTS orgunit.org_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_code text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT org_events_event_type_check CHECK (event_type IN ('CREATE','MOVE','RENAME','DISABLE','SET_BUSINESS_UNIT')),
  CONSTRAINT org_events_one_per_day_unique UNIQUE (tenant_uuid, org_id, effective_date)
);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS org_events_event_uuid_unique ON orgunit.org_events (event_uuid);`,
		`CREATE INDEX IF NOT EXISTS org_events_tenant_org_effective_idx ON orgunit.org_events (tenant_uuid, org_id, effective_date, id);`,
		`CREATE INDEX IF NOT EXISTS org_events_tenant_effective_idx ON orgunit.org_events (tenant_uuid, effective_date, id);`,
		`CREATE TABLE IF NOT EXISTS orgunit.org_unit_versions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
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
  CONSTRAINT org_unit_versions_status_check CHECK (status IN ('active','disabled')),
  CONSTRAINT org_unit_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT org_unit_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT org_unit_versions_no_overlap
    EXCLUDE USING gist (
      tenant_uuid gist_uuid_ops WITH =,
      org_id gist_int4_ops WITH =,
      validity WITH &&
    )
);`,
		`CREATE TABLE IF NOT EXISTS orgunit.org_unit_codes (
  tenant_uuid uuid NOT NULL,
  org_id int NOT NULL CHECK (org_id BETWEEN 10000000 AND 99999999),
  org_code text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, org_id),
  CONSTRAINT org_unit_codes_org_code_format CHECK (
    length(org_code) BETWEEN 1 AND 64
    AND org_code = upper(org_code)
    AND org_code ~ E'^[\t\x20-\x7E\u3000-\u303F\uFF01-\uFF60\uFFE0-\uFFEE]{1,64}$'
    AND org_code !~ E'^[\t\x20\u3000]+$'
  ),
  CONSTRAINT org_unit_codes_org_code_unique UNIQUE (tenant_uuid, org_code)
);`,
		`CREATE TABLE IF NOT EXISTS orgunit.org_id_allocators (
  tenant_uuid uuid NOT NULL,
  next_org_id int NOT NULL CHECK (next_org_id BETWEEN 10000000 AND 100000000),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid)
);`,
		`CREATE OR REPLACE FUNCTION orgunit.assert_current_tenant(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_setting('app.current_tenant', true) IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'RLS_TENANT_CONTEXT_MISSING';
  END IF;
  IF current_setting('app.current_tenant')::uuid <> p_tenant_uuid THEN
    RAISE EXCEPTION USING MESSAGE = 'RLS_TENANT_MISMATCH';
  END IF;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_next int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  INSERT INTO orgunit.org_id_allocators (tenant_uuid, next_org_id)
  VALUES (p_tenant_uuid, 10000001)
  ON CONFLICT (tenant_uuid) DO UPDATE
  SET next_org_id = orgunit.org_id_allocators.next_org_id + 1,
      updated_at = now()
  WHERE orgunit.org_id_allocators.next_org_id <= 99999999
  RETURNING next_org_id - 1 INTO v_next;

  IF v_next IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ID_EXHAUSTED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  RETURN v_next;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.split_org_unit_version_at(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_row orgunit.org_unit_versions%ROWTYPE;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  SELECT * INTO v_row
  FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND validity @> p_effective_date
    AND lower(validity) < p_effective_date
  ORDER BY lower(validity) DESC
  LIMIT 1
  FOR UPDATE;

  IF NOT FOUND THEN
    RETURN;
  END IF;

  UPDATE orgunit.org_unit_versions
  SET validity = daterange(lower(validity), p_effective_date, '[)')
  WHERE id = v_row.id;

  INSERT INTO orgunit.org_unit_versions (
    tenant_uuid,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    full_name_path,
    status,
    is_business_unit,
    manager_uuid,
    last_event_id
  )
  VALUES (
    v_row.tenant_uuid,
    v_row.org_id,
    v_row.parent_id,
    v_row.node_path,
    daterange(p_effective_date, upper(v_row.validity), '[)'),
    v_row.name,
    v_row.full_name_path,
    v_row.status,
    v_row.is_business_unit,
    v_row.manager_uuid,
    p_event_db_id
  );
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.apply_create_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_org_code text,
  p_parent_id int,
  p_effective_date date,
  p_name text,
  p_manager_uuid uuid,
  p_is_business_unit boolean,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_parent_path ltree;
  v_node_path ltree;
  v_root_org_id int;
  v_is_business_unit boolean;
  v_org_code text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_name IS NULL OR btrim(p_name) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'name is required';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid AND org_id = p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_ALREADY_EXISTS', DETAIL = format('org_id=%s', p_org_id);
  END IF;

  IF p_parent_id IS NULL THEN
    SELECT t.root_org_id INTO v_root_org_id
    FROM orgunit.org_trees t
    WHERE t.tenant_uuid = p_tenant_uuid
    FOR UPDATE;

    IF v_root_org_id IS NOT NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_ROOT_ALREADY_EXISTS',
        DETAIL = format('root_org_id=%s', v_root_org_id);
    END IF;

    INSERT INTO orgunit.org_trees (tenant_uuid, root_org_id)
    VALUES (p_tenant_uuid, p_org_id);

    v_node_path := text2ltree(orgunit.org_ltree_label(p_org_id));
    IF p_is_business_unit IS NOT NULL AND p_is_business_unit = false THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_ROOT_BUSINESS_UNIT_REQUIRED',
        DETAIL = format('org_id=%s', p_org_id);
    END IF;
    v_is_business_unit := true;
  ELSE
    SELECT t.root_org_id INTO v_root_org_id
    FROM orgunit.org_trees t
    WHERE t.tenant_uuid = p_tenant_uuid;

    IF v_root_org_id IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_TREE_NOT_INITIALIZED',
        DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
    END IF;

    SELECT v.node_path INTO v_parent_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_parent_id
      AND v.status = 'active'
      AND v.validity @> p_effective_date
    LIMIT 1;

    IF v_parent_path IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_PARENT_NOT_FOUND_AS_OF',
        DETAIL = format('parent_id=%s as_of=%s', p_parent_id, p_effective_date);
    END IF;

    v_node_path := v_parent_path || text2ltree(orgunit.org_ltree_label(p_org_id));
    v_is_business_unit := COALESCE(p_is_business_unit, false);
  END IF;

  v_org_code := NULLIF(p_org_code, '');
  IF v_org_code IS NOT NULL THEN
    v_org_code := upper(v_org_code);
    INSERT INTO orgunit.org_unit_codes (tenant_uuid, org_id, org_code)
    VALUES (p_tenant_uuid, p_org_id, v_org_code);
  END IF;

  INSERT INTO orgunit.org_unit_versions (
    tenant_uuid,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    full_name_path,
    status,
    is_business_unit,
    manager_uuid,
    last_event_id
  )
  VALUES (
    p_tenant_uuid,
    p_org_id,
    p_parent_id,
    v_node_path,
    daterange(p_effective_date, NULL, '[)'),
    p_name,
    p_name,
    'active',
    v_is_business_unit,
    p_manager_uuid,
    p_event_db_id
  );
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.apply_move_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_new_parent_id int,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_root_org_id int;
  v_old_path ltree;
  v_new_parent_path ltree;
  v_new_prefix ltree;
  v_old_level int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_new_parent_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'new_parent_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  IF v_root_org_id IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_TREE_NOT_INITIALIZED',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;
  IF v_root_org_id = p_org_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_CANNOT_BE_MOVED',
      DETAIL = format('root_org_id=%s', v_root_org_id);
  END IF;

  SELECT v.node_path INTO v_old_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.status = 'active'
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1
  FOR UPDATE;

  IF v_old_path IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  SELECT v.node_path INTO v_new_parent_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_new_parent_id
    AND v.status = 'active'
    AND v.validity @> p_effective_date
  LIMIT 1;

  IF v_new_parent_path IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_PARENT_NOT_FOUND_AS_OF',
      DETAIL = format('parent_id=%s as_of=%s', p_new_parent_id, p_effective_date);
  END IF;

  IF v_new_parent_path <@ v_old_path THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_CYCLE_MOVE',
      DETAIL = format('cycle move: org_id=%s new_parent_id=%s', p_org_id, p_new_parent_id);
  END IF;

  v_new_prefix := v_new_parent_path || text2ltree(orgunit.org_ltree_label(p_org_id));
  v_old_level := nlevel(v_old_path);

  WITH split AS (
    SELECT *
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND node_path <@ v_old_path
      AND validity @> p_effective_date
      AND lower(validity) < p_effective_date
  ),
  upd AS (
    UPDATE orgunit.org_unit_versions v
    SET validity = daterange(lower(v.validity), p_effective_date, '[)')
    FROM split s
    WHERE v.id = s.id
    RETURNING s.*
  )
  INSERT INTO orgunit.org_unit_versions (
    tenant_uuid,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    full_name_path,
    status,
    is_business_unit,
    manager_uuid,
    last_event_id
  )
  SELECT
    u.tenant_uuid,
    u.org_id,
    CASE WHEN u.org_id = p_org_id THEN p_new_parent_id ELSE u.parent_id END,
    CASE
      WHEN u.org_id = p_org_id THEN v_new_prefix
      ELSE v_new_prefix || subpath(u.node_path, v_old_level)
    END,
    daterange(p_effective_date, upper(u.validity), '[)'),
    u.name,
    u.full_name_path,
    u.status,
    u.is_business_unit,
    u.manager_uuid,
    p_event_db_id
  FROM upd u;

  UPDATE orgunit.org_unit_versions v
  SET
    node_path = CASE
      WHEN v.org_id = p_org_id THEN v_new_prefix
        ELSE v_new_prefix || subpath(v.node_path, v_old_level)
      END,
    parent_id = CASE WHEN v.org_id = p_org_id THEN p_new_parent_id ELSE v.parent_id END,
    last_event_id = p_event_db_id
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.node_path <@ v_old_path
    AND lower(v.validity) >= p_effective_date;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.apply_rename_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_new_name text,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_stop_date date;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_new_name IS NULL OR btrim(p_new_name) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'new_name is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.event_type = 'RENAME'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET name = p_new_name, last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.apply_disable_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  UPDATE orgunit.org_unit_versions
  SET status = 'disabled', last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.apply_set_business_unit_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_is_business_unit boolean,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_stop_date date;
  v_status text;
  v_root_org_id int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_is_business_unit IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'is_business_unit is required';
  END IF;

  SELECT v.status INTO v_status
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_status IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;
  IF v_status <> 'active' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  IF v_root_org_id IS NOT NULL AND v_root_org_id = p_org_id AND p_is_business_unit = false THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_BUSINESS_UNIT_REQUIRED',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.event_type = 'SET_BUSINESS_UNIT'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET is_business_unit = p_is_business_unit, last_event_id = p_event_db_id
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.rebuild_full_name_path_subtree(
  p_tenant_uuid uuid,
  p_root_path ltree,
  p_from_date date
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_root_path IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'root_path is required';
  END IF;
  IF p_from_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'from_date is required';
  END IF;

  UPDATE orgunit.org_unit_versions v
  SET full_name_path = (
    SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
    FROM unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
    JOIN orgunit.org_unit_versions a
      ON a.tenant_uuid = v.tenant_uuid
     AND a.org_id = t.uid
     AND a.validity @> lower(v.validity)
  )
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.node_path <@ p_root_path
    AND lower(v.validity) >= p_from_date;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.assert_org_unit_validity(
  p_tenant_uuid uuid,
  p_org_ids int[]
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_ids IS NULL OR array_length(p_org_ids, 1) IS NULL THEN
    RETURN;
  END IF;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        org_id,
        validity,
        lag(validity) OVER (PARTITION BY org_id ORDER BY lower(validity)) AS prev_validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
        AND org_id = ANY(p_org_ids)
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_GAP',
      DETAIL = 'org_unit_versions must be gapless';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT DISTINCT ON (org_id) org_id, validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
        AND org_id = ANY(p_org_ids)
      ORDER BY org_id, lower(validity) DESC
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_NOT_INFINITE',
      DETAIL = 'last version validity must be unbounded (infinity)';
  END IF;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.replay_org_unit_versions(
  p_tenant_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_trees
  WHERE tenant_uuid = p_tenant_uuid;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events
    WHERE tenant_uuid = p_tenant_uuid
    ORDER BY effective_date, id
  LOOP
    v_payload := COALESCE(v_event.payload, '{}'::jsonb);

    IF v_event.event_type = 'CREATE' THEN
      v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
      v_name := NULLIF(btrim(v_payload->>'name'), '');
      v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
      v_org_code := NULLIF(v_payload->>'org_code', '');
      v_is_business_unit := NULL;
      IF v_payload ? 'is_business_unit' THEN
        BEGIN
          v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_INVALID_ARGUMENT',
              DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
        END;
      END IF;
      PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_event.org_id, v_org_code, v_parent_id, v_event.effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event.id);
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'SET_BUSINESS_UNIT' THEN
      IF NOT (v_payload ? 'is_business_unit') THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = 'is_business_unit is required';
      END IF;
      BEGIN
        v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_INVALID_ARGUMENT',
            DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
      END;
      PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_is_business_unit, v_event.id);
    ELSE
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = format('unexpected event_type: %s', v_event.event_type);
    END IF;
  END LOOP;

  IF EXISTS (
    WITH ordered AS (
      SELECT
        org_id,
        validity,
        lag(validity) OVER (PARTITION BY org_id ORDER BY lower(validity)) AS prev_validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
    )
    SELECT 1
    FROM ordered
    WHERE prev_validity IS NOT NULL
      AND lower(validity) <> upper(prev_validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_GAP',
      DETAIL = 'org_unit_versions must be gapless';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM (
      SELECT DISTINCT ON (org_id) org_id, validity
      FROM orgunit.org_unit_versions
      WHERE tenant_uuid = p_tenant_uuid
      ORDER BY org_id, lower(validity) DESC
    ) last
    WHERE NOT upper_inf(last.validity)
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_VALIDITY_NOT_INFINITE',
      DETAIL = 'last version validity must be unbounded (infinity)';
  END IF;

  UPDATE orgunit.org_unit_versions v
  SET full_name_path = (
    SELECT string_agg(a.name, ' / ' ORDER BY t.idx)
    FROM unnest(v.path_ids) WITH ORDINALITY AS t(uid, idx)
    JOIN orgunit.org_unit_versions a
      ON a.tenant_uuid = v.tenant_uuid
     AND a.org_id = t.uid
     AND a.validity @> lower(v.validity)
  )
  WHERE v.tenant_uuid = p_tenant_uuid
;
END;
$$;`,
		`CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_org_id int;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  IF p_event_type NOT IN ('CREATE','MOVE','RENAME','DISABLE','SET_BUSINESS_UNIT') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF p_event_type = 'SET_BUSINESS_UNIT' THEN
    IF NOT (v_payload ? 'is_business_unit') THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = 'is_business_unit is required';
    END IF;
    BEGIN
      PERFORM (v_payload->>'is_business_unit')::boolean;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
    END;
  END IF;

  IF p_event_type = 'CREATE' AND p_org_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF FOUND THEN
      IF v_existing.tenant_uuid <> p_tenant_uuid
        OR v_existing.event_type <> p_event_type
        OR v_existing.effective_date <> p_effective_date
        OR v_existing.payload <> v_payload
        OR v_existing.request_code <> p_request_code
        OR v_existing.initiator_uuid <> p_initiator_uuid
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
          DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
      END IF;

      RETURN v_existing.id;
    END IF;

    v_org_id := orgunit.allocate_org_id(p_tenant_uuid);
  ELSE
    IF p_org_id IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
    END IF;
    v_org_id := p_org_id;
  END IF;

  INSERT INTO orgunit.org_events (
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid
  )
  VALUES (
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_code,
    p_initiator_uuid
  )
  ON CONFLICT (event_uuid) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_uuid = p_event_uuid;

    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_id <> v_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_code <> p_request_code
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  IF p_event_type = 'CREATE' THEN
    v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
    v_name := NULLIF(btrim(v_payload->>'name'), '');
    v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
    v_org_code := NULLIF(v_payload->>'org_code', '');
    v_is_business_unit := NULL;
    IF v_payload ? 'is_business_unit' THEN
      BEGIN
        v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
      EXCEPTION
        WHEN invalid_text_representation THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_INVALID_ARGUMENT',
            DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
      END;
    END IF;
    PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_org_id, v_org_code, v_parent_id, p_effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'MOVE' THEN
    v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
    PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_org_id, v_new_parent_id, p_effective_date, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.node_path <@ v_root_path;
  ELSIF p_event_type = 'RENAME' THEN
    v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
    PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_org_id, p_effective_date, v_new_name, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'DISABLE' THEN
    PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_org_id, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  END IF;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);

  RETURN v_event_db_id;
END;
$$;`,
	}

	for _, stmt := range ddl {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
