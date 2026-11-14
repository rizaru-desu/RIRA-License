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
	"github.com/pquerna/otp/totp"

	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
)

// SetupTOTP generates a new TOTP secret, encrypts it with AES-256-GCM at rest,
// and returns the provisioning URI + secret once (before activation).
func (s *Service) SetupTOTP(ctx context.Context, userID uuid.UUID) (*TOTPSetupResult, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.TwoFactorEnabled {
		return nil, ErrTwoFactorAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: user.Email,
	})
	if err != nil {
		return nil, err
	}

	encryptedSecret, err := crypto.EncryptAESGCM(s.totpKey, []byte(key.Secret()))
	if err != nil {
		return nil, err
	}

	if err := s.store.Queries().SetPendingTOTPSecret(ctx, sqlc.SetPendingTOTPSecretParams{
		ID:                  user.ID,
		TotpSecretEncrypted: encryptedSecret,
	}); err != nil {
		return nil, err
	}

	return &TOTPSetupResult{
		Secret:          key.Secret(),
		ProvisioningURI: key.URL(),
	}, nil
}

// VerifyAndEnableTOTP confirms a pending TOTP setup with one valid code,
// enables TOTP 2FA, and returns 10 single-use backup codes once.
func (s *Service) VerifyAndEnableTOTP(ctx context.Context, userID uuid.UUID, code string, meta RequestMeta) ([]string, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.TwoFactorEnabled {
		return nil, ErrTwoFactorAlreadyEnabled
	}
	if len(user.TotpSecretEncrypted) == 0 {
		return nil, ErrTwoFactorNotPending
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		return nil, ErrInvalidCredentials
	}

	secretBytes, err := crypto.DecryptAESGCM(s.totpKey, user.TotpSecretEncrypted)
	if err != nil {
		return nil, err
	}

	if !totp.Validate(strings.TrimSpace(code), string(secretBytes)) {
		s.recordFailedLogin(ctx, user.ID, "2fa_failed", meta)
		return nil, ErrInvalidCredentials
	}

	codes, hashedCodes, err := generateHashedBackupCodes()
	if err != nil {
		return nil, err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.EnableTOTP(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserBackupCodes(ctx, user.ID); txErr != nil {
			return txErr
		}
		for _, h := range hashedCodes {
			if txErr := q.CreateBackupCode(ctx, sqlc.CreateBackupCodeParams{
				UserID:   user.ID,
				CodeHash: h,
			}); txErr != nil {
				return txErr
			}
		}
		return q.ResetLoginFailuresAndUpdateLastLogin(ctx, user.ID)
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "2fa_enabled", meta, map[string]string{
		"method": MethodTOTP,
	})
	return codes, nil
}

// SendEmailOTP generates a 6-digit OTP (10 min TTL), invalidates any previous
// active OTP for the user, and sends it by email.
func (s *Service) SendEmailOTP(ctx context.Context, userID uuid.UUID) error {
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
	if user.EmailVerifiedAt == nil {
		return ErrEmailNotVerified
	}
	return s.issueAndSendEmailOTP(ctx, user)
}

func (s *Service) issueAndSendEmailOTP(ctx context.Context, user sqlc.User) error {
	code, err := crypto.GenerateNumericOTP(crypto.OTPDigits)
	if err != nil {
		return err
	}
	codeHash := crypto.HashToken(code)

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.InvalidateActiveEmailOTPs(ctx, user.ID); txErr != nil {
			return txErr
		}
		_, txErr := q.CreateEmailOTPCode(ctx, sqlc.CreateEmailOTPCodeParams{
			UserID:    user.ID,
			CodeHash:  codeHash,
			ExpiresAt: s.now().Add(EmailOTPTTL),
		})
		return txErr
	})
	if err != nil {
		return err
	}

	return s.mailer.SendEmailOTP(ctx, user.Email, user.Name, code)
}

