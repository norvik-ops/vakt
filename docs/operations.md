# Vakt — Betriebsdokumentation

Zielgruppe: DACH-Systemadministratoren, die eine selbst gehostete Vakt-Instanz betreiben.

---

## 0. SLA-Übersicht (RTO/RPO)

Vakt ist **self-hosted** — die SLAs werden durch das Betriebsmodell des Kunden selbst bestimmt. Vakt liefert die Mechanik (Backups, Healthchecks, Migrations), nicht die Garantien. Die unten genannten Zielwerte gelten für den **Default-Single-Node-Betrieb** mit täglichem Backup; eine Multi-Node-HA-Variante (Patroni + Redis-Sentinel) ist in `docs/roadmap-langfristig.md` als Q2 2027 vorgesehen.

| Szenario | RTO | RPO | Bemerkung |
|---|---|---|---|
| API-/Worker-Container-Crash | 30 s | 0 | Docker-Restart-Policy `unless-stopped`; Asynq-Jobs werden retried |
| Redis-Datenverlust | 2 min | 0 | Nur Cache + Queue, kein Persistent-Datenverlust |
| DB-Korruption (Single-Node) | 30–60 min | bis 24 h | Letztes tägliches Backup (`scripts/backup.sh`) |
| Vollständiger Serververlust | 1–2 h | bis 24 h | Re-Deploy + Backup-Restore |
| Kubernetes-Pod-Eviction (Helm) | 60 s | 0 | PodDisruptionBudget + Replicas ≥ 2 |
| Region-Outage (Cloud) | nicht garantiert | nicht garantiert | Erst mit HA-Story möglich; aktuell „run another instance in another region" |

**Empfehlungen für Customer-SLA-Definitionen:**

- Tägliches Backup ist Pflicht. Wer einen RPO < 24 h braucht, muss WAL-Archiving (Point-in-Time-Recovery) auf Postgres aktivieren — siehe Abschnitt 1.4 ("PITR optional").
- Wer einen RTO < 15 min braucht, muss eine zweite Vakt-Instanz als Hot-Standby vorhalten und manuell promoten. Eine native HA-Lösung (Patroni) kommt mit dem Q2-2027-Roadmap-Punkt.
- Cloud-Provider-Outages sind durch Multi-Region-Deployment des Kunden zu adressieren, nicht durch Vakt selbst.

Sprint 15 / S15-15: diese Übersicht ergänzt die Detail-Sektionen weiter unten.

---

## 1. Backup und Wiederherstellung

### Backup erstellen

Das beiliegende Skript sichert die PostgreSQL-Datenbank und (optional) den
Upload-Ordner. Es verschlüsselt das Archiv mit AES-256 und einer selbst
gewählten Passphrase.

```bash
# Tägliches vollständiges Backup
bash scripts/backup.sh \
  --db-url "postgres://vakt:PASSWORT@localhost:5432/vakt" \
  --upload-dir /var/vakt/uploads \
  --output-dir /mnt/backup/vakt \
  --passphrase "$(cat /etc/vakt/backup.key)"
```

Backups landen standardmäßig in `/mnt/backup/vakt/` als
`vakt-backup-YYYY-MM-DD.tar.gz.enc`.

**Empfohlener Zeitplan:** Täglich um 02:00 Uhr per Cron, Aufbewahrung 30 Tage.

```cron
0 2 * * *  root  bash /opt/vakt/scripts/backup.sh ... >> /var/log/vakt-backup.log 2>&1
```

### Wiederherstellung testen (Dry-Run)

```bash
bash scripts/restore.sh \
  --backup-file /mnt/backup/vakt/vakt-backup-2026-05-17.tar.gz.enc \
  --passphrase "$(cat /etc/vakt/backup.key)" \
  --dry-run
```

`--dry-run` entpackt und entschlüsselt das Archiv, spielt aber nichts in die
Datenbank ein. Regelmäßiger Dry-Run-Test (empfohlen: wöchentlich) stellt
sicher, dass das Backup lesbar und vollständig ist.

### Wiederherstellung auf neuem Server

1. Neuen Server provisionieren und Docker + Docker Compose installieren.
2. `.env`-Datei mit identischen Werten (`VAKT_SECRET_KEY`, `POSTGRES_PASSWORD`,
   usw.) übertragen.
