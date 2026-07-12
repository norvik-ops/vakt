#!/usr/bin/env python3
"""Jedes referenzierte Fremd-Image muss es wirklich geben.

Warum es dieses Gate gibt (2026-07-12):

`docker-compose.yml` pinnte Ollama seit dem initialen Monorepo-Merge auf
`ollama/ollama:${OLLAMA_TAG:-0.6}`. Diesen Tag hat Ollama nie veroeffentlicht — es
gibt 0.6.8, aber kein blankes 0.6. Aufgefallen ist es niemandem, weil der Dienst
hinter `--profile ai` hing und praktisch nie gezogen wurde. Am 2026-07-05 nahm
68bf237 ("AI advisor on by default") das Profil weg — und damit lief JEDER frische
`docker compose up` in einen Pull-Fehler, also genau der beworbene
"in unter 5 Minuten startbereit"-Start eines Neukunden.

Parallel dazu trug `helm/vakt/values.yaml` als Ollama-Tag ueber ein Dutzend Releases
hinweg die VAKT-Version (zuletzt 0.42.41), weil das Release-sed blind jede `tag:`-
Zeile mitzog — auch die, deren Kommentar ("pinned; update manually") genau das
ausschliessen sollte. `ollama/ollama:0.42.41` existiert nicht: ImagePullBackOff fuer
jeden, der das Chart mit ai.enabled deployt.

Beide Fehler sind fuer jeden statischen Check unsichtbar: Die YAML ist gueltig, der
String sieht wie eine Version aus. Nur ein Blick in die Registry entscheidet.

Geprueft wird:

  (A) offline — kein fremdes Image traegt die VAKT-Version. Faengt die sed-Sweep-
      Klasse ohne jedes Netz und sofort.

  (B) online — jeder Tag eines fremden Docker-Hub-Images existiert wirklich.

Bewusst ueber die Hub-Tags-API (hub.docker.com/v2/repositories/...), nicht ueber
`docker manifest inspect`: letzteres zaehlt gegen das anonyme Pull-Limit, und ein
rate-limitetes Gate meldet "existiert nicht" fuer ein Image, das es gibt. Ein Gate,
das bei gesundem Repo rot wird, wird abgeschaltet statt gefixt.

"Nicht pruefbar" (Netz weg, Nicht-Hub-Registry) ist deshalb NICHT dasselbe wie
"fehlt": es wird gezaehlt und ausgewiesen, faerbt den Lauf aber nicht rot.
"""
import json
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# Wo Images referenziert werden. Bewusst explizit: ein glob wuerde jede neue YAML
# mitnehmen und das Gate bei der ersten Test-Fixture rot faerben.
SOURCES = [
    "docker-compose.yml",
    "docker-compose.dev.yml",
    "docker-compose.tls.yml",
    "docker-compose.observability.yml",
    "docker-compose.backup.yml",
    "infra/server/docker-compose.yml",
    "helm/vakt/values.yaml",
]

# Unsere eigenen Images: der Tag ist der Release-Tag und existiert zur CI-Zeit noch
# nicht. Ein Tippfehler DARIN faellt beim Deploy sofort auf; ein Tippfehler in einem
# Fremd-Image erst beim Kunden.
OURS = re.compile(r"^ghcr\.io/norvik-ops/")

# [ \t] statt \s: \s frisst den Zeilenumbruch. In helm/values.yaml steht unter
# `image:` nur ein Block (repository/tag), und der Ausdruck griff dann die naechste
# Zeile — "repository:" landete als vermeintlicher Image-Name im Report. Ein Gate mit
# Phantom-Treffern wird abgeschaltet, nicht gefixt.
IMAGE_RE = re.compile(r"^[ \t]*image:[ \t]+[\"']?([^\s\"'#]+)", re.M)

# helm/values.yaml trennt repository/tag; hier zusammengesetzt.
HELM_RE = re.compile(
    r"repository:[ \t]+[\"']?([^\s\"'#]+)[\"']?[ \t]*\n"
    r"(?:[ \t]*#[^\n]*\n)*"  # Kommentarzeilen zwischen repository und tag
    r"[ \t]*tag:[ \t]+[\"']?([^\s\"'#]+)", re.M)

# ${VAR:-default}: compose setzt den Default ein, wenn der Operator nichts angibt.
# Der Default ist also genau das, was ein Kunde ohne eigene .env bekommt — und genau
# der muss stimmen.
VAR_RE = re.compile(r"\$\{([A-Z_]+)(?::-([^}]*))?\}")

