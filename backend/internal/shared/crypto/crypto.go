// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package crypto provides shared AES-256-GCM encryption primitives and
// HKDF-based key derivation used across modules that need cryptographic
// operations without depending on the vaktvault module.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// DeriveProjectKey derives a 32-byte AES-256 key for a project using HKDF-SHA256.
// The master key is used as the IKM, projectID as the info parameter, and a
// fixed app-specific salt (prevents cross-application key reuse).
func DeriveProjectKey(masterKey []byte, projectID string) ([]byte, error) {
	salt := []byte("vakt-derived-key-v1")
	r := hkdf.New(sha256.New, masterKey, salt, []byte(projectID))
	derived := make([]byte, 32)
	if _, err := io.ReadFull(r, derived); err != nil {
		return nil, fmt.Errorf("hkdf derive project key: %w", err)
	}
	return derived, nil
}

// DeriveServiceKey derives a 32-byte key for a specific internal service using
// HKDF-SHA256. The purpose string must be unique per service (e.g.
// "vakt-paseto-v1", "vakt-vault-v1") to guarantee domain separation — a
// compromise of one derived key cannot be extended to other services.
// Uses a distinct salt from DeriveProjectKey to prevent cross-context reuse.
func DeriveServiceKey(masterKey []byte, purpose string) ([]byte, error) {
	salt := []byte("vakt-service-key-v1")
	r := hkdf.New(sha256.New, masterKey, salt, []byte(purpose))
	derived := make([]byte, 32)
	if _, err := io.ReadFull(r, derived); err != nil {
		return nil, fmt.Errorf("hkdf derive service key (%s): %w", purpose, err)
	}
	return derived, nil
}

// Encrypt encrypts plaintext with AES-256-GCM. Returns ciphertext with the
// 12-byte nonce prepended: [nonce (12 bytes) | ciphertext+tag].
func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-256-GCM ciphertext where the 12-byte nonce is prepended.
func Decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}

	return plaintext, nil
}
