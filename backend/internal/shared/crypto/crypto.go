// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package crypto provides shared AES-256-GCM encryption primitives and
// HKDF-based key derivation used across modules that need cryptographic
// operations without depending on the vaktvault module.
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// aadMarkerV2 prefixes ciphertexts that were sealed with Associated Data (S90-3,
// ADR-0059). Legacy values (plain Encrypt / pre-v2 writes) carry NO marker and
// are decrypted without AAD for backward compatibility. The marker lets Decrypt
// choose the right path without a schema change or separate version column.
var aadMarkerV2 = []byte("enc:v2:")

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

// Encrypt encrypts plaintext with AES-256-GCM, no Associated Data. Returns the
// legacy format [nonce (12 bytes) | ciphertext+tag] with NO marker — unchanged
// so existing callers and stored values are not affected.
func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	return seal(key, plaintext, nil, false)
}

// EncryptWithAAD encrypts plaintext binding it to the given Associated Data
// (S90-3). The AAD is authenticated but NOT stored — the caller must supply the
// identical AAD on decrypt. Output carries the enc:v2: marker so Decrypt selects
// the AAD path. Use a context-unique AAD (e.g. "<org_id>:<secret_id>") so a valid
// ciphertext cannot be copied between rows/orgs without the GCM tag check failing.
func EncryptWithAAD(key, plaintext, aad []byte) ([]byte, error) {
	return seal(key, plaintext, aad, true)
}

func seal(key, plaintext, aad []byte, marker bool) ([]byte, error) {
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
	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad)
	if marker {
		return append(append([]byte{}, aadMarkerV2...), ciphertext...), nil
	}
	return ciphertext, nil
}

// Decrypt decrypts AES-256-GCM ciphertext (legacy, no AAD). It also transparently
// handles enc:v2: values that were sealed with empty AAD. Equivalent to
// DecryptWithAAD(key, ciphertext, nil).
func Decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	return DecryptWithAAD(key, ciphertext, nil)
}

// DecryptWithAAD decrypts a value, choosing the AAD path by marker (S90-3):
//   - enc:v2: prefix present → strip marker, GCM-Open with the supplied aad.
//   - no marker (legacy) → GCM-Open with NO aad (the supplied aad is ignored),
//     so pre-v2 ciphertexts remain decryptable (backward compatible / lazy-upgrade).
func DecryptWithAAD(key, data, aad []byte) ([]byte, error) {
	if bytes.HasPrefix(data, aadMarkerV2) {
		return open(key, data[len(aadMarkerV2):], aad)
	}
	return open(key, data, nil) // legacy: never bound to AAD
}

func open(key, ciphertext, aad []byte) ([]byte, error) {
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
	plaintext, err := gcm.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plaintext, nil
}
