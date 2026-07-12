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
| [Troubleshooting](troubleshooting.md) | IP-Lockout, Rate-Limit 429, Demo-Flow kaputt, Migrations-Fehler, Container startet nicht |
| [Support & Diagnose](support.md) | Logs einsammeln, Health prüfen, Diagnose-Bundle für Support-Fälle erstellen |
| [FAQ](faq.md) | Häufige Fragen zu Lizenz, Datenschutz, Updates und Unterschieden zu kommerziellen Tools |

### ISMS-Workflow-Guides (ISB-Alltag)

Aufgabenorientierte Schritt-für-Schritt-Anleitungen für die tägliche ISMS-Arbeit:

| Guide | Aufgabe |
|-------|---------|
| [Schutzbedarfsfeststellung](../guides/schutzbedarfsfeststellung.md) | CIA-Schutzbedarf je Zielobjekt bestimmen (BSI-Maximumprinzip) |
| [Vom Risiko zur Maßnahme](../guides/risiko-zu-massnahme.md) | Risiko bewerten → Behandlungsstrategie → Control verknüpfen → Restrisiko |
| [Internes Audit vorbereiten](../guides/internes-audit-vorbereiten.md) | Audit-Programm → Findings → CAPA → Audit-Paket-Export (ISO 9.2) |
| [NIS2-Vorfall melden](../guides/nis2-vorfall-melden.md) | Vorfall erfassen → Meldepflicht prüfen → T+24h/72h/30d dokumentieren |

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

- **Compliance-Frameworks** — NIS2, ISO 27001, BSI IT-Grundschutz, DSGVO TOM, CIS Controls v8, KRITIS, BSI C5, EU AI Act, CRA, ISO 42001, ISO 27017, ISO 27018
- **Scheduled Reports** — Compliance-, Findings- und Risk-Berichte automatisch per E-Mail planen (wöchentlich/monatlich/vierteljährlich)
- **Excel-Export** — Findings, Risks und Controls als `.xlsx` exportieren
- **CSV-Import** — Lieferanten, Assets und Controls per CSV-Datei importieren
- **Webhooks** — Ausgehende Webhooks für `finding.created`, `finding.severity_changed`, `incident.created`, `incident.status_changed`, `control.status_changed`; HMAC-SHA256-signiert

---

## Grundprinzipien

- **Lokal first** — Keine Daten verlassen deinen Server. Keine Telemetrie, kein Usage-Tracking.
  Die einzige Verbindung zu uns ist die Pro-Lizenz-Erneuerung: Läuft der Schlüssel ab, holt sich die Instanz einen neuen von `api.norvikops.de` (bei Jahreslizenz **etwa einmal pro Jahr**, dazwischen kein Aufruf). Übertragen wird **ausschließlich der Lizenz-Token** — keine Nutzungsdaten, keine Compliance-Inhalte. Abschaltbar mit `VAKT_LICENSE_AUTORENEW=false`; der Schlüssel kommt dann per Mail. **Die Community Edition ruft nie an.**
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
