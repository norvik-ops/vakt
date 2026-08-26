#!/usr/bin/env python3
"""G-UPGRADE-DOCS — ein dokumentierter Update-Weg muss die Auslieferungs-Dateien mitnehmen.

Warum es dieses Gate gibt (R1-06-D00, Codeaudit-v5b)
----------------------------------------------------
`docker-compose.yml`, `Caddyfile` und `scripts/` gehoeren zur Installation, nicht
zu den Images. `docker compose pull` fasst sie nicht an. Zwischen v0.42.48 und
v0.42.49 lagen dort 112 bzw. 33 geaenderte Zeilen — die komplette Umstellung auf
Docker-Secrets, `cap_drop` und `no-new-privileges`. Vier dokumentierte
Update-Wege beschrieben trotzdem nur `pull` + `up -d`; ein Bestandskunde bekam
damit neue Binaries in einer alten, ungehaerteten Umgebung.

`scripts/update.sh` traegt den Schritt jetzt selbst, und
`scripts/update_artifacts_test.sh` haelt das fest. Fuer die DOKU gab es keine
solche Klammer: wer den Schritt aus einer Anleitung wieder herausnimmt, macht
nichts rot. Genau diese Luecke schliesst dieses Gate.

Was es prueft
-------------
Geprueft wird je ABSCHNITT (Ueberschrift bis zur naechsten gleich- oder
hoeherrangigen), nicht je Zeile. Ein Abschnitt faellt auf, wenn beides gilt:

1. Er BESCHREIBT ein Update — seine Ueberschrift oder eine ihrer ECHTEN
   Elternueberschriften enthaelt "update", "upgrade", "aktualisier" oder
   "neue Version".
2. Und dann eines von dreien:
   a) Er zieht Images (`docker compose pull`), ohne irgendwo im Abschnitt einen
      Schritt fuer die Auslieferungs-Dateien zu nennen. Als solcher zaehlt
      `git pull`, `git merge`, `update-artifacts` oder ein Aufruf von
      `scripts/update.sh` (das Skript macht es selbst).
   b) Er spricht von Watchtower, ohne den Vorbehalt "Watchtower aktualisiert
      ausschliesslich Images" zu fuehren. Hier genuegt ein Artefakt-Schritt
      ausdruecklich NICHT — siehe unten.
   c) In einem AUSGEFUEHRTEN Block steht `docker compose up -d … --no-deps`.
      Das war die zweite Haelfte von R1-06-D00: caddy, postgres, redis und
      pgbouncer werden dann nicht neu erzeugt, und genau dort steckt die
      Haertung. Im Fliesstext bleibt es erlaubt.

Die Bindung an die Ueberschrift ist Absicht: ein `docker compose pull` in einem
Wiederherstellungs- oder Migrations-Runbook ist kein Update-Weg, und ein Gate,
das ueber gesunden Stellen rot wird, wird abgeschaltet statt gelesen.

Drei Regeln, die aus REV-W1C stammen und die frueheren Blindstellen schliessen
-----------------------------------------------------------------------------
* DIE H1 IST DER TITEL, NICHT DER WEG (F2). Vier Dateien heissen schon in ihrer
  H1 nach Update. Damit war der aeusserste Update-Abschnitt die ganze Datei, und
  ein einziges `./scripts/update.sh` in einem Weg machte jeden anderen gruen.
  Jetzt werden die H2-Wege einzeln bewertet — es sei denn, es gibt keine, dann
  bleibt die H1 die Einheit.
* GESCHWISTER SIND KEINE ELTERN. `sections()` bildete als "Kette" alle
  vorangehenden Ueberschriften bis zur eigenen Ebene ab. In
  `docs/update-guide.md` galt `## Quick Update` dadurch als Elternabschnitt von
  `## Manual Update`, und `## Manual Update` wurde nie bewertet. Das ist die
  eigentliche Wurzel von F2; jetzt wird die echte Vorfahrenkette gebildet.
* FUER WATCHTOWER ZAEHLT NUR DER VORBEHALT (F1). Ein `git pull` irgendwo im
  Abschnitt sagt dem Leser nicht, dass Watchtower die Compose-Datei stehen
  laesst. `docs/wiki/installation.md` war genau so gruen. Verlangt wird der
  Satz selbst.

Sein Nenner — was dieses Gate NICHT ansieht
-------------------------------------------
* Es liest MARKDOWN, nicht die Wirkung. Ob der beschriebene Befehl auch
  funktioniert, prueft `scripts/update_artifacts_test.sh` am Skript.
* Es sieht nur die Dateien unter DOC_ROOTS. Eine Update-Anleitung ausserhalb
  (Website, Wiki eines Kunden, Support-Ticket) erreicht es nicht.
* Ein WEG wird als EINE Einheit bewertet: steht der Schritt in einem
  Unterabschnitt und der Pull in einem anderen desselben Weges, gilt er als in
  Ordnung — eine nummerierte Anleitung darf ihre Schritte auf Unterabschnitte
  verteilen. Diese Grenze bleibt bestehen; sie ist als Fall im Selbsttest
  festgehalten.
* Es prueft die REIHENFOLGE nicht. Dass das Backup vor der Umstellung steht
  (REV-W1C F3), haelt `scripts/update_artifacts_test.sh` Fall 8 am Skript fest,
  nicht dieses Gate an der Doku.
* Rein historische Abschnitte (CHANGELOG, docs/history, docs/reviews,
  docs/sprints, docs/reports) sind ausgenommen, dazu docs/adr und docs/dev:
  sie beschreiben, was war oder was intern gilt, nicht, was der Kunde tun soll.
  Die Ausnahmeliste wird zur Laufzeit vollstaendig ausgegeben — sechs Eintraege,
  nicht die vier, die dieser Text frueher nannte (REV-W1C F8).

Aufruf
    python3 scripts/check_upgrade_docs.py            # prueft das Repo
    python3 scripts/check_upgrade_docs.py --selftest # prueft das Gate selbst
"""

