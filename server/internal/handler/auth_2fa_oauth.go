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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"riralicense/internal/auth"
	"riralicense/internal/crypto"
)

type twoFactorVerifyRequest struct {
	Code           string `json:"code" validate:"max=32"`
	BackupCode     string `json:"backup_code" validate:"max=32"`
	RememberDevice bool   `json:"remember_device"`
}

type twoFactorDisableRequest struct {
	Password   string `json:"password" validate:"max=128"`
	Code       string `json:"code" validate:"max=32"`
	BackupCode string `json:"backup_code" validate:"max=32"`
}

type regenerateBackupCodesRequest struct {
	Password string `json:"password" validate:"max=128"`
	Code     string `json:"code" validate:"max=32"`
}

func (h *AuthHandler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	res, err := h.svc.SetupTOTP(ctx, user.ID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *AuthHandler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	h.verifyTwoFactorEndpoint(w, r, auth.MethodTOTP)
}

func (h *AuthHandler) VerifyEmailOTP(w http.ResponseWriter, r *http.Request) {
	h.verifyTwoFactorEndpoint(w, r, auth.MethodEmail)
}

func (h *AuthHandler) verifyTwoFactorEndpoint(w http.ResponseWriter, r *http.Request, method string) {
	var req twoFactorVerifyRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if strings.TrimSpace(req.Code) == "" && strings.TrimSpace(req.BackupCode) == "" {
		writeValidationError(w, map[string]string{"code": "code or backup_code is required"})
		return
	}

	ipKey := "ip:" + extractClientIPString(r)

	// Path 1: Authenticated user verifying initial 2FA setup.
	if user, ok := UserFromContext(r.Context()); ok {
		acctKey := "acct:" + user.ID.String()
		if allowed, retry := h.limiters.Login.Allow(ipKey, acctKey); !allowed {
			writeTooManyRequests(w, retry)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
		defer cancel()

		var backupCodes []string
		var err error
		if method == auth.MethodTOTP {
			backupCodes, err = h.svc.VerifyAndEnableTOTP(ctx, user.ID, req.Code, requestMeta(r))
		} else {
			backupCodes, err = h.svc.VerifyAndEnableEmail2FA(ctx, user.ID, req.Code, requestMeta(r))
		}
		if err != nil {
			h.handleServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"two_factor_enabled": true,
			"method":             method,
			"backup_codes":       backupCodes,
		})
		return
	}

	// Path 2: Pending login 2FA challenge via signed httpOnly cookie.
	challengeUserID, _, valid := h.readTwoFactorChallengeCookie(r)
	if !valid {
		h.clearCookie(w, TwoFactorCookieName, "/api/v1/auth/2fa")
		writeError(w, http.StatusUnauthorized, "two-factor challenge expired or missing")
		return
	}

	acctKey := "acct:" + challengeUserID.String()
	if allowed, retry := h.limiters.Login.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	res, err := h.svc.VerifyTwoFactorLogin(
		ctx,
		challengeUserID,
		method,
		req.Code,
		req.BackupCode,
		req.RememberDevice,
		requestMeta(r),
	)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.clearCookie(w, TwoFactorCookieName, "/api/v1/auth/2fa")
	h.setSessionCookie(w, res.SessionToken, res.SessionExpiresAt)
	if res.TrustedDeviceRaw != "" {
		h.setTrustedDeviceCookie(w, res.TrustedDeviceRaw, res.TrustedDeviceExp)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"two_factor_required": false,
		"user":                res.User,
	})
}

func (h *AuthHandler) SendEmailOTP(w http.ResponseWriter, r *http.Request) {
	ipKey := "ip:" + extractClientIPString(r)

	var targetUserID uuid.UUID
	if user, ok := UserFromContext(r.Context()); ok {
		targetUserID = user.ID
	} else {
		uid, method, valid := h.readTwoFactorChallengeCookie(r)
		if !valid || method != auth.MethodEmail {
			h.clearCookie(w, TwoFactorCookieName, "/api/v1/auth/2fa")
			writeError(w, http.StatusUnauthorized, "two-factor challenge expired or missing")
			return
		}
		targetUserID = uid
	}

	acctKey := "acct:" + targetUserID.String()
	if allowed, retry := h.limiters.Login.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.SendEmailOTP(ctx, targetUserID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "verification code sent",
	})
}

