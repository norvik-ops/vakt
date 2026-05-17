# Changelog

All notable user-facing changes to Vakt are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### Geplant
- **Jira-Integration** — Findings und offene Controls direkt als Jira-Tickets erstellen (in Planung)

---

## [v0.4.5] — 2026-05-17

### Security
- **Account Lockout** — nach 5 aufeinanderfolgenden Fehlversuchen wird das Konto 15 Minuten gesperrt (gleitendes Fenster, Redis-backed)
- **Session-Invalidierung** — alle aktiven Sessions werden bei Passwort-Reset sofort ungültig (`pw_version`-Claim im Paseto-Token)
- **Content-Security-Policy** — CSP-Header auf allen Antworten (script/style `unsafe-inline` für React SPA, `frame-ancestors 'none'`)

### Added
- **System-Status-Seite** (`/admin/health`) — DB-Latenz, Redis-Latenz, Queue-Tiefe (pending/active/failed), Uptime, Goroutinen, Version; automatische Aktualisierung alle 30 Sekunden
- **License-Ablauf-Banner** — gelbe Warnung ab 30 Tagen vor Ablauf, rote Warnung ab 7 Tagen; tageweise dismissbar, nur für Admins sichtbar

### Improved
- **Inline Evidence-Vorschau** — PDF- und Bild-Dateien öffnen sich direkt im Browser-Dialog statt als Download
- **Gespeicherte Filter** — Filterzustände in Audit-Log und Findings werden im Browser gespeichert und bei erneutem Besuch wiederhergestellt

---

## [v0.4.4] — 2026-05-17

