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

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// TestVaktaware_AssignModule_Idempotent is the S126 (A17-01) regression guard for
// the born-broken vaktaware assignment class. This module historically produced
// the most born-broken bugs — the ON CONFLICT-against-a-DEFERRABLE-constraint
// upsert, path drift, DeleteFinding — and every one was found only by a live
// sweep, never by a test, because the module has concrete *Repository types that
// resist mocking. This testcontainers test exercises the real service against
// real Postgres so the class is caught by CI, not by a manual browse.
//
// It proves the AssignModule email→target→assignment flow works AND is idempotent:
// assigning the same emails twice must not raise (the old ON CONFLICT-on-a-
// deferrable-unique query raised "ON CONFLICT does not support deferrable unique
// constraints") and must not create duplicate assignments.
func TestVaktaware_AssignModule_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('aware-admin@acme.test') RETURNING id::text`).Scan(&userID))

	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{})
	repo := vaktaware.NewRepository(pool)

	mod, err := repo.CreateModule(ctx, orgID, userID, vaktaware.CreateModuleInput{
		Title:        "Phishing 101",
		Type:         "video",
		AttackType:   "phishing",
		ContentURL:   "https://example.test/video",
		PassingScore: 80,
	})
	require.NoError(t, err)

	emails := []string{"alice@acme.test", "bob@acme.test"}

	// First assignment round.
	assigned, failed := svc.AssignModule(ctx, orgID, mod.ID, emails)
	assert.Equal(t, 2, assigned, "both emails should be assigned")
	assert.Empty(t, failed)

	// Second round with the SAME emails — the born-broken class. Must not raise,
	// must not duplicate.
	assigned2, failed2 := svc.AssignModule(ctx, orgID, mod.ID, emails)
	assert.Equal(t, 2, assigned2, "re-assigning the same emails must succeed idempotently")
	assert.Empty(t, failed2)

	// Exactly two assignment rows exist (no duplicates from the second round).
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sr_assignments a
		JOIN sr_targets tg ON tg.id = a.target_id
		WHERE a.module_id = $1::uuid AND a.org_id = $2::uuid`,
		mod.ID, orgID).Scan(&count))
	assert.Equal(t, 2, count, "the same (module, target) pair must not create duplicate assignments")

	// The joined listing the frontend consumes returns both targets by email.
	details, err := repo.ListAssignmentsByModule(ctx, orgID, mod.ID)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, d := range details {
		got[d.UserEmail] = true
	}
	assert.True(t, got["alice@acme.test"] && got["bob@acme.test"],
		"ListSRAssignmentsByModule must return both assigned emails, got %v", got)
}
