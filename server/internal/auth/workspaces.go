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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/db/sqlc"
)

var (
	workspaceSlugPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)
	nonSlugAlnumHyphenRegex = regexp.MustCompile(`[^a-z0-9-]+`)
	multiHyphenRegex        = regexp.MustCompile(`-+`)
)

func ensureInitialWorkspaceForOwner(ctx context.Context, q sqlc.Querier, ownerID uuid.UUID, plan string) error {
	if plan == "" || !IsValidPlan(plan) {
		plan = FreePlan
	}
	ws, err := q.GetFirstActiveWorkspace(ctx)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		ws, err = q.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{
			Name:      "Default Workspace",
			Lower:     "default",
			Plan:      plan,
			CreatedBy: uuid.NullUUID{UUID: ownerID, Valid: true},
		})
		if err != nil {
			return err
		}
	}
	u, uErr := q.GetUserByID(ctx, ownerID)
	if uErr == nil && u.IsSuperadmin {
		// Platform superadmins stay outside tenant workspace_members.
		return nil
	}
	if _, memberErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      ownerID,
	}); errors.Is(memberErr, pgx.ErrNoRows) {
		_, err = q.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
			WorkspaceID: ws.ID,
			UserID:      ownerID,
			Role:        RoleOwner,
			InvitedBy:   uuid.NullUUID{},
		})
		return err
	} else if memberErr != nil {
		return memberErr
	}
	return nil
}

func createPersonalWorkspaceForOwner(ctx context.Context, q sqlc.Querier, ownerID uuid.UUID, ownerName string, plan string) error {
	if plan == "" || !IsValidPlan(plan) {
		plan = FreePlan
	}
	trimmed := strings.TrimSpace(ownerName)
	wsName := "Personal Workspace"
	if trimmed != "" {
		wsName = trimmed + "'s Workspace"
		if utf8.RuneCountInString(wsName) > 120 {
			runes := []rune(trimmed)
			if len(runes) > 108 {
				runes = runes[:108]
			}
			wsName = string(runes) + "'s Workspace"
		}
	}

	baseSlug := slugifyWorkspaceName(wsName)
	slug := baseSlug
	if _, lookupErr := q.GetWorkspaceBySlug(ctx, slug); lookupErr == nil {
		suffix := "-" + strings.ReplaceAll(ownerID.String(), "-", "")[:8]
		maxBase := 64 - len(suffix)
		if len(baseSlug) > maxBase {
			baseSlug = strings.TrimRight(baseSlug[:maxBase], "-")
		}
		slug = baseSlug + suffix
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}

	ws, err := q.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{
		Name:      wsName,
		Lower:     slug,
		Plan:      plan,
		CreatedBy: uuid.NullUUID{UUID: ownerID, Valid: true},
	})
	if err != nil {
		return err
	}
	_, err = q.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      ownerID,
		Role:        RoleOwner,
		InvitedBy:   uuid.NullUUID{},
	})
	return err
}

// WorkspaceToDTO converts a sqlc.Workspace to a WorkspaceDTO with effective limits computed at now.
func WorkspaceToDTO(ws sqlc.Workspace, role string, memberCount int64, now time.Time) WorkspaceDTO {
	limits := EffectiveWorkspaceLimits(ws, now)
	var suspendedReason *string
	if ws.SuspendedReason.Valid {
		v := ws.SuspendedReason.String
		suspendedReason = &v
	}
	var maxUsersOverride, maxProductsOverride, maxLicensesOverride *int32
	if ws.MaxUsersOverride.Valid {
		v := ws.MaxUsersOverride.Int32
		maxUsersOverride = &v
	}
	if ws.MaxProductsOverride.Valid {
		v := ws.MaxProductsOverride.Int32
		maxProductsOverride = &v
	}
	if ws.MaxLicensesOverride.Valid {
		v := ws.MaxLicensesOverride.Int32
		maxLicensesOverride = &v
	}
	var overrideNote *string
	if ws.OverrideNote.Valid {
		v := ws.OverrideNote.String
		overrideNote = &v
	}
	return WorkspaceDTO{
		ID:                  ws.ID,
		Name:                ws.Name,
		Slug:                ws.Slug,
		Plan:                ws.Plan,
		Status:              ws.Status,
		Role:                role,
		SuspendedReason:     suspendedReason,
		SuspendedAt:         ws.SuspendedAt,
		MaxUsers:            limits.MaxUsers,
		MaxProducts:         limits.MaxProducts,
		MaxLicenses:         limits.MaxLicenses,
		MaxUsersOverride:    maxUsersOverride,
		MaxProductsOverride: maxProductsOverride,
		MaxLicensesOverride: maxLicensesOverride,
		OverrideExpiresAt:   ws.OverrideExpiresAt,
		OverrideNote:        overrideNote,
		MemberCount:         memberCount,
		CreatedAt:           ws.CreatedAt,
	}
}

