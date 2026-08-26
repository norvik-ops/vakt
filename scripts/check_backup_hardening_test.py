#!/usr/bin/env python3
# Self-test for Gate G10 (scripts/check_backup_hardening.py).
#
# A gate that has never been proven to catch a regression is a gate that might
# be vacuous — the recurring lesson across check_routes.py, check_price_tax_marking
# and the interface ratchet (all grepped for something that never actually
# excluded anything, or an exception that always matched). This test builds
# fixture scripts in a temp dir, points the gate's SCAN_DIRS at it via
# monkeypatch, and asserts:
#   1. a hardened script (pipefail + size assertion + prefix-scoped retention,
#      no hardcoded secret) passes.
#   2. each individual defect (missing pipefail, missing size assertion, bare
#      wildcard retention, hardcoded PGPASSWORD) is caught on its own —
#      not just "some" defect combination.
#   3. a script that doesn't touch pg_dump|gzip at all is never flagged for the
#      pipe-pattern checks (no false positives on unrelated scripts).
#
# K2-04 (2026-07-30), two changes:
#   * This file used to re-implement the gate's scan loop instead of calling it.
#     That is the "two separate literals" class the gate itself warns about: the
#     copy could not see the denominator, the unparsed counter or the coverage
#     baseline, so five green cases certified a loop that was not the one CI
#     runs. It now drives gate.main() and reads its real stdout/exit code.
#   * The five original fixtures were all SINGLE-LINE. The defect K2-04 reported
#     is a pg_dump call wrapped over lines with `\` — structurally invisible to
#     every one of them. Cases 4a-4d cover the wrapped forms.

import contextlib
import importlib.util
import io
import pathlib
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
GATE_PATH = ROOT / "scripts" / "check_backup_hardening.py"

spec = importlib.util.spec_from_file_location("check_backup_hardening", GATE_PATH)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

HARDENED = """#!/bin/bash
set -uo pipefail
BACKUP_DIR=/var/backups/x
DB_CONTAINER=x-db-1
DB_NAME=x
MIN_SIZE=1000
OUT="$BACKUP_DIR/${DB_NAME}_$(date +%s).sql.gz"
docker exec "$DB_CONTAINER" pg_dump -U x "$DB_NAME" | gzip > "$OUT"
SIZE=$(stat -c%s "$OUT")
if [ "$SIZE" -lt "$MIN_SIZE" ]; then
    rm -f "$OUT"
    exit 1
fi
find "$BACKUP_DIR" -name "${DB_NAME}_*.sql.gz" -mtime +7 -delete
"""

NO_PIPEFAIL = HARDENED.replace("set -uo pipefail\n", "set -u\n")

NO_SIZE_ASSERTION = """#!/bin/bash
set -o pipefail
BACKUP_DIR=/var/backups/x
docker exec x-db-1 pg_dump -U x x | gzip > "$BACKUP_DIR/x.sql.gz"
find "$BACKUP_DIR" -name "x_*.sql.gz" -mtime +7 -delete
"""

BARE_WILDCARD_RETENTION = HARDENED.replace(
    'find "$BACKUP_DIR" -name "${DB_NAME}_*.sql.gz" -mtime +7 -delete',
    'find "$BACKUP_DIR" -name "*" -mtime +7 -delete',
)

HARDCODED_SECRET = """#!/bin/bash
set -euo pipefail
PGPASSWORD=deadbeefdeadbeefdeadbeefdeadbeef
pg_dump -U x -h localhost x > /tmp/x.sql
"""

UNRELATED_SCRIPT = """#!/bin/bash
set -euo pipefail
echo "no database backup here at all"
"""

