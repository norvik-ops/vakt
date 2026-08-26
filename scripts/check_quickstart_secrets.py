#!/usr/bin/env python3
"""G-QUICKSTART-SECRETS — wer der Anleitung folgt, faehrt nicht mit abgedruckten Passwoertern.

Warum es dieses Gate gibt (2026-08-02, R1-05-C04)
-------------------------------------------------
`.env.example` liefert POSTGRES_PASSWORD, REDIS_PASSWORD und VAKT_SECRET_KEY auf
dem Literal `ERSETZEN_SIE_DIESEN_WERT`. Diese Datei liegt im OEFFENTLICHEN Repo —
das Passwort ist also nicht "noch nicht gesetzt", es ist "allgemein bekannt".

Der README-Schnellstart ersetzte davon genau einen Wert (VAKT_SECRET_KEY) und
schickte den Leser dann in `docker compose up -d`. Gemessen am gebauten Mirror:
nach dem README-Ablauf standen POSTGRES_PASSWORD und REDIS_PASSWORD noch auf dem
Literal; psql-Login damit erfolgreich, `redis-cli AUTH` gab `+OK`. docs/setup.md
machte es richtig — die drei Zwillinge in docs/guides/getting-started.md,
docs/wiki/installation.md und docs/wiki/msp-onboarding.md nicht.

Ein Gate, das nur den README liest, haette die Zwillinge nicht gefunden. Dieses
hier faehrt JEDE Schnellstart-Anleitung im gebauten Mirror nach und schaut, was
danach in der `.env` steht.

Was es prueft
-------------
1. Es baut den Mirror nach einem Tempdir (wie die Schwesterngates) und liest von
   dort — nicht aus der Quelle. Der Kunde hat den Mirror, nicht dieses Repo.
2. Jedes gespiegelte *.md, das `cp .env.example .env` enthaelt, ist eine
   Schnellstart-Anleitung.
3. Aus ALLEN ```-Bloecken dieser Seite FUEHRT es die `cp`/`sed`/`echo`-Zeilen in
   Dokumentreihenfolge in einem Tempdir aus, mit der `.env.example` DES MIRRORS
   als Vorlage. Ueber alle Bloecke, weil eine Seite Einstieg und
   Schluesselerzeugung trennen darf und ein Leser sie von oben abarbeitet.
4. Danach: keine der drei Variablen aus SECRETS darf noch einen Platzhalter aus
   PLACEHOLDERS tragen.

Sein Nenner — was dieses Gate NICHT ansieht
-------------------------------------------
* NUR cp/sed/echo. Jede andere Zeile im Block (docker, git, curl, …) wird
  UEBERSPRUNGEN und gezaehlt. Ein `docker compose up` in einem Gate auszufuehren
  waere Minuten und Nebenwirkung.
* SEITEN, DIE AN `scripts/install.sh` DELEGIEREN, werden nicht nachgefahren
  (install.sh startet Container). Sie werden gezaehlt und benannt, und getrennt
  davon wird geprueft, dass install.sh fuer jede der drei Variablen einen
  Ersetzungszweig hat — das ist eine TEXTUELLE Pruefung des Skripts, kein Lauf.
* SEITEN MIT MANUELLEM EDITOR-SCHRITT (`nano .env`) werden nicht nachgefahren —
  ein Editor laesst sich nicht nachfahren. Stattdessen wird verlangt, dass die
  Seite alle drei Variablen NAMENTLICH nennt; sonst weiss der Leser nicht, was er
  eintragen soll, und der Platzhalter bleibt stehen.
* NUR DIE DREI VARIABLEN AUS `SECRETS`. `VAKT_DB_URL` und `VAKT_REDIS_URL` tragen
  den Platzhalter in `.env.example` ebenfalls, werden hier aber NICHT geprueft:
  docker-compose.yml setzt `VAKT_DB_URL: ""` fuer api/worker und baut
  `VAKT_REDIS_URL` selbst aus `REDIS_PASSWORD` (`redis://:${REDIS_PASSWORD:?}@redis:6379`).
  Auf dem dokumentierten Compose-Weg sind sie damit wirkungslos. Wer eine EXTERNE
  DB faehrt, setzt `VAKT_DB_URL` ohnehin von Hand — dieser Pfad ist hier nicht
  abgedeckt und steht darum in dieser Liste.
* KEIN LAUFENDER STACK. Dass der Wert wirkt, prueft dieses Gate nicht; es prueft,
  was die Anleitung in die Datei schreibt.

Selbsttest: `python3 scripts/check_quickstart_secrets.py --selftest`
Exit 0 = OK oder nicht pruefbar (Werkzeug fehlt) · Exit 1 = Fehler.
"""

