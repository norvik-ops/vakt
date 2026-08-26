#!/usr/bin/env python3
"""G-HELM-IMAGE-TAG — das Chart fragt die Registry nach einem Tag, den es dort gibt.

Warum es dieses Gate gibt (2026-08-02, R1-05-C02)
-------------------------------------------------
`helm/vakt/values.yaml` pinnte `tag: "0.42.49"` fuer vakt-api und vakt-frontend.
GHCR fuehrt 92 Tags, und ALLE tragen ein `v`: `manifests/0.42.49` antwortet 404,
`manifests/v0.42.49` antwortet 200 (gemessen). Auf dem Default-Pfad ging damit
jede Installation in ImagePullBackOff — api, worker, frontend und der
migrate-Job. Wer `--set image.api.tag=v0.42.49` setzte, war nicht betroffen.

Die Wurzel ist eine Naht, nicht ein Tippfehler: `.github/workflows/release.yml`
erzeugt `VER=${GITHUB_REF_NAME#v}` und PRUEFT damit values.yaml (ohne v — dieselbe
Zahl ist auch `Chart.version` und `appVersion`), pusht die Images aber mit
`${{ github.ref_name }}` (mit v). Beide Seiten sind fuer sich konsistent; nur
zwischen ihnen fehlte die Umrechnung. Sie sitzt seit dem Fix im Chart:
`vakt.ownImageTag` in templates/_helpers.tpl.

Das bestehende `check_image_tags.py` sieht das strukturell nicht: sein OURS-Filter
ueberspringt die eigenen Images (er jagt die Ollama-Klasse — FREMD-Image traegt die
Vakt-Version). Diese Luecke schliesst dieses Gate.

Was es prueft
-------------
A. OFFLINE, immer: jede `image:`-Zeile in helm/vakt/templates/, die auf ein
   EIGENES Image zeigt (Repository unter ghcr.io/norvik-ops/), muss ihren Tag
   durch `vakt.ownImageTag` fuehren, und zwar MIT dem Repository als Argument.
   Das faengt die Drift: ein neues Template, das den Tag roh einsetzt, faellt
   auf. Dazu wird der KOERPER des Helpers gelesen (siehe A2).
A2. DER HELPER SELBST, offline (seit 2026-08-02, REV-W1B N1): sein Koerper muss
   die beiden Entscheidungen wirklich treffen, die seine Ueberschrift verspricht
   — die Herkunftspruefung auf ghcr.io/norvik-ops/ und die v-Voranstellung nur
   bei nackter Semver-Form. Vorher sah Teil A nur die AUFRUFE. Wer den Helper
   auf `{{ $t }}` neutralisierte, brachte R1-05-C02 vollstaendig zurueck und das
   Gate meldete ohne helm rc=0 und woertlich "OK" — gemessen im Review, nicht
   vermutet. Diese Pruefung ist eine Formpruefung am Quelltext, keine
   Auswertung: sie faengt Entfernen und Neutralisieren, nicht jedes denkbare
   Umschreiben, das die Merkmale behaelt. Das Urteil ueber das TATSAECHLICHE
   Verhalten faellt weiterhin nur Teil B.
B. GERENDERT, wenn `helm` da ist und die Subcharts aufgeloest sind: sechs Faelle,
   je gegen `helm template` gemessen —
     1. Default (values.yaml)        -> eigene Images tragen `v<semver>`
     2. `--set …tag=v0.42.49`        -> KEIN doppeltes v
     3. `--set …tag=latest`          -> unveraendert (kein `vlatest`)
     4. Fremd-Image (ollama)         -> unveraendert (kein `v0.31.2`)
     5. FREMDE Registry, api         -> unveraendert (kein `v0.42.49`)
     6. FREMDE Registry, frontend    -> unveraendert
   Die Faelle 2–6 sind die Gegenproben: eine Normalisierung, die stumpf ein `v`
   voranstellt, wuerde `latest`, das Fremd-Image und jede fremde Registry
   kaputtmachen — sie waere ein neuer Bruch an der Stelle, die sie reparieren
   soll. Faelle 5 und 6 sind seit REV-W1B N2 dabei; bis dahin praefixte der
   Helper entgegen seinem eigenen Kommentar auch fremde Registries.

Sein Nenner — was dieses Gate NICHT ansieht
-------------------------------------------
* ES FRAGT DIE REGISTRY NICHT. Dass `v0.42.49` in GHCR liegt, ist einmal von Hand
  gemessen worden; ein Gate, das bei jedem Push GHCR anfragt, wird vom
  Pull-Limit rot gefaerbt (dieselbe Begruendung wie in check_image_tags.py).
  Geprueft wird die FORM des Tags, nicht seine Existenz.
* TEIL B IST NICHT UEBERALL MESSBAR — UND DAS IST KEIN ERFOLG. Ohne `helm` im
  PATH oder ohne aufgeloeste Subcharts (`helm dependency build helm/vakt/`)
  laeuft nur Teil A. Bis 2026-08-02 endete dieser Lauf mit rc=0 und der Zeile
  "OK — jeder eigene Image-Tag geht durch die Normalisierung" — einer Aussage
  ueber eine Messung, die nicht stattgefunden hat. Jetzt ist das ein eigener
  Ausgang: rc=2 und "NICHT MESSBAR". Wer Teil A allein bewusst akzeptiert, sagt
  das mit `--teil-a-genuegt`; dann wird rc=2 zu rc=0 heruntergestuft, die Zeile
  "NICHT MESSBAR" bleibt aber stehen. `make gates` ruft es so auf, weil auf
  Entwicklerrechnern kein helm liegt. Der Job `helm-chart` in ci.yml hat helm
  und die Subcharts — dort gehoert der Aufruf OHNE Flag hin. Diese Zeile liegt
  ausserhalb des Dateieigentums dieser Spur und ist im GATE INVENTORY von
  scripts/check_ci_steps.py benannt statt still uebersprungen.
* NUR helm/vakt/. Andere Charts gibt es nicht.

Selbsttest: `python3 scripts/check_helm_image_tags.py --selftest`
Exit 0 = OK (Teil A und B gemessen, oder Teil A mit --teil-a-genuegt)
Exit 1 = Fehler · Exit 2 = NICHT MESSBAR (Teil B nicht gefahren).
"""