3. Backup-Datei auf den neuen Server kopieren.
4. Wiederherstellung ausführen:
   ```bash
   bash scripts/restore.sh \
     --backup-file /path/to/vakt-backup-YYYY-MM-DD.tar.gz.enc \
     --passphrase "$(cat /etc/vakt/backup.key)"
   ```
5. Stack starten: `docker compose up -d`

---

## 2. Upgrade-Prozess

### Normales Upgrade (rolling)

```bash
# 1. Neue Images ziehen
docker compose pull

# 2. Container neu starten (kurze Downtime ~10 s)
docker compose up -d
```

Der `migrate`-Container läuft automatisch vor dem API-Container und wendet alle
neuen Datenbankmigrationen an. Der API-Container startet erst, wenn die
Migration erfolgreich beendet wurde (`depends_on: migrate: condition:
service_completed_successfully`).

### Vor dem Upgrade

- `CHANGELOG.md` auf Breaking Changes prüfen.
- Backup erstellen (siehe Abschnitt 1).
- Bei Major-Versionssprüngen: Upgrade-Anleitung im CHANGELOG lesen.

### Rollback nach fehlgeschlagenem Upgrade

1. Backup aus Schritt „Vor dem Upgrade" einspielen.
2. Altes Image-Tag in der `.env` oder im Compose-Override setzen:
   ```bash
   VAKT_IMAGE_TAG=1.4.2  # vorherige stabile Version
   ```
3. Stack mit altem Tag neu starten:
   ```bash
   docker compose up -d
   ```

> **Hinweis:** Ein Rollback der Datenbank ist nur möglich, wenn das Backup vor
> der Migration erstellt wurde. Migrationen sind nicht automatisch rückgängig
> zu machen.

---

## 3. Schlüssel-Rotation (`VAKT_SECRET_KEY`)

> **Warnung:** `VAKT_SECRET_KEY` ist der Masterschlüssel für alle
> verschlüsselten Geheimnisse in Vakt Vault und TOTP-Secrets. Den Schlüssel
> **nur** rotieren, wenn er kompromittiert wurde oder dies von internen
> Richtlinien vorgeschrieben wird.

### Ablauf mit `make rotate-key`

Das Rotationsskript (`scripts/rotate-key.sh` + `backend/cmd/rotate-key`) übernimmt die Bulk-Re-Verschlüsselung automatisch. Es re-verschlüsselt:

- Alle `so_secrets.encrypted_value` (Vakt Vault-Secrets)
- Alle `totp_secrets.secret` (TOTP-Secrets der Nutzer)

Plaintexts werden ausschließlich im RAM gehalten.

#### Schritt 1: Service stoppen

```bash
docker compose stop api worker
```

#### Schritt 2: Backup erstellen (zwingend vor jeder Rotation)

```bash
make backup
```

#### Schritt 3: Rotation ausführen

```bash
# Automatisch einen neuen Schlüssel generieren:
VAKT_DB_URL="postgres://vakt:PASSWORT@localhost:5432/vakt" \
VAKT_SECRET_KEY="<alter 64-Zeichen-Hex-Schlüssel>" \
  make rotate-key
# → Das Skript gibt den neuen VAKT_SECRET_KEY-Wert aus.

# Oder explizit mit eigenem Schlüssel:
VAKT_DB_URL="postgres://vakt:PASSWORT@localhost:5432/vakt" \
  bash scripts/rotate-key.sh \
    --old-key "<alter Schlüssel>" \
    --new-key "<neuer Schlüssel>"
```

> Das `backend/cmd/rotate-key`-Binary liest die Schlüssel direkt aus den
> Umgebungsvariablen `VAKT_OLD_SECRET_KEY` und `VAKT_NEW_SECRET_KEY` (je
> 64 Hex-Zeichen / 32 Byte). Das Skript oben setzt diese aus `--old-key` /
> `--new-key` — nur bei direktem Aufruf des Binaries selbst setzen.

#### Schritt 4: Schlüssel in der Konfiguration aktualisieren

```bash
# .env oder docker-compose.yml anpassen:
VAKT_SECRET_KEY=<neuer 64-Zeichen-Hex-Schlüssel>
```

#### Schritt 5: Stack neu starten und verifizieren

```bash
docker compose up -d
# Smoke-Test: Vault-Secret lesen und Secret-Wert dekodieren
curl -s -H "Authorization: Bearer <token>" \
  http://localhost/api/v1/vaktvault/projects/<id>/envs/<id>/secrets/<key>
```

