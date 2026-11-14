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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const aes256KeySize = 32

var (
	ErrInvalidKeyLength  = errors.New("AES-256 key must be 32 bytes")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// EncryptAESGCM encrypts plaintext with AES-256-GCM and prepends the random nonce.
func EncryptAESGCM(key, plaintext []byte) ([]byte, error) {
	if len(key) != aes256KeySize {
		return nil, ErrInvalidKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAESGCM decrypts a nonce-prefixed AES-256-GCM ciphertext.
func DecryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	if len(key) != aes256KeySize {
		return nil, ErrInvalidKeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}
	nonce, body := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}
