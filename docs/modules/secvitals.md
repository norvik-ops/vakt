# SecVitals (Vakt Comply) — Compliance-Hub

## Übersicht

Vakt Comply ist das zentrale Modul von Vakt. Es führt durch die Implementierung von NIS2, ISO 27001, BSI-Grundschutz, DORA, TISAX, EU AI Act und weiteren Frameworks, verfolgt den Status einzelner Controls, verwaltet Nachweise mit Reviewer-Workflow und produziert auditreife Dokumentation. Alle anderen Module liefern ihre Ergebnisse als Compliance-Evidence in Vakt Comply ein.

## Aktivierung

Das Modul ist standardmäßig aktiviert. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=secpulse,secvault,secreflex,secprivacy  # secvitals weglassen
```

## Features

### Compliance-Frameworks

- **Frameworks aktivieren** — NIS2, ISO 27001, BSI-Grundschutz, DORA, TISAX, EU AI Act, DSGVO-TOM, ISO 42001, CRA; Readiness-Score pro Framework
- **TISAX-Ansicht** — Schutzbedarf-Tabs (Normal / Hoch / Sehr hoch), Reifegradskala 0–3, Kapitel 15 (Prototypenschutz) nur bei passendem Schutzbedarf
- **TISAX ↔ ISO 27001 Mapping** — Tabellenansicht Covered/Lücke, Toggle „Nur Lücken", Coverage-Berechnung berücksichtigt ISO-Nachweise
- **DSGVO Art. 32 TOM-Mapping** — 13 TOMs (Zutrittskontrolle bis Datenschutz-Compliance) mit ISO 27001 Deckungsanalyse
- **DORA-Dashboard** — IKT-Vorfälle, Fristen-Ampeln, Drittanbieter-Risiken auf einen Blick
- **EU AI Act Dashboard** — KI-Systeme nach Risikoklasse, offene Klassifizierungen

### Controls & Nachweise

- **Controls tracken** — Status (covered / partial / missing / in_progress / implemented / not_applicable) pro Control; TISAX: Reifegrad 0–3
- **Implementierungsaufgaben** — Pro Control beliebig viele Umsetzungsschritte anlegen und abhaken
- **Nachweis-Management** — Evidence manuell hinzufügen, als Datei hochladen oder automatisch über Collector (GitHub, AWS, Azure, AD); Ablaufdaten und Versionierung
- **Reviewer-Workflow** — Evidence einem Reviewer zuweisen; approved/rejected-Status mit Notiz
- **Gap-Analyse** — Controls nach Status gruppiert, Grund der Lücke (no_evidence, evidence_expiring, review_pending)

### Vorfallsmanagement & Meldepflichten

- **Vorfallsregister** — Sicherheitsvorfälle mit Schweregrad, Status und betroffenen Systemen; IKT-Vorfälle (DORA) als eigener Typ
- **NIS2-Meldungsassistent** — Meldepflicht-Klassifizierung beim Anlegen (Kurzfragebogen); automatische Fristberechnung T+24h/72h/30d; Ampel-Status pro Frist
- **DORA-Meldepflichten** — Fristen T+4h/24h/72h/30d; Meldebericht als PDF + JSON
- **Meldungsformular-Generator** — Vorausgefüllte Formulare im BSI-/BaFin-Layout; Meldungshistorie mit Zeitstempeln
- **Behörden-Verzeichnis** — BSI, BaFin, BNetzA, Luftfahrtbundesamt mit Portal-URLs; automatische Behördenauswahl anhand des konfigurierten Sektors

### Lieferanten-Portal (Supply Chain Compliance)

- **Lieferanten-Register** — Name, Kritikalität (kritisch/wesentlich/standard), NIS2-relevant, DORA-relevant, Vertragslaufzeit; Contract- und Assessment-Status-Badges; Ampel-Status
- **Fragebogen-Builder** — Frage-Typen: Ja/Nein, Multiple Choice, Freitext, Datei-Upload; Vorlagen für NIS2/DORA/ISO 27001; Fragen optional/verpflichtend
- **Externes Lieferanten-Portal** — Tokenbasiert, kein Vakt-Account nötig; zeitlich begrenzt (7/14/30 Tage); Zertifikate-Upload; Bestätigungs-E-Mails
- **Auswertung** — Fragebogen-Status (Ausstehend/Eingegangen/Bewertet); Nachbesserungs-Markierung; automatische Control-Verknüpfung; CSV-Import/Export

### KI-System-Compliance (EU AI Act)

- **KI-System-Inventar** — Name, Anbieter, Einsatzbereich, Entscheidungsautonomie, Status
- **Risiko-Klassifizierungs-Wizard** — Entscheidungsbaum nach Annex III; Verbote → Hochrisiko → Transparenzpflicht; Klassifizierung mit Audit-Trail
- **Technische Dokumentation** — Template nach Art. 11 / Annex IV; PDF-Export; Versionierung

### DORA-Resilienztests

- **TLPT-Dokumentation** (DORA Art. 24–27) — Resilienztests mit Typ, Status, Datum, Ergebnissen und Handlungsempfehlungen

### Bestehende Features

- **Risikoregister** — Risiken mit Likelihood × Impact (je 1–5), Behandlung (avoid/mitigate/transfer/accept), Verknüpfung mit Controls
- **Richtlinien-Management** — Policies mit Draft/Active/Archived-Status und Versionierung; 10 deutsche Vorlagen
- **Interne Audit-Records** — Audit-Protokolle mit Scope, Auditor, Befunden und Empfehlungen
- **Auditor-Portal** — Zeitlich begrenzter URL-Token für externe Prüfer ohne Login; lesender Zugriff auf Framework-Report
- **Audit-Paket Export** — ZIP-Download (control.json + evidence.json) pro Control oder als vollständiges Bundle
- **Evidence-Ablauf-Alerts** — Täglicher Asynq-Job warnt 30 Tage vor Ablauf von Evidence-Items

## API-Endpunkte

Alle Endpunkte erfordern `Authorization: Bearer <token>`, sofern nicht anders angegeben.

### Frameworks

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/frameworks` | Alle aktivierten Frameworks auflisten |
| GET | `/api/v1/secvitals/frameworks/:id` | Einzelnes Framework abrufen |
| POST | `/api/v1/secvitals/frameworks/:name/enable` | Framework aktivieren (name: `nis2`, `iso27001`, `bsi`, `dora`, `tisax`, `eu-ai-act`, `dsgvo-tom`, …) |
| DELETE | `/api/v1/secvitals/frameworks/:id` | Framework deaktivieren |
| GET | `/api/v1/secvitals/frameworks/:id/report` | Readiness-Report abrufen |
| GET | `/api/v1/secvitals/frameworks/:id/gaps` | Gap-Analyse abrufen |
| GET | `/api/v1/secvitals/frameworks/:id/controls` | Alle Controls eines Frameworks auflisten |
| POST | `/api/v1/secvitals/frameworks/:id/auditor-link` | Auditor-Token erstellen (Body: `expires_in_hours`) |
| GET | `/api/v1/secvitals/frameworks/tisax/iso-mapping` | TISAX ↔ ISO 27001 Mapping mit Coverage-Status |
| GET | `/api/v1/secvitals/frameworks/tisax/coverage-after-iso` | TISAX-Controls ohne ISO-27001-Abdeckung |
| GET | `/api/v1/secvitals/frameworks/:id/tisax-report-pdf` | TISAX-Bereitschaftsbericht als PDF |
| GET | `/api/v1/secvitals/dsgvo/tom-coverage` | DSGVO Art. 32 TOM-Deckungsanalyse |

