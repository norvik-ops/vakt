#!/usr/bin/env python3
"""G-MIRROR-REFS — was der Public Mirror verspricht, muss IM Mirror liegen.

Warum es dieses Gate gibt (2026-08-02)
--------------------------------------
Vier Befunde einer Runde, eine Klasse: eine Datei, die gespiegelt wird, verweist
auf einen Pfad, den `scripts/build-public-mirror.sh` nicht mitnimmt. Im privaten
Repo existiert der Pfad, also war jeder Diff dort gruen.

  R1-05-C01  backend/Dockerfile baute `./cmd/billing` — Norviks eigenen Lizenz-
             dienst, den der Sync bewusst nicht mitspiegelt. `docker compose build
             api` starb im Kundenrepo mit `stat /app/cmd/billing: directory not
             found` (rc=1, gemessen im gebauten Mirror). Drei Zwillinge in
             derselben Fundstelle: `COPY /app/NOTICE` ohne backend/NOTICE,
             docker-compose.yml -> Dockerfile.scanners, docker-compose.dev.yml ->
             Dockerfile.dev.
  R1-04-L08  README/setup.md/UPGRADE.md/wiki wiesen zu `helm install vakt
             ./helm/vakt` an — helm/ war NIE im Mirror (gegen das echte
             Public-Repo geprueft: contents/helm = 404).

Der vorhandene Compile-Check in build-public-mirror.sh sieht das strukturell
nicht: er faehrt `go build ./...` im Mirror, und `./...` laeuft ueber die Pakete,
die DA sind. Ein Bauplan, der ein fehlendes Verzeichnis nennt, faellt dabei nicht
auf. Das Schwesterngate check_mirror_make_test.py macht dieselbe Messung fuer das
Makefile; dieses hier macht sie fuer Bauplaene und Doku.

Was es prueft
-------------
Es BAUT den Mirror nach einem Tempdir und arbeitet danach nur in diesem Baum.

  A  Compose-Build-Referenzen: jeder `build:`-Block in einer gespiegelten
     docker-compose*.yml -> `context` und `dockerfile` muessen im Mirror liegen.
  B  Dockerfile-Referenzen: in jedem gespiegelten Dockerfile
       · `go build … ./cmd/<x>`             -> <context>/cmd/<x>
       · `COPY --from=<stage> /app/<p> …`   -> <context>/<p>
       · `COPY <src> …` ohne --from         -> <context>/<src>
  C  Doku-Pfadverweise: ein Pfad, den eine gespiegelte *.md nennt, der im
     QUELLREPO existiert, im Mirror aber nicht. Die Einschraenkung auf "existiert
     in der Quelle" ist Absicht: ein Pfad, den es auch privat nicht gibt, ist ein
     Doku-Fehler und gehoert zu check-docs.py, nicht hierher.
  D  Chart-Repo-Behauptung: eine gespiegelte *.md darf den Kunden nicht zu
     `helm repo add/update` fuer ein Vakt-Chart schicken. Es gibt kein
     veroeffentlichtes Chart-Repository (kein chart-releaser, 0 Release-Assets,
     ghcr-Chart-Pull DENIED) — die Anweisung laeuft ins Leere. Wird eines
     publiziert, faellt dieser Teil ersatzlos weg; die Liste steht in
     CHART_REPO_PATTERNS.

Sein Nenner — was dieses Gate NICHT ansieht
-------------------------------------------
* ES BAUT KEIN IMAGE. Geprueft wird, dass jeder genannte Pfad da ist, nicht dass
  `docker build` durchlaeuft. Ein echter Bau ist Minuten und braucht Netz; er
  laeuft im Sync-Job. Gemessen wurde er einmal von Hand fuer R1-05-C01.
* ES BAUT DEN MIRROR OHNE `go build` (VAKT_MIRROR_SKIP_GO_BUILD=1) — wie das
  Schwesterngate, aus demselben Laufzeitgrund.
* MEHRSTUFIGE COPY-QUELLEN AUS FREMDEN STAGES (`COPY --from=trivy-src
  /usr/local/bin/trivy …`) sind Pfade IM Fremd-Image, nicht im Repo. Sie werden
  gezaehlt und benannt, nie aufgeloest.
* SHELL-EXPANSIONEN und Globs in Pfadposition (`$VAR`, `*`) werden gezaehlt und
  benannt, nie als OK verbucht.
* DOKU-PFADE nur in Backticks oder als `./`-Token in einem Codeblock. Ein Pfad im
  Fliesstext ohne Auszeichnung wird nicht erkannt — gezaehlt werden kann er nicht,
  also steht er hier als bekannte Luecke.
* NUR GESPIEGELTE DATEIEN. Was der Sync gar nicht mitnimmt, ist nicht das Thema
  dieses Gates.

Selbsttest: `python3 scripts/check_mirror_refs.py --selftest`
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

# ── D: Formen, die ein veroeffentlichtes Chart-Repository voraussetzen.
# `helm repo add|update` und `helm install|upgrade <release> <repo>/<chart>`.
# Der Pfad-Aufruf (`./helm/vakt`) ist ausdruecklich in Ordnung — er ist der Weg,
# der funktioniert, seit helm/ mitgespiegelt wird.
CHART_REPO_PATTERNS = [
    (re.compile(r"\bhelm\s+repo\s+(add|update)\b"),
     "`helm repo add/update` — es gibt kein veroeffentlichtes Vakt-Chart-Repository"),
    (re.compile(r"\bhelm\s+(install|upgrade)\s+\S+\s+(?!\./|/|-)[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+"),
     "`helm install/upgrade <release> <repo>/<chart>` — Chart-Repo-Form ohne Chart-Repo; "
     "Pfad-Form `./helm/vakt` benutzen"),
]

# ── C: Nur Pfade unter diesen Wurzeln werden ueberhaupt als Repo-Pfad gelesen.
# Ohne diese Klammer wuerde jedes `foo/bar` im Fliesstext mitgezaehlt.
DOC_PATH_ROOTS = (
    "helm/", "scripts/", "backend/", "frontend/", "docs/", "infra/", ".github/",
    "integrations/", "db/",
)
# In Backticks: `./helm/vakt`, `helm/vakt/values.yaml`, `scripts/backup.sh`
DOC_TICK = re.compile(r"`([^`\n]+)`")

RUNNER_GO_BUILD = re.compile(r"\bgo\s+build\b[^&|;\n]*?(\./cmd/[A-Za-z0-9_.-]+)")
COPY_LINE = re.compile(r"^\s*COPY\s+(.*)$", re.IGNORECASE)
FROM_FLAG = re.compile(r"^--from=([^\s]+)$")


# ────────────────────────────── reine Pruefteile ──────────────────────────────

def compose_build_refs(text):
    """-> [(service, context, dockerfile|None)] aus einer docker-compose*.yml.

    Bewusst zeilenbasiert statt per YAML-Parser: PyYAML ist in diesem Repo keine
    harte Abhaengigkeit der Gates, und die Compose-Dateien sind flach genug.
    """
    refs, service, in_build, ctx, dfile = [], None, False, None, None

    def flush():
        if service and ctx is not None:
            refs.append((service, ctx, dfile))

    for raw in text.splitlines():
        if re.match(r"^  [A-Za-z0-9_.-]+:\s*$", raw):
            flush()
            service, in_build, ctx, dfile = raw.strip().rstrip(":"), False, None, None
            continue
        if re.match(r"^    build:\s*$", raw):
            in_build = True
            continue
        if in_build:
            m = re.match(r"^      context:\s*(\S+)", raw)
            if m:
                ctx = m.group(1)
                continue
            m = re.match(r"^      dockerfile:\s*(\S+)", raw)
            if m:
                dfile = m.group(1)
                continue
            if not raw.startswith("      "):
                in_build = False
        # Einzeiler `build: ./backend`
        m = re.match(r"^    build:\s*(\S+)\s*$", raw)
        if m:
            ctx = m.group(1)
    flush()
    return refs


def dockerfile_refs(text):
    """-> (paths, optional, foreign, unresolved) fuer EIN Dockerfile, rel. zum Context.

    `paths`     muessen im Mirror liegen.
    `optional`  stehen in derselben RUN-Zeile unter einem `[ -d … ]`/`[ -f … ]`-Test.
                Genau so haengt der billing-Build seit dem Fix zu R1-05-C01: das
                private Repo hat cmd/billing und baut es, der Mirror hat es nicht
                und baut es nicht. Ein Pfad unter einem Test ist eine Bedingung,
                keine Zusage — er wird gezaehlt und benannt, nie stillschweigend
                als vorhanden verbucht.
    """
    paths, optional, foreign, unresolved = [], [], [], []
    stages = set()
    # Ein Dockerfile-RUN kann ueber Backslashes viele Zeilen gehen; die Wache und
    # der bewachte Build stehen dann in DERSELBEN logischen Zeile.
    logical, buf = [], ""
    for raw in text.splitlines():
        if raw.rstrip().endswith("\\"):
            buf += raw.rstrip()[:-1] + " "
            continue
        logical.append(buf + raw)
        buf = ""
    if buf:
        logical.append(buf)

    for raw in logical:
        line = raw.strip()
        m = re.match(r"^FROM\s+.*\bAS\s+(\S+)\s*$", line, re.IGNORECASE)
        if m:
            stages.add(m.group(1))
        guarded = {g.lstrip("./").rstrip("/")
                   for g in re.findall(r"\[\s+-[dfe]\s+(\S+)\s+\]", line)}
        for hit in RUNNER_GO_BUILD.findall(line):
            p = hit[2:]  # './cmd/x' -> 'cmd/x'
            if "$" in p:
                unresolved.append(p)
            elif p in guarded:
                optional.append(p)
            else:
                paths.append(p)
        m = COPY_LINE.match(raw)
        if not m:
            continue
        try:
            argv = shlex.split(m.group(1))
        except ValueError:
            unresolved.append(m.group(1))
            continue
        src_from = None
        rest = []
        for a in argv:
            f = FROM_FLAG.match(a)
            if f:
                src_from = f.group(1)
            elif a.startswith("--"):
                continue
            else:
                rest.append(a)
        if len(rest) < 2:
            continue
        for src in rest[:-1]:
            if "$" in src or "*" in src:
                unresolved.append(src)
                continue
            if src_from is not None:
                # Aus einer FREMDEN Stage (anderes Image): Pfad im Image, nicht im Repo.
                # Aus einer EIGENEN Stage: nur /app/<p> KANN ein Repo-Pfad sein —
                # /out/, /data und /app/dist entstehen erst im Bau. Ob es einer ist,
                # entscheidet der Aufrufer am Quellrepo (WORKDIR /app haelt genau
                # den Context-Inhalt); alles andere wird gezaehlt und benannt.
                if src_from in stages and src.startswith("/app/"):
                    foreign.append(("maybe-repo", src_from, src[len("/app/"):].rstrip("/"), src))
                else:
                    foreign.append(("image", src_from, None, src))
                continue
            if src in (".", "./"):
                continue
            paths.append(src.lstrip("./").rstrip("/"))
    return paths, optional, foreign, unresolved


def doc_path_refs(text):
    """-> Menge der Repo-Pfade, die dieses Markdown in Backticks nennt."""
    out = set()
    for tok in DOC_TICK.findall(text):
        tok = tok.strip()
        # Ein Backtick-Block kann eine ganze Befehlszeile sein.
        for word in tok.split():
            w = word.strip("(),;:\"'").rstrip(".")
            if w.startswith("./"):
                w = w[2:]
            if not w.startswith(DOC_PATH_ROOTS):
                continue
            if any(c in w for c in "$*<>{}|"):
                continue
            out.add(w.rstrip("/"))
    return out


def fenced(text):
    """Nur der Inhalt der ```-Bloecke. Eine Anweisung, die ein Kunde kopiert,
    steht im Codeblock; ein Satz UEBER einen Befehl ist keine Anweisung."""
    out, inside = [], False
    for line in text.splitlines():
        if line.lstrip().startswith("```"):
            inside = not inside
            continue
        if inside:
            out.append(line)
    return "\n".join(out)


