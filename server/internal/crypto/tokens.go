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

package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	TokenBytes      = 32
	BackupCodeCount = 10
	backupCodeHalf  = 4
	backupAlphabet  = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	OTPDigits       = 6
)

// GenerateRandomToken generates a cryptographically random URL-safe token and its SHA-256 hex digest.
func GenerateRandomToken(numBytes int) (raw string, hash string, err error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate random token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken returns the lowercase SHA-256 hex digest of a raw token or OTP code.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeEqualHex compares two hex digests in constant time.
func ConstantTimeEqualHex(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GenerateNumericOTP generates a zero-padded numeric OTP of the requested length using uniform sampling.
func GenerateNumericOTP(digits int) (string, error) {
	max := big.NewInt(10)
	var b strings.Builder
	b.Grow(digits)
	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate otp digit: %w", err)
		}
		b.WriteByte(byte('0' + n.Int64()))
	}
	return b.String(), nil
}

// GenerateBackupCodes generates count single-use backup codes formatted as XXXX-XXXX.
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, 0, count)
	alphaLen := big.NewInt(int64(len(backupAlphabet)))
	for i := 0; i < count; i++ {
		var b strings.Builder
		b.Grow(backupCodeHalf*2 + 1)
		for j := 0; j < backupCodeHalf*2; j++ {
			if j == backupCodeHalf {
				b.WriteByte('-')
			}
			idx, err := rand.Int(rand.Reader, alphaLen)
			if err != nil {
				return nil, fmt.Errorf("generate backup code: %w", err)
			}
			b.WriteByte(backupAlphabet[idx.Int64()])
		}
		codes = append(codes, b.String())
	}
	return codes, nil
}

// NormalizeBackupCode normalizes user input for backup code verification.
func NormalizeBackupCode(code string) string {
	cleaned := strings.ToUpper(strings.TrimSpace(code))
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if len(cleaned) == backupCodeHalf*2 {
		return cleaned[:backupCodeHalf] + "-" + cleaned[backupCodeHalf:]
	}
	return strings.ToUpper(strings.TrimSpace(code))
}

// SignPayload encodes payload as base64url and appends an HMAC-SHA256 signature.
func SignPayload(secret []byte, payload string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := computeHMAC(secret, encoded)
	return encoded + "." + sig
}

// VerifyPayload validates an HMAC-SHA256 signed payload and returns the original string.
func VerifyPayload(secret []byte, signed string) (string, bool) {
	encoded, sigHex, ok := strings.Cut(signed, ".")
	if !ok || encoded == "" || sigHex == "" {
		return "", false
	}
	expected := computeHMAC(secret, encoded)
	if !hmac.Equal([]byte(sigHex), []byte(expected)) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func computeHMAC(secret []byte, data string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
