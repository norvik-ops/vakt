// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoAccountDoS_AttackerCannotLockOutVictim is the S121-F4 (F1-Auth)
// regression, and it is a behavioural test against a real Redis rather than a
// key-format assertion — the defect it guards was precisely that the counters
// looked fine while the lockout decision ignored the source IP.
//
// The removed pure per-email lockout meant ANY caller could take ANY account
// offline for 15 minutes by sending five wrong passwords for that address. Here
// an attacker hammers the victim's address from their own IP; the attacker must
// end up locked out, while the victim — logging in from a different IP — must
// NOT be. If someone reintroduces an IP-agnostic lockout, the second assertion
// fails.
func TestNoAccountDoS_AttackerCannotLockOutVictim(t *testing.T) {
	url := os.Getenv("VAKT_REDIS_URL")
	if url == "" {
		t.Skip("VAKT_REDIS_URL not set — this lockout test needs a real Redis (set in CI)")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	require.NoError(t, rdb.Ping(ctx).Err(), "Redis must be reachable")

	const (
		attackerIP  = "203.0.113.9"
		victimIP    = "198.51.100.7"
		victimEmail = "s121-dos-victim@example.org"
	)

	// Clean slate, and don't leak counters into other tests.
	keys := []string{
		loginIPEmailFailKey(attackerIP, victimEmail),
		loginIPEmailFailKey(victimIP, victimEmail),
		loginIPFailKey(attackerIP),
		loginIPFailKey(victimIP),
	}
	_ = rdb.Del(ctx, keys...).Err()
	t.Cleanup(func() { _ = rdb.Del(context.Background(), keys...).Err() })

	svc := &Service{redis: rdb}

	// The attacker sends far more bad passwords than any threshold.
	for i := 0; i < ipEmailLockoutFailMax+5; i++ {
		svc.recordIPEmailLoginFailure(ctx, attackerIP, victimEmail)
	}

	// The attacker locked *themselves* out of that account — brute-force is stopped.
	attackerLocked, err := svc.checkIPEmailLocked(ctx, attackerIP, victimEmail)
	require.NoError(t, err)
	assert.True(t, attackerLocked,
		"the attacking IP must be locked out of the targeted account after %d failures",
		ipEmailLockoutFailMax)

	// The victim, from their own IP, must still be able to sign in. This is the
	// property the removed per-email lockout violated.
	victimLocked, err := svc.checkIPEmailLocked(ctx, victimIP, victimEmail)
	require.NoError(t, err)
	assert.False(t, victimLocked,
		"ACCOUNT DoS REGRESSION: an attacker's failed logins locked the victim out "+
			"of their own account from a different IP — the lockout must be keyed on (IP, email)")
}

// TestClearLoginFailures_ResetsPairCounter verifies that a successful login wipes
// the user's own (IP, email) counter. S121-F4: this previously cleared only the
// pure per-email counter, so the pair counter survived a successful login and a
// user who mistyped a few times before getting in could be locked out by their
// next single typo.
func TestClearLoginFailures_ResetsPairCounter(t *testing.T) {
	url := os.Getenv("VAKT_REDIS_URL")
	if url == "" {
		t.Skip("VAKT_REDIS_URL not set — this lockout test needs a real Redis (set in CI)")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	require.NoError(t, rdb.Ping(ctx).Err())

	const (
		userIP    = "198.51.100.22"
		userEmail = "s121-typo-user@example.org"
	)
	key := loginIPEmailFailKey(userIP, userEmail)
	_ = rdb.Del(ctx, key).Err()
	t.Cleanup(func() { _ = rdb.Del(context.Background(), key).Err() })

	svc := &Service{redis: rdb}

	// A few honest typos, still under the threshold.
	for i := 0; i < 3; i++ {
		svc.recordIPEmailLoginFailure(ctx, userIP, userEmail)
	}
	n, err := rdb.Get(ctx, key).Int64()
	require.NoError(t, err)
	require.EqualValues(t, 3, n)

	// Then they get the password right.
	svc.clearLoginFailures(ctx, userIP, userEmail)

	_, err = rdb.Get(ctx, key).Result()
	assert.ErrorIs(t, err, redis.Nil,
		"a successful login must clear the user's own (IP, email) failure counter")
}
