-- Create users table and updated_at trigger function.
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

