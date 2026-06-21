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
_EXCL = ("public-mirror/", ".beta-analyse/", "outputs/", "docs/history/", ".forgehive/", ".claude/")


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


# ── 4. Env-Var-Coverage ──────────────────────────────────────────────────────
# Zwei Invarianten, beide mit dem Code als Quelle der Wahrheit:
#   (A) Referenz-Vollständigkeit: jede Variable in .env.example MUSS in der
#       kanonischen User-Referenz docs/wiki/configuration.md stehen — so
#       verschwindet keine dokumentierte Variable beim Konsolidieren/Stubben
#       (Auslöser: VAKT_DEMO 2026-06-14). Anderswo-dokumentierte/Ops-Vars via
#       ENV_DOC_EXEMPT.
#   (B) Code-Coverage: jede Env-Var, die irgendwo in backend/** gelesen wird
#       (os.Getenv/getEnv*, ohne Tests), MUSS in irgendeiner echten Referenz-
#       Doku (docs/** außer internen Verzeichnissen) ODER .env.example
#       dokumentiert sein — so bleibt keine Code-Config-Var unauffindbar
#       (Auslöser: VAKT_EPSS_ENABLED, VAKT_SLO_* …). Interne/Dev/PoC-Vars via
#       CODE_VAR_EXEMPT.
#   (C) Frontend: jede import.meta.env.VITE_*-Lesestelle in frontend/src MUSS
#       ebenfalls dokumentiert sein (Vite-Built-ins DEV/PROD/MODE sind nicht
#       VITE_-präfixiert und damit ausgenommen).
#   (D) Deployment: jede ${VAR}-Referenz in docker-compose*.yml MUSS dokumentiert
#       sein (Self-Host-Surface). Helm-Templates bleiben bewusst außen vor.
CONFIG_REF = "docs/wiki/configuration.md"
_ENV_ASSIGN = re.compile(r"^\s*#?\s*([A-Z][A-Z0-9_]{2,})=")
# Alle Env-Read-Helper im Backend (os.Getenv + die config-Helper getEnv*,
# mustEnv, readEnvOrFile). os.Setenv/Unsetenv sind Writes und matchen nicht.
_ENV_READ = re.compile(
    r'(?:os\.(?:Getenv|LookupEnv)|getEnv\w*|mustEnv|readEnvOrFile)\(\s*"([A-Z][A-Z0-9_]+)"'
)
# Frontend: echte import.meta.env.VITE_*-Lesestellen (Vite-Built-ins wie
# DEV/PROD/MODE sind nicht VITE_-präfixiert und matchen daher nicht).
_VITE_READ = re.compile(r"import\.meta\.env\.(VITE_[A-Z0-9_]+)")
# Helm-Env-Deklarationen: `- name: VAKT_X` (Container-env) und `VAKT_X:` als
# ConfigMap-/values-Key. Kommentar-Erwähnungen (`# VAKT_X …`) matchen NICHT.
_HELM_ENV = re.compile(r'name:\s*"?(VAKT_[A-Z0-9_]+|CASDOOR_[A-Z0-9_]+)')
_HELM_KEY = re.compile(r"^\s*(VAKT_[A-Z0-9_]+|CASDOOR_[A-Z0-9_]+):", re.MULTILINE)
# Verzeichnisse, die KEINE Referenz-Doku sind (Historie/Planung/Analyse).
_DOC_EXCL = (
    "docs/history/", "docs/reviews/", "docs/planning/", "docs/sprints/",
    "docs/stories/", "docs/marketing/", "docs/audit-responses/", "docs/reports/",
)
# (A) In .env.example, aber bewusst NICHT in der zentralen User-Referenz
#     (anderswo dokumentiert oder reine Ops-/CI-/Install-Vars).
ENV_DOC_EXEMPT = {
    "VAKT_TAG", "OLLAMA_TAG", "VAKT_STAGING",    # Docker-Image-Pins / Staging-Ops
    "VAKT_PROMOTE_URL", "VAKT_PROMOTE_SECRET",   # internes Promote-Deploy
    "VAKT_LS_WEBHOOK_SECRET",                    # LemonSqueezy-Payment (intern)
    "VAKT_OPENVAS_URL", "VAKT_OPENVAS_USER", "VAKT_OPENVAS_PASS",  # → scanner-setup.md
    "VAKT_DB_URL_FILE", "VAKT_SECRET_KEY_FILE",  # → ADR-0049 / architecture.md
}
# (B) Im Backend gelesen, aber bewusst nirgends dokumentiert (intern/Dev/PoC).
CODE_VAR_EXEMPT = {
    "SEED_ENV",           # nur cmd/seed (Dev-Seeder)
    "VAKT_GITHUB_TOKEN",  # PoC/Fallback — primäre GitHub-Integration wird in-app (verschlüsselt) konfiguriert
}


