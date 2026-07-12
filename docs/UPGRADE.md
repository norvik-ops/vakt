# Vakt — Upgrade-Anleitung

## Upgrade-Strategie

Vakt verwendet semantische Versionierung (SemVer). Datenbankmigrationen laufen automatisch beim Start des API-Containers.

## Standard-Upgrade (Docker Compose)

1. **Backup erstellen** (zwingend vor jedem Upgrade):
   ```bash
   make backup
   ```
   Das Backup enthält: PostgreSQL-Dump + verschlüsselter Encryption-Key-Hinweis.

2. **Neue Images ziehen:**
   ```bash
   docker compose pull
   ```

3. **Dienste neu starten:**
   ```bash
   docker compose up -d
   ```
   Der `migrate`-Service läuft automatisch vor der API und führt ausstehende Migrationen aus.

## Wichtige Verhaltensänderungen

### MFA wird zum echten zweiten Faktor (ab v0.42.35)

Bis v0.42.34 war MFA **enrollment-only**: ein korrektes Passwort stellte sofort
eine volle Session aus, und die Prüfung testete nur, *ob* TOTP eingerichtet ist —
nicht, ob die aktuelle Anmeldung den Code eingegeben hat.

Ab v0.42.35 gilt für Konten **mit** aktiviertem TOTP ein **zweistufiger Login**:

1. E-Mail + Passwort → die Antwort enthält **keine Session**, nur ein
   kurzlebiges `mfa_pending`-Token (5 Minuten).
2. Der Nutzer gibt den 6-stelligen Authenticator-Code (oder einen Backup-/
   Recovery-Code) ein → erst dann wird die echte Session ausgestellt.

**Für Operatoren:**
- Wer bereits TOTP eingerichtet hat, wird ab dem Upgrade nach dem Passwort nach
  dem Code gefragt. Das ist gewollt — kommuniziere es vor dem Upgrade, wenn in
  deiner Instanz TOTP-Nutzer existieren (`SELECT count(*) FROM totp_secrets
  WHERE enabled = true;`).
- Wer **kein** TOTP hat, merkt nichts: der Login bleibt einstufig, solange die
  Org nicht `require_mfa=true` erzwingt.
- **Recovery:** Verlorener Authenticator → der Backup-/Recovery-Code-Pfad
  (`/auth/2fa/login-verify` mit `backup_code`) stellt die Session aus. Recovery-
  Codes werden beim TOTP-Setup einmalig angezeigt — Nutzer müssen sie sichern.
- Bei `require_mfa=true` reicht Enrollment allein nicht mehr: die Session muss den
  Faktor bewiesen haben. Eine alte (Passwort-only-)Session wird auf geschützten
  Routen mit `MFA_REQUIRED` abgelehnt und muss sich neu anmelden.

4. **Verify:**
   ```bash
   docker compose ps
   curl http://localhost/health
   ```

## Kubernetes / Helm-Upgrade

```bash
helm repo update
helm upgrade vakt vakt/vakt --namespace vakt --values values.yaml
```

Helm führt den migrate-Job vor dem API-Rollout aus (init-Container-Pattern).

## Migrationssicherheit

- Migrationen sind vorwärtskompatibel: v0.N → v0.N+1 sicher
- Rückwärts-Migrationen (`.down.sql`) existieren aber sind nur für Notfälle gedacht
- Breaking-Changes werden in Major-Versionen (v1.0, v2.0) angekündigt
- Jede Migration hat `up` und `down` — bei Fehler: `.down.sql` manuell ausführen

## Versionsspezifische Hinweise

### v0.29.0 — HKDF-Schlüsseltrennung (Breaking: Sessions)

**Sessions werden invalidiert.** Der PASETO-Signing-Key wird jetzt via HKDF-SHA256 aus `VAKT_SECRET_KEY` abgeleitet statt den Raw-Key direkt zu verwenden. Alle aktiven Paseto-Tokens werden ungültig — Nutzer müssen sich nach dem Upgrade neu anmelden. Tokens sind stateless, kein Datenverlust.

