-- +goose Up
-- modify "org_unit_versions" table
ALTER TABLE "orgunit"."org_unit_versions" ADD COLUMN "ext_str_01" text NULL, ADD COLUMN "ext_str_02" text NULL, ADD COLUMN "ext_str_03" text NULL, ADD COLUMN "ext_str_04" text NULL, ADD COLUMN "ext_str_05" text NULL, ADD COLUMN "ext_int_01" integer NULL, ADD COLUMN "ext_uuid_01" uuid NULL, ADD COLUMN "ext_bool_01" boolean NULL, ADD COLUMN "ext_date_01" date NULL, ADD COLUMN "ext_labels_snapshot" jsonb NOT NULL DEFAULT '{}';
-- create index "org_unit_versions_ext_str_01_idx" to table: "org_unit_versions"
CREATE INDEX "org_unit_versions_ext_str_01_idx" ON "orgunit"."org_unit_versions" ("tenant_uuid", "ext_str_01") WHERE (ext_str_01 IS NOT NULL);
-- create index "org_unit_versions_ext_str_02_idx" to table: "org_unit_versions"
CREATE INDEX "org_unit_versions_ext_str_02_idx" ON "orgunit"."org_unit_versions" ("tenant_uuid", "ext_str_02") WHERE (ext_str_02 IS NOT NULL);
-- create index "org_unit_versions_ext_str_03_idx" to table: "org_unit_versions"
CREATE INDEX "org_unit_versions_ext_str_03_idx" ON "orgunit"."org_unit_versions" ("tenant_uuid", "ext_str_03") WHERE (ext_str_03 IS NOT NULL);
-- create index "org_unit_versions_ext_str_04_idx" to table: "org_unit_versions"
CREATE INDEX "org_unit_versions_ext_str_04_idx" ON "orgunit"."org_unit_versions" ("tenant_uuid", "ext_str_04") WHERE (ext_str_04 IS NOT NULL);
-- create index "org_unit_versions_ext_str_05_idx" to table: "org_unit_versions"
CREATE INDEX "org_unit_versions_ext_str_05_idx" ON "orgunit"."org_unit_versions" ("tenant_uuid", "ext_str_05") WHERE (ext_str_05 IS NOT NULL);
-- create "tenant_field_config_events" table
CREATE TABLE "orgunit"."tenant_field_config_events" (
  "id" bigserial NOT NULL,
  "event_uuid" uuid NOT NULL,
  "tenant_uuid" uuid NOT NULL,
  "event_type" text NOT NULL,
  "field_key" text NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "request_id" text NOT NULL,
  "initiator_uuid" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "tenant_field_config_events_event_uuid_unique" UNIQUE ("event_uuid"),
  CONSTRAINT "tenant_field_config_events_request_id_unique" UNIQUE ("tenant_uuid", "request_id"),
  CONSTRAINT "tenant_field_config_events_event_type_check" CHECK (event_type = ANY (ARRAY['ENABLE'::text, 'DISABLE'::text])),
  CONSTRAINT "tenant_field_config_events_field_key_format_check" CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'::text),
  CONSTRAINT "tenant_field_config_events_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text)
);
-- create index "tenant_field_config_events_tenant_time_idx" to table: "tenant_field_config_events"
CREATE INDEX "tenant_field_config_events_tenant_time_idx" ON "orgunit"."tenant_field_config_events" ("tenant_uuid", "transaction_time" DESC, "id" DESC);
-- create "tenant_field_configs" table
CREATE TABLE "orgunit"."tenant_field_configs" (
  "tenant_uuid" uuid NOT NULL,
  "field_key" text NOT NULL,
  "physical_col" text NOT NULL,
  "value_type" text NOT NULL,
  "data_source_type" text NOT NULL,
  "data_source_config" jsonb NOT NULL DEFAULT '{}',
  "enabled_on" date NOT NULL,
  "disabled_on" date NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "disabled_at" timestamptz NULL,
  PRIMARY KEY ("tenant_uuid", "field_key"),
  CONSTRAINT "tenant_field_configs_physical_col_unique" UNIQUE ("tenant_uuid", "physical_col"),
  CONSTRAINT "tenant_field_configs_data_source_config_is_object_check" CHECK (jsonb_typeof(data_source_config) = 'object'::text),
  CONSTRAINT "tenant_field_configs_data_source_type_check" CHECK (data_source_type = ANY (ARRAY['PLAIN'::text, 'DICT'::text, 'ENTITY'::text])),
  CONSTRAINT "tenant_field_configs_dict_config_check" CHECK ((data_source_type <> 'DICT'::text) OR ((value_type = 'text'::text) AND (data_source_config ? 'dict_code'::text) AND (jsonb_typeof((data_source_config -> 'dict_code'::text)) = 'string'::text) AND (NULLIF(btrim((data_source_config ->> 'dict_code'::text)), ''::text) IS NOT NULL) AND (data_source_config = jsonb_build_object('dict_code', (data_source_config -> 'dict_code'::text))))),
  CONSTRAINT "tenant_field_configs_disabled_on_check" CHECK ((disabled_on IS NULL) OR (disabled_on >= enabled_on)),
  CONSTRAINT "tenant_field_configs_entity_config_check" CHECK ((data_source_type <> 'ENTITY'::text) OR ((data_source_config ? 'entity'::text) AND (jsonb_typeof((data_source_config -> 'entity'::text)) = 'string'::text) AND (NULLIF(btrim((data_source_config ->> 'entity'::text)), ''::text) IS NOT NULL) AND (data_source_config ? 'id_kind'::text) AND (jsonb_typeof((data_source_config -> 'id_kind'::text)) = 'string'::text) AND ((data_source_config ->> 'id_kind'::text) = ANY (ARRAY['uuid'::text, 'int'::text])) AND ((((data_source_config ->> 'id_kind'::text) = 'uuid'::text) AND (value_type = 'uuid'::text)) OR (((data_source_config ->> 'id_kind'::text) = 'int'::text) AND (value_type = 'int'::text))) AND (data_source_config = jsonb_build_object('entity', (data_source_config -> 'entity'::text), 'id_kind', (data_source_config -> 'id_kind'::text))))),
  CONSTRAINT "tenant_field_configs_field_key_format_check" CHECK (field_key ~ '^[a-z][a-z0-9_]{0,62}$'::text),
  CONSTRAINT "tenant_field_configs_physical_col_format_check" CHECK (physical_col ~ '^ext_(str|int|uuid|bool|date)_[0-9]{2}$'::text),
  CONSTRAINT "tenant_field_configs_physical_col_group_check" CHECK (((value_type = 'text'::text) AND (physical_col ~~ 'ext_str_%'::text)) OR ((value_type = 'int'::text) AND (physical_col ~~ 'ext_int_%'::text)) OR ((value_type = 'uuid'::text) AND (physical_col ~~ 'ext_uuid_%'::text)) OR ((value_type = 'bool'::text) AND (physical_col ~~ 'ext_bool_%'::text)) OR ((value_type = 'date'::text) AND (physical_col ~~ 'ext_date_%'::text))),
  CONSTRAINT "tenant_field_configs_plain_config_check" CHECK ((data_source_type <> 'PLAIN'::text) OR (data_source_config = '{}'::jsonb)),
  CONSTRAINT "tenant_field_configs_value_type_check" CHECK (value_type = ANY (ARRAY['text'::text, 'int'::text, 'uuid'::text, 'bool'::text, 'date'::text]))
);

