DROP INDEX IF EXISTS auth_audit_log_workspace_id_idx;

ALTER TABLE auth_audit_log
    DROP COLUMN IF EXISTS actor_user_id,
    DROP COLUMN IF EXISTS workspace_id;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS active_workspace_id;

DROP TRIGGER IF EXISTS workspace_members_set_updated_at ON workspace_members;
DROP TABLE IF EXISTS workspace_members;

DROP TRIGGER IF EXISTS workspaces_set_updated_at ON workspaces;
DROP TABLE IF EXISTS workspaces;

