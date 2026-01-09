-- +goose Up
-- modify "payslip_items" table
ALTER TABLE "staffing"."payslip_items" ADD CONSTRAINT "payslip_items_calc_mode_check" CHECK (calc_mode = ANY (ARRAY['amount'::text, 'net_guaranteed_iit'::text])), ADD CONSTRAINT "payslip_items_iit_delta_nonneg_check" CHECK ((iit_delta IS NULL) OR (iit_delta >= (0)::numeric)), ADD CONSTRAINT "payslip_items_net_guaranteed_contract_check" CHECK ((calc_mode <> 'net_guaranteed_iit'::text) OR ((tax_bearer = 'employer'::text) AND (item_kind = 'earning'::text) AND (target_net IS NOT NULL) AND (iit_delta IS NOT NULL) AND (amount = (target_net + iit_delta)))), ADD CONSTRAINT "payslip_items_target_net_positive_check" CHECK ((target_net IS NULL) OR (target_net > (0)::numeric)), ADD CONSTRAINT "payslip_items_tax_bearer_check" CHECK (tax_bearer = ANY (ARRAY['employee'::text, 'employer'::text])), ADD COLUMN "calc_mode" text NOT NULL DEFAULT 'amount', ADD COLUMN "tax_bearer" text NOT NULL DEFAULT 'employee', ADD COLUMN "target_net" numeric(15,2) NULL, ADD COLUMN "iit_delta" numeric(15,2) NULL;
-- create "payslip_item_input_events" table
CREATE TABLE "staffing"."payslip_item_input_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "run_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "event_type" text NOT NULL,
  "item_code" text NOT NULL,
  "item_kind" text NOT NULL,
  "currency" character(3) NOT NULL DEFAULT 'CNY',
  "calc_mode" text NOT NULL,
  "tax_bearer" text NOT NULL,
  "amount" numeric(15,2) NOT NULL,
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "payslip_item_input_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "payslip_item_input_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "payslip_item_input_events_run_fk" FOREIGN KEY ("tenant_id", "run_id") REFERENCES "staffing"."payroll_runs" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslip_item_input_events_amount_positive_check" CHECK (amount > (0)::numeric),
  CONSTRAINT "payslip_item_input_events_calc_mode_check" CHECK (calc_mode = ANY (ARRAY['amount'::text, 'net_guaranteed_iit'::text])),
  CONSTRAINT "payslip_item_input_events_code_check" CHECK ((btrim(item_code) <> ''::text) AND (item_code = btrim(item_code)) AND (item_code = upper(item_code)) AND (item_code ~ '^[A-Z0-9_]+$'::text)),
  CONSTRAINT "payslip_item_input_events_currency_check" CHECK (((currency)::text = btrim((currency)::text)) AND ((currency)::text = upper((currency)::text))),
  CONSTRAINT "payslip_item_input_events_event_type_check" CHECK (event_type = ANY (ARRAY['UPSERT'::text, 'DELETE'::text])),
  CONSTRAINT "payslip_item_input_events_item_kind_check" CHECK (item_kind = ANY (ARRAY['earning'::text, 'deduction'::text, 'employer_cost'::text])),
  CONSTRAINT "payslip_item_input_events_net_guaranteed_contract_check" CHECK ((calc_mode <> 'net_guaranteed_iit'::text) OR ((item_kind = 'earning'::text) AND (tax_bearer = 'employer'::text) AND (currency = 'CNY'::bpchar))),
  CONSTRAINT "payslip_item_input_events_tax_bearer_check" CHECK (tax_bearer = ANY (ARRAY['employee'::text, 'employer'::text]))
);
-- create index "payslip_item_input_events_lookup_btree" to table: "payslip_item_input_events"
CREATE INDEX "payslip_item_input_events_lookup_btree" ON "staffing"."payslip_item_input_events" ("tenant_id", "run_id", "person_uuid", "assignment_id", "item_code", "id");
-- create "payslip_item_inputs" table
CREATE TABLE "staffing"."payslip_item_inputs" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "run_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "item_code" text NOT NULL,
  "item_kind" text NOT NULL,
  "currency" character(3) NOT NULL DEFAULT 'CNY',
  "calc_mode" text NOT NULL,
  "tax_bearer" text NOT NULL,
  "amount" numeric(15,2) NOT NULL,
  "last_event_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id"),
  CONSTRAINT "payslip_item_inputs_natural_unique" UNIQUE ("tenant_id", "run_id", "person_uuid", "assignment_id", "item_code"),
  CONSTRAINT "payslip_item_inputs_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."payslip_item_input_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payslip_item_inputs_run_fk" FOREIGN KEY ("tenant_id", "run_id") REFERENCES "staffing"."payroll_runs" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslip_item_inputs_amount_positive_check" CHECK (amount > (0)::numeric),
  CONSTRAINT "payslip_item_inputs_calc_mode_check" CHECK (calc_mode = ANY (ARRAY['amount'::text, 'net_guaranteed_iit'::text])),
  CONSTRAINT "payslip_item_inputs_code_check" CHECK ((btrim(item_code) <> ''::text) AND (item_code = btrim(item_code)) AND (item_code = upper(item_code)) AND (item_code ~ '^[A-Z0-9_]+$'::text)),
  CONSTRAINT "payslip_item_inputs_currency_check" CHECK (((currency)::text = btrim((currency)::text)) AND ((currency)::text = upper((currency)::text))),
  CONSTRAINT "payslip_item_inputs_item_kind_check" CHECK (item_kind = ANY (ARRAY['earning'::text, 'deduction'::text, 'employer_cost'::text])),
  CONSTRAINT "payslip_item_inputs_net_guaranteed_contract_check" CHECK ((calc_mode <> 'net_guaranteed_iit'::text) OR ((item_kind = 'earning'::text) AND (tax_bearer = 'employer'::text) AND (currency = 'CNY'::bpchar))),
  CONSTRAINT "payslip_item_inputs_tax_bearer_check" CHECK (tax_bearer = ANY (ARRAY['employee'::text, 'employer'::text]))
);
-- create index "payslip_item_inputs_by_run_person_btree" to table: "payslip_item_inputs"
CREATE INDEX "payslip_item_inputs_by_run_person_btree" ON "staffing"."payslip_item_inputs" ("tenant_id", "run_id", "person_uuid", "assignment_id", "item_code");

