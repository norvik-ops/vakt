# Vakt Privacy (`vaktprivacy`) — DSGVO-Dokumentation

## Übersicht

Vakt Privacy ist die zentrale DSGVO-Dokumentationsplattform innerhalb von Vakt. Es deckt alle praxisrelevanten Pflichten der DSGVO ab: Verzeichnis der Verarbeitungstätigkeiten (Art. 30), Datenschutz-Folgeabschätzungen (Art. 35), Auftragsverarbeiterverträge (Art. 28) sowie Datenpannenmeldungen (Art. 33/34) und Betroffenenrechts-Anfragen (Art. 15–21). Pannenmeldungen werden automatisch mit dem Vakt Comply-Vorfallsregister verknüpft; abgeschlossene Betroffenenanfragen erzeugen automatisch einen Compliance-Nachweis in Vakt Comply.

## Aktivierung

Das Modul ist standardmäßig aktiviert. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=vaktcomply,vaktscan,vaktvault,vaktaware  # vaktprivacy weglassen
```

## Features

- **VVT** — Verzeichnis von Verarbeitungstätigkeiten nach Art. 30 DSGVO; Felder für Zweck, Rechtsgrundlage, Datenkategorien, Betroffene, Empfänger, Aufbewahrung, Drittlandtransfer; CSV-Export; Status active/archived
- **DPIA** — Datenschutz-Folgeabschätzungen nach Art. 35 DSGVO; Verknüpfung mit VVT-Einträgen; Notwendigkeits- und Risikobeurteilung, Minderungsmaßnahmen, Restrisiko, DSB-Konsultation; Genehmigungs-Workflow; Export
- **AVV** — Auftragsverarbeitungsverträge nach Art. 28 DSGVO; Ablaufdatum und Review-Datum pro Vertrag; automatische Statusänderung auf "expired"; täglicher Asynq-Job für Ablauf-Alerts
- **Datenpannenmeldungen** — Breach-Records nach Art. 33/34 DSGVO; 72-Stunden-Deadline automatisch berechnet; Behördenbenachrichtigung dokumentieren; PDF-Export der Meldung; automatische Verknüpfung mit Vakt Comply-Vorfallsregister via Asynq
- **DSR** — Betroffenenrechts-Anfragen nach Art. 15–21 DSGVO; 30-Tage-Frist automatisch berechnet (Art. 12 Abs. 3); Typen: access / erasure / portability / objection / rectification; CSV-Export; Asynq-Job für Überfälligkeits-Alerts; Abschluss erzeugt Vakt Comply-Evidence

## API-Endpunkte

Alle Endpunkte erfordern `Authorization: Bearer <token>`.

### VVT (Art. 30 DSGVO)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/vaktprivacy/vvt` | Alle VVT-Einträge auflisten |
| POST | `/api/v1/vaktprivacy/vvt` | VVT-Eintrag anlegen |
| GET | `/api/v1/vaktprivacy/vvt/export` | VVT als CSV exportieren |
| GET | `/api/v1/vaktprivacy/vvt/:id` | Einzelnen VVT-Eintrag abrufen |
| PUT | `/api/v1/vaktprivacy/vvt/:id` | VVT-Eintrag aktualisieren |
| DELETE | `/api/v1/vaktprivacy/vvt/:id` | VVT-Eintrag löschen |

### DPIA (Art. 35 DSGVO)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/vaktprivacy/dpias` | Alle DPIAs auflisten |
| POST | `/api/v1/vaktprivacy/dpias` | DPIA anlegen |
| GET | `/api/v1/vaktprivacy/dpias/export` | DPIAs exportieren |
| GET | `/api/v1/vaktprivacy/dpias/:id` | Einzelne DPIA abrufen |
| PUT | `/api/v1/vaktprivacy/dpias/:id` | DPIA aktualisieren |
| POST | `/api/v1/vaktprivacy/dpias/:id/approve` | DPIA genehmigen |
| DELETE | `/api/v1/vaktprivacy/dpias/:id` | DPIA löschen |

### AVV (Art. 28 DSGVO)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/vaktprivacy/avvs` | Alle AVVs auflisten |
| POST | `/api/v1/vaktprivacy/avvs` | AVV anlegen |
| GET | `/api/v1/vaktprivacy/avvs/:id` | Einzelnen AVV abrufen |
| PUT | `/api/v1/vaktprivacy/avvs/:id` | AVV aktualisieren |
| DELETE | `/api/v1/vaktprivacy/avvs/:id` | AVV löschen |