import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent

SECRETS = ["VAKT_SECRET_KEY", "POSTGRES_PASSWORD", "REDIS_PASSWORD"]
PLACEHOLDERS = ["ERSETZEN_SIE_DIESEN_WERT", "changeme", "MUST_BE_OVERRIDDEN"]
ENTRY = "cp .env.example .env"
REPLAYABLE = ("cp ", "sed ", "echo ")
INSTALL_SH = "scripts/install.sh"
# Ein Editor-Aufruf auf die .env ist ein MANUELLER Schritt. Er ist nur dann in
# Ordnung, wenn die Seite die zu setzenden Variablen auch NENNT — sonst weiss der
# Leser nicht, was er eintragen soll, und genau daraus wurde in
# docs/wiki/msp-onboarding.md ein `# vim .env  ← Pflichtfelder ausfuellen`, nach
# dem alle drei Platzhalter stehen blieben.
EDITOR = re.compile(r"^(nano|vim|vi|emacs|\$EDITOR|\$\{EDITOR\})\s+\S*\.env\b")


def all_blocks(text):
    """-> Inhalte ALLER ```-Bloecke, in Dokumentreihenfolge.

    Bewusst das ganze Dokument statt nur des Blocks mit dem Einstieg: eine Seite
    darf `cp .env.example .env` und die Schluesselerzeugung in ZWEI Bloecke
    trennen (docs/wiki/installation.md macht das), und ein Leser arbeitet die
    Seite von oben nach unten ab. Ein Gate, das nur den Einstiegsblock liest,
    haette diese Seite faelschlich rot gemeldet.
    """
    out, buf, inside = [], [], False
    for line in text.splitlines():
        if line.lstrip().startswith("```"):
            if inside and buf:
                out.append("\n".join(buf))
            buf, inside = [], not inside
            continue
        if inside:
            buf.append(line)
    return out


def split_doc(text):
    """-> (replay, skipped, ignored, delegate) fuer EIN Dokument.

    `replay`   cp/sed/echo auf die .env — wird ausgefuehrt.
    `skipped`  beruehrt die .env, ist aber nicht nachfahrbar — einzeln benannt.
    `ignored`  hat mit der .env nichts zu tun (git clone, docker compose …) — gezaehlt.
    `delegate` None, "install.sh" oder "manuell".

    Die Delegation ist KEIN Freibrief: nachgefahren wird trotzdem alles
    Nachfahrbare. Sie greift erst fuer die Variablen, die danach noch auf dem
    Platzhalter stehen. Sonst haette ein spaeteres `nano .env` irgendwo auf der
    Seite die ganze Pruefung abgeschaltet — docs/setup.md hat genau so eine Zeile
    in einem ANDEREN Abschnitt (Passwortwechsel), und seine korrekte
    Schnellstart-Sequenz waere nie mehr gemessen worden.
    """
    replay, skipped, ignored, delegate = [], [], [], None
    for block in all_blocks(text):
        for raw in block.splitlines():
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if INSTALL_SH in line or line.endswith("install.sh"):
                delegate = delegate or "install.sh"
                skipped.append(line)
                continue
            if EDITOR.match(line):
                delegate = delegate or "manuell"
                skipped.append(line)
                continue
            if line.startswith(REPLAYABLE) and ".env" in line:
                replay.append(line)
            elif ".env" in line:
                skipped.append(line)
            else:
                ignored.append(line)
    return replay, skipped, ignored, delegate


