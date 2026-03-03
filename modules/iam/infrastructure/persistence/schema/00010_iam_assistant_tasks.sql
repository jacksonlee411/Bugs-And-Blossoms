CREATE TABLE IF NOT EXISTS iam.assistant_tasks (
  tenant_uuid uuid NOT NULL,
  task_id uuid NOT NULL,
  conversation_id text NOT NULL,
  turn_id text NOT NULL,
  task_type text NOT NULL,
  request_id text NOT NULL,
  request_hash text NOT NULL,
  workflow_id text NOT NULL,
  status text NOT NULL,
  dispatch_status text NOT NULL DEFAULT 'pending',
  dispatch_attempt integer NOT NULL DEFAULT 0,
  dispatch_deadline_at timestamptz NOT NULL,
  attempt integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL,
  last_error_code text NULL,
  trace_id text NULL,
  intent_schema_version text NOT NULL,
  compiler_contract_version text NOT NULL,
  capability_map_version text NOT NULL,
  skill_manifest_digest text NOT NULL,
  context_hash text NOT NULL,
  intent_hash text NOT NULL,
  plan_hash text NOT NULL,
  submitted_at timestamptz NOT NULL,
  cancel_requested_at timestamptz NULL,
  completed_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, task_id),
  CONSTRAINT assistant_tasks_turn_fk FOREIGN KEY (tenant_uuid, conversation_id, turn_id)
    REFERENCES iam.assistant_turns(tenant_uuid, conversation_id, turn_id) ON DELETE CASCADE,
  CONSTRAINT assistant_tasks_workflow_unique UNIQUE (tenant_uuid, workflow_id),
  CONSTRAINT assistant_tasks_submit_idempotency_unique UNIQUE (tenant_uuid, conversation_id, turn_id, request_id),
  CONSTRAINT assistant_tasks_task_type_check CHECK (task_type IN ('assistant_async_plan')),
  CONSTRAINT assistant_tasks_status_check CHECK (
    status IN ('queued', 'running', 'succeeded', 'failed', 'manual_takeover_required', 'canceled')
  ),
  CONSTRAINT assistant_tasks_dispatch_status_check CHECK (dispatch_status IN ('pending', 'started', 'failed')),
  CONSTRAINT assistant_tasks_attempt_non_negative CHECK (attempt >= 0),
  CONSTRAINT assistant_tasks_dispatch_attempt_non_negative CHECK (dispatch_attempt >= 0),
  CONSTRAINT assistant_tasks_max_attempts_positive CHECK (max_attempts > 0)
);

CREATE INDEX IF NOT EXISTS assistant_tasks_status_idx
  ON iam.assistant_tasks (tenant_uuid, status, updated_at);

CREATE INDEX IF NOT EXISTS assistant_tasks_dispatch_idx
  ON iam.assistant_tasks (tenant_uuid, dispatch_status, dispatch_deadline_at);

ALTER TABLE iam.assistant_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.assistant_tasks FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.assistant_tasks;
CREATE POLICY tenant_isolation ON iam.assistant_tasks
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.assistant_task_events (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  task_id uuid NOT NULL,
  from_status text NULL,
  to_status text NOT NULL,
  event_type text NOT NULL,
  error_code text NULL,
  payload jsonb NULL,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assistant_task_events_task_fk FOREIGN KEY (tenant_uuid, task_id)
    REFERENCES iam.assistant_tasks(tenant_uuid, task_id) ON DELETE CASCADE,
  CONSTRAINT assistant_task_events_to_status_check CHECK (
    to_status IN ('queued', 'running', 'succeeded', 'failed', 'manual_takeover_required', 'canceled')
  ),
  CONSTRAINT assistant_task_events_from_status_check CHECK (
    from_status IS NULL OR from_status IN ('queued', 'running', 'succeeded', 'failed', 'manual_takeover_required', 'canceled')
  ),
  CONSTRAINT assistant_task_events_type_check CHECK (
    event_type IN (
      'queued',
      'running',
      'succeeded',
      'failed',
      'manual_takeover_required',
      'cancel_requested',
      'canceled',
      'dead_lettered'
    )
  )
);

CREATE INDEX IF NOT EXISTS assistant_task_events_lookup_idx
  ON iam.assistant_task_events (tenant_uuid, task_id, occurred_at);

ALTER TABLE iam.assistant_task_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.assistant_task_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.assistant_task_events;
CREATE POLICY tenant_isolation ON iam.assistant_task_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.assistant_task_dispatch_outbox (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  task_id uuid NOT NULL,
  workflow_id text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  attempt integer NOT NULL DEFAULT 0,
  next_retry_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT assistant_task_dispatch_outbox_task_fk FOREIGN KEY (tenant_uuid, task_id)
    REFERENCES iam.assistant_tasks(tenant_uuid, task_id) ON DELETE CASCADE,
  CONSTRAINT assistant_task_dispatch_outbox_task_unique UNIQUE (tenant_uuid, task_id),
  CONSTRAINT assistant_task_dispatch_outbox_status_check CHECK (
    status IN ('pending', 'started', 'failed', 'canceled')
  ),
  CONSTRAINT assistant_task_dispatch_outbox_attempt_non_negative CHECK (attempt >= 0)
);

CREATE INDEX IF NOT EXISTS assistant_task_dispatch_outbox_schedule_idx
  ON iam.assistant_task_dispatch_outbox (status, next_retry_at);

ALTER TABLE iam.assistant_task_dispatch_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.assistant_task_dispatch_outbox FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.assistant_task_dispatch_outbox;
CREATE POLICY tenant_isolation ON iam.assistant_task_dispatch_outbox
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
