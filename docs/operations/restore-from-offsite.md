# Runbook: Restore from Off-Site Backup

**Ziel:** Vollständige Wiederherstellung einer Vakt-Instanz aus einem Off-Site-Backup-Archiv auf einem frischen Server. Ziel-RTO: <30 Minuten.

**Voraussetzungen:**
- Zugriff auf das Off-Site-Backup-Ziel (S3, NAS, SFTP etc.)
- `VAKT_BACKUP_PASSPHRASE` aus dem Bitwarden Emergency Sheet
- `VAKT_SECRET_KEY` aus dem Bitwarden Emergency Sheet (derselbe Key, der beim Backup aktiv war)

---

## 1. Off-Site-Archiv herunterladen

```bash
# Beispiel für S3:
aws s3 cp s3://my-backup-bucket/vakt/vakt-backup-YYYY-MM-DD.tar.gz.gpg ./
aws s3 cp s3://my-backup-bucket/vakt/vakt-backup-YYYY-MM-DD.tar.gz.gpg.sig ./

# Beispiel für SFTP:
sftp user@backup-host:/vakt/ <<'EOF'
get vakt-backup-YYYY-MM-DD.tar.gz.gpg
get vakt-backup-YYYY-MM-DD.tar.gz.gpg.sig
EOF
```

Neuestes Archiv wählen — Dateiname enthält Datum (`YYYY-MM-DD`).

---

## 2. Signatur prüfen (optional aber empfohlen)

```bash
# Signatur gegen den öffentlichen Key prüfen
# Der Public Key liegt in docs/operations/backup-public.asc (kein Geheimnis)
gpg --import docs/operations/backup-public.asc
gpg --verify vakt-backup-YYYY-MM-DD.tar.gz.gpg.sig vakt-backup-YYYY-MM-DD.tar.gz.gpg
```

---

## 3. Archiv entschlüsseln und entpacken

```bash
# Passphrase aus Bitwarden Emergency Sheet
export PASSPHRASE="<VAKT_BACKUP_PASSPHRASE>"

gpg --batch --passphrase "$PASSPHRASE" \
    --output vakt-backup-YYYY-MM-DD.tar.gz \
    --decrypt vakt-backup-YYYY-MM-DD.tar.gz.gpg

tar -xzf vakt-backup-YYYY-MM-DD.tar.gz
# Erzeugt: vakt-backup-YYYY-MM-DD/
#   ├── vakt.dump          (pg_dump custom format)
#   ├── uploads/           (evidence files)
#   └── backup.env         (Snapshot der env vars ohne Secrets)
ls -lh vakt-backup-YYYY-MM-DD/
```

---

## 4. Frische Instanz aufsetzen

```bash
# Repo klonen und .env vorbereiten
git clone https://github.com/norvik-ops/vakt.git /opt/vakt
cd /opt/vakt
cp .env.example .env

# Wichtig: DENSELBEN VAKT_SECRET_KEY wie beim Backup setzen!
# Aus Bitwarden Emergency Sheet
nano .env
# Setze: VAKT_DB_URL, VAKT_REDIS_URL, VAKT_SECRET_KEY, POSTGRES_PASSWORD, REDIS_PASSWORD

# Infrastruktur hochfahren OHNE auto-migrate und OHNE API
docker compose up -d vakt-db redis
sleep 10   # DB-Start abwarten
```

---

## 5. Datenbank wiederherstellen

```bash
# Laufenden Container-Namen herausfinden
DB_CONTAINER=$(docker compose ps -q vakt-db)

# Dump einspielen
docker exec -i "$DB_CONTAINER" \
    pg_restore -U vakt -d vakt --clean --if-exists \
    < vakt-backup-YYYY-MM-DD/vakt.dump

# Restore-Status prüfen
docker exec "$DB_CONTAINER" \
    psql -U vakt -d vakt -c "SELECT COUNT(*) FROM users;"
```

---

## 6. Evidence-Uploads wiederherstellen

```bash
# Upload-Volume-Pfad auf dem Host
UPLOADS_VOL=$(docker volume inspect vakt_uploads \
    --format '{{ .Mountpoint }}')

# Backup-Uploads einspielen
sudo cp -a vakt-backup-YYYY-MM-DD/uploads/. "$UPLOADS_VOL/"
sudo chown -R 65532:65532 "$UPLOADS_VOL/"
```

---

## 7. Vollständigen Stack starten

```bash
docker compose up -d
docker compose logs -f vakt-api   # Auf "listening" warten
```

---

## 8. Funktionstest

```bash
# Health-Check
curl -s http://localhost/health | jq '.version, .demo'

# Login-Test (Admin-Credentials aus backup.env oder letztem bekannten Stand)
curl -sX POST http://localhost/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"email":"admin@example.com","password":"<admin-pass>"}' \
    | jq '.user.email'
```

---

## Troubleshooting

**pg_restore gibt Fehler "role does not exist":**
```bash
docker exec "$DB_CONTAINER" psql -U postgres -c "CREATE ROLE vakt LOGIN;"
# dann pg_restore nochmal
```

**Secrets werden nicht entschlüsselt (z.B. SMTP-Passwort als "[encrypted]"):**
VAKT_SECRET_KEY stimmt nicht mit dem Key überein, der beim Backup aktiv war.
Aus Bitwarden Emergency Sheet den richtigen Key für das Backup-Datum holen.

**Upload-Pfad fehlt:**
```bash
# Überprüfen ob Volume erstellt wurde
docker volume ls | grep vakt_uploads
# Falls nicht: docker volume create vakt_uploads  
```

---

## Timing (Referenzwerte)

| Schritt | Dauer (ca.) |
|---------|-------------|
| Download (10 GB Off-Site) | 5–10 min |
| Entschlüsseln + Entpacken | 1–2 min |
| Infrastruktur starten | 1 min |
| pg_restore (5 GB DB) | 5–8 min |
| Uploads kopieren | 2–5 min |
| Stack starten + Health-Check | 2 min |
| **Gesamt** | **~20–28 min** |

RTO-Ziel von 30 Minuten ist erreichbar.

---

*Letzte Überarbeitung: 2026-06-26 (S107-6)*
