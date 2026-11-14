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

	"riralicense/internal/db/sqlc"
)

// Store exposes sqlc queries backed by a pgxpool.Pool with transaction support.
type Store struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewStore wraps pool with sqlc generated queries.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// Queries returns the non-transactional sqlc querier.
func (s *Store) Queries() sqlc.Querier {
	return s.queries
}

// WithTx executes fn inside a PostgreSQL transaction, rolling back on error or panic.
func (s *Store) WithTx(ctx context.Context, fn func(q sqlc.Querier) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
