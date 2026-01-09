-- +goose Up
-- create "social_insurance_policies" table
CREATE TABLE "staffing"."social_insurance_policies" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "city_code" text NOT NULL,
  "hukou_type" text NOT NULL,
  "insurance_type" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id"),
  CONSTRAINT "social_insurance_policies_identity_unique" UNIQUE ("tenant_id", "city_code", "hukou_type", "insurance_type"),
  CONSTRAINT "social_insurance_policies_city_code_nonempty_check" CHECK (btrim(city_code) <> ''::text),
  CONSTRAINT "social_insurance_policies_city_code_trim_check" CHECK (city_code = btrim(city_code)),
  CONSTRAINT "social_insurance_policies_city_code_upper_check" CHECK (city_code = upper(city_code)),
  CONSTRAINT "social_insurance_policies_hukou_type_nonempty_check" CHECK (btrim(hukou_type) <> ''::text),
  CONSTRAINT "social_insurance_policies_hukou_type_trim_check" CHECK (hukou_type = btrim(hukou_type)),
  CONSTRAINT "social_insurance_policies_hukou_type_lower_check" CHECK (hukou_type = lower(hukou_type)),
  CONSTRAINT "social_insurance_policies_insurance_type_check" CHECK ((insurance_type)::text = ANY (ARRAY['PENSION'::text, 'MEDICAL'::text, 'UNEMPLOYMENT'::text, 'INJURY'::text, 'MATERNITY'::text, 'HOUSING_FUND'::text]))
);
-- create index "social_insurance_policies_lookup_btree" to table: "social_insurance_policies"
CREATE INDEX "social_insurance_policies_lookup_btree" ON "staffing"."social_insurance_policies" ("tenant_id", "city_code", "hukou_type", "insurance_type");

-- create "social_insurance_policy_events" table
CREATE TABLE "staffing"."social_insurance_policy_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "policy_id" uuid NOT NULL,
  "city_code" text NOT NULL,
  "hukou_type" text NOT NULL,
  "insurance_type" text NOT NULL,
  "event_type" text NOT NULL,
  "effective_date" date NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "social_insurance_policy_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "social_insurance_policy_events_one_per_day_unique" UNIQUE ("tenant_id", "policy_id", "effective_date"),
  CONSTRAINT "social_insurance_policy_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "social_insurance_policy_events_event_type_check" CHECK ((event_type)::text = ANY (ARRAY['CREATE'::text, 'UPDATE'::text])),
  CONSTRAINT "social_insurance_policy_events_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text),
  CONSTRAINT "social_insurance_policy_events_city_code_trim_check" CHECK (city_code = btrim(city_code)),
  CONSTRAINT "social_insurance_policy_events_city_code_upper_check" CHECK (city_code = upper(city_code)),
  CONSTRAINT "social_insurance_policy_events_hukou_type_trim_check" CHECK (hukou_type = btrim(hukou_type)),
  CONSTRAINT "social_insurance_policy_events_hukou_type_lower_check" CHECK (hukou_type = lower(hukou_type)),
  CONSTRAINT "social_insurance_policy_events_insurance_type_check" CHECK ((insurance_type)::text = ANY (ARRAY['PENSION'::text, 'MEDICAL'::text, 'UNEMPLOYMENT'::text, 'INJURY'::text, 'MATERNITY'::text, 'HOUSING_FUND'::text])),
  CONSTRAINT "social_insurance_policy_events_policy_fk" FOREIGN KEY ("tenant_id", "policy_id") REFERENCES "staffing"."social_insurance_policies" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT
);
-- create index "social_insurance_policy_events_tenant_policy_effective_idx" to table: "social_insurance_policy_events"
CREATE INDEX "social_insurance_policy_events_tenant_policy_effective_idx" ON "staffing"."social_insurance_policy_events" ("tenant_id", "policy_id", "effective_date", "id");

