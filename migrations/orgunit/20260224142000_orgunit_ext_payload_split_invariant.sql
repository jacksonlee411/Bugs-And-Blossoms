-- +goose Up
-- +goose StatementBegin
-- DEV-PLAN-162: 将 ext 生效切分不变量收敛到 apply_org_event_ext_payload。
-- 保证 UPDATE/CORRECT_EVENT 的 ext-only 在 effective_date 上总能命中版本切片。

CREATE OR REPLACE FUNCTION orgunit.apply_org_event_ext_payload(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_event_type text,
  p_payload jsonb,
  p_event_db_id bigint,
  p_target_event_type text DEFAULT NULL
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_payload jsonb;
  v_ext jsonb;
  v_labels jsonb;
  v_has_ext_payload boolean;
  v_has_ext_fields boolean;
  v_field_key text;
  v_label_key text;
  v_field_value jsonb;
  v_label_value jsonb;
  v_label_text text;
  v_value_text text;
  v_physical_col text;
  v_value_type text;
  v_data_source_type text;
  v_enabled_on date;
  v_disabled_on date;
  v_cast_type text;
  v_sql text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'effective_date is required';
  END IF;
  IF p_event_type IS NULL OR btrim(p_event_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_type is required';
  END IF;

  v_payload := COALESCE(p_payload, '{}'::jsonb);
  v_has_ext_payload := v_payload ? 'ext' OR v_payload ? 'ext_labels_snapshot';
  IF NOT v_has_ext_payload THEN
    RETURN;
  END IF;

  IF NOT orgunit.is_org_ext_payload_allowed_for_event(p_event_type, p_target_event_type) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EXT_PAYLOAD_NOT_ALLOWED_FOR_EVENT',
      DETAIL = format('event_type=%s target_event_type=%s', p_event_type, COALESCE(p_target_event_type, 'NULL'));
  END IF;

  v_ext := COALESCE(v_payload->'ext', '{}'::jsonb);
  v_labels := COALESCE(v_payload->'ext_labels_snapshot', '{}'::jsonb);

  IF jsonb_typeof(v_ext) <> 'object' OR jsonb_typeof(v_labels) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EXT_PAYLOAD_INVALID_SHAPE',
      DETAIL = 'payload.ext and payload.ext_labels_snapshot must be objects';
  END IF;

  FOR v_label_key IN
    SELECT key
    FROM jsonb_object_keys(v_labels) AS t(key)
  LOOP
    IF NOT (v_ext ? v_label_key) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_NOT_ALLOWED',
        DETAIL = format('field_key=%s', v_label_key);
    END IF;
  END LOOP;

  SELECT EXISTS (
    SELECT 1
    FROM jsonb_object_keys(v_ext)
  ) INTO v_has_ext_fields;

  IF NOT v_has_ext_fields THEN
    RETURN;
  END IF;

  IF p_event_db_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'event_db_id is required';
  END IF;

  PERFORM orgunit.split_org_unit_version_at(p_tenant_uuid, p_org_id, p_effective_date, p_event_db_id);

  FOR v_field_key, v_field_value IN
    SELECT key, value
    FROM jsonb_each(v_ext)
  LOOP
    IF v_field_key !~ '^[a-z][a-z0-9_]{0,62}$' THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_FIELD_NOT_CONFIGURED',
        DETAIL = format('field_key=%s', v_field_key);
    END IF;

    SELECT
      physical_col,
      value_type,
      data_source_type,
      enabled_on,
      disabled_on
    INTO v_physical_col, v_value_type, v_data_source_type, v_enabled_on, v_disabled_on
    FROM orgunit.tenant_field_configs
    WHERE tenant_uuid = p_tenant_uuid
      AND field_key = v_field_key
    LIMIT 1;

    IF NOT FOUND THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_FIELD_NOT_CONFIGURED',
        DETAIL = format('field_key=%s', v_field_key);
    END IF;

    IF NOT (
      v_enabled_on <= p_effective_date
      AND (v_disabled_on IS NULL OR p_effective_date < v_disabled_on)
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_FIELD_NOT_ENABLED_AS_OF',
        DETAIL = format('field_key=%s effective_date=%s', v_field_key, p_effective_date);
    END IF;

    IF v_physical_col !~ '^ext_(str|int|uuid|bool|date|num)_[0-9]{2}$'
      OR (v_value_type = 'text' AND v_physical_col NOT LIKE 'ext_str_%')
      OR (v_value_type = 'int' AND v_physical_col NOT LIKE 'ext_int_%')
      OR (v_value_type = 'uuid' AND v_physical_col NOT LIKE 'ext_uuid_%')
      OR (v_value_type = 'bool' AND v_physical_col NOT LIKE 'ext_bool_%')
      OR (v_value_type = 'date' AND v_physical_col NOT LIKE 'ext_date_%')
      OR (v_value_type = 'numeric' AND v_physical_col NOT LIKE 'ext_num_%')
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
        DETAIL = format('field_key=%s physical_col=%s value_type=%s', v_field_key, v_physical_col, v_value_type);
    END IF;

    IF NOT EXISTS (
      SELECT 1
      FROM information_schema.columns c
      WHERE c.table_schema = 'orgunit'
        AND c.table_name = 'org_unit_versions'
        AND c.column_name = v_physical_col
    ) THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
        DETAIL = format('field_key=%s physical_col=%s missing_in_org_unit_versions', v_field_key, v_physical_col);
    END IF;

    v_value_text := NULL;
    IF v_field_value <> 'null'::jsonb THEN
      IF v_value_type = 'text' THEN
        IF jsonb_typeof(v_field_value) <> 'string' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
            DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END IF;
        v_value_text := v_field_value #>> '{}';
      ELSIF v_value_type = 'int' THEN
        IF jsonb_typeof(v_field_value) NOT IN ('number', 'string') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
            DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END IF;
        BEGIN
          v_value_text := ((v_field_value #>> '{}')::int)::text;
        EXCEPTION
          WHEN invalid_text_representation OR numeric_value_out_of_range THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
              DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END;
      ELSIF v_value_type = 'uuid' THEN
        IF jsonb_typeof(v_field_value) <> 'string' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
            DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END IF;
        BEGIN
          v_value_text := ((v_field_value #>> '{}')::uuid)::text;
        EXCEPTION
          WHEN invalid_text_representation THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
              DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END;
      ELSIF v_value_type = 'bool' THEN
        IF jsonb_typeof(v_field_value) <> 'boolean' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
            DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END IF;
        v_value_text := ((v_field_value #>> '{}')::boolean)::text;
      ELSIF v_value_type = 'date' THEN
        IF jsonb_typeof(v_field_value) <> 'string' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
            DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END IF;
        BEGIN
          v_value_text := ((v_field_value #>> '{}')::date)::text;
        EXCEPTION
          WHEN invalid_text_representation OR datetime_field_overflow THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
              DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END;
      ELSIF v_value_type = 'numeric' THEN
        IF jsonb_typeof(v_field_value) NOT IN ('number', 'string') THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
            DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END IF;
        BEGIN
          v_value_text := ((v_field_value #>> '{}')::numeric)::text;
        EXCEPTION
          WHEN invalid_text_representation OR numeric_value_out_of_range THEN
            RAISE EXCEPTION USING
              MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
              DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
        END;
      ELSE
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
          DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
      END IF;
    END IF;

    v_label_text := NULL;
    IF v_data_source_type = 'DICT' THEN
      IF v_field_value <> 'null'::jsonb THEN
        IF NOT (v_labels ? v_field_key) THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_REQUIRED',
            DETAIL = format('field_key=%s', v_field_key);
        END IF;

        v_label_value := v_labels->v_field_key;
        IF jsonb_typeof(v_label_value) <> 'string'
          OR NULLIF(btrim(v_label_value #>> '{}'), '') IS NULL
        THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_REQUIRED',
            DETAIL = format('field_key=%s', v_field_key);
        END IF;
        v_label_text := btrim(v_label_value #>> '{}');
      ELSIF v_labels ? v_field_key THEN
        v_label_value := v_labels->v_field_key;
        IF jsonb_typeof(v_label_value) <> 'null' THEN
          RAISE EXCEPTION USING
            MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_REQUIRED',
            DETAIL = format('field_key=%s', v_field_key);
        END IF;
      END IF;
    ELSIF v_labels ? v_field_key THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_NOT_ALLOWED',
        DETAIL = format('field_key=%s', v_field_key);
    END IF;

    v_cast_type := CASE v_value_type
      WHEN 'text' THEN 'text'
      WHEN 'int' THEN 'int'
      WHEN 'uuid' THEN 'uuid'
      WHEN 'bool' THEN 'boolean'
      WHEN 'date' THEN 'date'
      WHEN 'numeric' THEN 'numeric'
      ELSE NULL
    END;

    IF v_cast_type IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_EXT_FIELD_TYPE_MISMATCH',
        DETAIL = format('field_key=%s value_type=%s', v_field_key, v_value_type);
    END IF;

    IF v_data_source_type = 'DICT' THEN
      IF v_field_value = 'null'::jsonb THEN
        v_sql := format(
          'UPDATE orgunit.org_unit_versions
             SET %1$I = $4::%2$s,
                 ext_labels_snapshot = COALESCE(ext_labels_snapshot, ''{}''::jsonb) - $5::text,
                 last_event_id = $6::bigint
           WHERE tenant_uuid = $1::uuid
             AND org_id = $2::int
             AND lower(validity) >= $3::date',
          v_physical_col,
          v_cast_type
        );
        EXECUTE v_sql USING p_tenant_uuid, p_org_id, p_effective_date, v_value_text, v_field_key, p_event_db_id;
      ELSE
        v_sql := format(
          'UPDATE orgunit.org_unit_versions
             SET %1$I = $4::%2$s,
                 ext_labels_snapshot = COALESCE(ext_labels_snapshot, ''{}''::jsonb)
                   || jsonb_build_object($5::text, to_jsonb($6::text)),
                 last_event_id = $7::bigint
           WHERE tenant_uuid = $1::uuid
             AND org_id = $2::int
             AND lower(validity) >= $3::date',
          v_physical_col,
          v_cast_type
        );
        EXECUTE v_sql USING p_tenant_uuid, p_org_id, p_effective_date, v_value_text, v_field_key, v_label_text, p_event_db_id;
      END IF;
    ELSE
      v_sql := format(
        'UPDATE orgunit.org_unit_versions
           SET %1$I = $4::%2$s,
               last_event_id = $5::bigint
         WHERE tenant_uuid = $1::uuid
           AND org_id = $2::int
           AND lower(validity) >= $3::date',
        v_physical_col,
        v_cast_type
      );
      EXECUTE v_sql USING p_tenant_uuid, p_org_id, p_effective_date, v_value_text, p_event_db_id;
    END IF;
  END LOOP;
END;
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
      v_payload,
      v_event.id
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
    v_payload,
    v_event_db_id
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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No down migration (DEV-PLAN-162)
-- +goose StatementEnd
