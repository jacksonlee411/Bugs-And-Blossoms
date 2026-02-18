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

CREATE OR REPLACE FUNCTION orgunit.fill_org_event_audit_snapshot()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v_name text;
  v_employee text;
BEGIN
  IF NEW.tx_time IS NULL THEN
    NEW.tx_time := COALESCE(NEW.transaction_time, now());
  END IF;

  IF NULLIF(btrim(COALESCE(NEW.initiator_name, '')), '') IS NOT NULL
    AND NULLIF(btrim(COALESCE(NEW.initiator_employee_id, '')), '') IS NOT NULL
  THEN
    RETURN NEW;
  END IF;

  IF to_regclass('iam.principals') IS NOT NULL THEN
    SELECT
      COALESCE(NULLIF(btrim(p.display_name), ''), NULLIF(btrim(p.email), ''), NEW.initiator_uuid::text),
      COALESCE(NULLIF(btrim(p.email), ''), NEW.initiator_uuid::text)
    INTO v_name, v_employee
    FROM iam.principals p
    WHERE p.tenant_uuid = NEW.tenant_uuid
      AND p.id = NEW.initiator_uuid
    LIMIT 1;
  END IF;

  NEW.initiator_name := COALESCE(NULLIF(btrim(COALESCE(NEW.initiator_name, '')), ''), v_name, NEW.initiator_uuid::text);
  NEW.initiator_employee_id := COALESCE(NULLIF(btrim(COALESCE(NEW.initiator_employee_id, '')), ''), v_employee, NEW.initiator_uuid::text);

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS org_events_fill_audit_snapshot ON orgunit.org_events;
CREATE TRIGGER org_events_fill_audit_snapshot
BEFORE INSERT ON orgunit.org_events
FOR EACH ROW
EXECUTE FUNCTION orgunit.fill_org_event_audit_snapshot();

CREATE OR REPLACE FUNCTION orgunit.merge_org_event_payload_with_correction(
  p_base_payload jsonb,
  p_correction_payload jsonb
)
RETURNS jsonb
LANGUAGE sql
IMMUTABLE
AS $$
  WITH base_payload AS (
    SELECT COALESCE(p_base_payload, '{}'::jsonb) AS payload
  ),
  correction_patch AS (
    SELECT COALESCE(p_correction_payload, '{}'::jsonb) - 'effective_date' - 'target_event_uuid' - 'op' AS payload
  ),
  parts AS (
    SELECT
      (b.payload - 'ext' - 'ext_labels_snapshot') AS base_noext,
      (c.payload - 'ext' - 'ext_labels_snapshot') AS patch_noext,
      CASE WHEN jsonb_typeof(b.payload->'ext') = 'object' THEN b.payload->'ext' ELSE '{}'::jsonb END AS base_ext,
      CASE WHEN jsonb_typeof(c.payload->'ext') = 'object' THEN c.payload->'ext' ELSE '{}'::jsonb END AS patch_ext,
      CASE WHEN jsonb_typeof(b.payload->'ext_labels_snapshot') = 'object' THEN b.payload->'ext_labels_snapshot' ELSE '{}'::jsonb END AS base_labels,
      CASE WHEN jsonb_typeof(c.payload->'ext_labels_snapshot') = 'object' THEN c.payload->'ext_labels_snapshot' ELSE '{}'::jsonb END AS patch_labels,
      (b.payload ? 'ext') OR (c.payload ? 'ext') AS has_ext,
      (b.payload ? 'ext_labels_snapshot') OR (c.payload ? 'ext_labels_snapshot') AS has_labels
    FROM base_payload b
    CROSS JOIN correction_patch c
  ),
  merged AS (
    SELECT
      (base_noext || patch_noext) AS merged_noext,
      (base_ext || patch_ext) AS ext_merged,
      (base_labels || patch_labels) AS labels_merged,
      has_ext,
      has_labels
    FROM parts
  )
  SELECT
    merged_noext
    || CASE
         WHEN has_ext
           THEN jsonb_build_object('ext', ext_merged)
         ELSE '{}'::jsonb
       END
    || CASE
         WHEN has_labels
           THEN jsonb_build_object(
             'ext_labels_snapshot',
             labels_merged - COALESCE((
               SELECT array_agg(key)
               FROM jsonb_each(ext_merged)
               WHERE value = 'null'::jsonb
             ), ARRAY[]::text[])
           )
         ELSE '{}'::jsonb
       END
  FROM merged;
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
  e.request_code,
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
  FROM orgunit.org_events_effective e
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

