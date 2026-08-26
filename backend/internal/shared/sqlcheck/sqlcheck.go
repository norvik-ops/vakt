// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package sqlcheck extracts raw SQL literals from Go source and validates them
// against a live schema with two oracles: PREPARE and EXPLAIN (GENERIC_PLAN).
//
// Raw SQL is invisible to the compiler. A dropped column, a renamed table or a
// parameter Postgres cannot type does not break `go build`, does not break a
// unit test that stops before the first repository call, and does not break a
// contract test — it breaks at query time, for every caller, forever. The S121
// live sweep found ~46 such 500s: vb_assets.deleted_at (the column is
// is_deleted), ck_controls.updated_at (it is last_reviewed_at),
// ($2 || ' days')::interval (pgx cannot type the bound int).
//
// cmd/worker has carried an ad-hoc version of this gate since 2026-05-26. This
// package is that gate, generalized (S126): any package can point it at its own
// sources and turn schema drift into a red test instead of a production 500.
//
// # Why two oracles, and what each one is blind to
//
// PREPARE parses and analyses a statement against the catalog. That is enough
// for the drift class above, and for a long time it was the only check here —
// which is how R1-SA23-02 got past it. UpsertSPFindingByRawID says
// `ON CONFLICT (org_id, raw_id, scanner)` while migration 120 provides only a
// PARTIAL unique index (`WHERE raw_id IS NOT NULL`). Arbiter inference for
// ON CONFLICT happens when the statement is PLANNED, not when it is parsed, so
// PREPARE returned success while every caller got 42P10. Measured against
// Postgres 16.14: PREPARE succeeds, EXECUTE fails.
//
// EXPLAIN (GENERIC_PLAN, COSTS OFF) (Postgres 16+) plans a statement whose
// parameters are still unbound and does not execute it — so it reaches the
// planner stage without touching a row. Measured on the same statement: it
// fails with 42P10, i.e. it sees exactly what PREPARE misses.
//
// Neither oracle sees a third class: an arbiter that exists and matches but is
// DEFERRABLE. Postgres rejects that in the EXECUTOR — measured on 16.14,
// PREPARE succeeds AND EXPLAIN (GENERIC_PLAN) returns a plan, while EXECUTE
// fails with "ON CONFLICT does not support deferrable unique constraints ... as
// arbiters". CLAUDE.md records UpsertSRAssignment dying exactly that way. No
// live query has that shape today; the gate is silent about it, so it is
// written down here rather than left for the next person to discover the hard
// way. Closing it would require executing statements in a rolled-back
// transaction, which is a different gate with different costs.
//
// Both oracles are read-only: PREPARE does not execute, and EXPLAIN without
// ANALYZE does not execute. See Check for the two conditions that keep the
// second oracle from executing anything by accident.
package sqlcheck

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Query is one raw SQL literal found in the source, with its origin so a failure
// points at the line to fix rather than at the SQL alone.
type Query struct {
	File string
	Line int
	SQL  string
}

// Result is what an extraction found: the queries it can check, and how many
// call sites it had to skip because their SQL is built at runtime.
//
// Skipped is not decoration. A gate that silently drops the inputs it cannot
// parse reports success for work it did not do — check_routes.py quietly skipped
// a quarter of the frontend and still printed OK. Callers are expected to log
// Skipped alongside the pass, so "green" never overstates its own reach.
type Result struct {
	Queries []Query
	Skipped int
}

// Failure is a Query an oracle rejected, with the server's reason. Oracle names
// which one spoke, because the two answer different questions and a reader who
// does not know which one failed cannot tell a schema drift ("column does not
// exist", PREPARE) from a planner-stage defect ("no unique or exclusion
// constraint matching the ON CONFLICT specification", GENERIC_PLAN).
type Failure struct {
	Query
	Oracle string
	Err    error
}

func (f Failure) String() string {
	where := "sqlcheck"
	if f.File != "" {
		where = fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
	}
	oracle := f.Oracle
	if oracle == "" {
		oracle = oraclePrepare
	}
	if f.SQL == "" {
		return fmt.Sprintf("%s: %s failed: %v", where, oracle, f.Err)
	}
	return fmt.Sprintf("%s: %s failed: %v\n    SQL: %s",
		where, oracle, f.Err, Condense(f.SQL))
}

const (
	oraclePrepare = "PREPARE"
	oraclePlan    = "EXPLAIN (GENERIC_PLAN)"
)