**Re-encryption entfällt.** Vault-Secrets, TOTP-Secrets und Webhook-URLs werden ab v0.29.0 mit domain-separated Keys verschlüsselt. Wer eine Installation mit bestehendem Vault/TOTP-Bestand upgradet (vor v0.29.0 angelegt), müsste theoretisch re-encrypten. In der Praxis gibt es keine solche Installation — alle Instanzen sind entweder ephemer (Demo) oder werden neu aufgesetzt (Pentest). Frische Installs ab v0.29.0 schreiben alle Daten sofort mit den korrekten derived Keys.

- **Migration 147** (SCIM `expires_at`): automatisch, nullable Spalte — keine bestehenden Tokens betroffen.
- Kein Eingriff bei `VAKT_SECRET_KEY`: der Key bleibt unverändert, die Ableitung passiert im Code.
- SCIM-Token-Ablauf: `POST /api/v1/admin/scim/tokens` nimmt jetzt `expires_in_days` (0 = unbegrenzt). Bestehende Tokens ohne `expires_at` laufen weiterhin nie ab.

### v0.28.0 — Risk-Trend-Snapshots

- **Migration 145** (`idx_audit_log_org_time`): Non-concurrent Index-Erstellung, läuft in der Migrations-Transaktion. Bei sehr großen `audit_log`-Tabellen (> 5 Mio. Rows) kann die Migration etwas länger dauern — `ALTER TABLE LOCK` für die Laufzeit.
- **Migration 146** (`vb_risk_trend_snapshots`): neue Tabelle, kein Impact auf bestehende Daten. Dashboard zeigt beim ersten Tag nach dem Upgrade noch die Live-Berechnung; Snapshots füllen sich ab dem nächsten Cron-Lauf (02:30 UTC).
- Keine neuen Env-Vars.

### v0.27.0 — Command Palette

- Keine Migrations, keine Env-Vars.
- Die Command Palette (`Cmd+K`) setzt `cmdk`-Paket voraus — ist in der Frontend-Build automatisch dabei.

### v0.26.0 — Helm Migration-Job + Webhook-Verschlüsselung

**Webhook-Secrets:** neue Secrets werden ab v0.26.0 verschlüsselt gespeichert. Bestehende Plaintext-Secrets werden beim nächsten Lesen transparent entschlüsselt (lazy migration, kein aktiver Startup-Pass). Nach dem Upgrade sind Secrets nicht mehr in List/Get-Responses sichtbar. Applikationen, die das Secret via API ausgelesen haben, müssen die Credentials lokal speichern.

**Helm:** der neue `migrate-job.yaml`-Hook setzt `restartPolicy: OnFailure` und `backoffLimit: 3`. Bei Migration-Fehlern schlägt der Hook fehl und blockiert das Rollout — das ist gewollt, damit kein inkompatibles API-Image startet.

**Operator:** CRD-Group geändert von `secretops.sechealth.io` → `vakt.io/v1alpha1`. Bestehende Operator-CRD-Instanzen müssen neu erstellt werden (experimentelles Feature, kein Bestand erwartet).

**Neue Env-Var:** `VAKT_DB_MAX_CONNS` (int, Default `25`). Bestehende Deployments ohne diese Variable verwenden den Default — kein Handlungsbedarf.

### v0.25.0 — Registrierung gesperrt + Redis-Auth

**Offene Registrierung:** `POST /api/v1/auth/register` gibt 403, sobald eine Org in der DB ist. Self-hosted-Instanzen mit manuell erstellten Accounts sind nicht betroffen — die Accounts existieren bereits. Neue Self-hosted-Deployments auf einer leeren DB können sich normal registrieren.

**Migration 144** (`open_registration` auf `organizations`): nullable Boolean mit Default `false`, additiv.

**Redis-URL-Fix:** Worker-Deployments mit Redis-Passwort (`redis://:pw@...`) müssen nach dem Upgrade `docker compose up -d` ausführen — der Worker verbindet sich jetzt korrekt. Kein Config-Eingriff nötig, nur Restart.

### v0.22.0–v0.20.0

