// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIPNATLockout verifies that the per-(IP, email) lockout isolates failures
// to the specific (IP, account) pair rather than the whole IP. This is the
// NAT-safety property: User A's bad passwords must not lock out User B behind
// the same corporate NAT/VPN.
func TestIPNATLockout(t *testing.T) {
	const sharedIP = "10.0.0.1"
	const victimEmail = "victim@example.com"
	const innocentEmail = "innocent@example.com"

	// Keys for the same IP but different emails must differ so Redis counters
	// are independent — the core NAT isolation invariant.
	victimKey := loginIPEmailFailKey(sharedIP, victimEmail)
	innocentKey := loginIPEmailFailKey(sharedIP, innocentEmail)

	assert.NotEqual(t, victimKey, innocentKey,
		"(IP,email) keys must differ so failures on one account don't affect others")

	// The pure-IP secondary key must differ from both (IP,email) keys.
	ipKey := loginIPFailKey(sharedIP)
	assert.NotEqual(t, ipKey, victimKey)
	assert.NotEqual(t, ipKey, innocentKey)
}

// TestIPNATLockout_FailClosed verifies that checkIPEmailLocked fails closed
// (returns locked=true, ErrLockoutCheckUnavailable) when Redis is unreachable,
// matching the same guarantee as checkIPLocked and checkAccountLocked.
func TestIPNATLockout_FailClosed(t *testing.T) {
	svc := &Service{redis: dialFailingRedis(t)}

	locked, err := svc.checkIPEmailLocked(context.Background(), "10.0.0.1", "victim@example.com")
	assert.True(t, locked, "fail-closed: unreachable Redis must report the pair as locked")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLockoutCheckUnavailable))
}

// TestIPNATLockout_FailOpen verifies the opt-in fail-open path for the
// new IP+email lockout, matching the behaviour of the existing lockouts.
func TestIPNATLockout_FailOpen(t *testing.T) {
	svc := (&Service{redis: dialFailingRedis(t)}).WithFailOpenOnRedisOutage(true)

	locked, err := svc.checkIPEmailLocked(context.Background(), "10.0.0.1", "victim@example.com")
	assert.False(t, locked, "fail-open: unreachable Redis must let the request through")
	assert.NoError(t, err)
}

// TestWithIPLockoutMax verifies that the secondary IP threshold is configurable.
func TestWithIPLockoutMax(t *testing.T) {
	svc := &Service{ipLockoutMax: ipLockoutSecondaryFailMax}
	assert.Equal(t, ipLockoutSecondaryFailMax, svc.ipLockoutMax,
		"default threshold must be ipLockoutSecondaryFailMax")

	svc = svc.WithIPLockoutMax(100)
	assert.Equal(t, 100, svc.ipLockoutMax)
}
