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
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
)

// Register creates an unverified user account, records initial password history,
// and emails an email-verification link. No session is created until verified.
func (s *Service) Register(ctx context.Context, email, password, name string, meta RequestMeta) (*UserDTO, error) {
	email = normalizeEmail(email)
	name = strings.TrimSpace(name)
	if len(password) < crypto.MinPasswordLen {
		return nil, ErrPasswordTooShort
	}

	_, err := s.store.Queries().GetUserByEmail(ctx, email)
	if err == nil {
		return nil, ErrConflictEmail
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, err
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return nil, err
	}

	var created sqlc.User
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		u, txErr := q.CreateUser(ctx, sqlc.CreateUserParams{
			Lower:           email,
			PasswordHash:    pgText(passwordHash),
			Name:            name,
			EmailVerifiedAt: nil,
		})
		if txErr != nil {
			return txErr
		}
		if txErr := q.InsertPasswordHistory(ctx, sqlc.InsertPasswordHistoryParams{
			UserID:       u.ID,
			PasswordHash: passwordHash,
		}); txErr != nil {
			return txErr
		}
		_, txErr = q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    u.ID,
			Type:      TokenTypeEmailVerify,
			TokenHash: tokenHash,
			ExpiresAt: s.now().Add(EmailVerificationTTL),
		})
		if txErr != nil {
			return txErr
		}
		created = u
		return nil
	})
	if err != nil {
		return nil, err
	}

	verifyURL := s.appURL + "/verify-email?token=" + rawToken
	if mailErr := s.mailer.SendVerificationEmail(ctx, created.Email, created.Name, verifyURL); mailErr != nil {
		return nil, mailErr
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: created.ID, Valid: true}, "register", meta, nil)
	dto := userToDTO(created)
	return &dto, nil
}

// VerifyEmail validates a single-use email verification token and marks the user verified.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string, meta RequestMeta) error {
	if rawToken == "" {
		return ErrInvalidToken
	}

	tok, err := s.store.Queries().GetVerificationTokenByHash(ctx, sqlc.GetVerificationTokenByHashParams{
		TokenHash: crypto.HashToken(rawToken),
		Type:      TokenTypeEmailVerify,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		return err
	}
	if tok.UsedAt != nil || s.now().After(tok.ExpiresAt) {
		return ErrInvalidToken
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if _, txErr := q.MarkVerificationTokenUsed(ctx, tok.ID); txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInvalidToken
			}
			return txErr
		}
		return q.UpdateUserEmailVerified(ctx, tok.UserID)
	})
	if err != nil {
		return err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: tok.UserID, Valid: true}, "email_verified", meta, nil)
	return nil
}

// ResendVerification issues a fresh verification email if the account exists and is unverified.
// It returns nil in all non-error cases to avoid user enumeration.
func (s *Service) ResendVerification(ctx context.Context, email string, meta RequestMeta) error {
	email = normalizeEmail(email)
	user, err := s.store.Queries().GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if user.Status == "suspended" || user.DeletedAt != nil || user.EmailVerifiedAt != nil {
		return nil
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.InvalidateUnusedVerificationTokens(ctx, sqlc.InvalidateUnusedVerificationTokensParams{
			UserID: user.ID,
			Type:   TokenTypeEmailVerify,
		}); txErr != nil {
			return txErr
		}
		_, txErr := q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    user.ID,
			Type:      TokenTypeEmailVerify,
			TokenHash: tokenHash,
			ExpiresAt: s.now().Add(EmailVerificationTTL),
		})
		return txErr
	})
	if err != nil {
		return err
	}

	verifyURL := s.appURL + "/verify-email?token=" + rawToken
	if mailErr := s.mailer.SendVerificationEmail(ctx, user.Email, user.Name, verifyURL); mailErr != nil {
		return mailErr
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "verification_resent", meta, nil)
	return nil
}