def evaluate(compose_files, dockerfiles, docs, in_mirror, in_source, baseline=frozenset()):
    """Reine Pruefung — vom Selbsttest mit synthetischen Eingaben gefahren.

    compose_files  {relpfad: text}
    dockerfiles    {relpfad: (context_relpfad, text)}
    docs           {relpfad: text}
    in_mirror      callable(relpfad) -> bool
    in_source      callable(relpfad) -> bool
    baseline       Menge "<mddatei>::<pfad>" bekannt haengender Doku-Verweise (C).
                   Jede Zeile darin ist ein OFFENER Defekt, kein gutgeheissener
                   Zustand — sie haelt nur den Stand fest, damit ein NEUER
                   haengender Verweis auffaellt. Ratchet in beide Richtungen:
                   ein Eintrag, der nicht mehr haengt, muss raus, sonst ist die
                   Verbesserung nicht festgehalten.
    """
    problems = []
    n_refs = 0
    optional, foreign, unresolved = [], [], []
    seen_dangling = set()

    # A — Compose
    for f, text in compose_files.items():
        for service, ctx, dfile in compose_build_refs(text):
            base = ctx.lstrip("./").rstrip("/")
            n_refs += 1
            if not in_mirror(base):
                problems.append(
                    f"{f}: Dienst `{service}` baut aus `{ctx}` — dieses Verzeichnis "
                    f"gibt es im gebauten Mirror nicht.")
            if dfile:
                n_refs += 1
                p = f"{base}/{dfile}"
                if not in_mirror(p):
                    problems.append(
                        f"{f}: Dienst `{service}` braucht `{p}` — im gebauten Mirror "
                        f"gibt es die Datei nicht. `docker compose build {service}` "
                        f"bricht damit im Kundenrepo ab.")

    # B — Dockerfiles
    for f, (ctx, text) in dockerfiles.items():
        paths, opts, foreigns, unres = dockerfile_refs(text)
        base = ctx.lstrip("./").rstrip("/")
        unresolved += [(f, x) for x in unres]
        optional += [(f, x) for x in opts]
        for kind, stage, maybe, shown in foreigns:
            # Eine /app/<p>-Quelle aus einer eigenen Stage ist genau dann ein
            # Repo-Pfad, wenn es sie im Quellrepo gibt (WORKDIR /app = Context).
            # Sonst ist sie eine Bau-Ausgabe (frontend/dist, /out/) — gezaehlt.
            if kind == "maybe-repo":
                full = f"{base}/{maybe}" if base else maybe
                if in_source(full):
                    n_refs += 1
                    if not in_mirror(full):
                        problems.append(
                            f"{f}: kopiert `{shown}` (also `{full}`) — im gebauten "
                            f"Mirror gibt es das nicht. Der Bau des Kunden-Images "
                            f"bricht hier ab.")
                    continue
            foreign.append((f, f"--from={stage} {shown}"))
        for p in paths:
            n_refs += 1
            full = f"{base}/{p}" if base else p
            if not in_mirror(full):
                problems.append(
                    f"{f}: verweist auf `{p}` (also `{full}`) — im gebauten Mirror "
                    f"gibt es das nicht. Der Bau des Kunden-Images bricht hier ab.")

    # C + D — Doku
    for f, text in docs.items():
        for p in sorted(doc_path_refs(text)):
            n_refs += 1
            if in_mirror(p):
                continue
            if not in_source(p):
                continue  # existiert auch privat nicht -> check-docs.py, nicht hier
            key = f"{f}::{p}"
            seen_dangling.add(key)
            if key in baseline:
                continue
            problems.append(
                f"{f}: nennt `{p}` — der Pfad existiert im Quellrepo, aber "
                f"scripts/build-public-mirror.sh spiegelt ihn nicht. Der Kunde "
                f"liest eine Anweisung, die er nicht ausfuehren kann.")
        code = fenced(text)
        for rx, why in CHART_REPO_PATTERNS:
            for m in rx.finditer(code):
                problems.append(
                    f"{f}: {why}. Gefunden: `{m.group(0).strip()}`")

    # Ratchet-Rueckrichtung: ein Baseline-Eintrag, der nicht mehr haengt, ist eine
    # Verbesserung — und die gehoert festgehalten, sonst kann sie unbemerkt
    # zurueckfallen. Ohne diese Haelfte waere die Baseline eine Muellhalde.
    for key in sorted(baseline - seen_dangling):
        problems.append(
            f"Baseline: `{key}` haengt nicht mehr — Zeile aus "
            f"scripts/mirror_refs_baseline.txt entfernen, damit der Verweis nicht "
            f"unbemerkt zurueckfallen kann.")

    return problems, {
        "refs": n_refs,
        "optional": optional,
        "foreign": foreign,
        "unresolved": unresolved,
        "baseline_used": len(seen_dangling & set(baseline)),
        "baseline_total": len(baseline),
    }


