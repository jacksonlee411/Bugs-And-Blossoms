-- +goose Up
-- modify "tenant_field_config_events" table
ALTER TABLE "orgunit"."tenant_field_config_events" DROP CONSTRAINT "tenant_field_config_events_event_type_check", ADD CONSTRAINT "tenant_field_config_events_event_type_check" CHECK (event_type = ANY (ARRAY['ENABLE'::text, 'DISABLE'::text, 'REKEY'::text]));
-- modify "tenant_field_configs" table
ALTER TABLE "orgunit"."tenant_field_configs" ADD CONSTRAINT "tenant_field_configs_display_label_check" CHECK ((display_label IS NULL) OR ((display_label = btrim(display_label)) AND (display_label <> ''::text))), ADD COLUMN "display_label" text NULL;

-- +goose Down
-- reverse: modify "tenant_field_configs" table
ALTER TABLE "orgunit"."tenant_field_configs" DROP COLUMN "display_label", DROP CONSTRAINT "tenant_field_configs_display_label_check";
-- reverse: modify "tenant_field_config_events" table
ALTER TABLE "orgunit"."tenant_field_config_events" DROP CONSTRAINT "tenant_field_config_events_event_type_check", ADD CONSTRAINT "tenant_field_config_events_event_type_check" CHECK (event_type = ANY (ARRAY['ENABLE'::text, 'DISABLE'::text]));

