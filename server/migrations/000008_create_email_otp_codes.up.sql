CREATE TABLE email_otp_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    attempts   INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX email_otp_codes_user_recent_idx ON email_otp_codes (user_id, created_at DESC);

