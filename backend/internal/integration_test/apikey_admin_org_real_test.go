//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/apikeys"
)

// TestAdminCanSeeAndRevokeForeignAPIKeys is the regression guard for S131-D15-08:
// the per-user List/Revoke are scoped to created_by, so an admin could neither see
// nor revoke another user's API key — a hole in the offboarding story. The new
// org-scoped ListOrg/RevokeOrg (admin-gated at the route) close it.
func TestAdminCanSeeAndRevokeForeignAPIKeys(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var adminID, otherID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name) VALUES ('admin@acme.test', 'Admin') RETURNING id::text`).Scan(&adminID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name) VALUES ('other@acme.test', 'Other') RETURNING id::text`).Scan(&otherID))

	svc := apikeys.NewService(pool)

	adminKey, err := svc.Create(ctx, orgID, adminID, apikeys.CreateInput{Name: "admin key"})
	require.NoError(t, err)
	otherKey, err := svc.Create(ctx, orgID, otherID, apikeys.CreateInput{Name: "other key"})
	require.NoError(t, err)

	// Per-user list for the admin sees ONLY the admin's own key (the old gap).
	adminOwn, err := svc.List(ctx, orgID, adminID)
	require.NoError(t, err)
	assert.Len(t, adminOwn, 1, "per-user List is created_by-scoped")

	// Org-wide admin list sees BOTH keys, with owner emails.
	all, err := svc.ListOrg(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, all, 2, "admin org list must see every user's key")
	emails := map[string]bool{}
	for _, k := range all {
		emails[k.CreatedByEmail] = true
	}
	assert.True(t, emails["admin@acme.test"] && emails["other@acme.test"], "org list must carry owner emails")

	// Admin revokes the OTHER user's key.
	require.NoError(t, svc.RevokeOrg(ctx, orgID, otherKey.APIKey.ID))

	// The other user's key is now gone from the org list; the admin's remains.
	afterRevoke, err := svc.ListOrg(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, afterRevoke, 1)
	assert.Equal(t, adminKey.APIKey.ID, afterRevoke[0].ID, "only the admin's key survives")

	// Revoking an unknown key → ErrNotFound (→ 404 at the handler).
	err = svc.RevokeOrg(ctx, orgID, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, apikeys.ErrNotFound)
}