def replay(env_example, lines, workdir):
    """Fuehrt die Zeilen im Tempdir aus. -> (env_text|None, fehler|None)"""
    wd = pathlib.Path(workdir)
    (wd / ".env.example").write_text(env_example, encoding="utf-8")
    script = "set -e\n" + "\n".join(lines) + "\n"
    p = subprocess.run(["bash", "-c", script], cwd=str(wd),
                       capture_output=True, text=True, timeout=120)
    if p.returncode != 0:
        return None, (p.stderr.strip() or f"rc={p.returncode}")
    env = wd / ".env"
    if not env.is_file():
        return None, "der Block legt keine .env an"
    return env.read_text(encoding="utf-8"), None


def leftover_placeholders(env_text):
    """-> [(var, wert)] fuer jede SECRETS-Variable, die noch einen Platzhalter traegt."""
    bad = []
    for var in SECRETS:
        m = re.search(rf"^{re.escape(var)}=(.*)$", env_text, re.MULTILINE)
        if not m:
            bad.append((var, "<Zeile fehlt in der .env>"))
            continue
        val = m.group(1).strip()
        if any(ph in val for ph in PLACEHOLDERS) or val == "":
            bad.append((var, val or "<leer>"))
    return bad


def install_sh_covers(text):
    """-> Menge der SECRETS, fuer die install.sh einen Ersetzungszweig hat."""
    return {v for v in SECRETS if re.search(rf"^\s*sed .*{re.escape(v)}=", text,
                                            re.MULTILINE)}


def evaluate(docs, env_example, install_text, workdir_factory):
    """Reine Pruefung — vom Selbsttest mit synthetischen Eingaben gefahren."""
    problems = []
    stats = {"docs": 0, "replayed": 0, "skipped": [], "ignored": 0,
             "install": [], "manual": []}
    covered = install_sh_covers(install_text)

    for f, text in sorted(docs.items()):
        if ENTRY not in text:
            continue
        stats["docs"] += 1
        lines, skipped, ignored, delegate = split_doc(text)
        stats["skipped"] += [(f, s) for s in skipped]
        stats["ignored"] += len(ignored)

        with workdir_factory() as wd:
            env_text, err = replay(env_example, lines, wd)
        if err:
            problems.append(f"{f}: laesst sich nicht nachfahren: {err}")
            continue
        stats["replayed"] += 1
        left = leftover_placeholders(env_text)
        if not left:
            continue

        # Erst jetzt greift eine Delegation — und nur fuer die Variablen, die
        # tatsaechlich noch offen sind.
        if delegate == "install.sh":
            stats["install"].append(f)
            for var, _ in left:
                if var not in covered:
                    problems.append(
                        f"{f}: delegiert an {INSTALL_SH}, und dort gibt es keinen "
                        f"Ersetzungszweig fuer {var}. Der Platzhalter bleibt stehen.")
            continue
        if delegate == "manuell":
            stats["manual"].append(f)
            missing = [v for v, _ in left if v not in text]
            if missing:
                problems.append(
                    f"{f}: schickt den Leser zum Editor (`nano .env` o.ae.), nennt "
                    f"aber {', '.join(missing)} nirgends auf der Seite. Der "
                    f"Platzhalter aus .env.example bleibt damit stehen — und der "
                    f"ist im oeffentlichen Repo abgedruckt.")
            continue

        for var, val in left:
            problems.append(
                f"{f}: nach dieser Anleitung steht {var}={val} — der Platzhalter "
                f"ist im oeffentlichen Repo abgedruckt. Wer der Anleitung folgt, "
                f"faehrt mit einem allgemein bekannten Wert.")

    return problems, stats


# ─────────────────────────── Mirror bauen + einlesen ────────────────────────────

def build_mirror(out):
    script = ROOT / "scripts" / "build-public-mirror.sh"
    if not script.is_file():
        return None, ("scripts/build-public-mirror.sh fehlt — dieses Repo ist "
                      "vermutlich selbst der Mirror. Nichts zu bauen.")
    for tool in ("bash", "rsync", "sed", "openssl"):
        if shutil.which(tool) is None:
            return None, f"{tool} fehlt im PATH — nicht nachfahrbar."
    env = dict(os.environ, VAKT_MIRROR_OUT=str(out), VAKT_MIRROR_SKIP_GO_BUILD="1")
    p = subprocess.run(["bash", str(script)], cwd=str(ROOT), env=env,
                       capture_output=True, text=True, timeout=900)
    return p, None


# ────────────────────────────────── Selbsttest ──────────────────────────────────

