# Vakt Scan (`secpulse`) — Schwachstellenmanagement

## Übersicht

Vakt Scan orchestriert bestehende Open-Source-Scanner (Trivy, Nuclei, OpenVAS) und normalisiert deren Ergebnisse in einem einheitlichen Finding-Modell. Duplikate werden konsolidiert, Findings nach CVSS- und EPSS-Score priorisiert und per SLA-Fristen verfolgt. Sobald ein Finding geschlossen wird, erzeugt ein Asynq-Job automatisch einen Compliance-Nachweis in Vakt Comply.

## Aktivierung

Das Modul ist standardmäßig aktiviert. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=secvitals,secvault,secreflex,secprivacy  # secpulse weglassen
```

## Features

- **Asset-Verwaltung** — Server, Container, Webapps und Repositories mit Criticality-Level und Tags; CSV-Massenimport
- **Scanner-Orchestrierung** — Trivy, Nuclei und OpenVAS per API-Aufruf oder Cron-Schedule starten
- **Geplante Scans** — Wiederkehrende Scan-Schedules (Cron-Ausdruck) pro Asset und Scanner
- **Finding-Normalisierung** — Einheitliches Finding-Modell über alle Scanner; CVSS- und EPSS-Anreicherung; Deduplizierung durch `raw_id`-Matching
- **Finding-Verwaltung** — Status setzen (open / in_progress / resolved / accepted_risk / false_positive), Zuweisung an Benutzer mit Benachrichtigung, Kommentare/Justifikation
- **Bulk-Updates** — Status oder Zuweisung für mehrere Findings gleichzeitig ändern
- **Unterdrückungsregeln** — Findings dauerhaft nach CVE-ID oder Asset-Tag unterdrücken
- **SLA-Dashboard** — Offene Findings mit konfigurierter Remediierungsfrist und Überfälligkeitsstatus
- **SLA-Konfiguration** — Remediierungsfristen in Tagen pro Schweregrad (critical/high/medium/low)
- **Risk-Trend** — Tagesaktuelle Aggregation von Risk-Score, offenen Findings und Critical-Count
- **Scan-Reports** — Asynchron generierte Executive-Reports (PDF/JSON) mit konfigurierbarem Scope
- **Findings-Export** — CSV- oder JSON-Export mit Filter nach Severity und Status
- **Findings-Import** — Bulk-Import von Findings aus externen Quellen
- **Automatische Vakt Comply-Evidence** — Asynq-Job `secpulse:auto_evidence` erstellt bei Finding-Schließung einen Nachweis im Patch-Management-Control

## Rollen

| Rolle | Rechte |
|-------|--------|
| Admin, SecurityAnalyst | Vollzugriff (lesen und schreiben) |
| Viewer, AuditorReadOnly | Nur lesend |

## API-Endpunkte

Alle Endpunkte erfordern `Authorization: Bearer <token>`.

### Assets

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secpulse/assets` | Assets auflisten (Query: `?page`, `?limit`, `?tag`) |
| POST | `/api/v1/secpulse/assets` | Asset anlegen |
| GET | `/api/v1/secpulse/assets/:id` | Einzelnes Asset abrufen |
| PUT | `/api/v1/secpulse/assets/:id` | Asset aktualisieren |
| DELETE | `/api/v1/secpulse/assets/:id` | Asset löschen |
| POST | `/api/v1/secpulse/assets/import` | Assets per CSV importieren (multipart/form-data, Feld: `file`) |

### Scans

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| POST | `/api/v1/secpulse/assets/:id/scans` | Scan auslösen (scanner: trivy / nuclei / openvas) |
| GET | `/api/v1/secpulse/scans/:id` | Scan-Status abrufen |
| GET | `/api/v1/secpulse/assets/:id/schedules` | Scan-Schedules eines Assets auflisten |
| POST | `/api/v1/secpulse/assets/:id/schedules` | Scan-Schedule anlegen |
| DELETE | `/api/v1/secpulse/assets/:id/schedules/:schedule_id` | Scan-Schedule löschen |

