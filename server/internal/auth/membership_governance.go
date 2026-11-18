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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"riralicense/internal/db/sqlc"
)

// Audit event names for workspace membership governance.
const (
	AuditEventMemberRemoved             = "MEMBER_REMOVED"
	AuditEventMemberSuspended           = "MEMBER_SUSPENDED"
	AuditEventMemberRoleChanged         = "MEMBER_ROLE_CHANGED"
	AuditEventMemberReactivated         = "MEMBER_REACTIVATED"
	AuditEventUserGloballySuspended     = "USER_GLOBALLY_SUSPENDED"
	AuditEventUserGloballyUnsuspended   = "USER_GLOBALLY_UNSUSPENDED"
	AuditEventMembershipOpRejected      = "MEMBERSHIP_OPERATION_REJECTED"
	AuditEventGlobalSuspendOpRejected   = "USER_GLOBAL_SUSPEND_REJECTED"
)

func isAdministrativeRole(role string) bool {
	r := normalizeRole(role)
	return r == RoleOwner || r == RoleAdmin
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

// validateWorkspaceMembershipIntegrity enforces BR-01 (minimum 1 active admin/owner)
// and BR-02 (minimum 1 active member) inside the active transaction for workspace ws.
func validateWorkspaceMembershipIntegrity(
	ctx context.Context,
	q sqlc.Querier,
	ws sqlc.Workspace,
	targetMember sqlc.WorkspaceMember,
	targetUser sqlc.User,
	nextRole string,
	nextMemberStatus string,
	nextUserStatus string,
	compatErr error,
) error {
	stats, err := q.GetWorkspaceActiveMembershipStats(ctx, ws.ID)
	if err != nil {
		return err
	}

	curRole := normalizeRole(targetMember.Role)
	nxtRole := normalizeRole(nextRole)

	wasActiveMember := targetMember.Status == "active" &&
		targetUser.Status == "active" &&
		targetUser.DeletedAt == nil &&
		!targetUser.IsSuperadmin
	wasActiveAdmin := wasActiveMember && isAdministrativeRole(curRole)
	wasActiveOwner := wasActiveMember && curRole == RoleOwner

	willBeActiveMember := nextMemberStatus == "active" &&
		nextUserStatus == "active" &&
		targetUser.DeletedAt == nil &&
		!targetUser.IsSuperadmin
	willBeActiveAdmin := willBeActiveMember && isAdministrativeRole(nxtRole)
	willBeActiveOwner := willBeActiveMember && nxtRole == RoleOwner

	nextActiveMembers := stats.ActiveMembers - boolToInt64(wasActiveMember) + boolToInt64(willBeActiveMember)
	nextActiveAdmins := stats.ActiveAdmins - boolToInt64(wasActiveAdmin) + boolToInt64(willBeActiveAdmin)
	nextActiveOwners := stats.ActiveOwners - boolToInt64(wasActiveOwner) + boolToInt64(willBeActiveOwner)

	if compatErr == nil {
		compatErr = ErrForbidden
	}

	if (wasActiveAdmin && !willBeActiveAdmin && nextActiveAdmins < 1) ||
		(wasActiveOwner && !willBeActiveOwner && nextActiveOwners < 1) {
		return &MembershipGovernanceError{
			Code:          GovCodeLastActiveAdmin,
			Message:       ErrLastActiveAdmin.Error(),
			WorkspaceID:   ws.ID,
			WorkspaceName: ws.Name,
			Base:          ErrLastActiveAdmin,
			Compat:        compatErr,
		}
	}

	if wasActiveMember && !willBeActiveMember && nextActiveMembers < 1 {
		return &MembershipGovernanceError{
			Code:          GovCodeLastActiveMember,
			Message:       ErrLastActiveMember.Error(),
			WorkspaceID:   ws.ID,
			WorkspaceName: ws.Name,
			Base:          ErrLastActiveMember,
			Compat:        compatErr,
		}
	}

	return nil
}

// validateGlobalSuspensionImpactTx locks all active workspaces for targetUser in deterministic
// order and validates BR-01 and BR-02 across every affected workspace.
func validateGlobalSuspensionImpactTx(
	ctx context.Context,
	q sqlc.Querier,
	targetUser sqlc.User,
) (*GlobalSuspendImpactDTO, *MembershipGovernanceError, error) {
	affected, err := q.ListActiveWorkspacesForUserForUpdate(ctx, targetUser.ID)
	if err != nil {
		return nil, nil, err
	}

	impact := &GlobalSuspendImpactDTO{
		UserID:     targetUser.ID,
		Allowed:    true,
		Workspaces: make([]GlobalSuspendWorkspaceImpactDTO, 0, len(affected)),
	}

	var firstViolation *MembershipGovernanceError
	for _, aw := range affected {
		stats, statsErr := q.GetWorkspaceActiveMembershipStats(ctx, aw.WorkspaceID)
		if statsErr != nil {
			return nil, nil, statsErr
		}

		role := normalizeRole(aw.MemberRole)
		wasActiveMember := aw.MemberStatus == "active" &&
			targetUser.Status == "active" &&
			targetUser.DeletedAt == nil &&
			!targetUser.IsSuperadmin
		wasActiveAdmin := wasActiveMember && isAdministrativeRole(role)
		wasActiveOwner := wasActiveMember && role == RoleOwner

		nextMembers := stats.ActiveMembers - boolToInt64(wasActiveMember)
		nextAdmins := stats.ActiveAdmins - boolToInt64(wasActiveAdmin)
		nextOwners := stats.ActiveOwners - boolToInt64(wasActiveOwner)

		item := GlobalSuspendWorkspaceImpactDTO{
			WorkspaceID:       aw.WorkspaceID,
			WorkspaceName:     aw.WorkspaceName,
			WorkspaceSlug:     aw.WorkspaceSlug,
			Role:              role,
			ActiveMembers:     stats.ActiveMembers,
			ActiveAdmins:      stats.ActiveAdmins,
			ActiveOwners:      stats.ActiveOwners,
			NextActiveMembers: nextMembers,
			NextActiveAdmins:  nextAdmins,
			NextActiveOwners:  nextOwners,
		}

		if (wasActiveAdmin && nextAdmins < 1) || (wasActiveOwner && nextOwners < 1) {
			item.Blocked = true
			item.BlockCode = GovCodeLastActiveAdmin
			item.BlockReason = fmt.Sprintf(
				"Workspace %q must have at least one active Admin. Assign another Admin before globally suspending this user.",
				aw.WorkspaceName,
			)
			impact.Allowed = false
			if firstViolation == nil {
				firstViolation = &MembershipGovernanceError{
					Code:          GovCodeLastActiveAdmin,
					Message:       item.BlockReason,
					WorkspaceID:   aw.WorkspaceID,
					WorkspaceName: aw.WorkspaceName,
					Base:          ErrLastActiveAdmin,
					Compat:        ErrGlobalSuspendBlocked,
				}
			}
		} else if wasActiveMember && nextMembers < 1 {
			item.Blocked = true
			item.BlockCode = GovCodeLastActiveMember
			item.BlockReason = fmt.Sprintf(
				"Workspace %q must have at least one active member. Add another member before globally suspending this user.",
				aw.WorkspaceName,
			)
			impact.Allowed = false
			if firstViolation == nil {
				firstViolation = &MembershipGovernanceError{
					Code:          GovCodeLastActiveMember,
					Message:       item.BlockReason,
					WorkspaceID:   aw.WorkspaceID,
					WorkspaceName: aw.WorkspaceName,
					Base:          ErrLastActiveMember,
					Compat:        ErrGlobalSuspendBlocked,
				}
			}
		}

		impact.Workspaces = append(impact.Workspaces, item)
	}

	return impact, firstViolation, nil
}

// GetGlobalSuspendImpact inspects all workspaces affected by globally suspending targetID
// without mutating state (Super Admin only).
func (s *Service) GetGlobalSuspendImpact(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
) (*GlobalSuspendImpactDTO, error) {
	if _, err := s.requireActiveSuperadmin(ctx, actorID); err != nil {
		return nil, err
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

	var impact *GlobalSuspendImpactDTO
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		imp, _, txErr := validateGlobalSuspensionImpactTx(ctx, q, target)
		if txErr != nil {
			return txErr
		}
		impact = imp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return impact, nil
}

func recordMembershipAuditTx(
	ctx context.Context,
	q sqlc.Querier,
	targetUserID uuid.UUID,
	workspaceID uuid.NullUUID,
	actorUserID uuid.UUID,
	event string,
	prevRole string,
	newRole string,
	prevStatus string,
	newStatus string,
	reason string,
	meta RequestMeta,
	extra map[string]any,
) error {
	payload := map[string]any{
		"target_user_id": targetUserID.String(),
		"actor_user_id":  actorUserID.String(),
	}
	if workspaceID.Valid {
		payload["workspace_id"] = workspaceID.UUID.String()
	}
	if prevRole != "" {
		payload["previous_role"] = prevRole
	}
	if newRole != "" {
		payload["new_role"] = newRole
	}
	if prevStatus != "" {
		payload["previous_status"] = prevStatus
	}
	if newStatus != "" {
		payload["new_status"] = newStatus
	}
	if reason != "" {
		payload["reason"] = reason
	}
	for k, v := range extra {
		payload[k] = v
	}

	rawMeta, _ := json.Marshal(payload)
	return q.InsertWorkspaceAuditLog(ctx, sqlc.InsertWorkspaceAuditLogParams{
		UserID:      uuid.NullUUID{UUID: targetUserID, Valid: true},
		WorkspaceID: workspaceID,
		ActorUserID: uuid.NullUUID{UUID: actorUserID, Valid: true},
		Event:       event,
		IpAddress:   meta.IP,
		UserAgent:   pgText(meta.UserAgent),
		Metadata:    rawMeta,
	})
}

func (s *Service) recordRejectedMembershipOperation(
	ctx context.Context,
	targetUserID uuid.UUID,
	workspaceID uuid.NullUUID,
	actorUserID uuid.UUID,
	event string,
	attemptedAction string,
	cause error,
	meta RequestMeta,
) {
	payload := map[string]any{
		"target_user_id":   targetUserID.String(),
		"actor_user_id":    actorUserID.String(),
		"attempted_action": attemptedAction,
		"rejected_reason":  cause.Error(),
	}
	var govErr *MembershipGovernanceError
	if errors.As(cause, &govErr) {
		payload["error_code"] = govErr.Code
		if govErr.WorkspaceID != uuid.Nil {
			payload["blocked_workspace_id"] = govErr.WorkspaceID.String()
		}
	}
	s.writeWorkspaceAudit(
		ctx,
		uuid.NullUUID{UUID: targetUserID, Valid: true},
		workspaceID,
		uuid.NullUUID{UUID: actorUserID, Valid: true},
		event,
		meta,
		payload,
	)
}

// ListWorkspaceAuditLogs returns preserved audit history entries for workspaceID.
func (s *Service) ListWorkspaceAuditLogs(ctx context.Context, workspaceID uuid.UUID, limit int32) ([]WorkspaceAuditLogDTO, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.store.Queries().ListWorkspaceAuditLogs(ctx, sqlc.ListWorkspaceAuditLogsParams{
		WorkspaceID: uuid.NullUUID{UUID: workspaceID, Valid: true},
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceAuditLogDTO, 0, len(rows))
	for _, r := range rows {
		var uid, wid, aid *uuid.UUID
		if r.UserID.Valid {
			v := r.UserID.UUID
			uid = &v
		}
		if r.WorkspaceID.Valid {
			v := r.WorkspaceID.UUID
			wid = &v
		}
		if r.ActorUserID.Valid {
			v := r.ActorUserID.UUID
			aid = &v
		}
		out = append(out, WorkspaceAuditLogDTO{
			ID:          r.ID,
			UserID:      uid,
			WorkspaceID: wid,
			ActorUserID: aid,
			Event:       r.Event,
			Metadata:    json.RawMessage(r.Metadata),
			CreatedAt:   r.CreatedAt,
		})
	}
	return out, nil
}

