// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// For full license text and ownership/attribution terms, see LICENSE and NOTICE.

package auth

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
)

// ListUsers returns users in the first active workspace (or all users if no workspace exists yet).
func (s *Service) ListUsers(ctx context.Context) ([]AdminUserDTO, error) {
	ws, err := s.store.Queries().GetFirstActiveWorkspace(ctx)
	if err == nil {
		return s.ListUsersInWorkspace(ctx, ws.ID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	rows, err := s.store.Queries().ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdminUserDTO, 0, len(rows))
	for _, u := range rows {
		out = append(out, userToAdminDTO(u))
	}
	return out, nil
}

// ListUsersInWorkspace returns all non-deleted members of workspaceID with their workspace role and status.
func (s *Service) ListUsersInWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]AdminUserDTO, error) {
	rows, err := s.store.Queries().ListWorkspaceMembersWithUsers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]AdminUserDTO, 0, len(rows))
	for _, r := range rows {
		var avatar *string
		if r.AvatarUrl.Valid {
			v := normalizeAvatarRelativePath(r.AvatarUrl.String)
			avatar = &v
		}
		status := r.MemberStatus
		var suspendedReason *string
		suspendedAt := r.MemberSuspendedAt
		if r.MemberSuspendedReason.Valid {
			v := r.MemberSuspendedReason.String
			suspendedReason = &v
		}
		joinedAt := r.JoinedAt
		out = append(out, AdminUserDTO{
			ID:              r.ID,
			Name:            r.Name,
			Email:           r.Email,
			AvatarURL:       avatar,
			Role:            normalizeRole(r.MemberRole),
			IsSuperadmin:    r.IsSuperadmin,
			Status:          status,
			GlobalStatus:    r.UserStatus,
			SuspendedReason: suspendedReason,
			SuspendedAt:     suspendedAt,
			EmailVerifiedAt: r.EmailVerifiedAt,
			JoinedAt:        &joinedAt,
			CreatedAt:       r.CreatedAt,
		})
	}
	return out, nil
}

// AdminCreateUser invites a user into the actor's default/first active workspace.
func (s *Service) AdminCreateUser(
	ctx context.Context,
	actorID uuid.UUID,
	email string,
	name string,
	role string,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.AdminCreateUserInWorkspace(ctx, wsID, actorID, email, name, role, meta)
}

