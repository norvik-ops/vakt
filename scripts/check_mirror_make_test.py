#!/usr/bin/env python3
"""G-MIRROR-TEST — `make test` muss im GEBAUTEN Public Mirror durchlaufen.

Warum es dieses Gate gibt (2026-07-30)
--------------------------------------
Dieselbe Zeile ist ZWEIMAL gedriftet. PR #67 stellte
`bash infra/server/scripts/vakt-deploy_test.sh` in das `test:`-Ziel des
Makefile; PR #76 stellte `bash infra/server/scripts/backup-offsite_test.sh`
daneben. `scripts/build-public-mirror.sh` spiegelt `infra/` nicht — und kann es
auch nicht: `vakt-deploy.sh` traegt den internen Deploy-Arbeitspfad,
`backup-offsite.sh` den internen Monitoring-Hostnamen, beides Muster des
LEAK_PATTERN in genau diesem Build-Skript. Gemessen: ein `rsync infra/` bricht
den Mirror-Build ab, statt ihn zu reparieren.

Im Kundenrepo lief `make test` damit in ein
`bash: infra/server/scripts/vakt-deploy_test.sh: No such file or directory`
an der dritten Zeile — und die vierte Suite, die im Mirror vorhanden UND
lauffaehig ist (`scripts/backup_restore_wiring_test.sh`), wurde nie erreicht.
Gemessen im echten Mirror-Build aus 42763c0.

Beide Male war der Diff im privaten Repo gruen: dort existiert `infra/`. Es gab
NICHTS, das `make test` im gebauten Mirror geprueft haette. Genau diese Luecke
schliesst dieses Gate — es misst am Build-Ergebnis, nicht an der Quelle.

Was es prueft
-------------
A. Es BAUT den Mirror (`scripts/build-public-mirror.sh` nach einem Tempdir) und
   arbeitet danach ausschliesslich in diesem Baum — nie an der Quelle.
B. Es laesst `make -n <ziel>` IM Mirror expandieren (also mit der dortigen
   Dateimenge, `$(wildcard …)` inklusive) und loest jeden Pfad in
   Befehlsposition gegen den Mirror auf. Ein Pfad, den der Mirror nicht hat,
   ist ein Fehler MIT Namensnennung.
C. Ratchet: die Suiten aus RATCHET muessen von `make test` im Mirror weiterhin
   aufgerufen werden. Faellt eine heraus, ist die Verbesserung nicht
   festgehalten — Fehler, ebenfalls mit Namensnennung. (Ohne diese Haelfte
   waere das Gate durch Loeschen der Zeile gruen zu bekommen.)

Sein Nenner — was dieses Gate NICHT ansieht
-------------------------------------------
Hausregel dieses Repos: ein Gate nennt, was es uebersprungen hat, sonst ist
gruen ueber einer unbenannten Teilmenge ein falsches Signal. Also ausdruecklich:

* ES FUEHRT DIE SUITEN NICHT AUS. Geprueft wird, dass `make test` im Mirror
  nicht an einer fehlenden Datei stirbt — nicht, dass `restore_test.sh` dort
  auch besteht. Ein `go test`/`npm test`/Docker-Lauf im Mirror waere Minuten
  und gehoert nicht in ein pre-push-Gate.
* ES BAUT DEN MIRROR OHNE `go build` (VAKT_MIRROR_SKIP_GO_BUILD=1, ~30 s zu
  teuer fuer jeden Push). Dass der Mirror kompiliert, prueft
  sync-public-repo.yml mit dem ungekuerzten Build.
* ZIELE: es liest die Test-/DoD-Kette (TARGETS), nicht jedes Makefile-Ziel.
  Bewusst draussen und hier benannt statt still uebersprungen:
    · `public-mirror` — ruft `./scripts/build-public-mirror.sh`, das per
      rsync --exclude bewusst NICHT gespiegelt wird. Im Kundenrepo ist das
      Ziel damit tot. Eigener, offener Befund, kein von diesem Gate
      gutgeheissener Zustand.
    · `dev`/`stop`/`billing`/`api-local`/`*-local` — Ops-/Dev-Ziele, die auf
      Docker, ssh oder eine lokale Postgres zeigen; kein Dateipfad im Repo.
* `make gates` LAEUFT IM MIRROR TROTZDEM NICHT DURCH, und dieses Gate faerbt
  sich davon nicht rot: alle Pfade existieren dort, aber `check_ci_steps.py`
  bricht ab, weil `.github/workflows/ci.yml` und `docs.yml` bewusst nicht
  gespiegelt werden. Das ist eine andere Fehlerklasse (Gate ohne sein
  Pruefsubjekt, nicht Makefile ohne seine Datei) und ein eigener offener
  Befund. Hier gezaehlt und genannt, nicht gefixt.
* NUR DAS MAKEFILE. Ein Skript, das INTERN eine nicht gespiegelte Datei
  sourcet oder aufruft, faellt hier nicht auf — dafuer muesste es laufen.
* PFADE AUS SHELL-VARIABLEN (`bash "$t"` in einer for-Schleife) sind statisch
  nicht aufloesbar; sie werden als `skipped` GEZAEHLT und aufgelistet, nie
  stillschweigend als OK verbucht.
* KOMMANDOFORMEN, die dieses Gate nicht kennt, werden ebenfalls gezaehlt und
  benannt — aber sie faerben es NICHT rot. Das war eine Luecke, nicht nur ein
  Nenner: die nackte Pfadform (`infra/server/scripts/x_test.sh` in eigener
  Zeile, ohne `bash` davor, ohne `./`) landete dort und liess das Gate gruen,
  waehrend `make test` im Mirror an genau dieser Zeile mit exit 127 starb —
  gemessen (Review REV-R76 §4/F4). Seit 2026-07-30 gilt deshalb: ein
  unklassifiziertes Kommando, das wie ein Repo-Pfad aussieht (enthaelt `/`,
  nicht absolut), IST ein Pfad und wird aufgeloest. Ebenso `make -C <dir>`.
  In `unclassified` bleiben damit nur noch Formen ohne Pfadbezug und absolute
  Systempfade (`/usr/bin/rrsync`) — beides weiterhin gezaehlt und benannt.

Selbsttest: `python3 scripts/check_mirror_make_test.py --selftest`
Exit 0 = OK oder nicht pruefbar (Werkzeug fehlt) · Exit 1 = Fehler.
"""

