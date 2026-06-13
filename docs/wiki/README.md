# Vakt — Wiki

Willkommen im Vakt-Wiki. Hier findest du alles, was du für Installation, Konfiguration und Betrieb der Plattform brauchst.

Vakt ist eine selbst gehostete Security- & Compliance-Plattform für KMU im DACH-Raum. Lizenz: [Elastic License 2.0 (ELv2)](../../LICENSE). Quellcode offen lesbar, kostenlos für den Eigenbetrieb.

---

## Inhalt

### Einstieg

| Seite | Beschreibung |
|-------|--------------|
| [Installation](installation.md) | Systemanforderungen, Docker-Compose-Quickstart, HTTPS, erste Schritte |
| [Konfigurationsreferenz](configuration.md) | Alle Umgebungsvariablen vollständig dokumentiert |
| [API-Referenz](api-reference.md) | REST API — Endpoints, Authentifizierung, Beispiele; OpenAPI 3.0 Spec |
| [FAQ](faq.md) | Häufige Fragen zu Lizenz, Datenschutz, Updates und Unterschieden zu kommerziellen Tools |

### Module

| Modul | Beschreibung |
|-------|--------------|
| [Vakt Comply](modules/comply.md) | Compliance-Hub: NIS2, ISO 27001, BSI-Grundschutz, Risikomanagement, Vorfallsregister, Audits |
| [Vakt Scan](modules/scan.md) | Scanner-Orchestrierung: Trivy, Nuclei, OpenVAS — Findings werden automatisch als Compliance-Evidenz übertragen |
| [Vakt Vault](modules/vault.md) | Secrets Management: AES-256-GCM-Verschlüsselung, Git-Repo-Scanning, automatische Rotation |
| [Vakt Aware](modules/aware.md) | Security Awareness: Phishing-Simulationen, Micro-Trainings, Betriebsrat-konformes Reporting |
| [Vakt Privacy](modules/privacy.md) | DSGVO-Hub: VVT (Art. 30), DPIA (Art. 35), AVV (Art. 28), DSR-Workflows, Meldungsregister |
| [Vakt HR](modules/hr.md) | Mitarbeiter-Lifecycle: Onboarding/Offboarding-Checklisten, Mitarbeiterverzeichnis, Compliance-Evidenz |

---

## Kurzübersicht

```
docker compose up -d
```

Das ist alles. Vakt ist in **unter 5 Minuten startbereit** unter `http://localhost`. Die mitgelieferte lokale KI braucht beim **ersten** Start zusätzlich Zeit, um das Default-Modell (`qwen2.5:7b`, ~4.5 GB, braucht 8 GB RAM; auf kleinen VMs `qwen2.5:3b`) zu laden — je nach Internet-Verbindung **3–30 Minuten extra**, bis die KI-Funktionen nutzbar sind. Die Plattform selbst funktioniert sofort, du musst nicht auf das Modell warten.

Datenbankmigrationen laufen automatisch beim Start. Kein manueller Setup-Schritt erforderlich.

---

## Features (Auswahl)

- **Compliance-Frameworks** — NIS2, ISO 27001, BSI IT-Grundschutz, DSGVO TOM, CIS Controls v8, KRITIS, BSI C5, EU AI Act, CRA, DORA, TISAX, ISO 42001, ISO 27017, ISO 27018
- **Scheduled Reports** — Compliance-, Findings- und Risk-Berichte automatisch per E-Mail planen (wöchentlich/monatlich/vierteljährlich)
- **Excel-Export** — Findings, Risks und Controls als `.xlsx` exportieren
- **CSV-Import** — Lieferanten, Assets und Controls per CSV-Datei importieren
- **Webhooks** — Ausgehende Webhooks für `finding.created`, `finding.severity_changed`, `incident.created`, `incident.status_changed`, `control.status_changed`; HMAC-SHA256-signiert

---

## Grundprinzipien

- **Lokal first** — Keine Daten verlassen deinen Server. Kein Phone-home, kein Telemetry.
- **Documentation-first** — Ziel ist auditreife Compliance-Evidenz, kein aktiver Sicherheitsbetrieb.
- **Modular** — Jedes Modul kann einzeln aktiviert oder deaktiviert werden.
- **Selbstgehostet** — `docker compose up -d` reicht. Kein Kubernetes erforderlich.

---

## Technischer Stack

| Schicht | Technologie |
|---------|-------------|
| Backend | Go 1.26+, Echo v4 |
| Datenbank | PostgreSQL 16 |
| Queue / Cache | Redis 7 |
| Frontend | React 18 + TypeScript (Vite) |
| UI | shadcn/ui + Tailwind CSS |
| Auth | Paseto-Token, OIDC/SAML via Casdoor |
| Deployment | Docker Compose, Helm (Kubernetes) |

---

## Beitragen

Issues und Pull Requests sind willkommen. Vor einem PR `make lint` ausführen und Tests schreiben. Keine Secrets committen — `.env.example` als Vorlage verwenden.
