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

THE SQLC HALF (added 2026-07-30, finding K2-03)
-----------------------------------------------
Until K2-03 this gate read Go string literals ONLY. But the regular DB path in
this repo is sqlc: the SQL lives in backend/db/queries/*.sql and the Go code
only calls `q.<Name>(...)`. 217 of the ~491 queries write a module-prefixed
table, and every one of them was outside this gate's search space. A live
violation was sitting in that gap: vaktscan (own prefix vb_) calls
`q.InsertCKCIEvidence` in internal/modules/vaktscan/handler_ci_evidence.go,
which is `INSERT INTO ck_evidence` — vaktcomply's prefix — the same class as A1,
merely routed through sqlc instead of written as a literal. Meanwhile
docs/adr/0079 claimed "kein Modul schreibt mehr ueber eine Praefix-Grenze; G11
haelt das gruen": the second half was true and the first half was not.

So the gate now resolves the seam: parse the query files, map query name ->
written tables, then find which module code calls which query. Query files it
cannot parse are counted as `unparsed` and make the gate fail — an unreadable
query file is not a clean one.

Known violations are recorded in KNOWN_SQLC_CROSS_PREFIX with a reason, printed
on every run, and stale-checked: an entry that stops reproducing fails the gate
so the exception cannot outlive the problem.

What the sqlc half still does NOT see (named, not left implicit):
  * The call-site matcher is `.<QueryName>(`. sqlc always generates methods on
    *Queries, so that is the shape today — but a query reached through a
    function-typed field or an interface method with a different name would be
    invisible. There are none in the tree right now; if one appears, this is the
    line that has to grow.
  * Only backend/internal/modules/** is scanned for call sites. A module that
    routes a foreign-prefix write through a helper OUTSIDE its own directory is
    not seen — the import half of the gate makes that hard, not impossible.

Usage:  python3 scripts/check_module_isolation.py [--modules-dir backend/internal/modules]
                                                  [--queries-dir backend/db/queries]
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

# Cross-prefix sqlc writes that EXIST TODAY and are tracked as open findings
# rather than fixed here. Key: (module-relative Go file, sqlc query name).
#
# This is not an amnesty list. Every entry is printed on every run, and an entry
# that stops reproducing FAILS the gate (see the stale check in main) — an
# exception that outlives its problem is a hole nobody can see any more, which
# is the same class as the stale ALLOWLIST in check_routes.py (K2-08).
KNOWN_SQLC_CROSS_PREFIX = {
    ("internal/modules/vaktscan/handler_ci_evidence.go", "InsertCKCIEvidence"): (
        "K2-03 (audit ledger, OPEN). vaktscan (vb_) writes vaktcomply's ck_evidence "
        "through sqlc: POST /vaktscan/assets/ci-evidence -> ReceiveCIEvidence -> "
        "q.InsertCKCIEvidence -> INSERT INTO ck_evidence. Same class as A1, routed "
        "through sqlc instead of a SQL literal. ADR-0079 requires the write to go "
        "through an owner interface in internal/shared/events; none exists for "
        "ck_evidence yet, so closing it is a backend change (new interface + wiring), "
        "not a gate change. REMOVE THIS ENTRY in the commit that routes the write."
    ),
}

# Write statement inside SQL: verb + optional schema qualifier + (optional
# quote) + table identifier, at the table position after INSERT INTO / UPDATE /
# DELETE FROM. Case-insensitive.
#
# The schema/quote tolerance is deliberate (I4 variant sweep for K2-06): the
# sibling gates chain_writer_guard_test.go (R1-F3A-09) and
# check_user_role_insert.py (K2-06) both had exactly this hole — they demanded
# the bare identifier, so `INSERT INTO public.ck_evidence` and
# `INSERT INTO "ck_evidence"` were invisible AND uncounted. Patching one
# occurrence and leaving the class is what I4 exists to prevent.
WRITE_RE = re.compile(
    r'(?i)(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM)\s+'
    r'(?:"?[a-z_][a-z0-9_]*"?\s*\.\s*)?'   # optional schema qualifier: public.
    r'"?([a-z_][a-z0-9_]*)'
)

# sqlc query header: `-- name: QueryName :one`
SQLC_NAME_RE = re.compile(r'^--\s*name:\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(\S+)\s*$', re.MULTILINE)
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


def strip_go_code_noise(src):
    """Blank out comments and string-literal CONTENTS, keeping line structure.

    The inverse of string_literals(): that one wants the literals, this one wants
    the executable code. Needed so a query name inside a doc comment or an error
    message is never mistaken for a call site (same reason string_literals exists
    at all — a gate that counts prose lies).

    POSITIONSERHALTEND, und das ist keine Nettigkeit: die gemeldete Zeile wird
    aus diesem Ergebnis gezählt (`code.count('\\n', 0, m.start()) + 1`). Ein
    verschluckter Zeilenumbruch heißt, dass das Gate den Leser an die falsche
    Stelle schickt — eine Fundstelle mit falscher Zeile schlägt niemand nach.

    R-03 (2026-07-30): genau das passierte. Beim Abbruch an einem
    unterminierten `"` — ausgelöst vom Go-Rune-Literal `'"'` — wurde der
    Zeilenumbruch durch ein Leerzeichen ERSETZT statt erhalten; ab dort meldete
    die Datei jede Fundstelle um 1 zu klein. Im echten Baum betroffen: 2 von 207
    Modul-Dateien (vaktaware/service.go, vaktcomply/service_ops.go).

    Deshalb ist der Rückbau jetzt STRUKTURELL statt zweigweise: jeder Zweig
    bestimmt nur noch das Ende des verbrauchten Bereichs, und `blank()` ersetzt
    diesen Bereich zeichenweise — `\\n` bleibt `\\n`, alles andere wird zum
    Leerzeichen. Länge und Zeilenzahl können damit nicht mehr auseinanderlaufen;
    die Schlussprüfung sagt es laut, falls doch."""
    out, i, n = [], 0, len(src)

    def blank(a, b):
        """src[a:b] verbrauchen: gleiche Länge, Zeilenumbrüche ERHALTEN."""
        out.append(''.join('\n' if ch == '\n' else ' ' for ch in src[a:b]))

    while i < n:
        c = src[i]
        if c == '/' and i + 1 < n and src[i + 1] == '/':
            j = src.find('\n', i)
            j = n if j < 0 else j          # das \n selbst NICHT verbrauchen
            blank(i, j)
            i = j
        elif c == '/' and i + 1 < n and src[i + 1] == '*':
            j = src.find('*/', i + 2)
            j = n if j < 0 else j + 2      # unterminiert -> bis Dateiende
            blank(i, j)
            i = j
        elif c in ('`', '"'):
            quote = c
            j = i + 1
            while j < n and src[j] != quote:
                # Escape-Paar überspringen — aber nie über eine Zeilengrenze:
                # `"...\` + Zeilenumbruch ist kein gültiges Go-Literal, und ein
                # verbrauchter Umbruch wäre wieder R-03.
                if quote == '"' and src[j] == '\\' and j + 1 < n and src[j + 1] != '\n':
                    j += 2
                    continue
                if quote == '"' and src[j] == '\n':
                    break                  # unterminiert (z. B. Rune-Literal '"')
                j += 1
            if j < n and src[j] == quote:
                j += 1                     # schließendes Quote mitverbrauchen
            blank(i, j)
            i = j
        else:
            out.append(c)
            i += 1

    res = ''.join(out)
    if len(res) != len(src) or res.count('\n') != src.count('\n'):
        raise RuntimeError(
            "strip_go_code_noise ist nicht positionserhaltend "
            f"({len(src)}/{src.count(chr(10))} Zeichen/Zeilen rein, "
            f"{len(res)}/{res.count(chr(10))} raus) — jede ab hier gemeldete "
            "Zeilennummer wäre falsch. Das ist die R-03-Klasse; nicht ignorieren."
        )
    return res


