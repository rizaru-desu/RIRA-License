// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pquerna/otp/totp"

	"riralicense/internal/config"
	"riralicense/internal/db/sqlc"
)

type sentMail struct {
	Kind    string
	To      string
	Payload string
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (m *fakeMailer) SendVerificationEmail(_ context.Context, toEmail, _, verifyURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{Kind: "verify", To: toEmail, Payload: verifyURL})
	return nil
}

func (m *fakeMailer) SendPasswordResetEmail(_ context.Context, toEmail, _, resetURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{Kind: "reset", To: toEmail, Payload: resetURL})
	return nil
}

func (m *fakeMailer) SendEmailChangeEmail(_ context.Context, toEmail, _, confirmURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{Kind: "email_change", To: toEmail, Payload: confirmURL})
	return nil
}

func (m *fakeMailer) SendEmailOTP(_ context.Context, toEmail, _, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{Kind: "otp", To: toEmail, Payload: code})
	return nil
}

func (m *fakeMailer) SendAccountDeletionEmail(_ context.Context, toEmail, _, confirmURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{Kind: "account_deletion", To: toEmail, Payload: confirmURL})
	return nil
}

func (m *fakeMailer) SendAccountRestoreEmail(_ context.Context, toEmail, _, restoreURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{Kind: "account_restore", To: toEmail, Payload: restoreURL})
	return nil
}

func (m *fakeMailer) lastToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	last := m.sent[len(m.sent)-1]
	u, err := url.Parse(last.Payload)
	if err != nil {
		return last.Payload
	}
	return u.Query().Get("token")
}

func (m *fakeMailer) lastPayload() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sent[len(m.sent)-1].Payload
}

// memStore implements Store and sqlc.Querier in memory for isolated unit tests.
type memStore struct {
	txMu             sync.Mutex
	mu               sync.Mutex
	now              func() time.Time
	users            map[uuid.UUID]sqlc.User
	oauthAccounts    map[string]sqlc.OauthAccount
	sessions         map[uuid.UUID]sqlc.Session
	verTokens        map[uuid.UUID]sqlc.VerificationToken
	pwHistory        []sqlc.PasswordHistory
	pwSeq            int64
	trustedDevices   map[uuid.UUID]sqlc.TrustedDevice
	backupCodes      map[uuid.UUID]sqlc.BackupCode
	emailOTPs        []sqlc.EmailOtpCode
	auditLogs        []sqlc.AuthAuditLog
	auditSeq         int64
	ownedResources   map[uuid.UUID][]string
	workspaces       map[uuid.UUID]sqlc.Workspace
	wsSeq            int64
	wsOrder          map[uuid.UUID]int64
	workspaceMembers map[string]sqlc.WorkspaceMember // "<wsID>:<userID>"
}

func newMemStore(now func() time.Time) *memStore {
	return &memStore{
		now:              now,
		users:            make(map[uuid.UUID]sqlc.User),
		oauthAccounts:    make(map[string]sqlc.OauthAccount),
		sessions:         make(map[uuid.UUID]sqlc.Session),
		verTokens:        make(map[uuid.UUID]sqlc.VerificationToken),
		trustedDevices:   make(map[uuid.UUID]sqlc.TrustedDevice),
		backupCodes:      make(map[uuid.UUID]sqlc.BackupCode),
		ownedResources:   make(map[uuid.UUID][]string),
		workspaces:       make(map[uuid.UUID]sqlc.Workspace),
		wsOrder:          make(map[uuid.UUID]int64),
		workspaceMembers: make(map[string]sqlc.WorkspaceMember),
	}
}

func (s *memStore) Queries() sqlc.Querier { return s }

func (s *memStore) WithTx(_ context.Context, fn func(q sqlc.Querier) error) error {
	s.txMu.Lock()
	defer s.txMu.Unlock()
	return fn(s)
}

func (s *memStore) CountAuthEventsByDay(_ context.Context, _ time.Time) ([]sqlc.CountAuthEventsByDayRow, error) {
	return nil, nil
}

func (s *memStore) CreateBackupCode(_ context.Context, arg sqlc.CreateBackupCodeParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := uuid.New()
	s.backupCodes[id] = sqlc.BackupCode{
		ID:        id,
		UserID:    arg.UserID,
		CodeHash:  arg.CodeHash,
		CreatedAt: s.now(),
	}
	return nil
}

func (s *memStore) CreateEmailOTPCode(_ context.Context, arg sqlc.CreateEmailOTPCodeParams) (sqlc.EmailOtpCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := sqlc.EmailOtpCode{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		CodeHash:  arg.CodeHash,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: s.now(),
	}
	s.emailOTPs = append(s.emailOTPs, rec)
	return rec, nil
}

func (s *memStore) CreateOAuthAccount(_ context.Context, arg sqlc.CreateOAuthAccountParams) (sqlc.OauthAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := arg.Provider + ":" + arg.ProviderUserID
	if _, exists := s.oauthAccounts[key]; exists {
		return sqlc.OauthAccount{}, pgx.ErrNoRows
	}
	rec := sqlc.OauthAccount{
		ID:             uuid.New(),
		UserID:         arg.UserID,
		Provider:       arg.Provider,
		ProviderUserID: arg.ProviderUserID,
		CreatedAt:      s.now(),
	}
	s.oauthAccounts[key] = rec
	return rec, nil
}

func (s *memStore) CreateSession(_ context.Context, arg sqlc.CreateSessionParams) (sqlc.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	rec := sqlc.Session{
		ID:             uuid.New(),
		UserID:         arg.UserID,
		TokenHash:      arg.TokenHash,
		UserAgent:      arg.UserAgent,
		IpAddress:      arg.IpAddress,
		ExpiresAt:      arg.ExpiresAt,
		LastActivityAt: now,
		CreatedAt:      now,
	}
	s.sessions[rec.ID] = rec
	return rec, nil
}

func (s *memStore) CreateTrustedDevice(_ context.Context, arg sqlc.CreateTrustedDeviceParams) (sqlc.TrustedDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := sqlc.TrustedDevice{
		ID:              uuid.New(),
		UserID:          arg.UserID,
		DeviceTokenHash: arg.DeviceTokenHash,
		UserAgent:       arg.UserAgent,
		IpAddress:       arg.IpAddress,
		ExpiresAt:       arg.ExpiresAt,
		CreatedAt:       s.now(),
	}
	s.trustedDevices[rec.ID] = rec
	return rec, nil
}

func (s *memStore) CreateUser(_ context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	role := arg.Role
	if role == "" {
		role = RoleMember
	}
	u := sqlc.User{
		ID:              uuid.New(),
		Email:           strings.ToLower(arg.Lower),
		PasswordHash:    arg.PasswordHash,
		Name:            arg.Name,
		AvatarUrl:       arg.AvatarUrl,
		EmailVerifiedAt: arg.EmailVerifiedAt,
		Role:            role,
		IsSuperadmin:    arg.IsSuperadmin,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.users[u.ID] = u
	return u, nil
}

func (s *memStore) CountAllUsers(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.users)), nil
}

func (s *memStore) CountActiveOwners(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, u := range s.users {
		if normalizeRole(u.Role) == RoleOwner && u.Status == "active" && u.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (s *memStore) ListUsers(_ context.Context) ([]sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]sqlc.User, 0, len(s.users))
	for _, u := range s.users {
		if u.DeletedAt == nil {
			out = append(out, u)
		}
	}
	slices.SortFunc(out, func(a, b sqlc.User) int {
		if !a.CreatedAt.Equal(b.CreatedAt) {
			if a.CreatedAt.Before(b.CreatedAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return out, nil
}

func (s *memStore) UpdateUserRole(_ context.Context, arg sqlc.UpdateUserRoleParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[arg.ID]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.Role = arg.Role
	u.UpdatedAt = s.now()
	s.users[arg.ID] = u
	return u, nil
}

func (s *memStore) SuspendUser(_ context.Context, arg sqlc.SuspendUserParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[arg.ID]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	now := s.now()
	u.Status = "suspended"
	u.SuspendedReason = arg.SuspendedReason
	u.SuspendedAt = &now
	u.UpdatedAt = now
	s.users[arg.ID] = u
	return u, nil
}

func (s *memStore) ActivateUser(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	now := s.now()
	u.Status = "active"
	u.SuspendedReason = pgtype.Text{}
	u.SuspendedAt = nil
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
	u.UpdatedAt = now
	s.users[id] = u
	return u, nil
}

func (s *memStore) CreateVerificationToken(_ context.Context, arg sqlc.CreateVerificationTokenParams) (sqlc.VerificationToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok := sqlc.VerificationToken{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		Type:      arg.Type,
		TokenHash: arg.TokenHash,
		NewEmail:  arg.NewEmail,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: s.now(),
	}
	s.verTokens[tok.ID] = tok
	return tok, nil
}

func (s *memStore) DeleteSessionByID(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *memStore) DeleteSessionByTokenHash(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.TokenHash == tokenHash {
			delete(s.sessions, id)
		}
	}
	return nil
}

func (s *memStore) DeleteUserBackupCodes(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, bc := range s.backupCodes {
		if bc.UserID == userID {
			delete(s.backupCodes, id)
		}
	}
	return nil
}

func (s *memStore) DeleteUserSessions(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, id)
		}
	}
	return nil
}

func (s *memStore) DeleteUserTrustedDevices(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, d := range s.trustedDevices {
		if d.UserID == userID {
			delete(s.trustedDevices, id)
		}
	}
	return nil
}

func (s *memStore) DisableTwoFactor(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	u.TwoFactorEnabled = false
	u.TwoFactorMethod = pgtype.Text{}
	u.TotpSecretEncrypted = nil
	u.TotpVerifiedAt = nil
	s.users[id] = u
	return nil
}

func (s *memStore) EnableEmail2FA(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	u.TwoFactorEnabled = true
	u.TwoFactorMethod = pgtype.Text{String: MethodEmail, Valid: true}
	s.users[id] = u
	return nil
}

func (s *memStore) EnableTOTP(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	now := s.now()
	u.TwoFactorEnabled = true
	u.TwoFactorMethod = pgtype.Text{String: MethodTOTP, Valid: true}
	u.TotpVerifiedAt = &now
	s.users[id] = u
	return nil
}

func (s *memStore) GetLatestActiveEmailOTP(_ context.Context, userID uuid.UUID) (sqlc.EmailOtpCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.emailOTPs) - 1; i >= 0; i-- {
		otp := s.emailOTPs[i]
		if otp.UserID == userID && otp.UsedAt == nil {
			return otp, nil
		}
	}
	return sqlc.EmailOtpCode{}, pgx.ErrNoRows
}

func (s *memStore) GetOAuthAccount(_ context.Context, arg sqlc.GetOAuthAccountParams) (sqlc.OauthAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.oauthAccounts[arg.Provider+":"+arg.ProviderUserID]
	if !ok {
		return sqlc.OauthAccount{}, pgx.ErrNoRows
	}
	return rec, nil
}

func (s *memStore) GetRecentPasswordHistory(_ context.Context, userID uuid.UUID) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for i := len(s.pwHistory) - 1; i >= 0 && len(out) < 5; i-- {
		if s.pwHistory[i].UserID == userID {
			out = append(out, s.pwHistory[i].PasswordHash)
		}
	}
	return out, nil
}

