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

package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort              = 7819
	defaultEnv               = "development"
	defaultDBMaxConns        = int32(20)
	defaultDBMinConns        = int32(2)
	defaultDBMaxConnLifetime = time.Hour
	defaultDBQueryTimeout    = 5 * time.Second
	defaultHTTPTimeout       = 15 * time.Second
	defaultSMTPPort          = 587
	defaultSMTPFrom          = "no-reply@riralicense.local"
	defaultSMTPTLS           = "starttls"
	minSessionSecretLen      = 32
	aes256KeyBytes           = 32
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port               int
	Env                string
	AppURL             string
	DatabaseURL        string
	DBMaxConns         int32
	DBMinConns         int32
	DBMaxConnLifetime  time.Duration
	DBQueryTimeout     time.Duration
	HTTPTimeout        time.Duration
	SessionSecret      []byte
	TOTPEncryptionKey  []byte
	CookieSecure       bool
	CORSAllowedOrigins []string
	SMTP               SMTPConfig
	OAuth              OAuthConfig
}

// SMTPConfig holds outbound SMTP mail configuration.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
	TLS  string
}

// OAuthConfig holds optional OAuth2 provider credentials.
type OAuthConfig struct {
	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
}

// GitHubEnabled reports whether GitHub OAuth is configured.
func (o OAuthConfig) GitHubEnabled() bool {
	return o.GitHubClientID != "" && o.GitHubClientSecret != ""
}

// GoogleEnabled reports whether Google OAuth is configured.
func (o OAuthConfig) GoogleEnabled() bool {
	return o.GoogleClientID != "" && o.GoogleClientSecret != ""
}

// Load reads and validates configuration from environment variables.
func Load() (*Config, error) {
	port, err := envInt("APP_PORT", defaultPort)
	if err != nil {
		return nil, err
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("APP_PORT must be between 1 and 65535")
	}

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = defaultEnv
	}

	appURL, err := parseAppURL(os.Getenv("APP_URL"))
	if err != nil {
		return nil, err
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	maxConns, err := envInt32("DB_MAX_CONNS", defaultDBMaxConns)
	if err != nil || maxConns <= 0 {
		return nil, errors.New("DB_MAX_CONNS must be a positive integer")
	}

	minConns, err := envInt32("DB_MIN_CONNS", defaultDBMinConns)
	if err != nil || minConns < 0 || minConns > maxConns {
		return nil, errors.New("DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
	}

	maxConnLifetime, err := envDuration("DB_MAX_CONN_LIFETIME", defaultDBMaxConnLifetime)
	if err != nil || maxConnLifetime <= 0 {
		return nil, errors.New("DB_MAX_CONN_LIFETIME must be a positive duration")
	}

	dbQueryTimeout, err := envDuration("DB_QUERY_TIMEOUT", defaultDBQueryTimeout)
	if err != nil || dbQueryTimeout <= 0 {
		return nil, errors.New("DB_QUERY_TIMEOUT must be a positive duration")
	}

	httpTimeout, err := envDuration("HTTP_TIMEOUT", defaultHTTPTimeout)
	if err != nil || httpTimeout <= 0 {
		return nil, errors.New("HTTP_TIMEOUT must be a positive duration")
	}

	sessionSecret, err := parseSecretBytes("SESSION_SECRET", os.Getenv("SESSION_SECRET"), minSessionSecretLen)
	if err != nil {
		return nil, err
	}

	totpKey, err := parseExact32ByteKey("TOTP_ENCRYPTION_KEY", os.Getenv("TOTP_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}

	cookieSecure := strings.HasPrefix(appURL, "https://") || env == "production"
	if raw := strings.TrimSpace(os.Getenv("COOKIE_SECURE")); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("COOKIE_SECURE must be a boolean: %w", parseErr)
		}
		cookieSecure = parsed
	}

	corsOrigins := parseCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(corsOrigins) == 0 {
		corsOrigins = []string{appURL}
	}

	smtpPort, err := envInt("SMTP_PORT", defaultSMTPPort)
	if err != nil || smtpPort <= 0 || smtpPort > 65535 {
		return nil, errors.New("SMTP_PORT must be between 1 and 65535")
	}

	smtpFrom := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if smtpFrom == "" {
		smtpFrom = defaultSMTPFrom
	}

	smtpTLS := strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_TLS")))
	if smtpTLS == "" {
		smtpTLS = defaultSMTPTLS
	}
	switch smtpTLS {
	case "none", "starttls", "tls":
	default:
		return nil, errors.New("SMTP_TLS must be one of: none, starttls, tls")
	}

	return &Config{
		Port:               port,
		Env:                env,
		AppURL:             appURL,
		DatabaseURL:        dbURL,
		DBMaxConns:         maxConns,
		DBMinConns:         minConns,
		DBMaxConnLifetime:  maxConnLifetime,
		DBQueryTimeout:     dbQueryTimeout,
		HTTPTimeout:        httpTimeout,
		SessionSecret:      sessionSecret,
		TOTPEncryptionKey:  totpKey,
		CookieSecure:       cookieSecure,
		CORSAllowedOrigins: corsOrigins,
		SMTP: SMTPConfig{
			Host: strings.TrimSpace(os.Getenv("SMTP_HOST")),
			Port: smtpPort,
			User: strings.TrimSpace(os.Getenv("SMTP_USER")),
			Pass: os.Getenv("SMTP_PASS"),
			From: smtpFrom,
			TLS:  smtpTLS,
		},
		OAuth: OAuthConfig{
			GitHubClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
			GitHubClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
			GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
			GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		},
	}, nil
}

func parseAppURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", errors.New("APP_URL is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("APP_URL is invalid: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("APP_URL must be an http or https URL with a host")
	}
	return trimmed, nil
}

func parseSecretBytes(name, raw string, minLen int) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) >= minLen {
		return decoded, nil
	}
	if len(trimmed) < minLen {
		return nil, fmt.Errorf("%s must be at least %d bytes", name, minLen)
	}
	return []byte(trimmed), nil
}

func parseExact32ByteKey(name, raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == aes256KeyBytes {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == aes256KeyBytes {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(decoded) == aes256KeyBytes {
		return decoded, nil
	}
	if len(trimmed) == aes256KeyBytes {
		return []byte(trimmed), nil
	}
	return nil, fmt.Errorf("%s must decode to exactly %d bytes (64 hex chars or base64)", name, aes256KeyBytes)
}

func envInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return v, nil
}

func envInt32(key string, fallback int32) (int32, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be a 32-bit integer", key)
	}
	return int32(v), nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration", key)
	}
	return d, nil
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimRight(strings.TrimSpace(p), "/")
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
