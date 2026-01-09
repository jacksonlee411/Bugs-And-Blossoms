CREATE TABLE IF NOT EXISTS staffing.payroll_balances (
  tenant_id uuid NOT NULL,
  tax_entity_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,

  first_tax_month smallint NOT NULL,
  last_tax_month smallint NOT NULL,

  ytd_income numeric(15,2) NOT NULL DEFAULT 0,
  ytd_tax_exempt_income numeric(15,2) NOT NULL DEFAULT 0,
  ytd_standard_deduction numeric(15,2) NOT NULL DEFAULT 0,
  ytd_special_deduction numeric(15,2) NOT NULL DEFAULT 0,
  ytd_special_additional_deduction numeric(15,2) NOT NULL DEFAULT 0,

  ytd_taxable_income numeric(15,2) NOT NULL DEFAULT 0,
  ytd_iit_tax_liability numeric(15,2) NOT NULL DEFAULT 0,
  ytd_iit_withheld numeric(15,2) NOT NULL DEFAULT 0,
  ytd_iit_credit numeric(15,2) NOT NULL DEFAULT 0,

  last_pay_period_id uuid NOT NULL,
  last_run_id uuid NOT NULL,

  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, tax_entity_id, person_uuid, tax_year),
  CONSTRAINT payroll_balances_tax_year_check CHECK (tax_year >= 2000 AND tax_year <= 9999),
  CONSTRAINT payroll_balances_first_month_check CHECK (first_tax_month >= 1 AND first_tax_month <= 12),
  CONSTRAINT payroll_balances_last_month_check CHECK (last_tax_month >= 1 AND last_tax_month <= 12),
  CONSTRAINT payroll_balances_months_order_check CHECK (last_tax_month >= first_tax_month),
  CONSTRAINT payroll_balances_amounts_nonneg_check CHECK (
    ytd_income >= 0 AND ytd_tax_exempt_income >= 0 AND ytd_standard_deduction >= 0
    AND ytd_special_deduction >= 0 AND ytd_special_additional_deduction >= 0
    AND ytd_taxable_income >= 0 AND ytd_iit_tax_liability >= 0 AND ytd_iit_withheld >= 0 AND ytd_iit_credit >= 0
  )
);

CREATE INDEX IF NOT EXISTS payroll_balances_lookup_btree
  ON staffing.payroll_balances (tenant_id, person_uuid, tax_year);

ALTER TABLE staffing.payroll_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payroll_balances FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payroll_balances;
CREATE POLICY tenant_isolation ON staffing.payroll_balances
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
