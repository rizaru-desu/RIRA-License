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
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"riralicense/internal/auth"
	"riralicense/internal/config"
	"riralicense/internal/ratelimit"
)

const (
	SessionCookieName       = "rira_session"
	TrustedDeviceCookieName = "rira_trusted_device"
	OAuthStateCookieName    = "rira_oauth_state"
	TwoFactorCookieName     = "rira_2fa_challenge"
	shortCookieMaxAge       = 600 // 10 minutes
)

type ctxKey string

const userContextKey ctxKey = "auth_user"

// UserFromContext retrieves the authenticated user stored by AuthMiddleware.
func UserFromContext(ctx context.Context) (*auth.UserDTO, bool) {
	u, ok := ctx.Value(userContextKey).(*auth.UserDTO)
	return u, ok && u != nil
}

// ContextWithUser stores an authenticated user on ctx.
func ContextWithUser(ctx context.Context, user *auth.UserDTO) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// AuthHandler serves authentication, session, 2FA, and OAuth HTTP endpoints.
type AuthHandler struct {
	svc      *auth.Service
	cfg      *config.Config
	limiters *ratelimit.AuthLimiters
	validate *validator.Validate
	logger   *slog.Logger
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(
	svc *auth.Service,
	cfg *config.Config,
	limiters *ratelimit.AuthLimiters,
	logger *slog.Logger,
) *AuthHandler {
	return &AuthHandler{
		svc:      svc,
		cfg:      cfg,
		limiters: limiters,
		validate: newValidator(),
		logger:   logger,
	}
}

type registerRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=12,max=128"`
	Name     string `json:"name" validate:"required,min=1,max=120"`
}

type tokenRequest struct {
	Token string `json:"token" validate:"required,min=10,max=256"`
}

type emailOnlyRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=1,max=128"`
}

type resetPasswordRequest struct {
	Token       string `json:"token" validate:"required,min=10,max=256"`
	NewPassword string `json:"new_password" validate:"required,min=12,max=128"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"max=128"`
	NewPassword     string `json:"new_password" validate:"required,min=12,max=128"`
}

