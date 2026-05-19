// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBackupCodes_Count(t *testing.T) {
	plain, hashed, err := GenerateBackupCodes()
	require.NoError(t, err)
	assert.Len(t, plain, 8, "must generate exactly 8 plain codes")
	assert.Len(t, hashed, 8, "must generate exactly 8 hashed codes")
}

func TestGenerateBackupCodes_Format(t *testing.T) {
	plain, hashed, err := GenerateBackupCodes()
	require.NoError(t, err)

	// Format must be XXXX-XXXX where X is an uppercase hex digit.
	pattern := regexp.MustCompile(`^[A-F0-9]{4}-[A-F0-9]{4}$`)
	for i, code := range plain {
		assert.Regexp(t, pattern, code, "plain code %d must match XXXX-XXXX hex format", i)
		assert.NotEmpty(t, hashed[i], "hash for code %d must not be empty", i)
	}
}

func TestGenerateBackupCodes_Unique(t *testing.T) {
	plain, _, err := GenerateBackupCodes()
	require.NoError(t, err)

	seen := make(map[string]bool, len(plain))
	for _, c := range plain {
		assert.False(t, seen[c], "duplicate backup code detected: %s", c)
		seen[c] = true
	}
}

func TestGenerateBackupCodes_DifferentBatches(t *testing.T) {
	plain1, _, err := GenerateBackupCodes()
	require.NoError(t, err)
	plain2, _, err := GenerateBackupCodes()
	require.NoError(t, err)

	// Two calls are statistically extremely unlikely to produce identical codes.
	// We check all 8 codes from the first batch don't all appear in the second.
	matches := 0
	for _, c := range plain1 {
		for _, c2 := range plain2 {
			if c == c2 {
				matches++
			}
		}
	}
	assert.Less(t, matches, 8, "two backup code batches should not be identical")
}

func TestCheckBackupCode_EachCodeMatchesItsHash(t *testing.T) {
	plain, hashed, err := GenerateBackupCodes()
	require.NoError(t, err)

	for i, code := range plain {
		idx := CheckBackupCode(code, hashed)
		assert.Equal(t, i, idx, "plain code %d (%s) should match at index %d", i, code, i)
	}
}

func TestCheckBackupCode_WrongCode(t *testing.T) {
	_, hashed, err := GenerateBackupCodes()
	require.NoError(t, err)

	assert.Equal(t, -1, CheckBackupCode("0000-0000", hashed), "non-existent code must return -1")
	assert.Equal(t, -1, CheckBackupCode("", hashed), "empty candidate must return -1")
	assert.Equal(t, -1, CheckBackupCode("AAAA-BBBB", hashed), "random non-matching code must return -1")
}

func TestCheckBackupCode_EmptyHashes(t *testing.T) {
	plain, _, err := GenerateBackupCodes()
	require.NoError(t, err)

	assert.Equal(t, -1, CheckBackupCode(plain[0], []string{}), "empty hashes must return -1")
	assert.Equal(t, -1, CheckBackupCode(plain[0], nil), "nil hashes must return -1")
}

func TestCheckBackupCode_CorrectIndexMatching(t *testing.T) {
	// Verify that CheckBackupCode returns the correct index so the caller can
	// remove the used code. Index must match the position in the hashed slice.
	plain, hashed, err := GenerateBackupCodes()
	require.NoError(t, err)

	// Check last code — index must be 7.
	idx := CheckBackupCode(plain[7], hashed)
	assert.Equal(t, 7, idx)

	// Check first code — index must be 0.
	idx = CheckBackupCode(plain[0], hashed)
	assert.Equal(t, 0, idx)
}

// TestValidateTOTP_Format verifies TOTP validation rejects obviously invalid codes
// without requiring a real time-synchronised secret.
func TestValidateTOTP_Format(t *testing.T) {
	// This is a valid base32 TOTP secret (JBSWY3DPEHPK3PXP = "Hello!" in base32).
	secret := "JBSWY3DPEHPK3PXP"

	t.Run("empty code returns false", func(t *testing.T) {
		assert.False(t, ValidateTOTP(secret, ""))
	})
	t.Run("non-numeric code returns false", func(t *testing.T) {
		assert.False(t, ValidateTOTP(secret, "abcdef"))
	})
	t.Run("too short — 5 digits returns false", func(t *testing.T) {
		assert.False(t, ValidateTOTP(secret, "12345"))
	})
	t.Run("too long — 7 digits returns false", func(t *testing.T) {
		assert.False(t, ValidateTOTP(secret, "1234567"))
	})
	t.Run("clearly wrong 6-digit code returns false", func(t *testing.T) {
		// "000000" is astronomically unlikely to be a valid TOTP at this moment.
		assert.False(t, ValidateTOTP(secret, "000000"))
	})
}

// TestGenerateTOTPSecret verifies that GenerateTOTPSecret returns a non-empty secret
// and a valid otpauth URI.
func TestGenerateTOTPSecret(t *testing.T) {
	secret, uri, err := GenerateTOTPSecret("user@example.com", "TestIssuer")
	require.NoError(t, err)

	assert.NotEmpty(t, secret, "secret must not be empty")
	assert.NotEmpty(t, uri, "URI must not be empty")
	assert.Contains(t, uri, "otpauth://totp/", "URI must be an otpauth URI")
	// The go-otp library formats the path as "Issuer:account@domain" (not URL-encoded in the path).
	assert.Contains(t, uri, "user@example.com", "URI must contain account name")
	assert.Contains(t, uri, "TestIssuer", "URI must contain issuer")
}
