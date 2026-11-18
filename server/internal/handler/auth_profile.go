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
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"riralicense/internal/auth"
)

type updateProfileRequest struct {
	Name *string `json:"name"`
}

// UpdateProfile handles PATCH /api/v1/users/me to update the authenticated user's display name.
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	acctKey := "acct:" + user.ID.String()
	if allowed, retry := h.limiters.ProfileUpdate.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()

	var req updateProfileRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if req.Name == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  "name is required",
			"fields": map[string]string{"name": "name is required"},
		})
		return
	}

	trimmed := strings.TrimSpace(*req.Name)
	runeCount := utf8.RuneCountInString(trimmed)
	if runeCount == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  "name is required",
			"fields": map[string]string{"name": "name is required"},
		})
		return
	}
	if runeCount < auth.MinDisplayNameLength || runeCount > auth.MaxDisplayNameLength {
		msg := auth.ErrInvalidDisplayName.Error()
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  msg,
			"fields": map[string]string{"name": msg},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.UpdateProfileName(ctx, user.ID, trimmed, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// UploadAvatar handles POST /api/v1/users/me/avatar (multipart/form-data).
func (h *AuthHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	acctKey := "acct:" + user.ID.String()
	if allowed, retry := h.limiters.Avatar.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	const multipartOverhead = 64 << 10
	r.Body = http.MaxBytesReader(w, r.Body, auth.MaxAvatarBytes+multipartOverhead)
	if err := r.ParseMultipartForm(auth.MaxAvatarBytes + multipartOverhead); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusBadRequest, auth.ErrAvatarTooLarge.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form data")
		return
	}
	if r.MultipartForm != nil {
		defer func() {
			_ = r.MultipartForm.RemoveAll()
		}()
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar file is required")
		return
	}
	defer file.Close()

	if header != nil && header.Size > auth.MaxAvatarBytes {
		writeError(w, http.StatusBadRequest, auth.ErrAvatarTooLarge.Error())
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, auth.MaxAvatarBytes+1))
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusBadRequest, auth.ErrAvatarTooLarge.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "failed to read avatar file")
		return
	}
	if len(data) > auth.MaxAvatarBytes {
		writeError(w, http.StatusBadRequest, auth.ErrAvatarTooLarge.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.UpdateAvatar(ctx, user.ID, data, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// DeleteAvatar handles DELETE /api/v1/users/me/avatar.
func (h *AuthHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ipKey := "ip:" + extractClientIPString(r)
	acctKey := "acct:" + user.ID.String()
	if allowed, retry := h.limiters.Avatar.Allow(ipKey, acctKey); !allowed {
		writeTooManyRequests(w, retry)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.HTTPTimeout)
	defer cancel()

	updated, err := h.svc.DeleteAvatar(ctx, user.ID, requestMeta(r))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

// ServeAvatar serves uploaded avatar images from the persistent upload directory.
func (h *AuthHandler) ServeAvatar(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if !auth.IsValidAvatarFilename(filename) {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(h.svc.UploadDir(), "avatars", filename)
	f, err := os.Open(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, filename, info.ModTime(), f)
}