// sqlMethods maps pgx call names that carry a SQL literal to the argument
// index holding it. Query/QueryRow/Exec have signature (ctx, sql, args...) —
// the literal is argument 1. pgx.Batch.Queue has signature (sql, args...) —
// no context parameter — so the literal is argument 0. Without this entry,
// batch.Queue(`INSERT ...`, args...) literals (vaktvault, vaktscan SBOM
// batch-upsert, vaktcomply SoA-entry batch-insert) were invisible to every
// extractor in this package: not counted as a Query, not counted as Skipped
// either, because the old `sqlMethods[sel.Sel.Name]` boolean check returned
// before Queue's receiver was ever looked at — GB-1/G-07's "silent skip"
// class, living inside the gate itself.
var sqlMethods = map[string]int{"Query": 1, "QueryRow": 1, "Exec": 1, "Queue": 0}

// FromCallSites returns every backtick SQL literal passed as the second argument
// to a Query/QueryRow/Exec call in the non-test .go files of the given dirs.
//
// Only backtick strings count: they are the SQL convention here, and the
// restriction filters out unrelated double-quoted "Query" selectors (HTTP query
// params, struct fields). SQL built at runtime (fmt.Sprintf, concatenation)
// cannot be validated statically and is counted in Result.Skipped instead of
// being dropped on the floor.
func FromCallSites(dirs ...string) (Result, error) {
	var res Result
	for _, dir := range dirs {
		files, err := goFiles(dir)
		if err != nil {
			return Result{}, err
		}
		for _, file := range files {
			if err := callSitesIn(file, &res); err != nil {
				return Result{}, err
			}
		}
	}
	return res, nil
}

// FromConsts returns every backtick string const declared in the given files.
//
// This is how sqlc-generated code stores its SQL: `const listFindings = ` plus a
// backtick literal, executed later as `q.db.Query(ctx, listFindings, ...)`.
// FromCallSites cannot see those — the call site's second argument is an
// identifier, not a literal — so a generated query with a stale column sails
// straight past a call-site-only gate. That matters more here than it would
// elsewhere: `sqlc generate` does not currently run (a pre-existing drift breaks
// it, see CLAUDE.md), so the committed .sql.go files are hand-maintained and
// nothing else checks them against the schema.
func FromConsts(files ...string) (Result, error) {
	var res Result
	fset := token.NewFileSet()
	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return Result{}, fmt.Errorf("parse %s: %w", file, err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, val := range vs.Values {
					lit, ok := val.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING || !strings.HasPrefix(lit.Value, "`") {
						continue
					}
					res.Queries = append(res.Queries, Query{
						File: fset.Position(lit.Pos()).Filename,
						Line: fset.Position(lit.Pos()).Line,
						SQL:  strings.Trim(lit.Value, "`"),
					})
				}
			}
		}
	}
	return res, nil
}

// CheckResult is what Check found, plus the denominators behind it.
//
// Planned and PlanSkipped exist for the same reason Result.Skipped does: a gate
// that reports "0 failures" without saying how much it looked at overstates its
// own reach. Checked is the PREPARE denominator (every query), Planned the
// GENERIC_PLAN one — the two differ, and the difference is a blind spot that
// has to be visible, not inferred.
type CheckResult struct {
	Failures    []Failure
	Checked     int
	Planned     int
	PlanSkipped int
	// PlanSkipReasons counts why statements were not planned, so the number is
	// explicable and not just a shrug.
	PlanSkipReasons map[string]int
}

// Summary is the one line to print next to a pass, so "green" arrives with its
// denominator attached.
//
// Stated plainly because the opposite is easy to assume: as of this commit NO
// caller prints it. All nine go through Prepare and drop the CheckResult, so the
// G5 log still reads "PREPAREing 1424 statements" while both oracles run. Today
// that hides nothing — the plan oracle covers the whole corpus, plan-skipped is
// 0 — but the moment one statement is skipped, the number that says so exists
// and is not on screen. Wiring it up belongs to each caller (rawsqlcov,
// cmd/worker, retention, the six module tests), which this package does not own:
// swap Prepare for Check and log Summary alongside Result.Skipped.
func (r CheckResult) Summary() string {
	var reasons []string
	for _, k := range sortedKeys(r.PlanSkipReasons) {
		reasons = append(reasons, fmt.Sprintf("%s=%d", k, r.PlanSkipReasons[k]))
	}
	s := fmt.Sprintf("sqlcheck: prepared: %d, planned: %d, plan-skipped: %d",
		r.Checked, r.Planned, r.PlanSkipped)
	if len(reasons) > 0 {
		s += " (" + strings.Join(reasons, ", ") + ")"
	}
	return s
}