func (h *AuthHandler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req twoFactorDisableRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if strings.TrimSpace(req.Code) == "" && strings.TrimSpace(req.BackupCode) == "" {
		writeValidationError(w, map[string]string{"code": "code or backup_code is required"})
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	acctKey := "acct:" + user.ID.String()
	if allowed, retry := h.limiters.Login.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.DisableTwoFactor(ctx, user.ID, req.Password, req.Code, req.BackupCode, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.clearCookie(w, SessionCookieName, "/")
	h.clearCookie(w, TrustedDeviceCookieName, "/")
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "two-factor authentication disabled; please sign in again",
	})
}

func (h *AuthHandler) RegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req regenerateBackupCodesRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if strings.TrimSpace(req.Password) == "" && strings.TrimSpace(req.Code) == "" {
		writeValidationError(w, map[string]string{"code": "password or current 2fa code is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	codes, err := h.svc.RegenerateBackupCodes(ctx, user.ID, req.Password, req.Code, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"backup_codes": codes,
	})
}

func (h *AuthHandler) Providers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": h.svc.EnabledProviders(),
	})
}

func (h *AuthHandler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	state, _, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		h.logger.Error("failed to generate oauth state", "err", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	authURL, err := h.svc.AuthCodeURL(provider, state)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	exp := time.Now().Add(time.Duration(shortCookieMaxAge) * time.Second).Unix()
	payload := fmt.Sprintf("%s|%s|%d", provider, state, exp)
	signed := crypto.SignPayload(h.cfg.SessionSecret, payload)

	http.SetCookie(w, &http.Cookie{
		Name:     OAuthStateCookieName,
		Value:    signed,
		Path:     "/api/v1/auth/oauth",
		MaxAge:   shortCookieMaxAge,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	defer h.clearCookie(w, OAuthStateCookieName, "/api/v1/auth/oauth")

	provider := chi.URLParam(r, "provider")
	if !h.isProviderEnabled(provider) {
		writeError(w, http.StatusNotFound, "oauth provider is not available")
		return
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, h.cfg.AppURL+"/login?error=oauth_denied", http.StatusFound)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing oauth code or state")
		return
	}

	if !h.validateOAuthStateCookie(r, provider, state) {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	res, err := h.svc.HandleOAuthCallback(ctx, provider, code, requestMeta(r))
	if err != nil {
		h.logger.Warn("oauth callback failed", "provider", provider, "err", err)
		http.Redirect(w, r, h.cfg.AppURL+"/login?error=oauth_failed", http.StatusFound)
		return
	}

	h.setSessionCookie(w, res.SessionToken, res.SessionExpiresAt)
	http.Redirect(w, r, h.cfg.AppURL+"/dashboard", http.StatusFound)
}

func (h *AuthHandler) isProviderEnabled(provider string) bool {
	for _, p := range h.svc.EnabledProviders() {
		if p == provider {
			return true
		}
	}
	return false
}

func (h *AuthHandler) setTwoFactorChallengeCookie(w http.ResponseWriter, userID uuid.UUID, method string) {
	exp := time.Now().Add(time.Duration(shortCookieMaxAge) * time.Second).Unix()
	payload := fmt.Sprintf("%s|%s|%d", userID.String(), method, exp)
	signed := crypto.SignPayload(h.cfg.SessionSecret, payload)

	http.SetCookie(w, &http.Cookie{
		Name:     TwoFactorCookieName,
		Value:    signed,
		Path:     "/api/v1/auth/2fa",
		MaxAge:   shortCookieMaxAge,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) readTwoFactorChallengeCookie(r *http.Request) (uuid.UUID, string, bool) {
	c, err := r.Cookie(TwoFactorCookieName)
	if err != nil || c.Value == "" {
		return uuid.Nil, "", false
	}
	raw, ok := crypto.VerifyPayload(h.cfg.SessionSecret, c.Value)
	if !ok {
		return uuid.Nil, "", false
	}
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return uuid.Nil, "", false
	}
	uid, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, "", false
	}
	expUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return uuid.Nil, "", false
	}
	return uid, parts[1], true
}

func (h *AuthHandler) validateOAuthStateCookie(r *http.Request, provider, state string) bool {
	c, err := r.Cookie(OAuthStateCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	raw, ok := crypto.VerifyPayload(h.cfg.SessionSecret, c.Value)
	if !ok {
		return false
	}
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return false
	}
	if parts[0] != provider || parts[1] != state {
		return false
	}
	expUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return false
	}
	return true
}