# ── K2-04 fixtures: the SAME defects, written with a line-wrapped pg_dump.
# This is the ordinary shell way to write a long command, and it made all three
# checks silently inapplicable while the file still counted as "scanned".
WRAPPED_HARDENED = """#!/bin/bash
set -uo pipefail
BACKUP_DIR=/var/backups/x
MIN_SIZE=1000
OUT="$BACKUP_DIR/x_$(date +%s).sql.gz"
pg_dump --no-owner --no-privileges \\
        --host="$PGHOST" --username="$PGUSER" "$PGDATABASE" \\
  | gzip -9 > "$OUT"
SIZE=$(stat -c%s "$OUT")
if [ "$SIZE" -lt "$MIN_SIZE" ]; then
    rm -f "$OUT"
    exit 1
fi
find "$BACKUP_DIR" -name "x_*.sql.gz" -mtime +7 -delete
"""

WRAPPED_ALL_THREE_DEFECTS = """#!/bin/bash
set -u
BACKUP_DIR=/var/backups/x
OUT="$BACKUP_DIR/x.sql.gz"
pg_dump --no-owner --no-privileges \\
        --host="$PGHOST" --username="$PGUSER" "$PGDATABASE" \\
  | gzip -9 > "$OUT"
find /var/backups -name '*' -mtime +7 -delete
"""

# Pipe at end of line instead of a backslash — also valid shell, also invisible
# to a line-bound matcher.
PIPE_AT_EOL_DEFECTS = """#!/bin/bash
set -u
pg_dump -U x x |
  gzip -9 > /var/backups/x.sql.gz
"""

# A bare-wildcard retention in a script with NO dump pipeline. Used to be
# unreachable: the check only ran inside check_pipe_pattern.
WILDCARD_WITHOUT_PIPE = """#!/bin/bash
set -euo pipefail
find /var/backups -name '*' -mtime +7 -delete
"""

# A dump piped into something the gate cannot classify. Must be reported as
# unparsed and FAIL — "no recognizable pattern" must never read as "nothing to
# check", which is precisely how K2-04 stayed green.
UNCLASSIFIABLE_PIPE = """#!/bin/bash
set -euo pipefail
pg_dump -U x x | zstd -19 -o /var/backups/x.sql.zst
"""

# A commented-out pipeline runs nothing, so it must neither trigger the checks
# nor fill the denominator.
COMMENTED_OUT_PIPE = """#!/bin/bash
set -euo pipefail
# pg_dump -U x x | gzip > /var/backups/x.sql.gz
echo "disabled"
"""


def run_gate_against(files, baseline=1):
    """Drive the gate's REAL main() against a fixture tree.

    Returns (exit_code, stdout). Driving main() rather than a hand-copied loop
    is the point: the denominator, the unparsed counter and the coverage
    baseline live there, and a self-test that cannot reach them certifies the
    half that was never in question."""
    with tempfile.TemporaryDirectory() as tmp:
        tmp_root = pathlib.Path(tmp)
        scripts_dir = tmp_root / "scripts"
        infra_dir = tmp_root / "infra" / "server" / "scripts"
        scripts_dir.mkdir(parents=True)
        infra_dir.mkdir(parents=True)
        for name, content in files.items():
            (scripts_dir / name).write_text(content)

        orig = (gate.ROOT, gate.SCAN_DIRS, gate.PIPE_PATTERN_BASELINE)
        gate.ROOT = tmp_root
        gate.SCAN_DIRS = ["scripts", "infra/server/scripts"]
        gate.PIPE_PATTERN_BASELINE = baseline
        buf = io.StringIO()
        code = 0
        try:
            with contextlib.redirect_stdout(buf):
                gate.main()
        except SystemExit as exc:
            code = exc.code or 0
        finally:
            gate.ROOT, gate.SCAN_DIRS, gate.PIPE_PATTERN_BASELINE = orig
        return code, buf.getvalue()


def fail(msg):
    print(f"FAIL: {msg}", file=sys.stderr)
    sys.exit(1)


# N-03 (2026-07-30): die Schlusszeile druckte `18 cases` als Literal, waehrend
# 17 PASS-Zeilen liefen — dieselbe Klasse wie der Docstring-31/Laufzeit-42 und
# die hartkodierte 2 im gpg-Test, die derselbe Diff auf `$TOTAL` umgestellt hat.
# Die Zahl wird jetzt gezaehlt statt korrigiert: eine korrigierte Konstante
# driftet beim naechsten `pass_`-Aufruf wieder weg, ein Zaehler nicht.
PASSED = 0


