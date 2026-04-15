-- name: ListFilesByTenant :many
SELECT *
FROM iam.cubebox_files
WHERE tenant_uuid = $1
ORDER BY uploaded_at DESC, file_id DESC
LIMIT $2;

-- name: GetFileByID :one
SELECT *
FROM iam.cubebox_files
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

-- name: ListFileLinksByConversation :many
SELECT *
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1
  AND conversation_id = $2
ORDER BY created_at ASC, id ASC;
