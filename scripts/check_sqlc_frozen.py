#!/usr/bin/env python3
"""Friert den hand-gepflegten sqlc-Zustand in backend/internal/db ein.

WARUM DIESES GATE EXISTIERT
---------------------------
`sqlc generate` schlaegt lokal auf `main` fehl (vorbestehender Schema-Drift in
`db/queries/vaktcomply.sql`, Spalte `updated_at` — dokumentiert in CLAUDE.md und
in docs/adr/0078-sqlc-hand-maintained-einfrieren.md). Solange das so ist, sind
die generierten Dateien `backend/internal/db/*.sql.go` und vor allem das
`Querier`-Interface in `querier.go` **hand-gepflegt**: Wer eine neue Query
haelt, schreibt den Go-Code von Hand. Genau da driftet es — vor S133 fehlten 21
konkrete `*Queries`-Methoden (der ganze S86-BCM-Block: BIA/Recovery/Emergency)
im `Querier`-Interface. `go build` faengt das NICHT: Ein fehlender
Interface-Eintrag ist kein Compile-Fehler, nur eine unvollstaendige Abstraktion,
und jeder Aufrufer, der `db.Querier` statt `*db.Queries` nimmt (Tests, Mocks,
WithTx-Wrapper), sieht die Methode dann nicht.

WAS DAS GATE PRUEFT (die eingefrorene Invariante)
-------------------------------------------------
Bidirektionale Deckung zwischen den konkreten Methoden und dem Interface:

  (A) Jede konkrete `func (q *Queries) X(...)` in `internal/db/*.sql.go` hat
      einen Eintrag `X(...)` im `Querier`-Interface (schliesst die
      emit_interface-Luecke; faengt jede neue Query ohne Interface-Zeile).
  (B) Jeder `Querier`-Interface-Eintrag hat eine konkrete Methode gleichen
      Namens (faengt eine umbenannte/geloeschte Methode, deren Interface-Zeile
      als Leiche stehenbleibt — `var _ Querier = (*Queries)(nil)` faengt das
      zwar auch, aber erst zur Compile-Zeit und ohne Namensnennung).

WAS DAS GATE BEWUSST NICHT PRUEFT
---------------------------------
Strenge globale alphabetische Ordnung. sqlc EMITTIERT zwar alphabetisch, aber
das hand-gepflegte Interface hat historisch gewachsene, BEWUSSTE thematische
Append-Bloecke (S60 Schutzbedarf/Berechtigungskonzept, S86 BCM, „SecReflex
cross-table lookups" — siehe CLAUDE.md). Beim letzten Messen: 42 legitime
Ausreisser. Ein Gate, das strenge Alphabetik erzwingt, waere auf gesundem Repo
dauerhaft rot — und ein Gate, das bei gesundem Repo rot ist, wird abgeschaltet
statt gefixt (CLAUDE.md-Lesson). Die Ordnung wird daher nur als INFO gezaehlt
und ausgegeben, nicht als Fehler. Der Schutz gegen Drift ist die Deckung, nicht
die Sortierung.

Die groessere Blindstelle: `db/queries/*.sql` <-> handgeschriebener Go-Code.
Dieses Gate vergleicht ausschliesslich `internal/db/*.sql.go` gegen
`internal/db/querier.go`. Ein neues `-- name: X :one` in einer .sql-Datei OHNE
zugehoerige `func (q *Queries) X` ist fuer dieses Gate vollstaendig unsichtbar —
die direkte Entsprechung der „Handler ohne Route"-Klasse, die dieses Repo
viermal getroffen hat. Wer eine Query hinzufuegt, muss den Go-Code weiterhin von
Hand nachziehen (`sqlc generate` ist auf `main` kaputt, siehe ADR-0078); dass er
es getan hat, prueft hier niemand. Das PREPARE-Gate G5 (internal/rawsqlcov)
faengt die andere Richtung — SQL, das gegen das echte Schema nicht praepariert.

NENNER / NICHT-VAKUAER
----------------------
Das Gate zaehlt und gibt aus: Anzahl konkreter Methoden, Anzahl
Interface-Eintraege, Anzahl nicht parsbarer Interface-Zeilen (Skips). Ein Skip
> 0 ist ein FEHLER (eine nicht gelesene Interface-Zeile ist ein blinder Fleck
im Freeze — dieselbe Klasse wie `check_routes.py`, das ein Viertel der Calls
still uebersprang). Ein leerer Nenner (0 konkrete Methoden / 0 Interface-
Eintraege) ist ebenfalls ein FEHLER, sonst meldet das Gate „OK" ueber Nichts.

Exit 0 = eingefroren & konsistent, 1 = Drift gefunden.
"""
import glob
import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DB_DIR = os.path.join(REPO_ROOT, "backend", "internal", "db")
QUERIER = os.path.join(DB_DIR, "querier.go")