### Datenpannenmeldungen (Art. 33/34 DSGVO)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/vaktprivacy/breaches` | Alle Breach-Records auflisten |
| POST | `/api/v1/vaktprivacy/breaches` | Breach anlegen (startet automatisch Vorfallseintrag in Vakt Comply) |
| GET | `/api/v1/vaktprivacy/breaches/:id` | Einzelnen Breach abrufen |
| PUT | `/api/v1/vaktprivacy/breaches/:id` | Breach aktualisieren |
| DELETE | `/api/v1/vaktprivacy/breaches/:id` | Breach löschen |
| POST | `/api/v1/vaktprivacy/breaches/:id/notify-authority` | Behördenbenachrichtigung als erledigt markieren |
| GET | `/api/v1/vaktprivacy/breaches/:id/notification-pdf` | Meldung als PDF exportieren |

### DSR — Betroffenenrechts-Anfragen (Art. 15–21 DSGVO)

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| GET | `/api/v1/vaktprivacy/dsr` | Alle DSRs auflisten |
| POST | `/api/v1/vaktprivacy/dsr` | DSR anlegen (30-Tage-Frist wird automatisch gesetzt) |
| GET | `/api/v1/vaktprivacy/dsrs/export.csv` | DSRs als CSV exportieren |
| PUT | `/api/v1/vaktprivacy/dsr/:id` | DSR-Status aktualisieren |
| DELETE | `/api/v1/vaktprivacy/dsr/:id` | DSR löschen |

## Datenmodelle

### VVTEntry

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `name` | string | Bezeichnung der Verarbeitungstätigkeit |
| `purpose` | string | Zweck der Verarbeitung |
| `legal_basis` | string | Rechtsgrundlage (z. B. Art. 6 Abs. 1 lit. b) |
| `data_categories` | []string | Verarbeitete Datenkategorien |
| `data_subjects` | []string | Betroffene Personengruppen |
| `recipients` | []string | Empfänger der Daten |
| `retention_period` | string | Aufbewahrungsdauer |
| `third_country_transfer` | bool | Drittlandtransfer vorhanden |
| `safeguards` | string | Schutzmaßnahmen bei Drittlandtransfer |
| `status` | string | active / archived |

### AVV

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `processor_name` | string | Name des Auftragsverarbeiters |
| `service_description` | string | Beschreibung der beauftragten Leistung |
| `contract_date` | time | Datum des Vertragsabschlusses |
| `review_date` | time | Datum der nächsten Überprüfung |
| `status` | string | active / expired / terminated |

### Breach

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `title` | string | Bezeichnung der Datenpanne |
| `discovered_at` | time | Zeitpunkt der Entdeckung |
| `authority_deadline_at` | time | Meldepflicht-Frist (72 Stunden nach Entdeckung) |
| `authority_notified_at` | time | Zeitpunkt der tatsächlichen Behördenmeldung |
| `subjects_notification_required` | bool | Betroffene müssen benachrichtigt werden (Art. 34) |
| `affected_count` | int | Anzahl betroffener Personen |
| `data_categories` | []string | Betroffene Datenkategorien |
| `status` | string | open / authority_notified / closed |

### DSR

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | UUID |
| `requester_name` | string | Name des Betroffenen |
| `requester_email` | string | E-Mail-Adresse des Betroffenen |
| `type` | string | access / erasure / portability / objection / rectification |
| `status` | string | open / in_progress / completed / rejected |
| `due_date` | string | Antwortfrist (30 Tage nach received_at, Art. 12 Abs. 3) |
| `received_at` | time | Eingang der Anfrage (Fristbeginn) |
| `completed_at` | time | Zeitpunkt des Abschlusses (optional) |

## Hintergrund-Jobs

| Job | Zeitplan | Beschreibung |
|-----|----------|--------------|
| `vaktprivacy:avv_expiry_check` | Täglich | Abgelaufene AVVs als "expired" markieren und Alerts versenden |
| `vaktprivacy:breach_incident_create` | Bei Breach-Erstellung | Verknüpften Vorfall im Vakt Comply-Vorfallsregister anlegen |

## Compliance-Mapping

| Standard | Abdeckung |
|----------|-----------|
| DSGVO Art. 28 | Auftragsverarbeitung — AVV-Verwaltung mit Ablauf-Tracking |
| DSGVO Art. 30 | Verzeichnis der Verarbeitungstätigkeiten (VVT) |
| DSGVO Art. 33/34 | Datenpannenmeldung an Behörde und Betroffene |
| DSGVO Art. 35 | Datenschutz-Folgeabschätzung (DPIA) |
| DSGVO Art. 15–21 | Betroffenenrechte — DSR mit 30-Tage-Fristen-Tracking |
| NIS2 Art. 21 Abs. 2d | Sicherheit der Lieferkette (AVVs als Nachweis) |
