-- name: CreateWorkspace :one
INSERT INTO workspaces (
    name,
    slug,
    plan,
    created_by
) VALUES (
    $1,
    lower($2),
    $3,
    $4
)
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT *
FROM workspaces
WHERE id = $1
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetWorkspaceByIDForUpdate :one
SELECT *
FROM workspaces
WHERE id = $1
  AND deleted_at IS NULL
LIMIT 1
FOR UPDATE;

-- name: GetWorkspaceBySlug :one
SELECT *
FROM workspaces
WHERE lower(slug) = lower($1)
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetFirstActiveWorkspace :one
SELECT *
FROM workspaces
WHERE deleted_at IS NULL
ORDER BY created_at ASC, id ASC
LIMIT 1;

-- name: ListWorkspacesForUser :many
SELECT
    w.*,
    wm.role AS member_role,
    wm.status AS member_status
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = $1
  AND wm.status = 'active'
  AND w.deleted_at IS NULL
ORDER BY w.created_at ASC, w.id ASC;

-- name: ListAllWorkspaces :many
SELECT
    w.*,
    COUNT(wm.id) FILTER (WHERE u.id IS NOT NULL AND u.is_superadmin = FALSE AND u.deleted_at IS NULL AND wm.status <> 'removed')::bigint AS member_count
FROM workspaces w
LEFT JOIN workspace_members wm ON wm.workspace_id = w.id
LEFT JOIN users u ON u.id = wm.user_id
WHERE w.deleted_at IS NULL
GROUP BY w.id
ORDER BY w.created_at ASC, w.id ASC;

-- name: UpdateWorkspacePlanAndOverrides :one
UPDATE workspaces
SET plan = $2,
    max_users_override = $3,
    max_products_override = $4,
    max_licenses_override = $5,
    override_expires_at = $6,
    override_note = $7
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: SuspendWorkspace :one
UPDATE workspaces
SET status = 'suspended',
    suspended_reason = $2,
    suspended_at = now(),
    suspended_by = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: ActivateWorkspace :one
UPDATE workspaces
SET status = 'active',
    suspended_reason = NULL,
    suspended_at = NULL,
    suspended_by = NULL
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateWorkspaceDetails :one
UPDATE workspaces
SET name = $2,
    slug = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteWorkspace :one