// Login authenticates a user with email + password and either creates a DB session
// or issues a short-lived 2FA challenge.
func (s *Service) Login(ctx context.Context, email, password, trustedDeviceRaw string, meta RequestMeta) (*LoginResult, error) {
	email = normalizeEmail(email)

	user, err := s.store.Queries().GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, _ = crypto.VerifyPassword(password, crypto.DummyArgon2Hash)
			s.writeAudit(ctx, uuid.NullUUID{}, "login_failed", meta, map[string]string{"reason": "unknown_account"})
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if user.Status == "suspended" {
		_ = s.store.Queries().DeleteUserSessions(ctx, user.ID)
		s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "login_failed", meta, map[string]string{"reason": "suspended"})
		return nil, ErrInvalidCredentials
	}
	if user.DeletedAt != nil {
		return nil, ErrInvalidCredentials
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "login_failed", meta, map[string]string{"reason": "locked"})
		return nil, ErrInvalidCredentials
	}
	if !user.PasswordHash.Valid {
		_, _ = crypto.VerifyPassword(password, crypto.DummyArgon2Hash)
		s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "login_failed", meta, map[string]string{"reason": "no_password"})
		return nil, ErrInvalidCredentials
	}

	ok, err := crypto.VerifyPassword(password, user.PasswordHash.String)
	if err != nil || !ok {
		s.recordFailedLogin(ctx, user.ID, "login_failed", meta)
		return nil, ErrInvalidCredentials
	}

	if user.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}

	if user.TwoFactorEnabled {
		if trustedDeviceRaw != "" {
			_, devErr := s.store.Queries().GetValidTrustedDevice(ctx, sqlc.GetValidTrustedDeviceParams{
				UserID:          user.ID,
				DeviceTokenHash: crypto.HashToken(trustedDeviceRaw),
			})
			if devErr == nil {
				return s.completeLoginSession(ctx, user, false, meta, map[string]string{"via": "trusted_device"})
			}
		}

		method := MethodTOTP
		if user.TwoFactorMethod.Valid && user.TwoFactorMethod.String != "" {
			method = user.TwoFactorMethod.String
		}
		if method == MethodEmail {
			if err := s.issueAndSendEmailOTP(ctx, user); err != nil {
				return nil, err
			}
		}
		return &LoginResult{
			TwoFactorRequired: true,
			TwoFactorMethod:   method,
			ChallengeUserID:   user.ID,
		}, nil
	}

	return s.completeLoginSession(ctx, user, false, meta, nil)
}

func (s *Service) completeLoginSession(
	ctx context.Context,
	user sqlc.User,
	rememberDevice bool,
	meta RequestMeta,
	auditMeta map[string]string,
) (*LoginResult, error) {
	var rawSession string
	var sessionExp = s.now().Add(SessionAbsoluteTTL)
	var rawDevice string
	var deviceExp = s.now().Add(TrustedDeviceTTL)

	err := s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.ResetLoginFailuresAndUpdateLastLogin(ctx, user.ID); txErr != nil {
			return txErr
		}
		var txErr error
		rawSession, sessionExp, txErr = s.createSession(ctx, q, user.ID, meta)
		if txErr != nil {
			return txErr
		}
		if rememberDevice {
			var devHash string
			rawDevice, devHash, txErr = crypto.GenerateRandomToken(crypto.TokenBytes)
			if txErr != nil {
				return txErr
			}
			_, txErr = q.CreateTrustedDevice(ctx, sqlc.CreateTrustedDeviceParams{
				UserID:          user.ID,
				DeviceTokenHash: devHash,
				UserAgent:       pgText(meta.UserAgent),
				IpAddress:       meta.IP,
				ExpiresAt:       deviceExp,
			})
			if txErr != nil {
				return txErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "login_success", meta, auditMeta)
	dto := userToDTO(user)
	return &LoginResult{
		TwoFactorRequired: false,
		SessionToken:      rawSession,
		SessionExpiresAt:  sessionExp,
		TrustedDeviceRaw:  rawDevice,
		TrustedDeviceExp:  deviceExp,
		User:              &dto,
	}, nil
}
