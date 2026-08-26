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

──────────────────────────────────────────────────────────────────────────────
Erweiterung R1-INT-02 (2026-07-30): Test- und CI-Images
──────────────────────────────────────────────────────────────────────────────

Dieses Gate deckte bis heute NUR `docker-compose*.yml` und `helm/` ab. Container,
die in Go-Tests via testcontainers hochgezogen werden, und Service-Container in
GitHub-Actions-Workflows hat es strukturell nie angeschaut — und dort standen
`axllent/mailpit:latest`, `postgres:16-alpine` und `redis:7-alpine`, also drei
bewegliche Referenzen. Das Gate meldete trotzdem "OK": derselbe Fehler wie beim
G5-Outbound-Ratchet, der `internal/modules/**` nie zaehlte.

Fuer diese neue Flaeche gilt eine STRENGERE Invariante als fuer Compose/Helm:

  (E) Jede Test-/CI-Image-Referenz MUSS einen `@sha256:`-Digest tragen.

Begruendung: Ein Compose-Stack beim Kunden SOLL Patch-Updates eines Minor-Tags
bekommen. Eine Testumgebung soll das Gegenteil — sie muss sich zwischen zwei
Commits nicht veraendern, sonst sagt ein gruener Lauf von gestern nichts ueber
heute und ein roter Lauf nichts ueber den Diff. Deshalb wird (E) NICHT auf
Compose/Helm angewandt (und die liegen ohnehin in anderer Zustaendigkeit).

  (F) Dieselbe Repository wird nicht mit zwei verschiedenen Digests gepinnt.
      Faengt den realen Drift-Fall: jemand kopiert eine alte gepinnte Zeile.

Fuer digest-gepinnte Referenzen ist die Online-Pruefung (B) bewusst NUR
informativ, nie rot: der Digest ist die Bindung, nicht der Tag. Wuerde upstream
den Tag loeschen oder umhaengen, waere der Pin weiter voll funktionsfaehig —
ein Gate, das dann rot wird, wird abgeschaltet statt gefixt.

Was dieses Gate NICHT anschaut (bewusst, und es sagt das auch in der Ausgabe):

  · Dockerfiles (`FROM`) — eigene Build-Images, Tag ist dort Teil des Builds.
  · `docker run`/`docker pull` in `run:`-Bloecken von Workflows und in
    `scripts/*.sh` — Shell-Text, nicht deklarative Image-Felder.

    OFFENE KANTE, konkret und nicht hypothetisch (F3-Review von d536261):
    `scripts/restore_drill.sh:46-47` startet ZWEI Container auf dem beweglichen
    `postgres:16-alpine`, und `release.yml` faehrt das Skript im Job
    `backup-drills` auf dem self-hosted Runner. Ein schlechter Upstream-Release
    kann dieses RELEASE-Gate also weiterhin roeten. Die Datei liegt ausserhalb
    der Zustaendigkeit des Commits, der dieses Gate erweitert hat — deshalb
    benannt statt gepinnt. Wer `scripts/` besitzt: mitpinnen, dann sind es 35.
    (Nebenbei: `.secretlintignore` behauptet fuer dieses Skript
    `postgres:18-alpine` — es sind `16-alpine`.)
  · Laufzeit-gebaute Referenzen (String-Konkatenation, `${{ }}`-Expressions,
    Env-Vars ohne Default) — werden als `skipped` GEZAEHLT und benannt.
  · Ob ein Digest in der Registry existiert — das braeuchte die Registry-v2-
    Manifest-API und damit einen Auth-Token gegen das anonyme Pull-Limit.
  · Compose/Helm auf Digest-Pflicht (siehe oben: bewusste Entscheidung).
  · `testcontainers/ryuk` — den Reaper zieht die Bibliothek selbst, der Tag steht
    in testcontainers-go, nicht in unserem Code. Gepinnt wird er ueber die
    Modulversion in `backend/go.mod` (v0.42.0 -> ryuk:0.13.0, beim Testlauf am
    2026-07-30 beobachtet). Ein Dependabot-Bump von testcontainers-go aendert ihn
    mit; dieses Gate kann und soll das nicht doppeln.
