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

package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"riralicense/internal/auth"
	"riralicense/internal/config"
	"riralicense/internal/handler"
)

// New builds the application router with all routes wired up.
func New(
	cfg *config.Config,
	health *handler.HealthHandler,
	authHandler *handler.AuthHandler,
	authSvc *auth.Service,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(SecurityHeaders)
	r.Use(CORS(cfg.CORSAllowedOrigins))

	// Health
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("state") != "" && (req.URL.Query().Get("code") != "" || req.URL.Query().Get("error") != "") {
			authHandler.OAuthCallback(w, req)
			return
		}
		health.Liveness(w, req)
	})
	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)

	// Top-level OAuth callback aliases
	r.Get("/callback", authHandler.OAuthCallback)
	r.Get("/auth/callback", authHandler.OAuthCallback)
	r.Get("/auth/{provider}/callback", authHandler.OAuthCallback)
	r.Get("/oauth/callback", authHandler.OAuthCallback)
	r.Get("/oauth/{provider}/callback", authHandler.OAuthCallback)
	r.Get("/api/auth/callback/{provider}", authHandler.OAuthCallback)
	r.Get("/api/oauth/{provider}/callback", authHandler.OAuthCallback)

	r.Route("/api/v1/auth", func(r chi.Router) {
		// Public endpoints
		r.Post("/register", authHandler.Register)
		r.Get("/verify-email", authHandler.VerifyEmail)
		r.Post("/verify-email", authHandler.VerifyEmail)
		r.Post("/resend-verification", authHandler.ResendVerification)
		r.Post("/login", authHandler.Login)
		r.Post("/forgot-password", authHandler.ForgotPassword)
		r.Post("/reset-password", authHandler.ResetPassword)
		r.Get("/email/confirm", authHandler.ConfirmEmailChange)
		r.Post("/email/confirm", authHandler.ConfirmEmailChange)
		r.Post("/account/delete/confirm", authHandler.ConfirmAccountDeletion)
		r.Post("/account/delete/cancel", authHandler.CancelAccountDeletion)
		r.Post("/account/restore", authHandler.RestoreAccount)
		r.Get("/providers", authHandler.Providers)

		// OAuth
		r.Get("/oauth/{provider}", authHandler.OAuthStart)
		r.Get("/oauth/{provider}/callback", authHandler.OAuthCallback)

		// Dual-mode endpoints (authenticated setup OR pending 2FA challenge cookie)
		r.Group(func(r chi.Router) {
			r.Use(OptionalAuth(authSvc, cfg))
			r.Post("/logout", authHandler.Logout)
			r.Post("/2fa/totp/verify", authHandler.VerifyTOTP)
			r.Post("/2fa/email/send", authHandler.SendEmailOTP)
			r.Post("/2fa/email/verify", authHandler.VerifyEmailOTP)
		})

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(authSvc, cfg))
			r.Get("/me", authHandler.Me)
			r.Post("/logout-all", authHandler.LogoutAll)
			r.Post("/email/change", authHandler.RequestEmailChange)
			r.Post("/password/change", authHandler.ChangePassword)
			r.Post("/2fa/totp/setup", authHandler.SetupTOTP)
			r.Post("/2fa/totp/disable", authHandler.DisableTOTP)
			r.Post("/2fa/backup-codes/regenerate", authHandler.RegenerateBackupCodes)
			r.Post("/account/delete/request", authHandler.RequestAccountDeletion)
		})
	})

	r.Route("/api/v1/users", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Patch("/me", authHandler.UpdateProfile)
		r.Post("/me/avatar", authHandler.UploadAvatar)
		r.Delete("/me/avatar", authHandler.DeleteAvatar)

		// User management aliases under /api/v1/users (workspace-scoped)
		r.Group(func(r chi.Router) {
			r.Use(RequireWorkspace(authSvc, cfg))
			r.With(RequirePermission(auth.PermUsersRead)).Get("/", authHandler.AdminListUsers)
			r.With(RequirePermission(auth.PermUsersInvite)).Post("/", authHandler.AdminCreateUser)
			r.With(RequirePermission(auth.PermRolesManage)).Patch("/{id}/role", authHandler.AdminUpdateUserRole)
			r.With(RequirePermission(auth.PermUsersSuspend)).Post("/{id}/suspend", authHandler.AdminSuspendUser)
			r.With(RequirePermission(auth.PermUsersSuspend)).Post("/{id}/activate", authHandler.AdminActivateUser)
			r.With(RequirePermission(auth.PermUsersUpdate)).Patch("/{id}", authHandler.AdminUpdateUser)
			r.With(RequirePermission(auth.PermUsersRemove)).Delete("/{id}", authHandler.AdminRemoveUser)
		})
	})

	// Multi-tenant workspace management (/api/v1/workspaces)
	r.Route("/api/v1/workspaces", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Get("/", authHandler.ListWorkspaces)
		r.Post("/", authHandler.CreateWorkspace)
		r.Patch("/{id}", authHandler.UpdateWorkspace)
		r.Delete("/{id}", authHandler.DeleteWorkspace)
		r.Get("/invites", authHandler.ListPendingWorkspaceInvites)
		r.Post("/exit", authHandler.ExitWorkspace)
		r.Post("/{id}/switch", authHandler.SwitchWorkspace)
		r.Post("/{id}/accept", authHandler.AcceptWorkspaceInvite)
		r.Post("/{id}/decline", authHandler.DeclineWorkspaceInvite)
		r.Post("/{id}/leave", authHandler.LeaveWorkspace)
		r.Post("/{id}/transfer-ownership", authHandler.TransferWorkspaceOwnership)
	})

	// Platform Super Admin console (/api/v1/platform)
	r.Route("/api/v1/platform", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireSuperadmin())
		r.Get("/workspaces", authHandler.PlatformListWorkspaces)
		r.Patch("/workspaces/{id}/plan", authHandler.PlatformUpdateWorkspacePlan)
		r.Post("/workspaces/{id}/suspend", authHandler.PlatformSuspendWorkspace)
		r.Post("/workspaces/{id}/activate", authHandler.PlatformActivateWorkspace)
		r.Get("/users", authHandler.PlatformListUsers)
		r.Get("/users/{id}/suspend-impact", authHandler.PlatformUserSuspendImpact)
		r.Post("/users/{id}/suspend", authHandler.PlatformSuspendUser)
		r.Post("/users/{id}/activate", authHandler.PlatformActivateUser)
	})

	// Admin user management (/api/v1/admin/users)
	r.Route("/api/v1/admin/users", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermUsersRead)).Get("/", authHandler.AdminListUsers)
		r.With(RequirePermission(auth.PermUsersInvite)).Post("/", authHandler.AdminCreateUser)
		r.With(RequirePermission(auth.PermRolesManage)).Patch("/{id}/role", authHandler.AdminUpdateUserRole)
		r.With(RequirePermission(auth.PermUsersSuspend)).Post("/{id}/suspend", authHandler.AdminSuspendUser)
		r.With(RequirePermission(auth.PermUsersSuspend)).Post("/{id}/activate", authHandler.AdminActivateUser)
		r.With(RequirePermission(auth.PermUsersUpdate)).Patch("/{id}", authHandler.AdminUpdateUser)
		r.With(RequirePermission(auth.PermUsersRemove)).Delete("/{id}", authHandler.AdminRemoveUser)
	})

	// RBAC-protected resource routes (/api/v1/... and /api/v1/admin/...)
	mountResourceRoutes(r, "/api/v1", authSvc, cfg, authHandler)
	mountResourceRoutes(r, "/api/v1/admin", authSvc, cfg, authHandler)

	// Static uploaded avatar serving
	r.Get("/uploads/avatars/{filename}", authHandler.ServeAvatar)
	r.Head("/uploads/avatars/{filename}", authHandler.ServeAvatar)

	return r
}

