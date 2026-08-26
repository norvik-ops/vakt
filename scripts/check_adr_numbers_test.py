#!/usr/bin/env python3
"""
Self-test for check_adr_numbers.py (ADR-Nummern-Dubletten-Gate).

Warum diese Datei existiert: CLAUDE.md verlangt drei Abnahmen pro Gate, nicht
eine — grün auf der Baseline, ROT auf der echten Regression MIT Namensnennung,
und ein gemeldeter Nenner statt stiller Leere. Ein Gate, dessen roter Pfad nie
gelaufen ist, beweist durch sein Grün nichts; genau daran hat sich dieses Repo
schon mehrfach verbrannt (check_routes.py übersprang still ein Viertel seiner
Eingabe, der Interface-Ratchet zählte Prosa).

Das Gate wird als PROZESS aufgerufen, nicht als Funktion: sein Vertrag ist der
Exit-Code plus die Meldung, und genau das prüft die CI-Zeile auch.

Läuft ohne Docker, ohne Netz: nur stdlib, nur temporäre Verzeichnisse.
Exit non-zero, wenn eine Abnahme scheitert.
"""

import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
GATE = HERE / "check_adr_numbers.py"
REPO_ADR = HERE.parent / "docs" / "adr"

FAILURES = []


def run_gate(adr_dir):
    p = subprocess.run(
        [sys.executable, str(GATE), "--adr-dir", str(adr_dir)],
        capture_output=True, text=True,
    )
    return p.returncode, p.stdout + p.stderr


def expect(name, adr_dir, code, must_contain=()):
    got, out = run_gate(adr_dir)
    if got != code:
        FAILURES.append(f"{name}: Exit {got} erwartet {code}\n{out}")
        return
    missing = [s for s in must_contain if s not in out]
    if missing:
        FAILURES.append(f"{name}: Meldung nennt {missing} nicht\n{out}")
        return
    print(f"PASS: {name}")


def adr_copy(td, name="adr"):
    """Wegwerf-Kopie des echten docs/adr — Mutationen laufen NIE im Repo."""
    dst = pathlib.Path(td) / name
    shutil.copytree(REPO_ADR, dst)
    return dst


# ── Abnahme 1: grün auf der Baseline (dem echten docs/adr) ───────────────────
expect("Baseline docs/adr ist eindeutig", REPO_ADR, 0,
       ["OK — jede Nummer genau einmal", "ADR-Datei(en)"])


# ── Abnahme 2: ROT bei einer eingebauten Dublette, MIT beiden Namen ──────────
# Das ist die gemessene Lage aus Audit v5b: fix/k4-money-path und
# fix/k5-fe-be-fieldnames legten beide ein 0080 an. Verschiedene Dateinamen,
# also merged git ohne Konflikt.
with tempfile.TemporaryDirectory() as td:
    d = adr_copy(td)
    (d / "0081-fe-be-feldnamen-oracle-ist-der-json-tag.md").write_text(
        "# ADR-0081: der Zwilling aus der Parallelspur\n", encoding="utf-8")
    expect("Dublette unter den Dateien ist ROT und nennt beide", d, 1,
           ["0081", "erneuerung-traegt-ihren-token-durch-konstruktion.md",
            "fe-be-feldnamen-oracle-ist-der-json-tag.md", "2× vergeben"])

# Gegenprobe zur Dublette: dieselbe Kopie OHNE den Zwilling muss grün sein —
# sonst wäre das Rot oben von einem Kopierfehler nicht zu unterscheiden.
with tempfile.TemporaryDirectory() as td:
    expect("dieselbe Kopie ohne Zwilling ist grün", adr_copy(td), 0, ["OK"])

# Die zweite Hälfte desselben Problems: zwei README-Zeilen unter einer Nummer.
with tempfile.TemporaryDirectory() as td:
    d = adr_copy(td)
    readme = d / "README.md"
    text = readme.read_text(encoding="utf-8")
    anchor = "| [0078](0078-sqlc-hand-maintained-einfrieren.md)"
    assert anchor in text, "Testanker nicht mehr in der README — Test anpassen"
    text = text.replace(
        anchor,
        "| [0081](0081-erneuerung-traegt-ihren-token-durch-konstruktion.md) | Zeile aus der Parallelspur | Accepted | — |\n" + anchor,
        1)
    readme.write_text(text, encoding="utf-8")
    expect("zwei README-Zeilen für eine Nummer sind ROT und nennen die Zeilennummern", d, 1,
           ["README-Tabelle hat 2 Zeilen für ADR-0081", "README.md:"])

# Umnummeriert, README-Zeile nicht mitgezogen: der Link zeigt ins Leere. Genau
# der Fehler, der bei BEF-3 zu vermeiden war.
with tempfile.TemporaryDirectory() as td:
    d = adr_copy(td)
    (d / "0081-erneuerung-traegt-ihren-token-durch-konstruktion.md").rename(
        d / "0082-erneuerung-traegt-ihren-token-durch-konstruktion.md")
    expect("umnummerierte Datei mit stehengelassener README-Zeile ist ROT", d, 1,
           ["existiert nicht", "0081"])

# Label und Ziel widersprechen sich (Nummer im Text mitgezogen, Link nicht).
with tempfile.TemporaryDirectory() as td:
    d = adr_copy(td)
    readme = d / "README.md"
    readme.write_text(
        readme.read_text(encoding="utf-8").replace(
            "| [0081](0081-erneuerung",
            "| [0082](0081-erneuerung", 1),
        encoding="utf-8")
    expect("Label ≠ verlinkte Nummer ist ROT", d, 1, ["beschriftet, verlinkt aber"])


# ── Abnahme 3: der Nenner — was nicht gelesen/zugeordnet werden konnte, ──────
#              wird GEZÄHLT und BENANNT, nicht still übersprungen.
with tempfile.TemporaryDirectory() as td:
    d = adr_copy(td)
    (d / "notizen-ohne-nummer.md").write_text("# kein ADR\n", encoding="utf-8")
    expect("Datei ohne NNNN--Präfix wird gezählt und benannt", d, 0,
           ["1 Datei(en) ohne NNNN--Präfix", "notizen-ohne-nummer.md", "NICHT auf "])

with tempfile.TemporaryDirectory() as td:
    d = pathlib.Path(td) / "leer"
    d.mkdir()
    expect("leeres Verzeichnis ist Exit 2, nicht grün", d, 2, ["leerer Nenner"])

expect("fehlendes Verzeichnis ist Exit 2", pathlib.Path("/nope/does/not/exist"), 2,
       ["kein Verzeichnis"])

with tempfile.TemporaryDirectory() as td:
    d = adr_copy(td)
    # README unlesbar machen (Rechte entziehen). Als root wirkt chmod nicht —
    # dann wird der Fall ehrlich übersprungen statt falsch grün gemeldet.
    readme = d / "README.md"
    readme.chmod(0o000)
    if os.access(readme, os.R_OK):
        print("SKIP: README-unlesbar (laeuft als root, chmod wirkungslos)")
    else:
        expect("unlesbare README ist ROT und wird benannt", d, 1,
               ["README nicht lesbar", "Tabellenhälfte ungeprüft"])
    readme.chmod(0o644)


if FAILURES:
    print("\n".join(["", "FEHLGESCHLAGENE ABNAHMEN:"] + FAILURES))
    sys.exit(1)
print("\nalle Abnahmen bestanden")
