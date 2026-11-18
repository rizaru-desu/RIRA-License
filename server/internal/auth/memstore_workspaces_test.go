package auth

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/db/sqlc"
)

func wsMemberKey(wsID, userID uuid.UUID) string {
	return wsID.String() + ":" + userID.String()
}

func (s *memStore) SetSessionActiveWorkspace(_ context.Context, arg sqlc.SetSessionActiveWorkspaceParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[arg.ID]
	if !ok {
		return nil
	}
	sess.ActiveWorkspaceID = arg.ActiveWorkspaceID
	s.sessions[arg.ID] = sess
	return nil
}

func (s *memStore) InsertWorkspaceAuditLog(_ context.Context, arg sqlc.InsertWorkspaceAuditLogParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditSeq++
	s.auditLogs = append(s.auditLogs, sqlc.AuthAuditLog{
		ID:          s.auditSeq,
		UserID:      arg.UserID,
		WorkspaceID: arg.WorkspaceID,
		ActorUserID: arg.ActorUserID,
		Event:       arg.Event,
		IpAddress:   arg.IpAddress,
		UserAgent:   arg.UserAgent,
		Metadata:    arg.Metadata,
		CreatedAt:   s.now(),
	})
	return nil
}

func (s *memStore) CreateWorkspace(_ context.Context, arg sqlc.CreateWorkspaceParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
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
	s.wsSeq++
	s.wsOrder[ws.ID] = s.wsSeq
	return ws, nil
}

func (s *memStore) GetWorkspaceByID(_ context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[id]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	return ws, nil
}

func (s *memStore) GetWorkspaceBySlug(_ context.Context, slug string) (sqlc.Workspace, error) {
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

func (s *memStore) GetFirstActiveWorkspace(_ context.Context) (sqlc.Workspace, error) {
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
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return s.wsOrder[list[i].ID] < s.wsOrder[list[j].ID]
	})
	return list[0], nil
}

func (s *memStore) ListWorkspacesForUser(_ context.Context, userID uuid.UUID) ([]sqlc.ListWorkspacesForUserRow, error) {
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

func (s *memStore) ListAllWorkspaces(_ context.Context) ([]sqlc.ListAllWorkspacesRow, error) {
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *memStore) UpdateWorkspaceDetails(_ context.Context, arg sqlc.UpdateWorkspaceDetailsParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[arg.ID]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	for id, other := range s.workspaces {
		if id != arg.ID && other.DeletedAt == nil && strings.EqualFold(other.Slug, arg.Slug) {
			return sqlc.Workspace{}, errors.New("duplicate key value violates unique constraint (23505)")
		}
	}
	ws.Name = arg.Name
	ws.Slug = strings.ToLower(arg.Slug)
	ws.UpdatedAt = s.now()
	s.workspaces[arg.ID] = ws
	return ws, nil
}

func (s *memStore) SoftDeleteWorkspace(_ context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[id]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	now := s.now()
	ws.DeletedAt = &now
	ws.UpdatedAt = now
	s.workspaces[id] = ws
	return ws, nil
}

func (s *memStore) ClearWorkspaceSessions(_ context.Context, workspaceID uuid.NullUUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !workspaceID.Valid {
		return nil
	}
	for id, sess := range s.sessions {
		if sess.ActiveWorkspaceID.Valid && sess.ActiveWorkspaceID.UUID == workspaceID.UUID {
			sess.ActiveWorkspaceID = uuid.NullUUID{}
			s.sessions[id] = sess
		}
	}
	return nil
}

func (s *memStore) UpdateWorkspacePlanAndOverrides(_ context.Context, arg sqlc.UpdateWorkspacePlanAndOverridesParams) (sqlc.Workspace, error) {
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
	ws.UpdatedAt = s.now()
	s.workspaces[arg.ID] = ws
	return ws, nil
}

func (s *memStore) SuspendWorkspace(_ context.Context, arg sqlc.SuspendWorkspaceParams) (sqlc.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[arg.ID]
	if !ok || ws.DeletedAt != nil {
		return sqlc.Workspace{}, pgx.ErrNoRows
	}
	now := s.now()
	ws.Status = "suspended"
	ws.SuspendedReason = arg.SuspendedReason
	ws.SuspendedAt = &now
	ws.SuspendedBy = arg.SuspendedBy
	s.workspaces[arg.ID] = ws
	return ws, nil
}

func (s *memStore) ActivateWorkspace(_ context.Context, id uuid.UUID) (sqlc.Workspace, error) {
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

func (s *memStore) CreateWorkspaceMember(_ context.Context, arg sqlc.CreateWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
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
	s.workspaceMembers[wsMemberKey(arg.WorkspaceID, arg.UserID)] = m
	return m, nil
}

func (s *memStore) GetWorkspaceMember(_ context.Context, arg sqlc.GetWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.workspaceMembers[wsMemberKey(arg.WorkspaceID, arg.UserID)]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	return m, nil
}

func (s *memStore) CountActiveWorkspaceOwners(_ context.Context, workspaceID uuid.UUID) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, m := range s.workspaceMembers {
		if m.WorkspaceID != workspaceID || m.Role != RoleOwner || m.Status != "active" {
			continue
		}
		if u, ok := s.users[m.UserID]; ok && u.Status == "active" && u.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (s *memStore) CountWorkspaceMembers(_ context.Context, workspaceID uuid.UUID) (int64, error) {
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

func (s *memStore) ListWorkspaceMembersWithUsers(_ context.Context, workspaceID uuid.UUID) ([]sqlc.ListWorkspaceMembersWithUsersRow, error) {
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].JoinedAt.Before(out[j].JoinedAt)
	})
	return out, nil
}

func (s *memStore) UpdateWorkspaceMemberRole(_ context.Context, arg sqlc.UpdateWorkspaceMemberRoleParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	m.Role = arg.Role
	m.UpdatedAt = s.now()
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *memStore) SuspendWorkspaceMember(_ context.Context, arg sqlc.SuspendWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := s.now()
	m.Status = "suspended"
	m.SuspendedReason = arg.SuspendedReason
	m.SuspendedAt = &now
	m.SuspendedBy = arg.SuspendedBy
	s.workspaceMembers[key] = m
	return m, nil
}

func (s *memStore) ActivateWorkspaceMember(_ context.Context, arg sqlc.ActivateWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
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

func (s *memStore) ListPlatformUsers(_ context.Context) ([]sqlc.ListPlatformUsersRow, error) {
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
			ID:                  u.ID,
			Email:               u.Email,
			PasswordHash:        u.PasswordHash,
			Name:                u.Name,
			AvatarUrl:           u.AvatarUrl,
			EmailVerifiedAt:     u.EmailVerifiedAt,
			Role:                u.Role,
			IsSuperadmin:        u.IsSuperadmin,
			Status:              u.Status,
			SuspendedReason:     u.SuspendedReason,
			SuspendedAt:         u.SuspendedAt,
			DeletedAt:           u.DeletedAt,
			TwoFactorEnabled:    u.TwoFactorEnabled,
			TwoFactorMethod:     u.TwoFactorMethod,
			TotpSecretEncrypted: u.TotpSecretEncrypted,
			TotpVerifiedAt:      u.TotpVerifiedAt,
			FailedLoginAttempts: u.FailedLoginAttempts,
			LockedUntil:         u.LockedUntil,
			LastLoginAt:         u.LastLoginAt,
			CreatedAt:           u.CreatedAt,
			UpdatedAt:           u.UpdatedAt,
		WorkspaceCount:      cnt,
		})
	}
	return out, nil
}

