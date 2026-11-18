// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"riralicense/internal/auth"
	"riralicense/internal/config"
	"riralicense/internal/crypto"
	"riralicense/internal/db/sqlc"
	"riralicense/internal/handler"
	"riralicense/internal/mail"
	"riralicense/internal/ratelimit"
	"riralicense/internal/service"
)

var testAuthUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

type noopStore struct {
	sqlc.Querier
	name      string
	avatarURL pgtype.Text
	userCount int64
}

func (n *noopStore) Queries() sqlc.Querier { return n }
func (n *noopStore) WithTx(_ context.Context, fn func(q sqlc.Querier) error) error {
	return fn(n)
}
func (n *noopStore) CountAllUsers(_ context.Context) (int64, error) {
	return n.userCount, nil
}
func (n *noopStore) CountActiveOwners(_ context.Context) (int64, error) {
	return 2, nil
}
func (n *noopStore) GetUserByEmail(_ context.Context, _ string) (sqlc.User, error) {
	return sqlc.User{}, pgx.ErrNoRows
}
func (n *noopStore) GetUserByEmailIncludingDeleted(_ context.Context, _ string) (sqlc.User, error) {
	return sqlc.User{}, pgx.ErrNoRows
}
func (n *noopStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	if id == testAuthUserID {
		name := n.name
		if name == "" {
			name = "Active"
		}
		return sqlc.User{
			ID:        testAuthUserID,
			Email:     "active@example.com",
			Name:      name,
			AvatarUrl: n.avatarURL,
			Status:    "active",
		}, nil
	}
	return sqlc.User{}, pgx.ErrNoRows
}
func (n *noopStore) UpdateUserName(_ context.Context, arg sqlc.UpdateUserNameParams) (sqlc.User, error) {
	if arg.ID != testAuthUserID {
		return sqlc.User{}, pgx.ErrNoRows
	}
	n.name = arg.Name
	return sqlc.User{
		ID:        testAuthUserID,
		Email:     "active@example.com",
		Name:      n.name,
		AvatarUrl: n.avatarURL,
		Status:    "active",
	}, nil
}
func (n *noopStore) UpdateUserAvatar(_ context.Context, arg sqlc.UpdateUserAvatarParams) (sqlc.User, error) {
	if arg.ID != testAuthUserID {
		return sqlc.User{}, pgx.ErrNoRows
	}
	n.avatarURL = arg.AvatarUrl
	name := n.name
	if name == "" {
		name = "Active"
	}
	return sqlc.User{
		ID:        testAuthUserID,
		Email:     "active@example.com",
		Name:      name,
		AvatarUrl: n.avatarURL,
		Status:    "active",
	}, nil
}
func (n *noopStore) GetLatestActiveEmailOTP(_ context.Context, _ uuid.UUID) (sqlc.EmailOtpCode, error) {
	return sqlc.EmailOtpCode{}, pgx.ErrNoRows
}
func (n *noopStore) GetVerificationTokenByHash(_ context.Context, _ sqlc.GetVerificationTokenByHashParams) (sqlc.VerificationToken, error) {
	return sqlc.VerificationToken{}, pgx.ErrNoRows
}
func (n *noopStore) InsertAuthAuditLog(_ context.Context, _ sqlc.InsertAuthAuditLogParams) error {
	return nil
}
func (n *noopStore) TouchSessionActivity(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (n *noopStore) GetSessionWithUserByTokenHash(_ context.Context, tokenHash string) (sqlc.GetSessionWithUserByTokenHashRow, error) {
	if tokenHash == crypto.HashToken("valid-test-session") {
		now := time.Now()
		return sqlc.GetSessionWithUserByTokenHashRow{
			SessionID:             uuid.New(),
			SessionUserID:         testAuthUserID,
			SessionTokenHash:      tokenHash,
			SessionExpiresAt:      now.Add(24 * time.Hour),
			SessionLastActivityAt: now,
			ID:                    testAuthUserID,
			Email:                 "active@example.com",
			Name:                  "Active",
			Status:                "active",
		}, nil
	}
	return sqlc.GetSessionWithUserByTokenHashRow{}, pgx.ErrNoRows
}
func (n *noopStore) DeleteSessionByTokenHash(_ context.Context, _ string) error {
	return nil
}
func (n *noopStore) CreateUser(_ context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	n.userCount++
	role := arg.Role
	if role == "" {
		role = auth.RoleMember
	}
	return sqlc.User{
		ID:           uuid.New(),
		Email:        arg.Lower,
		Name:         arg.Name,
		Role:         role,
		IsSuperadmin: arg.IsSuperadmin,
		Status:       "active",
	}, nil
}
func (n *noopStore) InsertPasswordHistory(_ context.Context, _ sqlc.InsertPasswordHistoryParams) error {
	return nil
}
func (n *noopStore) CreateVerificationToken(_ context.Context, arg sqlc.CreateVerificationTokenParams) (sqlc.VerificationToken, error) {
	return sqlc.VerificationToken{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		Type:      arg.Type,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func TestRouterValidationRateLimitAndDisabledOAuth(t *testing.T) {
	cfg := &config.Config{
		Port:               7819,
		Env:                "development",
		AppURL:             "http://localhost:5173",
		UploadDir:          t.TempDir(),
		HTTPTimeout:        5 * time.Second,
		SessionSecret:      bytes.Repeat([]byte("s"), 32),
		TOTPEncryptionKey:  bytes.Repeat([]byte("k"), 32),
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mailer, err := mail.NewMailer(config.SMTPConfig{}, "development", logger)
	if err != nil {
		t.Fatalf("NewMailer failed: %v", err)
	}
	limiters := ratelimit.NewAuthLimiters()
	defer limiters.Stop()

	store := &noopStore{}
	authSvc := auth.NewService(store, mailer, cfg, logger)
	authHandler := handler.NewAuthHandler(authSvc, cfg, limiters, logger)
	healthHandler := handler.NewHealthHandler(service.NewHealthService("development"))

	r := New(cfg, healthHandler, authHandler, authSvc)

	// 1. Providers endpoint returns empty list when no OAuth credentials are configured.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /providers, got %d", rec.Code)
	}
	var provBody struct {
		Providers []string `json:"providers"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&provBody)
	if len(provBody.Providers) != 0 {
		t.Fatalf("expected 0 providers, got %v", provBody.Providers)
	}

	// 2. Disabled OAuth provider returns 404.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for disabled github oauth, got %d", rec.Code)
	}

	// 3. Register validation returns 400 with field-level messages.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"bad","password":"short","name":""}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid register payload, got %d", rec.Code)
	}

	// 3b. Password minimum length across register, change-password, and reset-password:
	// 5 chars rejected with "password must be at least 6 characters", 6 chars accepted on register.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"five@example.com","password":"12345","name":"Five"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 5-char register password, got %d", rec.Code)
	}
	var errResp struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error != "password must be at least 6 characters" || errResp.Fields["password"] != "password must be at least 6 characters" {
		t.Fatalf("unexpected 5-char register error response: %+v", errResp)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"six@example.com","password":"abc123","name":"Six"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for 6-char register password, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/change", bytes.NewBufferString(`{"current_password":"oldpassword","new_password":"12345"}`))
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 5-char change-password, got %d", rec.Code)
	}
	errResp = struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}{}
	_ = json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error != "password must be at least 6 characters" || errResp.Fields["new_password"] != "password must be at least 6 characters" {
		t.Fatalf("unexpected 5-char change-password error response: %+v", errResp)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewBufferString(`{"token":"valid-length-reset-token","new_password":"12345"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 5-char reset-password, got %d", rec.Code)
	}
	errResp = struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}{}
	_ = json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error != "password must be at least 6 characters" || errResp.Fields["new_password"] != "password must be at least 6 characters" {
		t.Fatalf("unexpected 5-char reset-password error response: %+v", errResp)
	}

	// 4. Forgot password returns identical 200 and enforces 3/hr rate limit with Retry-After.
	for i := 0; i < 3; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewBufferString(`{"email":"nobody@example.com"}`))
		req.RemoteAddr = "203.0.113.10:12345"
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d", i+1, rec.Code)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewBufferString(`{"email":"nobody@example.com"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 4th forgot-password attempt, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}

	// 5. Protected endpoint /me rejects unauthenticated requests with 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated /me, got %d", rec.Code)
	}

	// 6. Account deletion endpoints enforce auth and validator/v10 body validation.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/request", bytes.NewBufferString(`{"password":"secret"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated /account/delete/request, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/confirm", bytes.NewBufferString(`{"token":"short"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid /account/delete/confirm payload, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/restore", bytes.NewBufferString(`{"token":"short"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid /account/restore payload, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/cancel", bytes.NewBufferString(`{"email":"not-an-email"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid /account/delete/cancel payload, got %d", rec.Code)
	}

	for i := 0; i < 3; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/cancel", bytes.NewBufferString(`{"email":"deleted@example.com"}`))
		req.RemoteAddr = "203.0.113.20:12345"
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("cancel attempt %d expected 200, got %d", i+1, rec.Code)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/cancel", bytes.NewBufferString(`{"email":"deleted@example.com"}`))
	req.RemoteAddr = "203.0.113.20:12345"
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 4th account delete cancel attempt, got %d", rec.Code)
	}

	// 7. Authenticated /account/delete/request enforces 3/hr rate limit per account + per IP.
	for i := 0; i < 3; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/request", bytes.NewBufferString(`{"email_otp":"000000"}`))
		req.RemoteAddr = "203.0.113.30:12345"
		req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("delete request attempt %d expected 401 (wrong OTP), got %d", i+1, rec.Code)
		}
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/account/delete/request", bytes.NewBufferString(`{"email_otp":"000000"}`))
	req.RemoteAddr = "203.0.113.30:12345"
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 4th /account/delete/request attempt, got %d", rec.Code)
	}

	// 8. Profile endpoints: PATCH /api/v1/users/me, POST /api/v1/users/me/avatar, DELETE /api/v1/users/me/avatar.
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", bytes.NewBufferString(`{"name":"Ada Lovelace"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated PATCH /api/v1/users/me, got %d", rec.Code)
	}

	// Empty name rejected with 400
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", bytes.NewBufferString(`{"name":"   "}`))
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d", rec.Code)
	}

	// Extra fields (e.g. email) rejected with 400
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", bytes.NewBufferString(`{"name":"Ada","email":"other@example.com"}`))
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for extra fields in PATCH /api/v1/users/me, got %d", rec.Code)
	}

	// Valid name accepted and trimmed
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", bytes.NewBufferString(`{"name":"  Ada Lovelace  "}`))
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid PATCH /api/v1/users/me, got %d body=%s", rec.Code, rec.Body.String())
	}
	var meResp struct {
		User auth.UserDTO `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&meResp)
	if meResp.User.Name != "Ada Lovelace" {
		t.Fatalf("expected updated name 'Ada Lovelace', got %q", meResp.User.Name)
	}

	// Upload avatar: .exe renamed to .png rejected with 400
	buildMultipart := func(filename string, content []byte) (*bytes.Buffer, string) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, partErr := mw.CreateFormFile("avatar", filename)
		if partErr != nil {
			t.Fatalf("CreateFormFile failed: %v", partErr)
		}
		_, _ = part.Write(content)
		_ = mw.Close()
		return &body, mw.FormDataContentType()
	}

	exeFakePNG := append([]byte{'M', 'Z', 0x90, 0x00}, bytes.Repeat([]byte{0x00}, 100)...)
	mpBody, mpType := buildMultipart("malware.png", exeFakePNG)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/me/avatar", mpBody)
	req.Header.Set("Content-Type", mpType)
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for .exe renamed to .png, got %d", rec.Code)
	}

	// Upload avatar: > 2 MB rejected with 400
	over2MB := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, (2<<20)+1024)...)
	mpBody, mpType = buildMultipart("big.png", over2MB)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/me/avatar", mpBody)
	req.Header.Set("Content-Type", mpType)
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for >2MB avatar upload, got %d", rec.Code)
	}

	// Upload avatar: valid 500 KB PNG succeeds and is served via /uploads/avatars/{filename}
	png500KB := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 500*1024-8)...)
	mpBody, mpType = buildMultipart("avatar.png", png500KB)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/me/avatar", mpBody)
	req.Header.Set("Content-Type", mpType)
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid 500 KB PNG upload, got %d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&meResp)
	if meResp.User.AvatarURL == nil || !strings.HasPrefix(*meResp.User.AvatarURL, "/uploads/avatars/") {
		t.Fatalf("expected avatar_url set after upload, got %v", meResp.User.AvatarURL)
	}

	// Serve uploaded file
	req = httptest.NewRequest(http.MethodGet, *meResp.User.AvatarURL, nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() != 500*1024 {
		t.Fatalf("expected 200 and 500 KB body from %s, got code=%d len=%d", *meResp.User.AvatarURL, rec.Code, rec.Body.Len())
	}

	// Delete avatar removes file and clears avatar_url
	oldURL := *meResp.User.AvatarURL
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/users/me/avatar", nil)
	req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "valid-test-session"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE /api/v1/users/me/avatar, got %d", rec.Code)
	}
	_ = json.NewDecoder(rec.Body).Decode(&meResp)
	if meResp.User.AvatarURL != nil {
		t.Fatalf("expected nil avatar_url after delete, got %v", *meResp.User.AvatarURL)
	}

	req = httptest.NewRequest(http.MethodGet, oldURL, nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted avatar %s, got %d", oldURL, rec.Code)
	}

	_ = uuid.Nil
}

type rbacTestStore struct {
	sqlc.Querier
	txMu             sync.Mutex
	mu               sync.Mutex
	users            map[uuid.UUID]sqlc.User
	sessions         map[string]uuid.UUID // tokenHash -> userID
	sessionWS        map[uuid.UUID]uuid.UUID
	audits           []sqlc.InsertAuthAuditLogParams
	workspaceAudits  []sqlc.InsertWorkspaceAuditLogParams
	workspaces       map[uuid.UUID]sqlc.Workspace
	workspaceMembers map[string]sqlc.WorkspaceMember
}

func newRBACTestStore() *rbacTestStore {
	return &rbacTestStore{
		users:            make(map[uuid.UUID]sqlc.User),
		sessions:         make(map[string]uuid.UUID),
		sessionWS:        make(map[uuid.UUID]uuid.UUID),
		workspaces:       make(map[uuid.UUID]sqlc.Workspace),
		workspaceMembers: make(map[string]sqlc.WorkspaceMember),
	}
}

func (s *rbacTestStore) Queries() sqlc.Querier { return s }
func (s *rbacTestStore) WithTx(_ context.Context, fn func(q sqlc.Querier) error) error {
	s.txMu.Lock()
	defer s.txMu.Unlock()
	return fn(s)
}

func (s *rbacTestStore) CountAllUsers(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.users)), nil
}

func (s *rbacTestStore) CountActiveOwners(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, u := range s.users {
		if u.Role == auth.RoleOwner && u.Status == "active" && u.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (s *rbacTestStore) GetUserByEmail(_ context.Context, email string) (sqlc.User, error) {
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

func (s *rbacTestStore) GetUserByEmailIncludingDeleted(_ context.Context, email string) (sqlc.User, error) {
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

func (s *rbacTestStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok || u.DeletedAt != nil {
		return sqlc.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (s *rbacTestStore) ListUsers(_ context.Context) ([]sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]sqlc.User, 0, len(s.users))
	for _, u := range s.users {
		if u.DeletedAt == nil {
			out = append(out, u)
		}
	}
	return out, nil
}

func (s *rbacTestStore) CreateUser(_ context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	role := arg.Role
	if role == "" {
		role = auth.RoleMember
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

func (s *rbacTestStore) UpdateUserRole(_ context.Context, arg sqlc.UpdateUserRoleParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[arg.ID]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.Role = arg.Role
	s.users[arg.ID] = u
	return u, nil
}

func (s *rbacTestStore) SuspendUser(_ context.Context, arg sqlc.SuspendUserParams) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[arg.ID]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	now := time.Now()
	u.Status = "suspended"
	u.SuspendedReason = arg.SuspendedReason
	u.SuspendedAt = &now
	s.users[arg.ID] = u
	return u, nil
}

func (s *rbacTestStore) ActivateUser(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.Status = "active"
	u.SuspendedReason = pgtype.Text{}
	u.SuspendedAt = nil
	s.users[id] = u
	return u, nil
}

func (s *rbacTestStore) InsertPasswordHistory(_ context.Context, _ sqlc.InsertPasswordHistoryParams) error {
	return nil
}

func (s *rbacTestStore) CreateVerificationToken(_ context.Context, arg sqlc.CreateVerificationTokenParams) (sqlc.VerificationToken, error) {
	return sqlc.VerificationToken{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		Type:      arg.Type,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func (s *rbacTestStore) InsertAuthAuditLog(_ context.Context, arg sqlc.InsertAuthAuditLogParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits = append(s.audits, arg)
	return nil
}

func (s *rbacTestStore) TouchSessionActivity(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *rbacTestStore) ResetLoginFailuresAndUpdateLastLogin(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *rbacTestStore) CreateSession(_ context.Context, arg sqlc.CreateSessionParams) (sqlc.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[arg.TokenHash] = arg.UserID
	return sqlc.Session{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func (s *rbacTestStore) DeleteUserSessions(_ context.Context, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for th, uid := range s.sessions {
		if uid == userID {
			delete(s.sessions, th)
		}
	}
	return nil
}

func (s *rbacTestStore) DeleteSessionByID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *rbacTestStore) GetSessionWithUserByTokenHash(_ context.Context, tokenHash string) (sqlc.GetSessionWithUserByTokenHashRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.sessions[tokenHash]
	if !ok {
		return sqlc.GetSessionWithUserByTokenHashRow{}, pgx.ErrNoRows
	}
	u, ok := s.users[uid]
	if !ok {
		return sqlc.GetSessionWithUserByTokenHashRow{}, pgx.ErrNoRows
	}
	now := time.Now()
	return sqlc.GetSessionWithUserByTokenHashRow{
		SessionID:             uuid.New(),
		SessionUserID:         u.ID,
		SessionTokenHash:      tokenHash,
		SessionExpiresAt:      now.Add(24 * time.Hour),
		SessionLastActivityAt: now,
		ID:                    u.ID,
		Email:                 u.Email,
		PasswordHash:          u.PasswordHash,
		Name:                  u.Name,
		Role:                  u.Role,
		IsSuperadmin:          u.IsSuperadmin,
		Status:                u.Status,
		SuspendedReason:       u.SuspendedReason,
		SuspendedAt:           u.SuspendedAt,
		DeletedAt:             u.DeletedAt,
		CreatedAt:             u.CreatedAt,
		UpdatedAt:             u.UpdatedAt,
	}, nil
}

func TestRBACRouterMiddlewareAndFreeLimits(t *testing.T) {
	cfg := &config.Config{
		Port:               7819,
		Env:                "development",
		AppURL:             "http://localhost:5173",
		UploadDir:          t.TempDir(),
		HTTPTimeout:        5 * time.Second,
		SessionSecret:      bytes.Repeat([]byte("s"), 32),
		TOTPEncryptionKey:  bytes.Repeat([]byte("k"), 32),
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mailer, err := mail.NewMailer(config.SMTPConfig{}, "development", logger)
	if err != nil {
		t.Fatalf("NewMailer failed: %v", err)
	}
	limiters := ratelimit.NewAuthLimiters()
	defer limiters.Stop()

	store := newRBACTestStore()
	authSvc := auth.NewService(store, mailer, cfg, logger)
	authHandler := handler.NewAuthHandler(authSvc, cfg, limiters, logger)
	healthHandler := handler.NewHealthHandler(service.NewHealthService("development"))

	r := New(cfg, healthHandler, authHandler, authSvc)

	// 1. First public registration creates owner + superadmin.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"owner@example.com","password":"password123","name":"Founder"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first registration, got %d body=%s", rec.Code, rec.Body.String())
	}
	var regResp struct {
		User auth.UserDTO `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&regResp)
	if regResp.User.Role != auth.RoleOwner || !regResp.User.IsSuperadmin {
		t.Fatalf("expected first registered user to be owner + superadmin, got role=%s super=%v", regResp.User.Role, regResp.User.IsSuperadmin)
	}

	// 2. Second public registration creates a separate non-superadmin tenant owner with their own Free workspace.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"second@example.com","password":"password123","name":"Second"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on second registration, got %d body=%s", rec.Code, rec.Body.String())
	}
	var secondRegResp struct {
		User auth.UserDTO `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&secondRegResp)
	if secondRegResp.User.Role != auth.RoleOwner || secondRegResp.User.IsSuperadmin {
		t.Fatalf("expected second user to be non-superadmin owner, got role=%s super=%v", secondRegResp.User.Role, secondRegResp.User.IsSuperadmin)
	}
	secondWorkspaces, err := authSvc.ListUserWorkspaces(context.Background(), secondRegResp.User.ID, false)
	if err != nil || len(secondWorkspaces) != 1 || secondWorkspaces[0].Plan != auth.FreePlan {
		t.Fatalf("expected second user to auto-provision 1 free workspace, got err=%v wss=%+v", err, secondWorkspaces)
	}
	var errMap map[string]string

	// Seed sessions for owner (superadmin), admin, member, viewer.
	defaultWS, _ := store.GetFirstActiveWorkspace(context.Background())
	seedUserWithSession := func(email, name, role string, isSuper bool, token string) uuid.UUID {
		pwHash, _ := crypto.HashPassword("password123")
		now := time.Now()
		u, _ := store.CreateUser(context.Background(), sqlc.CreateUserParams{
			Lower:           email,
			PasswordHash:    pgtype.Text{String: pwHash, Valid: true},
			Name:            name,
			EmailVerifiedAt: &now,
			Role:            role,
			IsSuperadmin:    isSuper,
		})
		_, _ = store.CreateWorkspaceMember(context.Background(), sqlc.CreateWorkspaceMemberParams{
			WorkspaceID: defaultWS.ID,
			UserID:      u.ID,
			Role:        role,
		})
		store.mu.Lock()
		store.sessions[crypto.HashToken(token)] = u.ID
		store.mu.Unlock()
		return u.ID
	}

	// Update the first user to verified with known password & session.
	pwHash, _ := crypto.HashPassword("password123")
	now := time.Now()
	store.mu.Lock()
	ownerUser := store.users[regResp.User.ID]
	ownerUser.PasswordHash = pgtype.Text{String: pwHash, Valid: true}
	ownerUser.EmailVerifiedAt = &now
	store.users[ownerUser.ID] = ownerUser
	store.sessions[crypto.HashToken("sess-owner")] = ownerUser.ID
	store.mu.Unlock()

	adminID := seedUserWithSession("admin@example.com", "Admin", auth.RoleAdmin, false, "sess-admin")
	memberID := seedUserWithSession("member@example.com", "Member", auth.RoleMember, false, "sess-member")
	viewerID := seedUserWithSession("viewer@example.com", "Viewer", auth.RoleViewer, false, "sess-viewer")
	_ = viewerID

	doReq := func(method, path, sessionToken, body string) *httptest.ResponseRecorder {
		var rBody io.Reader
		if body != "" {
			rBody = bytes.NewBufferString(body)
		}
		req := httptest.NewRequest(method, path, rBody)
		if sessionToken != "" {
			req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: sessionToken})
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 3. /api/v1/auth/me returns the right role, is_superadmin, and permissions[] per role.
	for _, tc := range []struct {
		token    string
		role     string
		isSuper  bool
		expected []string
	}{
		{"sess-owner", auth.RoleOwner, true, auth.PermissionsFor(auth.RoleOwner, true)},
		{"sess-admin", auth.RoleAdmin, false, auth.PermissionsFor(auth.RoleAdmin, false)},
		{"sess-member", auth.RoleMember, false, auth.PermissionsFor(auth.RoleMember, false)},
		{"sess-viewer", auth.RoleViewer, false, auth.PermissionsFor(auth.RoleViewer, false)},
	} {
		rec := doReq(http.MethodGet, "/api/v1/auth/me", tc.token, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on /me for %s, got %d", tc.role, rec.Code)
		}
		var mePayload struct {
			Role         string       `json:"role"`
			IsSuperadmin bool         `json:"is_superadmin"`
			Permissions  []string     `json:"permissions"`
			Plan         string       `json:"plan"`
			MaxUsers     int          `json:"max_users"`
			User         auth.UserDTO `json:"user"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&mePayload)
		if tc.isSuper {
			if mePayload.Plan != "platform" || mePayload.MaxUsers != 0 {
				t.Fatalf("expected platform mode on /me for superadmin, got plan=%q max_users=%d", mePayload.Plan, mePayload.MaxUsers)
			}
		} else if mePayload.Plan != auth.FreePlan || mePayload.MaxUsers != auth.FreeMaxUsers {
			t.Fatalf("expected free plan with max_users=%d on /me, got plan=%q max_users=%d", auth.FreeMaxUsers, mePayload.Plan, mePayload.MaxUsers)
		}
		if mePayload.Role != tc.role || mePayload.User.Role != tc.role {
			t.Fatalf("expected role %q, got top=%q user=%q", tc.role, mePayload.Role, mePayload.User.Role)
		}
		if mePayload.IsSuperadmin != tc.isSuper || mePayload.User.IsSuperadmin != tc.isSuper {
			t.Fatalf("expected is_superadmin=%v for %s", tc.isSuper, tc.role)
		}
		if !slices.Equal(mePayload.Permissions, tc.expected) || !slices.Equal(mePayload.User.Permissions, tc.expected) {
			t.Fatalf("unexpected permissions for %s: got %v want %v", tc.role, mePayload.Permissions, tc.expected)
		}
	}

	// 4. Viewer gets 403 {"error":"forbidden"} on create license.
	rec = doReq(http.MethodPost, "/api/v1/licenses", "sess-viewer", `{}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer creating license, got %d", rec.Code)
	}
	errMap = nil
	_ = json.NewDecoder(rec.Body).Decode(&errMap)
	if errMap["error"] != "forbidden" {
		t.Fatalf("expected {'error':'forbidden'}, got %v", errMap)
	}

	// 5. Member gets 403 on delete product and on user management.
	rec = doReq(http.MethodDelete, "/api/v1/products/prod-1", "sess-member", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member deleting product, got %d", rec.Code)
	}
	rec = doReq(http.MethodGet, "/api/v1/admin/users", "sess-member", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member listing users, got %d", rec.Code)
	}
	rec = doReq(http.MethodPost, "/api/v1/admin/users", "sess-member", `{"email":"x@example.com","name":"X","role":"viewer"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member creating user, got %d", rec.Code)
	}

	// 6. Superadmin bypasses free-plan invite limit (201 Created even above 1 seat),
	// while a non-superadmin owner on free plan gets 403 {"error":"invite requires a paid plan"}.
	nonSuperOwnerID := seedUserWithSession("nonsuper-owner@example.com", "Non Super Owner", auth.RoleOwner, false, "sess-nonsuper-owner")
	userCountBeforeInvite := len(store.users)
	rec = doReq(http.MethodPost, "/api/v1/admin/users", "sess-nonsuper-owner", `{"email":"invite@example.com","name":"Invitee","role":"member"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-superadmin owner inviting on free plan, got %d body=%s", rec.Code, rec.Body.String())
	}
	errMap = nil
	_ = json.NewDecoder(rec.Body).Decode(&errMap)
	if errMap["error"] != "invite requires a paid plan" {
		t.Fatalf("expected {'error':'invite requires a paid plan'}, got %v", errMap)
	}
	if len(store.users) != userCountBeforeInvite {
		t.Fatalf("expected no user created when invite rejected on free plan")
	}
	// Remove temporary nonSuperOwner so single-owner invariant assertions stay exact.
	store.mu.Lock()
	delete(store.users, nonSuperOwnerID)
	delete(store.workspaceMembers, defaultWS.ID.String()+":"+nonSuperOwnerID.String())
	delete(store.sessions, crypto.HashToken("sess-nonsuper-owner"))
	store.mu.Unlock()

	rec = doReq(http.MethodPost, "/api/v1/admin/users", "sess-owner", `{"email":"super-invited@example.com","name":"Super Invited","role":"member"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for superadmin inviting on free plan, got %d body=%s", rec.Code, rec.Body.String())
	}
	var inviteResp struct {
		User auth.AdminUserDTO `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&inviteResp)
	if inviteResp.User.Role != auth.RoleMember || inviteResp.User.IsSuperadmin {
		t.Fatalf("expected superadmin-invited user to be non-superadmin member, got %+v", inviteResp.User)
	}

	// Verify only 1 superadmin exists globally, and the second registered owner is isolated to their own workspace (not defaultWS).
	var superCount int
	for _, u := range store.users {
		if u.IsSuperadmin {
			superCount++
		}
	}
	if superCount != 1 {
		t.Fatalf("expected exactly 1 superadmin, got supers=%d", superCount)
	}
	if _, err := store.GetWorkspaceMember(context.Background(), sqlc.GetWorkspaceMemberParams{
		WorkspaceID: defaultWS.ID,
		UserID:      secondRegResp.User.ID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected second registered owner not in defaultWS, got err=%v", err)
	}

	rec = doReq(http.MethodPatch, "/api/v1/admin/users/"+memberID.String()+"/role", "sess-admin", `{"role":"viewer"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin changing role, got %d", rec.Code)
	}
	rec = doReq(http.MethodPatch, "/api/v1/admin/users/"+ownerUser.ID.String()+"/role", "sess-owner", `{"role":"admin"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for owner changing own role, got %d", rec.Code)
	}
	rec = doReq(http.MethodPatch, "/api/v1/admin/users/"+memberID.String()+"/role", "sess-owner", `{"role":"owner"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for promoting member to owner, got %d", rec.Code)
	}
	// Non-self superadmin attempting to demote the sole owner gets 403.
	store.mu.Lock()
	admRoleTest := store.users[adminID]
	admRoleTest.IsSuperadmin = true
	store.users[adminID] = admRoleTest
	store.mu.Unlock()
	rec = doReq(http.MethodPatch, "/api/v1/admin/users/"+ownerUser.ID.String()+"/role", "sess-admin", `{"role":"member"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for changing sole owner's role, got %d", rec.Code)
	}
	store.mu.Lock()
	admRoleTest.IsSuperadmin = false
	store.users[adminID] = admRoleTest
	store.mu.Unlock()

	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+ownerUser.ID.String()+"/suspend", "sess-admin", `{"reason":"testing"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin suspending owner, got %d", rec.Code)
	}

	// 7. Suspending self is 403; suspending the sole owner is 403.
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+ownerUser.ID.String()+"/suspend", "sess-owner", `{"reason":"self suspend"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for suspending self, got %d", rec.Code)
	}
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+adminID.String()+"/suspend", "sess-admin", `{"reason":"self suspend"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin suspending self, got %d", rec.Code)
	}
	// Non-self superadmin attempting to suspend the sole active owner gets 403.
	store.mu.Lock()
	adm := store.users[adminID]
	adm.IsSuperadmin = true
	store.users[adminID] = adm
	store.mu.Unlock()
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+ownerUser.ID.String()+"/suspend", "sess-admin", `{"reason":"suspend sole owner"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for suspending sole owner by non-self superadmin, got %d", rec.Code)
	}
	store.mu.Lock()
	adm.IsSuperadmin = false
	store.users[adminID] = adm
	store.mu.Unlock()
	// Sole owner cannot delete their own account (403).
	rec = doReq(http.MethodPost, "/api/v1/auth/account/delete/request", "sess-owner", `{"password":"password123"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for sole owner requesting account deletion, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 8. Suspend without a reason is 400; valid suspend deletes target's sessions and blocks login; activate restores access.
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+memberID.String()+"/suspend", "sess-admin", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when suspending without reason, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+memberID.String()+"/suspend", "sess-admin", `{"reason":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when suspending with blank reason, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+memberID.String()+"/suspend", "sess-admin", `{"reason":"compromised credentials"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin suspending member, got %d body=%s", rec.Code, rec.Body.String())
	}
	// Member's global session & login remain valid (BR-04), while access to defaultWS is revoked (403).
	rec = doReq(http.MethodGet, "/api/v1/auth/me", "sess-member", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on global /me after workspace-only suspend, got %d body=%s", rec.Code, rec.Body.String())
	}
	reqWS := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	reqWS.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "sess-member"})
	reqWS.Header.Set("X-Workspace-ID", defaultWS.ID.String())
	recWS := httptest.NewRecorder()
	r.ServeHTTP(recWS, reqWS)
	if recWS.Code != http.StatusForbidden {
		t.Fatalf("expected 403 accessing suspended workspace, got %d body=%s", recWS.Code, recWS.Body.String())
	}
	rec = doReq(http.MethodPost, "/api/v1/auth/login", "", `{"email":"member@example.com","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when workspace-suspended member logs in globally, got %d", rec.Code)
	}
	// Reactivate member -> workspace access succeeds again.
	rec = doReq(http.MethodPost, "/api/v1/admin/users/"+memberID.String()+"/activate", "sess-admin", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on member activate, got %d body=%s", rec.Code, rec.Body.String())
	}
	reqWS = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	reqWS.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "sess-member"})
	reqWS.Header.Set("X-Workspace-ID", defaultWS.ID.String())
	recWS = httptest.NewRecorder()
	r.ServeHTTP(recWS, reqWS)
	if recWS.Code != http.StatusOK {
		t.Fatalf("expected 200 accessing reactivated workspace, got %d body=%s", recWS.Code, recWS.Body.String())
	}

	// 9. Free-tier limits for non-superadmin: 4th product (FreeMaxProducts=3) and 101st license (FreeMaxLicenses=100) return 403 {"error":"free plan limit reached"}.
	// First verify superadmin bypasses product/license limits (creating products/licenses does not cap superadmin).
	// Use sess-admin (non-superadmin) to verify workspace free-tier enforcement on 4th product and 101st license.
	for i := 1; i <= auth.FreeMaxProducts; i++ {
		rec = doReq(http.MethodPost, "/api/v1/products", "sess-admin", `{}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("product %d expected 201, got %d", i, rec.Code)
		}
	}
	rec = doReq(http.MethodPost, "/api/v1/products", "sess-admin", `{}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on 4th product for non-superadmin, got %d", rec.Code)
	}
	errMap = nil
	_ = json.NewDecoder(rec.Body).Decode(&errMap)
	if errMap["error"] != "free plan limit reached" {
		t.Fatalf("expected 'free plan limit reached' on 4th product, got %q", errMap["error"])
	}
	// Superadmin bypasses the 4th product limit on the same free workspace:
	rec = doReq(http.MethodPost, "/api/v1/products", "sess-owner", `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for superadmin creating 4th product on free plan, got %d body=%s", rec.Code, rec.Body.String())
	}

	for i := 1; i <= auth.FreeMaxLicenses; i++ {
		rec = doReq(http.MethodPost, "/api/v1/licenses", "sess-admin", `{}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("license %d expected 201, got %d", i, rec.Code)
		}
	}
	rec = doReq(http.MethodPost, "/api/v1/licenses", "sess-admin", `{}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on 101st license for non-superadmin, got %d", rec.Code)
	}
	errMap = nil
	_ = json.NewDecoder(rec.Body).Decode(&errMap)
	if errMap["error"] != "free plan limit reached" {
		t.Fatalf("expected 'free plan limit reached' on 101st license, got %q", errMap["error"])
	}
	// Superadmin bypasses the 101st license limit on the same free workspace:
	rec = doReq(http.MethodPost, "/api/v1/licenses", "sess-owner", `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for superadmin creating 101st license on free plan, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 10. Multi-tenant workspaces & Super Admin platform console:
	// Refresh sess-member after reactivation (previous session was revoked on suspend).
	store.mu.Lock()
	store.sessions[crypto.HashToken("sess-member")] = memberID
	store.mu.Unlock()
	// - Member creates their own workspace (becomes owner of that workspace, while remaining member in Default Workspace).
	rec = doReq(http.MethodPost, "/api/v1/workspaces", "sess-member", `{"name":"Acme Corp","slug":"acme-corp"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating workspace, got %d body=%s", rec.Code, rec.Body.String())
	}
	var createWSResp struct {
		Workspace auth.WorkspaceDTO `json:"workspace"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&createWSResp)
	acmeID := createWSResp.Workspace.ID

	// - Using X-Workspace-ID for Acme Corp, member has role='owner' and can list users in Acme Corp,
	//   while Admin (not a member of Acme Corp) gets 403.
	reqAcme := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	reqAcme.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "sess-member"})
	reqAcme.Header.Set("X-Workspace-ID", acmeID.String())
	recAcme := httptest.NewRecorder()
	r.ServeHTTP(recAcme, reqAcme)
	if recAcme.Code != http.StatusOK {
		t.Fatalf("expected 200 for workspace owner in Acme Corp, got %d body=%s", recAcme.Code, recAcme.Body.String())
	}

	reqAdminAcme := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	reqAdminAcme.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "sess-admin"})
	reqAdminAcme.Header.Set("X-Workspace-ID", acmeID.String())
	recAdminAcme := httptest.NewRecorder()
	r.ServeHTTP(recAdminAcme, reqAdminAcme)
	if recAdminAcme.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member accessing Acme Corp, got %d body=%s", recAdminAcme.Code, recAdminAcme.Body.String())
	}

	// - Non-superadmin cannot access /api/v1/platform/* (403).
	rec = doReq(http.MethodGet, "/api/v1/platform/workspaces", "sess-admin", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-superadmin on /platform/workspaces, got %d", rec.Code)
	}

	// - Superadmin lists all workspaces, overrides Acme Corp plan to pro, suspends and reactivates Acme Corp.
	rec = doReq(http.MethodGet, "/api/v1/platform/workspaces", "sess-owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for superadmin /platform/workspaces, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodPatch, "/api/v1/platform/workspaces/"+acmeID.String()+"/plan", "sess-owner", `{"plan":"pro","max_users_override":5,"override_note":"partner"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating workspace plan, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodPost, "/api/v1/platform/workspaces/"+acmeID.String()+"/suspend", "sess-owner", `{"reason":"billing review"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 suspending workspace, got %d body=%s", rec.Code, rec.Body.String())
	}
	// Customer owner cannot access suspended workspace (403), while superadmin still can.
	reqAcme = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	reqAcme.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "sess-member"})
	reqAcme.Header.Set("X-Workspace-ID", acmeID.String())
	recAcme = httptest.NewRecorder()
	r.ServeHTTP(recAcme, reqAcme)
	if recAcme.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for customer accessing suspended workspace, got %d", recAcme.Code)
	}
	rec = doReq(http.MethodPost, "/api/v1/platform/workspaces/"+acmeID.String()+"/activate", "sess-owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 activating workspace, got %d body=%s", rec.Code, rec.Body.String())
	}

	// - Superadmin cannot globally suspend a sole active owner/admin (BR-07), nor self/superadmin.
	//   Once another active owner is present in Acme Corp, global suspend and reactivation succeed.
	rec = doReq(http.MethodPost, "/api/v1/platform/users/"+ownerUser.ID.String()+"/suspend", "sess-owner", `{"reason":"self"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for superadmin globally suspending self, got %d", rec.Code)
	}
	rec = doReq(http.MethodPost, "/api/v1/platform/users/"+memberID.String()+"/suspend", "sess-owner", `{"reason":"fraudulent workspace owner"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for globally suspending sole owner of Acme Corp (BR-07), got %d body=%s", rec.Code, rec.Body.String())
	}
	coOwnerUser, _ := store.CreateUser(context.Background(), sqlc.CreateUserParams{
		Lower:        "acme-coowner@example.com",
		Name:         "Acme CoOwner",
		Role:         auth.RoleOwner,
		IsSuperadmin: false,
	})
	_, _ = store.CreateWorkspaceMember(context.Background(), sqlc.CreateWorkspaceMemberParams{
		WorkspaceID: acmeID,
		UserID:      coOwnerUser.ID,
		Role:        auth.RoleOwner,
	})
	rec = doReq(http.MethodPost, "/api/v1/platform/users/"+memberID.String()+"/suspend", "sess-owner", `{"reason":"fraudulent workspace owner"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for superadmin globally suspending workspace owner when co-owner exists, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodPost, "/api/v1/platform/users/"+memberID.String()+"/activate", "sess-owner", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for superadmin reactivating globally suspended user, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 11. Inviting existing user (secondRegResp.User, already owner of their own Free workspace) into Acme Corp:
	//     creates a pending invite, user accepts via /api/v1/workspaces/{id}/accept, then leaves via /leave.
	store.mu.Lock()
	store.sessions[crypto.HashToken("sess-member")] = memberID
	store.sessions[crypto.HashToken("sess-second")] = secondRegResp.User.ID
	store.mu.Unlock()

	reqInv := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewBufferString(`{"email":"second@example.com","role":"member"}`))
	reqInv.Header.Set("Content-Type", "application/json")
	reqInv.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: "sess-member"})
	reqInv.Header.Set("X-Workspace-ID", acmeID.String())
	recInv := httptest.NewRecorder()
	r.ServeHTTP(recInv, reqInv)
	if recInv.Code != http.StatusCreated {
		t.Fatalf("expected 201 inviting existing user without name, got %d body=%s", recInv.Code, recInv.Body.String())
	}

	rec = doReq(http.MethodGet, "/api/v1/workspaces/invites", "sess-second", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), acmeID.String()) {
		t.Fatalf("expected pending invite for Acme Corp, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doReq(http.MethodPost, "/api/v1/workspaces/"+acmeID.String()+"/accept", "sess-second", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 accepting workspace invite, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = doReq(http.MethodPost, "/api/v1/workspaces/"+acmeID.String()+"/leave", "sess-second", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 leaving workspace, got %d body=%s", rec.Code, rec.Body.String())
	}
}
