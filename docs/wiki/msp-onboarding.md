# Vakt für MSPs — Onboarding-Guide

Dieser Guide richtet sich an Managed Security Provider (MSPs), die Vakt für ihre Kunden
betreiben. Er beschreibt das empfohlene Deployment-Modell und die wichtigsten
Betriebsaspekte.

---

## 1. Deployment-Modell

**Eine Instanz pro Kunde. Immer.**

Vakt ist ein Self-Hosted-Produkt ohne zentrales MSP-Portal. Jeder Kunde bekommt seine
eigene, vollständig isolierte Vakt-Instanz — mit eigenem Docker-Compose-Stack oder
Helm-Release, eigener PostgreSQL-Datenbank und eigenem Redis.

```
MSP-Infrastruktur
├── Kunde A  →  vakt.kunde-a.example.com  (eigener Stack)
├── Kunde B  →  vakt.kunde-b.example.com  (eigener Stack)
└── Kunde C  →  vakt.kunde-c.example.com  (eigener Stack)
```

**Warum keine geteilte Instanz?**
Vakt enthält hochsensible Compliance-Daten (Sicherheitslücken, DSGVO-Verfahrensverzeichnisse,
Audit-Nachweise, Incident-Register). Eine geteilte Datenbank würde das Mandanten-Trennungsversprechen
brechen und erfordert AVVs mit jedem betroffenen Kunden. Eine Instanz pro Kunde ist das einzige
Modell, das das Kern-Versprechen ("alle Daten bleiben in der Infrastruktur des Kunden")
vollständig erfüllt.

---

## 2. Schnellstart pro Kunde

### Voraussetzungen