func slugifyWorkspaceName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonSlugAlnumHyphenRegex.ReplaceAllString(s, "-")
	s = multiHyphenRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ResolveWorkspaceContext resolves the active workspace, effective role, permissions, and limits for user.
func (s *Service) ResolveWorkspaceContext(
	ctx context.Context,
	user *UserDTO,
	requestedWorkspaceID *uuid.UUID,
) (*WorkspaceContext, error) {
	if user == nil {
		return nil, ErrUnauthorized
	}

	var ws sqlc.Workspace
	var err error
	resolved := false

	if requestedWorkspaceID != nil && *requestedWorkspaceID != uuid.Nil {
		ws, err = s.store.Queries().GetWorkspaceByID(ctx, *requestedWorkspaceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrWorkspaceNotFound
			}
			return nil, err
		}
		resolved = true
	} else if user.ActiveWorkspaceID != nil && *user.ActiveWorkspaceID != uuid.Nil {
		candidate, candErr := s.store.Queries().GetWorkspaceByID(ctx, *user.ActiveWorkspaceID)
		if candErr == nil {
			if user.IsSuperadmin {
				ws = candidate
				resolved = true
			} else if candidate.Status == "active" {
				if m, mErr := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
					WorkspaceID: candidate.ID,
					UserID:      user.ID,
				}); mErr == nil && m.Status == "active" {
					ws = candidate
					resolved = true
				}
			}
		} else if !errors.Is(candErr, pgx.ErrNoRows) {
			return nil, candErr
		}
	}

	if !resolved {
		wss, listErr := s.store.Queries().ListWorkspacesForUser(ctx, user.ID)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range wss {
			if item.Status == "active" || user.IsSuperadmin {
				ws = sqlc.Workspace{
					ID:                  item.ID,
					Name:                item.Name,
					Slug:                item.Slug,
					Plan:                item.Plan,
					Status:              item.Status,
					SuspendedReason:     item.SuspendedReason,
					SuspendedAt:         item.SuspendedAt,
					SuspendedBy:         item.SuspendedBy,
					MaxUsersOverride:    item.MaxUsersOverride,
					MaxProductsOverride: item.MaxProductsOverride,
					MaxLicensesOverride: item.MaxLicensesOverride,
					OverrideExpiresAt:   item.OverrideExpiresAt,
					OverrideNote:        item.OverrideNote,
					CreatedBy:           item.CreatedBy,
					DeletedAt:           item.DeletedAt,
					CreatedAt:           item.CreatedAt,
					UpdatedAt:           item.UpdatedAt,
				}
				resolved = true
				break
			}
		}
		if !resolved && user.IsSuperadmin {
			ws, err = s.store.Queries().GetFirstActiveWorkspace(ctx)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, ErrWorkspaceNotFound
				}
				return nil, err
			}
			resolved = true
		}
	}

	if !resolved {
		return nil, ErrWorkspaceNotFound
	}

	member, memberErr := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
	})
	var effectiveRole string
	if !user.IsSuperadmin {
		if memberErr != nil {
			if errors.Is(memberErr, pgx.ErrNoRows) {
				return nil, ErrForbidden
			}
			return nil, memberErr
		}
		if member.Status != "active" {
			return nil, ErrForbidden
		}
		if ws.Status != "active" {
			return nil, ErrWorkspaceSuspended
		}
		effectiveRole = normalizeRole(member.Role)
	} else {
		if memberErr == nil {
			effectiveRole = normalizeRole(member.Role)
		} else if errors.Is(memberErr, pgx.ErrNoRows) {
			effectiveRole = RoleOwner
		} else {
			return nil, memberErr
		}
	}

	return &WorkspaceContext{
		Workspace:   ws,
		Role:        effectiveRole,
		Permissions: PermissionsFor(effectiveRole, user.IsSuperadmin),
		Limits:      EffectiveWorkspaceLimits(ws, s.now()),
	}, nil
}

