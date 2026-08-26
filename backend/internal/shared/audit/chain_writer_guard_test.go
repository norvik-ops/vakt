// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// G-AUDIT-CHAIN: only audit.Write may insert into audit_log.
//
// This gate exists because the exit-code decision in ADR-0040 rests on one
// claim: the set of unverifiable (hashless) audit rows cannot grow any more.
// Until this test existed, that was a wiring accident, not a guarantee —
// audit.Logger.Log still issues a raw `INSERT INTO audit_log` with no
// prev_hash/entry_hash, and AuditMiddleware wraps it. It happens to have no
// callers. Mounting that middleware would resume producing unchained rows and
// quietly invalidate a compliance decision that goes to the operator.
//
// The gate is a Go test on purpose rather than a script in scripts/: it runs
// inside the `go test ./...` that CI already executes, so it cannot become the
// project's signature failure — a gate that exists but was never wired up.
//
// What it does NOT check (stated here and printed, per the repo's rule that a
// gate must name what it skips). It only sees SQL that lives in ONE Go string
// literal inside this module. Specifically uncovered, verified by probing:
//
//   - concatenation: `"INSERT INTO " + "audit_log …"`
//   - format strings: `fmt.Sprintf("INSERT INTO %s …", "audit_log")`
//   - SQL in .sql files / sqlc-generated code (the walker reads only .go).
//     No practical gap today: backend/db/queries contains no audit_log write
//     and there is no go:embed of one — but a future sqlc query would slip past.
//   - anything issued from outside backend/
//
// Covered, because they are ordinary single literals and ordinary SQL:
// schema-qualified (`INSERT INTO public.audit_log`), quoted identifiers
// (`INSERT INTO "audit_log"`, `"public"."audit_log"`), arbitrary case and
// whitespace. Migrations (db/migrations) are deliberately out of scope — those
// are one-off data moves, not application writers.

// chainWriterAllowlist maps a repo-relative file path to the reason it may hold
// an INSERT into audit_log. Adding an entry here is a deliberate act: it means
// accepting a writer that does not extend the hash chain.
var chainWriterAllowlist = map[string]string{
	// The chained writer itself — the one legitimate INSERT (ADR-0040).
	"internal/shared/audit/writer.go": "audit.Write — the chain-extending writer",

	// Dead code. Logger.Log inserts without prev_hash/entry_hash; AuditMiddleware
	// is its only caller and has no callers itself. Allowed to exist ONLY as
	// long as it stays unreachable — TestAuditLoggerStaysUnwired enforces that.
	"internal/shared/audit/audit.go": "audit.Logger.Log — unchained, dead; kept unreachable by TestAuditLoggerStaysUnwired",
}

func TestOnlyAuditWriteInsertsIntoAuditLog(t *testing.T) {
	root := backendRoot(t)

	var (
		filesScanned int
		literals     int
		inserts      []string
	)

	walkGoSources(t, root, func(rel string, file *ast.File) {
		filesScanned++
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			literals++
			// Comments are never BasicLit nodes, so prose mentioning
			// "INSERT INTO audit_log" (agent.go documents the old defect that
			// way) cannot reach this. The repo has been bitten before by gates
			// that counted prose instead of code.
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				s = lit.Value
			}
			if insertsIntoAuditLog(s) {
				inserts = append(inserts, rel)
			}
			return true
		})
	})

	t.Logf("G-AUDIT-CHAIN: scanned %d Go file(s), %d string literal(s), found %d INSERT site(s) into audit_log",
		filesScanned, literals, len(inserts))
	require.Greater(t, filesScanned, 100,
		"denominator sanity: the walk found almost no Go files — the root path is wrong and this gate would pass vacuously")
	require.NotEmpty(t, inserts,
		"non-vacuity: not a single INSERT INTO audit_log was found — the matcher is broken, since audit.Write must always have one")

	for _, rel := range inserts {
		reason, allowed := chainWriterAllowlist[rel]
		require.True(t, allowed,
			"%s inserts into audit_log outside the chained writer.\n"+
				"Rows written this way carry no prev_hash/entry_hash: they hang on no chain, "+
				"audit-verify reports them as UNVERIFIABLE, and ADR-0040's exit-code decision "+
				"assumes their number cannot grow.\n"+
				"Use audit.Write(ctx, db, audit.WriteEntry{...}) instead — see internal/services/ai/agent.go "+
				"for the same defect and its fix (R1-27-V02 / ESK-4).", rel)
		t.Logf("  allowed: %s — %s", rel, reason)
	}
}

