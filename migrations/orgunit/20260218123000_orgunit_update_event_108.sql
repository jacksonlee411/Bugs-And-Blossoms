-- +goose Up
-- +goose StatementBegin
-- DEV-PLAN-108: 引入 UPDATE 有效事件（单事件多字段），并放宽 move/BU 的 active 硬依赖。
-- Source:
--   modules/orgunit/infrastructure/persistence/schema/00002_orgunit_org_schema.sql
--   modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql
--   modules/orgunit/infrastructure/persistence/schema/00015_orgunit_org_id_allocator.sql

ALTER TABLE orgunit.org_events
  DROP CONSTRAINT IF EXISTS org_events_event_type_check,
  ADD CONSTRAINT org_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT','CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG'));

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

CREATE OR REPLACE VIEW orgunit.org_events_effective AS
WITH correction_events AS (
  SELECT
    e.*,
    (e.payload->>'target_event_uuid')::uuid AS target_event_uuid
  FROM orgunit.org_events e
  WHERE e.event_type IN ('CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')
    AND e.payload ? 'target_event_uuid'
),
latest_corrections AS (
  SELECT DISTINCT ON (tenant_uuid, target_event_uuid)
    tenant_uuid,
    target_event_uuid,
    event_type AS correction_type,
    payload AS correction_payload,
    tx_time,
    id
  FROM correction_events
  ORDER BY tenant_uuid, target_event_uuid, tx_time DESC, id DESC
)
SELECT
  e.id,
  e.event_uuid,
  e.tenant_uuid,
  e.org_id,
  CASE
    WHEN lc.correction_type = 'CORRECT_STATUS'
      AND COALESCE(lc.correction_payload->>'target_status', '') = 'active'
      THEN 'ENABLE'
    WHEN lc.correction_type = 'CORRECT_STATUS'
      AND COALESCE(lc.correction_payload->>'target_status', '') = 'disabled'
      THEN 'DISABLE'
    ELSE e.event_type
  END AS event_type,
  CASE
    WHEN lc.correction_type = 'CORRECT_EVENT'
      AND lc.correction_payload ? 'effective_date'
      THEN NULLIF(btrim(lc.correction_payload->>'effective_date'), '')::date
    ELSE e.effective_date
  END AS effective_date,
  CASE
    WHEN lc.correction_type = 'CORRECT_EVENT'
      THEN orgunit.merge_org_event_payload_with_correction(e.payload, lc.correction_payload)
    ELSE e.payload
  END AS payload,
  e.request_id,
  e.initiator_uuid,
  e.transaction_time,
  e.created_at
FROM orgunit.org_events e
LEFT JOIN latest_corrections lc
  ON lc.tenant_uuid = e.tenant_uuid
 AND lc.target_event_uuid = e.event_uuid
WHERE e.event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
  AND COALESCE(lc.correction_type, '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG');