// ListUserWorkspaces returns workspaces accessible to the user.
// Superadmins see all non-deleted workspaces so they can jump into any tenant workspace.
func (s *Service) ListUserWorkspaces(ctx context.Context, userID uuid.UUID, isSuperadmin bool) ([]WorkspaceDTO, error) {
	now := s.now()
	if isSuperadmin {
		rows, err := s.store.Queries().ListAllWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]WorkspaceDTO, 0, len(rows))
		for _, r := range rows {
			ws := sqlc.Workspace{
				ID:                  r.ID,
				Name:                r.Name,
				Slug:                r.Slug,
				Plan:                r.Plan,
				Status:              r.Status,
				SuspendedReason:     r.SuspendedReason,
				SuspendedAt:         r.SuspendedAt,
				SuspendedBy:         r.SuspendedBy,
				MaxUsersOverride:    r.MaxUsersOverride,
				MaxProductsOverride: r.MaxProductsOverride,
				MaxLicensesOverride: r.MaxLicensesOverride,
				OverrideExpiresAt:   r.OverrideExpiresAt,
				OverrideNote:        r.OverrideNote,
				CreatedBy:           r.CreatedBy,
				DeletedAt:           r.DeletedAt,
				CreatedAt:           r.CreatedAt,
				UpdatedAt:           r.UpdatedAt,
			}
			out = append(out, WorkspaceToDTO(ws, RoleOwner, r.MemberCount, now))
		}
		return out, nil
	}

	rows, err := s.store.Queries().ListWorkspacesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceDTO, 0, len(rows))
	for _, r := range rows {
		ws := sqlc.Workspace{
			ID:                  r.ID,
			Name:                r.Name,
			Slug:                r.Slug,
			Plan:                r.Plan,
			Status:              r.Status,
			SuspendedReason:     r.SuspendedReason,
			SuspendedAt:         r.SuspendedAt,
			SuspendedBy:         r.SuspendedBy,
			MaxUsersOverride:    r.MaxUsersOverride,
			MaxProductsOverride: r.MaxProductsOverride,
			MaxLicensesOverride: r.MaxLicensesOverride,
			OverrideExpiresAt:   r.OverrideExpiresAt,
			OverrideNote:        r.OverrideNote,
			CreatedBy:           r.CreatedBy,
			DeletedAt:           r.DeletedAt,
			CreatedAt:           r.CreatedAt,
			UpdatedAt:           r.UpdatedAt,
		}
		out = append(out, WorkspaceToDTO(ws, normalizeRole(r.MemberRole), 0, now))
	}
	return out, nil
}

func validateWorkspaceNameAndSlug(name, slug string) (string, string, error) {
	name = strings.TrimSpace(name)
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 2 || nameLen > 120 {
		return "", "", ErrInvalidWorkspaceName
	}

	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		slug = slugifyWorkspaceName(name)
	}
	if !workspaceSlugPattern.MatchString(slug) {
		return "", "", ErrInvalidWorkspaceSlug
	}
	return name, slug, nil
}

// CreateWorkspace creates a new workspace, assigns the creator as owner, and switches their session to it.
func (s *Service) CreateWorkspace(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	name string,
	slug string,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return nil, err
	}

	name, slug, err = validateWorkspaceNameAndSlug(name, slug)
	if err != nil {
		return nil, err
	}

	if _, lookupErr := s.store.Queries().GetWorkspaceBySlug(ctx, slug); lookupErr == nil {
		return nil, ErrConflictWorkspaceSlug
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return nil, lookupErr
	}

	var created sqlc.Workspace
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		ws, txErr := q.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{
			Name:      name,
			Lower:     slug,
			Plan:      FreePlan,
			CreatedBy: uuid.NullUUID{UUID: actor.ID, Valid: true},
		})
		if txErr != nil {
			return txErr
		}
		if !actor.IsSuperadmin {
			if _, txErr := q.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
				WorkspaceID: ws.ID,
				UserID:      actor.ID,
				Role:        RoleOwner,
				InvitedBy:   uuid.NullUUID{},
			}); txErr != nil {
				return txErr
			}
		}
		if sessionID != uuid.Nil {
			if txErr := q.SetSessionActiveWorkspace(ctx, sqlc.SetSessionActiveWorkspaceParams{
				ID:                sessionID,
				ActiveWorkspaceID: uuid.NullUUID{UUID: ws.ID, Valid: true},
			}); txErr != nil {
				return txErr
			}
		}
		created = ws
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: created.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_created",
		meta,
		map[string]string{"slug": created.Slug, "name": created.Name},
	)

	dto := WorkspaceToDTO(created, RoleOwner, 1, s.now())
	return &dto, nil
}

// SwitchActiveWorkspace validates access to workspaceID and updates the session's active_workspace_id.
func (s *Service) SwitchActiveWorkspace(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	workspaceID uuid.UUID,
) (*WorkspaceContext, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	userDTO := userToDTO(actor)
	wsCtx, err := s.ResolveWorkspaceContext(ctx, &userDTO, &workspaceID)
	if err != nil {
		return nil, err
	}
	if sessionID != uuid.Nil {
		if err := s.store.Queries().SetSessionActiveWorkspace(ctx, sqlc.SetSessionActiveWorkspaceParams{
			ID:                sessionID,
			ActiveWorkspaceID: uuid.NullUUID{UUID: wsCtx.Workspace.ID, Valid: true},
		}); err != nil {
			return nil, err
		}
	}
	return wsCtx, nil
}

