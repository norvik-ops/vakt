# ADR-0016: Public Mirror per Script statt inline rsync im CI

**Status:** Accepted
**Datum:** 2026-05-20
**Entscheider:** Stefan Moseler

## Kontext

Vakt liegt in zwei Repos: `Matharnica/vakt-app` (privat, Mono-Repo) und `norvik-ops/vatk` (public, kuratierter Mirror). Bis v0.6.1 wurde der Mirror via `.github/workflows/sync-public-repo.yml` mit Inline-rsync-Aufrufen + Exclude-Listen gebaut. Diese Logik war:

- **unsichtbar lokal** — was rausgeht, sieht man nur indem man das Public Repo klont und vergleicht
- **fehleranfällig** — wenn ein neuer Import in `cmd/api/main.go` hinzukommt, der ein excluded Package referenziert, scheitert der Public Repo Build. Erst nachdem es 4 Wochen nicht aufgefallen ist (siehe v0.6.2-CHANGELOG: `internal/shared/demo`, `demoseed`, `feedback` waren excluded, aber `cmd/api/main.go` importierte sie → Public Repo kompilierte nicht)
- **doppelte Wartung** — der gleiche Code (Filter-Regeln, Exclude-Listen) hätte sowohl lokal als auch in CI ablaufbar sein müssen, war aber nur in CI

## Entscheidung

Wir extrahieren die Mirror-Build-Logik in `scripts/build-public-mirror.sh`. Der Script ist die **single source of truth** dafür, was ins Public Repo geht:

1. Lokal: `make public-mirror` baut den vollständigen Mirror-Inhalt in `./public-mirror/` (gitignored). Entwickler können `tree`, `diff`, `du`, `(cd public-mirror/backend && go build ./...)` darauf laufen lassen.
2. CI: `sync-public-repo.yml` ruft denselben Script auf und rsync't das Ergebnis nach `norvik-ops/vatk`.
3. Der Script bricht mit Exit 1 ab, wenn das Mirror-Backend nicht kompiliert — verhindert das v0.6.1-Pattern, in dem sich solche Bugs Wochen verstecken konnten.

## Alternativen

- **Inline-rsync im Workflow** (vorheriger Zustand) — verworfen weil unsichtbar lokal und doppelt zu warten.
- **Build-Tags** (`//go:build community`) — verworfen, weil das Pattern sich quer durch viele Files ziehen müsste (jedes Package, das vom Public ausgenommen ist, bräuchte Tags); Go-Code wird unleserlich. Außerdem hilft es nicht bei Doc-/Asset-Excludes.
- **Committed `public-mirror/` Branch** — verworfen weil Git-History-Pollution und unklares Mergen.
- **Symlinks** — verworfen wegen Windows-Inkompatibilität und rsync-Semantik (Follow-Symlink schreibt Inhalt doppelt, no-Follow bricht Public Repo).
- **Separates Public-Submodul** — verworfen weil die Code-Trennung an Modul-Grenzen müsste; aktuell sind Demo/Feedback nicht modular getrennt.

## Konsequenzen

### Positive

- **Lokal verifizierbar.** `make public-mirror` zeigt sofort, was ins Public Repo geht. `tree public-mirror/`, `diff -ruN public-mirror/ ../some-vatk-clone/`, `du -sh public-mirror/` alle möglich.
- **Compile-Check eingebaut.** Wenn `cmd/api/main.go` ein Package importiert, das excluded ist, fällt das beim nächsten `make public-mirror`-Lauf auf, nicht 6 Wochen später.
- **Eine Quelle, zwei Konsumenten.** Lokales Build-Tool und CI-Job nutzen dasselbe Script.
- **Exclude-Liste klein und dokumentiert.** Aktuell genau ein Eintrag (`license/generator/`, eigenes `package main`). Frühere Excludes für `shared/demo`, `demoseed`, `feedback`, `admin/staging_handler.go` sind weg — siehe „Refactor: staging_handler" unten.
- **CHANGELOG-Diff lesbarer.** Sync-Commits im Public Repo enthalten genau das, was sich am Mirror geändert hat, ohne Rauschen von Reorderings.

### Negative

- **CI braucht `go` installiert** für den Compile-Check. Vorher kam man ohne Go-Setup aus. `actions/setup-go@v5` ist trivial, aber zusätzlicher Step.
- **`public-mirror/` braucht ~11 MB Platz lokal** wenn man es baut. Wird durch `.gitignore` nicht committet.
- **rsync-Logik fließt zwischen Script und Workflow.** Workflow ist nun einfacher (~20 Zeilen statt ~80), aber das Script muss korrekt sein. Acceptable tradeoff.

### Neutrale

- Der Script kann später um zusätzliche Verifikationen erweitert werden: `go vet`, gofmt-Check, frontend `npm run build` im Mirror. Aktuell nur `go build ./...` als Minimum.
- ADR-0014, ADR-0015 (AI Community, Ephemeral Demo) sind durch diesen Mechanismus stabilisiert: die zugehörigen Demo-/AI-Packages sind im Public Repo verfügbar, weil sie nicht mehr exkludiert sind.

## Implementierungs-Details

### Was vom Mirror exkludiert wird

Aktuell nur:

- `backend/internal/license/generator/` — eigenes `package main`, von keinem importiert, NorvikOps-internes Tool

Alles andere geht ins Public. Insbesondere:

- `internal/shared/demo/`, `demoseed/`, `feedback/` — aktiv via `if cfg.DemoSeed` gegated, in Customer-Default-Installs inaktiv
- `internal/admin/staging_handler.go` (inkl. `RegisterStaging`) — aktiv via `if cfg.Staging` gegated; das Public-Build-Problem wurde durch Konsolidierung gelöst: `RegisterStaging` wohnt jetzt in `staging_handler.go` (war vorher in `routes.go`)

### .env.example-Filter

Interne Env-Vars werden mit `grep -v -E` ausgeschlossen: `VAKT_STAGING`, `VAKT_PROMOTE`, `VAKT_DEMO`, `VAKT_SERVER_TOKEN`. Wenn neue interne Vars dazukommen, muss der Filter erweitert werden.

### Doku-Scope

Nur User-relevante Docs sind im Mirror: `docs/setup.md`, `docs/configuration.md`, `docs/SECURITY-ASSESSMENT.md`, `docs/wiki/`, `docs/modules/`, `docs/adr/` (inkl. dieser Datei). Sprint-Changelogs, Marketing-Drafts, `docs/dev/` und `CLAUDE.md` bleiben privat.

## Referenzen

- Script: `scripts/build-public-mirror.sh`
- Workflow: `.github/workflows/sync-public-repo.yml`
- Trigger-Bug v0.6.1: siehe CHANGELOG.md `[v0.6.2]` Abschnitt „Behoben"
