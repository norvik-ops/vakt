# Vakt Aware (`secreflex`) — Security Awareness & Phishing-Simulation

## Übersicht

Vakt Aware ermöglicht interne Phishing-Simulationen und ordnet Mitarbeitern nach einer Simulation automatisch passende Trainingsmodule zu. Das Reporting ist standardmäßig anonymisiert (Betriebsrat-Modus), sodass keine individuellen Klickdaten an die Unternehmensleitung weitergegeben werden. Abgeschlossene Trainings fließen automatisch als Compliance-Nachweis in Vakt Comply ein.

## Aktivierung

Das Modul ist standardmäßig aktiviert. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=secvitals,secpulse,secvault,secprivacy  # secreflex weglassen
```

## Konfiguration

| Variable | Beschreibung |
|----------|--------------|
| `VAKT_SMTP_HOST` | SMTP-Server-Hostname (erforderlich für Kampagnen) |
| `VAKT_SMTP_PORT` | SMTP-Port (Standard: 1025 für Mailpit) |
| `VAKT_SMTP_USER` | SMTP-Benutzername (erforderlich für Port 587/465) |
| `VAKT_SMTP_PASS` | SMTP-Passwort (erforderlich für Port 587/465) |
| `VAKT_SMTP_FROM` | Absenderadresse für Kampagnen-E-Mails |

## Features

- **E-Mail-Vorlagen** — Phishing-Vorlagen für Angriffstypen phishing, vishing, usb, smishing; 15 vorgefertigte Presets eingebaut
- **Zielgruppen** — Empfänger in benannten Gruppen organisieren; Massenimport per CSV; Active-Directory-Synchronisierung (ADOU-Attribut)
- **Landing Pages** — Individuelle HTML-Captureseiten nach Klick konfigurierbar
- **Kampagnen** — Kampagne mit Template, Zielgruppe und Landing Page verknüpfen; einmalig oder wiederkehrend (none / monthly / quarterly)
- **Kampagnenversand** — E-Mails per SMTP versenden; Open-Tracking per Pixel optional zuschaltbar
- **Betriebsrat-Modus** — Bei aktiviertem `betriebsrat_mode` werden Events nur auf Abteilungsebene aggregiert, nie für einzelne Personen
- **Event-Tracking** — Öffnungen, Klicks und Credentials-Eingaben werden über URL-Token ohne Login aufgezeichnet
- **Kampagnen-Statistiken** — Aggregierte Rates (open_rate, click_rate, submission_rate) pro Kampagne
- **Trainingsmodule** — Video- oder Quiz-Module pro Angriffstyp; konfigurierbare Bestehensgrenze (1–100 %)
- **Zuweisungen** — Training einem Einzelziel oder einer Abteilung mit Fälligkeitsdatum zuweisen; Überfälligkeitsstatus automatisch berechnet
- **Trainingsabschluss** — Quiz-Antworten einreichen und Bestanden/Nicht-Bestanden ermitteln; Asynq-Job erzeugt automatisch einen Vakt Comply-Nachweis
- **Training-Reminder** — Asynq-Job erinnert an überfällige Zuweisungen

## Rollen

| Rolle | Rechte |
|-------|--------|
| Admin, SecurityAnalyst | Vollzugriff (lesen und schreiben) |
| Viewer, AuditorReadOnly | Nur lesend |

## API-Endpunkte

Alle Endpunkte erfordern `Authorization: Bearer <token>`, sofern nicht anders angegeben.

### Vorlagen

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/templates` | Alle benutzerdefinierten Vorlagen auflisten |
| GET | `/api/v1/secreflex/templates/presets` | Eingebaute Preset-Vorlagen auflisten |
| POST | `/api/v1/secreflex/templates` | Neue Vorlage anlegen |

