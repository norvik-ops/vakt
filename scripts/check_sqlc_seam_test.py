#!/usr/bin/env python3
"""Selbsttest fuer `check_sqlc_seam.py` — beweist die roten Pfade.

Ein Gate, dessen roter Pfad nie ausgefuehrt wurde, beweist durch Gruen nichts
(CLAUDE.md, Drei-Abnahmen-Regel: gruen auf der Baseline · ROT bei einer echten
Regression MIT Namensnennung · ROT bei einer neuen/einseitigen Fundstelle).
Dieses Skript baut fuer jeden Fall ein Wegwerf-Repo, praepariert genau EINE
Abweichung und fordert Exit-Code plus Namensnennung ein.

Die Faelle sind bewusst so gewaehlt, dass die teuren (sqlc-Orakel) nur dort
laufen, wo sie etwas beweisen: Schicht A braucht kein Orakel und wird mit
--allow-missing-oracle gefahren, Schicht B genau einmal.

Exit 0 = alle Faelle wie erwartet, 1 = ein Fall nicht wie erwartet.
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GATE = os.path.join(REPO_ROOT, "scripts", "check_sqlc_seam.py")


def make_repo():
    """Wegwerf-Repo mit backend/{sqlc.yaml,db/{queries,migrations},internal/db}."""
    tmp = tempfile.mkdtemp(prefix="seam-selftest-")
    backend = os.path.join(tmp, "backend")
    os.makedirs(os.path.join(backend, "db"), exist_ok=True)
    os.makedirs(os.path.join(backend, "internal"), exist_ok=True)
    shutil.copy(os.path.join(REPO_ROOT, "backend", "sqlc.yaml"),
                os.path.join(backend, "sqlc.yaml"))
    for sub in ("queries", "migrations"):
        shutil.copytree(os.path.join(REPO_ROOT, "backend", "db", sub),
                        os.path.join(backend, "db", sub))
    src_db = os.path.join(REPO_ROOT, "backend", "internal", "db")
    dst_db = os.path.join(backend, "internal", "db")
    os.makedirs(dst_db)
    for f in os.listdir(src_db):
        if f.endswith(".sql.go"):
            shutil.copy(os.path.join(src_db, f), os.path.join(dst_db, f))
    # git init, damit `git ls-files --cached --others` etwas liefert. Die
    # Dateien bleiben UNTRACKED — genau der Fall, den --others abdecken muss.
    subprocess.run(["git", "init", "-q", tmp], check=True,
                   capture_output=True)
    return tmp


def run(root, *extra):
    proc = subprocess.run(
        [sys.executable, GATE, "--repo-root", root, *extra],
        capture_output=True, text=True, timeout=1800,
    )
    return proc.returncode, proc.stdout + proc.stderr


FAILURES = []


def check(label, ok, detail=""):
    print(("  PASS  " if ok else "  FAIL  ") + label)
    if isinstance(detail, (list, tuple)):
        detail = "\n".join(str(d) for d in detail)
    if detail:
        for line in str(detail).strip().split("\n"):
            print(f"          {line}")
    if not ok:
        FAILURES.append(label)



def _count_query_consts(root):
    """Zaehlt die Query-Consts unabhaengig vom Gate — sonst prueft der Selbsttest
    die Zahl des Gates gegen die Zahl des Gates."""
    total = 0
    gendir = os.path.join(root, "backend", "internal", "db")
    for fn in os.listdir(gendir):
        if not fn.endswith(".sql.go"):
            continue
        with open(os.path.join(gendir, fn), encoding="utf-8") as fh:
            total += len(re.findall(r"^const \w+ = `-- name: ", fh.read(), re.MULTILINE))
    return total


def case_baseline_green():
    print("\n[1] Baseline unveraendert -> Exit 0 (Schicht A + B, echtes Orakel)")
    code, out = run(REPO_ROOT)
    check("Exit 0", code == 0, f"exit={code}")
    # KEINE feste Zahl. Eine fest eingetragene Erwartung testet die Groesse des
    # Repos, nicht das Gate — jede legitim hinzugefuegte Query bricht sie
    # (passiert mit 491 -> 493). Stattdessen unabhaengig nachzaehlen: die Zahl
    # muss der tatsaechlichen Menge der Query-Consts entsprechen, und es darf
    # nichts uebersprungen worden sein.
    expected = _count_query_consts(REPO_ROOT)
    m = re.search(r"verglichen:\s*(\d+)", out)
    reported = int(m.group(1)) if m else -1
    check(f"vergleicht ALLE {expected} Query-Consts, 0 skipped",
          reported == expected and "skipped: 0" in out,
          f"gemeldet: {reported} · unabhaengig gezaehlt: {expected} · "
          + ([l for l in out.split("\n") if "verglichen" in l][0] if "verglichen" in out else out[-200:]))
    check("keine SQL-DRIFT-Zeile", "SQL-DRIFT" not in out)


def case_sql_drift_named():
    print("\n[2] Echte Spalten-Drift in EINER Query -> Exit 1 MIT Namensnennung")
    root = make_repo()
    try:
        # Const im Go-Code um eine Spalte kuerzen == genau die Klasse der 12.
        # Anker MUSS eindeutig sein: `SELECT … FROM ck_bcp_plans` steht auch in
        # listCKBCPPlans, und das steht in der Datei VOR getCKBCPPlan — ein
        # kuerzerer Anker praepariert die falsche Query (im ersten Lauf
        # passiert). Also die WHERE-Zeile mitnehmen.
        path = os.path.join(root, "backend", "internal", "db", "vaktcomply.sql.go")
        src = open(path, encoding="utf-8").read()
        # Anker ist die CONST-DEKLARATION, nicht der SQL-Text. Ein Textanker ist
        # nicht stabil: dieselbe Spaltenliste taucht auf, sobald jemand eine
        # zweite Query auf dieselbe Tabelle schreibt (genau das ist zweimal
        # passiert — erst mit einem kurzen Anker, dann mit dem verlaengerten).
        # Eine Go-Const-Deklaration `const getCKBCPPlan = ` ist im Paket eindeutig.
        decl = "const getCKBCPPlan = `"
        start = src.find(decl)
        if start < 0:
            check("Praeparation moeglich", False,
                  "const getCKBCPPlan nicht gefunden — Test veraltet.")
            return
        end = src.find("`", start + len(decl))
        body = src[start:end]
        if ", last_tested_at" not in body:
            check("Praeparation moeglich", False,
                  "Spalte last_tested_at nicht im Const-Rumpf — Test veraltet.")
            return
        mutated = body.replace(", last_tested_at", "", 1)
        open(path, "w", encoding="utf-8").write(src[:start] + mutated + src[end:])
        code, out = run(root)
        check("Exit 1", code == 1, f"exit={code}")
        check("nennt `GetCKBCPPlan` beim Namen",
              "SQL-DRIFT `GetCKBCPPlan`" in out,
              [l for l in out.split("\n") if "SQL-DRIFT" in l][:3])
        # Nicht-Vakuitaet der Gegenrichtung: es darf NICHT alles rot sein.
        n = len(re.findall(r"SQL-DRIFT `", out))
        check("nur die praeparierte Query rot (nicht pauschal alle)", n == 1,
              f"{n} DRIFT-Meldungen")
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_go_missing():
    print("\n[3] Query nur in .sql, kein Go-Code -> Exit 1 (Schicht A)")
    root = make_repo()
    try:
        path = os.path.join(root, "backend", "db", "queries", "vaktcomply.sql")
        with open(path, "a", encoding="utf-8") as fh:
            fh.write("\n-- name: SelftestOrphanQuery :one\n"
                     "SELECT id FROM ck_bcp_plans WHERE org_id = $1;\n")
        code, out = run(root, "--allow-missing-oracle")
        check("Exit 1", code == 1, f"exit={code}")
        check("nennt `SelftestOrphanQuery` beim Namen",
              "GO-CODE FEHLT: `SelftestOrphanQuery`" in out,
              [l for l in out.split("\n") if "SelftestOrphanQuery" in l][:2])
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_sql_missing():
    print("\n[4] Const nur im Go-Code, keine .sql-Quelle -> Exit 1 (Schicht A)")
    root = make_repo()
    try:
        path = os.path.join(root, "backend", "internal", "db", "vaktcomply.sql.go")
        with open(path, "a", encoding="utf-8") as fh:
            fh.write("\nconst selftestGhostQuery = `-- name: SelftestGhostQuery :one\n"
                     "SELECT id FROM ck_bcp_plans WHERE org_id = $1\n`\n")
        code, out = run(root, "--allow-missing-oracle")
        check("Exit 1", code == 1, f"exit={code}")
        check("nennt `SelftestGhostQuery` beim Namen",
              "SQL-QUELLE FEHLT: Const `SelftestGhostQuery`" in out,
              [l for l in out.split("\n") if "SelftestGhostQuery" in l][:2])
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_verb_drift():
    print("\n[5] Verb-Drift (:one vs :many) -> Exit 1 (Schicht A)")
    root = make_repo()
    try:
        path = os.path.join(root, "backend", "db", "queries", "vaktcomply.sql")
        src = open(path, encoding="utf-8").read()
        assert "-- name: GetCKBCPPlan :one" in src
        open(path, "w", encoding="utf-8").write(
            src.replace("-- name: GetCKBCPPlan :one", "-- name: GetCKBCPPlan :many", 1))
        code, out = run(root, "--allow-missing-oracle")
        check("Exit 1", code == 1, f"exit={code}")
        check("nennt `GetCKBCPPlan` als VERB-DRIFT",
              "VERB-DRIFT `GetCKBCPPlan`" in out,
              [l for l in out.split("\n") if "VERB-DRIFT" in l][:2])
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_untracked_counted():
    print("\n[6] Untracked .sql-Datei wird MITGEZAEHLT (nicht lokal-gruen/CI-rot)")
    root = make_repo()
    try:
        # Neue, nicht getrackte Query-Datei mit einer Query ohne Go-Code.
        path = os.path.join(root, "backend", "db", "queries", "selftest_new.sql")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write("-- name: SelftestUntrackedQuery :one\n"
                     "SELECT id FROM ck_bcp_plans WHERE org_id = $1;\n")
        code, out = run(root, "--allow-missing-oracle")
        check("Exit 1 (Datei wurde gesehen)", code == 1, f"exit={code}")
        check("zaehlt 8 .sql-Dateien statt 7",
              ".sql-Dateien (git-gelistet):    8" in out,
              [l for l in out.split("\n") if ".sql-Dateien" in l][:1])
        check("nennt `SelftestUntrackedQuery`",
              "SelftestUntrackedQuery" in out)
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_missing_oracle_not_ok():
    print("\n[7] Orakel nicht verfuegbar ohne Flag -> Exit 2, NICHT 0")
    root = make_repo()
    try:
        # PATH so beschneiden, dass GENAU docker fehlt — `git` muss bleiben,
        # sonst scheitert das Gate schon am Datei-Einsammeln und man testet
        # etwas anderes als beabsichtigt (im ersten Lauf passiert: PATH=
        # /nonexistent nahm auch git weg -> Exit 1 statt 2).
        binstub = tempfile.mkdtemp(prefix="seam-nodocker-")
        git = shutil.which("git")
        if not git:
            check("git im PATH auffindbar", False, "kein git — Fall nicht testbar")
            return
        os.symlink(git, os.path.join(binstub, "git"))
        env = dict(os.environ, PATH=binstub)
        proc = subprocess.run(
            [sys.executable, GATE, "--repo-root", root],
            capture_output=True, text=True, env=env, timeout=600,
        )
        shutil.rmtree(binstub, ignore_errors=True)
        out = proc.stdout + proc.stderr
        check("Exit 2 (nicht gemessen != OK)", proc.returncode == 2,
              f"exit={proc.returncode}")
        check("meldet kein OK", "OK — Naht dicht" not in out)
        check("benennt den Grund", "NICHT VERGLEICHBAR" in out,
              [l for l in out.split("\n") if "NICHT VERGLEICHBAR" in l][:1])
    finally:
        shutil.rmtree(root, ignore_errors=True)


def case_skip_is_error():
    print("\n[8] Nicht parsbarer const-Block -> gezaehlter Skip UND Fehler")
    root = make_repo()
    try:
        path = os.path.join(root, "backend", "internal", "db", "vaktcomply.sql.go")
        with open(path, "a", encoding="utf-8") as fh:
            fh.write("\nconst selftestUnparsable = `SELECT 1`\n")
        code, out = run(root, "--allow-missing-oracle")
        check("Exit 1", code == 1, f"exit={code}")
        check("Skip-Zaehler > 0", "Skips gesamt: 0" not in out,
              [l for l in out.split("\n") if "Skips gesamt" in l][:1])
        check("Skip als blinder Fleck gemeldet",
              "SKIP (blinder Fleck)" in out)
    finally:
        shutil.rmtree(root, ignore_errors=True)


def main():
    print("Selbsttest check_sqlc_seam.py — Drei-Abnahmen-Regel")
    case_baseline_green()
    case_sql_drift_named()
    case_go_missing()
    case_sql_missing()
    case_verb_drift()
    case_untracked_counted()
    case_missing_oracle_not_ok()
    case_skip_is_error()

    print()
    if FAILURES:
        print(f"FEHLGESCHLAGEN ({len(FAILURES)}):", file=sys.stderr)
        for f in FAILURES:
            print(f"  - {f}", file=sys.stderr)
        return 1
    print("OK — alle 8 Faelle wie erwartet (1 gruen, 7 rot mit Namensnennung).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
