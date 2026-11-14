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

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"riralicense/migrations"
)

// RunMigrations applies all pending up migrations embedded in server/migrations.
func RunMigrations(ctx context.Context, databaseURL string) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration db: %w", err)
	}

	srcDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	dbDriver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		_ = srcDriver.Close()
		return fmt.Errorf("create pgx migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "pgx", dbDriver)
	if err != nil {
		_ = srcDriver.Close()
		_ = dbDriver.Close()
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