import pathlib
import re
import shutil
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
CHART = ROOT / "helm" / "vakt"
HELPER = "vakt.ownImageTag"
OWN_PREFIX = "ghcr.io/norvik-ops/"

IMAGE_LINE = re.compile(r'^\s*image:\s*"(.+)"\s*$')
# `{{ .Values.image.api.repository }}` -> image.api.repository
VALUE_REF = re.compile(r"\.Values\.([A-Za-z0-9_.]+)")
# `include "vakt.ownImageTag" (dict "repository" … "tag" …)` — der Helper muss das
# Repository sehen, sonst kann er die Herkunft nicht pruefen (N2).
HELPER_CALL = re.compile(
    r'include\s+"' + re.escape(HELPER) + r'"\s*\(\s*dict\s+"repository"\s')
# Der Koerper des Helpers zwischen define und end.
HELPER_BODY = re.compile(
    r'\{\{-?\s*define\s+"' + re.escape(HELPER) + r'"\s*-?\}\}(.*?)\{\{-?\s*end\s*-?\}\}',
    re.DOTALL)
# Vier Faelle fuer Teil B: (Beschreibung, --set-Argumente, Regex auf die Ausgabe)
CI_VALUES = [
    "--set", "secrets.dbUrl=postgres://vakt:test@db:5432/vakt",
    "--set", "secrets.redisUrl=redis://redis:6379",
    "--set", "secrets.secretKey=" + "0" * 64,
    "--set", "postgresql.auth.password=ci-test-password-only",
]


# ────────────────────────────── A: offline, reine Pruefung ──────────────────────

