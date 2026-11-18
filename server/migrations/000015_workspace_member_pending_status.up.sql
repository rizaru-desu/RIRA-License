-- Allow 'pending' status in workspace_members for invitation consent flow.

ALTER TABLE workspace_members
    DROP CONSTRAINT IF EXISTS workspace_members_status_check;

ALTER TABLE workspace_members
    ADD CONSTRAINT workspace_members_status_check
        CHECK (status IN ('pending', 'active', 'suspended'));

