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
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
)

const (
	DefaultHardDeleteInterval = 24 * time.Hour
	hardDeletePerUserTimeout  = 30 * time.Second
	hardDeletePassTimeout     = 5 * time.Minute
)

// RequestAccountDeletion verifies the user's primary credential (Argon2id password for
// password users, or a 6-digit email OTP for OAuth-only users) and second factor when
// 2FA is enabled, then issues a single-use 24h account_deletion confirmation token by email.
func (s *Service) RequestAccountDeletion(
	ctx context.Context,
	userID uuid.UUID,
	password string,
	emailOTP string,
	code string,
	backupCode string,
	meta RequestMeta,
) error {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	if user.Status == "suspended" || user.DeletedAt != nil {
		return ErrUnauthorized
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		return ErrInvalidCredentials
	}

	if user.PasswordHash.Valid {
		ok, verifyErr := crypto.VerifyPassword(password, user.PasswordHash.String)
		if verifyErr != nil || !ok {
			return ErrInvalidCredentials
		}

		if user.TwoFactorEnabled {
			method := MethodTOTP
			if user.TwoFactorMethod.Valid && user.TwoFactorMethod.String != "" {
				method = user.TwoFactorMethod.String
			}
			twoFactorCode := strings.TrimSpace(code)
			if twoFactorCode == "" {
				twoFactorCode = strings.TrimSpace(emailOTP)
			}
			verified, _, secondErr := s.verifySecondFactor(ctx, user, method, twoFactorCode, backupCode)
			if secondErr != nil {
				return secondErr
			}
			if !verified {
				return ErrInvalidCredentials
			}
		}
	} else {
		// OAuth-only user (password_hash IS NULL): primary verification via email OTP.
		if user.TwoFactorEnabled {
			method := MethodTOTP
			if user.TwoFactorMethod.Valid && user.TwoFactorMethod.String != "" {
				method = user.TwoFactorMethod.String
			}
			if method == MethodTOTP {
				if strings.TrimSpace(emailOTP) == "" {
					return ErrInvalidCredentials
				}
				ok, otpErr := s.verifyEmailOTPCode(ctx, user.ID, emailOTP)
				if otpErr != nil {
					return otpErr
				}
				if !ok {
					return ErrInvalidCredentials
				}
				verified, _, secondErr := s.verifySecondFactor(ctx, user, MethodTOTP, code, backupCode)
				if secondErr != nil {
					return secondErr
				}
				if !verified {
					return ErrInvalidCredentials
				}
			} else {
				otpCandidate := strings.TrimSpace(emailOTP)
				if otpCandidate == "" {
					otpCandidate = strings.TrimSpace(code)
				}
				if otpCandidate != "" {
					ok, otpErr := s.verifyEmailOTPCode(ctx, user.ID, otpCandidate)
					if otpErr != nil {
						return otpErr
					}
					if !ok {
						return ErrInvalidCredentials
					}
				} else if strings.TrimSpace(backupCode) != "" {
					ok, bcErr := s.verifyBackupCode(ctx, user.ID, backupCode)
					if bcErr != nil {
						return bcErr
					}
					if !ok {
						return ErrInvalidCredentials
					}
				} else {
					return ErrInvalidCredentials
				}
			}
		} else {
			otpCandidate := strings.TrimSpace(emailOTP)
			if otpCandidate == "" {
				otpCandidate = strings.TrimSpace(code)
			}
			if otpCandidate == "" {
				return ErrInvalidCredentials
			}
			ok, otpErr := s.verifyEmailOTPCode(ctx, user.ID, otpCandidate)
			if otpErr != nil {
				return otpErr
			}
			if !ok {
				return ErrInvalidCredentials
			}
		}
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.InvalidateUnusedVerificationTokens(ctx, sqlc.InvalidateUnusedVerificationTokensParams{
			UserID: user.ID,
			Type:   TokenTypeAccountDeletion,
		}); txErr != nil {
			return txErr
		}
		_, txErr := q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    user.ID,
			Type:      TokenTypeAccountDeletion,
			TokenHash: tokenHash,
			ExpiresAt: s.now().Add(AccountDeletionTTL),
		})
		return txErr
	})
	if err != nil {
		return err
	}

	confirmURL := s.appURL + "/settings/account/delete/confirm?token=" + rawToken
	if mailErr := s.mailer.SendAccountDeletionEmail(ctx, user.Email, user.Name, confirmURL); mailErr != nil {
		return mailErr
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "account_deletion_requested", meta, nil)
	return nil
}

