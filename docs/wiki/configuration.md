# Konfigurationsreferenz

Alle Konfigurationswerte werden über Umgebungsvariablen gesetzt. In Docker-Deployments wird die Datei `.env` im Projektverzeichnis verwendet (`env_file: .env` in `docker-compose.yml`). Eine Vorlage aller Variablen mit Kommentaren liegt in `.env.example`.

---

## Datenbank

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_DB_URL` | Ja | — | PostgreSQL-Verbindungsstring. Format: `postgres://user:pass@host:5432/db?sslmode=disable` |
| `VAKT_DB_MAX_CONNS` | — | `25` | Maximale Größe des PostgreSQL-Connection-Pools. Bei mehreren API-/Worker-Replikas ggf. anheben (PgBouncer-Limits beachten). |
| `POSTGRES_PASSWORD` | — | `vakt` | Passwort für den PostgreSQL-Container (wird von `docker-compose.yml` ausgelesen). Muss mit dem Passwort in `VAKT_DB_URL` übereinstimmen. |

**Beispiel:**

```env
VAKT_DB_URL=postgres://vakt:<dein-db-passwort>@postgres:5432/vakt?sslmode=disable
POSTGRES_PASSWORD=<dein-db-passwort>
```

### Pool-Sizing & Multi-Replica (PgBouncer) — wichtig ab 2 Replikas

`VAKT_DB_MAX_CONNS` (Default **25**) ist die Pool-Größe **pro API-/Worker-Prozess**.
Die **Gesamtzahl** der Verbindungen zu Postgres ist:

```
VAKT_DB_MAX_CONNS × (API-Replikas + Worker-Replikas)
```

Dieser Wert muss **unter** der Postgres-`max_connections` (Default **100**) bleiben —
mit Reserve für Migrationen/Admin-Zugriffe. Beispiel: 4 Replikas × 25 = **100** →
überläuft bereits den Default und führt unter Last zu `connection refused`.

**Empfehlung:**
- **1 Replika:** Default 25 ist passend.
- **≥ 2 Replikas:** entweder `VAKT_DB_MAX_CONNS` senken (grob `(max_connections − Reserve) / Replikas`)
  **oder** — bevorzugt — einen **PgBouncer** (Transaction-Pooling) davorschalten und
  `VAKT_DB_URL` auf den PgBouncer zeigen lassen. Der Pool ist bereits
  PgBouncer-Transaction-Mode-kompatibel (`QueryExecModeCacheDescribe`, keine
  serverseitigen Prepared Statements). Im Docker-Compose-Stack ist PgBouncer als
  Sidecar enthalten; in Kubernetes ein externes PgBouncer-Chart nutzen
  (`helm/vakt/values.yaml` → `api.env.VAKT_DB_MAX_CONNS`).

---

## Redis

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_REDIS_URL` | Ja | — | Redis-Verbindungsstring. Format: `redis://host:6379` oder `redis://:passwort@host:6379` |
| `REDIS_PASSWORD` | Ja | — | Passwort für den Redis-Container (`--requirepass`; wird von `docker-compose.yml` ausgelesen). Muss mit dem Passwort in `VAKT_REDIS_URL` übereinstimmen. Von `install.sh` generiert. |

**Beispiel:**

```env
VAKT_REDIS_URL=redis://:<dein-redis-passwort>@redis:6379
REDIS_PASSWORD=<dein-redis-passwort>
```

---

