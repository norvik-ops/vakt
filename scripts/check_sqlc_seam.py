#!/usr/bin/env python3
"""Naht-Gate: `backend/db/queries/*.sql` <-> `backend/internal/db/*.sql.go`.

WARUM DIESES GATE EXISTIERT
---------------------------
ADR-0078 haelt `internal/db/*.sql.go` bewusst hand-gepflegt (die Regeneration
formt 126 Aufrufstellen in 8 Paketen um — reine Go-Typformung, kein einziger
belegter Laufzeitdefekt). Damit gibt es eine Naht, die niemand geprueft hat:
Wer eine Query in `db/queries/*.sql` aendert, muss den Go-Code von Hand
nachziehen. Tut er es nicht, driftet die Naht — und zwar STILL:

  * `go build` sieht nichts (der Go-Code ist in sich konsistent).
  * `go vet`/Unit-Tests sehen nichts (die Query laeuft, sie liefert nur weniger).
  * Das PREPARE-Gate G5 (`internal/rawsqlcov`) sieht nichts — es praepariert die
    committeten Consts gegen das echte Schema. Eine Query, die 9 statt 13
    Spalten selektiert, ist schema-VALIDE. Sie ist nur falsch.

`scripts/check_sqlc_frozen.py` (das Schwester-Gate) prueft ausschliesslich
`internal/db/*.sql.go` <-> `internal/db/querier.go` und nennt diese Naht in
seinem eigenen Docstring als seine groesste Blindstelle. Dieses Gate schliesst
sie.

Gemessener Anlass (Codeaudit v5b, ESK-6): 12 von 491 Queries waren gedriftet,
alle mit Wirkung — u. a. lieferten die vier `ck_bcp_plans`-Queries
`rto_hours`/`rpo_hours`/`schutzbedarfsklasse` nicht mehr aus, obwohl
`repository_bcp.go` sie liest: die API meldete fuer jeden BCP-Plan 0/0/0.

WARUM sqlc DAS ORAKEL IST UND NICHT EIN TEXTVERGLEICH
-----------------------------------------------------
Ein naiver Textvergleich `.sql` <-> Const traegt hier NICHT, und zwar nicht
wegen Rauschen, sondern weil er blind ist:

  34 der 491 Queries benutzen `SELECT *` / `RETURNING *` / `tbl.*`. sqlc
  expandiert das beim Generieren gegen das Schema in eine explizite
  Spaltenliste. **Alle 12 gefundenen Drifts liegen in genau diesen 34** — eine
  Spalte kam per Migration hinzu, `*` deckt sie automatisch ab, die
  hand-gepflegte explizite Liste im Const nicht.

  Die Verteilung ist das eigentliche Argument: **12 von 34 `*`-Queries waren
  gedriftet (35 %), 0 von den anderen 457.** Die Expansion ist nicht eine
  Fehlerquelle unter vielen, sie ist bisher die einzige.

  > Hinweis (2026-07-30, F3-Review): Hier stand zuerst „65". Diese Zahl kam aus
  > einem Grep, der JEDEN Asterisk zaehlte — `COUNT(*)`,
  > `interval_hours * INTERVAL '1 hour'`, Kommentar-Sterne. Die Entscheidung
  > traegt auch mit 34, die Messung trug nicht. Dieselbe Klasse wie der
  > Interface-Ratchet, der das englische Wort „any" in Kommentaren mitzaehlte
  > (CLAUDE.md, 2026-07-14): ein Gate, dessen gedruckte Zahl etwas anderes misst
  > als es behauptet. Nachgemessen kommentar- und literal-bereinigt, nur Sterne
  > in Select-/Returning-Listen-Position.

Ein Vergleicher, der `*` nicht expandieren kann, ist damit fuer die gesamte
Fehlerklasse blind, fuer die dieses Gate existiert — und auf diesen 34 Queries
zugleich dauerhaft rot. `*` selbst zu expandieren heisst, sqlcs Schema-Engine
nachzubauen (44+ Migrationen, `ALTER TABLE ADD COLUMN IF NOT EXISTS`,
physische Spaltenreihenfolge). Also nimmt dieses Gate sqlc selbst als Orakel:
es generiert in ein Wegwerf-Verzeichnis und vergleicht die entstandene
SQL-Zeichenkette mit der committeten.

WARUM NICHT `sqlc generate && git diff --exit-code`
---------------------------------------------------
Das waere der vollstaendige Check — und er ist unter ADR-0078 dauerhaft rot:
eine Vollregeneration aendert +6307/-3923 Zeilen und bricht 126 Aufrufstellen.
Ein Gate, das auf gesundem Repo rot ist, wird abgeschaltet statt gefixt
(CLAUDE.md-Lesson). Dieses Gate vergleicht deshalb ausschliesslich die **SQL**
— das ist die Naht — und ignoriert die **Go-Typformung** (Row-/Params-Struct-
Felder, `pgtype.X` vs. `string`, Embedding), die ADR-0078 bewusst einfriert.
Die Vollregeneration bleibt Folge-Story.

ZWEI SCHICHTEN, ZWEI NENNER
---------------------------
  Schicht A (ohne Werkzeug): Menge der `-- name: X :verb`-Header auf beiden
      Seiten. Faengt: neue Query in `.sql` ohne Go-Code (die direkte
      Entsprechung der „Handler ohne Route"-Klasse), Go-Leiche ohne `.sql`,
      geaenderter Verb (`:one` -> `:many`) ohne Go-Aenderung.
  Schicht B (braucht sqlc): normalisierte SQL je Query gegen das Orakel.
      Faengt Spalten-/Parameter-/Praedikat-Drift — die Klasse der 12.

WAS DAS GATE BEWUSST NICHT PRUEFT
---------------------------------
  * Go-Typformung (siehe oben) — das ist der eingefrorene Teil.
  * Whitespace INNERHALB eines String-Literals. Beide Seiten werden auf einzelne
    Leerzeichen normalisiert, weil die hand-gepflegte Seite frei umbrechen darf.
    Damit kommt `INTERVAL '1 hour'` -> `INTERVAL '1  hour'` durch. Das ist die
    einzige bekannte Umgehung; sie ist im F3-Review (2026-07-30) gefunden UND
    vermessen worden:
      - Richtung: verschweigt hoechstens Drift, erfindet nie welche. Ein Gate,
        das Drift halluziniert, waere schlimmer.
      - Reichweite: **0** Gleichheitsvergleiche gegen mehrwortige String-
        Literale im Bestand — es gibt heute keine Query, deren Semantik daran
        haengt.
      - NICHT umgehbar ist der gefaehrlichere Nachbarpfad: ein `--`-Kommentar
        plus Newline-Kollaps, um z. B. einen `org_id`-Filter unsichtbar zu
        entfernen. sqlc STRIPPT Kommentare aus dem Const, also kann diese
        Konstruktion die Orakel-Seite nicht faelschen.
    Wer hier schaerfer werden will, muss literal-bewusst normalisieren (Literale
    maskieren statt kollabieren) — nicht die Normalisierung abschaffen.
  * Ob eine `func (q *Queries) X` auch WIRKLICH den Const `x` ausfuehrt. Eine
    Copy-Paste-Verwechslung (`q.db.Query(ctx, listFoo, …)` im Rumpf von `GetBar`)
    ist fuer dieses Gate UND fuer check_sqlc_frozen.py unsichtbar: beide sehen
    nur, dass Const und Methode existieren, keines verfolgt die Zuordnung.
    Heute sind alle 491 korrekt gepaart (F3-Review, 2026-07-30), aber unbewacht.
    Wer das schliessen will, braucht einen AST-Schritt, der im Methodenrumpf den
    zweiten `q.db.*`-Parameter gegen den erwarteten Const-Namen prueft.
  * `internal/db/*.sql.go` <-> `querier.go` — das macht check_sqlc_frozen.py.
  * Ob die SQL gegen das echte Schema praepariert — das macht G5
    (`internal/rawsqlcov`).

Exit 0 = Naht dicht, 1 = Drift gefunden (mit Namensnennung),
2 = nicht vergleichbar (sqlc nicht verfuegbar / Orakel fehlgeschlagen).
Exit 2 ist ausdruecklich KEIN Erfolg: ein Gate, das nicht messen konnte, darf
nicht „OK" melden. Mit `--allow-missing-oracle` degradiert Schicht B zu einem
gezaehlten Skip (fuer Arbeitsplaetze ohne Docker) und der Lauf endet mit 0,
sofern Schicht A sauber ist — in CI wird das Flag NICHT gesetzt.
"""
import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile

DEFAULT_REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class Paths:
    """Alle Pfade relativ zu einem Repo-Root.

    Injizierbar, damit `check_sqlc_seam_test.py` das Gate gegen praeparierte
    Wegwerf-Repos fahren kann — ein Gate, dessen roter Pfad nie ausgefuehrt
    wurde, beweist durch Gruen nichts (Drei-Abnahmen-Regel).
    """

    def __init__(self, root):
        self.root = os.path.abspath(root)
        backend = os.path.join(self.root, "backend")
        self.queries = os.path.join(backend, "db", "queries")
        self.migrations = os.path.join(backend, "db", "migrations")
        self.db = os.path.join(backend, "internal", "db")
        self.sqlc_yaml = os.path.join(backend, "sqlc.yaml")

# Pin: identisch zu backend/sqlc.yaml-Erwartung und CLAUDE.md. `:latest` zeigt
# derzeit auf dieselbe Version, aber ein Orakel darf nicht wandern.
SQLC_IMAGE = "sqlc/sqlc:1.31.1"

# `const getFoo = ` + Backtick-SQL, dessen erste Zeile der -- name:-Header ist.
CONST_RE = re.compile(
    r"^const \w+ = `(-- name: (\w+) :(\w+)\n)(.*?)`$",
    re.M | re.S,
)
SQL_HEADER_RE = re.compile(r"^-- name:\s*(\w+)\s*:(\w+)\s*$", re.M)