ENV_FIXTURE = (
    "VAKT_DB_URL=postgres://vakt:ERSETZEN_SIE_DIESEN_WERT@localhost:5432/vakt\n"
    "REDIS_PASSWORD=ERSETZEN_SIE_DIESEN_WERT\n"
    "VAKT_SECRET_KEY=ERSETZEN_SIE_DIESEN_WERT\n"
    "POSTGRES_PASSWORD=ERSETZEN_SIE_DIESEN_WERT\n"
)
INSTALL_FIXTURE = (
    'sed -i "s|^VAKT_SECRET_KEY=.*|VAKT_SECRET_KEY=${SECRET}|" .env\n'
    'sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$P/" .env\n'
    'sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$R/" .env\n'
)

GOOD = """```bash
git clone https://github.com/norvik-ops/vakt
cp .env.example .env
sed -i "s/^VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY=$(openssl rand -hex 32)/" .env
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 24)/" .env
sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$(openssl rand -hex 24)/" .env
docker compose up -d
```
"""


def selftest():
    """Die drei Abnahmen als ausgefuehrte Faelle, plus die Zaehl-Zusagen."""
    wf = lambda: tempfile.TemporaryDirectory(prefix="vakt-qs-")  # noqa: E731
    cases = []

    # 1 — gruen auf einer korrekten Anleitung.
    pr, _ = evaluate({"docs/setup.md": GOOD}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("1 vollstaendige Anleitung gruen", not pr, pr))

    # 2 — ECHTE Regression R1-05-C04: der README-Stand vor dem Fix, nur der
    # Secret-Key wird ersetzt. ROT, mit BEIDEN Variablen benannt.
    bad = GOOD.replace(
        'sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 24)/" .env\n', ""
    ).replace(
        'sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$(openssl rand -hex 24)/" .env\n', "")
    pr, _ = evaluate({"README.md": bad}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("2 nur VAKT_SECRET_KEY ersetzt -> rot, beide Variablen benannt",
                  len(pr) == 2 and any("POSTGRES_PASSWORD" in p for p in pr)
                  and any("REDIS_PASSWORD" in p for p in pr), pr))

    # 3 — der Zwilling: gar keine Ersetzung, nur `cp` und Start.
    none = "```bash\ncp .env.example .env\ndocker compose up -d\n```\n"
    pr, _ = evaluate({"docs/wiki/msp-onboarding.md": none}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("3 keine Ersetzung -> rot fuer alle drei", len(pr) == 3, pr))

    # 4 — Gegenprobe: die alte README-Form mit einfachen Anfuehrungszeichen und
    # ohne `^`-Anker ersetzt trotzdem korrekt und darf nicht rot faerben. Sonst
    # wuerde das Gate eine Schreibweise vorschreiben statt ein Ergebnis.
    old_style = """```bash
cp .env.example .env
sed -i 's/VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY='"$(openssl rand -hex 32)"'/' .env
sed -i 's/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=ERSETZT-DURCH-DEN-TEST/' .env
sed -i 's/^REDIS_PASSWORD=.*/REDIS_PASSWORD=EBENFALLS-ERSETZT/' .env
```
"""
    pr, _ = evaluate({"docs/x.md": old_style}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("4 andere sed-Schreibweise, gleiches Ergebnis -> nicht rot", not pr, pr))

    # 5 — Delegation an install.sh: nicht nachgefahren, aber gezaehlt …
    deleg = "```bash\ncp .env.example .env\n./scripts/install.sh\n```\n"
    pr, st = evaluate({"docs/wiki/demo-mode.md": deleg}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("5 Delegation an install.sh -> nicht rot, gezaehlt",
                  not pr and st["install"] == ["docs/wiki/demo-mode.md"],
                  (pr, st["install"])))

    # 6 — … und wenn install.sh eine der drei NICHT ersetzt, ist genau das rot.
    # Ohne diesen Fall waere die Delegation ein Schlupfloch.
    thin = INSTALL_FIXTURE.replace(
        'sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$R/" .env\n', "")
    pr, _ = evaluate({"docs/wiki/demo-mode.md": deleg}, ENV_FIXTURE, thin, wf)
    cases.append(("6 install.sh ohne REDIS_PASSWORD-Zweig -> rot + benannt",
                  len(pr) == 1 and "REDIS_PASSWORD" in pr[0], pr))

    # 7 — Manueller Schritt MIT Ansage: die Seite sagt, welche Variablen zu setzen
    # sind (docs/operations/restore-from-offsite.md macht das). Nicht rot, gezaehlt.
    manual_ok = ("```bash\ncp .env.example .env\nnano .env\n"
                 "# Setze: VAKT_SECRET_KEY, POSTGRES_PASSWORD, REDIS_PASSWORD\n```\n")
    pr, st = evaluate({"docs/operations/restore.md": manual_ok},
                      ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("7 `nano .env` MIT Variablen-Ansage -> nicht rot, gezaehlt",
                  not pr and st["manual"] == ["docs/operations/restore.md"],
                  (pr, st["manual"])))

    # 8 — Manueller Schritt OHNE Ansage: genau die Form, die in
    # docs/wiki/msp-onboarding.md stand (`# vim .env ← Pflichtfelder ausfuellen`).
    # Ohne diesen Fall waere `nano .env` der Freibrief, der alles gruen macht.
    manual_bad = "```bash\ncp .env.example .env\nvim .env\ndocker compose up -d\n```\n"
    pr, _ = evaluate({"docs/wiki/msp-onboarding.md": manual_bad},
                     ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("8 `vim .env` OHNE Variablen-Ansage -> rot + benannt",
                  len(pr) == 1 and all(v in pr[0] for v in SECRETS), pr))

    # 9 — Zaehl-Zusage: Zeilen ohne .env-Bezug (git clone, docker compose up)
    # werden gezaehlt, nicht verschluckt. GOOD hat zwei davon.
    _, st = evaluate({"docs/setup.md": GOOD}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("9 Zeilen ohne .env-Bezug gezaehlt", st["ignored"] == 2, st["ignored"]))

    # 10 — Gegenprobe: ein Markdown OHNE Schnellstart-Einstieg wird gar nicht
    # angefasst (sonst waere jede Doku ein Kandidat).
    _, st = evaluate({"docs/architecture.md": "```bash\nmake dev\n```\n"},
                     ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("10 Doku ohne `cp .env.example .env` wird nicht geprueft",
                  st["docs"] == 0, st))

    # 11 — Zwei Bloecke, eine Seite: Einstieg oben, Schluesselerzeugung unten
    # (docs/wiki/installation.md). Muss gruen sein — sonst misst das Gate die
    # Gliederung der Seite statt ihres Ergebnisses.
    split = ("```bash\ncp .env.example .env\n```\nDann die Geheimnisse setzen:\n"
             "```bash\n"
             'sed -i "s/^VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY=$(openssl rand -hex 32)/" .env\n'
             'sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 24)/" .env\n'
             'sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$(openssl rand -hex 24)/" .env\n'
             "```\n")
    pr, _ = evaluate({"docs/wiki/installation.md": split}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("11 Anleitung ueber zwei Bloecke -> gruen", not pr, pr))

    # 12 — Zaehl-Zusage: eine .env-Zeile, die sich nicht nachfahren laesst, wird
    # EINZELN benannt (nicht in den Sammelzaehler geworfen).
    _, st = evaluate({"docs/x.md": GOOD.replace("docker compose up -d\n",
                                                "grep VAKT_SECRET_KEY .env\n")},
                     ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("12 nicht nachfahrbare .env-Zeile einzeln benannt",
                  len(st["skipped"]) == 1 and ".env" in st["skipped"][0][1],
                  st["skipped"]))

    # 13 — DIE LUECKE, die der Umbau auf "ganzes Dokument" aufgerissen haette:
    # docs/setup.md hat in einem SPAETEREN Abschnitt (Passwortwechsel) ein
    # `nano .env`. Wuerde das die Seite zur Delegation machen, waere ihre korrekte
    # Schnellstart-Sequenz nie mehr gemessen — und ein Rueckbau dort bliebe gruen.
    late_editor = GOOD + "\nSpaeter, beim Passwortwechsel:\n```bash\nnano .env\n```\n"
    broken = late_editor.replace(
        'sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 24)/" .env\n', "")
    pr, _ = evaluate({"docs/setup.md": broken}, ENV_FIXTURE, INSTALL_FIXTURE, wf)
    cases.append(("13 spaeteres `nano .env` schaltet die Pruefung NICHT ab",
                  len(pr) == 1 and "POSTGRES_PASSWORD" in pr[0], pr))

    bad_cases = [c for c in cases if not c[1]]
    for name, ok, detail in cases:
        print(f"  {'ok  ' if ok else 'FAIL'}  {name}" + ("" if ok else f"   -> {detail}"))
    if bad_cases:
        print(f"\nSELBSTTEST FEHLGESCHLAGEN — {len(bad_cases)}/{len(cases)}")
        return 1
    print(f"\nSelbsttest: alle {len(cases)} Faelle wie erwartet "
          f"(6 gruen als Gegenprobe, 5 rot mit Namensnennung, 3 Zaehl-Zusagen)")
    return 0