UPDATE workspaces
SET deleted_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: CreateWorkspaceMember :one
INSERT INTO workspace_members (
    workspace_id,
    user_id,
    role,
    invited_by
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: CreateWorkspaceMemberWithStatus :one
INSERT INTO workspace_members (
    workspace_id,
    user_id,
    role,
    status,
    invited_by
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: AcceptWorkspaceMemberInvite :one
UPDATE workspace_members
SET status = 'active',
    joined_at = now(),
    removed_at = NULL,
    removed_by = NULL,
    removal_reason = NULL,
    suspended_reason = NULL,
    suspended_at = NULL,
    suspended_by = NULL
WHERE workspace_id = $1
  AND user_id = $2
  AND status = 'pending'
RETURNING *;

-- name: DeleteWorkspaceMember :exec
DELETE FROM workspace_members
WHERE workspace_id = $1
  AND user_id = $2;

-- name: RemoveWorkspaceMember :one
UPDATE workspace_members
SET status = 'removed',
    removed_at = now(),
    removed_by = $3,
    removal_reason = $4,
    suspended_reason = NULL,
    suspended_at = NULL,
    suspended_by = NULL
WHERE workspace_id = $1
  AND user_id = $2
  AND status <> 'removed'
RETURNING *;

-- name: ReinviteWorkspaceMember :one
UPDATE workspace_members
SET role = $3,
    status = 'pending',
    invited_by = $4,
    removed_at = NULL,
    removed_by = NULL,
    removal_reason = NULL,
    suspended_reason = NULL,
    suspended_at = NULL,
    suspended_by = NULL
WHERE workspace_id = $1
  AND user_id = $2
  AND status = 'removed'
RETURNING *;

-- name: ReactivateRemovedWorkspaceMember :one
UPDATE workspace_members
SET role = $3,
    status = 'active',
    invited_by = $4,
    joined_at = now(),
    removed_at = NULL,
    removed_by = NULL,
    removal_reason = NULL,
    suspended_reason = NULL,
    suspended_at = NULL,
    suspended_by = NULL
WHERE workspace_id = $1
  AND user_id = $2
  AND status = 'removed'
RETURNING *;

-- name: ListPendingWorkspaceInvitesForUser :many
SELECT
    w.id AS workspace_id,
    w.name AS workspace_name,
    w.slug AS workspace_slug,
    w.plan AS workspace_plan,
    wm.role AS member_role,
    wm.created_at AS invited_at,
    inv.name AS invited_by_name,
    inv.email AS invited_by_email
FROM workspace_members wm
JOIN workspaces w ON w.id = wm.workspace_id
LEFT JOIN users inv ON inv.id = wm.invited_by
WHERE wm.user_id = $1
  AND wm.status = 'pending'
  AND w.status = 'active'
  AND w.deleted_at IS NULL
ORDER BY wm.created_at DESC, w.id ASC;

-- name: GetWorkspaceMember :one
SELECT *
FROM workspace_members
WHERE workspace_id = $1
  AND user_id = $2
LIMIT 1;

-- name: CountActiveWorkspaceOwners :one
SELECT COUNT(*)
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
  AND wm.role = 'owner'
  AND wm.status = 'active'
  AND u.status = 'active'
  AND u.deleted_at IS NULL;

-- name: GetWorkspaceActiveMembershipStats :one
SELECT
    COUNT(*) FILTER (WHERE wm.status = 'active' AND u.status = 'active' AND u.deleted_at IS NULL)::bigint AS active_members,
    COUNT(*) FILTER (WHERE wm.status = 'active' AND wm.role IN ('owner', 'admin') AND u.status = 'active' AND u.deleted_at IS NULL)::bigint AS active_admins,
    COUNT(*) FILTER (WHERE wm.status = 'active' AND wm.role = 'owner' AND u.status = 'active' AND u.deleted_at IS NULL)::bigint AS active_owners
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
  AND u.is_superadmin = FALSE;

-- name: ListActiveWorkspacesForUserForUpdate :many
SELECT
    w.id AS workspace_id,
    w.name AS workspace_name,
    w.slug AS workspace_slug,
    wm.role AS member_role,
    wm.status AS member_status
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = $1
  AND wm.status = 'active'
  AND w.deleted_at IS NULL
ORDER BY w.id ASC
FOR UPDATE OF w;

-- name: ClearUserWorkspaceSession :exec
UPDATE sessions
SET active_workspace_id = NULL
WHERE user_id = $1
  AND active_workspace_id = $2;

-- name: ClearWorkspaceSessions :exec
UPDATE sessions
SET active_workspace_id = NULL
WHERE active_workspace_id = $1;

-- name: CountWorkspaceMembers :one
SELECT COUNT(*)
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
  AND wm.status <> 'removed'
  AND u.is_superadmin = FALSE
  AND u.deleted_at IS NULL;

-- name: ListWorkspaceMembersWithUsers :many
SELECT
    u.id,
    u.email,
    u.name,
    u.avatar_url,
    u.is_superadmin,
    u.status AS user_status,
    u.email_verified_at,
    u.created_at,
    wm.role AS member_role,
    wm.status AS member_status,
    wm.suspended_reason AS member_suspended_reason,
    wm.suspended_at AS member_suspended_at,
    wm.joined_at
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id
WHERE wm.workspace_id = $1
  AND wm.status <> 'removed'
  AND u.is_superadmin = FALSE
  AND u.deleted_at IS NULL
ORDER BY wm.joined_at ASC, u.id ASC;

-- name: UpdateWorkspaceMemberRole :one
UPDATE workspace_members
SET role = $3
WHERE workspace_id = $1
  AND user_id = $2
RETURNING *;

-- name: SuspendWorkspaceMember :one
UPDATE workspace_members
SET status = 'suspended',
    suspended_reason = $3,
    suspended_at = now(),
    suspended_by = $4
WHERE workspace_id = $1
  AND user_id = $2
RETURNING *;

-- name: ActivateWorkspaceMember :one
UPDATE workspace_members
SET status = 'active',
    suspended_reason = NULL,
    suspended_at = NULL,
    suspended_by = NULL
WHERE workspace_id = $1
  AND user_id = $2
RETURNING *;

-- name: ListPlatformUsers :many
SELECT
    u.*,
    COUNT(wm.id)::bigint AS workspace_count
FROM users u
LEFT JOIN workspace_members wm ON wm.user_id = u.id AND wm.status <> 'removed'
WHERE u.deleted_at IS NULL
GROUP BY u.id
ORDER BY u.created_at ASC, u.id ASC;

