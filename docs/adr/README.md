# Architecture Decision Records

Diese ADRs dokumentieren wichtige Architekturentscheidungen der Vakt-Plattform. Format: Michael Nygard's "Architecture Decision Records" (kurz, ein File pro Entscheidung, fortlaufend nummeriert, **immutable** nach Akzeptanz — Änderungen kommen als neue ADRs, die alte ADRs „supersede").

## Status-Lifecycle

- **Proposed** — vorgeschlagen, noch nicht entschieden
- **Accepted** — entschieden und in Umsetzung
- **Superseded by ADR-NNNN** — durch neuere Entscheidung ersetzt; Datei bleibt für die Historie

## Index

| # | Titel | Status |
|---|-------|--------|
| [0001](0001-self-hosted-no-phone-home.md) | Self-Hosted Architektur ohne Phone-Home | Accepted |
| [0002](0002-elastic-license-v2.md) | Elastic License v2 als Lizenzmodell | Accepted |
| [0003](0003-paseto-v4-statt-jwt.md) | Paseto V4 statt JWT für Authentifizierung | Accepted |
| [0004](0004-modul-isolation-via-package-und-prefix.md) | Modul-Isolation via Go-Package und DB-Prefix | Accepted |
| [0005](0005-sqlc-modulweise-keine-vollmigration.md) | sqlc modulweise einführen, keine Vollmigration | Accepted |
| [0006](0006-anonymisierung-statt-hard-delete.md) | Anonymisierung statt Hard-Delete bei DSGVO Art. 17 | Accepted |
| [0007](0007-betriebsrat-mode-write-time-anonymisierung.md) | Betriebsrat-Modus: Anonymisierung beim Schreiben | Accepted |
| [0008](0008-kein-msp-portal.md) | Kein MSP-Portal — Phone-Home-Verstoß | Accepted |
| [0009](0009-openapi-single-source-of-truth.md) | OpenAPI-Spec als Single Source of Truth, embedded | Accepted |
| [0010](0010-aes-256-gcm-fuer-app-secrets.md) | AES-256-GCM für Application-Level-Secrets | Accepted |
| [0011](0011-opentelemetry-optional-opt-in.md) | OpenTelemetry als opt-in, nicht als Pflicht | Accepted |
| [0012](0012-frontend-test-coverage-pragmatisch.md) | Test-Coverage pragmatisch nach Risiko, nicht nach Quote | Accepted |
| [0013](0013-sqlc-row-types-und-feld-mapper.md) | sqlc Row-Types und Feld-Mapper bei divergenter Spalten-Reihenfolge | Accepted |
| [0014](0014-ai-copilot-community-feature.md) | AI Copilot ist Community-Feature, kein Pro-Gate | Accepted |
| [0015](0015-ephemere-demo-sessions.md) | Ephemere Demo-Sessions pro Visitor (4 h Lebensdauer, Random-Credentials) | Accepted |
| [0016](0016-public-mirror-via-script.md) | Public Mirror per Script statt inline rsync im CI; Compile-Check als Gate | Accepted |
| [0017](0017-api-contract-tests.md) | API-Contract-Tests gegen Backend ↔ Frontend Drift (OpenAPI durchsetzen) | Accepted |
| [0018](0018-goroutine-lifecycle-und-panic-eskalation.md) | Goroutine-Lifecycle (Parent-Context-Pflicht) und Panic-Eskalation via `safego.Run` | Accepted |
| [0019](0019-sse-statt-websocket-fuer-realtime.md) | Server-Sent Events statt WebSockets für alle Realtime-Pfade (Notifications, Scan-Progress, AI-Streaming) | Accepted |
| [0020](0020-ai-agent-tool-permissions.md) | AI-Agent darf nur Tools nutzen, die der initiierende User auch darf — keine Privilege-Escalation via AI | Accepted |

## Wann eine neue ADR schreiben?

Für Entscheidungen die:

- mehrere Module betreffen
- schwer reversibel sind (Datenbank-Schema, externe Verträge, Lizenzmodell)
- eine erkennbare Alternative hatten (sonst dokumentier es als Code-Kommentar)
- für künftige Entwickler nicht aus dem Code allein erschließbar sind

Faustregel: Wenn du einem neuen Entwickler erklärt, „warum machen wir das so?" und die Antwort länger als ein Absatz ist — schreib ein ADR.

## Template

Siehe [`0000-template.md`](0000-template.md).