ALTER TABLE staffing.payslip_item_input_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_item_input_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_item_input_events;
CREATE POLICY tenant_isolation ON staffing.payslip_item_input_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payslip_item_inputs ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_item_inputs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_item_inputs;
CREATE POLICY tenant_isolation ON staffing.payslip_item_inputs
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose Down
-- reverse: create index "payslip_item_inputs_by_run_person_btree" to table: "payslip_item_inputs"
DROP INDEX "staffing"."payslip_item_inputs_by_run_person_btree";
-- reverse: create "payslip_item_inputs" table
DROP TABLE "staffing"."payslip_item_inputs";
-- reverse: create index "payslip_item_input_events_lookup_btree" to table: "payslip_item_input_events"
DROP INDEX "staffing"."payslip_item_input_events_lookup_btree";
-- reverse: create "payslip_item_input_events" table
DROP TABLE "staffing"."payslip_item_input_events";
-- reverse: modify "payslip_items" table
ALTER TABLE "staffing"."payslip_items" DROP COLUMN "iit_delta", DROP COLUMN "target_net", DROP COLUMN "tax_bearer", DROP COLUMN "calc_mode", DROP CONSTRAINT "payslip_items_tax_bearer_check", DROP CONSTRAINT "payslip_items_target_net_positive_check", DROP CONSTRAINT "payslip_items_net_guaranteed_contract_check", DROP CONSTRAINT "payslip_items_iit_delta_nonneg_check", DROP CONSTRAINT "payslip_items_calc_mode_check";
