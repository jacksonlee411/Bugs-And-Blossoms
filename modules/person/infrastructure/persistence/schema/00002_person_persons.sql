CREATE TABLE IF NOT EXISTS person.persons (
  tenant_id uuid NOT NULL,
  person_uuid uuid NOT NULL DEFAULT gen_random_uuid(),
  pernr text NOT NULL,
  display_name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, person_uuid),
  CONSTRAINT persons_pernr_trim_check CHECK (btrim(pernr) = pernr),
  CONSTRAINT persons_pernr_digits_max8_check CHECK (pernr ~ '^[0-9]{1,8}$'),
  CONSTRAINT persons_pernr_canonical_check CHECK (pernr = '0' OR pernr !~ '^0'),
  CONSTRAINT persons_display_name_trim_check CHECK (btrim(display_name) = display_name),
  CONSTRAINT persons_display_name_nonempty_check CHECK (display_name <> ''),
  CONSTRAINT persons_status_check CHECK (status IN ('active','inactive')),
  CONSTRAINT persons_tenant_pernr_unique UNIQUE (tenant_id, pernr)
);

CREATE INDEX IF NOT EXISTS persons_tenant_display_name_idx
  ON person.persons (tenant_id, display_name);

ALTER TABLE person.persons ENABLE ROW LEVEL SECURITY;
ALTER TABLE person.persons FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON person.persons;
CREATE POLICY tenant_isolation ON person.persons
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
