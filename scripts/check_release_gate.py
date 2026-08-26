#!/usr/bin/env python3
"""Liefer-Gate (PROCESS.md P7b): darf dieser Stand getaggt werden?

Das DoD je Story sagt "diese Aenderung ist fertig". Es sagt nie, ob der
GESAMTSTAND auslieferbar ist. Ohne pruefbare Antwort wird die Frage nach Gefuehl
beantwortet — und das Gefuehl sagt nein, solange irgendwo ein Befund offen ist.
Am 2026-08-26 lagen deshalb 200 Commits mit 21 behobenen kritischen Defekten
einen Monat lang auf einem Branch, waehrend Produktion die alte Fassung fuhr.

Geprueft wird gegen den Defekt-Ledger (docs/sprints/defekte.tsv):

  1. KEIN offener CRITICAL.        Harte Bedingung, keine Ausnahme.
  2. Jeder offene HIGH triagiert.  Triage heisst: Datum + Begruendung + Person
                                   im Statustext. "OFFEN" allein ist keine.
  3. MED/LOW blockieren nie.       Sie werden nur gezaehlt.

Was dieses Gate NICHT prueft — bewusst:
  * ob die Fixes fachlich richtig sind (dafuer sind Tests und CI da),
  * ob der Integrationsstand einen gruenen CI-Lauf hatte — das beantwortet
    scripts/ci-status.sh <sha>, das eine GitHub-API braucht und deshalb hier
    nicht eingebaut ist. /ship ruft beide nacheinander auf.
  * Befunde frueherer Audit-Runden ohne maschinenlesbaren Ledger.

Exit 0 = taggen erlaubt, 1 = nicht taggen, 2 = kein Urteil moeglich.
"""
from __future__ import annotations

import csv
import re
import sys
from pathlib import Path

LEDGER = Path("docs/sprints/defekte.tsv")
I_ID, I_SEV, I_STATUS = 0, 2, 10

# Eine Zurueckstellung ist nur dann eine Triage, wenn sie nachvollziehbar ist:
# ein Datum, damit sie verfaellt, und Prosa, die den Grund traegt.
DATE = re.compile(r"\b\d{4}-\d{2}-\d{2}\b")
DEFERRED = re.compile(r"ZURUECKGESTELLT|ZURÜCKGESTELLT|AKZEPTIERT|BEWUSST OFFEN",
                      re.IGNORECASE)


def main(argv: list[str]) -> int:
    strict_high = "--lax-high" not in argv

    if not LEDGER.exists():
        print(f"check_release_gate: {LEDGER} fehlt — kein Urteil moeglich.")
        return 2
    rows = list(csv.reader(LEDGER.read_text(encoding="utf-8").splitlines(), delimiter="\t"))
    body = [r for r in rows[1:] if len(r) > I_STATUS and r[I_ID].strip()]
    if not body:
        print("check_release_gate: Ledger ist leer — kein Urteil moeglich.")
        return 2

    open_rows = [r for r in body if r[I_STATUS].strip().upper().startswith("OFFEN")]
    crit = [r for r in open_rows if r[I_SEV].strip() == "CRITICAL"]
    high = [r for r in open_rows if r[I_SEV].strip() == "HIGH"]
    med = sum(1 for r in open_rows if r[I_SEV].strip() == "MED")
    low = sum(1 for r in open_rows if r[I_SEV].strip() == "LOW")

    untriaged_high = [r for r in high
                      if not (DEFERRED.search(r[I_STATUS]) and DATE.search(r[I_STATUS]))]

    print(f"check_release_gate: {LEDGER}")
    print(f"  geprueft: {len(body)} Befunde | offen: {len(open_rows)}")
    print(f"  offen nach Schweregrad: CRITICAL {len(crit)} · HIGH {len(high)} "
          f"· MED {med} · LOW {low}")
    print(f"  HIGH ohne Triage: {len(untriaged_high)} von {len(high)}")
    print("  MED/LOW blockieren nie (PROCESS.md P7b)")

    blocking: list[str] = []
    if crit:
        blocking.append(f"{len(crit)} offene(r) CRITICAL — harte Bedingung:")
        for r in crit:
            blocking.append(f"    {r[I_ID]}  {r[4][:90]}")
    if strict_high and untriaged_high:
        blocking.append(
            f"{len(untriaged_high)} offene HIGH ohne Triage. Triage heisst: im "
            f"Statustext ein Datum UND eine Begruendung "
            f"(ZURUECKGESTELLT/AKZEPTIERT). Beispiele:")
        for r in untriaged_high[:8]:
            blocking.append(f"    {r[I_ID]}  {r[4][:80]}")
        if len(untriaged_high) > 8:
            blocking.append(f"    … und {len(untriaged_high) - 8} weitere")

    if blocking:
        print("\n  NICHT TAGGEN:")
        for line in blocking:
            print(f"    - {line}" if not line.startswith("    ") else line)
        print("\n  Zuruecklegen ist erlaubt — aber es heisst, den Statustext im Ledger "
              "zu ergaenzen, nicht den Befund zu ignorieren.")
        return 1

    print("\n  OK — Liefer-Gate bestanden, Tag erlaubt.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
