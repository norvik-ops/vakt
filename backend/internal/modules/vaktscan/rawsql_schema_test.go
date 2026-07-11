// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestRawSQLAgainstSchema_S121 executes the two raw-SQL queries the live route
// sweep caught 500'ing against a real migrated schema. Both bugs were column
// drift that only surfaces at query time (the compiler can't see raw SQL):
//   - ListExpiringCertificates used `($2 || ' days')::interval`, whose bound int
//     parameter pgx could not type — now make_interval(days => $2::int).
//   - GetAssetProtectionNeedID filtered on vb_assets.deleted_at, a column that
//     does not exist (soft-delete is is_deleted) — SQLSTATE 42703 for every asset.
//
// A pass here means the query is valid against the current schema; a schema
// change that breaks it turns this red instead of shipping a 500.
func TestRawSQLAgainstSchema_S121(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — raw-SQL schema test needs a migrated Postgres (set in CI)")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewRepository(pool)
	// Any UUID works — we assert the query EXECUTES (0 rows is fine), not the data.
	const org = "00000000-0000-0000-0000-0000000000aa"
	const asset = "00000000-0000-0000-0000-0000000000bb"

	_, err = repo.ListExpiringCertificates(context.Background(), org, 30)
	require.NoError(t, err, "ListExpiringCertificates must execute against the real schema")

	_, err = repo.GetAssetProtectionNeedID(context.Background(), org, asset)
	require.NoError(t, err, "GetAssetProtectionNeedID must execute against the real schema (vb_assets.is_deleted, not deleted_at)")
}
