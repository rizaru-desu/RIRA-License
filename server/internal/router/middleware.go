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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"riralicense/internal/auth"
	"riralicense/internal/config"
	"riralicense/internal/handler"
)

// RequireAuth rejects requests without a valid, active DB session.
func RequireAuth(svc *auth.Service, cfg *config.Config) func(http.Handler) http.Handler {
	return authMiddleware(svc, cfg, true)
}

// OptionalAuth attaches the authenticated user when a valid session cookie exists.
func OptionalAuth(svc *auth.Service, cfg *config.Config) func(http.Handler) http.Handler {
	return authMiddleware(svc, cfg, false)
}

func authMiddleware(svc *auth.Service, cfg *config.Config, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(handler.SessionCookieName)
			if err != nil || cookie.Value == "" {
				if required {
					writeUnauthorized(w)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), cfg.HTTPTimeout)
			defer cancel()

			user, sessionID, err := svc.ValidateSession(ctx, cookie.Value)
			if err != nil || user == nil || user.Status != "active" {
				http.SetCookie(w, &http.Cookie{
					Name:     handler.SessionCookieName,
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					Expires:  time.Unix(0, 0),
					HttpOnly: true,
					Secure:   cfg.CookieSecure,
					SameSite: http.SameSiteLaxMode,
				})
				if required {
					writeUnauthorized(w)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			nextCtx := handler.ContextWithUser(r.Context(), user)
			nextCtx = handler.ContextWithSessionID(nextCtx, sessionID)
			next.ServeHTTP(w, r.WithContext(nextCtx))
		})
	}
}

// RequireWorkspace resolves the active workspace (from X-Workspace-ID or the session's active workspace)
// and updates the request user context with their workspace-scoped role and permissions.
func RequireWorkspace(svc *auth.Service, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := handler.UserFromContext(r.Context())
			if !ok || user == nil || user.Status != "active" {
				writeUnauthorized(w)
				return
			}

			var requestedWS *uuid.UUID
			if rawWS := strings.TrimSpace(r.Header.Get("X-Workspace-ID")); rawWS != "" {
				parsed, err := uuid.Parse(rawWS)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid workspace id")
					return
				}
				requestedWS = &parsed
			}

			ctx, cancel := context.WithTimeout(r.Context(), cfg.HTTPTimeout)
			defer cancel()

			wsCtx, err := svc.ResolveWorkspaceContext(ctx, user, requestedWS)
			if err != nil {
				switch {
				case errors.Is(err, auth.ErrWorkspaceNotFound):
					writeJSONError(w, http.StatusNotFound, auth.ErrWorkspaceNotFound.Error())
				case errors.Is(err, auth.ErrWorkspaceSuspended):
					writeJSONError(w, http.StatusForbidden, auth.ErrWorkspaceSuspended.Error())
				case errors.Is(err, auth.ErrForbidden):
					writeForbidden(w)
				case errors.Is(err, auth.ErrUnauthorized):
					writeUnauthorized(w)
				default:
					writeJSONError(w, http.StatusInternalServerError, "internal server error")
				}
				return
			}

			scopedUser := *user
			scopedUser.Role = wsCtx.Role
			scopedUser.Permissions = wsCtx.Permissions
			wsID := wsCtx.Workspace.ID
			scopedUser.ActiveWorkspaceID = &wsID

			nextCtx := handler.ContextWithUser(r.Context(), &scopedUser)
			nextCtx = handler.ContextWithWorkspace(nextCtx, wsCtx)
			next.ServeHTTP(w, r.WithContext(nextCtx))
		})
	}
}

// RequireSuperadmin enforces that the authenticated user holds platform superadmin privileges.
func RequireSuperadmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := handler.UserFromContext(r.Context())
			if !ok || user == nil || user.Status != "active" {
				writeUnauthorized(w)
				return
			}
			if !user.IsSuperadmin {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission enforces that the authenticated user is active and holds the given permission.
// It must be applied after RequireAuth. Missing permission always returns 403 {"error":"forbidden"}.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := handler.UserFromContext(r.Context())
			if !ok || user == nil || user.Status != "active" {
				writeUnauthorized(w)
				return
			}
			if !auth.HasPermission(user.Role, user.IsSuperadmin, perm) {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS restricts cross-origin credentialed requests to configured origins.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Workspace-ID")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds baseline defensive HTTP headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

func writeForbidden(w http.ResponseWriter) {
	writeJSONError(w, http.StatusForbidden, "forbidden")
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
