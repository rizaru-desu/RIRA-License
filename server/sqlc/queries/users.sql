-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash,
    name,
    avatar_url,
    email_verified_at
) VALUES (
    lower($1),
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE lower(email) = lower($1)
  AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateUserEmailVerified :exec
UPDATE users
SET email_verified_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = lower($2),
    email_verified_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2
WHERE id = $1
  AND deleted_at IS NULL;

-- name: IncrementFailedLogin :one
UPDATE users
SET failed_login_attempts = failed_login_attempts + 1,
    locked_until = CASE
        WHEN failed_login_attempts + 1 >= sqlc.arg(max_attempts)::int THEN sqlc.arg(lock_until)::timestamptz
        ELSE locked_until
    END
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING failed_login_attempts, locked_until;

-- name: ResetLoginFailuresAndUpdateLastLogin :exec
UPDATE users
SET failed_login_attempts = 0,
    locked_until = NULL,
    last_login_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: SetPendingTOTPSecret :exec
UPDATE users
SET totp_secret_encrypted = $2,
    totp_verified_at = NULL
WHERE id = $1
  AND deleted_at IS NULL;

-- name: EnableTOTP :exec
UPDATE users
SET two_factor_enabled = true,
    two_factor_method = 'totp',
    totp_verified_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: EnableEmail2FA :exec
UPDATE users
SET two_factor_enabled = true,
    two_factor_method = 'email'
WHERE id = $1
  AND deleted_at IS NULL;

-- name: DisableTwoFactor :exec
UPDATE users
SET two_factor_enabled = false,
    two_factor_method = NULL,
    totp_secret_encrypted = NULL,
    totp_verified_at = NULL
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UpdateUserAvatarIfEmpty :exec
UPDATE users
SET avatar_url = COALESCE(avatar_url, $2)
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetOAuthAccount :one
SELECT *
FROM oauth_accounts
WHERE provider = $1
  AND provider_user_id = $2
LIMIT 1;

-- name: CreateOAuthAccount :one
INSERT INTO oauth_accounts (
    user_id,
    provider,
    provider_user_id
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (provider, provider_user_id) DO NOTHING
RETURNING *;

