// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package migrations

import "embed"

// FS holds all embedded SQL migration files.
//
//go:embed *.sql
var FS embed.FS
