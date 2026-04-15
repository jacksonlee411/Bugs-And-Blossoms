-- name: ListConversationsByActor :many
SELECT *
FROM iam.cubebox_conversations
WHERE tenant_uuid = $1
  AND actor_id = $2
ORDER BY updated_at DESC, conversation_id DESC
LIMIT $3;

-- name: GetConversationByID :one
SELECT *
FROM iam.cubebox_conversations
WHERE tenant_uuid = $1
  AND conversation_id = $2;

-- name: ListConversationTurns :many
SELECT *
FROM iam.cubebox_turns
WHERE tenant_uuid = $1
  AND conversation_id = $2
ORDER BY created_at ASC, turn_id ASC;

-- name: ListConversationStateTransitions :many
SELECT *
FROM iam.cubebox_state_transitions
WHERE tenant_uuid = $1
  AND conversation_id = $2
ORDER BY changed_at ASC, id ASC;

-- name: CountBlockingTasksForConversation :one
SELECT count(*)
FROM iam.cubebox_tasks
WHERE tenant_uuid = $1
  AND conversation_id = $2
  AND status NOT IN ('succeeded', 'failed', 'canceled');

-- name: DeleteConversationByID :execrows
DELETE FROM iam.cubebox_conversations
WHERE tenant_uuid = $1
  AND conversation_id = $2;