def own_repositories(values_text):
    """-> Menge der values-Pfade, deren `repository` ein EIGENES Image ist.

    Liest values.yaml zeilenweise: ein `repository:` gehoert zum davor genannten
    Block, ein `tag:` zum zuletzt gesehenen `repository:` — dieselbe Lesart, die
    release-prep.sh und release.yml benutzen.
    """
    own, path = set(), []
    for raw in values_text.splitlines():
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        indent = len(raw) - len(raw.lstrip())
        m = re.match(r"^\s*([A-Za-z0-9_]+):\s*(.*)$", raw)
        if not m:
            continue
        key, val = m.group(1), m.group(2).strip().strip('"')
        path = [p for p in path if p[0] < indent]
        if key == "repository" and val.startswith(OWN_PREFIX):
            own.add(".".join(p[1] for p in path))
        elif not val:
            path.append((indent, key))
    return own


def check_templates(templates, own):
    """-> (problems, geprueft, uebergangen). `templates` = {name: text}."""
    problems, checked, skipped = [], [], []
    for name, text in sorted(templates.items()):
        for raw in text.splitlines():
            m = IMAGE_LINE.match(raw)
            if not m:
                continue
            expr = m.group(1)
            refs = VALUE_REF.findall(expr)
            # Der Block, zu dem die repository-Referenz gehoert: image.api aus
            # image.api.repository.
            blocks = {r.rsplit(".", 1)[0] for r in refs if r.endswith(".repository")}
            if not blocks:
                skipped.append((name, expr))
                continue
            if not blocks & own:
                skipped.append((name, expr))
                continue
            checked.append((name, expr))
            if HELPER not in expr:
                problems.append(
                    f"{name}: `{expr}` setzt den Tag eines EIGENEN Images roh ein. "
                    f"values.yaml fuehrt die Version ohne `v` (dieselbe Zahl ist "
                    f"Chart.version/appVersion, und release.yml prueft sie so), die "
                    f"Registry kennt nur `v<version>`. Tag durch "
                    f"`include \"{HELPER}\" …` fuehren.")
            elif not HELPER_CALL.search(expr):
                problems.append(
                    f"{name}: `{expr}` ruft `{HELPER}` ohne das Repository auf. "
                    f"Der Helper praeft die Herkunft selbst (nur "
                    f"{OWN_PREFIX}* bekommt das `v`) und braucht dafuer beides: "
                    f"`include \"{HELPER}\" (dict \"repository\" … \"tag\" …)`. "
                    f"Ohne Repository praefixt er auch fremde Registries — "
                    f"REV-W1B N2.")
    return problems, checked, skipped


def check_helper_body(helpers_text):
    """-> Liste von Problemen am KOERPER des Helpers (A2, REV-W1B N1).

    Teil A sah bis 2026-08-02 nur die Aufrufe. Wer den Helper zu `{{ $t }}`
    neutralisierte, hatte R1-05-C02 zurueck, und ohne helm meldete dieses Gate
    trotzdem rc=0 und "OK". Das hier ist die statische Haelfte des Schutzes:
    eine Formpruefung am Quelltext. Sie faengt Entfernen und Neutralisieren.
    Ueber das tatsaechliche Renderverhalten urteilt weiterhin nur Teil B.
    """
    m = HELPER_BODY.search(helpers_text)
    if m is None:
        return [f"templates/_helpers.tpl: `{HELPER}` fehlt — ohne den Helper gibt "
                f"es keine Stelle, an der die v-lose values-Version und der "
                f"v-behaftete Registry-Tag zusammenkommen."]
    body = m.group(1)
    problems = []
    if OWN_PREFIX not in body:
        problems.append(
            f"templates/_helpers.tpl: der Koerper von `{HELPER}` nennt "
            f"`{OWN_PREFIX}` nicht. Sein Kommentar verspricht, nur EIGENE Images "
            f"zu praefixen; ohne diese Pruefung bekommt auch "
            f"`registry.intern/vakt-api:0.42.49` ein `v` und laeuft in "
            f"ImagePullBackOff (REV-W1B N2, gemessen).")
    if "regexMatch" not in body:
        problems.append(
            f"templates/_helpers.tpl: der Koerper von `{HELPER}` prueft die Form "
            f"des Tags nicht mehr (`regexMatch` fehlt). Entweder praefixt er dann "
            f"stumpf alles — aus `latest` wird `vlatest` — oder er gibt den Tag "
            f"unveraendert durch, und R1-05-C02 ist zurueck.")
    if "v{{" not in body.replace(" ", ""):
        problems.append(
            f"templates/_helpers.tpl: der Koerper von `{HELPER}` stellt nirgends "
            f"ein `v` voran. Genau so sah der neutralisierte Helper aus, mit dem "
            f"REV-W1B N1 gemessen hat, dass dieses Gate ohne helm rc=0 und `OK` "
            f"meldet, waehrend der Default-Pfad in ImagePullBackOff laeuft.")
    return problems