### Findings

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secpulse/findings` | Findings auflisten (Filter: `severity`, `status`, `asset_id`, `sort`, `order`, `page`, `limit`) |
| GET | `/api/v1/secpulse/findings/:id` | Einzelnes Finding abrufen |
| PATCH | `/api/v1/secpulse/findings/:id` | Finding-Status, Zuweisung oder Justifikation setzen |
| GET | `/api/v1/secpulse/findings/bulk` | Findings auflisten (Bulk-Ansicht) |
| POST | `/api/v1/secpulse/findings/bulk` | Status oder Zuweisung für mehrere Findings setzen |
| GET | `/api/v1/secpulse/findings/export` | Findings exportieren (Query: `?format=csv\|json`, `?severity`, `?status`) |
| POST | `/api/v1/secpulse/findings/import` | Findings importieren |

### Unterdrückungsregeln

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secpulse/suppressions` | Alle Unterdrückungsregeln auflisten |
| POST | `/api/v1/secpulse/suppressions` | Regel anlegen (nach CVE-ID oder Asset-Tag) |
| DELETE | `/api/v1/secpulse/suppressions/:id` | Regel löschen |

### SLA

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secpulse/sla-dashboard` | SLA-Dashboard (offene Findings mit Fristenstatus) |
| GET | `/api/v1/secpulse/sla-config` | SLA-Konfiguration abrufen |
| PUT | `/api/v1/secpulse/sla-config` | SLA-Konfiguration aktualisieren (nur Admin) |

### Reports

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secpulse/reports/risk-trend` | Risk-Trend abrufen (Query: `?days=90`) |
| POST | `/api/v1/secpulse/reports` | Report asynchron generieren |
| GET | `/api/v1/secpulse/reports` | Alle Reports auflisten |
| GET | `/api/v1/secpulse/reports/:id` | Einzelnen Report abrufen |

## Datenmodelle

### Asset

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `name` | string | Bezeichnung des Assets |
| `type` | string | server / container / webapp / repository |
| `criticality` | string | low / medium / high / critical |
| `tags` | []string | Freitags-Labels für Gruppierung und Unterdrückungsregeln |
| `external_url` | string | URL des Assets (optional) |

### Finding

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `asset_id` | string | Zugehöriges Asset |
| `cve_id` | string | CVE-Kennung (optional) |
| `title` | string | Bezeichnung der Schwachstelle |
| `severity` | string | critical / high / medium / low |
| `cvss_score` | float | CVSS-Score (optional) |
| `epss_score` | float | EPSS-Ausnutzungswahrscheinlichkeit (optional) |
| `risk_score` | float | Berechneter Risikowert |
| `status` | string | open / in_progress / resolved / accepted_risk / false_positive |
| `scanner` | string | trivy / nuclei / openvas |
| `sla_due_at` | time | Remediierungsfrist gemäß SLA-Konfiguration |
| `assigned_to` | string | Zugewiesener Benutzer (optional) |
| `occurrence_count` | int | Anzahl Wiederholungen (Deduplizierung) |

### SLAConfig

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `critical_days` | int | Remediierungsfrist für Critical-Findings (Tage) |
| `high_days` | int | Remediierungsfrist für High-Findings (Tage) |
| `medium_days` | int | Remediierungsfrist für Medium-Findings (Tage) |
| `low_days` | int | Remediierungsfrist für Low-Findings (Tage) |

## Hintergrund-Jobs

| Job | Auslöser | Beschreibung |
|-----|----------|--------------|
| `secpulse:scan:trivy` | API-Aufruf / Schedule | Trivy-Scan ausführen |
| `secpulse:scan:nuclei` | API-Aufruf / Schedule | Nuclei-Scan ausführen |
| `secpulse:scan:openvas` | API-Aufruf / Schedule | OpenVAS-Scan ausführen |
| `secpulse:epss_enrich` | Nach Scan | EPSS-Scores für neue Findings nachladen |
| `secpulse:auto_evidence` | Finding-Schließung | Compliance-Nachweis in Vakt Comply erstellen |
| `secpulse:generate_report` | API-Aufruf | Report asynchron generieren |

## Compliance-Mapping

| Standard | Control |
|----------|---------|
| NIS2 Art. 21 Abs. 2f | Schwachstellenmanagement und Offenlegung |
| ISO 27001:2022 A.8.8 | Management von technischen Schwachstellen |
| BSI IT-Grundschutz OPS.1.1.6 | Software-Tests; SYS.1.1 M24 Schwachstellenmanagement |