import os
import pathlib
import re
import shlex
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent

# ── Ziele, deren Rezepte im Mirror aufloesbar sein muessen. Die Test-/DoD-Kette;
# was bewusst fehlt, steht oben im Nenner.
TARGETS = [
    "test",
    "gates",
    "check",
    "test-restore",
    "test-backup",
    "test-deploy-gate",
    "test-backup-wiring",
    "test-offsite-order",
    "build",
]

# ── RATCHET: das muss `make test` im Mirror weiterhin aufrufen, und es muss dort
# liegen. `backup_restore_wiring_test.sh` steht hier, weil genau sie es war, die
# der Abbruch aus #67/#76 unerreichbar machte — sie ist die Verbesserung, die
# festgehalten gehoert. `backend`/`frontend` stehen hier, damit eine Umstellung
# auf einen nicht gespiegelten Pfad (z.B. `cd backend/cmd`) auffaellt.
RATCHET = [
    "scripts/restore_test.sh",
    "scripts/backup_cron_test.sh",
    "scripts/backup_restore_wiring_test.sh",
    "backend",
    "frontend",
]

# Befehle, deren erstes Nicht-Flag-Argument ein Pfad im Repo ist.
RUNNERS = {"bash", "sh", "python3", "python", "node"}
# Shell-Schluesselwoerter, die einer einfachen Kommandozeile vorangehen koennen.
SHELL_LEAD = {"do", "then", "else", "{", "}", "(", ")", "!"}
# Eingebaute/Umgebungs-Kommandos ohne Repo-Pfad — als solche gezaehlt, nicht als
# "nicht klassifiziert" (sonst ertrinkt der Nenner in echo-Zeilen).
BENIGN = {
    "echo", "printf", "exit", "true", "false", "set", "test", "[",
    "for", "while", "done", "fi", "esac", "case", "if", "read", "shift",
    "export", "unset", "git", "chmod", "mkdir", "rm", "sed", "grep", "awk",
    "go", "npm", "make", "docker", "gofmt", "golangci-lint", "helm", "ssh",
    "sleep", "pkill", "xdg-open", "open", "cat", "tr", "ls", "find", "curl",
}
SHELL_SPLIT = re.compile(r"&&|\|\||;|\|")


