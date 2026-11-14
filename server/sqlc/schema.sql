-- Combined auth v1 schema matching server/migrations/000001..000009.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 TEXT NOT NULL UNIQUE,
    password_hash         TEXT NULL,
    name                  TEXT NOT NULL,
    avatar_url            TEXT NULL,
    email_verified_at     TIMESTAMPTZ NULL,
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    deleted_at            TIMESTAMPTZ NULL,
    two_factor_enabled    BOOLEAN NOT NULL DEFAULT false,
    two_factor_method     TEXT NULL CHECK (two_factor_method IN ('totp', 'email')),
    totp_secret_encrypted BYTEA NULL,
    totp_verified_at      TIMESTAMPTZ NULL,
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ NULL,
    last_login_at         TIMESTAMPTZ NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE oauth_accounts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL CHECK (provider IN ('github', 'google')),
    provider_user_id TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX oauth_accounts_user_id_idx ON oauth_accounts (user_id);

CREATE TABLE sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash       TEXT NOT NULL UNIQUE,
    user_agent       TEXT NULL,
    ip_address       INET NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

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

CREATE TABLE password_history (
    id            BIGSERIAL PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX password_history_user_recent_idx ON password_history (user_id, created_at DESC, id DESC);

CREATE TABLE trusted_devices (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_token_hash TEXT NOT NULL UNIQUE,
    user_agent        TEXT NULL,
    ip_address        INET NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX trusted_devices_user_id_idx ON trusted_devices (user_id);

CREATE TABLE backup_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX backup_codes_user_id_idx ON backup_codes (user_id);

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

CREATE TABLE auth_audit_log (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    event      TEXT NOT NULL,
    ip_address INET NULL,
    user_agent TEXT NULL,
    metadata   JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_audit_log_user_id_idx ON auth_audit_log (user_id);
CREATE INDEX auth_audit_log_event_idx ON auth_audit_log (event);
CREATE INDEX auth_audit_log_created_at_idx ON auth_audit_log (created_at);

