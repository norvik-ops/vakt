#!/usr/bin/env python3
"""
check_module_isolation.py — G11 module-isolation gate (Sprint 134, Spur C / S8).

Vakt's six product modules must not reach across the module boundary. The
boundary has two halves (the "6×6 import/schema graph"):

  1. IMPORT half — a module package must not import another module's Go package.
     Shared behaviour flows through interfaces in internal/shared/events
     (ADR-0004 / ADR-0079).

  2. SCHEMA-WRITE half — a module may only WRITE (INSERT/UPDATE/DELETE) its own
     DB table prefix. Writing another module's prefix couples the two at the
     schema level and is exactly what A1 (vaktcomply → vb_assets) and A2
     (vaktprivacy → hr_/sr_) were. Those now go through injected owner-module
     writers/erasers (ADR-0079).

Cross-prefix READS (SELECT) are the DELIBERATELY TOLERATED exception (A3,
Reporting-Reads) — see ADR-0079 §A3 — so this gate flags WRITES only. A DELETE
whose sub-SELECT reads a foreign prefix is fine as long as the deleted table is
the module's own.

Prefix ownership (CLAUDE.md → Module isolation):
    vb_ vaktscan   ck_ vaktcomply   so_ vaktvault
    sr_ vaktaware  po_ vaktprivacy  hr_ vakthr

Tables WITHOUT one of those six prefixes are platform-shared (users, org_members,
organizations, refresh_sessions, api_keys, audit_log, …) and any module may write
them — they are not part of the 6×6 grid.

The gate reads only STRING LITERALS from the Go source (a real Go string/comment
lexer, below), never raw text — so a table name mentioned in a `//` comment or a
prose sentence is never counted (the interface-ratchet lesson: gates that count
prose lie). Files it cannot read are counted and reported, never silently
dropped.

Usage:  python3 scripts/check_module_isolation.py [--modules-dir backend/internal/modules]
Exit 0 = clean, 1 = violation(s), 2 = nothing scanned (empty denominator).
"""
import argparse
import re
import sys
from pathlib import Path

# Module directory name → owned DB table prefix.
MODULE_PREFIX = {
    "vaktscan": "vb_",
    "vaktcomply": "ck_",
    "vaktvault": "so_",
    "vaktaware": "sr_",
    "vaktprivacy": "po_",
    "vakthr": "hr_",
}
ALL_PREFIXES = tuple(MODULE_PREFIX.values())
MODULE_NAMES = set(MODULE_PREFIX)

# Curated exceptions. Key: "relative/path.go" for an import violation, or
# "relative/path.go:TABLE" for a write violation. Value: reason (printed). Keep
# empty unless a violation is a deliberate, documented exception — every entry
# is a hole in the gate and must justify itself.
ALLOWLIST = {}

# Write statement inside SQL: verb + (optional quote) + table identifier. The
# table position after INSERT INTO / UPDATE / DELETE FROM. Case-insensitive.
WRITE_RE = re.compile(
    r'(?i)(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM)\s+"?([a-z_][a-z0-9_]*)'
)
# Module Go-package import path, e.g. .../internal/modules/vaktscan[/sub].
IMPORT_RE = re.compile(r'internal/modules/(vaktscan|vaktcomply|vaktvault|vaktaware|vaktprivacy|vakthr)\b')
# SQL comments inside a literal — strip before matching so a `-- update vb_x`
# note in our own SQL is never mistaken for a statement.
SQL_LINE_COMMENT = re.compile(r'--[^\n]*')
SQL_BLOCK_COMMENT = re.compile(r'/\*.*?\*/', re.DOTALL)


def string_literals(src):
    """Yield (start_line, content) for every Go string literal (raw + interpreted),
    skipping comments and rune literals. A hand-rolled Go lexer — the only way to
    tell a table name in SQL from the same word in a comment."""
    out = []
    i, n, line = 0, len(src), 1
    while i < n:
        c = src[i]
        if c == '\n':
            line += 1
            i += 1
        elif c == '/' and i + 1 < n and src[i + 1] == '/':      # // line comment
            i += 2
            while i < n and src[i] != '\n':
                i += 1
        elif c == '/' and i + 1 < n and src[i + 1] == '*':      # /* block comment */
            i += 2
            while i + 1 < n and not (src[i] == '*' and src[i + 1] == '/'):
                if src[i] == '\n':
                    line += 1
                i += 1
            i += 2
        elif c == '`':                                          # raw string
            start_line, j, buf = line, i + 1, []
            while j < n and src[j] != '`':
                if src[j] == '\n':
                    line += 1
                buf.append(src[j])
                j += 1
            out.append((start_line, ''.join(buf)))
            i = j + 1
        elif c == '"':                                          # interpreted string
            start_line, j, buf = line, i + 1, []
            while j < n and src[j] != '"':
                if src[j] == '\\' and j + 1 < n:                # skip escape
                    buf.append(src[j:j + 2])
                    j += 2
                    continue
                if src[j] == '\n':                              # unterminated; bail
                    break
                buf.append(src[j])
                j += 1
            out.append((start_line, ''.join(buf)))
            i = j + 1
        elif c == "'":                                          # rune literal
            j = i + 1
            while j < n and src[j] != "'":
                if src[j] == '\\' and j + 1 < n:
                    j += 2
                    continue
                j += 1
            i = j + 1
        else:
            i += 1
    return out