// AdminCreateUserInWorkspace invites a new or existing user into workspaceID with role admin/member/viewer.
// Superadmins bypass workspace plan and seat limits via BypassesPlanLimits.
func (s *Service) AdminCreateUserInWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	email string,
	name string,
	role string,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermUsersInvite)
	if err != nil {
		return nil, err
	}

	if err := s.checkWorkspaceInviteSeatLimitTx(ctx, s.store.Queries(), actor, ws); err != nil {
		return nil, err
	}

	role = strings.ToLower(strings.TrimSpace(role))
	if role == RoleOwner || role == "superadmin" {
		return nil, ErrForbidden
	}
	if !IsAssignableRole(role) {
		return nil, ErrInvalidRole
	}

	email = normalizeEmail(email)
	name = strings.TrimSpace(name)

	existing, lookupErr := s.store.Queries().GetUserByEmailIncludingDeleted(ctx, email)
	if lookupErr == nil {
		if existing.DeletedAt != nil {
			return nil, ErrConflictEmail
		}
		if existing.IsSuperadmin {
			return nil, ErrForbidden
		}

		// Existing users receive a pending workspace invitation so they must explicitly accept
		// before joining another tenant workspace. If previously removed, reactivate the existing
		// membership record into pending state without creating duplicate rows.
		var member sqlc.WorkspaceMember
		var reinvited bool
		txErr := s.store.WithTx(ctx, func(q sqlc.Querier) error {
			lockedWS, lockErr := q.GetWorkspaceByIDForUpdate(ctx, ws.ID)
			if lockErr != nil {
				if errors.Is(lockErr, pgx.ErrNoRows) {
					return ErrWorkspaceNotFound
				}
				return lockErr
			}
			if limitErr := s.checkWorkspaceInviteSeatLimitTx(ctx, q, actor, lockedWS); limitErr != nil {
				return limitErr
			}

			existingMember, memErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
				WorkspaceID: lockedWS.ID,
				UserID:      existing.ID,
			})
			if memErr == nil {
				if existingMember.Status != "removed" {
					return ErrConflictMember
				}
				m, reinvErr := q.ReinviteWorkspaceMember(ctx, sqlc.ReinviteWorkspaceMemberParams{
					WorkspaceID: lockedWS.ID,
					UserID:      existing.ID,
					Role:        role,
					InvitedBy:   uuid.NullUUID{UUID: actor.ID, Valid: true},
				})
				if reinvErr != nil {
					return reinvErr
				}
				member = m
				reinvited = true
			} else if errors.Is(memErr, pgx.ErrNoRows) {
				m, addErr := q.CreateWorkspaceMemberWithStatus(ctx, sqlc.CreateWorkspaceMemberWithStatusParams{
					WorkspaceID: lockedWS.ID,
					UserID:      existing.ID,
					Role:        role,
					Status:      "pending",
					InvitedBy:   uuid.NullUUID{UUID: actor.ID, Valid: true},
				})
				if addErr != nil {
					return addErr
				}
				member = m
			} else {
				return memErr
			}

			return recordMembershipAuditTx(
				ctx,
				q,
				existing.ID,
				uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
				actor.ID,
				"user_invited",
				"",
				member.Role,
				"",
				member.Status,
				"",
				meta,
				map[string]any{"reinvited": reinvited},
			)
		})
		if txErr != nil {
			return nil, txErr
		}

		dto := userToAdminDTO(existing)
		dto.Role = normalizeRole(member.Role)
		dto.Status = member.Status
		dto.GlobalStatus = existing.Status
		return &dto, nil
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return nil, lookupErr
	}

	runeCount := utf8.RuneCountInString(name)
	if runeCount < 1 || runeCount > 120 {
		return nil, ErrInvalidDisplayName
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return nil, err
	}

	var created sqlc.User
	var createdMember sqlc.WorkspaceMember
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		u, txErr := q.CreateUser(ctx, sqlc.CreateUserParams{
			Lower:           email,
			PasswordHash:    pgtype.Text{},
			Name:            name,
			EmailVerifiedAt: nil,
			Role:            role,
			IsSuperadmin:    false,
		})
		if txErr != nil {
			return txErr
		}
		m, txErr := q.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
			WorkspaceID: ws.ID,
			UserID:      u.ID,
			Role:        role,
			InvitedBy:   uuid.NullUUID{UUID: actor.ID, Valid: true},
		})
		if txErr != nil {
			return txErr
		}
		_, txErr = q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    u.ID,
			Type:      TokenTypeEmailVerify,
			TokenHash: tokenHash,
			ExpiresAt: s.now().Add(EmailVerificationTTL),
		})
		if txErr != nil {
			return txErr
		}
		created = u
		createdMember = m
		return nil
	})
	if err != nil {
		return nil, err
	}

	verifyURL := s.appURL + "/reset-password?token=" + rawToken
	if mailErr := s.mailer.SendVerificationEmail(ctx, created.Email, created.Name, verifyURL); mailErr != nil {
		return nil, mailErr
	}

	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: ws.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"user_invited",
		meta,
		map[string]string{
			"target_user_id": created.ID.String(),
			"role":           createdMember.Role,
		},
	)

	dto := userToAdminDTO(created)
	dto.Role = normalizeRole(createdMember.Role)
	dto.Status = createdMember.Status
	return &dto, nil
}

// AdminUpdateUserRole updates a member's role in the actor's default/first active workspace.
func (s *Service) AdminUpdateUserRole(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	newRole string,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.AdminUpdateUserRoleInWorkspace(ctx, wsID, actorID, targetID, newRole, meta)
}

