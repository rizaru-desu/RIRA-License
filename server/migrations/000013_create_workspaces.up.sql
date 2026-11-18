-- Add multi-tenant workspaces and workspace_members tables, plus session/audit workspace context.

CREATE TABLE IF NOT EXISTS workspaces (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  TEXT NOT NULL,
    slug                  TEXT NOT NULL,
    plan                  TEXT NOT NULL DEFAULT 'free'
                              CHECK (plan IN ('free', 'pro', 'enterprise')),
    status                TEXT NOT NULL DEFAULT 'active'
                              CHECK (status IN ('active', 'suspended')),
    suspended_reason      TEXT NULL,
    suspended_at          TIMESTAMPTZ NULL,
    suspended_by          UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    max_users_override    INT NULL CHECK (max_users_override IS NULL OR max_users_override > 0),
    max_products_override INT NULL CHECK (max_products_override IS NULL OR max_products_override > 0),
    max_licenses_override INT NULL CHECK (max_licenses_override IS NULL OR max_licenses_override > 0),
    override_expires_at   TIMESTAMPTZ NULL,
    override_note         TEXT NULL,
    created_by            UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    deleted_at            TIMESTAMPTZ NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS workspaces_slug_active_idx
    ON workspaces (lower(slug))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS workspaces_status_idx
    ON workspaces (status)
    WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS workspaces_set_updated_at ON workspaces;
CREATE TRIGGER workspaces_set_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workspace_members (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role             TEXT NOT NULL DEFAULT 'member'
                         CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    status           TEXT NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active', 'suspended')),
    suspended_reason TEXT NULL,
    suspended_at     TIMESTAMPTZ NULL,
    suspended_by     UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    invited_by       UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    joined_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, user_id)
);

CREATE INDEX IF NOT EXISTS workspace_members_user_idx
    ON workspace_members (user_id);

CREATE INDEX IF NOT EXISTS workspace_members_workspace_role_idx
    ON workspace_members (workspace_id, role, status);

DROP TRIGGER IF EXISTS workspace_members_set_updated_at ON workspace_members;
CREATE TRIGGER workspace_members_set_updated_at
    BEFORE UPDATE ON workspace_members
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS active_workspace_id UUID NULL
        REFERENCES workspaces(id) ON DELETE SET NULL;

ALTER TABLE auth_audit_log
    ADD COLUMN IF NOT EXISTS workspace_id UUID NULL
        REFERENCES workspaces(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS actor_user_id UUID NULL
        REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS auth_audit_log_workspace_id_idx
    ON auth_audit_log (workspace_id);

-- Idempotent backfill: if users already exist and no default workspace exists yet,
-- create the initial Default Workspace and migrate existing users into workspace_members.
WITH earliest_owner AS (
    SELECT id
    FROM users
    WHERE deleted_at IS NULL
    ORDER BY (role = 'owner') DESC, created_at ASC, id ASC
    LIMIT 1
),
inserted_ws AS (
    INSERT INTO workspaces (name, slug, plan, status, created_by)
    SELECT 'Default Workspace', 'default', 'free', 'active', e.id
    FROM earliest_owner e
    WHERE NOT EXISTS (SELECT 1 FROM workspaces WHERE deleted_at IS NULL)
    RETURNING id
),
target_ws AS (
    SELECT id FROM inserted_ws
    UNION ALL
    (SELECT id FROM workspaces WHERE deleted_at IS NULL ORDER BY created_at ASC, id ASC LIMIT 1)
)
INSERT INTO workspace_members (
    workspace_id,
    user_id,
    role,
    status,
    suspended_reason,
    suspended_at,
    joined_at
)
SELECT
    w.id,
    u.id,
    CASE WHEN u.role IN ('owner', 'admin', 'member', 'viewer') THEN u.role ELSE 'member' END,
    CASE WHEN u.status IN ('active', 'suspended') THEN u.status ELSE 'active' END,
    u.suspended_reason,
    u.suspended_at,
    u.created_at
FROM users u
CROSS JOIN (SELECT id FROM target_ws LIMIT 1) w
WHERE u.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id) DO NOTHING;

WITH default_ws AS (
    SELECT id FROM workspaces WHERE deleted_at IS NULL ORDER BY created_at ASC, id ASC LIMIT 1
)
UPDATE sessions s
SET active_workspace_id = w.id
FROM default_ws w
WHERE s.active_workspace_id IS NULL;

