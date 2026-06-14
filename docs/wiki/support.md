# Support & Diagnose-Daten einsammeln

Wenn etwas in deiner Vakt-Instanz nicht funktioniert, helfen uns ein paar
Standard-Artefakte, das Problem schnell zu finden. Diese Seite beschreibt, **was
du einsammelst und wie du es uns zuschickst**.

> **Datenschutz vorab:** Vakt ist self-hosted und hat **kein Phone-home**. Es
> werden niemals automatisch Daten an uns übertragen — du entscheidest, was du
> uns schickst. Die Anwendungs-Logs sind PII-redigiert (E-Mail-Adressen
> erscheinen als `***@firma.de`), enthalten aber Domains, IP-Adressen und
> aufgerufene URLs. Sieh die Dateien vor dem Versand kurz durch.

---

## 1. Schnell-Bundle (deckt 90 % der Fälle ab)

Im Verzeichnis mit deiner `docker-compose.yml` ausführen:

```bash
make support-bundle
```

Das erzeugt ein `vakt-support-<datum>.tar.gz` mit Version/Host-Infos,
Container-Status, Health-Ausgabe und den Logs **aller** Services. Optionen:

```bash
make support-bundle TAIL=5000        # mehr Logzeilen pro Service (Default 2000)
make support-bundle SINCE=30m        # nur die letzten 30 Minuten
```

Ohne `make` geht es auch direkt: `bash scripts/support-bundle.sh`.

Das Archiv kannst du uns schicken (siehe
[Abschnitt 5](#5-an-den-support-schicken)) — **vorher kurz reinschauen**:

```bash
tar -tzf vakt-support-*.tar.gz   # Dateiliste
tar -xzf vakt-support-*.tar.gz   # entpacken und durchsehen
```

Wenn du das Bundle lieber von Hand zusammenstellst, reicht für die meisten
Fälle:

```bash
docker compose logs --tail=2000 api worker > vakt-logs.txt 2>&1
```

---

## 2. Einzelne Service-Logs

Vakt schreibt **strukturierte JSON-Logs nach stdout** (zerolog). In der
Compose-Installation greifst du sie pro Service ab:

```bash
docker compose logs api        # HTTP-Server (Login, Requests, Auth, Rate-Limits)
docker compose logs worker     # Hintergrund-Jobs (Scans, Reports, Scheduler)
docker compose logs nginx      # Reverse Proxy (HTTP-Status, TLS)
docker compose logs postgres   # Datenbank
docker compose logs redis      # Cache / Queue
docker compose logs ollama     # Lokale KI (nur falls KI-Features aktiv)
```

Nützliche Flags:

```bash
docker compose logs -f api                 # live mitlaufen lassen
docker compose logs --tail=500 api         # nur die letzten 500 Zeilen
docker compose logs --since=30m api        # nur die letzten 30 Minuten
docker compose logs --since=2026-06-14T08:00:00 api   # ab Zeitpunkt
```

Jede API-Logzeile enthält `method`, `uri`, `status`, `latency` sowie eine
`X-Trace-ID` zur Korrelation. Wenn du uns einen konkreten Fehlerzeitpunkt
nennen kannst, finden wir die zugehörige Zeile darüber sehr schnell.

### Mehr Details: Log-Level hochdrehen

Standardmäßig wird auf `info` geloggt. Für eine schwer reproduzierbare Sache
kannst du temporär ausführlicher loggen — `VAKT_LOG_LEVEL=debug` in der `.env`
setzen und `api`/`worker` neu starten:

```bash
echo "VAKT_LOG_LEVEL=debug" >> .env
docker compose up -d api worker
# ... Problem reproduzieren, Bundle ziehen ...
# danach wieder zurücksetzen (Zeile entfernen oder auf info), erneut up -d
```

Erlaubte Werte: `trace`, `debug`, `info`, `warn`, `error` (ungültige Werte
fallen auf `info` zurück).

---

## 3. Version & Health

Bitte immer mitschicken — viele Probleme sind versions- oder
abhängigkeitsspezifisch:

```bash
curl -s http://localhost/health          | jq .   # Version, Demo-Flag, SSO
curl -s http://localhost/health/ready    | jq .   # DB + Redis erreichbar?
docker compose ps                                 # welche Container laufen / restarten?
```

Ein Container im Status `Restarting` oder `Exited` ist fast immer die Ursache —
dann sind seine Logs (`docker compose logs <service>`) der erste Anlaufpunkt.

---

## 4. Spezialfälle

### Frontend-/Browser-Fehler
JavaScript-Fehler aus dem Browser werden serverseitig in der Tabelle
`client_errors` gesammelt und sind als Admin über das Admin-Interface
einsehbar. Zusätzlich hilft uns die Browser-Konsole (F12 → Console/Network) als
Screenshot.

### Login funktioniert nicht / „IP gesperrt"
Nach 10 fehlgeschlagenen Logins wird die IP 15 Minuten gesperrt (Anti-Abuse,
kein Bug). Prüfen:

```bash
docker compose exec redis redis-cli KEYS 'login_fail_ip:*'
```

Mehr dazu und weitere häufige Probleme in
[Betrieb & Wartung](../operations.md#6-häufige-probleme).

### Audit-Evidenz (manipulationssicher)
Das **Audit-Log** ist getrennt von den Diagnose-Logs und liegt
hash-verkettet in der Datenbank. Es ist über das Audit-/Auditor-Portal als
Export (CSV/ZIP) abrufbar und dient als Compliance-*Evidenz*, nicht zur
Fehlersuche. Für Support-Fälle ist es normalerweise **nicht** nötig.

---

## 5. An den Support schicken

1. Bundle aus [Abschnitt 1](#1-schnell-bundle-deckt-90--der-fälle-ab) erstellen.
2. Dateien kurz durchsehen (siehe Datenschutz-Hinweis oben).
3. Per E-Mail an deinen Norvik-Ops-Ansprechpartner, mit:
   - **Was** ist passiert (erwartetes vs. tatsächliches Verhalten)?
   - **Wann** ungefähr (Uhrzeit + Zeitzone — hilft beim Log-Abgleich)?
   - **Reproduzierbar?** Welche Schritte führen dorthin?

---

## 6. Log-Aufbewahrung

Die mitgelieferte `docker-compose.yml` aktiviert für alle langlebigen Services
bereits eine **Log-Rotation** (`json-file`, `max-size: 10m`, `max-file: 5` →
max. ~50 MB pro Service). Das verhindert volllaufende Disks und stellt sicher,
dass aktuelle Logs für ein Support-Bundle vorhanden sind. Du musst dafür nichts
konfigurieren.

Wichtig: Beim Entfernen/Neuaufbau eines Containers (`docker compose down`,
Image-Update) werden dessen Logs verworfen. Wer Logs **dauerhaft** über
Container-Neustarts hinweg aufbewahren will, leitet sie an einen Aggregator
weiter (Loki, Datadog, Elastic, CloudWatch) — siehe
[Log-Forwarding in Betrieb & Wartung](../operations.md#log-forwarding).
