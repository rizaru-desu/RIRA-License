// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"riralicense/internal/auth"
	"riralicense/internal/config"
	"riralicense/internal/db/sqlc"
	"riralicense/internal/handler"
	"riralicense/internal/mail"
	"riralicense/internal/ratelimit"
	"riralicense/internal/service"
)

type noopStore struct {
	sqlc.Querier
}

func (n *noopStore) Queries() sqlc.Querier { return n }
func (n *noopStore) WithTx(_ context.Context, fn func(q sqlc.Querier) error) error {
	return fn(n)
}
func (n *noopStore) GetUserByEmail(_ context.Context, _ string) (sqlc.User, error) {
	return sqlc.User{}, pgx.ErrNoRows
}
func (n *noopStore) InsertAuthAuditLog(_ context.Context, _ sqlc.InsertAuthAuditLogParams) error {
	return nil
}
func (n *noopStore) GetSessionWithUserByTokenHash(_ context.Context, _ string) (sqlc.GetSessionWithUserByTokenHashRow, error) {
	return sqlc.GetSessionWithUserByTokenHashRow{}, pgx.ErrNoRows
}
func (n *noopStore) DeleteSessionByTokenHash(_ context.Context, _ string) error {
	return nil
}

func TestRouterValidationRateLimitAndDisabledOAuth(t *testing.T) {
	cfg := &config.Config{
		Port:               7819,
		Env:                "development",
		AppURL:             "http://localhost:5173",
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
	_ = uuid.Nil
}