- **Migration 139** (Asset `environment`-Spalte, DEFAULT `'prod'`): bestehende Assets erhalten automatisch `environment = 'prod'`.
- Keine weiteren Breaking Changes.
- Supplier Portal (`/supplier/:token`) ist öffentlich ohne Auth — sicherstellen dass der Reverse Proxy den Pfad nicht blockiert.

### v0.19.0

- **Migration 140** (BSI Reporting — `ck_incidents.classification_result`, `ck_incidents.reporting_deadlines`): automatisch angewendet.
- Neue Org-Settings-Felder `sector` und `admin_ip_allowlist` sind optional.

### v0.18.0

- **Migrationen 136–138**: werden automatisch angewendet.
- Keine Breaking Changes in bestehenden APIs.

### v0.18.x (S105 — Auth & User Provisioning)

- **SAML 2.0 ist jetzt Pro-Feature** (ADR-0067, S105-4): War in v0.17.0 als CE eingeführt,
  seit S105-4 hinter `FeatureSAMLAuth` (Pro). CE-Instanzen ohne Pro-Key sehen stattdessen
  einen Upgrade-Prompt in den Settings. → Upgrade-Pfad: Pro-Lizenz aktivieren oder
  auf passwortbasiertes Login wechseln.
- **Migration 227** (`org_oidc_configs`): Neue Tabelle für OIDC/Casdoor DB-Konfiguration.
  Env-Vars `CASDOOR_URL`/`CASDOOR_CLIENT_ID`/`CASDOOR_CLIENT_SECRET` funktionieren
  weiterhin als Fallback — kein Handlungsbedarf für bestehende Deployments.
- **Migration 228** (`org_saml_configs.jit_provisioning`): Neue Spalte mit Default `TRUE` —
  bestehende SAML-Konfigurationen bekommen JIT-Provisioning automatisch aktiviert.
- **Direktes User-Anlegen** (CE): `POST /api/v1/admin/users` — kein SMTP mehr erforderlich.

### v0.17.0

- **SAML (damals CE, jetzt Pro ab v0.18.x):** Konfiguration über Org-Settings → SSO.
- **SCIM/SIEM:** Nur für Pro-Lizenzen. Kein Handlungsbedarf ohne Pro-Lizenz.
- `docs/wiki/enterprise-sso.md` für Setup-Details.

### v0.16.0

- **Keine Breaking Changes.** Neue Worker-Queue-Namespaces sind rückwärtskompatibel (bestehende Jobs laufen in Default-Queue weiter).
- Empfohlen: `docker compose pull && docker compose up -d` genügt.

### v0.15.0 (Sprint 28 — NIS2 Pro-Layer)

- **Migration 127 automatisch:** Neue Tabelle `ck_nis2_assessment_runs` für Re-Assessment-History. Keine manuelle Aktion, keine Downtime.
- **NIS2 Pro-Features erfordern `FeatureNIS2Reporting`-License-Flag:** Embedded-Mode (iframe), Branded PDF-Export, Re-Assessment-History, Multi-Framework-Wizard sind hinter dem Pro-Gate. CE-Features (`/nis2-check`, Score-Engine, Sign-up-Migration) bleiben unverändert kostenlos.
- **Neue öffentliche Route `/nis2-check/multi`:** Reverse-Proxy / WAF: Pfad-Prefix `allow-list` ergänzen (analog `/nis2-check` aus v0.12.0). Standard-nginx-Config im Repo ist bereits korrekt.
- **Embedded-Mode CORS:** `Access-Control-Allow-Origin: *` wird ausschließlich auf öffentlichen NIS2-Wizard-Endpoints gesetzt. Kein Impact auf authenticated API-Endpoints. Wer einen strikten CORS-Filter vorgelagert hat, muss `/api/v1/public/nis2-assessment/*` und `/nis2-check*` in die Allow-List aufnehmen.
- **PDF-Generator Abhängigkeit:** `github.com/go-pdf/fpdf` ist seit v0.15.0 in `go.mod`. Self-hosted-Builds mit pinned Dependencies: Paket nachholen (`go mod download`).

### v0.14.3 (Sprints 24-27 — Interne Qualitätswelle)

