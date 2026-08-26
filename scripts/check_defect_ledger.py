#!/usr/bin/env python3
"""Gate: der Defekt-Ledger (docs/sprints/defekte.tsv) und die Git-Historie duerfen
nicht auseinanderlaufen.

Der Ledger lag bis 2026-08-26 ausserhalb des Repos und war damit unversioniert,
ungespiegelt und fuer niemanden ausser dem Autor sichtbar. Beim Umzug ins Repo
fiel eine strukturelle Beschaedigung auf: zwei Datensaetze waren ohne
Zeilenumbruch verschmolzen, ein Symptomtext dadurch abgeschnitten. Genau diese
Klasse — und die Drift zwischen "im Ledger offen" und "im Code laengst gefixt" —
faengt dieses Gate.

Geprueft wird dreierlei:

  A. STRUKTUR   Jede Zeile hat exakt 11 Spalten, die ID ist eindeutig und nicht
                leer, Schweregrad und Status stammen aus dem erlaubten
                Wertebereich. (Faengt die Verschmelzungs-Klasse.)
  B. VORWAERTS  Nennt eine Commit-Betreffzeile eine Befund-ID des aktuellen
                Namensraums, MUSS der Ledger diese ID kennen und ein
                Fix_Commit eingetragen haben.
  C. RUECKWAERTS Traegt eine Zeile einen Commit-Hash, MUSS dieser Commit
                existieren.

Was dieses Gate NICHT prueft — bewusst, und es sagt es in seiner Ausgabe:
  * ob ein Fix fachlich richtig ist (dafuer sind Tests da),
  * Befund-IDA aus frueheren Audit-Runden (Namensraeume S1xx-*, R-*): die
    liegen in keinem maschinenlesbaren Ledger, ihre Zeilen gibt es nicht,
  * Zeilen, deren Fix_Commit nur Prosa enthaelt ("GEFIXT DURCH WELLE 1
    (ADR-0082)") — ohne Hash ist nichts nachschlagbar.

Jeder dieser Faelle wird GEZAEHLT und AUSGEWIESEN, nie stillschweigend
uebersprungen: ein Gate, das Eingaben still verwirft, meldet Erfolg fuer
Arbeit, die es nicht getan hat.

Exit 0 = sauber, 1 = Befund, 2 = kein Urteil moeglich (Ledger fehlt/unlesbar).
"""
from __future__ import annotations

import csv
import re
import subprocess
import sys
from pathlib import Path

LEDGER = Path("docs/sprints/defekte.tsv")

COLUMNS = ["ID", "Klasse", "Sev", "Ort", "Symptom", "Runde_gefunden",
           "Falsifikation", "Fix_Commit", "Live_weg", "Regressionstest", "Status"]
I_ID, I_SEV, I_FIX, I_STATUS = 0, 2, 7, 10

SEVERITIES = {"CRITICAL", "HIGH", "MED", "LOW"}
# Nachtraeglich aus der Commit-Historie rekonstruierte Befunde: gefixt und gemergt,
# aber nie triagiert. Der Wert ist erlaubt, damit die Luecke im Ledger STEHT statt
# durch einen erfundenen Schweregrad verdeckt zu werden — er wird eigens ausgewiesen.
UNTRIAGED = "UNTRIAGIERT"

# Namensraum der Runde, die dieser Ledger fuehrt. Commits, die IDs anderer
# Namensraeume nennen (S131-*, R-M11, ...), gehoeren zu Audit-Runden ohne
# maschinenlesbaren Ledger und werden als "nicht beurteilbar" gezaehlt.
OWNED_NS = re.compile(r"(?<![\w-])(R1-[A-Za-z0-9]+(?:-[A-Za-z0-9]+)+)(?![\w-])")
FOREIGN_NS = re.compile(r"(?<![\w-])(S1\d{2}-[A-Za-z0-9-]+|R-[A-Z]\d+)(?![\w-])")
HASH = re.compile(r"\b([0-9a-f]{7,40})\b")


def git(*args: str) -> str:
    return subprocess.run(["git", *args], capture_output=True, text=True,
                          check=True).stdout


def load(path: Path) -> tuple[list[str], list[list[str]], list[str]]:
    rows = list(csv.reader(path.read_text(encoding="utf-8").splitlines(), delimiter="\t"))
    if not rows:
        raise ValueError("Ledger ist leer")
    header, body = rows[0], [r for r in rows[1:] if r]
    problems: list[str] = []
    if header != COLUMNS:
        problems.append(f"Kopfzeile weicht ab: {header} != {COLUMNS}")
    return header, body, problems


