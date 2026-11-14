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

package mail

import (
	"context"
	"errors"
)

var (
	ErrMailUnavailable = errors.New("email delivery is not configured")
	ErrMailSendFailed  = errors.New("failed to send email")
)

// Mailer delivers transactional authentication emails.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, toEmail, name, verifyURL string) error
	SendPasswordResetEmail(ctx context.Context, toEmail, name, resetURL string) error
	SendEmailChangeEmail(ctx context.Context, toEmail, name, confirmURL string) error
	SendEmailOTP(ctx context.Context, toEmail, name, code string) error
	SendAccountDeletionEmail(ctx context.Context, toEmail, name, confirmURL string) error
	SendAccountRestoreEmail(ctx context.Context, toEmail, name, restoreURL string) error
}
