# Backup & Restore

> ISO 27001 A.8.13 — Informationssicherung  
> Zielgruppe: Systemadministratoren selbst gehosteter Vakt-Instanzen

---

## Überblick

Vakt speichert alle Compliance-Daten in PostgreSQL. Zusätzlich existiert ein
**VAKT\_SECRET\_KEY** (AES-256-Master-Key), der alle im Vakt Vault-Modul gespeicherten
Secrets verschlüsselt. Dieser Key muss separat und sicher aufbewahrt werden —
ohne ihn sind verschlüsselte Vault-Einträge nicht wiederherstellbar.

| Datei | Inhalt | Backup-Methode |
|---|---|---|
| PostgreSQL | Alle Anwendungsdaten | `pg_dump --format=custom` |
| `VAKT_SECRET_KEY` | Vault-Verschlüsselungsschlüssel | AES-256-CBC-verschlüsselt im Archiv |
| Redis | Session-Tokens, Rate-Limit-State | Kein Backup nötig — wird beim Neustart regeneriert |

---

## Schnellstart

### Backup erstellen

```bash
# Im Vakt-Projektverzeichnis ausführen (wo .env liegt)
./scripts/backup.sh /backups/vakt
```

Das Skript:

1. Lädt automatisch `.env` (für `VAKT_DB_URL` und `VAKT_SECRET_KEY`)
2. Erstellt einen PostgreSQL-Custom-Format-Dump (`pg_dump --format=custom --compress=9`)
3. Verschlüsselt den `VAKT_SECRET_KEY` mit einer interaktiv eingegebenen Passphrase (AES-256-CBC)
4. Erzeugt ein signiertes `manifest.json`
5. Packt alles als `vakt-backup-YYYY-MM-DD_HH-MM-SS.tar.gz`

### Backup verifizieren (Dry-Run)

```bash
./scripts/backup-verify.sh /backups/vakt/vakt-backup-2026-05-18_020000.tar.gz
```

Prüft Archiv-Integrität und Dump-Vollständigkeit, ohne die Datenbank zu
berühren. **Empfohlen: wöchentlich ausführen.**

### Restore durchführen

```bash
./scripts/restore.sh /backups/vakt/vakt-backup-2026-05-18_020000.tar.gz
```

Das Skript entschlüsselt den `VAKT_SECRET_KEY` und fragt ihn ab, gibt ihn aus,
und stellt dann die Datenbank mit `pg_restore --clean --if-exists` wieder her.

Nach dem Restore:

```bash
# .env mit dem ausgegebenen VAKT_SECRET_KEY aktualisieren
# Dann Stack neu starten:
docker compose up -d
```

---

## Geplantes Backup (Cron)

Auf dem Produktivserver als `root` oder dem Deploy-Nutzer einrichten
(`crontab -e`):

```cron
# Täglich um 02:00 Uhr, Passphrase aus Schlüsseldatei lesen
0 2 * * * cd /opt/vakt && ./scripts/backup.sh /backups/vakt >> /var/log/vakt-backup.log 2>&1
```

> **Hinweis zum nicht-interaktiven Betrieb:** Das Backup-Skript liest die
> Passphrase interaktiv. Für vollautomatische Cron-Backups ohne Passphrase-Eingabe
> legen Sie den `VAKT_SECRET_KEY` als separates verschlüsseltes Geheimnis in
> Ihrem Secrets-Manager (z.B. HashiCorp Vault, AWS Secrets Manager) ab und
> passen Sie das Skript an Ihre Infrastruktur an.

---

## Aufbewahrungsempfehlung

| Umgebung | Häufigkeit | Aufbewahrung |
|---|---|---|
| Produktion | Täglich | 30 Tage |
| Staging | Wöchentlich | 4 Wochen |

Ältere Backups automatisch löschen (Beispiel für 30 Tage):

```bash
find /backups/vakt -name "vakt-backup-*.tar.gz" -mtime +30 -delete
```

---

## Backup-Test (quartalsweise, ISMS-Nachweis)

ISO 27001 A.8.13 fordert regelmäßige Restore-Tests. Vorgehen:

1. Backup vom Produktivsystem erstellen
2. Auf einer Staging-Instanz einspielen:
   ```bash
   ./scripts/restore.sh /backups/vakt/vakt-backup-DATUM.tar.gz
   ```
3. Datenintegrität prüfen: Anzahl Controls, Risiken und Incidents vergleichen
4. Testergebnis als Audit-Eintrag in Vakt (Vakt Comply → Interne Audits) dokumentieren

---

## Sicherheitshinweise

- **VAKT\_SECRET\_KEY nicht im Klartext im Backup-Archiv speichern** — das Skript
  verschlüsselt ihn standardmäßig mit Ihrer Passphrase.
- **Passphrase separat aufbewahren** — z.B. in einem Passwort-Manager oder
  physisch gesichertem Tresor. Ohne die Passphrase ist der Key und damit das
  Vault-Modul nicht wiederherstellbar.
- **Backup-Medien verschlüsseln** — bei Übertragung auf externe Träger
  (`rsync`, S3 etc.) zusätzliche Transportverschlüsselung verwenden.
- **Zugriffsrechte** — Backup-Verzeichnis nur für den Backup-Nutzer lesbar:
  ```bash
  chmod 700 /backups/vakt
  ```

---

## Externe Backup-Übertragung (optional)

Das Backup-Skript selbst überträgt keine Daten — kein Cloud-Zwang, kein
Phone-Home. Übertragung auf externe Medien liegt in der Verantwortung des
Administrators:

```bash
# Beispiel: rsync auf NFS-Share
rsync -av /backups/vakt/ backup-server:/mnt/offsite/vakt/

# Beispiel: rclone auf S3-kompatiblen Storage (self-hosted MinIO)
rclone copy /backups/vakt/ minio:vakt-backups/
```

---

## Skript-Referenz

| Skript | Zweck |
|---|---|
| `scripts/backup.sh [output-dir]` | Vollständiges Backup (DB + Key) |
| `scripts/restore.sh <backup-file.tar.gz>` | Wiederherstellung |
| `scripts/backup-verify.sh <backup-file.tar.gz>` | Archiv-Verifikation ohne DB-Eingriff |

Alle Skripte lesen `.env` automatisch, wenn die Datei im aktuellen Verzeichnis
liegt.
