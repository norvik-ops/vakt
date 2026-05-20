// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TestMain überschreibt den bcrypt-Cost-Faktor für alle Tests in diesem
// Package — cost=12 in Production, cost=MinCost (4) in Tests. 8 Codes ×
// bcrypt cost 12 dauern ~2.4s pro Aufruf; mit -race und vielen Tests
// schlägt das gegen 2-Min-Test-Timeout. Format/Unique-Assertions hängen
// nicht vom Cost ab.
func TestMain(m *testing.M) {
	recoveryCodeBcryptCost = bcrypt.MinCost
	m.Run()
}

func TestGenerateRecoveryCodes_Count(t *testing.T) {
	plain, hashed, err := generateRecoveryCodes()
	require.NoError(t, err)
	assert.Len(t, plain, 8, "must generate exactly 8 plain recovery codes")
	assert.Len(t, hashed, 8, "must generate exactly 8 hashed recovery codes")
}

func TestGenerateRecoveryCodes_Format(t *testing.T) {
	plain, hashed, err := generateRecoveryCodes()
	require.NoError(t, err)

	// Format must be XXXX-XXXX-XXXX where X is an uppercase hex digit (12 hex chars total).
	pattern := regexp.MustCompile(`^[A-F0-9]{4}-[A-F0-9]{4}-[A-F0-9]{4}$`)
	for i, code := range plain {
		assert.Regexp(t, pattern, code, "recovery code %d must match XXXX-XXXX-XXXX format, got: %s", i, code)
		assert.NotEmpty(t, hashed[i], "bcrypt hash for recovery code %d must not be empty", i)
	}
}

func TestGenerateRecoveryCodes_Unique(t *testing.T) {
	plain, _, err := generateRecoveryCodes()
	require.NoError(t, err)

	seen := make(map[string]bool, len(plain))
	for _, code := range plain {
		assert.False(t, seen[code], "duplicate recovery code detected: %s", code)
		seen[code] = true
	}
}

func TestGenerateRecoveryCodes_DifferentBatches(t *testing.T) {
	plain1, _, err := generateRecoveryCodes()
	require.NoError(t, err)
	plain2, _, err := generateRecoveryCodes()
	require.NoError(t, err)

	// Two independent calls should not produce all-identical codes.
	matches := 0
	for _, c1 := range plain1 {
		for _, c2 := range plain2 {
			if c1 == c2 {
				matches++
			}
		}
	}
	assert.Less(t, matches, 8, "two recovery code batches must not be identical")
}

func TestGenerateRecoveryCodes_HashesAreValidBcrypt(t *testing.T) {
	plain, hashed, err := generateRecoveryCodes()
	require.NoError(t, err)

	// Each hash must start with "$2a$" or "$2b$" (bcrypt prefix).
	for i, h := range hashed {
		assert.True(t,
			len(h) > 7 && (h[:4] == "$2a$" || h[:4] == "$2b$"),
			"hash %d (%s) does not look like a bcrypt hash (from code %s)", i, h, plain[i],
		)
	}
}

// TestVerifyRecoveryCode_RequiresDB documents why VerifyRecoveryCode is not unit-tested here.
//
// VerifyRecoveryCode is a method on *TotpHandler which uses a *pgxpool.Pool
// to query and update auth_recovery_codes rows. There is no in-memory DB shim
// available in the unit-test suite, so a proper test requires a running
// PostgreSQL instance with the schema applied.
//
// Integration test setup needed:
//  1. Start a PostgreSQL container (e.g. via testcontainers-go or a make target).
//  2. Run db/migrations against it.
//  3. Construct a TotpHandler with a live pool.
//  4. Insert hashed codes via StoreRecoveryCodes.
//  5. Call VerifyRecoveryCode with a correct and incorrect candidate.
//  6. Assert the correct one returns nil and marks used_at, the wrong one returns *recoveryCodeError.
func TestVerifyRecoveryCode_RequiresDB(t *testing.T) {
	t.Skip("VerifyRecoveryCode requires a live PostgreSQL DB — run as integration test")
}

// TestRecoveryCodeError verifies the error type returned for an invalid recovery code.
func TestRecoveryCodeError(t *testing.T) {
	err := &recoveryCodeError{}
	assert.Equal(t, "AUTH_INVALID_RECOVERY_CODE", err.Error())
	assert.Equal(t, "AUTH_INVALID_RECOVERY_CODE", err.Code())
}