## Sicherheit

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_SECRET_KEY` | Ja | — | 32-Byte Hex-Master-Key für AES-256-GCM-Verschlüsselung aller Secrets in der Datenbank. Generieren: `openssl rand -hex 32`. **Nie nach dem ersten Start ändern.** |
| `VAKT_ADMIN_ALLOWED_IPS` | — | — (offen) | Komma-separierte CIDRs/IPs, die auf Admin-Endpunkte zugreifen dürfen, z. B. `10.0.0.0/8,192.168.1.0/24`. Leer = alle IPs erlaubt. |
| `VAKT_CORS_ORIGINS` | **Prod: Ja** | `http://localhost,http://localhost:5173` | Komma-separierte Liste erlaubter Cross-Origin-Quellen, z. B. `https://vakt.meine-firma.de`. **In Produktion zwingend auf die echte Frontend-Domain setzen.** Der Wert `*` (alle Origins) wird zusammen mit Session-Cookies nur im Demo-Modus (`VAKT_DEMO=true`) akzeptiert — im Nicht-Demo-Modus **bricht der Start mit `*` bewusst ab** (Fail-Closed, S87-2). |
| `VAKT_FORCE_SECURE_COOKIES` | — | `false` | Wenn `true`, tragen alle Session-/CSRF-Cookies das `Secure`-Attribut **unabhängig** von TLS/`X-Forwarded-Proto`. Empfehlung für Produktion hinter einem TLS-terminierenden Reverse-Proxy: `=true` — schützt als hartes Sicherheitsnetz gegen einen fehlkonfigurierten Proxy, der `X-Forwarded-Proto: https` nicht setzt (S87-5, CWE-614). |
| `VAKT_DOMAIN` | — | `localhost` | Domain für den eingebauten **Caddy**-Frontdoor. Auf die öffentliche Domain setzen (z. B. `vakt.example.com`) → Caddy holt und erneuert automatisch ein Let's-Encrypt-Zertifikat (Ports **80+443** müssen aus dem Internet erreichbar sein). Default `localhost` = HTTPS mit lokal signiertem Cert (Tests). `:80` = nur HTTP (Betrieb hinter eigenem TLS-Terminator). Wird von Caddy/Compose gelesen, nicht vom Backend. |
| `VAKT_RATELIMIT_IP_MAX` | — | `50` | Sekundäre IP-Sperre beim Login: wie viele Fehlversuche **von einer einzelnen IP** (egal welche Email) innerhalb von 15 Minuten erlaubt sind. Primäre Sperre ist pro (IP, Email)-Paar (Threshold 10). Erhöhen für Corporate-NAT-Umgebungen (viele Nutzer hinter einer IP), senken für strengere Absicherung. Gilt für `POST /api/v1/auth/login`. |
| `VAKT_TRUSTED_PROXIES` | **Prod: Ja** | — (Compose-Stack: `172.16.0.0/12`) | Komma-separierte CIDR-Liste der Reverse-Proxies, deren `X-Forwarded-For`-Header die API glaubt. **Ohne diesen Wert läuft die API im Direct-IP-Modus**: hinter nginx/Caddy sehen dann alle IP-basierten Schutzmechanismen (Rate-Limits, sekundärer Login-Lockout, `VAKT_ADMIN_ALLOWED_IPS`) nur die Proxy-IP — ein einzelner Angreifer kann so den Login für alle Nutzer sperren. Das Root-`docker-compose.yml` setzt als Default das Docker-Bridge-Netz `172.16.0.0/12`; bei eigenem Proxy dessen IP/Netz eintragen. Nur die Proxy-Adressen eintragen, nie ganze öffentliche Netze (sonst werden XFF-Header spoofbar). |
| `VAKT_AUDIT_SYSLOG_ADDR` | — | — (aus) | **Opt-in.** Ziel `host:port` eines kunden-eigenen Syslog-/SIEM-Servers, an den Audit-Log-Ereignisse (Login, Rollenwechsel, Offboarding, Export …) ausgeleitet werden. Leer = kein ausgehender Traffic. **Datenschutz:** Der Endpunkt wird vom Kunden konfiguriert (analog Outgoing-Webhooks/SMTP) — der Kunde trägt die Verantwortung für die Datenweitergabe. **Kein Norvik-Relay, kein Phone-Home.** Der Audit-Schreibpfad wird nie blockiert (asynchron, Drop-Zähler `vakt_audit_forward_dropped`). |
| `VAKT_AUDIT_SYSLOG_PROTO` | — | `tcp` | Transport: `tcp` oder `tcp+tls` (TLS 1.2+). |
| `VAKT_AUDIT_SYSLOG_FORMAT` | — | `rfc5424` | Nachrichtenformat: `rfc5424` (Syslog) oder `cef` (ArcSight CEF). |
| `VAKT_AUDIT_SYSLOG_ALLOW_PRIVATE` | — | `false` | Erlaubt ein Ziel in privaten/Loopback-Netzen (RFC1918/IMDS), z. B. ein SIEM im selben LAN. Default: solche Ziele werden als SSRF-Schutz abgelehnt. |

**Beispiel:**

```env
VAKT_SECRET_KEY=$(openssl rand -hex 32)   # Beispiel — echten Wert generieren!
VAKT_CORS_ORIGINS=https://vakt.meine-firma.de
VAKT_FORCE_SECURE_COOKIES=true            # Produktion hinter HTTPS-Proxy
```

> **Wichtig:** Der Master-Key wird zur AES-256-GCM-Verschlüsselung aller Secrets (Vakt-Vault-Einträge, SMTP-Passwörter, API-Keys) verwendet. Wird der Key nach dem ersten Deployment geändert, sind alle verschlüsselten Daten dauerhaft unlesbar. Key sicher in einem Passwortmanager oder Vault aufbewahren, niemals in Git committen.

---

