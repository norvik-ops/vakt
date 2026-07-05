# Operations — Betriebs-Dokumentation

Operative Runbooks und Betriebsanleitungen für Self-Hoster und Operatoren.
Diese Seite ist der Einstieg; jede Datei deckt einen abgegrenzten Betriebsbereich ab.

## Inhalt

| Dokument | Zweck | Wann lesen |
|----------|-------|-----------|
| [`runbook.md`](runbook.md) | Tägliches Betriebs-Runbook: Health-Checks, Troubleshooting (API 500, OOM, Restart-Loop), Log-Interpretation, **pprof-Profiling** | Bei Störungen / im laufenden Betrieb |
| [`backup-restore.md`](backup-restore.md) | **Backup & Restore** — was gesichert wird (Postgres, `uploads_data`-Volume, `VAKT_SECRET_KEY`), manuelle + skriptbasierte Prozedur, RPO/RTO | Vor Produktivbetrieb (Pflicht) |
| [`upgrade.md`](upgrade.md) | Versions-Upgrade-Prozedur (Pull, `migrate`-Container, Rollback) | Vor jedem Upgrade |
| [`scaling.md`](scaling.md) | Skalierung & Sizing: Single-Instance-Grenzen, vertikal/horizontal, Stateless-Checkliste, Sizing-Tabelle | Bei Wachstum > ~100 Nutzer |
| [`pgbouncer.md`](pgbouncer.md) | Connection-Pooling (pgBouncer Transaction-Mode), Pool-Sizing | Multi-Instance / MSP-Setup |
| [`redis-ha.md`](redis-ha.md) | Redis-Hochverfügbarkeit via Sentinel | HA-Anforderung / SLA |
| [`migration-db-user.md`](migration-db-user.md) | DB-User-Migration / Rechte-Setup | Bei DB-User-Wechsel |
| [`redis-failure.md`](redis-failure.md) | **Redis-Ausfall** — Diagnose, Neustart, Daten-Recovery, Auswirkung auf Asynq-Queue | Bei Redis-Ausfall / Login-500-Fehlern |
| [`migrations-rollback.md`](migrations-rollback.md) | **Migrations-Rollback** — golang-migrate down, dirty-Flag, fehlendes down.sql, Smoke-Test | Nach fehlgeschlagener DB-Migration |
| [`bulk-deletion-recovery.md`](bulk-deletion-recovery.md) | **Bulk-Deletion-Recovery** — Erkennung, Audit-Log, partieller Restore, Managed-Hosting-Isolation | Nach versehentlicher Massen-Löschung |
| [`restore-from-offsite.md`](restore-from-offsite.md) | **Restore aus Off-Site-Backup** — vollständige Wiederherstellung auf frischer Instanz, RTO <30 min | Bei Totalverlust / Datacenter-Ausfall |

## Hierarchie zu anderen Doku-Bereichen

- **Erstinstallation:** [`../wiki/installation.md`](../wiki/installation.md) (nicht hier)
- **Konfigurations-Referenz** (alle Env-Vars): [`../wiki/configuration.md`](../wiki/configuration.md)
- **Disaster-Recovery-Szenarien** (DB-Korruption, Host-Verlust, Key-Rotation): [`../runbooks/disaster-recovery.md`](../runbooks/disaster-recovery.md)
- **Monitoring/Alerting** (Zabbix-Items, Alert-Rules): [`../wiki/monitoring.md`](../wiki/monitoring.md)

> Backup-Skripte (`scripts/backup.sh`, `restore.sh`, `backup-cron.sh`) sind die **Quelle der Wahrheit**;
> die Doku beschreibt sie, weicht aber nie von ihrem Verhalten ab.
