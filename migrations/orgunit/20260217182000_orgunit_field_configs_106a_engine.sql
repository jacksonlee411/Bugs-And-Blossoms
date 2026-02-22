-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v_allow_rekey boolean := (current_setting('app.orgunit_field_config_allow_rekey', true) = 'on');
BEGIN
  IF TG_OP <> 'UPDATE' THEN
    RETURN NEW;
  END IF;

  -- Mapping is immutable after enable. Rekey requires explicit one-shot opt-in.
  IF NEW.field_key <> OLD.field_key THEN
    IF NOT v_allow_rekey
      OR NEW.physical_col <> OLD.physical_col
      OR NEW.value_type <> OLD.value_type
      OR NEW.data_source_type <> OLD.data_source_type
      OR NEW.data_source_config <> OLD.data_source_config
      OR NEW.display_label IS DISTINCT FROM OLD.display_label
      OR NEW.enabled_on <> OLD.enabled_on
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_MAPPING_IMMUTABLE',
        DETAIL = format('field_key=%s', OLD.field_key);
    END IF;
  ELSIF NEW.physical_col <> OLD.physical_col
    OR NEW.value_type <> OLD.value_type
    OR NEW.data_source_type <> OLD.data_source_type
    OR NEW.data_source_config <> OLD.data_source_config
    OR NEW.display_label IS DISTINCT FROM OLD.display_label
    OR NEW.enabled_on <> OLD.enabled_on
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_MAPPING_IMMUTABLE',
      DETAIL = format('field_key=%s', OLD.field_key);
  END IF;

  -- disabled_on cannot be "unset".
  IF OLD.disabled_on IS NOT NULL AND NEW.disabled_on IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
      DETAIL = format('field_key=%s', OLD.field_key);
  END IF;

  -- When setting disabled_on for the first time, forbid backdating.
  IF OLD.disabled_on IS NULL AND NEW.disabled_on IS NOT NULL THEN
    IF NEW.disabled_on < current_date THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s disabled_on=%s', OLD.field_key, NEW.disabled_on);
    END IF;
    RETURN NEW;
  END IF;

  -- If disabled_on changes, it must be "not effective yet" and only postponed.
  IF OLD.disabled_on IS NOT NULL AND NEW.disabled_on IS NOT NULL AND NEW.disabled_on <> OLD.disabled_on THEN
    IF current_date >= OLD.disabled_on THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s disabled_on=%s', OLD.field_key, OLD.disabled_on);
    END IF;
    IF NEW.disabled_on <= OLD.disabled_on THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s old=%s new=%s', OLD.field_key, OLD.disabled_on, NEW.disabled_on);
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

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

  IF p_value_type NOT IN ('text','int','uuid','bool','date') THEN
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
    v_candidate_cols := ARRAY['ext_str_01','ext_str_02','ext_str_03','ext_str_04','ext_str_05'];
  ELSIF p_value_type = 'int' THEN
    v_candidate_cols := ARRAY['ext_int_01'];
  ELSIF p_value_type = 'uuid' THEN
    v_candidate_cols := ARRAY['ext_uuid_01'];
  ELSIF p_value_type = 'bool' THEN
    v_candidate_cols := ARRAY['ext_bool_01'];
  ELSIF p_value_type = 'date' THEN
    v_candidate_cols := ARRAY['ext_date_01'];
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

CREATE OR REPLACE FUNCTION orgunit.rename_jsonb_object_key_strict(
  p_obj jsonb,
  p_old_key text,
  p_new_key text
)
RETURNS jsonb
LANGUAGE plpgsql
AS $$
DECLARE
  v_obj jsonb := COALESCE(p_obj, '{}'::jsonb);