Reine interne Verbesserungen. **Keine neuen Migrations, keine API-Breaking-Changes, kein manueller Eingriff nötig.**

- **sqlc-Migration vollständig:** SecPulse, SecVitals, SecReflex, Vakt HR und Vakt Vault sind jetzt vollständig auf sqlc-generierte Queries umgestellt. Kein Verhaltensunterschied für laufende Instanzen — betrifft nur Maintainer, die Raw-SQL in `repository.go` gepatcht haben. Drei dokumentierte Ausnahmen in Vakt Vault bleiben Embedded-SQL (Crypto-Bytes, ON CONFLICT-Muster).
- **IP-Lockout aktiv:** Nach 10 fehlgeschlagenen Logins von derselben IP wird die IP für 15 Minuten gesperrt. Counter liegt in Redis — bei Redis-Ausfall: kein Lockout (Fail-Open). Falls eine IP fälschlicherweise gesperrt ist: Redis-Key `login_lockout:<ip>` löschen.
- **Backup-HMAC:** Backup-Archive (`make backup`) werden ab v0.14.3 mit HMAC-SHA256 signiert. Bestehende Archive ohne Signatur können weiterhin restored werden — Prüfung ist optional und wird via `--verify-hmac`-Flag aktiviert (Default off, damit alte Backups weiter funktionieren).

### v0.14.2 (Build-Hotfix)

Funktional identisch zu v0.14.1. Reiner Build-Fix: `npm run build` war seit v0.14.0 rot wegen zweier in der OpenAPI fehlender Schemas (`HealthResponse`, `DemoStartResponse`) und eines unused-state Warnings in Setup.tsx. Wer v0.14.0 oder v0.14.1 in CI mit `npm run build` benutzt: hier upgraden — sonst nichts zu tun.

### v0.14.1 (Sprint 22 Tail — Frontend-Komponenten + Cleanup-Job-Tests)

Schließt die in v0.14.0 zurückgestellten Items aus Sprint 22 ab. Keine Migration. Vollständig additiv.

- **AI-Agent-Page (`/vaktcomply/ai/agent`) nutzbar:** Login → SecVitals → AI-Agent → Goal eingeben. Backend-Endpoint `POST /api/v1/vaktcomply/ai/agent/run` existierte schon seit Sprint 18, hatte aber keine UI. Jetzt mit Live-Visualisierung der Plan/Tool-Call/Result-Events.
- **ApiKeysPage hat Scope-Picker:** Bei "Key erstellen" jetzt Checkbox-Liste pro Modul. Personal-Keys (alte Default) bleiben das Verhalten wenn keine Box gesetzt — sind aber explizit als „Full Access" markiert (amber Badge). Bestehende Keys bleiben unverändert.
- **API-Key-Rotation per Klick:** Drehbutton pro Key öffnet Modal mit Grace-Period-Erklärung. Frontend zeigt nach Rotation den neuen Raw-Key einmalig.
- **`last_used_ip` wird ab v0.14.1 gepflegt** — Middleware schreibt sie nun bei jedem API-Key-Auth-Erfolg. Bestehende Keys haben den Wert leer bis zur nächsten Verwendung.
- **SessionsPage zeigt aktuelle Session:** Vom Backend zurückgegebenes `is_current`-Flag macht die laufende Session in der Tabelle erkennbar. Neuer „Panic"-Button beendet auch die eigene Session (2-Step-confirm, auto-redirect auf `/login`). Bestehende Sessions vor dem Upgrade bekommen kein `session_id` im Frontend → die „Diese hier"-Markierung wird ab dem ersten Re-Login angezeigt.
- **Login-Response enthält neu `session_id`:** Custom-Frontend-Forks die `LoginResponse` typisieren: das Feld ist `string | undefined`, additiv. Wer es nicht nutzt, ignoriert es.
- **Integration-Tests ergänzt:** `cleanup_jobs_real_test.go` testet die zwei Cleanup-Jobs aus Sprint 22 mit echter Postgres via testcontainers. Run via `go test -tags=integration ./internal/integration_test/...`. CI: braucht Docker-Daemon.

