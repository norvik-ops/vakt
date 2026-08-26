#!/usr/bin/env python3
"""
check_adr_numbers.py — ADR-Nummern sind eindeutig (Audit v5b, Spur K4 / BEF-3).

Zwei ADR-Dateien mit derselben Nummer sind ein Merge, der NICHT konfligiert: die
Dateinamen unterscheiden sich, git legt beide ab, und danach hat das Repo zwei
Entscheidungen unter einer Nummer. Genau das ist dreimal passiert:

  * S132–135: drei Stories griffen unabhängig ADR-0077.
  * Audit v5b: `fix/k4-money-path` und `fix/k5-fe-be-fieldnames` legten beide ein
    ADR-0080 an (zentral neu vergeben: K4 -> 0081).

Beide Male war die Ursache dieselbe Anweisung an parallele Worker („nimm die
nächste freie Nummer") vom selben Basis-Commit aus. Ein Gate ist die einzige
Stelle, die das VOR dem Merge sieht — der Compiler sieht es nie, `git merge` sieht
es nie, und der Mensch sieht es erst, wenn jemand die falsche ADR liest.

Geprüft wird:

  1. DATEIEN — keine zwei `docs/adr/NNNN-*.md` mit derselben Nummer NNNN.
     Bei einem Treffer werden BEIDE (alle) Dateinamen genannt: eine Meldung
     „0080 ist doppelt" ohne Namen lässt genau die Arbeit übrig, die weh tut.
  2. README-TABELLE — keine zwei Zeilen mit derselben Nummer (der Textkonflikt
     im selben Hunk ist die zweite Hälfte desselben Problems), und jede Zeile
     zeigt auf eine Datei, die (a) existiert und (b) mit derselben Nummer
     beginnt. Eine umnummerierte Datei mit einer nicht mitgezogenen
     README-Zeile ist ein toter Link, der wie ein gültiger aussieht.

Absichtlich NICHT geprüft: dass jede ADR-Datei eine README-Zeile hat. Die
Tabelle ist heute unvollständig (79 Dateien, 63 Zeilen) — dieses Gate soll die
Dubletten fangen, nicht eine Altlast rot färben, die niemand in diesem Schritt
aufräumt. Wer das ändert, hat 15 Nachträge vor sich, nicht ein Gate zu fixen.

Der NENNER wird gemeldet: gezählt wird, was gelesen wurde, und benannt, was
NICHT gelesen werden konnte (unlesbare Datei, `.md` in `docs/adr` ohne
Nummernpräfix). Ein Gate, das still nichts liest, ist grün ohne Aussage.

Läuft ohne Docker, ohne Netz, ohne Abhängigkeiten: nur stdlib, nur Dateien lesen.

Usage:  python3 scripts/check_adr_numbers.py [--adr-dir docs/adr]
Exit 0 = eindeutig, 1 = Dublette/kaputter Link, 2 = nichts gelesen (leerer Nenner).

Verdrahtung (haengt der Orchestrator ein, sobald die CI-Spur gemergt ist — dieser
Fix fasst .github/workflows/ci.yml nicht an), im Job `backend` neben die anderen
Python-Gates:

    - name: ADR-Nummern eindeutig (Dubletten aus parallelen Branches)
      run: python3 scripts/check_adr_numbers.py

    - name: ADR-Nummern-Gate self-test
      run: python3 scripts/check_adr_numbers_test.py

Braucht kein Go, keine DB, kein Docker, kein Netz — laeuft in jedem Job, der den
Baum ausgecheckt hat.
"""
import argparse
import re
import sys
from collections import defaultdict
from pathlib import Path

# Dateiname einer ADR: vier Ziffern, Bindestrich, Rest. `README.md` und alles
# ohne Nummernpräfix fällt bewusst durch und wird als „nicht zuordenbar"
# GEMELDET, nicht still übersprungen.
ADR_FILE_RE = re.compile(r"^(\d{4})-.+\.md$")

# Eine Tabellenzeile der README: | [0081](0081-titel.md) | … |
TABLE_ROW_RE = re.compile(r"^\|\s*\[(\d{4})\]\(([^)]+)\)")

# Dateien in docs/adr, die keine ADR sind und keine Meldung wert sind.
NOT_AN_ADR = {"README.md"}


def collect_files(adr_dir):
    """-> (numbers: {nnnn: [name, …]}, scanned: int, unassignable: [name, …])"""
    numbers = defaultdict(list)
    unassignable = []
    scanned = 0
    for path in sorted(adr_dir.iterdir()):
        if not path.is_file() or path.suffix != ".md":
            continue
        if path.name in NOT_AN_ADR:
            continue
        scanned += 1
        m = ADR_FILE_RE.match(path.name)
        if not m:
            unassignable.append(path.name)
            continue
        numbers[m.group(1)].append(path.name)
    return numbers, scanned, unassignable


