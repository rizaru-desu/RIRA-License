-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    user_agent,
    ip_address,
    expires_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetSessionWithUserByTokenHash :one
SELECT
    s.id AS session_id,
    s.user_id AS session_user_id,
    s.token_hash AS session_token_hash,
    s.user_agent AS session_user_agent,
    s.ip_address AS session_ip_address,
    s.expires_at AS session_expires_at,
    s.last_activity_at AS session_last_activity_at,
    s.created_at AS session_created_at,
    u.id,
    u.email,
    u.password_hash,
    u.name,
    u.avatar_url,
    u.email_verified_at,
    u.status,
    u.deleted_at,
    u.two_factor_enabled,
    u.two_factor_method,
    u.totp_secret_encrypted,
    u.totp_verified_at,
    u.failed_login_attempts,
    u.locked_until,
    u.last_login_at,
    u.created_at,
    u.updated_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1
LIMIT 1;

-- name: TouchSessionActivity :exec
UPDATE sessions
SET last_activity_at = now()
WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = $1;

-- name: DeleteSessionByID :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions
WHERE user_id = $1;

