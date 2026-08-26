#!/usr/bin/env python3
"""
Self-test for check_module_isolation.py (Gate G11).

Why this file exists: CLAUDE.md requires three acceptances per gate, not one —
green on the baseline, RED on a real regression (naming the offender), and RED
when an improvement is lost. A gate whose red path was never exercised proves
nothing by being green; the repo has been bitten by exactly that (check_routes.py
silently skipping a quarter of its input, check_price_tax_marking's universally-
true exemption, the interface ratchet counting prose).

The gate's own scan_file() is the unit under test: it takes one Go source and
returns the (write, import) violations found in it, which is precisely the
decision the gate is made of.

Cases 9-13 (added 2026-07-30) cover the sqlc half. They exist because of K2-03:
this file previously imported scan_file ALONE, so it could not, by construction,
notice that the gate never looked at backend/db/queries/ — 217 prefix-writing
queries sat outside everything this self-test could reach, and the self-test's
green run read like coverage. A self-test whose scope is narrower than its gate
certifies the part that was never in doubt.

Exit non-zero on any failed case.
"""

import pathlib
import sys
import tempfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from check_module_isolation import (  # noqa: E402
    find_sqlc_call_sites,
    parse_sqlc_queries,
    scan_file,
    strip_go_code_noise,
)

FAILURES = []


def run_case(name, source, own_prefix, own_module, expect_writes, expect_imports):
    """Feed one synthetic Go file through scan_file and check the verdict."""
    with tempfile.TemporaryDirectory() as td:
        p = pathlib.Path(td) / "sample.go"
        p.write_text(source, encoding="utf-8")
        result, err = scan_file(p, "sample.go", own_prefix, own_module)

    if err is not None:
        FAILURES.append(f"{name}: unexpected read error {err}")
        return
    writes, imports = result
    if len(writes) != expect_writes:
        FAILURES.append(
            f"{name}: expected {expect_writes} write violation(s), got {len(writes)}: {writes}")
        return
    if len(imports) != expect_imports:
        FAILURES.append(
            f"{name}: expected {expect_imports} import violation(s), got {len(imports)}: {imports}")
        return
    print(f"PASS: {name}")


# ── 1. Baseline: a module writing ONLY its own prefix is clean ───────────────
run_case(
    "own-prefix write is clean",
    '''package vaktaware
func f(tx X) { tx.Exec(ctx, `DELETE FROM sr_targets WHERE org_id = $1`) }''',
    own_prefix="sr_", own_module="vaktaware",
    expect_writes=0, expect_imports=0,
)

# ── 2. RED on a real regression: the A2 finding, cross-prefix WRITE ──────────
run_case(
    "cross-prefix WRITE is caught (A2: vaktprivacy → hr_)",
    '''package vaktprivacy
func f(tx X) { tx.Exec(ctx, `DELETE FROM hr_employees WHERE org_id = $1`) }''',
    own_prefix="po_", own_module="vaktprivacy",
    expect_writes=1, expect_imports=0,
)

# ── 3. RED on the A1 finding: vaktcomply writing vb_ ────────────────────────
run_case(
    "cross-prefix WRITE is caught (A1: vaktcomply → vb_)",
    '''package vaktcomply
func f(tx X) { tx.Exec(ctx, `UPDATE vb_assets SET protection_need_id = $1`) }''',
    own_prefix="ck_", own_module="vaktcomply",
    expect_writes=1, expect_imports=0,
)

# ── 4. RED on a cross-module IMPORT ─────────────────────────────────────────
run_case(
    "cross-module import is caught",
    '''package vaktprivacy
import ("github.com/matharnica/vakt/internal/modules/vakthr")''',
    own_prefix="po_", own_module="vaktprivacy",
    expect_writes=0, expect_imports=1,
)

# ── 5. Cross-prefix READ stays tolerated (A3) — a false positive here would
#      make the gate red on a healthy repo, and a gate that is red on a healthy
#      repo gets switched off rather than fixed.
run_case(
    "cross-prefix READ is NOT flagged (A3 tolerated)",
    '''package vaktcomply
func f(tx X) { tx.Query(ctx, `SELECT id FROM hr_employees WHERE org_id = $1`) }''',
    own_prefix="ck_", own_module="vaktcomply",
    expect_writes=0, expect_imports=0,
)

