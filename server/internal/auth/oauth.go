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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"riralicense/internal/db/sqlc"
)

const maxOAuthBodyBytes = 1 << 20 // 1 MiB

type oauthProfile struct {
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

func defaultOAuthEndpoints() OAuthEndpoints {
	return OAuthEndpoints{
		GitHubAuthURL:     github.Endpoint.AuthURL,
		GitHubTokenURL:    github.Endpoint.TokenURL,
		GitHubUserURL:     "https://api.github.com/user",
		GitHubEmailsURL:   "https://api.github.com/user/emails",
		GoogleAuthURL:     google.Endpoint.AuthURL,
		GoogleTokenURL:    google.Endpoint.TokenURL,
		GoogleUserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
	}
}

// EnabledProviders returns the configured OAuth provider names in deterministic order.
func (s *Service) EnabledProviders() []string {
	providers := make([]string, 0, 2)
	if s.oauthCfg.GitHubEnabled() {
		providers = append(providers, ProviderGitHub)
	}
	if s.oauthCfg.GoogleEnabled() {
		providers = append(providers, ProviderGoogle)
	}
	return providers
}

// AuthCodeURL builds the OAuth authorization redirect URL for an enabled provider.
func (s *Service) AuthCodeURL(provider, state string) (string, error) {
	cfg, err := s.oauthConfigFor(provider)
	if err != nil {
		return "", err
	}
	return cfg.AuthCodeURL(state), nil
}

// HandleOAuthCallback exchanges the authorization code, fetches the provider
// profile, resolves or creates the user, and opens a DB session.
func (s *Service) HandleOAuthCallback(ctx context.Context, provider, code string, meta RequestMeta) (*LoginResult, error) {
	cfg, err := s.oauthConfigFor(provider)
	if err != nil {
		return nil, err
	}

	oauthCtx := context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)
	token, err := cfg.Exchange(oauthCtx, code)
	if err != nil {
		s.logger.Warn("oauth code exchange failed", "provider", provider, "err", err)
		return nil, ErrOAuthExchange
	}

	var profile *oauthProfile
	switch provider {
	case ProviderGitHub:
		profile, err = s.fetchGitHubProfile(ctx, token.AccessToken)
	case ProviderGoogle:
		profile, err = s.fetchGoogleProfile(ctx, token.AccessToken)
	default:
		return nil, ErrProviderDisabled
	}
	if err != nil {
		return nil, err
	}

	user, err := s.resolveOrCreateOAuthUser(ctx, provider, profile)
	if err != nil {
		return nil, err
	}

	if user.Status == "suspended" {
		_ = s.store.Queries().DeleteUserSessions(ctx, user.ID)
		s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "login_failed", meta, map[string]string{
			"provider": provider,
			"reason":   "suspended",
		})
		return nil, ErrInvalidCredentials
	}
	if user.DeletedAt != nil {
		return nil, ErrInvalidCredentials
	}

	return s.completeOAuthSession(ctx, user, provider, meta)
}

func (s *Service) oauthConfigFor(provider string) (*oauth2.Config, error) {
	switch provider {
	case ProviderGitHub:
		if !s.oauthCfg.GitHubEnabled() {
			return nil, ErrProviderDisabled
		}
		return &oauth2.Config{
			ClientID:     s.oauthCfg.GitHubClientID,
			ClientSecret: s.oauthCfg.GitHubClientSecret,
			RedirectURL:  s.oauthCfg.GitHubRedirectURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  s.oauthEndpoints.GitHubAuthURL,
				TokenURL: s.oauthEndpoints.GitHubTokenURL,
			},
		}, nil

	case ProviderGoogle:
		if !s.oauthCfg.GoogleEnabled() {
			return nil, ErrProviderDisabled
		}
		redirectURL := s.oauthCfg.GoogleRedirectURL
		if redirectURL == "" {
			redirectURL = s.appURL + "/api/v1/auth/oauth/" + provider + "/callback"
		}
		return &oauth2.Config{
			ClientID:     s.oauthCfg.GoogleClientID,
			ClientSecret: s.oauthCfg.GoogleClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  s.oauthEndpoints.GoogleAuthURL,
				TokenURL: s.oauthEndpoints.GoogleTokenURL,
			},
		}, nil

	default:
		return nil, ErrProviderDisabled
	}
}