// AdminUpdateUserRoleInWorkspace changes a target member's role within workspaceID to admin, member, or viewer.
func (s *Service) AdminUpdateUserRoleInWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	targetID uuid.UUID,
	newRole string,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermRolesManage)
	if err != nil {
		return nil, err
	}

	newRole = strings.ToLower(strings.TrimSpace(newRole))
	if newRole == RoleOwner || newRole == "superadmin" {
		return nil, ErrForbidden
	}
	if !IsAssignableRole(newRole) {
		return nil, ErrInvalidRole
	}
	if actor.ID == targetID {
		return nil, ErrForbidden
	}

	target, err := s.store.Queries().GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if target.IsSuperadmin {
		return nil, ErrForbidden
	}

	var oldRole string
	var updatedMember sqlc.WorkspaceMember
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := q.GetWorkspaceByIDForUpdate(ctx, ws.ID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return txErr
		}
		targetMember, txErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if targetMember.Status == "removed" {
			return ErrUserNotFound
		}

		oldRole = normalizeRole(targetMember.Role)
		if invErr := validateWorkspaceMembershipIntegrity(
			ctx,
			q,
			lockedWS,
			targetMember,
			target,
			newRole,
			targetMember.Status,
			target.Status,
			ErrForbidden,
		); invErr != nil {
			return invErr
		}

		m, txErr := q.UpdateWorkspaceMemberRole(ctx, sqlc.UpdateWorkspaceMemberRoleParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
			Role:        newRole,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		updatedMember = m

		return recordMembershipAuditTx(
			ctx,
			q,
			target.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			AuditEventMemberRoleChanged,
			oldRole,
			normalizeRole(m.Role),
			targetMember.Status,
			m.Status,
			"",
			meta,
			nil,
		)
	})
	if err != nil {
		var govErr *MembershipGovernanceError
		if errors.As(err, &govErr) {
			s.recordRejectedMembershipOperation(
				ctx,
				target.ID,
				uuid.NullUUID{UUID: ws.ID, Valid: true},
				actor.ID,
				AuditEventMembershipOpRejected,
				"change_role",
				err,
				meta,
			)
		}
		return nil, err
	}

	dto := userToAdminDTO(target)
	dto.Role = normalizeRole(updatedMember.Role)
	dto.Status = updatedMember.Status
	dto.GlobalStatus = target.Status
	joinedAt := updatedMember.JoinedAt
	dto.JoinedAt = &joinedAt
	return &dto, nil
}

// AdminSuspendUser suspends a target user in the actor's default/first active workspace.
func (s *Service) AdminSuspendUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.AdminSuspendUserInWorkspace(ctx, wsID, actorID, targetID, reason, meta)
}