-- create "social_insurance_policy_versions" table
CREATE TABLE "staffing"."social_insurance_policy_versions" (
  "id" bigserial NOT NULL,
  "tenant_id" uuid NOT NULL,
  "policy_id" uuid NOT NULL,
  "city_code" text NOT NULL,
  "hukou_type" text NOT NULL,
  "insurance_type" text NOT NULL,
  "employer_rate" numeric(9,6) NOT NULL,
  "employee_rate" numeric(9,6) NOT NULL,
  "base_floor" numeric(15,2) NOT NULL,
  "base_ceiling" numeric(15,2) NOT NULL,
  "rounding_rule" text NOT NULL,
  "precision" smallint NOT NULL DEFAULT 2,
  "rules_config" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "validity" daterange NOT NULL,
  "last_event_id" bigint NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "social_insurance_policy_versions_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."social_insurance_policy_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "social_insurance_policy_versions_policy_fk" FOREIGN KEY ("tenant_id", "policy_id") REFERENCES "staffing"."social_insurance_policies" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "social_insurance_policy_versions_no_overlap" EXCLUDE USING gist ("tenant_id" WITH =, "policy_id" WITH =, "validity" WITH &&),
  CONSTRAINT "social_insurance_policy_versions_rules_is_object_check" CHECK (jsonb_typeof(rules_config) = 'object'::text),
  CONSTRAINT "social_insurance_policy_versions_rate_check" CHECK ((employer_rate >= (0)::numeric) AND (employer_rate <= (1)::numeric) AND (employee_rate >= (0)::numeric) AND (employee_rate <= (1)::numeric)),
  CONSTRAINT "social_insurance_policy_versions_base_check" CHECK ((base_floor >= (0)::numeric) AND (base_ceiling >= base_floor)),
  CONSTRAINT "social_insurance_policy_versions_rounding_rule_check" CHECK ((rounding_rule)::text = ANY (ARRAY['HALF_UP'::text, 'CEIL'::text])),
  CONSTRAINT "social_insurance_policy_versions_precision_check" CHECK ((precision >= 0) AND (precision <= 2)),
  CONSTRAINT "social_insurance_policy_versions_validity_check" CHECK (NOT isempty(validity)),
  CONSTRAINT "social_insurance_policy_versions_validity_bounds_check" CHECK (lower_inc(validity) AND (NOT upper_inc(validity)))
);
-- create index "social_insurance_policy_versions_lookup_btree" to table: "social_insurance_policy_versions"
CREATE INDEX "social_insurance_policy_versions_lookup_btree" ON "staffing"."social_insurance_policy_versions" ("tenant_id", "policy_id", (lower(validity)));

-- create "payslip_social_insurance_items" table
CREATE TABLE "staffing"."payslip_social_insurance_items" (
  "id" bigserial NOT NULL,
  "tenant_id" uuid NOT NULL,
  "payslip_id" uuid NOT NULL,
  "run_id" uuid NOT NULL,
  "pay_period_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "city_code" text NOT NULL,
  "hukou_type" text NOT NULL,
  "insurance_type" text NOT NULL,
  "base_amount" numeric(15,2) NOT NULL,
  "employee_amount" numeric(15,2) NOT NULL,
  "employer_amount" numeric(15,2) NOT NULL,
  "currency" character(3) NOT NULL DEFAULT 'CNY'::bpchar,
  "policy_id" uuid NOT NULL,
  "policy_last_event_id" bigint NOT NULL,
  "last_run_event_id" bigint NOT NULL,
  "meta" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "payslip_social_insurance_items_payslip_fk" FOREIGN KEY ("tenant_id", "payslip_id") REFERENCES "staffing"."payslips" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "payslip_social_insurance_items_run_fk" FOREIGN KEY ("tenant_id", "run_id") REFERENCES "staffing"."payroll_runs" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslip_social_insurance_items_period_fk" FOREIGN KEY ("tenant_id", "pay_period_id") REFERENCES "staffing"."pay_periods" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslip_social_insurance_items_policy_fk" FOREIGN KEY ("tenant_id", "policy_id") REFERENCES "staffing"."social_insurance_policies" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslip_social_insurance_items_policy_last_event_id_fkey" FOREIGN KEY ("policy_last_event_id") REFERENCES "staffing"."social_insurance_policy_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payslip_social_insurance_items_last_run_event_id_fkey" FOREIGN KEY ("last_run_event_id") REFERENCES "staffing"."payroll_run_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payslip_social_insurance_items_identity_unique" UNIQUE ("tenant_id", "payslip_id", "insurance_type"),
  CONSTRAINT "payslip_social_insurance_items_currency_check" CHECK (((currency)::text = btrim((currency)::text)) AND ((currency)::text = upper((currency)::text))),
  CONSTRAINT "payslip_social_insurance_items_meta_is_object_check" CHECK (jsonb_typeof(meta) = 'object'::text),
  CONSTRAINT "payslip_social_insurance_items_amounts_check" CHECK ((base_amount >= (0)::numeric) AND (employee_amount >= (0)::numeric) AND (employer_amount >= (0)::numeric)),
  CONSTRAINT "payslip_social_insurance_items_insurance_type_check" CHECK ((insurance_type)::text = ANY (ARRAY['PENSION'::text, 'MEDICAL'::text, 'UNEMPLOYMENT'::text, 'INJURY'::text, 'MATERNITY'::text, 'HOUSING_FUND'::text]))
);
-- create index "payslip_social_insurance_items_by_run_btree" to table: "payslip_social_insurance_items"
CREATE INDEX "payslip_social_insurance_items_by_run_btree" ON "staffing"."payslip_social_insurance_items" ("tenant_id", "run_id", "person_uuid", "assignment_id", "insurance_type");

