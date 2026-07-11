// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialFailingRedis points a real go-redis client at a port that is
// guaranteed to be unreachable inside test sandboxes. Every call returns
// an i/o error within the configured timeout — exactly the situation the
// fail-closed path is designed for.
func dialFailingRedis(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // unbindable port → dial error
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
}

// TestCheckAccountLocked_FailClosedByDefault is the audit P1-6 regression.
// Without VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE the service must reject
// reads when Redis cannot answer: returns (true, ErrLockoutCheckUnavailable)
// so the caller knows to surface a 503 — never a 200.
//
// S121-F4: exercised against the (IP, email) lockout, which is now the primary
// account lockout (the pure per-email counter was removed as an account-DoS
// vector). The fail-closed guarantee itself is unchanged.
func TestCheckAccountLocked_FailClosedByDefault(t *testing.T) {
	svc := &Service{redis: dialFailingRedis(t)}

	locked, err := svc.checkIPEmailLocked(context.Background(), "203.0.113.5", "victim@example.org")
	assert.True(t, locked, "fail-closed must report the account as locked")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLockoutCheckUnavailable))
}

// TestCheckIPLocked_FailClosedByDefault same property, IP path.
func TestCheckIPLocked_FailClosedByDefault(t *testing.T) {
	svc := &Service{redis: dialFailingRedis(t)}

	locked, err := svc.checkIPLocked(context.Background(), "203.0.113.5")
	assert.True(t, locked)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLockoutCheckUnavailable))
}

// TestCheckAccountLocked_FailOpenIfConfigured covers the explicit opt-in
// path: an operator that prefers availability over brute-force protection
// during a Redis outage flips the flag and the historical behaviour is
// restored.
func TestCheckAccountLocked_FailOpenIfConfigured(t *testing.T) {
	svc := (&Service{redis: dialFailingRedis(t)}).WithFailOpenOnRedisOutage(true)

	locked, err := svc.checkIPEmailLocked(context.Background(), "203.0.113.5", "victim@example.org")
	assert.False(t, locked, "fail-open must let the request through")
	assert.NoError(t, err)
}

// TestCheckIPLocked_FailOpenIfConfigured — same, IP path.
func TestCheckIPLocked_FailOpenIfConfigured(t *testing.T) {
	svc := (&Service{redis: dialFailingRedis(t)}).WithFailOpenOnRedisOutage(true)

	locked, err := svc.checkIPLocked(context.Background(), "203.0.113.5")
	assert.False(t, locked)
	assert.NoError(t, err)
}
