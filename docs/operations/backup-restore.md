# Backup & Restore — Operator Guide

> ISO 27001 A.8.13 — Informationssicherung  
> Zielgruppe: Systemadministratoren selbst gehosteter Vakt-Instanzen

Ausführlichere Skript-Referenz: [`docs/backup-restore.md`](../backup-restore.md)

---

## Was wird gesichert?

| Komponente | Inhalt | Backup-Methode |
|---|---|---|
| **PostgreSQL** | Alle Anwendungsdaten (Controls, Findings, Evidence, Incidents, …) | `pg_dump --format=custom` |
| **`uploads_data` Volume** | Hochgeladene Anhänge (Evidence-Dateien, Richtlinien-PDFs, …) | Docker-Volume-Export via `tar` |
| **VAKT_SECRET_KEY** | AES-256-Master-Key für Vault-Secrets | Passphrase-verschlüsselt im Archiv |
| **Redis** | Session-Tokens, Rate-Limit-State | Kein Backup nötig — wird beim Neustart regeneriert |

> **VAKT_SECRET_KEY ist der wichtigste Backup-Inhalt — ohne ihn sind alle im Vakt Vault-Modul gespeicherten Secrets dauerhaft unentschlüsselbar.** Kein Secret-Key-Backup = Datenverlust bei Vault-Inhalten, auch wenn der PostgreSQL-Dump vollständig ist.

---

## Schnellstart-Skripte

Vakt liefert fertige Backup-Skripte in `scripts/`:

```bash
# Backup erstellen (interaktive Passphrase für Key-Verschlüsselung)
cd /opt/vakt
./scripts/backup.sh /backups/vakt

# Backup verifizieren (ohne DB-Eingriff)
./scripts/backup-verify.sh /backups/vakt/vakt-backup-2026-05-24_020000.tar.gz

# Restore durchführen
./scripts/restore.sh /backups/vakt/vakt-backup-2026-05-24_020000.tar.gz
```

---

## Manuelles Backup (ohne Skript)

Falls die Skripte nicht verwendet werden können:

### 1. PostgreSQL-Dump

```bash
# Aus laufendem Container heraus:
docker compose exec postgres pg_dump \
  --username=vakt \
  --format=custom \
  --compress=9 \
  --file=/tmp/vakt.dump \
  vakt

docker compose cp postgres:/tmp/vakt.dump ./vakt-$(date +%Y-%m-%d).dump
```

### 2. Uploads-Volume sichern

```bash
# Evidence-Anhänge aus dem Docker-Volume exportieren
docker run --rm \
  -v uploads_data:/data:ro \
  -v "$(pwd)":/backup \
  alpine:latest tar czf /backup/uploads-$(date +%Y-%m-%d).tar.gz -C /data .
```

### 3. VAKT_SECRET_KEY sichern

```bash
# Key aus .env lesen und verschlüsselt sichern
grep VAKT_SECRET_KEY .env | openssl enc -aes-256-cbc -pbkdf2 \
  -out vakt-key-$(date +%Y-%m-%d).enc
```

### 4. Redis BGSAVE (optional)

Redis-Daten enthalten keine kritischen Persistenzdaten, können aber für Continuity gesichert werden:

```bash
docker compose exec redis redis-cli BGSAVE
# RDB-Datei liegt im Container unter /data/dump.rdb
docker compose cp redis:/data/dump.rdb ./redis-$(date +%Y-%m-%d).rdb
```

---

## Wiederherstellungs-Prozedur

Reihenfolge bei einer vollständigen Wiederherstellung:

1. **Stack stoppen:**
   ```bash
   docker compose down
   ```

2. **Datenbank wiederherstellen:**
   ```bash
   docker compose up -d postgres
   # Warten bis postgres healthy ist
   docker compose cp vakt-2026-05-24.dump postgres:/tmp/vakt.dump
   docker compose exec postgres pg_restore \
     --username=vakt \
     --dbname=vakt \
     --clean --if-exists \
     /tmp/vakt.dump
   ```

3. **Uploads-Volume wiederherstellen:**
   ```bash
   docker volume create uploads_data 2>/dev/null || true
   docker run --rm \
     -v uploads_data:/data \
     -v "$(pwd)":/backup:ro \
     alpine:latest sh -c "cd /data && tar xzf /backup/uploads-2026-05-24.tar.gz"
   ```

4. **VAKT_SECRET_KEY wiederherstellen:**
   ```bash
   # Entschlüsseln:
   openssl enc -d -aes-256-cbc -pbkdf2 -in vakt-key-2026-05-24.enc
   # Ausgabe in .env eintragen: VAKT_SECRET_KEY=<wert>
   ```

5. **Redis-Dump einspielen (optional):**
   ```bash
   docker compose up -d redis
   docker compose cp redis-2026-05-24.rdb redis:/data/dump.rdb
   docker compose restart redis
   ```

6. **Stack starten:**
   ```bash
   docker compose up -d
   # migrate-Container läuft zuerst, dann api/worker
   ```

7. **Smoke-Test:**
   ```bash
   curl http://localhost/health | jq '.status'
   ```

---

## RPO/RTO-Empfehlung für KMU

| | Empfehlung | Begründung |
|---|---|---|
| **RPO** (Recovery Point Objective) | 24 Stunden | Täglicher pg_dump Cron reicht für Compliance-Dokumentation |
| **RTO** (Recovery Time Objective) | 30–45 Minuten | Restore + Migrations + Smoke-Test ohne HA |
| **Backup-Häufigkeit** | Täglich (Cron 02:00) | Kompromiss zwischen Datensicherheit und Aufwand |
| **Test-Restore** | Wöchentlich | Backup-Verifikation via `backup-verify.sh` |
| **Full Restore-Test** | Quartalsweise | ISO 27001 A.8.13 Nachweis; als Audit-Eintrag in Vakt Comply dokumentieren |

---

## Geplantes Backup (Cron)

```cron
# Täglich um 02:00 Uhr (als root oder deploy-Nutzer)
0 2 * * * cd /opt/vakt && ./scripts/backup.sh /backups/vakt >> /var/log/vakt-backup.log 2>&1

# Wöchentliche Verifikation des letzten Backups (Sonntag 03:00)
0 3 * * 0 cd /opt/vakt && ./scripts/backup-verify.sh $(ls -t /backups/vakt/vakt-backup-*.tar.gz | head -1) >> /var/log/vakt-backup-verify.log 2>&1

# Alte Backups nach 30 Tagen löschen
0 4 * * * find /backups/vakt -name "vakt-backup-*.tar.gz" -mtime +30 -delete
```

---

## Sicherheitshinweise

- **VAKT_SECRET_KEY niemals im Klartext auf demselben Medium wie der DB-Dump** — bei Kompromittierung wäre alles verloren.
- **Passphrase separat aufbewahren** — Passwort-Manager oder physisch gesicherter Tresor. Ohne Passphrase ist der Key und damit das Vault-Modul nicht wiederherstellbar.
- **Backup-Medien verschlüsseln** — bei externer Übertragung (rsync, S3) Transportverschlüsselung nutzen.
- **Zugriffsrechte restriktiv halten:**
  ```bash
  chmod 700 /backups/vakt
  chown root:root /backups/vakt
  ```

---

## Weiterführende Ressourcen

- Vollständige Skript-Dokumentation: [`docs/backup-restore.md`](../backup-restore.md)
- Key-Rotation-Prozedur: [`scripts/rotate-key.sh`](../../scripts/rotate-key.sh)
- Upgrade-Prozedur: [`docs/operations/upgrade.md`](upgrade.md)
