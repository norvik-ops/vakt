#!/usr/bin/env python3
"""G-BILLING-BINARY — wer einen /billing-Container ausliefert, muss ihn auch bauen.

Warum es dieses Gate gibt (2026-08-02, REV-W1B N3)
--------------------------------------------------
R1-05-C01 hat den Kundenbau repariert: `backend/Dockerfile` baute `cmd/billing`
unbedingt, im Public Mirror gibt es das Verzeichnis nicht, und
`docker compose build api` starb mit `stat /app/cmd/billing: directory not
found`. Der Fix hat den Bau bewacht — vorhanden wird gebaut, fehlend wird
gemeldet.

Damit war auf der PRIVATEN Seite eine Bau-Zeit-Pruefung weg. Vorher war
"cmd/billing fehlt" dort ein roter Build. Danach ist es eine Zeile im Log:
verschwindet das Verzeichnis durch eine Umbenennung oder ein Verschieben, baut
das Dockerfile still durch (rc=0), und `infra/server/docker-compose.yml` startet
den Dienst `vakt-billing` aus einem Image, in dem `/billing` nicht liegt. Der
Container ist der EINZIGE Halter von VAKT_LICENSE_PRIVATE_KEY; faellt er aus,
verkauft niemand mehr eine Lizenz, und das erste Anzeichen ist eine Mail.

`scripts/check_mirror_refs.py` faengt das nicht — dort zaehlt der bewachte Bau in
Fall 12 ausdruecklich als "optional, nicht rot". Das ist fuer den Mirror richtig
und laesst die private Seite offen. Diese Luecke schliesst dieses Gate.

Was es prueft
-------------
1. LIEFERT DIESER BAUM EINEN /billing-CONTAINER AUS? Gemessen an
   `infra/server/docker-compose.yml`: ein Dienst mit `entrypoint: ["/billing"]`.
   Das ist die Frage, an der sich privat und Mirror unterscheiden — `infra/`
   wird von build-public-mirror.sh gar nicht gespiegelt, und `cmd/billing` steht
   nicht in seiner cmd-Liste (nur api, migrate, worker, healthcheck).
2. WENN JA, dann muss `backend/cmd/billing/` existieren und Go-Quelltext
   enthalten. Genau die Zusicherung, die der bewachte Bau verloren hat.
3. WENN JA, dann muss `backend/Dockerfile` das Binary auch bauen
   (`-o /out/billing ./cmd/billing`). Ein Container ohne Bau ist derselbe
   Ausfall, nur eine Datei weiter.
4. IMMER: der Dockerfile muss den strengen Modus ueberhaupt kennen
   (`VAKT_BILLING` mit einem Zweig, der `exit 1` faehrt). Ohne ihn gibt es
   keinen Bau, den man streng stellen KANN, und Punkt 2 waere die einzige
   Absicherung.
5. GEGENRICHTUNG: `cmd/billing` da, aber kein Bau im Dockerfile -> rot. Und
   umgekehrt kein /billing-Container und kein cmd/billing -> gruen, mit
   ausdruecklicher Feststellung "das ist der Mirror-Fall", nicht stillschweigend.

Sein Nenner — was dieses Gate NICHT ansieht
-------------------------------------------
* ES BAUT NICHTS. Es liest Dateien. Dass ein fertiges Image `/billing`
  tatsaechlich enthaelt, kann nur ein Bau zeigen; dafuer ist der harte Zweig
  `VAKT_BILLING=required` im Dockerfile da. Dieses Gate stellt sicher, dass es
  diesen Zweig gibt und dass der Baum zu ihm passt — es faehrt ihn nicht.
* ES PRUEFT NICHT, OB DER RELEASE-BAU DEN SCHALTER SETZT.
  `.github/workflows/release.yml:214-220` baut das Backend-Image ohne
  `--build-arg VAKT_BILLING=required`. Diese Zeile liegt ausserhalb des
  Dateieigentums der Spur, die dieses Gate anlegt; sie ist im GATE INVENTORY von
  scripts/check_ci_steps.py namentlich benannt statt hier still vorausgesetzt.
* ES PRUEFT NICHT DEN INHALT von cmd/billing. Ein leeres `package main` genuegt
  ihm. Dass der Dienst das Richtige tut, ist Sache der Go-Tests.

Selbsttest: `python3 scripts/check_billing_binary.py --selftest`
Exit 0 = OK · Exit 1 = Fehler.
"""

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
COMPOSE = ROOT / "infra" / "server" / "docker-compose.yml"
DOCKERFILE = ROOT / "backend" / "Dockerfile"
CMD_BILLING = ROOT / "backend" / "cmd" / "billing"

# `entrypoint: ["/billing"]` — auch `entrypoint: ['/billing']` und ohne Klammern.
ENTRYPOINT_BILLING = re.compile(r'entrypoint:\s*\[?\s*["\']?/billing["\']?\s*\]?\s*$',
                                re.MULTILINE)
