// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package retention_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestRetentionRawSQLAgainstSchema (S135/GB-1) closes the last of the five
// gaps named in the audit's GB-1 finding: the raw-SQL schema gate covered
// only vaktscan and vaktaware, so a dropped column or an untyped interval
// concatenation in the retention job (audit_log/findings/notifications/scan
// history soft-delete, run against every org's retention_config on a cron)
// would break at query time in production with no PR-time signal — and this
// is exactly the package that already carries the documented lesson about
// `($2 || ' days')::interval` needing an explicit cast (see sqlAuditLogSoftDelete
// in service.go), so it is a natural place for the class to regress unnoticed.
//
// sqlAuditLogSoftDelete is a package-level const (not a call-site literal),
// so it is only reachable via FromConsts — FromCallSites alone would silently
// skip it as a dynamic identifier.
func TestRetentionRawSQLAgainstSchema(t *testing.T) {
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
	gen, err := sqlcheck.FromConsts("service.go")
	require.NoError(t, err)

	queries := append(own.Queries, gen.Queries...)
	require.NotEmpty(t, queries, "no SQL found — the extractor is broken, not the package")
	t.Logf("PREPAREing %d statements (%d hand-written, %d package-level const); %d call site(s) build SQL at runtime and cannot be checked statically",
		len(queries), len(own.Queries), len(gen.Queries), own.Skipped)

	for _, f := range sqlcheck.Prepare(ctx, conn, queries) {
		t.Errorf("%s", f)
	}
}