CREATE OR REPLACE FUNCTION orgunit.is_org_ext_payload_allowed_for_event(
  p_event_type text,
  p_target_event_type text
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT CASE
    WHEN p_event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN true
    WHEN p_event_type = 'CORRECT_EVENT'
      THEN p_target_event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
    ELSE false
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
  p_event_db_id bigint,
  p_status text DEFAULT 'active'
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
  v_status text;
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

  v_status := lower(btrim(COALESCE(p_status, 'active')));
  IF v_status IN ('enabled', '有效') THEN
    v_status := 'active';
  ELSIF v_status IN ('inactive', '无效') THEN
    v_status := 'disabled';
  END IF;
  IF v_status NOT IN ('active', 'disabled') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('status=%s', p_status);
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
    v_status,
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
    ext_str_01,
    ext_str_02,
    ext_str_03,
    ext_str_04,
    ext_str_05,
    ext_str_06,
    ext_str_07,
    ext_str_08,
    ext_str_09,
    ext_str_10,
    ext_str_11,
    ext_str_12,
    ext_str_13,
    ext_str_14,
    ext_str_15,
    ext_str_16,
    ext_str_17,
    ext_str_18,
    ext_str_19,
    ext_str_20,
    ext_str_21,
    ext_str_22,
    ext_str_23,
    ext_str_24,
    ext_str_25,
    ext_str_26,
    ext_str_27,
    ext_str_28,
    ext_str_29,
    ext_str_30,
    ext_str_31,
    ext_str_32,
    ext_str_33,
    ext_str_34,
    ext_str_35,
    ext_str_36,
    ext_str_37,
    ext_str_38,
    ext_str_39,
    ext_str_40,
    ext_str_41,
    ext_str_42,
    ext_str_43,
    ext_str_44,
    ext_str_45,
    ext_str_46,
    ext_str_47,
    ext_str_48,
    ext_str_49,
    ext_str_50,
    ext_str_51,
    ext_str_52,
    ext_str_53,
    ext_str_54,
    ext_str_55,
    ext_str_56,
    ext_str_57,
    ext_str_58,
    ext_str_59,
    ext_str_60,
    ext_str_61,
    ext_str_62,
    ext_str_63,
    ext_str_64,
    ext_str_65,
    ext_str_66,
    ext_str_67,
    ext_str_68,
    ext_str_69,
    ext_str_70,
    ext_int_01,
    ext_int_02,
    ext_int_03,
    ext_int_04,
    ext_int_05,
    ext_int_06,
    ext_int_07,
    ext_int_08,
    ext_int_09,
    ext_int_10,
    ext_int_11,
    ext_int_12,
    ext_int_13,
    ext_int_14,
    ext_int_15,
    ext_uuid_01,
    ext_uuid_02,
    ext_uuid_03,
    ext_uuid_04,
    ext_uuid_05,
    ext_uuid_06,
    ext_uuid_07,
    ext_uuid_08,
    ext_uuid_09,
    ext_uuid_10,
    ext_bool_01,
    ext_bool_02,
    ext_bool_03,
    ext_bool_04,
    ext_bool_05,
    ext_bool_06,
    ext_bool_07,
    ext_bool_08,
    ext_bool_09,
    ext_bool_10,
    ext_bool_11,
    ext_bool_12,
    ext_bool_13,
    ext_bool_14,
    ext_bool_15,
    ext_date_01,
    ext_date_02,
    ext_date_03,
    ext_date_04,
    ext_date_05,
    ext_date_06,
    ext_date_07,
    ext_date_08,
    ext_date_09,
    ext_date_10,
    ext_date_11,
    ext_date_12,
    ext_date_13,
    ext_date_14,
    ext_date_15,
    ext_num_01,
    ext_num_02,
    ext_num_03,
    ext_num_04,
    ext_num_05,
    ext_num_06,
    ext_num_07,
    ext_num_08,
    ext_num_09,
    ext_num_10,
    ext_labels_snapshot,
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
    u.ext_str_01,
    u.ext_str_02,
    u.ext_str_03,
    u.ext_str_04,
    u.ext_str_05,
    u.ext_str_06,
    u.ext_str_07,
    u.ext_str_08,
    u.ext_str_09,
    u.ext_str_10,
    u.ext_str_11,
    u.ext_str_12,
    u.ext_str_13,
    u.ext_str_14,
    u.ext_str_15,
    u.ext_str_16,
    u.ext_str_17,
    u.ext_str_18,
    u.ext_str_19,
    u.ext_str_20,
    u.ext_str_21,
    u.ext_str_22,
    u.ext_str_23,
    u.ext_str_24,
    u.ext_str_25,
    u.ext_str_26,
    u.ext_str_27,
    u.ext_str_28,
    u.ext_str_29,
    u.ext_str_30,
    u.ext_str_31,
    u.ext_str_32,
    u.ext_str_33,
    u.ext_str_34,
    u.ext_str_35,
    u.ext_str_36,
    u.ext_str_37,
    u.ext_str_38,
    u.ext_str_39,
    u.ext_str_40,
    u.ext_str_41,
    u.ext_str_42,
    u.ext_str_43,
    u.ext_str_44,
    u.ext_str_45,
    u.ext_str_46,
    u.ext_str_47,
    u.ext_str_48,
    u.ext_str_49,
    u.ext_str_50,
    u.ext_str_51,
    u.ext_str_52,
    u.ext_str_53,
    u.ext_str_54,
    u.ext_str_55,
    u.ext_str_56,
    u.ext_str_57,
    u.ext_str_58,
    u.ext_str_59,
    u.ext_str_60,
    u.ext_str_61,
    u.ext_str_62,
    u.ext_str_63,
    u.ext_str_64,
    u.ext_str_65,
    u.ext_str_66,
    u.ext_str_67,
    u.ext_str_68,
    u.ext_str_69,
    u.ext_str_70,
    u.ext_int_01,
    u.ext_int_02,
    u.ext_int_03,
    u.ext_int_04,
    u.ext_int_05,
    u.ext_int_06,
    u.ext_int_07,
    u.ext_int_08,
    u.ext_int_09,
    u.ext_int_10,
    u.ext_int_11,
    u.ext_int_12,
    u.ext_int_13,
    u.ext_int_14,
    u.ext_int_15,
    u.ext_uuid_01,
    u.ext_uuid_02,
    u.ext_uuid_03,
    u.ext_uuid_04,
    u.ext_uuid_05,
    u.ext_uuid_06,
    u.ext_uuid_07,
    u.ext_uuid_08,
    u.ext_uuid_09,
    u.ext_uuid_10,
    u.ext_bool_01,
    u.ext_bool_02,
    u.ext_bool_03,
    u.ext_bool_04,
    u.ext_bool_05,
    u.ext_bool_06,
    u.ext_bool_07,
    u.ext_bool_08,
    u.ext_bool_09,
    u.ext_bool_10,
    u.ext_bool_11,
    u.ext_bool_12,
    u.ext_bool_13,
    u.ext_bool_14,
    u.ext_bool_15,
    u.ext_date_01,
    u.ext_date_02,
    u.ext_date_03,
    u.ext_date_04,
    u.ext_date_05,
    u.ext_date_06,
    u.ext_date_07,
    u.ext_date_08,
    u.ext_date_09,
    u.ext_date_10,
    u.ext_date_11,
    u.ext_date_12,
    u.ext_date_13,
    u.ext_date_14,
    u.ext_date_15,
    u.ext_num_01,
    u.ext_num_02,
    u.ext_num_03,
    u.ext_num_04,
    u.ext_num_05,
    u.ext_num_06,
    u.ext_num_07,
    u.ext_num_08,
    u.ext_num_09,
    u.ext_num_10,
    u.ext_labels_snapshot,
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
  FROM orgunit.org_events_effective e
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

CREATE OR REPLACE FUNCTION orgunit.apply_update_logic(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_payload jsonb,
  p_event_db_id bigint
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_payload jsonb;
  v_parent_id int;
  v_name text;
  v_status text;
  v_is_business_unit boolean;
  v_manager_uuid uuid;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  IF jsonb_typeof(v_payload) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'payload must be an object';
  END IF;

  -- 1) parent (move)
  IF v_payload ? 'parent_id' OR v_payload ? 'new_parent_id' THEN
    v_parent_id := NULLIF(COALESCE(v_payload->>'parent_id', v_payload->>'new_parent_id'), '')::int;
    IF v_parent_id IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'parent_id is required';
    END IF;
    PERFORM orgunit.apply_move_logic(p_tenant_uuid, p_org_id, v_parent_id, p_effective_date, p_event_db_id);
  END IF;

  -- 2) status (enable/disable)
  IF v_payload ? 'status' THEN
    v_status := lower(btrim(COALESCE(v_payload->>'status', '')));
    IF v_status IN ('enabled', '有效') THEN
      v_status := 'active';
    ELSIF v_status IN ('inactive', '无效') THEN
      v_status := 'disabled';
    END IF;
    IF v_status = 'active' THEN
      PERFORM orgunit.apply_enable_logic(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);
    ELSIF v_status = 'disabled' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);
    ELSE
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'status invalid';
    END IF;
  END IF;

  -- 3) is_business_unit
  IF v_payload ? 'is_business_unit' THEN
    BEGIN
      v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_INVALID_ARGUMENT',
          DETAIL = format('is_business_unit=%s', v_payload->>'is_business_unit');
    END;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, p_org_id, p_effective_date, v_is_business_unit, p_event_db_id);
  END IF;

  -- 4) manager
  IF v_payload ? 'manager_uuid' THEN
    v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
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
    SET manager_uuid = v_manager_uuid, last_event_id = p_event_db_id
    WHERE tenant_uuid = p_tenant_uuid
      AND org_id = p_org_id
      AND lower(validity) >= p_effective_date;
  END IF;

  -- 5) name (rename)
  IF v_payload ? 'name' OR v_payload ? 'new_name' THEN
    v_name := NULLIF(btrim(COALESCE(v_payload->>'name', v_payload->>'new_name')), '');
    IF v_name IS NULL THEN
      RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'name is required';
    END IF;
    PERFORM orgunit.apply_rename_logic(p_tenant_uuid, p_org_id, p_effective_date, v_name, p_event_db_id);
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.org_events_effective_for_replay(
  p_tenant_uuid uuid,
  p_org_id int,
  p_pending_event_id bigint,
  p_pending_event_uuid uuid,
  p_pending_event_type text,
  p_pending_effective_date date,
  p_pending_payload jsonb,
  p_pending_request_id text,
  p_pending_initiator_uuid uuid,
  p_pending_tx_time timestamptz,
  p_pending_transaction_time timestamptz,
  p_pending_created_at timestamptz
)
RETURNS TABLE (
  id bigint,
  event_uuid uuid,
  tenant_uuid uuid,
  org_id int,
  event_type text,
  effective_date date,
  payload jsonb,
  request_id text,
  initiator_uuid uuid,
  transaction_time timestamptz,
  created_at timestamptz
)
LANGUAGE sql
STABLE
AS $$
  WITH source_events AS (
    SELECT
      e.id,
      e.event_uuid,
      e.tenant_uuid,
      e.org_id,
      e.event_type,
      e.effective_date,
      COALESCE(e.payload, '{}'::jsonb) AS payload,
      e.request_id,
      e.initiator_uuid,
      e.tx_time,
      e.transaction_time,
      e.created_at
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id

    UNION ALL

    SELECT
      p_pending_event_id,
      p_pending_event_uuid,
      p_tenant_uuid,
      p_org_id,
      p_pending_event_type,
      p_pending_effective_date,
      COALESCE(p_pending_payload, '{}'::jsonb),
      p_pending_request_id,
      p_pending_initiator_uuid,
      p_pending_tx_time,
      p_pending_transaction_time,
      p_pending_created_at
    WHERE p_pending_event_id IS NOT NULL
  ),
  correction_events AS (
    SELECT
      se.*,
      (se.payload->>'target_event_uuid')::uuid AS target_event_uuid
    FROM source_events se
    WHERE se.event_type IN ('CORRECT_EVENT','CORRECT_STATUS','RESCIND_EVENT','RESCIND_ORG')
      AND se.payload ? 'target_event_uuid'
  ),
  latest_corrections AS (
    SELECT DISTINCT ON (tenant_uuid, target_event_uuid)
      tenant_uuid,
      target_event_uuid,
      event_type AS correction_type,
      payload AS correction_payload,
      tx_time,
      id
    FROM correction_events
    ORDER BY tenant_uuid, target_event_uuid, tx_time DESC, id DESC
  )
  SELECT
    se.id,
    se.event_uuid,
    se.tenant_uuid,
    se.org_id,
    CASE
      WHEN lc.correction_type = 'CORRECT_STATUS'
        AND COALESCE(lc.correction_payload->>'target_status', '') = 'active'
        THEN 'ENABLE'
      WHEN lc.correction_type = 'CORRECT_STATUS'
        AND COALESCE(lc.correction_payload->>'target_status', '') = 'disabled'
        THEN 'DISABLE'
      WHEN lc.correction_type = 'CORRECT_EVENT'
        AND (
          orgunit.merge_org_event_payload_with_correction(se.payload, lc.correction_payload) ?| ARRAY[
            'name',
            'parent_id',
            'status',
            'is_business_unit',
            'manager_uuid',
            'manager_pernr',
            'ext',
            'new_name',
            'new_parent_id'
          ]
        )
        THEN 'UPDATE'
      ELSE se.event_type
    END AS event_type,
    CASE
      WHEN lc.correction_type = 'CORRECT_EVENT'
        AND lc.correction_payload ? 'effective_date'
        THEN NULLIF(btrim(lc.correction_payload->>'effective_date'), '')::date
      ELSE se.effective_date
    END AS effective_date,
    CASE
      WHEN lc.correction_type = 'CORRECT_EVENT'
        THEN orgunit.merge_org_event_payload_with_correction(se.payload, lc.correction_payload)
      ELSE se.payload
    END AS payload,
    se.request_id,
    se.initiator_uuid,
    se.transaction_time,
    se.created_at
  FROM source_events se
  LEFT JOIN latest_corrections lc
    ON lc.tenant_uuid = se.tenant_uuid
   AND lc.target_event_uuid = se.event_uuid
  WHERE se.event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT')
    AND COALESCE(lc.correction_type, '') NOT IN ('RESCIND_EVENT', 'RESCIND_ORG')
  ORDER BY effective_date, id;
