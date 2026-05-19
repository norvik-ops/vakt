// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// randomKey generates a fresh 32-byte AES-256 key for each test.
func randomKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err, "failed to generate random key")
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := randomKey(t)
	plaintext := []byte("hello, world — test data with ünicode and special chars!@#")

	ct, err := Encrypt(key, plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ct, "ciphertext must not be empty")
	assert.False(t, bytes.Equal(plaintext, ct), "ciphertext must differ from plaintext")

	recovered, err := Decrypt(key, ct)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered, "decrypted value must match original plaintext")
}

func TestEncryptDifferentEachTime(t *testing.T) {
	key := randomKey(t)
	plaintext := []byte("same plaintext every time")

	ct1, err := Encrypt(key, plaintext)
	require.NoError(t, err)
	ct2, err := Encrypt(key, plaintext)
	require.NoError(t, err)

	assert.False(t, bytes.Equal(ct1, ct2),
		"encrypting the same plaintext twice must produce different ciphertexts (random nonce)")
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := randomKey(t)
	key2 := randomKey(t)

	ct, err := Encrypt(key1, []byte("secret payload"))
	require.NoError(t, err)

	_, err = Decrypt(key2, ct)
	assert.Error(t, err, "decrypting with a wrong key must fail")
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := randomKey(t)

	ct, err := Encrypt(key, []byte("tamper me"))
	require.NoError(t, err)

	// Flip the last byte of the ciphertext to corrupt the GCM authentication tag.
	ct[len(ct)-1] ^= 0xFF

	_, err = Decrypt(key, ct)
	assert.Error(t, err, "a tampered ciphertext must fail authentication and decryption")
}

func TestDecryptTamperedNonce(t *testing.T) {
	key := randomKey(t)

	ct, err := Encrypt(key, []byte("nonce tamper"))
	require.NoError(t, err)

	// Flip the first byte (part of the prepended nonce).
	ct[0] ^= 0xFF

	_, err = Decrypt(key, ct)
	assert.Error(t, err, "a tampered nonce must cause decryption to fail")
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	key := randomKey(t)

	ct, err := Encrypt(key, []byte{})
	require.NoError(t, err, "encrypting empty plaintext must succeed")
	assert.NotEmpty(t, ct, "ciphertext for empty plaintext must still contain nonce+tag")

	recovered, err := Decrypt(key, ct)
	require.NoError(t, err)
	// gcm.Open returns nil (not []byte{}) for zero-length plaintext; both are
	// semantically empty so we check length rather than deep-equal.
	assert.Empty(t, recovered, "decrypting empty plaintext must return an empty result")
}

func TestDecryptTooShortCiphertext(t *testing.T) {
	key := randomKey(t)

	// AES-GCM nonce is 12 bytes; anything shorter must be rejected.
	short := make([]byte, 5)
	_, err := Decrypt(key, short)
	assert.Error(t, err, "ciphertext shorter than nonce size must fail")
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	// AES-256 requires a 32-byte key; a 16-byte key is AES-128 — still valid.
	// But a 7-byte key is not a valid AES key size.
	badKey := make([]byte, 7)
	_, err := Encrypt(badKey, []byte("test"))
	assert.Error(t, err, "encrypting with an invalid key length must fail")
}

func TestDecryptInvalidKeyLength(t *testing.T) {
	badKey := make([]byte, 7)
	// Attempt to decrypt a plausible-length ciphertext with an invalid key.
	fakeCT := make([]byte, 30) // nonce(12) + some bytes
	_, err := Decrypt(badKey, fakeCT)
	assert.Error(t, err, "decrypting with an invalid key length must fail")
}

func TestDeriveProjectKey_Deterministic(t *testing.T) {
	masterKey := randomKey(t)
	projectID := "project-abc-123"

	k1, err := DeriveProjectKey(masterKey, projectID)
	require.NoError(t, err)
	assert.Len(t, k1, 32, "derived key must be 32 bytes")

	k2, err := DeriveProjectKey(masterKey, projectID)
	require.NoError(t, err)

	assert.Equal(t, k1, k2, "DeriveProjectKey must be deterministic for the same inputs")
}

func TestDeriveProjectKey_DifferentProjects(t *testing.T) {
	masterKey := randomKey(t)

	k1, err := DeriveProjectKey(masterKey, "project-a")
	require.NoError(t, err)
	k2, err := DeriveProjectKey(masterKey, "project-b")
	require.NoError(t, err)

	assert.False(t, bytes.Equal(k1, k2), "different project IDs must produce different derived keys")
}

func TestDeriveProjectKey_DifferentMasterKeys(t *testing.T) {
	master1 := randomKey(t)
	master2 := randomKey(t)
	projectID := "shared-project"

	k1, err := DeriveProjectKey(master1, projectID)
	require.NoError(t, err)
	k2, err := DeriveProjectKey(master2, projectID)
	require.NoError(t, err)

	assert.False(t, bytes.Equal(k1, k2), "different master keys must produce different derived keys")
}

func TestDeriveAndEncryptRoundTrip(t *testing.T) {
	masterKey := randomKey(t)

	derived, err := DeriveProjectKey(masterKey, "my-project")
	require.NoError(t, err)

	plaintext := []byte("project-scoped secret value")
	ct, err := Encrypt(derived, plaintext)
	require.NoError(t, err)

	recovered, err := Decrypt(derived, ct)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered)
}