// ExitActiveWorkspace clears the session's active_workspace_id (returning a superadmin to platform mode).
func (s *Service) ExitActiveWorkspace(ctx context.Context, actorID uuid.UUID, sessionID uuid.UUID) error {
	if _, err := s.requireActiveUser(ctx, actorID); err != nil {
		return err
	}
	if sessionID == uuid.Nil {
		return nil
	}
	return s.store.Queries().SetSessionActiveWorkspace(ctx, sqlc.SetSessionActiveWorkspaceParams{
		ID:                sessionID,
		ActiveWorkspaceID: uuid.NullUUID{},
	})
}

// ListPendingWorkspaceInvites returns pending workspace invitations for userID.
func (s *Service) ListPendingWorkspaceInvites(ctx context.Context, userID uuid.UUID) ([]PendingWorkspaceInviteDTO, error) {
	if _, err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.store.Queries().ListPendingWorkspaceInvitesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]PendingWorkspaceInviteDTO, 0, len(rows))
	for _, r := range rows {
		var invName, invEmail *string
		if r.InvitedByName.Valid {
			v := r.InvitedByName.String
			invName = &v
		}
		if r.InvitedByEmail.Valid {
			v := r.InvitedByEmail.String
			invEmail = &v
		}
		out = append(out, PendingWorkspaceInviteDTO{
			WorkspaceID:    r.WorkspaceID,
			WorkspaceName:  r.WorkspaceName,
			WorkspaceSlug:  r.WorkspaceSlug,
			WorkspacePlan:  r.WorkspacePlan,
			Role:           normalizeRole(r.MemberRole),
			InvitedAt:      r.InvitedAt,
			InvitedByName:  invName,
			InvitedByEmail: invEmail,
		})
	}
	return out, nil
}

// AcceptWorkspaceInvite activates a pending workspace membership for userID and switches the session to it.
func (s *Service) AcceptWorkspaceInvite(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	workspaceID uuid.UUID,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ws, err := s.store.Queries().GetWorkspaceByID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}
	if ws.Status != "active" {
		return nil, ErrWorkspaceSuspended
	}

	var accepted sqlc.WorkspaceMember
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, lockErr := q.GetWorkspaceByIDForUpdate(ctx, ws.ID)
		if lockErr != nil {
			if errors.Is(lockErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return lockErr
		}
		if lockedWS.Status != "active" {
			return ErrWorkspaceSuspended
		}
		m, txErr := q.AcceptWorkspaceMemberInvite(ctx, sqlc.AcceptWorkspaceMemberInviteParams{
			WorkspaceID: lockedWS.ID,
			UserID:      actor.ID,
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInviteNotFound
			}
			return txErr
		}
		accepted = m
		if sessionID != uuid.Nil {
			if txErr := q.SetSessionActiveWorkspace(ctx, sqlc.SetSessionActiveWorkspaceParams{
				ID:                sessionID,
				ActiveWorkspaceID: uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			}); txErr != nil {
				return txErr
			}
		}
		return recordMembershipAuditTx(
			ctx,
			q,
			actor.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			"workspace_invite_accepted",
			normalizeRole(m.Role),
			normalizeRole(m.Role),
			"pending",
			m.Status,
			"",
			meta,
			nil,
		)
	})
	if err != nil {
		return nil, err
	}

	dto := WorkspaceToDTO(ws, normalizeRole(accepted.Role), 0, s.now())
	return &dto, nil
}

// DeclineWorkspaceInvite soft-removes a pending workspace invitation for userID.
func (s *Service) DeclineWorkspaceInvite(
	ctx context.Context,
	userID uuid.UUID,
	workspaceID uuid.UUID,
	meta RequestMeta,
) error {
	actor, err := s.requireActiveUser(ctx, userID)
	if err != nil {
		return err
	}
	member, err := s.store.Queries().GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      actor.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInviteNotFound
		}
		return err
	}
	if member.Status != "pending" {
		return ErrInviteNotFound
	}
	if _, err := s.store.Queries().RemoveWorkspaceMember(ctx, sqlc.RemoveWorkspaceMemberParams{
		WorkspaceID:   workspaceID,
		UserID:        actor.ID,
		RemovedBy:     uuid.NullUUID{UUID: actor.ID, Valid: true},
		RemovalReason: pgText("invite declined"),
	}); err != nil {
		return err
	}

	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: workspaceID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_invite_declined",
		meta,
		nil,
	)
	return nil
}