# `-o /out/billing ./cmd/billing`
DOCKERFILE_BUILDS = re.compile(r'-o\s+/out/billing\s+\./cmd/billing')
# Der strenge Modus: ein VAKT_BILLING-Zweig, der wirklich abbricht.
STRICT_ARG = re.compile(r'ARG\s+VAKT_BILLING')
STRICT_EXIT = re.compile(r'VAKT_BILLING.*?exit\s+1', re.DOTALL)


def evaluate(compose_text, dockerfile_text, billing_dir_go_files):
    """-> (problems, facts). Reine Funktion, damit der Selbsttest sie fuettern kann.

    `billing_dir_go_files` = Anzahl .go-Dateien unter cmd/billing (0 = fehlt).
    """
    problems = []
    ships_container = bool(ENTRYPOINT_BILLING.search(compose_text))
    has_source = billing_dir_go_files > 0
    builds = bool(DOCKERFILE_BUILDS.search(dockerfile_text))

    if ships_container and not has_source:
        problems.append(
            "infra/server/docker-compose.yml startet einen Dienst mit "
            "`entrypoint: [\"/billing\"]`, aber backend/cmd/billing/ enthaelt "
            "keinen Go-Quelltext. Der bewachte Bau in backend/Dockerfile baut "
            "dann nichts, meldet das nur per echo und laeuft mit rc=0 durch — "
            "der Container startet aus einem Image ohne /billing. Genau diese "
            "Bau-Zeit-Pruefung ging mit R1-05-C01 verloren (REV-W1B N3). "
            "Verschoben oder umbenannt? Dann den Dienst mitziehen.")
    if ships_container and has_source and not builds:
        problems.append(
            "backend/Dockerfile baut `cmd/billing` nicht mehr nach "
            "/out/billing, obwohl infra/server/docker-compose.yml einen "
            "/billing-Container ausliefert. Derselbe Ausfall wie oben, nur eine "
            "Datei weiter.")
    if has_source and not builds:
        problems.append(
            "backend/cmd/billing/ existiert, aber backend/Dockerfile baut es "
            "nicht. Dann traegt kein Image das Binary.")
    if not STRICT_ARG.search(dockerfile_text) or not STRICT_EXIT.search(dockerfile_text):
        problems.append(
            "backend/Dockerfile kennt keinen strengen Modus: es fehlt ein "
            "`ARG VAKT_BILLING` mit einem Zweig, der bei fehlendem cmd/billing "
            "`exit 1` faehrt. Ohne ihn kann der private Release-Bau das Fehlen "
            "nicht zum Fehler machen, und ein `echo` im Build-Log ersetzt kein "
            "Gate (REV-W1B N3).")

    facts = {
        "ships_container": ships_container,
        "has_source": has_source,
        "builds": builds,
        "go_files": billing_dir_go_files,
    }
    return problems, facts


# ────────────────────────────────── Selbsttest ──────────────────────────────────

COMPOSE_PRIVAT = (
    "  vakt-billing:\n"
    "    profiles: [internal]\n"
    "    image: ghcr.io/norvik-ops/vakt-api:${APP_VERSION:-latest}\n"
    '    entrypoint: ["/billing"]\n')
COMPOSE_MIRROR = "  api:\n    image: ghcr.io/norvik-ops/vakt-api:latest\n"

DOCKERFILE_GUT = (
    "ARG VAKT_BILLING=auto\n"
    'RUN if [ "$VAKT_BILLING" = "required" ] && [ ! -d ./cmd/billing ]; then \\\n'
    "      echo fehlt >&2; \\\n"
    "      exit 1; \\\n"
    "    fi\n"
    "RUN if [ -d ./cmd/billing ]; then \\\n"
    "      go build -o /out/billing ./cmd/billing; \\\n"
    "    else echo absent; fi\n")
# Der Stand vor dieser Nachbesserung: bewachter Bau, aber kein strenger Modus.
DOCKERFILE_OHNE_STRENG = (
    "RUN if [ -d ./cmd/billing ]; then \\\n"
    "      go build -o /out/billing ./cmd/billing; \\\n"
    "    else echo absent; fi\n")


