package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestWorkspaceMembershipGovernanceHTTPFlow(t *testing.T) {
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

	ctx := context.Background()
	pwHash, _ := crypto.HashPassword("password123")
	now := time.Now()

	seedUser := func(email, name, role string, isSuper bool, token string) uuid.UUID {
		u, err := store.CreateUser(ctx, sqlc.CreateUserParams{
			Lower:           email,
			PasswordHash:    pgtype.Text{String: pwHash, Valid: true},
			Name:            name,
			EmailVerifiedAt: &now,
			Role:            role,
			IsSuperadmin:    isSuper,
		})
		if err != nil {
			t.Fatalf("CreateUser(%s) failed: %v", email, err)
		}
		store.mu.Lock()
		store.sessions[crypto.HashToken(token)] = u.ID
		store.mu.Unlock()
		return u.ID
	}

	superID := seedUser("super@http.test", "Super", auth.RoleOwner, true, "sess-super")
	ownerID := seedUser("owner@http.test", "Owner", auth.RoleOwner, false, "sess-owner")
	adminID := seedUser("admin@http.test", "Admin", auth.RoleAdmin, false, "sess-admin")
	memberID := seedUser("member@http.test", "Member", auth.RoleMember, false, "sess-member")

	wsA, _ := store.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{
		Name:      "Alpha WS",
		Lower:     "alpha-ws",
		Plan:      auth.ProPlan,
		CreatedBy: uuid.NullUUID{UUID: ownerID, Valid: true},
	})
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: ownerID, Role: auth.RoleOwner})
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{WorkspaceID: wsA.ID, UserID: memberID, Role: auth.RoleMember})

	wsB, _ := store.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{
		Name:      "Beta WS",
		Lower:     "beta-ws",
		Plan:      auth.ProPlan,
		CreatedBy: uuid.NullUUID{UUID: superID, Valid: true},
	})
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{WorkspaceID: wsB.ID, UserID: adminID, Role: auth.RoleAdmin})
	_, _ = store.CreateWorkspaceMember(ctx, sqlc.CreateWorkspaceMemberParams{WorkspaceID: wsB.ID, UserID: memberID, Role: auth.RoleMember})

	doReq := func(method, path, token, wsID, body string) *httptest.ResponseRecorder {
		var rBody io.Reader
		if body != "" {
			rBody = bytes.NewBufferString(body)
		}
		req := httptest.NewRequest(method, path, rBody)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.AddCookie(&http.Cookie{Name: handler.SessionCookieName, Value: token})
		}
		if wsID != "" {
			req.Header.Set("X-Workspace-ID", wsID)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 1. Last active Admin in Beta WS (adminID) cannot be removed, suspended, or demoted.
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"remove last admin", http.MethodDelete, "/api/v1/admin/users/" + adminID.String(), ""},
		{"suspend last admin", http.MethodPost, "/api/v1/admin/users/" + adminID.String() + "/suspend", `{"reason":"test reason"}`},
		{"demote last admin", http.MethodPatch, "/api/v1/admin/users/" + adminID.String() + "/role", `{"role":"member"}`},
	} {
		rec := doReq(tc.method, tc.path, "sess-super", wsB.ID.String(), tc.body)
		if rec.Code != http.StatusConflict {
			t.Fatalf("%s: expected 409 Conflict, got %d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		var errPayload struct {
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&errPayload)
		if errPayload.Code != auth.GovCodeLastActiveAdmin {
			t.Fatalf("%s: expected code %q, got %q (body=%s)", tc.name, auth.GovCodeLastActiveAdmin, errPayload.Code, rec.Body.String())
		}
	}

	// 2. Removing memberID from Alpha WS succeeds with optional reason, preserves Beta WS & global account, and retains audit log.
	rec := doReq(http.MethodDelete, "/api/v1/admin/users/"+memberID.String(), "sess-owner", wsA.ID.String(), `{"reason":"project ended"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 removing member from wsA, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Member loses access to Alpha WS (403) but keeps access to Beta WS (200) and /me (200).
	if rec := doReq(http.MethodGet, "/api/v1/products", "sess-member", wsA.ID.String(), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 accessing removed wsA, got %d", rec.Code)
	}
	if rec := doReq(http.MethodGet, "/api/v1/products", "sess-member", wsB.ID.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 accessing wsB after removal from wsA, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/api/v1/auth/me", "sess-member", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /me after removal from wsA, got %d", rec.Code)
	}

	// Audit history for Alpha WS contains MEMBER_REMOVED even after member removal.
	rec = doReq(http.MethodGet, "/api/v1/audit", "sess-owner", wsA.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /audit, got %d body=%s", rec.Code, rec.Body.String())
	}
	var auditResp struct {
		Events []auth.WorkspaceAuditLogDTO `json:"events"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&auditResp)
	var sawMemberRemoved bool
	for _, ev := range auditResp.Events {
		if ev.Event == auth.AuditEventMemberRemoved && ev.UserID != nil && *ev.UserID == memberID {
			sawMemberRemoved = true
		}
	}
	if !sawMemberRemoved {
		t.Fatalf("expected MEMBER_REMOVED event preserved in /api/v1/audit, got %+v", auditResp.Events)
	}

	// 3. Re-inviting and accepting in Alpha WS does not create duplicate membership rows.
	rec = doReq(http.MethodPost, "/api/v1/admin/users", "sess-owner", wsA.ID.String(), `{"email":"member@http.test","role":"admin"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 re-inviting removed member, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doReq(http.MethodPost, "/api/v1/workspaces/"+wsA.ID.String()+"/accept", "sess-member", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 accepting re-invite, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doReq(http.MethodGet, "/api/v1/products", "sess-member", wsA.ID.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 accessing wsA after rejoin, got %d", rec.Code)
	}

	// 4. Platform global suspend impact endpoint & concurrent admin removal over HTTP.
	rec = doReq(http.MethodGet, "/api/v1/platform/users/"+adminID.String()+"/suspend-impact", "sess-super", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on suspend-impact, got %d body=%s", rec.Code, rec.Body.String())
	}
	var impact auth.GlobalSuspendImpactDTO
	_ = json.NewDecoder(rec.Body).Decode(&impact)
	if impact.Allowed {
		t.Fatalf("expected suspend-impact allowed=false for sole admin in Beta WS, got %+v", impact)
	}

	// Concurrent removal of the 2 admins in Alpha WS (ownerID + memberID who rejoined as admin): strictly 1 succeeds, 1 returns 409.
	start := make(chan struct{})
	var wg sync.WaitGroup
	codes := make([]int, 2)
	targets := []uuid.UUID{ownerID, memberID}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			resp := doReq(http.MethodDelete, "/api/v1/admin/users/"+targets[idx].String(), "sess-super", wsA.ID.String(), "")
			codes[idx] = resp.Code
		}(i)
	}
	close(start)
	wg.Wait()

	if !((codes[0] == http.StatusOK && codes[1] == http.StatusConflict) || (codes[1] == http.StatusOK && codes[0] == http.StatusConflict)) {
		t.Fatalf("expected one 200 and one 409 for concurrent admin removals, got codes=%v", codes)
	}
}

