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
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"riralicense/internal/auth"
)

type adminCreateUserRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
	Name  string `json:"name" validate:"max=120"`
	Role  string `json:"role" validate:"required,max=32"`
}

type adminUpdateRoleRequest struct {
	Role string `json:"role" validate:"required,max=32"`
}

type adminSuspendUserRequest struct {
	Reason string `json:"reason"`
}

type adminRemoveUserRequest struct {
	Reason string `json:"reason" validate:"max=500"`
}

// AdminListUsers handles GET /api/v1/admin/users.
func (h *AuthHandler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	var users []auth.AdminUserDTO
	var err error
	if wsCtx, ok := WorkspaceFromContext(r.Context()); ok {
		users, err = h.svc.ListUsersInWorkspace(ctx, wsCtx.Workspace.ID)
	} else {
		users, err = h.svc.ListUsers(ctx)
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// AdminCreateUser handles POST /api/v1/admin/users.
func (h *AuthHandler) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req adminCreateUserRequest
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

	var created *auth.AdminUserDTO
	var err error
	if wsCtx, hasWS := WorkspaceFromContext(r.Context()); hasWS {
		created, err = h.svc.AdminCreateUserInWorkspace(ctx, wsCtx.Workspace.ID, user.ID, req.Email, req.Name, req.Role, requestMeta(r))
	} else {
		created, err = h.svc.AdminCreateUser(ctx, user.ID, req.Email, req.Name, req.Role, requestMeta(r))
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	msg := "user invited; verification link sent to email"
	if created.Status == "pending" {
		msg = "workspace invitation sent to existing user"
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": msg,
		"user":    created,
	})
}

// AdminUpdateUserRole handles PATCH /api/v1/admin/users/{id}/role.
func (h *AuthHandler) AdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
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

	var req adminUpdateRoleRequest
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

	var updated *auth.AdminUserDTO
	if wsCtx, hasWS := WorkspaceFromContext(r.Context()); hasWS {
		updated, err = h.svc.AdminUpdateUserRoleInWorkspace(ctx, wsCtx.Workspace.ID, user.ID, targetID, req.Role, requestMeta(r))
	} else {
		updated, err = h.svc.AdminUpdateUserRole(ctx, user.ID, targetID, req.Role, requestMeta(r))
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// AdminSuspendUser handles POST /api/v1/admin/users/{id}/suspend.
func (h *AuthHandler) AdminSuspendUser(w http.ResponseWriter, r *http.Request) {
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

	var updated *auth.AdminUserDTO
	if wsCtx, hasWS := WorkspaceFromContext(r.Context()); hasWS {
		updated, err = h.svc.AdminSuspendUserInWorkspace(ctx, wsCtx.Workspace.ID, user.ID, targetID, trimmedReason, requestMeta(r))
	} else {
		updated, err = h.svc.AdminSuspendUser(ctx, user.ID, targetID, trimmedReason, requestMeta(r))
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// AdminActivateUser handles POST /api/v1/admin/users/{id}/activate.
func (h *AuthHandler) AdminActivateUser(w http.ResponseWriter, r *http.Request) {
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

	var updated *auth.AdminUserDTO
	if wsCtx, hasWS := WorkspaceFromContext(r.Context()); hasWS {
		updated, err = h.svc.AdminActivateUserInWorkspace(ctx, wsCtx.Workspace.ID, user.ID, targetID, requestMeta(r))
	} else {
		updated, err = h.svc.AdminActivateUser(ctx, user.ID, targetID, requestMeta(r))
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// AdminUpdateUser handles PATCH /api/v1/admin/users/{id}.
func (h *AuthHandler) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
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

	if wsCtx, hasWS := WorkspaceFromContext(r.Context()); hasWS {
		err = h.svc.AdminUpdateUserCheckInWorkspace(ctx, wsCtx.Workspace.ID, user.ID, targetID)
	} else {
		err = h.svc.AdminUpdateUserCheck(ctx, user.ID, targetID)
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AdminRemoveUser handles DELETE /api/v1/admin/users/{id}.
func (h *AuthHandler) AdminRemoveUser(w http.ResponseWriter, r *http.Request) {
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

	reason := strings.TrimSpace(r.URL.Query().Get("reason"))
	if r.Body != nil && r.ContentLength != 0 {
		var req adminRemoveUserRequest
		if fields, decErr := decodeAndValidate(r, h.validate, &req); decErr != nil {
			if fields != nil {
				writeValidationError(w, fields)
				return
			}
			writeError(w, http.StatusBadRequest, "invalid request payload")
			return
		}
		if strings.TrimSpace(req.Reason) != "" {
			reason = strings.TrimSpace(req.Reason)
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if wsCtx, hasWS := WorkspaceFromContext(r.Context()); hasWS {
		err = h.svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsCtx.Workspace.ID, user.ID, targetID, reason, requestMeta(r))
	} else {
		err = h.svc.AdminRemoveUser(ctx, user.ID, targetID, requestMeta(r))
	}
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListProducts handles GET /api/v1/products.
func (h *AuthHandler) ListProducts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"products": []any{}})
}

// CreateProduct handles POST /api/v1/products and enforces workspace product limits (bypassed for superadmin).
func (h *AuthHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	var actorID, wsID uuid.UUID
	if user, ok := UserFromContext(r.Context()); ok {
		actorID = user.ID
	}
	if wsCtx, ok := WorkspaceFromContext(r.Context()); ok {
		wsID = wsCtx.Workspace.ID
	}

	if err := h.svc.CreateProductInWorkspace(ctx, actorID, wsID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

// GetProduct handles GET /api/v1/products/{id}.
func (h *AuthHandler) GetProduct(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"product": map[string]any{}})
}

// UpdateProduct handles PATCH/PUT /api/v1/products/{id}.
func (h *AuthHandler) UpdateProduct(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

// DeleteProduct handles DELETE /api/v1/products/{id}.
func (h *AuthHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	var wsID uuid.UUID
	if wsCtx, ok := WorkspaceFromContext(r.Context()); ok {
		wsID = wsCtx.Workspace.ID
	}
	if err := h.svc.DeleteProductInWorkspace(ctx, wsID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ListLicenses handles GET /api/v1/licenses.
func (h *AuthHandler) ListLicenses(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"licenses": []any{}})
}

// CreateLicense handles POST /api/v1/licenses and enforces workspace license limits (bypassed for superadmin).
func (h *AuthHandler) CreateLicense(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	var actorID, wsID uuid.UUID
	if user, ok := UserFromContext(r.Context()); ok {
		actorID = user.ID
	}
	if wsCtx, ok := WorkspaceFromContext(r.Context()); ok {
		wsID = wsCtx.Workspace.ID
	}

	if err := h.svc.CreateLicenseInWorkspace(ctx, actorID, wsID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

// GetLicense handles GET /api/v1/licenses/{id}.
func (h *AuthHandler) GetLicense(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"license": map[string]any{}})
}

// UpdateLicense handles PATCH/PUT /api/v1/licenses/{id}.
func (h *AuthHandler) UpdateLicense(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

// RevokeLicense handles POST /api/v1/licenses/{id}/revoke.
func (h *AuthHandler) RevokeLicense(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

// DeleteLicense handles DELETE /api/v1/licenses/{id}.
func (h *AuthHandler) DeleteLicense(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	var wsID uuid.UUID
	if wsCtx, ok := WorkspaceFromContext(r.Context()); ok {
		wsID = wsCtx.Workspace.ID
	}
	if err := h.svc.DeleteLicenseInWorkspace(ctx, wsID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ListActivations handles GET /api/v1/activations.
func (h *AuthHandler) ListActivations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"activations": []any{}})
}

// CreateActivation handles POST /api/v1/activations.
func (h *AuthHandler) CreateActivation(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{"status": "created"})
}

// RevokeActivation handles POST /api/v1/activations/{id}/revoke and DELETE /api/v1/activations/{id}.
func (h *AuthHandler) RevokeActivation(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

// GetSettings handles GET /api/v1/settings.
func (h *AuthHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if wsCtx, ok := WorkspaceFromContext(r.Context()); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"plan":      wsCtx.Workspace.Plan,
			"max_users": wsCtx.Limits.MaxUsers,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":      h.svc.Plan(),
		"max_users": h.svc.MaxUsers(),
	})
}

// UpdateSettings handles PATCH/PUT /api/v1/settings.
func (h *AuthHandler) UpdateSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
}

// ListAuditLogs handles GET /api/v1/audit.
func (h *AuthHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	wsCtx, ok := WorkspaceFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	events, err := h.svc.ListWorkspaceAuditLogs(ctx, wsCtx.Workspace.ID, 100)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
