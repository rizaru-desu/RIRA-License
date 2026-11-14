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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
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
	mu             sync.Mutex
	now            func() time.Time
	users          map[uuid.UUID]sqlc.User
	oauthAccounts  map[string]sqlc.OauthAccount
	sessions       map[uuid.UUID]sqlc.Session
	verTokens      map[uuid.UUID]sqlc.VerificationToken
	pwHistory      []sqlc.PasswordHistory
	pwSeq          int64
	trustedDevices map[uuid.UUID]sqlc.TrustedDevice
	backupCodes    map[uuid.UUID]sqlc.BackupCode
	emailOTPs      []sqlc.EmailOtpCode
	auditLogs      []sqlc.AuthAuditLog
	auditSeq       int64
	ownedResources map[uuid.UUID][]string
}

func newMemStore(now func() time.Time) *memStore {
	return &memStore{
		now:            now,
		users:          make(map[uuid.UUID]sqlc.User),
		oauthAccounts:  make(map[string]sqlc.OauthAccount),
		sessions:       make(map[uuid.UUID]sqlc.Session),
		verTokens:      make(map[uuid.UUID]sqlc.VerificationToken),
		trustedDevices: make(map[uuid.UUID]sqlc.TrustedDevice),
		backupCodes:    make(map[uuid.UUID]sqlc.BackupCode),
		ownedResources: make(map[uuid.UUID][]string),
	}
}

func (s *memStore) Queries() sqlc.Querier { return s }

func (s *memStore) WithTx(_ context.Context, fn func(q sqlc.Querier) error) error {
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
	u := sqlc.User{
		ID:              uuid.New(),
		Email:           strings.ToLower(arg.Lower),
		PasswordHash:    arg.PasswordHash,
		Name:            arg.Name,
		AvatarUrl:       arg.AvatarUrl,
		EmailVerifiedAt: arg.EmailVerifiedAt,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.users[u.ID] = u
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
				Status:                u.Status,
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
	svc, _, mailer, _ := newTestService(t)
	ctx := context.Background()

	// 1. Password user without 2FA.
	user, err := svc.Register(ctx, "del-pw@example.com", "strong-delete-pass-1", "Delete PW", testMeta())
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

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

	// Create User A (will exceed 30 days) and User B (will remain <30 days).
	userA, err := svc.Register(ctx, "usera@example.com", "password-user-a-123", "User A", testMeta())
	if err != nil {
		t.Fatalf("Register User A failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail User A failed: %v", err)
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
	userB, err := svc.Register(ctx, "userb@example.com", "password-user-b-123", "User B", testMeta())
	if err != nil {
		t.Fatalf("Register User B failed: %v", err)
	}
	if err := svc.VerifyEmail(ctx, mailer.lastToken(), testMeta()); err != nil {
		t.Fatalf("VerifyEmail User B failed: %v", err)
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
