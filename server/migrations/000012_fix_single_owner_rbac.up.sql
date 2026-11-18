-- Corrective, idempotent RBAC data fix:
-- Keep role='owner' and is_superadmin=TRUE only for the earliest-created user;
-- set role='member' and is_superadmin=FALSE for all other users, and log 'role_corrected'.
WITH earliest AS (
    SELECT id
    FROM users
    ORDER BY created_at ASC, id ASC
    LIMIT 1
),
updated AS (
    UPDATE users u
    SET role = CASE WHEN u.id = e.id THEN 'owner' ELSE 'member' END,
        is_superadmin = (u.id = e.id)
    FROM earliest e
    WHERE (u.id = e.id AND (u.role <> 'owner' OR u.is_superadmin IS NOT TRUE))
       OR (u.id <> e.id AND (u.role <> 'member' OR u.is_superadmin IS NOT FALSE))
    RETURNING u.id, u.role, u.is_superadmin
)
INSERT INTO auth_audit_log (user_id, event, metadata)
SELECT
    updated.id,
    'role_corrected',
    jsonb_build_object(
        'new_role', updated.role,
        'is_superadmin', updated.is_superadmin
    )
FROM updated;