### v0.14.0 (Sprint 22 — Fertigstellungs-Welle für Sprints 17-20)

- **🔧 Bugfix Rotation (S22-1):** Customer mit aktiven rotierten API-Keys: die Rotation in v0.13.0 hat den alten Key sofort tot gemacht statt nach 24h Grace. **In v0.14.0 funktioniert die Grace-Period wie versprochen.** Wer in v0.13.0 eine Rotation gemacht hat und der alte Key ist immer noch im `previous_key_hash`-Feld (Grace nicht abgelaufen): nach v0.14.0-Upgrade ist der alte Key wieder gültig bis zum dokumentierten Sunset-Datum. Bestehende kaputte CI-Pipelines können sich erholen.
- **Login-History erweitert:** v0.13.0 schrieb nur Password-Logins. v0.14.0 schreibt auch OIDC, SAML, Register-Setup. Historische Login-Versuche bleiben unverändert; ab v0.14.0 sind alle Pfade abgedeckt.
- **Neue Endpoints (additiv):**
  - `POST /api/v1/vaktcomply/nis2-assessment/migrate-from-anonymous` — Sign-up-Übernahme der NIS2-Wizard-Antworten.
  - `GET /api/v1/account/login-history` — Liste der letzten 50 Login-Versuche.
- **Frontend NIS2-Sign-up-Flow:** Setup-Page liest `?nis2_token=` aus URL ODER `vakt_nis2_token` aus localStorage und ruft nach Account-Erstellung den Migrate-Endpoint. Wer einen Custom-Sign-up-Fork hat: gleichen Pfad einbauen.
- **Asynq-Cleanup-Jobs aktiv:** Customer im Worker-Profile bekommt jetzt zwei neue Periodic-Tasks: `nis2:cleanup_anonymous_runs` (täglich) und `auth:cleanup_login_history` (wöchentlich). Beide via Asynq-Scheduler — kein manueller Eingriff nötig. Schließt die UPGRADE-Hinweise aus v0.12.0 + v0.13.0 (manueller `DELETE`).
- **`ScanProgressIndicator`-Komponente** verfügbar in `modules/vaktscan/components/`. Nicht automatisch auf der ScanDetailPage gemountet — Customer-Forks können sie selber an passender Stelle einbauen. Mounten in Standard-Vakt kommt in Folge-Welle.
- **`LoginHistorySection`** ist automatisch in der AccountSettingsPage sichtbar. User sehen ab dem ersten Login nach v0.14.0 ihre Login-Historie.

### v0.13.0 (Sprint 20 — Enterprise-Auth CE-Tier)

- **Migration 126:** Erweitert `api_keys` (rotation-fields) + neue Tabelle `login_history`. Automatisch beim Rollout.
- **Bestehende API-Keys:** behalten `["*"]`-Scope (Default vor v0.13.0). Migration ist no-op für Bestands-Keys — sie funktionieren weiterhin wie bisher.
- **Neue Endpoints (additiv):**
  - `POST /api/v1/api-keys/:id/rotate` — Rotation mit 24-h-Grace-Period.
- **Empfehlung:** Audit-Log nach v0.13.0-Rollout filtern auf `auth_method=api_key, scope_used="*"`. Pro identifizierten Key Min-Scope-Liste ableiten und neuen Key erstellen → alten revoken (Anleitung in `docs/concepts/api-key-scopes.md`).
- **Login-History:** ab sofort werden alle Login-Versuche (auch failed) in `login_history` persistiert. 90-Tage-Retention-Cleanup-Job kommt in Folge-Welle — bis dahin manuell `DELETE FROM login_history WHERE ts < NOW() - INTERVAL '90 days'`. Volumen-Schätzung: 5 Logins/User/Tag × 30 Tage = 150 Rows/User/Monat, vernachlässigbar für < 1000 User.
- **Skeleton-Status RequireScope-Middleware:** verdrahtet + einsetzbar, aber nur als manueller Mount auf einzelnen Routen. Vollständige 200-Routes-Annotation ist Sprint-21+-Hardening-Welle. Bis dahin: API-Keys mit `["*"]`-Scope haben effektiv keine Scope-Restriktion auf Endpoints (RequirePermission greift weiterhin).

