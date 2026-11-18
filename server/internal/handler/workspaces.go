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

package handler

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"riralicense/internal/auth"
)

type createWorkspaceRequest struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	Slug string `json:"slug" validate:"max=64"`
}

type updateWorkspacePlanRequest struct {
	Plan                string     `json:"plan" validate:"required,max=32"`
	MaxUsersOverride    *int32     `json:"max_users_override"`
	MaxProductsOverride *int32     `json:"max_products_override"`
	MaxLicensesOverride *int32     `json:"max_licenses_override"`
	OverrideExpiresAt   *time.Time `json:"override_expires_at"`
	OverrideNote        string     `json:"override_note" validate:"max=500"`
}

// ListWorkspaces handles GET /api/v1/workspaces.
func (h *AuthHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	workspaces, err := h.svc.ListUserWorkspaces(ctx, user.ID, user.IsSuperadmin)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

// CreateWorkspace handles POST /api/v1/workspaces.
func (h *AuthHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createWorkspaceRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	ws, err := h.svc.CreateWorkspace(ctx, user.ID, SessionIDFromContext(r.Context()), req.Name, req.Slug, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"workspace": ws})
}

// SwitchWorkspace handles POST /api/v1/workspaces/{id}/switch.
func (h *AuthHandler) SwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	wsCtx, err := h.svc.SwitchActiveWorkspace(ctx, user.ID, SessionIDFromContext(r.Context()), workspaceID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	dto := auth.WorkspaceToDTO(wsCtx.Workspace, wsCtx.Role, 0, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{
		"workspace":   dto,
		"role":        wsCtx.Role,
		"permissions": wsCtx.Permissions,
		"plan":        wsCtx.Workspace.Plan,
		"max_users":   wsCtx.Limits.MaxUsers,
	})
}

// ExitWorkspace handles POST /api/v1/workspaces/exit.
func (h *AuthHandler) ExitWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.ExitActiveWorkspace(ctx, user.ID, SessionIDFromContext(r.Context())); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListPendingWorkspaceInvites handles GET /api/v1/workspaces/invites.
func (h *AuthHandler) ListPendingWorkspaceInvites(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	invites, err := h.svc.ListPendingWorkspaceInvites(ctx, user.ID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
}

// AcceptWorkspaceInvite handles POST /api/v1/workspaces/{id}/accept.
func (h *AuthHandler) AcceptWorkspaceInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	ws, err := h.svc.AcceptWorkspaceInvite(ctx, user.ID, SessionIDFromContext(r.Context()), workspaceID, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspace": ws})
}

// DeclineWorkspaceInvite handles POST /api/v1/workspaces/{id}/decline.
func (h *AuthHandler) DeclineWorkspaceInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.DeclineWorkspaceInvite(ctx, user.ID, workspaceID, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// LeaveWorkspace handles POST /api/v1/workspaces/{id}/leave.
func (h *AuthHandler) LeaveWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.LeaveWorkspace(ctx, user.ID, SessionIDFromContext(r.Context()), workspaceID, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateWorkspace handles PATCH /api/v1/workspaces/{id}.
func (h *AuthHandler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	ws, err := h.svc.UpdateWorkspace(ctx, workspaceID, user.ID, req.Name, req.Slug, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspace": ws})
}

// DeleteWorkspace handles DELETE /api/v1/workspaces/{id}.
func (h *AuthHandler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.DeleteWorkspace(ctx, workspaceID, user.ID, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// TransferWorkspaceOwnership handles POST /api/v1/workspaces/{id}/transfer-ownership.
func (h *AuthHandler) TransferWorkspaceOwnership(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	targetUserID, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	ws, err := h.svc.TransferWorkspaceOwnership(ctx, workspaceID, user.ID, targetUserID, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspace": ws})
}

// PlatformListWorkspaces handles GET /api/v1/platform/workspaces.
func (h *AuthHandler) PlatformListWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	workspaces, err := h.svc.ListAllWorkspacesPlatform(ctx, user.ID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

// PlatformUpdateWorkspacePlan handles PATCH /api/v1/platform/workspaces/{id}/plan.
func (h *AuthHandler) PlatformUpdateWorkspacePlan(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	var req updateWorkspacePlanRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.UpdateWorkspacePlanAndOverrides(
		ctx,
		user.ID,
		workspaceID,
		req.Plan,
		req.MaxUsersOverride,
		req.MaxProductsOverride,
		req.MaxLicensesOverride,
		req.OverrideExpiresAt,
		req.OverrideNote,
		requestMeta(r),
	)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspace": updated})
}

// PlatformSuspendWorkspace handles POST /api/v1/platform/workspaces/{id}/suspend.
func (h *AuthHandler) PlatformSuspendWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	var req adminSuspendUserRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	trimmedReason := strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(trimmedReason) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  auth.ErrInvalidSuspendReason.Error(),
			"fields": map[string]string{"reason": auth.ErrInvalidSuspendReason.Error()},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.SuspendWorkspacePlatform(ctx, user.ID, workspaceID, trimmedReason, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspace": updated})
}

// PlatformActivateWorkspace handles POST /api/v1/platform/workspaces/{id}/activate.
func (h *AuthHandler) PlatformActivateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.ActivateWorkspacePlatform(ctx, user.ID, workspaceID, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"workspace": updated})
}

// PlatformListUsers handles GET /api/v1/platform/users.
func (h *AuthHandler) PlatformListUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	users, err := h.svc.ListPlatformUsers(ctx, user.ID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// PlatformSuspendUser handles POST /api/v1/platform/users/{id}/suspend.
func (h *AuthHandler) PlatformSuspendUser(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req adminSuspendUserRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	trimmedReason := strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(trimmedReason) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  auth.ErrInvalidSuspendReason.Error(),
			"fields": map[string]string{"reason": auth.ErrInvalidSuspendReason.Error()},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.PlatformSuspendUser(ctx, user.ID, targetID, trimmedReason, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// PlatformActivateUser handles POST /api/v1/platform/users/{id}/activate.
func (h *AuthHandler) PlatformActivateUser(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.PlatformActivateUser(ctx, user.ID, targetID, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// PlatformUserSuspendImpact handles GET /api/v1/platform/users/{id}/suspend-impact.
func (h *AuthHandler) PlatformUserSuspendImpact(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	impact, err := h.svc.GetGlobalSuspendImpact(ctx, user.ID, targetID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, impact)
}
