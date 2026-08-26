#!/usr/bin/env python3
"""
Selbsttest für lint-orgid-queries.py — die Mandanten-Isolationsprüfung.

Warum diese Datei existiert: Nach ADR-0042 gibt es keine Row-Level-Security
mehr, das org_id-Scoping in der Anwendung ist die einzige Trennung zwischen
Mandanten — und dieses Gate ist die einzige Prüfung darauf. Ein Gate ohne
gelaufenen Rot-Pfad beweist durch sein Grün nichts (CLAUDE.md: drei Abnahmen,
nicht eine).

Der Anlass (R1-SA03-05): Die Unterdrückung `// orgid-lint: global` hing an
einer ZEILENDISTANZ — `preceding = lines[start_line-4:start_line]`. Ein
Kommentar exemptierte damit jedes SQL-Literal, das in den folgenden drei
Zeilen begann, also auch das org-pflichtige Statement neben dem berechtigt
globalen. Abnahme A2 unten baut genau diese Lage nach UND weist mit einer
Nachbildung der alten Regel nach, dass die Lage unter der alten Regel still
durchgelaufen wäre — sonst wäre der Test vakuär.

Das Gate wird als PROZESS aufgerufen, nicht als Funktion: sein Vertrag ist der
Exit-Code plus die gedruckte Meldung, und genau das prüft die CI-Zeile auch.

Läuft ohne Docker, ohne Netz, ohne DB: nur stdlib, nur temporäre Verzeichnisse.
Exit non-zero, wenn eine Abnahme scheitert.
"""

import os
import pathlib
import re
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
# ORGID_GATE erlaubt, denselben Test gegen eine ANDERE Fassung des Gates zu
# fahren — so wird der Nachweis „ohne den Fix ist der Test rot" reproduzierbar:
#   git show HEAD~1:scripts/lint-orgid-queries.py > /tmp/alt.py
#   ORGID_GATE=/tmp/alt.py python3 scripts/lint_orgid_queries_test.py
GATE = pathlib.Path(os.environ.get("ORGID_GATE", HERE / "lint-orgid-queries.py"))
REPO = HERE.parent

FAILURES = []

SCHEMA = """
CREATE TABLE ck_things (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    secret text
);
CREATE TABLE ck_other (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    note text
);
"""

# Eine saubere Query, damit Pass 1 nicht am Nicht-Vakuitäts-Guard (G-07)
# scheitert, bevor Pass 2 überhaupt läuft.
QUERIES = """-- name: GetThing :one
SELECT * FROM ck_things WHERE org_id = $1 AND id = $2;
"""

# Ein geprüftes, sauberes Statement in jeder Go-Fixture — ohne das meldet der
# Nicht-Vakuitäts-Guard von Pass 2 "ZERO tenant-table SQL literals" (Exit 2)
# und wir würden Rot für die falsche Ursache lesen.
CLEAN_GO = """package p

func clean(db any) {
\t_, _ = db.Exec(`SELECT note FROM ck_other WHERE org_id = $1`)
}
"""


def fixture(td, go_files, queries=QUERIES, schema=SCHEMA):
    """Legt ein Wegwerf-Repo an: migrations/, queries/, backend/."""
    root = pathlib.Path(td)
    (root / "migrations").mkdir(exist_ok=True)
    (root / "migrations" / "001_init.up.sql").write_text(schema, encoding="utf-8")
    (root / "queries").mkdir(exist_ok=True)
    if queries is not None:
        (root / "queries" / "test.sql").write_text(queries, encoding="utf-8")
    # Der Ordner MUSS "backend" heißen: rel() in scan_go_raw_sql leitet den
    # Pfad in der Meldung daraus ab.
    (root / "backend").mkdir(exist_ok=True)
    for name, body in go_files.items():
        (root / "backend" / name).write_text(body, encoding="utf-8")
    return root


def run_gate(root=None, query_dir=None, go_dir=None, migrations_dir=None):
    if root is not None:
        query_dir = query_dir or root / "queries"
        go_dir = go_dir or root / "backend"
        migrations_dir = migrations_dir or root / "migrations"
    p = subprocess.run(
        [sys.executable, str(GATE), "--raw-sql",
         "--query-dir", str(query_dir),
         "--go-dir", str(go_dir),
         "--migrations-dir", str(migrations_dir)],
        capture_output=True, text=True,
    )
    return p.returncode, p.stdout + p.stderr


