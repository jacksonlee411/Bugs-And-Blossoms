-- +goose Up
ALTER TABLE staffing.payroll_run_events
  ALTER COLUMN run_state SET DEFAULT 'draft';