HUB = "https://hub.docker.com/v2/repositories/{ns}/{name}/tags/{tag}"


def resolve(image: str) -> str | None:
    """Setzt ${VAR:-default} auf den Default. Ohne Default nicht pruefbar."""
    out = VAR_RE.sub(lambda m: m.group(2) if m.group(2) is not None else "\x00", image)
    return None if "\x00" in out else out


def hub_coords(ref: str):
    """(namespace, name, tag) fuer Docker Hub — oder None, wenn nicht Hub."""
    repo, _, tag = ref.rpartition(":")
    if not repo or "/" in tag:  # kein Tag, nur ein Pfad mit Port o.ae.
        return None
    head = repo.split("/")[0]
    if "." in head or ":" in head:  # eigene Registry (ghcr.io, quay.io, …)
        return None
    parts = repo.split("/")
    if len(parts) == 1:
        return "library", parts[0], tag  # offizielles Image: postgres -> library/postgres
    if len(parts) == 2:
        return parts[0], parts[1], tag
    return None


def tag_exists(ns: str, name: str, tag: str):
    """True / False / None (nicht pruefbar)."""
    try:
        req = urllib.request.Request(
            HUB.format(ns=ns, name=name, tag=urllib.parse.quote(tag, safe="")),
            headers={"User-Agent": "vakt-ci-image-check"},
        )
        with urllib.request.urlopen(req, timeout=20) as r:
            json.load(r)
            return True
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return False
        return None  # 429/5xx: nichts behaupten, was wir nicht wissen
    except Exception:
        return None


def vakt_version() -> str:
    spec = (ROOT / "backend/internal/shared/apidocs/openapi.yaml").read_text()
    m = re.search(r'^\s*version:\s*"([^"]+)"', spec, re.M)
    return m.group(1) if m else ""


def main() -> int:
    refs: dict[str, set[str]] = {}
    for rel in SOURCES:
        p = ROOT / rel
        if not p.exists():
            continue
        text = p.read_text()
        found = set(IMAGE_RE.findall(text))
        for repo, tag in HELM_RE.findall(text):
            found.add(f"{repo}:{tag}")
        for img in found:
            refs.setdefault(img, set()).add(rel)

    version = vakt_version()
    errors: list[str] = []
    unverifiable: list[str] = []
    ok = 0
    ours = 0

    for image in sorted(refs):
        where = ", ".join(sorted(refs[image]))
        ref = resolve(image)
        if ref is None:
            # Kein Default => der Operator MUSS die Variable setzen. Compose bricht
            # dann mit klarer Meldung ab, nicht still.
            unverifiable.append(f"{image} ({where}) — keine Default-Version")
            continue
        if OURS.match(ref):
            ours += 1
            continue

        coords = hub_coords(ref)
        if coords is None:
            unverifiable.append(f"{ref} ({where}) — keine Docker-Hub-Referenz")
            continue
        ns, name, tag = coords

        # (A) offline: ein Fremd-Image mit UNSERER Versionsnummer ist immer falsch.
        if version and tag == version:
            errors.append(
                f"{ref}\n      in: {where}\n"
                f"      trägt die VAKT-Version {version}. Das ist kein {ns}/{name}-Tag, "
                f"das ist unsere — vermutlich hat ein Release-sed die Zeile mitgezogen."
            )
            continue

        # (B) online: gibt es den Tag?
        res = tag_exists(ns, name, tag)
        if res is True:
            ok += 1
        elif res is False:
            errors.append(
                f"{ref}\n      in: {where}\n"
                f"      existiert nicht: {ns}/{name} hat keinen Tag '{tag}'."
            )
        else:
            unverifiable.append(f"{ref} ({where}) — Registry nicht erreichbar")

    if unverifiable:
        print("  Nicht verifiziert (kein Befund, nur ungeprüft):")
        for u in unverifiable:
            print(f"    · {u}")
        print()

    if errors:
        print("✗ Image-Tags, die es nicht gibt:\n")
        for e in errors:
            print(f"  - {e}\n")
        print("Ein Tag, den die Registry nicht kennt, ist kein Schönheitsfehler: er bricht")
        print("'docker compose up' bzw. erzeugt ImagePullBackOff — beim Kunden, nicht bei uns.")
        return 1

    print(f"✓ Image-Tags OK — {ok} Fremd-Images in der Registry verifiziert, "
          f"{ours} eigene übersprungen, {len(unverifiable)} nicht verifizierbar.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