ALTER TABLE staffing.social_insurance_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.social_insurance_policies FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.social_insurance_policies;
CREATE POLICY tenant_isolation ON staffing.social_insurance_policies
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.social_insurance_policy_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.social_insurance_policy_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.social_insurance_policy_events;
CREATE POLICY tenant_isolation ON staffing.social_insurance_policy_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.social_insurance_policy_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.social_insurance_policy_versions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.social_insurance_policy_versions;
CREATE POLICY tenant_isolation ON staffing.social_insurance_policy_versions
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payslip_social_insurance_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_social_insurance_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_social_insurance_items;
CREATE POLICY tenant_isolation ON staffing.payslip_social_insurance_items
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose Down
-- reverse: create policy "tenant_isolation" to table: "payslip_social_insurance_items"
DROP POLICY IF EXISTS "tenant_isolation" ON "staffing"."payslip_social_insurance_items";
-- reverse: enable row level security to table: "payslip_social_insurance_items"
ALTER TABLE IF EXISTS "staffing"."payslip_social_insurance_items" DISABLE ROW LEVEL SECURITY;
-- reverse: create index "payslip_social_insurance_items_by_run_btree" to table: "payslip_social_insurance_items"
DROP INDEX IF EXISTS "staffing"."payslip_social_insurance_items_by_run_btree";
-- reverse: create "payslip_social_insurance_items" table
DROP TABLE IF EXISTS "staffing"."payslip_social_insurance_items";

-- reverse: create policy "tenant_isolation" to table: "social_insurance_policy_versions"
DROP POLICY IF EXISTS "tenant_isolation" ON "staffing"."social_insurance_policy_versions";
-- reverse: enable row level security to table: "social_insurance_policy_versions"
ALTER TABLE IF EXISTS "staffing"."social_insurance_policy_versions" DISABLE ROW LEVEL SECURITY;
-- reverse: create index "social_insurance_policy_versions_lookup_btree" to table: "social_insurance_policy_versions"
DROP INDEX IF EXISTS "staffing"."social_insurance_policy_versions_lookup_btree";
-- reverse: create "social_insurance_policy_versions" table
DROP TABLE IF EXISTS "staffing"."social_insurance_policy_versions";

-- reverse: create policy "tenant_isolation" to table: "social_insurance_policy_events"
DROP POLICY IF EXISTS "tenant_isolation" ON "staffing"."social_insurance_policy_events";
-- reverse: enable row level security to table: "social_insurance_policy_events"
ALTER TABLE IF EXISTS "staffing"."social_insurance_policy_events" DISABLE ROW LEVEL SECURITY;
-- reverse: create index "social_insurance_policy_events_tenant_policy_effective_idx" to table: "social_insurance_policy_events"
DROP INDEX IF EXISTS "staffing"."social_insurance_policy_events_tenant_policy_effective_idx";
-- reverse: create "social_insurance_policy_events" table
DROP TABLE IF EXISTS "staffing"."social_insurance_policy_events";

-- reverse: create policy "tenant_isolation" to table: "social_insurance_policies"
DROP POLICY IF EXISTS "tenant_isolation" ON "staffing"."social_insurance_policies";
-- reverse: enable row level security to table: "social_insurance_policies"
ALTER TABLE IF EXISTS "staffing"."social_insurance_policies" DISABLE ROW LEVEL SECURITY;
-- reverse: create index "social_insurance_policies_lookup_btree" to table: "social_insurance_policies"
DROP INDEX IF EXISTS "staffing"."social_insurance_policies_lookup_btree";
-- reverse: create "social_insurance_policies" table
DROP TABLE IF EXISTS "staffing"."social_insurance_policies";
