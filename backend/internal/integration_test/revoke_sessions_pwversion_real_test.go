//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// TestRevokeAllSessionsBumpsPwVersion is the regression guard for S131-C1/R-H21
// (D21-01): a role downgrade, RemoveUser, or SCIM deprovision called
// RevokeAllSessions, which deleted the refresh sessions but left the stateless
// Paseto access token valid until natural expiry (TTL 1h) — SA-15/16 verified
// live that an old token still minted API keys after a downgrade.
//
// The fix bumps pw_version inside RevokeAllSessions; checkPwVersion rejects any
// token carrying the stale version on the very next request. This test asserts
// both effects at the DB level: refresh sessions gone AND pw_version incremented.
func TestRevokeAllSessionsBumpsPwVersion(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active)
		 VALUES ('revoke-target@acme.test', 'Revoke Target', TRUE)
		 RETURNING id::text`).Scan(&userID))

	// Two active refresh sessions on different "devices".
	for i, h := range []string{"hash-a", "hash-b"} {
		_, err := pool.Exec(ctx,
			`INSERT INTO refresh_sessions (user_id, org_id, token_hash, expires_at)
			 VALUES ($1::uuid, $2::uuid, $3, NOW() + INTERVAL '30 days')`,
			userID, orgID, h)
		require.NoError(t, err, "seed refresh session %d", i)
	}

	var pwBefore int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&pwBefore))
	assert.EqualValues(t, 0, pwBefore, "fresh user starts at pw_version 0")

	// nil redis is fine: bumpPwVersion writes the durable PG value, which
	// checkPwVersion falls back to when the Redis key is absent/stale.
	svc := auth.NewService(pool, nil, mustKeyIntegration(t))

	require.NoError(t, svc.RevokeAllSessions(ctx, userID))

	// Refresh sessions gone.
	var remaining int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM refresh_sessions WHERE user_id = $1::uuid`, userID).Scan(&remaining))
	assert.Equal(t, 0, remaining, "all refresh sessions must be deleted")

	// pw_version bumped → stale access tokens are now rejected per-request.
	var pwAfter int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&pwAfter))
	assert.EqualValues(t, 1, pwAfter, "RevokeAllSessions must bump pw_version so the access token dies immediately")

	// Idempotent-ish: a second revoke bumps again (no crash on zero sessions).
	require.NoError(t, svc.RevokeAllSessions(ctx, userID))
	var pwAfter2 int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&pwAfter2))
	assert.EqualValues(t, 2, pwAfter2)
}