def collect_table(readme):
    """-> (rows: [(nnnn, target, lineno)], read: bool, problem: str|None)"""
    try:
        text = readme.read_text(encoding="utf-8")
    except OSError as e:
        return [], False, f"{readme}: {e}"
    rows = []
    for lineno, line in enumerate(text.splitlines(), start=1):
        m = TABLE_ROW_RE.match(line)
        if m:
            rows.append((m.group(1), m.group(2), lineno))
    return rows, True, None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--adr-dir", default="docs/adr")
    args = ap.parse_args()

    adr_dir = Path(args.adr_dir)
    if not adr_dir.is_dir():
        print(f"FATAL: {adr_dir} ist kein Verzeichnis — das Gate hat nichts gelesen", file=sys.stderr)
        return 2

    numbers, scanned, unassignable = collect_files(adr_dir)
    readme = adr_dir / "README.md"
    rows, readme_read, readme_problem = collect_table(readme)

    failures = []

    # 1. Dubletten unter den Dateien.
    for nnnn in sorted(numbers):
        names = numbers[nnnn]
        if len(names) > 1:
            failures.append(
                f"ADR-Nummer {nnnn} ist {len(names)}× vergeben:\n"
                + "".join(f"    - {adr_dir}/{n}\n" for n in names)
                + "  git merged das OHNE Konflikt (verschiedene Dateinamen). Eine der beiden muss "
                  "zentral eine neue Nummer bekommen — Datei, README-Zeile und jede Nennung im "
                  "Code-/Doc-Kommentar mitziehen."
            )

    # 2. Dubletten in der README-Tabelle + Links, die ins Leere oder auf die
    #    falsche Nummer zeigen.
    if not readme_read:
        failures.append(f"README nicht lesbar: {readme_problem}")
    else:
        by_number = defaultdict(list)
        for nnnn, target, lineno in rows:
            by_number[nnnn].append((target, lineno))
        for nnnn in sorted(by_number):
            hits = by_number[nnnn]
            if len(hits) > 1:
                failures.append(
                    f"README-Tabelle hat {len(hits)} Zeilen für ADR-{nnnn}:\n"
                    + "".join(f"    - {readme}:{ln} -> {t}\n" for t, ln in hits)
                    + "  Zwei Zeilen unter einer Nummer heißt: zwei parallele Branches haben in "
                      "denselben Hunk geschrieben."
                )
        for nnnn, target, lineno in rows:
            if not (adr_dir / target).is_file():
                failures.append(
                    f"{readme}:{lineno}: Zeile für ADR-{nnnn} zeigt auf {target} — diese Datei "
                    f"existiert nicht. Wurde die ADR umnummeriert, ohne die Zeile mitzuziehen?"
                )
            elif not target.startswith(nnnn + "-"):
                failures.append(
                    f"{readme}:{lineno}: Zeile ist mit ADR-{nnnn} beschriftet, verlinkt aber "
                    f"{target}. Label und Datei müssen dieselbe Nummer nennen, sonst führt die "
                    f"Tabelle zur falschen Entscheidung."
                )

    # 3. Nenner. Erst melden, dann urteilen — auch im grünen Fall.
    print(f"check_adr_numbers: {scanned} ADR-Datei(en) in {adr_dir} gelesen, "
          f"{len(numbers)} Nummer(n), {len(rows)} README-Tabellenzeile(n)")
    if unassignable:
        # Kein Fehler (ein `notes.md` in docs/adr ist kein Defekt), aber es MUSS
        # dastehen: was das Gate nicht zuordnen konnte, hat es nicht geprüft.
        print(f"check_adr_numbers: {len(unassignable)} Datei(en) ohne NNNN--Präfix, also NICHT auf "
              f"Dubletten geprüft: {', '.join(unassignable)}")
    if not readme_read:
        print("check_adr_numbers: README-Tabelle NICHT gelesen — Tabellenhälfte ungeprüft", file=sys.stderr)

    if scanned == 0:
        print(f"FATAL: keine ADR-Datei in {adr_dir} gelesen — leerer Nenner, das Gate sagt nichts aus",
              file=sys.stderr)
        return 2

    if failures:
        print("\nFEHLER: ADR-Nummern sind nicht eindeutig\n", file=sys.stderr)
        for f in failures:
            print("  " + f.replace("\n", "\n  "), file=sys.stderr)
            print("", file=sys.stderr)
        return 1

    print("check_adr_numbers: OK — jede Nummer genau einmal, jede README-Zeile zeigt auf ihre Datei")
    return 0


if __name__ == "__main__":
    sys.exit(main())
