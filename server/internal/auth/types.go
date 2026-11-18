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
	"net/netip"
	"time"

	"github.com/google/uuid"

	"riralicense/internal/db/sqlc"
)

const (
	SessionAbsoluteTTL         = 30 * 24 * time.Hour
	SessionIdleTimeout         = 24 * time.Hour
	TrustedDeviceTTL           = 30 * 24 * time.Hour
	EmailVerificationTTL       = 24 * time.Hour
	PasswordResetTTL           = time.Hour
	EmailChangeTTL             = 24 * time.Hour
	AccountDeletionTTL         = 24 * time.Hour
	AccountDeletionGracePeriod = 30 * 24 * time.Hour
	EmailOTPTTL                = 10 * time.Minute
	MaxFailedLoginAttempts     = 5
	AccountLockDuration        = 15 * time.Minute
	MaxEmailOTPAttempts        = 5
	TOTPIssuer                 = "RiraLicense"
	TokenTypeEmailVerify       = "email_verification"
	TokenTypePasswordReset     = "password_reset"
	TokenTypeEmailChange       = "email_change"
	TokenTypeAccountDeletion   = "account_deletion"
	ProviderGitHub             = "github"
	ProviderGoogle             = "google"
	MethodTOTP                 = "totp"
	MethodEmail                = "email"
	MinDisplayNameLength       = 2
	MaxDisplayNameLength       = 100
	MaxAvatarBytes             = 2 << 20 // 2 MiB
	AvatarTargetDimension      = 256
	AvatarURLPrefix            = "/uploads/avatars/"
)

var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrEmailNotVerified        = errors.New("email address is not verified")
	ErrInvalidToken            = errors.New("invalid or expired token")
	ErrPasswordReused          = errors.New("new password cannot match any of your last 5 passwords")
	ErrPasswordTooShort        = errors.New("password must be at least 6 characters")
	ErrConflictEmail           = errors.New("an account with this email already exists")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrTwoFactorAlreadyEnabled = errors.New("two-factor authentication is already enabled")
	ErrTwoFactorNotEnabled     = errors.New("two-factor authentication is not enabled")
	ErrTwoFactorNotPending     = errors.New("two-factor setup has not been started")
	ErrProviderDisabled        = errors.New("oauth provider is not enabled")
	ErrOAuthEmailUnavailable   = errors.New("verified email could not be retrieved from oauth provider")
	ErrOAuthExchange           = errors.New("oauth exchange failed")
	ErrInvalidDisplayName      = errors.New("name must be between 2 and 100 characters")
	ErrAvatarTooLarge          = errors.New("avatar file must not exceed 2 MB")
	ErrInvalidAvatarType       = errors.New("unsupported avatar format; only JPG, PNG, and WebP images are allowed")
	ErrRegistrationClosed      = errors.New("registration is invite-only")
	ErrInviteRequiresPaidPlan  = errors.New("invite requires a paid plan")
	ErrForbidden               = errors.New("forbidden")
	ErrFreePlanLimitReached    = errors.New("free plan limit reached")
	ErrSoleOwnerCannotDelete   = errors.New("sole owner cannot delete their account until ownership is transferred to another owner")
	ErrSoleOwnerCannotLeave    = errors.New("sole owner cannot leave workspace until ownership is transferred")
	ErrInviteNotFound          = errors.New("pending workspace invitation not found")
	ErrInvalidRole             = errors.New("role must be one of admin, member, viewer")
	ErrInvalidSuspendReason    = errors.New("reason must be at least 3 characters")
	ErrUserNotFound            = errors.New("user not found")
	ErrWorkspaceNotFound       = errors.New("workspace not found")
	ErrWorkspaceSuspended      = errors.New("workspace is suspended")
	ErrConflictWorkspaceSlug   = errors.New("a workspace with this slug already exists")
	ErrConflictMember          = errors.New("user is already a member of this workspace")
	ErrInvalidWorkspaceName    = errors.New("workspace name must be between 2 and 120 characters")
	ErrInvalidWorkspaceSlug    = errors.New("workspace slug must be 2-64 lowercase letters, numbers, or hyphens")
	ErrInvalidPlan             = errors.New("plan must be one of free, pro, enterprise")
	ErrLastActiveAdmin         = errors.New("This Workspace must have at least one active Admin. Assign another Admin before removing or deactivating this user.")
	ErrLastActiveMember        = errors.New("A Workspace must have at least one active member. Add another member before removing this user.")
	ErrGlobalSuspendBlocked    = errors.New("cannot globally suspend user because one or more workspaces would be left without an active admin or active member")
)

const (
	GovCodeLastActiveAdmin      = "last_active_admin"
	GovCodeLastActiveMember     = "last_active_member"
	GovCodeGlobalSuspendBlocked = "global_suspend_blocked"
)

// MembershipGovernanceError represents a workspace membership invariant violation.
type MembershipGovernanceError struct {
	Code          string
	Message       string
	WorkspaceID   uuid.UUID
	WorkspaceName string
	Base          error
	Compat        error
}

