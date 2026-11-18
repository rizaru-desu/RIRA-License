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
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/config"
	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
	"riralicense/internal/mail"
)

// Service implements pure authentication business logic without HTTP dependencies.
type Service struct {
	store          Store
	mailer         mail.Mailer
	logger         *slog.Logger
	appURL         string
	uploadDir      string
	totpKey        []byte
	oauthCfg       config.OAuthConfig
	oauthEndpoints OAuthEndpoints
	httpClient     *http.Client
	now            func() time.Time
	plan           string
	limitsMu       sync.Mutex
	productCounts  map[uuid.UUID]int
	licenseCounts  map[uuid.UUID]int
}

// NewService constructs a fully configured auth Service.
func NewService(
	store Store,
	mailer mail.Mailer,
	cfg *config.Config,
	logger *slog.Logger,
) *Service {
	uploadDir := strings.TrimSpace(cfg.UploadDir)
	if uploadDir == "" {
		uploadDir = config.DefaultUploadDir()
	}
	return &Service{
		store:          store,
		mailer:         mailer,
		logger:         logger,
		appURL:         strings.TrimRight(cfg.AppURL, "/"),
		uploadDir:      uploadDir,
		totpKey:        cfg.TOTPEncryptionKey,
		oauthCfg:       cfg.OAuth,
		oauthEndpoints: defaultOAuthEndpoints(),
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		now:            time.Now,
		plan:           FreePlan,
		productCounts:  make(map[uuid.UUID]int),
		licenseCounts:  make(map[uuid.UUID]int),
	}
}

// Plan returns the current workspace plan identifier.
func (s *Service) Plan() string {
	if s.plan == "" {
		return FreePlan
	}
	return s.plan
}

// MaxUsers returns the seat limit for the current workspace plan.
func (s *Service) MaxUsers() int {
	if s.Plan() == FreePlan {
		return FreeMaxUsers
	}
	return 0
}

// UploadDir returns the base directory where uploaded files are stored.
func (s *Service) UploadDir() string {
	return s.uploadDir
}

// ValidateSession verifies a raw session cookie token against absolute (30d)
// and idle (24h) expiration, user status, and soft-delete state.
func (s *Service) ValidateSession(ctx context.Context, rawToken string) (*UserDTO, uuid.UUID, error) {
	if rawToken == "" {
		return nil, uuid.Nil, ErrUnauthorized
	}

	row, err := s.store.Queries().GetSessionWithUserByTokenHash(ctx, crypto.HashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, uuid.Nil, ErrUnauthorized
		}
		return nil, uuid.Nil, err
	}

	if row.Status != "active" {
		_ = s.store.Queries().DeleteUserSessions(ctx, row.ID)
		return nil, uuid.Nil, ErrUnauthorized
	}
	if row.DeletedAt != nil {
		_ = s.store.Queries().DeleteSessionByID(ctx, row.SessionID)
		return nil, uuid.Nil, ErrUnauthorized
	}

	now := s.now()
	if now.After(row.SessionExpiresAt) || now.Sub(row.SessionLastActivityAt) > SessionIdleTimeout {
		_ = s.store.Queries().DeleteSessionByID(ctx, row.SessionID)
		return nil, uuid.Nil, ErrUnauthorized
	}

	if err := s.store.Queries().TouchSessionActivity(ctx, row.SessionID); err != nil {
		s.logger.Warn("failed to update session activity", "err", err)
	}

	dto := sessionRowToUserDTO(row)
	return &dto, row.SessionID, nil
}

// Logout deletes only the current session row.
func (s *Service) Logout(ctx context.Context, rawToken string, userID uuid.UUID, meta RequestMeta) error {
	if rawToken != "" {
		if err := s.store.Queries().DeleteSessionByTokenHash(ctx, crypto.HashToken(rawToken)); err != nil {
			return err
		}
	}
	if userID != uuid.Nil {
		s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, "logout", meta, nil)
	}
	return nil
}

// LogoutAll deletes every active session belonging to the user.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID, meta RequestMeta) error {
	if err := s.store.Queries().DeleteUserSessions(ctx, userID); err != nil {
		return err
	}
	s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, "session_revoked", meta, map[string]string{
		"scope": "all",
	})
	return nil
}

func (s *Service) createSession(ctx context.Context, q sqlc.Querier, userID uuid.UUID, meta RequestMeta) (string, time.Time, error) {
	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := s.now().Add(SessionAbsoluteTTL)
	sess, err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:    userID,
		TokenHash: tokenHash,
		UserAgent: pgText(meta.UserAgent),
		IpAddress: meta.IP,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	if u, uErr := q.GetUserByID(ctx, userID); uErr == nil && !u.IsSuperadmin {
		if wss, wsErr := q.ListWorkspacesForUser(ctx, userID); wsErr == nil && len(wss) > 0 {
			_ = q.SetSessionActiveWorkspace(ctx, sqlc.SetSessionActiveWorkspaceParams{
				ID:                sess.ID,
				ActiveWorkspaceID: uuid.NullUUID{UUID: wss[0].ID, Valid: true},
			})
		}
	}
	return rawToken, expiresAt, nil
}

func (s *Service) recordFailedLogin(ctx context.Context, userID uuid.UUID, event string, meta RequestMeta) {
	lockUntil := s.now().Add(AccountLockDuration)
	res, err := s.store.Queries().IncrementFailedLogin(ctx, sqlc.IncrementFailedLoginParams{
		ID:          userID,
		MaxAttempts: MaxFailedLoginAttempts,
		LockUntil:   lockUntil,
	})
	if err != nil {
		s.logger.Warn("failed to increment login failures", "err", err)
	}
	var metadata map[string]any
	if err == nil && res.LockedUntil != nil && res.FailedLoginAttempts >= MaxFailedLoginAttempts {
		metadata = map[string]any{"locked": true}
	}
	s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, event, meta, metadata)
}