### Controls

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/controls/:id` | Einzelnes Control abrufen |
| PATCH | `/api/v1/secvitals/controls/:id` | Status / not_applicable setzen |
| GET | `/api/v1/secvitals/controls/:id/tasks` | Implementierungsaufgaben auflisten |
| POST | `/api/v1/secvitals/controls/:id/tasks` | Aufgabe anlegen |
| PATCH | `/api/v1/secvitals/controls/:id/tasks/:taskId` | Aufgabe abschließen oder bearbeiten |
| DELETE | `/api/v1/secvitals/controls/:id/tasks/:taskId` | Aufgabe löschen |
| GET | `/api/v1/secvitals/controls/:id/evidence` | Evidence-Liste abrufen |
| POST | `/api/v1/secvitals/controls/:id/evidence` | Evidence manuell hinzufügen |
| POST | `/api/v1/secvitals/controls/:id/evidence/upload` | Evidence als Datei hochladen (multipart/form-data) |
| POST | `/api/v1/secvitals/controls/:id/collect` | Evidence automatisch einsammeln (github/aws/azure/ad) |
| GET | `/api/v1/secvitals/controls/:id/export` | Evidence-Bundle als ZIP exportieren |

### Evidence

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| POST | `/api/v1/secvitals/evidence/:id/review` | Evidence genehmigen oder ablehnen |
| GET | `/api/v1/secvitals/evidence/expiring` | Bald ablaufende Evidence abrufen (Query: `?days=30`) |

### Auditor-Portal (kein Bearer-Token erforderlich)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/auditor/:token` | Framework-Report für Prüfer (lesend) |
| GET | `/api/v1/secvitals/auditor/:token/export` | Vollständiges Evidence-Bundle als ZIP |

