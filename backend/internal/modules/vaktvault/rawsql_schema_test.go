// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestVaktvaultRawSQLAgainstSchema (S135/GB-1) closes one of the five gaps
// named in the audit's GB-1 finding: the raw-SQL schema gate covered only
// vaktscan and vaktaware, so a dropped column or an untyped interval
// concatenation anywhere in vaktvault (secrets storage, git-leak scanning,
// access review) would break at query time in production with no PR-time
// signal. Also surfaces the git-scan batch-insert in repository.go
// (pgx.Batch.Queue) as an honest skip instead of a silent zero: its SQL is a
// function-local const referenced by identifier, not a literal, so it cannot
// be PREPAREd statically — but before the sqlcheck Queue-argument fix in this
// same sprint it was invisible to the extractor entirely, not even counted.
func TestVaktvaultRawSQLAgainstSchema(t *testing.T) {
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
	gen, err := sqlcheck.FromConsts("../../db/vaktvault.sql.go")
	require.NoError(t, err)

	queries := append(own.Queries, gen.Queries...)
	require.NotEmpty(t, queries, "no SQL found — the extractor is broken, not the module")
	t.Logf("PREPAREing %d statements (%d hand-written, %d sqlc-generated); %d call site(s) build SQL at runtime and cannot be checked statically",
		len(queries), len(own.Queries), len(gen.Queries), own.Skipped)

	for _, f := range sqlcheck.Prepare(ctx, conn, queries) {
		t.Errorf("%s", f)
	}
}
