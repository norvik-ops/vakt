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

import importlib.util
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


def run_gate_against(files):
    with tempfile.TemporaryDirectory() as tmp:
        tmp_root = pathlib.Path(tmp)
        scripts_dir = tmp_root / "scripts"
        infra_dir = tmp_root / "infra" / "server" / "scripts"
        scripts_dir.mkdir(parents=True)
        infra_dir.mkdir(parents=True)
        for name, content in files.items():
            (scripts_dir / name).write_text(content)

        orig_root, orig_dirs = gate.ROOT, gate.SCAN_DIRS
        gate.ROOT = tmp_root
        gate.SCAN_DIRS = ["scripts", "infra/server/scripts"]
        try:
            problems = []
            checked = 0
            pipe_hits = 0
            for path in gate.scripts():
                checked += 1
                text = path.read_text()
                pipe_problems, matched = gate.check_pipe_pattern(path, text)
                problems.extend(pipe_problems)
                if matched:
                    pipe_hits += 1
                problems.extend(gate.check_hardcoded_secret(path, text))
            return problems, checked, pipe_hits
        finally:
            gate.ROOT, gate.SCAN_DIRS = orig_root, orig_dirs


def fail(msg):
    print(f"FAIL: {msg}", file=sys.stderr)
    sys.exit(1)


def pass_(msg):
    print(f"PASS: {msg}")


# 1. A fully hardened script passes clean.
problems, checked, pipe_hits = run_gate_against({"backup-x.sh": HARDENED})
if problems:
    fail(f"hardened fixture flagged: {problems}")
if pipe_hits != 1:
    fail(f"expected 1 pipe-pattern hit, got {pipe_hits}")
pass_("hardened fixture passes clean")

# 2a. Missing pipefail is caught.
problems, _, _ = run_gate_against({"backup-x.sh": NO_PIPEFAIL})
if not any("pipefail" in p for p in problems):
    fail(f"missing-pipefail fixture not caught: {problems}")
pass_("missing `set -o pipefail` is caught")

# 2b. Missing size assertion is caught.
problems, _, _ = run_gate_against({"backup-x.sh": NO_SIZE_ASSERTION})
if not any("minimum-size" in p for p in problems):
    fail(f"missing-size-assertion fixture not caught: {problems}")
pass_("missing minimum-size assertion is caught")

# 2c. Bare-wildcard retention is caught.
problems, _, _ = run_gate_against({"backup-x.sh": BARE_WILDCARD_RETENTION})
if not any("bare wildcard" in p for p in problems):
    fail(f"bare-wildcard-retention fixture not caught: {problems}")
pass_("bare-wildcard retention delete is caught")

# 2d. Hardcoded PGPASSWORD is caught (even without the pipe pattern).
problems, _, _ = run_gate_against({"backup-x.sh": HARDCODED_SECRET})
if not any("PGPASSWORD" in p for p in problems):
    fail(f"hardcoded-PGPASSWORD fixture not caught: {problems}")
pass_("hardcoded PGPASSWORD is caught")

# 3. A script with no pg_dump|gzip pattern is never flagged for pipe checks.
problems, checked, pipe_hits = run_gate_against({"unrelated.sh": UNRELATED_SCRIPT})
if problems:
    fail(f"unrelated fixture wrongly flagged: {problems}")
if pipe_hits != 0:
    fail(f"expected 0 pipe-pattern hits on unrelated script, got {pipe_hits}")
pass_("unrelated script triggers no false positive")

print("ALL TESTS PASSED")