def evaluate(values_text, helpers_text, templates):
    problems = check_helper_body(helpers_text)
    own = own_repositories(values_text)
    if not own:
        problems.append(
            "values.yaml: kein Repository unter " + OWN_PREFIX + " gefunden — die "
            "Struktur hat sich geaendert, und dieses Gate haette still nichts "
            "geprueft.")
    tpl_problems, checked, skipped = check_templates(templates, own)
    problems += tpl_problems
    return problems, {"own": sorted(own), "checked": checked, "skipped": skipped}


# ────────────────────────────── B: gerendert ────────────────────────────────────

RENDER_CASES = [
    ("Default aus values.yaml", [],
     r'ghcr\.io/norvik-ops/vakt-\w+:v\d+\.\d+\.\d+', True,
     "eigene Images muessen den v-Tag tragen"),
    ("--set image.api.tag=v0.42.49", ["--set", "image.api.tag=v0.42.49"],
     r'ghcr\.io/norvik-ops/vakt-api:vv', False,
     "ein bereits v-behafteter Tag darf kein zweites v bekommen"),
    ("--set image.api.tag=latest", ["--set", "image.api.tag=latest"],
     r'ghcr\.io/norvik-ops/vakt-api:vlatest', False,
     "`latest` ist keine Semver-Zahl und darf unveraendert bleiben"),
    ("--set ollama.enabled=true", ["--set", "ollama.enabled=true"],
     r'ollama/ollama:v', False,
     "ein FREMD-Image darf die Normalisierung nie sehen"),
    # 5 und 6: REV-W1B N2. Der Helper praefixte, ohne das Repository zu kennen —
    # wer das Chart gegen seine eigene Registry fuhr, bekam einen Tag, den es
    # dort nicht gibt. Dieselbe Familie wie R1-05-C02, nur andersherum.
    ("fremde Registry (api)",
     ["--set", "image.api.repository=registry.intern/vakt-api",
      "--set", "image.api.tag=0.42.49"],
     r'registry\.intern/vakt-api:v0\.42\.49', False,
     "ein Image aus einer FREMDEN Registry darf kein `v` bekommen — dort gibt es "
     "den v-Tag nicht"),
    ("fremde Registry (frontend)",
     ["--set", "image.frontend.repository=registry.intern/vakt-fe",
      "--set", "image.frontend.tag=0.42.49"],
     r'registry\.intern/vakt-fe:v0\.42\.49', False,
     "dieselbe Zusage fuer das Frontend-Image"),
]


def render(helm, chart, extra):
    p = subprocess.run([helm, "template", "vakt", str(chart)] + CI_VALUES + extra,
                       capture_output=True, text=True, timeout=300)
    if p.returncode != 0:
        return None, (p.stderr.strip().splitlines() or ["rc != 0"])[0]
    return p.stdout, None