func (s *memStore) GetSessionWithUserByTokenHash(_ context.Context, tokenHash string) (sqlc.GetSessionWithUserByTokenHashRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.sessions {
		if sess.TokenHash == tokenHash {
			u := s.users[sess.UserID]
			return sqlc.GetSessionWithUserByTokenHashRow{
				SessionID:             sess.ID,
				SessionUserID:         sess.UserID,
				SessionTokenHash:      sess.TokenHash,
				SessionUserAgent:      sess.UserAgent,
				SessionIpAddress:      sess.IpAddress,
				SessionExpiresAt:      sess.ExpiresAt,
				SessionLastActivityAt: sess.LastActivityAt,
				SessionCreatedAt:      sess.CreatedAt,
				ID:                    u.ID,
				Email:                 u.Email,
				PasswordHash:          u.PasswordHash,
				Name:                  u.Name,
				AvatarUrl:             u.AvatarUrl,
				EmailVerifiedAt:       u.EmailVerifiedAt,
				Role:                  u.Role,
				IsSuperadmin:          u.IsSuperadmin,
				Status:                u.Status,
				SuspendedReason:       u.SuspendedReason,
				SuspendedAt:           u.SuspendedAt,
				DeletedAt:             u.DeletedAt,
				TwoFactorEnabled:      u.TwoFactorEnabled,
				TwoFactorMethod:       u.TwoFactorMethod,
				TotpSecretEncrypted:   u.TotpSecretEncrypted,
				TotpVerifiedAt:        u.TotpVerifiedAt,
				FailedLoginAttempts:   u.FailedLoginAttempts,
				LockedUntil:           u.LockedUntil,
				LastLoginAt:           u.LastLoginAt,
				CreatedAt:             u.CreatedAt,
				UpdatedAt:             u.UpdatedAt,
			}, nil
		}
	}
	return sqlc.GetSessionWithUserByTokenHashRow{}, pgx.ErrNoRows
}

func (s *memStore) GetUserByEmail(_ context.Context, email string) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.ToLower(email)
	for _, u := range s.users {
		if strings.ToLower(u.Email) == target && u.DeletedAt == nil {
			return u, nil
		}
	}
	return sqlc.User{}, pgx.ErrNoRows
}

func (s *memStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (s *memStore) GetValidTrustedDevice(_ context.Context, arg sqlc.GetValidTrustedDeviceParams) (sqlc.TrustedDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for _, d := range s.trustedDevices {
		if d.UserID == arg.UserID && d.DeviceTokenHash == arg.DeviceTokenHash && d.ExpiresAt.After(now) {
			return d, nil
		}
	}
	return sqlc.TrustedDevice{}, pgx.ErrNoRows
}

func (s *memStore) GetVerificationTokenByHash(_ context.Context, arg sqlc.GetVerificationTokenByHashParams) (sqlc.VerificationToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tok := range s.verTokens {
		if tok.TokenHash == arg.TokenHash && tok.Type == arg.Type {
			return tok, nil
		}
	}
	return sqlc.VerificationToken{}, pgx.ErrNoRows
}

func (s *memStore) IncrementEmailOTPAttempts(_ context.Context, arg sqlc.IncrementEmailOTPAttemptsParams) (sqlc.EmailOtpCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.emailOTPs {
		if s.emailOTPs[i].ID == arg.ID {
			s.emailOTPs[i].Attempts++
			if s.emailOTPs[i].Attempts >= arg.MaxAttempts {
				now := s.now()
				s.emailOTPs[i].UsedAt = &now
			}
			return s.emailOTPs[i], nil
		}
	}
	return sqlc.EmailOtpCode{}, pgx.ErrNoRows
}

func (s *memStore) IncrementFailedLogin(_ context.Context, arg sqlc.IncrementFailedLoginParams) (sqlc.IncrementFailedLoginRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[arg.ID]
	u.FailedLoginAttempts++
	if u.FailedLoginAttempts >= arg.MaxAttempts {
		lu := arg.LockUntil
		u.LockedUntil = &lu
	}
	s.users[arg.ID] = u
	return sqlc.IncrementFailedLoginRow{
		FailedLoginAttempts: u.FailedLoginAttempts,
		LockedUntil:         u.LockedUntil,
	}, nil
}

func (s *memStore) InsertAuthAuditLog(_ context.Context, arg sqlc.InsertAuthAuditLogParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditSeq++
	s.auditLogs = append(s.auditLogs, sqlc.AuthAuditLog{
		ID:        s.auditSeq,
		UserID:    arg.UserID,
		Event:     arg.Event,
		IpAddress: arg.IpAddress,
		UserAgent: arg.UserAgent,
		Metadata:  arg.Metadata,
		CreatedAt: s.now(),
	})
	return nil
}

func (s *memStore) InsertPasswordHistory(_ context.Context, arg sqlc.InsertPasswordHistoryParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pwSeq++
	s.pwHistory = append(s.pwHistory, sqlc.PasswordHistory{
		ID:           s.pwSeq,
		UserID:       arg.UserID,
		PasswordHash: arg.PasswordHash,
		CreatedAt:    s.now(),
	})
	return nil
}

func (s *memStore) InvalidateActiveEmailOTPs(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for i := range s.emailOTPs {
		if s.emailOTPs[i].UserID == userID && s.emailOTPs[i].UsedAt == nil {
			s.emailOTPs[i].UsedAt = &now
		}
	}
	return nil
}

func (s *memStore) InvalidateUnusedVerificationTokens(_ context.Context, arg sqlc.InvalidateUnusedVerificationTokensParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, tok := range s.verTokens {
		if tok.UserID == arg.UserID && tok.Type == arg.Type && tok.UsedAt == nil {
			tok.UsedAt = &now
			s.verTokens[id] = tok
		}
	}
	return nil
}

func (s *memStore) ListUnusedBackupCodes(_ context.Context, userID uuid.UUID) ([]sqlc.BackupCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sqlc.BackupCode
	for _, bc := range s.backupCodes {
		if bc.UserID == userID && bc.UsedAt == nil {
			out = append(out, bc)
		}
	}
	return out, nil
}

func (s *memStore) MarkBackupCodeUsed(_ context.Context, id uuid.UUID) (sqlc.BackupCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bc := s.backupCodes[id]
	if bc.UsedAt != nil {
		return sqlc.BackupCode{}, pgx.ErrNoRows
	}
	now := s.now()
	bc.UsedAt = &now
	s.backupCodes[id] = bc
	return bc, nil
}

func (s *memStore) MarkEmailOTPUsed(_ context.Context, id uuid.UUID) (sqlc.EmailOtpCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.emailOTPs {
		if s.emailOTPs[i].ID == id && s.emailOTPs[i].UsedAt == nil {
			now := s.now()
			s.emailOTPs[i].UsedAt = &now
			return s.emailOTPs[i], nil
		}
	}
	return sqlc.EmailOtpCode{}, pgx.ErrNoRows
}

func (s *memStore) MarkVerificationTokenUsed(_ context.Context, id uuid.UUID) (sqlc.VerificationToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok := s.verTokens[id]
	if tok.UsedAt != nil {
		return sqlc.VerificationToken{}, pgx.ErrNoRows
	}
	now := s.now()
	tok.UsedAt = &now
	s.verTokens[id] = tok
	return tok, nil
}

func (s *memStore) PrunePasswordHistory(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var indices []int
	for i := len(s.pwHistory) - 1; i >= 0; i-- {
		if s.pwHistory[i].UserID == userID {
			indices = append(indices, i)
		}
	}
	if len(indices) <= 5 {
		return nil
	}
	dropSet := make(map[int]bool)
	for _, idx := range indices[5:] {
		dropSet[idx] = true
	}
	filtered := s.pwHistory[:0]
	for i, item := range s.pwHistory {
		if !dropSet[i] {
			filtered = append(filtered, item)
		}
	}
	s.pwHistory = filtered
	return nil
}

func (s *memStore) ResetLoginFailuresAndUpdateLastLogin(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	now := s.now()
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
	u.LastLoginAt = &now
	s.users[id] = u
	return nil
}

func (s *memStore) SetPendingTOTPSecret(_ context.Context, arg sqlc.SetPendingTOTPSecretParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[arg.ID]
	u.TotpSecretEncrypted = arg.TotpSecretEncrypted
	u.TotpVerifiedAt = nil
	s.users[arg.ID] = u
	return nil
}

func (s *memStore) TouchSessionActivity(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.LastActivityAt = s.now()
		s.sessions[id] = sess
	}
	return nil
}

func (s *memStore) UpdateUserAvatarIfEmpty(_ context.Context, arg sqlc.UpdateUserAvatarIfEmptyParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[arg.ID]
	if !u.AvatarUrl.Valid {
		u.AvatarUrl = arg.AvatarUrl
		s.users[arg.ID] = u
	}
	return nil
}

func (s *memStore) UpdateUserAvatar(_ context.Context, arg sqlc.UpdateUserAvatarParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[arg.ID]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.AvatarUrl = arg.AvatarUrl
	u.UpdatedAt = s.now()
	s.users[arg.ID] = u
	return u, nil
}

func (s *memStore) UpdateUserName(_ context.Context, arg sqlc.UpdateUserNameParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[arg.ID]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.Name = arg.Name
	u.UpdatedAt = s.now()
	s.users[arg.ID] = u
	return u, nil
}

func (s *memStore) UpdateUserEmail(_ context.Context, arg sqlc.UpdateUserEmailParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[arg.ID]
	now := s.now()
	u.Email = strings.ToLower(arg.Lower)
	u.EmailVerifiedAt = &now
	s.users[arg.ID] = u
	return nil
}

func (s *memStore) UpdateUserEmailVerified(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	now := s.now()
	u.EmailVerifiedAt = &now
	s.users[id] = u
	return nil
}

func (s *memStore) UpdateUserPassword(_ context.Context, arg sqlc.UpdateUserPasswordParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[arg.ID]
	u.PasswordHash = arg.PasswordHash
	s.users[arg.ID] = u
	return nil
}

func (s *memStore) AnonymizeUserAuditLogs(_ context.Context, userID uuid.NullUUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.auditLogs {
		if s.auditLogs[i].UserID.Valid && s.auditLogs[i].UserID.UUID == userID.UUID {
			s.auditLogs[i].UserID = uuid.NullUUID{}
			s.auditLogs[i].IpAddress = nil
			s.auditLogs[i].UserAgent = pgtype.Text{}
			s.auditLogs[i].Metadata = nil
		}
	}
	return nil
}

func (s *memStore) DeleteUserByID(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ownedResources, id)
	delete(s.users, id)
	return nil
}

func (s *memStore) DeleteUserEmailOTPs(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.emailOTPs[:0]
	for _, otp := range s.emailOTPs {
		if otp.UserID != userID {
			filtered = append(filtered, otp)
		}
	}
	s.emailOTPs = filtered
	return nil
}

func (s *memStore) DeleteUserOAuthAccounts(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, acct := range s.oauthAccounts {
		if acct.UserID == userID {
			delete(s.oauthAccounts, k)
		}
	}
	return nil
}

func (s *memStore) DeleteUserPasswordHistory(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.pwHistory[:0]
	for _, item := range s.pwHistory {
		if item.UserID != userID {
			filtered = append(filtered, item)
		}
	}
	s.pwHistory = filtered
	return nil
}

func (s *memStore) DeleteUserVerificationTokens(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, tok := range s.verTokens {
		if tok.UserID == userID {
			delete(s.verTokens, id)
		}
	}
	return nil
}

func (s *memStore) GetUserByEmailIncludingDeleted(_ context.Context, email string) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.ToLower(email)
	for _, u := range s.users {
		if strings.ToLower(u.Email) == target {
			return u, nil
		}
	}
	return sqlc.User{}, pgx.ErrNoRows
}

