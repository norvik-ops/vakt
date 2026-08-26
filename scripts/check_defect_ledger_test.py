#!/usr/bin/env python3
"""Selbsttest fuer check_defect_ledger.py — die drei Abnahmen, die dieses
Projekt fuer jedes Gate verlangt:

  1. GRUEN auf dem echten Ledger (das Gate ist nicht dauerhaft rot).
  2. ROT bei einer echten Regression (das Gate ist nicht vakuoes) — je einmal
     fuer jede der drei Pruefungen A/B/C.
  3. ROT, wenn eine Verbesserung nicht festgehalten wird: ein Befund, dessen
     Fix committet ist, dessen Ledger-Zeile das aber verschweigt.

Zusaetzlich wird der Nenner geprueft: ein leerer Ledger darf NICHT als
"sauber" durchgehen.

Lauf: python3 scripts/check_defect_ledger_test.py   (Exit 0 = alle bestanden)
"""
from __future__ import annotations

import csv
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
GATE = REPO / "scripts" / "check_defect_ledger.py"
LEDGER = REPO / "docs" / "sprints" / "defekte.tsv"

results: list[tuple[str, bool, str]] = []


def run_gate(cwd: Path) -> tuple[int, str]:
    proc = subprocess.run([sys.executable, str(GATE)], cwd=cwd,
                          capture_output=True, text=True)
    return proc.returncode, proc.stdout + proc.stderr


def sandbox() -> Path:
    """Arbeitskopie des Repos: echtes .git (fuer die Commit-Suche), kopierter Ledger."""
    tmp = Path(tempfile.mkdtemp(prefix="ledger-gate-"))
    (tmp / "docs" / "sprints").mkdir(parents=True)
    (tmp / "scripts").mkdir()
    shutil.copy(LEDGER, tmp / "docs" / "sprints" / "defekte.tsv")
    shutil.copy(GATE, tmp / "scripts" / GATE.name)
    (tmp / ".git").symlink_to(REPO / ".git")
    return tmp


def load(p: Path) -> tuple[list[str], list[list[str]]]:
    rows = list(csv.reader(p.read_text(encoding="utf-8").splitlines(), delimiter="\t"))
    return rows[0], rows[1:]


def save(p: Path, header: list[str], body: list[list[str]]) -> None:
    with p.open("w", encoding="utf-8", newline="") as fh:
        w = csv.writer(fh, delimiter="\t", lineterminator="\n")
        w.writerow(header); w.writerows(body)


def check(name: str, ok: bool, detail: str = "") -> None:
    results.append((name, ok, detail))
    print(f"  {'PASS' if ok else 'FAIL'}  {name}" + (f" — {detail}" if detail and not ok else ""))


# ---------------------------------------------------------------- Abnahme 1
print("Abnahme 1 — gruen auf dem echten Ledger")
rc, out = run_gate(REPO)
check("Gate ist gruen auf dem unveraenderten Ledger", rc == 0, out.strip()[-300:])
check("Gate nennt seinen Nenner", "geprueft:" in out and "uebersprungen:" in out)
check("Gate weist Nicht-Beurteilbares aus", "nicht beurteilbar:" in out)

# ---------------------------------------------------------------- Abnahme 2
print("\nAbnahme 2 — rot bei einer echten Regression")

# A: Struktur — zwei Datensaetze verschmelzen (die Klasse, die den Umzug ausloeste)
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
raw = led.read_text(encoding="utf-8").split("\n")
merged = raw[1] + raw[2]
led.write_text("\n".join([raw[0], merged] + raw[3:]), encoding="utf-8")
rc, out = run_gate(box)
check("A/Struktur: verschmolzene Zeile wird rot", rc == 1 and "Spalten statt" in out, out[-300:])
check("A/Struktur: Gate NENNT die betroffene Zeile", "Zeile 2:" in out, out[-300:])
shutil.rmtree(box)

# A: Schweregrad ausserhalb des Wertebereichs
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, b = load(led); b[0][2] = "SCHLIMM"; save(led, h, b)
rc, out = run_gate(box)
check("A/Struktur: unbekannter Schweregrad wird rot", rc == 1 and "SCHLIMM" in out, out[-300:])
shutil.rmtree(box)