// LeaveWorkspace soft-removes userID from workspaceID unless they are the last active admin/owner or member.
func (s *Service) LeaveWorkspace(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	workspaceID uuid.UUID,
	meta RequestMeta,
) error {
	actor, err := s.requireActiveUser(ctx, userID)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, lockErr := q.GetWorkspaceByIDForUpdate(ctx, workspaceID)
		if lockErr != nil {
			if errors.Is(lockErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return lockErr
		}
		member, memErr := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
			WorkspaceID: lockedWS.ID,
			UserID:      actor.ID,
		})
		if memErr != nil {
			if errors.Is(memErr, pgx.ErrNoRows) {
				return ErrForbidden
			}
			return memErr
		}
		if member.Status == "removed" {
			return ErrForbidden
		}
		if member.Status == "pending" {
			if _, rmErr := q.RemoveWorkspaceMember(ctx, sqlc.RemoveWorkspaceMemberParams{
				WorkspaceID:   lockedWS.ID,
				UserID:        actor.ID,
				RemovedBy:     uuid.NullUUID{UUID: actor.ID, Valid: true},
				RemovalReason: pgText("invite declined"),
			}); rmErr != nil {
				return rmErr
			}
			return recordMembershipAuditTx(
				ctx,
				q,
				actor.ID,
				uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
				actor.ID,
				"workspace_invite_declined",
				normalizeRole(member.Role),
				"",
				member.Status,
				"removed",
				"",
				meta,
				nil,
			)
		}

		if invErr := validateWorkspaceMembershipIntegrity(
			ctx,
			q,
			lockedWS,
			member,
			actor,
			member.Role,
			"removed",
			actor.Status,
			ErrSoleOwnerCannotLeave,
		); invErr != nil {
			return invErr
		}

		removed, txErr := q.RemoveWorkspaceMember(ctx, sqlc.RemoveWorkspaceMemberParams{
			WorkspaceID:   lockedWS.ID,
			UserID:        actor.ID,
			RemovedBy:     uuid.NullUUID{UUID: actor.ID, Valid: true},
			RemovalReason: pgText("member left workspace"),
		})
		if txErr != nil {
			return txErr
		}
		if sessionID != uuid.Nil {
			if txErr := q.SetSessionActiveWorkspace(ctx, sqlc.SetSessionActiveWorkspaceParams{
				ID:                sessionID,
				ActiveWorkspaceID: uuid.NullUUID{},
			}); txErr != nil {
				return txErr
			}
		}
		if clearErr := q.ClearUserWorkspaceSession(ctx, sqlc.ClearUserWorkspaceSessionParams{
			UserID:            actor.ID,
			ActiveWorkspaceID: uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
		}); clearErr != nil {
			return clearErr
		}
		return recordMembershipAuditTx(
			ctx,
			q,
			actor.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			AuditEventMemberRemoved,
			normalizeRole(member.Role),
			"",
			member.Status,
			removed.Status,
			"member left workspace",
			meta,
			map[string]any{"self_leave": true},
		)
	})
	if err != nil {
		var govErr *MembershipGovernanceError
		if errors.As(err, &govErr) {
			s.recordRejectedMembershipOperation(
				ctx,
				actor.ID,
				uuid.NullUUID{UUID: workspaceID, Valid: true},
				actor.ID,
				AuditEventMembershipOpRejected,
				"leave_workspace",
				err,
				meta,
			)
		}
		return err
	}
	return nil
}

func requireWorkspaceOwnerOrSuperadminTx(
	ctx context.Context,
	q sqlc.Querier,
	workspaceID uuid.UUID,
	actor sqlc.User,
) (sqlc.Workspace, error) {
	ws, err := q.GetWorkspaceByIDForUpdate(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Workspace{}, ErrWorkspaceNotFound
		}
		return sqlc.Workspace{}, err
	}
	if actor.IsSuperadmin {
		return ws, nil
	}
	if ws.Status == "suspended" {
		return sqlc.Workspace{}, ErrWorkspaceSuspended
	}
	member, err := q.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      actor.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Workspace{}, ErrForbidden
		}
		return sqlc.Workspace{}, err
	}
	if member.Status != "active" || normalizeRole(member.Role) != RoleOwner {
		return sqlc.Workspace{}, ErrForbidden
	}
	return ws, nil
}

// UpdateWorkspace updates workspace name and slug (owner or superadmin only).
func (s *Service) UpdateWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	name string,
	slug string,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return nil, err
	}

	name, slug, err = validateWorkspaceNameAndSlug(name, slug)
	if err != nil {
		return nil, err
	}

	if existing, lookupErr := s.store.Queries().GetWorkspaceBySlug(ctx, slug); lookupErr == nil {
		if existing.ID != workspaceID {
			return nil, ErrConflictWorkspaceSlug
		}
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return nil, lookupErr
	}

	var updated sqlc.Workspace
	var memberCount int64
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := requireWorkspaceOwnerOrSuperadminTx(ctx, q, workspaceID, actor)
		if txErr != nil {
			return txErr
		}
		ws, txErr := q.UpdateWorkspaceDetails(ctx, sqlc.UpdateWorkspaceDetailsParams{
			ID:   lockedWS.ID,
			Name: name,
			Slug: slug,
		})
		if txErr != nil {
			if strings.Contains(txErr.Error(), "23505") {
				return ErrConflictWorkspaceSlug
			}
			return txErr
		}
		cnt, cntErr := q.CountWorkspaceMembers(ctx, ws.ID)
		if cntErr != nil {
			return cntErr
		}
		updated = ws
		memberCount = cnt
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: updated.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_updated",
		meta,
		map[string]string{"slug": updated.Slug, "name": updated.Name},
	)

	role := RoleOwner
	if actor.IsSuperadmin {
		role = RoleOwner
	}
	dto := WorkspaceToDTO(updated, role, memberCount, s.now())
	return &dto, nil
}

