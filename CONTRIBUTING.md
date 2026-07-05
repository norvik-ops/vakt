# Contributing to Vakt

Danke fürs Interesse, zu Vakt beizutragen. Diese Datei beschreibt, wie Issues, Pull Requests und Sicherheitsmeldungen funktionieren.

## Vorab — Repository-Topologie

Dieses Repository (`norvik-ops/vakt`) ist ein **kuratierter Mirror**. Die Entwicklung findet stromaufwärts in einem Mono-Repo statt, von dem aus per CI hierher synchronisiert wird. **Direkte PRs auf diesen Mirror werden beim nächsten Sync überschrieben.**

So trägst du bei:

- **Bug oder Feature-Wunsch:** Öffne ein [Issue](https://github.com/norvik-ops/vakt/issues). Wir übernehmen es stromaufwärts und pflegen den Fix zurück in den Mirror.
- **Code-Vorschlag:** Öffne trotzdem gern einen PR als Referenz — wir übernehmen die Änderung stromaufwärts (mit Credit) statt sie direkt zu mergen.

## Branch-Strategie

- `main` — produktiv, jeder Commit ist potenziell releasebar
- Feature-Branches: `feat/<sprint>-<kurzbeschreibung>` (z.B. `feat/s13-ssrf-guard`)
- Bugfix-Branches: `fix/<modul>-<kurzbeschreibung>` (z.B. `fix/auth-oidc-state`)
- Hotfix-Branches: `hotfix/v<version>-<kurzbeschreibung>` mit klarem Scope (< 50 Zeilen Diff)

Branches werden über Pull Request gemerged, nicht direkt nach `main` gepusht.

## Commit-Style

Conventional Commits sind keine Pflicht, aber empfohlen. Beispiele aus der jüngeren Historie:

- `fix(auth): /auth/login Response enthält user-Objekt`
- `fix(api): /health gibt demo, sso_enabled, version zurück`
- `docs: ADR-0017 + Maintainer-Checkliste gegen Backend↔Frontend Drift`
- `feat(s13): SSRF-Guard für VAKT_AI_BASE_URL`

Subject ≤ 72 Zeichen, optional ein Body mit dem **Warum**. Das **Was** steht im Diff — schreib in den Body, was ein zukünftiger Maintainer in 6 Monaten wissen muss, das aus dem Code allein nicht klar wird.

## Test-Erwartung

- **Service-Layer**: für jede neue Funktion mit Domain-Invariante einen Test (siehe `docs/dev/service-pattern.md` und ADR-0012 für die risikobasierte Coverage-Strategie).
- **Handler**: ein Happy-Path + ein Validation-Fail.
- **Frontend**: kritische Flows pro Modul mit Playwright (siehe `frontend/e2e/`). Reine View-Komponenten brauchen keine Unit-Tests.
- **CI-Gate**: `make lint` + `go test ./...` + `npm run build` müssen grün sein, bevor du Review anforderst.

Eine 80-%-Coverage-Quote ist **nicht** unsere Regel (siehe ADR-0012). Risiko- und Schichten-basiertes Testing geht vor Quoten-Erfüllung.

## ADR-Prozess

Architekturentscheidungen, die mehrere Module betreffen oder schwer reversibel sind, kommen in `docs/adr/`:

1. Kopier `docs/adr/0000-template.md` als `docs/adr/NNNN-kurz-titel.md`.
2. Schreib **Kontext**, **Entscheidung**, **Alternativen**, **Konsequenzen** sauber.
3. Status startet auf `Proposed`. Sobald du die ADR als Teil deines PR mergst, hebst du sie auf `Accepted`.
4. Trag die ADR in `docs/adr/README.md` ein.

Faustregel: wenn die Antwort auf "warum machen wir das so?" länger als ein Absatz wird, schreib eine ADR.

## Pull-Request-Workflow

1. Branch erstellen, Patch schreiben, Tests schreiben.
2. `make lint` lokal grün.
3. PR-Template ausfüllen — was, warum, getestet wie.
4. Mindestens ein Review-Approval erforderlich.
5. CI muss grün sein (Go-Build + Go-Test + Frontend-Build + Trivy + gitleaks).
6. Squash-Merge ist Standard. Branch wird nach Merge gelöscht.

## Pflicht-Checks vor jedem Release-Tag

Vor `git tag v*` läuft der Smoke-Test aus `docs/dev/api-contract-checklist.md` durch:

```bash
curl -s http://localhost/health | jq '.demo, .sso_enabled, .version'
DEMO=$(curl -sX POST http://localhost/api/v1/demo/start)
EMAIL=$(echo "$DEMO" | jq -r '.admin_email'); PASS=$(echo "$DEMO" | jq -r '.admin_password')
curl -sX POST http://localhost/api/v1/auth/login -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq '.user | keys'
```

Wenn ein Step fehlschlägt: **nicht taggen**, erst fixen. Siehe ADR-0017 für die langfristige Contract-Test-Strategie.

## Sicherheitslücken melden

Bitte **keine öffentlichen Issues** für Sicherheitslücken. Stattdessen:

- E-Mail an `security@norvikops.de` mit Beschreibung + Reproduktion + ggf. PoC.
- Wir antworten innerhalb von 5 Werktagen.
- Verantwortliche Offenlegung: 90 Tage Frist nach Bestätigung, früher in Abstimmung.

## Verhaltenskodex

Sei höflich, sei genau, sei auf den Code fokussiert. Kein Whataboutism, keine Personenangriffe, keine Diskussion über Politik im Issue-Tracker. Maintainer behalten sich vor, Beiträge oder Kommentare ohne Begründung zu löschen.

## Was wir NICHT annehmen

- **MSP-Portal-Features** mit zentraler Multi-Tenancy — siehe [ADR-0008](docs/adr/0008-kein-msp-portal.md).
- **Phone-Home-Telemetrie** ohne explizites Opt-In — siehe [ADR-0001](docs/adr/0001-self-hosted-no-phone-home.md).
- **Cloud-SaaS-Integrationen, die Kundendaten an Dritte senden** (Jira Cloud, Atlassian, etc.) — siehe Jira-Removal in v0.5.2 + DSGVO-Block in CLAUDE.md.
- **CDN-Abhängigkeiten** für Frontend-Assets — die Plattform muss vollständig self-hosted laufen.

Wenn du unsicher bist, ob deine Idee ins Produkt passt: öffne ein Issue mit dem Label `discussion` bevor du Code schreibst.

## Lizenz

Beiträge stehen unter der [Elastic License v2](LICENSE). Kommerzielle Weiterverkäufe als Managed-Service sind nicht gestattet — Self-Hosted-Nutzung ist frei.

Danke!
