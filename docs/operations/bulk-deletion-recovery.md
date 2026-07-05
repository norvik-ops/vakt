# Runbook: Bulk-Deletion Recovery

Dieses Runbook beschreibt, wie nach einer versehentlichen Massen-Löschung von Datensätzen
vorgegangen wird. Typische Szenarien: Falsches SQL im Adminbereich, fehlerhafte Migration,
oder ein Bug im Lösch-Endpoint ohne korrekte `org_id`-Filterung.

---

## Erkennung

### Welche Tabellen sind typischerweise betroffen?

| Modul | Tabelle | Typ |
|-------|---------|-----|
| Vakt Comply | `ck_controls` | Compliance-Controls |
| Vakt Comply | `ck_gaps` | Lücken-Assessments |
| Vakt Comply | `ck_evidence` | Audit-Evidenz-Einträge |
| Vakt Comply | `ck_incidents` | Incident-Register |
| Vakt Scan | `vb_findings` | Scanner-Findings |
| Vakt HR | `hr_employees` | Mitarbeiter-Verzeichnis |
| Vakt HR | `hr_checklist_runs` | Onboarding/Offboarding-Runs |
| Vakt Privacy | `po_breaches` | DSGVO-Breach-Einträge |
| Vakt Privacy | `po_vvt_entries` | VVT-Einträge (Art. 30) |

### Zeilenanzahl prüfen

```sql
-- Jeweils in psql ausführen:
SELECT COUNT(*) FROM ck_controls;
SELECT COUNT(*) FROM vb_findings;
SELECT COUNT(*) FROM hr_employees;
SELECT COUNT(*) FROM po_breaches;
```

Baseline-Werte sollten aus regulären Backups oder dem letzten bekannten Stand bekannt sein.

### Audit-Log auf DELETE-Operationen prüfen

```sql
SELECT
    id,
    action,
    entity_type,
    entity_id,
    user_id,
    org_id,
    created_at
FROM audit_log
WHERE action = 'DELETE'
  AND created_at > NOW() - INTERVAL '24 hours'
ORDER BY created_at DESC
LIMIT 50;
```

Wenn `VAKT_AUDIT_SYSLOG_ADDR` gesetzt ist, erscheinen DELETE-Aktionen auch im Syslog-Stream.

### Soft-Delete-Tabellen: gelöschte Records zählen

Einige Tabellen verwenden Soft-Deletes (`deleted_at IS NOT NULL`):

```sql
-- Kürzlich soft-gelöschte Controls
SELECT COUNT(*) FROM ck_controls WHERE deleted_at > NOW() - INTERVAL '1 hour';

-- Alle soft-gelöschten Einträge einer Org
SELECT COUNT(*) FROM ck_controls
WHERE org_id = '<betroffene_org_id>'
  AND deleted_at IS NOT NULL;
```

Für Hard-Delete-Tabellen (kein `deleted_at`): Die Datensätze sind weg — Recovery nur aus Backup möglich.

---

## Recovery-Strategie

### Option A: Vollständige Restore aus Backup

Bei großflächiger Löschung die sicherste Option. Vor jeder Aktion:

**App stoppen:**

```bash
docker compose stop api worker
```

**Backup einspielen:** Siehe [`backup-restore.md`](backup-restore.md) für die vollständige Prozedur.

**App neu starten:**

```bash
docker compose up -d api worker
```

### Option B: Partieller Recovery (empfohlen bei isolierter Löschung)

Ziel: Nur die gelöschten Zeilen einer bestimmten Tabelle wiederherstellen, ohne die gesamte DB zurückzurollen.

**Schritt 1 — Temporäre Datenbank aus Backup aufbauen:**

```bash
# Backup aus backup-restore.md beschaffen
# Neue temp DB anlegen
docker compose exec postgres createdb -U vakt vakt_recovery

# Backup einspielen
docker compose exec -T postgres psql -U vakt -d vakt_recovery < /path/to/backup.sql
```

**Schritt 2 — Fehlende Zeilen identifizieren:**

```sql
-- In vakt_recovery-DB: Zeilen, die in der Prod-DB fehlen
SELECT r.* FROM vakt_recovery.ck_controls r
LEFT JOIN ck_controls p ON r.id = p.id
WHERE p.id IS NULL
  AND r.org_id = '<betroffene_org_id>';
```