-- +goose Down
-- reverse: create "tenant_field_configs" table
DROP TABLE "orgunit"."tenant_field_configs";
-- reverse: create index "tenant_field_config_events_tenant_time_idx" to table: "tenant_field_config_events"
DROP INDEX "orgunit"."tenant_field_config_events_tenant_time_idx";
-- reverse: create "tenant_field_config_events" table
DROP TABLE "orgunit"."tenant_field_config_events";
-- reverse: create index "org_unit_versions_ext_str_05_idx" to table: "org_unit_versions"
DROP INDEX "orgunit"."org_unit_versions_ext_str_05_idx";
-- reverse: create index "org_unit_versions_ext_str_04_idx" to table: "org_unit_versions"
DROP INDEX "orgunit"."org_unit_versions_ext_str_04_idx";
-- reverse: create index "org_unit_versions_ext_str_03_idx" to table: "org_unit_versions"
DROP INDEX "orgunit"."org_unit_versions_ext_str_03_idx";
-- reverse: create index "org_unit_versions_ext_str_02_idx" to table: "org_unit_versions"
DROP INDEX "orgunit"."org_unit_versions_ext_str_02_idx";
-- reverse: create index "org_unit_versions_ext_str_01_idx" to table: "org_unit_versions"
DROP INDEX "orgunit"."org_unit_versions_ext_str_01_idx";
-- reverse: modify "org_unit_versions" table
ALTER TABLE "orgunit"."org_unit_versions" DROP COLUMN "ext_labels_snapshot", DROP COLUMN "ext_date_01", DROP COLUMN "ext_bool_01", DROP COLUMN "ext_uuid_01", DROP COLUMN "ext_int_01", DROP COLUMN "ext_str_05", DROP COLUMN "ext_str_04", DROP COLUMN "ext_str_03", DROP COLUMN "ext_str_02", DROP COLUMN "ext_str_01";
