//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
)

// TestCrossOrgIsolation_VaultAndHR closes S131-G6/R-L07: the org-isolation of
// vaktvault (GetProject/GetSecret/SetSecret) and vakthr (GetEmployee/
// UpdateEmployee) was asserted only by a `t.Skip("SECURITY GAP: ... enforced via
// WHERE org_id = $N")` placeholder — the guarantee was documented, never run.
//
// This drives both services with TWO real orgs against a live Postgres and
// proves that org B cannot read or write org A's data: the WHERE org_id clauses
// turn every cross-org access into a not-found error, never a leak or a write.
func TestCrossOrgIsolation_VaultAndHR(t *testing.T) {
	pool, cleanup := bootPostgres(t)
	defer cleanup()
	ctx := context.Background()

	// Two independent orgs, each with a member user.
	newOrg := func(slug, email string) (orgID, userID string) {
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&orgID))
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO users (email, display_name) VALUES ($1, 'U') RETURNING id::text`, email).Scan(&userID))
		require.NoError(t, ensureMember(ctx, pool, userID, orgID))
		return orgID, userID
	}
	orgA, userA := newOrg("iso-a", "iso-a@example.test")
	orgB, userB := newOrg("iso-b", "iso-b@example.test")

	// ── vaktvault ──────────────────────────────────────────────────────────
	master, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	vault := vaktvault.NewService(pool, master, nil)

	projA, err := vault.CreateProject(ctx, orgA, userA, "ProjA", "org A project")
	require.NoError(t, err)
	var envA string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO so_environments (project_id, org_id, name) VALUES ($1::uuid, $2::uuid, 'prod')
		 RETURNING id::text`, projA.ID, orgA).Scan(&envA))
	_, err = vault.SetSecret(ctx, orgA, envA, userA, "API_KEY", "org-a-secret")
	require.NoError(t, err)

	t.Run("vault_GetProject_crossOrg", func(t *testing.T) {
		_, err := vault.GetProject(ctx, orgB, projA.ID)
		assert.Error(t, err, "org B must not read org A's project")
	})
	t.Run("vault_GetSecret_crossOrg", func(t *testing.T) {
		_, err := vault.GetSecret(ctx, orgB, envA, "API_KEY", "api", "1.2.3.4")
		assert.Error(t, err, "org B must not read a secret in org A's environment")
	})
	t.Run("vault_SetSecret_crossOrg", func(t *testing.T) {
		_, err := vault.SetSecret(ctx, orgB, envA, userB, "API_KEY", "hijack")
		assert.Error(t, err, "org B must not write into org A's environment")
		// And org A's value must be unchanged.
		got, err := vault.GetSecret(ctx, orgA, envA, "API_KEY", "api", "1.2.3.4")
		require.NoError(t, err)
		assert.Equal(t, "org-a-secret", got.Value, "cross-org write must not have mutated org A's secret")
	})

	// ── vakthr ─────────────────────────────────────────────────────────────
	hr := vakthr.NewServiceFromPool(pool)
	actorA := vakthr.Actor{OrgID: orgA, UserID: userA, UserEmail: "iso-a@example.test"}
	actorB := vakthr.Actor{OrgID: orgB, UserID: userB, UserEmail: "iso-b@example.test"}

	empA, err := hr.CreateEmployee(ctx, actorA, vakthr.CreateEmployeeInput{
		FirstName: "Alice", LastName: "A", Email: "alice@iso-a.test",
	})
	require.NoError(t, err)

	t.Run("hr_GetEmployee_crossOrg", func(t *testing.T) {
		_, err := hr.GetEmployee(ctx, orgB, empA.ID)
		assert.Error(t, err, "org B must not read org A's employee")
	})
	t.Run("hr_UpdateEmployee_crossOrg", func(t *testing.T) {
		_, err := hr.UpdateEmployee(ctx, actorB, empA.ID, vakthr.UpdateEmployeeInput{
			FirstName: "Mallory", LastName: "A", Status: "terminated",
		})
		assert.Error(t, err, "org B must not update org A's employee")
		// org A's employee unchanged.
		got, err := hr.GetEmployee(ctx, orgA, empA.ID)
		require.NoError(t, err)
		assert.Equal(t, "Alice", got.FirstName, "cross-org update must not have mutated org A's employee")
	})

	// Checklist run isolation (the two remaining R-L07 skips).
	clA, err := hr.CreateChecklist(ctx, actorA, vakthr.CreateChecklistInput{
		Type: "onboarding", Name: "OnbA",
		Items: []vakthr.ChecklistItem{{ID: "step1", Label: "Provision laptop", Required: true}},
	})
	require.NoError(t, err)
	runA, err := hr.StartChecklistRun(ctx, actorA, vakthr.StartChecklistRunInput{
		EmployeeID: empA.ID, ChecklistID: clA.ID,
	})
	require.NoError(t, err)

	t.Run("hr_GetChecklistRun_crossOrg", func(t *testing.T) {
		_, err := hr.GetChecklistRun(ctx, orgB, runA.ID)
		assert.Error(t, err, "org B must not read org A's checklist run")
	})
	t.Run("hr_CompleteChecklistRun_crossOrg", func(t *testing.T) {
		_, err := hr.UpdateChecklistRun(ctx, actorB, runA.ID, vakthr.UpdateChecklistRunInput{
			Status: "completed", CompletedItems: []string{"step1"},
		})
		assert.Error(t, err, "org B must not complete org A's checklist run")
		// org A's run still in progress.
		got, err := hr.GetChecklistRun(ctx, orgA, runA.ID)
		require.NoError(t, err)
		assert.NotEqual(t, "completed", got.Status, "cross-org update must not have completed org A's run")
	})

	// Checklist template isolation (the 8th R-L07 skip).
	t.Run("hr_GetChecklist_crossOrg", func(t *testing.T) {
		_, err := hr.GetChecklist(ctx, orgB, clA.ID)
		assert.Error(t, err, "org B must not read org A's checklist template")
	})
	t.Run("hr_ListChecklists_crossOrg", func(t *testing.T) {
		listB, err := hr.ListChecklists(ctx, orgB)
		require.NoError(t, err)
		for _, c := range listB {
			assert.NotEqual(t, clA.ID, c.ID, "org B's checklist list must not contain org A's template")
		}
	})
}
