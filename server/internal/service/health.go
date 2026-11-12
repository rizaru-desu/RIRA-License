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

package service

import (
	"context"
	"time"
)

// HealthService contains business logic for health checks.
type HealthService struct {
	env       string
	startedAt time.Time
}

func NewHealthService(env string) *HealthService {
	return &HealthService{
		env:       env,
		startedAt: time.Now(),
	}
}

type HealthInfo struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Env       string `json:"env"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`
}

func (s *HealthService) Check(ctx context.Context) HealthInfo {
	return HealthInfo{
		Status:    "ok",
		Service:   "riralicense-api",
		Env:       s.env,
		Uptime:    time.Since(s.startedAt).Round(time.Second).String(),
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
