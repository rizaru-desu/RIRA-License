// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package config

import (
	"strings"
	"testing"
)

func TestLoadConfigValidation(t *testing.T) {
	setValidEnv := func(t *testing.T) {
		t.Helper()
		t.Setenv("APP_PORT", "7819")
		t.Setenv("APP_ENV", "development")
		t.Setenv("APP_URL", "http://localhost:5173/")
		t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/riralicense?sslmode=disable")
		t.Setenv("SESSION_SECRET", strings.Repeat("a", 32))
		t.Setenv("TOTP_ENCRYPTION_KEY", strings.Repeat("01", 32))
	}

	t.Run("valid config", func(t *testing.T) {
		setValidEnv(t)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("expected valid config, got error: %v", err)
		}
		if cfg.AppURL != "http://localhost:5173" {
			t.Fatalf("expected trailing slash trimmed, got %q", cfg.AppURL)
		}
		if len(cfg.TOTPEncryptionKey) != 32 {
			t.Fatalf("expected 32-byte TOTP key, got %d", len(cfg.TOTPEncryptionKey))
		}
		if cfg.OAuth.GitHubEnabled() || cfg.OAuth.GoogleEnabled() {
			t.Fatal("expected optional OAuth providers disabled when unset")
		}
	})

	t.Run("missing APP_URL", func(t *testing.T) {
		setValidEnv(t)
		t.Setenv("APP_URL", "")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for missing APP_URL")
		}
	})

	t.Run("short TOTP_ENCRYPTION_KEY", func(t *testing.T) {
		setValidEnv(t)
		t.Setenv("TOTP_ENCRYPTION_KEY", "short")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for invalid TOTP_ENCRYPTION_KEY")
		}
	})

	t.Run("short SESSION_SECRET", func(t *testing.T) {
		setValidEnv(t)
		t.Setenv("SESSION_SECRET", "too-short")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for short SESSION_SECRET")
		}
	})
}
