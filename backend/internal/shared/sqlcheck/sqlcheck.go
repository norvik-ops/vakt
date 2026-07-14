// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package sqlcheck extracts raw SQL literals from Go source and validates them
// against a live schema with PREPARE.
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
// PREPARE validates a statement against the current schema without executing it,
// so the check is read-only and needs no fixtures — only a migrated database.
package sqlcheck

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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

// Failure is a Query that PREPARE rejected, with the server's reason.
type Failure struct {
	Query
	Err error
}

func (f Failure) String() string {
	return fmt.Sprintf("%s:%d: PREPARE failed: %v\n    SQL: %s",
		filepath.Base(f.File), f.Line, f.Err, Condense(f.SQL))
}

// sqlMethods are the pgx call names whose signature is (ctx, sql, args...). A
// literal in argument position 1 of any of them is SQL.
var sqlMethods = map[string]bool{"Query": true, "QueryRow": true, "Exec": true}

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

// Prepare runs PREPARE for every query and returns the ones the server rejected.
// Each statement is deallocated again, so the connection is left as it was found.
func Prepare(ctx context.Context, conn *pgx.Conn, queries []Query) []Failure {
	var failures []Failure
	for i, q := range queries {
		name := fmt.Sprintf("sqlcheck_%d", i)
		if _, err := conn.Prepare(ctx, name, q.SQL); err != nil {
			failures = append(failures, Failure{Query: q, Err: err})
			continue
		}
		_ = conn.Deallocate(ctx, name)
	}
	return failures
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
		if !ok || !sqlMethods[sel.Sel.Name] || len(call.Args) < 2 {
			return true
		}
		lit, ok := call.Args[1].(*ast.BasicLit)
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
// this codebase are pools, connections, transactions and the sqlc Queries handle.
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
	case "db", "pool", "conn", "tx", "q", "queries":
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
