-- Payroll item inputs (net-guaranteed IIT / amount) — SSOT.

CREATE TABLE IF NOT EXISTS staffing.payslip_item_input_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,

  -- “输入归属 payslip”使用 natural key（避免依赖 payslip_id 稳定性）
  run_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,

  event_type text NOT NULL, -- UPSERT / DELETE

  item_code text NOT NULL,
  item_kind text NOT NULL,  -- earning / deduction / employer_cost（P0-6 只允许 earning；净额保证项仅允许 earning）

  currency char(3) NOT NULL DEFAULT 'CNY',
  calc_mode text NOT NULL,   -- amount / net_guaranteed_iit
  tax_bearer text NOT NULL,  -- employee / employer

  -- amount 语义：
  -- - calc_mode=amount：amount 为该输入项的金额（税前，正数）
  -- - calc_mode=net_guaranteed_iit：amount 为 target_net（仅扣 IIT 后净额目标，正数）
  amount numeric(15,2) NOT NULL,

  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT payslip_item_input_events_event_type_check CHECK (event_type IN ('UPSERT','DELETE')),
  CONSTRAINT payslip_item_input_events_code_check CHECK (btrim(item_code) <> '' AND item_code = btrim(item_code) AND item_code = upper(item_code) AND item_code ~ '^[A-Z0-9_]+$'),
  CONSTRAINT payslip_item_input_events_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_item_input_events_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_item_input_events_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_item_input_events_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_item_input_events_amount_positive_check CHECK (amount > 0),
  CONSTRAINT payslip_item_input_events_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      item_kind = 'earning'
      AND tax_bearer = 'employer'
      AND currency = 'CNY'
    )
  ),
  CONSTRAINT payslip_item_input_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT payslip_item_input_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT payslip_item_input_events_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payslip_item_input_events_lookup_btree
  ON staffing.payslip_item_input_events (tenant_id, run_id, person_uuid, assignment_id, item_code, id);

CREATE TABLE IF NOT EXISTS staffing.payslip_item_inputs (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),

  run_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,

  item_code text NOT NULL,
  item_kind text NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  calc_mode text NOT NULL,
  tax_bearer text NOT NULL,
  amount numeric(15,2) NOT NULL,

  last_event_id bigint NOT NULL REFERENCES staffing.payslip_item_input_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, id),
  CONSTRAINT payslip_item_inputs_code_check CHECK (btrim(item_code) <> '' AND item_code = btrim(item_code) AND item_code = upper(item_code) AND item_code ~ '^[A-Z0-9_]+$'),
  CONSTRAINT payslip_item_inputs_item_kind_check CHECK (item_kind IN ('earning','deduction','employer_cost')),
  CONSTRAINT payslip_item_inputs_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_item_inputs_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  CONSTRAINT payslip_item_inputs_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  CONSTRAINT payslip_item_inputs_amount_positive_check CHECK (amount > 0),
  CONSTRAINT payslip_item_inputs_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      item_kind = 'earning'
      AND tax_bearer = 'employer'
      AND currency = 'CNY'
    )
  ),
  CONSTRAINT payslip_item_inputs_natural_unique UNIQUE (tenant_id, run_id, person_uuid, assignment_id, item_code),
  CONSTRAINT payslip_item_inputs_run_fk FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS payslip_item_inputs_by_run_person_btree
  ON staffing.payslip_item_inputs (tenant_id, run_id, person_uuid, assignment_id, item_code);

ALTER TABLE staffing.payslip_items
  ADD COLUMN IF NOT EXISTS calc_mode text NOT NULL DEFAULT 'amount',
  ADD COLUMN IF NOT EXISTS tax_bearer text NOT NULL DEFAULT 'employee',
  ADD COLUMN IF NOT EXISTS target_net numeric(15,2) NULL,
  ADD COLUMN IF NOT EXISTS iit_delta numeric(15,2) NULL;

ALTER TABLE staffing.payslip_items
  DROP CONSTRAINT IF EXISTS payslip_items_calc_mode_check,
  DROP CONSTRAINT IF EXISTS payslip_items_tax_bearer_check,
  DROP CONSTRAINT IF EXISTS payslip_items_iit_delta_nonneg_check,
  DROP CONSTRAINT IF EXISTS payslip_items_target_net_positive_check,
  DROP CONSTRAINT IF EXISTS payslip_items_net_guaranteed_contract_check;

ALTER TABLE staffing.payslip_items
  ADD CONSTRAINT payslip_items_calc_mode_check CHECK (calc_mode IN ('amount','net_guaranteed_iit')),
  ADD CONSTRAINT payslip_items_tax_bearer_check CHECK (tax_bearer IN ('employee','employer')),
  ADD CONSTRAINT payslip_items_iit_delta_nonneg_check CHECK (iit_delta IS NULL OR iit_delta >= 0),
  ADD CONSTRAINT payslip_items_target_net_positive_check CHECK (target_net IS NULL OR target_net > 0),
  ADD CONSTRAINT payslip_items_net_guaranteed_contract_check CHECK (
    calc_mode <> 'net_guaranteed_iit'
    OR (
      tax_bearer = 'employer'
      AND item_kind = 'earning'
      AND target_net IS NOT NULL
      AND iit_delta IS NOT NULL
      AND amount = target_net + iit_delta
    )
  );

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
