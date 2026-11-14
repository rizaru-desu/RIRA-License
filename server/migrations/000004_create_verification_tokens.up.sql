CREATE TABLE verification_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('email_verification', 'password_reset', 'email_change')),
    token_hash TEXT NOT NULL UNIQUE,
    new_email  TEXT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX verification_tokens_user_type_idx ON verification_tokens (user_id, type);
CREATE INDEX verification_tokens_expires_at_idx ON verification_tokens (expires_at);

