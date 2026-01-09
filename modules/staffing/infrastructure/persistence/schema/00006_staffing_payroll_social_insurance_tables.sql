-- Payroll (P0-3) social insurance tables + RLS

CREATE TABLE IF NOT EXISTS staffing.social_insurance_policies (
  tenant_id uuid NOT NULL,
  id uuid NOT NULL DEFAULT gen_random_uuid(),
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  CONSTRAINT social_insurance_policies_city_code_nonempty_check CHECK (btrim(city_code) <> ''),
  CONSTRAINT social_insurance_policies_city_code_trim_check CHECK (city_code = btrim(city_code)),
  CONSTRAINT social_insurance_policies_city_code_upper_check CHECK (city_code = upper(city_code)),
  CONSTRAINT social_insurance_policies_hukou_type_nonempty_check CHECK (btrim(hukou_type) <> ''),
  CONSTRAINT social_insurance_policies_hukou_type_trim_check CHECK (hukou_type = btrim(hukou_type)),
  CONSTRAINT social_insurance_policies_hukou_type_lower_check CHECK (hukou_type = lower(hukou_type)),
  CONSTRAINT social_insurance_policies_insurance_type_check CHECK (
    insurance_type IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND')
  ),
  CONSTRAINT social_insurance_policies_identity_unique UNIQUE (tenant_id, city_code, hukou_type, insurance_type)
);

CREATE INDEX IF NOT EXISTS social_insurance_policies_lookup_btree
  ON staffing.social_insurance_policies (tenant_id, city_code, hukou_type, insurance_type);

CREATE TABLE IF NOT EXISTS staffing.social_insurance_policy_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  policy_id uuid NOT NULL,
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  event_type text NOT NULL,
  effective_date date NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT social_insurance_policy_events_event_type_check CHECK (event_type IN ('CREATE','UPDATE')),
  CONSTRAINT social_insurance_policy_events_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object'),
  CONSTRAINT social_insurance_policy_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT social_insurance_policy_events_one_per_day_unique UNIQUE (tenant_id, policy_id, effective_date),
  CONSTRAINT social_insurance_policy_events_request_id_unique UNIQUE (tenant_id, request_id),
  CONSTRAINT social_insurance_policy_events_city_code_trim_check CHECK (city_code = btrim(city_code)),
  CONSTRAINT social_insurance_policy_events_city_code_upper_check CHECK (city_code = upper(city_code)),
  CONSTRAINT social_insurance_policy_events_hukou_type_trim_check CHECK (hukou_type = btrim(hukou_type)),
  CONSTRAINT social_insurance_policy_events_hukou_type_lower_check CHECK (hukou_type = lower(hukou_type)),
  CONSTRAINT social_insurance_policy_events_insurance_type_check CHECK (
    insurance_type IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND')
  ),
  CONSTRAINT social_insurance_policy_events_policy_fk
    FOREIGN KEY (tenant_id, policy_id) REFERENCES staffing.social_insurance_policies(tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS social_insurance_policy_events_tenant_policy_effective_idx
  ON staffing.social_insurance_policy_events (tenant_id, policy_id, effective_date, id);

CREATE TABLE IF NOT EXISTS staffing.social_insurance_policy_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  policy_id uuid NOT NULL,
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  employer_rate numeric(9,6) NOT NULL,
  employee_rate numeric(9,6) NOT NULL,
  base_floor numeric(15,2) NOT NULL,
  base_ceiling numeric(15,2) NOT NULL,
  rounding_rule text NOT NULL,
  precision smallint NOT NULL DEFAULT 2,
  rules_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.social_insurance_policy_events(id),
  CONSTRAINT social_insurance_policy_versions_rules_is_object_check CHECK (jsonb_typeof(rules_config) = 'object'),
  CONSTRAINT social_insurance_policy_versions_rate_check CHECK (
    employer_rate >= 0 AND employer_rate <= 1 AND employee_rate >= 0 AND employee_rate <= 1
  ),
  CONSTRAINT social_insurance_policy_versions_base_check CHECK (
    base_floor >= 0 AND base_ceiling >= base_floor
  ),
  CONSTRAINT social_insurance_policy_versions_rounding_rule_check CHECK (rounding_rule IN ('HALF_UP','CEIL')),
  CONSTRAINT social_insurance_policy_versions_precision_check CHECK (precision >= 0 AND precision <= 2),
  CONSTRAINT social_insurance_policy_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT social_insurance_policy_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity)),
  CONSTRAINT social_insurance_policy_versions_policy_fk
    FOREIGN KEY (tenant_id, policy_id) REFERENCES staffing.social_insurance_policies(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT social_insurance_policy_versions_no_overlap
    EXCLUDE USING gist (
      tenant_id gist_uuid_ops WITH =,
      policy_id gist_uuid_ops WITH =,
      validity WITH &&
    )
);

CREATE INDEX IF NOT EXISTS social_insurance_policy_versions_lookup_btree
  ON staffing.social_insurance_policy_versions (tenant_id, policy_id, lower(validity));

CREATE TABLE IF NOT EXISTS staffing.payslip_social_insurance_items (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  run_id uuid NOT NULL,
  pay_period_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  assignment_id uuid NOT NULL,
  city_code text NOT NULL,
  hukou_type text NOT NULL,
  insurance_type text NOT NULL,
  base_amount numeric(15,2) NOT NULL,
  employee_amount numeric(15,2) NOT NULL,
  employer_amount numeric(15,2) NOT NULL,
  currency char(3) NOT NULL DEFAULT 'CNY',
  policy_id uuid NOT NULL,
  policy_last_event_id bigint NOT NULL REFERENCES staffing.social_insurance_policy_events(id),
  last_run_event_id bigint NOT NULL REFERENCES staffing.payroll_run_events(id),
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payslip_social_insurance_items_currency_check CHECK (currency = btrim(currency) AND currency = upper(currency)),
  CONSTRAINT payslip_social_insurance_items_meta_is_object_check CHECK (jsonb_typeof(meta) = 'object'),
  CONSTRAINT payslip_social_insurance_items_amounts_check CHECK (base_amount >= 0 AND employee_amount >= 0 AND employer_amount >= 0),
  CONSTRAINT payslip_social_insurance_items_insurance_type_check CHECK (
    insurance_type IN ('PENSION','MEDICAL','UNEMPLOYMENT','INJURY','MATERNITY','HOUSING_FUND')
  ),
  CONSTRAINT payslip_social_insurance_items_payslip_fk
    FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips(tenant_id, id) ON DELETE CASCADE,
  CONSTRAINT payslip_social_insurance_items_run_fk
    FOREIGN KEY (tenant_id, run_id) REFERENCES staffing.payroll_runs(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslip_social_insurance_items_period_fk
    FOREIGN KEY (tenant_id, pay_period_id) REFERENCES staffing.pay_periods(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslip_social_insurance_items_policy_fk
    FOREIGN KEY (tenant_id, policy_id) REFERENCES staffing.social_insurance_policies(tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT payslip_social_insurance_items_identity_unique UNIQUE (tenant_id, payslip_id, insurance_type)
);

CREATE INDEX IF NOT EXISTS payslip_social_insurance_items_by_run_btree
  ON staffing.payslip_social_insurance_items (tenant_id, run_id, person_uuid, assignment_id, insurance_type);

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