def render_half(chart):
    """-> (problems, notes, unmeasured). `unmeasured` = Grund oder None.

    Ist `unmeasured` gesetzt, hat Teil B NICHT stattgefunden. Das ist ein eigener
    Ausgang (rc=2), kein Erfolg — bis 2026-08-02 endete dieser Fall in derselben
    Erfolgsmeldung wie eine gefahrene Messung.
    """
    helm = shutil.which("helm")
    if helm is None:
        return [], [], "helm fehlt im PATH"
    if not (chart / "charts").is_dir():
        return [], [], (f"{chart}/charts/ fehlt (Subcharts nicht aufgeloest) — "
                        f"einmal `helm dependency build {chart}/`")
    problems, notes = [], []
    for name, extra, pattern, want, why in RENDER_CASES:
        out, err = render(helm, chart, extra)
        if err:
            problems.append(f"helm template ({name}) scheitert: {err}")
            continue
        hit = re.search(pattern, out) is not None
        if hit != want:
            problems.append(
                f"gerendert, Fall `{name}`: {why} — "
                f"Muster `{pattern}` {'fehlt' if want else 'trifft'}.")
        else:
            notes.append(f"{name}: wie erwartet")
    return problems, notes, None


# ────────────────────────────────── Selbsttest ──────────────────────────────────

VALUES_FIXTURE = """image:
  api:
    repository: ghcr.io/norvik-ops/vakt-api
    tag: "0.42.49"
  frontend:
    repository: ghcr.io/norvik-ops/vakt-frontend
    tag: "0.42.49"
ollama:
  image:
    repository: ollama/ollama
    tag: "0.31.2"
"""
# Der echte Helper in Kurzform: Herkunftspruefung + Formpruefung + v-Voranstellung.
HELPERS_FIXTURE = (
    '{{- define "vakt.ownImageTag" -}}\n'
    '{{- $repo := .repository | toString -}}\n'
    '{{- $t := .tag | toString -}}\n'
    '{{- if and (hasPrefix "ghcr.io/norvik-ops/" $repo) '
    '(regexMatch "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+" $t) -}}v{{ $t }}'
    '{{- else -}}{{ $t }}{{- end -}}\n'
    '{{- end -}}\n')
# Der neutralisierte Helper aus REV-W1B N1: die Aufrufe bleiben korrekt, der
# Mechanismus ist weg. Genau diese Fassung meldete "OK" ohne helm.
HELPERS_NEUTRALISIERT = (
    '{{- define "vakt.ownImageTag" -}}\n'
    '{{- $t := .tag | toString -}}\n'
    '{{ $t }}\n'
    '{{- end -}}\n')
# Der Helper VOR N2: praeft die Tag-Form, aber nicht die Herkunft.
HELPERS_OHNE_HERKUNFT = (
    '{{- define "vakt.ownImageTag" -}}\n'
    '{{- $t := . | toString -}}\n'
    '{{- if regexMatch "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+" $t -}}v{{ $t }}'
    '{{- else -}}{{ $t }}{{- end -}}\n'
    '{{- end -}}\n')
GOOD_TPL = {
    "api/deployment.yaml":
        '          image: "{{ .Values.image.api.repository }}:'
        '{{ include "vakt.ownImageTag" (dict "repository" '
        '.Values.image.api.repository "tag" .Values.image.api.tag) }}"\n',
    "ollama/statefulset.yaml":
        '          image: "{{ .Values.ollama.image.repository }}:'
        '{{ .Values.ollama.image.tag }}"\n',
}


