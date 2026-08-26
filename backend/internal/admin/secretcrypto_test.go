// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

func testMasterKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

// TestSealSecretUsesDerivedKeyNotRawMaster is the point of the whole change:
// before this, admin secrets were sealed with the RAW master key, so they had
// no rotation stage and a key swap made them permanently undecryptable.
func TestSealSecretUsesDerivedKeyNotRawMaster(t *testing.T) {
	master := testMasterKey(t)
	s := &Service{masterKey: master}

	sealed, err := s.sealSecret([]byte("top-secret"))
	require.NoError(t, err)
	require.True(t, len(sealed) > len(SecretMarker))
	assert.Equal(t, SecretMarker, string(sealed[:len(SecretMarker)]), "sealed values must carry the marker")

	body := sealed[len(SecretMarker):]

	derived, err := sharedcrypto.DeriveServiceKey(master, SecretKeyPurpose)
	require.NoError(t, err)
	plain, err := sharedcrypto.Decrypt(derived, body)
	require.NoError(t, err, "the derived sub-key must open it")
	assert.Equal(t, "top-secret", string(plain))

	_, err = sharedcrypto.Decrypt(master, body)
	assert.Error(t, err, "the RAW master must NOT open an admin secret any more")
}

// TestOpenSecretReadsAllThreeFormats is what makes the change deployable before
// the stock has been re-keyed: an instance that upgrades and never runs
// `rotate-key admin-rekey up` must keep working.
func TestOpenSecretReadsAllThreeFormats(t *testing.T) {
	master := testMasterKey(t)
	s := &Service{masterKey: master}
	const secret = "a-stored-credential"

	t.Run("current: marked, derived sub-key", func(t *testing.T) {
		sealed, err := s.sealSecret([]byte(secret))
		require.NoError(t, err)
		plain, err := s.openSecret(sealed)
		require.NoError(t, err)
		assert.Equal(t, secret, string(plain))
	})

	t.Run("legacy: bare raw-master ciphertext", func(t *testing.T) {
		ct, err := sharedcrypto.Encrypt(master, []byte(secret))
		require.NoError(t, err)
		plain, err := s.openSecret(ct)
		require.NoError(t, err)
		assert.Equal(t, secret, string(plain))
	})

	t.Run("legacy: enc:v1: + base64, the old OIDC wrapper", func(t *testing.T) {
		ct, err := sharedcrypto.Encrypt(master, []byte(secret))
		require.NoError(t, err)
		stored := []byte("enc:v1:" + base64.URLEncoding.EncodeToString(ct))
		plain, err := s.openSecret(stored)
		require.NoError(t, err)
		assert.Equal(t, secret, string(plain))
	})
}

// TestOpenSecretRejectsForeignValues: openSecret must fail loudly rather than
// return something plausible. A silent wrong answer here would be written back
// as a "migrated" secret.
func TestOpenSecretRejectsForeignValues(t *testing.T) {
	s := &Service{masterKey: testMasterKey(t)}

	_, err := s.openSecret(nil)
	assert.Error(t, err, "empty value")

	_, err = s.openSecret([]byte("not a ciphertext at all"))
	assert.Error(t, err, "garbage")

	other := &Service{masterKey: testMasterKey(t)}
	sealed, err := other.sealSecret([]byte("x"))
	require.NoError(t, err)
	_, err = s.openSecret(sealed)
	assert.Error(t, err, "a value sealed under a different master must not open")
}

// TestAdminSecretKeyRequiresMasterKey: without a key, sealing must refuse
// rather than write something unprotected.
func TestAdminSecretKeyRequiresMasterKey(t *testing.T) {
	s := &Service{}
	_, err := s.sealSecret([]byte("x"))
	assert.Error(t, err)
	_, err = s.openSecret([]byte("anything"))
	assert.Error(t, err)
}

// TestMasterKeyStaysRawForBearerToken pins the reason masterKey is kept beside
// the derived key: the internal backup-config endpoint authenticates with
// hex(masterKey). Deriving in place would have silently broken the backup
// sidecar's auth instead of failing a test.
func TestMasterKeyStaysRawForBearerToken(t *testing.T) {
	master := testMasterKey(t)
	s := NewService(nil, "").WithMasterKey(master)
	assert.Equal(t, master, s.masterKey, "the raw master must stay reachable for the bearer token")

	_, err := s.sealSecret([]byte("x"))
	require.NoError(t, err)
	assert.NotEqual(t, master, s.secretKey, "the sealing key must not BE the master key")
}