def pass_(msg):
    global PASSED
    PASSED += 1
    print(f"PASS: {msg}")


# 1. A fully hardened script passes clean.
code, out = run_gate_against({"backup-x.sh": HARDENED})
if code != 0:
    fail(f"hardened fixture flagged:\n{out}")
if "1 with a `pg_dump | gzip` pattern" not in out:
    fail(f"expected 1 pipe-pattern hit, got:\n{out}")
pass_("hardened fixture passes clean")

# 2a. Missing pipefail is caught.
code, out = run_gate_against({"backup-x.sh": NO_PIPEFAIL})
if code == 0 or "pipefail" not in out:
    fail(f"missing-pipefail fixture not caught:\n{out}")
pass_("missing `set -o pipefail` is caught")

# 2b. Missing size assertion is caught.
code, out = run_gate_against({"backup-x.sh": NO_SIZE_ASSERTION})
if code == 0 or "minimum-size" not in out:
    fail(f"missing-size-assertion fixture not caught:\n{out}")
pass_("missing minimum-size assertion is caught")

# 2c. Bare-wildcard retention is caught.
code, out = run_gate_against({"backup-x.sh": BARE_WILDCARD_RETENTION})
if code == 0 or "bare wildcard" not in out:
    fail(f"bare-wildcard-retention fixture not caught:\n{out}")
pass_("bare-wildcard retention delete is caught")

# 2d. Hardcoded PGPASSWORD is caught (even without the pipe pattern).
code, out = run_gate_against({"backup-x.sh": HARDCODED_SECRET}, baseline=0)
if code == 0 or "PGPASSWORD" not in out:
    fail(f"hardcoded-PGPASSWORD fixture not caught:\n{out}")
pass_("hardcoded PGPASSWORD is caught")

# 3. A script with no pg_dump|gzip pattern is never flagged for pipe checks.
code, out = run_gate_against({"unrelated.sh": UNRELATED_SCRIPT}, baseline=0)
if code != 0:
    fail(f"unrelated fixture wrongly flagged:\n{out}")
if "0 with a `pg_dump | gzip` pattern" not in out:
    fail(f"expected 0 pipe-pattern hits on unrelated script:\n{out}")
pass_("unrelated script triggers no false positive")

# ── K2-04: the wrapped forms ────────────────────────────────────────────────

# 4a. A hardened script whose pg_dump wraps over lines must still be RECOGNIZED
#     (counted in the denominator) and must still pass.
code, out = run_gate_against({"backup-x.sh": WRAPPED_HARDENED})
if code != 0:
    fail(f"wrapped hardened fixture flagged:\n{out}")
if "1 with a `pg_dump | gzip` pattern" not in out:
    fail(f"line-wrapped pipeline was not recognized — K2-04 regression:\n{out}")
pass_("line-wrapped pg_dump pipeline is recognized and counted")

# 4b. The K2-04 fixture verbatim: all three defects behind a `\` wrap. Before
#     the fix this printed "OK — 3 with a pg_dump|gzip pattern" and rc=0.
code, out = run_gate_against({"backup-x.sh": WRAPPED_ALL_THREE_DEFECTS})
missing = [k for k in ("pipefail", "minimum-size", "bare wildcard") if k not in out]
if code == 0 or missing:
    fail(f"line-wrapped defects not caught (missing {missing}):\n{out}")
pass_("line-wrapped pg_dump: all three defects caught")

# 4c. Same, with the pipe at the end of the line instead of a backslash.
code, out = run_gate_against({"backup-x.sh": PIPE_AT_EOL_DEFECTS})
if code == 0 or "pipefail" not in out:
    fail(f"pipe-at-end-of-line form not caught:\n{out}")
pass_("pipe-at-end-of-line pg_dump form is recognized")

