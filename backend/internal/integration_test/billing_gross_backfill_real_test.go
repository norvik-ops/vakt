//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// runSQLScript executes a multi-statement SQL file (comments included) through the
// simple query protocol, the way a migration runner would. pgxpool.Exec uses the
// extended protocol, which rejects multiple commands in one string.
func runSQLScript(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Conn().PgConn().Exec(ctx, sql).ReadAll()
	require.NoError(t, err)
}

// TestMigration246_RepairsBrokenGrossBackfill is the regression guard for the
// billing timebomb R-C02 (S131-0).
//
// Migration 244 added gross_amount_cents with `ADD COLUMN ... DEFAULT 0` and then
// tried to backfill existing rows with
//
//	UPDATE billing_invoices SET gross_amount_cents = COALESCE(gross_amount_cents, net_amount_cents)
//
// That backfill was a no-op: `ADD COLUMN ... DEFAULT 0` materialises the column as
// 0 (not NULL) for every existing row, so COALESCE(0, net) stays 0. The subsequent
// CHECK (gross = net + tax) then breaks with 23514 for any row where net != 0,
// leaving schema_migrations dirty. Prod only survived because billing_invoices is
// empty there.
//
// Migration 246 repairs this: drop the constraint, recompute gross = net + tax for
// every row, re-add the constraint. This test reproduces the exact post-244 broken
// state (gross forced to 0 on a row with net != 0) and asserts 246 heals it and the
// invariant holds again.
func TestMigration246_RepairsBrokenGrossBackfill(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	// A subscription (FK target) and one invoice with a real, non-zero net amount.
	var subID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_quote_requests (company_name, email, approval_token_hash)
		VALUES ('Acme GmbH', 'billing@acme.example', 'hash')
		RETURNING id`).Scan(&subID))

	var invID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO billing_invoices
		    (subscription_id, lexware_invoice_id, period_start, period_end,
		     net_amount_cents, tax_amount_cents, gross_amount_cents)
		VALUES ($1, 'LX-1', '2026-01-01', '2026-02-01', 29900, 0, 29900)
		RETURNING id`, subID).Scan(&invID))

	// Reproduce the exact broken state migration 244 left on a populated table:
	// the CHECK gone (244 could not add it) and gross wrongly at 0.
	_, err := pool.Exec(ctx, `ALTER TABLE billing_invoices DROP CONSTRAINT billing_invoices_amounts_consistent`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE billing_invoices SET gross_amount_cents = 0`)
	require.NoError(t, err)

	// Apply migration 246 up exactly as it ships. The file may contain multiple
	// statements + comments, so run it through the simple query protocol.
	up, err := os.ReadFile(filepath.Join(migrationsDir(t), "246_billing_gross_backfill_fix.up.sql"))
	require.NoError(t, err)
	runSQLScript(t, ctx, pool, string(up))

	// gross is healed to net + tax.
	var gross, net, tax int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT gross_amount_cents, net_amount_cents, tax_amount_cents FROM billing_invoices WHERE id = $1`, invID).
		Scan(&gross, &net, &tax))
	require.Equal(t, int64(29900), net)
	require.Equal(t, int64(0), tax)
	require.Equal(t, net+tax, gross, "246 must recompute gross = net + tax")

	// The invariant is enforced again: a violating row is rejected.
	_, err = pool.Exec(ctx, `
		INSERT INTO billing_invoices
		    (subscription_id, lexware_invoice_id, period_start, period_end,
		     net_amount_cents, tax_amount_cents, gross_amount_cents)
		VALUES ($1, 'LX-2', '2026-01-01', '2026-02-01', 29900, 0, 0)`, subID)
	require.Error(t, err, "CHECK must reject gross != net + tax after 246")
	require.Contains(t, err.Error(), "billing_invoices_amounts_consistent")

	// down/up round-trips cleanly (schema-only; keeps the repaired data).
	down, err := os.ReadFile(filepath.Join(migrationsDir(t), "246_billing_gross_backfill_fix.down.sql"))
	require.NoError(t, err)
	runSQLScript(t, ctx, pool, string(down))
	runSQLScript(t, ctx, pool, string(up))

	var stillGross int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT gross_amount_cents FROM billing_invoices WHERE id = $1`, invID).Scan(&stillGross))
	require.Equal(t, int64(29900), stillGross)
}
