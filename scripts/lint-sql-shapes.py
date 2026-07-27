#!/usr/bin/env python3
"""
lint-sql-shapes.py — Lint for three SQL shape bugs documented as hard-won
lessons in CLAUDE.md (GB-3, Codeaudit v4). None of them are visible to the
compiler, to `go build`, or to a unit test that mocks the database — each one
shipped broken at least once and was found only by a human clicking through a
live stack against real Postgres.

1. CTE_INSERT_JOIN — `WITH x AS (INSERT ... RETURNING ...) SELECT ... FROM/JOIN
   <same table>`. Every WITH sub-statement plus the main query runs under ONE
   Postgres snapshot: the outer SELECT never sees the row the CTE just
   inserted, so the query deterministically returns 0 rows for every caller.
   (S121: CreateManagementReview was born-broken this way — every call
   inserted a row, then failed to see it, returning 500 while a duplicate
   silently accumulated in the table on every retry.)

2. ON_CONFLICT_DEFERRABLE — `INSERT ... ON CONFLICT (<cols>)` targeting a
   `UNIQUE(...) DEFERRABLE` constraint. PostgreSQL rejects a deferrable
   unique constraint as an ON CONFLICT arbiter outright (not a logic bug — a
   SQLSTATE-level rejection every single time). (S121: UpsertSRAssignment
   against sr_assignments.UNIQUE(module_id, target_id) DEFERRABLE INITIALLY
   DEFERRED could never have succeeded for any caller; it just never had one
   until the "assign by email" feature was built.)

3. UNTYPED_INTERVAL_CONCAT — `$N || ' days'` (or hour/month/year/...) with no
   explicit cast on the placeholder. pgx cannot infer the bound parameter's
   type from string concatenation alone and fails to bind it. (S121:
   ListExpiringCertificates used `($2 || ' days')::interval` — the fix casts
   the placeholder explicitly, e.g. `$2::int || ' days'` or
   `$2::text || ' days'`.)

Usage: python3 scripts/lint-sql-shapes.py [--query-dir backend/db/queries]
                                           [--go-dir backend]
                                           [--migrations-dir backend/db/migrations]
Exit code 1 if a violation is found, 2 if the gate itself found nothing to
check (vacuous — same SA-03 rule as every other gate in this repo), 0
otherwise.

Opt-out (should almost never be needed — all three patterns are unsound by
construction, not a false-positive-prone heuristic like the org_id lint):
    // sqlshape-lint: ok — <reason>   (Go backtick literal, within 3 lines before)
    -- sqlshape-lint: ok — <reason>   (sqlc query file, within 3 lines before)
"""
import argparse
import re
import sys
from pathlib import Path

QUERY_HEADER = re.compile(r'--\s*name:\s*(\w+)\s*:\s*(\w+)')
SKIP_RE = re.compile(r'sqlshape-lint:\s*ok', re.IGNORECASE)

# ── Pattern 1: CTE inserts into a table, then the outer query re-reads that
# same table in the same statement — the CTE's own write is invisible to it.
CTE_INSERT_JOIN_RE = re.compile(
    r'WITH\s+(\w+)\s+AS\s*\(\s*INSERT\s+INTO\s+(\w+)\b.*?RETURNING\b.*?\)\s*'
    r'SELECT\b.*?\b(?:FROM|JOIN)\s+\2\b',
    re.IGNORECASE | re.DOTALL,
)

# ── Pattern 3: a bound placeholder concatenated directly with an interval
# unit string, with no cast between the placeholder and `||` to give pgx a
# type to bind. `$2::int || ' days'` / `$2::text || ' days'` do NOT match
# (the cast sits between $N and ||); a bare `$2 || ' days'` does.
INTERVAL_UNITS = "day|days|hour|hours|minute|minutes|month|months|year|years|week|weeks"
UNTYPED_INTERVAL_RE = re.compile(
    rf"\$\d+\s*\|\|\s*'\s*(?:{INTERVAL_UNITS})\b",
    re.IGNORECASE,
)

# ── Pattern 2 support: schema-derived DEFERRABLE UNIQUE constraints ─────────
_CREATE_TABLE_RE = re.compile(
    r'CREATE TABLE\s+(?:IF NOT EXISTS\s+)?"?(\w+)"?\s*\(',
    re.IGNORECASE,
)
_UNIQUE_DEFERRABLE_RE = re.compile(
    r'UNIQUE\s*\(([^)]+)\)\s*DEFERRABLE',
    re.IGNORECASE,
)
_INSERT_ON_CONFLICT_RE = re.compile(
    r'INSERT\s+INTO\s+(\w+)\b.*?\bON\s+CONFLICT\s*\(([^)]+)\)',
    re.IGNORECASE | re.DOTALL,
)
_ALTER_RENAME_TABLE_RE = re.compile(
    r'ALTER TABLE\s+"?(\w+)"?\s+RENAME TO\s+"?(\w+)"?',
    re.IGNORECASE,
)