CONCRETE_RE = re.compile(r"^func \(q \*Queries\) ([A-Za-z0-9_]+)\(", re.M)
# Interface-Methodenzeile: Tab, Grossbuchstabe, Name, offene Klammer.
IFACE_METHOD_RE = re.compile(r"^\t([A-Z][A-Za-z0-9_]+)\(")


def collect_concrete():
    """name -> 'relpath:line' der ERSTEN Definition (fuer Fehlermeldungen)."""
    out = {}
    files = sorted(glob.glob(os.path.join(DB_DIR, "*.sql.go")))
    for path in files:
        if os.path.basename(path) == "querier.go":
            continue
        rel = os.path.relpath(path, REPO_ROOT)
        with open(path, encoding="utf-8") as fh:
            for i, line in enumerate(fh, 1):
                m = CONCRETE_RE.match(line)
                if m and m.group(1) not in out:
                    out[m.group(1)] = f"{rel}:{i}"
    return out


def collect_interface():
    """Liest den `type Querier interface { ... }`-Block.

    Gibt (methoden: dict name->line, skips: list[(line, text)], order: list[name])
    zurueck. Jede Zeile im Block, die weder Kommentar noch leer noch Klammer ist
    und NICHT als Methode parst, wird als Skip gemeldet — ein blinder Fleck.
    """
    methods = {}
    skips = []
    order = []
    with open(QUERIER, encoding="utf-8") as fh:
        lines = fh.readlines()
    in_block = False
    for i, raw in enumerate(lines, 1):
        stripped = raw.strip()
        if not in_block:
            if stripped.startswith("type Querier interface {"):
                in_block = True
            continue
        if stripped == "}":
            break
        if stripped == "" or stripped.startswith("//"):
            continue
        m = IFACE_METHOD_RE.match(raw)
        if m:
            methods[m.group(1)] = i
            order.append(m.group(1))
        else:
            skips.append((i, stripped))
    return methods, skips, order, in_block


def main():
    concrete = collect_concrete()
    iface, skips, order, found_block = collect_interface()

    errors = []

    if not found_block:
        errors.append("FATAL: `type Querier interface {` in querier.go nicht gefunden — Parser-Annahme gebrochen.")
    if not concrete:
        errors.append("FATAL: 0 konkrete `func (q *Queries)`-Methoden gefunden — leerer Nenner, Gate waere vakuaer.")
    if not iface:
        errors.append("FATAL: 0 `Querier`-Interface-Eintraege gefunden — leerer Nenner, Gate waere vakuaer.")

    # (A) konkrete Methode ohne Interface-Eintrag
    missing_iface = sorted(set(concrete) - set(iface))
    for name in missing_iface:
        errors.append(
            f"FEHLT IM INTERFACE: konkrete Methode `{name}` ({concrete[name]}) "
            f"hat keinen Eintrag in internal/db/querier.go (emit_interface-Luecke)."
        )

    # (B) Interface-Eintrag ohne konkrete Methode
    missing_concrete = sorted(set(iface) - set(concrete))
    for name in missing_concrete:
        errors.append(
            f"INTERFACE-LEICHE: `{name}` (querier.go:{iface[name]}) hat keine "
            f"konkrete `func (q *Queries) {name}` in internal/db/*.sql.go."
        )

    # Skips sind blinde Flecken -> Fehler
    for line, text in skips:
        errors.append(
            f"NICHT PARSBARE INTERFACE-ZEILE querier.go:{line}: {text!r} — "
            f"Parser konnte sie nicht als Methode lesen; Freeze haette hier einen blinden Fleck."
        )

    # INFO: alphabetische Ausreisser (kein Fehler, siehe Modul-Docstring)
    out_of_order = 0
    prev = None
    for name in order:
        if prev is not None and name < prev:
            out_of_order += 1
        prev = name

    print(f"sqlc-Freeze-Gate — Nenner:")
    print(f"  konkrete *Queries-Methoden: {len(concrete)}")
    print(f"  Querier-Interface-Eintraege: {len(iface)}")
    print(f"  nicht parsbare Interface-Zeilen (Skips): {len(skips)}")
    print(f"  alphabetische Ausreisser (INFO, kein Fehler): {out_of_order}")

    if errors:
        print(f"\nFEHLER ({len(errors)}):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print(f"\nOK — alle {len(concrete)} konkreten Methoden im Interface, "
          f"keine Leichen, keine Skips. sqlc-Zustand eingefroren.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
