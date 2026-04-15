CREATE TABLE IF NOT EXISTS iam.cubebox_files (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  file_id text NOT NULL,
  storage_provider text NOT NULL,
  storage_key text NOT NULL,
  file_name text NOT NULL,
  media_type text NOT NULL,
  size_bytes bigint NOT NULL,
  sha256 text NOT NULL,
  scan_status text NOT NULL DEFAULT 'ready',
  scan_error_code text NULL,
  uploaded_by text NOT NULL,
  uploaded_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, file_id),
  CONSTRAINT cubebox_files_id_format_check CHECK (
    file_id ~ '^file_[0-9a-f-]{36}$'
  ),
  CONSTRAINT cubebox_files_storage_provider_check CHECK (
    storage_provider IN ('localfs', 's3_compat')
  ),
  CONSTRAINT cubebox_files_size_positive_check CHECK (
    size_bytes > 0 AND size_bytes <= 20971520
  ),
  CONSTRAINT cubebox_files_sha256_hex_check CHECK (
    sha256 ~ '^[0-9a-f]{64}$'
  ),
  CONSTRAINT cubebox_files_scan_status_check CHECK (
    scan_status IN ('pending', 'ready', 'failed')
  ),
  CONSTRAINT cubebox_files_storage_key_unique UNIQUE (tenant_uuid, storage_key)
);

CREATE INDEX IF NOT EXISTS cubebox_files_uploaded_idx
  ON iam.cubebox_files (tenant_uuid, uploaded_at DESC, file_id DESC);

ALTER TABLE iam.cubebox_files ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_files FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_files;
CREATE POLICY tenant_isolation ON iam.cubebox_files
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.cubebox_file_links (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  file_id text NOT NULL,
  conversation_id text NOT NULL,
  turn_id text NULL,
  link_role text NOT NULL,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cubebox_file_links_file_fk FOREIGN KEY (tenant_uuid, file_id)
    REFERENCES iam.cubebox_files(tenant_uuid, file_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_file_links_conversation_fk FOREIGN KEY (tenant_uuid, conversation_id)
    REFERENCES iam.cubebox_conversations(tenant_uuid, conversation_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_file_links_turn_fk FOREIGN KEY (tenant_uuid, conversation_id, turn_id)
    REFERENCES iam.cubebox_turns(tenant_uuid, conversation_id, turn_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_file_links_role_check CHECK (
    link_role IN ('conversation_attachment', 'turn_input', 'turn_output')
  ),
  CONSTRAINT cubebox_file_links_shape_check CHECK (
    (
      turn_id IS NULL
      AND link_role = 'conversation_attachment'
    ) OR (
      turn_id IS NOT NULL
      AND link_role IN ('turn_input', 'turn_output')
    )
  )
);

CREATE INDEX IF NOT EXISTS cubebox_file_links_conversation_idx
  ON iam.cubebox_file_links (tenant_uuid, conversation_id, created_at, id);

CREATE INDEX IF NOT EXISTS cubebox_file_links_turn_idx
  ON iam.cubebox_file_links (tenant_uuid, conversation_id, turn_id, created_at, id);

CREATE INDEX IF NOT EXISTS cubebox_file_links_file_idx
  ON iam.cubebox_file_links (tenant_uuid, file_id, created_at, id);

CREATE UNIQUE INDEX IF NOT EXISTS cubebox_file_links_conversation_unique
  ON iam.cubebox_file_links (tenant_uuid, file_id, conversation_id, link_role)
  WHERE turn_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS cubebox_file_links_turn_unique
  ON iam.cubebox_file_links (tenant_uuid, file_id, conversation_id, turn_id, link_role)
  WHERE turn_id IS NOT NULL;

ALTER TABLE iam.cubebox_file_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_file_links FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_file_links;
CREATE POLICY tenant_isolation ON iam.cubebox_file_links
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