BEGIN
  IF jsonb_typeof(v_obj) <> 'object' THEN
    RETURN '{}'::jsonb;
  END IF;
  IF v_obj ? p_old_key AND v_obj ? p_new_key THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_REKEY_CONFLICT',
      DETAIL = format('old_field_key=%s new_field_key=%s', p_old_key, p_new_key);
  END IF;
  IF v_obj ? p_old_key THEN
    RETURN (v_obj - p_old_key) || jsonb_build_object(p_new_key, v_obj->p_old_key);
  END IF;
  RETURN v_obj;
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.rekey_tenant_field_config(
  p_tenant_uuid uuid,
  p_old_field_key text,
  p_new_field_key text,
  p_request_id text,
  p_initiator_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_existing orgunit.tenant_field_config_events%ROWTYPE;
  v_cfg orgunit.tenant_field_configs%ROWTYPE;
  v_dict_code text;
  v_payload jsonb;
  v_updated_events int := 0;
  v_updated_versions int := 0;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  p_old_field_key := lower(btrim(p_old_field_key));
  p_new_field_key := lower(btrim(p_new_field_key));

  IF p_old_field_key IS NULL OR p_old_field_key = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'old_field_key is required';
  END IF;
  IF p_new_field_key IS NULL OR p_new_field_key = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'new_field_key is required';
  END IF;
  IF p_request_id IS NULL OR btrim(p_request_id) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_id is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;
  IF p_old_field_key !~ '^[a-z][a-z0-9_]{0,62}$' OR p_new_field_key !~ '^[a-z][a-z0-9_]{0,62}$' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'field_key invalid';
  END IF;
  IF p_old_field_key = p_new_field_key THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'old_field_key/new_field_key must differ';
  END IF;

  v_lock_key := format('org:field-config-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing
  FROM orgunit.tenant_field_config_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_id = p_request_id
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type <> 'REKEY'
      OR COALESCE(v_existing.payload->>'old_field_key', '') <> p_old_field_key
      OR COALESCE(v_existing.payload->>'new_field_key', '') <> p_new_field_key
      OR v_existing.initiator_uuid <> p_initiator_uuid
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_id=%s', p_request_id);
    END IF;
    RETURN;
  END IF;

  SELECT * INTO v_cfg
  FROM orgunit.tenant_field_configs
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_old_field_key
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_NOT_FOUND',
      DETAIL = format('field_key=%s', p_old_field_key);
  END IF;

  IF v_cfg.data_source_type <> 'DICT' OR v_cfg.value_type <> 'text' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
      DETAIL = format('field_key=%s', p_old_field_key);
  END IF;

  v_dict_code := NULLIF(btrim(v_cfg.data_source_config->>'dict_code'), '');
  IF v_dict_code IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
      DETAIL = format('field_key=%s missing_dict_code', p_old_field_key);
  END IF;
  IF p_new_field_key <> ('d_' || v_dict_code) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG',
      DETAIL = format('field_key=%s expected_new_field_key=%s', p_old_field_key, 'd_' || v_dict_code);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.tenant_field_configs
    WHERE tenant_uuid = p_tenant_uuid
      AND field_key = p_new_field_key
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_ALREADY_ENABLED',
      DETAIL = format('field_key=%s', p_new_field_key);
  END IF;

  IF EXISTS (
    SELECT 1
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND jsonb_typeof(e.payload->'ext') = 'object'
      AND (e.payload->'ext') ? p_old_field_key
      AND (e.payload->'ext') ? p_new_field_key
    LIMIT 1
  ) OR EXISTS (
    SELECT 1
    FROM orgunit.org_events e
    WHERE e.tenant_uuid = p_tenant_uuid
      AND jsonb_typeof(e.payload->'ext_labels_snapshot') = 'object'
      AND (e.payload->'ext_labels_snapshot') ? p_old_field_key
      AND (e.payload->'ext_labels_snapshot') ? p_new_field_key
    LIMIT 1
  ) OR EXISTS (
    SELECT 1
    FROM orgunit.org_unit_versions v
    WHERE v.tenant_uuid = p_tenant_uuid
      AND v.ext_labels_snapshot ? p_old_field_key
      AND v.ext_labels_snapshot ? p_new_field_key
    LIMIT 1
  ) THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_REKEY_CONFLICT',
      DETAIL = format('old_field_key=%s new_field_key=%s', p_old_field_key, p_new_field_key);
  END IF;

  PERFORM set_config('app.orgunit_field_config_allow_rekey', 'on', true);
  UPDATE orgunit.tenant_field_configs
  SET field_key = p_new_field_key,
      updated_at = now()
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_old_field_key;

  WITH rewritten AS (
    SELECT
      id,
      payload,
      CASE
        WHEN jsonb_typeof(payload->'ext') = 'object' THEN
          orgunit.rename_jsonb_object_key_strict(payload->'ext', p_old_field_key, p_new_field_key)
        ELSE NULL
      END AS new_ext,
      CASE
        WHEN jsonb_typeof(payload->'ext_labels_snapshot') = 'object' THEN
          orgunit.rename_jsonb_object_key_strict(payload->'ext_labels_snapshot', p_old_field_key, p_new_field_key)
        ELSE NULL
      END AS new_labels
    FROM orgunit.org_events
    WHERE tenant_uuid = p_tenant_uuid
  ), updates AS (
    UPDATE orgunit.org_events e
    SET payload = (
      (r.payload - 'ext' - 'ext_labels_snapshot')
      || CASE WHEN r.new_ext IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('ext', r.new_ext) END
      || CASE WHEN r.new_labels IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('ext_labels_snapshot', r.new_labels) END
    )
    FROM rewritten r
    WHERE e.id = r.id
      AND e.tenant_uuid = p_tenant_uuid
      AND (
        (r.new_ext IS NOT NULL AND r.new_ext IS DISTINCT FROM COALESCE(r.payload->'ext', '{}'::jsonb))
        OR
        (r.new_labels IS NOT NULL AND r.new_labels IS DISTINCT FROM COALESCE(r.payload->'ext_labels_snapshot', '{}'::jsonb))
      )
    RETURNING 1
  )
  SELECT count(*) INTO v_updated_events FROM updates;

  UPDATE orgunit.org_unit_versions v
  SET ext_labels_snapshot = orgunit.rename_jsonb_object_key_strict(v.ext_labels_snapshot, p_old_field_key, p_new_field_key)
  WHERE v.tenant_uuid = p_tenant_uuid
    AND jsonb_typeof(v.ext_labels_snapshot) = 'object'
    AND v.ext_labels_snapshot ? p_old_field_key;
  GET DIAGNOSTICS v_updated_versions = ROW_COUNT;

  v_payload := jsonb_build_object(
    'old_field_key', p_old_field_key,
    'new_field_key', p_new_field_key,
    'dict_code', v_dict_code,
    'physical_col', v_cfg.physical_col,
    'enabled_on', v_cfg.enabled_on::text,
    'display_label', v_cfg.display_label,
    'updated_org_events', v_updated_events,
    'updated_org_unit_versions', v_updated_versions
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
    'REKEY',
    p_new_field_key,
    v_payload,
    p_request_id,
    p_initiator_uuid
  );
END;
$$;

ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.rename_jsonb_object_key_strict(jsonb, text, text)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rename_jsonb_object_key_strict(jsonb, text, text)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.rename_jsonb_object_key_strict(jsonb, text, text)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid)
  SET search_path = pg_catalog, orgunit, public;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS orgunit.rekey_tenant_field_config(uuid, text, text, text, uuid);
DROP FUNCTION IF EXISTS orgunit.rename_jsonb_object_key_strict(jsonb, text, text);
DROP FUNCTION IF EXISTS orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, text, uuid);

