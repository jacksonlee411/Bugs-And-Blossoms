-- name: GetTaskByID :one
SELECT *
FROM iam.cubebox_tasks
WHERE tenant_uuid = $1
  AND task_id = $2;

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
ORDER BY next_retry_at ASC, id ASC
LIMIT $3;
