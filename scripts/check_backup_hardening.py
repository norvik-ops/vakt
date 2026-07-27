#!/usr/bin/env python3
# Gate G10 — S17/S29 (Codeaudit-v4, Sprint 135, Spur B).
#
# Fund-Klasse (S29-01/S29-02, dieselbe Klasse wie S121-A1/O1): ein Backup-Skript,
# das `pg_dump ... | gzip > out`
# schreibt, hat drei stille Fehlerquellen, die jede für sich ein leeres Backup als
# "OK" durchgehen lassen — und ein leeres Backup ist schlimmer als ein
# fehlgeschlagenes, weil es gute Kopien per Retention verdrängt, bevor jemand
# reagiert:
#   1. Ohne `set -o pipefail` meldet `$?` gzips Exit-Code, nicht pg_dumps — ein
#      fehlgeschlagener Dump hinter einem erfolgreichen gzip sieht wie Erfolg aus.
#   2. Ohne eine Mindestgrößen-Assertion wird ein ~20-Byte-Leergzip behalten und
#      repliziert.
#   3. Eine Retention, die NICHT auf den eigenen Datei-Präfix scoped ist (bare
#      `rm *` / `-name '*'`), kann über die eigenen Backups hinaus löschen.
# Separat (S29-02): ein hartcodiertes Klartext-PGPASSWORD in einem committeten
# Skript ist CWE-798/CWE-260, unabhängig vom Pipe-Muster oben.
#
# Scope-Grenzen (bewusst, damit dieses Gate nicht wie check_routes.py/check_docs.py
# still einen Teil überspringt und trotzdem "OK" meldet):
#   - Die pipefail/Mindestgröße/Präfix-Prüfung gilt NUR für Skripte, die exakt das
#     `pg_dump ... | gzip` (oder `| gzip -c` / `| gzip -9` etc.) -Muster enthalten —
#     das ist die Klasse, die S29-01 tatsächlich betraf. Ein Skript, das pg_dump
#     direkt in eine Datei schreibt (`--format=custom -f out`, kein Pipe), hat
#     dieses Fehlerbild strukturell nicht (ein `set -e` lässt einen fehlgeschlagenen
#     pg_dump sofort abbrechen) und wird hier nicht verlangt, eine inline-
#     Mindestgrößen-Assertion zu tragen — dafür sorgt ggf. ein aufrufender Wrapper
#     (z. B. backup-cron.sh für backup.sh).
#   - Der Secret-Scan (PGPASSWORD) läuft dagegen über JEDES .sh unter scripts/ und
#     infra/server/scripts/, unabhängig vom Pipe-Muster.
#   - Jede gescannte/übersprungene Datei wird gezählt und ausgegeben — kein Skip
#     bleibt unsichtbar.
#
# Exit non-zero on any violation.

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCAN_DIRS = ["scripts", "infra/server/scripts"]

PG_DUMP_PIPE_RE = re.compile(r"pg_dump\b[^\n|]*\|\s*gzip\b")
PIPEFAIL_RE = re.compile(r"\bset\s+-[a-zA-Z]*o\s+pipefail\b|\bset\s+-o\s+pipefail\b")
# A minimum-size assertion: a byte-count read (stat -c%s / wc -c) whose result is
# later compared with -lt/-le against a numeric threshold, followed somewhere
# after by a rejection (rm + exit, or a bare exit non-zero).
SIZE_READ_RE = re.compile(r"stat\s+-c%s|wc\s+-c")
SIZE_CMP_RE = re.compile(r"-lt\s+\"?\$\{?\w+\}?\"?|\[\s*\"?\$\{?\w+\}?\"?\s+-lt\b")
# Rejecting the undersized file: either removing it (rm -f — the STATUS=1/return
# pattern used by multi-DB batch scripts) or a hard `exit N` (single-DB scripts).
REJECT_RE = re.compile(r"\brm\s+-\w*f\w*\b|\bexit\s+[1-9]\d*\b")
# A delete/prune whose glob target is a bare wildcard, not scoped to a prefix.
BARE_WILDCARD_DELETE_RE = re.compile(
    r"(?:-name\s+['\"]\*['\"]|rm\s+-\w*f\w*\s+\*(?:\s|$)|>\s*\*\s)"
)

