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

K2-06 (2026-07-30) — was dieses Gate NICHT sah und NICHT zählte:
Sowohl der Vorfilter (`"INSERT INTO users" not in src`) als auch INSERT_RE
verlangten den NACKTEN Bezeichner `users` unmittelbar nach INTO. Ein
production-Insert als `INSERT INTO public.users (…)` oder `INSERT INTO "users"
(…)` OHNE role-Spalte wurde weder gemeldet noch als Skip gezählt — es fiel aus
dem Nenner heraus, den das Gate als Nicht-Vakuitäts-Beweis ausgibt. Gemessen:
drei Probe-Konstanten (qualifiziert / gequotet / nackt), gefangen wurde eine,
der Zähler ging von 10 auf 11.

Das war ein VARIANT_MISS: das Schwester-Gate chain_writer_guard_test.go hatte
exakt diese Lücke (`INSERT INTO public.audit_log`, `INSERT INTO "audit_log"`),
sie wurde als R1-F3A-09 gefunden und gefixt — hier blieb sie stehen. Die
Fundstelle wurde gepatcht, die Klasse nicht (I4). Dieselbe Korrektur ist im
selben Lauf auch in check_module_isolation.py eingezogen.

Zusätzlich zählt das Gate jetzt, was es NICHT beurteilen kann: ein `INSERT INTO
users`, dessen Spaltenliste nicht lesbar ist (INSERT … SELECT, DEFAULT VALUES,
dynamisch zusammengesetzt), ist `unparsed` und ein FEHLER — nicht "geprüft" und
nicht stillschweigend übergangen. Ein Insert, dessen Spalten niemand sehen kann,
ist genau der Fall, in dem der DB-Default greift.

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
  * Go-KOMMENTARE werden vor der Auswertung entfernt. Ohne das zählte die
    breitere Schreibweisen-Toleranz oben eine Prosa-Zeile mit
    ("… `INSERT INTO users` *mentions* the `role` column …",
    sso_jit_role_viewer_real_test.go:35) in den Skip-Nenner — ein Gate, das Prosa
    zählt, lügt (die interface-ratchet-Lektion). Was nach dem Entfernen übrig
    bleibt, steht in einem String-Literal und ist echtes SQL.
  * Ein Alias-Insert über eine VIEW (`INSERT INTO app_users …`, wo app_users auf
    users zeigt) wird nicht erkannt. Es existiert heute keine solche View; sollte
    eine dazukommen, ist dies der Ort, der nachgezogen werden muss.

Exit 0 = jedes gefundene production-Insert setzt role; Exit 1 = mindestens eines
nicht (mit Datei:Zeile) ODER mindestens eines ist unparsed. Findet das Gate 0
Inserts, ist das ein Defekt AM GATE (Pfad/Muster falsch), kein sauberes Ergebnis
— dann Exit 2.
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

# Der Tabellenbezug: optional schema-qualifiziert (`public.users`), optional
# gequotet (`"users"`), beides in beliebiger Kombination. K2-06/R1-F3A-09.
TABLE_RE = r'INSERT\s+INTO\s+(?:"?[a-z_][a-z0-9_]*"?\s*\.\s*)?"?users"?'
# Jedes `INSERT INTO users` in irgendeiner Schreibweise — der Nenner. Was hier
# trifft, MUSS anschließend entweder geprüft oder als unparsed gezählt werden.
ANY_INSERT_RE = re.compile(TABLE_RE, re.IGNORECASE)
# Capture `INSERT INTO users ( <column-list> )`. DOTALL so a column list that
# spans lines is still captured whole; non-greedy up to the first closing paren,
# which is the column-list terminator for a normal INSERT.
INSERT_RE = re.compile(TABLE_RE + r"\s*\((?P<cols>.*?)\)", re.IGNORECASE | re.DOTALL)
# A bare `role` column token — not `role_id`, not `scim_...role`. Word-bounded and
# excludes an immediately following `_`.
ROLE_COL_RE = re.compile(r"(?<![A-Za-z0-9_])role(?![A-Za-z0-9_])")


