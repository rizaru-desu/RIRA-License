// SPDX-License-Identifier: AGPL-3.0-or-later
//
// RiraLicense — license server & API
// Copyright (C) 2026 RiraLab — Rizaru Desu. All rights reserved.

package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "correct-horse-battery-staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected PHC prefix: %s", hash)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = VerifyPassword("wrong-password-1234", hash)
	if err != nil {
		t.Fatalf("VerifyPassword wrong failed: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail verification")
	}

	if _, err := VerifyPassword(password, "$bcrypt$invalid"); err == nil {
		t.Fatal("expected error for invalid hash format")
	}
}

func TestAESGCMEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plaintext := []byte("JBSWY3DPEHPK3PXP")

	ciphertext, err := EncryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESGCM failed: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}

	got, err := DecryptAESGCM(key, ciphertext)
	if err != nil {
		t.Fatalf("DecryptAESGCM failed: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, got)
	}

	// Tampering should fail authentication.
	ciphertext[len(ciphertext)-1] ^= 0xff
	if _, err := DecryptAESGCM(key, ciphertext); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}

func TestTokenAndHMAC(t *testing.T) {
	raw, hash, err := GenerateRandomToken(TokenBytes)
	if err != nil {
		t.Fatalf("GenerateRandomToken failed: %v", err)
	}
	if raw == "" || len(hash) != 64 {
		t.Fatalf("unexpected token/hash: raw=%q len(hash)=%d", raw, len(hash))
	}
	if HashToken(raw) != hash {
		t.Fatal("HashToken mismatch")
	}

	secret := []byte("0123456789abcdef0123456789abcdef")
	signed := SignPayload(secret, "state-123|1700000000")
	got, ok := VerifyPayload(secret, signed)
	if !ok || got != "state-123|1700000000" {
		t.Fatalf("VerifyPayload failed: ok=%v got=%q", ok, got)
	}
	if _, ok := VerifyPayload(secret, signed+"x"); ok {
		t.Fatal("expected tampered signed payload to fail")
	}

	otp, err := GenerateNumericOTP(OTPDigits)
	if err != nil || len(otp) != OTPDigits {
		t.Fatalf("GenerateNumericOTP failed: otp=%q err=%v", otp, err)
	}

	codes, err := GenerateBackupCodes(BackupCodeCount)
	if err != nil || len(codes) != BackupCodeCount {
		t.Fatalf("GenerateBackupCodes failed: len=%d err=%v", len(codes), err)
	}
	if NormalizeBackupCode(strings.ToLower(strings.ReplaceAll(codes[0], "-", ""))) != codes[0] {
		t.Fatalf("NormalizeBackupCode failed for %s", codes[0])
	}
}
