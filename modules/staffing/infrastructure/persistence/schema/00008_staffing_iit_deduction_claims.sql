CREATE TABLE IF NOT EXISTS staffing.iit_special_additional_deduction_claim_events (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  tax_month smallint NOT NULL,
  amount numeric(15,2) NOT NULL,
  request_id text NOT NULL,
  initiator_id uuid NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT iit_sad_claim_events_tax_year_check CHECK (tax_year >= 2000 AND tax_year <= 9999),
  CONSTRAINT iit_sad_claim_events_tax_month_check CHECK (tax_month >= 1 AND tax_month <= 12),
  CONSTRAINT iit_sad_claim_events_amount_check CHECK (amount >= 0),
  CONSTRAINT iit_sad_claim_events_event_id_unique UNIQUE (event_id),
  CONSTRAINT iit_sad_claim_events_request_id_unique UNIQUE (tenant_id, request_id)
);

CREATE INDEX IF NOT EXISTS iit_sad_claim_events_lookup_btree
  ON staffing.iit_special_additional_deduction_claim_events (tenant_id, person_uuid, tax_year, tax_month, id);

CREATE TABLE IF NOT EXISTS staffing.iit_special_additional_deduction_claims (
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL,
  tax_year integer NOT NULL,
  tax_month smallint NOT NULL,
  amount numeric(15,2) NOT NULL DEFAULT 0,
  last_event_id bigint NOT NULL REFERENCES staffing.iit_special_additional_deduction_claim_events(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, person_uuid, tax_year, tax_month),
  CONSTRAINT iit_sad_claims_tax_year_check CHECK (tax_year >= 2000 AND tax_year <= 9999),
  CONSTRAINT iit_sad_claims_tax_month_check CHECK (tax_month >= 1 AND tax_month <= 12),
  CONSTRAINT iit_sad_claims_amount_check CHECK (amount >= 0)
);

ALTER TABLE staffing.iit_special_additional_deduction_claim_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.iit_special_additional_deduction_claim_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.iit_special_additional_deduction_claim_events;
CREATE POLICY tenant_isolation ON staffing.iit_special_additional_deduction_claim_events
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE staffing.iit_special_additional_deduction_claims ENABLE ROW LEVEL SECURITY;
ALTER TABLE staffing.iit_special_additional_deduction_claims FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON staffing.iit_special_additional_deduction_claims;
CREATE POLICY tenant_isolation ON staffing.iit_special_additional_deduction_claims
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