def strip_go_comments(src: str) -> str:
    """Kommentare durch Whitespace ersetzen, Zeilenstruktur und Offsets erhalten.

    Positionserhaltend, damit die gemeldeten Zeilennummern die des Originals
    bleiben — eine Fundstelle mit falscher Zeile ist eine Fundstelle, die niemand
    nachschlägt."""
    out, i, n = [], 0, len(src)
    while i < n:
        c = src[i]
        if c == "/" and i + 1 < n and src[i + 1] == "/":
            while i < n and src[i] != "\n":
                out.append(" ")
                i += 1
        elif c == "/" and i + 1 < n and src[i + 1] == "*":
            out.append("  ")
            i += 2
            while i + 1 < n and not (src[i] == "*" and src[i + 1] == "/"):
                out.append("\n" if src[i] == "\n" else " ")
                i += 1
            out.append("  ")
            i += 2
        elif c in ("`", '"'):
            # String-Literale unangetastet lassen: dort steht das SQL.
            quote = c
            out.append(c)
            i += 1
            while i < n and src[i] != quote:
                if quote == '"' and src[i] == "\\" and i + 1 < n:
                    out.append(src[i:i + 2])
                    i += 2
                    continue
                if quote == '"' and src[i] == "\n":
                    break
                out.append(src[i])
                i += 1
            if i < n:
                out.append(src[i])
            i += 1
        else:
            out.append(c)
            i += 1
    return "".join(out)


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
    unparsed: list[str] = []

    for path in iter_go_files():
        rel = os.path.relpath(path, REPO_ROOT)
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            src = strip_go_comments(fh.read())

        # Vorfilter über dieselbe Schreibweisen-Toleranz wie die Auswertung. Ein
        # Vorfilter, der enger ist als das Muster dahinter, verwirft Treffer,
        # bevor irgendein Zähler sie sieht — genau das war K2-06.
        if not ANY_INSERT_RE.search(src):
            continue

        is_test = path.endswith("_test.go")

        # Jede Fundstelle des Nenners einer Auswertung zuordnen. Was ANY_INSERT_RE
        # trifft, INSERT_RE an derselben Stelle aber nicht, hat keine lesbare
        # Spaltenliste — das ist `unparsed`, nicht "geprüft und in Ordnung".
        parsed_at = {m.start() for m in INSERT_RE.finditer(src)}
        for m in ANY_INSERT_RE.finditer(src):
            if is_test:
                skipped_tests += 1
                continue
            line = src.count("\n", 0, m.start()) + 1
            if m.start() not in parsed_at:
                unparsed.append(
                    f"{rel}:{line}: `INSERT INTO users` ohne lesbare Spaltenliste "
                    f"(INSERT … SELECT, DEFAULT VALUES oder dynamisch gebaut) — "
                    f"nicht prüfbar, also NICHT geprüft: {src[m.start():m.start() + 90].strip()!r}"
                )
                continue
            checked += 1

        for m in INSERT_RE.finditer(src):
            if is_test:
                continue
            cols = m.group("cols")
            if not ROLE_COL_RE.search(cols):
                line = src.count("\n", 0, m.start()) + 1
                violations.append(
                    f"{rel}:{line}: INSERT INTO users ohne explizite `role`-Spalte "
                    f"(Spalten: {cols.strip()!r})"
                )

    print(
        f"check_user_role_insert: {checked} production INSERT INTO users geprüft, "
        f"{skipped_tests} in *_test.go übersprungen (Fixtures, bewusst), "
        f"unparsed: {len(unparsed)}."
    )

    if checked == 0:
        print(
            "FEHLER: kein einziges production `INSERT INTO users` gefunden — "
            "Scope-Pfad oder Muster stimmt nicht mehr. Das ist ein Gate-Defekt, "
            "kein sauberes Ergebnis.",
            file=sys.stderr,
        )
        return 2

    rc = 0
    if violations:
        print(
            "\nFEHLER: jedes `INSERT INTO users` MUSS `role` explizit setzen "
            "(sonst greift der DB-Default — D24-1). Betroffen:\n",
            file=sys.stderr,
        )
        for v in violations:
            print("  " + v, file=sys.stderr)
        rc = 1

    if unparsed:
        print(
            "\nFEHLER: `INSERT INTO users` gefunden, dessen Spaltenliste dieses Gate "
            "nicht lesen kann. Ein Insert, dessen Spalten niemand sieht, ist genau der "
            "Fall, in dem der DB-Default greift — er darf nicht aus dem Nenner "
            "herausfallen. Spaltenliste ausschreiben oder das Muster erweitern:\n",
            file=sys.stderr,
        )
        for u in unparsed:
            print("  " + u, file=sys.stderr)
        rc = 1

    return rc


if __name__ == "__main__":
    sys.exit(main())
