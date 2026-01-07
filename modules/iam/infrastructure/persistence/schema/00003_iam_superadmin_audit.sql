CREATE TABLE IF NOT EXISTS iam.superadmin_audit_logs (
  id bigserial PRIMARY KEY,
  event_id uuid NOT NULL DEFAULT gen_random_uuid(),
  actor text NOT NULL,
  action text NOT NULL,
  target_tenant_id uuid NULL REFERENCES iam.tenants(id) ON DELETE SET NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text NOT NULL,
  transaction_time timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT superadmin_audit_logs_actor_nonempty_check CHECK (btrim(actor) <> ''),
  CONSTRAINT superadmin_audit_logs_action_nonempty_check CHECK (btrim(action) <> ''),
  CONSTRAINT superadmin_audit_logs_payload_is_object_check CHECK (jsonb_typeof(payload) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS superadmin_audit_logs_event_id_unique ON iam.superadmin_audit_logs (event_id);
CREATE INDEX IF NOT EXISTS superadmin_audit_logs_target_tenant_idx ON iam.superadmin_audit_logs (target_tenant_id, id);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    EXECUTE 'GRANT INSERT, SELECT ON iam.superadmin_audit_logs TO superadmin_runtime';
    EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE iam.superadmin_audit_logs_id_seq TO superadmin_runtime';
  END IF;
END
$$;
