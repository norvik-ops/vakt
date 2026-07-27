// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package sqlcheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/sqlcheck"
)

// The extractor is the load-bearing half of the schema gate: if it silently
// finds nothing, every module it guards reports a green "0 failures" while its
// SQL rots. So the extractor gets its own tests — what it must find, and what it
// must not mistake for SQL.
const source = "package p\n" + `

import "context"

func f(ctx context.Context, db, url, batch any) {
	db.Query(ctx, ` + "`SELECT id FROM sr_targets WHERE org_id = $1`" + `, org)
	db.QueryRow(ctx, ` + "`SELECT count(*) FROM sr_modules WHERE org_id = $1`" + `).Scan(&n)
	tx.Exec(ctx, ` + "`DELETE FROM sr_assignments WHERE org_id = $1 AND id = $2`" + `, id)

	// pgx.Batch.Queue: no ctx, SQL literal is argument 0, not 1.
	batch.Queue(` + "`INSERT INTO sr_events (org_id, campaign_id) VALUES ($1, $2)`" + `, org, campaign)

	// Built at runtime: unreadable statically, must be COUNTED, not dropped.
	db.Query(ctx, "SELECT "+cols+" FROM t")

	// A sqlc const executed by identifier — invisible here, covered by FromConsts.
	q.Query(ctx, listTargets, org)

	// A dynamic batch.Queue (local var, not a literal) — must be COUNTED as
	// skipped too, not silently dropped like it was before "batch" was added
	// to isSQLReceiver.
	batch.Queue(dynamicSQL, org)

	// Not SQL at all: a Query() on a non-database receiver must not inflate the
	// skip counter, or the number stops meaning anything.
	url.Query()

	// Not SQL at all either: Exec() with zero args (pgx.BatchResults.Exec()
	// reads a batch result, it does not submit one) must not be treated as a
	// SQL call just because the method name matches.
	br.Exec()
}
`

const constSource = "package db\n" + `
const listTargets = ` + "`SELECT id FROM sr_targets WHERE org_id = $1`" + `

const notSQL = "plain double-quoted string"
`

func writeGo(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestFromCallSites(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, dir, "repo.go", source)
	// A _test.go file must be ignored — test fixtures are not production SQL.
	writeGo(t, dir, "repo_test.go", source)

	res, err := sqlcheck.FromCallSites(dir)
	require.NoError(t, err)

	require.Len(t, res.Queries, 4, "the four backtick literals, and only those")
	assert.Contains(t, res.Queries[0].SQL, "FROM sr_targets")
	assert.Contains(t, res.Queries[1].SQL, "FROM sr_modules")
	assert.Contains(t, res.Queries[2].SQL, "DELETE FROM sr_assignments")
	assert.Contains(t, res.Queries[3].SQL, "INSERT INTO sr_events", "batch.Queue's SQL is argument 0, not 1 — must still be found")
	assert.NotZero(t, res.Queries[0].Line, "a failure must point at a line")

	assert.Equal(t, 3, res.Skipped,
		"the concatenated query, the sqlc-const call, and the dynamic batch.Queue are skipped; "+
			"url.Query() and br.Exec() are not SQL and must not be counted")
}

func TestFromConsts(t *testing.T) {
	dir := t.TempDir()
	path := writeGo(t, dir, "gen.go", constSource)

	res, err := sqlcheck.FromConsts(path)
	require.NoError(t, err)

	require.Len(t, res.Queries, 1, "only the backtick const is SQL")
	assert.Contains(t, res.Queries[0].SQL, "FROM sr_targets")
}

func TestCondense(t *testing.T) {
	assert.Equal(t, "SELECT a FROM t WHERE x = $1",
		sqlcheck.Condense("SELECT a\n  FROM t\n  WHERE x = $1"))

	long := sqlcheck.Condense(string(make([]byte, 0)) + longSQL())
	assert.Len(t, long, 120, "a long query is truncated so a failure list stays readable")
}

func longSQL() string {
	s := "SELECT "
	for i := 0; i < 100; i++ {
		s += "column_name, "
	}
	return s + "x FROM t"
}
