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

package auth

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/db/sqlc"
)

// UpdateProfileName trims and validates the display name (2-100 chars), updates the user record,
// and logs a profile_updated audit event.
func (s *Service) UpdateProfileName(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	meta RequestMeta,
) (*UserDTO, error) {
	trimmed := strings.TrimSpace(name)
	runeCount := utf8.RuneCountInString(trimmed)
	if runeCount < MinDisplayNameLength || runeCount > MaxDisplayNameLength {
		return nil, ErrInvalidDisplayName
	}

	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.Status == "suspended" || user.DeletedAt != nil {
		return nil, ErrUnauthorized
	}

	updated, err := s.store.Queries().UpdateUserName(ctx, sqlc.UpdateUserNameParams{
		ID:   userID,
		Name: trimmed,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, "profile_updated", meta, map[string]string{
		"field": "name",
	})

	dto := userToDTO(updated)
	return &dto, nil
}

// UpdateAvatar validates avatar bytes by MIME sniffing (JPEG, PNG, WebP only, max 2 MB),
// crops/resizes decodable images to a 256x256 square, writes the image to the persistent
// upload directory with a random UUID filename, stores the relative path in users.avatar_url,
// removes the previous avatar file if present, and logs an audit event.
func (s *Service) UpdateAvatar(
	ctx context.Context,
	userID uuid.UUID,
	fileBytes []byte,
	meta RequestMeta,
) (*UserDTO, error) {
	if len(fileBytes) == 0 {
		return nil, ErrInvalidAvatarType
	}
	if len(fileBytes) > MaxAvatarBytes {
		return nil, ErrAvatarTooLarge
	}

	ext, ok := sniffAvatarExtension(fileBytes)
	if !ok {
		return nil, ErrInvalidAvatarType
	}

	processedBytes, finalExt := resizeAvatarToSquare(fileBytes, ext, AvatarTargetDimension)

	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.Status == "suspended" || user.DeletedAt != nil {
		return nil, ErrUnauthorized
	}

	var oldAvatarURL string
	if user.AvatarUrl.Valid {
		oldAvatarURL = user.AvatarUrl.String
	}

	avatarsDir, err := s.ensureAvatarsDir()
	if err != nil {
		return nil, err
	}

	filename := uuid.NewString() + finalExt
	destPath := filepath.Join(avatarsDir, filename)
	if err := os.WriteFile(destPath, processedBytes, 0o644); err != nil {
		return nil, err
	}

	newAvatarURL := AvatarURLPrefix + filename
	updated, err := s.store.Queries().UpdateUserAvatar(ctx, sqlc.UpdateUserAvatarParams{
		ID:        userID,
		AvatarUrl: pgText(newAvatarURL),
	})
	if err != nil {
		_ = os.Remove(destPath)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	if oldAvatarURL != "" && oldAvatarURL != newAvatarURL {
		s.removeAvatarFileIfLocal(oldAvatarURL)
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, "avatar_updated", meta, map[string]string{
		"avatar_url": newAvatarURL,
	})

	dto := userToDTO(updated)
	return &dto, nil
}

// DeleteAvatar removes the user's current avatar file from disk (if locally stored),
// sets users.avatar_url to NULL, and logs an audit event.
func (s *Service) DeleteAvatar(
	ctx context.Context,
	userID uuid.UUID,
	meta RequestMeta,
) (*UserDTO, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.Status == "suspended" || user.DeletedAt != nil {
		return nil, ErrUnauthorized
	}

	var oldAvatarURL string
	if user.AvatarUrl.Valid {
		oldAvatarURL = user.AvatarUrl.String
	}

	updated, err := s.store.Queries().UpdateUserAvatar(ctx, sqlc.UpdateUserAvatarParams{
		ID:        userID,
		AvatarUrl: pgtype.Text{},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	if oldAvatarURL != "" {
		s.removeAvatarFileIfLocal(oldAvatarURL)
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, "avatar_removed", meta, nil)

	dto := userToDTO(updated)
	return &dto, nil
}

func (s *Service) ensureAvatarsDir() (string, error) {
	if filepath.Clean(s.uploadDir) == "uploads" {
		if info, err := os.Stat(filepath.Join("..", ".git")); err == nil && info.IsDir() {
			rootUploads := filepath.Join("..", "uploads")
			if err := os.MkdirAll(filepath.Join(rootUploads, "avatars"), 0o755); err != nil {
				return "", err
			}
			if _, lerr := os.Lstat("uploads"); errors.Is(lerr, os.ErrNotExist) {
				_ = os.Symlink(rootUploads, "uploads")
			}
		}
	}
	avatarsDir := filepath.Join(s.uploadDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0o755); err != nil {
		return "", err
	}
	return avatarsDir, nil
}

func (s *Service) removeAvatarFileIfLocal(avatarURL string) {
	filename, ok := extractLocalAvatarFilename(avatarURL)
	if !ok {
		return
	}
	fullPath := filepath.Join(s.uploadDir, "avatars", filename)
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("failed to remove old avatar file", "file", filename, "err", err)
	}
}

func extractLocalAvatarFilename(avatarURL string) (string, bool) {
	trimmed := strings.TrimSpace(avatarURL)
	if trimmed == "" || strings.HasPrefix(trimmed, "data:") {
		return "", false
	}
	pathPart := trimmed
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		if u, err := url.Parse(trimmed); err == nil {
			pathPart = u.Path
		}
	}
	if !strings.HasPrefix(pathPart, AvatarURLPrefix) {
		return "", false
	}
	filename := strings.TrimPrefix(pathPart, AvatarURLPrefix)
	if !IsValidAvatarFilename(filename) {
		return "", false
	}
	return filename, true
}

func normalizeAvatarRelativePath(avatarURL string) string {
	if filename, ok := extractLocalAvatarFilename(avatarURL); ok {
		return AvatarURLPrefix + filename
	}
	return avatarURL
}

// IsValidAvatarFilename checks that filename is a flat UUID-based image filename with no path traversal.
func IsValidAvatarFilename(filename string) bool {
	if filename == "" || filename == "." || filename == ".." {
		return false
	}
	if filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\`) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return false
	}
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	_, err := uuid.Parse(stem)
	return err == nil
}

func sniffAvatarExtension(data []byte) (string, bool) {
	if len(data) < 12 {
		return "", false
	}
	if bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return ".webp", true
	}
	mimeType := http.DetectContentType(data)
	switch mimeType {
	case "image/jpeg":
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return ".jpg", true
		}
	case "image/png":
		if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
			return ".png", true
		}
	case "image/webp":
		return ".webp", true
	}
	return "", false
}

// resizeAvatarToSquare center-crops and resizes decodable images (JPEG/PNG) to targetSize x targetSize.
// Since no WebP encoder library is in go.mod, the original format is preserved on the server.
func resizeAvatarToSquare(data []byte, ext string, targetSize int) ([]byte, string) {
	if targetSize <= 0 || (ext != ".png" && ext != ".jpg" && ext != ".jpeg") {
		return data, ext
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, ext
	}

	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return data, ext
	}

	if w == targetSize && h == targetSize {
		return data, ext
	}

	side := w
	if h < side {
		side = h
	}
	cropMinX := bounds.Min.X + (w-side)/2
	cropMinY := bounds.Min.Y + (h-side)/2

	dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
	for y := 0; y < targetSize; y++ {
		srcY := cropMinY + (y*side)/targetSize
		if srcY >= cropMinY+side {
			srcY = cropMinY + side - 1
		}
		for x := 0; x < targetSize; x++ {
			srcX := cropMinX + (x*side)/targetSize
			if srcX >= cropMinX+side {
				srcX = cropMinX + side - 1
			}
			r, g, b, a := src.At(srcX, srcY).RGBA()
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}

	var out bytes.Buffer
	switch ext {
	case ".png":
		if err := png.Encode(&out, dst); err != nil {
			return data, ext
		}
	case ".jpg", ".jpeg":
		if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 90}); err != nil {
			return data, ".jpg"
		}
		ext = ".jpg"
	}

	if out.Len() == 0 || out.Len() > MaxAvatarBytes {
		return data, ext
	}
	return out.Bytes(), ext
}