import re
import sys
import tempfile
from pathlib import Path

DOC_ROOTS = ["docs", "README.md"]
SKIP_DIRS = {"history", "reviews", "sprints", "reports", "adr", "dev"}

PULL_PAT = re.compile(r"docker[- ]compose\s+(?:-f\s+\S+\s+)*pull\b")
# REV-W1C F1: bis 2026-08-02 stand hier nur `containrrr/watchtower`, also der
# IMAGE-Name. Vier Kundenstellen im gebauten Mirror sprechen ueber Watchtower,
# ohne das Image zu nennen — darunter `docs/setup.md:231`, das dem
# Bestandskunden woertlich sagt, es sei "nichts weiter zu tun". Das Gate war an
# allen vier blind; `docs/wiki/faq.md` schreibt sogar `containrrr.dev/watchtower`,
# ein Zeichen daneben. Geprueft wird jetzt das WORT.
WATCHTOWER_PAT = re.compile(r"watchtower", re.I)
# Und was einen Watchtower-Abschnitt heil macht, ist NICHT irgendein
# Artefakt-Schritt im selben Abschnitt, sondern der Vorbehalt selbst.
# `docs/wiki/installation.md` ist der Beleg: der Watchtower-Satz stand dort
# unqualifiziert, aber zwei Absaetze weiter unten erklaerte ein `git pull` die
# Migrationen — und das Gate war zufrieden. Ein Kunde, der nur den
# Watchtower-Satz liest, erfaehrt davon nichts. Verlangt wird deshalb die
# Aussage, die drei Stellen im Repo schon woertlich fuehren.
WATCHTOWER_CAVEAT = re.compile(
    r"aktualisiert\s+aussch(?:ließ|liess)lich\s+Images"
    r"|aktualisiert\s+nur\s+(?:die\s+)?Images"
    r"|only\s+updates\s+(?:the\s+)?images"
    r"|updates\s+images\s+only", re.I)