### v0.12.0 (Sprint 19 — NIS2-Self-Assessment-Wizard CE)

- **Neue Migration 125:** `nis2_anonymous_runs` + `ck_nis2_assessments`. Automatisch beim Rollout.
- **Neue Public-Endpoints `/api/v1/public/nis2-assessment/*`** — kein Auth, Rate-Limit 5/min/IP. Customer-WAF/Reverse-Proxy: Pfad-Prefix allow-listen.
- **Neue Public-Route `/nis2-check`** im Frontend — kein Auth, kein Layout-Wrapper. Reverse-Proxy: explizit allow-listen.
- **Cleanup-Job pending:** abgelaufene anonyme Runs müssen manuell via `DELETE FROM nis2_anonymous_runs WHERE expires_at < NOW()` gelöscht werden, bis Asynq-Periodic-Task in Folge-Welle hinzukommt. Bei < 1000 Wizard-Starts/Monat unkritisch.
- **DSGVO:** Anonyme Runs speichern IP-Hash (sha256 + `VAKT_SECRET_KEY`-Salt), keinen Klartext. Kein VVT-Eintrag. Magic-Token in localStorage ist functional.

### v0.11.0 (Sprint 18 — Agentic-AI v2)

- **Keine API-Breaking-Changes.** Ein neuer SSE-Endpoint `POST /api/v1/vaktcomply/ai/agent/run` — additiv.
- **Keine DB-Migrations.** Audit-Trail-Erweiterung nutzt die bestehende `audit_log`-Tabelle mit `action='agent_run_start'`.
- **Permissions-Plumbing:** Der AgentRunner liest User-Permissions aus dem Echo-Context-Key `permissions []string`. Customer-Forks mit anderer Permission-Repräsentation müssen das hier ergänzen. Bei leeren Permissions kann der Agent keinen Default-Tool nutzen — alle drei haben Scope.
- **ADR-0020 als Pflicht-Pattern:** Custom-Tools müssen `RequireScope()` explizit deklarieren. Leere Implementation auf mutierenden Tools wird in einer Folge-Welle vom Linter geblockt.
- **AI-Quota:** Ein Agent-Run zählt wie eine AI-Anfrage in `VAKT_AI_RATE_LIMIT_RPM` und `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG`. Bei intensiver Agent-Nutzung Quota anheben.
- **`X-Accel-Buffering: no` ist gesetzt** — die nginx-Config aus v0.10.0 deckt den Agent-Stream ohne Änderung ab (matcht `/api/v1/.+/stream$`).

### v0.10.0 (Sprint 17 — Realtime-Welle)

- **Keine API-Breaking-Changes.** Zwei neue SSE-Endpoints (`/dashboard/notifications/stream`, `/vaktscan/scans/:id/progress/stream`) — additive Erweiterungen.
- **nginx-Konfig anpassen:** Customer mit eigenem nginx-Reverse-Proxy MUSS einen `location ~ ^/api/v1/.+/stream$`-Block mit `proxy_buffering off` + `proxy_read_timeout 1h` setzen, sonst werden SSE-Frames gepuffert. Vollständiges Snippet in `docs/wiki/reverse-proxy.md`. Caddy/Traefik/HAProxy funktionieren ohne Anpassung. Cloudflare-Tunnel: SSE-Limit 100 s, Client-Reconnect ist im Hook bereits behandelt.
- **Polling-Pfad bleibt funktional:** `GET /dashboard/notifications` und `GET /vaktscan/scans/:id` bleiben verfügbar. Wer ohne SSE-fähigen Proxy fährt, kann die alten Polling-Endpoints nutzen — keine Pflicht-Migration.
- **NotificationBell-Polling entfernt:** `refetchInterval` aus `useNotifications` ist raus. Frontend hängt jetzt am Stream + React-Query-Cache. Wenn der Stream-Endpoint nicht erreichbar ist, sieht der User keine neuen Notifications mehr automatisch — das ist der Trade-off. Workaround: bei deaktiviertem Stream zurück auf `refetchInterval: 60_000` im Customer-Fork.

