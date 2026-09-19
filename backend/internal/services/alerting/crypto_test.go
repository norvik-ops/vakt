// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package alerting

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
)

// testKey is a fixed 32-byte AES-256 key. Not a secret — a test constant.
var testKey = bytes.Repeat([]byte{0x2a}, 32)

// legacyEncrypt reproduces byte-for-byte the inline AES-256-GCM encryption the
// alerting service used before R1-W9B-N1 consolidated it onto sharedcrypto:
// [nonce (12 bytes) | ciphertext+tag], no Associated Data. It exists only to
// prove that ciphertexts written by the old code path still decrypt with the
// new shared implementation (backward compatibility).
func legacyEncrypt(key, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil)
}

// legacyDecrypt is the old inline decryption, used to prove that ciphertexts the
// NEW shared path writes are still readable by the old layout expectation.
func legacyDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

func newTestService() *Service {
	return &Service{masterKey: testKey}
}

// TestAlertingCrypto_RoundTrip verifies the consolidated encrypt/decrypt pair
// round-trips through the shared crypto primitive.
func TestAlertingCrypto_RoundTrip(t *testing.T) {
	s := newTestService()
	plain := []byte("https://hooks.example.com/webhook/abc?token=xyz")

	ct, err := s.encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext equals plaintext — encryption did not run")
	}

	got, err := s.decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plain)
	}
}

// TestAlertingCrypto_DecryptsLegacyCiphertext is the load-bearing backward-
// compatibility check for R1-W9B-N1: a channel URL/HMAC secret encrypted with
// the OLD inline path must still decrypt after the switch to sharedcrypto.
func TestAlertingCrypto_DecryptsLegacyCiphertext(t *testing.T) {
	s := newTestService()
	plain := []byte("super-secret-hmac-key-material")

	legacyCT := legacyEncrypt(testKey, plain)

	got, err := s.decrypt(legacyCT)
	if err != nil {
		t.Fatalf("new decrypt failed on legacy ciphertext — stored secrets would be unreadable: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("legacy ciphertext decrypted to %q, want %q", got, plain)
	}
}

// TestAlertingCrypto_NewCiphertextReadableByLegacy proves the format is
// unchanged in the other direction: what the new path writes still fits the old
// [nonce | ciphertext+tag] layout, so a rollback could still read it.
func TestAlertingCrypto_NewCiphertextReadableByLegacy(t *testing.T) {
	s := newTestService()
	plain := []byte("channel-url-value")

	newCT, err := s.encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := legacyDecrypt(testKey, newCT)
	if err != nil {
		t.Fatalf("legacy decrypt failed on new ciphertext — format drifted: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("format mismatch: got %q want %q", got, plain)
	}
}
