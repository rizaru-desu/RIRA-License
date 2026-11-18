package router

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/auth"
	"riralicense/internal/db/sqlc"
)

func (n *noopStore) GetFirstActiveWorkspace(_ context.Context) (sqlc.Workspace, error) {
	return sqlc.Workspace{}, pgx.ErrNoRows
}

func (n *noopStore) CreateWorkspace(_ context.Context, arg sqlc.CreateWorkspaceParams) (sqlc.Workspace, error) {
	return sqlc.Workspace{
		ID:        uuid.New(),
		Name:      arg.Name,
		Slug:      strings.ToLower(arg.Lower),
		Plan:      arg.Plan,
		Status:    "active",
		CreatedBy: arg.CreatedBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (n *noopStore) GetWorkspaceMember(_ context.Context, _ sqlc.GetWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{}, pgx.ErrNoRows
}

func (n *noopStore) CreateWorkspaceMember(_ context.Context, arg sqlc.CreateWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		Role:        arg.Role,
		Status:      "active",
		JoinedAt:    time.Now(),
	}, nil
}

func (n *noopStore) ListWorkspacesForUser(_ context.Context, _ uuid.UUID) ([]sqlc.ListWorkspacesForUserRow, error) {
	return nil, nil
}

func (n *noopStore) SetSessionActiveWorkspace(_ context.Context, _ sqlc.SetSessionActiveWorkspaceParams) error {
	return nil
}

func (n *noopStore) CreateWorkspaceMemberWithStatus(_ context.Context, arg sqlc.CreateWorkspaceMemberWithStatusParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		Role:        arg.Role,
		Status:      arg.Status,
		JoinedAt:    time.Now(),
	}, nil
}

func (n *noopStore) AcceptWorkspaceMemberInvite(_ context.Context, _ sqlc.AcceptWorkspaceMemberInviteParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{}, pgx.ErrNoRows
}

func (n *noopStore) DeleteWorkspaceMember(_ context.Context, _ sqlc.DeleteWorkspaceMemberParams) error {
	return nil
}

func (n *noopStore) ListPendingWorkspaceInvitesForUser(_ context.Context, _ uuid.UUID) ([]sqlc.ListPendingWorkspaceInvitesForUserRow, error) {
	return nil, nil
}

func (n *noopStore) GetWorkspaceByIDForUpdate(_ context.Context, _ uuid.UUID) (sqlc.Workspace, error) {
	return sqlc.Workspace{}, pgx.ErrNoRows
}

func (n *noopStore) GetUserByIDForUpdate(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	return n.GetUserByID(ctx, id)
}

func (n *noopStore) RemoveWorkspaceMember(_ context.Context, _ sqlc.RemoveWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{}, pgx.ErrNoRows
}

func (n *noopStore) UpdateWorkspaceDetails(_ context.Context, _ sqlc.UpdateWorkspaceDetailsParams) (sqlc.Workspace, error) {
	return sqlc.Workspace{}, pgx.ErrNoRows
}

func (n *noopStore) SoftDeleteWorkspace(_ context.Context, _ uuid.UUID) (sqlc.Workspace, error) {
	return sqlc.Workspace{}, pgx.ErrNoRows
}

func (n *noopStore) ClearWorkspaceSessions(_ context.Context, _ uuid.NullUUID) error {
	return nil
}

func (n *noopStore) ReinviteWorkspaceMember(_ context.Context, _ sqlc.ReinviteWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{}, pgx.ErrNoRows
}

func (n *noopStore) ReactivateRemovedWorkspaceMember(_ context.Context, _ sqlc.ReactivateRemovedWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	return sqlc.WorkspaceMember{}, pgx.ErrNoRows
}

func (n *noopStore) GetWorkspaceActiveMembershipStats(_ context.Context, _ uuid.UUID) (sqlc.GetWorkspaceActiveMembershipStatsRow, error) {
	return sqlc.GetWorkspaceActiveMembershipStatsRow{}, nil
}