def selftest():
    cases = []

    # 1 — gruen auf dem gefixten Stand.
    pr, st = evaluate(VALUES_FIXTURE, HELPERS_FIXTURE, GOOD_TPL)
    cases.append(("1 gefixter Stand gruen", not pr, pr))

    # 2 — ECHTE Regression R1-05-C02: der rohe Tag ist zurueck. ROT MIT NAMEN.
    raw = dict(GOOD_TPL, **{"api/deployment.yaml":
        '          image: "{{ .Values.image.api.repository }}:'
        '{{ .Values.image.api.tag }}"\n'})
    pr, _ = evaluate(VALUES_FIXTURE, HELPERS_FIXTURE, raw)
    cases.append(("2 roher Tag auf eigenem Image -> rot + benannt",
                  len(pr) == 1 and "api/deployment.yaml" in pr[0], pr))

    # 3 — Verbesserung nicht festgehalten: der Helper selbst verschwindet.
    pr, _ = evaluate(VALUES_FIXTURE, "{{/* leer */}}\n", GOOD_TPL)
    cases.append(("3 Helper geloescht -> rot + benannt",
                  any(HELPER in p and "_helpers.tpl" in p for p in pr), pr))

    # 3b — REV-W1B N1, der Kern dieser Nachbesserung: die Aufrufe bleiben
    # korrekt, der Helper-Koerper ist neutralisiert. Teil A sah das frueher
    # nicht, und ohne helm im PATH endete der Lauf in "OK".
    pr, _ = evaluate(VALUES_FIXTURE, HELPERS_NEUTRALISIERT, GOOD_TPL)
    cases.append(("3b Helper neutralisiert, Aufrufe heil -> rot (N1)",
                  any("_helpers.tpl" in p for p in pr)
                  and any("regexMatch" in p for p in pr), pr))

    # 3c — REV-W1B N2: der Helper von vor der Nachbesserung. Er praeft die Form
    # des Tags, aber nicht die Herkunft, und praefixt deshalb auch fremde
    # Registries.
    pr, _ = evaluate(VALUES_FIXTURE, HELPERS_OHNE_HERKUNFT, GOOD_TPL)
    cases.append(("3c Helper ohne Herkunftspruefung -> rot (N2)",
                  any(OWN_PREFIX in p and "_helpers.tpl" in p for p in pr), pr))

    # 3d — der Aufruf faellt auf die alte Ein-Argument-Form zurueck. Dann sieht
    # der Helper das Repository nicht und kann N2 nicht verhindern.
    einarg = dict(GOOD_TPL, **{"api/deployment.yaml":
        '          image: "{{ .Values.image.api.repository }}:'
        '{{ include "vakt.ownImageTag" .Values.image.api.tag }}"\n'})
    pr, _ = evaluate(VALUES_FIXTURE, HELPERS_FIXTURE, einarg)
    cases.append(("3d Aufruf ohne Repository -> rot + benannt",
                  any("ohne das Repository" in p for p in pr), pr))

    # 4 — Gegenprobe: das FREMD-Image bleibt roh, und das ist richtig. Wuerde
    # dieses Gate es einfordern, wuerde es `ollama/ollama:v0.31.2` erzwingen —
    # den Tag gibt es beim Hersteller nicht (die v0.42.42-Klasse in Reinform).
    cases.append(("4 Fremd-Image ohne Helper -> nicht rot",
                  not any("ollama" in p for p in evaluate(
                      VALUES_FIXTURE, HELPERS_FIXTURE, GOOD_TPL)[0]),
                  st["skipped"]))

    # 5 — Struktur weggezogen: kein eigenes Repository mehr in values.yaml. Das
    # Gate darf dann nicht schweigend gruen sein.
    pr, _ = evaluate("image:\n  api:\n    repository: example.com/foo\n    tag: \"1\"\n",
                     HELPERS_FIXTURE, GOOD_TPL)
    cases.append(("5 kein eigenes Image in values.yaml -> rot (nicht still gruen)",
                  any("still nichts geprueft" in p for p in pr), pr))

    # 6 — Zaehl-Zusage: das Fremd-Image wird als uebergangen GEZAEHLT, nicht
    # verschluckt.
    _, st = evaluate(VALUES_FIXTURE, HELPERS_FIXTURE, GOOD_TPL)
    cases.append(("6 Fremd-Image gezaehlt", len(st["skipped"]) == 1
                  and len(st["checked"]) == 1, (st["skipped"], st["checked"])))

    bad = [c for c in cases if not c[1]]
    for name, ok, detail in cases:
        print(f"  {'ok  ' if ok else 'FAIL'}  {name}" + ("" if ok else f"   -> {detail}"))
    if bad:
        print(f"\nSELBSTTEST FEHLGESCHLAGEN — {len(bad)}/{len(cases)}")
        return 1
    print(f"\nSelbsttest: alle {len(cases)} Faelle wie erwartet "
          f"(2 gruen als Gegenprobe, 6 rot mit Namensnennung, 1 Zaehl-Zusage)")
    return 0


