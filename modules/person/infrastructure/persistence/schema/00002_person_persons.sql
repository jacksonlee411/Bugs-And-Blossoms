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

CREATE TABLE IF NOT EXISTS person.external_identity_links (
  tenant_id uuid NOT NULL,
  provider text NOT NULL,
  external_user_id text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  person_uuid uuid NULL,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  seen_count int NOT NULL DEFAULT 1,
  last_seen_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, provider, external_user_id),
  CONSTRAINT external_identity_links_provider_check CHECK (provider IN ('DINGTALK','WECOM')),
  CONSTRAINT external_identity_links_external_user_id_nonempty_check CHECK (btrim(external_user_id) <> ''),
  CONSTRAINT external_identity_links_external_user_id_trim_check CHECK (external_user_id = btrim(external_user_id)),
  CONSTRAINT external_identity_links_status_check CHECK (status IN ('pending','active','disabled','ignored')),
  CONSTRAINT external_identity_links_status_person_uuid_check CHECK (
    (status IN ('pending','ignored') AND person_uuid IS NULL)
    OR (status IN ('active','disabled') AND person_uuid IS NOT NULL)
  ),
  CONSTRAINT external_identity_links_last_seen_payload_is_object_check CHECK (jsonb_typeof(last_seen_payload) = 'object')
);

CREATE INDEX IF NOT EXISTS external_identity_links_lookup_idx
  ON person.external_identity_links (tenant_id, provider, status, last_seen_at DESC);

ALTER TABLE person.external_identity_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE person.external_identity_links FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON person.external_identity_links;
CREATE POLICY tenant_isolation ON person.external_identity_links
USING (tenant_id = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
