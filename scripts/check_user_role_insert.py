#!/usr/bin/env python3
# Copyright (c) 2026 NorvikOps. All rights reserved.
# SPDX-License-Identifier: Elastic-2.0
"""
S132 Spur A — G2: jedes production-seitige `INSERT INTO users` MUSS die Spalte
`role` explizit in der Spaltenliste setzen.

Warum ein Gate und nicht nur ein Default: users.role ist die zweite, denormalisierte
Rollenquelle, die usermgmt.requireAdmin liest (die erste, maßgebliche ist
org_members.role → ADR-0077). Solange der DB-Default 'admin' war, wurde JEDES
INSERT ohne explizites role stillschweigend zum Admin — die D24-1-Privilege-
Escalation. Migration 249 flippt den Default auf 'viewer', aber ein Default schützt
nur vor der Escalation, nicht vor dem umgekehrten Fehler: ein Org-Gründungs-Insert,
der sich auf den Default verlässt, macht seinen Admin versehentlich zum Viewer.
Beide Richtungen sind nur zu verhindern, wenn die Rolle an der Insert-Grenze
AUSGESPROCHEN wird — genau das erzwingt dieses Gate.

Scope bewusst eng und ausgewiesen (die Gate-Lektion aus CLAUDE.md — ein Gate muss
sagen, was es NICHT anschaut):
  * Nur production-Go unter backend/ (internal/, services/, cmd/, pkg/).
  * `*_test.go` ist AUSGENOMMEN und wird gezählt: Testfixtures legen Wegwerf-Nutzer
    gegen eine frische DB an; ihre Rolle ergibt sich aus dem 249-Default und ist für
    die Escalation irrelevant. Der Zähler macht diese Ausnahme sichtbar, statt sie
    stumm zu verschlucken.
  * db/queries/**.sql und Migrationen tragen aktuell kein `INSERT INTO users`; träte
    dort je eines auf, würde es hier NICHT gesehen — deshalb ist der Scope oben
    benannt und nicht implizit.

Exit 0 = jedes gefundene production-Insert setzt role; Exit 1 = mindestens eines
nicht (mit Datei:Zeile). Findet das Gate 0 Inserts, ist das ein Defekt AM GATE
(Pfad/Muster falsch), kein sauberes Ergebnis — dann Exit 2.
"""

from __future__ import annotations

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCAN_ROOTS = [
    os.path.join(REPO_ROOT, "backend", "internal"),
    os.path.join(REPO_ROOT, "backend", "services"),
    os.path.join(REPO_ROOT, "backend", "cmd"),
    os.path.join(REPO_ROOT, "backend", "pkg"),
]

# Capture `INSERT INTO users ( <column-list> )`. DOTALL so a column list that
# spans lines is still captured whole; non-greedy up to the first closing paren,
# which is the column-list terminator for a normal INSERT.
INSERT_RE = re.compile(r"INSERT\s+INTO\s+users\s*\((?P<cols>.*?)\)", re.IGNORECASE | re.DOTALL)
# A bare `role` column token — not `role_id`, not `scim_...role`. Word-bounded and
# excludes an immediately following `_`.
ROLE_COL_RE = re.compile(r"(?<![A-Za-z0-9_])role(?![A-Za-z0-9_])")


def iter_go_files():
    for root in SCAN_ROOTS:
        if not os.path.isdir(root):
            continue
        for dirpath, _dirs, files in os.walk(root):
            for name in files:
                if name.endswith(".go"):
                    yield os.path.join(dirpath, name)


def main() -> int:
    checked = 0
    skipped_tests = 0
    violations: list[str] = []

    for path in iter_go_files():
        rel = os.path.relpath(path, REPO_ROOT)
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            src = fh.read()

        if "INSERT INTO users" not in src and "insert into users" not in src.lower():
            continue

        is_test = path.endswith("_test.go")

        for m in INSERT_RE.finditer(src):
            if is_test:
                skipped_tests += 1
                continue
            checked += 1
            cols = m.group("cols")
            if not ROLE_COL_RE.search(cols):
                line = src.count("\n", 0, m.start()) + 1
                violations.append(
                    f"{rel}:{line}: INSERT INTO users ohne explizite `role`-Spalte "
                    f"(Spalten: {cols.strip()!r})"
                )

    print(
        f"check_user_role_insert: {checked} production INSERT INTO users geprüft, "
        f"{skipped_tests} in *_test.go übersprungen (Fixtures, bewusst)."
    )

    if checked == 0:
        print(
            "FEHLER: kein einziges production `INSERT INTO users` gefunden — "
            "Scope-Pfad oder Muster stimmt nicht mehr. Das ist ein Gate-Defekt, "
            "kein sauberes Ergebnis.",
            file=sys.stderr,
        )
        return 2

    if violations:
        print(
            "\nFEHLER: jedes `INSERT INTO users` MUSS `role` explizit setzen "
            "(sonst greift der DB-Default — D24-1). Betroffen:\n",
            file=sys.stderr,
        )
        for v in violations:
            print("  " + v, file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