func (n *noopStore) ListActiveWorkspacesForUserForUpdate(_ context.Context, _ uuid.UUID) ([]sqlc.ListActiveWorkspacesForUserForUpdateRow, error) {
	return nil, nil
}

func (n *noopStore) ClearUserWorkspaceSession(_ context.Context, _ sqlc.ClearUserWorkspaceSessionParams) error {
	return nil
}

func (n *noopStore) ListWorkspaceAuditLogs(_ context.Context, _ sqlc.ListWorkspaceAuditLogsParams) ([]sqlc.AuthAuditLog, error) {
	return nil, nil
}

func wsKey(wsID, userID uuid.UUID) string {
	return wsID.String() + ":" + userID.String()
}

func (s *rbacTestStore) SetSessionActiveWorkspace(_ context.Context, arg sqlc.SetSessionActiveWorkspaceParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if arg.ActiveWorkspaceID.Valid {
		s.sessionWS[arg.ID] = arg.ActiveWorkspaceID.UUID
	} else {
		delete(s.sessionWS, arg.ID)
	}
	return nil
}

func (s *rbacTestStore) InsertWorkspaceAuditLog(_ context.Context, arg sqlc.InsertWorkspaceAuditLogParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaceAudits = append(s.workspaceAudits, arg)
	return nil
}

func (s *rbacTestStore) CreateWorkspace(_ context.Context, arg sqlc.CreateWorkspaceParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	ws := sqlc.Workspace{
		ID:        uuid.New(),
		Name:      arg.Name,
		Slug:      strings.ToLower(arg.Lower),
		Plan:      arg.Plan,
		Status:    "active",
		CreatedBy: arg.CreatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.workspaces[ws.ID] = ws
	return ws, nil
}

func (s *rbacTestStore) GetWorkspaceByID(_ context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[id]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	return ws, nil
}

func (s *rbacTestStore) GetWorkspaceBySlug(_ context.Context, slug string) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.ToLower(slug)
	for _, ws := range s.workspaces {
		if ws.DeletedAt == nil && strings.ToLower(ws.Slug) == target {
			return ws, nil
		}
	}
	return sqlc.Workspace{}, pgx.ErrNoRows
}

func (s *rbacTestStore) GetFirstActiveWorkspace(_ context.Context) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []sqlc.Workspace
	for _, ws := range s.workspaces {
		if ws.DeletedAt == nil {
			list = append(list, ws)
		}
	}
	if len(list) == 0 {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list[0], nil
}