def norm(sql):
    """Whitespace-normalisierte SQL ohne abschliessendes Semikolon."""
    return re.sub(r"\s+", " ", sql).strip().rstrip(";").strip()


def git_listed(paths, directory, suffix):
    """Tracked UND untracked-nicht-ignorierte Dateien.

    `git ls-files` allein sieht keine neuen, noch nicht getrackten Dateien —
    dann ist das Gate lokal gruen und in CI rot (CLAUDE.md-Lesson). Also
    ausdruecklich `--cached --others --exclude-standard`.
    """
    rel = os.path.relpath(directory, paths.root)
    out = subprocess.run(
        ["git", "-C", paths.root, "ls-files", "--cached", "--others",
         "--exclude-standard", "--", rel],
        capture_output=True, text=True, check=True,
    ).stdout.split("\n")
    return sorted(
        os.path.join(paths.root, p) for p in out
        if p.endswith(suffix)
    )


def parse_sql_files(paths):
    """path-Liste -> (dict name->(verb, normalisierte SQL, basename), skips).

    Die normalisierte SQL dieser Seite wird von Schicht B NICHT verglichen —
    Schicht B stellt Orakel gegen Go, nicht `.sql` gegen Go. Das ist Absicht:
    ein Eigenparser muesste `*` selbst expandieren und Parameter selbst
    nummerieren, also sqlc nachbauen, und wuerde damit genau die Fehlerklasse
    verfehlen, um die es geht. Der Wert bleibt hier stehen, weil er den
    Verb/Name-Vergleich der Schicht A traegt und die Diagnose lesbar macht;
    er ist bewusst nicht die Vergleichsgrundlage.
    """
    out, skips = {}, []
    for path in paths:
        base = os.path.basename(path)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        marks = list(SQL_HEADER_RE.finditer(src))
        if not marks:
            skips.append(f"{base}: keine `-- name:`-Header gefunden")
            continue
        for i, m in enumerate(marks):
            end = marks[i + 1].start() if i + 1 < len(marks) else len(src)
            name, verb = m.group(1), m.group(2)
            body = src[m.end():end]
            if name in out:
                skips.append(f"{base}: doppelter Query-Name `{name}`")
                continue
            out[name] = (verb, norm(body), base)
        # Gegenprobe: jede Nicht-Kommentar-, Nicht-Leerzeile muss innerhalb
        # eines Query-Blocks liegen. Text VOR dem ersten Header ist Vorspann.
        head = src[: marks[0].start()]
        for lineno, line in enumerate(head.split("\n"), 1):
            s = line.strip()
            if s and not s.startswith("--"):
                skips.append(f"{base}:{lineno}: SQL ausserhalb jedes Query-Blocks: {s[:60]!r}")
    return out, skips


def parse_go_files(paths):
    """path-Liste -> (dict name->(verb, normalisierte SQL, basename), skips)."""
    out, skips = {}, []
    for path in paths:
        base = os.path.basename(path)
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        found = 0
        for m in CONST_RE.finditer(src):
            name, verb, body = m.group(2), m.group(3), m.group(4)
            found += 1
            if name in out:
                skips.append(f"{base}: doppelter Const-Query-Name `{name}`")
                continue
            out[name] = (verb, norm(body), base)
        # Jede `const x = ` + Backtick-Zeile, die NICHT als Query geparst wurde,
        # ist ein blinder Fleck und wird gezaehlt — nicht stillschweigend
        # uebersprungen (CLAUDE.md: ein Gate, das still skippt, meldet Erfolg
        # fuer Arbeit, die es nicht getan hat).
        raw = len(re.findall(r"^const \w+ = `", src, re.M))
        if raw != found:
            skips.append(
                f"{base}: {raw - found} `const … = `-Block(e) nicht als "
                f"`-- name: X :verb`-Query parsbar"
            )
    return out, skips


