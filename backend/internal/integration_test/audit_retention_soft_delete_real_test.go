//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

// TestAuditRetention_SoftDelete guards FINDING DATA-002:
// RunRetention must soft-delete audit_log rows (set deleted_at) rather than
// hard-deleting them so that the SHA-256 hash chain (migration 149 / ADR-0040)
// remains intact for cmd/audit-verify.
//
// Three invariants are tested:
//  1. Rows older than the retention window are marked deleted_at IS NOT NULL.
//  2. Fresh rows (within the window) are NOT touched.
//  3. After soft-delete, audit.VerifyOrgChain must still report the chain as
//     clean — i.e. the chain verifier ignores deleted_at and verifies all rows
//     including soft-deleted ones.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/retention"
)

func TestAuditRetention_SoftDeletePreservesChain(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	// ── 1. Insert three chained audit entries ────────────────────────────────
	// The two "old" rows are written with a past CreatedAt so the hash is
	// consistent from the start — no SQL back-dating needed (which would
	// invalidate the stored entry_hash and break the verifier).
	twoDaysAgo := time.Now().UTC().Add(-48 * time.Hour)
	// Distinct per-row timestamps: the hash chain is ordered by (created_at, id),
	// and id is a random UUID. Two rows sharing the exact same created_at would
	// order non-deterministically by their random ids — flaking VerifyOrgChain
	// ~50% of the time (same class as commit 2d2be0ff). A 1s stride per row keeps
	// both well over the 1-day retention window while making the order stable.
	for i, action := range []string{"create", "update"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID:        orgID,
			UserEmail:    "ops@example.org",
			Action:       action,
			ResourceType: "control",
			ResourceID:   "ctrl-1",
			CreatedAt:    twoDaysAgo.Add(time.Duration(i) * time.Second),
		})
	}
	audit.Write(ctx, pool, audit.WriteEntry{
		OrgID:        orgID,
		UserEmail:    "ops@example.org",
		Action:       "delete",
		ResourceType: "control",
		ResourceID:   "ctrl-1",
	})

	// Verify chain is clean before we do anything.
	bad, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	require.Empty(t, bad, "pre-condition: freshly-written chain must be clean")

	// ── 3. Run retention with a 1-day window ─────────────────────────────────
	_, err = pool.Exec(ctx, `
		INSERT INTO retention_config (org_id, audit_log_days, findings_resolved_days, notifications_days, scan_history_days, updated_at)
		VALUES ($1::uuid, 1, 0, 0, 0, NOW())
		ON CONFLICT (org_id) DO UPDATE SET audit_log_days = EXCLUDED.audit_log_days`, orgID)
	require.NoError(t, err)

	err = retention.RunRetention(ctx, pool, orgID)
	require.NoError(t, err)

	// ── 4. Invariant A: the two old rows must be soft-deleted ─────────────────
	var softDeletedCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE org_id = $1::uuid AND deleted_at IS NOT NULL`, orgID).Scan(&softDeletedCount))
	assert.Equal(t, 2, softDeletedCount,
		"retention must soft-delete exactly the two rows that exceed the window")

	// ── 5. Invariant B: the fresh row must be untouched ───────────────────────
	var freshCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE org_id = $1::uuid AND deleted_at IS NULL`, orgID).Scan(&freshCount))
	assert.Equal(t, 1, freshCount,
		"retention must not touch rows still within the retention window")

	// ── 6. Invariant C: chain must still verify clean after soft-delete ───────
	// The verifier must walk ALL rows (incl. soft-deleted) to check the chain.
	bad, err = audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Empty(t, bad,
		"chain must remain verifiable after soft-delete — soft-deleted rows stay in the chain")
}

func TestAuditRetention_IdempotentSoftDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	audit.Write(ctx, pool, audit.WriteEntry{
		OrgID: orgID, Action: "create", ResourceType: "policy",
	})

	// Back-date the row.
	_, err := pool.Exec(ctx, `
		UPDATE audit_log SET created_at = NOW() - INTERVAL '10 days'
		WHERE org_id = $1::uuid`, orgID)
	require.NoError(t, err)

	// Insert a short retention window so the 10-day-old row falls outside it.
	_, err = pool.Exec(ctx, `
		INSERT INTO retention_config (org_id, audit_log_days, findings_resolved_days, notifications_days, scan_history_days, updated_at)
		VALUES ($1::uuid, 1, 0, 0, 0, NOW())
		ON CONFLICT (org_id) DO UPDATE SET audit_log_days = EXCLUDED.audit_log_days`, orgID)
	require.NoError(t, err)

	// Run retention twice.
	require.NoError(t, retention.RunRetention(ctx, pool, orgID))
	require.NoError(t, retention.RunRetention(ctx, pool, orgID))

	// Still only one soft-deleted row — no duplicate timestamp writes or errors.
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE org_id = $1::uuid AND deleted_at IS NOT NULL`, orgID).Scan(&count))
	assert.Equal(t, 1, count, "idempotent: running retention twice must not double-count")
}

func TestAuditRetention_DisabledDoesNotSoftDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	audit.Write(ctx, pool, audit.WriteEntry{
		OrgID: orgID, Action: "create", ResourceType: "control",
	})

	// Back-date the row well beyond any window.
	_, err := pool.Exec(ctx, `
		UPDATE audit_log SET created_at = NOW() - INTERVAL '3650 days'
		WHERE org_id = $1::uuid`, orgID)
	require.NoError(t, err)

	// Upsert a retention config with AuditLogDays = 0 (disabled).
	_, err = pool.Exec(ctx, `
		INSERT INTO retention_config (org_id, audit_log_days, findings_resolved_days, notifications_days, scan_history_days, updated_at)
		VALUES ($1::uuid, 0, 0, 0, 0, $2)
		ON CONFLICT (org_id) DO UPDATE SET audit_log_days = EXCLUDED.audit_log_days`, orgID, time.Now())
	require.NoError(t, err)

	require.NoError(t, retention.RunRetention(ctx, pool, orgID))

	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE org_id = $1::uuid AND deleted_at IS NOT NULL`, orgID).Scan(&count))
	assert.Equal(t, 0, count, "AuditLogDays=0 must disable retention — no rows soft-deleted")
}