// TestAuditLoggerStaysUnwired is the second half of the guard. The allowlist
// tolerates audit.Logger.Log only because nothing reaches it; this test is what
// makes "unreachable" an enforced property instead of an observation.
func TestAuditLoggerStaysUnwired(t *testing.T) {
	root := backendRoot(t)
	const defining = "internal/shared/audit/audit.go"

	// Identifiers that would put the unchained writer back into a request path.
	watched := map[string]int{"AuditMiddleware": 0, "NewLogger": 0}
	var callers []string
	declared := map[string]bool{}

	walkGoSources(t, root, func(rel string, file *ast.File) {
		if rel == defining {
			for _, d := range file.Decls {
				switch decl := d.(type) {
				case *ast.FuncDecl:
					declared[decl.Name.Name] = true
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							declared[s.Name.Name] = true
						case *ast.ValueSpec:
							for _, n := range s.Names {
								declared[n.Name] = true
							}
						}
					}
				}
			}
			return // the declarations themselves are not callers
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if _, watch := watched[sel.Sel.Name]; watch {
					watched[sel.Sel.Name]++
					callers = append(callers, rel+" → "+sel.Sel.Name)
					// Do NOT descend: the Sel child is the same Ident and would
					// be counted a second time. The printed number has to match
					// reality — this repo has paid for gates whose figure
					// contradicted their own docstring.
					return false
				}
				return true
			}
			if id, ok := n.(*ast.Ident); ok {
				if _, watch := watched[id.Name]; watch {
					watched[id.Name]++
					callers = append(callers, rel+" → "+id.Name)
				}
			}
			return true
		})
	})

	// EXISTENCE ANCHOR. Without this the whole test is one rename away from
	// being silently vacuous: renaming NewLogger/AuditMiddleware and wiring the
	// unchained writer left BOTH assertions green while reporting "0 references"
	// for identifiers that no longer existed. A gate that reports zero because
	// it is looking for the wrong name is worse than no gate — it certifies.
	// Making a rename RED is the intended behaviour, not collateral: reactivating
	// this path must change the decision (ADR-0040) and the guard together.
	for name := range watched {
		require.True(t, declared[name],
			"G-AUDIT-CHAIN is watching %q, but %s no longer declares it — most likely a rename.\n"+
				"This test would otherwise pass vacuously while the unchained writer is wired up.\n"+
				"Update `watched` (and ADR-0040's exit-code section, which relies on this guard) "+
				"to the new name, or remove the dead writer outright.", name, defining)
	}

	t.Logf("G-AUDIT-CHAIN: unchained-writer reachability — AuditMiddleware=%d, NewLogger=%d reference(s) outside %s (both declarations present)",
		watched["AuditMiddleware"], watched["NewLogger"], defining)

	require.Empty(t, callers,
		"the unchained audit writer (audit.Logger.Log via %s) gained a caller: %v\n"+
			"Mounting it resumes writing audit rows without prev_hash/entry_hash. ADR-0040's exit-code "+
			"decision states the set of unverifiable rows cannot grow — this test is what makes that true.\n"+
			"Either route the write through audit.Write, or change ADR-0040 and this guard together.",
		defining, callers)
}