def _simple_commands(line):
    """Eine Rezeptzeile in ihre einfachen Kommandos zerlegen."""
    return [p.strip() for p in SHELL_SPLIT.split(line) if p.strip()]


def _looks_like_repo_path(token):
    """Ist dieses Kommando-Token ein Pfad IN DIESES Repo?

    Ja bei `infra/server/scripts/x_test.sh` — die nackte Aufrufform, die dieses
    Gate frueher umging. Nein bei `/usr/bin/rrsync`: absolut heisst Werkzeug des
    Systems, nicht Datei des Repos; solche Zeilen bleiben `unclassified`, also
    gezaehlt und benannt. Traegt das Token eine Shell-Expansion, faengt es der
    `$`-Zweig weiter unten als `unresolved` ab — auch das gezaehlt, nie still.
    """
    return "/" in token and not token.startswith("/")


def classify(line):
    """-> (paths, unresolved, unclassified)

    paths        Pfade in Befehlsposition, statisch aufgeloest.
    unresolved   Pfade in Befehlsposition, die eine Shell-Variable/Glob tragen.
    unclassified Kommandos, deren Form dieses Gate nicht kennt.
    """
    paths, unresolved, unclassified = [], [], []
    for cmd in _simple_commands(line):
        try:
            argv = shlex.split(cmd)
        except ValueError:
            unclassified.append(cmd)
            continue
        while argv and (argv[0] in SHELL_LEAD or "=" in argv[0].split("/")[0][:64] and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*=.*", argv[0])):
            argv = argv[1:]
        if not argv:
            continue
        head = argv[0]
        cand = None
        if head == "cd" and len(argv) > 1:
            cand = argv[1]
        elif head in RUNNERS:
            rest = [a for a in argv[1:] if not a.startswith("-")]
            cand = rest[0] if rest else None
        elif head.startswith("./") or head.startswith("../"):
            cand = head
        elif head == "make" and "-C" in argv:
            # `make -C infra/server …` haengt genauso an einem Verzeichnis, das es
            # im Mirror geben muss. Ohne diesen Zweig faellt `make` unter BENIGN
            # und das Verzeichnis wird nicht einmal gezaehlt.
            i = argv.index("-C")
            cand = argv[i + 1] if i + 1 < len(argv) else None
        elif head in BENIGN:
            cand = None
        elif _looks_like_repo_path(head):
            # Die NACKTE Pfadform: `infra/server/scripts/backup-offsite_test.sh`
            # in eigener Zeile, ohne `bash` davor und ohne `./`. Genau die Form
            # umging dieses Gate (Review REV-R76 §4/F4, gemessen): sie landete in
            # `unclassified`, das Gate blieb gruen — waehrend `make test` im
            # Mirror an derselben Zeile mit exit 127 starb, also exakt der Defekt,
            # gegen den dieses Gate gebaut ist.
            cand = head
        else:
            unclassified.append(cmd)
            continue
        if cand is None:
            continue
        if "$" in cand or "*" in cand or "`" in cand:
            unresolved.append(cand)
        else:
            paths.append(cand.lstrip("./") if cand.startswith("./") else cand)
    return paths, unresolved, unclassified


def evaluate(recipes, exists, ratchet=RATCHET):
    """Reine Pruefung. `recipes` = {ziel: [zeile, …]}, `exists` = callable(pfad)->bool.

    -> (problems, stats). Wird vom Selbsttest mit synthetischen Eingaben gefahren.
    """
    problems = []
    n_lines = n_paths = 0
    unresolved, unclassified = [], []
    test_paths = set()

    for target, lines in recipes.items():
        for line in lines:
            n_lines += 1
            paths, unres, unclass = classify(line)
            unresolved += [(target, u) for u in unres]
            unclassified += [(target, u) for u in unclass]
            for p in paths:
                n_paths += 1
                if target == "test":
                    test_paths.add(p)
                if not exists(p):
                    problems.append(
                        f"make {target}: ruft `{p}` auf — im gebauten Mirror gibt es "
                        f"die Datei nicht. Zeile: {line.strip()}"
                    )

    for want in ratchet:
        if want not in test_paths:
            problems.append(
                f"Ratchet: `make test` ruft `{want}` im Mirror nicht mehr auf. Diese "
                f"Suite lief dort — eine Verbesserung, die nicht wieder verloren "
                f"gehen darf. Absicht? Dann RATCHET in scripts/check_mirror_make_test.py "
                f"im selben Commit anpassen."
            )
        elif not exists(want):
            problems.append(
                f"Ratchet: `make test` ruft `{want}` auf, aber der Mirror hat es nicht."
            )

    return problems, {
        "lines": n_lines,
        "paths": n_paths,
        "unresolved": unresolved,
        "unclassified": unclassified,
        "ratchet_ok": sum(1 for w in ratchet if w in test_paths and exists(w)),
        "ratchet_total": len(ratchet),
    }


# ─────────────────────────── Mirror bauen + auslesen ────────────────────────────

def build_mirror(out):
    script = ROOT / "scripts" / "build-public-mirror.sh"
    if not script.is_file():
        return None, (
            "scripts/build-public-mirror.sh fehlt — dieses Repo ist vermutlich "
            "selbst der Mirror. Nichts zu bauen."
        )
    for tool in ("bash", "rsync", "make"):
        if shutil.which(tool) is None:
            return None, f"{tool} fehlt im PATH — Mirror nicht baubar."
    env = dict(os.environ, VAKT_MIRROR_OUT=str(out), VAKT_MIRROR_SKIP_GO_BUILD="1")
    p = subprocess.run(
        ["bash", str(script)], cwd=str(ROOT), env=env,
        capture_output=True, text=True, timeout=900,
    )
    if p.returncode != 0:
        return p, None
    return p, None


def recipe_lines(mirror, target):
    """`make -n <target>` IM Mirror. -> (zeilen, fehler|None)"""
    env = dict(os.environ)
    env.pop("MAKEFLAGS", None)
    env.pop("MAKELEVEL", None)
    p = subprocess.run(
        ["make", "-n", "--no-print-directory", target],
        cwd=str(mirror), env=env, capture_output=True, text=True, timeout=300,
    )
    if p.returncode != 0:
        return [], (p.stderr.strip() or p.stdout.strip() or f"rc={p.returncode}")
    return [ln for ln in p.stdout.splitlines() if ln.strip()], None


# ────────────────────────────────── Selbsttest ──────────────────────────────────

def selftest():
    """Die drei Abnahmen als ausgefuehrte Faelle, plus die Zaehl-Zusagen."""
    present = {
        "backend", "frontend",
        "scripts/restore_test.sh", "scripts/backup_cron_test.sh",
        "scripts/backup_restore_wiring_test.sh",
    }
    ex = lambda p: p in present  # noqa: E731

    baseline = {"test": [
        "cd backend && go test ./...",
        "cd frontend && npm test",
        "bash scripts/restore_test.sh",
        "bash scripts/backup_cron_test.sh",
        "bash scripts/backup_restore_wiring_test.sh",
        'for t in ; do echo "bash $t"; bash "$t" || exit 1; done',
        "echo interner Sites-Stack: 0/2 Suiten gelaufen",
    ]}

    cases = []

    # 1 — gruen auf der Baseline.
    pr, st = evaluate(baseline, ex)
    cases.append(("1 Baseline gruen", not pr, pr))

    # 2 — ECHTE Regression: die #67/#76-Zeile ist wieder da. ROT MIT NAMEN.
    regress = {"test": baseline["test"] + ["bash infra/server/scripts/vakt-deploy_test.sh"]}
    pr, _ = evaluate(regress, ex)
    cases.append((
        "2 Regression (infra-Suite zurueck) rot + benannt",
        len(pr) == 1 and "infra/server/scripts/vakt-deploy_test.sh" in pr[0],
        pr,
    ))

    # 3 — Verbesserung nicht festgehalten: die im Mirror lauffaehige Suite fliegt raus.
    dropped = {"test": [l for l in baseline["test"] if "wiring" not in l]}
    pr, _ = evaluate(dropped, ex)
    cases.append((
        "3 Ratchet (gespiegelte Suite entfernt) rot + benannt",
        len(pr) == 1 and "backup_restore_wiring_test.sh" in pr[0] and "Ratchet" in pr[0],
        pr,
    ))

    # 4 — Shell-Variable wird gezaehlt, nicht als OK verbucht.
    _, st = evaluate({"test": baseline["test"] + ['bash "$t"']}, ex)
    cases.append(("4 Shell-Variable als skipped gezaehlt", len(st["unresolved"]) == 2, st["unresolved"]))

    # 5 — unbekannte Kommandoform wird gezaehlt, nicht verschluckt.
    _, st = evaluate({"test": baseline["test"] + ["frobnicate --all"]}, ex)
    cases.append(("5 unbekanntes Kommando gezaehlt", len(st["unclassified"]) == 1, st["unclassified"]))

    # 6 — Pfad existiert nicht: ROT (die Grundzusage, ohne Ratchet-Umweg).
    pr, _ = evaluate({"gates": ["python3 scripts/gibtsnicht.py"]}, ex, ratchet=[])
    cases.append(("6 fehlender Pfad in beliebigem Ziel rot", len(pr) == 1 and "gibtsnicht" in pr[0], pr))

    # 7 — DIE GESCHLOSSENE UMGEHUNG (REV-R76 §4/F4, gemessen): dieselbe
    # #67/#76-Regression, nur NACKT geschrieben — kein `bash` davor, kein `./`.
    # Diese Form landete in `unclassified`, das Gate blieb gruen, und `make test`
    # starb im Mirror trotzdem mit exit 127 an genau dieser Zeile.
    naked = {"test": baseline["test"] + ["infra/server/scripts/backup-offsite_test.sh"]}
    pr, st = evaluate(naked, ex)
    cases.append((
        "7 nackte Pfadform (Umgehung) rot + benannt",
        len(pr) == 1 and "infra/server/scripts/backup-offsite_test.sh" in pr[0]
        and not st["unclassified"],
        (pr, st["unclassified"]),
    ))

    # 8 — `make -C <verzeichnis>`: dasselbe Loch eine Ebene hoeher. `make` steht
    # in BENIGN, das Verzeichnis wurde vorher nicht einmal GEZAEHLT.
    pr, _ = evaluate({"test": baseline["test"] + ["make -C infra/server all"]}, ex)
    cases.append((
        "8 make -C auf nicht gespiegeltes Verzeichnis rot",
        len(pr) == 1 and "infra/server" in pr[0],
        pr,
    ))

    # 9 — Gegenprobe zur Umgehungs-Erkennung: ein absoluter Systempfad ist KEIN
    # Repo-Pfad. Er darf nicht rot faerben (sonst waere jede `[ -x /usr/bin/… ]`-
    # Zeile ein Dauerfehler), muss aber gezaehlt und benannt werden.
    pr, st = evaluate({"gates": ["/usr/bin/rrsync --help"]}, ex, ratchet=[])
    cases.append((
        "9 absoluter Systempfad nicht rot, aber gezaehlt",
        not pr and len(st["unclassified"]) == 1,
        (pr, st["unclassified"]),
    ))

    bad = [c for c in cases if not c[1]]
    for name, ok, detail in cases:
        print(f"  {'ok  ' if ok else 'FAIL'}  {name}" + ("" if ok else f"   -> {detail}"))
    if bad:
        print(f"\nSELBSTTEST FEHLGESCHLAGEN — {len(bad)}/{len(cases)}")
        return 1
    print(f"\nSelbsttest: alle {len(cases)} Faelle wie erwartet "
          f"(1 gruen, 5 rot mit Namensnennung, 3 Zaehl-Zusagen)")
    return 0


# ──────────────────────────────────── main ──────────────────────────────────────

NOT_CHECKED = [
    "die Suiten werden NICHT ausgefuehrt — nur aufgeloest",
    "der go-Build des Mirrors (VAKT_MIRROR_SKIP_GO_BUILD=1) — laeuft in sync-public-repo.yml",
    "`make public-mirror` im Mirror: totes Ziel (build-public-mirror.sh wird nicht gespiegelt)",
    "`make gates` im Mirror: alle Pfade da, bricht aber an check_ci_steps.py ab "
    "(.github/workflows/ nicht gespiegelt) — andere Klasse, offener Befund",
    "Ops-/Dev-Ziele (dev, stop, billing, *-local) — kein Repo-Pfad im Rezept",
]


def main():
    if "--selftest" in sys.argv:
        print("G-MIRROR-TEST — Selbsttest")
        return selftest()

    print("G-MIRROR-TEST — `make test` im gebauten Public Mirror")

    with tempfile.TemporaryDirectory(prefix="vakt-mirror-gate-") as tmp:
        out = pathlib.Path(tmp) / "mirror"
        proc, unmeasurable = build_mirror(out)
        if unmeasurable:
            # Nicht pruefbar ist NICHT gruen und NICHT rot: gezaehlt, benannt,
            # exit 0. Ein Gate, das an der Umgebung scheitert statt am Code,
            # wird abgeschaltet statt gefixt.
            print(f"  nicht pruefbar : {unmeasurable}")
            print("  ziele geprueft : 0/%d" % len(TARGETS))
            print("NICHT GEMESSEN — kein Urteil ueber `make test` im Mirror (exit 0).")
            return 0
        if proc.returncode != 0:
            print("\nFEHLER — der Mirror-Build selbst bricht ab:")
            for ln in (proc.stdout + proc.stderr).splitlines()[-25:]:
                print("  " + ln)
            return 1

        n_files = sum(1 for _ in out.rglob("*") if _.is_file())
        recipes, target_errors = {}, []
        for t in TARGETS:
            lines, err = recipe_lines(out, t)
            if err:
                target_errors.append((t, err))
            recipes[t] = lines

        problems, st = evaluate(recipes, lambda p: (out / p).exists())

    for t, err in target_errors:
        problems.append(f"make -n {t} im Mirror scheitert: {err}")

    print(f"  mirror         : {n_files} Dateien, gebaut ohne go-Build")
    print(f"  ziele geprueft : {len(TARGETS) - len(target_errors)}/{len(TARGETS)}")
    print(f"  rezeptzeilen   : {st['lines']}")
    print(f"  pfade geprueft : {st['paths']}")
    print(f"  ratchet        : {st['ratchet_ok']}/{st['ratchet_total']} erwartete Ziele "
          f"weiterhin in `make test` und im Mirror vorhanden")
    print(f"  skipped        : {len(st['unresolved'])} Pfad(e) aus Shell-Variablen, "
          f"{len(st['unclassified'])} unbekannte Kommandoform(en)")
    for tgt, item in st["unresolved"] + st["unclassified"]:
        print(f"                   · {tgt}: {item}")
    print("  nicht geprueft :")
    for n in NOT_CHECKED:
        print(f"                   · {n}")

    if problems:
        print(f"\nFEHLER ({len(problems)}):\n")
        for p in problems:
            print(f"  · {p}")
        if any(not p.startswith("Ratchet:") for p in problems):
            print("\n  Ein fehlender Pfad bricht `make test` im Kundenrepo ab, bevor die")
            print("  nachfolgenden Suiten ueberhaupt laufen. Fix: den Aufruf ueber")
            print("  $(wildcard …) fuehren (siehe INTERNAL_TEST_SUITES im Makefile) — oder")
            print("  die Datei in scripts/build-public-mirror.sh mitspiegeln, falls sie")
            print("  wirklich ins Kundenprodukt gehoert.")
        return 1

    print("\nOK — jeder Pfad, den die Test-/DoD-Kette im Mirror aufruft, liegt dort.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