## Anwendung

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_API_PORT` | — | `8080` | Port, auf dem der API-Server innerhalb des Containers lauscht. |
| `VAKT_INTERNAL_PORT` | — | `8081` | Interner Port für `/api/v1/internal/*`-Routen (z. B. Backup-Config). Niemals über den Reverse-Proxy exponieren — Docker-interner Zugriff (z. B. `backup-cron.sh`) via `VAKT_INTERNAL_API_URL`. |
| `VAKT_INTERNAL_API_URL` | — | `http://vakt-api:8081` | URL des internen API-Ports. Nur in `backup-cron.sh` relevant — überschreiben wenn das Skript außerhalb des Docker-Netzwerks läuft (z. B. Host-Cron). |
| `APP_VERSION` | — | `0.1.0` | Versionsnummer. Wird im `/health`-Endpunkt zurückgegeben. |
| `VAKT_MODULES_ENABLED` | — | alle | Kommagetrennte Liste aktiver Module. Mögliche Werte: `vaktscan`, `vaktcomply`, `vaktvault`, `vaktaware`, `vaktprivacy`, `vakthr`. |
| `AUTO_MIGRATE` | — | `false` | Wenn `true`, führt der API-Container beim Start automatisch ausstehende Datenbankmigrationen aus. |
| `VAKT_DEMO` | — | `false` | Wenn `true`: Beispieldaten + öffentlich erreichbarer `/api/v1/demo/start`-Endpoint. **Nur für Test-/Demo-Umgebungen — niemals mit echten Compliance-Daten.** |
| `VAKT_FRONTEND_URL` | — | `http://localhost:5173` | Öffentlich erreichbare URL des Frontends. Wird für E-Mail-Links in Benachrichtigungen, Vakt-Aware-Kampagnen und Policy-Akzeptanz-E-Mails verwendet. In Produktion auf die echte Domain setzen. |
| `VAKT_UPLOAD_DIR` | — | `/app/data/uploads` | Verzeichnis für hochgeladene Dateien (Evidence-Anhänge). Im Docker-Compose-Stack als Volume `uploads_data` gemountet — nicht als Host-Pfad ändern. |
| `VAKT_WORKER_CONCURRENCY` | — | `8` | Anzahl paralleler Asynq-Hintergrund-Jobs im Worker-Container. Bei vielen gleichzeitigen Scans/Reports ggf. erhöhen, auf kleinen VMs senken. |
| `WORKER_HEALTH_PORT` | — | `9090` | Port des Worker-internen Health-Endpoints (`/health`, `/health/ready`, `/metrics`). Nur im Docker-Netzwerk erreichbar — nicht extern exponiert. |
| `WORKER_REPLICAS` | — | `1` | Anzahl Worker-Replicas für horizontale Skalierung (`deploy.replicas` in docker-compose.yml). Jede Replica benötigt ~200 MB RAM. Redis `maxclients` muss ≥ `VAKT_WORKER_CONCURRENCY` × `WORKER_REPLICAS` sein. Asynq stellt sicher, dass Tasks exakt einmal ausgeführt werden. |

**Beispiel:**

```env
VAKT_API_PORT=8080
APP_VERSION=1.0.0
VAKT_MODULES_ENABLED=vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,vakthr
AUTO_MIGRATE=false
VAKT_FRONTEND_URL=https://vakt.meine-firma.de
VAKT_UPLOAD_DIR=/app/data/uploads
```

> **`VAKT_DEMO=true` nur für Test-/Demo-Umgebungen:** Aktiviert Beispieldaten und
> einen öffentlich erreichbaren `/api/v1/demo/start`-Endpoint — **niemals** in
> Produktion mit echten Compliance-Daten. Nach einer Demo-Installation für den
> produktiven Einsatz: `VAKT_DEMO=false` setzen und ephemerale Demo-Orgs
> (Slug-Muster `demo-*`) löschen.

---

## SMTP (Benachrichtigungen und Vakt Aware)

SMTP wird für Benachrichtigungs-E-Mails, Phishing-Simulations-Kampagnen (Vakt Aware) und Policy-Akzeptanz-Links (Vakt Comply) benötigt.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_SMTP_HOST` | — | `localhost` | Hostname des SMTP-Servers. |
| `VAKT_SMTP_PORT` | — | `1025` | SMTP-Port. `1025` für Mailpit (Entwicklung), `587` für STARTTLS, `465` für SSL/TLS. |
| `VAKT_SMTP_USER` | — | — | SMTP-Benutzername. Erforderlich für Port 587/465. |
| `VAKT_SMTP_PASS` | — | — | SMTP-Passwort. Erforderlich für Port 587/465. |
| `VAKT_SMTP_FROM` | — | `vaktaware@vakt.local` | Absenderadresse für alle E-Mails. Muss eine gültige Adresse sein, die der SMTP-Server akzeptiert. |

**Beispiel Entwicklung (Mailpit):**

```env
VAKT_SMTP_HOST=localhost
VAKT_SMTP_PORT=1025
VAKT_SMTP_FROM=vakt@beispiel.de
```

**Beispiel Produktion:**

```env
VAKT_SMTP_HOST=smtp.mein-anbieter.de
VAKT_SMTP_PORT=587
VAKT_SMTP_USER=vakt@meine-firma.de
VAKT_SMTP_PASS=sicheres-passwort
VAKT_SMTP_FROM=vakt@meine-firma.de
```

---

## KI-Berater (Standard — lokal via Ollama)

Vakt enthält einen integrierten KI-Compliance-Berater, der lokal via Ollama läuft — **standardmäßig aktiv**. Das Default-Modell `qwen2.5:7b` (~4.5 GB, Apache 2.0, CPU-tauglich) wird beim ersten `docker compose up` automatisch geladen — kein GPU, kein Cloud-API-Key, kein manueller Schritt nötig. Auf VMs mit < 8 GB RAM `qwen2.5:3b` verwenden. Deaktivieren: `VAKT_AI_PROVIDER=disabled` in `.env` setzen.

Cloud-Alternative: **Mistral AI** (EU-Server, DSGVO-freundlich via AVV) — schneller, aber Daten verlassen die Instanz.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_AI_PROVIDER` | — | `ollama` | KI-Provider. `ollama` = lokales Ollama (Standard). `openai` = andere OpenAI-kompatible Endpunkte (z.B. Mistral). `disabled` schaltet den Berater ab. |
| `VAKT_AI_BASE_URL` | — | `http://ollama:11434/v1` | API-Basisendpunkt des Providers. |
| `VAKT_AI_API_KEY` | — | — | API-Key des Providers. Für lokale Provider (Ollama, LM Studio) leer lassen. |
| `VAKT_AI_MODEL` | — | `qwen2.5:7b` | Modellname (Default; auf VMs mit < 8 GB RAM `qwen2.5:3b`). |
| `VAKT_AI_REPORT_TIMEOUT` | — | `120` | HTTP-Timeout für KI-Report-Generierung in Sekunden. Bei langsamen lokalen Modellen erhöhen. |
| `VAKT_AI_RATE_LIMIT_RPM` | — | `30` | Max. KI-Anfragen pro Minute und Organisation. |
| `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG` | — | `0` | Tägliches Token-Budget pro Organisation. `0` = unbegrenzt. |
| `VAKT_AI_CACHE_TTL_SECONDS` | — | `3600` | Cache-Dauer identischer KI-Antworten (Key = `sha256(Modell+Prompt)`). `0` = Cache aus. |
| `VAKT_AI_COST_PER_MTOKEN_IN_MICRO_EUR` | — | `0` | Kosten pro 1 Mio. Input-Tokens in Mikro-EUR (Kosten-Tracking bei Cloud-Providern). Lokales Ollama = `0`. |
| `VAKT_AI_COST_PER_MTOKEN_OUT_MICRO_EUR` | — | `0` | Kosten pro 1 Mio. Output-Tokens in Mikro-EUR. Lokales Ollama = `0`. |
| `VAKT_AI_FAIL_OPEN_ON_OUTAGE` | — | `false` | Wenn `true`, lassen die KI-Rate-Limit-/Quota-Checks bei Redis-/Postgres-Ausfall „fail open" (KI bleibt erreichbar statt zu blocken). Audit-relevante Abwägung — Default sicher (`false`). |

**Standard (Ollama, lokal):**

```env
VAKT_AI_PROVIDER=ollama
VAKT_AI_BASE_URL=http://ollama:11434/v1
VAKT_AI_MODEL=qwen2.5:7b
```

Das Modell wird beim ersten Start automatisch von `ollama-init` geladen — kein manueller `ollama pull` nötig.

**Beispiel Mistral AI (EU-Server, DSGVO-freundlich):**

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

**KI deaktivieren:**

```env
VAKT_AI_PROVIDER=disabled
```

---

## Update-Benachrichtigungen (optional)

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_UPDATE_CHECK` | — | `false` | Aktiviert den täglichen Check auf neue Vakt-Versionen via GitHub Releases API. Zeigt Admins und Eigentümern ein Banner in der UI wenn eine neue Version verfügbar ist. Es werden keine Daten gesendet — nur ein lesender GET-Request an die öffentliche GitHub-API, ohne Instanz-ID oder Telemetrie. |

**Env-Var (Boot-Default):**

```env
VAKT_UPDATE_CHECK=true
```

**UI-Toggle (Admin-only, überschreibt die Env-Var, persistiert in Redis):**

Admins können die Update-Prüfung auch zur Laufzeit unter **Einstellungen → Updates** ein- und ausschalten, ohne die Instanz neu zu starten. Der Redis-Wert hat Vorrang vor der Env-Var; wird er gelöscht, gilt wieder der Env-Var-Default.

---

## Observability (optional)

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_METRICS_DISABLED` | — | `false` | Wenn `true`, wird der Prometheus-`/metrics`-Endpunkt deaktiviert. |
| `VAKT_PPROF_ENABLED` | — | `false` | Wenn `true`, wird ein Go-pprof-Server auf `127.0.0.1:6060` gestartet (nur für Diagnose, nie im Internet exponieren). Siehe [pprof-Anleitung](../operations/runbook.md#pprof). |
| `VAKT_AUDIT_RETENTION_YEARS` | — | `6` | Aufbewahrung der `audit_log`-Jahres-Partitionen. Ein monatlicher Worker-Job legt kommende Partitionen an und droppt Partitionen, die älter als dieser Wert sind. `0` = nichts droppen (Vorab-Anlage läuft weiter). |
| `VAKT_LOG_LEVEL` | — | `info` | Globales Log-Level (`trace`, `debug`, `info`, `warn`, `error`). Ungültige Werte fallen auf `info` zurück. |
| `VAKT_SLO_UPTIME` | — | `99.9` | Ziel-Verfügbarkeit (%) für die Fehlerbudget-Berechnung. |
| `VAKT_SLO_P99_LATENCY_MS` | — | `500` | Ziel-p99-Latenz (ms) für die Fehlerbudget-Berechnung. |

---

## Vakt Scan (optional)

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_SCAN_ALLOW_PRIVATE` | — | `false` | Wenn `true`, darf Vakt Scan auch interne IP-Adressen (RFC-1918, Loopback, Link-Local) als Scan-Ziele akzeptieren. **Standardmäßig blockiert (SSRF-Schutz).** Nur in vollständig isolierten internen Netzwerken setzen, in denen Scanner-Zugriff auf interne Hosts erwünscht ist. |
| `VAKT_SCAN_CONCURRENCY` | — | `2` | Max. gleichzeitig laufende Scanner-Subprozesse (Trivy/Nuclei/Syft) pro Worker. Jeder Subprozess puffert seine komplette Ausgabe im Speicher — der Wert ist auf das Worker-Memory-Limit (768m) abgestimmt. Auf größeren Hosts zusammen mit dem Memory-Limit erhöhen. |

---

## Benutzerverwaltung & Rollen

Vakt unterscheidet zwischen der kostenlosen **Community Edition** mit vier festen Rollen und der **Pro**-Edition mit granularen Modul-Berechtigungen.

### Community-Rollen

| Rolle | Rechte |
|-------|--------|
| **Admin** | Vollzugriff — Benutzer verwalten, Module konfigurieren |
| **Analyst** | Lesen + Schreiben in allen Modulen |
| **Viewer** | Nur lesen — alle Module |
| **Auditor** | Nur lesen + Audit-Bericht exportieren |

### Pro: Granulare Modul-Berechtigungen

Mit einer Pro-Lizenz können Benutzerrechte zusätzlich auf einzelne Module eingeschränkt werden. Jeder Benutzer erhält pro Modul (Vakt Scan, Vakt Comply, Vakt Vault, Vakt Aware, Vakt Privacy) eine separate `can_read`- und `can_write`-Berechtigung.

**Verwaltung:** Einstellungen → Benutzerverwaltung → Shield-Icon neben dem jeweiligen Benutzer.

---

## OIDC / SAML Single Sign-On (optional)

SSO wird über [Casdoor](https://casdoor.org) als OIDC/SAML-Proxy unterstützt. Damit können Azure AD, Okta, Keycloak und Google Workspace eingebunden werden.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `CASDOOR_URL` | — | — | URL des Casdoor-Servers. |
| `CASDOOR_CLIENT_ID` | — | — | OAuth2 / OIDC Client-ID der Vakt-Anwendung in Casdoor. |
| `CASDOOR_CLIENT_SECRET` | — | — | OAuth2 / OIDC Client-Secret. Nicht in Git committen. |

**Beispiel:**

```env
CASDOOR_URL=https://auth.meine-firma.de
CASDOOR_CLIENT_ID=vakt-app
CASDOOR_CLIENT_SECRET=<dein-casdoor-client-secret>
```

---

## LDAP / Active Directory (optional)

Vakt kann Benutzerkonten aus einem LDAP/AD synchronisieren.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_LDAP_URL` | — | — | LDAP-Server-URL. Format: `ldap://host:389` oder `ldaps://host:636`. |
| `VAKT_LDAP_BIND_DN` | — | — | Distinguished Name des Service-Accounts für die Verbindung. |
| `VAKT_LDAP_BIND_PASS` | — | — | Passwort des Service-Accounts. |
| `VAKT_LDAP_BASE_DN` | — | — | Basis-DN für die Benutzersuche. |
| `VAKT_LDAP_USER_FILTER` | — | `(objectClass=person)` | LDAP-Filter für Benutzer. |
| `VAKT_LDAP_GROUP_FILTER` | — | `(objectClass=group)` | LDAP-Filter für Gruppen. |
| `VAKT_LDAP_TLS` | — | `false` | TLS für LDAP-Verbindung aktivieren (`true`/`false`). |

**Beispiel:**

```env
VAKT_LDAP_URL=ldap://dc.meine-firma.local:389
VAKT_LDAP_BIND_DN=CN=vakt-service,OU=ServiceAccounts,DC=meine-firma,DC=local
VAKT_LDAP_BIND_PASS=geheimes-passwort
VAKT_LDAP_BASE_DN=OU=Users,DC=meine-firma,DC=local
VAKT_LDAP_USER_FILTER=(objectClass=person)
VAKT_LDAP_GROUP_FILTER=(objectClass=group)
VAKT_LDAP_TLS=false
```

---

## Lizenz (Vakt Pro)

Self-Hosted-Instanzen laufen standardmäßig als **Community Edition** (kostenlos, unbegrenzt). Für Pro-Features wird ein License Key benötigt, der nach dem Kauf automatisch per E-Mail zugestellt wird.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_LICENSE_KEY` | — | — | Pro License Key. Nach dem Kauf per E-Mail erhalten. Kann auch direkt unter **Einstellungen → Lizenz** eingetragen werden — dann ist kein Neustart nötig. |
| `VAKT_LICENSE_TOKEN` | — | — | Renewal-Token für automatische Key-Erneuerung (ebenfalls in der Kauf-E-Mail). Wenn gesetzt, holt die Instanz täglich den aktuellen Key von `api.norvikops.de` — kein manueller Eingriff bei Verlängerungen nötig. Opt-in, siehe unten. |
| `VAKT_LICENSE_REFRESH_URL` | — | `https://api.norvikops.de` | Überschreibt den Renewal-Endpunkt (nur für Air-Gap-/Eigenbetrieb des Lizenzservers nötig — normalerweise leer lassen). |

**Beispiel:**

```env
VAKT_LICENSE_KEY=eyJ0aWVyIjoicHJvIn0.signatur...
VAKT_LICENSE_TOKEN=550e8400-e29b-41d4-a716-446655440000
```

**Manuelle Aktivierung:** Key unter **Einstellungen → Lizenz → License Key eingeben → Aktivieren** eintragen. Nur Admin-Benutzer haben Zugriff.

**Testphase:** Der Kauf über Polar startet mit einer 30-tägigen kostenlosen Testphase. Während des Trials ist der Key ~45 Tage gültig (30 Tage Trial + Puffer für die manuelle Aktivierung). Wandelt sich die Testphase in ein bezahltes Abo um, wird automatisch ein Key mit voller Laufzeit ausgestellt und per Mail zugeschickt; kündigst du vor Trial-Ende, läuft der Key einfach aus.

**Laufzeit & Auto-Renewal:** Pro-Keys haben ein Ablaufdatum (35 Tage bei Monatsabo, 395 Tage bei Jahresabo). **Primär (offline):** Nach jedem Kauf/jeder Verlängerung kommt der Key per Mail — unter **Einstellungen → Lizenz** eintragen, keine ausgehende Verbindung nötig. **Optional (Opt-in):** Mit `VAKT_LICENSE_TOKEN` erneuert sich der Key täglich automatisch von `api.norvikops.de` — bequem, aber eine ausgehende Verbindung (siehe Datenschutz-Hinweis unten). Ohne Token erscheint ab 30 Tage vor Ablauf ein Banner mit Hinweis auf die Kauf-E-Mail.

**Datenschutz-Hinweis zu `VAKT_LICENSE_TOKEN`:** Die Instanz stellt einmal täglich eine ausgehende HTTPS-Verbindung zu `api.norvikops.de` her. Dabei wird ausschließlich der Token übertragen — keine Geschäftsdaten, keine Nutzungsdaten. Wer ausgehende Verbindungen in der Firewall kontrolliert (typisch für ISO 27001 / NIS2 Umgebungen), muss `api.norvikops.de:443` in der Egress-Whitelist eintragen oder `VAKT_LICENSE_TOKEN` weglassen und die manuelle Aktivierung nutzen.

**Datenschutz-Hinweis zu `VAKT_BSI_FEED_ENABLED`:** Wenn aktiviert (Standard), ruft Vakt einmal täglich den BSI CERT-Bund RSS-Feed (`www.bsi.bund.de`) ab, um aktuelle Sicherheitswarnungen in das Dashboard einzuspeisen. Dabei werden **keine Inhaltsdaten übertragen** — es ist ein reiner HTTP GET-Abruf des öffentlichen Feeds. Wer diese ausgehende Verbindung unterbinden möchte (Air-Gap-Umgebungen, strikte Egress-Policies), setzt `VAKT_BSI_FEED_ENABLED=false`. Das Sicherheitshinweis-Widget im Dashboard bleibt dann leer.

**Datenschutz-Hinweis zu `VAKT_EPSS_ENABLED`:** **Standardmäßig aus.** Wenn auf `true` gesetzt, reichert Vakt Findings mit EPSS-Scores (Exploit Prediction Scoring System) aus einer externen API an — eine ausgehende Verbindung. Bewusst opt-in, um das No-Phone-Home-Versprechen nicht zu unterlaufen. In Air-Gap-/strikten Egress-Umgebungen auf `false` (Default) belassen.

> **Hinweis:** Die Variablen `VAKT_LICENSE_PRIVATE_KEY`, `VAKT_LEXWARE_API_KEY`, `VAKT_BILLING_BASE_URL` und `VAKT_BILLING_NOTIFY_EMAIL` sind ausschließlich für den Norvik-eigenen Billing-Server — sie gehören **nicht** in die Kunden-Konfiguration.

---

## Vollständige .env-Vorlage

```env
# ── Pflichtfelder ──────────────────────────────────────────────────────────────
VAKT_DB_URL=postgres://vakt:passwort@postgres:5432/vakt?sslmode=disable
VAKT_REDIS_URL=redis://redis:6379
VAKT_SECRET_KEY=<openssl rand -hex 32>

# ── Datenbank-Container ────────────────────────────────────────────────────────
POSTGRES_PASSWORD=<dein-db-passwort>

# ── Anwendung ──────────────────────────────────────────────────────────────────
VAKT_API_PORT=8080
APP_VERSION=1.0.0
VAKT_MODULES_ENABLED=vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,vakthr
AUTO_MIGRATE=false
VAKT_FRONTEND_URL=https://vakt.meine-firma.de
VAKT_UPLOAD_DIR=/app/data/uploads
# VAKT_UPDATE_CHECK=false   # opt-in: täglicher Check auf neue Versionen via GitHub API

# ── SMTP ────────────────────────────────────────────────────────────────────────
VAKT_SMTP_HOST=smtp.meine-firma.de
VAKT_SMTP_PORT=587
VAKT_SMTP_USER=vakt@meine-firma.de
VAKT_SMTP_PASS=
VAKT_SMTP_FROM=vakt@meine-firma.de

# ── KI-Berater (Ollama lokal, kein GPU nötig) ──────────────────────────────────
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=http://ollama:11434/v1
VAKT_AI_API_KEY=
VAKT_AI_MODEL=qwen2.5:7b

# ── OIDC / SSO (optional) ──────────────────────────────────────────────────────
CASDOOR_URL=
CASDOOR_CLIENT_ID=
CASDOOR_CLIENT_SECRET=

# ── LDAP / Active Directory (optional) ────────────────────────────────────────
VAKT_LDAP_URL=
VAKT_LDAP_BIND_DN=
VAKT_LDAP_BIND_PASS=
VAKT_LDAP_BASE_DN=
VAKT_LDAP_USER_FILTER=(objectClass=person)
VAKT_LDAP_GROUP_FILTER=(objectClass=group)
VAKT_LDAP_TLS=false

# ── Lizenz (Vakt Pro, optional) ───────────────────────────────────────────────
# VAKT_LICENSE_KEY=      # Pro License Key — nach Kauf per E-Mail erhalten
# VAKT_LICENSE_TOKEN=    # Renewal-Token — aktiviert Auto-Renewal (ebenfalls in der Kauf-E-Mail)
```

---

## Pro-Integrationen (keine Env-Vars)

Die folgenden Integrationen werden **pro Organisation** in der Vakt-Oberfläche konfiguriert (Admin → Einstellungen) und benötigen keine eigenen Umgebungsvariablen:

| Integration | Edition | Setup |
|-------------|---------|-------|
| **SAML 2.0 Direct SP** | Pro | Admin → SSO → SAML → IdP-Metadaten hochladen |
| **OIDC via Casdoor** | Pro | `CASDOOR_*`-Vars (siehe oben) |
| **SCIM 2.0 Provisioning** | Pro | Admin → SSO → SCIM → Token generieren |
| **SIEM-Forwarder** (Splunk, Elastic, Webhook) | Pro | Admin → Integrationen → SIEM → Adapter + Endpoint konfigurieren |
| **IP-Allowlist für Admin-Endpunkte** | Pro | Admin → Sicherheit → IP-Allowlist → CIDR-Einträge |
| **MFA für sensitive API-Calls** | Pro | Admin → Sicherheit → MFA-Enforcement |

Ausführliche Setup-Anleitungen: `docs/wiki/enterprise-sso.md`

---

## Automatische Backups (S89-4)

Vakt liefert signierte, verschlüsselte Backups (`scripts/backup.sh`) plus einen
Automations-Wrapper (`scripts/backup-cron.sh`): **erstellen → verifizieren →
optional off-site pushen → nach Retention rotieren**, mit Benachrichtigung bei
Fehlschlag.

| Variable | Standard | Beschreibung |
|----------|----------|--------------|
| `VAKT_BACKUP_PASSPHRASE` | — | Passphrase, die den Master-Key im Archiv umschließt. **Pflicht für automatische (nicht-interaktive) Backups.** Alternativ `VAKT_BACKUP_PASSPHRASE_FILE`. |
| `VAKT_BACKUP_DIR` | `./data/backups` | Host-Verzeichnis für die Archive. |
| `VAKT_BACKUP_SCHEDULE` | `0 2 * * *` | Cron-Ausdruck des Scheduler-Service (täglich 02:00). |
| `VAKT_BACKUP_RETENTION_DAYS` | `30` | Archive älter als N Tage werden rotiert (gelöscht). |
| `VAKT_BACKUP_OFFSITE_CMD` | — | **Opt-in** Off-Site-Push. Läuft mit `$ARCHIVE` + `$SIG`. **Kunden-konfiguriertes Ziel — niemals ein Norvik-Endpoint** (Datenschutz-Grundsatz). Beispiel: `aws s3 cp "$ARCHIVE" s3://my-bucket/ && aws s3 cp "$SIG" s3://my-bucket/`. |
| `VAKT_BACKUP_NOTIFY_WEBHOOK` | — | Eigener Webhook, der bei Fehlschlag `{"text":…}` per POST erhält. |
| `VAKT_BACKUP_NOTIFY_CMD` | — | Generischer Fehler-Hook; läuft mit `$MESSAGE`. Beispiel: `logger -t vakt-backup "$MESSAGE"`. |

**Variante A — Compose-Service (empfohlen):**

```bash
docker compose -f docker-compose.yml -f docker-compose.backup.yml --profile backup up -d
```

Der opt-in `backup`-Service läuft `backup-cron.sh` auf `VAKT_BACKUP_SCHEDULE`.

**Variante B — Host-Cron:**

```cron
# /etc/cron.d/vakt-backup  (täglich 02:00)
0 2 * * * deploy cd /opt/vakt && VAKT_BACKUP_DIR=/backups/vakt bash scripts/backup-cron.sh run >> /var/log/vakt-backup.log 2>&1
```

> **Off-Site & Datenschutz:** Der Off-Site-Push ist optional und zielt immer auf
> ein **vom Kunden konfiguriertes** Ziel (S3-kompatibel, rsync, SFTP …). Vakt
> sendet niemals Backups an Norvik. Off-Site-Ziel wie das Backup selbst behandeln
> (verschlüsselt, Zugriff begrenzt).

> **Restore testen:** Ein automatisiertes Backup ist nur so viel wert wie sein
> getesteter Restore — siehe [Disaster-Recovery-Runbook](../runbooks/disaster-recovery.md).

## Hinweise

### AUTO_MIGRATE nur kontrolliert einsetzen

`AUTO_MIGRATE=true` ist praktisch für einfache Setups. In Produktionsumgebungen mit kritischen Daten empfiehlt sich:

1. Backup anlegen
2. Migration manuell prüfen und ausführen
3. Ergebnis verifizieren
4. Erst dann neuen Anwendungscode starten

### Module einzeln deaktivieren

Jedes Modul kann unabhängig deaktiviert werden. Die Modul-Namen sind case-insensitiv:

```env
# Nur Vakt Comply und Vakt Vault aktiv
VAKT_MODULES_ENABLED=vaktcomply,vaktvault
```

---

## Verschlüsselung im Betrieb (Encryption at Rest)

Diese Übersicht beantwortet die Frage, die ISO-27001-Auditoren der eigenen Kunden stellen:
„Was ist in der Datenbank verschlüsselt, was nicht — und was muss der Betreiber selbst sichern?"

### Was AES-256-GCM-verschlüsselt ist

Vakt verschlüsselt sensible Zugangsdaten auf Anwendungsebene mit dem `VAKT_SECRET_KEY`:

| Datenkategorie | Speicherort | Verschlüsselung |
|---|---|---|
| Vakt-Vault-Einträge (Secrets, API-Keys, Passwörter) | `so_vault_entries.encrypted_value` | AES-256-GCM |
| SMTP-Passwörter und Integration-Credentials | `integration_credentials.*` | AES-256-GCM |
| Webhook-Signing-Secrets | `webhooks.secret` | AES-256-GCM |
| OIDC-/SAML-Client-Secrets | `sso_configs.*` | AES-256-GCM |

### Was bewusst im Klartext gespeichert ist

ISMS-Inhaltsdaten liegen unverschlüsselt in der Datenbank:

| Datenkategorie | Begründung |
|---|---|
| Controls, Risiken, Incidents, Maßnahmen (CAPAs) | Kein Zugangsdaten-Charakter; Klartext ermöglicht Volltextsuche und Reporting |
| Policies und Evidence-Metadaten | ISO-Auditoren müssen Inhalte einsehen; kein Vorteil durch Feldverschlüsselung |
| Personenbezogene Daten (DSGVO-Modul: VVT, DSRs) | Werden durch organisatorische Maßnahmen + Zugriffsschutz geschützt (s.u.) |

**Vertretbare Entscheidung:** Vakt ist ein Self-Hosted-Produkt. Die Verantwortung für
Disk-Encryption liegt beim Betreiber — nicht bei der Anwendung. ISO 27001 A.8.24 fordert
kryptographische Maßnahmen nur dort, wo das Risiko es rechtfertigt.

### Was der Betreiber sicherstellen muss

Um den ISMS-Daten dasselbe Schutzniveau wie den verschlüsselten Secrets zu geben,
muss der Betreiber auf Infrastrukturebene absichern:

| Maßnahme | Umsetzung |
|---|---|
| **Disk-Encryption** für den PostgreSQL-Volume-Pfad | LUKS (Linux), BitLocker (Windows), verschlüsselte EBS-Volumes (AWS), Persistent-Disk-Encryption (GCP/Azure) |
| **Backup-Verschlüsselung** | `backup.sh` erzeugt SQL-Dump — GPG-Verschlüsselung vor Ablage in externem Speicher empfohlen |
| **Netzwerk-Isolation** | PostgreSQL-Port nur im internen Docker-Netzwerk; nie nach außen exponieren |
| **Zugriffsprotokoll PostgreSQL** | `log_connections = on`, `log_disconnections = on` in PostgreSQL-Config |

### Opt-in: Lizenzerneuerung

Einzige ausgehende Verbindung von Vakt: wenn `VAKT_LICENSE_TOKEN` gesetzt ist, kontaktiert
die Instanz täglich `api.norvikops.de` zur Lizenzerneuerung — es werden ausschließlich der
Lizenz-Token übertragen, keine Geschäfts- oder Personendaten.

