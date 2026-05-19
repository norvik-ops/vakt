# Vakt HR — SecHR

**Modul:** SecHR  
**API-Prefix:** `/api/v1/hr`  
**Aktivierung:** `VAKT_MODULES_ENABLED=...,sechr`

SecHR ist das HR-Modul für strukturiertes Onboarding und Offboarding. Es dokumentiert Mitarbeiterlebenszyklen und erzeugt auditfähige Evidenz, dass Zugriffsberechtigungen korrekt vergeben und entzogen wurden.

---

## Features

- **Mitarbeiterverzeichnis** — Anlegen, Bearbeiten und Statusverfolgung (aktiv / offboarding / ausgeschieden)
- **Checklisten-Templates** — Onboarding- und Offboarding-Vorlagen mit beliebig vielen Schritten
- **Checklist Runs** — Ausführungen pro Mitarbeiter mit Fortschrittserfassung (abgeschlossene Schritte, Status)
- **Compliance-Evidenz** — Abgeschlossene Runs fließen automatisch als Evidenz in SecVitals

---

## Mitarbeiter verwalten

### Status-Werte

| Status | Bedeutung |
|--------|-----------|
| `active` | Aktiver Mitarbeiter |
| `offboarding` | Offboarding läuft |
| `terminated` | Ausgeschieden |

### Mitarbeiter anlegen

```bash
curl -X POST /api/v1/hr/employees \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "Anna",
    "last_name": "Müller",
    "email": "a.mueller@example.com",
    "department": "Engineering",
    "role": "Backend-Entwicklerin",
    "start_date": "2026-06-01"
  }'
```

---

## Checklisten

Checklisten sind Templates vom Typ `onboarding` oder `offboarding`. Jeder Schritt hat ein Label und ein optionales `required`-Flag.

### Checklist anlegen

```bash
curl -X POST /api/v1/hr/checklists \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "onboarding",
    "name": "Standard Onboarding",
    "items": [
      { "label": "GitHub-Zugang einrichten", "required": true },
      { "label": "Laptop übergeben", "required": true },
      { "label": "Datenschutz-Schulung absolvieren", "required": true },
      { "label": "Einführungsgespräch HR", "required": false }
    ]
  }'
```

---

## Checklist Runs

Ein Run verknüpft einen Mitarbeiter mit einem Checklist-Template und trackt den Fortschritt.

### Run starten

```bash
curl -X POST /api/v1/hr/checklist-runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "employee_id": "emp-uuid",
    "checklist_id": "checklist-uuid"
  }'
```

### Fortschritt aktualisieren

```bash
curl -X PUT /api/v1/hr/checklist-runs/$RUN_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "completed_items": ["item-uuid-1", "item-uuid-2"],
    "status": "in_progress"
  }'
```

`status` kann `in_progress` oder `completed` sein.

---

## API-Übersicht

| Methode | Pfad | Beschreibung |
|---------|------|--------------|
| `GET` | `/hr/employees` | Mitarbeiterliste (paginiert) |
| `POST` | `/hr/employees` | Mitarbeiter anlegen |
| `GET` | `/hr/employees/:id` | Mitarbeiter abrufen |
| `PUT` | `/hr/employees/:id` | Mitarbeiter aktualisieren |
| `DELETE` | `/hr/employees/:id` | Mitarbeiter löschen |
| `GET` | `/hr/checklists` | Checklist-Templates auflisten |
| `POST` | `/hr/checklists` | Checklist-Template anlegen |
| `DELETE` | `/hr/checklists/:id` | Checklist-Template löschen |
| `POST` | `/hr/checklist-runs` | Run starten |
| `GET` | `/hr/checklist-runs/:id` | Run abrufen |
| `GET` | `/hr/employees/:id/checklist-runs` | Runs eines Mitarbeiters |
| `PUT` | `/hr/checklist-runs/:id` | Fortschritt aktualisieren |

---

## Compliance-Integration

Abgeschlossene Checklist Runs (Status `completed`) werden als Evidenz in SecVitals gespeichert:
- **Typ:** `hr_checklist_completed`
- **Enthält:** Mitarbeitername, Checklist-Name, Abschlusszeitpunkt, abgeschlossene Schritte

Diese Evidenz kann in ISO-27001- und BSI-Grundschutz-Controls verknüpft werden (z.B. A.7 Personalsicherheit).
