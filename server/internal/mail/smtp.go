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
	"fmt"
	"log/slog"
	"time"

	gomail "github.com/wneessen/go-mail"

	"riralicense/internal/config"
)

const mailSendTimeout = 10 * time.Second

type smtpMailer struct {
	client *gomail.Client
	from   string
	logger *slog.Logger
}

type devLogMailer struct {
	isProd bool
	logger *slog.Logger
}

// NewMailer builds an SMTP-backed Mailer when SMTP_HOST is configured, or a
// DEV-ONLY log fallback otherwise.
func NewMailer(cfg config.SMTPConfig, env string, logger *slog.Logger) (Mailer, error) {
	if cfg.Host == "" {
		return &devLogMailer{
			isProd: env == "production",
			logger: logger,
		}, nil
	}

	opts := []gomail.Option{
		gomail.WithPort(cfg.Port),
		gomail.WithTimeout(mailSendTimeout),
	}

	switch cfg.TLS {
	case "none":
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	case "tls":
		opts = append(opts, gomail.WithSSL())
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	}

	if cfg.User != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(cfg.User),
			gomail.WithPassword(cfg.Pass),
		)
	}

	client, err := gomail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("init smtp client: %w", err)
	}

	return &smtpMailer{
		client: client,
		from:   cfg.From,
		logger: logger,
	}, nil
}

func (m *smtpMailer) SendVerificationEmail(ctx context.Context, toEmail, name, verifyURL string) error {
	subject := "Verify your RiraLicense email"
	body := fmt.Sprintf(
		"Hello %s,\n\nVerify your RiraLicense account by opening this link:\n%s\n\nThis link expires in 24 hours.\n",
		name,
		verifyURL,
	)
	return m.send(ctx, toEmail, subject, body)
}

func (m *smtpMailer) SendPasswordResetEmail(ctx context.Context, toEmail, name, resetURL string) error {
	subject := "Reset your RiraLicense password"
	body := fmt.Sprintf(
		"Hello %s,\n\nReset your password by opening this link:\n%s\n\nThis link expires in 1 hour. If you did not request this, you can ignore this email.\n",
		name,
		resetURL,
	)
	return m.send(ctx, toEmail, subject, body)
}

func (m *smtpMailer) SendEmailChangeEmail(ctx context.Context, toEmail, name, confirmURL string) error {
	subject := "Confirm your new RiraLicense email address"
	body := fmt.Sprintf(
		"Hello %s,\n\nConfirm your new email address by opening this link:\n%s\n\nThis link expires in 24 hours.\n",
		name,
		confirmURL,
	)
	return m.send(ctx, toEmail, subject, body)
}

func (m *smtpMailer) SendEmailOTP(ctx context.Context, toEmail, name, code string) error {
	subject := "Your RiraLicense verification code"
	body := fmt.Sprintf(
		"Hello %s,\n\nYour one-time verification code is: %s\n\nThis code expires in 10 minutes.\n",
		name,
		code,
	)
	return m.send(ctx, toEmail, subject, body)
}

func (m *smtpMailer) send(ctx context.Context, toEmail, subject, body string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.from); err != nil {
		m.logger.Error("invalid smtp sender address", "err", err)
		return ErrMailSendFailed
	}
	if err := msg.To(toEmail); err != nil {
		m.logger.Error("invalid smtp recipient address", "err", err)
		return ErrMailSendFailed
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, body)

	sendCtx, cancel := context.WithTimeout(ctx, mailSendTimeout)
	defer cancel()

	if err := m.client.DialAndSendWithContext(sendCtx, msg); err != nil {
		m.logger.Error("smtp send failed", "err", err)
		return ErrMailSendFailed
	}
	return nil
}

func (m *devLogMailer) SendVerificationEmail(_ context.Context, toEmail, _ string, verifyURL string) error {
	return m.logDev("verification_email", toEmail, "verify_url", verifyURL)
}

func (m *devLogMailer) SendPasswordResetEmail(_ context.Context, toEmail, _ string, resetURL string) error {
	return m.logDev("password_reset_email", toEmail, "reset_url", resetURL)
}

func (m *devLogMailer) SendEmailChangeEmail(_ context.Context, toEmail, _ string, confirmURL string) error {
	return m.logDev("email_change_email", toEmail, "confirm_url", confirmURL)
}

func (m *devLogMailer) SendEmailOTP(_ context.Context, toEmail, _ string, code string) error {
	return m.logDev("email_otp", toEmail, "otp_code", code)
}

func (m *devLogMailer) logDev(kind, toEmail, field, value string) error {
	if m.isProd {
		m.logger.Warn("smtp not configured in production; dropping email", "kind", kind)
		return ErrMailUnavailable
	}
	m.logger.Info("DEV-ONLY email fallback (SMTP_HOST not configured)",
		"kind", kind,
		"to", toEmail,
		field, value,
	)
	return nil
}