### v0.9.0 (Sprint 16 — Frontend-Polish + Doku)

- **Keine API-Änderungen, keine DB-Migrations.** Sprint 16 ist eine reine Frontend- + Doku-Welle.
- **Severity-Farben als Design-Tokens:** Customer-Forks mit Custom-Tailwind-Komponenten, die `bg-[#ef4444]` u.a. nutzen, bekommen jetzt `bg-severity-critical` als saubere semantische Klasse. Alte bracket-Notations sind in `frontend/src/` 0× vorhanden — Forks sollten ihre Patches anpassen.
- **`React.lazy()` Code-Splitting:** Wer Custom-Pages in `src/pages/` ergänzt hat, sollte sie analog auf `lazy()` umstellen, sonst landen sie im Initial-Bundle.
- **`formatLocale()` statt `'de-DE'`:** Custom-Code, der Datums-Formatierung über `Intl.*`-APIs macht, sollte den Helper aus `frontend/src/shared/utils/locale.ts` nutzen statt hardcoded Strings. Andernfalls bleibt die Sprache des Customer-Codes auf Deutsch festgenagelt unabhängig von der User-Locale-Auswahl.
- **`openapi-typescript` als devDependency:** `npm install` läuft 1× im Build-Pfad. CI-Step `npm run api-types:check` enforced, dass `src/api/generated.ts` synchron zur OpenAPI-Spec ist — wer den OpenAPI-Spec ändert, muss `npm run api-types` nochmal laufen lassen und das Resultat committen.

### v0.8.0 (Sprint 14 + 15 — Stabilität + AI-Härtung + Observability)

- **Neue Migration 124:** `ai_usage`-Tabelle für Token-/Cost-Tracking pro Org. Läuft beim Rollout automatisch über `cmd/migrate`. Keine Customer-Aktion nötig.
- **Goroutine-Pattern via `safego.Run` (ADR-0018):** existierende Customer-Deployments brauchen nichts zu tun. Custom-Forks, die rohe `go func()` in `internal/` einsetzen, werden vom neuen `forbidigo`-Linter geblockt — Migration auf `safego.Run` empfohlen.
- **`cmd/worker/main.go` aufgeteilt** in main/handlers/scheduler/util. Wer das Worker-Image selbst baut, muss nichts ändern — der Build-Pfad bleibt `./cmd/worker`.
- **`internal/shared/` Konsolidierung:** vier Pakete sind nach `internal/services/` umgezogen (`ai`, `alerting`, `evidence_auto`, `crossevidence`). Customer-Forks mit Custom-Patches in diesen Pfaden müssen den Import-Pfad aktualisieren. Eine sed-Hilfe steht im Sprint-15-Commit-Body.
- **AI-Härtung default-on (S15-1/2/3):** Rate-Limit 30 req/min pro Org, Response-Cache 1h. Beide opt-out über die jeweiligen Env-Vars (siehe CHANGELOG). Wer Cloud-LLMs nutzt und Kosten tracken will: `VAKT_AI_COST_PER_MTOKEN_IN/OUT_MICRO_EUR` setzen.
- **Prometheus `/metrics` default-on (S15-11):** vorher opt-in via `VAKT_METRICS_ENABLED=true`. Jetzt immer aktiv, opt-out via `VAKT_METRICS_DISABLED=true`. Endpoint bleibt IP-allowlisted (Loopback + Docker-Netz) — Customer ohne Prometheus brauchen nichts zu ändern.
- **Observability-Stack erweitert:** wer das Profile `observability` nutzt, bekommt jetzt Prometheus (Port 9091) + AlertManager (Port 9093) zusätzlich zu Loki + Tempo + Grafana. Vier provisionierte Dashboards (API/Worker/AI/Demo) sind automatisch im Grafana-Folder „Vakt" verfügbar.

### v0.7.0 (Sprint 13 — Reife-Sanierung Welle 2)

