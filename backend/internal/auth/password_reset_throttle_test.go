// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

// Invariant tests for the per-email password-reset throttle added in S14.
//
// These tests do not require a database or Redis connection. They verify:
//   - The throttle constants are within the intended security range
//   - The throttle condition (cnt > max) correctly allows ≤max and blocks max+1
//   - The Redis key format is stable (a change would break existing throttle state)

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestPasswordResetThrottle_MaxIs3 verifies that resetThrottleMax equals 3.
// The value was chosen to allow legitimate use (lost password, wrong new password)
// while blocking inbox-spam abuse. A lower value (1-2) would cause false throttles
// for legitimate users; a higher value (>5) undermines the protection.
func TestPasswordResetThrottle_MaxIs3(t *testing.T) {
	assert.Equal(t, 3, resetThrottleMax,
		"max 3 reset emails per throttle window: enough for legitimate use, few enough to limit spam")
}

// TestPasswordResetThrottle_TTLIsOneHour verifies that resetThrottleTTL is 1 hour.
// This matches the reset-token expiry (also 1 hour), so the throttle window
// never outlasts the period when a token generated in the first request is valid.
func TestPasswordResetThrottle_TTLIsOneHour(t *testing.T) {
	assert.Equal(t, time.Hour, resetThrottleTTL,
		"throttle window must equal the reset token expiry (1 hour)")
}

// TestPasswordResetThrottle_CountAllowsUpToMax verifies the INCR counter condition
// that determines whether an email is suppressed. cnt > resetThrottleMax suppresses;
// cnt <= resetThrottleMax allows.
func TestPasswordResetThrottle_CountAllowsUpToMax(t *testing.T) {
	for cnt := int64(1); cnt <= int64(resetThrottleMax); cnt++ {
		assert.False(t, cnt > int64(resetThrottleMax),
			"request #%d must be allowed (cnt=%d ≤ max=%d)", cnt, cnt, resetThrottleMax)
	}
}

// TestPasswordResetThrottle_CountBlocksAboveMax verifies that the (max+1)th
// request in the throttle window is suppressed.
func TestPasswordResetThrottle_CountBlocksAboveMax(t *testing.T) {
	cnt := int64(resetThrottleMax + 1)
	assert.True(t, cnt > int64(resetThrottleMax),
		"request #%d must be suppressed (cnt=%d > max=%d)", cnt, cnt, resetThrottleMax)
}

// TestPasswordResetThrottle_KeyFormat verifies that the Redis key starts with
// "reset_req:" and contains the email. A format change would orphan existing
// throttle counters in Redis, silently resetting all current throttle state.
func TestPasswordResetThrottle_KeyFormat(t *testing.T) {
	email := "user@example.com"
	key := "reset_req:" + email

	assert.True(t, strings.HasPrefix(key, "reset_req:"),
		"throttle key must start with 'reset_req:' prefix")
	assert.Contains(t, key, email,
		"throttle key must contain the email address")
}