def main() -> int:
    if not LEDGER.exists():
        print(f"check_defect_ledger: {LEDGER} fehlt — kein Urteil moeglich.")
        return 2
    try:
        _, body, problems = load(LEDGER)
    except (OSError, ValueError) as exc:
        print(f"check_defect_ledger: {LEDGER} unlesbar ({exc}) — kein Urteil moeglich.")
        return 2

    # ---------- A. Struktur ----------
    seen: dict[str, int] = {}
    for lineno, row in enumerate(body, start=2):
        if len(row) != len(COLUMNS):
            problems.append(
                f"Zeile {lineno}: {len(row)} Spalten statt {len(COLUMNS)} "
                f"(ID {row[0] if row else '?'!r}) — vermutlich zwei verschmolzene Datensaetze")
            continue
        fid = row[I_ID].strip()
        if not fid:
            problems.append(f"Zeile {lineno}: leere Befund-ID")
            continue
        if fid in seen:
            problems.append(f"Zeile {lineno}: ID {fid} schon in Zeile {seen[fid]}")
        seen[fid] = lineno
        if row[I_SEV].strip() not in SEVERITIES | {UNTRIAGED}:
            problems.append(
                f"Zeile {lineno} ({fid}): Schweregrad {row[I_SEV].strip()!r} "
                f"nicht aus {sorted(SEVERITIES | {UNTRIAGED})}")
        if not row[I_STATUS].strip():
            problems.append(f"Zeile {lineno} ({fid}): Status ist leer")

    intact = [r for r in body if len(r) == len(COLUMNS) and r[I_ID].strip()]
    by_id = {r[I_ID].strip(): r for r in intact}

    # ---------- B. Vorwaerts: Commit nennt ID -> Ledger muss sie kennen ----------
    #
    # Ein flacher Klon (actions/checkout ohne fetch-depth: 0) traegt nur einen
    # einzigen Commit. Das Gate faende dann keine einzige Befund-ID und meldete
    # gruen — Erfolg fuer Arbeit, die es nicht getan hat. Also: kein Urteil.
    if subprocess.run(["git", "rev-parse", "--is-shallow-repository"],
                      capture_output=True, text=True).stdout.strip() == "true":
        print("check_defect_ledger: flacher Klon (shallow) — die Commit-Historie "
              "fehlt, kein Urteil moeglich. In CI 'fetch-depth: 0' setzen.")
        return 2

    subjects = git("log", "--pretty=%h\x1f%s").splitlines()
    if len(subjects) < 2:
        print(f"check_defect_ledger: nur {len(subjects)} Commit(s) sichtbar — "
              "die Historie ist unvollstaendig, kein Urteil moeglich.")
        return 2
    commits = [s.split("\x1f", 1) for s in subjects if "\x1f" in s]

    mentioned: dict[str, str] = {}
    foreign = 0
    for sha, subject in commits:
        for fid in OWNED_NS.findall(subject):
            mentioned.setdefault(fid, sha)
        if FOREIGN_NS.search(subject):
            foreign += 1

    for fid, sha in sorted(mentioned.items()):
        row = by_id.get(fid)
        if row is None:
            problems.append(
                f"Commit {sha} nennt Befund {fid}, den der Ledger nicht kennt")
        elif not row[I_FIX].strip():
            problems.append(
                f"Befund {fid} hat einen Fix-Commit ({sha}), aber die Spalte "
                f"Fix_Commit im Ledger ist leer")

    # ---------- C. Rueckwaerts: Hash im Ledger -> Commit muss existieren ----------
    checked_hashes = 0
    prosa_only: list[str] = []
    for fid, row in sorted(by_id.items()):
        cell = row[I_FIX].strip()
        if not cell:
            continue
        hashes = HASH.findall(cell)
        if not hashes:
            prosa_only.append(fid)
            continue
        for h in hashes:
            checked_hashes += 1
            # ERREICHBARKEIT, nicht Existenz. `git cat-file -e` findet auch
            # Objekte, die nur noch lokal herumliegen, weil ein geloeschter
            # Branch sie einmal enthielt — im frischen CI-Klon gibt es sie
            # nicht. Ein Gate, das so prueft, ist auf der Maschine des Autors
            # gruen und in CI rot; genau das ist hier beim ersten Lauf
            # passiert (5 Vor-Merge-SHAs, beim Mergen durch neue ersetzt).
            # `merge-base --is-ancestor` fragt den Verlauf statt die Platte
            # und antwortet ueberall gleich.
            if subprocess.run(["git", "merge-base", "--is-ancestor", h, "HEAD"],
                              capture_output=True).returncode != 0:
                problems.append(
                    f"Befund {fid}: Fix_Commit nennt {h} — von HEAD aus nicht "
                    f"erreichbar. Entweder ein Vor-Merge-SHA (beim Mergen ersetzt) "
                    f"oder ein Tippfehler; den Commit eintragen, der im Verlauf steht")

    # ---------- Bilanz ----------
    open_by_sev = {s: 0 for s in SEVERITIES}
    for row in intact:
        if row[I_STATUS].strip().upper().startswith("OFFEN"):
            open_by_sev[row[I_SEV].strip()] = open_by_sev.get(row[I_SEV].strip(), 0) + 1
    untriaged = sum(1 for r in intact if r[I_SEV].strip() == UNTRIAGED)
    no_test = sum(1 for r in intact if r[9].strip().lower() in ("", "fehlt"))

    print(f"check_defect_ledger: {LEDGER}")
    print(f"  geprueft: {len(intact)} Befunde | uebersprungen: {len(body) - len(intact)} "
          f"(strukturell defekte Zeilen)")
    print(f"  Vorwaerts: {len(mentioned)} Befund-IDs in Commit-Betreffzeilen gefunden")
    print(f"  Rueckwaerts: {checked_hashes} Commit-Hashes nachgeschlagen | "
          f"{len(prosa_only)} Zeilen ohne nachschlagbaren Hash (nur Prosa)")
    print(f"  nicht beurteilbar: {foreign} Commits nennen IDs frueherer Audit-Runden "
          f"(kein maschinenlesbarer Ledger)")
    print(f"  offen: " + " · ".join(f"{s} {open_by_sev.get(s, 0)}"
                                    for s in ("CRITICAL", "HIGH", "MED", "LOW")))
    print(f"  ohne Regressionstest: {no_test} von {len(intact)}")
    if untriaged:
        print(f"  UNTRIAGIERT: {untriaged} Befunde sind gefixt und gemergt, "
              f"haben aber nie einen Schweregrad bekommen")

    if problems:
        print(f"\n  {len(problems)} BEFUND(E):")
        for p in problems:
            print(f"    - {p}")
        return 1
    print("\n  OK — Ledger und Git-Historie stimmen ueberein.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