def expect(name, got, out, code, must_contain=(), must_not_contain=()):
    short = out if len(out) < 2500 else out[:2500] + "\n… [gekuerzt]"
    if got != code:
        FAILURES.append(f"{name}: Exit {got}, erwartet {code}\n{short}")
        return
    missing = [s for s in must_contain if s not in out]
    if missing:
        FAILURES.append(f"{name}: Meldung nennt {missing} nicht\n{short}")
        return
    present = [s for s in must_not_contain if s in out]
    if present:
        FAILURES.append(f"{name}: Meldung nennt {present}, darf sie aber nicht\n{short}")
        return
    print(f"PASS: {name}")


# ── Nachbildung der ALTEN Regel, nur für den Nicht-Vakuitäts-Nachweis ────────
# Wortgleich zur entfernten Stelle: Kommentar auf der Zeile des öffnenden
# Backticks ODER in den drei Zeilen davor.
_OLD_SKIP_RE = re.compile(r'orgid-lint:\s*global', re.IGNORECASE)


def old_rule_would_suppress(go_text, needle):
    """True, wenn die alte Zeilennähe-Regel das Literal mit `needle` schluckte."""
    lines = go_text.splitlines()
    for idx, line in enumerate(lines):
        if needle not in line:
            continue
        window = "\n".join(lines[max(0, idx - 3):idx + 1])
        if _OLD_SKIP_RE.search(window):
            return True
    return False


# ── Abnahme 1: GRÜN auf der Baseline — dem echten Repo ───────────────────────
# Die 90 heutigen Unterdrückungen müssen Unterdrückungen bleiben. Wäre der Fix
# zu streng, stünden hier auf einen Schlag Dutzende FAIL-Zeilen.
code, out = run_gate(
    query_dir=REPO / "backend" / "db" / "queries",
    go_dir=REPO / "backend",
    migrations_dir=REPO / "backend" / "db" / "migrations",
)
expect("A1 Baseline (echtes Repo) ist grün", code, out, 0,
       ["org_id query lint: OK", "stale=0", "allowlist_stale=0", "skipped=", "unparsed=0"])

# Der Nenner ist Teil des Vertrags, nicht Kosmetik: ein Gate, das nicht sagt,
# wie viele Statements es gesehen und wie viele es übersprungen hat, meldet
# Erfolg für Arbeit, die es vielleicht nicht getan hat.
for token in ("NENNER (sqlc):", "raw_sql_checked=", "skipped=", "suppressions=",
              "multi_stmt_suppressions=", "redundant=", "stale=", "unparsed=",
              "unreadable="):
    if token not in out:
        FAILURES.append(f"A1 Nenner: '{token}' fehlt in der Ausgabe\n{out}")
if all(t in out for t in ("raw_sql_checked=", "skipped=", "suppressions=")):
    print("PASS: A1 Nenner nennt Geprüfte, Übersprungene, Unterdrückungen, Stale")

# Mehrfachdeckung ist erlaubt (ein Go-Statement), darf aber nie still sein —
# genau ihre Unsichtbarkeit war der Defekt.
if "MULTI  " not in out:
    FAILURES.append("A1: keine MULTI-Zeile — Mehrfachdeckung wird nicht ausgewiesen\n" + out)
else:
    print("PASS: A1 Mehrfachdeckung wird namentlich ausgewiesen")


# ── Abnahme 2: ROT bei der Zeilennähe-Unterdrückung (der gemeldete Defekt) ───
# Ein Kommentar, zwei Statements: das erste absichtlich global, das zweite
# org-pflichtig. Unter der alten Regel wurden BEIDE geschluckt.
PROXIMITY = """package p

func leak(db any) {
\t// orgid-lint: global — Wartungsjob, laeuft absichtlich ueber alle Orgs
\t_, _ = db.Exec(`SELECT id FROM ck_things ORDER BY id`)
\t_, _ = db.Exec(`SELECT secret FROM ck_things WHERE id = $1`)
}
"""
if not old_rule_would_suppress(PROXIMITY, "SELECT secret FROM ck_things"):
    FAILURES.append("A2 ist vakuär: die alte Zeilennähe-Regel haette das zweite "
                    "Statement gar nicht geschluckt — Fixture trifft den Defekt nicht.")
else:
    print("PASS: A2 Nicht-Vakuität — die alte Regel haette das zweite Statement geschluckt")