ARTIFACT_PAT = re.compile(r"git\s+pull|git\s+merge|update-artifacts|scripts/update\.sh")
# REV-W1C F2, zweite Haelfte: `--no-deps` war die zweite Haelfte von R1-06-D00
# („prevents accidentally restarting PostgreSQL or Redis" — genau die Dienste,
# deren Haertung in den Compose-Bloecken steckt). Das Gate prueft es jetzt, aber
# nur in einem AUSGEFUEHRTEN Block: in `docs/update-guide.md` steht dieselbe
# Zeichenkette im Fliesstext als ausdrueckliche, begruendete Ausnahme, und ein
# Gate, das eine Erklaerung roetet, erzieht zum Loeschen der Erklaerung.
NODEPS_PAT = re.compile(r"docker[- ]compose\s+(?:-f\s+\S+\s+)*up\b[^\n]*--no-deps")

# "neue Version" gehoert dazu: `docs/wiki/faq.md` fragt "Wie erfahre ich, wenn
# eine neue Version verfuegbar ist?" und antwortet mit Watchtower. Ohne dieses
# Wort war der Abschnitt fuer dieses Gate kein Update-Abschnitt (REV-W1C F1).
SECTION_PAT = re.compile(r"update|upgrade|aktualisier|neue\s+Version|new\s+version", re.I)
HEADING_PAT = re.compile(r"^(#{1,6})\s+(.*)$")


def markdown_files():
    files = []
    for root in DOC_ROOTS:
        p = Path(root)
        if p.is_file():
            files.append(p)
            continue
        for f in sorted(p.rglob("*.md")):
            if SKIP_DIRS & set(f.parts):
                continue
            files.append(f)
    return files


def fenced(lines):
    """-> Menge der Zeilenindizes, die INNERHALB eines Code-Blocks liegen."""
    inside, in_fence = set(), False
    for i, line in enumerate(lines):
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence:
            inside.add(i)
    return inside


def sections(lines):
    """Liefert (start, end, kette, eigene_ueberschrift, ebene), 0-basiert.

    `kette` ist die Liste (ebene, text) der Elternueberschriften EINSCHLIESSLICH
    der eigenen.

    Zeilen INNERHALB eines Code-Blocks sind keine Ueberschriften. Das ist nicht
    kosmetisch: `# 1. Neue Images ziehen` ist ein Shell-Kommentar in einer
    Anleitung, und wer ihn als Ueberschrift liest, beendet den Abschnitt genau
    VOR dem `docker compose pull`, das er pruefen wollte. Dieses Gate war in
    seiner ersten Fassung dadurch blind fuer docs/operations.md,
    docs/operations/upgrade.md und docs/wiki/upgrade.md — also fuer drei der
    Fundstellen, wegen derer es existiert.
    """
    heads = []  # (zeile, ebene, text)
    in_fence = False
    for i, line in enumerate(lines):
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        m = HEADING_PAT.match(line)
        if m:
            heads.append((i, len(m.group(1)), m.group(2)))
    out = []
    for idx, (start, level, _text) in enumerate(heads):
        end = len(lines)
        for nstart, nlevel, _ in heads[idx + 1 :]:
            if nlevel <= level:
                end = nstart
                break
        # Die ECHTE Vorfahrenkette, nicht "alle vorangehenden Ueberschriften bis
        # zu dieser Ebene". Der Unterschied ist die Wurzel von REV-W1C F2: in
        # `docs/update-guide.md` stehen `## Quick Update` und `## Manual Update`
        # nebeneinander. Die alte Lesart zaehlte `## Quick Update` zu den Eltern
        # von `## Manual Update` — ein Geschwister als Vorfahr. Damit galt
        # `## Manual Update` als Unterabschnitt einer Update-Anleitung und wurde
        # nie selbst bewertet; einzig die H1 `# Vakt Update Guide` blieb uebrig,
        # und die deckt die ganze Datei. Ein `./scripts/update.sh` im einen Weg
        # machte den anderen gruen.
        chain, cur = [], level
        for _hstart, hlevel, t in reversed(heads[:idx]):
            if hlevel < cur:
                chain.append((hlevel, t))
                cur = hlevel
        chain.reverse()
        chain.append((level, _text))
        out.append((start, end, chain, _text, level))
    if not heads:
        out.append((0, len(lines), [], "", 0))
    return out