// VerifyAndEnableEmail2FA verifies a 6-digit email OTP for an authenticated user,
// enables email 2FA, and returns 10 backup codes once.
func (s *Service) VerifyAndEnableEmail2FA(ctx context.Context, userID uuid.UUID, code string, meta RequestMeta) ([]string, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if user.TwoFactorEnabled {
		return nil, ErrTwoFactorAlreadyEnabled
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		return nil, ErrInvalidCredentials
	}

	ok, err := s.verifyEmailOTPCode(ctx, user.ID, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		s.recordFailedLogin(ctx, user.ID, "2fa_failed", meta)
		return nil, ErrInvalidCredentials
	}

	codes, hashedCodes, err := generateHashedBackupCodes()
	if err != nil {
		return nil, err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.EnableEmail2FA(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserBackupCodes(ctx, user.ID); txErr != nil {
			return txErr
		}
		for _, h := range hashedCodes {
			if txErr := q.CreateBackupCode(ctx, sqlc.CreateBackupCodeParams{
				UserID:   user.ID,
				CodeHash: h,
			}); txErr != nil {
				return txErr
			}
		}
		return q.ResetLoginFailuresAndUpdateLastLogin(ctx, user.ID)
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "2fa_enabled", meta, map[string]string{
		"method": MethodEmail,
	})
	return codes, nil
}

// VerifyTwoFactorLogin completes a pending 2FA login challenge using TOTP,
// email OTP, or a single-use backup code, optionally issuing a trusted device token.
func (s *Service) VerifyTwoFactorLogin(
	ctx context.Context,
	userID uuid.UUID,
	expectedMethod string,
	code string,
	backupCode string,
	rememberDevice bool,
	meta RequestMeta,
) (*LoginResult, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.Status == "suspended" {
		_ = s.store.Queries().DeleteUserSessions(ctx, user.ID)
		return nil, ErrInvalidCredentials
	}
	if user.DeletedAt != nil || !user.TwoFactorEnabled {
		return nil, ErrInvalidCredentials
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "2fa_failed", meta, map[string]string{"reason": "locked"})
		return nil, ErrInvalidCredentials
	}

	verified, auditMethod, err := s.verifySecondFactor(ctx, user, expectedMethod, code, backupCode)
	if err != nil {
		return nil, err
	}
	if !verified {
		s.recordFailedLogin(ctx, user.ID, "2fa_failed", meta)
		return nil, ErrInvalidCredentials
	}

	return s.completeLoginSession(ctx, user, rememberDevice, meta, map[string]string{"via": auditMethod})
}

// DisableTwoFactor requires password (if set) + current TOTP/OTP (or backup code),
// then clears 2FA state and deletes ALL sessions and ALL trusted devices.
func (s *Service) DisableTwoFactor(
	ctx context.Context,
	userID uuid.UUID,
	password string,
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
	if !user.TwoFactorEnabled {
		return ErrTwoFactorNotEnabled
	}
	if user.LockedUntil != nil && user.LockedUntil.After(s.now()) {
		return ErrInvalidCredentials
	}

	if user.PasswordHash.Valid {
		ok, verifyErr := crypto.VerifyPassword(password, user.PasswordHash.String)
		if verifyErr != nil || !ok {
			s.recordFailedLogin(ctx, user.ID, "2fa_failed", meta)
			return ErrInvalidCredentials
		}
	}

	method := MethodTOTP
	if user.TwoFactorMethod.Valid && user.TwoFactorMethod.String != "" {
		method = user.TwoFactorMethod.String
	}

	verified, _, err := s.verifySecondFactor(ctx, user, method, code, backupCode)
	if err != nil {
		return err
	}
	if !verified {
		s.recordFailedLogin(ctx, user.ID, "2fa_failed", meta)
		return ErrInvalidCredentials
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.DisableTwoFactor(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserBackupCodes(ctx, user.ID); txErr != nil {
			return txErr
		}
		if txErr := q.DeleteUserSessions(ctx, user.ID); txErr != nil {
			return txErr
		}
		return q.DeleteUserTrustedDevices(ctx, user.ID)
	})
	if err != nil {
		return err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "2fa_disabled", meta, nil)
	return nil
}