func (s *memStore) GetUserByIDIncludingDeleted(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (s *memStore) InvalidateAllUserVerificationTokens(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, tok := range s.verTokens {
		if tok.UserID == userID && tok.UsedAt == nil {
			tok.UsedAt = &now
			s.verTokens[id] = tok
		}
	}
	return nil
}

func (s *memStore) ListUsersForHardDelete(_ context.Context, deletedAt *time.Time) ([]uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if deletedAt == nil {
		return nil, nil
	}
	var out []uuid.UUID
	for _, u := range s.users {
		if u.DeletedAt != nil && u.DeletedAt.Before(*deletedAt) {
			out = append(out, u.ID)
		}
	}
	return out, nil
}

func (s *memStore) RestoreUser(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok || u.DeletedAt == nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.DeletedAt = nil
	u.Status = "active"
	u.SuspendedReason = pgtype.Text{}
	u.SuspendedAt = nil
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
	s.users[id] = u
	return u, nil
}

func (s *memStore) SoftDeleteUser(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	now := s.now()
	u.DeletedAt = &now
	u.Status = "suspended"
	u.TwoFactorEnabled = false
	u.TwoFactorMethod = pgtype.Text{}
	u.TotpSecretEncrypted = nil
	u.TotpVerifiedAt = nil
	s.users[id] = u
	return u, nil
}

func newTestService(t *testing.T) (*Service, *memStore, *fakeMailer, *time.Time) {
	t.Helper()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := newMemStore(clock)
	mailer := &fakeMailer{}
	cfg := &config.Config{
		AppURL:            "https://app.riralicense.test",
		UploadDir:         t.TempDir(),
		SessionSecret:     bytes.Repeat([]byte("s"), 32),
		TOTPEncryptionKey: bytes.Repeat([]byte("k"), 32),
		OAuth: config.OAuthConfig{
			GitHubClientID:     "gh-id",
			GitHubClientSecret: "gh-secret",
			GoogleClientID:     "google-id",
			GoogleClientSecret: "google-secret",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(store, mailer, cfg, logger)
	svc.now = clock
	return svc, store, mailer, &now
}

func testMeta() RequestMeta {
	ip := netip.MustParseAddr("127.0.0.1")
	return RequestMeta{IP: &ip, UserAgent: "go-test"}
}

func TestSessionLifecycleAndExpiry(t *testing.T) {
	svc, store, mailer, now := newTestService(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, "Alice@Example.com", "super-secret-pass-1", "Alice", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Cannot login before verification.
	if _, err := svc.Login(ctx, "alice@example.com", "super-secret-pass-1", "", testMeta()); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}

	// Verify email token is single-use.
	verifyTok := mailer.lastToken()
	if err := svc.VerifyEmail(ctx, verifyTok, testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, verifyTok, testMeta()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected reuse of verification token to fail, got %v", err)
	}

	res, err := svc.Login(ctx, "alice@example.com", "super-secret-pass-1", "", testMeta())
	if err != nil || res.SessionToken == "" {
		t.Fatalf("Login failed: %v", err)
	}

	// Valid session refreshes last_activity_at.
	u, _, err := svc.ValidateSession(ctx, res.SessionToken)
	if err != nil || u.Email != "alice@example.com" {
		t.Fatalf("ValidateSession failed: %v", err)
	}

	// Advance 23h -> still valid and touches activity.
	*now = now.Add(23 * time.Hour)
	if _, _, err := svc.ValidateSession(ctx, res.SessionToken); err != nil {
		t.Fatalf("expected session valid at 23h, got %v", err)
	}

	// Advance 25h without activity -> idle timeout (24h).
	*now = now.Add(25 * time.Hour)
	if _, _, err := svc.ValidateSession(ctx, res.SessionToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected idle timeout after 24h, got %v", err)
	}

	// New login + keep active every 12h for 31 days -> fails at absolute 30d expiry.
	res2, err := svc.Login(ctx, "alice@example.com", "super-secret-pass-1", "", testMeta())
	if err != nil {
		t.Fatalf("second login failed: %v", err)
	}
	for i := 0; i < 58; i++ {
		*now = now.Add(12 * time.Hour)
		if _, _, err := svc.ValidateSession(ctx, res2.SessionToken); err != nil {
			t.Fatalf("expected valid before 30d at step %d, got %v", i, err)
		}
	}
	*now = now.Add(25 * time.Hour)
	if _, _, err := svc.ValidateSession(ctx, res2.SessionToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected absolute 30d expiry, got %v", err)
	}

	// Suspended user rejected and sessions deleted.
	res3, err := svc.Login(ctx, "alice@example.com", "super-secret-pass-1", "", testMeta())
	if err != nil {
		t.Fatalf("third login failed: %v", err)
	}
	store.mu.Lock()
	usr := store.users[res3.User.ID]
	usr.Status = "suspended"
	store.users[usr.ID] = usr
	store.mu.Unlock()

	if _, _, err := svc.ValidateSession(ctx, res3.SessionToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected suspended user session rejected, got %v", err)
	}
	if len(store.sessions) != 0 {
		t.Fatalf("expected all suspended user sessions deleted, got %d", len(store.sessions))
	}
}

func TestTOTPBackupCodesTrustedDeviceAndLockout(t *testing.T) {
	svc, store, mailer, now := newTestService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, "totp@example.com", "initial-password-123", "TOTP User", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	setup, err := svc.SetupTOTP(ctx, user.ID)
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}
	if !strings.Contains(setup.ProvisioningURI, "RiraLicense") {
		t.Fatalf("expected issuer RiraLicense in URI: %s", setup.ProvisioningURI)
	}

	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}
	backupCodes, err := svc.VerifyAndEnableTOTP(ctx, user.ID, code, testMeta())
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}
	if len(backupCodes) != 10 {
		t.Fatalf("expected 10 backup codes, got %d", len(backupCodes))
	}

	// Login requires 2FA challenge.
	loginRes, err := svc.Login(ctx, "totp@example.com", "initial-password-123", "", testMeta())
	if err != nil || !loginRes.TwoFactorRequired || loginRes.TwoFactorMethod != MethodTOTP {
		t.Fatalf("expected 2FA TOTP challenge, got %+v err=%v", loginRes, err)
	}

	// Use backup code once -> succeeds & issues trusted device.
	verifyRes, err := svc.VerifyTwoFactorLogin(ctx, user.ID, MethodTOTP, "", backupCodes[0], true, testMeta())
	if err != nil || verifyRes.SessionToken == "" || verifyRes.TrustedDeviceRaw == "" {
		t.Fatalf("backup code login failed: %+v err=%v", verifyRes, err)
	}

	// Reuse same backup code -> fails.
	if _, err := svc.VerifyTwoFactorLogin(ctx, user.ID, MethodTOTP, "", backupCodes[0], false, testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected backup code reuse rejected, got %v", err)
	}

	// Trusted device skips 2FA on subsequent login.
	trustedLogin, err := svc.Login(ctx, "totp@example.com", "initial-password-123", verifyRes.TrustedDeviceRaw, testMeta())
	if err != nil || trustedLogin.TwoFactorRequired || trustedLogin.SessionToken == "" {
		t.Fatalf("expected trusted device to skip 2FA, got %+v err=%v", trustedLogin, err)
	}

	// 5 failed TOTP attempts lock the account for 15 minutes (using same counter).
	for i := 0; i < 5; i++ {
		if _, err := svc.VerifyTwoFactorLogin(ctx, user.ID, MethodTOTP, "000000", "", false, testMeta()); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	}
	store.mu.Lock()
	u := store.users[user.ID]
	locked := u.LockedUntil
	attempts := u.FailedLoginAttempts
	store.mu.Unlock()
	if attempts < MaxFailedLoginAttempts || locked == nil || !locked.After(*now) {
		t.Fatalf("expected account locked after 5 TOTP failures: attempts=%d locked=%v", attempts, locked)
	}

	// Disable 2FA clears sessions and trusted devices after lock expires.
	*now = now.Add(16 * time.Minute)
	validCode, _ := totp.GenerateCode(setup.Secret, time.Now())
	if err := svc.DisableTwoFactor(ctx, user.ID, "initial-password-123", validCode, "", testMeta()); err != nil {
		t.Fatalf("DisableTwoFactor failed: %v", err)
	}
	if len(store.sessions) != 0 || len(store.trustedDevices) != 0 {
		t.Fatalf("expected all sessions and trusted devices cleared, got sessions=%d devices=%d", len(store.sessions), len(store.trustedDevices))
	}
}

func TestPasswordMinLength5Rejected6AcceptedAndHistoryReuse(t *testing.T) {
	svc, _, mailer, _ := newTestService(t)
	ctx := context.Background()

	// 1. Register: 5 chars rejected, 6 chars accepted.
	if _, err := svc.Register(ctx, "min6@example.com", "12345", "Min Six", testMeta()); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected 5-char password rejected with ErrPasswordTooShort, got %v", err)
	}
	if ErrPasswordTooShort.Error() != "password must be at least 6 characters" {
		t.Fatalf("unexpected ErrPasswordTooShort message: %q", ErrPasswordTooShort.Error())
	}

	user, err := svc.Register(ctx, "min6@example.com", "abc123", "Min Six", testMeta())
	if err != nil {
		t.Fatalf("expected 6-char password accepted on Register, got %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// 2. ChangePassword: 5 chars rejected, 6 chars accepted, history check blocks reuse.
	if err := svc.ChangePassword(ctx, user.ID, "abc123", "12345", testMeta()); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected 5-char new password rejected on ChangePassword, got %v", err)
	}
	if err := svc.ChangePassword(ctx, user.ID, "abc123", "def456", testMeta()); err != nil {
		t.Fatalf("expected 6-char new password accepted on ChangePassword, got %v", err)
	}
	if err := svc.ChangePassword(ctx, user.ID, "def456", "abc123", testMeta()); !errors.Is(err, ErrPasswordReused) {
		t.Fatalf("expected password history check to block reuse of 6-char password, got %v", err)
	}

	// 3. ResetPassword: 5 chars rejected, history reuse blocked, 6 chars accepted.
	if err := svc.ForgotPassword(ctx, "min6@example.com", testMeta()); err != nil {
		t.Fatalf("ForgotPassword failed: %v", err)
	}
	resetTok := mailer.lastToken()

	if err := svc.ResetPassword(ctx, resetTok, "12345", testMeta()); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected 5-char password rejected on ResetPassword, got %v", err)
	}
	if err := svc.ResetPassword(ctx, resetTok, "abc123", testMeta()); !errors.Is(err, ErrPasswordReused) {
		t.Fatalf("expected password history check to block reuse on ResetPassword, got %v", err)
	}
	if err := svc.ResetPassword(ctx, resetTok, "ghi789", testMeta()); err != nil {
		t.Fatalf("expected 6-char password accepted on ResetPassword, got %v", err)
	}
}