// DeleteWorkspace soft-deletes a workspace and clears active sessions pointing to it (owner or superadmin only).
func (s *Service) DeleteWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	meta RequestMeta,
) error {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return err
	}

	var deletedWS sqlc.Workspace
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := requireWorkspaceOwnerOrSuperadminTx(ctx, q, workspaceID, actor)
		if txErr != nil {
			return txErr
		}
		ws, txErr := q.SoftDeleteWorkspace(ctx, lockedWS.ID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrWorkspaceNotFound
			}
			return txErr
		}
		if clearErr := q.ClearWorkspaceSessions(ctx, uuid.NullUUID{UUID: ws.ID, Valid: true}); clearErr != nil {
			return clearErr
		}
		deletedWS = ws
		return nil
	})
	if err != nil {
		return err
	}

	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: deletedWS.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_deleted",
		meta,
		map[string]string{"slug": deletedWS.Slug, "name": deletedWS.Name},
	)
	return nil
}

// TransferWorkspaceOwnership promotes an active non-superadmin member to owner and demotes
// the acting owner to admin within the same transaction.
func (s *Service) TransferWorkspaceOwnership(
	ctx context.Context,
	workspaceID uuid.UUID,
	actorID uuid.UUID,
	targetUserID uuid.UUID,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if actor.ID == targetUserID {
		return nil, ErrForbidden
	}

	target, err := s.store.Queries().GetUserByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if target.DeletedAt != nil || target.Status != "active" || target.IsSuperadmin {
		return nil, ErrForbidden
	}

	var ws sqlc.Workspace
	var memberCount int64
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		lockedWS, txErr := requireWorkspaceOwnerOrSuperadminTx(ctx, q, workspaceID, actor)
		if txErr != nil {
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
		if targetMember.Status != "active" {
			return ErrForbidden
		}

		oldTargetRole := normalizeRole(targetMember.Role)
		if _, txErr := q.UpdateWorkspaceMemberRole(ctx, sqlc.UpdateWorkspaceMemberRoleParams{
			WorkspaceID: lockedWS.ID,
			UserID:      target.ID,
			Role:        RoleOwner,
		}); txErr != nil {
			return txErr
		}

		if !actor.IsSuperadmin {
			if _, txErr := q.UpdateWorkspaceMemberRole(ctx, sqlc.UpdateWorkspaceMemberRoleParams{
				WorkspaceID: lockedWS.ID,
				UserID:      actor.ID,
				Role:        RoleAdmin,
			}); txErr != nil {
				return txErr
			}
		}

		if auditErr := recordMembershipAuditTx(
			ctx,
			q,
			target.ID,
			uuid.NullUUID{UUID: lockedWS.ID, Valid: true},
			actor.ID,
			"workspace_ownership_transferred",
			oldTargetRole,
			RoleOwner,
			targetMember.Status,
			targetMember.Status,
			"ownership transferred",
			meta,
			map[string]any{"from_user_id": actor.ID.String(), "to_user_id": target.ID.String()},
		); auditErr != nil {
			return auditErr
		}

		cnt, cntErr := q.CountWorkspaceMembers(ctx, lockedWS.ID)
		if cntErr != nil {
			return cntErr
		}
		ws = lockedWS
		memberCount = cnt
		return nil
	})
	if err != nil {
		return nil, err
	}

	role := RoleAdmin
	if actor.IsSuperadmin {
		role = RoleOwner
	}
	dto := WorkspaceToDTO(ws, role, memberCount, s.now())
	return &dto, nil
}

// ListAllWorkspacesPlatform returns all non-deleted workspaces for the platform Super Admin console.
func (s *Service) ListAllWorkspacesPlatform(ctx context.Context, actorID uuid.UUID) ([]WorkspaceDTO, error) {
	if _, err := s.requireActiveSuperadmin(ctx, actorID); err != nil {
		return nil, err
	}
	return s.ListUserWorkspaces(ctx, actorID, true)
}