def parse_sqlc_queries(queries_dir):
    """backend/db/queries/*.sql -> (writes, stats).

    writes: query name -> sorted list of (table, owning prefix) it WRITES, kept
    only for module-prefixed tables (platform tables are shared, ADR-0079).
    stats: counters that go into the printed denominator."""
    writes, stats = {}, {'files': 0, 'queries': 0, 'unparsed': [], 'prefix_writers': 0}
    if not queries_dir.is_dir():
        stats['unparsed'].append((str(queries_dir), 'queries dir not found'))
        return writes, stats

    for path in sorted(queries_dir.glob('*.sql')):
        try:
            text = path.read_text(encoding='utf-8')
        except (OSError, UnicodeDecodeError) as exc:
            stats['unparsed'].append((path.name, str(exc)))
            continue
        stats['files'] += 1

        headers = list(SQLC_NAME_RE.finditer(text))
        if not headers:
            # A .sql file in the queries dir with no `-- name:` header is either
            # not a sqlc file or a file whose headers this parser no longer
            # recognises. Both are "not measured", not "clean".
            stats['unparsed'].append((path.name, 'no `-- name:` headers found'))
            continue

        for idx, m in enumerate(headers):
            name = m.group(1)
            end = headers[idx + 1].start() if idx + 1 < len(headers) else len(text)
            body = text[m.end():end]
            stats['queries'] += 1

            body = SQL_BLOCK_COMMENT.sub(' ', body)
            body = SQL_LINE_COMMENT.sub(' ', body)
            body = re.sub(r'\s+', ' ', body)

            hits = set()
            for w in WRITE_RE.finditer(body):
                table = w.group(1).lower()
                for prefix in ALL_PREFIXES:
                    if table.startswith(prefix):
                        hits.add((table, prefix))
                        break
            if hits:
                writes[name] = sorted(hits)
                stats['prefix_writers'] += 1
    return writes, stats


