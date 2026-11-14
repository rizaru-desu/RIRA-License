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