func TestPasswordHistoryLast5AndRevocation(t *testing.T) {
	svc, store, mailer, _ := newTestService(t)
	ctx := context.Background()

	passwords := []string{
		"password-number-01",
		"password-number-02",
		"password-number-03",
		"password-number-04",
		"password-number-05",
		"password-number-06",
	}

	user, err := svc.Register(ctx, "pw@example.com", passwords[0], "PW User", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// Rotate through passwords[1..4] so history holds [0,1,2,3,4].
	for i := 1; i <= 4; i++ {
		if err := svc.ChangePassword(ctx, user.ID, passwords[i-1], passwords[i], testMeta()); err != nil {
			t.Fatalf("ChangePassword %d failed: %v", i, err)
		}
	}

	// Reusing password[0] (5th in history) must be rejected.
	if err := svc.ChangePassword(ctx, user.ID, passwords[4], passwords[0], testMeta()); !errors.Is(err, ErrPasswordReused) {
		t.Fatalf("expected 5th recent password to be blocked, got %v", err)
	}

	// Rotate to password[5] so history holds [1,2,3,4,5]; password[0] is now 6th oldest -> allowed!
	if err := svc.ChangePassword(ctx, user.ID, passwords[4], passwords[5], testMeta()); err != nil {
		t.Fatalf("ChangePassword 5 failed: %v", err)
	}
	if err := svc.ChangePassword(ctx, user.ID, passwords[5], passwords[0], testMeta()); err != nil {
		t.Fatalf("expected 6th oldest password reuse to be allowed, got %v", err)
	}

	// Verify audit logs never contain plaintext passwords.
	for _, entry := range store.auditLogs {
		raw := string(entry.Metadata)
		for _, pw := range passwords {
			if strings.Contains(raw, pw) {
				t.Fatalf("audit log leaked password: %s", raw)
			}
		}
	}
}

func TestEmailChangeAndOAuthMockCallback(t *testing.T) {
	svc, store, mailer, _ := newTestService(t)
	ctx := context.Background()

	// Mock GitHub OAuth provider.
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gh-test-token",
				"token_type":   "bearer",
			})
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         4242,
				"login":      "octocat",
				"name":       "Octo Cat",
				"avatar_url": "https://avatars.example/4242.png",
			})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"email": "secondary@example.com", "primary": false, "verified": true},
				{"email": "octo@example.com", "primary": true, "verified": true},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ghServer.Close()

	svc.oauthEndpoints.GitHubAuthURL = ghServer.URL + "/login/oauth/authorize"
	svc.oauthEndpoints.GitHubTokenURL = ghServer.URL + "/login/oauth/access_token"
	svc.oauthEndpoints.GitHubUserURL = ghServer.URL + "/user"
	svc.oauthEndpoints.GitHubEmailsURL = ghServer.URL + "/user/emails"

	providers := svc.EnabledProviders()
	if !slices.Equal(providers, []string{ProviderGitHub, ProviderGoogle}) {
		t.Fatalf("unexpected enabled providers: %v", providers)
	}

	res, err := svc.HandleOAuthCallback(ctx, ProviderGitHub, "mock-code", testMeta())
	if err != nil {
		t.Fatalf("HandleOAuthCallback failed: %v", err)
	}
	if res.User.Email != "octo@example.com" || res.User.HasPassword || res.SessionToken == "" {
		t.Fatalf("unexpected OAuth user result: %+v", res)
	}

	// Change email flow sends token to new address and updates on confirm.
	if err := svc.RequestEmailChange(ctx, res.User.ID, "new-octo@example.com", "", testMeta()); err != nil {
		t.Fatalf("RequestEmailChange failed: %v", err)
	}
	if err := svc.ConfirmEmailChange(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("ConfirmEmailChange failed: %v", err)
	}
	updated, err := store.GetUserByID(ctx, res.User.ID)
	if err != nil || updated.Email != "new-octo@example.com" {
		t.Fatalf("expected updated email new-octo@example.com, got %q err=%v", updated.Email, err)
	}
}

func TestEmailOTP2FAFlow(t *testing.T) {
	svc, _, mailer, _ := newTestService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, "email2fa@example.com", "strong-password-99", "Email 2FA", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	if err := svc.SendEmailOTP(ctx, user.ID); err != nil {
		t.Fatalf("SendEmailOTP failed: %v", err)
	}
	firstCode := mailer.lastPayload()

	// Issuing a new code invalidates the first one.
	if err := svc.SendEmailOTP(ctx, user.ID); err != nil {
		t.Fatalf("second SendEmailOTP failed: %v", err)
	}
	secondCode := mailer.lastPayload()

	if _, err := svc.VerifyAndEnableEmail2FA(ctx, user.ID, firstCode, testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected superseded OTP to fail, got %v", err)
	}

	codes, err := svc.VerifyAndEnableEmail2FA(ctx, user.ID, secondCode, testMeta())
	if err != nil || len(codes) != 10 {
		t.Fatalf("VerifyAndEnableEmail2FA failed: len=%d err=%v", len(codes), err)
	}

	// Login triggers email OTP automatically.
	loginRes, err := svc.Login(ctx, "email2fa@example.com", "strong-password-99", "", testMeta())
	if err != nil || !loginRes.TwoFactorRequired || loginRes.TwoFactorMethod != MethodEmail {
		t.Fatalf("expected email 2FA challenge, got %+v err=%v", loginRes, err)
	}
	loginOTP := mailer.lastPayload()

	// 5 wrong attempts invalidate the active OTP code.
	for i := 0; i < 5; i++ {
		if _, err := svc.VerifyTwoFactorLogin(ctx, user.ID, MethodEmail, "999999", "", false, testMeta()); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected wrong OTP rejected, got %v", err)
		}
	}
	if _, err := svc.VerifyTwoFactorLogin(ctx, user.ID, MethodEmail, loginOTP, "", false, testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected exhausted OTP to be dead even with correct code, got %v", err)
	}
}

func TestAccountDeletionPasswordAnd2FARequirements(t *testing.T) {
	svc, store, mailer, _ := newTestService(t)
	ctx := context.Background()

	// 1. Password user without 2FA.
	user, err := svc.Register(ctx, "del-pw@example.com", "strong-delete-pass-1", "Delete PW", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// Sole owner cannot delete their own account until ownership is transferred.
	if err := svc.RequestAccountDeletion(ctx, user.ID, "strong-delete-pass-1", "", "", "", testMeta()); !errors.Is(err, ErrSoleOwnerCannotDelete) {
		t.Fatalf("expected sole owner deletion blocked with ErrSoleOwnerCannotDelete, got %v", err)
	}

	// Pre-create the OAuth user and a second active owner in the store
	// so `user` is no longer the sole owner and can test credential verification on delete.
	_, _ = store.CreateUser(ctx, sqlc.CreateUserParams{
		Lower: "oauthdel@example.com",
		Name:  "OAuth Del",
		Role:  RoleMember,
	})
	_, _ = store.CreateUser(ctx, sqlc.CreateUserParams{
		Lower:        "second-owner@example.com",
		Name:         "Second Owner",
		Role:         RoleOwner,
		IsSuperadmin: true,
	})

	if err := svc.RequestAccountDeletion(ctx, user.ID, "wrong-password", "", "", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected wrong password rejected, got %v", err)
	}
	if err := svc.RequestAccountDeletion(ctx, user.ID, "strong-delete-pass-1", "", "", "", testMeta()); err != nil {
		t.Fatalf("RequestAccountDeletion failed: %v", err)
	}
	if !strings.Contains(mailer.lastPayload(), "/settings/account/delete/confirm?token=") {
		t.Fatalf("unexpected deletion confirm link: %s", mailer.lastPayload())
	}

	// 2. Enable TOTP 2FA -> RequestAccountDeletion now requires password + TOTP/backup code.
	setup, err := svc.SetupTOTP(ctx, user.ID)
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}
	totpCode, _ := totp.GenerateCode(setup.Secret, time.Now())
	backupCodes, err := svc.VerifyAndEnableTOTP(ctx, user.ID, totpCode, testMeta())
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	// Password alone fails when 2FA is enabled.
	if err := svc.RequestAccountDeletion(ctx, user.ID, "strong-delete-pass-1", "", "", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected missing 2FA rejected, got %v", err)
	}
	// Password + wrong TOTP fails.
	if err := svc.RequestAccountDeletion(ctx, user.ID, "strong-delete-pass-1", "", "000000", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid TOTP rejected, got %v", err)
	}
	// Password + valid backup code succeeds.
	if err := svc.RequestAccountDeletion(ctx, user.ID, "strong-delete-pass-1", "", "", backupCodes[0], testMeta()); err != nil {
		t.Fatalf("RequestAccountDeletion with 2FA backup code failed: %v", err)
	}

	// 3. OAuth-only user (password_hash NULL) uses email OTP (max 5 attempts).
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "bearer"})
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9001, "login": "oauthdel", "name": "OAuth Del"})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"email": "oauthdel@example.com", "primary": true, "verified": true}})
		}
	}))
	defer ghServer.Close()
	svc.oauthEndpoints.GitHubTokenURL = ghServer.URL + "/login/oauth/access_token"
	svc.oauthEndpoints.GitHubUserURL = ghServer.URL + "/user"
	svc.oauthEndpoints.GitHubEmailsURL = ghServer.URL + "/user/emails"

	oauthRes, err := svc.HandleOAuthCallback(ctx, ProviderGitHub, "code", testMeta())
	if err != nil {
		t.Fatalf("HandleOAuthCallback failed: %v", err)
	}
	oauthUserID := oauthRes.User.ID

	if err := svc.RequestAccountDeletion(ctx, oauthUserID, "", "", "", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected OAuth deletion without OTP rejected, got %v", err)
	}

	if err := svc.SendEmailOTP(ctx, oauthUserID); err != nil {
		t.Fatalf("SendEmailOTP failed: %v", err)
	}
	validOTP := mailer.lastPayload()

	// 5 wrong attempts exhaust the email OTP.
	for i := 0; i < MaxEmailOTPAttempts; i++ {
		if err := svc.RequestAccountDeletion(ctx, oauthUserID, "", "000000", "", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected wrong OTP attempt %d rejected, got %v", i+1, err)
		}
	}
	if err := svc.RequestAccountDeletion(ctx, oauthUserID, "", validOTP, "", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected exhausted OTP rejected even with matching code, got %v", err)
	}

	// Fresh email OTP succeeds.
	if err := svc.SendEmailOTP(ctx, oauthUserID); err != nil {
		t.Fatalf("SendEmailOTP failed: %v", err)
	}
	freshOTP := mailer.lastPayload()
	if err := svc.RequestAccountDeletion(ctx, oauthUserID, "", freshOTP, "", "", testMeta()); err != nil {
		t.Fatalf("OAuth RequestAccountDeletion with valid OTP failed: %v", err)
	}
}