# ──────────────────────────────────── main ──────────────────────────────────────

NOT_CHECKED = [
    "nur cp/sed/echo werden nachgefahren — docker/git/curl-Zeilen uebersprungen",
    "Seiten, die an scripts/install.sh delegieren — dort textuell geprueft, nicht gelaufen",
    "Seiten mit manuellem Editor-Schritt — statt Nachfahren wird die Variablen-Ansage verlangt",
    "VAKT_DB_URL/VAKT_REDIS_URL — auf dem Compose-Weg von docker-compose.yml ueberschrieben "
    "bzw. aus REDIS_PASSWORD abgeleitet",
    "kein laufender Stack — geprueft wird der Inhalt der .env, nicht seine Wirkung",
]


def main():
    if "--selftest" in sys.argv:
        print("G-QUICKSTART-SECRETS — Selbsttest")
        return selftest()

    print("G-QUICKSTART-SECRETS — keine abgedruckten Passwoerter nach der Anleitung")

    with tempfile.TemporaryDirectory(prefix="vakt-qs-gate-") as tmp:
        out = pathlib.Path(tmp) / "mirror"
        proc, unmeasurable = build_mirror(out)
        if unmeasurable:
            print(f"  nicht pruefbar : {unmeasurable}")
            print("NICHT GEMESSEN — kein Urteil ueber die Schnellstarts (exit 0).")
            return 0
        if proc.returncode != 0:
            print("\nFEHLER — der Mirror-Build selbst bricht ab:")
            for ln in (proc.stdout + proc.stderr).splitlines()[-25:]:
                print("  " + ln)
            return 1

        env_example = (out / ".env.example").read_text(encoding="utf-8")
        install = out / INSTALL_SH
        install_text = install.read_text(encoding="utf-8") if install.is_file() else ""
        docs = {str(p.relative_to(out)): p.read_text(encoding="utf-8", errors="replace")
                for p in sorted(out.rglob("*.md"))
                if str(p.relative_to(out)) != "CHANGELOG.md"}

        problems, st = evaluate(
            docs, env_example, install_text,
            lambda: tempfile.TemporaryDirectory(prefix="vakt-qs-run-"))

    print(f"  anleitungen    : {st['docs']} Datei(en) mit `{ENTRY}`, "
          f"{st['replayed']} nachgefahren")
    print(f"  variablen      : {', '.join(SECRETS)}")
    print(f"  platzhalter    : {', '.join(PLACEHOLDERS)}")
    print(f"  delegiert      : {len(st['install'])} an {INSTALL_SH} "
          f"({', '.join(st['install']) or '—'}), "
          f"{len(st['manual'])} an einen manuellen Editor-Schritt "
          f"({', '.join(st['manual']) or '—'})")
    print(f"  skipped        : {len(st['skipped'])} .env-Zeile(n) nicht nachfahrbar, "
          f"{st['ignored']} Zeile(n) ohne .env-Bezug uebergangen")
    for f, item in st["skipped"]:
        print(f"                   · {f}: {item}")
    print("  nicht geprueft :")
    for n in NOT_CHECKED:
        print(f"                   · {n}")

    if problems:
        print(f"\nFEHLER ({len(problems)}):\n")
        for p in problems:
            print(f"  · {p}")
        return 1

    print("\nOK — jede Schnellstart-Anleitung im Mirror ersetzt alle drei Geheimnisse.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
