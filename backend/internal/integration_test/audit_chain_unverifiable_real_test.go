//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/audit"
)

// Regression tests for R1-27-V02 / ESK-4.
//
// The defect: VerifyOrgChain skipped every row with a NULL entry_hash and then
// reported the org as clean. An audit trail that contains rows nobody checked
// was announced as verified — plausibly wrong, which for ISO 27001 / NIS2
// evidence is the worst failure mode.
//
// The fix does NOT back-fill hashes for legacy rows: a hash computed today
// proves nothing about whether the row was altered yesterday, so retro-active
// chaining would fabricate evidence. The honest mixed state is reported
// instead, with both denominators.

// TestAuditChain_HashlessRowIsUnverifiableNotIntact is the core regression: an
// org whose log contains one unchained row must NOT come back as intact.
func TestAuditChain_HashlessRowIsUnverifiableNotIntact(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	// One legacy row without prev_hash/entry_hash — exactly the shape the AI
	// agent's raw INSERT produced before the fix.
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_log (org_id, user_email, action, resource_type)
		VALUES ($1::uuid, 'ai_agent', 'agent_run_start', 'ai/agent')`, orgID)
	require.NoError(t, err)

	// Two properly chained rows on top.
	for _, a := range []string{"create", "update"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID: orgID, Action: a, ResourceType: "control",
		})
	}

	res, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)

	assert.Equal(t, audit.ChainUnverifiable, res.Status,
		"an org with an unchained row must not be reported as intact")
	assert.False(t, res.FullyVerified(), "FullyVerified() must be false when rows went unchecked")
	assert.Equal(t, 1, res.Unverifiable, "the hashless row must be counted, not skipped")
	assert.Equal(t, 2, res.Verified, "the two chained rows must still be verified")
	assert.Equal(t, 3, res.Total, "the denominator must cover every examined row")
	assert.Empty(t, res.FirstBadRow, "an unchained row is not a chain break")
}

// TestAuditChain_BreakStillDetectedNextToHashlessRows guards the opposite
// mistake: making the verifier honest about unchained rows must not blunt it.
// A real tamper alongside a legacy row must still be reported as BROKEN, with
// the exact row named.
func TestAuditChain_BreakStillDetectedNextToHashlessRows(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO audit_log (org_id, user_email, action, resource_type)
		VALUES ($1::uuid, 'legacy@example.org', 'legacy', 'control')`, orgID)
	require.NoError(t, err)

	for _, a := range []string{"create", "update", "approve"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID: orgID, Action: a, ResourceType: "control",
		})
	}

	var targetID string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id::text FROM audit_log
		WHERE org_id = $1::uuid AND action = 'update'
		ORDER BY created_at ASC LIMIT 1`, orgID).Scan(&targetID))

	// orgid-lint: global — tamper-simulation fixture; targetID was already fetched via an org-scoped query above
	_, err = pool.Exec(ctx, `UPDATE audit_log SET action = 'EVIL' WHERE id = $1::uuid`, targetID)
	require.NoError(t, err)

	res, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)

	assert.Equal(t, audit.ChainBroken, res.Status,
		"broken must outrank unverifiable — a tamper next to legacy rows is still a tamper")
	assert.Equal(t, targetID, res.FirstBadRow, "the tampered row must be named")
	assert.Equal(t, 1, res.Unverifiable, "the legacy row is still counted")
}

// TestAuditChain_AllChainedStaysIntact pins the third state: with no hashless
// rows at all the result is intact and Unverifiable is zero. Without this the
// suite could not tell "intact" from "unverifiable" being returned everywhere.
func TestAuditChain_AllChainedStaysIntact(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	for _, a := range []string{"create", "update"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID: orgID, Action: a, ResourceType: "control",
		})
	}

	res, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Equal(t, audit.ChainIntact, res.Status)
	assert.True(t, res.FullyVerified())
	assert.Zero(t, res.Unverifiable)
	assert.Equal(t, 2, res.Verified)
}
