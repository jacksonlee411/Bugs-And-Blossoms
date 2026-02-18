-- OrgUnit tenant field configs (metadata) + wide-table ext slots (Phase 1)
-- SSOT: docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md
-- SSOT: docs/dev-plans/100b-org-metadata-wide-table-phase1-schema-and-metadata-skeleton.md

CREATE TABLE IF NOT EXISTS orgunit.tenant_field_configs (
  tenant_uuid uuid NOT NULL,
  field_key text NOT NULL,
  physical_col text NOT NULL,
  value_type text NOT NULL,
  data_source_type text NOT NULL,
  data_source_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  display_label text NULL,
  enabled_on date NOT NULL,
  disabled_on date NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz NULL,
  PRIMARY KEY (tenant_uuid, field_key),
  -- field_key will be used as payload.ext key; keep it simple and safe.
  CONSTRAINT tenant_field_configs_field_key_format_check CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'),
  CONSTRAINT tenant_field_configs_value_type_check CHECK (value_type IN ('text','int','uuid','bool','date','numeric')),
  CONSTRAINT tenant_field_configs_data_source_type_check CHECK (data_source_type IN ('PLAIN','DICT','ENTITY')),
  CONSTRAINT tenant_field_configs_data_source_config_is_object_check CHECK (jsonb_typeof(data_source_config) = 'object'),
  CONSTRAINT tenant_field_configs_display_label_check CHECK (
    display_label IS NULL OR (display_label = btrim(display_label) AND display_label <> '')
  ),
  -- data_source_config shape (SSOT: DEV-PLAN-100A §4.3)
  CONSTRAINT tenant_field_configs_plain_config_check CHECK (
    data_source_type <> 'PLAIN' OR data_source_config = '{}'::jsonb
  ),
  CONSTRAINT tenant_field_configs_dict_config_check CHECK (
    data_source_type <> 'DICT' OR (
      value_type = 'text'
      AND data_source_config ? 'dict_code'
      AND jsonb_typeof(data_source_config->'dict_code') = 'string'
      AND NULLIF(btrim(data_source_config->>'dict_code'), '') IS NOT NULL
      -- forbid extra keys to avoid "hidden config" drift
      AND data_source_config = jsonb_build_object('dict_code', data_source_config->'dict_code')
    )
  ),
  CONSTRAINT tenant_field_configs_entity_config_check CHECK (
    data_source_type <> 'ENTITY' OR (
      data_source_config ? 'entity'
      AND jsonb_typeof(data_source_config->'entity') = 'string'
      AND NULLIF(btrim(data_source_config->>'entity'), '') IS NOT NULL
      AND data_source_config ? 'id_kind'
      AND jsonb_typeof(data_source_config->'id_kind') = 'string'
      AND (data_source_config->>'id_kind') IN ('uuid','int')
      AND (
        ((data_source_config->>'id_kind') = 'uuid' AND value_type = 'uuid')
        OR
        ((data_source_config->>'id_kind') = 'int' AND value_type = 'int')
      )
      -- forbid extra keys to avoid "hidden config" drift
      AND data_source_config = jsonb_build_object(
        'entity', data_source_config->'entity',
        'id_kind', data_source_config->'id_kind'
      )
    )
  ),
  -- physical_col is used for dynamic SQL allowlist; keep the format strict.
  CONSTRAINT tenant_field_configs_physical_col_format_check CHECK (
    physical_col ~ '^ext_(str|int|uuid|bool|date|num)_[0-9]{2}$'
  ),
  CONSTRAINT tenant_field_configs_physical_col_group_check CHECK (
    (value_type = 'text' AND physical_col LIKE 'ext_str_%')
    OR (value_type = 'int' AND physical_col LIKE 'ext_int_%')
    OR (value_type = 'uuid' AND physical_col LIKE 'ext_uuid_%')
    OR (value_type = 'bool' AND physical_col LIKE 'ext_bool_%')
    OR (value_type = 'date' AND physical_col LIKE 'ext_date_%')
    OR (value_type = 'numeric' AND physical_col LIKE 'ext_num_%')
  ),
  CONSTRAINT tenant_field_configs_disabled_on_check CHECK (disabled_on IS NULL OR disabled_on >= enabled_on),
  CONSTRAINT tenant_field_configs_physical_col_unique UNIQUE (tenant_uuid, physical_col)
);