def _table_bodies(sql: str):
    """Yield (table_name, balanced-paren body) for every CREATE TABLE statement."""
    for m in _CREATE_TABLE_RE.finditer(sql):
        name = m.group(1)
        start = m.end() - 1
        depth = 0
        i = start
        while i < len(sql):
            if sql[i] == '(':
                depth += 1
            elif sql[i] == ')':
                depth -= 1
                if depth == 0:
                    i += 1
                    break
            i += 1
        yield name, sql[start:i]


def _cols(raw: str) -> tuple:
    return tuple(sorted(c.strip().strip('"').lower() for c in raw.split(',')))


def derive_deferrable_constraints(migrations_dir: Path) -> set:
    """Return {(table, sorted_cols_tuple)} for every UNIQUE(...) DEFERRABLE
    constraint declared in db/migrations/*.up.sql.

    Table names are resolved through later `ALTER TABLE ... RENAME TO`
    migrations (same as scripts/lint-orgid-queries.py's derive_tenant_tables)
    — without this, the one real DEFERRABLE constraint in the schema
    (sr_assignments.UNIQUE(module_id, target_id), declared under its
    pre-rename name pg_assignments in migration 009, renamed by migration
    122) would never match any current query and this check would be
    permanently vacuous for the one case it exists to catch.
    """
    out = set()
    for f in sorted(migrations_dir.glob("*.up.sql")):
        content = f.read_text()
        for table, body in _table_bodies(content):
            for m in _UNIQUE_DEFERRABLE_RE.finditer(body):
                out.add((table.lower(), _cols(m.group(1))))
        for old, new in _ALTER_RENAME_TABLE_RE.findall(content):
            old, new = old.lower(), new.lower()
            renamed = {(t, c) for (t, c) in out if t == old}
            for t, c in renamed:
                out.discard((t, c))
                out.add((new, c))
    return out


def find_pattern1(sql: str) -> str | None:
    m = CTE_INSERT_JOIN_RE.search(sql)
    if not m:
        return None
    return (f"CTE_INSERT_JOIN: CTE inserts into '{m.group(2)}' with RETURNING, "
            f"then the outer query re-reads '{m.group(2)}' in the same statement — "
            f"it cannot see the row the CTE just inserted (0 rows, every call)")


def find_pattern2(sql: str, deferrable: set) -> str | None:
    for m in _INSERT_ON_CONFLICT_RE.finditer(sql):
        table = m.group(1).lower()
        cols = _cols(m.group(2))
        if (table, cols) in deferrable:
            return (f"ON_CONFLICT_DEFERRABLE: INSERT INTO {m.group(1)} ON CONFLICT "
                    f"({m.group(2).strip()}) targets a UNIQUE(...) DEFERRABLE constraint — "
                    f"Postgres rejects a deferrable unique constraint as an ON CONFLICT "
                    f"arbiter outright; use find-then-insert-or-update instead")
    return None


def find_pattern3(sql: str) -> str | None:
    m = UNTYPED_INTERVAL_RE.search(sql)
    if not m:
        return None
    return (f"UNTYPED_INTERVAL_CONCAT: {m.group(0)!r} concatenates a bound placeholder "
            f"with an interval-unit string with no cast in between — pgx cannot type "
            f"the bound parameter; cast the placeholder explicitly (e.g. $N::int || '...')")


def check_chunk(sql: str, deferrable: set) -> list:
    checks = [find_pattern1(sql), find_pattern2(sql, deferrable), find_pattern3(sql)]
    return [c for c in checks if c]


def parse_sqlc_queries(sql_content: str):
    """Yield (query_name, body, header_line) for every -- name: X :type block."""
    lines = sql_content.splitlines()
    name, body_lines, header_line = None, [], 0
    for i, line in enumerate(lines, start=1):
        m = QUERY_HEADER.match(line.strip())
        if m:
            if name:
                yield name, "\n".join(body_lines), header_line
            name = m.group(1)
            header_line = i
            body_lines = []
        elif name:
            body_lines.append(line)
    if name:
        yield name, "\n".join(body_lines), header_line