CREATE OR REPLACE FUNCTION orgunit.apply_enable_logic(
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
  SET status = 'active', last_event_id = p_event_db_id
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

CREATE OR REPLACE FUNCTION orgunit.rebuild_full_name_path_subtree(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.assert_org_unit_validity(
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
$$;

CREATE OR REPLACE FUNCTION orgunit.org_events_effective_for_replay(
  p_tenant_uuid uuid,
  p_org_id int,
  p_pending_event_id bigint,
  p_pending_event_uuid uuid,
  p_pending_event_type text,
  p_pending_effective_date date,
  p_pending_payload jsonb,
  p_pending_request_code text,
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
  request_code text,
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
      e.request_code,
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
      p_pending_request_code,
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
        AND se.event_type <> 'CREATE'
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
    se.request_code,
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
  p_pending_request_code text,
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
      OR p_pending_request_code IS NULL
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
      p_pending_request_code,
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
      p_pending_request_code,
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

CREATE OR REPLACE FUNCTION orgunit.rebuild_org_unit_versions_for_org(
  p_tenant_uuid uuid,
  p_org_id int
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.extract_orgunit_snapshot(
  p_tenant_uuid uuid,
  p_org_id int,
  p_as_of date
)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
  v_row orgunit.org_unit_versions%ROWTYPE;
  v_snapshot jsonb;
  v_org_code text;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_as_of IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'as_of is required';
  END IF;

  SELECT * INTO v_row
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
    AND v.validity @> p_as_of
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF NOT FOUND THEN
    RETURN NULL;
  END IF;

  SELECT c.org_code INTO v_org_code
  FROM orgunit.org_unit_codes c
  WHERE c.tenant_uuid = p_tenant_uuid
    AND c.org_id = p_org_id
  LIMIT 1;

  v_snapshot := to_jsonb(v_row) - 'id' - 'tenant_uuid' - 'last_event_id' - 'path_ids';
  IF v_org_code IS NOT NULL THEN
    v_snapshot := jsonb_set(v_snapshot, '{org_code}', to_jsonb(v_org_code), true);
  END IF;

  IF jsonb_typeof(v_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('org_id=%s as_of=%s snapshot_type=%s', p_org_id, p_as_of, COALESCE(jsonb_typeof(v_snapshot), 'null'));
  END IF;

  RETURN v_snapshot;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.assert_org_event_snapshots(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb,
  p_rescind_outcome text
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_before_snapshot IS NOT NULL AND jsonb_typeof(p_before_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('event_type=%s before_snapshot_type=%s', p_event_type, jsonb_typeof(p_before_snapshot));
  END IF;

  IF p_after_snapshot IS NOT NULL AND jsonb_typeof(p_after_snapshot) <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format('event_type=%s after_snapshot_type=%s', p_event_type, jsonb_typeof(p_after_snapshot));
  END IF;

  IF NOT orgunit.is_org_event_snapshot_presence_valid(
    p_event_type,
    p_before_snapshot,
    p_after_snapshot,
    p_rescind_outcome
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_MISSING',
      DETAIL = format(
        'event_type=%s before=%s after=%s rescind_outcome=%s',
        p_event_type,
        p_before_snapshot IS NOT NULL,
        p_after_snapshot IS NOT NULL,
        COALESCE(p_rescind_outcome, 'NULL')
      );
  END IF;

  IF NOT orgunit.is_org_event_snapshot_content_valid(
    p_event_type,
    p_before_snapshot,
    p_after_snapshot,
    p_rescind_outcome
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_AUDIT_SNAPSHOT_INVALID',
      DETAIL = format(
        'event_type=%s incomplete_snapshot_content=true rescind_outcome=%s',
        p_event_type,
        COALESCE(p_rescind_outcome, 'NULL')
      );
  END IF;
END;
$$;

-- Backward-compatible wrapper for existing 3-arg callers during rollout.
CREATE OR REPLACE FUNCTION orgunit.assert_org_event_snapshots(
  p_event_type text,
  p_before_snapshot jsonb,
  p_after_snapshot jsonb
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM orgunit.assert_org_event_snapshots(
    p_event_type,
    p_before_snapshot,
    p_after_snapshot,
    NULL
  );
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
  v_existing_request orgunit.org_events%ROWTYPE;
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
  v_status text;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
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

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_uuid <> p_event_uuid
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.event_type <> p_event_type
      OR v_existing_request.effective_date <> p_effective_date
      OR v_existing_request.payload <> v_payload
      OR v_existing_request.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_code);
    END IF;

    RETURN v_existing_request.id;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_effective_date);

  SELECT * INTO v_existing
  FROM orgunit.org_events
  WHERE event_uuid = p_event_uuid;

  IF FOUND THEN
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
    PERFORM orgunit.apply_create_logic(p_tenant_uuid, p_org_id, v_org_code, v_parent_id, p_effective_date, v_name, v_manager_uuid, v_is_business_unit, v_event_db_id, v_status);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[p_org_id];
  ELSIF p_event_type = 'UPDATE' THEN
    PERFORM orgunit.apply_update_logic(p_tenant_uuid, p_org_id, p_effective_date, v_payload, v_event_db_id);
    IF (v_payload ? 'parent_id')
      OR (v_payload ? 'new_parent_id')
      OR (v_payload ? 'name')
      OR (v_payload ? 'new_name')
    THEN
      SELECT v.node_path INTO v_root_path
      FROM orgunit.org_unit_versions v
      WHERE v.tenant_uuid = p_tenant_uuid
        AND v.org_id = p_org_id
        AND v.validity @> p_effective_date
      ORDER BY lower(v.validity) DESC
      LIMIT 1;
      IF v_root_path IS NOT NULL THEN
        PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
      END IF;
    END IF;
    SELECT array_agg(DISTINCT v.org_id) INTO v_org_ids
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id;
  ELSIF p_event_type = 'MOVE' THEN
    v_new_parent_id := NULLIF(v_payload->>'new_parent_id', '')::int;
    PERFORM orgunit.apply_move_logic(p_tenant_uuid, p_org_id, v_new_parent_id, p_effective_date, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
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
    PERFORM orgunit.apply_rename_logic(p_tenant_uuid, p_org_id, p_effective_date, v_new_name, v_event_db_id);
    SELECT v.node_path INTO v_root_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
      AND v.validity @> p_effective_date
    ORDER BY lower(v.validity) DESC
    LIMIT 1;
    PERFORM orgunit.rebuild_full_name_path_subtree(p_tenant_uuid, v_root_path, p_effective_date);
    v_org_ids := ARRAY[p_org_id];
  ELSIF p_event_type = 'DISABLE' THEN
    PERFORM orgunit.apply_disable_logic(p_tenant_uuid, p_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[p_org_id];
  ELSIF p_event_type = 'ENABLE' THEN
    PERFORM orgunit.apply_enable_logic(p_tenant_uuid, p_org_id, p_effective_date, v_event_db_id);
    v_org_ids := ARRAY[p_org_id];
  ELSIF p_event_type = 'SET_BUSINESS_UNIT' THEN
    v_is_business_unit := (v_payload->>'is_business_unit')::boolean;
    PERFORM orgunit.apply_set_business_unit_logic(p_tenant_uuid, p_org_id, p_effective_date, v_is_business_unit, v_event_db_id);
    v_org_ids := ARRAY[p_org_id];
  END IF;

  PERFORM orgunit.assert_org_unit_validity(p_tenant_uuid, v_org_ids);

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_effective_date);
  PERFORM orgunit.assert_org_event_snapshots(p_event_type, v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    event_uuid,
    tenant_uuid,
    org_id,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid,
    before_snapshot,
    after_snapshot
  )
  VALUES (
    v_event_db_id,
    p_event_uuid,
    p_tenant_uuid,
    p_org_id,
    p_event_type,
    p_effective_date,
    v_payload,
    p_request_code,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot
  );

  RETURN v_event_db_id;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_rescind(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_reason text;
  v_event_uuid uuid;
  v_payload jsonb;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_event_db_id bigint;
  v_rescind_outcome text;
  v_pending_time timestamptz;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  v_reason := btrim(COALESCE(p_reason, ''));
  IF v_reason = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'reason is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_type <> 'RESCIND_EVENT'
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.effective_date <> p_target_effective_date
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_id);
    END IF;

    RETURN v_existing_request.event_uuid;
  END IF;

  SELECT e.* INTO v_target
  FROM orgunit.org_events e
  JOIN orgunit.org_events_effective ee
    ON ee.event_uuid = e.event_uuid
   AND ee.tenant_uuid = e.tenant_uuid
   AND ee.org_id = e.org_id
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  SELECT * INTO v_existing_rescind
  FROM orgunit.org_events r
  WHERE r.tenant_uuid = p_tenant_uuid
    AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
    AND r.payload->>'target_event_uuid' = v_target.event_uuid::text
  LIMIT 1;

  IF FOUND THEN
    RETURN v_existing_rescind.event_uuid;
  END IF;

  v_payload := jsonb_build_object(
    'op', 'RESCIND_EVENT',
    'reason', v_reason,
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  );

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_event_uuid := gen_random_uuid();
  v_pending_time := now();
  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    v_event_db_id,
    v_event_uuid,
    'RESCIND_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_rescind_outcome := CASE WHEN v_after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END;
  PERFORM orgunit.assert_org_event_snapshots('RESCIND_EVENT', v_before_snapshot, v_after_snapshot, v_rescind_outcome);

  INSERT INTO orgunit.org_events (
    id,
    tenant_uuid,
    org_id,
    event_uuid,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid,
    reason,
    before_snapshot,
    after_snapshot,
    rescind_outcome,
    tx_time,
    transaction_time,
    created_at
  )
  VALUES (
    v_event_db_id,
    p_tenant_uuid,
    p_org_id,
    v_event_uuid,
    'RESCIND_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_reason,
    v_before_snapshot,
    v_after_snapshot,
    v_rescind_outcome,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  RETURN v_event_uuid;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_rescind(
  p_tenant_uuid uuid,
  p_org_id int,
  p_reason text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS int
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_reason text;
  v_root_org_id int;
  v_node_path ltree;
  v_event_count int;
  v_existing_batch_count int;
  v_need_apply boolean;
  v_request_id_seq text;
  v_payload jsonb;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_rescind_event_uuid uuid;
  v_rescind_event_id bigint;
  v_rescind_outcome text;
  v_pending_time timestamptz;
  rec record;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  v_reason := btrim(COALESCE(p_reason, ''));
  IF v_reason = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'reason is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_id
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
      DETAIL = format('request_code=%s', p_request_id);
  END IF;

  SELECT t.root_org_id INTO v_root_org_id
  FROM orgunit.org_trees t
  WHERE t.tenant_uuid = p_tenant_uuid
  LIMIT 1;

  IF v_root_org_id = p_org_id THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_ROOT_DELETE_FORBIDDEN',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT v.node_path INTO v_node_path
  FROM orgunit.org_unit_versions v
  WHERE v.tenant_uuid = p_tenant_uuid
    AND v.org_id = p_org_id
  ORDER BY lower(v.validity) DESC
  LIMIT 1;

  IF v_node_path IS NOT NULL AND EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions c
    WHERE c.tenant_uuid = p_tenant_uuid
      AND c.node_path <@ v_node_path
      AND c.org_id <> p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_HAS_CHILDREN_CANNOT_DELETE',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.setid_binding_versions b
    WHERE b.tenant_uuid = p_tenant_uuid
      AND b.org_id = p_org_id
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_HAS_DEPENDENCIES_CANNOT_DELETE',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT COUNT(*) INTO v_event_count
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id;

  IF v_event_count = 0 THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s', p_org_id);
  END IF;

  SELECT COUNT(*) INTO v_existing_batch_count
  FROM orgunit.org_events e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.request_code LIKE p_request_id || '#%';

  IF v_existing_batch_count > 0 AND v_existing_batch_count <> v_event_count THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
      DETAIL = format('request_code=%s', p_request_id);
  END IF;

  v_need_apply := false;

  FOR rec IN
    SELECT
      row_number() OVER (ORDER BY e.effective_date, e.id) AS seq,
      e.event_uuid,
      COALESCE(ee.effective_date, e.effective_date) AS target_effective_date
    FROM orgunit.org_events e
    LEFT JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
    ORDER BY e.effective_date, e.id
  LOOP
    v_request_id_seq := format('%s#%s', p_request_id, lpad(rec.seq::text, 4, '0'));
    SELECT * INTO v_existing_request
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.request_code = v_request_id_seq
    LIMIT 1;

    IF FOUND THEN
      IF v_existing_request.event_type <> 'RESCIND_ORG'
        OR v_existing_request.org_id <> p_org_id
        OR COALESCE(v_existing_request.payload->>'target_event_uuid', '') <> rec.event_uuid::text
      THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
          DETAIL = format('request_code=%s', p_request_id);
      END IF;
      CONTINUE;
    END IF;

    v_need_apply := true;
  END LOOP;

  IF NOT v_need_apply THEN
    RETURN v_event_count;
  END IF;

  FOR rec IN
    SELECT
      row_number() OVER (ORDER BY e.effective_date, e.id) AS seq,
      e.event_uuid,
      COALESCE(ee.effective_date, e.effective_date) AS target_effective_date
    FROM orgunit.org_events e
    LEFT JOIN orgunit.org_events_effective ee
      ON ee.event_uuid = e.event_uuid
     AND ee.tenant_uuid = e.tenant_uuid
     AND ee.org_id = e.org_id
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
    ORDER BY e.effective_date, e.id
  LOOP
    v_request_id_seq := format('%s#%s', p_request_id, lpad(rec.seq::text, 4, '0'));
    SELECT * INTO v_existing_request
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.request_code = v_request_id_seq
    LIMIT 1;

    IF FOUND THEN
      CONTINUE;
    END IF;

    SELECT * INTO v_existing_rescind
    FROM orgunit.org_events r
    WHERE r.tenant_uuid = p_tenant_uuid
      AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
      AND r.payload->>'target_event_uuid' = rec.event_uuid::text
    LIMIT 1;

    IF FOUND THEN
      CONTINUE;
    END IF;

    v_payload := jsonb_build_object(
      'op', 'RESCIND_ORG',
      'reason', v_reason,
      'batch_request_id', p_request_id,
      'target_event_uuid', rec.event_uuid,
      'target_effective_date', rec.target_effective_date
    );

    v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, rec.target_effective_date);
    v_rescind_event_uuid := gen_random_uuid();
    v_pending_time := now();
    SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_rescind_event_id;

    PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
      p_tenant_uuid,
      p_org_id,
      v_rescind_event_id,
      v_rescind_event_uuid,
      'RESCIND_ORG',
      rec.target_effective_date,
      v_payload,
      v_request_id_seq,
      p_initiator_uuid,
      v_pending_time,
      v_pending_time,
      v_pending_time
    );

    v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, rec.target_effective_date);
    v_rescind_outcome := CASE WHEN v_after_snapshot IS NULL THEN 'ABSENT' ELSE 'PRESENT' END;
    PERFORM orgunit.assert_org_event_snapshots('RESCIND_ORG', v_before_snapshot, v_after_snapshot, v_rescind_outcome);

    INSERT INTO orgunit.org_events (
      id,
      tenant_uuid,
      org_id,
      event_uuid,
      event_type,
      effective_date,
      payload,
      request_code,
      initiator_uuid,
      reason,
      before_snapshot,
      after_snapshot,
      rescind_outcome,
      tx_time,
      transaction_time,
      created_at
    )
    VALUES (
      v_rescind_event_id,
      p_tenant_uuid,
      p_org_id,
      v_rescind_event_uuid,
      'RESCIND_ORG',
      rec.target_effective_date,
      v_payload,
      v_request_id_seq,
      p_initiator_uuid,
      v_reason,
      v_before_snapshot,
      v_after_snapshot,
      v_rescind_outcome,
      v_pending_time,
      v_pending_time,
      v_pending_time
    );
  END LOOP;

  RETURN v_event_count;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_event_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_patch jsonb,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_patch jsonb;
  v_payload jsonb;
  v_effective_payload jsonb;
  v_target_effective date;
  v_prev_effective date;
  v_new_effective date;
  v_next_effective date;
  v_parent_id int;
  v_target_path ltree;
  v_descendant_min_create date;
  v_event_uuid uuid;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_event_db_id bigint;
  v_pending_time timestamptz;
  v_base_ext jsonb;
  v_patch_ext jsonb;
  v_ext_merged jsonb;
  v_base_labels jsonb;
  v_patch_labels jsonb;
  v_labels_merged jsonb;
  v_label_key text;
  v_label_remove_keys text[];
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_patch IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_patch := p_patch;
  IF jsonb_typeof(v_patch) <> 'object' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'patch must be an object';
  END IF;

  IF v_patch ? 'ext' AND jsonb_typeof(v_patch->'ext') <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EXT_PAYLOAD_INVALID_SHAPE',
      DETAIL = 'patch.ext must be an object';
  END IF;
  IF v_patch ? 'ext_labels_snapshot' AND jsonb_typeof(v_patch->'ext_labels_snapshot') <> 'object' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EXT_PAYLOAD_INVALID_SHAPE',
      DETAIL = 'patch.ext_labels_snapshot must be an object';
  END IF;
  IF v_patch ? 'ext_labels_snapshot' THEN
    FOR v_label_key IN
      SELECT key
      FROM jsonb_object_keys(v_patch->'ext_labels_snapshot') AS t(key)
    LOOP
      IF NOT (COALESCE(v_patch->'ext', '{}'::jsonb) ? v_label_key) THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_EXT_LABEL_SNAPSHOT_NOT_ALLOWED',
          DETAIL = format('field_key=%s', v_label_key);
      END IF;
    END LOOP;
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT e.* INTO v_target
  FROM orgunit.org_events e
  JOIN orgunit.org_events_effective ee
    ON ee.event_uuid = e.event_uuid
   AND ee.tenant_uuid = e.tenant_uuid
   AND ee.org_id = e.org_id
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  SELECT * INTO v_existing_rescind
  FROM orgunit.org_events r
  WHERE r.tenant_uuid = p_tenant_uuid
    AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
    AND r.payload->>'target_event_uuid' = v_target.event_uuid::text
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', v_target.event_uuid);
  END IF;

  SELECT ee.effective_date, ee.payload
  INTO v_target_effective, v_effective_payload
  FROM orgunit.org_events_effective ee
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.event_uuid = v_target.event_uuid
  LIMIT 1;

  v_payload := (COALESCE(v_effective_payload, '{}'::jsonb) - 'ext' - 'ext_labels_snapshot')
    || (v_patch - 'ext' - 'ext_labels_snapshot');

  v_base_ext := CASE
    WHEN jsonb_typeof(COALESCE(v_effective_payload, '{}'::jsonb)->'ext') = 'object'
      THEN COALESCE(v_effective_payload, '{}'::jsonb)->'ext'
    ELSE '{}'::jsonb
  END;
  v_patch_ext := CASE
    WHEN jsonb_typeof(v_patch->'ext') = 'object'
      THEN v_patch->'ext'
    ELSE '{}'::jsonb
  END;
  v_ext_merged := v_base_ext || v_patch_ext;

  v_base_labels := CASE
    WHEN jsonb_typeof(COALESCE(v_effective_payload, '{}'::jsonb)->'ext_labels_snapshot') = 'object'
      THEN COALESCE(v_effective_payload, '{}'::jsonb)->'ext_labels_snapshot'
    ELSE '{}'::jsonb
  END;
  v_patch_labels := CASE
    WHEN jsonb_typeof(v_patch->'ext_labels_snapshot') = 'object'
      THEN v_patch->'ext_labels_snapshot'
    ELSE '{}'::jsonb
  END;
  v_labels_merged := v_base_labels || v_patch_labels;

  SELECT array_agg(key) INTO v_label_remove_keys
  FROM jsonb_each(v_ext_merged)
  WHERE value = 'null'::jsonb;
  v_label_remove_keys := COALESCE(v_label_remove_keys, ARRAY[]::text[]);
  v_labels_merged := v_labels_merged - v_label_remove_keys;

  IF COALESCE(v_effective_payload, '{}'::jsonb) ? 'ext' OR v_patch ? 'ext' THEN
    v_payload := v_payload || jsonb_build_object('ext', v_ext_merged);
  END IF;
  IF COALESCE(v_effective_payload, '{}'::jsonb) ? 'ext_labels_snapshot' OR v_patch ? 'ext_labels_snapshot' THEN
    v_payload := v_payload || jsonb_build_object('ext_labels_snapshot', v_labels_merged);
  END IF;
  v_new_effective := v_target_effective;

  IF v_payload ? 'effective_date' THEN
    BEGIN
      v_new_effective := NULLIF(btrim(v_payload->>'effective_date'), '')::date;
    EXCEPTION
      WHEN invalid_text_representation THEN
        RAISE EXCEPTION USING
          MESSAGE = 'EFFECTIVE_DATE_INVALID',
          DETAIL = format('effective_date=%s', v_payload->>'effective_date');
    END;
    IF v_new_effective IS NULL THEN
      RAISE EXCEPTION USING
        MESSAGE = 'EFFECTIVE_DATE_INVALID',
        DETAIL = 'effective_date is required';
    END IF;
    -- keep effective_date in payload for audit and correction view
  END IF;

  SELECT MAX(e.effective_date) INTO v_prev_effective
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date < v_target_effective;

  SELECT MIN(e.effective_date) INTO v_next_effective
  FROM orgunit.org_events_effective e
  WHERE e.tenant_uuid = p_tenant_uuid
    AND e.org_id = p_org_id
    AND e.effective_date > v_target_effective;

  IF v_prev_effective IS NOT NULL AND v_new_effective <= v_prev_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('prev=%s new=%s', v_prev_effective, v_new_effective);
  END IF;
  IF v_next_effective IS NOT NULL AND v_new_effective >= v_next_effective THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EFFECTIVE_DATE_OUT_OF_RANGE',
      DETAIL = format('next=%s new=%s', v_next_effective, v_new_effective);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_events_effective e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND e.org_id = p_org_id
      AND e.effective_date = v_new_effective
      AND e.event_uuid <> v_target.event_uuid
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'EVENT_DATE_CONFLICT',
      DETAIL = format('org_id=%s effective_date=%s', p_org_id, v_new_effective);
  END IF;

  IF v_payload ? 'parent_id' THEN
    v_parent_id := NULLIF(v_payload->>'parent_id', '')::int;
    IF v_parent_id IS NOT NULL THEN
      IF v_parent_id = p_org_id THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_CYCLE_DETECTED',
          DETAIL = format('org_id=%s parent_id=%s', p_org_id, v_parent_id);
      END IF;
      IF NOT EXISTS (
        SELECT 1
        FROM orgunit.org_unit_versions v
        WHERE v.tenant_uuid = p_tenant_uuid
          AND v.org_id = v_parent_id
          AND v.validity @> v_new_effective
        LIMIT 1
      ) THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_PARENT_NOT_FOUND_AS_OF',
          DETAIL = format('parent_id=%s as_of=%s', v_parent_id, v_new_effective);
      END IF;
    END IF;
  END IF;

  -- Guard high-risk create reordering that would force replay to fail after full-table churn.
  IF v_target.event_type = 'CREATE' AND v_new_effective > v_target_effective THEN
    SELECT v.node_path INTO v_target_path
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.org_id = p_org_id
      AND v.validity @> v_target_effective
    ORDER BY lower(v.validity) DESC
    LIMIT 1;

    IF v_target_path IS NOT NULL THEN
      SELECT MIN(e.effective_date) INTO v_descendant_min_create
      FROM orgunit.org_events_effective e
      WHERE e.tenant_uuid = p_tenant_uuid
        AND e.event_type = 'CREATE'
        AND e.org_id <> p_org_id
        AND EXISTS (
          SELECT 1
          FROM orgunit.org_unit_versions dv
          WHERE dv.tenant_uuid = p_tenant_uuid
            AND dv.org_id = e.org_id
            AND dv.node_path <@ v_target_path
          LIMIT 1
        );

      IF v_descendant_min_create IS NOT NULL AND v_descendant_min_create < v_new_effective THEN
        RAISE EXCEPTION USING
          MESSAGE = 'ORG_HIGH_RISK_REORDER_FORBIDDEN',
          DETAIL = format(
            'org_id=%s target_effective=%s new_effective=%s descendant_create=%s',
            p_org_id,
            v_target_effective,
            v_new_effective,
            v_descendant_min_create
          );
      END IF;
    END IF;
  END IF;

  v_payload := jsonb_build_object(
    'op', 'CORRECT_EVENT',
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  ) || v_payload;

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_type <> 'CORRECT_EVENT'
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.payload <> v_payload
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_id);
    END IF;

    RETURN v_existing_request.event_uuid;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_event_uuid := gen_random_uuid();
  v_pending_time := now();
  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    v_event_db_id,
    v_event_uuid,
    'CORRECT_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  PERFORM orgunit.assert_org_event_snapshots('CORRECT_EVENT', v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    tenant_uuid,
    org_id,
    event_uuid,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid,
    before_snapshot,
    after_snapshot,
    tx_time,
    transaction_time,
    created_at
  )
  VALUES (
    v_event_db_id,
    p_tenant_uuid,
    p_org_id,
    v_event_uuid,
    'CORRECT_EVENT',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  RETURN v_event_uuid;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.submit_org_status_correction(
  p_tenant_uuid uuid,
  p_org_id int,
  p_target_effective_date date,
  p_target_status text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_target orgunit.org_events%ROWTYPE;
  v_existing_request orgunit.org_events%ROWTYPE;
  v_existing_rescind orgunit.org_events%ROWTYPE;
  v_target_status text;
  v_target_effective date;
  v_effective_payload jsonb;
  v_event_uuid uuid;
  v_payload jsonb;
  v_before_snapshot jsonb;
  v_after_snapshot jsonb;
  v_event_db_id bigint;
  v_pending_time timestamptz;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_org_id IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'org_id is required';
  END IF;
  IF p_target_effective_date IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_effective_date is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_target_status := lower(btrim(COALESCE(p_target_status, '')));
  IF v_target_status IN ('enabled', '有效') THEN
    v_target_status := 'active';
  ELSIF v_target_status IN ('inactive', '无效') THEN
    v_target_status := 'disabled';
  END IF;
  IF v_target_status NOT IN ('active', 'disabled') THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'target_status invalid';
  END IF;

  v_lock_key := format('org:write-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT e.* INTO v_target
  FROM orgunit.org_events e
  JOIN orgunit.org_events_effective ee
    ON ee.event_uuid = e.event_uuid
   AND ee.tenant_uuid = e.tenant_uuid
   AND ee.org_id = e.org_id
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.effective_date = p_target_effective_date
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_NOT_FOUND',
      DETAIL = format('org_id=%s target_effective_date=%s', p_org_id, p_target_effective_date);
  END IF;

  IF v_target.event_type NOT IN ('ENABLE', 'DISABLE') THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_STATUS_CORRECTION_UNSUPPORTED_TARGET',
      DETAIL = format('event_type=%s', v_target.event_type);
  END IF;

  SELECT * INTO v_existing_rescind
  FROM orgunit.org_events r
  WHERE r.tenant_uuid = p_tenant_uuid
    AND r.event_type IN ('RESCIND_EVENT','RESCIND_ORG')
    AND r.payload->>'target_event_uuid' = v_target.event_uuid::text
  LIMIT 1;

  IF FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_EVENT_RESCINDED',
      DETAIL = format('event_uuid=%s', v_target.event_uuid);
  END IF;

  SELECT ee.effective_date, ee.payload
  INTO v_target_effective, v_effective_payload
  FROM orgunit.org_events_effective ee
  WHERE ee.tenant_uuid = p_tenant_uuid
    AND ee.org_id = p_org_id
    AND ee.event_uuid = v_target.event_uuid
  LIMIT 1;

  IF v_target_effective IS NULL THEN
    v_target_effective := v_target.effective_date;
    v_effective_payload := v_target.payload;
  END IF;

  v_payload := jsonb_build_object(
    'op', 'CORRECT_STATUS',
    'target_status', v_target_status,
    'target_event_uuid', v_target.event_uuid,
    'target_effective_date', p_target_effective_date
  );

  SELECT * INTO v_existing_request
  FROM orgunit.org_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing_request.event_type <> 'CORRECT_STATUS'
      OR v_existing_request.org_id <> p_org_id
      OR v_existing_request.payload <> v_payload
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_id);
    END IF;

    RETURN v_existing_request.event_uuid;
  END IF;

  v_before_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  v_event_uuid := gen_random_uuid();
  v_pending_time := now();
  SELECT nextval(pg_get_serial_sequence('orgunit.org_events', 'id')) INTO v_event_db_id;

  PERFORM orgunit.rebuild_org_unit_versions_for_org_with_pending_event(
    p_tenant_uuid,
    p_org_id,
    v_event_db_id,
    v_event_uuid,
    'CORRECT_STATUS',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  v_after_snapshot := orgunit.extract_orgunit_snapshot(p_tenant_uuid, p_org_id, p_target_effective_date);
  PERFORM orgunit.assert_org_event_snapshots('CORRECT_STATUS', v_before_snapshot, v_after_snapshot, NULL);

  INSERT INTO orgunit.org_events (
    id,
    tenant_uuid,
    org_id,
    event_uuid,
    event_type,
    effective_date,
    payload,
    request_code,
    initiator_uuid,
    before_snapshot,
    after_snapshot,
    tx_time,
    transaction_time,
    created_at
  )
  VALUES (
    v_event_db_id,
    p_tenant_uuid,
    p_org_id,
    v_event_uuid,
    'CORRECT_STATUS',
    p_target_effective_date,
    v_payload,
    p_request_id,
    p_initiator_uuid,
    v_before_snapshot,
    v_after_snapshot,
    v_pending_time,
    v_pending_time,
    v_pending_time
  );

  RETURN v_event_uuid;
END;
$$;