// AdminSuspendUserInWorkspace suspends a workspace member within workspaceID only (BR-04),
// enforcing BR-01 and BR-02 inside a locked transaction and preserving the global user account.
func (s *Service) AdminSuspendUserInWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermUsersSuspend)
	if err != nil {
		return nil, err
	}

	trimmedReason := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmedReason) < 3 {
		return nil, ErrInvalidSuspendReason
	}
	if actor.ID == targetID {
		return nil, ErrForbidden
	}

	target, err := s.store.Queries().GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if target.IsSuperadmin {
		return nil, ErrForbidden
	}

	actorRole := RoleOwner
	if !actor.IsSuperadmin {
		actorMember, amErr := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: ws.ID,
			UserID:      actor.ID,
		})
		if amErr != nil {
			return nil, ErrForbidden
		}
		actorRole = normalizeRole(actorMember.Role)
	}

	var updatedMember sqlc.WorkspaceMember
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := q.GetWorkspaceByIDForUpdate(ctx, ws.ID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return txErr
		}
		targetMember, txErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if targetMember.Status == "removed" {
			return ErrUserNotFound
		}
		if targetMember.Status == "pending" {
			return ErrForbidden
		}

		targetRole := normalizeRole(targetMember.Role)
		if !actor.IsSuperadmin {
			if actorRole == RoleAdmin && targetRole != RoleMember && targetRole != RoleViewer {
				return ErrForbidden
			}
			if actorRole != RoleOwner && actorRole != RoleAdmin {
				return ErrForbidden
			}
		}

		if invErr := validateWorkspaceMembershipIntegrity(
			ctx,
			q,
			lockedWS,
			targetMember,
			target,
			targetRole,
			"suspended",
			target.Status,
			ErrForbidden,
		); invErr != nil {
			return invErr
		}

		m, txErr := q.SuspendWorkspaceMember(ctx, sqlc.SuspendWorkspaceMemberParams{
			WorkspaceID:     lockedWS.ID,
			UserID:          target.ID,
			SuspendedReason: pgText(trimmedReason),
			SuspendedBy:     uuid.NullUUID{UUID: actor.ID, Valid: true},
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		updatedMember = m

		// Revoke active workspace pinning on sessions for this workspace only (preserve global sessions & other workspaces).
		if clearErr := q.ClearUserWorkspaceSession(ctx, sqlc.ClearUserWorkspaceSessionParams{
			UserID:            target.ID,
			ActiveWorkspaceID: uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
		}); clearErr != nil {
			return clearErr
		}

		return recordMembershipAuditTx(
			ctx,
			q,
			target.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			AuditEventMemberSuspended,
			targetRole,
			targetRole,
			targetMember.Status,
			m.Status,
			trimmedReason,
			meta,
			nil,
		)
	})
	if err != nil {
		var govErr *MembershipGovernanceError
		if errors.As(err, &govErr) {
			s.recordRejectedMembershipOperation(
				ctx,
				target.ID,
				uuid.NullUUID{UUID: ws.ID, Valid: true},
				actor.ID,
				AuditEventMembershipOpRejected,
				"suspend_member",
				err,
				meta,
			)
		}
		return nil, err
	}

	dto := userToAdminDTO(target)
	dto.Role = normalizeRole(updatedMember.Role)
	dto.Status = updatedMember.Status
	dto.GlobalStatus = target.Status
	if updatedMember.SuspendedReason.Valid {
		v := updatedMember.SuspendedReason.String
		dto.SuspendedReason = &v
	}
	dto.SuspendedAt = updatedMember.SuspendedAt
	joinedAt := updatedMember.JoinedAt
	dto.JoinedAt = &joinedAt
	return &dto, nil
}

// AdminActivateUser clears a suspended member in the actor's default/first active workspace.
func (s *Service) AdminActivateUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.AdminActivateUserInWorkspace(ctx, wsID, actorID, targetID, meta)
}

// AdminActivateUserInWorkspace reactivates a suspended member within workspaceID only.
func (s *Service) AdminActivateUserInWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	targetID uuid.UUID,
	meta RequestMeta,
) (*AdminUserDTO, error) {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermUsersSuspend)
	if err != nil {
		return nil, err
	}

	target, err := s.store.Queries().GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	actorRole := RoleOwner
	if !actor.IsSuperadmin {
		actorMember, amErr := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: ws.ID,
			UserID:      actor.ID,
		})
		if amErr != nil {
			return nil, ErrForbidden
		}
		actorRole = normalizeRole(actorMember.Role)
	}

	var updatedMember sqlc.WorkspaceMember
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := q.GetWorkspaceByIDForUpdate(ctx, ws.ID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return txErr
		}
		targetMember, txErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if targetMember.Status == "removed" {
			return ErrUserNotFound
		}
		if targetMember.Status == "pending" {
			return ErrForbidden
		}

		targetRole := normalizeRole(targetMember.Role)
		if !actor.IsSuperadmin {
			if actorRole == RoleAdmin && targetRole != RoleMember && targetRole != RoleViewer {
				return ErrForbidden
			}
			if actorRole != RoleOwner && actorRole != RoleAdmin {
				return ErrForbidden
			}
		}

		m, txErr := q.ActivateWorkspaceMember(ctx, sqlc.ActivateWorkspaceMemberParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		updatedMember = m

		return recordMembershipAuditTx(
			ctx,
			q,
			target.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			AuditEventMemberReactivated,
			targetRole,
			targetRole,
			targetMember.Status,
			m.Status,
			"",
			meta,
			nil,
		)
	})
	if err != nil {
		return nil, err
	}

	dto := userToAdminDTO(target)
	dto.Role = normalizeRole(updatedMember.Role)
	dto.Status = updatedMember.Status
	dto.GlobalStatus = target.Status
	dto.SuspendedReason = nil
	dto.SuspendedAt = nil
	joinedAt := updatedMember.JoinedAt
	dto.JoinedAt = &joinedAt
	return &dto, nil
}

