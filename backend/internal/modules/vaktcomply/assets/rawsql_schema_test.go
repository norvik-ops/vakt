// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestGetAssetProtectionNeedIDAgainstSchema (S121, moved from vaktscan with the
// inventory) executes the query for real: it filtered on vb_assets.deleted_at, a
// column that does not exist (soft-delete is is_deleted) — SQLSTATE 42703 for
// every asset, found by the live route sweep (docs/lessons.md). PREPARE alone
// would not have caught it, so the statement runs.
func TestGetAssetProtectionNeedIDAgainstSchema(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — raw-SQL schema test needs a migrated Postgres (set in CI)")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer pool.Close()

	// Any UUID works — we assert the query EXECUTES (0 rows is fine), not the data.
	_, err = NewRepository(pool).GetAssetProtectionNeedID(context.Background(),
		"00000000-0000-0000-0000-0000000000aa", "00000000-0000-0000-0000-0000000000bb")
	require.NoError(t, err, "GetAssetProtectionNeedID must execute against the real schema (vb_assets.is_deleted, not deleted_at)")
}

// TestAssetsRawSQLAgainstSchema PREPAREs every hand-written statement of the
// package against the migrated schema (same gate as vaktscan's S126 test). The
// sqlc consts stay covered there until the queries move in S5.
func TestAssetsRawSQLAgainstSchema(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — raw-SQL schema test needs a migrated Postgres (CI sets it)")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err, "connect to the migrated test database")
	defer func() { _ = conn.Close(ctx) }()

	own, err := sqlcheck.FromCallSites(".")
	require.NoError(t, err)
	require.NotEmpty(t, own.Queries, "no SQL found — the extractor is broken, not the package")
	for _, f := range sqlcheck.Prepare(ctx, conn, own.Queries) {
		t.Errorf("%s", f)
	}
}
