// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// TestGenerateSymmetricKeyFromBytes_WrongLength verifies that Paseto v4 rejects
// keys that are not exactly 32 bytes — incorrect key material must not silently
// produce a usable but weak key.
func TestGenerateSymmetricKeyFromBytes_WrongLength(t *testing.T) {
	cases := []int{0, 1, 16, 31, 33, 64}
	for _, n := range cases {
		_, err := auth.GenerateSymmetricKeyFromBytes(make([]byte, n))
		assert.Error(t, err, "key of %d bytes should be rejected", n)
	}
}

// TestGenerateSymmetricKeyFromBytes_ValidLength verifies that exactly 32 bytes
// produces a usable key — round-trip a token to confirm.
func TestGenerateSymmetricKeyFromBytes_ValidLength(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1) // non-zero bytes to avoid trivially weak key
	}
	key, err := auth.GenerateSymmetricKeyFromBytes(raw)
	require.NoError(t, err)

	claims := auth.Claims{UserID: "u1", OrgID: "o1", Roles: []string{"Viewer"}, PwVersion: 3}
	tok, err := auth.IssueAccessToken(key, claims)
	require.NoError(t, err)

	parsed, err := auth.ParseAccessToken(key, tok)
	require.NoError(t, err)
	assert.Equal(t, claims.UserID, parsed.UserID)
	assert.Equal(t, claims.PwVersion, parsed.PwVersion)
}

// TestIssueAccessToken_PwVersionRoundTrip verifies that the pw_version claim
// survives the encode → decode cycle. This is a security invariant: a stale
// pw_version in the token must be detectable so middleware can force re-auth
// after a password change.
func TestIssueAccessToken_PwVersionRoundTrip(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	for _, version := range []int64{0, 1, 42, 9999} {
		claims := auth.Claims{
			UserID:    "user-pw",
			OrgID:     "org-pw",
			Roles:     []string{"Admin"},
			PwVersion: version,
		}
		tok, err := auth.IssueAccessToken(key, claims)
		require.NoError(t, err)

		parsed, err := auth.ParseAccessToken(key, tok)
		require.NoError(t, err)
		assert.Equal(t, version, parsed.PwVersion, "pw_version %d must survive round-trip", version)
	}
}

// TestParseAccessToken_WrongKey verifies that a token encrypted under key A
// cannot be decrypted under key B — tamper-evidence at the token level.
func TestParseAccessToken_WrongKey(t *testing.T) {
	keyA, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	// Key B: flip all bits of testHexKey
	altHex := "fefdfcfbfaf9f8f7f6f5f4f3f2f1f0efeeedecebeae9e8e7e6e5e4e3e2e1e0df"
	keyB, err := auth.GenerateSymmetricKey(altHex)
	require.NoError(t, err)

	tok, err := auth.IssueAccessToken(keyA, auth.Claims{UserID: "u", OrgID: "o", Roles: []string{"Viewer"}})
	require.NoError(t, err)

	_, err = auth.ParseAccessToken(keyB, tok)
	assert.Error(t, err, "token from keyA must not parse under keyB")
}
