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
	"slices"
	"time"

	"riralicense/internal/db/sqlc"
)

// Workspace role names stored in users.role.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// Workspace plan identifiers and free-tier plan limits.
const (
	FreePlan        = "free"
	ProPlan         = "pro"
	EnterprisePlan  = "enterprise"
	FreeMaxUsers    = 1
	FreeMaxProducts = 3
	FreeMaxLicenses = 100
)

// Permission identifiers ("resource:action").
const (
	PermProductsCreate = "products:create"
	PermProductsRead   = "products:read"
	PermProductsUpdate = "products:update"
	PermProductsDelete = "products:delete"

	PermLicensesCreate = "licenses:create"
	PermLicensesRead   = "licenses:read"
	PermLicensesUpdate = "licenses:update"
	PermLicensesRevoke = "licenses:revoke"
	PermLicensesDelete = "licenses:delete"

	PermActivationsCreate = "activations:create"
	PermActivationsRead   = "activations:read"
	PermActivationsRevoke = "activations:revoke"

	PermUsersInvite  = "users:invite"
	PermUsersRead    = "users:read"
	PermUsersUpdate  = "users:update"
	PermUsersRemove  = "users:remove"
	PermUsersSuspend = "users:suspend"

	PermRolesManage = "roles:manage"

	PermSettingsRead   = "settings:read"
	PermSettingsUpdate = "settings:update"

	PermAuditRead = "audit:read"
)

// allPermissions is the full canonical set of permissions granted to superadmin and owner.
var allPermissions = []string{
	PermProductsCreate,
	PermProductsRead,
	PermProductsUpdate,
	PermProductsDelete,
	PermLicensesCreate,
	PermLicensesRead,
	PermLicensesUpdate,
	PermLicensesRevoke,
	PermLicensesDelete,
	PermActivationsCreate,
	PermActivationsRead,
	PermActivationsRevoke,
	PermUsersInvite,
	PermUsersRead,
	PermUsersUpdate,
	PermUsersRemove,
	PermUsersSuspend,
	PermRolesManage,
	PermSettingsRead,
	PermSettingsUpdate,
	PermAuditRead,
}

// RolePermissions maps each workspace role to its concrete permission list.
var RolePermissions = map[string][]string{
	RoleOwner: allPermissions,
	RoleAdmin: {
		PermProductsCreate,
		PermProductsRead,
		PermProductsUpdate,
		PermLicensesCreate,
		PermLicensesRead,
		PermLicensesUpdate,
		PermLicensesRevoke,
		PermLicensesDelete,
		PermActivationsCreate,
		PermActivationsRead,
		PermActivationsRevoke,
		PermUsersInvite,
		PermUsersRead,
		PermUsersUpdate,
		PermUsersRemove,
		PermUsersSuspend,
		PermSettingsRead,
		PermAuditRead,
	},
	RoleMember: {
		PermProductsRead,
		PermLicensesCreate,
		PermLicensesRead,
		PermLicensesUpdate,
		PermLicensesRevoke,
		PermActivationsCreate,
		PermActivationsRead,
		PermActivationsRevoke,
	},
	RoleViewer: {
		PermProductsRead,
		PermLicensesRead,
		PermActivationsRead,
	},
}

// PermissionsFor returns a fresh copy of the computed permissions slice for the given role and superadmin flag.
func PermissionsFor(role string, isSuperadmin bool) []string {
	if isSuperadmin {
		out := make([]string, len(allPermissions))
		copy(out, allPermissions)
		return out
	}
	perms, ok := RolePermissions[role]
	if !ok {
		return []string{}
	}
	out := make([]string, len(perms))
	copy(out, perms)
	return out
}

// HasPermission reports whether the given role / superadmin status grants perm.
func HasPermission(role string, isSuperadmin bool, perm string) bool {
	if perm == "" {
		return false
	}
	if isSuperadmin {
		return slices.Contains(allPermissions, perm)
	}
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	return slices.Contains(perms, perm)
}

// IsValidRole reports whether role is one of the four workspace roles.
func IsValidRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// IsAssignableRole reports whether role can be assigned via admin user-creation or role-change endpoints.
// Owner and superadmin can never be assigned through those endpoints.
func IsAssignableRole(role string) bool {
	switch role {
	case RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// BypassesPlanLimits reports whether the user sits above workspace plan/tier limits.
func BypassesPlanLimits(user sqlc.User) bool {
	return user.IsSuperadmin
}

// IsValidPlan reports whether plan is a supported workspace plan tier.
func IsValidPlan(plan string) bool {
	switch plan {
	case FreePlan, ProPlan, EnterprisePlan:
		return true
	default:
		return false
	}
}

// WorkspaceLimits holds effective numeric limits for a workspace (0 means unlimited).
type WorkspaceLimits struct {
	MaxUsers    int
	MaxProducts int
	MaxLicenses int
}

// EffectiveWorkspaceLimits computes the workspace's active limits considering plan defaults and active overrides.
func EffectiveWorkspaceLimits(ws sqlc.Workspace, now time.Time) WorkspaceLimits {
	limits := WorkspaceLimits{}
	if ws.Plan == "" || ws.Plan == FreePlan {
		limits = WorkspaceLimits{
			MaxUsers:    FreeMaxUsers,
			MaxProducts: FreeMaxProducts,
			MaxLicenses: FreeMaxLicenses,
		}
	}
	overrideActive := ws.OverrideExpiresAt == nil || ws.OverrideExpiresAt.After(now)
	if overrideActive {
		if ws.MaxUsersOverride.Valid && ws.MaxUsersOverride.Int32 > 0 {
			limits.MaxUsers = int(ws.MaxUsersOverride.Int32)
		}
		if ws.MaxProductsOverride.Valid && ws.MaxProductsOverride.Int32 > 0 {
			limits.MaxProducts = int(ws.MaxProductsOverride.Int32)
		}
		if ws.MaxLicensesOverride.Valid && ws.MaxLicensesOverride.Int32 > 0 {
			limits.MaxLicenses = int(ws.MaxLicensesOverride.Int32)
		}
	}
	return limits
}
