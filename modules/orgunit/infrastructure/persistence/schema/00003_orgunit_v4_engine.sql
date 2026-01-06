CREATE OR REPLACE FUNCTION orgunit.assert_current_tenant(p_tenant_id uuid)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_ctx_raw text;
  v_ctx_tenant uuid;
BEGIN
  IF p_tenant_id IS NULL THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = 'tenant_id is required';
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

  IF v_ctx_tenant <> p_tenant_id THEN
    RAISE EXCEPTION USING
      ERRCODE = 'P0001',
      MESSAGE = 'RLS_TENANT_MISMATCH',
      DETAIL = format('tenant_param=%s tenant_ctx=%s', p_tenant_id, v_ctx_tenant);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.split_org_unit_version_at(
  p_tenant_id uuid,
  p_hierarchy_type text,
  p_org_id uuid,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_row orgunit.org_unit_versions%ROWTYPE;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  SELECT * INTO v_row
  FROM orgunit.org_unit_versions
  WHERE tenant_id = p_tenant_id
    AND hierarchy_type = p_hierarchy_type
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
    tenant_id,
    hierarchy_type,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    status,
    manager_id,
    last_event_id
  )
  VALUES (
    v_row.tenant_id,
    v_row.hierarchy_type,
    v_row.org_id,
    v_row.parent_id,
    v_row.node_path,
    daterange(p_effective_date, upper(v_row.validity), '[)'),
    v_row.name,
    v_row.status,
    v_row.manager_id,
    p_event_db_id
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_create_logic(
  p_tenant_id uuid,
  p_hierarchy_type text,
  p_org_id uuid,
  p_parent_id uuid,
  p_effective_date date,
  p_name text,
  p_manager_id uuid,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_parent_path ltree;
  v_node_path ltree;
  v_root_org_id uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
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
    WHERE tenant_id = p_tenant_id AND hierarchy_type = p_hierarchy_type AND org_id = p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_ALREADY_EXISTS', DETAIL = format('org_id=%s', p_org_id);
  END IF;

  IF p_parent_id IS NULL THEN
    SELECT t.root_org_id INTO v_root_org_id
    FROM orgunit.org_trees t
    WHERE t.tenant_id = p_tenant_id AND t.hierarchy_type = p_hierarchy_type
    FOR UPDATE;

    IF v_root_org_id IS NOT NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_ROOT_ALREADY_EXISTS',
        DETAIL = format('root_org_id=%s', v_root_org_id);
    END IF;

    INSERT INTO orgunit.org_trees (tenant_id, hierarchy_type, root_org_id)
    VALUES (p_tenant_id, p_hierarchy_type, p_org_id);

    v_node_path := text2ltree(orgunit.org_ltree_label(p_org_id));
  ELSE
    SELECT t.root_org_id INTO v_root_org_id
    FROM orgunit.org_trees t
    WHERE t.tenant_id = p_tenant_id AND t.hierarchy_type = p_hierarchy_type;

    IF v_root_org_id IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_TREE_NOT_INITIALIZED',
        DETAIL = format('tenant_id=%s', p_tenant_id);
    END IF;

    SELECT v.node_path INTO v_parent_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_id = p_tenant_id
      AND v.hierarchy_type = p_hierarchy_type
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
  END IF;

  INSERT INTO orgunit.org_unit_versions (
    tenant_id,
    hierarchy_type,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    status,
    manager_id,
    last_event_id
  )
  VALUES (
    p_tenant_id,
	    p_hierarchy_type,
	    p_org_id,
	    p_parent_id,
	    v_node_path,
	    daterange(p_effective_date, NULL, '[)'),
	    p_name,
	    'active',
	    p_manager_id,
	    p_event_db_id
	  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_move_logic(
  p_tenant_id uuid,
  p_hierarchy_type text,
  p_org_id uuid,
  p_new_parent_id uuid,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_root_org_id uuid;
  v_old_path ltree;
  v_new_parent_path ltree;
  v_new_prefix ltree;
  v_old_level int;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
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
  WHERE t.tenant_id = p_tenant_id AND t.hierarchy_type = p_hierarchy_type;

  IF v_root_org_id IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_TREE_NOT_INITIALIZED',
      DETAIL = format('tenant_id=%s', p_tenant_id);
  END IF;
  IF v_root_org_id = p_org_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_CANNOT_BE_MOVED',
      DETAIL = format('root_org_id=%s', v_root_org_id);
  END IF;

  SELECT v.node_path INTO v_old_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = p_hierarchy_type
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
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = p_hierarchy_type
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
    WHERE tenant_id = p_tenant_id
      AND hierarchy_type = p_hierarchy_type
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
    tenant_id,
    hierarchy_type,
    org_id,
    parent_id,
    node_path,
    validity,
    name,
    status,
    manager_id,
    last_event_id
  )
  SELECT
    u.tenant_id,
    u.hierarchy_type,
    u.org_id,
    CASE WHEN u.org_id = p_org_id THEN p_new_parent_id ELSE u.parent_id END,
    CASE
      WHEN u.org_id = p_org_id THEN v_new_prefix
      ELSE v_new_prefix || subpath(u.node_path, v_old_level)
    END,
    daterange(p_effective_date, upper(u.validity), '[)'),
    u.name,
    u.status,
    u.manager_id,
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
  WHERE v.tenant_id = p_tenant_id
    AND v.hierarchy_type = p_hierarchy_type
    AND v.node_path <@ v_old_path
    AND lower(v.validity) >= p_effective_date;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_rename_logic(
  p_tenant_id uuid,
  p_hierarchy_type text,
  p_org_id uuid,
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
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
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
    WHERE tenant_id = p_tenant_id
      AND hierarchy_type = p_hierarchy_type
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_id, p_hierarchy_type, p_org_id, p_effective_date, p_event_db_id);

  SELECT MIN(e.effective_date) INTO v_stop_date
  FROM orgunit.org_events e
  WHERE e.tenant_id = p_tenant_id
    AND e.hierarchy_type = p_hierarchy_type
    AND e.org_id = p_org_id
    AND e.event_type = 'RENAME'
    AND e.effective_date > p_effective_date;

  UPDATE orgunit.org_unit_versions
  SET name = p_new_name, last_event_id = p_event_db_id
  WHERE tenant_id = p_tenant_id
    AND hierarchy_type = p_hierarchy_type
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date
    AND (v_stop_date IS NULL OR lower(validity) < v_stop_date);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.apply_disable_logic(
  p_tenant_id uuid,
  p_hierarchy_type text,
  p_org_id uuid,
  p_effective_date date,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions
    WHERE tenant_id = p_tenant_id
      AND hierarchy_type = p_hierarchy_type
      AND org_id = p_org_id
      AND validity @> p_effective_date
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_NOT_FOUND_AS_OF',
      DETAIL = format('org_id=%s as_of=%s', p_org_id, p_effective_date);
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_id, p_hierarchy_type, p_org_id, p_effective_date, p_event_db_id);

  UPDATE orgunit.org_unit_versions
  SET status = 'disabled', last_event_id = p_event_db_id
  WHERE tenant_id = p_tenant_id
    AND hierarchy_type = p_hierarchy_type
    AND org_id = p_org_id
    AND lower(validity) >= p_effective_date;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.replay_org_unit_versions(
  p_tenant_id uuid,
  p_hierarchy_type text
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_parent_id uuid;
  v_new_parent_id uuid;
  v_name text;
  v_new_name text;
  v_manager_id uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;

  v_lock_key := format('org:v4:%s:%s', p_tenant_id, p_hierarchy_type);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_id = p_tenant_id AND hierarchy_type = p_hierarchy_type;

  DELETE FROM orgunit.org_trees
  WHERE tenant_id = p_tenant_id AND hierarchy_type = p_hierarchy_type;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events
    WHERE tenant_id = p_tenant_id AND hierarchy_type = p_hierarchy_type
    ORDER BY effective_date, id
  LOOP
    v_payload := COALESCE(v_event.payload, '{}'::jsonb);

    IF v_event.event_type = 'CREATE' THEN
      v_parent_id := NULLIF(v_payload->>'parent_id', '')::uuid;
      v_name := NULLIF(btrim(v_payload->>'name'), '');
      v_manager_id := NULLIF(v_payload->>'manager_id', '')::uuid;
      PERFORM orgunit.apply_create_logic(p_tenant_id, p_hierarchy_type, v_event.org_id, v_parent_id, v_event.effective_date, v_name, v_manager_id, v_event.id);
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::uuid;
      PERFORM orgunit.apply_move_logic(p_tenant_id, p_hierarchy_type, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_id, p_hierarchy_type, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_id, p_hierarchy_type, v_event.org_id, v_event.effective_date, v_event.id);
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
      WHERE tenant_id = p_tenant_id AND hierarchy_type = p_hierarchy_type
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
      WHERE tenant_id = p_tenant_id AND hierarchy_type = p_hierarchy_type
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
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_id uuid,
  p_tenant_id uuid,
  p_hierarchy_type text,
  p_org_id uuid,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_id uuid
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
  PERFORM orgunit.assert_current_tenant(p_tenant_id);

  IF p_event_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_id is required';
  END IF;
  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_id is required';
  END IF;

  IF p_hierarchy_type <> 'OrgUnit' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported hierarchy_type: %s', p_hierarchy_type);
  END IF;
  IF p_event_type NOT IN ('CREATE','MOVE','RENAME','DISABLE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('unsupported event_type: %s', p_event_type);
  END IF;

  v_lock_key := format('org:v4:%s:%s', p_tenant_id, p_hierarchy_type);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_payload := COALESCE(p_payload, '{}'::jsonb);

  INSERT INTO orgunit.org_events (
    event_id,
    tenant_id,
    hierarchy_type,
    org_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_id
  )
  VALUES (
    p_event_id,
    p_tenant_id,
    p_hierarchy_type,
    p_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_id
  )
  ON CONFLICT (event_id) DO NOTHING
  RETURNING id INTO v_event_db_id;

  IF v_event_db_id IS NULL THEN
    SELECT * INTO v_existing
    FROM orgunit.org_events
    WHERE event_id = p_event_id;

    IF v_existing.tenant_id <> p_tenant_id
      OR v_existing.hierarchy_type <> p_hierarchy_type
      OR v_existing.org_id <> p_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_id <> p_initiator_id
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_id=%s existing_id=%s', p_event_id, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  PERFORM orgunit.replay_org_unit_versions(p_tenant_id, p_hierarchy_type);

  RETURN v_event_db_id;
END;
$$;
