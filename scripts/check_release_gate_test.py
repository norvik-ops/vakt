#!/usr/bin/env python3
"""Selbsttest fuer check_release_gate.py — die drei Abnahmen aus PROCESS.md P9.

  1. GRUEN auf einem Ledger, das die Bedingungen erfuellt.
  2. ROT bei einer echten Regression: ein CRITICAL wird geoeffnet, ein HIGH
     verliert seine Triage.
  3. ROT, wenn eine Verbesserung nicht festgehalten wird: eine Zurueckstellung
     ohne Datum bzw. ohne Begruendung zaehlt NICHT als Triage.

Zusaetzlich Nenner: ein leerer oder fehlender Ledger darf nicht als "taggen
erlaubt" durchgehen.
"""
from __future__ import annotations

import csv
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
GATE = REPO / "scripts" / "check_release_gate.py"
COLS = ["ID", "Klasse", "Sev", "Ort", "Symptom", "Runde_gefunden",
        "Falsifikation", "Fix_Commit", "Live_weg", "Regressionstest", "Status"]

results: list[tuple[str, bool]] = []


def row(fid: str, sev: str, status: str) -> list[str]:
    return [fid, "SILENT_FAILURE", sev, "backend/x.go:1", f"Symptom {fid}",
            "1", "BESTAETIGT", "", "", "fehlt", status]


def build(rows: list[list[str]]) -> Path:
    tmp = Path(tempfile.mkdtemp(prefix="release-gate-"))
    (tmp / "docs" / "sprints").mkdir(parents=True)
    (tmp / "scripts").mkdir()
    shutil.copy(GATE, tmp / "scripts" / GATE.name)
    with (tmp / "docs" / "sprints" / "defekte.tsv").open("w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, delimiter="\t", lineterminator="\n")
        w.writerow(COLS); w.writerows(rows)
    return tmp


def run(box: Path, *args: str) -> tuple[int, str]:
    proc = subprocess.run([sys.executable, str(box / "scripts" / GATE.name), *args],
                          cwd=box, capture_output=True, text=True)
    return proc.returncode, proc.stdout + proc.stderr


def check(name: str, ok: bool, out: str = "") -> None:
    results.append((name, ok))
    print(f"  {'PASS' if ok else 'FAIL'}  {name}")
    if not ok and out:
        print("        " + out.strip().replace("\n", "\n        ")[:400])


SAUBER = [
    row("A-1", "CRITICAL", "GEMERGT — Fix in abc1234"),
    row("A-2", "HIGH", "OFFEN — ZURUECKGESTELLT 2026-08-26, braucht Serverzugriff, Stefan"),
    row("A-3", "MED", "OFFEN — noch nicht angeschaut"),
    row("A-4", "LOW", "OFFEN"),
]

# ---------------------------------------------------------------- Abnahme 1
print("Abnahme 1 — gruen, wenn die Bedingungen erfuellt sind")
box = build(SAUBER); rc, out = run(box)
check("sauberer Stand ist taggbar", rc == 0 and "Tag erlaubt" in out, out)
check("Gate nennt seinen Nenner", "geprueft:" in out and "offen:" in out, out)
check("MED offen blockiert nicht", "MED 1" in out and rc == 0, out)
check("LOW offen blockiert nicht", "LOW 1" in out and rc == 0, out)
shutil.rmtree(box)

# ---------------------------------------------------------------- Abnahme 2
print("\nAbnahme 2 — rot bei einer echten Regression")

box = build(SAUBER[:0] + [row("B-1", "CRITICAL", "OFFEN — Restore-Drill scheitert")] + SAUBER[1:])
rc, out = run(box)
check("ein offener CRITICAL blockiert", rc == 1 and "NICHT TAGGEN" in out, out)
check("Gate NENNT den blockierenden Befund", "B-1" in out, out)
shutil.rmtree(box)

box = build([SAUBER[0], row("B-2", "HIGH", "OFFEN"), SAUBER[2], SAUBER[3]])
rc, out = run(box)
check("ein untriagierter HIGH blockiert", rc == 1 and "B-2" in out, out)
shutil.rmtree(box)

# ---------------------------------------------------------------- Abnahme 3
print("\nAbnahme 3 — rot, wenn die Triage nur behauptet ist")

box = build([SAUBER[0], row("C-1", "HIGH", "OFFEN — ZURUECKGESTELLT, braucht Serverzugriff"),
             SAUBER[2], SAUBER[3]])
rc, out = run(box)
check("Zurueckstellung OHNE Datum ist keine Triage", rc == 1 and "C-1" in out, out)
shutil.rmtree(box)

box = build([SAUBER[0], row("C-2", "HIGH", "OFFEN — 2026-08-26"), SAUBER[2], SAUBER[3]])
rc, out = run(box)
check("Datum OHNE Begruendungswort ist keine Triage", rc == 1 and "C-2" in out, out)
shutil.rmtree(box)

box = build([SAUBER[0], row("C-3", "HIGH",
                            "OFFEN — AKZEPTIERT 2026-08-26: Router-Limit, Produktentscheidung, Stefan"),
             SAUBER[2], SAUBER[3]])
rc, out = run(box)
check("vollstaendige Triage laesst den Tag durch", rc == 0, out)
shutil.rmtree(box)

# ---------------------------------------------------------------- Nenner
print("\nNenner — Abwesenheit darf kein Erfolg sein")

box = build([]); rc, out = run(box)
check("leerer Ledger gibt kein Urteil (Exit 2)", rc == 2, out)
shutil.rmtree(box)

box = build(SAUBER); (box / "docs" / "sprints" / "defekte.tsv").unlink()
rc, out = run(box)
check("fehlender Ledger gibt kein Urteil (Exit 2)", rc == 2, out)
shutil.rmtree(box)

# ---------------------------------------------------------------- Escape-Hatch
print("\nEscape-Hatch — --lax-high wirkt, aber nur auf HIGH")

box = build([SAUBER[0], row("D-1", "HIGH", "OFFEN"), SAUBER[2], SAUBER[3]])
rc, out = run(box, "--lax-high")
check("--lax-high laesst untriagierte HIGH durch", rc == 0, out)
shutil.rmtree(box)

box = build([row("D-2", "CRITICAL", "OFFEN"), SAUBER[1], SAUBER[2], SAUBER[3]])
rc, out = run(box, "--lax-high")
check("--lax-high hebelt CRITICAL NICHT aus", rc == 1 and "D-2" in out, out)
shutil.rmtree(box)

failed = [n for n, ok in results if not ok]
print(f"\n{len(results) - len(failed)}/{len(results)} Abnahmen bestanden")
if failed:
    print("Fehlgeschlagen: " + ", ".join(failed))
sys.exit(1 if failed else 0)