// Prepare validates every query and returns the ones a server oracle rejected.
//
// The name is historical: it is the entry point nine call sites already use
// (rawsqlcov's G5 gate, cmd/worker, retention, six module tests), and it now
// runs both oracles, so those gates gained the ON CONFLICT class without a
// change on their side. New callers should use Check, whose result also carries
// the denominators worth printing.
func Prepare(ctx context.Context, conn *pgx.Conn, queries []Query) []Failure {
	return Check(ctx, conn, queries).Failures
}

// Check runs PREPARE for every query, and EXPLAIN (GENERIC_PLAN, COSTS OFF) for
// every query that survived it and that EXPLAIN can accept. Each prepared
// statement is deallocated again, so the connection is left as it was found and
// no transaction of the caller's is touched.
//
// Two conditions decide whether the second oracle runs, and both are load-
// bearing rather than cosmetic:
//
//   - PREPARE must have succeeded. Beyond avoiding a second complaint about a
//     statement that is already reported, this is what makes the second oracle
//     safe: Postgres refuses to prepare a text holding several commands
//     ("cannot insert multiple commands into a prepared statement"), so such a
//     text never reaches EXPLAIN. It matters — measured on 16.14, EXPLAIN
//     (GENERIC_PLAN) SELECT 1; SELECT 2 plans the first command and EXECUTES
//     the second.
//   - The statement must be plannable. EXPLAIN only accepts SELECT/INSERT/
//     UPDATE/DELETE/MERGE/VALUES/TABLE/WITH; measured on 16.14, prefixing it to
//     SET, LISTEN, COPY or CREATE yields a syntax error — a false positive that
//     would turn the gate red over healthy code, and a gate that goes red on a
//     healthy repository gets switched off instead of fixed. Everything else is
//     counted in PlanSkipped, never dropped silently.
//
// If the server cannot do GENERIC_PLAN at all (Postgres < 16), Check reports one
// explicit failure instead of quietly degrading to the old, blinder check.
func Check(ctx context.Context, conn *pgx.Conn, queries []Query) CheckResult {
	res := CheckResult{Checked: len(queries), PlanSkipReasons: map[string]int{}}

	planOK, probeErr := planOracleAvailable(ctx, conn)
	if !planOK {
		res.Failures = append(res.Failures, Failure{
			Oracle: oraclePlan,
			Err: fmt.Errorf("this server cannot run EXPLAIN (GENERIC_PLAN) (Postgres 16+ required), "+
				"so the ON CONFLICT/planner class of defects is NOT checked here: %w", probeErr),
		})
	}

	for i, q := range queries {
		name := fmt.Sprintf("sqlcheck_%d", i)
		if _, err := conn.Prepare(ctx, name, q.SQL); err != nil {
			res.Failures = append(res.Failures, Failure{Query: q, Oracle: oraclePrepare, Err: err})
			res.PlanSkipped++
			res.PlanSkipReasons["prepare-failed"]++
			continue
		}
		_ = conn.Deallocate(ctx, name)

		if !planOK {
			res.PlanSkipped++
			res.PlanSkipReasons["no-generic-plan-support"]++
			continue
		}
		if !plannable(q.SQL) {
			res.PlanSkipped++
			res.PlanSkipReasons["not-a-plannable-statement"]++
			continue
		}
		res.Planned++
		if err := explainGenericPlan(ctx, conn, q.SQL); err != nil {
			res.Failures = append(res.Failures, Failure{Query: q, Oracle: oraclePlan, Err: err})
		}
	}
	return res
}