// TestInsertMatcherCoversOrdinarySQLForms pins what insertsIntoAuditLog claims
// to cover. The first version used a plain substring search and missed
// `public.audit_log` and `"audit_log"` — both single literals, both ordinary
// SQL, i.e. squarely inside the gate's advertised scope. The uncovered forms are
// asserted here too, so the documented limits cannot quietly drift from the code.
func TestInsertMatcherCoversOrdinarySQLForms(t *testing.T) {
	covered := []string{
		"INSERT INTO audit_log (org_id) VALUES ($1)",
		"insert into audit_log(org_id) values ($1)",
		"iNsErT\n\t InTo AUDIT_LOG (org_id)",
		"INSERT INTO public.audit_log (org_id) VALUES ($1)",
		`INSERT INTO "audit_log" (org_id) VALUES ($1)`,
		`INSERT INTO "public"."audit_log" (org_id)`,
		`INSERT INTO public."audit_log" (org_id)`,
		"WITH x AS (SELECT 1) INSERT INTO audit_log (org_id) SELECT 1",
	}
	for _, q := range covered {
		require.True(t, insertsIntoAuditLog(q), "must be detected: %q", q)
	}

	notAuditLog := []string{
		"INSERT INTO audit_log_new (org_id) VALUES ($1)",
		"INSERT INTO audit_log_legacy SELECT * FROM audit_log",
		`INSERT INTO "audit_log_new" (org_id)`,
		"INSERT INTO other_table (org_id) VALUES ($1)",
		"SELECT * FROM audit_log WHERE org_id = $1",
		"UPDATE audit_log SET deleted_at = now()",
	}
	for _, q := range notAuditLog {
		require.False(t, insertsIntoAuditLog(q), "must NOT be detected: %q", q)
	}

	// Known blind spots, asserted so the header comment cannot become a lie. If
	// one of these starts matching, the documentation is now too pessimistic —
	// that is a real change to review, not noise.
	knownMisses := []string{
		`"INSERT INTO " + "audit_log (org_id)"`, // concatenation: two separate literals
		"INSERT INTO %s (org_id) VALUES ($1)",   // fmt.Sprintf target
	}
	for _, q := range knownMisses {
		require.False(t, insertsIntoAuditLog(q),
			"documented blind spot unexpectedly detected — update the header comment: %q", q)
	}

	t.Logf("G-AUDIT-CHAIN matcher: %d covered form(s), %d non-match(es), %d documented blind spot(s)",
		len(covered), len(notAuditLog), len(knownMisses))
}

// insertTargetRe captures the table an INSERT targets, tolerating an optional
// schema qualifier and double-quoted identifiers on either part. A plain
// `strings.Index(norm, "insert into audit_log")` — the first version of this
// matcher — missed `INSERT INTO public.audit_log` and `INSERT INTO "audit_log"`,
// both of which are ordinary SQL in a single literal, i.e. squarely inside what
// this gate claims to cover.
var insertTargetRe = regexp.MustCompile(
	`insert\s+into\s+(?:(?:"[^"]*"|[a-z_][a-z0-9_$]*)\s*\.\s*)?("[^"]*"|[a-z_][a-z0-9_$]*)`)

// insertsIntoAuditLog reports whether a SQL string inserts into the audit_log
// table. Whitespace is normalised because the queries are multi-line raw
// strings; audit_log_new/audit_log_legacy (migration-only names) are excluded
// by comparing the full identifier rather than a prefix.
func insertsIntoAuditLog(s string) bool {
	norm := strings.ToLower(strings.Join(strings.Fields(s), " "))
	for _, m := range insertTargetRe.FindAllStringSubmatch(norm, -1) {
		if strings.Trim(m[1], `"`) == "audit_log" {
			return true
		}
	}
	return false
}

// walkGoSources parses every non-test Go file under root and hands the caller a
// repo-relative path plus its AST.
func walkGoSources(t *testing.T, root string, fn func(rel string, file *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "vendor" || name == "testdata" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		fn(filepath.ToSlash(rel), f)
		return nil
	})
	require.NoError(t, err, "walking backend sources")
}

// backendRoot resolves .../backend from this file's location.
func backendRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// here = .../backend/internal/shared/audit/<file> → ../../..
	return filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
}
