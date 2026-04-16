-- name: ListFilesByTenant :many
SELECT *
FROM iam.cubebox_files
WHERE tenant_uuid = $1
ORDER BY uploaded_at DESC, file_id DESC
LIMIT $2;

-- name: InsertFile :one
INSERT INTO iam.cubebox_files (
  tenant_uuid,
  file_id,
  storage_provider,
  storage_key,
  file_name,
  media_type,
  size_bytes,
  sha256,
  scan_status,
  uploaded_by,
  uploaded_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetFileByID :one
SELECT *
FROM iam.cubebox_files
WHERE tenant_uuid = $1
  AND file_id = $2;

-- name: DeleteFileByID :execrows
DELETE FROM iam.cubebox_files
WHERE tenant_uuid = $1
  AND file_id = $2;

-- name: ListFilesByConversation :many
SELECT f.*
FROM iam.cubebox_files AS f
INNER JOIN iam.cubebox_file_links AS l
  ON l.tenant_uuid = f.tenant_uuid
 AND l.file_id = f.file_id
WHERE f.tenant_uuid = $1
  AND l.conversation_id = $2
ORDER BY f.uploaded_at DESC, f.file_id DESC;

-- name: ListFileLinksByTenant :many
SELECT *
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1
ORDER BY created_at ASC, id ASC;

-- name: ListFileLinksByConversation :many
SELECT *
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1
  AND conversation_id = $2
ORDER BY created_at ASC, id ASC;

-- name: ListFileLinksByFileID :many
SELECT *
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1
  AND file_id = $2
ORDER BY created_at ASC, id ASC;

-- name: CountFileLinksByFileID :one
SELECT COUNT(*)::bigint
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1
  AND file_id = $2;

-- name: InsertConversationFileLink :one
INSERT INTO iam.cubebox_file_links (
  tenant_uuid,
  file_id,
  conversation_id,
  turn_id,
  link_role,
  created_by
) VALUES (
  $1, $2, $3, NULL, 'conversation_attachment', $4
)
RETURNING *;

-- name: ConversationExists :one
SELECT EXISTS(
  SELECT 1
  FROM iam.cubebox_conversations
  WHERE tenant_uuid = $1
    AND conversation_id = $2
) AS exists;

-- name: InsertFileCleanupJob :one
INSERT INTO iam.cubebox_file_cleanup_jobs (
  tenant_uuid,
  file_id,
  storage_provider,
  storage_key,
  cleanup_reason,
  status,
  attempt_count,
  next_retry_at,
  last_error,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;