- **Helm-Chart umbenannt:** `helm/sechealth/` → `helm/vakt/`. Eigene CI/CD-Pipelines, die per Pfad installieren, müssen angepasst werden:
  ```bash
  # Vorher
  helm install vakt ./helm/sechealth ...
  # Jetzt
  helm install vakt ./helm/vakt ...
  ```
  Das chart-interne Template-Namespace (`define "sechealth.xxx"`) wurde ebenfalls zu `vakt.xxx` migriert — externe Konsumenten sind davon nicht betroffen, weil das namespace nicht in den Rendered Manifests landet.
- **Helm-Defaults verschärft:** `postgresql.auth.password` darf nicht mehr den Default-Wert `"changeme"` haben und muss mindestens 16 Zeichen lang sein. Bei Upgrade einer bestehenden Installation entweder explizit setzen:
  ```bash
  helm upgrade vakt ./helm/vakt --set postgresql.auth.password=$(openssl rand -hex 32) ...
  ```
  oder über `postgresql.auth.existingSecret` referenzieren (Bitnami-Standardpattern).
- **Redis-Auth default-on:** `redis.auth.enabled` ist von `false` auf `true` gewechselt. Bitnami generiert das Passwort automatisch, wenn `redis.auth.password` leer bleibt. Cluster mit NetworkPolicy default-deny können `--set redis.auth.enabled=false` setzen und tragen damit die Verantwortung.
- **CLI-Binary `cmd/sechealth` entfernt:** war Legacy nach Rebrand. Wer den Vakt-CLI-Client braucht, baut ihn aus dem `pkg/sdk/`-Code oder wartet auf eine neue `cmd/vakt-cli/`-Iteration in einer späteren Welle.
- **Trivy CI-Gate verschärft:** `ignore-unfixed: false`. Builds können neue Vulnerabilities ohne verfügbaren Fix anschlagen; Akzeptanzen kommen in `.trivyignore` mit Begründung + Re-Check-Datum.
- **SSRF-Guard für `VAKT_AI_BASE_URL`:** beim Startup werden URLs auf IMDS (169.254.169.254), Loopback, link-local und `localhost` als Hostname abgelehnt, wenn `VAKT_AI_PROVIDER != "disabled"`. Allowliste: `ollama`, `ai-llm`, `llm-proxy`, `lm-studio` als Service-Discovery-Namen plus alle Public-DNS-Hostnames.
- **bcrypt cost-upgrade-on-login:** Legacy-User mit cost < 12 bekommen beim nächsten erfolgreichen Login transparent einen Re-Hash. Keine Aktion erforderlich.
- **Audit-Redaction erweitert:** `recovery_code`, `backup_code`, `otp`, `totp_code`, `mfa_token` werden jetzt in Audit-Bodies redacted. Bestehende Audit-Einträge bleiben unverändert.
- **LemonSqueezy Webhook-Replay-Schutz:** neue Migration `123_lemonsqueezy_webhook_events` deduped Events auf sha256(body). Beim Rollout läuft die Migration automatisch.

### v0.6.0
- **Neue Migrations:** 104 (Access Reviews), 105 (Control Exceptions), 106 (Control Owner)
- **Auth-Änderung:** Token-Storage wechselt von localStorage zu httpOnly-Cookie. Alle aktiven Sessions werden nach dem Upgrade automatisch neu authentifiziert.
- **Prometheus-Metriken:** Prefix geändert von `sechealth_` → `vakt_`. Bestehende Grafana-Dashboards müssen aktualisiert werden.
- **CSP verschärft:** `script-src` erlaubt kein `unsafe-inline` mehr. Custom Inline-Scripts brechen.

### v0.5.x → v0.6.0
Kein manueller Eingriff nötig. Alle Änderungen sind rückwärtskompatibel im DB-Schema.

## Rollback

Falls ein Upgrade schief geht:

1. Services stoppen: `docker compose down`
2. Backup wiederherstellen: `make restore BACKUP=<datei.tar.gz>`
3. Alte Images taggen und neu starten

## Support

- GitHub Issues: https://github.com/norvik-ops/vakt/issues
- Dokumentation: https://vakt.norvikops.de/docs
