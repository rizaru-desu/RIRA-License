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

-- name: CountAuthEventsByDay :many
SELECT
    date_trunc('day', created_at)::timestamptz AS day,
    event,
    count(*)::bigint AS total
FROM auth_audit_log
WHERE created_at >= $1
GROUP BY date_trunc('day', created_at), event
ORDER BY day ASC, event ASC;

