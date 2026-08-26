// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package sqlcheck_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// These tests need a real Postgres because the defect they pin down does not
// exist anywhere else: PREPARE and EXPLAIN disagree about the same statement,
// and only the server can be asked which one is right.
//
// Everything happens on TEMP tables. A temp table lives in the session's own
// schema and dies with the connection, so this can run against the shared CI
// database without touching a single row anyone else can see.
func conn(t *testing.T) (context.Context, *pgx.Conn) {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — the oracle tests need a Postgres")
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close(ctx) })
	return ctx, c
}

// partialIndexFixture mirrors migration 120: a UNIQUE index that is PARTIAL,
// because the dedup columns may be NULL and several NULLs must be allowed.
// An ON CONFLICT clause can only use such an index as its arbiter if it repeats
// the index predicate — without it Postgres refuses with 42P10.
func partialIndexFixture(t *testing.T, ctx context.Context, c *pgx.Conn) {
	t.Helper()
	_, err := c.Exec(ctx, `CREATE TEMP TABLE sqlcheck_findings (
		org_id  uuid NOT NULL,
		raw_id  text,
		scanner text NOT NULL,
		title   text NOT NULL
	)`)
	require.NoError(t, err)
	_, err = c.Exec(ctx, `CREATE UNIQUE INDEX sqlcheck_findings_dedup
		ON sqlcheck_findings (org_id, raw_id, scanner) WHERE raw_id IS NOT NULL`)
	require.NoError(t, err)
}

const upsertAgainstPartialIndex = `INSERT INTO sqlcheck_findings (org_id, raw_id, scanner, title)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, raw_id, scanner) DO UPDATE SET title = EXCLUDED.title`

// TestCheckCatchesOnConflictWithoutMatchingArbiter is the regression test for
// R1-SA23-02. UpsertSPFindingByRawID (db/queries/vaktscan.sql) targets exactly
// this shape: ON CONFLICT against a PARTIAL unique index, without repeating the
// predicate. PREPARE accepts it — arbiter inference happens when the statement
// is planned, not when it is parsed — so the G5 gate reported PASS over 1424
// statements while this one raised 42P10 for every caller that ran it.
//
// Without the second oracle this test fails: Prepare returns zero failures.
func TestCheckCatchesOnConflictWithoutMatchingArbiter(t *testing.T) {
	ctx, c := conn(t)
	partialIndexFixture(t, ctx, c)

	q := sqlcheck.Query{File: "vaktscan.sql.go", Line: 1641, SQL: upsertAgainstPartialIndex}

	// First: prove PREPARE alone is blind to it. If this ever starts failing,
	// the premise of the whole fix changed and the rest of the test is moot.
	_, err := c.Prepare(ctx, "sqlcheck_blindspot", q.SQL)
	require.NoError(t, err, "PREPARE is expected to accept the broken statement — that is the defect")
	require.NoError(t, c.Deallocate(ctx, "sqlcheck_blindspot"))

	failures := sqlcheck.Prepare(ctx, c, []sqlcheck.Query{q})
	require.Len(t, failures, 1,
		"the gate must reject an ON CONFLICT whose arbiter no unique index matches — "+
			"PREPARE says yes, the planner says 42P10")
	assert.Contains(t, failures[0].Err.Error(), "42P10")
	assert.Contains(t, failures[0].String(), "vaktscan.sql.go:1641",
		"a failure has to point at the line to fix")
}