// RegenerateBackupCodes verifies either password (if set) or current TOTP/email OTP,
// replaces all existing backup codes, and returns 10 new single-use codes once.
func (s *Service) RegenerateBackupCodes(
	ctx context.Context,
	userID uuid.UUID,
	password string,
	code string,
	meta RequestMeta,
) ([]string, error) {
	user, err := s.store.Queries().GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if !user.TwoFactorEnabled {
		return nil, ErrTwoFactorNotEnabled
	}

	authorized := false
	if password != "" && user.PasswordHash.Valid {
		ok, verifyErr := crypto.VerifyPassword(password, user.PasswordHash.String)
		if verifyErr == nil && ok {
			authorized = true
		}
	}
	if !authorized && code != "" {
		method := MethodTOTP
		if user.TwoFactorMethod.Valid && user.TwoFactorMethod.String != "" {
			method = user.TwoFactorMethod.String
		}
		ok, _, verifyErr := s.verifySecondFactor(ctx, user, method, code, "")
		if verifyErr != nil {
			return nil, verifyErr
		}
		authorized = ok
	}
	if !authorized {
		s.recordFailedLogin(ctx, user.ID, "2fa_failed", meta)
		return nil, ErrInvalidCredentials
	}

	codes, hashedCodes, err := generateHashedBackupCodes()
	if err != nil {
		return nil, err
	}

	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.DeleteUserBackupCodes(ctx, user.ID); txErr != nil {
			return txErr
		}
		for _, h := range hashedCodes {
			if txErr := q.CreateBackupCode(ctx, sqlc.CreateBackupCodeParams{
				UserID:   user.ID,
				CodeHash: h,
			}); txErr != nil {
				return txErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "backup_codes_regenerated", meta, nil)
	return codes, nil
}

func (s *Service) verifySecondFactor(
	ctx context.Context,
	user sqlc.User,
	method string,
	code string,
	backupCode string,
) (bool, string, error) {
	if strings.TrimSpace(backupCode) != "" {
		ok, err := s.verifyBackupCode(ctx, user.ID, backupCode)
		return ok, "backup_code", err
	}

	activeMethod := method
	if user.TwoFactorMethod.Valid && user.TwoFactorMethod.String != "" {
		activeMethod = user.TwoFactorMethod.String
	}
	if method != "" && method != activeMethod {
		return false, method, nil
	}

	switch activeMethod {
	case MethodTOTP:
		if len(user.TotpSecretEncrypted) == 0 {
			return false, MethodTOTP, nil
		}
		secretBytes, err := crypto.DecryptAESGCM(s.totpKey, user.TotpSecretEncrypted)
		if err != nil {
			return false, MethodTOTP, err
		}
		return totp.Validate(strings.TrimSpace(code), string(secretBytes)), MethodTOTP, nil

	case MethodEmail:
		ok, err := s.verifyEmailOTPCode(ctx, user.ID, code)
		return ok, MethodEmail, err

	default:
		return false, activeMethod, nil
	}
}

func (s *Service) verifyEmailOTPCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	otp, err := s.store.Queries().GetLatestActiveEmailOTP(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if otp.UsedAt != nil || s.now().After(otp.ExpiresAt) || otp.Attempts >= MaxEmailOTPAttempts {
		return false, nil
	}

	candidateHash := crypto.HashToken(strings.TrimSpace(code))
	if !crypto.ConstantTimeEqualHex(candidateHash, otp.CodeHash) {
		_, _ = s.store.Queries().IncrementEmailOTPAttempts(ctx, sqlc.IncrementEmailOTPAttemptsParams{
			ID:          otp.ID,
			MaxAttempts: MaxEmailOTPAttempts,
		})
		return false, nil
	}

	if _, err := s.store.Queries().MarkEmailOTPUsed(ctx, otp.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Service) verifyBackupCode(ctx context.Context, userID uuid.UUID, rawCode string) (bool, error) {
	normalized := crypto.NormalizeBackupCode(rawCode)
	unused, err := s.store.Queries().ListUnusedBackupCodes(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, bc := range unused {
		matched, verifyErr := crypto.VerifyPassword(normalized, bc.CodeHash)
		if verifyErr == nil && matched {
			if _, markErr := s.store.Queries().MarkBackupCodeUsed(ctx, bc.ID); markErr != nil {
				if errors.Is(markErr, pgx.ErrNoRows) {
					return false, nil
				}
				return false, markErr
			}
			return true, nil
		}
	}
	return false, nil
}

func generateHashedBackupCodes() ([]string, []string, error) {
	codes, err := crypto.GenerateBackupCodes(crypto.BackupCodeCount)
	if err != nil {
		return nil, nil, err
	}
	hashed := make([]string, 0, len(codes))
	for _, code := range codes {
		h, hashErr := crypto.HashPassword(code)
		if hashErr != nil {
			return nil, nil, hashErr
		}
		hashed = append(hashed, h)
	}
	return codes, hashed, nil
}