### Auditor-Links (Verwaltung)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/auditor-links` | Alle Auditor-Links auflisten |
| DELETE | `/api/v1/secvitals/auditor-links/:id` | Auditor-Link widerrufen |

### Risikoregister

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/risks` | Alle Risiken auflisten |
| POST | `/api/v1/secvitals/risks` | Risiko anlegen |
| GET | `/api/v1/secvitals/risks/:id` | Einzelnes Risiko abrufen |
| PATCH | `/api/v1/secvitals/risks/:id` | Risiko aktualisieren |
| GET | `/api/v1/secvitals/risks/:id/controls` | Verknüpfte Controls abrufen |
| POST | `/api/v1/secvitals/risks/:id/controls` | Control mit Risiko verknüpfen |
| DELETE | `/api/v1/secvitals/risks/:id/controls/:controlId` | Verknüpfung aufheben |

### Vorfallsregister & Meldepflichten

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/incidents` | Alle Vorfälle auflisten |
| POST | `/api/v1/secvitals/incidents` | Vorfall anlegen |
| GET | `/api/v1/secvitals/incidents/:id` | Einzelnen Vorfall abrufen |
| PATCH | `/api/v1/secvitals/incidents/:id` | Vorfall aktualisieren |
| POST | `/api/v1/secvitals/incidents/:id/reports` | Meldungsformular generieren (Body: `type`: `early`/`full`/`final`) |
| GET | `/api/v1/secvitals/incidents/:id/reports` | Meldungshistorie abrufen |

### Lieferanten-Portal

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/suppliers` | Alle Lieferanten auflisten |
| POST | `/api/v1/secvitals/suppliers` | Lieferant anlegen |
| PATCH | `/api/v1/secvitals/suppliers/:id` | Lieferant aktualisieren |
| DELETE | `/api/v1/secvitals/suppliers/:id` | Lieferant löschen |
| POST | `/api/v1/secvitals/suppliers/import` | CSV-Import |
| GET | `/api/v1/secvitals/suppliers/:id/status` | Ampel-Status (grün/gelb/rot) |
| GET | `/api/v1/secvitals/questionnaires` | Fragebögen auflisten |
| POST | `/api/v1/secvitals/questionnaires` | Fragebogen erstellen |
| POST | `/api/v1/secvitals/suppliers/:id/assessments` | Assessment starten (sendet Einladungslink) |
| GET | `/api/v1/secvitals/suppliers/:id/assessments` | Assessments eines Lieferanten |
| GET | `/api/v1/supplier/:token` | Externes Portal (kein Auth) |
| POST | `/api/v1/supplier/:token/submit` | Fragebogen einreichen (kein Auth) |

### KI-Systeme (EU AI Act)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/ai-systems` | Alle KI-Systeme auflisten |
| POST | `/api/v1/secvitals/ai-systems` | KI-System erfassen |
| PATCH | `/api/v1/secvitals/ai-systems/:id` | KI-System aktualisieren |
| POST | `/api/v1/secvitals/ai-systems/:id/classify` | Risikoklassifizierung speichern |
| GET | `/api/v1/secvitals/ai-systems/:id/documentation` | Technische Dokumentation abrufen |
| PUT | `/api/v1/secvitals/ai-systems/:id/documentation` | Dokumentation erstellen/aktualisieren |

### Resilienztests (DORA TLPT)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/resilience-tests` | Alle Tests auflisten |
| POST | `/api/v1/secvitals/resilience-tests` | Test anlegen |
| PATCH | `/api/v1/secvitals/resilience-tests/:id` | Test aktualisieren |

### Sektor-Konfiguration

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/org/sector` | Sektor und Bundesland der Organisation abrufen |
| PATCH | `/api/v1/secvitals/org/sector` | Sektor und Bundesland setzen |
| GET | `/api/v1/secvitals/authorities` | Zuständige Behörden für konfigurierten Sektor |

### Richtlinien

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/policies` | Alle Richtlinien auflisten |
| POST | `/api/v1/secvitals/policies` | Richtlinie anlegen |
| GET | `/api/v1/secvitals/policies/:id` | Einzelne Richtlinie abrufen |
| PATCH | `/api/v1/secvitals/policies/:id` | Richtlinie aktualisieren |
| GET | `/api/v1/secvitals/policy-templates` | Eingebaute Vorlagen auflisten (10 deutsche Templates) |
| POST | `/api/v1/secvitals/policy-templates/:id/apply` | Richtlinie aus Vorlage erstellen |