# ──────────────────────────────────── main ──────────────────────────────────────

NOT_CHECKED = [
    "die Registry wird nicht gefragt — geprueft wird die FORM des Tags, nicht seine Existenz",
    "andere Charts — es gibt nur helm/vakt/",
]


def main():
    if "--selftest" in sys.argv:
        print("G-HELM-IMAGE-TAG — Selbsttest")
        return selftest()

    print("G-HELM-IMAGE-TAG — eigene Image-Tags in der Form, die die Registry fuehrt")

    teil_a_genuegt = "--teil-a-genuegt" in sys.argv

    values = CHART / "values.yaml"
    helpers = CHART / "templates" / "_helpers.tpl"
    if not values.is_file() or not helpers.is_file():
        print(f"\nNICHT MESSBAR — {CHART} unvollstaendig, auch Teil A ist nicht "
              f"gelaufen. Kein Urteil.")
        return 0 if teil_a_genuegt else 2

    templates = {}
    for p in sorted((CHART / "templates").rglob("*.yaml")):
        templates[str(p.relative_to(CHART / "templates"))] = p.read_text(encoding="utf-8")

    problems, st = evaluate(values.read_text(encoding="utf-8"),
                            helpers.read_text(encoding="utf-8"), templates)
    rp, notes, unmeasured = render_half(CHART)
    problems += rp

    print(f"  eigene images  : {', '.join(st['own']) or '—'}")
    print(f"  image-zeilen   : {len(st['checked'])} geprueft, "
          f"{len(st['skipped'])} uebergangen (Fremd-Image oder ohne repository-Bezug)")
    for name, expr in st["skipped"]:
        print(f"                   · {name}: {expr}")
    if unmeasured:
        print(f"  gerendert      : NICHT GEFAHREN — {unmeasured}")
    else:
        print(f"  gerendert      : {len(notes)}/{len(RENDER_CASES)} Faelle gemessen")
        for n in notes:
            print(f"                   · {n}")
    print("  nicht geprueft :")
    for n in NOT_CHECKED:
        print(f"                   · {n}")

    if problems:
        print(f"\nFEHLER ({len(problems)}):\n")
        for p in problems:
            print(f"  · {p}")
        return 1

    if unmeasured:
        # Kein Erfolg. Teil A ist gruen, aber Teil A urteilt nur ueber die FORM
        # des Quelltexts; ob das Chart wirklich den Tag rendert, den die Registry
        # fuehrt, hat hier niemand gemessen.
        print(f"\nNICHT MESSBAR — Teil A gruen, Teil B nicht gefahren "
              f"({unmeasured}).")
        print("  Teil A urteilt ueber die Form von Templates und Helper-Koerper,")
        print("  nicht ueber das Ergebnis von `helm template`. Wer nur Teil A")
        print("  will, sagt das mit --teil-a-genuegt; im Job `helm-chart` (helm")
        print("  und Subcharts vorhanden) gehoert der Aufruf OHNE Flag hin.")
        if teil_a_genuegt:
            print("  --teil-a-genuegt gesetzt -> rc 2 auf 0 heruntergestuft.")
            return 0
        return 2

    print(f"\nOK — jeder eigene Image-Tag geht durch die Normalisierung "
          f"(Teil A und alle {len(RENDER_CASES)} gerenderten Faelle gemessen).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
