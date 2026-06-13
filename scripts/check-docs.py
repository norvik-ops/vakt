#!/usr/bin/env python3
"""Doku-Konsistenz-Guard für vakt-app.

Fängt drei Klassen von Doku-Drift, die in der Vergangenheit real aufgetreten
sind (Go-Version 1.22 statt 1.26, AI-Default qwen2.5:7b statt :3b, kaputte
Cross-Doc-Links nach Datei-Umzügen):

  1. Go-Version in den Stack-Docs == Minor aus ``backend/go.mod``
  2. AI-Default-Modell in ``.env.example`` == Default in ``config.go``
  3. Keine kaputten relativen ``.md``-Links in versionierten Docs

Quelle der Wahrheit ist immer der Code (go.mod, config.go), nicht die Doku.

Läuft in CI (``.github/workflows/docs.yml``) und lokal:

    python3 scripts/check-docs.py

Exit-Code != 0 bei gefundenem Drift.
"""
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
os.chdir(ROOT)

errors: list[str] = []


def err(msg: str) -> None:
    errors.append(msg)


# ── 1. Go-Version ────────────────────────────────────────────────────────────
# Die kuratierte Liste enthält nur Docs, die den AKTUELLEN Stack beschreiben.
# History/Audit-/Review-Dokumente nennen ältere Go-Versionen absichtlich und
# sind hier bewusst NICHT gelistet.
STACK_DOCS = [
    "README.md",
    "docs/architecture.md",
    "docs/wiki/README.md",
    "docs/security/pentest-rfp.md",
    "backend/internal/integration_test/README.md",
]
_GO_VER = re.compile(r"go[ -](\d+\.\d{1,2})", re.IGNORECASE)


def check_go_version() -> None:
    m = re.search(r"^go (\d+\.\d+)", open("backend/go.mod").read(), re.M)
    if not m:
        err("backend/go.mod: keine go-Direktive gefunden")
        return
    canon = m.group(1)

    for f in STACK_DOCS:
        if not os.path.exists(f):
            continue
        for lineno, line in enumerate(open(f, encoding="utf-8", errors="ignore"), 1):
            for mm in _GO_VER.finditer(line):
                if mm.group(1) != canon:
                    err(f"{f}:{lineno}: Go-Version '{mm.group(0)}' ≠ backend/go.mod ({canon})")

    # Das separate operator-Modul muss zur Haupt-Minor passen.
    if os.path.exists("operator/go.mod"):
        om = re.search(r"^go (\d+\.\d+)", open("operator/go.mod").read(), re.M)
        if om and om.group(1) != canon:
            err(f"operator/go.mod: go {om.group(1)} ≠ backend/go.mod ({canon})")


# ── 2. AI-Default-Modell ─────────────────────────────────────────────────────
# Kanonische Docs, die Nutzern den Default nennen (sollen "what you get out of
# the box" korrekt beschreiben). Andere Modelle dürfen als Alternative genannt
# werden — nur die als *Default* deklarierte Zeile wird geprüft.
MODEL_DOCS = [
    "README.md",
    "CLAUDE.md",
    "docs/wiki/README.md",
    "docs/wiki/ai-features.md",
    "docs/wiki/configuration.md",
    "docs/wiki/installation.md",
    "docs/wiki/monitoring.md",
    "docs/wiki/modules/comply.md",
    "docs/setup.md",
    "docs/guides/getting-started.md",
    "docs/operations/runbook.md",
    "docs/landingpage-ai-briefing.md",
]
_DEFAULT_WORD = re.compile(r"default|standard", re.IGNORECASE)


def check_ai_default() -> None:
    cfg = open("backend/internal/config/config.go").read()
    m = re.search(r'getEnv\("VAKT_AI_MODEL",\s*"([^"]+)"\)', cfg)
    if not m:
        err("config.go: VAKT_AI_MODEL-Default nicht gefunden")
        return
    canon = m.group(1)

    # 2a. .env.example: exakter Default-Abgleich gegen config.go.
    env = open(".env.example").read()
    em = re.search(r"^VAKT_AI_MODEL=(\S+)", env, re.M)
    if not em:
        err(".env.example: VAKT_AI_MODEL nicht gesetzt")
    elif em.group(1) != canon:
        err(f".env.example: VAKT_AI_MODEL={em.group(1)} ≠ config.go-Default ({canon})")

    # 2b. Kanonische Docs: jede Zeile, die ein Modell derselben Familie *als
    #     Default* nennt, MUSS den config.go-Default enthalten. Aus dem Code
    #     abgeleitet (Familie = Teil vor ':'), daher null Fehlalarm bei
    #     Alternativ-Modellen wie qwen2.5:7b in Upgrade-Hinweisen.
    family = re.escape(canon.split(":")[0])  # z.B. "qwen2\\.5"
    tag_re = re.compile(family + r":[\w.]+", re.IGNORECASE)
    for f in MODEL_DOCS:
        if not os.path.exists(f):
            continue
        for lineno, line in enumerate(open(f, encoding="utf-8", errors="ignore"), 1):
            if not _DEFAULT_WORD.search(line):
                continue
            if tag_re.search(line) and canon not in line:
                err(
                    f"{f}:{lineno}: nennt ein Modell als Default, aber nicht den "
                    f"config.go-Default ({canon}): {line.strip()[:90]}"
                )


# ── 3. Interne .md-Links ─────────────────────────────────────────────────────
_LINK = re.compile(r"\]\(([^)#?]+\.md)(?:#[^)]*)?\)")
_EXCL = ("public-mirror/", ".beta-analyse/", "outputs/", "docs/history/", ".forgehive/")


def check_links() -> None:
    tracked = subprocess.run(
        ["git", "ls-files", "*.md"], capture_output=True, text=True
    ).stdout.split()
    for f in tracked:
        if any(x in f for x in _EXCL):
            continue
        base = os.path.dirname(f)
        for lineno, line in enumerate(open(f, encoding="utf-8", errors="ignore"), 1):
            for mm in _LINK.finditer(line):
                target = mm.group(1)
                if target.startswith(("http://", "https://", "mailto:")):
                    continue
                if not os.path.exists(os.path.normpath(os.path.join(base, target))):
                    err(f"{f}:{lineno}: kaputter interner Link → {target}")


def main() -> int:
    check_go_version()
    check_ai_default()
    check_links()
    if errors:
        print("Doku-Drift gefunden:\n")
        for e in errors:
            print("  ❌", e)
        print(f"\n{len(errors)} Problem(e). Quelle der Wahrheit ist der Code (go.mod/config.go).")
        return 1
    print("✓ Doku-Konsistenz OK (Go-Version, AI-Default, interne Links)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