CREATE TABLE IF NOT EXISTS orgunit.tenant_field_config_events (
  id bigserial PRIMARY KEY,
  event_uuid uuid NOT NULL,
  tenant_uuid uuid NOT NULL,
  event_type text NOT NULL,
  field_key text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_code text NOT NULL,
  initiator_uuid uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tenant_field_config_events_event_type_check CHECK (event_type IN ('ENABLE','DISABLE','REKEY')),
  CONSTRAINT tenant_field_config_events_field_key_format_check CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'),
  CONSTRAINT tenant_field_config_events_request_code_unique UNIQUE (tenant_uuid, request_code),
  CONSTRAINT tenant_field_config_events_event_uuid_unique UNIQUE (event_uuid),
  CONSTRAINT tenant_field_config_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS tenant_field_config_events_tenant_time_idx
  ON orgunit.tenant_field_config_events (tenant_uuid, transaction_time DESC, id DESC);

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

DROP TRIGGER IF EXISTS tenant_field_configs_update_allowed ON orgunit.tenant_field_configs;
CREATE TRIGGER tenant_field_configs_update_allowed
BEFORE UPDATE ON orgunit.tenant_field_configs
FOR EACH ROW EXECUTE FUNCTION orgunit.assert_tenant_field_configs_update_allowed();

-- -------------------------------------------------------------------
-- Wide-table ext slots.
-- -------------------------------------------------------------------

ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_01 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_02 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_03 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_04 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_05 text NULL;

ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_01 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_02 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_03 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_04 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_05 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_06 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_07 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_08 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_09 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_10 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_11 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_12 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_13 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_14 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_int_15 int NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_01 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_02 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_03 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_04 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_05 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_06 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_07 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_08 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_09 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_uuid_10 uuid NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_01 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_01 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_02 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_03 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_04 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_05 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_06 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_07 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_08 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_09 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_10 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_11 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_12 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_13 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_14 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_bool_15 boolean NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_02 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_03 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_04 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_05 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_06 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_07 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_08 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_09 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_10 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_11 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_12 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_13 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_14 date NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_date_15 date NULL;

ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_01 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_02 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_03 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_04 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_05 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_06 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_07 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_08 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_09 numeric NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_num_10 numeric NULL;

ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_06 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_07 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_08 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_09 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_10 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_11 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_12 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_13 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_14 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_15 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_16 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_17 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_18 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_19 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_20 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_21 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_22 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_23 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_24 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_25 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_26 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_27 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_28 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_29 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_30 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_31 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_32 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_33 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_34 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_35 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_36 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_37 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_38 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_39 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_40 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_41 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_42 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_43 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_44 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_45 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_46 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_47 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_48 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_49 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_50 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_51 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_52 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_53 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_54 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_55 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_56 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_57 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_58 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_59 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_60 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_61 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_62 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_63 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_64 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_65 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_66 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_67 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_68 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_69 text NULL;
ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_str_70 text NULL;

ALTER TABLE orgunit.org_unit_versions
  ADD COLUMN IF NOT EXISTS ext_labels_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;

-- MVP: at least one DICT field requires list filter/sort (see DEV-PLAN-100A §9.1).
-- Since physical_col is assigned dynamically, we pre-index all ext_str slots.
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_01_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_01)
  WHERE ext_str_01 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_02_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_02)
  WHERE ext_str_02 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_03_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_03)
  WHERE ext_str_03 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_04_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_04)
  WHERE ext_str_04 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_05_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_05)
  WHERE ext_str_05 IS NOT NULL;

CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_06_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_06)
  WHERE ext_str_06 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_07_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_07)
  WHERE ext_str_07 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_08_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_08)
  WHERE ext_str_08 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_09_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_09)
  WHERE ext_str_09 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_10_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_10)
  WHERE ext_str_10 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_11_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_11)
  WHERE ext_str_11 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_12_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_12)
  WHERE ext_str_12 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_13_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_13)
  WHERE ext_str_13 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_14_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_14)
  WHERE ext_str_14 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_15_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_15)
  WHERE ext_str_15 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_16_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_16)
  WHERE ext_str_16 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_17_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_17)
  WHERE ext_str_17 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_18_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_18)
  WHERE ext_str_18 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_19_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_19)
  WHERE ext_str_19 IS NOT NULL;
CREATE INDEX IF NOT EXISTS org_unit_versions_ext_str_20_idx
  ON orgunit.org_unit_versions (tenant_uuid, ext_str_20)
  WHERE ext_str_20 IS NOT NULL;

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
  p_display_label text,
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
  v_display_label := NULLIF(btrim(p_display_label), '');

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
      OR COALESCE(v_existing.payload->>'display_label', '') <> COALESCE(v_display_label, '')
      OR v_existing.payload->>'disabled_on' IS NOT NULL
    THEN
      RAISE EXCEPTION USING
        MESSAGE = 'ORG_REQUEST_ID_CONFLICT',
        DETAIL = format('request_code=%s', p_request_code);
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
  IF p_request_code IS NULL OR btrim(p_request_code) = '' THEN
    RAISE EXCEPTION USING MESSAGE = 'ORG_INVALID_ARGUMENT', DETAIL = 'request_code is required';
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
    AND request_code = p_request_code
  LIMIT 1;

  IF FOUND THEN
    IF v_existing.event_type <> 'REKEY'
      OR COALESCE(v_existing.payload->>'old_field_key', '') <> p_old_field_key
      OR COALESCE(v_existing.payload->>'new_field_key', '') <> p_new_field_key
      OR v_existing.initiator_uuid <> p_initiator_uuid
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
    request_code,
    initiator_uuid
  )
  VALUES (
    gen_random_uuid(),
    p_tenant_uuid,
    'REKEY',
    p_new_field_key,
    v_payload,
    p_request_code,
    p_initiator_uuid
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
