-- Keep platform superadmins above tenant workspace memberships and clear their pinned workspace session.

DELETE FROM workspace_members wm
USING users u
WHERE wm.user_id = u.id
  AND u.is_superadmin = TRUE;

UPDATE sessions s
SET active_workspace_id = NULL
FROM users u
WHERE s.user_id = u.id
  AND u.is_superadmin = TRUE
  AND s.active_workspace_id IS NOT NULL;