type changeEmailRequest struct {
	NewEmail        string `json:"new_email" validate:"required,email,max=254"`
	CurrentPassword string `json:"current_password" validate:"max=128"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ipKey := "ip:" + extractClientIPString(r)
	if allowed, retry := h.limiters.Register.Allow(ipKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	var req registerRequest
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

	user, err := h.svc.Register(ctx, req.Email, req.Password, req.Name, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "registration successful; please check your email to verify your account",
		"user":    user,
	})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var rawToken string
	if r.Method == http.MethodGet {
		rawToken = strings.TrimSpace(r.URL.Query().Get("token"))
		if rawToken == "" {
			writeError(w, http.StatusBadRequest, "token is required")
			return
		}
	} else {
		var req tokenRequest
		if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
			if fields != nil {
				writeValidationError(w, fields)
				return
			}
			writeError(w, http.StatusBadRequest, "invalid request payload")
			return
		}
		rawToken = req.Token
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.VerifyEmail(ctx, rawToken, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "email verified; you can now sign in",
	})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req emailOnlyRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	emailKey := "email:" + strings.ToLower(strings.TrimSpace(req.Email))
	if allowed, retry := h.limiters.ForgotPassword.Allow(ipKey, emailKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.ResendVerification(ctx, req.Email, requestMeta(r)); err != nil {
		h.logger.Error("resend verification failed", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "if an unverified account exists for that email, a verification link has been sent",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	emailKey := "email:" + strings.ToLower(strings.TrimSpace(req.Email))
	if allowed, retry := h.limiters.Login.Allow(ipKey, emailKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	var trustedDevice string
	if c, err := r.Cookie(TrustedDeviceCookieName); err == nil {
		trustedDevice = c.Value
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	res, err := h.svc.Login(ctx, req.Email, req.Password, trustedDevice, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	if res.TwoFactorRequired {
		h.setTwoFactorChallengeCookie(w, res.ChallengeUserID, res.TwoFactorMethod)
		writeJSON(w, http.StatusOK, map[string]any{
			"two_factor_required": true,
			"method":              res.TwoFactorMethod,
		})
		return
	}

	h.setSessionCookie(w, res.SessionToken, res.SessionExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"two_factor_required": false,
		"user":                res.User,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	var userID uuid.UUID
	if user != nil {
		userID = user.ID
	}

	var rawSession string
	if c, err := r.Cookie(SessionCookieName); err == nil {
		rawSession = c.Value
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.Logout(ctx, rawSession, userID, requestMeta(r)); err != nil {
		h.logger.Error("logout error", "err", err)
		h.clearCookie(w, SessionCookieName, "/")
		writeError(w, http.StatusInternalServerError, "failed to sign out")
		return
	}

	h.clearCookie(w, SessionCookieName, "/")
	writeJSON(w, http.StatusOK, map[string]string{"message": "signed out"})
}

func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.LogoutAll(ctx, user.ID, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.clearCookie(w, SessionCookieName, "/")
	writeJSON(w, http.StatusOK, map[string]string{"message": "all sessions revoked"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req emailOnlyRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	emailKey := "email:" + strings.ToLower(strings.TrimSpace(req.Email))
	if allowed, retry := h.limiters.ForgotPassword.Allow(ipKey, emailKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.ForgotPassword(ctx, req.Email, requestMeta(r)); err != nil {
		h.logger.Error("forgot password failed", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "if an account exists for that email, password reset instructions have been sent",
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
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

	if err := h.svc.ResetPassword(ctx, req.Token, req.NewPassword, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.clearCookie(w, SessionCookieName, "/")
	h.clearCookie(w, TrustedDeviceCookieName, "/")
	writeJSON(w, http.StatusOK, map[string]string{"message": "password has been reset"})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req changePasswordRequest
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

	if err := h.svc.ChangePassword(ctx, user.ID, req.CurrentPassword, req.NewPassword, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.clearCookie(w, SessionCookieName, "/")
	h.clearCookie(w, TrustedDeviceCookieName, "/")
	writeJSON(w, http.StatusOK, map[string]string{"message": "password updated; please sign in again"})
}

func (h *AuthHandler) RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req changeEmailRequest
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

	if err := h.svc.RequestEmailChange(ctx, user.ID, req.NewEmail, req.CurrentPassword, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "confirmation link sent to the new email address",
	})
}

func (h *AuthHandler) ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	var rawToken string
	if r.Method == http.MethodGet {
		rawToken = strings.TrimSpace(r.URL.Query().Get("token"))
		if rawToken == "" {
			writeError(w, http.StatusBadRequest, "token is required")
			return
		}
	} else {
		var req tokenRequest
		if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
			if fields != nil {
				writeValidationError(w, fields)
				return
			}
			writeError(w, http.StatusBadRequest, "invalid request payload")
			return
		}
		rawToken = req.Token
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.ConfirmEmailChange(ctx, rawToken, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "email address updated"})
}

func (h *AuthHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, auth.ErrEmailNotVerified):
		writeErrorCode(w, http.StatusForbidden, "email_not_verified", "please verify your email address before signing in")
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, auth.ErrInvalidToken):
		writeError(w, http.StatusBadRequest, "invalid or expired token")
	case errors.Is(err, auth.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, "password must be at least 12 characters")
	case errors.Is(err, auth.ErrPasswordReused):
		writeError(w, http.StatusBadRequest, "new password cannot match any of your last 5 passwords")
	case errors.Is(err, auth.ErrConflictEmail):
		writeError(w, http.StatusConflict, "unable to use that email address")
	case errors.Is(err, auth.ErrTwoFactorAlreadyEnabled):
		writeError(w, http.StatusConflict, "two-factor authentication is already enabled")
	case errors.Is(err, auth.ErrTwoFactorNotEnabled):
		writeError(w, http.StatusBadRequest, "two-factor authentication is not enabled")
	case errors.Is(err, auth.ErrTwoFactorNotPending):
		writeError(w, http.StatusBadRequest, "two-factor setup has not been started")
	case errors.Is(err, auth.ErrProviderDisabled):
		writeError(w, http.StatusNotFound, "oauth provider is not available")
	case errors.Is(err, auth.ErrOAuthEmailUnavailable):
		writeError(w, http.StatusBadRequest, "a verified primary email is required from the oauth provider")
	case errors.Is(err, auth.ErrOAuthExchange):
		writeError(w, http.StatusBadGateway, "oauth authentication failed")
	default:
		h.logger.Error("auth service error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) setTrustedDeviceCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     TrustedDeviceCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
