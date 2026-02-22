-- +goose Up
-- +goose StatementBegin
-- DEV-PLAN-107: 扩槽后的内核/投射函数同步（numeric + 全量 ext 列拷贝）
-- Source:
--   modules/orgunit/infrastructure/persistence/schema/00016_orgunit_field_configs_schema.sql
--   modules/orgunit/infrastructure/persistence/schema/00003_orgunit_engine.sql

CREATE OR REPLACE FUNCTION orgunit.enable_tenant_field_config(
  p_tenant_uuid uuid,
  p_field_key text,
  p_value_type text,
  p_enabled_on date,
  p_data_source_type text,
  p_data_source_config jsonb,
  p_display_label text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_existing orgunit.tenant_field_config_events%ROWTYPE;
  v_config jsonb;
  v_display_label text;
  v_physical_col text;
  v_candidate_cols text[];
  v_col text;
  v_payload jsonb;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_field_key IS NULL OR btrim(p_field_key) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'field_key is required';
  END IF;
  IF p_field_key !~ '^[a-z][a-z0-9_]{0,62}$' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = format('field_key=%s', p_field_key);
  END IF;
  IF p_value_type IS NULL OR btrim(p_value_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'value_type is required';
  END IF;
  IF p_enabled_on IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'enabled_on is required';
  END IF;
  IF p_data_source_type IS NULL OR btrim(p_data_source_type) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'data_source_type is required';
  END IF;
  IF p_display_label IS NOT NULL AND btrim(p_display_label) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'display_label invalid';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:field-config-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  v_config := COALESCE(p_data_source_config, '{}'::jsonb);
  IF jsonb_typeof(v_config) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
      DETAIL = 'data_source_config must be an object';
  END IF;
  v_display_label := NULLIF(btrim(p_display_label), '');

  SELECT * INTO v_existing
  FROM orgunit.tenant_field_config_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type <> 'ENABLE'
      OR v_existing.field_key <> p_field_key
      OR v_existing.initiator_uuid <> p_initiator_uuid
      OR COALESCE(v_existing.payload->>'value_type', '') <> p_value_type
      OR COALESCE(v_existing.payload->>'data_source_type', '') <> p_data_source_type
      OR COALESCE(v_existing.payload->>'enabled_on', '') <> p_enabled_on::text
      OR COALESCE(v_existing.payload->'data_source_config', '{}'::jsonb) <> v_config
      OR COALESCE(v_existing.payload->>'display_label', '') <> COALESCE(v_display_label, '')
      OR v_existing.payload->>'disabled_on' IS NOT NULL
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;

    RETURN;
  END IF;

  IF p_value_type NOT IN ('text','int','uuid','bool','date','numeric') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = format('value_type=%s', p_value_type);
  END IF;
  IF p_data_source_type NOT IN ('PLAIN','DICT','ENTITY') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = format('data_source_type=%s', p_data_source_type);
  END IF;

  IF p_data_source_type = 'PLAIN' THEN
    IF v_config <> '{}'::jsonb THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
        DETAIL = format('field_key=%s data_source_type=PLAIN requires {}', p_field_key);
    END IF;
  ELSIF p_data_source_type = 'DICT' THEN
    IF p_value_type <> 'text'
      OR NOT (v_config ? 'dict_code')
      OR jsonb_typeof(v_config->'dict_code') <> 'string'
      OR NULLIF(btrim(v_config->>'dict_code'), '') IS NULL
      OR v_config <> jsonb_build_object('dict_code', v_config->'dict_code')
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
        DETAIL = format('field_key=%s data_source_type=DICT', p_field_key);
    END IF;
  ELSIF p_data_source_type = 'ENTITY' THEN
    IF NOT (v_config ? 'entity')
      OR jsonb_typeof(v_config->'entity') <> 'string'
      OR NULLIF(btrim(v_config->>'entity'), '') IS NULL
      OR NOT (v_config ? 'id_kind')
      OR jsonb_typeof(v_config->'id_kind') <> 'string'
      OR (v_config->>'id_kind') NOT IN ('uuid','int')
      OR (
        ((v_config->>'id_kind') = 'uuid' AND p_value_type <> 'uuid')
        OR
        ((v_config->>'id_kind') = 'int' AND p_value_type <> 'int')
      )
      OR v_config <> jsonb_build_object(
        'entity', v_config->'entity',
        'id_kind', v_config->'id_kind'
      )
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
        DETAIL = format('field_key=%s data_source_type=ENTITY', p_field_key);
    END IF;
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.tenant_field_configs
    WHERE tenant_uuid = p_tenant_uuid
      AND field_key = p_field_key
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_ALREADY_ENABLED',
      DETAIL = format('field_key=%s', p_field_key);
  END IF;

  IF p_value_type = 'text' THEN
    v_candidate_cols := ARRAY[
      'ext_str_01','ext_str_02','ext_str_03','ext_str_04','ext_str_05',
      'ext_str_06','ext_str_07','ext_str_08','ext_str_09','ext_str_10',
      'ext_str_11','ext_str_12','ext_str_13','ext_str_14','ext_str_15',
      'ext_str_16','ext_str_17','ext_str_18','ext_str_19','ext_str_20',
      'ext_str_21','ext_str_22','ext_str_23','ext_str_24','ext_str_25',
      'ext_str_26','ext_str_27','ext_str_28','ext_str_29','ext_str_30',
      'ext_str_31','ext_str_32','ext_str_33','ext_str_34','ext_str_35',
      'ext_str_36','ext_str_37','ext_str_38','ext_str_39','ext_str_40',
      'ext_str_41','ext_str_42','ext_str_43','ext_str_44','ext_str_45',
      'ext_str_46','ext_str_47','ext_str_48','ext_str_49','ext_str_50',
      'ext_str_51','ext_str_52','ext_str_53','ext_str_54','ext_str_55',
      'ext_str_56','ext_str_57','ext_str_58','ext_str_59','ext_str_60',
      'ext_str_61','ext_str_62','ext_str_63','ext_str_64','ext_str_65',
      'ext_str_66','ext_str_67','ext_str_68','ext_str_69','ext_str_70'
    ];
  ELSIF p_value_type = 'int' THEN
    v_candidate_cols := ARRAY[
      'ext_int_01','ext_int_02','ext_int_03','ext_int_04','ext_int_05',
      'ext_int_06','ext_int_07','ext_int_08','ext_int_09','ext_int_10',
      'ext_int_11','ext_int_12','ext_int_13','ext_int_14','ext_int_15'
    ];
  ELSIF p_value_type = 'uuid' THEN
    v_candidate_cols := ARRAY[
      'ext_uuid_01','ext_uuid_02','ext_uuid_03','ext_uuid_04','ext_uuid_05',
      'ext_uuid_06','ext_uuid_07','ext_uuid_08','ext_uuid_09','ext_uuid_10'
    ];
  ELSIF p_value_type = 'bool' THEN
    v_candidate_cols := ARRAY[
      'ext_bool_01','ext_bool_02','ext_bool_03','ext_bool_04','ext_bool_05',
      'ext_bool_06','ext_bool_07','ext_bool_08','ext_bool_09','ext_bool_10',
      'ext_bool_11','ext_bool_12','ext_bool_13','ext_bool_14','ext_bool_15'
    ];
  ELSIF p_value_type = 'date' THEN
    v_candidate_cols := ARRAY[
      'ext_date_01','ext_date_02','ext_date_03','ext_date_04','ext_date_05',
      'ext_date_06','ext_date_07','ext_date_08','ext_date_09','ext_date_10',
      'ext_date_11','ext_date_12','ext_date_13','ext_date_14','ext_date_15'
    ];
  ELSIF p_value_type = 'numeric' THEN
    v_candidate_cols := ARRAY[
      'ext_num_01','ext_num_02','ext_num_03','ext_num_04','ext_num_05',
      'ext_num_06','ext_num_07','ext_num_08','ext_num_09','ext_num_10'
    ];
  ELSE
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = format('value_type=%s', p_value_type);
  END IF;

  v_physical_col := NULL;
  FOREACH v_col IN ARRAY v_candidate_cols LOOP
    IF NOT EXISTS (
      SELECT 1
      FROM orgunit.tenant_field_configs
      WHERE tenant_uuid = p_tenant_uuid
        AND physical_col = v_col
    ) THEN
      v_physical_col := v_col;
      EXIT;
    END IF;
  END LOOP;

  IF v_physical_col IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_SLOT_EXHAUSTED',
      DETAIL = format('field_key=%s value_type=%s', p_field_key, p_value_type);
  END IF;

  v_payload := jsonb_build_object(
    'physical_col', v_physical_col,
    'value_type', p_value_type,
    'data_source_type', p_data_source_type,
    'data_source_config', v_config,
    'display_label', v_display_label,
    'enabled_on', p_enabled_on::text,
    'disabled_on', NULL
  );

  INSERT INTO orgunit.tenant_field_config_events (
    event_uuid,
    tenant_uuid,
    event_type,
    field_key,
    payload,
    request_id,
    initiator_uuid
  )
  VALUES (
    gen_random_uuid(),
    p_tenant_uuid,
    'ENABLE',
    p_field_key,
    v_payload,
    p_request_id,
    p_initiator_uuid
  );

  INSERT INTO orgunit.tenant_field_configs (
    tenant_uuid,
    field_key,
    physical_col,
    value_type,
    data_source_type,
    data_source_config,
    display_label,
    enabled_on,
    disabled_on,
    created_at,
    updated_at
  )
  VALUES (
    p_tenant_uuid,
    p_field_key,
    v_physical_col,
    p_value_type,
    p_data_source_type,
    v_config,
    v_display_label,
    p_enabled_on,
    NULL,
    now(),
    now()
  );
END;
$$;

-- Important: CREATE OR REPLACE resets function attributes (e.g. SECURITY DEFINER).
-- This function is a kernel write entrypoint and must bypass table write guards.
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

CREATE OR REPLACE FUNCTION orgunit.apply_org_event_ext_payload(
  p_tenant_uuid uuid,
  p_org_id int,
  p_effective_date date,
  p_event_type text,
  p_payload jsonb,
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
                 ext_labels_snapshot = COALESCE(ext_labels_snapshot, ''{}''::jsonb) - $5::text
           WHERE tenant_uuid = $1::uuid
             AND org_id = $2::int
             AND lower(validity) >= $3::date',
          v_physical_col,
          v_cast_type
        );
        EXECUTE v_sql USING p_tenant_uuid, p_org_id, p_effective_date, v_value_text, v_field_key;
      ELSE
        v_sql := format(
          'UPDATE orgunit.org_unit_versions
             SET %1$I = $4::%2$s,
                 ext_labels_snapshot = COALESCE(ext_labels_snapshot, ''{}''::jsonb)
                   || jsonb_build_object($5::text, to_jsonb($6::text))
           WHERE tenant_uuid = $1::uuid
             AND org_id = $2::int
             AND lower(validity) >= $3::date',
          v_physical_col,
          v_cast_type
        );
        EXECUTE v_sql USING p_tenant_uuid, p_org_id, p_effective_date, v_value_text, v_field_key, v_label_text;
      END IF;
    ELSE
      v_sql := format(
        'UPDATE orgunit.org_unit_versions
           SET %1$I = $4::%2$s
         WHERE tenant_uuid = $1::uuid
           AND org_id = $2::int
           AND lower(validity) >= $3::date',
        v_physical_col,
        v_cast_type
      );
      EXECUTE v_sql USING p_tenant_uuid, p_org_id, p_effective_date, v_value_text;
    END IF;
  END LOOP;
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
    v_row.ext_str_01,
    v_row.ext_str_02,
    v_row.ext_str_03,
    v_row.ext_str_04,
    v_row.ext_str_05,
    v_row.ext_str_06,
    v_row.ext_str_07,
    v_row.ext_str_08,
    v_row.ext_str_09,
    v_row.ext_str_10,
    v_row.ext_str_11,
    v_row.ext_str_12,
    v_row.ext_str_13,
    v_row.ext_str_14,
    v_row.ext_str_15,
    v_row.ext_str_16,
    v_row.ext_str_17,
    v_row.ext_str_18,
    v_row.ext_str_19,
    v_row.ext_str_20,
    v_row.ext_str_21,
    v_row.ext_str_22,
    v_row.ext_str_23,
    v_row.ext_str_24,
    v_row.ext_str_25,
    v_row.ext_str_26,
    v_row.ext_str_27,
    v_row.ext_str_28,
    v_row.ext_str_29,
    v_row.ext_str_30,
    v_row.ext_str_31,
    v_row.ext_str_32,
    v_row.ext_str_33,
    v_row.ext_str_34,
    v_row.ext_str_35,
    v_row.ext_str_36,
    v_row.ext_str_37,
    v_row.ext_str_38,
    v_row.ext_str_39,
    v_row.ext_str_40,
    v_row.ext_str_41,
    v_row.ext_str_42,
    v_row.ext_str_43,
    v_row.ext_str_44,
    v_row.ext_str_45,
    v_row.ext_str_46,
    v_row.ext_str_47,
    v_row.ext_str_48,
    v_row.ext_str_49,
    v_row.ext_str_50,
    v_row.ext_str_51,
    v_row.ext_str_52,
    v_row.ext_str_53,
    v_row.ext_str_54,
    v_row.ext_str_55,
    v_row.ext_str_56,
    v_row.ext_str_57,
    v_row.ext_str_58,
    v_row.ext_str_59,
    v_row.ext_str_60,
    v_row.ext_str_61,
    v_row.ext_str_62,
    v_row.ext_str_63,
    v_row.ext_str_64,
    v_row.ext_str_65,
    v_row.ext_str_66,
    v_row.ext_str_67,
    v_row.ext_str_68,
    v_row.ext_str_69,
    v_row.ext_str_70,
    v_row.ext_int_01,
    v_row.ext_int_02,
    v_row.ext_int_03,
    v_row.ext_int_04,
    v_row.ext_int_05,
    v_row.ext_int_06,
    v_row.ext_int_07,
    v_row.ext_int_08,
    v_row.ext_int_09,
    v_row.ext_int_10,
    v_row.ext_int_11,
    v_row.ext_int_12,
    v_row.ext_int_13,
    v_row.ext_int_14,
    v_row.ext_int_15,
    v_row.ext_uuid_01,
    v_row.ext_uuid_02,
    v_row.ext_uuid_03,
    v_row.ext_uuid_04,
    v_row.ext_uuid_05,
    v_row.ext_uuid_06,
    v_row.ext_uuid_07,
    v_row.ext_uuid_08,
    v_row.ext_uuid_09,
    v_row.ext_uuid_10,
    v_row.ext_bool_01,
    v_row.ext_bool_02,
    v_row.ext_bool_03,
    v_row.ext_bool_04,
    v_row.ext_bool_05,
    v_row.ext_bool_06,
    v_row.ext_bool_07,
    v_row.ext_bool_08,
    v_row.ext_bool_09,
    v_row.ext_bool_10,
    v_row.ext_bool_11,
    v_row.ext_bool_12,
    v_row.ext_bool_13,
    v_row.ext_bool_14,
    v_row.ext_bool_15,
    v_row.ext_date_01,
    v_row.ext_date_02,
    v_row.ext_date_03,
    v_row.ext_date_04,
    v_row.ext_date_05,
    v_row.ext_date_06,
    v_row.ext_date_07,
    v_row.ext_date_08,
    v_row.ext_date_09,
    v_row.ext_date_10,
    v_row.ext_date_11,
    v_row.ext_date_12,
    v_row.ext_date_13,
    v_row.ext_date_14,
    v_row.ext_date_15,
    v_row.ext_num_01,
    v_row.ext_num_02,
    v_row.ext_num_03,
    v_row.ext_num_04,
    v_row.ext_num_05,
    v_row.ext_num_06,
    v_row.ext_num_07,
    v_row.ext_num_08,
    v_row.ext_num_09,
    v_row.ext_num_10,
    v_row.ext_labels_snapshot,
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

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOTE: 函数演进迁移，Down 维持 no-op（避免误回滚造成链路断裂）。
SELECT 1;
-- +goose StatementEnd