def scan_query_dir(query_dir: Path, deferrable: set):
    violations, total, skipped = [], 0, 0
    for sql_file in sorted(query_dir.glob("*.sql")):
        content = sql_file.read_text()
        for qname, body, header_line in parse_sqlc_queries(content):
            total += 1
            lines = content.splitlines()
            preceding = "\n".join(lines[max(0, header_line - 4):header_line])
            if SKIP_RE.search(preceding):
                # Not checked -> belongs in the skipped column, not the checked
                # one. A hardcoded skipped=0 reports work the gate did not do.
                total -= 1
                skipped += 1
                continue
            for msg in check_chunk(body, deferrable):
                violations.append((sql_file.name, qname, msg))
    return violations, total, skipped


def scan_go_dir(go_dir: Path, deferrable: set):
    violations, total, skipped = [], 0, 0
    for go_file in sorted(go_dir.rglob("*.go")):
        if go_file.name.endswith(".sql.go") or "internal/db/" in str(go_file):
            continue
        text = go_file.read_text(errors="replace")
        lines = text.splitlines()

        i = 0
        while i < len(text):
            if text[i] != '`':
                i += 1
                continue
            start = i
            start_line = text[:start].count('\n') + 1
            i += 1
            while i < len(text) and text[i] != '`':
                i += 1
            snippet = text[start + 1:i]
            i += 1

            stripped = snippet.strip()
            if not re.match(r'(?is)^(SELECT|INSERT|UPDATE|DELETE|WITH)\b', stripped):
                continue
            total += 1

            preceding = "\n".join(lines[max(0, start_line - 4):start_line])
            if SKIP_RE.search(preceding):
                total -= 1
                skipped += 1
                continue

            for msg in check_chunk(snippet, deferrable):
                violations.append((str(go_file), start_line, msg))
    return violations, total, skipped


def main():
    parser = argparse.ArgumentParser(
        description="Lint for the 3 SQL-shape bugs in CLAUDE.md (GB-3): "
                     "CTE-insert-then-reread, ON CONFLICT on a deferrable "
                     "constraint, untyped interval-concat placeholder."
    )
    parser.add_argument("--query-dir", default="backend/db/queries")
    parser.add_argument("--go-dir", default="backend")
    parser.add_argument("--migrations-dir", default="backend/db/migrations")
    args = parser.parse_args()

    query_dir = Path(args.query_dir)
    go_dir = Path(args.go_dir)
    migrations_dir = Path(args.migrations_dir)
    for d in (query_dir, go_dir, migrations_dir):
        if not d.exists():
            print(f"ERROR: {d} not found", file=sys.stderr)
            sys.exit(1)

    deferrable = derive_deferrable_constraints(migrations_dir)

    q_violations, q_total, q_skipped = scan_query_dir(query_dir, deferrable)
    g_violations, g_total, g_skipped = scan_go_dir(go_dir, deferrable)
    total = q_total + g_total
    skipped = q_skipped + g_skipped

    print(f"NENNER: sqlc_queries_checked={q_total} go_sql_chunks_checked={g_total} "
          f"deferrable_constraints={len(deferrable)} | skipped={skipped}")

    if total == 0:
        print("sql-shape lint: VAKUAER — 0 SQL statements found to check "
              f"({query_dir}, {go_dir})")
        sys.exit(2)

    # Pattern 2 (ON CONFLICT against a DEFERRABLE constraint) can only ever fire
    # if the constraint set was actually derived. An empty set makes that half of
    # the gate permanently vacuous while the other half keeps it green and
    # exit 0 — the gate would report OK for a check it never performed. The
    # schema has had deferrable unique constraints since migration 009
    # (sr_assignments), so zero means the derivation broke, not that the schema
    # changed.
    if not deferrable:
        print("sql-shape lint: VAKUAER — derive_deferrable_constraints() returned an "
              f"EMPTY set from {migrations_dir}; the ON CONFLICT/DEFERRABLE check "
              "cannot fire. Fix the derivation rather than trusting this green.")
        sys.exit(2)

    violations = q_violations + g_violations
    if violations:
        print(f"\nsql-shape lint: {len(violations)} violation(s) found\n")
        for item in q_violations:
            fname, qname, msg = item
            print(f"  FAIL  {fname}:{qname}\n        {msg}")
        for item in g_violations:
            fpath, lineno, msg = item
            print(f"  FAIL  {fpath}:{lineno}\n        {msg}")
        print()
        print("  Fix the query shape, or annotate with a justification:")
        print("    // sqlshape-lint: ok — <reason>   (Go)")
        print("    -- sqlshape-lint: ok — <reason>   (sqlc .sql file)")
        print()
        sys.exit(1)

    print(f"sql-shape lint: OK ({total} SQL statements checked, 0 violations)")


if __name__ == "__main__":
    main()