def _ref_doc_blob() -> str:
    md = [
        f
        for f in subprocess.run(
            ["git", "ls-files", "docs"], capture_output=True, text=True
        ).stdout.split()
        if f.endswith(".md") and not any(f.startswith(x) for x in _DOC_EXCL)
    ]
    md.append(".env.example")
    return "".join(
        open(f, encoding="utf-8", errors="ignore").read() for f in md if os.path.exists(f)
    )


def check_env_vars() -> None:
    if not os.path.exists(CONFIG_REF):
        err(f"{CONFIG_REF} fehlt — kanonische Config-Referenz nicht gefunden")
        return

    # (A) .env.example ⊆ docs/wiki/configuration.md
    ref = open(CONFIG_REF, encoding="utf-8", errors="ignore").read()
    seen_a: set[str] = set()
    for line in open(".env.example", encoding="utf-8", errors="ignore"):
        m = _ENV_ASSIGN.match(line)
        if not m or m.group(1) in seen_a or m.group(1) in ENV_DOC_EXEMPT:
            continue
        seen_a.add(m.group(1))
        if m.group(1) not in ref:
            err(
                f"{m.group(1)} (.env.example) ist nicht in {CONFIG_REF} dokumentiert "
                f"(dort ergänzen oder in ENV_DOC_EXEMPT eintragen)"
            )

    # (B) backend/** Env-Reads ⊆ irgendeine echte Referenz-Doku
    blob = _ref_doc_blob()
    gofiles = [
        f
        for f in subprocess.run(
            ["git", "ls-files", "backend"], capture_output=True, text=True
        ).stdout.split()
        if f.endswith(".go") and not f.endswith("_test.go")
    ]
    seen_b: set[str] = set()
    for f in gofiles:
        for var in _ENV_READ.findall(open(f, encoding="utf-8", errors="ignore").read()):
            if var in seen_b or var in CODE_VAR_EXEMPT:
                continue
            seen_b.add(var)
            if var not in blob:
                err(
                    f"{var} (gelesen in {f.split('backend/')[-1]}) ist in keiner "
                    f"Referenz-Doku dokumentiert (in docs/** dokumentieren oder in "
                    f"CODE_VAR_EXEMPT eintragen)"
                )

    # (C) frontend/src import.meta.env.VITE_*-Reads ⊆ echte Referenz-Doku
    fefiles = [
        f
        for f in subprocess.run(
            ["git", "ls-files", "frontend/src"], capture_output=True, text=True
        ).stdout.split()
        if f.endswith((".ts", ".tsx", ".js", ".jsx", ".vue"))
    ]
    seen_c: set[str] = set()
    for f in fefiles:
        for var in _VITE_READ.findall(open(f, encoding="utf-8", errors="ignore").read()):
            if var in seen_c or var in CODE_VAR_EXEMPT:
                continue
            seen_c.add(var)
            if var not in blob:
                err(
                    f"{var} (gelesen in {f}) ist in keiner Referenz-Doku dokumentiert "
                    f"(in docs/** dokumentieren oder in CODE_VAR_EXEMPT eintragen)"
                )

    # (D) docker-compose ${VAR}-Referenzen ⊆ echte Referenz-Doku (Deployment-
    #     Surface, das Self-Hoster anfassen). Helm-Templates bleiben außen vor
    #     (K8s-Minderheit, eigenes Templating; OLLAMA_HOST dort = Standard-Wiring).
    compose = "".join(
        open(f, encoding="utf-8", errors="ignore").read()
        for f in subprocess.run(
            ["git", "ls-files", "docker-compose*.yml"], capture_output=True, text=True
        ).stdout.split()
        if os.path.exists(f)
    )
    seen_d: set[str] = set()
    for var in re.findall(r"\$\{([A-Z][A-Z0-9_]+)", compose):
        if var in seen_d or var in ENV_DOC_EXEMPT or var in CODE_VAR_EXEMPT:
            continue
        seen_d.add(var)
        if var not in blob:
            err(
                f"{var} (referenziert in docker-compose) ist in keiner Referenz-Doku "
                f"dokumentiert (in docs/** / .env.example dokumentieren oder exempten)"
            )

    # (E) Helm-deklarierte VAKT_*/CASDOOR_*-Env-Vars MÜSSEN im Backend-Code
    #     gelesen werden — fängt tote Config (Helm setzt eine Var, die die App
    #     ignoriert; Auslöser: VAKT_LOG_LEVEL 2026-06-14).
    backend_read: set[str] = set()
    for f in gofiles:
        backend_read.update(_ENV_READ.findall(open(f, encoding="utf-8", errors="ignore").read()))
    helm_decl: set[str] = set()
    for f in subprocess.run(
        ["git", "ls-files", "helm"], capture_output=True, text=True
    ).stdout.split():
        if not f.endswith((".yaml", ".yml", ".tpl")):
            continue
        content = open(f, encoding="utf-8", errors="ignore").read()
        helm_decl.update(_HELM_ENV.findall(content))
        helm_decl.update(_HELM_KEY.findall(content))
    for var in sorted(helm_decl):
        if var in CODE_VAR_EXEMPT or var in backend_read:
            continue
        err(
            f"{var} (in helm/ deklariert) wird nirgends im Backend-Code gelesen — "
            f"tote Config (im Code lesen oder aus helm/ entfernen)"
        )