// TestCheckAcceptsHealthyStatements is the false-positive half. A gate that
// goes red on a healthy repository gets switched off instead of fixed, so the
// second oracle has to stay quiet for: an ON CONFLICT that does repeat the
// partial index predicate, plain parameterised statements (which pass only
// because of the GENERIC_PLAN option — without it every $N raises 42P02, 1333
// of this repository's 1424), and a statement kind EXPLAIN cannot parse at all.
func TestCheckAcceptsHealthyStatements(t *testing.T) {
	ctx, c := conn(t)
	partialIndexFixture(t, ctx, c)

	// The repaired shape, kept beside the broken one so the test documents the
	// actual fix and not just the rejection. Note WHERE the predicate goes:
	// `ON CONFLICT (...) WHERE ... DO UPDATE`. Appended to the end instead, it
	// is the DO UPDATE's own row filter and the arbiter stays unmatched — a
	// plausible-looking repair that still raises 42P10.
	const repaired = `INSERT INTO sqlcheck_findings (org_id, raw_id, scanner, title)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, raw_id, scanner) WHERE raw_id IS NOT NULL
DO UPDATE SET title = EXCLUDED.title`

	queries := []sqlcheck.Query{
		{File: "a.go", Line: 1, SQL: repaired},
		{File: "b.go", Line: 2, SQL: `SELECT title FROM sqlcheck_findings WHERE org_id = $1 AND scanner = $2`},
		{File: "c.go", Line: 3, SQL: `UPDATE sqlcheck_findings SET title = $2 WHERE org_id = $1`},
		{File: "d.go", Line: 4, SQL: `SET search_path = public`},
	}
	res := sqlcheck.Check(ctx, c, queries)
	assert.Empty(t, res.Failures, "no healthy statement may be flagged")
	assert.Equal(t, 4, res.Checked)
	assert.Equal(t, 3, res.Planned, "SET is not a plannable statement")
	assert.Equal(t, 1, res.PlanSkipped, "and the one that is not must be COUNTED, not dropped")
	assert.Contains(t, res.Summary(), "plan-skipped: 1",
		"the summary has to name its own blind spot")
}

// TestGenericPlanIsWhatCarriesTheParameters exists because the comment on
// explainGenericPlan makes a checkable claim, and a claim at a protective spot
// that the next reader cannot verify gets treated as superstition and removed.
// So the claim is a test: without GENERIC_PLAN, a parameterised statement — the
// shape of 1333 of this repository's 1424 — is rejected with 42P02, and with it
// the same statement plans fine. Dropping the option is therefore not a
// simplification, it is 1333 false failures.
func TestGenericPlanIsWhatCarriesTheParameters(t *testing.T) {
	ctx, c := conn(t)
	partialIndexFixture(t, ctx, c)
	const parameterised = `SELECT title FROM sqlcheck_findings WHERE org_id = $1 AND scanner = $2`

	_, err := c.PgConn().Exec(ctx, "EXPLAIN (COSTS OFF)\n"+parameterised).ReadAll()
	require.Error(t, err, "plain EXPLAIN wants values for $1/$2")
	assert.Contains(t, err.Error(), "42P02")

	_, err = c.PgConn().Exec(ctx, "EXPLAIN (GENERIC_PLAN, COSTS OFF)\n"+parameterised).ReadAll()
	assert.NoError(t, err, "GENERIC_PLAN is the option that makes unbound parameters plannable")

	// And the transport is not what carries it: the extended-protocol path
	// returns the same verdict today. It is not the reason PgConn().Exec was
	// chosen — see explainGenericPlan — but claiming otherwise would be the
	// same kind of unverifiable comment this test exists to prevent.
	_, err = c.Exec(ctx, "EXPLAIN (GENERIC_PLAN, COSTS OFF)\n"+parameterised)
	assert.NoError(t, err)
}

// TestPlanOracleBlindSpotIsDocumented pins the honest limit of the fix: a
// DEFERRABLE unique constraint used as an ON CONFLICT arbiter is rejected by
// the EXECUTOR, and neither PREPARE nor EXPLAIN (GENERIC_PLAN) sees it. This is
// not hypothetical — CLAUDE.md records UpsertSRAssignment dying exactly this
// way. No live query has this shape today, so the gate stays silent, and the
// test exists so nobody later reads the package doc as "both classes covered".
func TestPlanOracleBlindSpotIsDocumented(t *testing.T) {
	ctx, c := conn(t)
	_, err := c.Exec(ctx, `CREATE TEMP TABLE sqlcheck_deferrable (
		a int, b int, v text,
		UNIQUE (a, b) DEFERRABLE INITIALLY DEFERRED
	)`)
	require.NoError(t, err)

	q := sqlcheck.Query{File: "x.go", Line: 1, SQL: `INSERT INTO sqlcheck_deferrable (a, b, v)
		VALUES ($1, $2, $3) ON CONFLICT (a, b) DO UPDATE SET v = EXCLUDED.v`}

	assert.Empty(t, sqlcheck.Check(ctx, c, []sqlcheck.Query{q}).Failures,
		"both oracles pass this — documented blind spot, not an accident")

	// And the proof that it really is broken: executing it raises the error
	// neither oracle produced.
	_, err = c.Exec(ctx, q.SQL, 1, 2, "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deferrable",
		"if this stops failing, Postgres changed and the blind spot can be closed")
}
