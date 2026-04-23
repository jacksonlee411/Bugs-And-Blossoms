-- name: CreateConversation :one
INSERT INTO iam.cubebox_conversations (
  tenant_uuid,
  conversation_id,
  principal_id,
  title,
  status,
  archived,
  created_at,
  updated_at,
  archived_at
) VALUES (
  $1::uuid,
  $2,
  $3::uuid,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9
)
RETURNING
  tenant_uuid,
  conversation_id,
  principal_id,
  title,
  status,
  archived,
  created_at,
  updated_at,
  archived_at;

-- name: GetConversation :one
SELECT
  tenant_uuid,
  conversation_id,
  principal_id,
  title,
  status,
  archived,
  created_at,
  updated_at,
  archived_at
FROM iam.cubebox_conversations
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND principal_id = $3::uuid;

-- name: ListConversations :many
SELECT
  tenant_uuid,
  conversation_id,
  principal_id,
  title,
  status,
  archived,
  created_at,
  updated_at,
  archived_at
FROM iam.cubebox_conversations
WHERE tenant_uuid = $1::uuid
  AND principal_id = $2::uuid
ORDER BY updated_at DESC, conversation_id DESC
LIMIT $3;

-- name: UpdateConversationTitle :one
UPDATE iam.cubebox_conversations
SET
  title = $4,
  updated_at = $5
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND principal_id = $3::uuid
RETURNING
  tenant_uuid,
  conversation_id,
  principal_id,
  title,
  status,
  archived,
  created_at,
  updated_at,
  archived_at;

-- name: UpdateConversationArchive :one
UPDATE iam.cubebox_conversations
SET
  status = $4,
  archived = $5,
  archived_at = $6,
  updated_at = $7
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
  AND principal_id = $3::uuid
RETURNING
  tenant_uuid,
  conversation_id,
  principal_id,
  title,
  status,
  archived,
  created_at,
  updated_at,
  archived_at;

-- name: AppendConversationEvent :one
INSERT INTO iam.cubebox_conversation_events (
  tenant_uuid,
  conversation_id,
  event_id,
  sequence,
  turn_id,
  event_type,
  payload,
  created_at
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8
)
RETURNING
  tenant_uuid,
  conversation_id,
  event_id,
  sequence,
  turn_id,
  event_type,
  payload,
  created_at;

-- name: ListConversationEvents :many
SELECT
  tenant_uuid,
  conversation_id,
  event_id,
  sequence,
  turn_id,
  event_type,
  payload,
  created_at
FROM iam.cubebox_conversation_events
WHERE tenant_uuid = $1::uuid
  AND conversation_id = $2
ORDER BY sequence ASC;