# ── 5. Volume/Path-Drift ─────────────────────────────────────────────────────
# Fängt zwei Drift-Klassen rund um Docker-Volumes:
#   (a) Volumes in docker-compose.yml, die Backup-Daten enthalten, MÜSSEN in
#       docs/operations/backup-restore.md erwähnt sein — verhindert Datenverlust
#       beim Restore nach einem Volume-Hinzufügen.
#   (b) Stale Host-Pfad `./data/uploads` in Docs — das korrekte Äquivalent ist
#       das Docker-Volume `uploads_data`; ein Host-Pfad existiert im Stack nicht.
_BACKUP_DOC = "docs/operations/backup-restore.md"
# Volumes die kein Volume-Export-Backup brauchen (ephemer, regenerierbar, oder
# über andere Mechanismen gesichert).
_VOLUME_BACKUP_EXEMPT = {
    "postgres_data",   # Inhalt via pg_dump gesichert, nicht als Volume-Export
    "redis_data",      # Session-State — explizit als kein Backup nötig dokumentiert
    "scanner_bins",    # Scanner-Binaries werden beim Start neu geladen
    "frontend_dist",   # Build-Artefakte — werden beim Start regeneriert
    "ollama_models",   # KI-Modell-Download — kein Kundendaten, re-downloadbar
}


def check_volume_backup() -> None:
    compose_path = "docker-compose.yml"
    if not os.path.exists(compose_path):
        return
    compose = open(compose_path, encoding="utf-8", errors="ignore").read()

    # Extract named volumes from the top-level `volumes:` block.
    top_volumes: set[str] = set()
    in_vol_block = False
    for line in compose.splitlines():
        stripped = line.rstrip()
        if re.match(r"^volumes:", stripped):
            in_vol_block = True
            continue
        if in_vol_block:
            # A new top-level key ends the block
            if stripped and not stripped.startswith(" ") and not stripped.startswith("#"):
                in_vol_block = False
                continue
            m = re.match(r"^\s{2}(\w+):", stripped)
            if m:
                top_volumes.add(m.group(1))

    if not os.path.exists(_BACKUP_DOC):
        err(f"{_BACKUP_DOC} fehlt — Backup-Dokumentation nicht gefunden")
        return
    backup_text = open(_BACKUP_DOC, encoding="utf-8", errors="ignore").read()

    for vol in sorted(top_volumes):
        if vol in _VOLUME_BACKUP_EXEMPT:
            continue
        if vol not in backup_text:
            err(
                f"Docker-Volume '{vol}' (docker-compose.yml) ist nicht in "
                f"{_BACKUP_DOC} erwähnt — Backup-Doku nachziehen oder in "
                f"_VOLUME_BACKUP_EXEMPT eintragen"
            )

    # (b) Stale host path references in user-facing docs (not planning/story docs,
    #     which may reference the old path as the problem description).
    stale_path = "./data/uploads"
    doc_files = [
        f
        for f in subprocess.run(
            ["git", "ls-files", "docs"], capture_output=True, text=True
        ).stdout.split()
        if f.endswith(".md") and not any(f.startswith(x) for x in _DOC_EXCL)
    ]
    for f in doc_files:
        text = open(f, encoding="utf-8", errors="ignore").read()
        if stale_path in text:
            err(
                f"{f}: enthält veralteten Host-Pfad '{stale_path}' — "
                f"korrekt ist das Docker-Volume 'uploads_data'"
            )


def main() -> int:
    check_go_version()
    check_ai_default()
    check_links()
    check_env_vars()
    check_volume_backup()
    if errors:
        print("Doku-Drift gefunden:\n")
        for e in errors:
            print("  ❌", e)
        print(f"\n{len(errors)} Problem(e). Quelle der Wahrheit ist der Code (go.mod/config.go).")
        return 1
    print("✓ Doku-Konsistenz OK (Go-Version, AI-Default, interne Links, Env-Var-Coverage, Volume-Backup)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