// AdminUpdateUserCheck enforces role-scoped target permissions for users:update.
func (s *Service) AdminUpdateUserCheck(ctx context.Context, actorID, targetID uuid.UUID) error {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return err
	}
	return s.AdminUpdateUserCheckInWorkspace(ctx, wsID, actorID, targetID)
}

// AdminUpdateUserCheckInWorkspace enforces role-scoped target permissions for users:update within workspaceID.
func (s *Service) AdminUpdateUserCheckInWorkspace(ctx context.Context, workspaceID, actorID, targetID uuid.UUID) error {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermUsersUpdate)
	if err != nil {
		return err
	}
	target, err := s.store.Queries().GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	targetMember, err := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      target.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if targetMember.Status == "removed" {
		return ErrUserNotFound
	}
	if targetMember.Status == "pending" {
		return ErrForbidden
	}

	targetRole := normalizeRole(targetMember.Role)
	if !actor.IsSuperadmin {
		if target.IsSuperadmin {
			return ErrForbidden
		}
		actorMember, _ := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: ws.ID,
			UserID:      actor.ID,
		})
		if normalizeRole(actorMember.Role) == RoleAdmin && targetRole != RoleMember && targetRole != RoleViewer {
			return ErrForbidden
		}
	}
	return nil
}

// AdminRemoveUser removes targetID from the actor's active workspace.
func (s *Service) AdminRemoveUser(ctx context.Context, actorID, targetID uuid.UUID, meta RequestMeta) error {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return err
	}
	return s.AdminRemoveUserInWorkspace(ctx, wsID, actorID, targetID, meta)
}

// AdminRemoveUserInWorkspace soft-removes targetID from workspaceID (including canceling pending invites),
// enforcing BR-01, BR-02, BR-03, and BR-06 inside a locked transaction.
func (s *Service) AdminRemoveUserInWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	targetID uuid.UUID,
	meta RequestMeta,
) error {
	return s.AdminRemoveUserInWorkspaceWithReason(ctx, workspaceID, actorID, targetID, "", meta)
}

// AdminRemoveUserInWorkspaceWithReason soft-removes targetID from workspaceID with an optional audit reason.
func (s *Service) AdminRemoveUserInWorkspaceWithReason(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
	meta RequestMeta,
) error {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermUsersRemove)
	if err != nil {
		return err
	}
	if actor.ID == targetID {
		return ErrForbidden
	}
	target, err := s.store.Queries().GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if target.IsSuperadmin {
		return ErrForbidden
	}

	trimmedReason := strings.TrimSpace(reason)

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := q.GetWorkspaceByIDForUpdate(ctx, ws.ID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return txErr
		}
		targetMember, txErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if targetMember.Status == "removed" {
			return ErrUserNotFound
		}

		targetRole := normalizeRole(targetMember.Role)
		if !canManageTargetRole(actor.IsSuperadmin, normalizeRole(actor.Role), targetRole) {
			return ErrForbidden
		}
		if targetMember.Status != "pending" {
			if invErr := validateWorkspaceMembershipIntegrity(
				ctx,
				q,
				lockedWS,
				targetMember,
				target,
				targetRole,
				"removed",
				target.Status,
				ErrForbidden,
			); invErr != nil {
				return invErr
			}
		}

		removed, txErr := q.RemoveWorkspaceMember(ctx, sqlc.RemoveWorkspaceMemberParams{
			WorkspaceID:   lockedWS.ID,
			UserID:        target.ID,
			RemovedBy:     uuid.NullUUID{UUID: actor.ID, Valid: true},
			RemovalReason: pgText(trimmedReason),
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}

		if clearErr := q.ClearUserWorkspaceSession(ctx, sqlc.ClearUserWorkspaceSessionParams{
			UserID:            target.ID,
			ActiveWorkspaceID: uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
		}); clearErr != nil {
			return clearErr
		}

		return recordMembershipAuditTx(
			ctx,
			q,
			target.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			AuditEventMemberRemoved,
			targetRole,
			"",
			targetMember.Status,
			removed.Status,
			trimmedReason,
			meta,
			map[string]any{"canceled_pending_invite": targetMember.Status == "pending"},
		)
	})
	if err != nil {
		var govErr *MembershipGovernanceError
		if errors.As(err, &govErr) {
			s.recordRejectedMembershipOperation(
				ctx,
				target.ID,
				uuid.NullUUID{UUID: ws.ID, Valid: true},
				actor.ID,
				AuditEventMembershipOpRejected,
				"remove_member",
				err,
				meta,
			)
		}
		return err
	}
	return nil
}