// UpdateWorkspacePlanAndOverrides updates a workspace's plan and optional custom quota overrides (Super Admin only).
func (s *Service) UpdateWorkspacePlanAndOverrides(
	ctx context.Context,
	actorID uuid.UUID,
	workspaceID uuid.UUID,
	plan string,
	maxUsersOverride *int32,
	maxProductsOverride *int32,
	maxLicensesOverride *int32,
	overrideExpiresAt *time.Time,
	overrideNote string,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveSuperadmin(ctx, actorID)
	if err != nil {
		return nil, err
	}

	plan = strings.ToLower(strings.TrimSpace(plan))
	if !IsValidPlan(plan) {
		return nil, ErrInvalidPlan
	}

	toPGInt4 := func(v *int32) pgtype.Int4 {
		if v == nil || *v <= 0 {
			return pgtype.Int4{}
		}
		return pgtype.Int4{Int32: *v, Valid: true}
	}

	updated, err := s.store.Queries().UpdateWorkspacePlanAndOverrides(ctx, sqlc.UpdateWorkspacePlanAndOverridesParams{
		ID:                  workspaceID,
		Plan:                plan,
		MaxUsersOverride:    toPGInt4(maxUsersOverride),
		MaxProductsOverride: toPGInt4(maxProductsOverride),
		MaxLicensesOverride: toPGInt4(maxLicensesOverride),
		OverrideExpiresAt:   overrideExpiresAt,
		OverrideNote:        pgText(strings.TrimSpace(overrideNote)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}

	memberCount, _ := s.store.Queries().CountWorkspaceMembers(ctx, updated.ID)

	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: updated.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_plan_updated",
		meta,
		map[string]any{
			"plan":                  updated.Plan,
			"max_users_override":    maxUsersOverride,
			"max_products_override": maxProductsOverride,
			"max_licenses_override": maxLicensesOverride,
		},
	)

	dto := WorkspaceToDTO(updated, RoleOwner, memberCount, s.now())
	return &dto, nil
}

// SuspendWorkspacePlatform freezes an entire tenant workspace with a required reason (Super Admin only).
func (s *Service) SuspendWorkspacePlatform(
	ctx context.Context,
	actorID uuid.UUID,
	workspaceID uuid.UUID,
	reason string,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveSuperadmin(ctx, actorID)
	if err != nil {
		return nil, err
	}
	trimmedReason := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmedReason) < 3 {
		return nil, ErrInvalidSuspendReason
	}

	updated, err := s.store.Queries().SuspendWorkspace(ctx, sqlc.SuspendWorkspaceParams{
		ID:              workspaceID,
		SuspendedReason: pgText(trimmedReason),
		SuspendedBy:     uuid.NullUUID{UUID: actor.ID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}

	memberCount, _ := s.store.Queries().CountWorkspaceMembers(ctx, updated.ID)
	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: updated.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_suspended",
		meta,
		map[string]string{"reason": trimmedReason},
	)

	dto := WorkspaceToDTO(updated, RoleOwner, memberCount, s.now())
	return &dto, nil
}

// ActivateWorkspacePlatform unfreezes a suspended workspace (Super Admin only).
func (s *Service) ActivateWorkspacePlatform(
	ctx context.Context,
	actorID uuid.UUID,
	workspaceID uuid.UUID,
	meta RequestMeta,
) (*WorkspaceDTO, error) {
	actor, err := s.requireActiveSuperadmin(ctx, actorID)
	if err != nil {
		return nil, err
	}

	updated, err := s.store.Queries().ActivateWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}

	memberCount, _ := s.store.Queries().CountWorkspaceMembers(ctx, updated.ID)
	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		uuid.NullUUID{UUID: updated.ID, Valid: true},
		uuid.NullUUID{UUID: actor.ID, Valid: true},
		"workspace_activated",
		meta,
		nil,
	)

	dto := WorkspaceToDTO(updated, RoleOwner, memberCount, s.now())
	return &dto, nil
}

// ListPlatformUsers returns all non-deleted users across the platform (Super Admin only).
func (s *Service) ListPlatformUsers(ctx context.Context, actorID uuid.UUID) ([]PlatformUserDTO, error) {
	if _, err := s.requireActiveSuperadmin(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := s.store.Queries().ListPlatformUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformUserDTO, 0, len(rows))
	for _, r := range rows {
		var avatar *string
		if r.AvatarUrl.Valid {
			v := normalizeAvatarRelativePath(r.AvatarUrl.String)
			avatar = &v
		}
		var suspendedReason *string
		if r.SuspendedReason.Valid {
			v := r.SuspendedReason.String
			suspendedReason = &v
		}
		out = append(out, PlatformUserDTO{
			ID:              r.ID,
			Name:            r.Name,
			Email:           r.Email,
			AvatarURL:       avatar,
			IsSuperadmin:    r.IsSuperadmin,
			Status:          r.Status,
			SuspendedReason: suspendedReason,
			SuspendedAt:     r.SuspendedAt,
			WorkspaceCount:  r.WorkspaceCount,
			CreatedAt:       r.CreatedAt,
		})
	}
	return out, nil
}

