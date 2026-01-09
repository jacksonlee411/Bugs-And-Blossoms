-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS staffing.payroll_recalc_requests (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  recalc_request_id uuid NOT NULL DEFAULT gen_random_uuid(),
  trigger_event_id uuid NOT NULL,
  trigger_source text NOT NULL DEFAULT 'assignment',
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  effective_date date NOT NULL,
  hit_pay_period_id uuid NOT NULL,
  hit_run_id uuid NOT NULL,
  hit_payslip_id uuid NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payroll_recalc_requests_trigger_source_check CHECK (trigger_source IN ('assignment')),
  CONSTRAINT payroll_recalc_requests_request_id_nonempty_check CHECK (btrim(request_id) <> ''),
  CONSTRAINT payroll_recalc_requests_tenant_recalc_request_id_unique UNIQUE (tenant_id, recalc_request_id),
  CONSTRAINT payroll_recalc_requests_trigger_event_unique UNIQUE (tenant_id, trigger_event_id),
  CONSTRAINT payroll_recalc_requests_assignment_fk FOREIGN KEY (tenant_id, assignment_id) REFERENCES staffing.assignments(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_recalc_requests_hit_period_fk FOREIGN KEY (tenant_id, hit_pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_recalc_requests_hit_run_fk FOREIGN KEY (tenant_id, hit_run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_recalc_requests_hit_payslip_fk FOREIGN KEY (tenant_id, hit_payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payroll_recalc_requests_tenant_created_idx
  ON staffing.payroll_recalc_requests (tenant_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS payroll_recalc_requests_tenant_person_effective_idx
  ON staffing.payroll_recalc_requests (tenant_id, person_uuid, effective_date, id);

CREATE TABLE IF NOT EXISTS staffing.payroll_recalc_applications (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  recalc_request_id uuid NOT NULL,
  target_run_id uuid NOT NULL,
  target_pay_period_id uuid NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payroll_recalc_applications_request_id_nonempty_check CHECK (btrim(request_id) <> ''),
  CONSTRAINT payroll_recalc_applications_event_id_unique UNIQUE (event_id),
  CONSTRAINT payroll_recalc_applications_one_per_request_unique UNIQUE (tenant_id, recalc_request_id),
  CONSTRAINT payroll_recalc_applications_request_fk FOREIGN KEY (tenant_id, recalc_request_id) REFERENCES staffing.payroll_recalc_requests(tenant_id, recalc_request_id) ON DELETE RESTRICT,
  CONSTRAINT payroll_recalc_applications_target_run_fk FOREIGN KEY (tenant_id, target_run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_recalc_applications_target_period_fk FOREIGN KEY (tenant_id, target_pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payroll_recalc_applications_tenant_created_idx
  ON staffing.payroll_recalc_applications (tenant_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS staffing.payroll_adjustments (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  application_id bigint NOT NULL REFERENCES staffing.payroll_recalc_applications(id) ON DELETE RESTRICT,
  recalc_request_id uuid NOT NULL,
  target_run_id uuid NOT NULL,
  target_pay_period_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  origin_pay_period_id uuid NOT NULL,
  origin_run_id uuid NOT NULL,
  origin_payslip_id uuid NULL,
  item_kind text NOT NULL,
  item_code text NOT NULL,
  amount numeric(15,2) NOT NULL,
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payroll_adjustments_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payroll_adjustments_item_code_nonempty_check CHECK (btrim(item_code) <> ''),
  CONSTRAINT payroll_adjustments_item_code_trim_check CHECK (item_code = btrim(item_code)),
  CONSTRAINT payroll_adjustments_item_code_upper_check CHECK (item_code = upper(item_code)),
  CONSTRAINT payroll_adjustments_meta_is_object_check CHECK (jsonb_typeof(meta) = 'object'),
  CONSTRAINT payroll_adjustments_request_fk FOREIGN KEY (tenant_id, recalc_request_id) REFERENCES staffing.payroll_recalc_requests(tenant_id, recalc_request_id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_target_run_fk FOREIGN KEY (tenant_id, target_run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_target_period_fk FOREIGN KEY (tenant_id, target_pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_assignment_fk FOREIGN KEY (tenant_id, assignment_id) REFERENCES staffing.assignments(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_origin_period_fk FOREIGN KEY (tenant_id, origin_pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_origin_run_fk FOREIGN KEY (tenant_id, origin_run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_origin_payslip_fk FOREIGN KEY (tenant_id, origin_payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payroll_adjustments_no_duplicate_per_apply_unique UNIQUE (
    tenant_id,
    application_id,
    person_uuid,
    assignment_id,
    origin_pay_period_id,
    item_kind,
    item_code
  )
);

CREATE INDEX IF NOT EXISTS payroll_adjustments_target_lookup_idx
  ON staffing.payroll_adjustments (tenant_id, target_run_id, person_uuid, assignment_id, id);

CREATE INDEX IF NOT EXISTS payroll_adjustments_origin_lookup_idx
  ON staffing.payroll_adjustments (tenant_id, origin_pay_period_id, person_uuid, assignment_id, id);

ALTER TABLE staffing.payroll_recalc_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_recalc_requests FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_recalc_requests;
CREATE POLICY tenant_isolation ON staffing.payroll_recalc_requests
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payroll_recalc_applications ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_recalc_applications FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_recalc_applications;
CREATE POLICY tenant_isolation ON staffing.payroll_recalc_applications
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.payroll_adjustments ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_adjustments FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_adjustments;
CREATE POLICY tenant_isolation ON staffing.payroll_adjustments
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_adjustments;
ALTER TABLE IF EXISTS staffing.payroll_adjustments DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS staffing.payroll_adjustments;

DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_recalc_applications;
ALTER TABLE IF EXISTS staffing.payroll_recalc_applications DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS staffing.payroll_recalc_applications;

DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_recalc_requests;
ALTER TABLE IF EXISTS staffing.payroll_recalc_requests DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS staffing.payroll_recalc_requests;
-- +goose StatementEnd