func TestAccountDeletionConfirmSingleUseExpirySoftDeleteAndRestore(t *testing.T) {
	svc, store, mailer, now := newTestService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, "softdel@example.com", "soft-delete-pass-12", "Soft Del", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}
	_, _ = store.CreateUser(ctx, sqlc.CreateUserParams{
		Lower:        "backup-owner@example.com",
		Name:         "Backup Owner",
		Role:         RoleOwner,
		IsSuperadmin: true,
	})

	// Enable TOTP + login to create active session and trusted device.
	setup, err := svc.SetupTOTP(ctx, user.ID)
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	backupCodes, err := svc.VerifyAndEnableTOTP(ctx, user.ID, code, testMeta())
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}
	_, err = svc.Login(ctx, "softdel@example.com", "soft-delete-pass-12", "", testMeta())
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	loginRes, err := svc.VerifyTwoFactorLogin(ctx, user.ID, MethodTOTP, "", backupCodes[0], true, testMeta())
	if err != nil {
		t.Fatalf("VerifyTwoFactorLogin failed: %v", err)
	}

	// Seed owned licenses/products/activations to verify they remain frozen on soft delete.
	store.mu.Lock()
	store.ownedResources[user.ID] = []string{"product:1", "license:1", "activation:1"}
	store.mu.Unlock()

	// Request deletion token and let it expire after >24h.
	if err := svc.RequestAccountDeletion(ctx, user.ID, "soft-delete-pass-12", "", "", backupCodes[1], testMeta()); err != nil {
		t.Fatalf("RequestAccountDeletion failed: %v", err)
	}
	expiredTok := mailer.lastToken()
	*now = now.Add(25 * time.Hour)
	if err := svc.ConfirmAccountDeletion(ctx, expiredTok, testMeta()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected expired deletion token rejected, got %v", err)
	}

	// Issue fresh deletion token and confirm within 24h.
	if err := svc.RequestAccountDeletion(ctx, user.ID, "soft-delete-pass-12", "", "", backupCodes[2], testMeta()); err != nil {
		t.Fatalf("second RequestAccountDeletion failed: %v", err)
	}
	confirmTok := mailer.lastToken()
	if err := svc.ConfirmAccountDeletion(ctx, confirmTok, testMeta()); err != nil {
		t.Fatalf("ConfirmAccountDeletion failed: %v", err)
	}

	// Token is single-use.
	if err := svc.ConfirmAccountDeletion(ctx, confirmTok, testMeta()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected reused deletion token rejected, got %v", err)
	}

	// Soft-deleted state checks:
	// - sessions & trusted devices cleared
	// - 2FA cleared (two_factor_enabled=false, totp_secret_encrypted=nil)
	// - status='suspended', deleted_at!=nil
	// - owned resources still frozen (not deleted yet)
	if _, _, err := svc.ValidateSession(ctx, loginRes.SessionToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected session revoked on soft delete, got %v", err)
	}
	store.mu.Lock()
	deletedUser := store.users[user.ID]
	sessCount := len(store.sessions)
	devCount := len(store.trustedDevices)
	ownedCount := len(store.ownedResources[user.ID])
	store.mu.Unlock()

	if deletedUser.DeletedAt == nil || deletedUser.Status != "suspended" {
		t.Fatalf("expected user soft-deleted and suspended, got deleted_at=%v status=%s", deletedUser.DeletedAt, deletedUser.Status)
	}
	if deletedUser.TwoFactorEnabled || len(deletedUser.TotpSecretEncrypted) != 0 {
		t.Fatalf("expected 2FA cleared on soft delete, got enabled=%v secretLen=%d", deletedUser.TwoFactorEnabled, len(deletedUser.TotpSecretEncrypted))
	}
	if sessCount != 0 || devCount != 0 {
		t.Fatalf("expected 0 sessions and 0 trusted devices, got sessions=%d devices=%d", sessCount, devCount)
	}
	if ownedCount != 3 {
		t.Fatalf("expected owned resources frozen until hard delete, got %d", ownedCount)
	}

	// Soft-deleted user gets generic ErrInvalidCredentials (401) on login.
	if _, err := svc.Login(ctx, "softdel@example.com", "soft-delete-pass-12", "", testMeta()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials on soft-deleted login, got %v", err)
	}

	// Cancel / restore within 30 days (<30d).
	*now = now.Add(10 * 24 * time.Hour)
	if err := svc.CancelAccountDeletion(ctx, "softdel@example.com", testMeta()); err != nil {
		t.Fatalf("CancelAccountDeletion failed: %v", err)
	}
	restoreTok := mailer.lastToken()
	if err := svc.RestoreAccount(ctx, restoreTok, testMeta()); err != nil {
		t.Fatalf("RestoreAccount failed: %v", err)
	}
	// Restore token is single-use.
	if err := svc.RestoreAccount(ctx, restoreTok, testMeta()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected reused restore token rejected, got %v", err)
	}

	// User can log in again after restoration.
	restoredLogin, err := svc.Login(ctx, "softdel@example.com", "soft-delete-pass-12", "", testMeta())
	if err != nil || restoredLogin.SessionToken == "" {
		t.Fatalf("expected login after restore to succeed, got %+v err=%v", restoredLogin, err)
	}
}

func TestAccountDeletionHardDeleteJobAndAuditAnonymization(t *testing.T) {
	svc, store, mailer, now := newTestService(t)
	ctx := context.Background()

	// Register initial owner, then seed User A (will exceed 30 days) and User B (will remain <30 days).
	owner, err := svc.Register(ctx, "owner@example.com", "password-owner-123", "Owner", testMeta())
	if err != nil {
		t.Fatalf("Register Owner failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail Owner failed: %v", err)
	}
	_ = owner

	svc.plan = "pro"
	userA, err := svc.AdminCreateUser(ctx, owner.ID, "usera@example.com", "User A", RoleMember, testMeta())
	if err != nil {
		t.Fatalf("AdminCreateUser User A failed: %v", err)
	}
	if err := svc.ResetPassword(ctx, mailer.lastToken(), "password-user-a-123", testMeta()); err != nil {
		t.Fatalf("ResetPassword User A failed: %v", err)
	}
	if err := svc.RequestAccountDeletion(ctx, userA.ID, "password-user-a-123", "", "", "", testMeta()); err != nil {
		t.Fatalf("RequestAccountDeletion User A failed: %v", err)
	}
	if err := svc.ConfirmAccountDeletion(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("ConfirmAccountDeletion User A failed: %v", err)
	}

	store.mu.Lock()
	store.ownedResources[userA.ID] = []string{"product:a", "license:a", "activation:a"}
	store.mu.Unlock()

	// Advance 20 days and soft-delete User B.
	*now = now.Add(20 * 24 * time.Hour)
	userB, err := svc.AdminCreateUser(ctx, owner.ID, "userb@example.com", "User B", RoleMember, testMeta())
	svc.plan = FreePlan
	if err != nil {
		t.Fatalf("AdminCreateUser User B failed: %v", err)
	}
	if err := svc.ResetPassword(ctx, mailer.lastToken(), "password-user-b-123", testMeta()); err != nil {
		t.Fatalf("ResetPassword User B failed: %v", err)
	}
	if err := svc.RequestAccountDeletion(ctx, userB.ID, "password-user-b-123", "", "", "", testMeta()); err != nil {
		t.Fatalf("RequestAccountDeletion User B failed: %v", err)
	}
	if err := svc.ConfirmAccountDeletion(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("ConfirmAccountDeletion User B failed: %v", err)
	}
	store.mu.Lock()
	store.ownedResources[userB.ID] = []string{"product:b"}
	store.mu.Unlock()

	// Advance 11 more days: User A is at 31 days (>30d), User B is at 11 days (<30d).
	*now = now.Add(11 * 24 * time.Hour)

	// User A cannot restore after 30 days.
	sentBefore := len(mailer.sent)
	if err := svc.CancelAccountDeletion(ctx, "usera@example.com", testMeta()); err != nil {
		t.Fatalf("CancelAccountDeletion >30d returned error: %v", err)
	}
	if len(mailer.sent) != sentBefore {
		t.Fatalf("expected no restore email sent after 30 days")
	}

	// Run background worker with a short ticker and stop cleanly.
	workerCtx, workerCancel := context.WithCancel(ctx)
	worker := NewAccountDeletionWorker(svc, svc.logger, 10*time.Millisecond)
	worker.Start(workerCtx)
	time.Sleep(35 * time.Millisecond)
	workerCancel()
	worker.Stop()

	store.mu.Lock()
	_, userAExists := store.users[userA.ID]
	_, userAOwned := store.ownedResources[userA.ID]
	_, userBExists := store.users[userB.ID]
	userBOwned := len(store.ownedResources[userB.ID])
	logs := append([]sqlc.AuthAuditLog(nil), store.auditLogs...)
	store.mu.Unlock()

	if userAExists || userAOwned {
		t.Fatalf("expected User A and owned data hard-deleted after >30d: exists=%v owned=%v", userAExists, userAOwned)
	}
	if !userBExists || userBOwned != 1 {
		t.Fatalf("expected User B (<30d) retained with frozen data: exists=%v owned=%d", userBExists, userBOwned)
	}

	// Verify User A's audit logs are anonymized (user_id=NULL, ip/ua/metadata cleared, event+created_at kept).
	foundDeletedEvent := false
	for _, entry := range logs {
		if entry.UserID.Valid && entry.UserID.UUID == userA.ID {
			t.Fatalf("expected User A audit log user_id anonymized to NULL, found %+v", entry)
		}
		if !entry.UserID.Valid && entry.Event == "account_deleted" {
			foundDeletedEvent = true
			if entry.IpAddress != nil || entry.UserAgent.Valid || len(entry.Metadata) != 0 || entry.CreatedAt.IsZero() {
				t.Fatalf("expected anonymized audit log to clear ip/ua/metadata and keep created_at: %+v", entry)
			}
		}
	}
	if !foundDeletedEvent {
		t.Fatal("expected account_deleted audit event preserved after anonymization")
	}
}

func TestProfileUpdateAndAvatarLifecycle(t *testing.T) {
	svc, store, mailer, _ := newTestService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, "profile@example.com", "strong-profile-pass-1", "Original Name", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// 1. Display name validation: empty, 1 char, and >100 chars rejected; 2-100 chars after trim accepted.
	if _, err := svc.UpdateProfileName(ctx, user.ID, "   ", testMeta()); !errors.Is(err, ErrInvalidDisplayName) {
		t.Fatalf("expected empty trimmed name rejected, got %v", err)
	}
	if _, err := svc.UpdateProfileName(ctx, user.ID, " A ", testMeta()); !errors.Is(err, ErrInvalidDisplayName) {
		t.Fatalf("expected 1-char trimmed name rejected, got %v", err)
	}
	if _, err := svc.UpdateProfileName(ctx, user.ID, strings.Repeat("a", 101), testMeta()); !errors.Is(err, ErrInvalidDisplayName) {
		t.Fatalf("expected 101-char name rejected, got %v", err)
	}

	updatedUser, err := svc.UpdateProfileName(ctx, user.ID, "  Ada Lovelace  ", testMeta())
	if err != nil {
		t.Fatalf("UpdateProfileName failed: %v", err)
	}
	if updatedUser.Name != "Ada Lovelace" {
		t.Fatalf("expected trimmed name 'Ada Lovelace', got %q", updatedUser.Name)
	}

	// 2. Avatar upload MIME sniffing & size checks:
	// - .exe (MZ header) rejected
	exeBytes := append([]byte{'M', 'Z', 0x90, 0x00}, bytes.Repeat([]byte{0x00}, 64)...)
	if _, err := svc.UpdateAvatar(ctx, user.ID, exeBytes, testMeta()); !errors.Is(err, ErrInvalidAvatarType) {
		t.Fatalf("expected .exe bytes rejected with ErrInvalidAvatarType, got %v", err)
	}
	// - > 2 MB rejected
	tooLargePNG := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, MaxAvatarBytes)...)
	if _, err := svc.UpdateAvatar(ctx, user.ID, tooLargePNG, testMeta()); !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("expected >2MB PNG rejected with ErrAvatarTooLarge, got %v", err)
	}

	// - Real non-square PNG (400x300) is resized/cropped to a 256x256 square on disk
	rectImg := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			rectImg.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	var rectPNGBuf bytes.Buffer
	if err := png.Encode(&rectPNGBuf, rectImg); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}
	afterRectPNG, err := svc.UpdateAvatar(ctx, user.ID, rectPNGBuf.Bytes(), testMeta())
	if err != nil {
		t.Fatalf("UpdateAvatar with 400x300 PNG failed: %v", err)
	}
	if afterRectPNG.AvatarURL == nil || !strings.HasPrefix(*afterRectPNG.AvatarURL, AvatarURLPrefix) {
		t.Fatalf("expected relative path avatar_url, got %v", afterRectPNG.AvatarURL)
	}
	rectFile := filepath.Join(svc.UploadDir(), "avatars", strings.TrimPrefix(*afterRectPNG.AvatarURL, AvatarURLPrefix))
	savedImgFile, err := os.Open(rectFile)
	if err != nil {
		t.Fatalf("open resized avatar file failed: %v", err)
	}
	decodedSaved, err := png.Decode(savedImgFile)
	_ = savedImgFile.Close()
	if err != nil {
		t.Fatalf("decode saved PNG failed: %v", err)
	}
	if decodedSaved.Bounds().Dx() != AvatarTargetDimension || decodedSaved.Bounds().Dy() != AvatarTargetDimension {
		t.Fatalf("expected saved PNG resized to %dx%d, got %dx%d", AvatarTargetDimension, AvatarTargetDimension, decodedSaved.Bounds().Dx(), decodedSaved.Bounds().Dy())
	}

	// - Valid 500 KB PNG accepted and stored on disk (and deletes previous rectFile)
	validPNG := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 500*1024-8)...)
	afterPNG, err := svc.UpdateAvatar(ctx, user.ID, validPNG, testMeta())
	if err != nil {
		t.Fatalf("UpdateAvatar with 500 KB PNG failed: %v", err)
	}
	if _, statErr := os.Stat(rectFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected previous avatar %s deleted on replacement, statErr=%v", rectFile, statErr)
	}
	if afterPNG.AvatarURL == nil || !strings.HasPrefix(*afterPNG.AvatarURL, AvatarURLPrefix) || !strings.HasSuffix(*afterPNG.AvatarURL, ".png") {
		t.Fatalf("unexpected avatar_url after PNG upload: %v", afterPNG.AvatarURL)
	}
	firstFile := filepath.Join(svc.UploadDir(), "avatars", strings.TrimPrefix(*afterPNG.AvatarURL, AvatarURLPrefix))
	if _, statErr := os.Stat(firstFile); statErr != nil {
		t.Fatalf("expected uploaded PNG file to exist at %s: %v", firstFile, statErr)
	}

	// - Replacing with WebP deletes the old PNG file and writes new WebP file
	validWebP := append([]byte{'R', 'I', 'F', 'F', 0x24, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}, make([]byte, 128)...)
	afterWebP, err := svc.UpdateAvatar(ctx, user.ID, validWebP, testMeta())
	if err != nil {
		t.Fatalf("UpdateAvatar with WebP failed: %v", err)
	}
	if afterWebP.AvatarURL == nil || !strings.HasSuffix(*afterWebP.AvatarURL, ".webp") {
		t.Fatalf("unexpected avatar_url after WebP upload: %v", afterWebP.AvatarURL)
	}
	if _, statErr := os.Stat(firstFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected old avatar file %s to be deleted upon replacement, statErr=%v", firstFile, statErr)
	}
	secondFile := filepath.Join(svc.UploadDir(), "avatars", strings.TrimPrefix(*afterWebP.AvatarURL, AvatarURLPrefix))
	if _, statErr := os.Stat(secondFile); statErr != nil {
		t.Fatalf("expected new WebP file to exist at %s: %v", secondFile, statErr)
	}

	// 3. DeleteAvatar removes file from disk and sets avatar_url to NULL.
	afterDelete, err := svc.DeleteAvatar(ctx, user.ID, testMeta())
	if err != nil {
		t.Fatalf("DeleteAvatar failed: %v", err)
	}
	if afterDelete.AvatarURL != nil {
		t.Fatalf("expected nil AvatarURL after DeleteAvatar, got %v", *afterDelete.AvatarURL)
	}
	if _, statErr := os.Stat(secondFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected avatar file %s deleted after DeleteAvatar, statErr=%v", secondFile, statErr)
	}

	// 4. Verify auth_audit_log events recorded for profile_updated, avatar_updated, avatar_removed.
	store.mu.Lock()
	var events []string
	for _, entry := range store.auditLogs {
		events = append(events, entry.Event)
	}
	store.mu.Unlock()
	for _, expected := range []string{"profile_updated", "avatar_updated", "avatar_removed"} {
		if !slices.Contains(events, expected) {
			t.Fatalf("expected audit log event %q in %v", expected, events)
		}
	}
}