func (s *memStore) CreateWorkspaceMemberWithStatus(_ context.Context, arg sqlc.CreateWorkspaceMemberWithStatusParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
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
	s.workspaceMembers[wsMemberKey(arg.WorkspaceID, arg.UserID)] = m
	return m, nil
}

func (s *memStore) AcceptWorkspaceMemberInvite(_ context.Context, arg sqlc.AcceptWorkspaceMemberInviteParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status != "pending" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := s.now()
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

func (s *memStore) DeleteWorkspaceMember(_ context.Context, arg sqlc.DeleteWorkspaceMemberParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workspaceMembers, wsMemberKey(arg.WorkspaceID, arg.UserID))
	return nil
}

func (s *memStore) ListPendingWorkspaceInvitesForUser(_ context.Context, userID uuid.UUID) ([]sqlc.ListPendingWorkspaceInvitesForUserRow, error) {
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

func (s *memStore) GetWorkspaceByIDForUpdate(ctx context.Context, id uuid.UUID) (sqlc.Workspace, error) {
	return s.GetWorkspaceByID(ctx, id)
}

func (s *memStore) GetUserByIDForUpdate(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	return s.GetUserByID(ctx, id)
}

func (s *memStore) RemoveWorkspaceMember(_ context.Context, arg sqlc.RemoveWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status == "removed" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := s.now()
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

func (s *memStore) ReinviteWorkspaceMember(_ context.Context, arg sqlc.ReinviteWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status != "removed" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := s.now()
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

func (s *memStore) ReactivateRemovedWorkspaceMember(_ context.Context, arg sqlc.ReactivateRemovedWorkspaceMemberParams) (sqlc.WorkspaceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := wsMemberKey(arg.WorkspaceID, arg.UserID)
	m, ok := s.workspaceMembers[key]
	if !ok || m.Status != "removed" {
		return sqlc.WorkspaceMember{}, pgx.ErrNoRows
	}
	now := s.now()
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

func (s *memStore) GetWorkspaceActiveMembershipStats(_ context.Context, workspaceID uuid.UUID) (sqlc.GetWorkspaceActiveMembershipStatsRow, error) {
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
		r := normalizeRole(m.Role)
		if r == RoleOwner || r == RoleAdmin {
			stats.ActiveAdmins++
		}
		if r == RoleOwner {
			stats.ActiveOwners++
		}
	}
	return stats, nil
}

func (s *memStore) ListActiveWorkspacesForUserForUpdate(_ context.Context, userID uuid.UUID) ([]sqlc.ListActiveWorkspacesForUserForUpdateRow, error) {
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

func (s *memStore) ClearUserWorkspaceSession(_ context.Context, arg sqlc.ClearUserWorkspaceSessionParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.UserID == arg.UserID && arg.ActiveWorkspaceID.Valid && sess.ActiveWorkspaceID.Valid && sess.ActiveWorkspaceID.UUID == arg.ActiveWorkspaceID.UUID {
			sess.ActiveWorkspaceID = uuid.NullUUID{}
			s.sessions[id] = sess
		}
	}
	return nil
}

func (s *memStore) ListWorkspaceAuditLogs(_ context.Context, arg sqlc.ListWorkspaceAuditLogsParams) ([]sqlc.AuthAuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.AuthAuditLog
	for i := len(s.auditLogs) - 1; i >= 0; i-- {
		entry := s.auditLogs[i]
		if arg.WorkspaceID.Valid && entry.WorkspaceID.Valid && entry.WorkspaceID.UUID == arg.WorkspaceID.UUID {
			out = append(out, entry)
			if arg.Limit > 0 && int32(len(out)) >= arg.Limit {
				break
			}
		}
	}
	return out, nil
}
