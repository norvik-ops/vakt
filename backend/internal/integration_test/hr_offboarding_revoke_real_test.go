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
	"github.com/matharnica/vakt/internal/modules/vakthr"
)

// TestHROffboardingRevokesAccessToken is the regression guard for S131-C1/R-H21
// on the vakthr path: terminating an employee must not only delete the platform
// user's refresh sessions but also bump pw_version, so the stateless access
// token dies immediately. Before the SessionRevoker was wired, HR offboarding
// deleted refresh sessions and removed org membership but left the access token
// valid for up to the 1h TTL — vakthr's core promise ("audit-ready evidence that
// access revocation occurred") was false for that window.
func TestHROffboardingRevokesAccessToken(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	const email = "departing@acme.test"

	// A platform user who is a member of the org, with an active refresh session.
	var userID, roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active) VALUES ($1, 'Dep', TRUE)
		 RETURNING id::text`, email).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO refresh_sessions (user_id, org_id, token_hash, expires_at)
		 VALUES ($1::uuid, $2::uuid, 'hr-off-hash', NOW() + INTERVAL '30 days')`, userID, orgID)
	require.NoError(t, err)

	// HR service with the auth session revoker wired, exactly as cmd/api does.
	authSvc := auth.NewService(pool, nil, mustKeyIntegration(t))
	hr := vakthr.NewServiceFromPool(pool).WithSessionRevoker(authSvc)
	actor := vakthr.Actor{OrgID: orgID, UserID: userID, UserEmail: "admin@acme.test"}

	emp, err := hr.CreateEmployee(ctx, actor, vakthr.CreateEmployeeInput{
		FirstName: "Dep", LastName: "Arting", Email: email,
	})
	require.NoError(t, err)

	// Terminate → revokeUserAccess.
	_, err = hr.UpdateEmployee(ctx, actor, emp.ID, vakthr.UpdateEmployeeInput{
		FirstName: "Dep", LastName: "Arting", Status: "terminated",
	})
	require.NoError(t, err)

	// Refresh sessions gone.
	var sessions int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM refresh_sessions WHERE user_id = $1::uuid`, userID).Scan(&sessions))
	assert.Equal(t, 0, sessions, "offboarding must delete the refresh sessions")

	// pw_version bumped → the stateless access token is now rejected per-request.
	var pwVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&pwVersion))
	assert.EqualValues(t, 1, pwVersion,
		"offboarding must bump pw_version so the terminated employee's access token dies immediately")

	// Org membership removed.
	var members int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE user_id = $1::uuid AND org_id = $2::uuid`,
		userID, orgID).Scan(&members))
	assert.Equal(t, 0, members, "offboarding must remove org membership")
}
