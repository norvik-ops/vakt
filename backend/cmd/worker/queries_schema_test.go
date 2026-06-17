package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestWorkerRawSQLAgainstSchema parses every handlers_*.go file in this package,
// extracts each raw-SQL literal passed to pool.Query / pool.QueryRow / pool.Exec
// (and matching tx.* / conn.* variants), and runs PREPARE on each statement
// against the migrated test DB. PREPARE validates the query against the current
// schema without executing it — so this catches column drift like the
// is_deleted bug that filled the production Asynq retry queue on 2026-05-26.
//
// Only backtick string literals are considered; dynamically built queries
// (fmt.Sprintf, string concatenation) are skipped because we cannot validate
// them statically.
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
	defer conn.Close(ctx)

	// Scan all non-test .go files in this package, not only handlers_*.go,
	// so that shared helpers (shared.go, handlers_shared.go, …) are covered too.
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, f := range all {
		if !strings.HasSuffix(f, "_test.go") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatal("no .go files found — wrong working directory?")
	}

	queries := extractRawQueries(t, files)
	if len(queries) == 0 {
		t.Fatal("no raw SQL literals found — extractor probably broken")
	}
	t.Logf("validating %d raw SQL queries from %d files against schema", len(queries), len(files))

	for _, q := range queries {
		stmtName := fmt.Sprintf("drift_check_%d", q.id)
		if _, err := conn.Prepare(ctx, stmtName, q.sql); err != nil {
			t.Errorf("%s:%d: PREPARE failed: %v\n    SQL: %s",
				filepath.Base(q.file), q.line, err, condense(q.sql))
			continue
		}
		_ = conn.Deallocate(ctx, stmtName)
	}
}

type rawQuery struct {
	file string
	line int
	sql  string
	id   int
}

func extractRawQueries(t *testing.T, files []string) []rawQuery {
	t.Helper()
	fset := token.NewFileSet()
	var queries []rawQuery
	id := 0

	sqlMethods := map[string]bool{"Query": true, "QueryRow": true, "Exec": true}

	for _, file := range files {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
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
			if !sqlMethods[sel.Sel.Name] {
				return true
			}
			// SQL methods we care about always have (ctx, sql, args...) signature.
			if len(call.Args) < 2 {
				return true
			}
			lit, ok := call.Args[1].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			// Only consider backtick strings — they're the SQL convention here,
			// and double-quoted "Query"/"Exec" usages elsewhere (e.g. struct
			// fields, HTTP) get filtered out.
			if !strings.HasPrefix(lit.Value, "`") {
				return true
			}
			sql := strings.Trim(lit.Value, "`")

			queries = append(queries, rawQuery{
				file: fset.Position(call.Pos()).Filename,
				line: fset.Position(call.Pos()).Line,
				sql:  sql,
				id:   id,
			})
			id++
			return true
		})
	}

	return queries
}

func condense(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		s = s[:117] + "..."
	}
	return s
}