func (s *Service) resolveOrCreateOAuthUser(ctx context.Context, provider string, profile *oauthProfile) (sqlc.User, error) {
	acct, err := s.store.Queries().GetOAuthAccount(ctx, sqlc.GetOAuthAccountParams{
		Provider:       provider,
		ProviderUserID: profile.ProviderUserID,
	})
	if err == nil {
		user, userErr := s.store.Queries().GetUserByID(ctx, acct.UserID)
		if userErr != nil {
			if errors.Is(userErr, pgx.ErrNoRows) {
				return sqlc.User{}, ErrInvalidCredentials
			}
			return sqlc.User{}, userErr
		}
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, err
	}

	email := normalizeEmail(profile.Email)
	if email == "" {
		return sqlc.User{}, ErrOAuthEmailUnavailable
	}

	existing, err := s.store.Queries().GetUserByEmailIncludingDeleted(ctx, email)
	if err == nil {
		if existing.Status == "suspended" || existing.DeletedAt != nil {
			return existing, nil
		}
		err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
			if _, txErr := q.CreateOAuthAccount(ctx, sqlc.CreateOAuthAccountParams{
				UserID:         existing.ID,
				Provider:       provider,
				ProviderUserID: profile.ProviderUserID,
			}); txErr != nil {
				return txErr
			}
			if existing.EmailVerifiedAt == nil {
				if txErr := q.UpdateUserEmailVerified(ctx, existing.ID); txErr != nil {
					return txErr
				}
			}
			if profile.AvatarURL != "" {
				if txErr := q.UpdateUserAvatarIfEmpty(ctx, sqlc.UpdateUserAvatarIfEmptyParams{
					ID:        existing.ID,
					AvatarUrl: pgText(profile.AvatarURL),
				}); txErr != nil {
					return txErr
				}
			}
			return nil
		})
		if err != nil {
			return sqlc.User{}, err
		}
		return s.store.Queries().GetUserByID(ctx, existing.ID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, err
	}

	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	now := s.now()
	var created sqlc.User
	err = s.store.WithTx(ctx, func(q sqlc.Querier) error {
		u, txErr := q.CreateUser(ctx, sqlc.CreateUserParams{
			Lower:           email,
			Name:            name,
			AvatarUrl:       pgText(profile.AvatarURL),
			EmailVerifiedAt: &now,
		})
		if txErr != nil {
			return txErr
		}
		if _, txErr := q.CreateOAuthAccount(ctx, sqlc.CreateOAuthAccountParams{
			UserID:         u.ID,
			Provider:       provider,
			ProviderUserID: profile.ProviderUserID,
		}); txErr != nil {
			return txErr
		}
		created = u
		return nil
	})
	if err != nil {
		return sqlc.User{}, err
	}
	return created, nil
}

func (s *Service) completeOAuthSession(ctx context.Context, user sqlc.User, provider string, meta RequestMeta) (*LoginResult, error) {
	var rawSession string
	var sessionExp time.Time

	err := s.store.WithTx(ctx, func(q sqlc.Querier) error {
		if txErr := q.ResetLoginFailuresAndUpdateLastLogin(ctx, user.ID); txErr != nil {
			return txErr
		}
		var txErr error
		rawSession, sessionExp, txErr = s.createSession(ctx, q, user.ID, meta)
		return txErr
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, uuid.NullUUID{UUID: user.ID, Valid: true}, "oauth_login", meta, map[string]string{
		"provider": provider,
	})
	dto := userToDTO(user)
	return &LoginResult{
		SessionToken:     rawSession,
		SessionExpiresAt: sessionExp,
		User:             &dto,
	}, nil
}

func (s *Service) fetchGitHubProfile(ctx context.Context, accessToken string) (*oauthProfile, error) {
	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := s.fetchJSON(ctx, s.oauthEndpoints.GitHubUserURL, accessToken, &ghUser); err != nil {
		return nil, ErrOAuthExchange
	}
	if ghUser.ID == 0 {
		return nil, ErrOAuthExchange
	}

	var ghEmails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := s.fetchJSON(ctx, s.oauthEndpoints.GitHubEmailsURL, accessToken, &ghEmails); err != nil {
		s.logger.Warn("github emails fetch failed", "err", err)
	}

	var primaryEmail string
	for _, entry := range ghEmails {
		if entry.Primary && entry.Verified && entry.Email != "" {
			primaryEmail = entry.Email
			break
		}
	}
	if primaryEmail == "" {
		for _, entry := range ghEmails {
			if entry.Verified && entry.Email != "" {
				primaryEmail = entry.Email
				break
			}
		}
	}
	if primaryEmail == "" && strings.TrimSpace(ghUser.Email) != "" {
		primaryEmail = strings.TrimSpace(ghUser.Email)
	}
	if primaryEmail == "" {
		return nil, ErrOAuthEmailUnavailable
	}

	name := strings.TrimSpace(ghUser.Name)
	if name == "" {
		name = ghUser.Login
	}

	return &oauthProfile{
		ProviderUserID: strconv.FormatInt(ghUser.ID, 10),
		Email:          primaryEmail,
		Name:           name,
		AvatarURL:      ghUser.AvatarURL,
	}, nil
}

func (s *Service) fetchGoogleProfile(ctx context.Context, accessToken string) (*oauthProfile, error) {
	var userInfo struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := s.fetchJSON(ctx, s.oauthEndpoints.GoogleUserInfoURL, accessToken, &userInfo); err != nil {
		return nil, ErrOAuthExchange
	}
	if userInfo.Sub == "" {
		return nil, ErrOAuthExchange
	}
	if !userInfo.EmailVerified || strings.TrimSpace(userInfo.Email) == "" {
		return nil, ErrOAuthEmailUnavailable
	}

	return &oauthProfile{
		ProviderUserID: userInfo.Sub,
		Email:          userInfo.Email,
		Name:           userInfo.Name,
		AvatarURL:      userInfo.Picture,
	}, nil
}

func (s *Service) fetchJSON(ctx context.Context, targetURL, accessToken string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RiraLicense-Server")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxOAuthBodyBytes)
	return json.NewDecoder(limited).Decode(dst)
}