def judged(sec, secs):
    """Wird der Pull-Teil in DIESEM Abschnitt bewertet? (REV-W1C F2)

    Bis 2026-08-02 galt: der aeusserste Abschnitt, dessen Ueberschrift ein
    Update-Wort traegt. Vier Dateien heissen aber schon in ihrer H1 so —
    `# Vakt Update Guide`, `# Upgrade`, … Damit war der aeusserste
    Update-Abschnitt die GANZE DATEI, und ein einziges `./scripts/update.sh`
    irgendwo darin machte jeden anderen Weg gruen. `docs/update-guide.md` liess
    sich deshalb vollstaendig zurueckdrehen, ohne dass etwas rot wurde —
    gemessen, und das ist genau die Datei, die `--no-deps` empfahl.

    Eine H1 ist der TITEL des Dokuments, kein Weg. Sie zaehlt deshalb nicht als
    Update-Abschnitt, solange es tiefere gibt; dann werden die einzeln
    bewertet. `## Quick Update` und `## Manual Update` sind zwei Wege, und jeder
    muss fuer sich vollstaendig sein.

    Hat die Datei nur die H1 und keinen tieferen Update-Abschnitt, wird sie
    weiterhin als Einheit bewertet — sonst faende dieses Gate dort gar nichts.
    """
    start, end, chain, own, level = sec
    if not SECTION_PAT.search(own):
        return False
    if level <= 1:
        return not any(
            s2 > start and e2 <= end and SECTION_PAT.search(o2)
            for (s2, e2, _c2, o2, _l2) in secs)
    # Eltern ab Ebene 2, die selbst Update-Abschnitte sind, machen diesen hier zu
    # einem Schritt INNERHALB einer Anleitung — dann urteilt der Elternabschnitt.
    return not any(SECTION_PAT.search(t) for (lv, t) in chain[:-1] if lv >= 2)


def check_file(path):
    """Liefert eine Liste von Befunden (Zeilennummer 1-basiert, Art).

    Zwei Reichweiten, mit Absicht verschieden:

    * `docker compose pull` wird im AEUSSERSTEN Update-Abschnitt bewertet (dem,
      dessen Elternueberschriften nicht selbst schon Update-Abschnitte sind).
      Eine nummerierte Anleitung ist eine Einheit: der Schritt "Dateien holen"
      steht dort in Schritt 1 und der Pull in Schritt 3, beide unter derselben
      Ueberschrift. Wuerde man je Unterabschnitt urteilen, waere jede korrekte
      Anleitung rot.
    * Watchtower wird im INNERSTEN Abschnitt bewertet, in dem es vorkommt.
      "Watchtower laeuft, es ist nichts weiter zu tun" ist genau die Aussage,
      die falsch ist — und wer nur diesen Unterabschnitt liest, muss es dort
      erfahren, nicht drei Ueberschriften weiter oben.
    """
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    secs = sections(lines)
    code = fenced(lines)
    findings = []

    for sec in secs:
        start, end, chain, own, _level = sec
        if not judged(sec, secs):
            continue
        body = "\n".join(lines[start:end])
        if PULL_PAT.search(body) and not ARTIFACT_PAT.search(body):
            findings.append((start + 1, "docker compose pull"))
        # `--no-deps` nur, wenn es wirklich ausgefuehrt werden soll.
        block = "\n".join(lines[i] for i in range(start, end) if i in code)
        if NODEPS_PAT.search(block):
            findings.append((start + 1, "up -d --no-deps"))

    for start, end, chain, _own, _level in secs:
        body = "\n".join(lines[start:end])
        if not WATCHTOWER_PAT.search(body):
            continue
        # Nur der INNERSTE Abschnitt zaehlt: traegt ein tieferer denselben Fund,
        # wird er dort gemeldet, nicht hier.
        if any(
            s2 > start and e2 <= end and WATCHTOWER_PAT.search("\n".join(lines[s2:e2]))
            for s2, e2, _c, _o, _l in secs
        ):
            continue
        if not any(SECTION_PAT.search(t) for (_lv, t) in chain):
            continue
        if not WATCHTOWER_CAVEAT.search(body):
            findings.append((start + 1, "Watchtower"))

    return sorted(set(findings))