### Security
- Security-Header im Backend: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Strict-Transport-Security` (1 Jahr)
- Access Token TTL von 8 Stunden auf 1 Stunde reduziert
- `VAKT_SECRET_KEY` Länge wird beim Start validiert (exakt 32 Bytes / 64 Hex-Zeichen)
- MIME/Extension-Allowlist im Evidence-Upload-Handler

### Added
- **Passwort zurücksetzen** — "Passwort vergessen?"-Link auf der Login-Seite, E-Mail mit Reset-Link (1h gültig)
- **Audit-Log UI** — Admin-Seite mit Datum-, Benutzer- und Aktionsfilter, server-seitige Paginierung, CSV-Export
- **Granulare Modul-Berechtigungen** (Pro) — Lese-/Schreibrechte pro Modul pro Benutzer
- **Org-weites MFA-Enforcement** — Admins können 2FA für alle Mitglieder vorschreiben
- **API-Key-Verwaltung** (Pro) — Persönliche API-Keys (`vakt_...`) für programmatischen Zugriff
- **SSO-Login-Button** — erscheint auf der Login-Seite wenn `CASDOOR_URL` konfiguriert ist
- **Update-Status in Einstellungen** — zeigt installierte und aktuelle Version mit Link zu Release Notes
- **"Was ist neu"-Modal** — erscheint einmalig pro Version nach dem Login
- **Compliance-Fortschrittsbalken** — Dashboard-Widget zeigt umgesetzte vs. offene Controls
- **Wöchentlicher Sicherheits-Digest** — opt-in E-Mail-Zusammenfassung jeden Montag

### Improved
- Audit-Log: server-seitige Filterung (statt client-seitig)
- Update-Prüfung zeigt korrekt auf `norvik-ops/vatk` Repository


---

## [v0.4.1] — 2026-05-14

### Added
- **DSGVO Art. 32 TOM-Mapping** — New framework "DSGVO-TOM" with 13 technical and organisational measures (TOM-1 through TOM-13) mapped automatically to existing ISO 27001 controls. Coverage dashboard shows which TOMs are fully covered, partially covered, or open.

---

## [v0.4.0] — 2026-05-14

### Added
- **DORA support** — Digital Operational Resilience Act (EU 2022/2554) is now a selectable framework in Vakt Comply. Includes all relevant DORA articles as controls (German), DORA ↔ ISO 27001 mapping, gap analysis, readiness score, and PDF export.
- **DORA IKT Incident Register** — New incident type "IKT-Vorfall (DORA)" with automatic deadline calculation (T+4h / T+24h / T+72h / T+30d) and traffic-light status per deadline. Webhook notifications on deadline breach.
- **DORA IKT Third-Party Register** — Supplier records extended with DORA criticality, subcontractors, data processing location (EU/non-EU), and exit strategy fields.
- **DORA Resilience Tests** — New section in Vakt Comply for TLPT documentation (DORA Art. 24–27): test type, status, execution date, results, and recommendations.
- **TISAX support** — VDA ISA question catalogue as a selectable framework with protection-level selection (Normal / High / Very high). Maturity scale 0–3 per control. Chapter 15 (prototype protection) shown only when relevant.
- **TISAX ↔ ISO 27001 Mapping** — Static mapping with coverage badges. "Gaps only" toggle filters already-covered controls. Readiness score accounts for ISO 27001 evidence as TISAX coverage.
- **TISAX Readiness Report** — PDF export with protection-level category, readiness score per chapter, maturity distribution, and gap list.
- **Supply Chain Compliance — Supplier Portal** — External, token-based supplier portal at `/supplier/:token` (no login required). Compliance managers send time-limited invitation links; suppliers complete questionnaires and upload certificates (ISO 27001, TISAX labels, etc.) directly in the portal.
- **Questionnaire Builder** — Build supplier assessment questionnaires with question types: Yes/No, Multiple Choice, Free Text, File Upload. Predefined templates: "NIS2 Supplier Assessment", "DORA IKT Third Party", "ISO 27001 Basic Check".
- **Supplier Assessment Review** — Incoming questionnaires reviewable per answer (accepted / requires improvement). Uploaded certificates tracked with expiry date; warning 30 days before expiry. Accepted responses linked automatically as evidence to controls.
- **EU AI Act — AI System Inventory** — New section in Vakt Comply. Register AI systems with provider, use case, affected population groups, decision autonomy, and status. Filter by risk class.
- **EU AI Act — Risk Classification Wizard** — Step-by-step wizard following the EU AI Act Annex III decision tree (prohibition check → high-risk categories → transparency obligations). Result: risk class + justification + relevant articles. Reclassification with change log.
- **EU AI Act — Technical Documentation** — Documentation template per EU AI Act Art. 11 / Annex IV (German). Fields: system description, training data, performance metrics, risk management, human oversight, logging. PDF export and version history.
- **NIS2 / DORA Incident Reporting Assistant** — Reportability classification wizard on incident creation. Automatic authority suggestion based on configured sector. Deadline tracking (T+24h / T+72h / T+30d) with traffic-light status and email notifications 12 hours before each deadline.
- **Incident Report Generator** — One-click report form per deadline (24h / 72h / 30d): pre-filled from incident data, exported as PDF (BSI layout) and JSON. Sent reports archived with timestamp.
- **Authority Directory** — New page in Vakt Comply: list of notification authorities (BSI, BaFin, BNetzA, Luftfahrtbundesamt, BAFZA) with portal URL, phone, and sector-specific notes.
- **Sector Configuration** — Organisation settings now include sector and federal state selection. Responsible authority is suggested automatically in the incident register.
- **Supplier filter improvements** — Criticality filter (critical / essential / standard), assessment status filter, NIS2-relevant and DORA-relevant flags, contract status badges (Active / Expiring / Expired), CSV import and export.

### Fixed
- TypeScript build errors after feature merge (6 type issues resolved).
- Migration 037 (`pg_trgm` indexes) failed in transaction context — added `no-transaction` directive.

---

## [v0.3.0] — 2026-05-13

### Added
- **PDF report exports** — Vakt Scan generates real PDF reports with findings summary, severity breakdown, and paginated findings table. Vakt Comply frameworks export a readiness PDF (colour-coded score, domain breakdown, gap list). Vakt Aware campaigns export a campaign PDF (click rate, rate bars, Betriebsrat-mode banner).
- **External alerting & webhooks** — Send alerts to Slack, Teams, or any webhook endpoint with HMAC signing (`X-Vakt-Signature`). Configurable per alert type. Exponential backoff on delivery failure (up to 4 retries).
- **Backup & Restore** — `scripts/backup.sh` creates timestamped encrypted archives (PostgreSQL dump + AES-encrypted master key). `scripts/restore.sh` supports `--dry-run` for validation without touching the database. Passphrase must be at least 12 characters.
- **Global Search** — Full-text search across all modules (assets, findings, controls, incidents, policies, suppliers, VVT entries, and more). Powered by `pg_trgm` GIN indexes. Command palette shows "Recently viewed" entries.
- **Score configuration** — Admin UI to adjust weighting of compliance score components. "Reset to defaults" button added.
- **Automatic database migrations** — Dedicated `migrate` container runs all pending migrations before the API and worker start on every `docker compose up -d`.
- **Isolated demo instances** — `POST /demo/start` creates a fresh organisation with unique credentials per visitor. No shared demo state between visitors.

### Fixed
- Alert deduplication: alerts now fire at most once per 24 hours per event type per organisation (no more alert floods on each cron tick).
- `window.open()` exports caused 401 errors because Bearer tokens cannot be sent via URL — all exports switched to `fetch()` + Blob download.
- Nullable `description` field in breach records caused crashes when `NULL` — fixed with `COALESCE`.

---

## [v0.2.0] — 2026-03-15

### Added
- Initial SecVitals (Vakt Comply) module with NIS2 and ISO 27001 control frameworks
- SecPulse (Vakt Scan) scanner orchestration: Trivy, Nuclei, OpenVAS integration
- SecVault (Vakt Vault) secrets management with AES-256-GCM encryption and Git repo scanning
- SecReflex (Vakt Aware) phishing simulation engine with SMTP campaign delivery
- SecPrivacy (Vakt Privacy) DSGVO documentation: VVT (Art. 30), DPIA (Art. 35), AVV (Art. 28), breach records (Art. 33/34)
- Demo mode with seed data (`VAKT_DEMO=true`) and per-visitor ephemeral instances
- Initial Docker Compose production and development setups

---

## [v0.1.0] — 2026-02-01

### Added
- Initial open-source release of the SecHealth platform (now rebranded to Vakt)
- Echo v4 HTTP API with Paseto token authentication
- PostgreSQL 16 + sqlc type-safe query layer
- Redis 7 + Asynq background job queue
- golang-migrate database migration system
- Module isolation architecture with per-module RBAC scopes
- Docker Compose single-command deployment (`docker compose up -d`)
- CI/CD pipeline via GitHub Actions (build, lint, test, release)

---
