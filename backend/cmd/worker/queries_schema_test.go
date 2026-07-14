package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// TestWorkerRawSQLAgainstSchema PREPAREs every raw-SQL literal in this package
// against the migrated test DB. PREPARE validates a query against the current
// schema without executing it — this is what catches column drift like the
// is_deleted bug that filled the production Asynq retry queue on 2026-05-26.
//
// The AST extractor used to live here. It now lives in internal/shared/sqlcheck
// (S126), because the same gate was needed for vaktaware and vaktscan — the two
// modules whose born-broken queries were only ever found by a live sweep. One
// extractor, three callers.
func TestWorkerRawSQLAgainstSchema(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — skipping schema-drift test (run via CI or migrate-local first)")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect %s: %v", dbURL, err)
	}
	defer func() { _ = conn.Close(ctx) }()

	res, err := sqlcheck.FromCallSites(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Queries) == 0 {
		t.Fatal("no raw SQL literals found — extractor probably broken")
	}
	t.Logf("validating %d raw SQL queries against schema; %d call site(s) build SQL at runtime and cannot be checked statically",
		len(res.Queries), res.Skipped)

	for _, f := range sqlcheck.Prepare(ctx, conn, res.Queries) {
		t.Errorf("%s", f)
	}
}