def run(argv):
    files = markdown_files()
    findings = []
    for f in files:
        for line, kind in check_file(f):
            findings.append((f, line, kind))

    print(f"G-UPGRADE-DOCS: {len(files)} Markdown-Dateien geprueft.")
    print("  ausgenommen (historisch, keine Handlungsanweisung): " + ", ".join(sorted(SKIP_DIRS)))
    print("  nicht geprueft: ob der beschriebene Befehl WIRKT (das misst")
    print("                  scripts/update_artifacts_test.sh am Skript selbst)")
    if findings:
        print(f"\nFEHLER ({len(findings)}): Update-Weg, der die Auslieferungs-Dateien "
              f"stehen laesst.")
        for f, line, kind in findings:
            print(f"  {f}:{line} — Update-Abschnitt mit '{kind}'.")
            if kind == "Watchtower":
                print("      Es fehlt der Vorbehalt: 'Watchtower aktualisiert ausschliesslich")
                print("      Images'. Ein 'git pull' anderswo im Abschnitt genuegt hier NICHT —")
                print("      wer nur diesen Absatz liest, erfaehrt davon nichts (REV-W1C F1).")
            elif kind == "up -d --no-deps":
                print("      '--no-deps' in einem ausgefuehrten Block einer Update-Anleitung:")
                print("      caddy, postgres, redis und pgbouncer werden dann NICHT neu erzeugt,")
                print("      und genau in deren Compose-Bloecken steckt die Haertung. Im")
                print("      Fliesstext als begruendete Ausnahme ist es erlaubt.")
            else:
                print("      Aber ohne 'git pull' / 'scripts/update.sh' / 'update-artifacts'")
                print("      im ganzen Abschnitt.")
        print("\n  docker-compose.yml, Caddyfile und scripts/ gehoeren zur Installation,")
        print("  nicht zu den Images. Siehe docs/UPGRADE.md.")
        return 1
    print("\nOK — jeder dokumentierte Update-Weg nimmt die Auslieferungs-Dateien mit.")
    return 0