**Schritt 3 — Daten zurückkopieren:**

```sql
-- Aus vakt_recovery in Prod-DB
INSERT INTO ck_controls
SELECT * FROM vakt_recovery.ck_controls
WHERE id NOT IN (SELECT id FROM ck_controls)
  AND org_id = '<betroffene_org_id>';
```

**Schritt 4 — Temporäre DB aufräumen:**

```bash
docker compose exec postgres dropdb -U vakt vakt_recovery
```

### Option C: Soft-Delete rückgängig machen

Wenn die Tabelle Soft-Delete unterstützt (`deleted_at`-Column):

```sql
-- Alle innerhalb der letzten Stunde soft-gelöschten Controls einer Org wiederherstellen
UPDATE ck_controls
SET deleted_at = NULL
WHERE org_id = '<betroffene_org_id>'
  AND deleted_at > NOW() - INTERVAL '1 hour';
```

---

## Verifizierungs-Queries nach Recovery

```sql
-- Zeilenanzahl nach Recovery (sollte Baseline matchen)
SELECT COUNT(*) FROM ck_controls WHERE org_id = '<betroffene_org_id>';
SELECT COUNT(*) FROM vb_findings WHERE org_id = '<betroffene_org_id>';
SELECT COUNT(*) FROM hr_employees WHERE org_id = '<betroffene_org_id>';

-- Soft-Delete-Status prüfen (keine ungewollten Löschungen offen)
SELECT COUNT(*) FROM ck_controls
WHERE org_id = '<betroffene_org_id>'
  AND deleted_at IS NOT NULL;

-- Letzte erfolgreiche Imports/Erstellungen sichtbar?
SELECT id, created_at FROM ck_controls
WHERE org_id = '<betroffene_org_id>'
ORDER BY created_at DESC
LIMIT 10;
```

---

## Managed-Hosting: Kunden-Datenisolation

Jeder Kunde hat seine eigene `org_id`. Bei Managed-Hosting-Deployments **darf Recovery-SQL
niemals ohne `org_id`-Filter ausgeführt werden**.

**Betroffene Org-ID identifizieren:**

```sql
SELECT id, name, slug FROM organisations WHERE slug = '<kunden-slug>';
```

**Alle Recovery-Queries immer mit org_id-Filter:**

```sql
-- KORREKT
SELECT * FROM ck_controls WHERE org_id = '018f4a2b-...';

-- FALSCH — betrifft alle Kunden
SELECT * FROM ck_controls;
```

**Isoliertes Backup für eine einzelne Org:**

```bash
# Nur die Daten einer bestimmten Org exportieren
docker compose exec postgres psql -U vakt -d vakt \
  -c "\COPY (SELECT * FROM ck_controls WHERE org_id = '018f4a2b-...') TO STDOUT CSV HEADER" \
  > controls-backup-org-$(date +%Y%m%d).csv
```

---

## Prävention

**1. Audit-Logging aktivieren:**

```env
VAKT_AUDIT_SYSLOG_ADDR=syslog-host:514
```

Alle DELETE-Operationen werden damit in den Syslog-Stream geschrieben.

**2. Reguläre automatische Backups:**

Backup-Cron einrichten: `scripts/backup-cron.sh` (siehe [`backup-restore.md`](backup-restore.md)).
Empfehlung: stündliche Snapshots auf Off-Site-Storage.

**3. Soft-Delete für kritische Tabellen:**

Für Tabellen, die kritische Compliance-Evidenz speichern (`ck_evidence`, `ck_incidents`),
sollte `deleted_at TIMESTAMPTZ` als Alternative zu Hard-Delete vorhanden sein.
Neue Tabellen in Migrations immer mit `deleted_at`-Column anlegen, wenn Daten sicherheitskritisch sind.

**4. Lösch-Endpoints absichern:**

- Bulk-Löschungen immer mit expliziter Bestätigung in der UI
- Backend: Lösch-Endpoints validieren `org_id` aus Token, niemals aus Query-Parameter

---

*Stand: 2026-06-26*
