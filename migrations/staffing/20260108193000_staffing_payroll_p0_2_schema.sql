-- +goose Up
-- +goose StatementBegin
ALTER TABLE staffing.assignment_versions
  ADD COLUMN base_salary numeric(15,2) NULL,
  ADD COLUMN currency character(3) NOT NULL DEFAULT 'CNY',
  ADD COLUMN profile jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE staffing.assignment_versions
  ADD CONSTRAINT assignment_versions_currency_check CHECK (((currency)::text = btrim((currency)::text)) AND ((currency)::text = upper((currency)::text))),
  ADD CONSTRAINT assignment_versions_base_salary_check CHECK ((base_salary IS NULL) OR (base_salary >= (0)::numeric)),
  ADD CONSTRAINT assignment_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object'::text);

CREATE TABLE staffing.payslip_items (
  id bigserial NOT NULL,
  tenant_id uuid NOT NULL,
  payslip_id uuid NOT NULL,
  item_code text NOT NULL,
  item_kind text NOT NULL,
  amount numeric(15,2) NOT NULL,
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_run_event_id bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id),
  CONSTRAINT payslip_items_last_run_event_id_fkey FOREIGN KEY (last_run_event_id) REFERENCES staffing.payroll_run_events (id) ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT payslip_items_payslip_fk FOREIGN KEY (tenant_id, payslip_id) REFERENCES staffing.payslips (tenant_id, id) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT payslip_items_item_code_nonempty_check CHECK (btrim(item_code) <> ''::text),
  CONSTRAINT payslip_items_item_code_trim_check CHECK (item_code = btrim(item_code)),
  CONSTRAINT payslip_items_item_code_upper_check CHECK (item_code = upper(item_code)),
  CONSTRAINT payslip_items_item_kind_check CHECK (item_kind = ANY (ARRAY['earning'::text, 'deduction'::text, 'employer_cost'::text])),
  CONSTRAINT payslip_items_meta_is_object_check CHECK (jsonb_typeof(meta) = 'object'::text)
);

CREATE INDEX payslip_items_by_payslip_btree
  ON staffing.payslip_items (tenant_id, payslip_id, id);

CREATE INDEX payslip_items_by_event_btree
  ON staffing.payslip_items (tenant_id, last_run_event_id, id);

ALTER TABLE staffing.payslip_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.payslip_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_items;
CREATE POLICY tenant_isolation ON staffing.payslip_items
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON staffing.payslip_items;
ALTER TABLE IF EXISTS staffing.payslip_items DISABLE ROW LEVEL SECURITY;
DROP TABLE IF EXISTS staffing.payslip_items;

ALTER TABLE staffing.assignment_versions
  DROP CONSTRAINT IF EXISTS assignment_versions_profile_is_object_check,
  DROP CONSTRAINT IF EXISTS assignment_versions_base_salary_check,
  DROP CONSTRAINT IF EXISTS assignment_versions_currency_check,
  DROP COLUMN IF EXISTS profile,
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS base_salary;
-- +goose StatementEnd
