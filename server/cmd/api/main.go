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

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"riralicense/internal/auth"
	"riralicense/internal/config"
	"riralicense/internal/db"
	"riralicense/internal/handler"
	"riralicense/internal/mail"
	"riralicense/internal/ratelimit"
	"riralicense/internal/router"
	"riralicense/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startupCancel()

	if err := db.RunMigrations(startupCtx, cfg.DatabaseURL); err != nil {
		logger.Error("database migration failed", "err", err)
		os.Exit(1)
	}

	pool, err := db.NewPool(startupCtx, cfg)
	if err != nil {
		logger.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	mailer, err := mail.NewMailer(cfg.SMTP, cfg.Env, logger)
	if err != nil {
		logger.Error("mailer initialization failed", "err", err)
		os.Exit(1)
	}

	limiters := ratelimit.NewAuthLimiters()
	defer limiters.Stop()

	store := db.NewStore(pool)
	authSvc := auth.NewService(store, mailer, cfg, logger)
	authHandler := handler.NewAuthHandler(authSvc, cfg, limiters, logger)

	healthSvc := service.NewHealthService(cfg.Env)
	healthHandler := handler.NewHealthHandler(healthSvc)

	r := router.New(cfg, healthHandler, authHandler, authSvc)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in a goroutine so we can listen for shutdown signals.
	go func() {
		logger.Info("server starting", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "err", err)
	}

	logger.Info("server stopped")
}