def run_oracle(paths):
    """sqlc in ein Wegwerf-Verzeichnis generieren.

    Beruehrt den Arbeitsbaum NIE: kopiert sqlc.yaml + db/{queries,migrations}
    in ein tempdir; `out: internal/db` loest relativ zur Config auf, landet
    also unter tempdir/internal/db.

    Rueckgabe: (pfad-liste, fehler_oder_None). Bei Fehler pfad-liste == [].
    """
    if shutil.which("docker") is None:
        return [], "docker nicht im PATH — Orakel nicht ausfuehrbar"
    tmp = tempfile.mkdtemp(prefix="sqlc-seam-")
    try:
        shutil.copy(paths.sqlc_yaml, os.path.join(tmp, "sqlc.yaml"))
        os.makedirs(os.path.join(tmp, "db"), exist_ok=True)
        shutil.copytree(paths.queries, os.path.join(tmp, "db", "queries"))
        shutil.copytree(paths.migrations, os.path.join(tmp, "db", "migrations"))
        proc = subprocess.run(
            ["docker", "run", "--rm", "-v", f"{tmp}:/src", "-w", "/src",
             SQLC_IMAGE, "generate"],
            capture_output=True, text=True, timeout=900,
        )
        if proc.returncode != 0:
            msg = (proc.stderr or proc.stdout).strip()
            shutil.rmtree(tmp, ignore_errors=True)
            return [], f"`sqlc generate` fehlgeschlagen (exit {proc.returncode}): {msg[:800]}"
        outdir = os.path.join(tmp, "internal", "db")
        paths = sorted(
            os.path.join(outdir, f) for f in os.listdir(outdir)
            if f.endswith(".sql.go")
        )
        if not paths:
            shutil.rmtree(tmp, ignore_errors=True)
            return [], "Orakel erzeugte keine *.sql.go — Annahme ueber sqlc-Layout gebrochen"
        # Inhalte einlesen, dann aufraeumen: der Aufrufer braucht kein tempdir.
        data = [(p, open(p, encoding="utf-8").read()) for p in paths]
        shutil.rmtree(tmp, ignore_errors=True)
        return data, None
    except Exception as exc:  # pragma: no cover - Orakel-Infrastruktur
        shutil.rmtree(tmp, ignore_errors=True)
        return [], f"Orakel-Lauf abgebrochen: {exc}"