// PlatformSuspendUser globally suspends a non-superadmin user after validating BR-01 and BR-02
// across every affected workspace inside a locked transaction (BR-07).
func (s *Service) PlatformSuspendUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
	meta RequestMeta,
) (*PlatformUserDTO, error) {
	actor, err := s.requireActiveSuperadmin(ctx, actorID)
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

	var updated sqlc.User
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		target, txErr := q.GetUserByIDForUpdate(ctx, targetID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if target.IsSuperadmin {
			return ErrForbidden
		}

		impact, violation, valErr := validateGlobalSuspensionImpactTx(ctx, q, target)
		if valErr != nil {
			return valErr
		}
		if violation != nil {
			return violation
		}

		u, txErr := q.SuspendUser(ctx, sqlc.SuspendUserParams{
			ID:              target.ID,
			SuspendedReason: pgText(trimmedReason),
		})
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if txErr := q.DeleteUserSessions(ctx, target.ID); txErr != nil {
			return txErr
		}

		wsIDs := make([]string, 0, len(impact.Workspaces))
		for _, w := range impact.Workspaces {
			wsIDs = append(wsIDs, w.WorkspaceID.String())
		}
		if txErr := recordMembershipAuditTx(
			ctx,
			q,
			target.ID,
			uuid.NullUUID{},
			actor.ID,
			AuditEventUserGloballySuspended,
			"",
			"",
			target.Status,
			u.Status,
			trimmedReason,
			meta,
			map[string]any{"affected_workspace_ids": wsIDs},
		); txErr != nil {
			return txErr
		}
		updated = u
		return nil
	})
	if err != nil {
		var govErr *MembershipGovernanceError
		if errors.As(err, &govErr) {
			s.recordRejectedMembershipOperation(
				ctx,
				targetID,
				uuid.NullUUID{UUID: govErr.WorkspaceID, Valid: govErr.WorkspaceID != uuid.Nil},
				actor.ID,
				AuditEventGlobalSuspendOpRejected,
				"global_suspend",
				err,
				meta,
			)
		}
		return nil, err
	}

	wsList, _ := s.store.Queries().ListWorkspacesForUser(ctx, updated.ID)
	var avatar *string
	if updated.AvatarUrl.Valid {
		v := normalizeAvatarRelativePath(updated.AvatarUrl.String)
		avatar = &v
	}
	var suspendedReason *string
	if updated.SuspendedReason.Valid {
		v := updated.SuspendedReason.String
		suspendedReason = &v
	}
	return &PlatformUserDTO{
		ID:              updated.ID,
		Name:            updated.Name,
		Email:           updated.Email,
		AvatarURL:       avatar,
		IsSuperadmin:    updated.IsSuperadmin,
		Status:          updated.Status,
		SuspendedReason: suspendedReason,
		SuspendedAt:     updated.SuspendedAt,
		WorkspaceCount:  int64(len(wsList)),
		CreatedAt:       updated.CreatedAt,
	}, nil
}

// PlatformActivateUser globally reactivates a user account (Super Admin only).
func (s *Service) PlatformActivateUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	meta RequestMeta,
) (*PlatformUserDTO, error) {
	actor, err := s.requireActiveSuperadmin(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var updated sqlc.User
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		target, txErr := q.GetUserByIDForUpdate(ctx, targetID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		u, txErr := q.ActivateUser(ctx, target.ID)
		if txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return txErr
		}
		if txErr := recordMembershipAuditTx(
			ctx,
			q,
			u.ID,
			uuid.NullUUID{},
			actor.ID,
			AuditEventUserGloballyUnsuspended,
			"",
			"",
			target.Status,
			u.Status,
			"",
			meta,
			nil,
		); txErr != nil {
			return txErr
		}
		updated = u
		return nil
	})
	if err != nil {
		return nil, err
	}

	wsList, _ := s.store.Queries().ListWorkspacesForUser(ctx, updated.ID)
	var avatar *string
	if updated.AvatarUrl.Valid {
		v := normalizeAvatarRelativePath(updated.AvatarUrl.String)
		avatar = &v
	}
	return &PlatformUserDTO{
		ID:             updated.ID,
		Name:           updated.Name,
		Email:          updated.Email,
		AvatarURL:      avatar,
		IsSuperadmin:   updated.IsSuperadmin,
		Status:         updated.Status,
		WorkspaceCount: int64(len(wsList)),
		CreatedAt:      updated.CreatedAt,
	}, nil
}

func (s *Service) requireActiveUser(ctx context.Context, userID uuid.UUID) (sqlc.User, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, ErrUnauthorized
		}
		return sqlc.User{}, err
	}
	if user.Status != "active" || user.DeletedAt != nil {
		return sqlc.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s *Service) requireActiveSuperadmin(ctx context.Context, actorID uuid.UUID) (sqlc.User, error) {
	actor, err := s.requireActiveUser(ctx, actorID)
	if err != nil {
		return sqlc.User{}, err
	}
	if !actor.IsSuperadmin {
		return sqlc.User{}, ErrForbidden
	}
	return actor, nil
}
