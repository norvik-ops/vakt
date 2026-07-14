// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestVaktawareRawSQLAgainstSchema (S126) PREPAREs every raw SQL statement this
// module can reach — the backtick literals in its own repository, plus the
// sqlc-generated consts in internal/db/vaktaware.sql.go — against the migrated
// schema.
//
// vaktaware is one of the two modules that produced the most born-broken bugs,
// and every one of them was found by a human clicking through a live stack, never
// by a test: the ON CONFLICT-against-a-DEFERRABLE-unique upsert that could not
// have worked for any caller, the path drift, the missing routes. The reason is
// structural — the service holds a concrete *Repository, so no unit test ever
// reaches a query. This gate closes the schema half of that gap: it needs no
// fixtures and no service wiring, only a migrated database, and it fails on the
// pull request instead of in a customer's browser.
//
// The sqlc consts matter as much as the hand-written SQL: `sqlc generate` does
// not currently run (pre-existing drift, see CLAUDE.md), so those files are
// hand-maintained and nothing else validates them against the schema.
func TestVaktawareRawSQLAgainstSchema(t *testing.T) {
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
	gen, err := sqlcheck.FromConsts("../../db/vaktaware.sql.go")
	require.NoError(t, err)

	queries := append(own.Queries, gen.Queries...)
	require.NotEmpty(t, queries, "no SQL found — the extractor is broken, not the module")
	t.Logf("PREPAREing %d statements (%d hand-written, %d sqlc-generated); %d call site(s) build SQL at runtime and cannot be checked statically",
		len(queries), len(own.Queries), len(gen.Queries), own.Skipped)

	for _, f := range sqlcheck.Prepare(ctx, conn, queries) {
		t.Errorf("%s", f)
	}
}
