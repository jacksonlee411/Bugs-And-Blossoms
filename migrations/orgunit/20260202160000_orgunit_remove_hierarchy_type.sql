-- +goose Up
-- +goose StatementBegin
ALTER TABLE orgunit.org_trees
  DROP CONSTRAINT IF EXISTS org_trees_hierarchy_type_check,
  DROP CONSTRAINT IF EXISTS org_trees_pkey,
  DROP COLUMN IF EXISTS hierarchy_type;
ALTER TABLE orgunit.org_trees
  ADD PRIMARY KEY (tenant_uuid);

ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_hierarchy_type_check,
  DROP CONSTRAINT IF EXISTS org_events_one_per_day_unique,
  DROP COLUMN IF EXISTS hierarchy_type;
DROP INDEX IF EXISTS orgunit.org_events_tenant_type_effective_idx;
ALTER TABLE orgunit.org_events
  ADD CONSTRAINT org_events_one_per_day_unique UNIQUE (tenant_uuid, org_id, effective_date);
CREATE INDEX IF NOT EXISTS org_events_tenant_org_effective_idx
  ON orgunit.org_events (tenant_uuid, org_id, effective_date, id);
CREATE INDEX IF NOT EXISTS org_events_tenant_effective_idx
  ON orgunit.org_events (tenant_uuid, effective_date, id);

ALTER TABLE orgunit.org_unit_versions
  DROP CONSTRAINT IF EXISTS org_unit_versions_hierarchy_type_check,
  DROP CONSTRAINT IF EXISTS org_unit_versions_no_overlap,
  DROP COLUMN IF EXISTS hierarchy_type;
DROP INDEX IF EXISTS orgunit.org_unit_versions_search_gist;
DROP INDEX IF EXISTS orgunit.org_unit_versions_active_day_gist;
DROP INDEX IF EXISTS orgunit.org_unit_versions_lookup_btree;
ALTER TABLE orgunit.org_unit_versions
  ADD CONSTRAINT org_unit_versions_no_overlap
  EXCLUDE USING gist (
    tenant_uuid gist_uuid_ops WITH =,
    org_id gist_int4_ops WITH =,
    validity WITH &&
  );
CREATE INDEX IF NOT EXISTS org_unit_versions_search_gist
  ON orgunit.org_unit_versions
  USING gist (tenant_uuid gist_uuid_ops, node_path, validity);
CREATE INDEX IF NOT EXISTS org_unit_versions_active_day_gist
  ON orgunit.org_unit_versions
  USING gist (tenant_uuid gist_uuid_ops, validity)
  WHERE status = 'active';
CREATE INDEX IF NOT EXISTS org_unit_versions_lookup_btree
  ON orgunit.org_unit_versions (tenant_uuid, org_id, lower(validity));

DROP FUNCTION IF EXISTS orgunit.split_org_unit_version_at(uuid, text, int, date, bigint);
DROP FUNCTION IF EXISTS orgunit.apply_create_logic(uuid, text, int, int, date, text, uuid, boolean, bigint);
DROP FUNCTION IF EXISTS orgunit.apply_create_logic(uuid, text, int, text, int, date, text, uuid, boolean, bigint);
DROP FUNCTION IF EXISTS orgunit.apply_move_logic(uuid, text, int, int, date, bigint);
DROP FUNCTION IF EXISTS orgunit.apply_rename_logic(uuid, text, int, date, text, bigint);
DROP FUNCTION IF EXISTS orgunit.apply_disable_logic(uuid, text, int, date, bigint);
DROP FUNCTION IF EXISTS orgunit.apply_set_business_unit_logic(uuid, text, int, date, boolean, bigint);
DROP FUNCTION IF EXISTS orgunit.replay_org_unit_versions(uuid, text);
DROP FUNCTION IF EXISTS orgunit.submit_org_event(uuid, uuid, text, int, text, date, jsonb, text, uuid);

