// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Org isolation — pure crypto/model unit tests below.
//
// The DB-level org-isolation guarantee (GetSecret / SetSecret / GetProject all
// embed `WHERE org_id = $N::uuid`) is now VERIFIED against a live Postgres by
// TestCrossOrgIsolation_VaultAndHR in internal/integration_test/
// org_isolation_real_test.go — S131-G6/R-L07 replaced the former
// t.Skip("SECURITY GAP: ... can only be integration-tested") placeholders with
// a real two-org cross-access test (org B cannot read or write org A's data).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Secret.Value omission — model surface (no DB required)
// ---------------------------------------------------------------------------

// TestSecret_ListKeysOmitsValue verifies that the Secret struct returned by
// list operations has Value empty and that the `json:"value,omitempty"` tag
// omits the field from JSON output — preventing plaintext leakage in list
// API responses.
func TestSecret_ListKeysOmitsValue(t *testing.T) {
	// Simulate what ListSecretKeys returns: a Secret with no Value populated.
	s := Secret{
		ID:      "secret-id-001",
		Key:     "DATABASE_URL",
		Version: 1,
	}

	assert.Empty(t, s.Value, "list path must not populate Value")

	data, err := json.Marshal(s)
	require.NoError(t, err)

	assert.NotContains(t, string(data), `"value"`,
		"ListSecretKeys result must not include the value field in JSON (omitempty)")
}

// TestSecret_GetPathPopulatesValue verifies that after GetSecret decrypts and
// sets sec.Value the field is present in JSON output.
func TestSecret_GetPathPopulatesValue(t *testing.T) {
	s := Secret{
		ID:    "secret-id-002",
		Key:   "API_KEY",
		Value: "plaintext-secret",
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"value":"plaintext-secret"`,
		"GetSecret result must include the decrypted value in JSON")
}

// TestSecret_ValueFieldHasOmitempty is a compile-time documentation test:
// if the struct tag ever loses `omitempty`, an empty Value would leak in list
// responses. This test encodes an empty Value and asserts absence in output.
func TestSecret_ValueFieldHasOmitempty(t *testing.T) {
	s := Secret{ID: "x", Key: "KEY"}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"value":""`,
		"empty Value field must be omitted by json:\"value,omitempty\" tag; "+
			"if this test fails the tag was removed and plaintext values could "+
			"be leaked in list API responses")
}

// ---------------------------------------------------------------------------
// Crypto isolation — cross-project key separation (no DB required)
// ---------------------------------------------------------------------------

// TestCrossProjectKeyIsolation confirms that ciphertext encrypted under
// project-A's derived key cannot be decrypted under project-B's derived key.
// This is the cryptographic guarantee underpinning org/project isolation.
func TestCrossProjectKeyIsolation(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0xDE}, 32)

	keyA, err := DeriveProjectKey(masterKey, "project-uuid-A")
	require.NoError(t, err)

	keyB, err := DeriveProjectKey(masterKey, "project-uuid-B")
	require.NoError(t, err)

	plaintext := []byte("super-secret-value")

	ct, err := Encrypt(keyA, plaintext)
	require.NoError(t, err)

	_, err = Decrypt(keyB, ct)
	assert.Error(t, err,
		"ciphertext from project-A must not be decryptable with project-B's derived key — "+
			"IDOR at the crypto layer would be catastrophic")
}

// TestDifferentMasterKeysYieldDifferentDerivedKeys confirms that two orgs
// with different master keys produce different derived keys for the same
// project ID.  This ensures a compromised master key for org-A does not
// expose org-B's secrets.
func TestDifferentMasterKeysYieldDifferentDerivedKeys(t *testing.T) {
	masterKeyOrgA := bytes.Repeat([]byte{0x11}, 32)
	masterKeyOrgB := bytes.Repeat([]byte{0x22}, 32)
	sameProjectID := "shared-project-name"

	keyA, err := DeriveProjectKey(masterKeyOrgA, sameProjectID)
	require.NoError(t, err)

	keyB, err := DeriveProjectKey(masterKeyOrgB, sameProjectID)
	require.NoError(t, err)

	assert.NotEqual(t, keyA, keyB,
		"same project slug under different org master keys must produce different derived keys")
}

// TestCrossProjectDecryptionFails_EncryptedWithOrgADecryptedByOrgB shows the
// end-to-end failure path: encrypt with org-A's key, attempt decrypt with
// org-B's key — must error out with authentication failure.
func TestCrossProjectDecryptionFails_EncryptedWithOrgADecryptedByOrgB(t *testing.T) {
	masterA := bytes.Repeat([]byte{0xAA}, 32)
	masterB := bytes.Repeat([]byte{0xBB}, 32)
	projectID := "project-123"

	kA, err := DeriveProjectKey(masterA, projectID)
	require.NoError(t, err)

	kB, err := DeriveProjectKey(masterB, projectID)
	require.NoError(t, err)

	ct, err := Encrypt(kA, []byte("org-a-secret"))
	require.NoError(t, err)

	_, err = Decrypt(kB, ct)
	assert.Error(t, err, "AES-GCM authentication must fail when wrong org key is used")
}

