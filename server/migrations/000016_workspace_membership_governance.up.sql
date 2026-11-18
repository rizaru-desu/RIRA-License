-- Workspace Membership Governance: soft-removal status, removal metadata, and active-membership indexing.

ALTER TABLE workspace_members
    DROP CONSTRAINT IF EXISTS workspace_members_status_check;

ALTER TABLE workspace_members
    ADD CONSTRAINT workspace_members_status_check
        CHECK (status IN ('pending', 'active', 'suspended', 'removed'));

ALTER TABLE workspace_members
    ADD COLUMN IF NOT EXISTS removed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS removed_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS removal_reason TEXT NULL;

CREATE INDEX IF NOT EXISTS workspace_members_active_idx
    ON workspace_members (workspace_id, status, role)
    WHERE status <> 'removed';