"""
import json
import re
import subprocess
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


# ── Test-/CI-Flaeche (R1-INT-02) ──────────────────────────────────────────────

# Kommentare zuerst weg. Ein Gate, das den ROHTEXT durchsucht, zaehlt Prosa mit —
# genau der Fehler des Interface-Ratchets, der das englische Wort "any" in
# Kommentaren als Typ-`any` zaehlte. Die Datei images_test.go NENNT die alten
# beweglichen Tags in ihrer Begruendung; ohne diesen Schritt waere das Gate durch
# seine eigene Dokumentation rot.
#
# Kommentare werden mit einem kleinen Scanner entfernt, NICHT mit einem Regex:
# ein Regex ohne String-Literal-Bewusstsein laesst ein Literal wie "/*" einen
# Block-Kommentar eroeffnen und verschluckt danach beliebig viele Zeilen —
# inklusive echter Image-Referenzen. Das war ein stiller Bypass des ersten
# Entwurfs (F3-Review von d536261): eine Datei mit "/*" irgendwo davor wurde
# vollstaendig unsichtbar, bei Exit 0 und `skipped: 0`.

# Die zwei Idiome, ueber die in diesem Repo ueberhaupt ein Container entsteht.
# Bewusst an der VERWENDUNG angeknuepft, nicht an "sieht aus wie ein Image":
# ein Muster auf `"wort:wort"` wuerde Asynq-Task-Namen ("vaktscan:scans"),
# Redis-Keys ("license:org-123"), Adressen ("10.0.0.1:12345") und AI-Modelle
# ("qwen2.5:7b") einsammeln und das Gate mit Phantom-Treffern unbrauchbar machen.
#
# Zweite Einschraenkung, aus dem ersten Lauf dieses Gates gelernt: die Datei muss
# testcontainers-go IMPORTIEREN. Ein generisches `<pkg>.Run(ctx, <x>)` fischt
# sonst OTel-Span-Namen ("vaktscan.webhook.trigger" via `telemetry.Run`) und
# tabellengetriebene Subtests (`t.Run(tc.name, func(...))`) ein — 12 halluzinierte
# "Images" und 80 unnoetige Skips beim ersten Lauf. Ein Gate, das Referenzen
# erfindet, ist schlimmer als eines, das zugibt, sie nicht zu sehen.
GO_IMAGE_FIELD_RE = re.compile(r"\bImage:\s*([^,\n}]+)")
GO_CONST_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\"([^\"]+)\"")
GO_IDENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_.]*$")
GO_IMAGE_ASSIGN_RE = re.compile(r"\.Image\s*=\s*[^\n=][^\n]*")
GO_TC_IMPORT_RE = re.compile(r"\"github\.com/testcontainers/testcontainers-go(?:/[^\"]*)?\"")
GO_TC_MODULE_RE = re.compile(
    r"(?:([A-Za-z_][A-Za-z0-9_]*)\s+)?"
    r"\"github\.com/testcontainers/testcontainers-go/modules/([A-Za-z0-9_]+)\"")

# Workflows: `image:` (Service-Container und `container:`-Map) sowie die
# Skalarform `container: repo:tag`.
WF_IMAGE_RE = re.compile(r"^[ \t]*(?:image|container):[ \t]+[\"']?([^\s\"'#]+)", re.M)

DIGEST_RE = re.compile(r"@sha256:[0-9a-f]{64}$")


def repo_files() -> list[Path]:
    """Getrackte UND untracked Dateien.

    `git ls-files` allein sieht keine neuen Dateien — dann ist das Gate lokal
    gruen und in CI rot, und zwar ausgerechnet fuer die gerade angelegte Datei.
    `--exclude-standard` haelt .gitignore-Muell heraus.
    """
    out = subprocess.run(
        ["git", "-C", str(ROOT), "ls-files", "--cached", "--others", "--exclude-standard"],
        capture_output=True, text=True, check=True).stdout
    return [ROOT / line for line in out.splitlines() if line]


def _skip_literal(text: str, i: int) -> int:
    """Index hinter dem Literal, das an `i` beginnt (", ` oder ')."""
    q, n = text[i], len(text)
    j = i + 1
    while j < n:
        if q != "`" and text[j] == "\\":
            j += 2
            continue
        if text[j] == q:
            return j + 1
        if q != "`" and text[j] == "\n":  # unterminiert, nicht ueber die Zeile
            return j
        j += 1
    return n


def strip_go_comments(text: str) -> str:
    """Kommentare entfernen, String-/Rune-/Raw-Literale unangetastet lassen."""
    out: list[str] = []
    i, n = 0, len(text)
    while i < n:
        c = text[i]
        if c in "\"`'":
            j = _skip_literal(text, i)
            out.append(text[i:j])
            i = j
        elif c == "/" and i + 1 < n and text[i + 1] == "/":
            j = text.find("\n", i)
            i = n if j < 0 else j  # \n bleibt, wird im naechsten Schritt kopiert
        elif c == "/" and i + 1 < n and text[i + 1] == "*":
            j = text.find("*/", i + 2)
            i = n if j < 0 else j + 2
            out.append(" ")
        else:
            out.append(c)
            i += 1
    return "".join(out)


def literal_spans(code: str) -> list[tuple[int, int]]:
    """(start, end) jedes String-/Raw-/Rune-Literals, aufsteigend.

    Warum das noetig ist (F3-Delta-Review von 6033775): der Stripper war
    literal-bewusst, die MATCHER nicht. `strip_go_comments` erhaelt Literale
    korrekt — und `re.finditer("postgres\\.Run\\(")` suchte danach IN sie hinein.
    Ein Raw-String mit Beispielcode, etwa

        doc := `postgres.Run(ctx, "postgres:15-alpine")`

    erzeugte damit einen Befund fuer eine Referenz, die es nur als Text gibt.
    Das ist die Klasse, die der Docstring oben selbst als die schlimmste nennt,
    und sie roetet ein GESUNDES Repo — so ein Gate wird abgeschaltet, nicht
    gefixt. Ein Treffer in einem Literal ist keine ungelesene Referenz, sondern
    gar keine; er wird verworfen und nicht als `skipped` gezaehlt.
    """
    spans: list[tuple[int, int]] = []
    i, n = 0, len(code)
    while i < n:
        if code[i] in "\"`'":
            j = _skip_literal(code, i)
            spans.append((i, j))
            i = j
        else:
            i += 1
    return spans


def in_spans(idx: int, spans: list[tuple[int, int]]) -> bool:
    lo, hi = 0, len(spans)
    while lo < hi:
        mid = (lo + hi) // 2
        s, e = spans[mid]
        if idx < s:
            hi = mid
        elif idx >= e:
            lo = mid + 1
        else:
            return True
    return False


def brace_blocks(code: str, needle: str) -> list[tuple[int, int]]:
    """Spans der geschweiften Bloecke direkt hinter `needle`.

    Braucht es, weil das aufgeweitete `Image:`-Muster sonst JEDES Feld dieses
    Namens trifft. In einem Repo, dessen vaktscan-Modul Container-Images
    *scannt*, ist ein Fixture wie `scanTarget{Image: "postgres:15-alpine"}`
    voellig plausibel — und wuerde das Gate ohne Ausweg rot faerben.
    """
    out: list[tuple[int, int]] = []
    for m in re.finditer(re.escape(needle), code):
        i = m.end() - 1  # zeigt auf '{'
        depth, j, n = 0, i, len(code)
        while j < n:
            c = code[j]
            if c in "\"`'":
                j = _skip_literal(code, j)
                continue
            if c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
                if depth == 0:
                    out.append((i, j))
                    break
            j += 1
    return out


def in_blocks(idx: int, blocks: list[tuple[int, int]]) -> bool:
    return any(s <= idx <= e for s, e in blocks)


def call_args(code: str, open_paren: int) -> list[str] | None:
    """Argumente eines Aufrufs, klammer- und literal-bewusst getrennt.

    Ein Regex reicht hier nicht: das erste Argument ist mal `ctx`, mal
    `context.Background()`, mal `t.Context()`. Ein Muster, das dort einen
    klammerfreien Bezeichner verlangt, uebersieht die letzten beiden — und zwar
    STILL, ohne sie als `skipped` zu zaehlen. Genau so sind im ersten Entwurf
    zwei Bypaesse entstanden.
    """
    depth, args, cur = 0, [], []
    i, n = open_paren, len(code)
    while i < n:
        c = code[i]
        if c in "\"`'":
            j = _skip_literal(code, i)
            cur.append(code[i:j])
            i = j
            continue
        if c in "([{":
            depth += 1
            if depth == 1 and c == "(":
                i += 1
                continue
            cur.append(c)
        elif c in ")]}":
            depth -= 1
            if depth == 0:
                args.append("".join(cur).strip())
                return args
            cur.append(c)
        elif c == "," and depth == 1:
            args.append("".join(cur).strip())
            cur = []
        else:
            cur.append(c)
        i += 1
    return None


def collect_go_refs(files: list[Path]):
    """(refs, skipped, scanned) fuer alle Go-Dateien unter backend/.

    In Scope ist genau, was testcontainers-go importiert — das ist der einzige
    Weg, auf dem in diesem Repo aus Go heraus ein Container entsteht.
    """
    refs: dict[str, set[str]] = {}
    skipped: list[str] = []
    scanned = 0
    # Vorkommen, nicht (Referenz, Datei)-Paare: auth_oidc_email_verified_real_test.go
    # startet ZWEI Postgres-Container. Ein Nenner, der die zweite Stelle
    # wegdedupliziert, behauptet weniger Abdeckung als er hat.
    occurrences = 0
    for p in files:
        if p.suffix != ".go" or "backend/" not in p.as_posix():
            continue
        try:
            raw_src = p.read_text()
        except OSError:
            continue
        if not GO_TC_IMPORT_RE.search(strip_go_comments(raw_src)):
            continue
        rel = p.relative_to(ROOT).as_posix()
        scanned += 1

        # Identifier -> const. Paketweit, also alle Go-Dateien im selben Ordner:
        # die Konstanten liegen in images_test.go, benutzt werden sie nebenan.
        consts: dict[str, str] = {}
        for sib in sorted(p.parent.glob("*.go")):
            try:
                consts.update(GO_CONST_RE.findall(strip_go_comments(sib.read_text())))
            except OSError:
                continue

        r, s, o = scan_go_source(rel, raw_src, consts)
        for ref, where in r.items():
            refs.setdefault(ref, set()).update(where)
        skipped += s
        occurrences += o
    return refs, skipped, scanned, occurrences


def scan_go_source(rel: str, raw_src: str, consts: dict[str, str]):
    """Eine Go-Quelle -> (refs, skipped, occurrences).

    Als eigene Funktion, damit `check_image_tags_test.py` sie mit Quelltext-
    Strings treiben kann, so wie check_module_isolation_test.py `scan_file()`
    treibt. Dieser handgeschriebene Parser ist das fragilste Teil des Gates;
    seine Verifikation muss reproduzierbar im Repo liegen, nicht in einer
    Wegwerf-Datei.
    """
    refs: dict[str, set[str]] = {}
    skipped: list[str] = []
    occurrences = 0

    code = strip_go_comments(raw_src)
    if not GO_TC_IMPORT_RE.search(code):
        return refs, skipped, occurrences

    # Beide Matcher unten laufen ueber Text, in dem String-Literale ERHALTEN
    # sind (Kommentare sind weg). Ohne diese Spans suchen sie in Literale
    # hinein und erfinden Referenzen aus Beispielcode.
    lit = literal_spans(code)

    # `Image:` nur innerhalb eines ContainerRequest{...}-Blocks als Referenz
    # lesen. Ausserhalb ist es irgendein Feld namens Image (vaktscan hat
    # Fixtures, die genau so heissen) — das wird GEZAEHLT und BENANNT, aber
    # nicht rot: ein Gate, das ein gesundes Repo roetet, wird abgeschaltet.
    cr = brace_blocks(code, "ContainerRequest{")

    hits: list[str] = []
    for m in GO_IMAGE_FIELD_RE.finditer(code):
        if in_spans(m.start(), lit):
            continue  # Beispielcode in einem Literal, keine Referenz
        if in_blocks(m.start(), cr):
            hits.append(m.group(1).strip())
        else:
            occurrences += 1
            skipped.append(
                f"{rel} — `Image:` ausserhalb eines ContainerRequest{{…}}-Blocks "
                f"(`{m.group(1).strip()}`), nicht als Container-Image gelesen")

    # `postgres.Run(ctx, <image>, …)` — nur fuer tatsaechlich importierte
    # testcontainers-Modulpakete, nie fuer ein beliebiges `X.Run(`.
    for alias, pkg in GO_TC_MODULE_RE.findall(code):
        recv = alias or pkg
        for m in re.finditer(re.escape(recv) + r"\.Run\(", code):
            if in_spans(m.start(), lit):
                continue  # Beispielcode in einem Literal, keine Referenz
            args = call_args(code, m.end() - 1)
            if args is None or len(args) < 2:
                # Aufruf gefunden, Argumentliste nicht lesbar: AUSWEISEN,
                # nicht verwerfen. Ein stiller Skip ist der Bypass.
                occurrences += 1
                skipped.append(
                    f"{rel} — `{recv}.Run(` gefunden, aber die Argumentliste "
                    f"nicht lesbar (Image-Argument unbestimmt)")
                continue
            hits.append(args[1])

    # Eine Zuweisung `x.Image = …` erreicht keinen der Matcher oben. Sie kommt
    # heute nicht vor (geprueft); wenn sie auftaucht, soll sie SICHTBAR sein
    # statt still zu fehlen — als Skip, nicht als Befund, weil hier nicht
    # entscheidbar ist, ob das Ziel ein Container ist.
    for m in GO_IMAGE_ASSIGN_RE.finditer(code):
        if in_spans(m.start(), lit):
            continue
        occurrences += 1
        skipped.append(
            f"{rel} — `{m.group(0).strip()}` ist eine Zuweisung, kein "
            f"ContainerRequest-Feld; nicht als Container-Image gelesen")

    for raw in hits:
        occurrences += 1
        if raw.startswith('"') and raw.endswith('"') and len(raw) > 1:
            refs.setdefault(raw[1:-1], set()).add(rel)
        elif GO_IDENT_RE.match(raw) and raw in consts:
            refs.setdefault(consts[raw], set()).add(rel)
        else:
            # Nicht aufloesbar (Variable, Funktionsaufruf, Cross-Package).
            # Zaehlen und benennen, nicht still verwerfen.
            skipped.append(f"{rel} — Image aus `{raw}`, nicht zu einem Literal aufloesbar")
    return refs, skipped, occurrences


def collect_workflow_refs(files: list[Path]):
    refs: dict[str, set[str]] = {}
    skipped: list[str] = []
    occurrences = 0
    for p in files:
        rel = p.relative_to(ROOT).as_posix()
        if not rel.startswith(".github/workflows/") or p.suffix not in (".yml", ".yaml"):
            continue
        try:
            text = p.read_text()
        except OSError:
            continue
        for ref in WF_IMAGE_RE.findall(text):
            occurrences += 1
            if "${{" in ref or "${" in ref:
                skipped.append(f"{rel} — `{ref}` ist eine Expression, kein fester Tag")
                continue
            refs.setdefault(ref, set()).add(rel)
    return refs, skipped, occurrences


def split_digest(ref: str):
    """`repo:tag@sha256:…` -> (`repo:tag`, `sha256:…`); ohne Digest -> (ref, None)."""
    if "@" in ref:
        head, _, dig = ref.rpartition("@")
        return head, dig
    return ref, None


def tag_info(ns: str, name: str, tag: str):
    """(existiert: True/False/None, digest: str|None)."""
    try:
        req = urllib.request.Request(
            HUB.format(ns=ns, name=name, tag=urllib.parse.quote(tag, safe="")),
            headers={"User-Agent": "vakt-ci-image-check"},
        )
        with urllib.request.urlopen(req, timeout=20) as r:
            return True, json.load(r).get("digest")
    except urllib.error.HTTPError as e:
        return (False, None) if e.code == 404 else (None, None)
    except Exception:
        return None, None


def check_pinned_surface(refs: dict[str, set[str]], version: str):
    """(errors, notes) fuer Test-/CI-Images. Invarianten (E) und (F) plus (A)."""
    errors: list[str] = []
    notes: list[str] = []
    by_repo: dict[str, dict[str, set[str]]] = {}

    for ref in sorted(refs):
        where = ", ".join(sorted(refs[ref]))
        tagged, digest = split_digest(ref)
        repo, _, tag = tagged.rpartition(":")
        if not repo:
            repo, tag = tagged, ""

        # (E) Digest-Pflicht.
        if digest is None or not DIGEST_RE.search(ref):
            errors.append(
                f"{ref}\n      in: {where}\n"
                f"      hat keinen @sha256:-Digest. Ein beweglicher Tag heisst: die "
                f"Testumgebung kann sich ohne Commit aendern.\n"
                f"      Fix: exakte Version + Digest, z. B. `{repo}:<version>@sha256:<64 hex>`.")
            continue

        by_repo.setdefault(repo, {}).setdefault(digest, set()).update(refs[ref])

        # (A) offline: unsere Version an einem Fremd-Image ist immer falsch.
        if version and tag == version:
            errors.append(
                f"{ref}\n      in: {where}\n"
                f"      traegt die VAKT-Version {version} — das ist kein {repo}-Tag.")
            continue

        # (B) online, NUR informativ (der Digest bindet, nicht der Tag).
        coords = hub_coords(tagged)
        if coords is None:
            notes.append(f"{ref} ({where}) — keine Docker-Hub-Referenz, Tag nicht geprueft")
            continue
        exists, hub_digest = tag_info(*coords)
        if exists is None:
            notes.append(f"{ref} ({where}) — Registry nicht erreichbar, Tag nicht geprueft")
        elif exists is False:
            notes.append(f"{ref} ({where}) — Tag `{tag}` existiert nicht mehr; "
                         f"der Digest bindet weiter, aber der Tag ist irrefuehrend")
        elif hub_digest and hub_digest != digest:
            notes.append(f"{ref} ({where}) — Tag `{tag}` zeigt inzwischen auf "
                         f"{hub_digest[:19]}…; Pin ist aelter als der Tag")

    # (F) Ein Repository, ein Digest.
    for repo, digests in sorted(by_repo.items()):
        if len(digests) > 1:
            lines = "\n".join(f"        {d[:26]}… in {', '.join(sorted(w))}"
                              for d, w in sorted(digests.items()))
            errors.append(
                f"{repo}\n      ist mit {len(digests)} verschiedenen Digests gepinnt:\n{lines}\n"
                f"      Eine Testumgebung, die je nach Paket eine andere DB startet, "
                f"ist keine Umgebung. Alle Stellen auf denselben Digest ziehen.")
    return errors, notes


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

    # G-07: zero image references means none of SOURCES existed/matched — not
    # that the stack has no images. That would otherwise fall through to a
    # cheerful "✓ 0 Fremd-Images verifiziert" instead of flagging the gate itself
    # as broken (wrong ROOT, moved/renamed compose files).
    if not refs:
        print("✗ FAIL — 0 image references found across all SOURCES "
              "(non-vacuity guard, G-07). Check ROOT/SOURCES paths.")
        return 2

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

    # ── Test-/CI-Flaeche (R1-INT-02) ─────────────────────────────────────────
    files = repo_files()
    go_refs, go_skipped, go_scanned, n_go = collect_go_refs(files)
    wf_refs, wf_skipped, n_wf = collect_workflow_refs(files)

    # Non-Vakuitaets-Guard, gleiche Begruendung wie G-07 oben: 0 Treffer heisst
    # "der Sammler ist kaputt", nicht "es gibt keine Test-Container".
    if not go_refs:
        print("✗ FAIL — 0 Go-Image-Referenzen gefunden (Non-Vakuitäts-Guard). "
              "GO_IMAGE_FIELD_RE/GO_MODULE_RUN_RE oder die Dateiauswahl ist kaputt.")
        return 2

    pinned: dict[str, set[str]] = {}
    for src in (go_refs, wf_refs):
        for ref, where in src.items():
            pinned.setdefault(ref, set()).update(where)

    pin_errors, pin_notes = check_pinned_surface(pinned, version)
    errors += pin_errors
    skipped = go_skipped + wf_skipped

    if unverifiable or pin_notes:
        print("  Nicht verifiziert (kein Befund, nur ungeprüft):")
        for u in unverifiable + pin_notes:
            print(f"    · {u}")
        print()

    if skipped:
        print(f"  skipped: {len(skipped)} Referenzen konnte ich nicht lesen:")
        for s in skipped:
            print(f"    · {s}")
        print()
    else:
        print("  skipped: 0\n")

    if errors:
        print("✗ Image-Referenzen mit Befund:\n")
        for e in errors:
            print(f"  - {e}\n")
        print("Ein Tag, den die Registry nicht kennt, bricht 'docker compose up' bzw. erzeugt")
        print("ImagePullBackOff — beim Kunden, nicht bei uns. Ein beweglicher Tag in einer")
        print("Testumgebung ist die andere Haelfte: der Lauf wird unreproduzierbar, ohne dass")
        print("je jemand etwas committet.")
        return 1

    print(f"✓ Image-Referenzen OK — Compose/Helm: {ok} Fremd-Images in der Registry "
          f"verifiziert, {ours} eigene übersprungen, {len(unverifiable)} nicht verifizierbar.")
    print(f"  Test/CI: {len(pinned)} eindeutige Referenzen an {n_go + n_wf} Stellen "
          f"({n_go} in Go-Tests aus {go_scanned} testcontainers-Dateien, "
          f"{n_wf} in Workflows) — alle digest-gepinnt.")
    print("  Nicht angeschaut: Dockerfile-FROM, `docker run/pull` in run:-Blöcken und "
          "scripts/*.sh,\n"
          "  Digest-Existenz in der Registry (Auth-Token/Pull-Limit), "
          "Digest-Pflicht für Compose/Helm,\n"
          "  testcontainers/ryuk (Tag kommt aus testcontainers-go, nicht aus unserem Code).")
    print("  OFFENE KANTE: scripts/restore_drill.sh:46-47 fährt zwei Container auf dem\n"
          "  beweglichen postgres:16-alpine, gestartet von release.yml (Job backup-drills).\n"
          "  Dieses Gate sieht das nicht — die 33 oben sind alle DEKLARATIVEN Referenzen,\n"
          "  nicht alle Referenzen überhaupt.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