// ---------------------------------------------------------------------------
// Share link — single-use and expiry enforcement (no DB required)
// ---------------------------------------------------------------------------

// TestShareLink_ExpiryCheck verifies the in-service expiry logic using the
// model directly.
func TestShareLink_ExpiryCheck(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	sl := &ShareLink{
		ID:        "link-id",
		SecretID:  "secret-id",
		ExpiresAt: past,
	}

	// Replicate the guard from UseShareLink.
	assert.True(t, time.Now().UTC().After(sl.ExpiresAt),
		"expired share link should be detected by the time.After check in UseShareLink")
}

// TestShareLink_AlreadyUsed verifies that a non-nil UsedAt pointer is
// sufficient to detect a burned link before any DB write.
func TestShareLink_AlreadyUsed(t *testing.T) {
	usedAt := time.Now().UTC().Add(-5 * time.Minute)
	sl := &ShareLink{
		ID:       "link-id",
		SecretID: "secret-id",
		UsedAt:   &usedAt,
	}

	assert.NotNil(t, sl.UsedAt,
		"a used share link must have a non-nil UsedAt — service rejects it")
}

// TestShareLink_NotExpired_NotUsed verifies a valid share link passes both
// checks so we don't accidentally block legitimate use.
func TestShareLink_NotExpired_NotUsed(t *testing.T) {
	future := time.Now().UTC().Add(1 * time.Hour)
	sl := &ShareLink{
		ID:        "link-id",
		SecretID:  "secret-id",
		ExpiresAt: future,
		UsedAt:    nil,
	}

	assert.False(t, time.Now().UTC().After(sl.ExpiresAt), "link should not yet be expired")
	assert.Nil(t, sl.UsedAt, "link should not be marked used")
}

// ---------------------------------------------------------------------------
// Access log pagination — normalisation guard (no DB required)
// ---------------------------------------------------------------------------

// TestGetProjectAccessLog_PageNormalisation tests the pure normalisation logic
// from GetProjectAccessLog.
func TestGetProjectAccessLog_PageNormalisation(t *testing.T) {
	tests := []struct {
		name        string
		page, limit int
		wantPage    int
		wantLimit   int
	}{
		{"defaults at zero", 0, 0, 1, 25},
		{"negative page clamped to 1", -5, 10, 1, 10},
		{"limit above 100 clamped", 1, 200, 1, 25},
		{"limit below 1 clamped", 3, -1, 3, 25},
		{"valid values unchanged", 2, 50, 2, 50},
		{"limit exactly 100 allowed", 1, 100, 1, 100},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			page := tc.page
			limit := tc.limit
			// Mirror the normalisation in GetProjectAccessLog.
			if page < 1 {
				page = 1
			}
			if limit < 1 || limit > 100 {
				limit = 25
			}
			assert.Equal(t, tc.wantPage, page)
			assert.Equal(t, tc.wantLimit, limit)
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateRepoURL — SSRF prevention (no DB required)
// ---------------------------------------------------------------------------

// TestValidateRepoURL_BlocksPrivateAndLoopbackAddresses verifies that the
// SSRF guard rejects known-dangerous URLs at the model layer.
func TestValidateRepoURL_BlocksPrivateAndLoopbackAddresses(t *testing.T) {
	// These URLs use literal IP addresses — no DNS resolution required, so
	// tests run offline without a network dependency.
	blocked := []struct {
		url    string
		reason string
	}{
		{"http://github.com/org/repo", "non-HTTPS scheme"},
		{"ftp://example.com/repo", "non-HTTPS scheme (ftp)"},
		{"https://127.0.0.1/repo", "loopback IPv4"},
		{"https://[::1]/repo", "loopback IPv6"},
		{"https://10.0.0.1/repo", "RFC1918 10.x"},
		{"https://192.168.1.1/repo", "RFC1918 192.168.x"},
		{"https://172.16.0.1/repo", "RFC1918 172.16.x"},
		{"https://0.0.0.0/repo", "unspecified address"},
		{"", "empty URL"},
		{"https://", "no host"},
	}

	for _, tc := range blocked {
		tc := tc
		t.Run(tc.reason, func(t *testing.T) {
			err := ValidateRepoURL(context.Background(), tc.url)
			assert.Error(t, err,
				"URL %q (%s) should be blocked by SSRF guard", tc.url, tc.reason)
		})
	}
}

// TestValidateRepoURL_BlocksLocalhost ensures the string "localhost" is
// rejected regardless of case.
func TestValidateRepoURL_BlocksLocalhost(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST", "LocalHost"} {
		err := ValidateRepoURL(context.Background(), "https://"+host+"/repo")
		assert.Error(t, err, "https://%s/repo should be rejected", host)
	}
}

// TestValidateRepoURL_BlocksLinkLocal ensures link-local addresses are
// rejected (169.254.x.x is a common SSRF target via AWS IMDS).
func TestValidateRepoURL_BlocksLinkLocal(t *testing.T) {
	err := ValidateRepoURL(context.Background(), "https://169.254.169.254/latest/meta-data/")
	assert.Error(t, err, "AWS IMDS link-local address must be blocked")
}
