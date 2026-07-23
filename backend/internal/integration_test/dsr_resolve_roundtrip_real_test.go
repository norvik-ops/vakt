//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hibiken/asynq"

	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

// TestDSRResolveRoundTrip is the regression guard for S131-G3/D27-04: the
// resolve/extend action writes resolved_by + extension_reason, but before the fix
// NO read (and not even the action's own RETURNING) selected them back — the API
// returned both silently empty. The variant-miss review (subreview #G3) found the
// gap spanned four paths of the same write action: ResolveDSR's own response,
// GetDSR, the offset-mode list (ListDSRs) and the cursor-mode list (ListDSRsCursor).
//
// This test exercises all four so a future scan-order slip or a dropped column on
// any single path fails loudly.
func TestDSRResolveRoundTrip(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name) VALUES ('dsr-resolver@acme.test', 'Resolver')
		 RETURNING id::text`).Scan(&userID))

	repo := vaktprivacy.NewRepository(pool)
	svc := vaktprivacy.NewService(pool, asynq.RedisClientOpt{}) // no asynq client needed

	created, err := repo.CreateDSR(ctx, orgID, vaktprivacy.CreateDSRInput{
		RequesterName:  "Data Subject",
		RequesterEmail: "subject@example.test",
		Type:           "access",
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	const reason = "awaiting third-party controller confirmation"

	// 1) The resolve action's OWN response must carry what it just wrote.
	resolved, err := svc.ResolveDSR(ctx, orgID, created.ID, userID, vaktprivacy.ResolveDSRInput{
		ResolutionType:  "extended",
		ExtensionReason: reason,
	})
	require.NoError(t, err)
	require.NotNil(t, resolved.ResolvedBy, "ResolveDSR response: resolved_by")
	assert.Equal(t, userID, *resolved.ResolvedBy)
	assert.Equal(t, reason, resolved.ExtensionReason, "ResolveDSR response: extension_reason")

	assertDSR := func(t *testing.T, d *vaktprivacy.DSR, where string) {
		t.Helper()
		require.NotNil(t, d.ResolvedBy, "%s: resolved_by", where)
		assert.Equal(t, userID, *d.ResolvedBy, where)
		assert.Equal(t, reason, d.ExtensionReason, "%s: extension_reason", where)
	}

	// 2) Single read.
	got, err := repo.GetDSR(ctx, orgID, created.ID)
	require.NoError(t, err)
	assertDSR(t, got, "GetDSR")

	// 3) Offset-mode list.
	list, err := repo.ListDSRs(ctx, orgID)
	require.NoError(t, err)
	assertDSR(t, findDSR(t, list, created.ID), "ListDSRs")

	// 4) Cursor-mode list.
	cursorList, err := repo.ListDSRsCursor(ctx, orgID, "", time.Time{}, 50)
	require.NoError(t, err)
	assertDSR(t, findDSR(t, cursorList, created.ID), "ListDSRsCursor")

	// 5) A subsequent generic PATCH must not drop the fields from its own response
	//    (symmetric with UpdateCAPA→GetCAPA).
	updated, err := repo.UpdateDSR(ctx, orgID, created.ID, vaktprivacy.UpdateDSRInput{Status: "completed"})
	require.NoError(t, err)
	assertDSR(t, updated, "UpdateDSR")
}

func findDSR(t *testing.T, list []vaktprivacy.DSR, id string) *vaktprivacy.DSR {
	t.Helper()
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	require.Failf(t, "DSR not found", "id %s missing from list of %d", id, len(list))
	return nil
}