with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"leak.go": PROXIMITY, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("A2 zweites Statement neben einem Opt-out ist ROT und wird benannt",
           code, out, 1,
           ["backend/leak.go:6", "missing org_id in query body",
            "SELECT secret FROM ck_things"],
           # Das erste, berechtigt globale Statement bleibt unterdrückt.
           must_not_contain=["SELECT id FROM ck_things ORDER BY id"])

# Gegenprobe: dasselbe Fixture ohne das zweite Statement ist grün — sonst
# waere das Rot oben von einem kaputten Fixture nicht zu unterscheiden.
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {
        "leak.go": PROXIMITY.replace(
            "\t_, _ = db.Exec(`SELECT secret FROM ck_things WHERE id = $1`)\n", ""),
        "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("A2 Gegenprobe: nur das globale Statement ist grün", code, out, 0,
           ["org_id query lint: OK", "skipped=1"])

# Dieselbe Klasse in Trailing-Form: role_update_boundary_test.go:157 → :158.
# Ein Opt-out am Zeilenende gilt fuer SEINE Zeile, die darunter erbt nichts.
TRAILING = """package p

var cases = []struct{ name, sql string }{
\t{"a", `SELECT id FROM ck_things`}, // orgid-lint: global — Sondentext, wird nie ausgefuehrt
\t{"b", `SELECT secret FROM ck_things WHERE id = $1`},
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"trailing.go": TRAILING, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("A2b Trailing-Opt-out erbt nicht auf die naechste Zeile", code, out, 1,
           ["backend/trailing.go:5", "SELECT secret FROM ck_things"],
           must_not_contain=["backend/trailing.go:4"])


# ── Abnahme 3: ROT bei einer stale gewordenen Unterdrückung ──────────────────
# Ein Opt-out, dessen Statement geloescht wurde. Es prueft nichts mehr, steht
# aber als offene Tuer bereit: wer dort die naechste Query einfuegt, ist
# ungeprueft. (K2-08: eine Ausnahme braucht ihren Gegenstand.)
STALE = """package p

func gone(db any) {
\t// orgid-lint: global — die Query hierunter wurde entfernt
\treturn
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"stale.go": STALE, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("A3 verwaistes Opt-out ist ROT und wird benannt", code, out, 1,
           ["STALE", "backend/stale.go:4", "stale=1"])

# Zweite Haelfte derselben Klasse: ein ALLOWLIST-Eintrag ohne Query in
# --query-dir. Genau 20 solcher Eintraege standen bis zu diesem Commit im Repo
# (alle user_permissions.sql:*) — sie wurden hier entfernt.
# Die Datei muss existieren, sonst ist der Eintrag nicht stale, sondern
# unpruefbar (fremder Baum) — siehe naechste Abnahme.
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"clean.go": CLEAN_GO}, queries=None)
    (root / "queries" / "vaktaware.sql").write_text(
        "-- name: CountSREventsByType :one\n"
        "SELECT count(*) FROM ck_things WHERE org_id = $1;\n", encoding="utf-8")
    code, out = run_gate(root)
    expect("A3b ALLOWLIST-Eintrag ohne Query in vorhandener Datei ist ROT",
           code, out, 1,
           ["STALE ALLOWLIST", "vaktaware.sql:CountSRTargetsInGroup", "allowlist_stale=6"],
           # Eintraege aus Dateien, die es hier gar nicht gibt, sind unpruefbar,
           # nicht stale — sonst faerbte jeder Teil-Checkout das Gate rot.
           must_not_contain=["STALE  vaktcomply.sql:"])

with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"clean.go": CLEAN_GO},
                   queries="-- name: Unrelated :one\n"
                           "SELECT 1 FROM ck_things WHERE org_id = $1;\n")
    code, out = run_gate(root)
    expect("A3c fremder Query-Baum: ALLOWLIST unpruefbar statt stale, gezaehlt",
           code, out, 0, ["unpruefbar=48", "allowlist_stale=0"])


