-- name: GetTaskByID :one
SELECT *
FROM iam.cubebox_tasks
WHERE tenant_uuid = $1
  AND task_id = $2;

-- name: GetTaskByIDForUpdate :one
SELECT *
FROM iam.cubebox_tasks
WHERE tenant_uuid = $1
  AND task_id = $2
FOR UPDATE;

-- name: GetTaskBySubmitKey :one
SELECT *
FROM iam.cubebox_tasks
WHERE tenant_uuid = $1
  AND conversation_id = $2
  AND turn_id = $3
  AND request_id = $4;

-- name: GetConversationActorByTaskID :one
SELECT c.actor_id
FROM iam.cubebox_tasks AS t
INNER JOIN iam.cubebox_conversations AS c
  ON c.tenant_uuid = t.tenant_uuid
 AND c.conversation_id = t.conversation_id
WHERE t.tenant_uuid = $1
  AND t.task_id = $2;

-- name: InsertTask :one
INSERT INTO iam.cubebox_tasks (
  tenant_uuid,
  task_id,
  conversation_id,
  turn_id,
  task_type,
  request_id,
  request_hash,
  workflow_id,
  status,
  dispatch_status,
  dispatch_attempt,
  dispatch_deadline_at,
  attempt,
  max_attempts,
  last_error_code,
  trace_id,
  intent_schema_version,
  compiler_contract_version,
  capability_map_version,
  skill_manifest_digest,
  context_hash,
  intent_hash,
  plan_hash,
  knowledge_snapshot_digest,
  route_catalog_version,
  resolver_contract_version,
  context_template_version,
  reply_guidance_version,
  policy_context_digest,
  effective_policy_version,
  resolved_setid,
  setid_source,
  precheck_projection_digest,
  mutation_policy_version,
  submitted_at,
  cancel_requested_at,
  completed_at,
  created_at,
  updated_at
) VALUES (
  $1,  $2,  $3,  $4,  $5,  $6,  $7,  $8,  $9,  $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
  $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
  $31, $32, $33, $34, $35, $36, $37, $38, $39
)
RETURNING *;

-- name: InsertTaskEvent :exec
INSERT INTO iam.cubebox_task_events (
  tenant_uuid,
  task_id,
  from_status,
  to_status,
  event_type,
  error_code,
  payload,
  occurred_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: UpsertTaskDispatchOutbox :exec
INSERT INTO iam.cubebox_task_dispatch_outbox (
  tenant_uuid,
  task_id,
  workflow_id,
  status,
  attempt,
  next_retry_at,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (tenant_uuid, task_id)
DO UPDATE SET
  workflow_id = EXCLUDED.workflow_id,
  status = EXCLUDED.status,
  attempt = EXCLUDED.attempt,
  next_retry_at = EXCLUDED.next_retry_at,
  updated_at = EXCLUDED.updated_at;

-- name: UpdateTaskState :one
UPDATE iam.cubebox_tasks
SET status = $3,
    dispatch_status = $4,
    dispatch_attempt = $5,
    attempt = $6,
    last_error_code = $7,
    cancel_requested_at = $8,
    completed_at = $9,
    updated_at = $10
WHERE tenant_uuid = $1
  AND task_id = $2
RETURNING *;

-- name: UpdateTaskDispatchOutbox :execrows
UPDATE iam.cubebox_task_dispatch_outbox
SET status = $3,
    attempt = $4,
    next_retry_at = $5,
    updated_at = $6
WHERE tenant_uuid = $1
  AND task_id = $2;

-- name: MarkTaskOutboxCanceled :execrows
UPDATE iam.cubebox_task_dispatch_outbox
SET status = 'canceled',
    updated_at = $3
WHERE tenant_uuid = $1
  AND task_id = $2
  AND status <> 'canceled';

-- name: ListTaskEventsByTask :many
SELECT *
FROM iam.cubebox_task_events
WHERE tenant_uuid = $1
  AND task_id = $2
ORDER BY occurred_at ASC, id ASC;

-- name: ListDispatchOutboxByStatus :many
SELECT *
FROM iam.cubebox_task_dispatch_outbox
WHERE tenant_uuid = $1
  AND status = $2
  AND next_retry_at <= now()
ORDER BY next_retry_at ASC, id ASC
LIMIT $3;
