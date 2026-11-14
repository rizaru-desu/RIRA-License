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
	"net/http"
	"slices"
	"time"

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

			user, _, err := svc.ValidateSession(ctx, cookie.Value)
			if err != nil {
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

			next.ServeHTTP(w, r.WithContext(handler.ContextWithUser(r.Context(), user)))
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
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