func (s *rbacTestStore) ListWorkspacesForUser(_ context.Context, userID uuid.UUID) ([]sqlc.ListWorkspacesForUserRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.ListWorkspacesForUserRow
	for _, m := range s.workspaceMembers {
		if m.UserID != userID || m.Status != "active" {
			continue
		}
		ws, ok := s.workspaces[m.WorkspaceID]
		if !ok || ws.DeletedAt != nil {
			continue
		}
		out = append(out, sqlc.ListWorkspacesForUserRow{
			ID:                  ws.ID,
			Name:                ws.Name,
			Slug:                ws.Slug,
			Plan:                ws.Plan,
			Status:              ws.Status,
			SuspendedReason:     ws.SuspendedReason,
			SuspendedAt:         ws.SuspendedAt,
			SuspendedBy:         ws.SuspendedBy,
			MaxUsersOverride:    ws.MaxUsersOverride,
			MaxProductsOverride: ws.MaxProductsOverride,
			MaxLicensesOverride: ws.MaxLicensesOverride,
			OverrideExpiresAt:   ws.OverrideExpiresAt,
			OverrideNote:        ws.OverrideNote,
			CreatedBy:           ws.CreatedBy,
			DeletedAt:           ws.DeletedAt,
			CreatedAt:           ws.CreatedAt,
			UpdatedAt:           ws.UpdatedAt,
			MemberRole:          m.Role,
			MemberStatus:        m.Status,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *rbacTestStore) ListAllWorkspaces(_ context.Context) ([]sqlc.ListAllWorkspacesRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.ListAllWorkspacesRow
	for _, ws := range s.workspaces {
		if ws.DeletedAt != nil {
			continue
		}
		var cnt int64
		for _, m := range s.workspaceMembers {
			if m.WorkspaceID == ws.ID && m.Status != "removed" {
				if u, ok := s.users[m.UserID]; ok && !u.IsSuperadmin && u.DeletedAt == nil {
					cnt++
				}
			}
		}
		out = append(out, sqlc.ListAllWorkspacesRow{
			ID:                  ws.ID,
			Name:                ws.Name,
			Slug:                ws.Slug,
			Plan:                ws.Plan,
			Status:              ws.Status,
			SuspendedReason:     ws.SuspendedReason,
			SuspendedAt:         ws.SuspendedAt,
			SuspendedBy:         ws.SuspendedBy,
			MaxUsersOverride:    ws.MaxUsersOverride,
			MaxProductsOverride: ws.MaxProductsOverride,
			MaxLicensesOverride: ws.MaxLicensesOverride,
			OverrideExpiresAt:   ws.OverrideExpiresAt,
			OverrideNote:        ws.OverrideNote,
			CreatedBy:           ws.CreatedBy,
			DeletedAt:           ws.DeletedAt,
			CreatedAt:           ws.CreatedAt,
			UpdatedAt:           ws.UpdatedAt,
			MemberCount:         cnt,
		})
	}
	return out, nil
}

func (s *rbacTestStore) UpdateWorkspaceDetails(_ context.Context, arg sqlc.UpdateWorkspaceDetailsParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[arg.ID]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	ws.Name = arg.Name
	ws.Slug = arg.Slug
	ws.UpdatedAt = time.Now()
	s.workspaces[arg.ID] = ws
	return ws, nil
}

func (s *rbacTestStore) SoftDeleteWorkspace(_ context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[id]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	now := time.Now()
	ws.DeletedAt = &now
	ws.UpdatedAt = now
	s.workspaces[id] = ws
	return ws, nil
}

func (s *rbacTestStore) ClearWorkspaceSessions(_ context.Context, workspaceID uuid.NullUUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !workspaceID.Valid {
		return nil
	}
	for sessID, wsID := range s.sessionWS {
		if wsID == workspaceID.UUID {
			delete(s.sessionWS, sessID)
		}
	}
	return nil
}

func (s *rbacTestStore) UpdateWorkspacePlanAndOverrides(_ context.Context, arg sqlc.UpdateWorkspacePlanAndOverridesParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[arg.ID]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	ws.Plan = arg.Plan
	ws.MaxUsersOverride = arg.MaxUsersOverride
	ws.MaxProductsOverride = arg.MaxProductsOverride
	ws.MaxLicensesOverride = arg.MaxLicensesOverride
	ws.OverrideExpiresAt = arg.OverrideExpiresAt
	ws.OverrideNote = arg.OverrideNote
	s.workspaces[arg.ID] = ws
	return ws, nil
}

func (s *rbacTestStore) SuspendWorkspace(_ context.Context, arg sqlc.SuspendWorkspaceParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[arg.ID]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	now := time.Now()
	ws.Status = "suspended"
	ws.SuspendedReason = arg.SuspendedReason
	ws.SuspendedAt = &now
	ws.SuspendedBy = arg.SuspendedBy
	s.workspaces[arg.ID] = ws
	return ws, nil
}

func (s *rbacTestStore) ActivateWorkspace(_ context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[id]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	ws.Status = "active"
	ws.SuspendedReason = sqlc.Workspace{}.SuspendedReason
	ws.SuspendedAt = nil
	ws.SuspendedBy = uuid.NullUUID{}
	s.workspaces[id] = ws
	return ws, nil
}

func (s *rbacTestStore) CreateWorkspaceMember(_ context.Context, arg sqlc.CreateWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	m := sqlc.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		Role:        arg.Role,
		Status:      "active",
		InvitedBy:   arg.InvitedBy,
		JoinedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.workspaceMembers[wsKey(arg.WorkspaceID, arg.UserID)] = m
	return m, nil
}

func (s *rbacTestStore) GetWorkspaceMember(_ context.Context, arg sqlc.GetWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.workspaceMembers[wsKey(arg.WorkspaceID, arg.UserID)]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	return m, nil
}

func (s *rbacTestStore) CountActiveWorkspaceOwners(_ context.Context, workspaceID uuid.UUID) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, m := range s.workspaceMembers {
		if m.WorkspaceID != workspaceID || m.Role != auth.RoleOwner || m.Status != "active" {
			continue
		}
		if u, ok := s.users[m.UserID]; ok && u.Status == "active" && u.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (s *rbacTestStore) CountWorkspaceMembers(_ context.Context, workspaceID uuid.UUID) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, m := range s.workspaceMembers {
		if m.WorkspaceID != workspaceID || m.Status == "removed" {
			continue
		}
		if u, ok := s.users[m.UserID]; ok && !u.IsSuperadmin && u.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (s *rbacTestStore) ListWorkspaceMembersWithUsers(_ context.Context, workspaceID uuid.UUID) ([]sqlc.ListWorkspaceMembersWithUsersRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.ListWorkspaceMembersWithUsersRow
	for _, m := range s.workspaceMembers {
		if m.WorkspaceID != workspaceID || m.Status == "removed" {
			continue
		}
		u, ok := s.users[m.UserID]
		if !ok || u.IsSuperadmin || u.DeletedAt != nil {
			continue
		}
		out = append(out, sqlc.ListWorkspaceMembersWithUsersRow{
			ID:                    u.ID,
			Email:                 u.Email,
			Name:                  u.Name,
			AvatarUrl:             u.AvatarUrl,
			IsSuperadmin:          u.IsSuperadmin,
			UserStatus:            u.Status,
			EmailVerifiedAt:       u.EmailVerifiedAt,
			CreatedAt:             u.CreatedAt,
			MemberRole:            m.Role,
			MemberStatus:          m.Status,
			MemberSuspendedReason: m.SuspendedReason,
			MemberSuspendedAt:     m.SuspendedAt,
			JoinedAt:              m.JoinedAt,
		})
	}
	return out, nil
}

func (s *rbacTestStore) UpdateWorkspaceMemberRole(_ context.Context, arg sqlc.UpdateWorkspaceMemberRoleParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	m.Role = arg.Role
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) SuspendWorkspaceMember(_ context.Context, arg sqlc.SuspendWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := time.Now()
	m.Status = "suspended"
	m.SuspendedReason = arg.SuspendedReason
	m.SuspendedAt = &now
	m.SuspendedBy = arg.SuspendedBy
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) ActivateWorkspaceMember(_ context.Context, arg sqlc.ActivateWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	m.Status = "active"
	m.SuspendedReason = sqlc.WorkspaceMember{}.SuspendedReason
	m.SuspendedAt = nil
	m.SuspendedBy = uuid.NullUUID{}
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) ListPlatformUsers(_ context.Context) ([]sqlc.ListPlatformUsersRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.ListPlatformUsersRow
	for _, u := range s.users {
		if u.DeletedAt != nil {
			continue
		}
		var cnt int64
		for _, m := range s.workspaceMembers {
			if m.UserID == u.ID && m.Status != "removed" {
				cnt++
			}
		}
		out = append(out, sqlc.ListPlatformUsersRow{
			ID:              u.ID,
			Email:           u.Email,
			Name:            u.Name,
			IsSuperadmin:    u.IsSuperadmin,
			Status:          u.Status,
			SuspendedReason: u.SuspendedReason,
			SuspendedAt:     u.SuspendedAt,
			CreatedAt:       u.CreatedAt,
			WorkspaceCount:  cnt,
		})
	}
	return out, nil
}

func (s *rbacTestStore) CreateWorkspaceMemberWithStatus(_ context.Context, arg sqlc.CreateWorkspaceMemberWithStatusParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	m := sqlc.WorkspaceMember{
		ID:          uuid.New(),
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		Role:        arg.Role,
		Status:      arg.Status,
		InvitedBy:   arg.InvitedBy,
		JoinedAt:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.workspaceMembers[wsKey(arg.WorkspaceID, arg.UserID)] = m
	return m, nil
}

func (s *rbacTestStore) AcceptWorkspaceMemberInvite(_ context.Context, arg sqlc.AcceptWorkspaceMemberInviteParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status != "pending" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := time.Now()
	m.Status = "active"
	m.JoinedAt = now
	m.UpdatedAt = now
	m.RemovedAt = nil
	m.RemovedBy = uuid.NullUUID{}
	m.RemovalReason = pgtype.Text{}
	m.SuspendedReason = pgtype.Text{}
	m.SuspendedAt = nil
	m.SuspendedBy = uuid.NullUUID{}
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) DeleteWorkspaceMember(_ context.Context, arg sqlc.DeleteWorkspaceMemberParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workspaceMembers, wsKey(arg.WorkspaceID, arg.UserID))
	return nil
}

func (s *rbacTestStore) ListPendingWorkspaceInvitesForUser(_ context.Context, userID uuid.UUID) ([]sqlc.ListPendingWorkspaceInvitesForUserRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.ListPendingWorkspaceInvitesForUserRow
	for _, m := range s.workspaceMembers {
		if m.UserID != userID || m.Status != "pending" {
			continue
		}
		ws, ok := s.workspaces[m.WorkspaceID]
		if !ok || ws.DeletedAt != nil || ws.Status != "active" {
			continue
		}
		var invName, invEmail pgtype.Text
		if m.InvitedBy.Valid {
			if inv, ok := s.users[m.InvitedBy.UUID]; ok {
				invName = pgtype.Text{String: inv.Name, Valid: true}
				invEmail = pgtype.Text{String: inv.Email, Valid: true}
			}
		}
		out = append(out, sqlc.ListPendingWorkspaceInvitesForUserRow{
			WorkspaceID:    ws.ID,
			WorkspaceName:  ws.Name,
			WorkspaceSlug:  ws.Slug,
			WorkspacePlan:  ws.Plan,
			MemberRole:     m.Role,
			InvitedAt:      m.CreatedAt,
			InvitedByName:  invName,
			InvitedByEmail: invEmail,
		})
	}
	return out, nil
}

func (s *rbacTestStore) GetWorkspaceByIDForUpdate(ctx context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	return s.GetWorkspaceByID(ctx, id)
}

func (s *rbacTestStore) GetUserByIDForUpdate(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	return s.GetUserByID(ctx, id)
}

func (s *rbacTestStore) RemoveWorkspaceMember(_ context.Context, arg sqlc.RemoveWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status == "removed" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := time.Now()
	m.Status = "removed"
	m.RemovedAt = &now
	m.RemovedBy = arg.RemovedBy
	m.RemovalReason = arg.RemovalReason
	m.SuspendedReason = pgtype.Text{}
	m.SuspendedAt = nil
	m.SuspendedBy = uuid.NullUUID{}
	m.UpdatedAt = now
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) ReinviteWorkspaceMember(_ context.Context, arg sqlc.ReinviteWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status != "removed" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := time.Now()
	m.Role = arg.Role
	m.Status = "pending"
	m.InvitedBy = arg.InvitedBy
	m.RemovedAt = nil
	m.RemovedBy = uuid.NullUUID{}
	m.RemovalReason = pgtype.Text{}
	m.SuspendedReason = pgtype.Text{}
	m.SuspendedAt = nil
	m.SuspendedBy = uuid.NullUUID{}
	m.UpdatedAt = now
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) ReactivateRemovedWorkspaceMember(_ context.Context, arg sqlc.ReactivateRemovedWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status != "removed" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := time.Now()
	m.Role = arg.Role
	m.Status = "active"
	m.InvitedBy = arg.InvitedBy
	m.JoinedAt = now
	m.RemovedAt = nil
	m.RemovedBy = uuid.NullUUID{}
	m.RemovalReason = pgtype.Text{}
	m.SuspendedReason = pgtype.Text{}
	m.SuspendedAt = nil
	m.SuspendedBy = uuid.NullUUID{}
	m.UpdatedAt = now
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *rbacTestStore) GetWorkspaceActiveMembershipStats(_ context.Context, workspaceID uuid.UUID) (sqlc.GetWorkspaceActiveMembershipStatsRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var stats sqlc.GetWorkspaceActiveMembershipStatsRow
	for _, m := range s.workspaceMembers {
		if m.WorkspaceID != workspaceID || m.Status != "active" {
			continue
		}
		u, ok := s.users[m.UserID]
		if !ok || u.IsSuperadmin || u.Status != "active" || u.DeletedAt != nil {
			continue
		}
		stats.ActiveMembers++
		if m.Role == auth.RoleOwner || m.Role == auth.RoleAdmin {
			stats.ActiveAdmins++
		}
		if m.Role == auth.RoleOwner {
			stats.ActiveOwners++
		}
	}
	return stats, nil
}

func (s *rbacTestStore) ListActiveWorkspacesForUserForUpdate(_ context.Context, userID uuid.UUID) ([]sqlc.ListActiveWorkspacesForUserForUpdateRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.ListActiveWorkspacesForUserForUpdateRow
	for _, m := range s.workspaceMembers {
		if m.UserID != userID || m.Status != "active" {
			continue
		}
		ws, ok := s.workspaces[m.WorkspaceID]
		if !ok || ws.DeletedAt != nil {
			continue
		}
		out = append(out, sqlc.ListActiveWorkspacesForUserForUpdateRow{
			WorkspaceID:   ws.ID,
			WorkspaceName: ws.Name,
			WorkspaceSlug: ws.Slug,
			MemberRole:    m.Role,
			MemberStatus:  m.Status,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkspaceID.String() < out[j].WorkspaceID.String()
	})
	return out, nil
}

func (s *rbacTestStore) ClearUserWorkspaceSession(_ context.Context, arg sqlc.ClearUserWorkspaceSessionParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if arg.ActiveWorkspaceID.Valid {
		for sessID, wsID := range s.sessionWS {
			if wsID == arg.ActiveWorkspaceID.UUID {
				delete(s.sessionWS, sessID)
			}
		}
	}
	return nil
}

func (s *rbacTestStore) ListWorkspaceAuditLogs(_ context.Context, arg sqlc.ListWorkspaceAuditLogsParams) ([]sqlc.AuthAuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.AuthAuditLog
	for i := len(s.workspaceAudits) - 1; i >= 0; i-- {
		a := s.workspaceAudits[i]
		if arg.WorkspaceID.Valid && a.WorkspaceID.Valid && a.WorkspaceID.UUID == arg.WorkspaceID.UUID {
			out = append(out, sqlc.AuthAuditLog{
				ID:          int64(i + 1),
				UserID:      a.UserID,
				WorkspaceID: a.WorkspaceID,
				ActorUserID: a.ActorUserID,
				Event:       a.Event,
				IpAddress:   a.IpAddress,
				UserAgent:   a.UserAgent,
				Metadata:    a.Metadata,
				CreatedAt:   time.Now(),
			})
			if arg.Limit > 0 && int32(len(out)) >= arg.Limit {
				break
			}
		}
	}
	return out, nil
}