CREATE OR REPLACE FUNCTION orgunit.assert_current_tenant(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'tenant_uuid is required';
  END IF;

  v_ctx_raw := current_setting('app.current_tenant', true);
  IF v_ctx_raw IS NULL OR btrim(v_ctx_raw) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_CONTEXT_MISSING',
      DETAIL = 'app.current_tenant is required';
  END IF;

  BEGIN
    v_ctx_tenant := v_ctx_raw::uuid;
  EXCEPTION
    WHEN invalid_text_representation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'RLS_TENANT_CONTEXT_INVALID',
        DETAIL = format('app.current_tenant=%s', v_ctx_raw);
  END;

  IF v_ctx_tenant <> p_tenant_uuid THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_MISMATCH',
      DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_uuid, v_ctx_tenant);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.split_org_unit_version_at(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_create_logic(
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

  v_org_code := NULLIF(btrim(p_org_code), '');
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
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_move_logic(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_rename_logic(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_disable_logic(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_set_business_unit_logic(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.replay_org_unit_versions(
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
      v_org_code := NULLIF(btrim(v_payload->>'org_code'), '');
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
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
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
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
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
    p_org_id,
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
      OR v_existing.org_id <> p_org_id
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

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_event_db_id;
END;
$$;
CREATE OR REPLACE FUNCTION orgunit.get_org_snapshot(p_tenant_uuid uuid, p_query_date date)
RETURNS TABLE (
  org_id int,
  parent_id int,
  name varchar(255),
  is_business_unit boolean,
  full_name_path text,
  depth int,
  manager_uuid uuid,
  node_path ltree
)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_query_date IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'query_date is required';
  END IF;

  RETURN QUERY
  SELECT
    v.org_id,
    v.parent_id,
    v.name,
    v.is_business_unit,
    v.full_name_path,
    nlevel(v.node_path) - 1 AS depth,
    v.manager_uuid,
    v.node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.status = 'active'
    AND v.validity @> p_query_date
  ORDER BY v.node_path;
END;
$$;
CREATE OR REPLACE FUNCTION orgunit.normalize_setid(p_setid text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  v text;
BEGIN
  IF p_setid IS NULL OR btrim(p_setid) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_FORMAT',
      DETAIL = 'setid is required';
  END IF;

  v := upper(btrim(p_setid));
  IF v !~ '^[A-Z0-9]{5}$' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_FORMAT',
      DETAIL = format('setid=%s', v);
  END IF;

  RETURN v;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.lock_setid_governance(p_tenant_uuid uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  k bigint;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  k := hashtextextended('orgunit.setid.governance:' || p_tenant_uuid::text, 0);
  PERFORM pg_advisory_xact_lock(k);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.assert_actor_scope_saas()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_scope text;
BEGIN
  v_scope := current_setting('app.current_actor_scope', true);
  IF v_scope IS NULL OR btrim(v_scope) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = 'app.current_actor_scope is required';
  END IF;
  IF v_scope <> 'saas' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = format('app.current_actor_scope=%s', v_scope);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.ensure_setid_bootstrap(
  p_tenant_uuid uuid,
  p_initiator_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_evt_id uuid;
  v_evt_db_id bigint;
  v_root_org_id int;
  v_root_valid_from date;
  v_scope_code text;
  v_scope_share_mode text;
  v_package_id uuid;
  v_global_tenant_id uuid;
  v_prev_actor text;
  v_prev_allow_share text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  PERFORM orgunit.lock_setid_governance(p_tenant_uuid);

  v_global_tenant_id := orgunit.global_tenant_id();
  v_prev_actor := current_setting('app.current_actor_scope', true);
  v_prev_allow_share := current_setting('app.allow_share_read', true);

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_uuid = p_tenant_uuid AND setid = 'DEFLT'
  ) THEN
    v_evt_id := gen_random_uuid();
    INSERT INTO orgunit.setid_events (event_uuid, tenant_uuid, event_type, setid, payload, request_code, initiator_uuid)
    VALUES (v_evt_id, p_tenant_uuid, 'BOOTSTRAP', 'DEFLT', jsonb_build_object('name', 'Default'), 'bootstrap:deflt', p_initiator_uuid)
    ON CONFLICT (tenant_uuid, request_code) DO NOTHING;

    SELECT id INTO v_evt_db_id
    FROM orgunit.setid_events
    WHERE tenant_uuid = p_tenant_uuid AND request_code = 'bootstrap:deflt'
    ORDER BY id DESC
    LIMIT 1;

    INSERT INTO orgunit.setids (tenant_uuid, setid, name, status, last_event_id)
    VALUES (p_tenant_uuid, 'DEFLT', 'Default', 'active', v_evt_db_id)
    ON CONFLICT (tenant_uuid, setid) DO NOTHING;
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid
  FOR UPDATE;

  IF v_root_org_id IS NULL THEN
    RETURN;
  END IF;

  SELECT lower(v.validity)::date INTO v_root_valid_from
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = v_root_org_id
    AND v.status = 'active'
    AND v.is_business_unit = true
    AND v.validity @> current_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_root_valid_from IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_BUSINESS_UNIT_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', v_root_org_id, current_date);
  END IF;

  FOR v_scope_code, v_scope_share_mode IN
    SELECT scope_code, share_mode
    FROM orgunit.scope_code_registry()
    WHERE is_stable = true
  LOOP
    IF v_scope_share_mode = 'shared-only' THEN
      PERFORM set_config('app.current_actor_scope', 'saas', true);
      PERFORM set_config('app.current_tenant', v_global_tenant_id::text, true);
      PERFORM set_config('app.allow_share_read', 'on', true);

      SELECT p.package_id INTO v_package_id
      FROM orgunit.global_setid_scope_packages p
      WHERE p.tenant_uuid = v_global_tenant_id
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';

      IF v_package_id IS NULL THEN
        v_package_id := gen_random_uuid();
        PERFORM orgunit.submit_global_scope_package_event(
          gen_random_uuid(),
          v_global_tenant_id,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          v_root_valid_from,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
          format('bootstrap:global-scope-package:deflt:%s', v_scope_code),
          v_global_tenant_id
        );

        SELECT p.package_id INTO v_package_id
        FROM orgunit.global_setid_scope_packages p
        WHERE p.tenant_uuid = v_global_tenant_id
          AND p.scope_code = v_scope_code
          AND p.package_code = 'DEFLT';
      END IF;

      IF v_package_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
          DETAIL = format('scope_code=%s', v_scope_code);
      END IF;

      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.global_setid_scope_package_versions v
        WHERE v.tenant_uuid = v_global_tenant_id
          AND v.scope_code = v_scope_code
          AND v.package_id = v_package_id
          AND v.status = 'active'
          AND v.validity @> v_root_valid_from
      ) THEN
        PERFORM orgunit.submit_global_scope_package_event(
          gen_random_uuid(),
          v_global_tenant_id,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          v_root_valid_from,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
          format('bootstrap:global-scope-package:deflt:%s:%s', v_scope_code, v_root_valid_from),
          v_global_tenant_id
        );
      END IF;

      PERFORM set_config('app.current_tenant', p_tenant_uuid::text, true);
      PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);

      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.setid_scope_subscriptions s
        WHERE s.tenant_uuid = p_tenant_uuid
          AND s.setid = 'DEFLT'
          AND s.scope_code = v_scope_code
          AND s.validity @> v_root_valid_from
      ) THEN
        PERFORM orgunit.submit_scope_subscription_event(
          gen_random_uuid(),
          p_tenant_uuid,
          'DEFLT',
          v_scope_code,
          v_package_id,
          v_global_tenant_id,
          'BOOTSTRAP',
          v_root_valid_from,
          format('bootstrap:scope-subscription:deflt:%s', v_scope_code),
          p_initiator_uuid
        );
      END IF;

      CONTINUE;
    END IF;

    SELECT p.package_id INTO v_package_id
    FROM orgunit.setid_scope_packages p
    WHERE p.tenant_uuid = p_tenant_uuid
      AND p.scope_code = v_scope_code
      AND p.package_code = 'DEFLT';

    IF v_package_id IS NULL THEN
      v_package_id := gen_random_uuid();
      PERFORM orgunit.submit_scope_package_event(
        gen_random_uuid(),
        p_tenant_uuid,
        v_scope_code,
        v_package_id,
        'BOOTSTRAP',
        v_root_valid_from,
        jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
        format('bootstrap:scope-package:deflt:%s', v_scope_code),
        p_initiator_uuid
      );

      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_uuid = p_tenant_uuid
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';
    END IF;

    IF v_package_id IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
        DETAIL = format('scope_code=%s', v_scope_code);
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.setid_scope_package_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.scope_code = v_scope_code
        AND v.package_id = v_package_id
        AND v.status = 'active'
        AND v.validity @> v_root_valid_from
    ) THEN
      PERFORM orgunit.submit_scope_package_event(
        gen_random_uuid(),
        p_tenant_uuid,
        v_scope_code,
        v_package_id,
        'BOOTSTRAP',
        v_root_valid_from,
        jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
        format('bootstrap:scope-package:deflt:%s:%s', v_scope_code, v_root_valid_from),
        p_initiator_uuid
      );
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.setid_scope_subscriptions s
      WHERE s.tenant_uuid = p_tenant_uuid
        AND s.setid = 'DEFLT'
        AND s.scope_code = v_scope_code
        AND s.validity @> v_root_valid_from
    ) THEN
      PERFORM orgunit.submit_scope_subscription_event(
        gen_random_uuid(),
        p_tenant_uuid,
        'DEFLT',
        v_scope_code,
        v_package_id,
        p_tenant_uuid,
        'BOOTSTRAP',
        v_root_valid_from,
        format('bootstrap:scope-subscription:deflt:%s', v_scope_code),
        p_initiator_uuid
      );
    END IF;
  END LOOP;

  PERFORM set_config('app.current_tenant', p_tenant_uuid::text, true);
  PERFORM set_config('app.current_actor_scope', COALESCE(v_prev_actor, ''), true);
  PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.setid_binding_versions
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = v_root_org_id
      AND validity @> v_root_valid_from
  ) THEN
    PERFORM orgunit.submit_setid_binding_event(
      gen_random_uuid(),
      p_tenant_uuid,
      v_root_org_id,
      v_root_valid_from,
      'DEFLT',
      'bootstrap:binding:deflt',
      p_initiator_uuid
    );
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_setid_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_event_type text,
  p_setid text,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_evt_db_id bigint;
  v_name text;
  v_scope_code text;
  v_scope_share_mode text;
  v_package_id uuid;
  v_effective_date date;
  v_global_tenant_id uuid;
  v_prev_actor text;
  v_prev_allow_share text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  PERFORM orgunit.lock_setid_governance(p_tenant_uuid);

  v_global_tenant_id := orgunit.global_tenant_id();
  v_prev_actor := current_setting('app.current_actor_scope', true);
  v_prev_allow_share := current_setting('app.allow_share_read', true);

  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_code is required';
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'event_type is required';
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid = 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'SHARE is reserved';
  END IF;

  INSERT INTO orgunit.setid_events (event_uuid, tenant_uuid, event_type, setid, payload, request_code, initiator_uuid)
  VALUES (p_event_uuid, p_tenant_uuid, p_event_type, v_setid, COALESCE(p_payload, '{}'::jsonb), p_request_code, p_initiator_uuid)
  ON CONFLICT (tenant_uuid, request_code) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_events
  WHERE tenant_uuid = p_tenant_uuid AND request_code = p_request_code
  ORDER BY id DESC
  LIMIT 1;

  IF p_event_type IN ('BOOTSTRAP','CREATE') THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    IF p_event_type = 'CREATE' AND EXISTS (
      SELECT 1 FROM orgunit.setids WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_ALREADY_EXISTS',
        DETAIL = format('setid=%s', v_setid);
    END IF;

    INSERT INTO orgunit.setids (tenant_uuid, setid, name, status, last_event_id)
    VALUES (p_tenant_uuid, v_setid, v_name, 'active', v_evt_db_id)
    ON CONFLICT (tenant_uuid, setid) DO UPDATE
    SET name = EXCLUDED.name,
        status = 'active',
        last_event_id = EXCLUDED.last_event_id,
        updated_at = now();

    v_effective_date := current_date;
    IF p_payload ? 'effective_date' THEN
      v_effective_date := NULLIF(btrim(p_payload->>'effective_date'), '')::date;
    END IF;
    IF v_effective_date IS NULL THEN
      v_effective_date := current_date;
    END IF;

    FOR v_scope_code, v_scope_share_mode IN
      SELECT scope_code, share_mode
      FROM orgunit.scope_code_registry()
      WHERE is_stable = true
    LOOP
      IF v_scope_share_mode = 'shared-only' THEN
        PERFORM set_config('app.current_actor_scope', 'saas', true);
        PERFORM set_config('app.current_tenant', v_global_tenant_id::text, true);
        PERFORM set_config('app.allow_share_read', 'on', true);

        SELECT p.package_id INTO v_package_id
        FROM orgunit.global_setid_scope_packages p
        WHERE p.tenant_uuid = v_global_tenant_id
          AND p.scope_code = v_scope_code
          AND p.package_code = 'DEFLT';

        IF v_package_id IS NULL THEN
          v_package_id := gen_random_uuid();
          PERFORM orgunit.submit_global_scope_package_event(
            gen_random_uuid(),
            v_global_tenant_id,
            v_scope_code,
            v_package_id,
            'BOOTSTRAP',
            v_effective_date,
            jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
            format('bootstrap:global-scope-package:deflt:%s', v_scope_code),
            v_global_tenant_id
          );

          SELECT p.package_id INTO v_package_id
          FROM orgunit.global_setid_scope_packages p
          WHERE p.tenant_uuid = v_global_tenant_id
            AND p.scope_code = v_scope_code
            AND p.package_code = 'DEFLT';
        END IF;

        IF v_package_id IS NULL THEN
          RAISE EXCEPTION USING
            ERRCODE = 'P0001',
            MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
            DETAIL = format('setid=%s scope_code=%s', v_setid, v_scope_code);
        END IF;

        PERFORM set_config('app.current_tenant', p_tenant_uuid::text, true);
        PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);

        IF NOT EXISTS (
          SELECT 1
          FROM orgunit.setid_scope_subscriptions s
          WHERE s.tenant_uuid = p_tenant_uuid
            AND s.setid = v_setid
            AND s.scope_code = v_scope_code
            AND s.validity @> v_effective_date
        ) THEN
          PERFORM orgunit.submit_scope_subscription_event(
            gen_random_uuid(),
            p_tenant_uuid,
            v_setid,
            v_scope_code,
            v_package_id,
            v_global_tenant_id,
            'BOOTSTRAP',
            v_effective_date,
            format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
            p_initiator_uuid
          );
        END IF;

        CONTINUE;
      END IF;

      SELECT p.package_id INTO v_package_id
      FROM orgunit.setid_scope_packages p
      WHERE p.tenant_uuid = p_tenant_uuid
        AND p.scope_code = v_scope_code
        AND p.package_code = 'DEFLT';

      IF v_package_id IS NULL THEN
        v_package_id := gen_random_uuid();
        PERFORM orgunit.submit_scope_package_event(
          gen_random_uuid(),
          p_tenant_uuid,
          v_scope_code,
          v_package_id,
          'BOOTSTRAP',
          v_effective_date,
          jsonb_build_object('package_code', 'DEFLT', 'name', 'Default', 'owner_setid', 'DEFLT'),
          format('bootstrap:scope-package:deflt:%s', v_scope_code),
          p_initiator_uuid
        );

        SELECT p.package_id INTO v_package_id
        FROM orgunit.setid_scope_packages p
        WHERE p.tenant_uuid = p_tenant_uuid
          AND p.scope_code = v_scope_code
          AND p.package_code = 'DEFLT';
      END IF;

      IF v_package_id IS NULL THEN
        RAISE EXCEPTION USING
          ERRCODE = 'P0001',
          MESSAGE = 'SUBSCRIPTION_DEFLT_MISSING',
          DETAIL = format('setid=%s scope_code=%s', v_setid, v_scope_code);
      END IF;

      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.setid_scope_subscriptions s
        WHERE s.tenant_uuid = p_tenant_uuid
          AND s.setid = v_setid
          AND s.scope_code = v_scope_code
          AND s.validity @> current_date
      ) THEN
        PERFORM orgunit.submit_scope_subscription_event(
          gen_random_uuid(),
          p_tenant_uuid,
          v_setid,
          v_scope_code,
          v_package_id,
          p_tenant_uuid,
          'BOOTSTRAP',
          v_effective_date,
          format('bootstrap:scope-subscription:%s:%s', v_setid, v_scope_code),
          p_initiator_uuid
        );
      END IF;
    END LOOP;

    PERFORM set_config('app.current_tenant', p_tenant_uuid::text, true);
    PERFORM set_config('app.current_actor_scope', COALESCE(v_prev_actor, ''), true);
    PERFORM set_config('app.allow_share_read', COALESCE(v_prev_allow_share, 'off'), true);
  ELSIF p_event_type = 'RENAME' THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;
    UPDATE orgunit.setids
    SET name = v_name,
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_setid);
    END IF;
  ELSIF p_event_type = 'DISABLE' THEN
    IF v_setid = 'DEFLT' THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_RESERVED_WORD',
        DETAIL = 'DEFLT is reserved';
    END IF;
    IF EXISTS (
      SELECT 1 FROM orgunit.setid_binding_versions
      WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid
    ) THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_IN_USE',
        DETAIL = format('setid=%s', v_setid);
    END IF;
    UPDATE orgunit.setids
    SET status = 'disabled',
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_setid);
    END IF;
  ELSE
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_global_setid_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_event_type text,
  p_setid text,
  p_payload jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_evt_db_id bigint;
  v_name text;
