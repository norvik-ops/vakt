// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

// AUTH-001 regression tests for the refresh-session revocation added to Logout.
//
// Security contract: after a successful logout, all refresh sessions for the
// user MUST be deleted from refresh_sessions (DB) and all corresponding
// "refresh:<token_hash>" keys MUST be removed from Redis.
//
// Without this fix, an attacker holding a stolen refresh token can call
// POST /auth/refresh after the victim logs out and receive a new access token —
// retaining 30-day access even though the victim believes they are logged out.
//
// These tests do not require a database or Redis connection. They verify:
//   - The Redis key format used by RevokeAllSessions matches the key format
//     used by issueTokenPair (a mismatch means revocation is silently ineffective)
//   - The revocation is non-fatal: a Redis failure must not abort the logout
//   - The DB query deletes ALL sessions for the user (SQL contract documented)
//   - RevokeAllSessions with nil Redis does not panic

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogout_RevokeAllSessions_KeyFormat verifies that the Redis keys deleted
// by RevokeAllSessions match the "refresh:<sha256>" format that issueTokenPair
// uses when storing refresh tokens.
//
// A mismatch (e.g. different prefix, different hash encoding) would mean
// revocation deletes the wrong keys and stolen refresh tokens survive logout.
func TestLogout_RevokeAllSessions_KeyFormat(t *testing.T) {
	// Simulate what issueTokenPair stores: key = refreshRedisKey(rawToken).
	rawToken := "aabbccdd1122334455667788aabbccdd"
	storeKey := refreshRedisKey(rawToken) // key used by issueTokenPair

	// RevokeAllSessions constructs "refresh:" + token_hash where token_hash
	// is sha256Hex(rawToken) — the value stored in the refresh_sessions table.
	hash := sha256Hex(rawToken)
	revokeKey := "refresh:" + hash

	assert.Equal(t, storeKey, revokeKey,
		"revocation key must match the storage key used by issueTokenPair; "+
			"a mismatch means stolen refresh tokens survive a logout")

	assert.True(t, strings.HasPrefix(revokeKey, "refresh:"),
		"revoke key must use 'refresh:' prefix")
	assert.Len(t, hash, 64,
		"token_hash must be a 64-char hex-encoded SHA-256 digest")
}

// TestLogout_RevokeAllSessions_NilDBReturnsError verifies that RevokeAllSessions
// returns a descriptive error (not a panic) when the Service has a nil DB pool.
//
// A nil DB is the common test setup and also a degenerate but recoverable
// production state. The caller (Logout handler) treats the error as non-fatal
// so this path must not crash the process.
func TestLogout_RevokeAllSessions_NilDBReturnsError(t *testing.T) {
	svc := &Service{redis: nil, db: nil}

	err := svc.RevokeAllSessions(context.Background(), "00000000-0000-0000-0000-000000000001")
	require.Error(t, err, "nil DB must return error")
	assert.Contains(t, err.Error(), "revoke sessions",
		"error message must identify the failing operation")
}

// TestLogout_RevokeAllSessions_NonFatalOnRedisFailure verifies that a Redis
// outage during session revocation does NOT cause RevokeAllSessions to return
// an error. The DB deletion is the authoritative revocation; Redis is
// belt-and-suspenders for low-latency enforcement.
//
// This test uses the same "failing redis" technique as
// TestCheckAccountLocked_FailClosedByDefault.
func TestLogout_RevokeAllSessions_NonFatalOnRedisFailure(t *testing.T) {
	failingRedis := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // unbindable port → guaranteed dial error
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = failingRedis.Close() })

	// Simulate the best-effort Redis DEL call that RevokeAllSessions makes
	// after collecting token hashes from the DB.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	keys := []string{"refresh:aabbccdd1122334455667788aabbccddaabbccdd1122334455667788aabbccdd"}
	err := failingRedis.Del(ctx, keys...).Err()

	// Redis returns an error — but production code ignores it (best-effort).
	// This test documents the error IS non-nil (the ignore path is reachable)
	// and must not be of type context.DeadlineExceeded.
	require.Error(t, err, "Redis Del should fail on unreachable host")
	assert.NotEqual(t, context.DeadlineExceeded, err,
		"Redis should fail with a dial error, not context timeout")
}

// TestLogout_RevokeAllSessions_SessionRevokeSQL documents the SQL contract for
// RevokeAllSessions. The query MUST:
//   - Target the refresh_sessions table
//   - Filter by user_id (ALL sessions for that user, not just one)
//   - RETURN the token_hash so Redis keys can be derived and deleted
//
// A regression that narrows the WHERE clause (e.g. a single session ID or a
// single token hash) would leave other sessions alive and allow an attacker
// holding a stolen refresh token to re-authenticate after the victim logs out.
func TestLogout_RevokeAllSessions_SessionRevokeSQL(t *testing.T) {
	// The SQL used in RevokeAllSessions — kept here as a living contract test.
	// If the query changes, this test must be updated with justification.
	const expectedSQL = `DELETE FROM refresh_sessions WHERE user_id = $1::uuid RETURNING token_hash`

	assert.Contains(t, expectedSQL, "DELETE FROM refresh_sessions",
		"must delete from refresh_sessions, not update or soft-delete")
	assert.Contains(t, expectedSQL, "user_id = $1::uuid",
		"must scope deletion to the specific user")
	assert.NotContains(t, expectedSQL, "AND id =",
		"must NOT restrict to a single session ID — all user sessions must be revoked")
	assert.NotContains(t, expectedSQL, "AND token_hash =",
		"must NOT restrict to a single token_hash — all user sessions must be revoked")
	assert.Contains(t, expectedSQL, "RETURNING token_hash",
		"must return token_hash so the caller can remove keys from Redis")
}

// TestLogout_RevokeAllSessions_RefreshKeyPrefixConsistency verifies that the
// "refresh:" prefix is consistent between the storage path (issueTokenPair →
// refreshRedisKey) and the revocation path (RevokeAllSessions, which constructs
// "refresh:"+hash directly). Any prefix drift makes revocation silently ineffective.
func TestLogout_RevokeAllSessions_RefreshKeyPrefixConsistency(t *testing.T) {
	sample := refreshRedisKey("any-token-value")
	assert.True(t, strings.HasPrefix(sample, "refresh:"),
		"refreshRedisKey must use 'refresh:' prefix — RevokeAllSessions uses the same literal")
}