def parse_oracle(data):
    out, skips = {}, []
    for path, src in data:
        base = "ORAKEL/" + os.path.basename(path)
        for m in CONST_RE.finditer(src):
            name, verb, body = m.group(2), m.group(3), m.group(4)
            if name in out:
                skips.append(f"{base}: doppelter Query-Name `{name}`")
                continue
            out[name] = (verb, norm(body), base)
    return out, skips


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--allow-missing-oracle", action="store_true",
        help="Schicht B als gezaehlten Skip behandeln, wenn sqlc/docker fehlt "
             "(fuer lokale Arbeitsplaetze; in CI NICHT setzen).",
    )
    ap.add_argument(
        "--repo-root", default=DEFAULT_REPO_ROOT,
        help="Repo-Wurzel (nur fuer den Selbsttest; Default ist dieses Repo).",
    )
    args = ap.parse_args()

    paths = Paths(args.repo_root)
    sql_paths = git_listed(paths, paths.queries, ".sql")
    go_paths = git_listed(paths, paths.db, ".sql.go")

    errors = []
    skips = []

    sql_q, s1 = parse_sql_files(sql_paths)
    go_q, s2 = parse_go_files(go_paths)
    skips += s1 + s2

    print("Naht-Gate db/queries/*.sql <-> internal/db/*.sql.go")
    print(f"  .sql-Dateien (git-gelistet):    {len(sql_paths)}")
    print(f"  *.sql.go-Dateien (git-gelistet): {len(go_paths)}")
    print(f"  Queries in .sql:                 {len(sql_q)}")
    print(f"  Query-Consts in *.sql.go:        {len(go_q)}")

    # Leerer Nenner -> vakuaeres Gate.
    if not sql_paths or not go_paths:
        errors.append("FATAL: leerer Nenner (0 .sql- oder 0 *.sql.go-Dateien) — Gate waere vakuaer.")
    if not sql_q or not go_q:
        errors.append("FATAL: 0 Queries auf einer Seite geparst — Gate waere vakuaer.")

    # ── Schicht A: Namens-/Verb-Deckung ─────────────────────────────────────
    only_sql = sorted(set(sql_q) - set(go_q))
    only_go = sorted(set(go_q) - set(sql_q))
    for name in only_sql:
        verb, _, base = sql_q[name]
        errors.append(
            f"[A] GO-CODE FEHLT: `{name}` (:{verb}) steht in db/queries/{base}, "
            f"hat aber keinen `const … = \\`-- name: {name} …\\`` in internal/db/*.sql.go."
        )
    for name in only_go:
        verb, _, base = go_q[name]
        errors.append(
            f"[A] SQL-QUELLE FEHLT: Const `{name}` (:{verb}) steht in internal/db/{base}, "
            f"hat aber kein `-- name: {name}` in db/queries/*.sql (Leiche oder Umbenennung)."
        )
    both = sorted(set(sql_q) & set(go_q))
    for name in both:
        sv, _, sbase = sql_q[name]
        gv, _, gbase = go_q[name]
        if sv != gv:
            errors.append(
                f"[A] VERB-DRIFT `{name}`: db/queries/{sbase} sagt :{sv}, "
                f"internal/db/{gbase} sagt :{gv}."
            )
    print(f"  [A] nur in .sql: {len(only_sql)} · nur in Go: {len(only_go)} · beidseitig: {len(both)}")

    # ── Schicht B: SQL-Vergleich via sqlc-Orakel ────────────────────────────
    b_checked = 0
    b_skipped = 0
    data, oracle_err = run_oracle(paths)
    if oracle_err:
        b_skipped = len(both)
        line = f"  [B] SQL-Vergleich NICHT durchgefuehrt — {oracle_err}"
        print(line)
        if args.allow_missing_oracle:
            skips.append(
                f"[B] {b_skipped} Queries ungeprueft (Orakel nicht verfuegbar: {oracle_err})"
            )
        else:
            print(f"\nNICHT VERGLEICHBAR: {oracle_err}", file=sys.stderr)
            print("  Schicht B braucht das gepinnte sqlc-Image "
                  f"({SQLC_IMAGE}) via docker. Exit 2 = nicht gemessen, "
                  "ausdruecklich kein OK. Fuer lokale Laeufe ohne Docker: "
                  "--allow-missing-oracle.", file=sys.stderr)
            if errors:
                print(f"\nZUSAETZLICH aus Schicht A ({len(errors)}):", file=sys.stderr)
                for e in errors:
                    print(f"  - {e}", file=sys.stderr)
            return 2
    else:
        oracle_q, s3 = parse_oracle(data)
        skips += s3
        print(f"  [B] Queries im Orakel: {len(oracle_q)}")
        for name in both:
            if name not in oracle_q:
                b_skipped += 1
                skips.append(
                    f"[B] `{name}` nicht im Orakel-Output — sqlc hat die Query "
                    f"nicht emittiert; nicht vergleichbar."
                )
                continue
            b_checked += 1
            _, want, _ = oracle_q[name]
            _, have, gbase = go_q[name]
            if want != have:
                errors.append(
                    f"[B] SQL-DRIFT `{name}` (internal/db/{gbase}):\n"
                    f"        Const (hand-gepflegt): {have}\n"
                    f"        aus db/queries/*.sql:  {want}"
                )

    print(f"  [B] verglichen: {b_checked} · skipped: {b_skipped}")
    print(f"  Skips gesamt: {len(skips)}")

    # Ein Skip ist ein blinder Fleck. In Schicht A immer ein Fehler; in Schicht
    # B nur dann keiner, wenn --allow-missing-oracle bewusst gesetzt wurde.
    hard_skips = [
        s for s in skips
        if not (args.allow_missing_oracle and s.startswith("[B] ") and "Orakel nicht verfuegbar" in s)
    ]
    for s in hard_skips:
        errors.append(f"SKIP (blinder Fleck): {s}")

    if skips:
        print("\n  Skip-Liste:")
        for s in skips:
            print(f"    - {s}")

    if errors:
        print(f"\nFEHLER ({len(errors)}):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print(f"\nOK — Naht dicht: {len(both)} Queries beidseitig, "
          f"{b_checked} SQL-identisch, 0 Skips.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