BEGIN
  IF p_tenant_uuid <> orgunit.global_tenant_id() THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ACTOR_SCOPE_FORBIDDEN',
      DETAIL = format('tenant_uuid=%s', p_tenant_uuid);
  END IF;

  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  PERFORM orgunit.assert_actor_scope_saas();

  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_code is required';
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'event_type is required';
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid <> 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'only SHARE is allowed';
  END IF;

  INSERT INTO orgunit.global_setid_events (event_uuid, tenant_uuid, event_type, setid, payload, request_code, initiator_uuid)
  VALUES (p_event_uuid, p_tenant_uuid, p_event_type, v_setid, COALESCE(p_payload, '{}'::jsonb), p_request_code, p_initiator_uuid)
  ON CONFLICT (tenant_uuid, request_code) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.global_setid_events
  WHERE tenant_uuid = p_tenant_uuid AND request_code = p_request_code
  ORDER BY id DESC
  LIMIT 1;

  IF p_event_type IN ('BOOTSTRAP','CREATE') THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;

    INSERT INTO orgunit.global_setids (tenant_uuid, setid, name, status, last_event_id)
    VALUES (p_tenant_uuid, v_setid, v_name, 'active', v_evt_db_id)
    ON CONFLICT (tenant_uuid, setid) DO UPDATE
    SET name = EXCLUDED.name,
        status = 'active',
        last_event_id = EXCLUDED.last_event_id,
        updated_at = now();
  ELSIF p_event_type = 'RENAME' THEN
    v_name := NULLIF(btrim(COALESCE(p_payload->>'name', '')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_INVALID_ARGUMENT',
        DETAIL = 'name is required';
    END IF;
    UPDATE orgunit.global_setids
    SET name = v_name,
        last_event_id = v_evt_db_id,
        updated_at = now()
    WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid;
    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_NOT_FOUND',
        DETAIL = format('setid=%s', v_setid);
    END IF;
  ELSIF p_event_type = 'DISABLE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_RESERVED_WORD',
      DETAIL = 'SHARE cannot be disabled';
  ELSE
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type=%s', p_event_type);
  END IF;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_setid_binding_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_setid text,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_setid text;
  v_evt_db_id bigint;
  v_org_status text;
  v_org_is_bu boolean;
  v_existing orgunit.setid_binding_versions%ROWTYPE;
  v_next_start date;
  v_current_end date;
  v_root_org_id int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);
  PERFORM orgunit.lock_setid_governance(p_tenant_uuid);

  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'request_code is required';
  END IF;
  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'event_uuid is required';
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'effective_date is required';
  END IF;

  v_setid := orgunit.normalize_setid(p_setid);
  IF v_setid = 'SHARE' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_SHARE_FORBIDDEN',
      DETAIL = 'SHARE is reserved';
  END IF;

  SELECT status INTO v_org_status
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_org_status IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;
  IF v_org_status <> 'active' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  SELECT is_business_unit INTO v_org_is_bu
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_effective_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_org_is_bu IS DISTINCT FROM true THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_BUSINESS_UNIT_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_NOT_FOUND',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  IF EXISTS (
    SELECT 1 FROM orgunit.setids WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid AND status <> 'active'
  ) THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_DISABLED',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  IF v_root_org_id IS NOT NULL AND v_root_org_id = p_org_id AND v_setid <> 'DEFLT' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_ROOT_BINDING_FORBIDDEN',
      DETAIL = format('org_id=%s setid=%s', p_org_id, v_setid);
  END IF;

  INSERT INTO orgunit.setid_binding_events (
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
    p_org_id,
    'BIND',
    p_effective_date,
    jsonb_build_object('setid', v_setid),
    p_request_code,
    p_initiator_uuid
  )
  ON CONFLICT (tenant_uuid, request_code) DO NOTHING;

  SELECT id INTO v_evt_db_id
  FROM orgunit.setid_binding_events
  WHERE tenant_uuid = p_tenant_uuid AND request_code = p_request_code
  ORDER BY id DESC
  LIMIT 1;

  SELECT min(lower(validity)) INTO v_next_start
  FROM orgunit.setid_binding_versions
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND lower(validity) > p_effective_date;

  SELECT * INTO v_existing
  FROM orgunit.setid_binding_versions
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id
    AND validity @> p_effective_date
  ORDER BY lower(validity) DESC
  LIMIT 1
  FOR UPDATE;

  BEGIN
    IF FOUND THEN
      v_current_end := upper(v_existing.validity);
      IF lower(v_existing.validity) = p_effective_date THEN
        UPDATE orgunit.setid_binding_versions
        SET setid = v_setid,
            last_event_id = v_evt_db_id,
            updated_at = now()
        WHERE id = v_existing.id;
      ELSE
        UPDATE orgunit.setid_binding_versions
        SET validity = daterange(lower(v_existing.validity), p_effective_date, '[)'),
            updated_at = now()
        WHERE id = v_existing.id;

        INSERT INTO orgunit.setid_binding_versions (
          tenant_uuid,
          org_id,
          setid,
          validity,
          last_event_id
        )
        VALUES (
          p_tenant_uuid,
          p_org_id,
          v_setid,
          daterange(p_effective_date, v_current_end, '[)'),
          v_evt_db_id
        );
      END IF;
    ELSE
      INSERT INTO orgunit.setid_binding_versions (
        tenant_uuid,
        org_id,
        setid,
        validity,
        last_event_id
      )
      VALUES (
        p_tenant_uuid,
        p_org_id,
        v_setid,
        daterange(p_effective_date, v_next_start, '[)'),
        v_evt_db_id
      );
    END IF;
  EXCEPTION
    WHEN exclusion_violation THEN
      RAISE EXCEPTION USING
        ERRCODE = 'P0001',
        MESSAGE = 'SETID_BINDING_OVERLAP',
        DETAIL = format('org_id=%s effective_date=%s', p_org_id, p_effective_date);
  END;

  RETURN v_evt_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.resolve_setid(
  p_tenant_uuid uuid,
  p_org_id int,
  p_as_of_date date
)
RETURNS text
LANGUAGE plpgsql
AS $$
DECLARE
  v_node_path ltree;
  v_org_status text;
  v_setid text;
  v_setid_status text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'org_id is required';
  END IF;
  IF p_as_of_date IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_INVALID_ARGUMENT',
      DETAIL = 'as_of_date is required';
  END IF;

  SELECT v.status, v.node_path INTO v_org_status, v_node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_as_of_date
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_org_status IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_as_of_date);
  END IF;
  IF v_org_status <> 'active' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INACTIVE_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_as_of_date);
  END IF;

  SELECT b.setid INTO v_setid
  FROM orgunit.setid_binding_versions b
  JOIN orgunit.org_unit_versions o
    ON o.tenant_uuid = b.tenant_uuid
   AND o.org_id = b.org_id
  WHERE b.tenant_uuid = p_tenant_uuid
    AND b.validity @> p_as_of_date
    AND o.validity @> p_as_of_date
    AND o.status = 'active'
    AND o.is_business_unit = true
    AND o.node_path @> v_node_path
  ORDER BY nlevel(o.node_path) DESC
  LIMIT 1;

  IF v_setid IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_BINDING_MISSING',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_as_of_date);
  END IF;

  SELECT status INTO v_setid_status
  FROM orgunit.setids
  WHERE tenant_uuid = p_tenant_uuid AND setid = v_setid;

  IF v_setid_status IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_NOT_FOUND',
      DETAIL = format('setid=%s', v_setid);
  END IF;
  IF v_setid_status <> 'active' THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'SETID_DISABLED',
      DETAIL = format('setid=%s', v_setid);
  END IF;

  RETURN v_setid;
END;
$$;
CREATE TABLE IF NOT EXISTS orgunit.org_id_allocators (
  tenant_uuid uuid NOT NULL,
  next_org_id int NOT NULL CHECK (next_org_id BETWEEN 10000000 AND 100000000),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid)
);

ALTER TABLE orgunit.org_id_allocators ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.org_id_allocators FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.org_id_allocators;
CREATE POLICY tenant_isolation ON orgunit.org_id_allocators
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE OR REPLACE FUNCTION orgunit.allocate_org_id(p_tenant_uuid uuid)
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
$$;

ALTER TABLE IF EXISTS orgunit.org_id_allocators OWNER TO orgunit_kernel;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE orgunit.org_id_allocators TO orgunit_kernel;

ALTER FUNCTION orgunit.allocate_org_id(uuid) OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SECURITY DEFINER;
ALTER FUNCTION orgunit.allocate_org_id(uuid) SET search_path = pg_catalog, orgunit, public;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
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

  PERFORM orgunit.replay_org_unit_versions(p_tenant_uuid);

  RETURN v_event_db_id;
END;
$$;

ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.submit_org_event(uuid, uuid, int, text, date, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