// AdminRemoveUserCheck enforces target guards for users:remove.
func (s *Service) AdminRemoveUserCheck(ctx context.Context, actorID, targetID uuid.UUID) error {
	wsID, err := s.resolveActorWorkspaceID(ctx, actorID)
	if err != nil {
		return err
	}
	return s.AdminRemoveUserCheckInWorkspace(ctx, wsID, actorID, targetID)
}

// AdminRemoveUserCheckInWorkspace enforces target guards for users:remove within workspaceID.
func (s *Service) AdminRemoveUserCheckInWorkspace(ctx context.Context, workspaceID, actorID, targetID uuid.UUID) error {
	actor, ws, err := s.requireWorkspaceActorWithPerm(ctx, workspaceID, actorID, PermUsersRemove)
	if err != nil {
		return err
	}
	if actor.ID == targetID {
		return ErrForbidden
	}
	target, err := s.store.Queries().GetUserByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if target.IsSuperadmin {
		return ErrForbidden
	}
	targetMember, err := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      target.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if targetMember.Status == "removed" {
		return ErrUserNotFound
	}
	if !canManageTargetRole(actor.IsSuperadmin, normalizeRole(actor.Role), normalizeRole(targetMember.Role)) {
		return ErrForbidden
	}
	if targetMember.Status != "pending" {
		return validateWorkspaceMembershipIntegrity(
			ctx,
			s.store.Queries(),
			ws,
			targetMember,
			target,
			targetMember.Role,
			"removed",
			target.Status,
			ErrForbidden,
		)
	}
	return nil
}

// canManageTargetRole enforces that workspace admins may only manage member/viewer targets,
// while owners and platform superadmins may manage any non-superadmin target role.
func canManageTargetRole(isSuperadmin bool, actorRole, targetRole string) bool {
	if isSuperadmin {
		return true
	}
	if actorRole == RoleOwner {
		return true
	}
	if actorRole == RoleAdmin {
		return targetRole == RoleMember || targetRole == RoleViewer
	}
	return false
}

func (s *Service) checkWorkspaceInviteSeatLimitTx(ctx context.Context, q sqlc.Querier, actor sqlc.User, ws sqlc.Workspace) error {
	if BypassesPlanLimits(actor) {
		return nil
	}
	limits := EffectiveWorkspaceLimits(ws, s.now())
	hasSeatOverride := ws.MaxUsersOverride.Valid && (ws.OverrideExpiresAt == nil || ws.OverrideExpiresAt.After(s.now()))
	effectivePlan := ws.Plan
	if effectivePlan == "" {
		effectivePlan = s.Plan()
	}
	if effectivePlan == FreePlan && !hasSeatOverride {
		return ErrInviteRequiresPaidPlan
	}
	if limits.MaxUsers > 0 {
		memberCount, countErr := q.CountWorkspaceMembers(ctx, ws.ID)
		if countErr != nil {
			return countErr
		}
		if memberCount >= int64(limits.MaxUsers) {
			if effectivePlan == FreePlan {
				return ErrInviteRequiresPaidPlan
			}
			return ErrFreePlanLimitReached
		}
	}
	return nil
}

// CreateProduct checks FreeMaxProducts before recording a created product.
func (s *Service) CreateProduct(ctx context.Context) error {
	return s.CreateProductInWorkspace(ctx, uuid.Nil, uuid.Nil)
}

