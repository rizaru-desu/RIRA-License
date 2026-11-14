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

	return r
}
