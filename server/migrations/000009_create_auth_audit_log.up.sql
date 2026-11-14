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