def selftest():
    """Jede Defektform wird rot, jede gesunde Form bleibt gruen."""
    cases = [
        ("gut: Update-Abschnitt mit git pull",
         "## Update\n\n```bash\ngit pull --ff-only\ndocker compose pull\n```\n", 0),
        ("gut: Hinweis weit weg, aber im selben Abschnitt",
         "## Upgrade\n\n```bash\ngit pull\n```\n" + "\nFuell\n" * 20 +
         "```bash\ndocker compose pull\n```\n", 0),
        ("gut: Verweis auf scripts/update.sh statt git",
         "## Aktualisieren\n\nNimm `./scripts/update.sh`.\n\n```bash\ndocker compose pull\n```\n", 0),
        ("schlecht: Shell-Kommentar im Block ist KEINE Ueberschrift",
         "## Upgrade\n\n```bash\n# 1. Neue Images ziehen\ndocker compose pull\n# 2. Neu starten\ndocker compose up -d\n```\n", 1),
        ("gut: pull ausserhalb eines Update-Abschnitts (Wiederherstellung)",
         "## Disaster Recovery\n\n```bash\ndocker compose pull\ndocker compose up -d\n```\n", 0),
        ("schlecht: Update-Abschnitt zieht nur Images",
         "## Update\n\n```bash\ndocker compose pull\ndocker compose up -d\n```\n", 1),
        ("schlecht: Watchtower als einziger Update-Weg",
         "## Updates\n\n```bash\ndocker run -d containrrr/watchtower\n```\n", 1),
        # ── REV-W1C F1: das Wort, nicht der Image-Name ──────────────────────
        ("schlecht: Watchtower ohne Image-Namen (die vier Kundenstellen)",
         "## Updates\n\nWatchtower holt naechtlich neue Images. Es ist nichts "
         "weiter zu tun.\n", 1),
        ("schlecht: `git pull` im Abschnitt deckt den Watchtower-Satz NICHT",
         "## Updates\n\n**Option 2 — Watchtower:** automatische Updates.\n\n"
         "Migrationen laufen ueber den migrate-Container:\n\n```bash\ngit pull\n"
         "docker compose up -d\n```\n", 1),
        ("gut: Watchtower mit dem Vorbehalt",
         "## Updates\n\nWatchtower holt naechtlich neue Images.\n\n"
         "> **Watchtower aktualisiert ausschliesslich Images.** docker-compose.yml "
         "bleibt stehen; zusaetzlich `./scripts/update.sh`.\n", 0),
        ("gut: Watchtower ausserhalb jedes Update-Abschnitts (Glossar)",
         "## Begriffe\n\n**Watchtower** — Werkzeug, das Container automatisch "
         "auf neue Image-Tags hebt.\n", 0),
        # ── REV-W1C F2: die H1 ist der Titel, nicht der Weg ─────────────────
        ("schlecht: zweiter Weg unter derselben Update-H1 ohne Schritt",
         "# Vakt Update Guide\n\n## Quick Update\n\n```bash\n./scripts/update.sh\n"
         "```\n\n## Manual Update\n\n### 1. Pull new images\n\n```bash\n"
         "docker compose pull\n```\n", 1),
        ("gut: derselbe zweite Weg MIT Schritt",
         "# Vakt Update Guide\n\n## Quick Update\n\n```bash\n./scripts/update.sh\n"
         "```\n\n## Manual Update\n\n### 1. Update the deployment files\n\n"
         "```bash\ngit pull --ff-only\n```\n\n### 2. Pull new images\n\n```bash\n"
         "docker compose pull\n```\n", 0),
        ("gut: nur eine Update-H1, keine tieferen — weiter als Einheit bewertet",
         "# Upgrade\n\nErst holen, dann ziehen:\n\n```bash\ngit pull\n"
         "docker compose pull\n```\n", 0),
        # ── REV-W1C F2, zweite Haelfte: --no-deps ───────────────────────────
        ("schlecht: --no-deps im ausgefuehrten Block einer Update-Anleitung",
         "## Update\n\n```bash\ngit pull\ndocker compose pull\n"
         "docker compose up -d --no-deps api worker\n```\n", 1),
        ("gut: --no-deps im Fliesstext als begruendete Ausnahme",
         "## Update\n\n```bash\ngit pull\ndocker compose pull\n"
         "docker compose up -d\n```\n\nOhne Compose-Aenderungen ist "
         "`docker compose up -d --no-deps worker api` der engere Befehl.\n", 0),
        ("gut: --no-deps ausserhalb eines Update-Abschnitts",
         "## LDAP einrichten\n\n```bash\ndocker compose up -d --no-deps api worker\n"
         "```\n", 0),
        ("schlecht: Unterabschnitt erbt die Update-Ueberschrift",
         "## Update\n\n### Automatisch\n\n```bash\ndocker compose pull\n```\n", 1),
        # Bewusste Grenze, siehe Nenner: die Anleitung wird als EINE Einheit
        # bewertet. Steht der Schritt im Nachbar-Unterabschnitt derselben
        # Anleitung, laesst das Gate es durchgehen.
        ("Grenze: Schritt im Nachbar-Unterabschnitt derselben Anleitung",
         "## Update\n\n### Manuell\n\n```bash\ngit pull\n```\n\n### Automatisch\n\n```bash\ndocker compose pull\n```\n", 0),
    ]
    failed = 0
    with tempfile.TemporaryDirectory() as td:
        for name, body, want in cases:
            p = Path(td) / "t.md"
            p.write_text(body, encoding="utf-8")
            got = 1 if check_file(p) else 0
            mark = "ok " if got == want else "ROT"
            if got != want:
                failed += 1
            print(f"  [{mark}] {name}: erwartet {want}, gemessen {got}")
    print(f"\nSelbsttest: {len(cases)} Faelle, {failed} abweichend.")
    return 1 if failed else 0


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        sys.exit(selftest())
    sys.exit(run(sys.argv))
