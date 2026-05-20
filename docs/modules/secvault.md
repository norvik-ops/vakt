# Vakt Vault (`secvault`) — Secrets-Management

## Übersicht

Vakt Vault speichert Secrets verschlüsselt mit AES-256-GCM (Master-Key aus Umgebungsvariable) und protokolliert jeden Zugriff in einem unveränderlichen Audit-Log. Zusätzlich scannt das Modul Git-Repositories auf versehentlich eingecheckte Credentials (via gitleaks) und unterstützt manuelle sowie geplante Secret-Rotation. CI/CD-Pipelines können per API-Token ohne Benutzer-Login auf Secrets zugreifen.

## Aktivierung

Das Modul ist standardmäßig aktiviert. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=secvitals,secpulse,secreflex,secprivacy  # secvault weglassen
```

## Konfiguration

| Variable | Beschreibung |
|----------|--------------|
| `VAKT_SECRET_KEY` | 32-Byte Hex-Masterkey für AES-256-GCM (erforderlich) |

## Features

- **Projekte und Umgebungen** — Secrets in Projekte (z. B. „backend-api") und Umgebungen (dev / staging / prod) strukturieren
- **Secrets speichern und abrufen** — Key-Value-Paare AES-256-GCM-verschlüsselt ablegen; Value wird nur bei direktem Abruf entschlüsselt, nicht in Listen
- **Audit-Log** — Jeder Lesezugriff auf ein Secret wird mit Benutzer, IP, User-Agent und Zeitstempel protokolliert; abrufbar pro Secret und pro Projekt
- **Secret-Rotation** — Manuell ausgelöste Rotation mit drei Strategien (random_string, uuid, db_password); erzeugt automatisch einen Compliance-Nachweis in Vakt Comply
- **Rotationsrichtlinie** — Automatische Rotation nach konfigurierbarem Intervall (Tage)
- **Project Health** — Berechneter Health-Score (0–100) pro Projekt mit konkreten Issues (Alter, fehlende Rotation, hohe Zugriffszahl)
- **Share-Links** — Einmalig verwendbarer, zeitlich begrenzter URL-Token zum sicheren Teilen eines Secrets (kein Login erforderlich)
- **API-Token** — Scoped API-Keys für CI/CD-Integration; nur einmal im Klartext ausgeliefert
- **Import** — Massenimport aus .env-Dateien, HashiCorp Vault oder AWS Secrets Manager
- **Export** — Alle Secrets einer Umgebung exportieren
- **Git-Scanner** — Repository per URL scannen (gitleaks); Findings mit redaktierter Vorschau (first4...last4); Ergebnisse einzeln verwerfen
- **Git-Scan via Asynq** — Scanner läuft asynchron; Status und Ergebnisse per API abrufbar

## API-Endpunkte

Alle Endpunkte erfordern `Authorization: Bearer <token>` oder einen gültigen API-Token, sofern nicht anders angegeben.

### Projekte

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvault/projects` | Alle Projekte auflisten |
| POST | `/api/v1/secvault/projects` | Projekt anlegen |
| DELETE | `/api/v1/secvault/projects/:id` | Projekt löschen |
| GET | `/api/v1/secvault/projects/:project_id/health` | Health-Score des Projekts abrufen |
| GET | `/api/v1/secvault/projects/:project_id/access-log` | Projekt-weites Zugriffs-Log abrufen |

### Umgebungen

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvault/projects/:project_id/envs` | Umgebungen eines Projekts auflisten |
| POST | `/api/v1/secvault/projects/:project_id/envs` | Umgebung anlegen |

### Secrets

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets` | Secret-Keys auflisten (ohne Values) |
| PUT | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets/:key` | Secret anlegen oder aktualisieren |
| GET | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets/:key` | Secret-Value abrufen (wird im Audit-Log vermerkt) |
| DELETE | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets/:key` | Secret löschen |
| GET | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets/:key/log` | Zugriffs-Log für ein Secret abrufen |
| POST | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets/:key/rotate` | Secret rotieren |
| POST | `/api/v1/secvault/projects/:project_id/envs/:env_id/secrets/:key/share` | Share-Link erstellen |

### Import / Export

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| POST | `/api/v1/secvault/projects/:project_id/import` | Secrets importieren (.env / Vault / AWS) |
| GET | `/api/v1/secvault/projects/:project_id/envs/:env_id/export` | Alle Secrets einer Umgebung exportieren |

### Share-Links (kein Bearer-Token erforderlich)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvault/share/:token` | Einmaligen Share-Link einlösen |

### API-Token

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secvault/tokens` | API-Tokens auflisten |
| POST | `/api/v1/secvault/tokens` | API-Token erstellen (Raw-Key einmalig in Antwort) |
| DELETE | `/api/v1/secvault/tokens/:id` | API-Token widerrufen |

### Git-Scanner

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| POST | `/api/v1/secvault/git-scans` | Git-Repository-Scan starten |
| GET | `/api/v1/secvault/git-scans` | Alle Scan-Läufe auflisten |
| GET | `/api/v1/secvault/git-scans/:id` | Einzelnen Scan abrufen |
| GET | `/api/v1/secvault/git-scans/:id/results` | Scan-Ergebnisse abrufen |
| POST | `/api/v1/secvault/git-scans/results/:result_id/dismiss` | Einzelnes Ergebnis verwerfen |

## Datenmodelle

### Secret

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `key` | string | Name des Secrets |
| `value` | string | Entschlüsselter Wert — nur bei GetSecret befüllt |
| `version` | int | Versionsnummer (wird bei Überschreiben inkrementiert) |
| `rotation_due_at` | time | Nächstes Rotationsfälligkeitsdatum (optional) |
| `last_rotated_at` | time | Zeitpunkt der letzten Rotation (optional) |
| `access_count` | int64 | Gesamtanzahl der Lesezugriffe |

### GitScan

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `repo_url` | string | URL des gescannten Repositories |
| `branch` | string | Gescannter Branch |
| `status` | string | pending / running / completed / failed |
| `finding_count` | int | Gesamtanzahl gefundener Credentials |
| `open_count` | int | Anzahl noch offener Findings |
| `dismissed_count` | int | Anzahl verworfener Findings |

### ScanResult

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `file_path` | string | Pfad zur betroffenen Datei im Repository |
| `line_number` | int | Zeilennummer des Treffers |
| `pattern_name` | string | Name des ausgelösten gitleaks-Musters |
| `match_preview` | string | Redaktierter Wert (first4...last4) |
| `severity` | string | Schweregrad des Findings |
| `status` | string | open / dismissed |

### APIToken

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `name` | string | Bezeichnung des Tokens |
| `key_prefix` | string | Erste Zeichen zur Identifikation |
| `scopes` | []string | Berechtigungsumfang |
| `key` | string | Raw-Key — nur einmalig bei Erstellung in der Antwort |
| `expires_at` | time | Ablaufdatum (optional) |

## Hintergrund-Jobs

| Job | Auslöser | Beschreibung |
|-----|----------|--------------|
| `secvault:git_scan` | API-Aufruf | Git-Repository asynchron scannen |

## Compliance-Mapping

| Standard | Control |
|----------|---------|
| NIS2 Art. 21 Abs. 2i | Zugangskontrollen und Authentifizierung |
| NIS2 Art. 21 Abs. 2j | Kryptographie und Schlüsselmanagement |
| ISO 27001:2022 A.8.13 | Informationssicherung |
| ISO 27001:2022 A.8.24 | Kryptographische Verfahren |
| BSI IT-Grundschutz ORP.4 | Identitäts- und Berechtigungsmanagement |
