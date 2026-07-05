# Upgrade-Prozedur

Vakt verwendet semantische Versionierung (SemVer). Datenbankmigrationen laufen automatisch beim Start des `migrate`-Containers.

Vollständige versionsspezifische Hinweise: [`docs/UPGRADE.md`](../UPGRADE.md)

---

## Standard-Upgrade (Docker Compose)

```bash
# 1. Backup erstellen (zwingend vor jedem Upgrade)
cd /opt/vakt
./scripts/backup.sh /backups/vakt

# 2. Neue Images ziehen
docker compose pull

# 3. Migrationen ausführen (separater Schritt — Pflicht für Prod)
docker compose run --rm migrate up
docker compose up -d

# 4. Verify
docker compose ps
curl http://localhost/health | jq '.status, .version'
```

Der `migrate`-Service führt alle ausstehenden Migrationen aus, bevor API und Worker starten.
Das Entkoppeln von Migration und Container-Start ist bewusst: So bleibt Rollback sauber
(kein Image-Swap der gleichzeitig eine Migration auslöst) und ein fehlgeschlagenes `migrate up`
bricht den Deploy sofort ab statt einer API-Restart-Loop — genau das Muster das die historischen
Demo-Outages (Migration 128/129) verursacht hat. CI-Lint blockiert beide SQL-Fehlerklassen
(NOW()-in-Index-Predicate, CONCURRENTLY ohne no-transaction-Direktive).

> **`AUTO_MIGRATE=true` — nur für Dev und lokalen Quickstart.**  
> Diese Einstellung startet die Migration automatisch im API-Container-Start und ist in
> `docker-compose.dev.yml` aktiv. Für Produktion nicht verwenden: ein fehlgeschlagenes
> `migrate up` führt zu einer API-Restart-Loop statt zu einem sauberen Deploy-Fehler.
> Im `docker-compose.yml` (Prod-Stack) ist `AUTO_MIGRATE` **nicht gesetzt**.

---

## Kubernetes / Helm-Upgrade

```bash
helm repo update
helm upgrade vakt vakt/vakt \
  --namespace vakt \
  --values values.yaml \
  --wait
```

Helm führt den `migrate`-Job als init-Container vor dem API-Rollout aus.

---

## Rollback

Falls ein Upgrade schief geht:

### Option A — Migration rückgängig machen

```bash
# Eine Migration zurück
docker compose run --rm migrate down 1

# Dann alten Image-Tag starten
docker compose up -d
```

### Option B — Vollständiger Backup-Restore

```bash
# Stack stoppen
docker compose down

# Backup wiederherstellen (siehe docs/operations/backup-restore.md)
./scripts/restore.sh /backups/vakt/<datei.tar.gz>

# Alten Image-Tag in docker-compose.yml oder .env eintragen
# Stack neu starten
docker compose up -d
```

---

## Breaking-Changes-Checkliste für Major-Upgrades

Bei Upgrades über eine Major-Version hinweg (z.B. v0.x → v1.0) vor dem Upgrade prüfen:

- [ ] **CHANGELOG.md lesen** — alle Versionen zwischen aktuellem Stand und Zielversion durchlesen
- [ ] **UPGRADE.md lesen** — versionsspezifische Hinweise und manuelle Schritte
- [ ] **ENV-Variablen prüfen** — neue Pflicht-Variablen? Umbenannte Variablen? (z.B. `AUTO_MIGRATE` war in v0.6.0 neu)
- [ ] **Nginx-Konfiguration prüfen** — neue Pfade erfordern u.U. neue `location`-Blöcke (z.B. SSE-Streams seit v0.10.0: `proxy_buffering off`)
- [ ] **Helm-Values prüfen** — geänderte Default-Werte (z.B. `redis.auth.enabled` auf `true` seit v0.7.0)
- [ ] **Prometheus-Metriken** — bei Prefix-Änderungen Grafana-Dashboards anpassen (v0.6.0: `sechealth_` → `vakt_`)
- [ ] **Backup erstellt** — immer, auch wenn keine Breaking Changes dokumentiert sind

---

## Migrationsicherheit

- Migrationen sind vorwärtskompatibel: v0.N → v0.N+1 sicher
- Rückwärts-Migrationen (`.down.sql`) existieren für Notfälle — `migrate down 1`
- Breaking Changes werden in Major-Versionen (v1.0, v2.0) angekündigt
- Jede Migration hat `up` und `down` — bei Fehler: `migrate down 1` ausführen

### Bekannte SQL-Fehlerquellen in Migrationen

Zwei Klassen von SQL-Fehlern haben in der Vergangenheit Outages verursacht und sind durch CI-Lint geblockt:

1. **`NOW()` in Partial-Index WHERE** — SQLSTATE 42P17 (volatile function nicht IMMUTABLE)
2. **`CREATE INDEX CONCURRENTLY` ohne `-- migrate: no transaction`** — SQLSTATE 25001 (CONCURRENTLY in Transaktion verboten)

Falls eine Migration fehlschlägt: `docker compose logs migrate` lesen, SQLSTATE identifizieren, dann `.down.sql` ausführen.

---

## Support und weiterführende Ressourcen

- Versionsspezifische Upgrade-Hinweise: [`docs/UPGRADE.md`](../UPGRADE.md)
- Backup vor dem Upgrade: [`docs/operations/backup-restore.md`](backup-restore.md)
- Runbook für Produktionsprobleme: [`docs/operations/runbook.md`](runbook.md)
- GitHub Issues: https://github.com/norvik-ops/vakt/issues