$$;

CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
  p_tenant_uuid uuid,
  p_org_id int,
  p_pending_event_id bigint,
  p_pending_event_uuid uuid,
  p_pending_event_type text,
  p_pending_effective_date date,
  p_pending_payload jsonb,
  p_pending_request_id text,
  p_pending_initiator_uuid uuid,
  p_pending_tx_time timestamptz,
  p_pending_transaction_time timestamptz,
  p_pending_created_at timestamptz
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_event record;
  v_payload jsonb;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
  v_root_org_id int;
  v_has_create boolean;
  v_status text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;

  IF p_pending_event_id IS NOT NULL THEN
    IF p_pending_event_uuid IS NULL
      OR p_pending_event_type IS NULL
      OR p_pending_effective_date IS NULL
      OR p_pending_request_id IS NULL
      OR p_pending_initiator_uuid IS NULL
      OR p_pending_tx_time IS NULL
      OR p_pending_transaction_time IS NULL
      OR p_pending_created_at IS NULL
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_INVALID_ARGUMENT',
        DETAIL = 'pending event metadata is incomplete';
    END IF;
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid;

  SELECT EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective_for_replay(
      p_tenant_uuid,
      p_org_id,
      p_pending_event_id,
      p_pending_event_uuid,
      p_pending_event_type,
      p_pending_effective_date,
      p_pending_payload,
      p_pending_request_id,
      p_pending_initiator_uuid,
      p_pending_tx_time,
      p_pending_transaction_time,
      p_pending_created_at
    ) e
    WHERE e.event_type = 'CREATE'
  ) INTO v_has_create;

  IF v_has_create AND v_root_org_id = p_org_id THEN
    DELETE FROM orgunit.org_trees
    WHERE tenant_uuid = p_tenant_uuid;
  ELSIF v_root_org_id = p_org_id AND NOT v_has_create THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_INVALID_ARGUMENT',
      DETAIL = format('root org missing create event: org_id=%s', p_org_id);
  END IF;

  DELETE FROM orgunit.org_unit_codes
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id;

  DELETE FROM orgunit.org_unit_versions
  WHERE tenant_uuid = p_tenant_uuid
    AND org_id = p_org_id;

  FOR v_event IN
    SELECT *
    FROM orgunit.org_events_effective_for_replay(
      p_tenant_uuid,
      p_org_id,
      p_pending_event_id,
      p_pending_event_uuid,
      p_pending_event_type,
      p_pending_effective_date,
      p_pending_payload,
      p_pending_request_id,
      p_pending_initiator_uuid,
      p_pending_tx_time,
      p_pending_transaction_time,
      p_pending_created_at
    )
    ORDER BY effective_date, id
  LOOP
    v_payload := COALESCE(v_event.payload, '{}'::jsonb);

    IF v_event.event_type = 'CREATE' THEN
      v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
      v_name := NULLIF(btrim(v_payload->>'name'), '');
      v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
      v_org_code := NULLIF(v_payload->>'org_code', '');
      v_status := NULLIF(btrim(v_payload->>'status'), '');
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
      PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_event.org_id, v_org_code, v_parent_id, v_event.effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event.id, v_status);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'UPDATE' THEN
      PERFORM orgunit.apply_update_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_payload, v_event.id);
      IF (v_payload ? 'parent_id')
        OR (v_payload ? 'new_parent_id')
        OR (v_payload ? 'name')
        OR (v_payload ? 'new_name')
      THEN
        SELECT v.node_path INTO v_root_path
        FROM orgunit.org_unit_versions v
        WHERE v.tenant_uuid = p_tenant_uuid
          AND v.org_id = p_org_id
          AND v.validity @> v_event.effective_date
        ORDER BY lower(v.validity) DESC
        LIMIT 1;
        IF v_root_path IS NOT NULL THEN
          PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
        END IF;
      END IF;
    ELSIF v_event.event_type = 'MOVE' THEN
      v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
      PERFORM orgunit.apply_move_logic(p_tenant_uuid, v_event.org_id, v_new_parent_id, v_event.effective_date, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'RENAME' THEN
      v_new_name := NULLIF(btrim(v_payload->>'new_name'), '');
      PERFORM orgunit.apply_rename_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_new_name, v_event.id);
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> v_event.effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, v_event.effective_date);
      END IF;
    ELSIF v_event.event_type = 'DISABLE' THEN
      PERFORM orgunit.apply_disable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
    ELSIF v_event.event_type = 'ENABLE' THEN
      PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_event.org_id, v_event.effective_date, v_event.id);
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

    PERFORM orgunit.apply_org_event_ext_payload(
      p_tenant_uuid,
      v_event.org_id,
      v_event.effective_date,
      v_event.event_type,
      v_payload
    );
  END LOOP;

  SELECT v.node_path INTO v_root_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_root_path IS NULL THEN
    RETURN;
  END IF;

  SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.node_path <@ v_root_path;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event(
  p_event_uuid uuid,
  p_tenant_uuid uuid,
  p_org_id int,
  p_event_type text,
  p_effective_date date,
  p_payload jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_event_db_id bigint;
  v_existing orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_payload jsonb;
  v_org_id int;
  v_parent_id int;
  v_new_parent_id int;
  v_name text;
  v_new_name text;
  v_status text;
  v_manager_uuid uuid;
  v_is_business_unit boolean;
  v_org_code text;
  v_root_path ltree;
  v_org_ids int[];
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_event_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_uuid is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  IF p_event_type NOT IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN
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
        OR v_existing.request_id <> p_request_id
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

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    -- Idempotency key is request_id: allow server-generated event_uuid to differ across retries.
    IF v_existing_request.org_id <> v_org_id
      OR v_existing_request.event_type <> p_event_type
      OR v_existing_request.effective_date <> p_effective_date
      OR v_existing_request.payload <> v_payload
      OR v_existing_request.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN v_existing_request.id;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_id, p_effective_date);

  SELECT * INTO v_existing
  FROM orgunit.org_events
  WHERE event_uuid = p_event_uuid;

  IF FOUND THEN
    IF v_existing.tenant_uuid <> p_tenant_uuid
      OR v_existing.org_id <> v_org_id
      OR v_existing.event_type <> p_event_type
      OR v_existing.effective_date <> p_effective_date
      OR v_existing.payload <> v_payload
      OR v_existing.request_id <> p_request_id
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_IDEMPOTENCY_REUSED',
        DETAIL = format('event_uuid=%s existing_id=%s', p_event_uuid, v_existing.id);
    END IF;

    RETURN v_existing.id;
  END IF;

  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  IF p_event_type = 'CREATE' THEN
    v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
    v_name := NULLIF(btrim(v_payload->>'name'), '');
    v_manager_uuid := NULLIF(v_payload->>'manager_uuid', '')::uuid;
    v_org_code := NULLIF(v_payload->>'org_code', '');
    v_status := NULLIF(btrim(v_payload->>'status'), '');
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
    PERFORM orgunit.apply_create_logic(p_tenant_uuid, v_org_id, v_org_code, v_parent_id, p_effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event_db_id, v_status);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = v_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'UPDATE' THEN
    PERFORM orgunit.apply_update_logic(p_tenant_uuid, v_org_id, p_effective_date, v_payload, v_event_db_id);

    IF (v_payload ? 'parent_id')
      OR (v_payload ? 'new_parent_id')
      OR (v_payload ? 'name')
      OR (v_payload ? 'new_name')
    THEN
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = v_org_id
        AND v.validity @> p_effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;

      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
      END IF;
    END IF;

    IF (v_payload ? 'parent_id') OR (v_payload ? 'new_parent_id') THEN
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = v_org_id
        AND v.validity @> p_effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;

      SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.node_path <@ v_root_path;
    ELSE
      v_org_ids := ARRAY[v_org_id];
    END IF;
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
  ELSIF p_event_type = 'ENABLE' THEN
    PERFORM orgunit.apply_enable_logic(p_tenant_uuid, v_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, v_org_id, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_ids := ARRAY[v_org_id];
  END IF;

  PERFORM orgunit.apply_org_event_ext_payload(
    p_tenant_uuid,
    v_org_id,
    p_effective_date,
    p_event_type,
    v_payload
  );

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, v_org_id, p_effective_date);
  PERFORM orgunit.assert_org_event_snapshots(p_event_type, v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_id,
    initiator_uuid,
    before_snapshot,
    after_snapshot
  )
  VALUES (
    v_event_db_id,
    p_event_uuid,
    p_tenant_uuid,
    v_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot
  );

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.guard_org_events_one_per_day_effective()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.event_type IN ('CREATE','UPDATE','MOVE','RENAME','DISABLE','ENABLE','SET_BUSINESS_UNIT') THEN
    IF EXISTS (
      SELECT 1
      FROM orgunit.org_events_effective e
      WHERE e.tenant_uuid = NEW.tenant_uuid
        AND e.org_id = NEW.org_id
        AND e.effective_date = NEW.effective_date
      LIMIT 1
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'EVENT_DATE_CONFLICT',
        DETAIL = format('org_id=%s effective_date=%s', NEW.org_id, NEW.effective_date);
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No down migration (DEV-PLAN-108)
-- +goose StatementEnd
