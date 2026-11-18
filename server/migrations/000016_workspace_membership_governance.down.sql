-- Revert Workspace Membership Governance schema additions.

DROP INDEX IF EXISTS workspace_members_active_idx;

DELETE FROM workspace_members WHERE status = 'removed';

ALTER TABLE workspace_members
    DROP CONSTRAINT IF EXISTS workspace_members_status_check;

ALTER TABLE workspace_members
    ADD CONSTRAINT workspace_members_status_check
        CHECK (status IN ('pending', 'active', 'suspended'));

ALTER TABLE workspace_members
    DROP COLUMN IF EXISTS removal_reason,
    DROP COLUMN IF EXISTS removed_by,
    DROP COLUMN IF EXISTS removed_at;

