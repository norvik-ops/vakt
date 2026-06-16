// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S87-6 (F-06, CWE-636): checkPwVersion falls back to PostgreSQL when Redis is
// unavailable instead of failing open. These tests cover the DB-free branches;
// the full PG-fallback rejection path is exercised in the integration suite
// (internal/integration_test, build tag `integration`).
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// dialFailingRedisClient points at an unbindable port so every call errors fast.
func dialFailingRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
}

func TestPwVersionFromDB_NilPool(t *testing.T) {
	v, ok := pwVersionFromDB(context.Background(), nil, "00000000-0000-0000-0000-000000000001")
	assert.False(t, ok, "nil pool must report no value available")
	assert.Equal(t, int64(0), v)
}

// TestCheckPwVersion_RedisDownNoDB_PassesThrough verifies the no-lockout
// guarantee: when Redis is unreachable AND no PG pool is wired (e.g. unit tests),
// a legitimate token is allowed through rather than rejected. This is the
// "kein Lockout legitimer User bei transientem Redis-Ausfall" acceptance
// criterion for the degenerate test path.
func TestCheckPwVersion_RedisDownNoDB_PassesThrough(t *testing.T) {
	claims := &Claims{UserID: "00000000-0000-0000-0000-000000000001", PwVersion: 0}
	err := checkPwVersion(context.Background(), dialFailingRedisClient(t), nil, claims)
	assert.NoError(t, err, "Redis down + no PG fallback must pass through (no lockout)")
}

// TestCheckPwVersion_RedisDownNoDB_StaleAlsoPasses documents that without a PG
// fallback we cannot detect staleness during an outage — the integration test
// proves the PG path rejects it. Here, even a non-zero token version passes
// because there is no source of truth to compare against.
func TestCheckPwVersion_RedisDownNoDB_StaleAlsoPasses(t *testing.T) {
	claims := &Claims{UserID: "00000000-0000-0000-0000-000000000001", PwVersion: 5}
	err := checkPwVersion(context.Background(), dialFailingRedisClient(t), nil, claims)
	assert.NoError(t, err)
}