CREATE OR REPLACE FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP <> 'UPDATE' THEN
    RETURN NEW;
  END IF;

  -- Mapping is immutable after enable.
  IF NEW.field_key <> OLD.field_key
    OR NEW.physical_col <> OLD.physical_col
    OR NEW.value_type <> OLD.value_type
    OR NEW.data_source_type <> OLD.data_source_type
    OR NEW.data_source_config <> OLD.data_source_config
    OR NEW.enabled_on <> OLD.enabled_on
  THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_MAPPING_IMMUTABLE',
      DETAIL = format('field_key=%s', OLD.field_key);
  END IF;

  -- disabled_on cannot be "unset".
  IF OLD.disabled_on IS NOT NULL AND NEW.disabled_on IS NULL THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
      DETAIL = format('field_key=%s', OLD.field_key);
  END IF;

  -- When setting disabled_on for the first time, forbid backdating.
  IF OLD.disabled_on IS NULL AND NEW.disabled_on IS NOT NULL THEN
    IF NEW.disabled_on < current_date THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s disabled_on=%s', OLD.field_key, NEW.disabled_on);
    END IF;
    RETURN NEW;
  END IF;

  -- If disabled_on changes, it must be "not effective yet" and only postponed.
  IF OLD.disabled_on IS NOT NULL AND NEW.disabled_on IS NOT NULL AND NEW.disabled_on <> OLD.disabled_on THEN
    IF current_date >= OLD.disabled_on THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s disabled_on=%s', OLD.field_key, OLD.disabled_on);
    END IF;
    IF NEW.disabled_on <= OLD.disabled_on THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s old=%s new=%s', OLD.field_key, OLD.disabled_on, NEW.disabled_on);
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  SET search_path = pg_catalog, orgunit, public;

-- +goose StatementEnd