// ConfirmAccountDeletion validates a single-use account_deletion token and soft-deletes
// the user account in a single transaction, revoking all sessions, trusted devices,
// 2FA state, and active tokens/OTPs, and recording the account_deleted audit event.
func (s *Service) ConfirmAccountDeletion(ctx context.Context, rawToken string, meta RequestMeta) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ErrInvalidToken
	}

	tok, err := s.store.Queries().GetVerificationTokenByHash(ctx, sqlc.GetVerificationTokenByHashParams{
		TokenHash: crypto.HashToken(rawToken),
		Type:      TokenTypeAccountDeletion,
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
	if user.DeletedAt != nil {
		return ErrInvalidToken
	}

	return s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if _, txErr := q.MarkVerificationTokenUsed(ctx, tok.ID); txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInvalidToken
			}
			return txErr
		}
		if _, txErr := q.SoftDeleteUser(ctx, user.ID); txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInvalidToken
			}
			return txErr
		}
		if txErr := q.DeleteUserSessions(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserTrustedDevices(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserBackupCodes(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.InvalidateAllUserVerificationTokens(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.InvalidateActiveEmailOTPs(ctx, user.ID); txErr != nil {
			return txErr
		}
		return q.InsertAuthAuditLog(ctx, sqlc.InsertAuthAuditLogParams{
			UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
			Event:     "account_deleted",
			IpAddress: meta.IP,
			UserAgent: pgText(meta.UserAgent),
			Metadata:  nil,
		})
	})
}

// CancelAccountDeletion sends an account restore link to a soft-deleted user's email
// if the account is within the 30-day grace period. Returns nil in non-error cases
// to prevent account enumeration.
func (s *Service) CancelAccountDeletion(ctx context.Context, email string, _ RequestMeta) error {
	email = normalizeEmail(email)

	user, err := s.store.Queries().GetUserByEmailIncludingDeleted(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if user.DeletedAt == nil {
		return nil
	}
	if !s.now().Before(user.DeletedAt.Add(AccountDeletionGracePeriod)) {
		return nil
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(crypto.TokenBytes)
	if err != nil {
		return err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.InvalidateUnusedVerificationTokens(ctx, sqlc.InvalidateUnusedVerificationTokensParams{
			UserID: user.ID,
			Type:   TokenTypeAccountDeletion,
		}); txErr != nil {
			return txErr
		}
		_, txErr := q.CreateVerificationToken(ctx, sqlc.CreateVerificationTokenParams{
			UserID:    user.ID,
			Type:      TokenTypeAccountDeletion,
			TokenHash: tokenHash,
			ExpiresAt: s.now().Add(AccountDeletionTTL),
		})
		return txErr
	})
	if err != nil {
		return err
	}

	restoreURL := s.appURL + "/account/restore?token=" + rawToken
	return s.mailer.SendAccountRestoreEmail(ctx, user.Email, user.Name, restoreURL)
}

// RestoreAccount validates a restore token for a soft-deleted account within the
// 30-day grace period, clears deleted_at, sets status='active', and logs account_restored.
func (s *Service) RestoreAccount(ctx context.Context, rawToken string, meta RequestMeta) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ErrInvalidToken
	}

	tok, err := s.store.Queries().GetVerificationTokenByHash(ctx, sqlc.GetVerificationTokenByHashParams{
		TokenHash: crypto.HashToken(rawToken),
		Type:      TokenTypeAccountDeletion,
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

	user, err := s.store.Queries().GetUserByIDIncludingDeleted(ctx, tok.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		return err
	}
	if user.DeletedAt == nil || !s.now().Before(user.DeletedAt.Add(AccountDeletionGracePeriod)) {
		return ErrInvalidToken
	}

	return s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if _, txErr := q.MarkVerificationTokenUsed(ctx, tok.ID); txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInvalidToken
			}
			return txErr
		}
		if txErr := q.InvalidateAllUserVerificationTokens(ctx, user.ID); txErr != nil {
			return txErr
		}
		if _, txErr := q.RestoreUser(ctx, user.ID); txErr != nil {
			if errors.Is(txErr, pgx.ErrNoRows) {
				return ErrInvalidToken
			}
			return txErr
		}
		return q.InsertAuthAuditLog(ctx, sqlc.InsertAuthAuditLogParams{
			UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
			Event:     "account_restored",
			IpAddress: meta.IP,
			UserAgent: pgText(meta.UserAgent),
			Metadata:  nil,
		})
	})
}

