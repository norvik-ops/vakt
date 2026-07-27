// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package rawsqlcov is the G5 gate: it PREPAREs raw-SQL statements in the
// backend against a real migrated schema.
//
// Why a coverage gate and not per-module tests: the per-module raw-SQL tests
// (vaktscan, vaktaware) each covered their own directory, which left every other
// module BLIND. The G5-outbound-ratchet had the same shape and the same failure
// — it scanned some dirs, skipped internal/modules/** entirely, and reported OK
// while four unguarded clients lived in the blind spot. This gate walks the whole
// internal/ AND cmd/ trees, so a new repository file in a never-before-checked
// package is covered the day it lands. It counts its own denominator
// (require.Greater) so an empty walk cannot pass as "clean".
//
// WHAT THIS GATE DOES NOT SEE (stated here and printed on every run, because a
// gate that does not name its blind spots invites being trusted past them):
//   - SQL assembled at runtime (string concatenation, fmt.Sprintf, query
//     builders). Those call sites are counted and reported as `Skipped`, never
//     silently dropped.
//   - Semantics. PREPARE proves a statement is valid against the schema — not
//     that it returns the right rows, scans into the right struct, or is called
//     by anyone.
//   - Anything outside internal/ and cmd/ (e.g. form-handler/, loadtest/).
package rawsqlcov

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// backendRoot returns the absolute path to backend/ from this test file's
// location (internal/rawsqlcov), independent of the working directory.
func backendRoot(t *testing.T) string {
	wd, err := os.Getwd()
	require.NoError(t, err)
	// wd is …/backend/internal/rawsqlcov
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// TestG5_RawSQLAgainstSchema PREPAREs every hand-written call-site literal under
// internal/** plus every sqlc-generated const in internal/db/*.sql.go against the
// migrated test schema. A statement Postgres cannot prepare — a dropped column, a
// renamed table, an untypeable parameter — is a query-time 500 for every caller,
// invisible to `go build`. This turns that into a red test on the pull request.
func TestG5_RawSQLAgainstSchema(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — G5 raw-SQL gate needs a migrated Postgres (CI sets it)")
	}
	root := backendRoot(t)

	// Every directory under internal/ AND cmd/ can hold raw SQL. Walk both so no
	// package is a blind spot; FromCallSites returns nothing for dirs without SQL.
	//
	// cmd/ is included deliberately: SA23-D4 was a broken statement in
	// cmd/seed/main.go, i.e. exactly the kind of defect an internal/-only walk
	// cannot see. A gate whose first question is not "what do I NOT look at" is
	// how the outbound ratchet came to report OK over four unguarded clients.
	var dirs []string
	for _, top := range []string{"internal", "cmd"} {
		walkErr := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				dirs = append(dirs, path)
			}
			return nil
		})
		require.NoError(t, walkErr, "walk %s/", top)
	}
	sort.Strings(dirs)

	callsites, err := sqlcheck.FromCallSites(dirs...)
	require.NoError(t, err)

	// sqlc-generated consts live in internal/db/*.sql.go and are executed via an
	// identifier, so FromCallSites cannot see them — extract them separately.
	genFiles, err := filepath.Glob(filepath.Join(root, "internal", "db", "*.sql.go"))
	require.NoError(t, err)
	sort.Strings(genFiles)
	gen, err := sqlcheck.FromConsts(genFiles...)
	require.NoError(t, err)

	queries := append(callsites.Queries, gen.Queries...)
	require.Greater(t, len(queries), 500,
		"far fewer statements than expected — the extractor or the walk is broken, not the code")
	t.Logf("G5: PREPAREing %d statements (%d call-site, %d sqlc-generated across %d files); "+
		"%d call site(s) build SQL at runtime and cannot be checked statically",
		len(queries), len(callsites.Queries), len(gen.Queries), len(genFiles), callsites.Skipped)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err, "connect to the migrated test database")
	defer func() { _ = conn.Close(ctx) }()

	var msgs []string
	for _, f := range sqlcheck.Prepare(ctx, conn, queries) {
		if allowed(f) {
			continue
		}
		msgs = append(msgs, f.String())
	}
	if len(msgs) > 0 {
		t.Errorf("G5: %d raw-SQL statement(s) fail PREPARE against the real schema:\n%s",
			len(msgs), strings.Join(msgs, "\n"))
	}
}

// allowedFailure is a statement PREPARE rejects for a reason that is NOT schema
// drift, documented so the exception is auditable (mirrors the check_routes.py
// ALLOWLIST discipline). Keep this empty unless there is a concrete, justified
// reason — every entry is a hole in the gate.
type allowedFailure struct {
	fileSuffix string // path suffix, e.g. "internal/foo/repository.go"
	sqlNeedle  string // a stable substring of the offending SQL
	reason     string
}

var allowlist = []allowedFailure{
	// Forward-compat with a runtime fallback: the primary INSERT references
	// nis2_anonymous_runs.meta, a column no migration creates. The code catches
	// the failure and re-inserts without meta (multiframework_service.go). It is a
	// no-op against the current schema, not a 500. Remove if a migration adds meta.
	{
		fileSuffix: "internal/shared/nis2wizard/multiframework_service.go",
		sqlNeedle:  "expires_at, meta)",
		reason:     "primary INSERT targets a not-yet-existing meta column; a runtime fallback inserts without it",
	},
	// SA23-D (deferred to S6, needs a migration — OUT of S4's file ownership):
	// these cloud collectors write a literal '' into vb_findings.asset_id, which is
	// a NOT NULL uuid — so every insert fails with 22P02 and the collector silently
	// drops the finding (log.Warn + continue). The real fix is either making
	// asset_id nullable or resolving a per-project synthetic asset; both are schema
	// work S4 does not own. Documented here so the gate stays green without hiding
	// that these two integrations are born-broken.
	{
		fileSuffix: "internal/shared/platform/integrations/cloud/gitlab_collector.go",
		sqlNeedle:  "::uuid, ''",
		reason:     "SA23-D/S6: '' into NOT NULL uuid asset_id — needs a migration",
	},
	{
		fileSuffix: "internal/shared/platform/integrations/cloud/sonarqube_collector.go",
		sqlNeedle:  "::uuid, ''",
		reason:     "SA23-D/S6: '' into NOT NULL uuid asset_id — needs a migration",
	},
}

func allowed(f sqlcheck.Failure) bool {
	// Match on the FULL SQL with whitespace collapsed — not sqlcheck.Condense,
	// which truncates at 120 chars and would drop a needle in a long VALUES clause.
	norm := strings.Join(strings.Fields(f.SQL), " ")
	for _, a := range allowlist {
		if strings.HasSuffix(filepath.ToSlash(f.File), a.fileSuffix) &&
			strings.Contains(norm, a.sqlNeedle) {
			return true
		}
	}
	return false
}