### Zielgruppen

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/groups` | Alle Zielgruppen auflisten |
| POST | `/api/v1/secreflex/groups` | Zielgruppe anlegen |
| GET | `/api/v1/secreflex/groups/:id/targets` | Empfänger einer Gruppe auflisten |
| POST | `/api/v1/secreflex/groups/:id/targets/import` | Empfänger per CSV importieren |

### Landing Pages

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/landing-pages` | Alle Landing Pages auflisten |
| POST | `/api/v1/secreflex/landing-pages` | Landing Page anlegen |

### Kampagnen

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/campaigns` | Alle Kampagnen auflisten |
| POST | `/api/v1/secreflex/campaigns` | Kampagne anlegen |
| GET | `/api/v1/secreflex/campaigns/:id` | Einzelne Kampagne abrufen |
| POST | `/api/v1/secreflex/campaigns/:id/launch` | Kampagne starten (E-Mails versenden) |
| POST | `/api/v1/secreflex/campaigns/:id/abort` | Kampagne abbrechen |
| GET | `/api/v1/secreflex/campaigns/:id/stats` | Kampagnen-Statistiken abrufen |

### Trainingsmodule

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/training-modules` | Alle Trainingsmodule auflisten |
| POST | `/api/v1/secreflex/training-modules` | Trainingsmodul anlegen |

### Zuweisungen

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/assignments` | Alle Zuweisungen auflisten |
| POST | `/api/v1/secreflex/assignments/:id/complete` | Zuweisung als abgeschlossen markieren (Quiz-Antworten einreichen) |

### Tracking (kein Bearer-Token erforderlich)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/secreflex/t/:token` | Klick-Event aufzeichnen (Tracking-Pixel / Link) |
| POST | `/api/v1/secreflex/t/:token/submit` | Formular-Submission-Event aufzeichnen |

## Datenmodelle

### Campaign

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `name` | string | Bezeichnung der Kampagne |
| `status` | string | draft / scheduled / running / completed / aborted |
| `from_email` | string | Absenderadresse |
| `recurrence` | string | none / monthly / quarterly |
| `track_opens` | bool | Öffnungen per Pixel tracken |
| `betriebsrat_mode` | bool | Nur Abteilungs-Aggregation, keine Einzeldaten |
| `scheduled_at` | time | Geplanter Versandzeitpunkt (optional) |
| `next_run_at` | time | Nächste Wiederholung bei wiederkehrenden Kampagnen |

### TrainingModule

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `title` | string | Bezeichnung des Moduls |
| `type` | string | video / quiz |
| `attack_type` | string | phishing / vishing / usb / smishing |
| `content_url` | string | URL zum Video oder Quiz-Inhalt |
| `duration_seconds` | int | Dauer des Moduls in Sekunden |
| `passing_score` | int | Mindest-Prozentsatz zum Bestehen (1–100) |
| `questions` | []Question | Quiz-Fragen mit Optionen und korrekter Antwort |

### CampaignStats

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `total_targets` | int | Gesamtanzahl angeschriebener Empfänger |
| `emails_sent` | int | Tatsächlich versendete E-Mails |
| `opens` | int | Anzahl Öffnungen |
| `clicks` | int | Anzahl Link-Klicks |
| `form_submissions` | int | Anzahl Credential-Eingaben |
| `open_rate` | float | Öffnungsrate (0–1) |
| `click_rate` | float | Klickrate (0–1) |
| `submission_rate` | float | Einreichungsrate (0–1) |

## Hintergrund-Jobs

| Job | Auslöser | Beschreibung |
|-----|----------|--------------|
| `secreflex:send_campaign` | Kampagnen-Launch | E-Mails an alle Zielgruppen-Empfänger versenden |
| `secreflex:training_reminder` | Täglich | Erinnerung an überfällige Trainings-Zuweisungen |

## Compliance-Mapping

| Standard | Control |
|----------|---------|
| NIS2 Art. 21 Abs. 2g | Schulungen zur Cybersicherheit und Grundhygiene |
| ISO 27001:2022 A.6.3 | Sicherheitsbewusstsein, Aus- und Weiterbildung |
| BSI IT-Grundschutz ORP.3 | Sensibilisierung und Schulung |