# ─────────────────────────── Mirror bauen + einlesen ────────────────────────────

def build_mirror(out):
    script = ROOT / "scripts" / "build-public-mirror.sh"
    if not script.is_file():
        return None, ("scripts/build-public-mirror.sh fehlt — dieses Repo ist "
                      "vermutlich selbst der Mirror. Nichts zu bauen.")
    for tool in ("bash", "rsync"):
        if shutil.which(tool) is None:
            return None, f"{tool} fehlt im PATH — Mirror nicht baubar."
    env = dict(os.environ, VAKT_MIRROR_OUT=str(out), VAKT_MIRROR_SKIP_GO_BUILD="1")
    p = subprocess.run(["bash", str(script)], cwd=str(ROOT), env=env,
                       capture_output=True, text=True, timeout=900)
    return p, None


def source_index():
    """-> (callable(relpfad)->bool, wie-ermittelt)

    "Existiert im Quellrepo" heisst VON GIT VERFOLGT, nicht "liegt im
    Arbeitsverzeichnis". Der Unterschied ist gemessen und nicht theoretisch:
    `make check` faehrt `npm run build` VOR `make gates`, danach liegt
    `frontend/dist/` im Baum — und dieses Gate hielt `COPY --from=builder
    /app/dist` prompt fuer einen nicht gespiegelten Repo-Pfad und wurde rot.
    Ein Gate, dessen Urteil davon abhaengt, was vorher gebaut wurde, ist kein
    Gate. Verfolgte Dateien sind die stabile Frage.
    """
    try:
        p = subprocess.run(["git", "-C", str(ROOT), "ls-files", "-z"],
                           capture_output=True, text=True, timeout=120)
        if p.returncode == 0:
            tracked = set()
            for f in p.stdout.split("\0"):
                if not f:
                    continue
                tracked.add(f)
                parts = f.split("/")
                for i in range(1, len(parts)):
                    tracked.add("/".join(parts[:i]))
            return (lambda x: x in tracked), f"git ls-files ({len(tracked)} Pfade)"
    except (OSError, subprocess.SubprocessError):
        pass
    return ((lambda x: (ROOT / x).exists()),
            "Dateisystem (git nicht verfuegbar) — Bau-Ausgaben koennen mitzaehlen")


