//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/usermgmt"
)

// TestAdminMFABreakGlass covers S131-R-H23: an admin can clear a locked-out
// member's TOTP + recovery codes (break-glass), org-scoped so an admin can never
// reset a user in another org.
func TestAdminMFABreakGlass(t *testing.T) {
	pool, orgA, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	// A second org, to prove isolation.
	var orgB string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('OrgB', 'orgb') RETURNING id::text`).Scan(&orgB))

	// A member of orgA with TOTP enrolled + recovery codes.
	var userID, roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, is_active) VALUES ('locked@acme.test', 'Locked', TRUE)
		 RETURNING id::text`).Scan(&userID))
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgA, userID, roleID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO totp_secrets (user_id, secret, enabled) VALUES ($1::uuid, 'JBSWY3DPEHPK3PXP', TRUE)`, userID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO auth_recovery_codes (user_id, code_hash) VALUES ($1::uuid, 'hash-a'), ($1::uuid, 'hash-b')`, userID)
	require.NoError(t, err)

	svc := usermgmt.NewService(pool, usermgmt.SMTPConfig{}, "")

	count := func(table string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM `+table+` WHERE user_id = $1::uuid`, userID).Scan(&n))
		return n
	}

	// Cross-org: an admin of orgB must NOT be able to reset orgA's member.
	err = svc.ResetUserMFA(ctx, orgB, userID)
	require.ErrorIs(t, err, usermgmt.ErrUserNotInOrg)
	assert.Equal(t, 1, count("totp_secrets"), "cross-org reset must not touch the TOTP secret")
	assert.Equal(t, 2, count("auth_recovery_codes"), "cross-org reset must not touch recovery codes")

	// In-org: the admin resets the member's MFA → TOTP + recovery codes gone.
	require.NoError(t, svc.ResetUserMFA(ctx, orgA, userID))
	assert.Equal(t, 0, count("totp_secrets"), "reset must delete the TOTP secret")
	assert.Equal(t, 0, count("auth_recovery_codes"), "reset must delete the recovery codes")

	// Idempotent-ish: resetting again (nothing to delete) still succeeds.
	require.NoError(t, svc.ResetUserMFA(ctx, orgA, userID))
}
