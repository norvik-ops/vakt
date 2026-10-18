// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestRawSQLAgainstSchema_S121 executes a raw-SQL query the live route sweep
// caught 500'ing against a real migrated schema: ListExpiringCertificates used
// `($2 || ' days')::interval`, whose bound int parameter pgx could not type —
// now make_interval(days => $2::int). It runs for real (not just PREPARE)
// because this is an execute-time type error. The second query of the original
// pair, GetAssetProtectionNeedID, moved with the asset inventory to
// vaktcomply/assets (rawsql_schema_test.go there).
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

	_, err = repo.ListExpiringCertificates(context.Background(), org, 30)
	require.NoError(t, err, "ListExpiringCertificates must execute against the real schema")
}

// TestVaktscanRawSQLAgainstSchema (S126) widens the gate above from the two known
// bugs to every statement the module can reach: the backtick literals across its
// seven repository files, plus the sqlc-generated consts in
// internal/db/vaktscan.sql.go. PREPARE validates each against the current schema
// without executing it, so no fixtures are needed.
//
// The two-query version was a regression test — it only ever proved the bugs we
// had already been bitten by stayed fixed. It could not have found the next one.
// This finds the next one, on the pull request, for the whole module.
func TestVaktscanRawSQLAgainstSchema(t *testing.T) {
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
	gen, err := sqlcheck.FromConsts("../../db/vaktscan.sql.go")
	require.NoError(t, err)

	queries := append(own.Queries, gen.Queries...)
	require.NotEmpty(t, queries, "no SQL found — the extractor is broken, not the module")
	t.Logf("PREPAREing %d statements (%d hand-written, %d sqlc-generated); %d call site(s) build SQL at runtime and cannot be checked statically",
		len(queries), len(own.Queries), len(gen.Queries), own.Skipped)

	for _, f := range sqlcheck.Prepare(ctx, conn, queries) {
		t.Errorf("%s", f)
	}
}
