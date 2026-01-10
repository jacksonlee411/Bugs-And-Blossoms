-- +goose Up
-- modify "daily_attendance_results" table
ALTER TABLE "staffing"."daily_attendance_results" DROP CONSTRAINT "daily_attendance_results_minutes_nonneg_check", ADD CONSTRAINT "daily_attendance_results_minutes_nonneg_check" CHECK ((scheduled_minutes >= 0) AND (worked_minutes >= 0) AND (late_minutes >= 0) AND (early_leave_minutes >= 0)), DROP CONSTRAINT "daily_attendance_results_overtime_nonneg_check", ADD CONSTRAINT "daily_attendance_results_overtime_nonneg_check" CHECK ((overtime_minutes_150 >= 0) AND (overtime_minutes_200 >= 0) AND (overtime_minutes_300 >= 0));
-- modify "time_punch_events" table
ALTER TABLE "staffing"."time_punch_events" DROP CONSTRAINT "time_punch_events_punch_type_check", ADD CONSTRAINT "time_punch_events_punch_type_check" CHECK (punch_type = ANY (ARRAY['IN'::text, 'OUT'::text, 'RAW'::text])), DROP CONSTRAINT "time_punch_events_source_provider_check", ADD CONSTRAINT "time_punch_events_source_provider_check" CHECK (source_provider = ANY (ARRAY['MANUAL'::text, 'IMPORT'::text, 'DINGTALK'::text, 'WECOM'::text]));

-- +goose Down
-- reverse: modify "time_punch_events" table
ALTER TABLE "staffing"."time_punch_events" DROP CONSTRAINT "time_punch_events_source_provider_check", ADD CONSTRAINT "time_punch_events_source_provider_check" CHECK (source_provider = ANY (ARRAY['MANUAL'::text, 'IMPORT'::text])), DROP CONSTRAINT "time_punch_events_punch_type_check", ADD CONSTRAINT "time_punch_events_punch_type_check" CHECK (punch_type = ANY (ARRAY['IN'::text, 'OUT'::text]));
-- reverse: modify "daily_attendance_results" table
ALTER TABLE "staffing"."daily_attendance_results" DROP CONSTRAINT "daily_attendance_results_overtime_nonneg_check", ADD CONSTRAINT "daily_attendance_results_overtime_nonneg_check" CHECK ((scheduled_minutes >= 0) AND (overtime_minutes_150 >= 0) AND (overtime_minutes_200 >= 0) AND (overtime_minutes_300 >= 0)), DROP CONSTRAINT "daily_attendance_results_minutes_nonneg_check", ADD CONSTRAINT "daily_attendance_results_minutes_nonneg_check" CHECK ((worked_minutes >= 0) AND (late_minutes >= 0) AND (early_leave_minutes >= 0));
