-- +goose Up
-- create "external_identity_links" table
CREATE TABLE "person"."external_identity_links" (
  "tenant_id" uuid NOT NULL,
  "provider" text NOT NULL,
  "external_user_id" text NOT NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "person_uuid" uuid NULL,
  "first_seen_at" timestamptz NOT NULL DEFAULT now(),
  "last_seen_at" timestamptz NOT NULL DEFAULT now(),
  "seen_count" integer NOT NULL DEFAULT 1,
  "last_seen_payload" jsonb NOT NULL DEFAULT '{}',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "provider", "external_user_id"),
  CONSTRAINT "external_identity_links_external_user_id_nonempty_check" CHECK (btrim(external_user_id) <> ''::text),
  CONSTRAINT "external_identity_links_external_user_id_trim_check" CHECK (external_user_id = btrim(external_user_id)),
  CONSTRAINT "external_identity_links_last_seen_payload_is_object_check" CHECK (jsonb_typeof(last_seen_payload) = 'object'::text),
  CONSTRAINT "external_identity_links_provider_check" CHECK (provider = ANY (ARRAY['DINGTALK'::text, 'WECOM'::text])),
  CONSTRAINT "external_identity_links_status_check" CHECK (status = ANY (ARRAY['pending'::text, 'active'::text, 'disabled'::text, 'ignored'::text])),
  CONSTRAINT "external_identity_links_status_person_uuid_check" CHECK (((status = ANY (ARRAY['pending'::text, 'ignored'::text])) AND (person_uuid IS NULL)) OR ((status = ANY (ARRAY['active'::text, 'disabled'::text])) AND (person_uuid IS NOT NULL)))
);
-- create index "external_identity_links_lookup_idx" to table: "external_identity_links"
CREATE INDEX "external_identity_links_lookup_idx" ON "person"."external_identity_links" ("tenant_id", "provider", "status", "last_seen_at" DESC);
ALTER TABLE "person"."external_identity_links" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "person"."external_identity_links" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "person"."external_identity_links";
CREATE POLICY tenant_isolation ON "person"."external_identity_links"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose Down
-- reverse: create index "external_identity_links_lookup_idx" to table: "external_identity_links"
DROP INDEX "person"."external_identity_links_lookup_idx";
-- reverse: create "external_identity_links" table
DROP POLICY IF EXISTS tenant_isolation ON "person"."external_identity_links";
ALTER TABLE IF EXISTS "person"."external_identity_links" DISABLE ROW LEVEL SECURITY;
DROP TABLE "person"."external_identity_links";
