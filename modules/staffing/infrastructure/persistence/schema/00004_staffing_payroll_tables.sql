-- Payroll (P0-1) tables + RLS

CREATE TABLE IF NOT EXISTS staffing.pay_period_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  event_type text NOT NULL,
  pay_group text NOT NULL,
  period daterange NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pay_period_events_event_type_check CHECK (event_type IN ('CREATE')),
  CONSTRAINT pay_period_events_pay_group_nonempty_check CHECK (btrim(pay_group) <> ''),
  CONSTRAINT pay_period_events_pay_group_trim_check CHECK (pay_group = btrim(pay_group)),
  CONSTRAINT pay_period_events_pay_group_lower_check CHECK (pay_group = lower(pay_group)),
  CONSTRAINT pay_period_events_period_check CHECK (NOT isempty(period)),
  CONSTRAINT pay_period_events_period_bounds_check CHECK (lower_inc(period) AND NOT upper_inc(period)),
  CONSTRAINT pay_period_events_period_bounded_check CHECK (NOT lower_inf(period) AND NOT upper_inf(period)),
  CONSTRAINT pay_period_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT pay_period_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS pay_period_events_tenant_period_idx
  ON staffing.pay_period_events (tenant_id, pay_group, lower(period), id);

CREATE TABLE IF NOT EXISTS staffing.pay_periods (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  pay_group text NOT NULL,
  period daterange NOT NULL,
  status text NOT NULL DEFAULT 'open',
  closed_at timestamptz NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.pay_period_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT pay_periods_pay_group_nonempty_check CHECK (btrim(pay_group) <> ''),
  CONSTRAINT pay_periods_pay_group_trim_check CHECK (pay_group = btrim(pay_group)),
  CONSTRAINT pay_periods_pay_group_lower_check CHECK (pay_group = lower(pay_group)),
  CONSTRAINT pay_periods_period_check CHECK (NOT isempty(period)),
  CONSTRAINT pay_periods_period_bounds_check CHECK (lower_inc(period) AND NOT upper_inc(period)),
  CONSTRAINT pay_periods_period_bounded_check CHECK (NOT lower_inf(period) AND NOT upper_inf(period)),
  CONSTRAINT pay_periods_status_check CHECK (status IN ('open','closed')),
  CONSTRAINT pay_periods_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      pay_group gist_text_ops WITH =,
      period WITH &&
    )
);

CREATE INDEX IF NOT EXISTS pay_periods_lookup_btree
  ON staffing.pay_periods (tenant_id, pay_group, lower(period) DESC);

CREATE TABLE IF NOT EXISTS staffing.payroll_run_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  run_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  event_type text NOT NULL,
  run_state text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payroll_run_events_event_type_check CHECK (event_type IN ('CREATE','CALC_START','CALC_FINISH','CALC_FAIL','FINALIZE')),
  CONSTRAINT payroll_run_events_run_state_check CHECK (run_state IN ('draft','calculating','calculated','failed','finalized')),
  CONSTRAINT payroll_run_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT payroll_run_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT payroll_run_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS payroll_run_events_tenant_run_idx
  ON staffing.payroll_run_events (tenant_id, run_id, id);

CREATE TABLE IF NOT EXISTS staffing.payroll_runs (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  pay_period_id uuid NOT NULL,
  run_state text NOT NULL DEFAULT 'draft',
  needs_recalc boolean NOT NULL DEFAULT false,
  calc_started_at timestamptz NULL,
  calc_finished_at timestamptz NULL,
  finalized_at timestamptz NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payroll_runs_run_state_check CHECK (run_state IN ('draft','calculating','calculated','failed','finalized')),
  CONSTRAINT payroll_runs_pay_period_fk FOREIGN KEY (tenant_id, pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payroll_runs_by_period_btree
  ON staffing.payroll_runs (tenant_id, pay_period_id, created_at DESC, id);

-- 每个 pay period 最多允许 1 个 finalized run（避免“多个权威结果”）
CREATE UNIQUE INDEX IF NOT EXISTS payroll_runs_one_finalized_per_period_unique
  ON staffing.payroll_runs (tenant_id, pay_period_id)
  WHERE run_state = 'finalized';

CREATE TABLE IF NOT EXISTS staffing.payslips (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  gross_pay numeric(15,2) NOT NULL DEFAULT 0,
  net_pay numeric(15,2) NOT NULL DEFAULT 0,
  employer_total numeric(15,2) NOT NULL DEFAULT 0,
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payslips_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslips_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslips_period_fk FOREIGN KEY (tenant_id, pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslips_run_person_assignment_unique UNIQUE (tenant_id, run_id, person_uuid, assignment_id)
);

CREATE INDEX IF NOT EXISTS payslips_by_run_btree
  ON staffing.payslips (tenant_id, run_id, person_uuid, assignment_id);

CREATE TABLE IF NOT EXISTS staffing.payslip_items (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  item_code text NOT NULL,
  item_kind text NOT NULL,
  amount numeric(15,2) NOT NULL,
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_items_item_code_nonempty_check CHECK (btrim(item_code) <> ''),
  CONSTRAINT payslip_items_item_code_trim_check CHECK (item_code = btrim(item_code)),
  CONSTRAINT payslip_items_item_code_upper_check CHECK (item_code = upper(item_code)),
  CONSTRAINT payslip_items_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_items_meta_is_object_check CHECK (jsonb_typeof(meta) = 'object'),
  CONSTRAINT payslip_items_payslip_fk FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS payslip_items_by_payslip_btree
  ON staffing.payslip_items (tenant_id, payslip_id, id);

CREATE INDEX IF NOT EXISTS payslip_items_by_event_btree
  ON staffing.payslip_items (tenant_id, last_run_event_id, id);

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

ALTER TABLE staffing.payslip_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_items;
CREATE POLICY tenant_isolation ON staffing.payslip_items
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
