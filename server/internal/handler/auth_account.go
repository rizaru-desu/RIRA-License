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
)

type accountDeleteRequest struct {
	Password   string `json:"password" validate:"omitempty,max=128"`
	EmailOTP   string `json:"email_otp" validate:"omitempty,max=32"`
	Code       string `json:"code" validate:"omitempty,max=32"`
	BackupCode string `json:"backup_code" validate:"omitempty,max=32"`
}

func (h *AuthHandler) RequestAccountDeletion(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req accountDeleteRequest
	if fields, err := decodeAndValidate(r, h.validate, &req); err != nil {
		if fields != nil {
			writeValidationError(w, fields)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	acctKey := "acct:" + user.ID.String()
	if allowed, retry := h.limiters.AccountDelete.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.RequestAccountDeletion(
		ctx,
		user.ID,
		req.Password,
		req.EmailOTP,
		req.Code,
		req.BackupCode,
		requestMeta(r),
	); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "if the request is valid, a confirmation link has been sent to your email",
	})
}

func (h *AuthHandler) ConfirmAccountDeletion(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
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

	if err := h.svc.ConfirmAccountDeletion(ctx, req.Token, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.clearCookie(w, SessionCookieName, "/")
	h.clearCookie(w, TrustedDeviceCookieName, "/")
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "account scheduled for deletion; you can restore it within 30 days",
	})
}

func (h *AuthHandler) CancelAccountDeletion(w http.ResponseWriter, r *http.Request) {
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
	if allowed, retry := h.limiters.AccountDelete.Allow(ipKey, emailKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	if err := h.svc.CancelAccountDeletion(ctx, req.Email, requestMeta(r)); err != nil {
		h.logger.Error("cancel account deletion failed", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "if a recently deleted account exists for that email, a restore link has been sent",
	})
}

func (h *AuthHandler) RestoreAccount(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
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

	if err := h.svc.RestoreAccount(ctx, req.Token, requestMeta(r)); err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "account has been restored; you can now sign in",
	})
}
