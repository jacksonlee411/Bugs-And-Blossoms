-- +goose Up
-- +goose StatementBegin
-- OrgUnit tenant field configs: RLS + guards + kernel functions + privileges (Phase 1)
-- SSOT: docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md

-- -------------------------------------------------------------------
-- RLS (fail-closed): tenant isolation must be enforced for new tables.
-- -------------------------------------------------------------------

ALTER TABLE orgunit.tenant_field_config_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.tenant_field_config_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_config_events;
CREATE POLICY tenant_isolation ON orgunit.tenant_field_config_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

ALTER TABLE orgunit.tenant_field_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE orgunit.tenant_field_configs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_configs;
CREATE POLICY tenant_isolation ON orgunit.tenant_field_configs
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

-- -------------------------------------------------------------------
-- DB guards (fail-closed): no direct DML; only orgunit_kernel can write.
-- -------------------------------------------------------------------

CREATE OR REPLACE FUNCTION orgunit.guard_tenant_field_configs_write()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_user <> 'orgunit_kernel' THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORGUNIT_FIELD_CONFIGS_WRITE_FORBIDDEN',
      DETAIL = format('role=%s table=%s', current_user, TG_TABLE_NAME);
  END IF;

  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS guard_tenant_field_configs_write ON orgunit.tenant_field_configs;
CREATE TRIGGER guard_tenant_field_configs_write
BEFORE INSERT OR UPDATE OR DELETE ON orgunit.tenant_field_configs
FOR EACH ROW EXECUTE FUNCTION orgunit.guard_tenant_field_configs_write();

DROP TRIGGER IF EXISTS guard_tenant_field_config_events_write ON orgunit.tenant_field_config_events;
CREATE TRIGGER guard_tenant_field_config_events_write
BEFORE INSERT OR UPDATE OR DELETE ON orgunit.tenant_field_config_events
FOR EACH ROW EXECUTE FUNCTION orgunit.guard_tenant_field_configs_write();

-- -------------------------------------------------------------------
-- Mapping immutability + disabled_on rules (defense-in-depth).
-- -------------------------------------------------------------------

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

DROP TRIGGER IF EXISTS tenant_field_configs_update_allowed ON orgunit.tenant_field_configs;
CREATE TRIGGER tenant_field_configs_update_allowed
BEFORE UPDATE ON orgunit.tenant_field_configs
FOR EACH ROW EXECUTE FUNCTION orgunit.assert_tenant_field_configs_update_allowed();

-- -------------------------------------------------------------------
-- Kernel write entrypoints (One Door for metadata writes).
-- -------------------------------------------------------------------

CREATE OR REPLACE FUNCTION orgunit.enable_tenant_field_config(
  p_tenant_uuid uuid,
  p_field_key text,
  p_value_type text,
  p_enabled_on date,
  p_data_source_type text,
  p_data_source_config jsonb,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_existing orgunit.tenant_field_config_events%ROWTYPE;
  v_config jsonb;
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
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
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

  SELECT * INTO v_existing
  FROM orgunit.tenant_field_config_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type <> 'ENABLE'
      OR v_existing.field_key <> p_field_key
      OR v_existing.initiator_uuid <> p_initiator_uuid
      OR COALESCE(v_existing.payload->>'value_type', '') <> p_value_type
      OR COALESCE(v_existing.payload->>'data_source_type', '') <> p_data_source_type
      OR COALESCE(v_existing.payload->>'enabled_on', '') <> p_enabled_on::text
      OR COALESCE(v_existing.payload->'data_source_config', '{}'::jsonb) <> v_config
      OR v_existing.payload->>'disabled_on' IS NOT NULL
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_code);
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
    'enabled_on', p_enabled_on::text,
    'disabled_on', NULL
  );

  INSERT INTO orgunit.tenant_field_config_events (
    event_uuid,
    tenant_uuid,
    event_type,
    field_key,
    payload,
    request_code,
    initiator_uuid
  )
  VALUES (
    gen_random_uuid(),
    p_tenant_uuid,
    'ENABLE',
    p_field_key,
    v_payload,
    p_request_code,
    p_initiator_uuid
  );

  INSERT INTO orgunit.tenant_field_configs (
    tenant_uuid,
    field_key,
    physical_col,
    value_type,
    data_source_type,
    data_source_config,
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
    p_enabled_on,
    NULL,
    now(),
    now()
  );
END;
$$;

