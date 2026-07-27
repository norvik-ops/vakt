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

Exit non-zero on any failed case.
"""

import pathlib
import sys
import tempfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from check_module_isolation import scan_file  # noqa: E402

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

if FAILURES:
    print("\nFAILED:")
    for f in FAILURES:
        print(f"  - {f}")
    sys.exit(1)

print("\nALL TESTS PASSED (8 cases)")
