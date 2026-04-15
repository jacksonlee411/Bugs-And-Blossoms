-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS iam.cubebox_conversations (
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  conversation_id text NOT NULL,
  actor_id text NOT NULL,
  actor_role text NOT NULL,
  state text NOT NULL,
  current_phase text NOT NULL DEFAULT 'idle',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, conversation_id),
  CONSTRAINT cubebox_conversations_id_format_check CHECK (
    conversation_id ~ '^conv_[0-9a-f]{32}$'
  ),
  CONSTRAINT cubebox_conversations_state_check CHECK (
    state IN ('validated', 'confirmed', 'committed', 'canceled', 'expired')
  ),
  CONSTRAINT cubebox_conversations_phase_check CHECK (
    current_phase IN (
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  )
);

CREATE INDEX IF NOT EXISTS cubebox_conversations_actor_idx
  ON iam.cubebox_conversations (tenant_uuid, actor_id, updated_at DESC, conversation_id DESC);

ALTER TABLE iam.cubebox_conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_conversations FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_conversations;
CREATE POLICY tenant_isolation ON iam.cubebox_conversations
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.cubebox_turns (
  tenant_uuid uuid NOT NULL,
  conversation_id text NOT NULL,
  turn_id text NOT NULL,
  user_input text NOT NULL,
  state text NOT NULL,
  phase text NOT NULL,
  risk_tier text NOT NULL,
  request_id text NOT NULL,
  trace_id text NOT NULL,
  policy_version text NOT NULL,
  composition_version text NOT NULL,
  mapping_version text NOT NULL,
  intent_json jsonb NOT NULL,
  plan_json jsonb NOT NULL,
  candidates_json jsonb NOT NULL,
  candidate_options jsonb NOT NULL DEFAULT '[]'::jsonb,
  resolved_candidate_id text NULL,
  selected_candidate_id text NULL,
  ambiguity_count integer NOT NULL,
  confidence double precision NOT NULL,
  resolution_source text NULL,
  route_decision_json jsonb NULL,
  clarification_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  dry_run_json jsonb NOT NULL,
  pending_draft_summary text NULL,
  missing_fields jsonb NOT NULL DEFAULT '[]'::jsonb,
  commit_result_json jsonb NULL,
  commit_reply jsonb NULL,
  error_code text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_uuid, conversation_id, turn_id),
  CONSTRAINT cubebox_turns_conversation_fk FOREIGN KEY (tenant_uuid, conversation_id)
    REFERENCES iam.cubebox_conversations(tenant_uuid, conversation_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_turns_turn_id_format_check CHECK (
    turn_id ~ '^turn_[0-9a-f]{32}$'
  ),
  CONSTRAINT cubebox_turns_state_check CHECK (
    state IN ('validated', 'confirmed', 'committed', 'canceled', 'expired')
  ),
  CONSTRAINT cubebox_turns_phase_check CHECK (
    phase IN (
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  ),
  CONSTRAINT cubebox_turns_ambiguity_non_negative CHECK (ambiguity_count >= 0),
  CONSTRAINT cubebox_turns_confidence_range_check CHECK (confidence >= 0 AND confidence <= 1),
  CONSTRAINT cubebox_turns_intent_object_check CHECK (jsonb_typeof(intent_json) = 'object'),
  CONSTRAINT cubebox_turns_plan_object_check CHECK (jsonb_typeof(plan_json) = 'object'),
  CONSTRAINT cubebox_turns_candidates_array_check CHECK (jsonb_typeof(candidates_json) = 'array'),
  CONSTRAINT cubebox_turns_candidate_options_array_check CHECK (jsonb_typeof(candidate_options) = 'array'),
  CONSTRAINT cubebox_turns_dry_run_object_check CHECK (jsonb_typeof(dry_run_json) = 'object'),
  CONSTRAINT cubebox_turns_route_decision_object_or_null_check CHECK (
    route_decision_json IS NULL OR jsonb_typeof(route_decision_json) = 'object'
  ),
  CONSTRAINT cubebox_turns_clarification_object_check CHECK (
    jsonb_typeof(clarification_json) = 'object'
  ),
  CONSTRAINT cubebox_turns_missing_fields_array_check CHECK (
    jsonb_typeof(missing_fields) = 'array'
  ),
  CONSTRAINT cubebox_turns_commit_result_object_or_null_check CHECK (
    commit_result_json IS NULL OR jsonb_typeof(commit_result_json) = 'object'
  ),
  CONSTRAINT cubebox_turns_commit_reply_object_or_null_check CHECK (
    commit_reply IS NULL OR jsonb_typeof(commit_reply) = 'object'
  )
);

CREATE INDEX IF NOT EXISTS cubebox_turns_lookup_idx
  ON iam.cubebox_turns (tenant_uuid, conversation_id, created_at, turn_id);

ALTER TABLE iam.cubebox_turns ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_turns FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_turns;
CREATE POLICY tenant_isolation ON iam.cubebox_turns
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.cubebox_state_transitions (
  id bigserial PRIMARY KEY,
  tenant_uuid uuid NOT NULL,
  conversation_id text NOT NULL,
  turn_id text NULL,
  turn_action text NULL,
  request_id text NOT NULL,
  trace_id text NOT NULL,
  from_state text NOT NULL,
  to_state text NOT NULL,
  from_phase text NOT NULL,
  to_phase text NOT NULL,
  reason_code text NULL,
  actor_id text NOT NULL,
  changed_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cubebox_state_transitions_conversation_fk FOREIGN KEY (tenant_uuid, conversation_id)
    REFERENCES iam.cubebox_conversations(tenant_uuid, conversation_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_state_transitions_from_state_check CHECK (
    from_state IN ('init', 'validated', 'confirmed', 'committed', 'canceled', 'expired')
  ),
  CONSTRAINT cubebox_state_transitions_to_state_check CHECK (
    to_state IN ('validated', 'confirmed', 'committed', 'canceled', 'expired')
  ),
  CONSTRAINT cubebox_state_transitions_from_phase_check CHECK (
    from_phase IN (
      'init',
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  ),
  CONSTRAINT cubebox_state_transitions_to_phase_check CHECK (
    to_phase IN (
      'idle',
      'await_clarification',
      'await_missing_fields',
      'await_candidate_pick',
      'await_candidate_confirm',
      'await_commit_confirm',
      'committing',
      'committed',
      'failed',
      'canceled',
      'expired'
    )
  ),
  CONSTRAINT cubebox_state_transitions_turn_action_check CHECK (
    turn_action IS NULL OR turn_action IN ('confirm', 'commit')
  )
);

CREATE INDEX IF NOT EXISTS cubebox_state_transitions_lookup_idx
  ON iam.cubebox_state_transitions (tenant_uuid, conversation_id, changed_at, id);

ALTER TABLE iam.cubebox_state_transitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_state_transitions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_state_transitions;
CREATE POLICY tenant_isolation ON iam.cubebox_state_transitions
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

CREATE TABLE IF NOT EXISTS iam.cubebox_idempotency (
  tenant_uuid uuid NOT NULL,
  conversation_id text NOT NULL,
  turn_id text NOT NULL,
  turn_action text NOT NULL,
  request_id text NOT NULL,
  request_hash text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  http_status integer NULL,
  error_code text NULL,
  response_body jsonb NULL,
  response_hash text NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  finalized_at timestamptz NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (tenant_uuid, conversation_id, turn_id, turn_action, request_id),
  CONSTRAINT cubebox_idempotency_turn_fk FOREIGN KEY (tenant_uuid, conversation_id, turn_id)
    REFERENCES iam.cubebox_turns(tenant_uuid, conversation_id, turn_id) ON DELETE CASCADE,
  CONSTRAINT cubebox_idempotency_turn_action_check CHECK (
    turn_action IN ('confirm', 'commit')
  ),
  CONSTRAINT cubebox_idempotency_status_check CHECK (
    status IN ('pending', 'done')
  ),
  CONSTRAINT cubebox_idempotency_response_size_check CHECK (
    response_body IS NULL OR octet_length(response_body::text) <= 65536
  )
);

CREATE INDEX IF NOT EXISTS cubebox_idempotency_expire_idx
  ON iam.cubebox_idempotency (tenant_uuid, expires_at);

ALTER TABLE iam.cubebox_idempotency ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.cubebox_idempotency FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON iam.cubebox_idempotency;
CREATE POLICY tenant_isolation ON iam.cubebox_idempotency
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.cubebox_idempotency;
DROP TABLE IF EXISTS iam.cubebox_state_transitions;
DROP TABLE IF EXISTS iam.cubebox_turns;
DROP TABLE IF EXISTS iam.cubebox_conversations;
-- +goose StatementEnd