#### Schritt 6: Alten Schlüssel vernichten

Den alten Schlüssel aus Passwort-Manager, `.env`-History und Backup-Notizen entfernen.

### Was das Skript NICHT rotiert

- Paseto-Token-Signing-Schlüssel (Paseto-Tokens sind kurzlebig; nach Ablauf erneuern sie sich automatisch)
- Redis-Auth-Passwort (kein Vakt-gemanagtes Secret)
- Postgres-Passwort (kein Vakt-gemanagtes Secret)

### Fehlerbehebung

Wenn das Skript bei einem Secret abbricht mit `cannot decrypt with old key — skipping`:  
Das Secret wurde bereits mit dem neuen Schlüssel verschlüsselt (vorheriger Lauf wurde unterbrochen). Das Skript ist idempotent — gleiche `--new-key`-Eingabe erneut ausführen.

---

## 4. Disaster Recovery

### Datenbankkorruption

1. API und Worker stoppen: `docker compose stop api worker`
2. Aktuellstes Backup einspielen (siehe Abschnitt 1).
3. Stack neu starten: `docker compose up -d`

**RTO:** ca. 30–60 Minuten je nach Backup-Größe und Netzwerkgeschwindigkeit.
**RPO:** Letztes tägliches Backup (max. 24 Stunden Datenverlust).

### Redis-Datenverlust

Redis enthält ausschließlich Cache-Daten und Job-Queues. Alle persistenten Daten
liegen in PostgreSQL.

1. Redis-Container neu starten: `docker compose restart redis`
2. Worker neu starten: `docker compose restart worker`
   Asynq-Jobs, die sich beim Absturz im `active`-Zustand befanden, werden
   automatisch nach dem konfigurierten Timeout erneut eingereiht (Retry-Policy
   greift).

**Kein Datenverlust** durch Redis-Ausfall zu erwarten.

### Vollständiger Serververlust