### Interne Audits

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvitals/audits` | Alle Audit-Records auflisten |
| POST | `/api/v1/secvitals/audits` | Audit-Record anlegen |
| GET | `/api/v1/secvitals/audits/:id` | Einzelnen Audit-Record abrufen |
| PATCH | `/api/v1/secvitals/audits/:id` | Audit-Record aktualisieren |

## Datenmodelle

### Control

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `framework_id` | string | Zugehöriges Framework |
| `control_id` | string | Framework-spezifische Kennung (z. B. „A.8.8") |
| `title` | string | Bezeichnung des Controls |
| `domain` | string | Themenbereich innerhalb des Frameworks |
| `status` | string | covered / partial / missing / not_applicable / in_progress / implemented |
| `not_applicable` | bool | Control als nicht anwendbar markiert |
| `manual_status` | string | Manuell gesetzter Status (in_progress / implemented) |
| `evidence_count` | int | Anzahl verknüpfter Evidence-Einträge |
| `weight` | int | Gewichtung im Readiness-Score |

### Evidence

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `control_id` | string | Zugehöriges Control |
| `title` | string | Bezeichnung des Nachweises |
| `source` | string | manual / github / aws / azure / ad |
| `status` | string | Aktueller Review-Status |
| `version` | int | Versionsnummer (wird bei Ersatz inkrementiert) |
| `expires_at` | time | Ablaufdatum des Nachweises |
| `reviewed_by` | string | Benutzer, der die Review durchgeführt hat |
| `file_path` | string | Pfad zur hochgeladenen Datei (optional) |

### Risk

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `title` | string | Bezeichnung des Risikos |
| `likelihood` | int | Eintrittswahrscheinlichkeit 1–5 |
| `impact` | int | Schadensausmaß 1–5 |
| `risk_score` | int | Berechnet: likelihood × impact |
| `status` | string | open / mitigated / accepted / closed |
| `treatment` | string | avoid / mitigate / transfer / accept |

## Hintergrund-Jobs

| Job | Zeitplan | Beschreibung |
|-----|----------|--------------|
| `secvitals:evidence_expiry_alert` | Täglich | Benachrichtigung bei Evidence, die in 30 Tagen abläuft |

## Policy-Vorlagen (eingebaut)

10 deutsche Templates in folgenden Kategorien:

- Informationssicherheitsrichtlinie (ISMS)
- Passwort-Richtlinie (Zugangskontrolle)
- Richtlinie zur akzeptablen Nutzung (Nutzung)
- Homeoffice- und Fernarbeitsrichtlinie (Fernarbeit)
- Datenklassifizierungsrichtlinie (Datenschutz)
- Incident-Response-Richtlinie (Vorfallsmanagement)
- Änderungsmanagement-Richtlinie (Betrieb)
- Zugangs- und Zugriffskontrollrichtlinie (Zugangskontrolle)
- Datensicherungsrichtlinie (Verfügbarkeit)
- Lieferanten- und Dienstleistersicherheit (Lieferantenmanagement)

## Hintergrund-Jobs (aktualisiert)

| Job | Zeitplan | Beschreibung |
|-----|----------|--------------|
| `secvitals:evidence_expiry_alert` | Täglich | Benachrichtigung bei Evidence, die in 30 Tagen abläuft |
| `secvitals:incident_deadline_check` | Stündlich | Prüft NIS2/DORA-Meldefristen; E-Mail 12h vor Ablauf, Webhook bei Überschreitung |
| `secvitals:supplier_cert_expiry` | Täglich | Warnung bei Lieferanten-Zertifikaten, die in 30 Tagen ablaufen |

## Compliance-Mapping

| Standard | Abdeckung |
|----------|-----------|
| NIS2 Art. 21 Abs. 2 | Vollständige Abdeckung aller Maßnahmen (a–j) |
| ISO 27001:2022 | Alle 93 Annex-A-Controls trackbar |
| BSI IT-Grundschutz | 38 Bausteine als Framework abbildbar |
| DORA EU 2022/2554 | Kapitel II–VI; Incident-Register mit 4h/24h/72h/30d-Fristen; Drittanbieter-Register |
| TISAX (VDA ISA) | Kapitel 1–15; Schutzbedarf Normal/Hoch/Sehr hoch; Reifegradskala 0–3 |
| EU AI Act 2024/1689 | KI-Inventar; Risikoklassen; technische Dokumentation nach Art. 11 / Annex IV |
| DSGVO Art. 32 | 13 TOMs mit ISO-27001-Deckungsanalyse |
| ISO 42001 | KI-Management-System-Framework |
| CRA (Cyber Resilience Act) | Framework-Controls für Hersteller von Produkten mit digitalen Elementen |
