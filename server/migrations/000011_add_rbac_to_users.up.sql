-- Add RBAC role, superadmin flag, and suspension metadata to users.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'member'
        CONSTRAINT users_role_check CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    ADD COLUMN IF NOT EXISTS is_superadmin BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS suspended_reason TEXT NULL,
    ADD COLUMN IF NOT EXISTS suspended_at TIMESTAMPTZ NULL;

-- Idempotent data backfill: only the earliest-created user becomes owner + superadmin;
-- all other pre-existing users remain/become 'member' and non-superadmin.
WITH earliest AS (
    SELECT id
    FROM users
    ORDER BY created_at ASC, id ASC
    LIMIT 1
)
UPDATE users u
SET role = CASE WHEN u.id = e.id THEN 'owner' ELSE 'member' END,
    is_superadmin = (u.id = e.id)
FROM earliest e
WHERE (u.id = e.id AND (u.role <> 'owner' OR u.is_superadmin IS NOT TRUE))
   OR (u.id <> e.id AND (u.role <> 'member' OR u.is_superadmin IS NOT FALSE));