// PurgeExpiredDeletedAccounts permanently deletes users whose deleted_at timestamp
// is older than 30 days. Each user is purged in an isolated transaction that removes
// dependent records and anonymizes auth_audit_log rows without logging PII or secrets.
func (s *Service) PurgeExpiredDeletedAccounts(ctx context.Context) (int, error) {
	cutoff := s.now().Add(-AccountDeletionGracePeriod)
	userIDs, err := s.store.Queries().ListUsersForHardDelete(ctx, &cutoff)
	if err != nil {
		return 0, err
	}

	purged := 0
	for _, userID := range userIDs {
		if err := ctx.Err(); err != nil {
			return purged, err
		}

		if err := s.hardDeleteSingleUser(ctx, userID); err != nil {
			s.logger.Error("hard delete user failed", "err", err)
			return purged, err
		}
		purged++
	}
	return purged, nil
}

func (s *Service) hardDeleteSingleUser(ctx context.Context, userID uuid.UUID) error {
	txCtx, cancel := context.WithTimeout(ctx, hardDeletePerUserTimeout)
	defer cancel()

	return s.store.WithTx(txCtx, func(q sqlc.Querier) error {
		if err := q.AnonymizeUserAuditLogs(txCtx, uuid.NullUUID{UUID: userID, Valid: true}); err != nil {
			return err
		}
		if err := q.DeleteUserOAuthAccounts(txCtx, userID); err != nil {
			return err
		}
		if err := q.DeleteUserSessions(txCtx, userID); err != nil {
			return err
		}
		if err := q.DeleteUserVerificationTokens(txCtx, userID); err != nil {
			return err
		}
		if err := q.DeleteUserPasswordHistory(txCtx, userID); err != nil {
			return err
		}
		if err := q.DeleteUserTrustedDevices(txCtx, userID); err != nil {
			return err
		}
		if err := q.DeleteUserBackupCodes(txCtx, userID); err != nil {
			return err
		}
		if err := q.DeleteUserEmailOTPs(txCtx, userID); err != nil {
			return err
		}
		return q.DeleteUserByID(txCtx, userID)
	})
}

// AccountDeletionWorker runs a single stoppable background ticker (once per day by default)
// that hard-deletes accounts soft-deleted more than 30 days ago.
type AccountDeletionWorker struct {
	svc       *Service
	logger    *slog.Logger
	interval  time.Duration
	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewAccountDeletionWorker creates a stoppable daily background worker for hard-deleting expired accounts.
func NewAccountDeletionWorker(svc *Service, logger *slog.Logger, interval time.Duration) *AccountDeletionWorker {
	if interval <= 0 {
		interval = DefaultHardDeleteInterval
	}
	return &AccountDeletionWorker{
		svc:      svc,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start launches the single background ticker goroutine.
func (w *AccountDeletionWorker) Start(ctx context.Context) {
	w.startOnce.Do(func() {
		go w.loop(ctx)
	})
}

// Stop signals the background worker to stop and waits for it to exit cleanly.
func (w *AccountDeletionWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	<-w.doneCh
}

func (w *AccountDeletionWorker) loop(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, hardDeletePassTimeout)
			purged, err := w.svc.PurgeExpiredDeletedAccounts(runCtx)
			cancel()
			if err != nil && ctx.Err() == nil {
				w.logger.Error("account hard-delete worker error", "err", err)
			} else if purged > 0 {
				w.logger.Info("account hard-delete worker completed", "purged_count", purged)
			}
		}
	}
}