# ── Gegenproben: der Fix darf nicht zu streng sein ───────────────────────────
# (a) Ein Kommentar innerhalb eines Composite-Literals deckt dessen Felder mit,
#     weil sie EIN Go-Statement sind (cmd/rotate-key/rotate.go:141).
COMPOSITE = """package p

func rotate() rotation {
\treturn run(rotation{
\t\tTable: "ck_things",
\t\t// orgid-lint: global — Schluesselrotation, laeuft ueber alle Orgs
\t\tSelectSQL: `SELECT id FROM ck_things ORDER BY id`,
\t\tUpdateSQL: `UPDATE ck_things SET secret = $1 WHERE id = $2`,
\t})
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"composite.go": COMPOSITE, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("B1 Composite-Literal: ein Kommentar deckt beide Felder", code, out, 0,
           ["org_id query lint: OK", "skipped=2", "multi_stmt_suppressions=1", "MULTI"])

# (b) Zwischen Kommentar und Statement darf eine Deklaration stehen — im Repo
#     mehrfach so geschrieben (auth/session_handler.go:90, alerts.go:253).
DECL_BETWEEN = """package p

func f(db any) {
\t// orgid-lint: global — eigene Session, ueber user_id aus dem Token gescoped
\tvar hash string
\t_ = db.QueryRow(`DELETE FROM ck_things WHERE id = $1 RETURNING secret`).Scan(&hash)
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"decl.go": DECL_BETWEEN, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("B2 Deklaration zwischen Kommentar und Statement bleibt unterdrueckt",
           code, out, 0, ["org_id query lint: OK", "skipped=1", "stale=0"])

# (c) Ein Kommentar ueber einer Funktion darf NICHT deren ganzen Rumpf decken.
#     (vaktaware/repository.go:1105 tat das in der ersten Fassung des Fixes.)
ABOVE_FUNC = """package p

// orgid-lint: global — der erste Zugriff ist absichtlich global
func f(db any) {
\t_, _ = db.Exec(`SELECT id FROM ck_things ORDER BY id`)
\tfor i := 0; i < 3; i++ {
\t\t_, _ = db.Exec(`SELECT secret FROM ck_things WHERE id = $1`)
\t}
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"above.go": ABOVE_FUNC, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("B3 Kommentar ueber einer Funktion deckt nicht den ganzen Rumpf",
           code, out, 1, ["backend/above.go:7", "SELECT secret FROM ck_things"])

# (d) Ein Opt-out auf einer Katalogtabelle ohne org_id ist ueberfluessig, aber
#     nicht verwaist — es zeigt auf ein existierendes Statement. Redundant
#     wird gezaehlt, nicht rot gefaerbt (sonst faerbte der Stale-Check ~10
#     korrekt geschriebene Kommentare im Repo rot).
REDUNDANT = """package p

func f(db any) {
\t// orgid-lint: global — globaler Katalog, nicht pro Org
\t_, _ = db.Exec(`SELECT code FROM po_adequacy_decisions ORDER BY code`)
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"redundant.go": REDUNDANT, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("B4 Opt-out auf nicht pruefpflichtigem SQL: redundant, nicht stale",
           code, out, 0, ["redundant=1", "stale=0"])


# ── Nenner-Ehrlichkeit: was nicht gelesen werden konnte, wird gezaehlt ───────
BLOCK_COMMENT = """package p

/* orgid-lint: global — steht in einem Blockkommentar, nicht in // */
func f(db any) {
\t_, _ = db.Exec(`SELECT secret FROM ck_things WHERE id = $1`)
}
"""
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"block.go": BLOCK_COMMENT, "clean.go": CLEAN_GO})
    code, out = run_gate(root)
    expect("C1 orgid-lint ausserhalb eines //-Kommentars wird gezaehlt, nicht "
           "still als Unterdrueckung gewertet", code, out, 1,
           ["unparsed=1", "backend/block.go:5"])


# ── Nicht-Vakuitaet: leere Eingaben sind kein bestandenes Ergebnis ──────────
with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {})
    code, out = run_gate(root)
    expect("C2 leeres Go-Verzeichnis ist Exit 2, nicht gruen", code, out, 2,
           ["ZERO tenant-table SQL", "G-07"])

with tempfile.TemporaryDirectory() as td:
    root = fixture(td, {"clean.go": CLEAN_GO},
                   schema="CREATE TABLE plain (id uuid PRIMARY KEY);\n")
    code, out = run_gate(root)
    expect("C3 Schema ohne org_id-Tabelle ist Exit 2 (VAKUAER), nicht gruen",
           code, out, 2, ["VAKUAER"])


if FAILURES:
    print("\n".join(["", "FEHLGESCHLAGENE ABNAHMEN:"] + FAILURES))
    sys.exit(1)
print("\nalle Abnahmen bestanden")
