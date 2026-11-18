-- name: InsertAuthAuditLog :exec
INSERT INTO auth_audit_log (
    user_id,
    event,
    ip_address,
    user_agent,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: InsertWorkspaceAuditLog :exec
INSERT INTO auth_audit_log (
    user_id,
    workspace_id,
    actor_user_id,
    event,
    ip_address,
    user_agent,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
);

-- name: CountAuthEventsByDay :many
SELECT
    date_trunc('day', created_at)::timestamptz AS day,
    event,
    count(*)::bigint AS total
FROM auth_audit_log
WHERE created_at >= $1
GROUP BY date_trunc('day', created_at), event
ORDER BY day ASC, event ASC;

-- name: AnonymizeUserAuditLogs :exec
UPDATE auth_audit_log
SET user_id = NULL,
    ip_address = NULL,
    user_agent = NULL,
    metadata = NULL
WHERE user_id = $1;

-- name: ListWorkspaceAuditLogs :many
SELECT *
FROM auth_audit_log
WHERE workspace_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;


