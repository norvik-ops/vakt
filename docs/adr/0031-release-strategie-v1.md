# ADR-0031: Phase-1-Release-Strategie (v0.22.0 → v1.0)

**Status:** Akzeptiert (Amendment 2026-05-23: Pre-v1.0-Scope auf 4 Sprints erweitert)
**Datum:** 2026-05-23
**Entscheider:** Stefan Moseler

## Amendment 2026-05-23: v1.0 muss genuinely good sein

Die ursprüngliche Entscheidung sah einen einzigen Pre-v1.0-Sprint (v0.23.0) vor. Nach Diskussion wurde das als zu dünn erkannt: First impressions in Open Source sind permanent. Ein v1.0 mit fehlenden Metriken, inkonsistenten UX-Zuständen oder veralteter Dokumentation ist kein "wir bessern nach" — es ist eine Absage an Nutzer die keine zweite Chance geben.

**Geänderte Entscheidung:** Pre-v1.0-Scope wird auf 4 Sprints (S45–S48) ausgeweitet:

| Sprint | Fokus | Milestone |
|--------|-------|-----------|
| S45 | Infra-Hygiene: sechealth→vakt, SBOM, cosign, lint-Gates | v0.23.0 |
| S46 | Observability: `/metrics`, Startup-Diagnostics, Graceful-Shutdown-Test, Runbook | — |
| S47 | UX-Qualität: Empty States, Onboarding-Wizard, Error-Messages, Mobile | — |
| S48 | Trust+Docs: README, Getting-Started, Operator-Runbook, ADR-Index, SECURITY.md | **→ v1.0.0** |

Die 9 Quality-Gates aus der ursprünglichen Entscheidung bleiben gültig und werden durch Sprint-48-Akzeptanzkriterien erweitert. Details in `.forgehive/PRODUKTREIFE-BACKLOG.md` Sprints 45–48.

## Kontext

Mit v0.22.0 (Supplier Portal + Vakt Scan Substantiierung) sind alle 44 geplanten Sprints
des initialen Backlogs abgeschlossen. 163 Features aus dem Produktreife-Backlog wurden
geliefert; der Sprint-Backlog zeigt 228 Items als erledigt.

Die Frage lautet: **Was passiert jetzt zwischen v0.22.0 und v1.0?**

Ohne explizite Entscheidung besteht das Risiko, entweder:
1. v1.0 vorschnell zu taggen, bevor Release-Qualitätsgates erfüllt sind, oder
2. v1.0 indefinit zu verschieben, weil immer neue Nice-to-Haves auftauchen.

Zusätzlich hat ein externes Review-Board (Mai 2026) vier konkrete Lücken identifiziert,
die vor einem öffentlichen v1.0-Start geschlossen sein müssen:

- Enterprise-Auth Frontend nicht vollständig (API-Key-Scope-UI, SessionsPage mit
  Confirm-Dialog, Login-History-Section)
- i18n-Datumsformatierung inkonsistent (62 raw date-Calls ignorieren die gewählte Sprache)
- Kein strukturiertes Brand-System dokumentiert
- Doku-Vollständigkeit für Self-Hosted-Nutzer nicht gegeben (MSP-Onboarding fehlt,
  Launch-Checkliste fehlt)

Pen-Test (zurückgestellt wegen Budget), ISO 27001-Zertifizierung (geplant 2027) und
MSP-Portal (dauerhaft ausgeschlossen per ADR-0008) sind explizit kein v1.0-Blocker.

## Entscheidung

Wir folgen einer zweistufigen Strategie:

### Stufe 1: v0.23.0 — Release-Readiness-Sprint (heute)

v0.23.0 schließt alle vier identifizierten Lücken:

1. **Enterprise-Auth Frontend** (S20-3, S20-5, S20-7): ApiKeys-Audit-Trail-Verweis,
   SessionsPage mit Confirm-Dialog für Session-Widerruf, Login-History-Section in
   Account-Settings.
2. **i18n-Datumsformatierung** (S13-27): Bulk-Migration aller 62 raw date-Calls auf
   `useFormatDate`; `shared/utils/date.ts` auf `navigator.language` umgestellt.
3. **Dokumentation**: MSP-Onboarding-Guide (`docs/wiki/msp-onboarding.md`),
   interne Launch-Checkliste (9 Gates), ADR-0030 + ADR-0031.
4. **CHANGELOG** aktualisiert, Backlog-Status-Marker auf `[x]` gesetzt.

### Stufe 2: v1.0.0 — Public Launch

v1.0.0 wird getaggt, wenn alle folgenden Quality-Gates grün sind:

| Gate | Kriterium |
|------|-----------|
| Code Quality | TSC 0 errors, ESLint 0 errors, `go build` + `go test ./...` + `golangci-lint` grün |
| API Smoke-Test | `/health`-Felder (demo, sso_enabled, version, status), Demo-Flow, Login-Response |
| Security | CSRF-Headers, MFA-Enforcement, Rate-Limits, kein Secret in Git |
| Doku | Wiki vollständig, API-Reference aktuell, CHANGELOG aktuell |
| Demo-Instanz | Ephemerer Flow funktioniert, 4h-Cleanup-Job läuft |
| Infrastruktur | Staging healthy, HTTPS-Cert gültig |
| Marketing | Landing Page live, Product Hunt Draft bereit |
| Legal | ELv2 in Repo, DSGVO-konformer Demo-Betrieb bestätigt |

### Public-Launch-Sequenz

1. Self-Hosted Demo auf `secdemo.norvikops.de` — bereits aktiv
2. **Product Hunt Launch** + **Hacker News: Show HN** — gleichzeitig, ~09:00 ET am
   Launch-Tag (optimales Engagement-Fenster)
3. GitHub-Star-Kampagne via bestehende DACH-Community-Kontakte
4. Optional: LinkedIn-Beitrag, BSI-Forum-Post

### Explizit kein v1.0-Blocker

- **Pen-Test** — zurückgestellt auf post-v1.0 (Budget-Constraint). Wird als
  bekanntes Gap in Security-Assessment-Docs transparent gemacht.
- **ISO 27001-Zertifizierung** — geplant 2027. Vakt dokumentiert ISO-27001-Compliance
  der Kunden, ist aber selbst (noch) nicht zertifiziert.
- **MSP-Portal** — dauerhaft ausgeschlossen (ADR-0008). MSPs deployen pro Kunde.
- **Weitere Feature-Sprints** — alle Feature-Requests nach v0.22.0 werden in einem
  separaten Post-v1.0-Backlog gesammelt und nach dem Launch priorisiert.

## Alternativen

- **Direkt v1.0.0 taggen (heute)** — verworfen. Die vier identifizierten Lücken sind
  reputationsrelevant: Nicht-DE-Nutzer sehen falsche Datumsformate; Enterprise-Auth-UI
  fehlt auf Seiten, die bereits im Changelog als "implementiert" stehen. Ein v1.0 mit
  diesen Lücken schwächt das Vertrauen beim ersten Eindruck.

- **v1.0 nach Pen-Test** — verworfen. Pen-Test ist eine Budget-Entscheidung, keine
  Produkt-Reifeentscheidung. v1.0 kommuniziert Feature-Completeness, nicht Security-
  Zertifizierung. Das SECURITY-ASSESSMENT.md dokumentiert den Status transparent.

- **v0.9.0 + v0.10.0 als Zwischenstufen** — verworfen. Aus Marketing-Sicht ist
  "v1.0" der relevante Meilenstein. Weitere 0.x-Stufen strecken die Kommunikation
  ohne Mehrwert für Nutzer. v0.23.0 als letzter Prep-Release, dann v1.0.

- **Feature-Freeze auf v0.22.0, nur Bug-Fixes bis v1.0** — verworfen. Die vier
  Lücken sind keine Bugs, sondern Fertigstellungs-Items die zum Commit-Stand in
  v0.22.0 noch nicht vollständig implementiert waren.

## Konsequenzen

### Positive

- **Klare v1.0-Definition** — das Team und externe Beobachter wissen, was v1.0 bedeutet.
  Kein vager "wenn es gut genug ist"-Zustand.
- **Reputationsschutz** — Lücken in Enterprise-Auth-UI und i18n werden vor dem
  öffentlichen Launch behoben; kein erster Eindruck mit offensichtlichen Inkonsistenzen.
- **Launch-Checkliste als Qualitätsgate** — die 9 Quality-Gates aus dieser ADR
  sind wiederverwendbar für alle zukünftigen Major-Releases.
- **Pen-Test-Verschiebung explizit** — die Entscheidung ist dokumentiert, nicht still
  vergessen. SECURITY-ASSESSMENT.md reflektiert den Status; Kunden können informiert
  entscheiden.

### Negative

- **Verzögerung** — v1.0 kommt nicht heute, sondern nach dem v0.23.0-Release-Readiness-
  Sprint. Bei einem einzelnen Entwickler beträgt das ~1–2 Tage, bei laufender Infrastruktur.
- **Launch-Checkliste als Bremse** — wenn ein Gate unerwartet rot ist (z.B. golangci-lint
  meldet neuen Fehler), kann das den Launch verzögern. Das ist gewollt, muss aber als
  Risiko kommuniziert werden.

### Neutrale

- v0.23.0 ist kein Feature-Release, sondern ein Reife-Release. CHANGELOG und Release-Notes
  kommunizieren das entsprechend.
- Post-v1.0-Planung (weitere Module, MSP-Tier-Pricing, Enterprise-Support-Contracts)
  beginnt nach dem Launch und wird in einem separaten ADR dokumentiert.

## Referenzen

- MSP-Portal-Ausschluss: ADR-0008
- Security-Assessment: `docs/SECURITY-ASSESSMENT.md`
- DSGVO Demo-Betrieb: ADR-0015 (Ephemere Demo-Sessions)
- i18n-Datumsformatierung: ADR-0030