func TestRBACPermissionsUserManagementAndRegistrationRules(t *testing.T) {
	svc, store, mailer, _ := newTestService(t)
	ctx := context.Background()

	// 1. Permission map checks per role + superadmin flag.
	viewerPerms := PermissionsFor(RoleViewer, false)
	if !slices.Equal(viewerPerms, []string{PermProductsRead, PermLicensesRead, PermActivationsRead}) {
		t.Fatalf("unexpected viewer permissions: %v", viewerPerms)
	}
	memberPerms := PermissionsFor(RoleMember, false)
	for _, p := range []string{PermProductsRead, PermLicensesCreate, PermLicensesRead, PermLicensesUpdate, PermLicensesRevoke, PermActivationsCreate, PermActivationsRead, PermActivationsRevoke} {
		if !slices.Contains(memberPerms, p) {
			t.Fatalf("member missing permission %q: %v", p, memberPerms)
		}
	}
	for _, forbidden := range []string{PermProductsDelete, PermLicensesDelete, PermUsersRead, PermUsersInvite, PermSettingsRead, PermAuditRead, PermRolesManage} {
		if slices.Contains(memberPerms, forbidden) {
			t.Fatalf("member must not have %q: %v", forbidden, memberPerms)
		}
	}
	adminPerms := PermissionsFor(RoleAdmin, false)
	for _, p := range []string{PermProductsCreate, PermProductsRead, PermProductsUpdate, PermLicensesCreate, PermLicensesDelete, PermActivationsCreate, PermActivationsRead, PermActivationsRevoke, PermUsersInvite, PermUsersRead, PermUsersUpdate, PermUsersRemove, PermUsersSuspend, PermSettingsRead, PermAuditRead} {
		if !slices.Contains(adminPerms, p) {
			t.Fatalf("admin missing permission %q: %v", p, adminPerms)
		}
	}
	for _, forbidden := range []string{PermProductsDelete, PermRolesManage, PermSettingsUpdate} {
		if slices.Contains(adminPerms, forbidden) {
			t.Fatalf("admin must not have %q: %v", forbidden, adminPerms)
		}
	}
	ownerPerms := PermissionsFor(RoleOwner, false)
	if !slices.Equal(ownerPerms, allPermissions) {
		t.Fatalf("expected owner to have allPermissions, got %v", ownerPerms)
	}
	superPerms := PermissionsFor(RoleViewer, true)
	if !slices.Equal(superPerms, allPermissions) {
		t.Fatalf("expected superadmin flag to grant allPermissions regardless of role, got %v", superPerms)
	}

	// 2. First user registered becomes role='owner' AND is_superadmin=TRUE.
	firstUser, err := svc.Register(ctx, "founder@example.com", "founder-pass-123", "Founder", testMeta())
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if firstUser.Role != RoleOwner || !firstUser.IsSuperadmin || !slices.Equal(firstUser.Permissions, allPermissions) {
		t.Fatalf("expected first user to be owner + superadmin with all permissions, got role=%s super=%v perms=%v", firstUser.Role, firstUser.IsSuperadmin, firstUser.Permissions)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// 3. Subsequent public & OAuth registrations create independent non-superadmin owners with their own Free workspace.
	secondUser, err := svc.Register(ctx, "second@example.com", "second-pass-123", "Second Owner", testMeta())
	if err != nil {
		t.Fatalf("second public Register failed: %v", err)
	}
	if secondUser.IsSuperadmin || secondUser.Role != RoleOwner {
		t.Fatalf("expected second user to be non-superadmin owner, got role=%s super=%v", secondUser.Role, secondUser.IsSuperadmin)
	}
	secondWorkspaces, err := svc.ListUserWorkspaces(ctx, secondUser.ID, false)
	if err != nil || len(secondWorkspaces) != 1 || secondWorkspaces[0].Plan != FreePlan {
		t.Fatalf("expected second user to auto-provision 1 free workspace, got err=%v wss=%+v", err, secondWorkspaces)
	}

	// OAuth registration for an unknown email after first user exists also creates a tenant owner + Free workspace.
	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/oauth/access_token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "token_type": "bearer"})
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7777, "login": "newgh", "name": "New GH"})
		case "/user/emails":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"email": "newgh@example.com", "primary": true, "verified": true}})
		}
	}))
	defer ghServer.Close()
	svc.oauthEndpoints.GitHubTokenURL = ghServer.URL + "/login/oauth/access_token"
	svc.oauthEndpoints.GitHubUserURL = ghServer.URL + "/user"
	svc.oauthEndpoints.GitHubEmailsURL = ghServer.URL + "/user/emails"

	oauthLogin, err := svc.HandleOAuthCallback(ctx, ProviderGitHub, "code", testMeta())
	if err != nil {
		t.Fatalf("expected OAuth self-registration to succeed with personal workspace, got %v", err)
	}
	if oauthLogin.User == nil || oauthLogin.User.IsSuperadmin {
		t.Fatalf("expected OAuth user to be non-superadmin, got %+v", oauthLogin.User)
	}
	oauthWorkspaces, err := svc.ListUserWorkspaces(ctx, oauthLogin.User.ID, false)
	if err != nil || len(oauthWorkspaces) != 1 || oauthWorkspaces[0].Plan != FreePlan {
		t.Fatalf("expected OAuth user to auto-provision 1 free workspace, got err=%v wss=%+v", err, oauthWorkspaces)
	}

	// 4. Superadmin bypasses Free plan invite limits, while non-superadmin owner/admin are locked (403 ErrInviteRequiresPaidPlan).
	nonSuperOwner, _ := store.CreateUser(ctx, sqlc.CreateUserParams{
		Lower:        "nonsuperowner@example.com",
		Name:         "Non Super Owner",
		Role:         RoleOwner,
		IsSuperadmin: false,
	})
	defaultWS, _ := store.GetFirstActiveWorkspace(ctx)
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
		WorkspaceID: defaultWS.ID,
		UserID:      nonSuperOwner.ID,
		Role:        RoleOwner,
	})
	countBeforeBlocked, _ := store.CountAllUsers(ctx)
	if _, err := svc.AdminCreateUser(ctx, nonSuperOwner.ID, "blocked@example.com", "Blocked", RoleMember, testMeta()); !errors.Is(err, ErrInviteRequiresPaidPlan) {
		t.Fatalf("expected AdminCreateUser on free plan by non-superadmin owner rejected with ErrInviteRequiresPaidPlan, got %v", err)
	}
	if count, _ := store.CountAllUsers(ctx); count != countBeforeBlocked {
		t.Fatalf("expected no user created when invite rejected on free plan, got count=%d want=%d", count, countBeforeBlocked)
	}
	// Clean up temporary nonSuperOwner before continuing single-owner checks.
	store.mu.Lock()
	delete(store.users, nonSuperOwner.ID)
	delete(store.workspaceMembers, defaultWS.ID.String()+":"+nonSuperOwner.ID.String())
	store.mu.Unlock()

	// Superadmin on free plan cannot invite owner or superadmin, even though plan limits are bypassed.
	if _, err := svc.AdminCreateUser(ctx, firstUser.ID, "badowner@example.com", "Bad Owner", RoleOwner, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected AdminCreateUser with role=owner rejected with ErrForbidden, got %v", err)
	}
	if _, err := svc.AdminCreateUser(ctx, firstUser.ID, "badsuper@example.com", "Bad Super", "superadmin", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected AdminCreateUser with role=superadmin rejected with ErrForbidden, got %v", err)
	}

	// Superadmin invites admin, member, and viewer while still on FreePlan (pushing above 1-seat limit).
	adminDTO, err := svc.AdminCreateUser(ctx, firstUser.ID, "admin@example.com", "Admin User", RoleAdmin, testMeta())
	if err != nil {
		t.Fatalf("AdminCreateUser admin by superadmin on free plan failed: %v", err)
	}
	if err := svc.ResetPassword(ctx, mailer.lastToken(), "admin-pass-123", testMeta()); err != nil {
		t.Fatalf("admin ResetPassword failed: %v", err)
	}

	// Non-superadmin admin on free plan is blocked from inviting.
	if _, err := svc.AdminCreateUser(ctx, adminDTO.ID, "blocked-by-admin@example.com", "Blocked Admin", RoleMember, testMeta()); !errors.Is(err, ErrInviteRequiresPaidPlan) {
		t.Fatalf("expected AdminCreateUser on free plan by non-superadmin admin rejected with ErrInviteRequiresPaidPlan, got %v", err)
	}

	memberDTO, err := svc.AdminCreateUser(ctx, firstUser.ID, "member@example.com", "Member User", RoleMember, testMeta())
	if err != nil {
		t.Fatalf("AdminCreateUser member failed: %v", err)
	}
	if err := svc.ResetPassword(ctx, mailer.lastToken(), "member-pass-123", testMeta()); err != nil {
		t.Fatalf("member ResetPassword failed: %v", err)
	}

	viewerDTO, err := svc.AdminCreateUser(ctx, firstUser.ID, "viewer@example.com", "Viewer User", RoleViewer, testMeta())
	if err != nil {
		t.Fatalf("AdminCreateUser viewer failed: %v", err)
	}
	if err := svc.ResetPassword(ctx, mailer.lastToken(), "viewer-pass-123", testMeta()); err != nil {
		t.Fatalf("viewer ResetPassword failed: %v", err)
	}

	// 5. Role change rules:
	// - Admin cannot change roles (403).
	if _, err := svc.AdminUpdateUserRole(ctx, adminDTO.ID, memberDTO.ID, RoleViewer, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected admin role change rejected with ErrForbidden, got %v", err)
	}
	// - Owner cannot change own role (403).
	if _, err := svc.AdminUpdateUserRole(ctx, firstUser.ID, firstUser.ID, RoleAdmin, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected self role change rejected with ErrForbidden, got %v", err)
	}
	// - Cannot promote to owner/superadmin via endpoint (403).
	if _, err := svc.AdminUpdateUserRole(ctx, firstUser.ID, memberDTO.ID, RoleOwner, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected role change to owner rejected with ErrForbidden, got %v", err)
	}
	// - Owner can change member -> viewer -> member.
	changed, err := svc.AdminUpdateUserRole(ctx, firstUser.ID, memberDTO.ID, RoleViewer, testMeta())
	if err != nil || changed.Role != RoleViewer {
		t.Fatalf("AdminUpdateUserRole failed: %+v err=%v", changed, err)
	}
	changed, err = svc.AdminUpdateUserRole(ctx, firstUser.ID, memberDTO.ID, RoleMember, testMeta())
	if err != nil || changed.Role != RoleMember {
		t.Fatalf("AdminUpdateUserRole restore failed: %+v err=%v", changed, err)
	}

	// 6. Suspension & activation rules:
	// - Member logs in and gets an active session.
	memberLogin, err := svc.Login(ctx, "member@example.com", "member-pass-123", "", testMeta())
	if err != nil || memberLogin.SessionToken == "" {
		t.Fatalf("member Login failed: %v", err)
	}
	// - Suspending self is 403.
	if _, err := svc.AdminSuspendUser(ctx, firstUser.ID, firstUser.ID, "self suspend attempt", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected suspending self rejected with ErrForbidden, got %v", err)
	}
	// - Admin suspending an owner / superadmin is 403.
	if _, err := svc.AdminSuspendUser(ctx, adminDTO.ID, firstUser.ID, "admin vs owner", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected admin suspending owner rejected with ErrForbidden, got %v", err)
	}
	// - Admin suspending another admin is 403.
	secondAdmin, err := svc.AdminCreateUser(ctx, firstUser.ID, "admin2@example.com", "Admin Two", RoleAdmin, testMeta())
	svc.plan = FreePlan
	if err != nil {
		t.Fatalf("create secondAdmin failed: %v", err)
	}
	if _, err := svc.AdminSuspendUser(ctx, adminDTO.ID, secondAdmin.ID, "admin vs admin", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected admin suspending admin rejected with ErrForbidden, got %v", err)
	}
	// - Suspending the sole owner is 403 even for a superadmin.
	soleOwnerNonSuper, _ := store.CreateUser(ctx, sqlc.CreateUserParams{
		Lower:        "owner2@example.com",
		Name:         "Owner Two",
		Role:         RoleOwner,
		IsSuperadmin: false,
	})
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
		WorkspaceID: defaultWS.ID,
		UserID:      soleOwnerNonSuper.ID,
		Role:        RoleOwner,
	})
	if _, err := svc.AdminSuspendUser(ctx, firstUser.ID, soleOwnerNonSuper.ID, "suspend sole owner", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected suspending the sole owner rejected with ErrForbidden, got %v", err)
	}
	if _, err := svc.AdminUpdateUserRole(ctx, firstUser.ID, soleOwnerNonSuper.ID, RoleMember, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected demoting the sole owner rejected with ErrForbidden, got %v", err)
	}

	// - Empty or reason shorter than 3 chars is rejected.
	if _, err := svc.AdminSuspendUser(ctx, adminDTO.ID, memberDTO.ID, "", testMeta()); !errors.Is(err, ErrInvalidSuspendReason) {
		t.Fatalf("expected empty suspend reason rejected, got %v", err)
	}
	if _, err := svc.AdminSuspendUser(ctx, adminDTO.ID, memberDTO.ID, "ab", testMeta()); !errors.Is(err, ErrInvalidSuspendReason) {
		t.Fatalf("expected short suspend reason rejected, got %v", err)
	}

	// - Admin suspends member with valid reason: workspace membership status='suspended', global account & login preserved (BR-04),
	//   access to defaultWS revoked while other workspaces remain accessible, and audit logged.
	suspendedMember, err := svc.AdminSuspendUser(ctx, adminDTO.ID, memberDTO.ID, "policy violation", testMeta())
	if err != nil {
		t.Fatalf("AdminSuspendUser failed: %v", err)
	}
	if suspendedMember.Status != "suspended" || suspendedMember.SuspendedReason == nil || *suspendedMember.SuspendedReason != "policy violation" || suspendedMember.SuspendedAt == nil {
		t.Fatalf("unexpected suspended user state: %+v", suspendedMember)
	}
	if validatedUser, _, valErr := svc.ValidateSession(ctx, memberLogin.SessionToken); valErr != nil || validatedUser == nil {
		t.Fatalf("expected global session preserved on workspace-level suspend (BR-04), got user=%+v err=%v", validatedUser, valErr)
	} else {
		if _, wsErr := svc.ResolveWorkspaceContext(ctx, validatedUser, &defaultWS.ID); !errors.Is(wsErr, ErrForbidden) {
			t.Fatalf("expected suspended workspace access revoked with ErrForbidden, got %v", wsErr)
		}
	}
	if _, err := svc.Login(ctx, "member@example.com", "member-pass-123", "", testMeta()); err != nil {
		t.Fatalf("expected globally active user able to log in after workspace-only suspend (BR-04), got %v", err)
	}

	// - Superadmin suspends second owner (when >=2 owners exist); non-superadmin cannot reactivate suspended owner.
	coOwner, _ := store.CreateUser(ctx, sqlc.CreateUserParams{
		Lower:        "coowner@example.com",
		Name:         "Co Owner",
		Role:         RoleOwner,
		IsSuperadmin: false,
	})
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{
		WorkspaceID: defaultWS.ID,
		UserID:      coOwner.ID,
		Role:        RoleOwner,
	})
	suspendedOwner2, err := svc.AdminSuspendUser(ctx, firstUser.ID, soleOwnerNonSuper.ID, "security review", testMeta())
	if err != nil || suspendedOwner2.Status != "suspended" {
		t.Fatalf("expected superadmin to suspend second owner, got %+v err=%v", suspendedOwner2, err)
	}
	if _, err := svc.AdminActivateUser(ctx, adminDTO.ID, soleOwnerNonSuper.ID, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected non-superadmin blocked from activating suspended owner, got %v", err)
	}
	activatedOwner2, err := svc.AdminActivateUser(ctx, firstUser.ID, soleOwnerNonSuper.ID, testMeta())
	if err != nil || activatedOwner2.Status != "active" || activatedOwner2.SuspendedReason != nil || activatedOwner2.SuspendedAt != nil {
		t.Fatalf("expected superadmin to reactivate owner, got %+v err=%v", activatedOwner2, err)
	}

	// - Admin reactivates suspended member -> member can log in again.
	activatedMember, err := svc.AdminActivateUser(ctx, adminDTO.ID, memberDTO.ID, testMeta())
	if err != nil || activatedMember.Status != "active" || activatedMember.SuspendedReason != nil || activatedMember.SuspendedAt != nil {
		t.Fatalf("AdminActivateUser member failed: %+v err=%v", activatedMember, err)
	}
	if _, err := svc.Login(ctx, "member@example.com", "member-pass-123", "", testMeta()); err != nil {
		t.Fatalf("expected reactivated member to log in, got %v", err)
	}

	// - Verify audit log recorded MEMBER_SUSPENDED (with reason) and MEMBER_REACTIVATED.
	store.mu.Lock()
	var foundSuspendLog, foundActivateLog bool
	for _, logEntry := range store.auditLogs {
		if logEntry.Event == AuditEventMemberSuspended && strings.Contains(string(logEntry.Metadata), "policy violation") {
			foundSuspendLog = true
		}
		if logEntry.Event == AuditEventMemberReactivated {
			foundActivateLog = true
		}
	}
	store.mu.Unlock()
	if !foundSuspendLog || !foundActivateLog {
		t.Fatalf("expected MEMBER_SUSPENDED and MEMBER_REACTIVATED audit logs, got suspend=%v activate=%v", foundSuspendLog, foundActivateLog)
	}

	_ = viewerDTO
}

