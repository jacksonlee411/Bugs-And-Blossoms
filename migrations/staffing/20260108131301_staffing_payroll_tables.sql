-- +goose Up
-- create "pay_period_events" table
CREATE TABLE "staffing"."pay_period_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "pay_period_id" uuid NOT NULL,
  "event_type" text NOT NULL,
  "pay_group" text NOT NULL,
  "period" daterange NOT NULL,
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "pay_period_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "pay_period_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "pay_period_events_event_type_check" CHECK (event_type = 'CREATE'::text),
  CONSTRAINT "pay_period_events_pay_group_lower_check" CHECK (pay_group = lower(pay_group)),
  CONSTRAINT "pay_period_events_pay_group_nonempty_check" CHECK (btrim(pay_group) <> ''::text),
  CONSTRAINT "pay_period_events_pay_group_trim_check" CHECK (pay_group = btrim(pay_group)),
  CONSTRAINT "pay_period_events_period_bounded_check" CHECK ((NOT lower_inf(period)) AND (NOT upper_inf(period))),
  CONSTRAINT "pay_period_events_period_bounds_check" CHECK (lower_inc(period) AND (NOT upper_inc(period))),
  CONSTRAINT "pay_period_events_period_check" CHECK (NOT isempty(period))
);
-- create index "pay_period_events_tenant_period_idx" to table: "pay_period_events"
CREATE INDEX "pay_period_events_tenant_period_idx" ON "staffing"."pay_period_events" ("tenant_id", "pay_group", (lower(period)), "id");
-- create "pay_periods" table
CREATE TABLE "staffing"."pay_periods" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "pay_group" text NOT NULL,
  "period" daterange NOT NULL,
  "status" text NOT NULL DEFAULT 'open',
  "closed_at" timestamptz NULL,
  "last_event_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id"),
  CONSTRAINT "pay_periods_no_overlap" EXCLUDE USING gist ("tenant_id" WITH =, "pay_group" WITH =, "period" WITH &&),
  CONSTRAINT "pay_periods_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."pay_period_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "pay_periods_pay_group_lower_check" CHECK (pay_group = lower(pay_group)),
  CONSTRAINT "pay_periods_pay_group_nonempty_check" CHECK (btrim(pay_group) <> ''::text),
  CONSTRAINT "pay_periods_pay_group_trim_check" CHECK (pay_group = btrim(pay_group)),
  CONSTRAINT "pay_periods_period_bounded_check" CHECK ((NOT lower_inf(period)) AND (NOT upper_inf(period))),
  CONSTRAINT "pay_periods_period_bounds_check" CHECK (lower_inc(period) AND (NOT upper_inc(period))),
  CONSTRAINT "pay_periods_period_check" CHECK (NOT isempty(period)),
  CONSTRAINT "pay_periods_status_check" CHECK (status = ANY (ARRAY['open'::text, 'closed'::text]))
);
-- create index "pay_periods_lookup_btree" to table: "pay_periods"
CREATE INDEX "pay_periods_lookup_btree" ON "staffing"."pay_periods" ("tenant_id", "pay_group", (lower(period)) DESC);
-- create "payroll_run_events" table
CREATE TABLE "staffing"."payroll_run_events" (
  "id" bigserial NOT NULL,
  "event_id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "run_id" uuid NOT NULL,
  "pay_period_id" uuid NOT NULL,
  "event_type" text NOT NULL,
  "run_state" text NOT NULL,
  "payload" jsonb NOT NULL DEFAULT '{}',
  "request_id" text NOT NULL,
  "initiator_id" uuid NOT NULL,
  "transaction_time" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "payroll_run_events_event_id_unique" UNIQUE ("event_id"),
  CONSTRAINT "payroll_run_events_request_id_unique" UNIQUE ("tenant_id", "request_id"),
  CONSTRAINT "payroll_run_events_event_type_check" CHECK (event_type = ANY (ARRAY['CREATE'::text, 'CALC_START'::text, 'CALC_FINISH'::text, 'CALC_FAIL'::text, 'FINALIZE'::text])),
  CONSTRAINT "payroll_run_events_payload_is_object_check" CHECK (jsonb_typeof(payload) = 'object'::text),
  CONSTRAINT "payroll_run_events_run_state_check" CHECK (run_state = ANY (ARRAY['draft'::text, 'calculating'::text, 'calculated'::text, 'failed'::text, 'finalized'::text]))
);
-- create index "payroll_run_events_tenant_run_idx" to table: "payroll_run_events"
CREATE INDEX "payroll_run_events_tenant_run_idx" ON "staffing"."payroll_run_events" ("tenant_id", "run_id", "id");
-- create "payroll_runs" table
CREATE TABLE "staffing"."payroll_runs" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "pay_period_id" uuid NOT NULL,
  "run_state" text NOT NULL DEFAULT 'draft',
  "needs_recalc" boolean NOT NULL DEFAULT false,
  "calc_started_at" timestamptz NULL,
  "calc_finished_at" timestamptz NULL,
  "finalized_at" timestamptz NULL,
  "last_event_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id"),
  CONSTRAINT "payroll_runs_last_event_id_fkey" FOREIGN KEY ("last_event_id") REFERENCES "staffing"."payroll_run_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payroll_runs_pay_period_fk" FOREIGN KEY ("tenant_id", "pay_period_id") REFERENCES "staffing"."pay_periods" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payroll_runs_run_state_check" CHECK (run_state = ANY (ARRAY['draft'::text, 'calculating'::text, 'calculated'::text, 'failed'::text, 'finalized'::text]))
);
-- create index "payroll_runs_by_period_btree" to table: "payroll_runs"
CREATE INDEX "payroll_runs_by_period_btree" ON "staffing"."payroll_runs" ("tenant_id", "pay_period_id", "created_at" DESC, "id");
-- create index "payroll_runs_one_finalized_per_period_unique" to table: "payroll_runs"
CREATE UNIQUE INDEX "payroll_runs_one_finalized_per_period_unique" ON "staffing"."payroll_runs" ("tenant_id", "pay_period_id") WHERE (run_state = 'finalized'::text);
-- create "payslips" table
CREATE TABLE "staffing"."payslips" (
  "tenant_id" uuid NOT NULL,
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "run_id" uuid NOT NULL,
  "pay_period_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "assignment_id" uuid NOT NULL,
  "currency" character(3) NOT NULL DEFAULT 'CNY',
  "gross_pay" numeric(15,2) NOT NULL DEFAULT 0,
  "net_pay" numeric(15,2) NOT NULL DEFAULT 0,
  "employer_total" numeric(15,2) NOT NULL DEFAULT 0,
  "last_run_event_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "id"),
  CONSTRAINT "payslips_run_person_assignment_unique" UNIQUE ("tenant_id", "run_id", "person_uuid", "assignment_id"),
  CONSTRAINT "payslips_last_run_event_id_fkey" FOREIGN KEY ("last_run_event_id") REFERENCES "staffing"."payroll_run_events" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payslips_period_fk" FOREIGN KEY ("tenant_id", "pay_period_id") REFERENCES "staffing"."pay_periods" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslips_run_fk" FOREIGN KEY ("tenant_id", "run_id") REFERENCES "staffing"."payroll_runs" ("tenant_id", "id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "payslips_currency_check" CHECK (((currency)::text = btrim((currency)::text)) AND ((currency)::text = upper((currency)::text)))
);
-- create index "payslips_by_run_btree" to table: "payslips"
CREATE INDEX "payslips_by_run_btree" ON "staffing"."payslips" ("tenant_id", "run_id", "person_uuid", "assignment_id");

ALTER TABLE staffing.pay_period_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.pay_period_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.pay_period_events;
CREATE POLICY tenant_isolation ON staffing.pay_period_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.pay_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.pay_periods FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.pay_periods;
CREATE POLICY tenant_isolation ON staffing.pay_periods
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payroll_run_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_run_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_run_events;
CREATE POLICY tenant_isolation ON staffing.payroll_run_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payroll_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_runs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_runs;
CREATE POLICY tenant_isolation ON staffing.payroll_runs
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payslips ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslips FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslips;
CREATE POLICY tenant_isolation ON staffing.payslips
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose Down
-- reverse: create index "payslips_by_run_btree" to table: "payslips"
DROP INDEX "staffing"."payslips_by_run_btree";
-- reverse: create "payslips" table
DROP TABLE "staffing"."payslips";
-- reverse: create index "payroll_runs_one_finalized_per_period_unique" to table: "payroll_runs"
DROP INDEX "staffing"."payroll_runs_one_finalized_per_period_unique";
-- reverse: create index "payroll_runs_by_period_btree" to table: "payroll_runs"
DROP INDEX "staffing"."payroll_runs_by_period_btree";
-- reverse: create "payroll_runs" table
DROP TABLE "staffing"."payroll_runs";
-- reverse: create index "payroll_run_events_tenant_run_idx" to table: "payroll_run_events"
DROP INDEX "staffing"."payroll_run_events_tenant_run_idx";
-- reverse: create "payroll_run_events" table
DROP TABLE "staffing"."payroll_run_events";
-- reverse: create index "pay_periods_lookup_btree" to table: "pay_periods"
DROP INDEX "staffing"."pay_periods_lookup_btree";
-- reverse: create "pay_periods" table
DROP TABLE "staffing"."pay_periods";
-- reverse: create index "pay_period_events_tenant_period_idx" to table: "pay_period_events"
DROP INDEX "staffing"."pay_period_events_tenant_period_idx";
-- reverse: create "pay_period_events" table
DROP TABLE "staffing"."pay_period_events";