func (s *Service) writeAudit(ctx context.Context, userID uuid.NullUUID, event string, meta RequestMeta, metadata any) {
	var rawJSON []byte
	if metadata != nil {
		encoded, err := json.Marshal(metadata)
		if err == nil {
			rawJSON = encoded
		}
	}
	if err := s.store.Queries().InsertAuthAuditLog(ctx, sqlc.InsertAuthAuditLogParams{
		UserID:    userID,
		Event:     event,
		IpAddress: meta.IP,
		UserAgent: pgText(meta.UserAgent),
		Metadata:  rawJSON,
	}); err != nil {
		s.logger.Warn("failed to write auth audit log", "event", event, "err", err)
	}
}

func (s *Service) writeWorkspaceAudit(
	ctx context.Context,
	userID uuid.NullUUID,
	workspaceID uuid.NullUUID,
	actorUserID uuid.NullUUID,
	event string,
	meta RequestMeta,
	metadata any,
) {
	var rawJSON []byte
	if metadata != nil {
		encoded, err := json.Marshal(metadata)
		if err == nil {
			rawJSON = encoded
		}
	}
	if err := s.store.Queries().InsertWorkspaceAuditLog(ctx, sqlc.InsertWorkspaceAuditLogParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
		ActorUserID: actorUserID,
		Event:       event,
		IpAddress:   meta.IP,
		UserAgent:   pgText(meta.UserAgent),
		Metadata:    rawJSON,
	}); err != nil {
		s.logger.Warn("failed to write workspace audit log", "event", event, "err", err)
	}
}

func pgText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeRole(role string) string {
	if IsValidRole(role) {
		return role
	}
	return RoleMember
}

func userToDTO(u sqlc.User) UserDTO {
	var avatar *string
	if u.AvatarUrl.Valid {
		v := normalizeAvatarRelativePath(u.AvatarUrl.String)
		avatar = &v
	}
	var method *string
	if u.TwoFactorMethod.Valid {
		v := u.TwoFactorMethod.String
		method = &v
	}
	var suspendedReason *string
	if u.SuspendedReason.Valid {
		v := u.SuspendedReason.String
		suspendedReason = &v
	}
	role := normalizeRole(u.Role)
	return UserDTO{
		ID:               u.ID,
		Email:            u.Email,
		Name:             u.Name,
		AvatarURL:        avatar,
		EmailVerifiedAt:  u.EmailVerifiedAt,
		Role:             role,
		IsSuperadmin:     u.IsSuperadmin,
		Permissions:      PermissionsFor(role, u.IsSuperadmin),
		Status:           u.Status,
		SuspendedReason:  suspendedReason,
		SuspendedAt:      u.SuspendedAt,
		TwoFactorEnabled: u.TwoFactorEnabled,
		TwoFactorMethod:  method,
		HasPassword:      u.PasswordHash.Valid,
		LastLoginAt:      u.LastLoginAt,
		CreatedAt:        u.CreatedAt,
	}
}

func sessionRowToUserDTO(r sqlc.GetSessionWithUserByTokenHashRow) UserDTO {
	var avatar *string
	if r.AvatarUrl.Valid {
		v := normalizeAvatarRelativePath(r.AvatarUrl.String)
		avatar = &v
	}
	var method *string
	if r.TwoFactorMethod.Valid {
		v := r.TwoFactorMethod.String
		method = &v
	}
	var suspendedReason *string
	if r.SuspendedReason.Valid {
		v := r.SuspendedReason.String
		suspendedReason = &v
	}
	var activeWorkspaceID *uuid.UUID
	if r.SessionActiveWorkspaceID.Valid {
		id := r.SessionActiveWorkspaceID.UUID
		activeWorkspaceID = &id
	}
	role := normalizeRole(r.Role)
	return UserDTO{
		ID:                r.ID,
		Email:             r.Email,
		Name:              r.Name,
		AvatarURL:         avatar,
		EmailVerifiedAt:   r.EmailVerifiedAt,
		Role:              role,
		IsSuperadmin:      r.IsSuperadmin,
		Permissions:       PermissionsFor(role, r.IsSuperadmin),
		Status:            r.Status,
		SuspendedReason:   suspendedReason,
		SuspendedAt:       r.SuspendedAt,
		ActiveWorkspaceID: activeWorkspaceID,
		TwoFactorEnabled:  r.TwoFactorEnabled,
		TwoFactorMethod:   method,
		HasPassword:       r.PasswordHash.Valid,
		LastLoginAt:       r.LastLoginAt,
		CreatedAt:         r.CreatedAt,
	}
}

func userToAdminDTO(u sqlc.User) AdminUserDTO {
	var avatar *string
	if u.AvatarUrl.Valid {
		v := normalizeAvatarRelativePath(u.AvatarUrl.String)
		avatar = &v
	}
	var suspendedReason *string
	if u.SuspendedReason.Valid {
		v := u.SuspendedReason.String
		suspendedReason = &v
	}
	return AdminUserDTO{
		ID:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		AvatarURL:       avatar,
		Role:            normalizeRole(u.Role),
		IsSuperadmin:    u.IsSuperadmin,
		Status:          u.Status,
		SuspendedReason: suspendedReason,
		SuspendedAt:     u.SuspendedAt,
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
	}
}