def collect(mirror):
    compose_files, dockerfiles, docs = {}, {}, {}
    for p in sorted(mirror.glob("docker-compose*.yml")):
        compose_files[p.name] = p.read_text(encoding="utf-8", errors="replace")
    for p in sorted(mirror.rglob("Dockerfile*")):
        if not p.is_file():
            continue
        rel = p.relative_to(mirror)
        ctx = str(rel.parent) if str(rel.parent) != "." else ""
        dockerfiles[str(rel)] = (ctx, p.read_text(encoding="utf-8", errors="replace"))
    for p in sorted(mirror.rglob("*.md")):
        rel = str(p.relative_to(mirror))
        if rel == "CHANGELOG.md":
            continue  # Historie, wie beim Docs-Guard in build-public-mirror.sh
        docs[rel] = p.read_text(encoding="utf-8", errors="replace")
    return compose_files, dockerfiles, docs


# ────────────────────────────────── Selbsttest ──────────────────────────────────

def selftest():
    """Die drei Abnahmen als ausgefuehrte Faelle, plus die Zaehl-Zusagen."""
    mirror = {
        "backend", "backend/Dockerfile", "backend/Dockerfile.scanners",
        "backend/cmd/api", "backend/cmd/worker", "backend/NOTICE",
        "backend/go.mod", "backend/go.sum",
        "helm", "helm/vakt", "helm/vakt/values.yaml", "scripts/backup.sh",
    }
    # `backend/CREDITS` steht NUR in der Quelle — das ist die NOTICE-Lage aus
    # R1-05-C01: die Datei existiert privat, der Sync nimmt sie nicht mit.
    source = mirror | {"backend/cmd/billing", "backend/CREDITS", "infra/server",
                       "scripts/deploy-gate.sh"}
    inm = lambda p: p in mirror     # noqa: E731
    ins = lambda p: p in source     # noqa: E731

    good_df = (
        "FROM golang AS builder\n"
        "COPY go.mod go.sum ./\n"
        "RUN go build -o /out/api ./cmd/api && go build -o /out/worker ./cmd/worker\n"
        "FROM scratch\n"
        "COPY --from=builder /out/ /\n"
        "COPY --from=builder /app/NOTICE /NOTICE\n"
    )
    base_dockerfiles = {"backend/Dockerfile": ("backend", good_df)}
    base_compose = {"docker-compose.yml": (
        "services:\n"
        "  scanners:\n"
        "    build:\n"
        "      context: ./backend\n"
        "      dockerfile: Dockerfile.scanners\n"
    )}
    base_docs = {"docs/setup.md": "Installiere mit `helm install vakt ./helm/vakt`.\n"}

    cases = []

    # 1 — gruen auf der Baseline.
    pr, _ = evaluate(base_compose, base_dockerfiles, base_docs, inm, ins)
    cases.append(("1 Baseline gruen", not pr, pr))

    # 2 — ECHTE Regression R1-05-C01: der billing-Build ist zurueck. ROT MIT NAMEN.
    regress = {"backend/Dockerfile": ("backend", good_df.replace(
        "./cmd/worker\n", "./cmd/worker && go build -o /out/billing ./cmd/billing\n"))}
    pr, _ = evaluate(base_compose, regress, base_docs, inm, ins)
    cases.append(("2 Dockerfile baut nicht gespiegeltes cmd/ -> rot + benannt",
                  len(pr) == 1 and "cmd/billing" in pr[0], pr))

    # 3 — ECHTE Regression: COPY einer Datei, die der Sync nicht mitnimmt.
    regress = {"backend/Dockerfile": ("backend", good_df.replace(
        "/app/NOTICE /NOTICE\n", "/app/NOTICE /NOTICE\nCOPY --from=builder /app/CREDITS /CREDITS\n"))}
    pr, _ = evaluate(base_compose, regress, base_docs, inm, ins)
    cases.append(("3 COPY auf fehlende Datei -> rot + benannt",
                  len(pr) == 1 and "CREDITS" in pr[0], pr))

    # 4 — ECHTE Regression R1-05-C01 (Compose-Haelfte): dockerfile: fehlt im Mirror.
    regress = {"docker-compose.dev.yml": (
        "services:\n  api:\n    build:\n      context: ./backend\n"
        "      dockerfile: Dockerfile.dev\n")}
    pr, _ = evaluate(regress, base_dockerfiles, base_docs, inm, ins)
    cases.append(("4 Compose zeigt auf nicht gespiegeltes Dockerfile -> rot + benannt",
                  len(pr) == 1 and "Dockerfile.dev" in pr[0], pr))

    # 5 — ECHTE Regression R1-04-L08: die Doku nennt einen Pfad, den der Sync
    # weglaesst, den es privat aber gibt.
    pr, _ = evaluate(base_compose, base_dockerfiles,
                     {"docs/x.md": "Siehe `infra/server` fuer Details.\n"}, inm, ins)
    cases.append(("5 Doku nennt nicht gespiegelten Pfad -> rot + benannt",
                  len(pr) == 1 and "infra/server" in pr[0], pr))

    # 6 — Gegenprobe zu 5: ein Pfad, den es auch privat NICHT gibt, gehoert
    # check-docs.py. Er darf hier nicht rot faerben (sonst zwei Gates, ein Thema).
    pr, _ = evaluate(base_compose, base_dockerfiles,
                     {"docs/x.md": "Siehe `docs/gibtsnicht.md`.\n"}, inm, ins)
    cases.append(("6 Pfad existiert auch privat nicht -> nicht rot", not pr, pr))

    # 7 — ECHTE Regression R1-04-L08 (zweite Haelfte): die Chart-Repo-Anweisung.
    pr, _ = evaluate(base_compose, base_dockerfiles,
                     {"docs/UPGRADE.md": "```bash\nhelm repo update\n"
                                         "helm upgrade vakt vakt/vakt\n```\n"}, inm, ins)
    cases.append(("7 Chart-Repo-Anweisung -> rot + benannt (2 Formen)",
                  len(pr) == 2 and any("helm repo" in p for p in pr)
                  and any("<repo>/<chart>" in p for p in pr), pr))

    # 8 — Gegenprobe zu 7: die Pfad-Form ist der Weg, der funktioniert.
    pr, _ = evaluate(base_compose, base_dockerfiles,
                     {"docs/UPGRADE.md": "```bash\nhelm upgrade vakt ./helm/vakt "
                                         "-f values.yaml\n```\n"}, inm, ins)
    cases.append(("8 Pfad-Form helm upgrade ./helm/vakt -> nicht rot", not pr, pr))

    # 9 — Gegenprobe zu 7, zweite Haelfte: ein SATZ ueber `helm repo add` ist keine
    # Anweisung. Genau diese Form steht seit dem Fix in docs/UPGRADE.md ("es gibt
    # kein Chart-Repo, `helm repo add` hat hier nichts zu holen") — wuerde sie rot
    # faerben, koennte man den Defekt nicht erklaeren, ohne ihn zu begehen.
    pr, _ = evaluate(base_compose, base_dockerfiles,
                     {"docs/UPGRADE.md": "Es gibt kein Chart-Repo, `helm repo add` "
                                         "hat hier nichts zu holen.\n"}, inm, ins)
    cases.append(("9 Chart-Repo-Erwaehnung im Fliesstext -> nicht rot", not pr, pr))

    # 10 — Zaehl-Zusage: COPY aus einem FREMDEN Image wird gezaehlt, nicht aufgeloest.
    _, st = evaluate({}, {"backend/Dockerfile.scanners": ("backend",
        "FROM aquasec/trivy AS trivy-src\nFROM alpine\n"
        "COPY --from=trivy-src /usr/local/bin/trivy /scanners/trivy\n")}, {}, inm, ins)
    cases.append(("10 Fremd-Stage-COPY gezaehlt, nicht rot", len(st["foreign"]) == 1,
                  st["foreign"]))

    # 11 — Zaehl-Zusage: Shell-Expansion in Pfadposition wird gezaehlt.
    _, st = evaluate({}, {"backend/Dockerfile": ("backend",
        "FROM x\nCOPY ${SRC} /dst\n")}, {}, inm, ins)
    cases.append(("11 Shell-Expansion als unresolved gezaehlt",
                  len(st["unresolved"]) == 1, st["unresolved"]))

    # 12 — Zaehl-Zusage + Kern des C01-Fixes: ein Build unter `[ -d … ]` ist eine
    # Bedingung, keine Zusage. Nicht rot, aber gezaehlt und benannt.
    guarded = {"backend/Dockerfile": ("backend", good_df.replace(
        "FROM scratch\n",
        "RUN if [ -d ./cmd/billing ]; then go build -o /out/billing ./cmd/billing; fi\n"
        "FROM scratch\n"))}
    pr, st = evaluate(base_compose, guarded, base_docs, inm, ins)
    cases.append(("12 bewachter Build (cmd/billing) -> nicht rot, aber gezaehlt",
                  not pr and len(st["optional"]) == 1
                  and st["optional"][0][1] == "cmd/billing", (pr, st["optional"])))

    # 13 — Gegenprobe zu 12: DERSELBE Build OHNE Wache ist wieder rot. Ohne diesen
    # Fall koennte man die Wache-Erkennung zum Freibrief machen.
    unguarded = {"backend/Dockerfile": ("backend", good_df.replace(
        "FROM scratch\n",
        "RUN go build -o /out/billing ./cmd/billing\nFROM scratch\n"))}
    pr, _ = evaluate(base_compose, unguarded, base_docs, inm, ins)
    cases.append(("13 derselbe Build ohne Wache -> rot",
                  len(pr) == 1 and "cmd/billing" in pr[0], pr))

    # 14 — Zaehl-Zusage: /app/<p> aus eigener Stage, das im Quellrepo NICHT
    # VERFOLGT ist, ist eine Bau-Ausgabe (frontend/dist), kein Repo-Pfad. Nicht
    # rot, gezaehlt. Gemessen: `make check` baut das Frontend VOR `make gates`,
    # danach LIEGT frontend/dist im Baum — verfolgt ist es trotzdem nicht.
    _, st = evaluate({}, {"frontend/Dockerfile": ("frontend",
        "FROM node AS builder\nFROM nginx\n"
        "COPY --from=builder /app/dist /usr/share/nginx/html\n")}, {}, inm, ins)
    cases.append(("14 Bau-Ausgabe /app/dist gezaehlt, nicht rot",
                  len(st["foreign"]) == 1, st["foreign"]))

    # 15 — Baseline: ein bekannter haengender Verweis faerbt nicht rot …
    dangling = {"SECURITY.md": "Siehe `infra/server`.\n"}
    pr, _ = evaluate(base_compose, base_dockerfiles, dangling, inm, ins,
                     baseline={"SECURITY.md::infra/server"})
    cases.append(("15 Baseline-Eintrag -> nicht rot", not pr, pr))

    # 16 — … und ein Baseline-Eintrag, der nicht mehr haengt, MUSS raus (Ratchet
    # rueckwaerts). Sonst waere die Baseline eine Muellhalde, die stillschweigend
    # waechst und nie schrumpft.
    pr, _ = evaluate(base_compose, base_dockerfiles, base_docs, inm, ins,
                     baseline={"SECURITY.md::infra/server"})
    cases.append(("16 erledigter Baseline-Eintrag -> rot + benannt",
                  len(pr) == 1 and "haengt nicht mehr" in pr[0], pr))

    bad = [c for c in cases if not c[1]]
    for name, ok, detail in cases:
        print(f"  {'ok  ' if ok else 'FAIL'}  {name}" + ("" if ok else f"   -> {detail}"))
    if bad:
        print(f"\nSELBSTTEST FEHLGESCHLAGEN — {len(bad)}/{len(cases)}")
        return 1
    print(f"\nSelbsttest: alle {len(cases)} Faelle wie erwartet "
          f"(6 gruen als Gegenprobe, 6 rot mit Namensnennung, 4 Zaehl-Zusagen)")
    return 0


