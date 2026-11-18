ALTER TABLE users
    DROP COLUMN IF EXISTS suspended_at,
    DROP COLUMN IF EXISTS suspended_reason,
    DROP COLUMN IF EXISTS is_superadmin,
    DROP COLUMN IF EXISTS role;