# PGPASSWORD (or a *_DB_PASSWORD-style var) assigned a literal value, not a
# reference to an env var (${...} / $VAR) and not left empty.
HARDCODED_SECRET_RE = re.compile(
    r"^\s*(?:export\s+)?(PGPASSWORD|[A-Z_]*DB_PASSWORD|[A-Z_]*PGPASS)\s*=\s*"
    r"['\"]?([^\s'\"$][^\n'\"]*)['\"]?\s*$",
    re.MULTILINE,
)


def scripts():
    for sub in SCAN_DIRS:
        d = ROOT / sub
        if not d.is_dir():
            continue
        for p in sorted(d.glob("*.sh")):
            yield p


def check_pipe_pattern(path, text):
    """S29-01 class: pg_dump | gzip must be pipefail-guarded, size-asserted,
    and its retention must be prefix-scoped."""
    problems = []
    m = PG_DUMP_PIPE_RE.search(text)
    if not m:
        return problems, False

    if not PIPEFAIL_RE.search(text):
        problems.append(
            f"{path.relative_to(ROOT)}: pipes `pg_dump | gzip` without `set -o "
            f"pipefail` — a failed pg_dump is masked by gzip's exit code "
            f"(S29-01 class)."
        )

    has_size_read = SIZE_READ_RE.search(text)
    has_size_cmp = SIZE_CMP_RE.search(text)
    has_reject = REJECT_RE.search(text)
    if not (has_size_read and has_size_cmp and has_reject):
        problems.append(
            f"{path.relative_to(ROOT)}: pipes `pg_dump | gzip` without a visible "
            f"minimum-size assertion (byte-count read + `-lt` comparison + reject) "
            f"— an empty/partial gzip would be kept and replicated (S29-01 class)."
        )

    if BARE_WILDCARD_DELETE_RE.search(text):
        problems.append(
            f"{path.relative_to(ROOT)}: retention/cleanup deletes with a bare "
            f"wildcard (`*` / `-name '*'`) instead of a script-specific prefix — "
            f"can evict files it doesn't own."
        )

    return problems, True


def check_hardcoded_secret(path, text):
    problems = []
    for m in HARDCODED_SECRET_RE.finditer(text):
        var, val = m.group(1), m.group(2).strip()
        if not val:
            continue  # `PGPASSWORD=` (empty) — not a leak
        problems.append(
            f"{path.relative_to(ROOT)}: hardcoded {var} with a literal value — "
            f"CWE-798/CWE-260 (S29-02 class). Use a `.pgpass` file (mode 0600) or "
            f"docker-exec against the container's local socket (peer/trust auth, "
            f"no PGPASSWORD needed — see infra/server/scripts/backup-vakt-isms.sh)."
        )
    return problems


def main():
    problems = []
    checked = 0
    pipe_pattern_hits = 0

    for path in scripts():
        checked += 1
        text = path.read_text(encoding="utf-8", errors="ignore")

        pipe_problems, matched = check_pipe_pattern(path, text)
        problems.extend(pipe_problems)
        if matched:
            pipe_pattern_hits += 1

        problems.extend(check_hardcoded_secret(path, text))

    if checked == 0:
        print("❌ Gate G10: 0 shell scripts found under "
              f"{', '.join(SCAN_DIRS)} — scan directories moved or empty?")
        sys.exit(1)

    if problems:
        print("❌ Gate G10 (backup hardening) failed:\n")
        for p in problems:
            print(" - " + p)
        sys.exit(1)

    print(
        f"✓ Gate G10 OK — {checked} shell script(s) scanned "
        f"({', '.join(SCAN_DIRS)}), {pipe_pattern_hits} with a `pg_dump | gzip` "
        f"pattern all pipefail-guarded/size-asserted/prefix-scoped, 0 hardcoded "
        f"PGPASSWORD/DB_PASSWORD literals."
    )


if __name__ == "__main__":
    main()
