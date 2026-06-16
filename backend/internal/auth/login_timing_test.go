// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S87-3 (F-05, CWE-208): Login must perform constant bcrypt work regardless of
// whether the e-mail exists, so response latency cannot be used to enumerate
// users. These white-box tests cover the dummy-hash mechanism that backs the
// unknown-e-mail branch.
package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestNewDummyBcryptHash_IsCost12(t *testing.T) {
	h := newDummyBcryptHash()
	require.NotEmpty(t, h)
	cost, err := bcrypt.Cost(h)
	require.NoError(t, err)
	assert.Equal(t, 12, cost, "dummy hash must use the same cost as real hashes so timing matches")
}

func TestNewService_PopulatesDummyHash(t *testing.T) {
	key, err := GenerateSymmetricKey("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	svc := NewService(nil, nil, key)
	require.NotEmpty(t, svc.dummyBcryptHash, "NewService must precompute the dummy hash")
	cost, err := bcrypt.Cost(svc.dummyBcryptHash)
	require.NoError(t, err)
	assert.Equal(t, 12, cost)
}

// TestDummyHash_NeverMatches guarantees the unknown-e-mail compare always fails
// (so it can never be mistaken for a successful login) yet still runs bcrypt.
func TestDummyHash_NeverMatches(t *testing.T) {
	h := newDummyBcryptHash()
	for _, pw := range []string{"", "password", "Sup3r$ecret!!", "any-random-guess"} {
		assert.Error(t, bcrypt.CompareHashAndPassword(h, []byte(pw)),
			"dummy hash must not match arbitrary passwords")
	}
}

// TestDummyHashCompare_DoesRealWork asserts the unknown-user compare path is not
// a no-op: a cost-12 bcrypt compare takes meaningful time, which is exactly the
// work that masks the existence of the account.
func TestDummyHashCompare_DoesRealWork(t *testing.T) {
	if testing.Short() {
		t.Skip("timing assertion skipped in -short mode")
	}
	h := newDummyBcryptHash()
	start := time.Now()
	_ = bcrypt.CompareHashAndPassword(h, []byte("any-password"))
	elapsed := time.Since(start)
	assert.Greater(t, elapsed, 2*time.Millisecond,
		"cost-12 bcrypt compare should take non-trivial time (constant-work timing defense)")
}