# A: doppelte ID
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, b = load(led); b.append(list(b[0])); save(led, h, b)
rc, out = run_gate(box)
check("A/Struktur: doppelte Befund-ID wird rot", rc == 1 and "schon in Zeile" in out, out[-300:])
shutil.rmtree(box)

# C: Fix_Commit nennt einen Commit, den es nicht gibt
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, b = load(led)
target = next(r for r in b if r[7].strip())
target[7] = "deadbee"
save(led, h, b)
rc, out = run_gate(box)
check("C/Rueckwaerts: erfundener Commit-Hash wird rot",
      rc == 1 and "deadbee" in out and "nicht erreichbar" in out, out[-300:])
shutil.rmtree(box)

# C: Ein Hash, den es zwar als Objekt gibt, der aber von HEAD aus NICHT
# erreichbar ist (Vor-Merge-SHA). Genau hier war das Gate beim ersten CI-Lauf
# lokal gruen und in CI rot: `git cat-file -e` findet Objekte, die nur noch von
# einem geloeschten Branch herumliegen — im frischen Klon gibt es sie nicht.
# Der Test greift in BEIDEN Welten: lokal ist das Objekt da und unerreichbar,
# in CI fehlt es ganz. Beide Male muss das Gate rot werden.
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, b = load(led)
next(r for r in b if r[7].strip())[7] = "ecec12b"   # ersetzt durch c35f641 beim Merge
save(led, h, b)
rc, out = run_gate(box)
check("C/Rueckwaerts: unerreichbarer Vor-Merge-SHA wird rot",
      rc == 1 and "nicht\n" not in out and "ecec12b" in out, out[-300:])
check("C/Rueckwaerts: das echte Gegenstueck ist erreichbar (Test nicht vakuoes)",
      subprocess.run(["git", "merge-base", "--is-ancestor", "c35f641", "HEAD"],
                     cwd=REPO, capture_output=True).returncode == 0)
shutil.rmtree(box)

# ---------------------------------------------------------------- Abnahme 3
print("\nAbnahme 3 — rot, wenn eine Verbesserung nicht festgehalten wird")

# B: Der Fix ist committet, die Ledger-Zeile verschweigt ihn
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, b = load(led)
victim = next(r for r in b if r[0] == "R1-36c-01")
victim[7] = ""
save(led, h, b)
rc, out = run_gate(box)
check("B/Vorwaerts: geleertes Fix_Commit trotz Fix-Commit wird rot",
      rc == 1 and "R1-36c-01" in out and "leer" in out, out[-300:])
shutil.rmtree(box)

# B: Der Befund ist ganz aus dem Ledger verschwunden
box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, b = load(led)
save(led, h, [r for r in b if r[0] != "R1-36c-01"])
rc, out = run_gate(box)
check("B/Vorwaerts: geloeschter Befund mit Fix-Commit wird rot",
      rc == 1 and "R1-36c-01" in out and "nicht kennt" in out, out[-300:])
shutil.rmtree(box)

# ---------------------------------------------------------------- Nenner
print("\nNenner — Abwesenheit darf kein Erfolg sein")

box = sandbox(); led = box / "docs" / "sprints" / "defekte.tsv"
h, _ = load(led); save(led, h, [])
rc, out = run_gate(box)
check("leerer Ledger geht nicht als sauber durch", rc != 0, out[-300:])
shutil.rmtree(box)

box = sandbox(); (box / "docs" / "sprints" / "defekte.tsv").unlink()
rc, out = run_gate(box)
check("fehlender Ledger meldet 'kein Urteil moeglich' (Exit 2)",
      rc == 2 and "kein Urteil" in out, out[-300:])
shutil.rmtree(box)

# ---------------------------------------------------------------- Bilanz
failed = [n for n, ok, _ in results if not ok]
print(f"\n{len(results) - len(failed)}/{len(results)} Abnahmen bestanden")
if failed:
    print("Fehlgeschlagen: " + ", ".join(failed))
sys.exit(1 if failed else 0)
