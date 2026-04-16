-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS iam.cubebox_file_cleanup_jobs (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  file_id text NOT NULL,
  storage_provider text NOT NULL,
  storage_key text NOT NULL,
  cleanup_reason text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  attempt_count integer NOT NULL DEFAULT 0,
  next_retry_at timestamptz NOT NULL DEFAULT now(),
  last_error text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cubebox_file_cleanup_jobs_file_id_format_check CHECK (
    file_id ~ '^file_[0-9a-f-]{36}$'
  ),
  CONSTRAINT cubebox_file_cleanup_jobs_storage_provider_check CHECK (
    storage_provider IN ('localfs', 's3_compat')
  ),
  CONSTRAINT cubebox_file_cleanup_jobs_reason_check CHECK (
    cleanup_reason IN ('metadata_write_failed', 'object_delete_failed')
  ),
  CONSTRAINT cubebox_file_cleanup_jobs_status_check CHECK (
    status IN ('pending', 'running', 'succeeded', 'failed', 'manual_takeover_required')
  ),
  CONSTRAINT cubebox_file_cleanup_jobs_attempt_non_negative CHECK (
    attempt_count >= 0
  )
);

CREATE INDEX IF NOT EXISTS cubebox_file_cleanup_jobs_schedule_idx
  ON iam.cubebox_file_cleanup_jobs (status, next_retry_at);

CREATE INDEX IF NOT EXISTS cubebox_file_cleanup_jobs_file_idx
  ON iam.cubebox_file_cleanup_jobs (tenant_uuid, file_id, created_at DESC, id DESC);

ALTER TABLE iam.cubebox_file_cleanup_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_file_cleanup_jobs FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_file_cleanup_jobs;
CREATE POLICY tenant_isolation ON iam.cubebox_file_cleanup_jobs
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.cubebox_file_cleanup_jobs;
-- +goose StatementEnd