- Docker + Docker Compose v2 (oder Kubernetes + Helm 3)
- Mindestens 2 CPU-Cores, 4 GB RAM, 20 GB Disk pro Instanz
- Erreichbare Domain + TLS-Zertifikat (Let's Encrypt empfohlen)
- Pro-License-Key für den Kunden (siehe Abschnitt 4)

### `.env`-Vorlage

Kopiere `.env.example` aus dem Repository und befülle die Pflichtfelder:

```env
# Pflicht
VAKT_DB_URL=postgres://vakt:SICHERES_PASSWORT@postgres:5432/vakt
VAKT_REDIS_URL=redis://:REDIS_PASSWORT@redis:6379

# 32-Byte Hex — pro Instanz individuell generieren:
# openssl rand -hex 32
VAKT_SECRET_KEY=<32-byte-hex>

# Domain dieser Instanz (ohne trailing Slash)
VAKT_BASE_URL=https://vakt.kunde-a.example.com

# Lizenz
VAKT_LICENSE_KEY=<pro-license-key-kunde-a>

# Optionale AI-Features (Ollama lokal oder externer Anbieter)
VAKT_AI_PROVIDER=disabled

# SMTP für Vakt Aware Phishing-Simulationen
VAKT_SMTP_HOST=
VAKT_SMTP_PORT=587
VAKT_SMTP_FROM=vakt@kunde-a.example.com
```

### Starten

```bash
# Repository klonen (oder als Zip-Archiv herunterladen)
git clone https://github.com/norvik-ops/vakt.git vakt-kunde-a
cd vakt-kunde-a

# .env befüllen (aus Vorlage)
cp .env.example .env
# vim .env  ← Pflichtfelder ausfüllen

# Starten
docker compose up -d

# Status prüfen
docker compose ps
curl -s http://localhost/health | jq .
```

Migrations laufen beim ersten Start automatisch. Die Plattform ist danach unter
`http://localhost` (bzw. über den Reverse-Proxy unter der konfigurierten Domain) erreichbar.

**Erster Login:** Vakt erstellt beim ersten Start einen Admin-Account. Die Credentials
werden einmalig in den Logs ausgegeben:

```bash
docker compose logs api | grep "initial admin"
```

---

## 3. Kunden-Isolation sicherstellen

Jede Kunden-Instanz benötigt vollständig getrennte Infrastruktur:

| Komponente | Empfehlung |
|------------|------------|
| PostgreSQL | Separate Datenbank-Instanz (nicht nur separater Schema-Name) |
| Redis | Separate Redis-Instanz (kein DB-Index-Sharing) |
| `VAKT_SECRET_KEY` | Individuell pro Instanz — niemals wiederverwenden |
| Netzwerk | Jeder Stack in eigenem Docker-Netzwerk oder Kubernetes-Namespace |
| Backups | Separate Backup-Jobs und Backup-Destinations pro Kunde |

**Secret-Key-Generierung:**
```bash
# Einen neuen Key für jeden Kunden erzeugen
openssl rand -hex 32
```

Der `VAKT_SECRET_KEY` verschlüsselt alle in der Datenbank gespeicherten Secrets (Vault-Einträge,
API-Keys, Webhooks) mit AES-256-GCM. Ein verlorener Key macht die verschlüsselten Daten
dauerhaft unlesbar. Key sicher aufbewahren — außerhalb der Vakt-Instanz (z.B. in einem
Passwort-Manager oder einem KMS).

---

## 4. License-Key pro Instanz

Jede Kunden-Instanz benötigt einen eigenen Pro-License-Key. License-Keys sind nicht
zwischen Instanzen teilbar.

- **Community-Edition:** Kein License-Key erforderlich. Funktionsumfang eingeschränkt
  (kein SCIM/SIEM, keine White-Label-Reports, kein OIDC/OAuth2-SSO).
- **Pro-Edition:** License-Key über das Vakt-Reseller-Programm beziehen. Der Key wird
  in `VAKT_LICENSE_KEY` eingetragen und beim Start validiert (offline-fähig via
  signiertes Token).

License-Keys werden pro Instanz ausgestellt und sind an die Domain der Instanz gebunden.
Bei Domain-Änderungen neuen Key beantragen.

---

## 5. Branding

### Organisations-Logo

Im Admin-Bereich unter **Einstellungen → Organisation → Erscheinungsbild** kann das
Kunden-Logo hochgeladen werden. Das Logo erscheint in der Navigationsleiste und in
PDF-Exporten.

Format: PNG oder SVG, empfohlen 200×60 px, transparent.

### White-Label-PDF-Export (Pro)

Mit einer Pro-Lizenz können PDF-Reports (Compliance-Berichte, Audit-Nachweise, Assessment-Reports)
mit dem Kunden-Logo und einem konfigurierbaren Fußzeilentext versehen werden. Konfiguration
im Admin-Bereich unter **Einstellungen → Berichte**.

Das Vakt-Logo und der Vakt-Produktname bleiben in der Standard-Edition sichtbar.
White-Labeling ist ausschließlich für den MSP-Einsatz im Rahmen der Pro-Lizenz vorgesehen
und nicht für den Weiterverkauf als eigenständiges Produkt (ELv2-Lizenz).

---

## 6. Updates

### Docker Compose

```bash
# Aktuelles Verzeichnis: vakt-kunde-a/
docker compose pull
docker compose up -d

# Migrations werden automatisch beim Start ausgeführt
# Status nach Update prüfen:
docker compose ps
curl -s http://localhost/health | jq .version
```

### Helm (Kubernetes)

```bash
helm repo update
helm upgrade vakt vakt/vakt \
  --namespace vakt-kunde-a \
  --values values-kunde-a.yaml
```

### Vor jedem Update

Backup anlegen (siehe Abschnitt 7). Bei Major-Versions-Sprüngen (z.B. v0.x → v1.x)
`UPGRADE.md` im Repository lesen — dort sind Breaking Changes und manuelle Migrations-Schritte
dokumentiert.

---

## 7. Backup & Restore

### Backup erstellen

```bash
# PostgreSQL-Dump + .env-Keys sichern
make backup
# Legt vakt-backup-YYYY-MM-DD.tar.gz im aktuellen Verzeichnis ab
# Enthält: pg_dump, Redis-RDB-Snapshot, .env (ohne .env.secret-Abschnitt)
```

Das Backup beinhaltet:
- PostgreSQL-Dump (alle Vakt-Tabellen)
- Redis-RDB-Snapshot (Queue-State, Cache)
- `.env`-Datei (ohne Passwörter — diese separat sichern!)

**VAKT_SECRET_KEY separat sichern** — ohne diesen Key sind verschlüsselte Vault-Daten
im Backup unlesbar. Key niemals im selben Backup wie die verschlüsselten Daten ablegen.

### Backup wiederherstellen

```bash
make restore BACKUP=vakt-backup-2026-05-23.tar.gz
```

Restore stoppt die laufenden Container, spielt PostgreSQL-Dump ein, lädt Redis-Snapshot
und startet die Container neu. Migrations werden erneut ausgeführt (idempotent).

### Backup-Empfehlung

| Kunde | Backup-Frequenz | Aufbewahrung |
|-------|-----------------|--------------|
| Aktive Nutzung | Täglich | 30 Tage lokal, 90 Tage offsite |
| Compliance-kritisch | Täglich + nach jeder Änderung | 1 Jahr offsite |

Backups offsite ablegen — nicht auf derselben VM wie die Vakt-Instanz.

---

## 8. Monitoring

### Health-Endpoint

```bash
curl -s https://vakt.kunde-a.example.com/health | jq .
# {
#   "status": "ok",
#   "version": "1.0.0",
#   "demo": false,
#   "sso_enabled": false
# }
```

Wenn `status` nicht `"ok"` ist oder der Endpoint nicht antwortet, ist die Instanz
nicht betriebsbereit. Der Endpoint gibt keinen 200-Status bei DB-Fehler — alle kritischen
Abhängigkeiten werden geprüft.

### Empfohlene Monitoring-Checks

| Check | Intervall | Alert wenn |
|-------|-----------|------------|
| `GET /health` → `status: ok` | 1 min | Antwort fehlt oder `status != "ok"` |
| Container-Status (`docker compose ps`) | 5 min | Container `Restarting` oder `Exited` |
| Disk-Usage PostgreSQL-Volume | 15 min | > 80% |
| Redis-Memory | 15 min | > 80% `maxmemory` |

### Watchtower (optionales Auto-Update)

Für Kunden, die automatische Patch-Updates bevorzugen:

```yaml
# docker-compose.override.yml
services:
  watchtower:
    image: containrrr/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: --interval 86400 --cleanup vakt-api vakt-worker
```

Watchtower prüft täglich auf neue Images und aktualisiert automatisch. Nur empfehlenswert
für Patch-Updates (gleiche Minor-Version). Vor Major-Version-Updates immer manuell prüfen
und Backup anlegen.

---

## 9. Support-Abgrenzung

| Bereich | Verantwortung |
|---------|---------------|
| VM/Cloud-Infrastruktur, Netzwerk, TLS, Firewalls | MSP |
| Docker/Kubernetes-Betrieb, Backup-Jobs, Monitoring | MSP |
| Betriebssystem-Updates, Host-Security | MSP |
| Vakt-Produkt-Bugs, Feature-Anfragen | Vakt-Support |
| Vakt-Migrations-Fehler nach Update | Vakt-Support |
| Kunden-Konfiguration in Vakt (Org-Settings, User-Management) | MSP (1st Level) |

**Vakt-Support kontaktieren:**
- GitHub Issues: [github.com/norvik-ops/vakt/issues](https://github.com/norvik-ops/vakt/issues)
- Für Sicherheitslücken: `security@vakt.io` (nicht öffentlich melden)

Bei einem Support-Ticket bitte immer folgende Informationen beifügen — am
einfachsten als **Diagnose-Bundle** (`make support-bundle` erzeugt ein Archiv
mit Version, Health und Logs aller Services; siehe [Support & Diagnose](support.md)):
- Vakt-Version (`curl /health | jq .version`)
- Deployment-Art (Docker Compose / Helm / Kubernetes-Version)
- Relevante Logs (`docker compose logs api --tail=100` oder `make support-bundle`)
- Fehlermeldung im Browser-Konsole (falls Frontend-Problem)

---

## Checkliste: Neue Kunden-Instanz

- [ ] Domain + TLS-Zertifikat eingerichtet
- [ ] `.env` aus Vorlage befüllt (alle Pflichtfelder)
- [ ] `VAKT_SECRET_KEY` individuell generiert und sicher gespeichert
- [ ] PostgreSQL und Redis als separate Instanzen bereitgestellt
- [ ] `docker compose up -d` ausgeführt, Health-Endpoint antwortet
- [ ] Initialer Admin-Account-Passwort gesichert
- [ ] Pro-License-Key eingetragen (falls Pro-Features benötigt)
- [ ] Kunden-Logo hochgeladen
- [ ] Backup-Job konfiguriert
- [ ] Monitoring-Check auf `/health` eingerichtet
- [ ] Kunden erhalten Zugangsdaten und Kurzeinführung