func TestExistingOwnerInvitedPendingAndLeaveRemove(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	// Bootstrap superadmin + two tenant owners (each gets their own Free workspace).
	superUser, err := svc.Register(ctx, "super@example.com", "super-pass-123", "Super", testMeta())
	if err != nil {
		t.Fatalf("super Register failed: %v", err)
	}
	ownerA, err := svc.Register(ctx, "ownera@example.com", "ownera-pass-123", "Owner A", testMeta())
	if err != nil {
		t.Fatalf("ownerA Register failed: %v", err)
	}
	ownerB, err := svc.Register(ctx, "ownerb@example.com", "ownerb-pass-123", "Owner B", testMeta())
	if err != nil {
		t.Fatalf("ownerB Register failed: %v", err)
	}

	wsAList, _ := svc.ListUserWorkspaces(ctx, ownerA.ID, false)
	wsBList, _ := svc.ListUserWorkspaces(ctx, ownerB.ID, false)
	if len(wsAList) != 1 || len(wsBList) != 1 {
		t.Fatalf("expected 1 personal workspace each, got A=%d B=%d", len(wsAList), len(wsBList))
	}
	wsA := wsAList[0]
	wsB := wsBList[0]

	// Upgrade Workspace B to Pro so Owner B can invite existing Owner A.
	if _, err := svc.UpdateWorkspacePlanAndOverrides(ctx, superUser.ID, wsB.ID, ProPlan, nil, nil, nil, nil, "", testMeta()); err != nil {
		t.Fatalf("upgrade wsB failed: %v", err)
	}

	// Owner B invites existing Owner A (without needing name) -> status is pending.
	invited, err := svc.AdminCreateUserInWorkspace(ctx, wsB.ID, ownerB.ID, ownerA.Email, "", RoleMember, testMeta())
	if err != nil {
		t.Fatalf("invite existing ownerA failed: %v", err)
	}
	if invited.Status != "pending" || invited.Role != RoleMember {
		t.Fatalf("expected pending member invite, got status=%s role=%s", invited.Status, invited.Role)
	}

	// Owner A does not see wsB in active workspaces yet, but sees it in pending invites.
	wsABefore, _ := svc.ListUserWorkspaces(ctx, ownerA.ID, false)
	if len(wsABefore) != 1 {
		t.Fatalf("expected pending invite not yet active in ListUserWorkspaces, got %d", len(wsABefore))
	}
	invites, err := svc.ListPendingWorkspaceInvites(ctx, ownerA.ID)
	if err != nil || len(invites) != 1 || invites[0].WorkspaceID != wsB.ID {
		t.Fatalf("expected 1 pending invite for wsB, got err=%v invites=%+v", err, invites)
	}

	// Owner A accepts invite -> now active member in wsB while still owner in wsA.
	if _, err := svc.AcceptWorkspaceInvite(ctx, ownerA.ID, uuid.Nil, wsB.ID, testMeta()); err != nil {
		t.Fatalf("AcceptWorkspaceInvite failed: %v", err)
	}
	wsAAfter, _ := svc.ListUserWorkspaces(ctx, ownerA.ID, false)
	if len(wsAAfter) != 2 {
		t.Fatalf("expected 2 active workspaces after accepting invite, got %d", len(wsAAfter))
	}

	// Sole owner cannot leave their own workspace (wsA), but can leave wsB where they are member.
	if err := svc.LeaveWorkspace(ctx, ownerA.ID, uuid.Nil, wsA.ID, testMeta()); !errors.Is(err, ErrSoleOwnerCannotLeave) {
		t.Fatalf("expected ErrSoleOwnerCannotLeave for wsA, got %v", err)
	}
	if err := svc.LeaveWorkspace(ctx, ownerA.ID, uuid.Nil, wsB.ID, testMeta()); err != nil {
		t.Fatalf("expected ownerA to leave wsB, got %v", err)
	}

	// Owner B re-invites Owner A, then removes (cancels) the pending invite via AdminRemoveUserInWorkspace.
	if _, err := svc.AdminCreateUserInWorkspace(ctx, wsB.ID, ownerB.ID, ownerA.Email, "", RoleViewer, testMeta()); err != nil {
		t.Fatalf("re-invite ownerA failed: %v", err)
	}
	if err := svc.AdminRemoveUserInWorkspace(ctx, wsB.ID, ownerB.ID, ownerA.ID, testMeta()); err != nil {
		t.Fatalf("AdminRemoveUserInWorkspace failed: %v", err)
	}
	invitesAfterRemove, _ := svc.ListPendingWorkspaceInvites(ctx, ownerA.ID)
	if len(invitesAfterRemove) != 0 {
		t.Fatalf("expected 0 pending invites after cancel/remove, got %d", len(invitesAfterRemove))
	}
}