def find_sqlc_call_sites(module_files, query_writes):
    """[(rel_path, line, query_name, tables, prefix)] for every call from module
    code to a sqlc query that writes a table prefix the module does not own."""
    found = []
    if not query_writes:
        return found
    # `\.Name(` — sqlc always generates methods on *Queries, so a call always
    # carries a receiver dot. Anchored on the dot so a same-named local helper
    # or a bare identifier in a type name is not counted.
    call_re = re.compile(r'\.\s*(' + '|'.join(map(re.escape, sorted(query_writes))) + r')\s*\(')
    for rel_path, own_module, own_prefix, src in module_files:
        code = strip_go_code_noise(src)
        for m in call_re.finditer(code):
            name = m.group(1)
            for table, prefix in query_writes[name]:
                if prefix != own_prefix:
                    line = code.count('\n', 0, m.start()) + 1
                    found.append((rel_path, line, name, table, prefix, own_module))
    return found


def main():
    # Anchor to the repo, not the CWD: a gate that reads the caller's working
    # directory is a gate that passes/fails by accident (the check-docs lesson).
    repo_root = Path(__file__).resolve().parent.parent
    ap = argparse.ArgumentParser()
    ap.add_argument('--modules-dir', default=str(repo_root / 'backend/internal/modules'))
    ap.add_argument('--queries-dir', default=str(repo_root / 'backend/db/queries'))
    args = ap.parse_args()

    root = Path(args.modules_dir)
    if not root.is_dir():
        print(f"ERROR: modules dir {root} not found", file=sys.stderr)
        sys.exit(2)

    write_violations, import_violations = [], []
    files_checked, unreadable = 0, []
    module_files = []  # (rel_path, own_module, own_prefix, src) for the sqlc half

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
        module_files.append((rel_path, own_module, own_prefix, path.read_text(encoding='utf-8')))
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

    # ── sqlc half (K2-03) ────────────────────────────────────────────────
    query_writes, qstats = parse_sqlc_queries(Path(args.queries_dir))
    sqlc_hits = find_sqlc_call_sites(module_files, query_writes)

    sqlc_new, sqlc_known_seen = [], set()
    for rel_path, line, name, table, prefix, own_module in sqlc_hits:
        key = (rel_path, name)
        if key in KNOWN_SQLC_CROSS_PREFIX:
            sqlc_known_seen.add(key)
        else:
            sqlc_new.append((rel_path, line, name, table, prefix, own_module))

    # Second non-vacuity guard, for the half that was just added: if the query
    # parser reads nothing, every sqlc check below is silently empty and the
    # gate would go back to reporting OK over the exact gap K2-03 found.
    sqlc_errors = []
    if qstats['queries'] == 0:
        sqlc_errors.append(
            f"parsed 0 sqlc queries from {args.queries_dir} — the sqlc half of this "
            "gate measured NOTHING. That is the K2-03 state, not a clean result."
        )
    if qstats['unparsed']:
        sqlc_errors.append(
            f"{len(qstats['unparsed'])} query file(s) could not be parsed — their "
            "writes were NOT checked:\n" +
            "\n".join(f"      {n}: {e}" for n, e in qstats['unparsed'])
        )
    # Stale check: a recorded exception that no longer reproduces is a hole
    # nobody can see any more.
    for key, reason in sorted(KNOWN_SQLC_CROSS_PREFIX.items()):
        if key not in sqlc_known_seen:
            sqlc_errors.append(
                f"KNOWN_SQLC_CROSS_PREFIX entry {key[0]} / {key[1]}() no longer "
                "reproduces — the violation is gone (good) but the exception is still "
                "here (bad: it would silently excuse the write if it came back). "
                "Delete the entry from scripts/check_module_isolation.py."
            )

    if write_violations or import_violations or sqlc_new or sqlc_errors:
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
        if sqlc_new:
            print(f"  Cross-prefix WRITE via sqlc ({len(sqlc_new)}): a module calling a query")
            print("  that writes a foreign table prefix. The SQL lives in backend/db/queries/,")
            print("  so it is invisible to the Go-literal scan above — that is K2-03.")
            print("  Fix: route the write through an owner interface (ADR-0079), or, if it is")
            print("  a tracked open finding, record it in KNOWN_SQLC_CROSS_PREFIX with a reason.\n")
            for rel_path, line, name, table, prefix, own_module in sqlc_new:
                owner = next(m for m, p in MODULE_PREFIX.items() if p == prefix)
                print(f"    SQLC   {rel_path}:{line}  {name}() → {table}  "
                      f"({own_module} owns {MODULE_PREFIX[own_module]}, {table} owned by {owner})")
            print()
        if sqlc_errors:
            print(f"  sqlc seam ({len(sqlc_errors)}):\n")
            for e in sqlc_errors:
                print(f"    · {e}")
            print()
        if unreadable:
            print(f"  ({len(unreadable)} file(s) unreadable — not scanned)")
            for rel_path, err in unreadable:
                print(f"    SKIP   {rel_path}: {err}")
        sys.exit(1)

    tail = f"; {len(unreadable)} unreadable" if unreadable else ""
    print(f"G11 module-isolation: OK — {files_checked} module files, "
          f"0 cross-prefix writes, 0 cross-module imports{tail}")
    print(f"  sqlc seam      : {qstats['files']} query file(s), {qstats['queries']} queries, "
          f"unparsed: {len(qstats['unparsed'])}, "
          f"{qstats['prefix_writers']} write a module prefix")
    print(f"  sqlc call-sites: {len(sqlc_hits)} cross-prefix write call(s) from module code, "
          f"0 new, {len(sqlc_known_seen)} recorded as open finding(s)")
    for key in sorted(sqlc_known_seen):
        print(f"    KNOWN  {key[0]}  {key[1]}()")
        print(f"           {KNOWN_SQLC_CROSS_PREFIX[key].splitlines()[0]}")
    if unreadable:
        for rel_path, err in unreadable:
            print(f"    SKIP {rel_path}: {err}")
    # Wo dieses Gate wegschaut — bei jedem Lauf, nicht nur im Kopf. Ein Nenner,
    # den nur der Quelltext kennt, ist kein Nenner (R-02-Lehre, hier angewandt).
    print("  blind by design: *_test.go (nicht gescannt) · Query-Aufrufe außerhalb "
          "von backend/internal/modules/** · Aufrufe, die nicht die Form "
          "`.<QueryName>(` haben (Funktionsfeld, umbenannte Interface-Methode) · "
          "Cross-Prefix-READS (ADR-0079 §A3, bewusst toleriert). "
          "Zeilennummern sind positionserhaltend geprüft (R-03).")


if __name__ == '__main__':
    main()