# 4d. Bare-wildcard retention in a script with NO pipeline — the check used to
#     live inside check_pipe_pattern and could not reach this.
code, out = run_gate_against({"cleanup.sh": WILDCARD_WITHOUT_PIPE}, baseline=0)
if code == 0 or "bare wildcard" not in out:
    fail(f"bare-wildcard delete outside a dump script not caught:\n{out}")
pass_("bare-wildcard delete is caught in a script without a dump pipeline")

# 5. A dump piped into an unclassifiable command must FAIL as unparsed, not
#    pass as "nothing to check".
code, out = run_gate_against({"backup-x.sh": UNCLASSIFIABLE_PIPE}, baseline=0)
if code == 0 or "cannot classify" not in out:
    fail(f"unclassifiable dump pipeline was not reported as unparsed:\n{out}")
pass_("unclassifiable dump pipeline fails as unparsed, not as clean")

# 6. A commented-out pipeline is neither checked nor counted — otherwise the
#    gate measures writing style instead of behaviour.
code, out = run_gate_against({"backup-x.sh": COMMENTED_OUT_PIPE}, baseline=0)
if code != 0:
    fail(f"commented-out pipeline wrongly flagged:\n{out}")
if "0 with a `pg_dump | gzip` pattern" not in out:
    fail(f"commented-out pipeline wrongly counted in the denominator:\n{out}")
pass_("commented-out pipeline is neither checked nor counted")

# 7. The coverage ratchet: a recognized pipeline dropping out of the checked set
#    must go red, not quietly reduce the denominator (K2-04 one level up).
code, out = run_gate_against({"backup-x.sh": HARDENED}, baseline=2)
if code == 0 or "SHRANK" not in out:
    fail(f"coverage ratchet did not fire:\n{out}")
pass_("coverage ratchet fires when the checked set shrinks")

# 8. R-02: a PATH-QUALIFIED pg_dump must land in a counter. Before the fix the
#    anchor class excluded `/`, so every one of these fell out of ALL THREE
#    counters and the gate printed OK over an unchecked backup.
#    N-04: `scripts/pg_dump` — bare relative, no `./` — was still outside every
#    counter after the R-02 fix, because R-02 took in the reported VARIANT
#    (an absolute path) instead of the FORM. It is in the list now, and so are
#    the two positions that decide whether it is code or prose (case 8d).
PATH_FORMS = {
    "/usr/bin/pg_dump": "/usr/bin/pg_dump",
    "./scripts/pg_dump": "./scripts/pg_dump",
    "${BIN}/pg_dump": "${BIN}/pg_dump",
    '"$BIN_DIR"/pg_dump': '"$BIN_DIR"/pg_dump',
    '$(dirname "$0")/pg_dump': '$(dirname "$0")/pg_dump',
    "scripts/pg_dump (bare relative, N-04)": "scripts/pg_dump",
    "infra/server/scripts/pg_dump (bare relative, N-04)":
        "infra/server/scripts/pg_dump",
    "sudo -u postgres /usr/bin/pg_dump (argument position, absolute)":
        "sudo -u postgres /usr/bin/pg_dump",
}
for label, call in PATH_FORMS.items():
    body = f'#!/usr/bin/env bash\nset -e\n{call} --no-owner "$DB" | zstd -19 -o "$OUT"\n'
    code, out = run_gate_against({"backup-x.sh": body}, baseline=0)
    if code == 0 or "cannot classify" not in out or "unparsed: 1" not in out:
        fail(f"path-qualified form {label} did not reach the unparsed counter:\n{out}")
pass_(f"path-qualified pg_dump reaches the counters ({len(PATH_FORMS)} forms, R-02)")

# 8b. Same forms, but piped into gzip: they must be FULLY checked, i.e. show up
#     in `pipeline checked` and get the pipefail/size verdict — not in unparsed.
for label, call in PATH_FORMS.items():
    body = f'#!/usr/bin/env bash\nset -e\n{call} --no-owner "$DB" | gzip -9 > "$OUT"\n'
    code, out = run_gate_against({"backup-x.sh": body}, baseline=0)
    if code == 0 or "pipefail" not in out or "1 with a recognized dump pipeline" not in out:
        fail(f"path-qualified {label} | gzip was not fully checked:\n{out}")
