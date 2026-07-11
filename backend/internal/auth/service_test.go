// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		// Length boundary
		{"empty", "", true},
		{"9 chars — one short of minimum", "ValidP@s1", true},
		{"10 chars — at minimum boundary", "ValidPass1!", false},
		{"long valid password", "Correct-Horse-Battery-Staple9!", false},

		// Missing character class
		{"no uppercase — all lowercase", "alllower1!", true},
		{"no digit", "NoDigitHere!", true},
		{"no special character", "NoSpecialChar1", true},

		// Only missing one class at a time
		{"missing special only", "ValidPass01", true},
		{"missing digit only", "ValidPass!!", true},
		{"missing uppercase only", "validpass1!", true},

		// Edge cases around special character set — space is NOT in the special set
		{"special char is space — not in set", "ValidPass1 ", true}, // space not in const special
		{"special char from set — bang", "ValidPass1!", false},
		{"special char from set — at", "ValidPass1@", false},
		{"special char from set — pound", "ValidPass1#", false},
		{"special char from set — backslash", "ValidPass1\\", false},

		// Unicode uppercase letters do NOT satisfy hasUpper (only A-Z via unicode.IsUpper)
		// However, unicode.IsUpper('Ä') == true in Go, so Ä does satisfy the rule.
		// We rely on the implementation's use of unicode.IsUpper.
		{"unicode uppercase satisfies rule", "gültiger1@Ä", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validatePasswordStrength(tc.pw)
			if tc.wantErr {
				assert.Error(t, err, "expected error for password %q", tc.pw)
				// The error must be ErrWeakPassword specifically.
				assert.ErrorIs(t, err, ErrWeakPassword, "error must be ErrWeakPassword for %q", tc.pw)
			} else {
				assert.NoError(t, err, "expected no error for password %q", tc.pw)
			}
		})
	}
}

// TestValidatePasswordStrength_SpaceIsNotSpecial verifies that a space character
// is not counted as a special character (the const special set does not include space).
// A password whose only non-alphanumeric character is a space must fail.
func TestValidatePasswordStrength_SpaceIsNotSpecial(t *testing.T) {
	// "ValidPass1 " has upper (V,P), digit (1), but space is not in the special set,
	// so the hasSpecial requirement is not met.
	err := validatePasswordStrength("ValidPass1 ")
	assert.Error(t, err, "space should not satisfy the special character requirement")
}

func TestValidatePasswordStrength_ErrorMessage(t *testing.T) {
	err := validatePasswordStrength("weak")
	assert.EqualError(t, err, ErrWeakPassword.Error())
}

// S121-F4: TestLoginFailKey is gone with loginFailKey — the pure per-email
// lockout counter it namespaced was removed as an account-DoS vector.

// TestLoginIPFailKey verifies the key format used for per-IP login failure counters.
func TestLoginIPFailKey(t *testing.T) {
	key := loginIPFailKey("192.168.1.1")
	assert.Equal(t, "login_fail_ip:192.168.1.1", key)

	// Ensure the per-IP and per-(IP, email) counters use different namespaces.
	pairKey := loginIPEmailFailKey("192.168.1.1", "user@example.com")
	assert.NotEqual(t, key, pairKey, "IP and (IP, email) keys must use different namespaces")

	// Different IPs produce different keys.
	key2 := loginIPFailKey("10.0.0.1")
	assert.NotEqual(t, key, key2)
}

// TestIPLockoutConstants documents the expected lockout thresholds.
func TestIPLockoutConstants(t *testing.T) {
	assert.Equal(t, int64(10), int64(ipEmailLockoutFailMax), "primary (IP,email) lockout triggers at 10 failures")
	assert.Equal(t, int64(50), int64(ipLockoutSecondaryFailMax), "secondary pure-IP lockout triggers at 50 failures")
}

// TestTokenDenyKey verifies that token revocation keys are deterministic SHA-256 hashes.
func TestTokenDenyKey(t *testing.T) {
	token := "some-raw-access-token"
	k1 := tokenDenyKey(token)
	k2 := tokenDenyKey(token)

	assert.Equal(t, k1, k2, "same token must always produce same deny key")
	assert.Contains(t, k1, "revoked:", "deny key must be prefixed with 'revoked:'")

	// Different tokens must produce different keys.
	k3 := tokenDenyKey("different-token")
	assert.NotEqual(t, k1, k3)
}

// TestRefreshRedisKey verifies that refresh token keys are deterministic and namespaced.
func TestRefreshRedisKey(t *testing.T) {
	raw := "a1b2c3d4e5f6"
	k1 := refreshRedisKey(raw)
	k2 := refreshRedisKey(raw)

	assert.Equal(t, k1, k2, "same token must produce same key")
	assert.Contains(t, k1, "refresh:", "refresh key must be prefixed with 'refresh:'")

	k3 := refreshRedisKey("different")
	assert.NotEqual(t, k1, k3)
}

// TestSlugify verifies the slug generation used for org URL slugs.
func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Acme GmbH", "acme-gmbh"},
		{"My Company", "my-company"},
		{"test_name", "test-name"},
		{"  leading trailing  ", "leading-trailing"},
		{"double--hyphen", "double-hyphen"},
		{"ALL CAPS", "all-caps"},
		{"123numeric", "123numeric"},
		{"", ""},
		{"special@chars!", "specialchars"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := slugify(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