func TestWorkspaceAdminRemoveTargetGuardsAndIsolation(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	ctx := context.Background()

	superUser, err := svc.Register(ctx, "super-adm-rm@example.com", "super-pass-123", "Super", testMeta())
	if err != nil {
		t.Fatalf("super Register failed: %v", err)
	}
	ownerA, err := svc.Register(ctx, "ownera-rm@example.com", "ownera-pass-123", "Owner A", testMeta())
	if err != nil {
		t.Fatalf("ownerA Register failed: %v", err)
	}
	ownerB, err := svc.Register(ctx, "ownerb-rm@example.com", "ownerb-pass-123", "Owner B", testMeta())
	if err != nil {
		t.Fatalf("ownerB Register failed: %v", err)
	}

	wsAList, _ := svc.ListUserWorkspaces(ctx, ownerA.ID, false)
	wsBList, _ := svc.ListUserWorkspaces(ctx, ownerB.ID, false)
	wsA := wsAList[0]
	wsB := wsBList[0]

	if _, err := svc.UpdateWorkspacePlanAndOverrides(ctx, superUser.ID, wsA.ID, ProPlan, nil, nil, nil, nil, "", testMeta()); err != nil {
		t.Fatalf("upgrade wsA failed: %v", err)
	}
	if _, err := svc.UpdateWorkspacePlanAndOverrides(ctx, superUser.ID, wsB.ID, ProPlan, nil, nil, nil, nil, "", testMeta()); err != nil {
		t.Fatalf("upgrade wsB failed: %v", err)
	}

	// Seed active members in wsA: admin1, admin2, member1, viewer1.
	admin1, err := svc.AdminCreateUserInWorkspace(ctx, wsA.ID, ownerA.ID, "admin1-rm@example.com", "Admin One", RoleAdmin, testMeta())
	if err != nil {
		t.Fatalf("create admin1 failed: %v", err)
	}
	admin2, err := svc.AdminCreateUserInWorkspace(ctx, wsA.ID, ownerA.ID, "admin2-rm@example.com", "Admin Two", RoleAdmin, testMeta())
	if err != nil {
		t.Fatalf("create admin2 failed: %v", err)
	}
	member1, err := svc.AdminCreateUserInWorkspace(ctx, wsA.ID, ownerA.ID, "member1-rm@example.com", "Member One", RoleMember, testMeta())
	if err != nil {
		t.Fatalf("create member1 failed: %v", err)
	}
	viewer1, err := svc.AdminCreateUserInWorkspace(ctx, wsA.ID, ownerA.ID, "viewer1-rm@example.com", "Viewer One", RoleViewer, testMeta())
	if err != nil {
		t.Fatalf("create viewer1 failed: %v", err)
	}

	// Also invite admin1 into wsB as member (accepted) and seed targetB as viewer in wsB.
	if _, err := svc.AdminCreateUserInWorkspace(ctx, wsB.ID, ownerB.ID, admin1.Email, "", RoleMember, testMeta()); err != nil {
		t.Fatalf("invite admin1 to wsB failed: %v", err)
	}
	if _, err := svc.AcceptWorkspaceInvite(ctx, admin1.ID, uuid.Nil, wsB.ID, testMeta()); err != nil {
		t.Fatalf("admin1 accept wsB invite failed: %v", err)
	}
	targetB, err := svc.AdminCreateUserInWorkspace(ctx, wsB.ID, ownerB.ID, "targetb-rm@example.com", "Target B", RoleViewer, testMeta())
	if err != nil {
		t.Fatalf("create targetB failed: %v", err)
	}

	// admin1 cannot remove ownerA or fellow admin2 in wsA.
	if err := svc.AdminRemoveUserCheckInWorkspace(ctx, wsA.ID, admin1.ID, ownerA.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin1 check removing ownerA, got %v", err)
	}
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, admin1.ID, ownerA.ID, "nope", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin1 removing ownerA, got %v", err)
	}
	if err := svc.AdminRemoveUserCheckInWorkspace(ctx, wsA.ID, admin1.ID, admin2.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin1 check removing admin2, got %v", err)
	}
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, admin1.ID, admin2.ID, "nope", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin1 removing admin2, got %v", err)
	}

	// admin1 cannot remove targetB in wsB (where admin1 is only a member).
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsB.ID, admin1.ID, targetB.ID, "cross-ws", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin1 removing targetB in wsB, got %v", err)
	}

	// admin1 CAN remove member1 and viewer1 in wsA.
	if err := svc.AdminRemoveUserCheckInWorkspace(ctx, wsA.ID, admin1.ID, member1.ID); err != nil {
		t.Fatalf("expected admin1 check removing member1 to succeed, got %v", err)
	}
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, admin1.ID, member1.ID, "offboarded", testMeta()); err != nil {
		t.Fatalf("expected admin1 removing member1 to succeed, got %v", err)
	}
	if err := svc.AdminRemoveUserInWorkspaceWithReason(ctx, wsA.ID, admin1.ID, viewer1.ID, "offboarded", testMeta()); err != nil {
		t.Fatalf("expected admin1 removing viewer1 to succeed, got %v", err)
	}

	// Verify member1 and viewer1 are removed in wsA while targetB in wsB is still active.
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: member1.ID}); err != nil || m.Status != "removed" {
		t.Fatalf("expected member1 removed in wsA, got err=%v status=%s", err, m.Status)
	}
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: viewer1.ID}); err != nil || m.Status != "removed" {
		t.Fatalf("expected viewer1 removed in wsA, got err=%v status=%s", err, m.Status)
	}
	if m, err := store.GetWorkspaceMember(ctx, sqlc.GetWorkspaceMemberParams{WorkspaceID: wsB.ID, UserID: targetB.ID}); err != nil || m.Status != "active" {
		t.Fatalf("expected targetB still active in wsB, got err=%v status=%s", err, m.Status)
	}
}

func TestWorkspaceUpdateDeleteAndOwnershipTransfer(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	superUser, err := svc.Register(ctx, "super-ws-ops@example.com", "super-pass-123", "Super", testMeta())
	if err != nil {
		t.Fatalf("super Register failed: %v", err)
	}
	owner, err := svc.Register(ctx, "owner-ws-ops@example.com", "owner-pass-123", "Owner", testMeta())
	if err != nil {
		t.Fatalf("owner Register failed: %v", err)
	}
	otherOwner, err := svc.Register(ctx, "other-ws-ops@example.com", "other-pass-123", "Other", testMeta())
	if err != nil {
		t.Fatalf("otherOwner Register failed: %v", err)
	}

	wsList, _ := svc.ListUserWorkspaces(ctx, owner.ID, false)
	otherList, _ := svc.ListUserWorkspaces(ctx, otherOwner.ID, false)
	ws := wsList[0]
	otherWS := otherList[0]

	if _, err := svc.UpdateWorkspacePlanAndOverrides(ctx, superUser.ID, ws.ID, ProPlan, nil, nil, nil, nil, "", testMeta()); err != nil {
		t.Fatalf("upgrade ws failed: %v", err)
	}

	adminUser, err := svc.AdminCreateUserInWorkspace(ctx, ws.ID, owner.ID, "admin-ws-ops@example.com", "Admin Ops", RoleAdmin, testMeta())
	if err != nil {
		t.Fatalf("create adminUser failed: %v", err)
	}

	// Non-owner admin cannot update, transfer ownership, or delete workspace.
	if _, err := svc.UpdateWorkspace(ctx, ws.ID, adminUser.ID, "Hacked", "hacked-slug", testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin updating workspace, got %v", err)
	}
	if _, err := svc.TransferWorkspaceOwnership(ctx, ws.ID, adminUser.ID, adminUser.ID, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin transferring ownership, got %v", err)
	}
	if err := svc.DeleteWorkspace(ctx, ws.ID, adminUser.ID, testMeta()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for admin deleting workspace, got %v", err)
	}

	// Owner cannot update slug to an existing workspace's slug.
	if _, err := svc.UpdateWorkspace(ctx, ws.ID, owner.ID, "Renamed WS", otherWS.Slug, testMeta()); !errors.Is(err, ErrConflictWorkspaceSlug) {
		t.Fatalf("expected ErrConflictWorkspaceSlug, got %v", err)
	}

	// Owner can update name and slug.
	updated, err := svc.UpdateWorkspace(ctx, ws.ID, owner.ID, "Renamed WS", "renamed-ws-ops", testMeta())
	if err != nil {
		t.Fatalf("UpdateWorkspace failed: %v", err)
	}
	if updated.Name != "Renamed WS" || updated.Slug != "renamed-ws-ops" {
		t.Fatalf("unexpected updated workspace: %+v", updated)
	}

	// Owner transfers ownership to adminUser -> owner becomes admin and can now leave workspace.
	if _, err := svc.TransferWorkspaceOwnership(ctx, ws.ID, owner.ID, adminUser.ID, testMeta()); err != nil {
		t.Fatalf("TransferWorkspaceOwnership failed: %v", err)
	}
	if err := svc.LeaveWorkspace(ctx, owner.ID, uuid.Nil, ws.ID, testMeta()); err != nil {
		t.Fatalf("expected previous owner (now admin) to leave workspace after transfer, got %v", err)
	}

	// New owner (adminUser) can delete workspace.
	if err := svc.DeleteWorkspace(ctx, ws.ID, adminUser.ID, testMeta()); err != nil {
		t.Fatalf("DeleteWorkspace failed: %v", err)
	}
	remaining, err := svc.ListUserWorkspaces(ctx, adminUser.ID, false)
	if err != nil {
		t.Fatalf("ListUserWorkspaces failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected 0 workspaces after delete, got %d", len(remaining))
	}
}