# ── 6. A DELETE of an OWN table whose sub-SELECT reads a foreign prefix is
#      legal — the deleted table is what counts. This is the exact shape the
#      vaktaware eraser used to have; flagging it would be wrong.
run_case(
    "own-table DELETE with foreign sub-SELECT is legal",
    '''package vaktaware
func f(tx X) { tx.Exec(ctx, `DELETE FROM sr_campaign_enrollments WHERE employee_id IN (
    SELECT id::text FROM hr_employees WHERE org_id = $1)`) }''',
    own_prefix="sr_", own_module="vaktaware",
    expect_writes=0, expect_imports=0,
)

# ── 7. Platform-shared tables (no module prefix) are writable by any module ──
run_case(
    "platform-shared table write is legal",
    '''package vaktprivacy
func f(tx X) { tx.Exec(ctx, `UPDATE users SET is_active = FALSE WHERE id = $1`) }''',
    own_prefix="po_", own_module="vaktprivacy",
    expect_writes=0, expect_imports=0,
)

# ── 8. A commented-out violation must not count — otherwise the gate measures
#      writing style rather than behaviour (the interface-ratchet lesson).
run_case(
    "SQL-commented violation is not counted",
    '''package vaktprivacy
func f(tx X) { tx.Exec(ctx, `SELECT 1 -- DELETE FROM hr_employees
`) }''',
    own_prefix="po_", own_module="vaktprivacy",
    expect_writes=0, expect_imports=0,
)


# ── sqlc half (K2-03) ───────────────────────────────────────────────────────

def run_sqlc_case(name, sql, go_files, expect_hits, expect_unparsed=0):
    """Feed synthetic query file(s) + module Go file(s) through the sqlc
    resolver and check how many cross-prefix write call sites come out."""
    with tempfile.TemporaryDirectory() as td:
        qdir = pathlib.Path(td) / "queries"
        qdir.mkdir()
        (qdir / "sample.sql").write_text(sql, encoding="utf-8")
        writes, stats = parse_sqlc_queries(qdir)

        module_files = [
            (rel, own_module, own_prefix, src)
            for rel, own_module, own_prefix, src in go_files
        ]
        hits = find_sqlc_call_sites(module_files, writes)

    ok = True
    if len(hits) != expect_hits:
        FAILURES.append(
            f"{name}: expected {expect_hits} cross-prefix sqlc call site(s), got "
            f"{len(hits)}: {hits}"
        )
        ok = False
    if len(stats["unparsed"]) != expect_unparsed:
        FAILURES.append(
            f"{name}: expected {expect_unparsed} unparsed query file(s), got "
            f"{stats['unparsed']}"
        )
        ok = False
    print(("PASS: " if ok else "FAIL: ") + name)


GO_SCAN_CALL = [(
    "internal/modules/vaktscan/h.go", "vaktscan", "vb_",
    "package vaktscan\nfunc f(q *Q) { _, _ = q.InsertCKThing(ctx, arg) }\n",
)]

# ── 9. The K2-03 shape itself: module code calls a query that writes a
#      foreign prefix. Nothing about this is visible in the Go source.
run_sqlc_case(
    "cross-prefix write through sqlc is caught",
    "-- name: InsertCKThing :one\nINSERT INTO ck_evidence (org_id) VALUES ($1) RETURNING id;\n",
    GO_SCAN_CALL,
    expect_hits=1,
)

# ── 10. Same call, own prefix — must stay silent, or the gate is unusable.
run_sqlc_case(
    "own-prefix write through sqlc is legal",
    "-- name: InsertCKThing :one\nINSERT INTO vb_assets (org_id) VALUES ($1) RETURNING id;\n",
    GO_SCAN_CALL,
    expect_hits=0,
)

# ── 11. Schema-qualified / quoted table names. This is the I4 variant of
#      K2-06: the sibling gates demanded the bare identifier and therefore
#      neither reported NOR counted `public.ck_x` / `"ck_x"`.
run_sqlc_case(
    "schema-qualified + quoted cross-prefix write is caught",
    '-- name: InsertCKThing :exec\nINSERT INTO public."ck_evidence" (org_id) VALUES ($1);\n',
    GO_SCAN_CALL,
    expect_hits=1,
)

# ── 12. A query name that only APPEARS in a comment or a string is not a call
#      site — the same rule the Go-literal half already follows (a gate that
#      counts prose lies), and the mirror image of K2-07.
run_sqlc_case(
    "query name in a comment / string is not a call site",
    "-- name: InsertCKThing :exec\nINSERT INTO ck_evidence (org_id) VALUES ($1);\n",
    [(
        "internal/modules/vaktscan/h.go", "vaktscan", "vb_",
        'package vaktscan\n'
        '// historical: this used q.InsertCKThing(ctx, a) before ADR-0079\n'
        'const note = "removed q.InsertCKThing("\n',
    )],
    expect_hits=0,
)