// explainGenericPlan asks the planner about a statement whose $N placeholders
// are still unbound.
//
// GENERIC_PLAN is the load-bearing part, not a refinement. Measured over this
// repository's 1424 statements against Postgres 16.14: with the option, 1
// failure (the real one); without it, 1333 failures, every single one "there is
// no parameter $N" (SQLSTATE 42P02) — plain EXPLAIN wants values, and nearly
// every statement here is parameterised. Whoever finds the option superfluous
// and drops it turns the gate red over 1333 healthy statements, and a gate that
// is red on a healthy repository gets switched off instead of fixed.
//
// PgConn().Exec — the simple query protocol — is a deliberate choice for the
// transport, but NOT because the alternatives were measured to break: over the
// same 1424 statements, conn.Exec with no arguments and
// QueryExecModeSimpleProtocol both return the identical single failure. The
// reason is that they reach that harmless result through a pgx decision nobody
// promised: with zero arguments there is nothing to bind. Pass this path an
// argument, or let pgx treat the empty-argument case differently, and a BIND
// step appears — and a bound parameter is precisely what GENERIC_PLAN must not
// see. PgConn().Exec has no BIND step at all, so the property holds by protocol
// instead of by coincidence.
//
// The newline after the options is not decoration: a SQL literal may start with
// a `-- name: X :one` comment line (that is how sqlc writes them), and on one
// line the comment would swallow the statement.
func explainGenericPlan(ctx context.Context, conn *pgx.Conn, sql string) error {
	_, err := conn.PgConn().Exec(ctx, "EXPLAIN (GENERIC_PLAN, COSTS OFF)\n"+sql).ReadAll()
	return err
}

// planOracleAvailable probes the capability instead of computing it from a
// version string — the server is the only authority on what it accepts, and a
// version comparison would also have to guess about forks and backports.
func planOracleAvailable(ctx context.Context, conn *pgx.Conn) (bool, error) {
	if err := explainGenericPlan(ctx, conn, "SELECT $1::int"); err != nil {
		return false, err
	}
	return true, nil
}

// plannable reports whether EXPLAIN accepts this statement kind. Leading
// comments and an opening parenthesis (`(SELECT ...) UNION (SELECT ...)`) are
// skipped to find the keyword that decides it.
func plannable(sql string) bool {
	switch firstKeyword(sql) {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "WITH", "VALUES", "TABLE":
		return true
	}
	return false
}

func firstKeyword(sql string) string {
	inBlockComment := false
	for _, line := range strings.Split(sql, "\n") {
		l := strings.TrimSpace(line)
		for inBlockComment {
			end := strings.Index(l, "*/")
			if end < 0 {
				l = ""
				break
			}
			inBlockComment = false
			l = strings.TrimSpace(l[end+2:])
		}
		for strings.HasPrefix(l, "/*") {
			end := strings.Index(l, "*/")
			if end < 0 {
				inBlockComment = true
				l = ""
				break
			}
			l = strings.TrimSpace(l[end+2:])
		}
		if l == "" || strings.HasPrefix(l, "--") {
			continue
		}
		fields := strings.Fields(strings.TrimLeft(l, "("))
		if len(fields) == 0 {
			continue
		}
		return strings.ToUpper(fields[0])
	}
	return ""
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func callSitesIn(file string, res *Result) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		argIdx, known := sqlMethods[sel.Sel.Name]
		if !known || len(call.Args) <= argIdx {
			return true
		}
		lit, ok := call.Args[argIdx].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || !strings.HasPrefix(lit.Value, "`") {
			// A Query/Exec call whose SQL is an identifier or an expression:
			// either a sqlc const (covered by FromConsts) or SQL assembled at
			// runtime. Either way this extractor cannot see the text — count it.
			if isSQLReceiver(sel.X) {
				res.Skipped++
			}
			return true
		}
		res.Queries = append(res.Queries, Query{
			File: fset.Position(call.Pos()).Filename,
			Line: fset.Position(call.Pos()).Line,
			SQL:  strings.Trim(lit.Value, "`"),
		})
		return true
	})
	return nil
}

// isSQLReceiver keeps the skip counter honest. Without it every `url.Query()`
// and `r.Exec()` on a non-database receiver would inflate the count and the
// number would stop meaning anything. The receivers that actually carry SQL in
// this codebase are pools, connections, transactions, the sqlc Queries handle,
// and pgx.Batch (for Queue — a dynamic `batch.Queue(sqlVar, ...)` must count as
// skipped, the same as a dynamic Query/Exec, not vanish silently).
func isSQLReceiver(x ast.Expr) bool {
	var name string
	switch v := x.(type) {
	case *ast.Ident:
		name = v.Name
	case *ast.SelectorExpr:
		name = v.Sel.Name
	default:
		return false
	}
	switch strings.ToLower(name) {
	case "db", "pool", "conn", "tx", "q", "queries", "batch":
		return true
	}
	return false
}

func goFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out, nil
}

// Condense flattens a multi-line query onto one line and truncates it, so a
// failure list stays readable when a dozen queries break at once.
func Condense(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		s = s[:117] + "..."
	}
	return s
}
