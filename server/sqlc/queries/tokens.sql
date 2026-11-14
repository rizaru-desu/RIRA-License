-- name: CreateVerificationToken :one
INSERT INTO verification_tokens (
    user_id,
    type,
    token_hash,
    new_email,
    expires_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetVerificationTokenByHash :one
SELECT *
FROM verification_tokens
WHERE token_hash = $1
  AND type = $2
LIMIT 1;

-- name: MarkVerificationTokenUsed :one
UPDATE verification_tokens
SET used_at = now()
WHERE id = $1
  AND used_at IS NULL
RETURNING *;

-- name: InvalidateUnusedVerificationTokens :exec
UPDATE verification_tokens
SET used_at = now()
WHERE user_id = $1
  AND type = $2
  AND used_at IS NULL;

-- name: InsertPasswordHistory :exec
INSERT INTO password_history (
    user_id,
    password_hash
) VALUES (
    $1,
    $2
);

-- name: GetRecentPasswordHistory :many
SELECT password_hash
FROM password_history
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 5;

-- name: PrunePasswordHistory :exec
DELETE FROM password_history ph
WHERE ph.user_id = $1
  AND ph.id NOT IN (
      SELECT ph2.id
      FROM password_history ph2
      WHERE ph2.user_id = $1
      ORDER BY ph2.created_at DESC, ph2.id DESC
      LIMIT 5
  );

-- name: InvalidateActiveEmailOTPs :exec
UPDATE email_otp_codes
SET used_at = now()
WHERE user_id = $1
  AND used_at IS NULL;

-- name: CreateEmailOTPCode :one
INSERT INTO email_otp_codes (
    user_id,
    code_hash,
    expires_at
) VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: GetLatestActiveEmailOTP :one
SELECT *
FROM email_otp_codes
WHERE user_id = $1
  AND used_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementEmailOTPAttempts :one
UPDATE email_otp_codes
SET attempts = attempts + 1,
    used_at = CASE
        WHEN attempts + 1 >= sqlc.arg(max_attempts)::int THEN now()
        ELSE used_at
    END
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: MarkEmailOTPUsed :one
UPDATE email_otp_codes
SET used_at = now()
WHERE id = $1
  AND used_at IS NULL
RETURNING *;
