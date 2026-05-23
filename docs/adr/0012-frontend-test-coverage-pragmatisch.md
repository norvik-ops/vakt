# ADR-0012: Test-Coverage pragmatisch nach Risiko, nicht nach Quote

**Status:** Accepted  
**Datum:** 2026-05-20

## Kontext

Die externe Code-Analyse markierte die Frontend-Test-Coverage als „kritisch" (29 Test-Files für 57k LOC ≈ 0.5%). Übliche Reaktionen wäre ein „CI-Gate auf 60% Coverage" — was in der Vakt-Realität problematisch wäre:

- Coverage-% messen, ohne welche Tests gut sind, führt zu Junk-Tests („covered but useless").
- Das Frontend hat viele Layout-Komponenten (skeleton loaders, leere Listen, Headers) — die zu testen treibt die Zahl, sagt aber nichts über Korrektheit.
- Critical-Path-Tests fehlen tatsächlich (Login, Control-Status-Änderung, Incident-Erstellung).

## Entscheidung

**Coverage-Quote ist kein CI-Gate.** Stattdessen:

1. **Risiko-priorisierte Test-Liste**: Pro Modul sind die kritischen Flows explizit benannt (siehe unten) und müssen Tests haben.
2. **A11y-Tests** (`vitest-axe`, `axe-playwright`) sind Pflicht für UI-Komponenten, nicht Pages.
3. **E2E-Tests** (Playwright) decken die Top-10 User-Journeys ab — CI läuft auf **Chromium, Firefox und WebKit** (seit 2026-05-23).
4. **Service-Layer-Tests** im Backend haben Vorrang vor UI-Tests bei der Aufwandsallokation — Business-Logik ist dort.

### Kritische Flows mit Test-Pflicht

| Modul | Flow | Test-Typ |
|-------|------|----------|
| Auth | Login, Logout, Sessions-Widerruf, TOTP-Setup | E2E + Unit |
| Vakt Comply | Control-Status-Änderung, Evidence-Upload, Framework-Anlage | E2E + Service-Test |
| Vakt Scan | Finding-Import, Status-Wechsel | E2E + Service-Test |
| Vakt Privacy | DSR-Anlage, DSR-Status-Wechsel | Service-Test |
| Vakt HR | Onboarding-Flow, Offboarding-Flow, Step-Completion | Service-Test (Sprint 1 ✓) |
| Account | Data-Export, Account-Delete | Handler-Test (Sprint 7 ✓) |

## Alternativen

- **CI-Gate 60%** (externe Analyse-Vorschlag) — verworfen: produziert Junk-Tests, blockiert valides Refactoring.
- **CI-Gate 60% nur für `src/api` und `src/modules/*/hooks`** — erwogen: produktiver, aber engineering-getrieben („was leicht zu messen ist") statt risikobasiert.
- **Mutation Testing** (Stryker für TS) — erwogen aufzubauen, aber Kosten/Nutzen unklar bei aktueller Codebase-Größe.

## Konsequenzen

### Positive

- Test-Aufwand fließt dort hin, wo Bugs am teuersten wären.
- Service-Tests haben sehr hohen ROI (eine Test-Datei deckt 20+ Bug-Klassen ab).
- Tests bleiben les- und wartbar — keine Coverage-Junk-Tests.

### Negative

- Schwerer zu kommunizieren („wir testen mehr" als Zahl) gegenüber „60% Coverage".
- Erfordert Disziplin im Code-Review: „ist das ein kritischer Flow? Dann braucht es einen Test."

### Neutrale

- Externe Auditoren akzeptieren in der Regel „Risk-based Testing" als ISO-27001-konformes Vorgehen — explizit benannte kritische Pfade > nackte %-Zahlen.

## Referenzen

- `frontend/src/**/*.test.tsx` — bestehende Tests
- `frontend/e2e/**/*.spec.ts` — E2E-Coverage
- `backend/internal/shared/account/account_test.go` — Service-Test-Pattern
- ADR-0005 (sqlc inkrementell — gleiche Pragmatik)
