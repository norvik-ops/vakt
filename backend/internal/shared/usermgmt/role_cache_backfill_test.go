// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package usermgmt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backfillMigration is the migration this test runs. It lives here, not next to
// cmd/migrate, because the rule it encodes is this package's: users.role is the
// cache usermgmt used to authorise over, and the direction of the backfill is a
// privilege decision, not a schema detail.
const backfillMigration = "253_users_role_cache_drift_backfill.up.sql"

// TestMigration253_onlyEverLowersTheCache runs the real migration file against
// real drifted rows and pins its direction.
//
// A migration that touches roles is a privileged path, and this one has exactly
// one safe direction. The drift it cleans up has two shapes and they mean
// opposite things:
//
//	org_members=Viewer, users.role='admin'  — the remainder of a PROMOTION that
//	    only ever reached the cache. Lowering the cache to match org_members is
//	    what the platform already believes (the member's claims say Viewer).
//	org_members=Admin,  users.role='viewer' — the remainder of a DEMOTION. Raising
//	    the cache to 'admin' here would undo an intent the operator expressed, and
//	    since users.role is a single global column while org_members is per-org, it
//	    would also carry an Admin from one org into every other one. That is the
//	    shape migration 249 exists to remove.
//
// So: downwards only. The subtests below are the assertion that it stays that
// way — the "no promotion" cases fail the moment someone makes the UPDATE
// symmetric.
//
// The migration is applied inside a transaction that is rolled back: it is a
// global UPDATE and other packages share this database.
func TestMigration253_onlyEverLowersTheCache(t *testing.T) {
	pool, _ := testDB(t)
	ctx := context.Background()

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", backfillMigration))
	require.NoError(t, err, "migration file missing — was it renumbered?")

	orgA := seedOrg(t, pool, "esk13-backfill-a")
	orgB := seedOrg(t, pool, "esk13-backfill-b")

	cases := []struct {
		name     string
		platform string
		cache    string
		want     string
		why      string
	}{
		{
			name: "promotion remainder is lowered", platform: "Viewer", cache: "admin", want: "viewer",
			why: "a cache-only 'admin' is the leftover of a promotion that never reached the token claim",
		},
		{
			name: "lowering stops at the derived role", platform: "SecurityAnalyst", cache: "admin", want: "editor",
			why: "the target is the role org_members actually grants, not viewer-for-everyone",
		},
		{
			name: "demotion remainder is NOT raised", platform: "Admin", cache: "viewer", want: "viewer",
			why: "raising here would re-grant an admin the operator had demoted",
		},
		{
			name: "an auditor is not lowered further", platform: "InternalAuditor", cache: "viewer", want: "viewer",
			why: "InternalAuditor derives to the viewer cache value — nothing to do",
		},
		{
			name: "a consistent admin is left alone", platform: "Admin", cache: "admin", want: "admin",
			why: "the control: a backfill that demoted everyone would pass the cases above",
		},
	}

	ids := make([]string, len(cases))
	for i, tc := range cases {
		ids[i] = seedDriftedMember(t, pool, orgA, tc.platform, tc.cache)
	}

	// users.role is global, org_members per-org. A member who is Admin somewhere
	// keeps the 'admin' cache — 249's rule, and the reason the WHERE derives from
	// the highest role over ALL orgs instead of one.
	multiOrgID := seedDriftedMember(t, pool, orgA, "Admin", "admin")
	addToOrg(t, pool, orgB, multiOrgID, "Viewer")

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, string(sqlBytes))
	require.NoError(t, err, "the migration must apply cleanly")

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cacheRoleTx(t, tx, ids[i]), tc.why)
		})
	}

	t.Run("a member who is Admin in any org keeps the admin cache", func(t *testing.T) {
		assert.Equal(t, "admin", cacheRoleTx(t, tx, multiOrgID),
			"deriving per-org would strip the Admin of org A because org B says Viewer")
	})
}

// addToOrg makes an existing user a member of a second organisation.
func addToOrg(t *testing.T, pool *pgxpool.Pool, orgID, userID, platformRole string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = $1`, platformRole).Scan(&roleID))
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM org_members WHERE org_id = $1::uuid AND user_id = $2::uuid`, orgID, userID)
	})
}

func cacheRoleTx(t *testing.T, tx pgx.Tx, userID string) string {
	t.Helper()
	var role string
	require.NoError(t, tx.QueryRow(context.Background(),
		`SELECT role FROM users WHERE id = $1::uuid`, userID).Scan(&role))
	return role
}