def selftest():
    cases = []

    # 1 — der private Baum, wie er sein soll.
    pr, f = evaluate(COMPOSE_PRIVAT, DOCKERFILE_GUT, 3)
    cases.append(("1 privat: Container + Quelltext + Bau -> gruen", not pr, pr))

    # 2 — DER BEFUND: cmd/billing verschwindet, der Container bleibt stehen.
    pr, _ = evaluate(COMPOSE_PRIVAT, DOCKERFILE_GUT, 0)
    cases.append(("2 Container ohne cmd/billing -> rot + benannt",
                  any("keinen Go-Quelltext" in p for p in pr), pr))

    # 3 — der Mirror-Fall: weder Container noch Quelltext. Muss gruen sein,
    #     sonst roetet dieses Gate den Bau, den R1-05-C01 gerade repariert hat.
    pr, f = evaluate(COMPOSE_MIRROR, DOCKERFILE_GUT, 0)
    cases.append(("3 Mirror: kein Container, kein Quelltext -> gruen",
                  not pr and not f["ships_container"], pr))

    # 4 — der Bau faellt aus dem Dockerfile, alles andere bleibt.
    pr, _ = evaluate(COMPOSE_PRIVAT, DOCKERFILE_GUT.replace(
        "go build -o /out/billing ./cmd/billing", "true"), 3)
    cases.append(("4 Bau aus dem Dockerfile entfernt -> rot + benannt",
                  any("baut `cmd/billing` nicht mehr" in p for p in pr), pr))

    # 5 — der strenge Modus fehlt (der Stand VOR dieser Nachbesserung). Auch mit
    #     heilem Baum ist das rot: ohne ihn gibt es keinen Bau, den der private
    #     Release streng stellen kann.
    pr, _ = evaluate(COMPOSE_PRIVAT, DOCKERFILE_OHNE_STRENG, 3)
    cases.append(("5 kein VAKT_BILLING-Zweig -> rot (N3-Ausgangsstand)",
                  any("strengen Modus" in p for p in pr), pr))

    # 6 — Gegenprobe zu 5: ein ARG ohne exit 1 genuegt nicht. Ein Schalter, der
    #     nichts abbricht, ist die Attrappe, gegen die dieses Gate gebaut ist.
    pr, _ = evaluate(COMPOSE_PRIVAT,
                     "ARG VAKT_BILLING=auto\nRUN echo $VAKT_BILLING\n"
                     "RUN go build -o /out/billing ./cmd/billing\n", 3)
    cases.append(("6 ARG ohne exit 1 -> rot (Attrappe faellt auf)",
                  any("strengen Modus" in p for p in pr), pr))

    # 7 — Quelltext ohne Bau, auch ohne Container. Faengt die Umbenennung von
    #     der anderen Seite.
    pr, _ = evaluate(COMPOSE_MIRROR, DOCKERFILE_GUT.replace(
        "go build -o /out/billing ./cmd/billing", "true"), 3)
    cases.append(("7 Quelltext ohne Bau -> rot",
                  any("traegt kein Image das Binary" in p for p in pr), pr))

    bad = [c for c in cases if not c[1]]
    for name, ok, detail in cases:
        print(f"  {'ok  ' if ok else 'FAIL'}  {name}" + ("" if ok else f"   -> {detail}"))
    if bad:
        print(f"\nSELBSTTEST FEHLGESCHLAGEN — {len(bad)}/{len(cases)}")
        return 1
    print(f"\nSelbsttest: alle {len(cases)} Faelle wie erwartet "
          f"(2 gruen als Gegenprobe, 5 rot mit Namensnennung)")
    return 0


# ──────────────────────────────────── main ──────────────────────────────────────

NOT_CHECKED = [
    "es baut nichts — dass ein fertiges Image /billing traegt, zeigt nur ein Bau",
    "der Inhalt von cmd/billing — das ist Sache der Go-Tests",
    "ob release.yml --build-arg VAKT_BILLING=required setzt (ausserhalb des "
    "Dateieigentums; benannt im GATE INVENTORY von check_ci_steps.py)",
]


def main():
    if "--selftest" in sys.argv:
        print("G-BILLING-BINARY — Selbsttest")
        return selftest()

    print("G-BILLING-BINARY — wer /billing ausliefert, muss es auch bauen")

    if not DOCKERFILE.is_file():
        print(f"\nFEHLER: {DOCKERFILE} fehlt — ohne den Dockerfile hat dieses "
              f"Gate nichts zu messen und darf nicht gruen melden.")
        return 1

    compose_text = COMPOSE.read_text(encoding="utf-8") if COMPOSE.is_file() else ""
    go_files = len(list(CMD_BILLING.glob("*.go"))) if CMD_BILLING.is_dir() else 0

    problems, f = evaluate(compose_text, DOCKERFILE.read_text(encoding="utf-8"),
                           go_files)

    print(f"  /billing-Container : "
          f"{'ja (infra/server/docker-compose.yml)' if f['ships_container'] else 'nein'}"
          + ("" if COMPOSE.is_file() else "  [infra/server/docker-compose.yml fehlt]"))
    print(f"  cmd/billing        : {f['go_files']} Go-Datei(en)")
    print(f"  Dockerfile baut es : {'ja' if f['builds'] else 'nein'}")
    if not f["ships_container"] and not f["has_source"]:
        print("  -> Mirror-Fall: dieser Baum liefert den Billing-Dienst nicht "
              "aus und baut ihn nicht.")
    print("  nicht geprueft     :")
    for n in NOT_CHECKED:
        print(f"                       · {n}")

    if problems:
        print(f"\nFEHLER ({len(problems)}):\n")
        for p in problems:
            print(f"  · {p}")
        return 1

    print("\nOK — Container, Quelltext und Bau sagen dasselbe, und der strenge "
          "Modus existiert.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
