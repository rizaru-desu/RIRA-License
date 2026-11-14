-- name: CreateTrustedDevice :one
INSERT INTO trusted_devices (
    user_id,
    device_token_hash,
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

-- name: GetValidTrustedDevice :one
SELECT *
FROM trusted_devices
WHERE user_id = $1
  AND device_token_hash = $2
  AND expires_at > now()
LIMIT 1;

-- name: DeleteUserTrustedDevices :exec
DELETE FROM trusted_devices
WHERE user_id = $1;

-- name: CreateBackupCode :exec
INSERT INTO backup_codes (
    user_id,
    code_hash
) VALUES (
    $1,
    $2
);

-- name: ListUnusedBackupCodes :many
SELECT *
FROM backup_codes
WHERE user_id = $1
  AND used_at IS NULL
ORDER BY created_at ASC;

-- name: MarkBackupCodeUsed :one
UPDATE backup_codes
SET used_at = now()
WHERE id = $1
  AND used_at IS NULL
RETURNING *;

-- name: DeleteUserBackupCodes :exec
DELETE FROM backup_codes
WHERE user_id = $1;