// CreateProductInWorkspace checks workspace product limits (skipped when actor is superadmin).
func (s *Service) CreateProductInWorkspace(ctx context.Context, actorID, workspaceID uuid.UUID) error {
	var bypass bool
	limit := FreeMaxProducts
	if actorID != uuid.Nil {
		actor, err := s.requireActiveUser(ctx, actorID)
		if err != nil {
			return err
		}
		bypass = BypassesPlanLimits(actor)
	}
	if !bypass && workspaceID != uuid.Nil {
		if ws, err := s.store.Queries().GetWorkspaceByID(ctx, workspaceID); err == nil {
			limit = EffectiveWorkspaceLimits(ws, s.now()).MaxProducts
		}
	}

	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()
	if !bypass && limit > 0 && s.productCounts[workspaceID] >= limit {
		return ErrFreePlanLimitReached
	}
	s.productCounts[workspaceID]++
	return nil
}

// DeleteProduct decrements the tracked product count if positive.
func (s *Service) DeleteProduct(_ context.Context) error {
	return s.DeleteProductInWorkspace(context.Background(), uuid.Nil)
}

// DeleteProductInWorkspace decrements the tracked product count for workspaceID if positive.
func (s *Service) DeleteProductInWorkspace(_ context.Context, workspaceID uuid.UUID) error {
	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()
	if s.productCounts[workspaceID] > 0 {
		s.productCounts[workspaceID]--
	}
	return nil
}

// CreateLicense checks FreeMaxLicenses before recording a created license.
func (s *Service) CreateLicense(ctx context.Context) error {
	return s.CreateLicenseInWorkspace(ctx, uuid.Nil, uuid.Nil)
}

// CreateLicenseInWorkspace checks workspace license limits (skipped when actor is superadmin).
func (s *Service) CreateLicenseInWorkspace(ctx context.Context, actorID, workspaceID uuid.UUID) error {
	var bypass bool
	limit := FreeMaxLicenses
	if actorID != uuid.Nil {
		actor, err := s.requireActiveUser(ctx, actorID)
		if err != nil {
			return err
		}
		bypass = BypassesPlanLimits(actor)
	}
	if !bypass && workspaceID != uuid.Nil {
		if ws, err := s.store.Queries().GetWorkspaceByID(ctx, workspaceID); err == nil {
			limit = EffectiveWorkspaceLimits(ws, s.now()).MaxLicenses
		}
	}

	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()
	if !bypass && limit > 0 && s.licenseCounts[workspaceID] >= limit {
		return ErrFreePlanLimitReached
	}
	s.licenseCounts[workspaceID]++
	return nil
}

// DeleteLicense decrements the tracked license count if positive.
func (s *Service) DeleteLicense(_ context.Context) error {
	return s.DeleteLicenseInWorkspace(context.Background(), uuid.Nil)
}

// DeleteLicenseInWorkspace decrements the tracked license count for workspaceID if positive.
func (s *Service) DeleteLicenseInWorkspace(_ context.Context, workspaceID uuid.UUID) error {
	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()
	if s.licenseCounts[workspaceID] > 0 {
		s.licenseCounts[workspaceID]--
	}
	return nil
}

func (s *Service) resolveActorWorkspaceID(ctx context.Context, actorID uuid.UUID) (uuid.UUID, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return uuid.Nil, err
	}
	dto := userToDTO(actor)
	wsCtx, err := s.ResolveWorkspaceContext(ctx, &dto, nil)
	if err != nil {
		return uuid.Nil, err
	}
	return wsCtx.Workspace.ID, nil
}

func (s *Service) requireWorkspaceActorWithPerm(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	perm string,
) (sqlc.User, sqlc.Workspace, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return sqlc.User{}, sqlc.Workspace{}, err
	}
	dto := userToDTO(actor)
	var reqWS *uuid.UUID
	if workspaceID != uuid.Nil {
		reqWS = &workspaceID
	}
	wsCtx, err := s.ResolveWorkspaceContext(ctx, &dto, reqWS)
	if err != nil {
		return sqlc.User{}, sqlc.Workspace{}, err
	}
	if !HasPermission(wsCtx.Role, actor.IsSuperadmin, perm) {
		return sqlc.User{}, sqlc.Workspace{}, ErrForbidden
	}
	return actor, wsCtx.Workspace, nil
}