func mountResourceRoutes(
	r chi.Router,
	prefix string,
	authSvc *auth.Service,
	cfg *config.Config,
	authHandler *handler.AuthHandler,
) {
	r.Route(prefix+"/products", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermProductsRead)).Get("/", authHandler.ListProducts)
		r.With(RequirePermission(auth.PermProductsCreate)).Post("/", authHandler.CreateProduct)
		r.With(RequirePermission(auth.PermProductsRead)).Get("/{id}", authHandler.GetProduct)
		r.With(RequirePermission(auth.PermProductsUpdate)).Patch("/{id}", authHandler.UpdateProduct)
		r.With(RequirePermission(auth.PermProductsUpdate)).Put("/{id}", authHandler.UpdateProduct)
		r.With(RequirePermission(auth.PermProductsDelete)).Delete("/{id}", authHandler.DeleteProduct)
	})

	r.Route(prefix+"/licenses", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermLicensesRead)).Get("/", authHandler.ListLicenses)
		r.With(RequirePermission(auth.PermLicensesCreate)).Post("/", authHandler.CreateLicense)
		r.With(RequirePermission(auth.PermLicensesRead)).Get("/{id}", authHandler.GetLicense)
		r.With(RequirePermission(auth.PermLicensesUpdate)).Patch("/{id}", authHandler.UpdateLicense)
		r.With(RequirePermission(auth.PermLicensesUpdate)).Put("/{id}", authHandler.UpdateLicense)
		r.With(RequirePermission(auth.PermLicensesRevoke)).Post("/{id}/revoke", authHandler.RevokeLicense)
		r.With(RequirePermission(auth.PermLicensesDelete)).Delete("/{id}", authHandler.DeleteLicense)
	})

	r.Route(prefix+"/activations", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermActivationsRead)).Get("/", authHandler.ListActivations)
		r.With(RequirePermission(auth.PermActivationsCreate)).Post("/", authHandler.CreateActivation)
		r.With(RequirePermission(auth.PermActivationsRevoke)).Post("/{id}/revoke", authHandler.RevokeActivation)
		r.With(RequirePermission(auth.PermActivationsRevoke)).Delete("/{id}", authHandler.RevokeActivation)
	})

	r.Route(prefix+"/settings", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermSettingsRead)).Get("/", authHandler.GetSettings)
		r.With(RequirePermission(auth.PermSettingsUpdate)).Patch("/", authHandler.UpdateSettings)
		r.With(RequirePermission(auth.PermSettingsUpdate)).Put("/", authHandler.UpdateSettings)
	})

	r.Route(prefix+"/audit", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermAuditRead)).Get("/", authHandler.ListAuditLogs)
	})

	r.Route(prefix+"/audit-logs", func(r chi.Router) {
		r.Use(RequireAuth(authSvc, cfg))
		r.Use(RequireWorkspace(authSvc, cfg))
		r.With(RequirePermission(auth.PermAuditRead)).Get("/", authHandler.ListAuditLogs)
	})
}
