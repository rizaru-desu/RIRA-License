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
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"riralicense/internal/config"
)

// NewPool creates and verifies a bounded pgxpool.Pool using explicit config limits.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.MinConns = cfg.DBMinConns
	poolCfg.MaxConnLifetime = cfg.DBMaxConnLifetime

	connCtx, cancel := context.WithTimeout(ctx, cfg.DBQueryTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(connCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
