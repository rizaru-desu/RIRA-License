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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
)

// ForgotPassword creates a 1-hour reset token and sends a link if the email
// belongs to an active, verified account. Always returns nil to avoid enumeration.
func (s *Service) ForgotPassword(ctx context.Context, email string, meta RequestMeta) error {
	email = normalizeEmail(email)

	user, err := s.store.Queries().GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if user.Status != "active" || user.DeletedAt != nil || user.EmailVerifiedAt == nil {
		return nil
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.InvalidateUnusedVerificationTokens(ctx, sqlc.InvalidateUnusedVerificationTokensParams{
			UserID: user.ID,
			Type:   TokenTypePasswordReset,
		}); txErr != nil {
			return txErr
		}
		_, txErr := q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    user.ID,
			Type:      TokenTypePasswordReset,
			TokenHash: tokenHash,
			ExpiresAt: s.now().Add(PasswordResetTTL),
		})
		return txErr
	})
	if err != nil {
		return err
	}

	resetURL := s.appURL + "/reset-password?token=" + rawToken
	if mailErr := s.mailer.SendPasswordResetEmail(ctx, user.Email, user.Name, resetURL); mailErr != nil {
		return mailErr
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "password_reset_requested", meta, nil)
	return nil
}

// ResetPassword validates a password reset token, checks password history (last 5),
// updates the password, and revokes all sessions and trusted devices.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string, meta RequestMeta) error {
	if rawToken == "" {
		return ErrInvalidToken
	}
	if len(newPassword) < crypto.MinPasswordLen {
		return ErrPasswordTooShort
	}

	tok, err := s.store.Queries().GetVerificationTokenByHash(ctx, sqlc.GetVerificationTokenByHashParams{
		TokenHash: crypto.HashToken(rawToken),
		Type:      TokenTypePasswordReset,
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

	user, err := s.store.Queries().GetUserByID(ctx, tok.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		return err
	}
	if user.Status == "suspended" || user.DeletedAt != nil {
		return ErrInvalidToken
	}

	return s.updatePasswordAndRevoke(ctx, user.ID, newPassword, &tok.ID, meta)
}

// ChangePassword verifies currentPassword (when set), enforces password history,
// updates the password, and revokes all sessions and trusted devices.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string, meta RequestMeta) error {
	if len(newPassword) < crypto.MinPasswordLen {
		return ErrPasswordTooShort
	}

	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if user.PasswordHash.Valid {
		ok, verifyErr := crypto.VerifyPassword(currentPassword, user.PasswordHash.String)
		if verifyErr != nil || !ok {
			return ErrInvalidCredentials
		}
	}

	return s.updatePasswordAndRevoke(ctx, user.ID, newPassword, nil, meta)
}

func (s *Service) updatePasswordAndRevoke(
	ctx context.Context,
	userID uuid.UUID,
	newPassword string,
	resetTokenID *uuid.UUID,
	meta RequestMeta,
) error {
	history, err := s.store.Queries().GetRecentPasswordHistory(ctx, userID)
	if err != nil {
		return err
	}
	for _, oldHash := range history {
		matched, verifyErr := crypto.VerifyPassword(newPassword, oldHash)
		if verifyErr == nil && matched {
			return ErrPasswordReused
		}
	}

	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if resetTokenID != nil {
			if _, txErr := q.MarkVerificationTokenUsed(ctx, *resetTokenID); txErr != nil {
				if errors.Is(txErr, pgx.ErrNoRows) {
					return ErrInvalidToken
				}
				return txErr
			}
		}
		if txErr := q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
			ID:           userID,
			PasswordHash: pgText(newHash),
		}); txErr != nil {
			return txErr
		}
		if txErr := q.InsertPasswordHistory(ctx, sqlc.InsertPasswordHistoryParams{
			UserID:       userID,
			PasswordHash: newHash,
		}); txErr != nil {
			return txErr
		}
		if txErr := q.PrunePasswordHistory(ctx, userID); txErr != nil {
			return txErr
		}
		if txErr := q.ResetLoginFailuresAndUpdateLastLogin(ctx, userID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserSessions(ctx, userID); txErr != nil {
			return txErr
		}
		return q.DeleteUserTrustedDevices(ctx, userID)
	})
	if err != nil {
		return err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: userID, Valid: true}, "password_changed", meta, nil)
	return nil
}

// RequestEmailChange creates a 24h email_change token and sends it to the NEW address.
func (s *Service) RequestEmailChange(ctx context.Context, userID uuid.UUID, newEmail, currentPassword string, meta RequestMeta) error {
	newEmail = normalizeEmail(newEmail)

	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if user.PasswordHash.Valid {
		ok, verifyErr := crypto.VerifyPassword(currentPassword, user.PasswordHash.String)
		if verifyErr != nil || !ok {
			return ErrInvalidCredentials
		}
	}
	if newEmail == user.Email {
		return ErrConflictEmail
	}

	if _, lookupErr := s.store.Queries().GetUserByEmail(ctx, newEmail); lookupErr == nil {
		return ErrConflictEmail
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.InvalidateUnusedVerificationTokens(ctx, sqlc.InvalidateUnusedVerificationTokensParams{
			UserID: user.ID,
			Type:   TokenTypeEmailChange,
		}); txErr != nil {
			return txErr
		}
		_, txErr := q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    user.ID,
			Type:      TokenTypeEmailChange,
			TokenHash: tokenHash,
			NewEmail:  pgText(newEmail),
			ExpiresAt: s.now().Add(EmailChangeTTL),
		})
		return txErr
	})
	if err != nil {
		return err
	}

	confirmURL := s.appURL + "/confirm-email-change?token=" + rawToken
	if mailErr := s.mailer.SendEmailChangeEmail(ctx, newEmail, user.Name, confirmURL); mailErr != nil {
		return mailErr
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "email_change_requested", meta, nil)
	return nil
}

// ConfirmEmailChange validates the token sent to the new address and updates the user's email.
func (s *Service) ConfirmEmailChange(ctx context.Context, rawToken string, meta RequestMeta) error {
	if rawToken == "" {
		return ErrInvalidToken
	}

	tok, err := s.store.Queries().GetVerificationTokenByHash(ctx, sqlc.GetVerificationTokenByHashParams{
		TokenHash: crypto.HashToken(rawToken),
		Type:      TokenTypeEmailChange,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		return err
	}
	if tok.UsedAt != nil || s.now().After(tok.ExpiresAt) || !tok.NewEmail.Valid {
		return ErrInvalidToken
	}

	newEmail := normalizeEmail(tok.NewEmail.String)
	if existing, lookupErr := s.store.Queries().GetUserByEmail(ctx, newEmail); lookupErr == nil && existing.ID != tok.UserID {
		return ErrConflictEmail
	} else if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if _, txErr := q.MarkVerificationTokenUsed(ctx, tok.ID); txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInvalidToken
			}
			return txErr
		}
		return q.UpdateUserEmail(ctx, sqlc.UpdateUserEmailParams{
			ID:    tok.UserID,
			Lower: newEmail,
		})
	})
	if err != nil {
		return err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: tok.UserID, Valid: true}, "email_changed", meta, nil)
	return nil
}
