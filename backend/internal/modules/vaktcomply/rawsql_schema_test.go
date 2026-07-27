// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestVaktcomplyRawSQLAgainstSchema (S135/GB-1) closes the largest gap in the
// raw-SQL schema gate: vaktcomply is by far the biggest module by surface
// area (catalogs, bsi, policy, bcm, reporting, audit, risk — plus the package
// root) and, until this test, had NO raw-SQL PREPARE coverage at all. GB-1
// (Codeaudit v4) named it explicitly: "Raw-SQL-Gate deckt vaktcomply ...
// nicht -> SA23-Klasse unsichtbar bis Laufzeit". SA23-D1/D2/D3 (missing
// column, untyped interval concat) all happened in modules this class of
// gate did not reach.
//
// vaktcomply is not a single flat package — sqlcheck.FromCallSites is
// per-directory (it only reads the .go files directly in a dir, not
// subpackages), so every subpackage that carries its own repository files
// must be listed explicitly. A subpackage added later without updating this
// list would silently stop being checked — same class of bug this whole gate
// exists to catch — so keep this list in sync with `find . -maxdepth 1
// -type d` under internal/modules/vaktcomply.
func TestVaktcomplyRawSQLAgainstSchema(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — raw-SQL schema test needs a migrated Postgres (CI sets it)")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err, "connect to the migrated test database")
	defer func() { _ = conn.Close(ctx) }()

	own, err := sqlcheck.FromCallSites(
		".",
		"catalogs",
		"bsi",
		"policy",
		"policy/catalogs",
		"bcm",
		"reporting",
		"audit",
		"risk",
	)
	require.NoError(t, err)
	gen, err := sqlcheck.FromConsts("../../db/vaktcomply.sql.go")
	require.NoError(t, err)

	queries := append(own.Queries, gen.Queries...)
	require.NotEmpty(t, queries, "no SQL found — the extractor is broken, not the module")
	t.Logf("PREPAREing %d statements (%d hand-written, %d sqlc-generated); %d call site(s) build SQL at runtime and cannot be checked statically",
		len(queries), len(own.Queries), len(gen.Queries), own.Skipped)

	for _, f := range sqlcheck.Prepare(ctx, conn, queries) {
		t.Errorf("%s", f)
	}
}