def module_of(rel_path):
    """internal/modules/<module>/... → <module>, else None."""
    parts = rel_path.split('/')
    if len(parts) >= 3 and parts[0] == 'internal' and parts[1] == 'modules':
        return parts[2] if parts[2] in MODULE_NAMES else None
    return None


def scan_file(path, rel_path, own_prefix, own_module):
    """Return (write_violations, import_violations) for one module .go file."""
    writes, imports = [], []
    try:
        src = path.read_text(encoding='utf-8')
    except (OSError, UnicodeDecodeError) as exc:
        return None, str(exc)  # unreadable → counted by caller

    for start_line, content in string_literals(src):
        # Import half: a module-package import path in a string literal.
        for m in IMPORT_RE.finditer(content):
            imported = m.group(1)
            if imported != own_module:
                imports.append((rel_path, start_line, imported))

        # Schema-write half: only look at literals that carry a write verb.
        if not re.search(r'(?i)\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM)\b', content):
            continue
        cleaned = SQL_BLOCK_COMMENT.sub(' ', content)
        cleaned = SQL_LINE_COMMENT.sub(' ', cleaned)
        cleaned = re.sub(r'\s+', ' ', cleaned)
        for m in WRITE_RE.finditer(cleaned):
            table = m.group(1).lower()
            for prefix in ALL_PREFIXES:
                if table.startswith(prefix) and prefix != own_prefix:
                    line = start_line + content[:m.start()].count('\n')
                    writes.append((rel_path, line, table, prefix))
                    break
    return (writes, imports), None


def main():
    # Anchor to the repo, not the CWD: a gate that reads the caller's working
    # directory is a gate that passes/fails by accident (the check-docs lesson).
    repo_root = Path(__file__).resolve().parent.parent
    ap = argparse.ArgumentParser()
    ap.add_argument('--modules-dir', default=str(repo_root / 'backend/internal/modules'))
    args = ap.parse_args()

    root = Path(args.modules_dir)
    if not root.is_dir():
        print(f"ERROR: modules dir {root} not found", file=sys.stderr)
        sys.exit(2)

    write_violations, import_violations = [], []
    files_checked, unreadable = 0, []

    for path in sorted(root.rglob('*.go')):
        if path.name.endswith('_test.go'):
            continue  # test fixtures may seed any table; not part of runtime coupling
        rel = path.relative_to(root)
        rel_path = f"internal/modules/{rel.as_posix()}"
        own_module = module_of(rel_path)
        if own_module is None:
            continue
        own_prefix = MODULE_PREFIX[own_module]
        result, err = scan_file(path, rel_path, own_prefix, own_module)
        if result is None:
            unreadable.append((rel_path, err))
            continue
        files_checked += 1
        w, im = result
        for v in w:
            if f"{v[0]}:{v[2]}" not in ALLOWLIST:
                write_violations.append(v)
        for v in im:
            if v[0] not in ALLOWLIST:
                import_violations.append(v)

    # Non-vacuity: an empty denominator is not "clean", it is "measured nothing".
    if files_checked == 0:
        print("ERROR: scanned 0 module files — selector matched nothing", file=sys.stderr)
        sys.exit(2)

    if write_violations or import_violations:
        print("G11 module-isolation: FAIL\n")
        if write_violations:
            print(f"  Cross-prefix WRITE ({len(write_violations)}): a module writing a foreign table prefix.")
            print("  Fix: write only your own prefix; route the foreign write through a shared")
            print("  interface implemented by the owning module (see ADR-0079).\n")
            for rel_path, line, table, prefix in write_violations:
                owner = next(m for m, p in MODULE_PREFIX.items() if p == prefix)
                print(f"    WRITE  {rel_path}:{line}  → {table}  (owned by {owner})")
            print()
        if import_violations:
            print(f"  Cross-module IMPORT ({len(import_violations)}): a module importing another module's package.")
            print("  Fix: depend on internal/shared/events interfaces, not the other module.\n")
            for rel_path, line, imported in import_violations:
                print(f"    IMPORT {rel_path}:{line}  → internal/modules/{imported}")
            print()
        if unreadable:
            print(f"  ({len(unreadable)} file(s) unreadable — not scanned)")
            for rel_path, err in unreadable:
                print(f"    SKIP   {rel_path}: {err}")
        sys.exit(1)

    tail = f"; {len(unreadable)} unreadable" if unreadable else ""
    print(f"G11 module-isolation: OK — {files_checked} module files, "
          f"0 cross-prefix writes, 0 cross-module imports{tail}")
    if unreadable:
        for rel_path, err in unreadable:
            print(f"    SKIP {rel_path}: {err}")


if __name__ == '__main__':
    main()