1. Neuen Server provisionieren (Hardware oder VM).
2. Docker + Docker Compose installieren.
3. Repository klonen bzw. Compose-Dateien und `.env` übertragen.
4. Backup einspielen (siehe Abschnitt 1, „Wiederherstellung auf neuem Server").
5. DNS-Eintrag auf neue IP aktualisieren.
6. Stack starten: `docker compose up -d`
7. TLS-Zertifikat neu ausstellen (Caddy/Let's Encrypt erneuert automatisch).

**RTO:** ca. 1–2 Stunden.
**RPO:** Letztes tägliches Backup.

---

## 5. Monitoring

### Wichtige Metriken

| Metrik | Quelle | Warnschwelle |
|---|---|---|
| DB-Verbindungen aktiv | Prometheus `/metrics` | > 80 % des `max_connections`-Werts |
| Redis Queue-Tiefe (`default`) | Asynq-Inspector / Prometheus | > 500 Jobs in Queue |
| API P95-Antwortzeit | Prometheus `/metrics` | > 2 s |
| Worker-Fehlerrate | Asynq-Inspector | > 5 % der Jobs fehlgeschlagen |
| Festplattenauslastung | Host-Monitoring | > 80 % |

### Health-Endpunkte

```
GET /health          → Liveness (immer 200 solange der Prozess läuft)
GET /health/ready    → Readiness (prüft DB + Redis; 503 bei Ausfall)
GET /api/v1/admin/health  → Detaillierter Status (erfordert Admin-Token)
```

### Log-Forwarding

Vakt schreibt strukturierte JSON-Logs nach stdout (zerolog). Die mitgelieferte
`docker-compose.yml` aktiviert für alle langlebigen Services bereits eine
**Log-Rotation** (`json-file`, `max-size: 10m`, `max-file: 5` → max. ~50 MB pro
Service); volllaufende Disks sind damit ausgeschlossen. Der Log-Level lässt sich
per `VAKT_LOG_LEVEL` (`trace|debug|info|warn|error`, Default `info`) steuern.

Für ein Support-Ticket reicht `make support-bundle` (Logs aller Services +
Health + Versionsinfos als Archiv) — siehe
[Support & Diagnose](wiki/support.md). Für langfristige, zentrale Aufbewahrung
über Container-Neustarts hinweg an einen Log-Aggregator weiterleiten:

```bash
# Beispiel: Logs per docker logs in Loki pipen (Promtail/Alloy)
# oder direkt als Docker-Logging-Driver konfigurieren:
# logging:
#   driver: loki
#   options:
#     loki-url: "http://loki:3100/loki/api/v1/push"
```

Für Cloudwatch, Datadog oder Elastic: `docker logs -f vakt-api-1 | <aggregator-agent>`.

### Empfohlene Alert-Schwellwerte

- **Kritisch:** API antwortet nicht auf `/health` (PagerDuty / SMS)
- **Kritisch:** Datenbankverbindungen erschöpft (> 95 %)
- **Warnung:** Redis Queue-Tiefe > 500 Jobs (kann auf abgestürzten Worker
  hinweisen)
- **Warnung:** Fehlerrate API > 1 % (5xx)
- **Info:** Tägliches Backup fehlgeschlagen

---

## 6. Häufige Probleme

### „API startet nicht"

**Symptom:** Container startet und stoppt sofort; Logs zeigen Fehler beim
Hochfahren.

**Mögliche Ursachen:**

1. **Datenbankverbindung fehlgeschlagen**
   ```
   VAKT_DB_URL not set  /  DB unavailable — all routes disabled
   ```
   Prüfen: `docker compose exec postgres pg_isready -U vakt`

2. **`VAKT_SECRET_KEY` fehlt oder falsche Länge**
   ```
   VAKT_SECRET_KEY is required  /  invalid secret key
   ```
   Der Schlüssel muss ein 64-Zeichen-Hex-String (32 Byte) sein:
   ```bash
   openssl rand -hex 32
   ```

3. **Redis nicht erreichbar**
   ```
   invalid Redis URL — auth/module routes disabled
   ```
   Prüfen: `docker compose exec redis redis-cli ping`

---

### „Migration fehlgeschlagen"

**Symptom:** `migrate`-Container beendet sich mit Exit-Code != 0.

1. **Dirty-Flag prüfen:**
   ```sql
   SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1;
   ```
   Wenn `dirty = true`: Migration wurde unterbrochen.

2. **Dirty-Flag zurücksetzen** (nur wenn die fehlerhafte Migration vollständig
   manuell rückgängig gemacht oder die DB aus einem Backup wiederhergestellt
   wurde):
   ```sql
   UPDATE schema_migrations SET dirty = false WHERE version = <VERSION>;
   ```

3. **Migration erneut ausführen:**
   ```bash
   docker compose run --rm migrate
   ```

---

### „Benutzer kann sich nicht einloggen"

**Symptom:** Login schlägt fehl, obwohl Passwort korrekt ist.

1. **Passwort-Lockout prüfen** (Redis-Key):
   ```bash
   docker compose exec redis redis-cli GET "lockout:<user-id>"
   ```
   Gesetzter Wert = Benutzer ist temporär gesperrt. Key manuell löschen:
   ```bash
   docker compose exec redis redis-cli DEL "lockout:<user-id>"
   ```

2. **MFA-Status prüfen** (in DB):
   ```sql
   SELECT id, email, totp_enabled, is_active FROM users WHERE email = 'user@example.com';
   ```
   Wenn `totp_enabled = true` und der Benutzer keinen TOTP-Code hat: TOTP in
   der Admin-UI deaktivieren oder `totp_secret` auf NULL setzen.

3. **OIDC/SAML:** Wenn SSO konfiguriert ist, prüfen ob Casdoor erreichbar ist:
   ```bash
   curl -s http://casdoor:8000/healthz
   ```

---

### „Lizenz ungültig" / „License invalid"

**Symptom:** API liefert `{"error": "license invalid"}` oder Feature wird nicht
freigeschaltet.

1. **Systemzeit prüfen** — Lizenzen enthalten ein Ablaufdatum; Zeitabweichungen
   > 5 Minuten können zur Ablehnung führen:
   ```bash
   timedatectl status
   # NTP aktiv?
   systemctl status systemd-timesyncd
   ```

2. **Lizenzschlüssel in `.env` prüfen:**
   ```bash
   grep VAKT_LICENSE_KEY .env
   ```
   Schlüssel darf keine Leerzeichen oder Zeilenumbrüche enthalten.

3. **Demo-Modus:** Bei `VAKT_DEMO=true` werden Lizenzen ignoriert — kein
   Fehler zu erwarten.