func (e *MembershipGovernanceError) Error() string { return e.Message }
func (e *MembershipGovernanceError) Unwrap() error { return e.Base }
func (e *MembershipGovernanceError) Is(target error) bool {
	if e.Base != nil && errors.Is(e.Base, target) {
		return true
	}
	if e.Compat != nil && errors.Is(e.Compat, target) {
		return true
	}
	return false
}

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
	ID                uuid.UUID  `json:"id"`
	Email             string     `json:"email"`
	Name              string     `json:"name"`
	AvatarURL         *string    `json:"avatar_url"`
	EmailVerifiedAt   *time.Time `json:"email_verified_at"`
	Role              string     `json:"role"`
	IsSuperadmin      bool       `json:"is_superadmin"`
	Permissions       []string   `json:"permissions"`
	Status            string     `json:"status"`
	SuspendedReason   *string    `json:"suspended_reason,omitempty"`
	SuspendedAt       *time.Time `json:"suspended_at,omitempty"`
	ActiveWorkspaceID *uuid.UUID `json:"active_workspace_id,omitempty"`
	TwoFactorEnabled  bool       `json:"two_factor_enabled"`
	TwoFactorMethod   *string    `json:"two_factor_method"`
	HasPassword       bool       `json:"has_password"`
	LastLoginAt       *time.Time `json:"last_login_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

// WorkspaceDTO represents a tenant workspace and its effective limits.
type WorkspaceDTO struct {
	ID                  uuid.UUID  `json:"id"`
	Name                string     `json:"name"`
	Slug                string     `json:"slug"`
	Plan                string     `json:"plan"`
	Status              string     `json:"status"`
	Role                string     `json:"role,omitempty"`
	SuspendedReason     *string    `json:"suspended_reason,omitempty"`
	SuspendedAt         *time.Time `json:"suspended_at,omitempty"`
	MaxUsers            int        `json:"max_users"`
	MaxProducts         int        `json:"max_products"`
	MaxLicenses         int        `json:"max_licenses"`
	MaxUsersOverride    *int32     `json:"max_users_override,omitempty"`
	MaxProductsOverride *int32     `json:"max_products_override,omitempty"`
	MaxLicensesOverride *int32     `json:"max_licenses_override,omitempty"`
	OverrideExpiresAt   *time.Time `json:"override_expires_at,omitempty"`
	OverrideNote        *string    `json:"override_note,omitempty"`
	MemberCount         int64      `json:"member_count,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

// PendingWorkspaceInviteDTO represents a pending workspace invitation for the current user.
type PendingWorkspaceInviteDTO struct {
	WorkspaceID    uuid.UUID `json:"workspace_id"`
	WorkspaceName  string    `json:"workspace_name"`
	WorkspaceSlug  string    `json:"workspace_slug"`
	WorkspacePlan  string    `json:"workspace_plan"`
	Role           string    `json:"role"`
	InvitedAt      time.Time `json:"invited_at"`
	InvitedByName  *string   `json:"invited_by_name,omitempty"`
	InvitedByEmail *string   `json:"invited_by_email,omitempty"`
}

// PlatformUserDTO represents a global user account in the Super Admin platform console.
type PlatformUserDTO struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	AvatarURL       *string    `json:"avatar_url,omitempty"`
	IsSuperadmin    bool       `json:"is_superadmin"`
	Status          string     `json:"status"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	WorkspaceCount  int64      `json:"workspace_count"`
	CreatedAt       time.Time  `json:"created_at"`
}

// WorkspaceContext describes the resolved active workspace and caller role/limits for a request.
type WorkspaceContext struct {
	Workspace   sqlc.Workspace
	Role        string
	Permissions []string
	Limits      WorkspaceLimits
}

// AdminUserDTO represents a user row in the user-management list and admin endpoints.
type AdminUserDTO struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	AvatarURL       *string    `json:"avatar_url,omitempty"`
	Role            string     `json:"role"`
	IsSuperadmin    bool       `json:"is_superadmin"`
	Status          string     `json:"status"`
	GlobalStatus    string     `json:"global_status,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	JoinedAt        *time.Time `json:"joined_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// WorkspaceAuditLogDTO represents a preserved workspace audit event.
type WorkspaceAuditLogDTO struct {
	ID          int64           `json:"id"`
	UserID      *uuid.UUID      `json:"user_id,omitempty"`
	WorkspaceID *uuid.UUID      `json:"workspace_id,omitempty"`
	ActorUserID *uuid.UUID      `json:"actor_user_id,omitempty"`
	Event       string          `json:"event"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// GlobalSuspendWorkspaceImpactDTO describes how globally suspending a user affects a single workspace.
type GlobalSuspendWorkspaceImpactDTO struct {
	WorkspaceID       uuid.UUID `json:"workspace_id"`
	WorkspaceName     string    `json:"workspace_name"`
	WorkspaceSlug     string    `json:"workspace_slug"`
	Role              string    `json:"role"`
	ActiveMembers     int64     `json:"active_members"`
	ActiveAdmins      int64     `json:"active_admins"`
	ActiveOwners      int64     `json:"active_owners"`
	NextActiveMembers int64     `json:"next_active_members"`
	NextActiveAdmins  int64     `json:"next_active_admins"`
	NextActiveOwners  int64     `json:"next_active_owners"`
	Blocked           bool      `json:"blocked"`
	BlockCode         string    `json:"block_code,omitempty"`
	BlockReason       string    `json:"block_reason,omitempty"`
}

// GlobalSuspendImpactDTO summarizes global suspension preflight validation across all affected workspaces.
type GlobalSuspendImpactDTO struct {
	UserID     uuid.UUID                         `json:"user_id"`
	Allowed    bool                              `json:"allowed"`
	Workspaces []GlobalSuspendWorkspaceImpactDTO `json:"workspaces"`
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