# ── 13. A .sql file the parser cannot read is counted as unparsed, never
#      silently treated as containing no writes.
run_sqlc_case(
    "query file without `-- name:` headers is counted as unparsed",
    "INSERT INTO ck_evidence (org_id) VALUES ($1);\n",
    GO_SCAN_CALL,
    expect_hits=0, expect_unparsed=1,
)

# ── 14. R-03: strip_go_code_noise must be POSITION-PRESERVING.
#      The gate computes the reported line as `code.count('\n', 0, m.start())+1`
#      from the STRIPPED text. A swallowed newline therefore does not weaken
#      detection — it sends the reader to the wrong line, which is worse than
#      no finding at all, because it looks like one. Measured on the real tree
#      before the fix: 2 of 207 module files shifted by 1, both triggered by the
#      Go rune literal '"'.
LINE_SHAPES = {
    "rune literal '\"' (the R-03 trigger)":
        "package a\nfunc f(s string) bool {\n\treturn s[0] == '\"'\n}\nvar x = 1\n",
    "rune literal '`'":
        "package a\nvar q = '`'\nvar x = 1\n",
    "block comment across lines":
        "package a\n/* one\n   two\n   three */\nvar x = 1\n",
    "unterminated block comment at EOF":
        "package a\n/* one\n   two\n",
    "raw string across lines":
        "package a\nvar s = `SELECT 1\nFROM t\nWHERE x`\nvar y = 2\n",
    "escaped quote inside a string":
        "package a\nvar s = \"he said \\\"hi\\\"\"\nvar y = 2\n",
    "backslash at end of a string, then newline (invalid Go, must not eat the line)":
        "package a\nvar s = \"trailing \\\nvar y = 2\n",
    "line comment at EOF without a trailing newline":
        "package a\n// note",
    "CRLF line endings":
        "package a\r\n// c\r\nvar x = 1\r\n",
}
shape_ok = True
for label, src in LINE_SHAPES.items():
    out = strip_go_code_noise(src)
    if out.count("\n") != src.count("\n") or len(out) != len(src):
        FAILURES.append(
            f"strip_go_code_noise not position-preserving for {label}: "
            f"{len(src)}/{src.count(chr(10))} in, {len(out)}/{out.count(chr(10))} out"
        )
        shape_ok = False
print(("PASS: " if shape_ok else "FAIL: ")
      + f"strip_go_code_noise preserves lines and length ({len(LINE_SHAPES)} shapes, R-03)")

# ── 15. The same defect where it actually hurts: the REPORTED line number.
#      Before the fix this reported :7 for a call that stands on line 8.
LINE_PROBE_SRC = (
    "package vaktvault\n"                                     # 1
    "\n"                                                      # 2
    "func quoteChar(s string) bool {\n"                       # 3
    "\treturn len(s) > 0 && s[0] == '\"'\n"                   # 4  <- the trigger
    "}\n"                                                     # 5
    "\n"                                                      # 6
    "func writeIt(ctx context.Context, q *db.Queries) error {\n"   # 7
    "\t_, err := q.InsertCKThing(ctx, db.InsertCKThingParams{})\n"  # 8  <- the call
    "\treturn err\n"                                          # 9
    "}\n"                                                     # 10
)
with tempfile.TemporaryDirectory() as td:
    qdir = pathlib.Path(td) / "queries"
    qdir.mkdir()
    (qdir / "sample.sql").write_text(
        "-- name: InsertCKThing :exec\nINSERT INTO ck_evidence (org_id) VALUES ($1);\n",
        encoding="utf-8",
    )
    writes, _ = parse_sqlc_queries(qdir)
    hits = find_sqlc_call_sites(
        [("internal/modules/vaktvault/probe.go", "vaktvault", "so_", LINE_PROBE_SRC)],
        writes,
    )
if len(hits) != 1:
    FAILURES.append(f"line-number probe: expected 1 hit, got {hits}")
    print("FAIL: reported line number survives a rune literal above the call")
elif hits[0][1] != 8:
    FAILURES.append(
        f"line-number probe: call stands on line 8, gate reports line {hits[0][1]} "
        "— R-03 is back"
    )
    print("FAIL: reported line number survives a rune literal above the call")
else:
    print("PASS: reported line number survives a rune literal above the call (R-03)")

if FAILURES:
    print("\nFAILED:")
    for f in FAILURES:
        print(f"  - {f}")
    sys.exit(1)

print("\nALL TESTS PASSED (15 cases: 8 Go-literal, 5 sqlc seam, 2 line fidelity)")
