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
	"net/netip"
	"time"

	"github.com/google/uuid"

	"riralicense/internal/db/sqlc"
)

const (
	SessionAbsoluteTTL     = 30 * 24 * time.Hour
	SessionIdleTimeout     = 24 * time.Hour
	TrustedDeviceTTL       = 30 * 24 * time.Hour
	EmailVerificationTTL   = 24 * time.Hour
	PasswordResetTTL       = time.Hour
	EmailChangeTTL         = 24 * time.Hour
	EmailOTPTTL            = 10 * time.Minute
	MaxFailedLoginAttempts = 5
	AccountLockDuration    = 15 * time.Minute
	MaxEmailOTPAttempts    = 5
	TOTPIssuer             = "RiraLicense"
	TokenTypeEmailVerify   = "email_verification"
	TokenTypePasswordReset = "password_reset"
	TokenTypeEmailChange   = "email_change"
	ProviderGitHub         = "github"
	ProviderGoogle         = "google"
	MethodTOTP             = "totp"
	MethodEmail            = "email"
)

var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrEmailNotVerified        = errors.New("email address is not verified")
	ErrInvalidToken            = errors.New("invalid or expired token")
	ErrPasswordReused          = errors.New("new password cannot match any of your last 5 passwords")
	ErrPasswordTooShort        = errors.New("password must be at least 12 characters")
	ErrConflictEmail           = errors.New("an account with this email already exists")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrTwoFactorAlreadyEnabled = errors.New("two-factor authentication is already enabled")
	ErrTwoFactorNotEnabled     = errors.New("two-factor authentication is not enabled")
	ErrTwoFactorNotPending     = errors.New("two-factor setup has not been started")
	ErrProviderDisabled        = errors.New("oauth provider is not enabled")
	ErrOAuthEmailUnavailable   = errors.New("verified email could not be retrieved from oauth provider")
	ErrOAuthExchange           = errors.New("oauth exchange failed")
)

// Store defines the persistence contract required by the auth service.
type Store interface {
	Queries() sqlc.Querier
	WithTx(ctx context.Context, fn func(q sqlc.Querier) error) error
}

// RequestMeta carries client metadata attached to auth operations for sessions and audit logs.
type RequestMeta struct {
	IP        *netip.Addr
	UserAgent string
}

// UserDTO is the safe public representation of an authenticated user.
type UserDTO struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	Name             string     `json:"name"`
	AvatarURL        *string    `json:"avatar_url"`
	EmailVerifiedAt  *time.Time `json:"email_verified_at"`
	Status           string     `json:"status"`
	TwoFactorEnabled bool       `json:"two_factor_enabled"`
	TwoFactorMethod  *string    `json:"two_factor_method"`
	HasPassword      bool       `json:"has_password"`
	LastLoginAt      *time.Time `json:"last_login_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// LoginResult describes either a completed session login or a pending 2FA challenge.
type LoginResult struct {
	TwoFactorRequired bool      `json:"two_factor_required"`
	TwoFactorMethod   string    `json:"two_factor_method,omitempty"`
	ChallengeUserID   uuid.UUID `json:"-"`
	SessionToken      string    `json:"-"`
	SessionExpiresAt  time.Time `json:"-"`
	TrustedDeviceRaw  string    `json:"-"`
	TrustedDeviceExp  time.Time `json:"-"`
	User              *UserDTO  `json:"user,omitempty"`
}

// TOTPSetupResult returns the one-time provisioning URI and secret for authenticator apps.
type TOTPSetupResult struct {
	Secret          string `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
}

// OAuthEndpoints allows overriding upstream provider URLs in tests.
type OAuthEndpoints struct {
	GitHubAuthURL     string
	GitHubTokenURL    string
	GitHubUserURL     string
	GitHubEmailsURL   string
	GoogleAuthURL     string
	GoogleTokenURL    string
	GoogleUserInfoURL string
}
