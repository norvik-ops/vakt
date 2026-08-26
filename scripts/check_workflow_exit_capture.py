#!/usr/bin/env python3
"""Gate: kein `$?`-Lesen nach einem Kommando, das `-e` bereits abgebrochen hätte.

WARUM DIESES GATE EXISTIERT
Zweimal in diesem Repo stand ein diagnostischer Block, der `$?` liest, in einem Schritt mit
`set -e` (bzw. Actions' Default-Shell `bash -e`). Unter `-e` bricht die Shell beim ersten
nicht-null Exit ab — der Block darunter ist damit toter Code, und zwar genau der Block, der
einen erwarteten Fehlschlag von einem echten unterscheiden soll:

  * `.github/workflows/deploy-sites.yml` — „ssh kaputt (255)" vs. „rrsync fehlt" wurde nie
    unterschieden; der Kommentar sagte ausdrücklich, eine Fehlmeldung würde den Leser in die
    falsche Richtung schicken, und es wurde gar nichts gemeldet.
  * `.github/workflows/ci.yml` (Fuzz) — der Kommentar sagte wörtlich „must not redden a healthy
    repo (Hard-Won: a gate that flakes gets disabled)", und der Code löste es nicht ein: eine
    Fuzz-Budget-Expiry hätte den Schritt beim ersten Target abgebrochen.

Beide Male war die Absicht im Kommentar korrekt und die Umsetzung falsch. Ein Kommentar ist keine
Durchsetzung — deshalb dieses Gate.

Richtige Form:  rc=0; out=$(cmd) || rc=$?      bzw.   cmd || rc=$?
Falsche Form:   out=$(cmd); rc=$?               bzw.   cmd
                                                       rc=$?

ZUR ANNAHME ÜBER `-e` — hier lag die erste Fassung dieses Gates falsch
GitHub Actions fährt `run:` mit `bash -e {0}`. `-e` ist also in JEDEM Schritt aktiv, OHNE dass
`set -e` dort steht — `deploy-sites.yml` setzt nur `set -uo pipefail` und war trotzdem betroffen.
Die erste Fassung feuerte nur bei explizitem `set -e` und meldete deshalb OK für genau die
Fundstelle, die sie motiviert hat: grün über einer Teilmenge. `-e` gilt hier deshalb als
DEFAULT-AKTIV; `set +e` hebt es auf, ein `shell:`-Override ohne `-e` ebenso.
Und: geprüft wird `$?` nach JEDEM Kommando, nicht nur nach einer Zuweisung — die Fundstelle in
`deploy-sites.yml` war ein nackter, über Zeilen fortgesetzter `ssh`-Aufruf.

WAS DIESES GATE NICHT PRÜFT — bewusst, und es sagt es auch in seiner Ausgabe
  * Nur `.github/workflows/*.yml`. Shell-Skripte unter `scripts/` und `infra/` sind NICHT im
    Suchraum: dort ist `set -e` nicht der Default, und die Formen sind vielfältiger.
    `scripts/backup-pg-target.sh` trug eine verwandte, aber andere Klasse (`exit` in einer
    Kommandosubstitution) — die fängt dieses Gate NICHT.
  * Es liest den Schritt-Text, nicht die Shell-Semantik. Ein `set +e` mitten im Schritt hebt die
    Bedingung real auf; das Gate erkennt genau diesen Fall und überspringt ihn — gezählt als
    `set+e` in der Ausgabe, nicht stillschweigend.
  * Mehrzeilige Kommandosubstitutionen (`$( ... \\\\n ... )`) werden als `unparsed` gezählt und
    ausgewiesen, nicht geraten.
  * `pipefail`-Fragen, `if`-Wrapper und `&&`-Ketten sind nicht Gegenstand.

Exit 0 = sauber (Nenner wird genannt) · Exit 1 = Verstoß, mit Datei:Zeile · Exit 2 = Nutzungsfehler.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

WORKFLOW_DIR = Path(".github/workflows")

# `VAR=$(...)` — einzeilig, schließende Klammer auf derselben Zeile.
ASSIGN_SUBST = re.compile(r"^\s*(?P<var>[A-Za-z_][A-Za-z0-9_]*)=\$\((?P<body>.*)\)\s*(?:;.*)?$")
# Mehrzeilige Substitution: öffnet, schließt aber nicht auf derselben Zeile.
ASSIGN_SUBST_OPEN = re.compile(r"^\s*[A-Za-z_][A-Za-z0-9_]*=\$\((?![^()]*\))")
READS_STATUS = re.compile(r"=\$\?")
# Die korrekte Form trägt `|| var=$?` auf derselben logischen Zeile.
SAFE_FORM = re.compile(r"\|\|\s*[A-Za-z_][A-Za-z0-9_]*=\$\?")
SET_E = re.compile(r"^\s*set\s+-[a-z]*e[a-z]*\b")
SET_PLUS_E = re.compile(r"^\s*set\s+\+[a-z]*e[a-z]*\b")
# Ein `run:`-Block beginnt hier; wir arbeiten rein textuell, weil die YAML-Struktur für die
# Frage irrelevant ist: entscheidend ist, was in EINEM Shell-Skript zusammen läuft.
RUN_START = re.compile(r"^(?P<indent>\s*)(?:- )?run:\s*\|")
STEP_START = re.compile(r"^\s*- (?:name|uses|run):")


def tracked_workflows() -> list[Path]:
    """Getrackte UND untrackte Workflows — `git ls-files` allein sieht neue Dateien nicht,
    und ein Gate, das lokal grün und in CI rot ist, ist schlimmer als eines, das immer rot ist."""
    try:
        out = subprocess.run(
            ["git", "ls-files", "--cached", "--others", "--exclude-standard", str(WORKFLOW_DIR)],
            capture_output=True, text=True, check=True,
        ).stdout
        files = [Path(line) for line in out.splitlines() if line.endswith((".yml", ".yaml"))]
    except (subprocess.CalledProcessError, FileNotFoundError):
        files = sorted(WORKFLOW_DIR.glob("*.y*ml"))
    return sorted({f for f in files if f.is_file()})


def scan(path: Path) -> tuple[list[tuple[int, str]], int, int]:
    """→ (Verstöße, Zahl der -e-abgeschalteten Ausnahmen, Zahl der nicht geparsten Substitutionen)."""
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    violations: list[tuple[int, str]] = []
    skipped_no_e = 0
    unparsed = 0

    in_run = False
    run_indent = 0
    e_active = True           # Actions-Default `bash -e {0}` — siehe Docstring
    last_cmd: tuple[int, str] | None = None
    continued = False

    for no, raw in enumerate(lines, start=1):
        m = RUN_START.match(raw)
        if m:
            in_run, run_indent = True, len(m.group("indent"))
            e_active = True
            last_cmd = None
            continued = False
            continue
        if in_run:
            if raw.strip():
                indent = len(raw) - len(raw.lstrip())
                if indent <= run_indent and (STEP_START.match(raw) or re.match(r"^\s*[a-z-]+:", raw)):
                    in_run = False
                    last_cmd = None
                    continue
        if not in_run or not raw.strip():
            continue

        stripped = raw.strip()

        # Fortsetzungszeile: gehört zum vorigen Kommando, setzt es nicht zurück.
        if continued:
            continued = stripped.endswith("\\")
            continue

        if SET_PLUS_E.match(stripped):
            e_active = False
            continue
        if SET_E.match(stripped):
            e_active = True
            continue
        if stripped.startswith("#"):
            continue

        # Liest diese Zeile $? ohne die sichere `|| var=$?`-Form?
        if READS_STATUS.search(stripped) and not SAFE_FORM.search(stripped):
            # Steht auf DIESER Zeile auch das Kommando (`cmd; rc=$?`), ist DIESE Zeile die
            # Fundstelle — nicht das vorige Kommando. Sonst nennt das Gate die falsche Stelle
            # und schickt den Leser in die falsche Richtung, genau der Fehler, den es meldet.
            before = stripped.split("=$?")[0]
            same_line_cmd = ";" in before and before.split(";")[0].strip() != ""
            culprit = (no, stripped) if same_line_cmd else last_cmd
            if culprit is not None:
                if e_active:
                    violations.append(culprit)
                else:
                    skipped_no_e += 1
            last_cmd = None
            continue

        if ASSIGN_SUBST_OPEN.match(raw) and not ASSIGN_SUBST.match(raw):
            unparsed += 1
            last_cmd = None
            continued = stripped.endswith("\\")
            continue

        # `cmd; rc=$?` auf EINER Zeile
        if READS_STATUS.search(stripped):
            if not SAFE_FORM.search(stripped):
                if e_active:
                    violations.append((no, stripped))
                else:
                    skipped_no_e += 1
            last_cmd = None
            continued = stripped.endswith("\\")
            continue

        # Jedes andere Kommando ist Kandidat für ein nachfolgendes $?.
        if SAFE_FORM.search(stripped) or stripped.endswith(("||", "&&", "then", "do", "{")):
            last_cmd = None
        elif re.match(r"^(if|elif|while|until|case|esac|fi|done|else|for|\}|\))\b", stripped):
            last_cmd = None
        else:
            last_cmd = (no, stripped)
        continued = stripped.endswith("\\")

    return violations, skipped_no_e, unparsed


def main() -> int:
    if len(sys.argv) > 1:
        print(f"usage: {sys.argv[0]}", file=sys.stderr)
        return 2

    files = tracked_workflows()
    if not files:
        print("check_workflow_exit_capture: FEHLER — keine Workflow-Datei gefunden. "
              "Ein leerer Nenner ist kein grünes Ergebnis.", file=sys.stderr)
        return 2

    total_violations: list[tuple[Path, int, str]] = []
    setpluse = unparsed = 0
    for f in files:
        v, s, u = scan(f)
        setpluse += s
        unparsed += u
        total_violations.extend((f, no, text) for no, text in v)

    denom = (f"{len(files)} Workflow-Datei(en) geprüft; "
             f"-e-abgeschaltete Ausnahmen: {setpluse}; nicht geparste Substitutionen: {unparsed}")

    if total_violations:
        print("check_workflow_exit_capture: FEHLER — "
              f"{len(total_violations)} Stelle(n) lesen $? nach einem Kommando in einem Schritt "
              "mit aktivem -e. Der Block darunter ist toter Code.", file=sys.stderr)
        for f, no, text in total_violations:
            print(f"  {f}:{no}: {text}", file=sys.stderr)
        print("\n  Richtig:  rc=0; out=$(cmd) || rc=$?", file=sys.stderr)
        print(f"  {denom}", file=sys.stderr)
        return 1

    print(f"check_workflow_exit_capture: OK — keine $?-Lesestelle hinter einem "
          f"Kommando bei aktivem -e. {denom}.")
    if unparsed:
        print(f"  Hinweis: {unparsed} mehrzeilige Substitution(en) nicht geprüft "
              "(siehe Docstring — bewusst ausgewiesen, nicht stillschweigend übersprungen).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