pass_("path-qualified `pg_dump | gzip` is fully checked, not unparsed (R-02)")

# 8c. The counter-probe that makes 8 non-vacuous: prose mentioning a pg_dump
#     path must STILL not count. A wider anchor that swallows this would trade
#     one blind spot for the interface-ratchet defect (counting prose as code).
PROSE = (
    '#!/usr/bin/env bash\n'
    'set -e\n'
    'echo "siehe docs/pg_dump fuer die Anleitung"\n'
    "grep 'host pg_dump' /etc/hba.conf\n"
    "printf '%s\\n' 'host pg_dump/pg_restore'\n"
)
code, out = run_gate_against({"notes.sh": PROSE}, baseline=0)
if code != 0:
    fail(f"prose mentioning pg_dump was flagged:\n{out}")
for counter in ("0 with a `pg_dump | gzip` pattern", "unparsed         : 0",
                "pg_dump, no pipe : 0"):
    if counter not in out:
        fail(f"prose leaked into a counter (missing {counter!r}):\n{out}")
pass_("prose naming a pg_dump path stays out of every counter (R-02 counter-probe)")

# 8d. N-04: the REMAINING limit, pinned in both directions.
#
#     Taking in `scripts/pg_dump` means deciding what a bare word with a slash
#     is. In COMMAND position (line start, or after `| ; & ( ) { }`) it is a
#     command; after mere whitespace it is an argument, and there `sudo -u
#     postgres scripts/pg_dump` is indistinguishable from `echo "siehe
#     docs/pg_dump …"`. So the gate looks away there — deliberately, printed in
#     `blind by design`, and pinned here so a later widening has to delete this
#     case on purpose rather than discover the trade-off again. The same call
#     WITH a leading `/ . ~ $` is recognized (case 8/8b above), so the gap is
#     exactly bare + relative + argument position.
BLIND_ARG_POSITION = (
    '#!/usr/bin/env bash\n'
    'set -e\n'
    'sudo -u postgres scripts/pg_dump --no-owner "$DB" | zstd -19 -o "$OUT"\n'
)
code, out = run_gate_against({"backup-x.sh": BLIND_ARG_POSITION}, baseline=0)
if code != 0:
    fail(f"argument-position bare relative path: expected the NAMED blind spot "
         f"(green), got rc={code}. If this was widened on purpose, delete this "
         f"case and the matching line in `blind by design`:\n{out}")
if "unparsed         : 0" not in out:
    fail(f"blind-spot fixture unexpectedly reached a counter:\n{out}")
if "ARGUMENTposition" not in out:
    fail(f"the gate no longer PRINTS the argument-position limit it relies on "
         f"— a limit only the source knows is not a limit:\n{out}")
pass_("bare relative path in argument position: named limit, printed and pinned")

# 9. The gate must PRINT where it looks away, not only say so in the header.
code, out = run_gate_against({"backup-x.sh": HARDENED})
if "blind by design" not in out:
    fail(f"clean run does not print its blind spots:\n{out}")
pass_("a clean run names its own blind spots in the output")

# "cases" ist ab jetzt exakt definiert, damit die Zahl nicht auf eine zweite Art
# falsch werden kann: sie ist die Zahl der PASS-Zeilen oben, nicht die Zahl der
# geprueften FORMEN — Fall 8 und 8b fahren je eine Schleife ueber mehrere
# Schreibweisen und melden eine PASS-Zeile. Eine zweite gedruckte Zahl dafuer
# waere eine zweite Zahl, die driften kann.
print(f"ALL TESTS PASSED ({PASSED} cases — genau die PASS-Zeilen oben; "
      f"einzelne Faelle pruefen mehrere Schreibweisen)")