CREATE OR REPLACE FUNCTION orgunit.disable_tenant_field_config(
  p_tenant_uuid uuid,
  p_field_key text,
  p_disabled_on date,
  p_request_code text,
  p_initiator_uuid uuid
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  v_lock_key text;
  v_existing orgunit.tenant_field_config_events%ROWTYPE;
  v_cfg orgunit.tenant_field_configs%ROWTYPE;
  v_payload jsonb;
BEGIN
  PERFORM orgunit.assert_current_tenant(p_tenant_uuid);

  IF p_field_key IS NULL OR btrim(p_field_key) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'field_key is required';
  END IF;
  IF p_field_key !~ '^[a-z][a-z0-9_]{0,62}$' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = format('field_key=%s', p_field_key);
  END IF;
  IF p_disabled_on IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'disabled_on is required';
  END IF;
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
  END IF;
  IF p_initiator_uuid IS NULL THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'initiator_uuid is required';
  END IF;

  v_lock_key := format('org:field-config-lock:%s', p_tenant_uuid);
  PERFORM pg_advisory_xact_lock(hashtextextended(v_lock_key, 0));

  SELECT * INTO v_existing
  FROM orgunit.tenant_field_config_events
  WHERE tenant_uuid = p_tenant_uuid
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type <> 'DISABLE'
      OR v_existing.field_key <> p_field_key
      OR v_existing.initiator_uuid <> p_initiator_uuid
      OR COALESCE(v_existing.payload->>'disabled_on', '') <> p_disabled_on::text
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_code);
    END IF;

    RETURN;
  END IF;

  SELECT * INTO v_cfg
  FROM orgunit.tenant_field_configs
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_field_key
  LIMIT 1;

  IF NOT FOUND THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_NOT_FOUND',
      DETAIL = format('field_key=%s', p_field_key);
  END IF;

  IF p_disabled_on < v_cfg.enabled_on THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
      DETAIL = format('field_key=%s enabled_on=%s disabled_on=%s', p_field_key, v_cfg.enabled_on, p_disabled_on);
  END IF;
  -- MVP freeze: forbid backdating disables.
  IF p_disabled_on < current_date THEN
    RAISE EXCEPTION USING
      MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
      DETAIL = format('field_key=%s disabled_on=%s', p_field_key, p_disabled_on);
  END IF;

  IF v_cfg.disabled_on IS NOT NULL THEN
    IF current_date >= v_cfg.disabled_on THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s disabled_on=%s', p_field_key, v_cfg.disabled_on);
    END IF;
    IF p_disabled_on <= v_cfg.disabled_on THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_FIELD_CONFIG_DISABLED_ON_INVALID',
        DETAIL = format('field_key=%s old=%s new=%s', p_field_key, v_cfg.disabled_on, p_disabled_on);
    END IF;
  END IF;

  v_payload := jsonb_build_object('disabled_on', p_disabled_on::text);

  INSERT INTO orgunit.tenant_field_config_events (
    event_uuid,
    tenant_uuid,
    event_type,
    field_key,
    payload,
    request_code,
    initiator_uuid
  )
  VALUES (
    gen_random_uuid(),
    p_tenant_uuid,
    'DISABLE',
    p_field_key,
    v_payload,
    p_request_code,
    p_initiator_uuid
  );

  UPDATE orgunit.tenant_field_configs
  SET disabled_on = p_disabled_on,
      disabled_at = now(),
      updated_at = now()
  WHERE tenant_uuid = p_tenant_uuid
    AND field_key = p_field_key;
END;
$$;

-- -------------------------------------------------------------------
-- Kernel privileges (security definer + owner + DML revokes).
-- -------------------------------------------------------------------

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'orgunit_kernel') THEN
    CREATE ROLE orgunit_kernel NOLOGIN NOBYPASSRLS;
  END IF;
END $$;

GRANT USAGE ON SCHEMA orgunit TO orgunit_kernel;

ALTER TABLE IF EXISTS orgunit.tenant_field_config_events OWNER TO orgunit_kernel;
ALTER TABLE IF EXISTS orgunit.tenant_field_configs OWNER TO orgunit_kernel;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  orgunit.tenant_field_config_events,
  orgunit.tenant_field_configs
TO orgunit_kernel;

GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA orgunit TO orgunit_kernel;

ALTER FUNCTION orgunit.guard_tenant_field_configs_write()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.guard_tenant_field_configs_write()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.assert_tenant_field_configs_update_allowed()
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)
  OWNER TO orgunit_kernel;
ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)
  SECURITY DEFINER;
ALTER FUNCTION orgunit.disable_tenant_field_config(uuid, text, date, text, uuid)
  SET search_path = pg_catalog, orgunit, public;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM app';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO app';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM app_runtime';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO app_runtime';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM app_nobypassrls';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO app_nobypassrls';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'FROM superadmin_runtime';
    EXECUTE 'GRANT SELECT ON TABLE ' ||
      'orgunit.tenant_field_config_events, ' ||
      'orgunit.tenant_field_configs ' ||
      'TO superadmin_runtime';
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- reverse: privileges
REVOKE ALL ON TABLE orgunit.tenant_field_config_events FROM orgunit_kernel;
REVOKE ALL ON TABLE orgunit.tenant_field_configs FROM orgunit_kernel;

-- reverse: kernel functions / triggers
DROP TRIGGER IF EXISTS tenant_field_configs_update_allowed ON orgunit.tenant_field_configs;
DROP TRIGGER IF EXISTS guard_tenant_field_configs_write ON orgunit.tenant_field_configs;
DROP TRIGGER IF EXISTS guard_tenant_field_config_events_write ON orgunit.tenant_field_config_events;

DROP FUNCTION IF EXISTS orgunit.disable_tenant_field_config(uuid, text, date, text, uuid);
DROP FUNCTION IF EXISTS orgunit.enable_tenant_field_config(uuid, text, text, date, text, jsonb, text, uuid);
DROP FUNCTION IF EXISTS orgunit.assert_tenant_field_configs_update_allowed();
DROP FUNCTION IF EXISTS orgunit.guard_tenant_field_configs_write();

-- reverse: RLS
DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_configs;
ALTER TABLE IF EXISTS orgunit.tenant_field_configs DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON orgunit.tenant_field_config_events;
ALTER TABLE IF EXISTS orgunit.tenant_field_config_events DISABLE ROW LEVEL SECURITY;
-- +goose StatementEnd

