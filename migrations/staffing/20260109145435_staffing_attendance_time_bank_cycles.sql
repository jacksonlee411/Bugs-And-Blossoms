-- +goose Up
-- modify "daily_attendance_results" table
ALTER TABLE "staffing"."daily_attendance_results" DROP CONSTRAINT "daily_attendance_results_minutes_nonneg_check", ADD CONSTRAINT "daily_attendance_results_minutes_nonneg_check" CHECK ((scheduled_minutes >= 0) AND (worked_minutes >= 0) AND (late_minutes >= 0) AND (early_leave_minutes >= 0)), DROP CONSTRAINT "daily_attendance_results_overtime_nonneg_check", ADD CONSTRAINT "daily_attendance_results_overtime_nonneg_check" CHECK ((overtime_minutes_150 >= 0) AND (overtime_minutes_200 >= 0) AND (overtime_minutes_300 >= 0));
-- create "time_bank_cycles" table
CREATE TABLE "staffing"."time_bank_cycles" (
  "tenant_id" uuid NOT NULL,
  "person_uuid" uuid NOT NULL,
  "cycle_type" text NOT NULL,
  "cycle_start_date" date NOT NULL,
  "cycle_end_date" date NOT NULL,
  "ruleset_version" text NOT NULL,
  "worked_minutes_total" integer NOT NULL DEFAULT 0,
  "overtime_minutes_150" integer NOT NULL DEFAULT 0,
  "overtime_minutes_200" integer NOT NULL DEFAULT 0,
  "overtime_minutes_300" integer NOT NULL DEFAULT 0,
  "comp_earned_minutes" integer NOT NULL DEFAULT 0,
  "comp_used_minutes" integer NOT NULL DEFAULT 0,
  "input_max_punch_event_db_id" bigint NULL,
  "input_max_punch_time" timestamptz NULL,
  "computed_at" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("tenant_id", "person_uuid", "cycle_type", "cycle_start_date"),
  CONSTRAINT "time_bank_cycles_cycle_bounds_check" CHECK (cycle_end_date >= cycle_start_date),
  CONSTRAINT "time_bank_cycles_cycle_type_check" CHECK (cycle_type = 'MONTH'::text),
  CONSTRAINT "time_bank_cycles_minutes_nonneg_check" CHECK ((worked_minutes_total >= 0) AND (overtime_minutes_150 >= 0) AND (overtime_minutes_200 >= 0) AND (overtime_minutes_300 >= 0) AND (comp_earned_minutes >= 0) AND (comp_used_minutes >= 0))
);
-- create index "time_bank_cycles_lookup_idx" to table: "time_bank_cycles"
CREATE INDEX "time_bank_cycles_lookup_idx" ON "staffing"."time_bank_cycles" ("tenant_id", "person_uuid", "cycle_start_date" DESC);
-- enable rls + tenant isolation policy
ALTER TABLE "staffing"."time_bank_cycles" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "staffing"."time_bank_cycles" FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."time_bank_cycles";
CREATE POLICY tenant_isolation ON "staffing"."time_bank_cycles"
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- +goose Down
-- reverse: enable rls + tenant isolation policy
DROP POLICY IF EXISTS tenant_isolation ON "staffing"."time_bank_cycles";
ALTER TABLE IF EXISTS "staffing"."time_bank_cycles" DISABLE ROW LEVEL SECURITY;
-- reverse: create index "time_bank_cycles_lookup_idx" to table: "time_bank_cycles"
DROP INDEX "staffing"."time_bank_cycles_lookup_idx";
-- reverse: create "time_bank_cycles" table
DROP TABLE "staffing"."time_bank_cycles";
-- reverse: modify "daily_attendance_results" table
ALTER TABLE "staffing"."daily_attendance_results" DROP CONSTRAINT "daily_attendance_results_overtime_nonneg_check", ADD CONSTRAINT "daily_attendance_results_overtime_nonneg_check" CHECK ((scheduled_minutes >= 0) AND (overtime_minutes_150 >= 0) AND (overtime_minutes_200 >= 0) AND (overtime_minutes_300 >= 0)), DROP CONSTRAINT "daily_attendance_results_minutes_nonneg_check", ADD CONSTRAINT "daily_attendance_results_minutes_nonneg_check" CHECK ((worked_minutes >= 0) AND (late_minutes >= 0) AND (early_leave_minutes >= 0));