# ──────────────────────────────────── main ──────────────────────────────────────

NOT_CHECKED = [
    "kein `docker build` — nur Pfadaufloesung (der echte Bau laeuft im Sync-Job)",
    "der go-Build des Mirrors (VAKT_MIRROR_SKIP_GO_BUILD=1) — laeuft in sync-public-repo.yml",
    "COPY-Quellen aus FREMDEN Image-Stages — Pfade im Fremd-Image, nicht im Repo",
    "Doku-Pfade ohne Backticks (Fliesstext) — nicht erkennbar, bekannte Luecke",
    "Pfade, die auch im Quellrepo fehlen — Zustaendigkeit von check-docs.py",
    "die BASELINE-Zeilen unten — bekannte, OFFENE haengende Doku-Verweise",
]

BASELINE_FILE = ROOT / "scripts" / "mirror_refs_baseline.txt"


def load_baseline():
    if not BASELINE_FILE.is_file():
        return set()
    out = set()
    for line in BASELINE_FILE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            out.add(line)
    return out


def main():
    if "--selftest" in sys.argv:
        print("G-MIRROR-REFS — Selbsttest")
        return selftest()

    print("G-MIRROR-REFS — jeder Verweis im Public Mirror zeigt in den Mirror")

    with tempfile.TemporaryDirectory(prefix="vakt-mirror-refs-") as tmp:
        out = pathlib.Path(tmp) / "mirror"
        proc, unmeasurable = build_mirror(out)
        if unmeasurable:
            print(f"  nicht pruefbar : {unmeasurable}")
            print("NICHT GEMESSEN — kein Urteil ueber die Verweise im Mirror (exit 0).")
            return 0
        if proc.returncode != 0:
            print("\nFEHLER — der Mirror-Build selbst bricht ab:")
            for ln in (proc.stdout + proc.stderr).splitlines()[-25:]:
                print("  " + ln)
            return 1

        compose_files, dockerfiles, docs = collect(out)
        n_files = sum(1 for _ in out.rglob("*") if _.is_file())
        in_source, how = source_index()
        problems, st = evaluate(
            compose_files, dockerfiles, docs,
            lambda p: (out / p).exists(),
            in_source,
            baseline=load_baseline(),
        )

    print(f"  mirror         : {n_files} Dateien, gebaut ohne go-Build")
    print(f"  quellrepo      : {how}")
    print(f"  compose        : {len(compose_files)} Datei(en)")
    print(f"  dockerfiles    : {len(dockerfiles)}")
    print(f"  markdown       : {len(docs)} (CHANGELOG.md ausgenommen)")
    print(f"  verweise       : {st['refs']} aufgeloest")
    print(f"  baseline       : {st['baseline_used']}/{st['baseline_total']} bekannte "
          f"haengende Doku-Verweise — OFFENE Defekte, nicht gutgeheissen "
          f"(scripts/mirror_refs_baseline.txt)")
    print(f"  skipped        : {len(st['optional'])} bewachte(r) Build-Pfad(e), "
          f"{len(st['foreign'])} Fremd-/Bau-Quelle(n), "
          f"{len(st['unresolved'])} Shell-Expansion(en)")
    for f, item in st["optional"] + st["foreign"] + st["unresolved"]:
        print(f"                   · {f}: {item}")
    print("  nicht geprueft :")
    for n in NOT_CHECKED:
        print(f"                   · {n}")

    if problems:
        print(f"\nFEHLER ({len(problems)}):\n")
        for p in problems:
            print(f"  · {p}")
        print("\n  Fix: den Pfad in scripts/build-public-mirror.sh mitspiegeln, falls er")
        print("  ins Kundenprodukt gehoert — oder den Verweis aus der gespiegelten Datei")
        print("  entfernen, falls nicht. Beides ist eine Entscheidung; still stehen")
        print("  lassen ist keine.")
        return 1

    print("\nOK — jeder Bauplan- und Doku-Verweis im Mirror zeigt auf etwas, das dort liegt.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
