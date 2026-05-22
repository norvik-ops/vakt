# Changelog

All notable user-facing changes to Vakt are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### v0.15.0 — NIS2 Pro-Layer (Tag-Kandidat, Sprint 28)

Schließt die Pro-Schicht aus Sprint 19 vollständig ab. Kein Breaking-Change — alle neuen Features sind additiv und hinter `FeatureNIS2Reporting` Pro-gated. CE-Features des NIS2-Wizards bleiben unverändert.

**S28-1 Embedded-Mode:**
- NIS2-Self-Assessment-Wizard via `<iframe>` einbettbar auf Partner- und Berater-Sites.
- CORS `Access-Control-Allow-Origin: *` auf öffentlichen Wizard-Endpoints (`/api/v1/public/nis2-assessment/*`).
- `X-Frame-Options`-Header wird auf `/nis2-check*`-Routen entfernt; CSP `frame-ancestors *` gesetzt.
- Resize-Helper `public/nis2-embed.js` (PostMessage-basiert, 26 Zeilen, kein Tracking, kein Cookie).

**S28-2 Branded PDF-Export (Pro, `FeatureNIS2Reporting`):**
- `GET /api/v1/public/nis2-assessment/:token/export-pdf` — generiert mehrseitiges PDF: Cover mit Gesamtscore, Bereichs-Tabelle, Top-Gaps, Detailantworten.
- Footer „Erstellt mit Vakt · vakt.io". Rückgabe als `application/pdf` Blob (filename `nis2-assessment.pdf`).
- Frontend-Download-Button im Result-Screen — sichtbar nur wenn authentifiziert. Bei `402 Payment Required`: Upgrade-CTA.

**S28-3 Re-Assessment-History (Pro, `FeatureNIS2Reporting`):**
- Neue Tabelle `ck_nis2_assessment_runs` (Migration 127): speichert vollständige Assessment-Runs mit Scores + Top-Gaps.
- 90-Tage-Cooldown zwischen Re-Assessments — `429 Too Many Requests` mit `Retry-After`-Header bei Verletzung.
- Endpoint `GET /api/v1/secvitals/nis2-assessment/history` liefert alle Runs sortiert nach Datum.
- Frontend-Seite `/secvitals/nis2-history`: Trend-Pfeile (TrendingUp / TrendingDown) pro Bereich, Delta-Spalte zum Vorrun, Cooldown-Restanzeige, Leer-State mit CTA.

**S28-4 Multi-Framework-Wizard (Pro, `FeatureNIS2Reporting`):**
- 80 kombinierte Fragen: NIS2 (~30), ISO 27001 (~25), DSGVO-TOM (~25). Stabile IDs mit `mf.`-Prefix.
- 23 Cross-Mapping-Fragen, die mehreren Frameworks angerechnet werden (Ref-Feld pro Frage).
- Score-Engine `MultiFrameworkScore`: `NIS2`, `ISO27001`, `DSGVO`, `Overall`, `TopGaps`, `ByFramework`.
- Neue Route `/nis2-check/multi` — eigene Frontend-Page mit drei Fortschrittsbalken (NIS2 indigo, ISO27001 emerald, DSGVO violet) + Cross-Mapping-Hinweis im Result.

**S28-5 Landing-Page SEO:**
- `docs/marketing/nis2-check-landing.md` — deutschsprachige SEO-Vorlage für `vakt.io/nis2-check`.
- Meta-Block (title, description, canonical), Hero, NIS2-Bereichs-Tabelle, 3-Schritt-Flow, Zielgruppen-Blöcke, FAQ (5 Fragen inkl. DSGVO-Hinweis), Legal-Disclaimer. Optimiert auf „NIS2 Self-Assessment", „NIS2 Umsetzungsgesetz", „BSI NIS2 Compliance Check".

---

### v0.14.3 — Interne Qualitätswelle (Sprints 24-27, kein User-Impact)

Keine neuen User-facing-Features. Keine DB-Migrations. Kein Upgrade-Eingriff nötig.

**S24 — UX-Polish + Security-Hardening:**
- **`Spinner`-Komponente** als zentrale Ladeanimation eingeführt; Inline-`div`-Spinner in Frontend entfernt.
- **`StatusMapping`-Bibliothek** — zentralisierte `Record`-Typen für Status/Severity-Farb- und Label-Mappings; keine gestreuten `switch`-Blöcke mehr.
- **Toast-Migration** — verbleibende Inline-`fixed-bottom`-Toast-Blöcke auf globalen `toast()`-Hook umgestellt.
- **Settings-Modul** — 6 Settings-Pages nach `modules/settings/pages/` migriert (saubere Modul-Struktur).
- **IP-Lockout** — per-IP Redis-Failure-Counter: nach 10 fehlgeschlagenen Logins wird die IP für 15 Minuten gesperrt. Brute-Force-Schutz auf Login-Endpoint.
- **Backup-HMAC** — Backup-Archive werden mit HMAC-SHA256 signiert; Integritätsprüfung beim Restore.

**S25 — sqlc-Welle 1 (SecPulse + SecVitals) + E2E:**
- **SecPulse sqlc-Abschluss** — 3 verbleibende Raw-SQL-Stellen in `secpulse/` auf sqlc migriert.
- **SecVitals sqlc Wellen 1+2** — `service_soa`, `approvals_handler`, `handler_my_tasks`, `milestones_repository` auf sqlc.
- **Playwright E2E V22-1** — Sessions-Panic-2-Step-Confirm, ApiKeys-Rotate-Modal, AgentRunPanel-Visualisierung. Schließt V22-1 aus dem Verifizierungs-Backlog ab.

**S26 — sqlc-Welle 2 (SecVitals + SecReflex + HR):**
- **SecVitals sqlc Wellen 3+4+5** — `handler_ical`, `handler_templates`, `service_policies`, `service_frameworks`, `handler_boardreport`, `service_reporting`, `policy_acceptance` auf sqlc.
- **SecReflex + Vakt HR sqlc-Abschluss** — alle verbleibenden Raw-SQL-Stellen in beiden Modulen migriert.

**S27 — sqlc-Abschluss Vakt Vault + E2E Verification:**
- **Vakt Vault sqlc komplett** — 29 neue sqlc-Queries (Shares, API-Tokens, Git-Scans, Scan-Results, Rotation-Policies, Access-Log, Secrets-Metadata). Drei dokumentierte Ausnahmen bleiben Embedded-SQL: `UpsertSecret` (ON CONFLICT + Crypto-Bytes), `GetSecretRaw`, `GetSecretByID` — beide geben `[]byte`-Encrypted-Value zurück, das sqlc-Code-Gen nicht abbilden kann.
- **SecPulse CI-Evidence** — `INSERT INTO ck_evidence` in `handler_ci_evidence.go` auf `r.q.InsertCKCIEvidence` migriert.
- **E2E Grace-Period-Badge** — Playwright-Test für `API_KEYS_IN_GRACE`-Fixture (rotated_at = jetzt → `text=Grace 24h aktiv` sichtbar). Schließt V22-1 vollständig ab.

---

### v0.14.2 — Build-Hotfix (2026-05-23)

Pure Build-Fix. Funktional identisch zu v0.14.1 für den Runtime-Pfad.

- **OpenAPI-Drift gefixt:** `HealthResponse` und `DemoStartResponse` Schemas waren in `backend/internal/shared/apidocs/openapi.yaml` nie definiert, wurden aber in `frontend/src/pages/Login.tsx` per `components['schemas']` referenziert. `npm run build` (tsc -b) ist deshalb seit v0.14.0 rot. Schemas nachgezogen, Types regeneriert. ADR-0017-Honesty-Audit-Miss.
- **`Setup.tsx` dead state entfernt:** `migratedMsg`-useState wurde gesetzt, dann `navigate('/')` — gerendert wurde es nie. Auf `toast()` umgestellt, damit der User die NIS2-Migrations-Bestätigung nach dem Sign-up auch tatsächlich sieht.
- **Verifizierung:** `go test ./...` + `npm run build` + `npm run test` alle grün.

### Sprint 22 Tail — Verbleibende Frontend-Komponenten + Tests (Tag-Kandidat v0.14.1)

Schließt die 4 in v0.14.0 zurückgestellten Items aus Sprint 22 ab. Damit ist der Sprint-22-Honesty-Audit vollständig abgearbeitet.

**S22-8 AgentRunPanel-Frontend:**
- Neuer Hook `useAgentRun` (`frontend/src/shared/hooks/useAgentRun.ts`) konsumiert den SSE-Stream von `POST /api/v1/secvitals/ai/agent/run`, parsed strukturierte `AgentEvent`-Frames (plan / tool_call / tool_result / reflect / final / error) und liefert `events[]`, `isRunning`, `error`, `durationMs`, `start()`, `stop()`.
- Neue Komponente `AgentRunPanel` (`frontend/src/shared/components/AgentRunPanel.tsx`): Goal-Input, Start/Stop-Button, Event-Cards mit farbcodierten Typen, JSON-Expand/Collapse pro Event für Arguments + Result.
- Neue Page `AIAgentPage` unter `secvitals/ai/agent` — mountet das Panel, listet erlaubte Tools/Approve-Skelett.

**S22-9 ApiKeysPage-Refactor:**
- **Scope-Picker im Create-Dialog**: Checkbox-Liste pro Modul (`secvitals.*`, `secpulse.*`, `secvault.*`, `secreflex.*`, `secprivacy.*`, `hr.*`) mit Beschreibungstexten. Leer = Personal-Key (Full Access, ambers gekennzeichnet).
- **Rotate-Button pro Key** mit eigenem Modal: Erklärt die 24h Grace-Period explizit, zeigt den neuen Raw-Key nach Rotation einmalig im New-Key-Dialog.
- **Scope-Tags und Grace-Indicator** pro Row: code-style-Pills mit dem Scope-String, oder „Personal (Full Access)"-Badge wenn leer. Während aktiver Grace-Period zusätzlich „Grace 24h aktiv"-Marker.
- **last_used_ip-Anzeige** unterhalb von last_used_at (klein, monospace).

**Backend-Begleitänderungen:**
- `apikeys.APIKey` Struct um `LastUsedIP` + `RotatedAt` erweitert; `List` selectiert beide Felder mit. Middleware-Hook für API-Key-Auth-Erfolg updated jetzt zusätzlich `last_used_ip` aus `c.RealIP()`.

**S22-10 Session-Management — Current-Session-Marker + Panic-Button:**
- `auth.AuthResponse` um `session_id` (UUID der `refresh_sessions`-Row) erweitert. `issueTokenPair` nutzt `RETURNING id::text`, damit Login/Register/Refresh die ID mitliefern.
- Frontend `api/client.ts` um `getSessionId()`/`setSessionId()`-Helpers erweitert; `apiFetch` sendet die ID als `X-Vakt-Session-Id` Header automatisch mit. `Login.tsx` persistiert die ID in localStorage; `setAuthToken(null)` löscht sie wieder.
- `auth.SessionHandler.ListSessions` markiert die zur Header-ID passende Row mit `is_current: true`. `RevokeAllOtherSessions` nutzt die Header-ID statt einer nicht-funktionierenden Token-Hash-Vergleichslogik.
- `SessionsPage` zeigt „Diese hier"-Badge + last_used pro Session, separiert „Andere abmelden" und einen 2-Step-confirm Panic-Button („inkl. dieser") mit auto-redirect auf `/login` nach Revoke.
- OpenAPI-Spec entsprechend nachgezogen: `LoginResponse` um `session_id`, `SessionInfo` an Backend-Form angepasst (`device_hint`, `last_used`, `is_current`) — gem. ADR-0017.

**S22-14 Integration-Tests für Cleanup-Jobs:**
- Neue Test-Datei `internal/integration_test/cleanup_jobs_real_test.go` (build-tag `integration`):
  - `TestCleanupAnonymousRuns_DeletesExpiredRows` — seedet 1 expired + 1 fresh Row in `nis2_anonymous_runs`, ruft `nis2wizard.CleanupAnonymousRuns`, asserted nur expired ist weg.
  - `TestCleanupLoginHistory_DeletesOldEntries` — seedet 1 Eintrag vor 100 Tagen + 1 frischer Eintrag in `login_history`, ruft `auth.CleanupLoginHistory`, asserted Retention-Grenze 90d sauber.
- Beide Tests bootstrap Postgres via testcontainers-go (analog zu `hr_evidence_real_test.go`), skippen sauber wenn Docker nicht verfügbar.

**Operations-Doku:**
- `docs/operations/maintenance-window-server-upgrade.md` — Wartungsfenster-Plan für Strato VC-2-4 → VC-6-12 Upgrade: Pre-Flight (T-24h, T-1h), Live-Migration vs. Backup-Restore-Variante, Post-Flight-Validierung (Health-Smoke aus ADR-0017 Checklist), Rollback-Strategie, Kommunikations-Schema.

### Sprint 22 — Fertigstellungs-Welle für Sprints 17-20 (Tag-Kandidat v0.14.0)

Schließt die Skeleton-Lücken aus 17-20 nach dem Honesty-Audit vom 2026-05-22. Kein neues Feature-Versprechen, sondern Einlösung alter. 12 Items voll-implementiert, 4 größere Frontend-Komponenten als [~] in nachfolgende Welle verschoben.

**22.1 Backend-Bugs (echte Defekte):**
- **S22-1 Auth-Lookup mit Grace-Period:** API-Key-Auth-Middleware akzeptiert jetzt `previous_key_hash` während `previous_key_grace_expires_at > NOW()`. Beim Match über alten Hash: Response-Header `X-Vakt-Key-Deprecated: true` + `Sunset: <RFC1123>` als Migrations-Signal. **Bug aus Sprint 20 effektiv broken Rotation** ist gefixt.
- **S22-2 RequireScope-Kontext-Plumbing:** Auth-Middleware setzt jetzt `auth_method=api_key`, `api_key_scopes`, `api_key_id` im Echo-Context. `apikeys.RequireScope(scope)`-Middleware kann das nun nutzen — manuelles Mounten auf Routen ist möglich. Volle 200-Route-Annotation ist noch eigener Sprint, aber das Plumbing steht.
- **S22-3 OIDC + SAML + Register schreiben login_history:** `auth.OIDCLogin`, `auth.SAMLLogin`, `auth.Register` rufen jetzt `recordLogin` mit source=`oidc`/`saml`/`register`. Failed-OIDC-Provisioning auch als `oidc_failed`. Sprint 20 hatte nur Password-Pfad — Audit-Gap geschlossen.

**22.2 Sign-up-Integration (NIS2-Akquise-Loop schließen):**
- **S22-4 Setup.tsx liest `?nis2_token=` + localStorage** und ruft nach erfolgreichem Setup `POST /secvitals/nis2-assessment/migrate-from-anonymous` auf. CTA aus dem Public-Wizard läuft jetzt nicht mehr ins Leere.
- **S22-5 Auto-Mapping auf NIS2-Controls** in `nis2wizard.AutoMapToControls`: value 0-1 → `not_implemented`, 2 → `partial`, 3-4 → `implemented`. Mapping via NIS2-Ref-Substring auf `ck_controls.description`/`control_id`. Nur Controls ohne aktiven manual_status werden überschrieben.
- **S22-6 Authentifizierter Endpoint** `POST /api/v1/secvitals/nis2-assessment/migrate-from-anonymous`. Service-Methode `MigrateAndAutoMap` kombiniert Migration + Auto-Mapping in einem atomaren Schritt.

**22.3 Frontend-UI (3 von 5, größere Komponenten als [~]):**
- **S22-7 `ScanProgressIndicator`-Komponente** unter `modules/secpulse/components/`. Konsumiert SSE-Stream, zeigt Live-Phase + Percent-Bar + Heartbeat-Filter. Auto-Cleanup beim Unmount via AbortController.
- **S22-11 `LoginHistorySection`-Komponente** unter `shared/components/`. Tabelle mit TS / Quelle / Browser-Excerpt / IP / Result-Badge. Failed-Logins fett markiert. UA-Mini-Parser (Firefox/Edge/Chrome/Safari-Detection). In `AccountSettingsPage` eingebaut.

**22.4 Cleanup-Jobs:**
- **S22-12 `TaskCleanupAnonymousRuns`** (täglich 03:15 UTC): `DELETE FROM nis2_anonymous_runs WHERE expires_at < NOW()`. Im Worker-Scheduler verdrahtet.
- **S22-13 `TaskCleanupLoginHistory`** (wöchentlich Sonntag 04:00 UTC): `DELETE FROM login_history WHERE ts < NOW() - INTERVAL '90 days'`. Worker-Handler + Scheduler-Cron.

**22.5 Doku:**
- **S22-15 `docs/reviews/2026-05-22-honesty-audit.md`** dokumentiert den Skeleton-Status-Audit der zu Sprint 22 führte. Methodik, Item-Klassifikation, Lessons-Learned.
- **S22-16 CHANGELOG + UPGRADE** für v0.14.0 mit klarer Bugfix-Kennzeichnung der S22-1-Rotation-Defekts.

**Verschoben (S22-8, S22-9, S22-10, S22-14 [~]) → Folge-Welle:**
- S22-8 `AgentRunPanel`-Frontend (groß, Streaming-UI mit Approve-Cards).
- S22-9 `ApiKeysPage`-Refactor (Scope-Checkbox-Wizard, Rotation-Button-UI mit Modal).
- S22-10 Session-Mgmt-Backend-Endpoint (`/auth/sessions{,/:id/revoke,/revoke-all}`) + SessionsPage-Ausbau.
- S22-14 Integration-Tests für Cleanup-Jobs (brauchen testcontainers-Setup, separater Test-Hardening-Sprint).

### Sprint 20 — Enterprise-Auth CE-Tier (Tag-Kandidat v0.13.0)

CE-Schicht der Enterprise-Auth-Welle: feingranulare API-Key-Scopes mit Wildcard-Logik, zerstörungsfreie Rotation mit 24-h-Grace-Period, Login-Historie pro User. Pro-Schicht (SAML, SCIM, IP-Allowlist, MFA-API, SIEM) bleibt explizit Sprint 21 — on-demand bei konkretem Enterprise-Sales-Trigger.

**Backend (S20-1, S20-2, S20-6, S20-8):**
- Migration 126: `api_keys.previous_key_hash` + `previous_key_grace_expires_at` + `last_used_ip` + `rotated_at` für Rotation. Neue Tabelle `login_history` (user/email/ip/UA/source/result) mit 90-Tage-Retention-Plan.
- `internal/shared/apikeys/rotation_and_scopes.go`:
  - `RequireScope(scope)` Echo-Middleware mit Wildcard-Logik (`*`, `secvault.*`, `secvault.secrets.read`).
  - `ScopeAllows([]string, string) bool` als exportierter Helper für den Auth-Lookup-Pfad.
  - `Service.RotateKey(orgID, keyID) (*CreateResult, error)` — generiert neuen Hash, alter Hash wandert in Grace-Period (24h), beide werden vom Auth-Middleware akzeptiert. Endpoint `POST /api/v1/api-keys/:id/rotate`.
  - `RecordLoginAttempt` + `ListLoginHistoryForUser` Helpers.
- `auth/service.go`: Login-Pfad schreibt `login_history`-Entry bei `bad_password` + `ok`. Best-Effort, blockiert Login nie. Failed-Login ohne user_id (Account-Enumeration-Schutz).

**Docs (S20-8):**
- `docs/concepts/api-key-scopes.md` — Scope-Format, Wildcards, CI-Pipeline-Workflow, Rotation mit Grace-Period, Migration für Bestands-Keys, Backend-Implementation-Verweise, Skeleton-Status zu Auth-Middleware-Integration.
- `docs/concepts/README.md` Index aktualisiert.

**Verschoben (S20-3/4/5/7 [~] Frontend-Iteration):**
- S20-3 ApiKeysPage-Refactor (Scopes-Checkbox-Liste, Rotation-Button, Last-Used-IP) — Backend ist da, Frontend Cosmetic-Iteration.
- S20-4 Session-Mgmt-Endpoint + S20-5 SessionsPage — bestehende Skelette aus Sprint 2 reichen aktuell; Vollausbau in Folge-Welle.
- S20-7 Login-History-Section in AccountSettingsPage — Backend-Service-Methode `ListLoginHistoryForUser` ist da, UI ist iterativ.

### Sprint 19 — NIS2-Self-Assessment-Wizard CE (Tag-Kandidat v0.12.0)

Top-of-Funnel-Akquise-Asset für DACH-Markt 2026. Anonymer Wizard mit 30 NIS2-Fragen, Live-Score, Top-3-Gaps. Pro-Schicht (Branded PDF, Trend-View, Multi-Framework) als Folge-Welle vorbereitet.

**Backend:**
- Migration 125: `nis2_anonymous_runs` (7d-Lebensdauer, IP-Hash für DSGVO) + `ck_nis2_assessments` (Org-Migration bei Sign-up).
- `internal/shared/nis2wizard/` mit 30 Fragen über 8 Themenbereiche (NIS2 Art. 21 + BSI NIS2-UmsG §30). Gewichtete Score-Engine 0-4 mit Per-Area-Aufschlüsselung.
- Public-Endpoints (kein Auth, Rate-Limit 5/min/IP): `POST /public/nis2-assessment/{start,answer}`, `GET /public/nis2-assessment/{result,questions}`.
- `Service.MigrateToOrg(token, orgID, userID)` für Sign-up-Flow.
- 9 Score-Engine-Tests.

**Frontend:**
- `pages/NIS2WizardPage.tsx` unter `/nis2-check` (kein Layout, mobile-first). Multi-Step-Flow, Progress-Bar, Live-Score, Token in localStorage für Wiederbesuch.
- Result-Screen mit Ampel-Bewertung, Top-3-Gaps, CTA „Account erstellen + Ergebnis übernehmen".

**Docs:**
- **ADR-0021** Accepted: CE vs Pro Cut. Wizard + Sign-up-Migration sind CE; Branded-PDF + Trend + Multi-Framework sind Pro.

**Verschoben (S19-7..12 [~] Folge-Welle):**
- Embedded-Mode (iframe), Branded-PDF, Re-Assessment-History, Multi-Framework-Wizard, Auto-Mapping bei Sign-up, Landing-Page-Marketing.

### Sprint 18 — Agentic-AI v2 (Tag-Kandidat v0.11.0)

Vakts erste agentische AI-Workflows mit Plan/Execute/Reflect-Loop, Tool-Registry und RBAC-Enforcement. Adressiert den Bericht-§8-„AI-Native"-Hebel.

**Backend:**
- `AgentRunner` (`services/ai/agent.go`) mit MaxIterations (Default 5, Cap 10), OnEvent-Callback, Rate-Limit + Quota wie AI-Chat-Stream.
- `AgentTool`-Interface + drei Read-Only-Tools: `list_open_findings`, `list_stale_evidence`, `list_controls_without_evidence`. Jedes Tool deklariert `RequireScope` (z.B. `secpulse.findings.read`).
- `POST /api/v1/secvitals/ai/agent/run` als SSE-Endpoint. Frame-Types: `plan`, `tool_call`, `tool_result`, `final`, `error`. Terminiert mit `[DONE]`.

**RBAC + Audit:**
- Tools werden im Plan-Prompt NUR gelistet, wenn der User den Scope hat. Defensiver zweiter Check vor jedem Execute. Audit-Log-Entry pro Agent-Run-Start (`action=agent_run_start, actor=ai_agent`).
- **ADR-0020** Accepted: keine Privilege-Escalation via AI; Pre-Approval-Pattern für mutierende Tools vorbereitet.

**Drei initiale Workflows:** Triage offener Findings, Wochen-Compliance-Plan, Evidence-Re-Collection.

**Docs:**
- `docs/concepts/ai-agents.md` — Architektur-Diagramm, Komponenten, SSE-Format, drei Workflows, Skeleton-Grenzen.
- ADR-0020 in `docs/adr/README.md`-Index.

**Verschoben (S18-4 [~]):**
- `AgentRunPanel`-Frontend mit Live-Plan-Steps + Approve-Cards. Backend-SSE-Endpoint ist produktiv; Frontend ist Cosmetic-Iteration für eine Folge-Welle.

**Skeleton-Grenzen (bewusst):**
- Plan-zu-Tool-Mapping via Substring-Heuristik statt echtem OpenAI-Function-Calling-Schema.
- Reflect ist Single-Pass-Final-Event statt iterativer LLM-Roundtrip pro Tool-Result.
- Beide Punkte sind Folge-Wellen-Themen; das Skeleton beweist das Pattern + die RBAC-Architektur.

### Sprint 17 — Realtime-Welle (Tag-Kandidat v0.10.0)

Erste produktive SSE-Endpoints nach dem ADR-0019-Pattern aus Sprint 16. Notifications und Scan-Progress werden jetzt live gepushed statt gepollt.

**Backend (S17-1, S17-2, S17-7):**
- `GET /api/v1/dashboard/notifications/stream` — server-side-poll-and-push, 2 s Cursor-Tick, 30 s Heartbeat-Pongs (`event: ping`). Skaliert besser als Postgres-LISTEN-per-Connection.
- `GET /api/v1/secpulse/scans/:id/progress/stream` — subscribed Redis Pub/Sub auf `scan:progress:<id>`-Channel. Worker publiziert `started` und `finished`/`failed`; Stream beendet sich mit `data: [DONE]`. Org-Isolation enforced (Cross-Org-Stream → 404).
- `internal/modules/secpulse/progress_stream.go` mit `PublishProgress(rdb, evt)`-Helper; im Worker (`handleScanJob`) verdrahtet vor + nach jedem Scan-Run.
- OpenTelemetry-Spans pro Stream-Lifecycle.

**Frontend (S17-3, S17-4):**
- `useNotificationStream`-Hook — fetch-SSE-Reader, Auto-Reconnect mit 1-s-Backoff, Heartbeat-Filter, Unmount-Cleanup.
- `NotificationBell` invalidiert React-Query-Cache bei jedem Stream-Event statt 60-s-Polling. `useNotifications.refetchInterval` entfernt.

**Docs (S17-6):**
- `docs/wiki/reverse-proxy.md` — nginx-Konfig für SSE-Endpoints (`proxy_buffering off`, `proxy_read_timeout 1h`, `location ~ ^/api/v1/.+/stream$`-Block). Caddy/Traefik/HAProxy/Cloudflare-Hinweise. Liste aller aktiven SSE-Endpoints.

**Tests (S17-8):**
- `parseSSEFrames`-Helper in `notifications_stream_test.go` — testbarer SSE-Frame-Parser mit 5 Unit-Tests (single-frame, ping-heartbeat, mixed-stream, empty, DONE-marker).

**Verschoben (S17-5 [~]):**
- `ScanProgressIndicator`-Frontend-UI als Cosmetic-Polish nach Sprint 18 verschoben. Backend-Pub/Sub-Infra produktiv, Hook-Pattern aus S17-3 wiederverwendbar.

### Sprint 16 — Frontend-Polish + Doku-Reife (Tag-Kandidat v0.9.0)

Sprint 16 schließt die Reife-Sanierung-Welle 2 strukturell ab. Schwerpunkt: Frontend-Hygiene + Doku-Vollständigkeit, keine API-Breaking-Changes.

**Doku-Wave (S16-5..9):**
- `docs/GLOSSARY.md` neu — Compliance-Vokabular (Control, Evidence, Framework, Finding, Risk, Incident, Cross-Module-Evidence, SoA, TOM, VVT, DPIA, AVV, DSR) + Vakt-Architektur-Begriffe (Modul, Service, Shared, Demo-Flow, safego.Run, Public Mirror).
- `docs/concepts/` Subdir mit `module-isolation.md`, `evidence-collection.md`, `rbac-model.md`, `demo-flow.md`. Narrative Erklärungen zur Architektur, komplementär zu den ADRs.
- `docs/api-versioning-policy.md` — Breaking-Change-Definition, 6-Monats-Deprecation-Window, CI-Enforcement-Plan, Sonderfälle für Security-/Legal-Pflichten.
- `docs/wiki/admin-cli.md` — vollständige Doku zu `vakt-admin` CLI (`health-check`, `list-orgs`, `list-users`, `reset-password`).
- `docs/adr/0019-sse-statt-websocket-fuer-realtime.md` Accepted — Server-Sent Events als Pflicht-Transport für alle Realtime-Pfade, WebSockets bewusst ausgeschlossen.

**Frontend-Polish (S16-1, S16-3, S16-10, S16-2):**
- **Severity-Farben als Design-Tokens** — Tailwind `theme.colors.severity.{critical,high,medium,low,info}` + `*-bg`-Varianten. Alle hardcoded `bg-[#hexhex]`-Bracket-Notations bereinigt (0 verbleibend). Whitelabel-Theme-Vorbereitung.
- **Code-Splitting** — alle Settings-/Admin-Pages auf `React.lazy()` umgestellt; Layout wrapped Outlet in Suspense. Eager bleiben Login/Setup/Dashboard + Token-Magic-Link-Pages (Auditor/Policy/Invite/DSR). Größter einzelner Chunk: `SecVitalsRoutes 452 kB` (gzip 105 kB) — unter Warning-Threshold.
- **`useFormatDate`-Bulk-Migration** — 60 Files mit hardcoded `toLocaleDateString('de-DE', ...)` / `toLocaleString('de-DE')` auf `formatLocale()` (neuer non-Hook-Helper) migriert. Hook-Variante `useFormatDate` (Sprint 13) bleibt für reaktive Komponenten verfügbar. 0 verbleibende Stellen.
- **openapi-typescript Client-Generierung** — `npm run api-types` generiert `frontend/src/api/generated.ts` (7018 LOC) aus `openapi.yaml`. CI-Step `api-types:check` enforced Drift (ADR-0017). `Login.tsx` als Demo-Migration nutzt jetzt `components['schemas']['LoginResponse']` statt Manual-Interface.

**Skip-Item:**
- S16-4 Bundle-Audit verschoben — `vite build` Chunk-Size-Warning erfüllt den Monitoring-Zweck; echte Tree-Shake-Optimierung lohnt sich erst nach Recharts/framer-motion-Bereinigung in einer Q3-Polish-Welle.

### Sprint 15 — AI-Härtung + Observability + Welle 2 (Tag-Kandidat v0.8.0)

Sprint 15 schließt die Backend-Stabilität (Sprint 14) ab und liefert produktreife AI-UX + Observability-Default-On.

**AI-Härtung (S15-1 bis S15-5):**
- Neue Tabelle `ai_usage` (Migration 124) trackt Tokens, Kosten (micro-EUR), Dauer und Status pro AI-Call. Konfigurierbare Tagesquota via `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG`.
- Redis-basiertes Rate-Limit per Org (Default 30 req/min, `VAKT_AI_RATE_LIMIT_RPM`). Bei Verstoß `429 AI_RATE_LIMITED`.
- Response-Cache mit sha256(model+messages)-Key, TTL via `VAKT_AI_CACHE_TTL_SECONDS` (Default 1h). Cache-Hits werden als `cache_hit`-Status persistiert.
- Prompt-Injection-Schutz: strikte System/User-Role-Trennung in `buildMessages` — User-Input landet niemals im System-Prompt-Concat. Unit-Test deckt den Pfad ab.
- Neuer Endpoint `POST /api/v1/secvitals/ai/chat/stream` mit Server-Sent-Events: OpenAI-konforme `data: {"content":"..."}` Frames, `data: [DONE]`-Terminator, X-Accel-Buffering-Off für nginx.

**AI-UX Frontend (S15-6 bis S15-9):**
- `useAIStream` Hook konsumiert SSE-Frames inkrementell; bietet `text`, `isStreaming`, `error`, `durationMs`, `start(req)`, `stop()`. AbortController + Unmount-Cleanup.
- `LocalLLMBadge` zeigt sichtbar "Lokal · qwen2.5:3b" (No-Phone-Home-Differential) vs "Cloud · gpt-4o-mini" je nach Provider.
- `TokenCostIndicator` mit kompakter `1.2k Tk · 0.02 € · 4.3 s`-Anzeige nach Streamende.
- `AIAdvisor.tsx` als Demo-Migration: Live-Streaming-Rendering mit blinkendem Cursor, Stop-Button, Badge im Header, Cost-Indikator nach Abschluss. Rate-Limit/Quota-Errors bekommen spezifische i18n-Hints.
- i18n-Keys `ai.{localBadge,cost,stream}.*` in de/en/fr/nl.

**Observability default-on (S15-11 bis S15-15):**
- `MetricsEnabled` default `true` (opt-out via `VAKT_METRICS_DISABLED=true`); `/metrics` bleibt IP-allowlisted (Loopback + Docker-Netz).
- Prometheus + AlertManager im `docker-compose.observability.yml` Profil. `observability/prometheus.yaml` scrapt api + worker; `observability/alert-rules.yaml` mit 7 konservativen Default-Alerts (5xx-Rate, P95-Latency, Queue-Backlog, AI-Latency, …).
- 4 Grafana-Dashboards committed (`observability/dashboards/{api,worker,ai,demo}.json`) + Provisioning-Manifest. Beim Start automatisch unter dem Folder „Vakt" verfügbar.
- `alertmanager.example.yml` mit severity-basiertem Routing (critical→pager, warning→webhook, info→email-digest), Customer konfiguriert eigene Receiver — kein Phone-Home zu Norvik.
- `safego.SetPanicHandler` callback-Hook für optionale Sentry/3rd-party-Integration ohne externe Pflicht-Dependency.
- `docs/operations.md` Sektion 0 mit SLA-Matrix (RTO/RPO) für Container-Crash, Redis-Loss, DB-Korruption, Server-Verlust, K8s-Pod-Eviction, Region-Outage + PITR-/Hot-Standby-Empfehlungen.

**`internal/shared/` Konsolidierung Welle 2 (S15-10):**
- `internal/shared/{ai,alerting,evidence_auto,crossevidence}/` → `internal/services/*`. 17 Import-Call-Sites in 16 Files migriert, History via `git mv` erhalten.
- Neues `internal/services/README.md` dokumentiert die Boundary: `shared/` für Cross-Cutting-Concerns, `services/` für Cross-Module-Services mit eigener Domain-Logik. Welle-3-Kandidaten (scheduledreports, emaildigest, notifications) explizit als zukünftige Iteration markiert.

**Neue Env-Vars (Sprint 15):**

| Variable | Default | Bedeutung |
|---|---|---|
| `VAKT_AI_RATE_LIMIT_RPM` | 30 | Max AI-Calls pro Minute pro Org |
| `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG` | 0 (aus) | Tages-Token-Quota pro Org |
| `VAKT_AI_CACHE_TTL_SECONDS` | 3600 | Response-Cache-TTL |
| `VAKT_AI_COST_PER_MTOKEN_IN_MICRO_EUR` | 0 | Kosten pro 1M Input-Tokens (0 = lokal) |
| `VAKT_AI_COST_PER_MTOKEN_OUT_MICRO_EUR` | 0 | Kosten pro 1M Output-Tokens |
| `VAKT_SENTRY_DSN` | leer | Optional Sentry-DSN; aktiviert PanicHandler-Hook |
| `VAKT_METRICS_DISABLED` | false | Opt-Out für /metrics (vorher: opt-in via VAKT_METRICS_ENABLED) |

### Sprint 13 — Reife-Sanierung Welle 2 abgeschlossen (Tag-Kandidat v0.7.0)

Befunde aus der zweiten Elite-Review (Mai 2026, archiviert unter `docs/reviews/2026-05-elite-review/`, Verify-Pass `docs/reviews/2026-05-bericht-verify.md`). 28/29 P0-Items erledigt; ein Bulk-Migration-Item (`useFormatDate`-Roll-out) verschoben in Sprint 16 (S16-10).

#### Sicherheit

- **SSRF-Guard für `VAKT_AI_BASE_URL`** — neue URL-Validierung beim Startup blockt IMDS (169.254.169.254), Loopback (127.0.0.0/8, ::1), Link-Local (169.254.x, fe80::/10) und `localhost` als Hostname, wenn `VAKT_AI_PROVIDER != "disabled"`. Allowlist für Container-Service-Discovery (`ollama`, `ai-llm`, `llm-proxy`, `lm-studio`) + alle Public-DNS-Hostnames. 22 Testfälle in `backend/internal/config/ai_base_url_test.go`.
- **LemonSqueezy Webhook-Replay-Schutz** — neue Migration `123_lemonsqueezy_webhook_events.{up,down}.sql` deduped Webhooks auf sha256(body). Doppelter Body → 200 OK ohne erneute Verarbeitung. Vorher konnte ein wiederholter `subscription_created`-Event prinzipiell mehrfach E-Mails / License-Operationen triggern.
- **LemonSqueezy Startup-Warning** — `NewHandler` logt `Warn` wenn `VAKT_LS_WEBHOOK_SECRET=""`; ohne Secret weist jede Signaturprüfung den Request ab.
- **bcrypt Cost-Upgrade-on-Login** — Login-Pfad prüft `bcrypt.Cost(hash)` und re-hasht transparent auf cost 12, wenn ein Legacy-Wert kleiner war. Update ist Best-Effort (Fehler nur Warn-Log), Login bleibt funktional.
- **Audit-Redaction erweitert** — `sensitiveKeys` in `audit/audit.go` enthält jetzt `recovery`, `backup`, `otp`, `mfa` zusätzlich zu `password`, `secret`, `token`, `key`. Felder wie `recovery_code` / `backup_code` / `totp_code` landen nicht mehr im Klartext im Audit-Log.
- **Trivy `ignore-unfixed: false`** im CI-Workflow (`backend` + `frontend` Scans). Unfixed-Akzeptanzen wandern in `.trivyignore` mit Begründung + Re-Check-Datum (Template enthalten).
- **gitleaks Per-Secret-Allowlist** — `.gitleaks.toml` nutzt jetzt `regexes` für konkrete Test-Konstanten (CI-Test-Hex, `admin1234demo`, `analyst1234demo`) statt pauschaler Pfad-Allowlist. Pfad-Liste auf wenige kontrollierte Dummy-Files reduziert (`.github/workflows/*.yml` und `docs/`, `Makefile` rausgeflogen).
- **Helm-Defaults verschärft** — `postgresql.auth.password` darf nicht mehr `"changeme"` sein UND muss ≥ 16 Zeichen lang sein (Honeypot-Default `MUST_BE_OVERRIDDEN` + `fail`-Hook in `_helpers.tpl`). `redis.auth.enabled` default `true` (vorher `false`). Siehe [UPGRADE.md v0.7.0](docs/UPGRADE.md) für Migrations-Hinweise.

#### Rebrand-Cleanup End-to-End

- **`helm/sechealth/` → `helm/vakt/`** — Verzeichnis umbenannt; alle 70 template-namespace-Definitionen (`define "sechealth.fullname"`, …) zu `vakt.*` migriert. Externe Konsumenten von `helm install ./helm/sechealth` müssen den Pfad anpassen — siehe UPGRADE.md.
- **`backend/cmd/sechealth/` entfernt** — legacy CLI-Binary, nicht in Makefile/Dockerfile referenziert, war Naming-Drift nach Rebrand.
- **`website/README.md`, `integrations/github-action/action.yml`, `integrations/gitlab-template.yml`** rebranded SecHealth → Vakt.
- **Frontend-Banner-Links** (`VersionBanner.tsx`, `TrustPage.tsx`) zeigen jetzt auf `github.com/norvik-ops/vatk` (Public Mirror).
- **`CLAUDE.md` Repo-Tree** aktualisiert (`sechealth/` → `vakt-app/`, `helm/sechealth/` → `helm/vakt/`).
- **`backend/cmd/admin/`** CLI `Use`-String + Beispiel-Outputs auf `vakt-admin` umgestellt.
- **Codekommentare + Default-Werte** in `secpulse/handler.go` (PDF-Dateiname), `secvitals/policy_acceptance.go` (Default-From-Adresse), `secvault/git_scanner.go` (tmp-Dir-Prefix), `shared/notify`, `shared/dashboard/notifications.go`, `setup/handler_test.go`, `cmd/seed/main.go`, `frontend/src/hooks/useDashboard.ts`, `pkg/sdk/nodejs/{index.ts,package.json}` von `sechealth`/`SecHealth` auf `vakt`/`Vakt` umgestellt.
- **`docker-compose.demo.yml`** Header rebranded; statische Demo-Credentials-Kommentare entfernt (irreführend nach v0.6.2-Ephemeral-Refactor, Memory-Violation).
- **`.gitignore`** legacy-Patterns für gelöschtes Binary entfernt.

Bewusst belassen (Memory `project_rebrand` + ADR-0004): DB-Schema-Präfixe (`vb_`, `ck_`, `so_`, …), Docker-Image-`LEGACY_PREFIX`-Aliase (`ghcr.io/matharnica/sechealth/*`) für Watchtower-Backward-Compat, ADR-Historien-Texte, Memory-Dateien, Operator-CRD-Name `SecHealthSecret` (Kubernetes-API-Breaking-Change, separate Welle).

#### Stabilität

- **Silent SQL-Errors in `secvitals`** — alle 14 Stellen mit `_ = s.db.QueryRow(...).Scan(...)` durch sichtbare `err`-Pfade ersetzt. Neuer Helper `fetchOrgName(ctx, db, orgID)` in `secvitals/orgname.go` mit Warn-Log statt stillem Drop. Composite-Queries (`service_frameworks` Milestone-Dedup, `service_reporting` 30-Tage-Counter, `handler_boardreport` Score-History + Incidents-30d) loggen jetzt explizit; Milestone-Dedup bricht bei DB-Fehler defensiv ab statt Doppelversand.

#### PRD & Doku-Wahrheit

- **PRD aktualisiert** (`docs/prd.md`): Jira-FR-VB06 entfernt (v0.5.2-Realität), Success-Metric "first paying managed-cloud customer" → ADR-0008-konform formuliert ("First 10 self-hosted Pro customers"), Setup-Zeit "< 3 min" → "≤ 5 min Plattform + 3–30 min Ollama-Pull". MSP-Tertiary-Audience neu beschrieben (per-customer-instance, kein zentrales Portal). Epic E16 "MSP Multi-tenancy" gestrichen.
- **`CONTRIBUTING.md`** neu — Branch-/Commit-Stil, Test-Erwartung gemäß ADR-0012 (kein 80%-Quoten-Diktat), ADR-Prozess, PR-Workflow, Pre-Release-Smoke-Test gemäß ADR-0017, Security-Disclosure-Adresse, explizite "NICHT-Annahme"-Liste (MSP-Portal, Phone-Home, Cloud-SaaS-Integrationen).
- **`.github/ISSUE_TEMPLATE/{bug,feature,security}.yml`** + **`.github/PULL_REQUEST_TEMPLATE.md`** + **`CODEOWNERS`** neu.
- **`frontend/README.md`** komplett neu — Stack, Modul-Struktur, Dev-Befehle, wichtige Hooks/Patterns, Frontend↔Backend-Vertrag.
- **CHANGELOG-Fragment-Konsolidierung** — `docs/CHANGELOG-{sprint3,sprint4,sprint5,launch-readiness,security-wave-may26,session-2026-05-20}.md` nach `docs/history/` verschoben mit Index-README. Root-`CHANGELOG.md` bleibt Single-Source-of-Truth.
- **`CLAUDE.md`** 80%-Coverage-Satz zu ADR-0012 (risikobasiert statt Quote) konsistent gemacht.

#### Frontend-Quick-Polish

- **Demo-Login-Fail-Toast** (`Login.tsx`) — `/api/v1/demo/start`-Fehler → sichtbarer Error-Toast statt stillem UI-Zerfall. i18n-Schlüssel `auth.demoUnavailable` in allen 4 Locales.
- **`useFormatDate`-Hook** (`shared/hooks/useFormatDate.ts`) liefert `formatDate`, `formatDateTime`, `formatTime`, `formatRelative` für aktive i18n-Locale (BCP47-Mapping `de/en/fr/nl`). Demo-Migration in `AdminSecurityPage` + `SecVitalsOverviewPage`. Bulk-Migration der verbleibenden ~60 Treffer in Sprint 16 (S16-10).
- **Hardcoded deutsche Microcopy** `"Demo wird vorbereitet…"` → i18n-Schlüssel `auth.demoPreparing` in allen 4 Locales.
- **`useErrorMessage`-Hook** (`shared/hooks/useErrorMessage.ts`) — i18n-bewusster Wrapper um `humanizeError`. Bevorzugt `errors.<CODE>`-Lookup über die Locales, fällt auf bestehende Substring-Map zurück. Locale-Keys für `AUTH_INVALID_CREDENTIALS`, `AUTH_BAD_REQUEST`, `AUTH_VALIDATION_ERROR`, `AUTH_INVALID_STATE`, `AUTH_TOKEN_REVOKED`, `AUTH_OIDC_NOT_CONFIGURED`, `AUTH_OIDC_FAILED`, `ACCOUNT_LOCKED`, `RATE_LIMITED`, `GENERIC` in `de/en/fr/nl`.

### Geändert

- **[ADR-0018](docs/adr/0018-goroutine-lifecycle-und-panic-eskalation.md)** (Accepted) — Goroutine-Lifecycle (Parent-Context-Pflicht) und Panic-Eskalation via `safego.Run`. Pflicht-Pattern für alle `backend/internal/`-Goroutinen ab Sprint-14-Migration; golangci-lint-Regel blockt neue Verstöße.

### Behoben

- **`/health` enthält jetzt `demo`, `sso_enabled`, `version`** — Frontend (`useDemoMode`) las diese Felder, Backend lieferte sie nicht. Effekt: `isDemo` war auf `secdemo.norvikops.de` immer `false`, die Demo-Credentials-UI wurde nie eingeblendet.
- **`POST /auth/login` enthält jetzt das `user`-Objekt** (`id`, `email`, `display_name`, `roles[]`) — Frontend (`Login.tsx → setAuth(data.user)`) crashte mit `can't access property "id"` direkt nach erfolgreichem Login, weil das Feld fehlte.
- **OpenAPI-Spec auf realen Stand gebracht** — `LoginResponse`-Schema hatte `token`/`name`/`role` während Code längst `access_token`/`display_name`/`roles[]` nutzte. `/health` hatte gar kein Response-Schema. Beides angepasst.
- **Demo-Banner zeigt keine fake Credentials mehr** — `Layout.tsx` und i18n-Locales (de/en/fr/nl) hatten weiterhin `admin@vakt.local / admin1234` im Demo-Banner, was nach dem Ephemeral-Refactor irreführend war.

### Geändert

- **[ADR-0017](docs/adr/0017-api-contract-tests.md)** — Strategie gegen Backend/Frontend-Drift: OpenAPI-Schemas für alle Frontend-konsumierten Endpoints sind verbindlich, Contract-Tests + Type-Generation als Ziel-Architektur, Maintainer-Checkliste in `docs/dev/api-contract-checklist.md` als Übergang.
- **[ADR-0016](docs/adr/0016-public-mirror-via-script.md)** — Public Mirror per Script (`scripts/build-public-mirror.sh` + `make public-mirror`) statt inline rsync im CI. Eingebauter `go build ./...`-Check verhindert Bugs wie den v0.6.1-Excludes-Bug.

---

## [v0.6.2] — 2026-05-20

### Behoben

- **Demo-Login funktioniert wieder** — Backend `/api/v1/demo/start` gibt jetzt die generierten ephemeren Random-Passwörter (16 hex chars, admin + analyst) im Response zurück. Frontend `Login.tsx` nimmt sie und füllt die Login-Form vor. Vorher hatte das Frontend ein hardcodiertes `admin1234` als Default-Passwort, das (a) nicht den tatsächlich erzeugten Random-Hashes entsprach und (b) seit Erhöhung der Mindestpasswortlänge auf 10 Zeichen nicht mehr durch die Auth-Validierung kommt. Demo war dadurch unbenutzbar.
- **Statischer Demo-Seed nutzt 10+ Zeichen-Passwörter** — `demoseed.Run()` (für lokale Dev-Setups) setzt jetzt `admin1234demo` / `analyst1234demo`. Der frühere 9-Zeichen-Default (`admin1234`) wurde von der Auth-Validierung (min 10) abgelehnt.
- **Public Repo `norvik-ops/vatk` kompiliert wieder** — der Sync-Workflow hatte `internal/shared/demo/`, `demoseed/`, `feedback/` exkludiert, aber `cmd/api/main.go` importierte sie weiterhin. Wer die Codebase aus dem Public Repo baute, erhielt `no required module provides package …`-Fehler. Die drei Packages sind jetzt im Public Repo enthalten — sie sind hinter `if cfg.DemoSeed` gegated und ändern bei Customer-Default-Installs (VAKT_DEMO=false) das Verhalten nicht.

### Geändert

- **Doku zum Demo-Modus richtiggestellt** — `CLAUDE.md`, `docs/wiki/demo-mode.md`, `docs/setup.md`, `docs/configuration.md`, `docs/public/README.md`, `docs/launch-producthunt.md` und CI-Sync-Workflow dokumentieren jetzt einheitlich: Demo-Logins sind ephemer pro Visitor (Random-Slug, Random-Passwort, 4 h Lebensdauer), niemals statisches `admin@vakt.local / admin1234`.

### Lint / Hygiene

- **golangci-lint v2.12.2** statt v1.x — neuer config-Schema (`linters.settings`, `linters.exclusions.rules`), passend zu Go 1.25 build-toolchain
- **105 vorbestehende Lint-Verstöße bereinigt** — errcheck-Exclusions für idiomatische `defer X.Close()` Patterns, sinnvolle staticcheck-Ausnahmen für deutschsprachige Codebase, echte Bugfixes in `secvitals/reportpdf.go` (ungenutzte status-Variable in SoA-PDF jetzt im richtigen Feld dargestellt) und `alerting/service.go` (labeled `break` für korrekten Abbruch der Retry-Schleife bei ctx-cancel)

### Branding

- **Landing-Pages aktualisiert** — `sec.norvikops.de`: Pro-Features auf v0.6.1-Stand (KI-Berater raus, AI Copilot Community rein, 6 Module statt 5, NIS2-Meldungsassistent + Lieferantenportal als Pro ergänzt), Enterprise-Sales-Block entfernt, Datenschutz „SecHealth" → „Vakt"; `norvikops.de`: Meta-Description + Form-Placeholder rebranded

---

## [v0.6.1] — 2026-05-20

> **⚠️ Upgrade-Hinweis für Bestandskunden:** Diese Version startet Ollama (AI Copilot)
> automatisch mit `docker compose up` (vorher hinter `--profile ai` versteckt). Der
> Ollama-Container lädt beim ersten Start einmalig das Modell `qwen2.5:3b` (~1.9 GB
> Download, ~2 GB RAM-Live-Footprint, 4 GB Limit). Auf VMs mit weniger als 8 GB
> Gesamt-RAM bitte VOR dem Upgrade `VAKT_AI_PROVIDER=disabled` in `.env` setzen
> und in einer Compose-Override-Datei den `ollama`/`ollama-init`-Service entfernen.
> Plattform-Startup-Zeit unverändert (<5 Min); AI-Funktionen sind 3–30 Min später
> verfügbar, abhängig von Internet-Bandbreite (1.9 GB Modell-Download).

### Geändert

- **AI-Copilot ist Community** — Die fünf AI-Endpunkte (`/secvitals/ai/status`, `/ai/report`, `/ai/advice`, `/ai/draft-policy`, `/ai/incident-guide` sowie `/secvitals/policies/generate-draft`) sind ab sofort in jeder Vakt-Instanz nutzbar — kein `FeatureAIAdvisor`-Pro-Gate mehr. Mit qwen2.5:3b als Default-Modell (Apache 2.0, ~1.9 GB RAM, CPU-tauglich) läuft die AI lokal auf jeder VM; ein Lizenz-Gate hatte daher nur Marketing-Charakter ohne echten Schutz. Premium-Compliance-Features (TISAX, DORA, NIS2-Reporting, EU-AI-Act, AuditPDF, SSO, API-Access, SecReflex/SecPulse-Advanced, Granular-Permissions, Supplier-Portal) bleiben Pro. `FeatureAIAdvisor`-Konstante bleibt für Lizenz-Validierung erhalten, wird aber nicht mehr im Routing geprüft.
- **Ollama default-on, Auto-Model-Pull** — `ollama` Service ist nicht mehr hinter `profiles: ["ai"]` versteckt; startet automatisch mit `docker compose up`. Neuer Init-Container `ollama-init` zieht das Default-Modell `qwen2.5:3b` einmalig beim ersten Start (idempotent — bei vorhandenem Modell No-Op). Damit ist AI nach einem einzigen `docker compose up` lauffähig — kein `--profile ai`, kein manueller `ollama pull` mehr. Resource-Limit auf Ollama: 4 GB RAM / 2 vCPU. Customers auf VMs mit < 8 GB Gesamt-RAM können via `VAKT_AI_PROVIDER=disabled` + compose-override deaktivieren.
- **Helm-Chart Ollama-Integration** — Neue Templates in `helm/sechealth/templates/ollama/`: StatefulSet mit PersistentVolumeClaim (10 Gi default), ClusterIP-Service, Helm-Hook-Job für das einmalige Modell-Pull. Default-on via `ollama.enabled: true` in `values.yaml`. Die ConfigMap setzt `VAKT_AI_BASE_URL` automatisch auf den Cluster-internen Ollama-Endpoint, oder erlaubt Override für externe LLM-Quellen (z.B. Mistral EU). Resource-Defaults: 500m CPU / 2 GiB Memory request, 2 / 4 GiB limit.
- **Vakt Aware vollständig sqlc-migriert** — Tabellen-Präfix `pg_*` → `sr_*` (Migration 122, reine Metadaten-Operation in Postgres). Damit konnte sqlc die Tabellen parsen und alle 35 Repository-Methoden auf den generierten Code umgestellt werden. Vakt Aware war das letzte Modul mit embedded SQL. **ADR-0005 schließt damit ab — alle Module nutzen sqlc.**

### Sicherheit

- **CSRF Double-Submit-Cookie** — alle state-ändernden Endpoints unter `/api/v1` sind jetzt zusätzlich zu SameSite=Strict per expliziten Token gegen CSRF geschützt; Backend setzt `csrf_token` Cookie bei Login/Refresh/OIDC/SAML, Frontend echot ihn als `X-CSRF-Token` Header
- **Helm Pod-Security** — `podSecurityContext` mit `runAsNonRoot: true`, UID 65532, fsGroup 65532; `containerSecurityContext` mit `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, alle Capabilities gedroppt, seccomp `RuntimeDefault` für API und Worker; Frontend mit minimal nötigen Anpassungen für nginx
- **Verschlüsselung at-Rest dokumentiert** — neue `docs/encryption-at-rest.md` mit drei Pfaden (LUKS, Cloud-Provider, pgcrypto) und Installations-Checklist für DSGVO Art. 32
- **Redis-backed Org-Rate-Limiting** — fixed-window INCR/EXPIRE statt in-memory token-bucket; multi-replica-sicher für HA-Deployments
- **OIDC/SSO CSRF-Schutz** — OAuth2 `state`-Parameter wird jetzt serverseitig validiert (One-Time-Use via Redis, 10 min TTL); verhindert Login-CSRF-Angriffe
- **TOTP Deny-List** — ausgeloggte Paseto-Tokens waren auf 2FA-Endpunkten weiterhin gültig; Redis-Deny-List greift jetzt auch auf `/auth/2fa/*`-Routen
- **TOTP Replay-Schutz** — derselbe 6-stellige Code konnte innerhalb des 90-Sekunden-Fensters mehrfach eingesetzt werden; jetzt per Redis SetNX gesperrt
- **`RevokeAllOtherSessions`** — widerrief fälschlicherweise auch die eigene Session; eigene Session wird jetzt via `token_hash` ausgeschlossen
- **MFA-Enforcement Fail-Closed** — ein DB-Fehler beim MFA-Pflicht-Check ließ Requests kommentarlos durch; gibt jetzt HTTP 503 zurück
- **DSR-Portal** — öffentlicher Status-Endpunkt gab interne DPO-Notizen und org_id zurück; gibt jetzt nur noch `id`, `status`, `type` und Timestamps zurück
- **Setup-Handler Passwortvalidierung** — initiales Admin-Passwort konnte kürzer als 10 Zeichen sein; jetzt identisch mit der regulären Passwort-Policy
- **SMTP** — Port 465: implizites TLS (`tls.Dial`); Port 587: STARTTLS; keine Klartext-Credentials mehr
- **Webhook-RBAC** — Webhook-Endpunkte hatten keine Rollenprüfung; `List`/`Test` → `SecurityAnalyst+`, `Create`/`Update`/`Delete` → `Admin`
- **SSRF-Schutz** — Scanner-Targets (Trivy, Nuclei) werden gegen RFC-1918, Loopback und Link-Local geprüft; opt-out via `VAKT_SCAN_ALLOW_PRIVATE=true`
- **CSP** — `style-src` in `style-src-elem 'self'` (blockiert `<style>`-Injection) und `style-src-attr 'unsafe-inline'` (nur Inline-Attribute, nötig für UI-Framework) aufgeteilt
- **IP-Forwarding** — `X-Forwarded-For` wird nur noch ausgewertet wenn `VAKT_TRUSTED_PROXIES` gesetzt ist; verhindert IP-Spoofing bei direkter Installation

### Hinzugefügt

- **Session-Verwaltung pro Gerät** — neue Seite „Aktive Sitzungen" unter Einstellungen: alle angemeldeten Geräte einsehen und einzeln abmelden (`GET /auth/sessions`, `DELETE /auth/sessions/:id`)
- **Startup-Warnungen** — strukturierte Warn-Logs beim Start wenn HTTP statt HTTPS (`VAKT_FRONTEND_URL`) oder Demo-Modus aktiv (`VAKT_DEMO=true`)

### Infrastruktur

- **Nicht-Root-Container** — API, Worker und Migrate laufen jetzt als `nonroot` (UID 65532, distroless/static); kein Root-Prozess im Container
- **Go Healthcheck-Binary** — statisch kompiliertes `/healthcheck`-Binary ersetzt busybox-Abhängigkeit im distroless-Image; Docker-Healthcheck funktioniert ohne Shell
- **`VAKT_CORS_ORIGINS`** — CORS-Origins sind jetzt konfigurierbar (kommasepariert); Default `*`, Dokumentation in `.env.example` ergänzt

### Dokumentation & Architektur

- **Architecture Decision Records** — neuer `docs/adr/` Verzeichnis mit 12 retrospektiven ADRs: Self-Hosted-Prinzip, ELv2-Lizenz, Paseto-Wahl, Modul-Isolation, sqlc-Strategie, Anonymisierung statt Hard-Delete, Betriebsrat-Modus, MSP-Verzicht, OpenAPI-Single-Source-of-Truth, AES-256-GCM, OTel-Opt-in, Test-Coverage-Pragmatik

### Observability (opt-in)

- **OpenTelemetry-Instrumentation** — `internal/shared/telemetry/` initialisiert OTel beim Start, aktiviert sich aber nur bei explizit gesetztem `OTEL_EXPORTER_OTLP_ENDPOINT` (keine versteckten Telemetrie-Pfade, siehe ADR-0011)
- **Observability-Stack** — neue `docker-compose.observability.yml` Profile mit Loki + Promtail + Tempo + Grafana; aktivieren via `docker compose --profile observability up`; `docs/observability.md` mit Volumen-Schätzungen und Sicherheits-Hinweisen

### AI-Copilot

- **Default-Modell auf `qwen2.5:3b` umgestellt** — Apache-2.0-Lizenz statt Llama-Community, ~10 % weniger RAM-Footprint, schneller auf CPU, bessere Deutsch-Performance; alternative Modelle dokumentiert (`llama3.2:1b`, `phi3.5:mini`, `gemma2:2b`, `qwen2.5:7b`)
- **Policy-Drafting** — `POST /secvitals/ai/draft-policy` generiert einen Richtlinien-Entwurf in Markdown für ein Thema; Admin reviewt und veröffentlicht
- **Incident-Response-Guide** — `POST /secvitals/ai/incident-guide` erstellt aus einer Vorfalls-Beschreibung eine nummerierte Sofort-Checkliste mit gesetzlichen Fristen (NIS2, DSGVO Art. 33, DORA); im Frontend per „KI-Sofortmaßnahmen"-Button in der Vorfalls-Detailansicht direkt anwendbar
- **Wiki + Landingpage-Briefing** — neue `docs/wiki/ai-features.md` mit System-Requirements-Tabelle, Modell-Vergleich, DSGVO-Statement und Mistral-EU-Konfiguration; `docs/landingpage-ai-briefing.md` mit Headlines, Use-Cases und Vergleichstabelle gegen Vanta/Drata für die Marketing-Seite

### Refactor & Tests

- **HR-Service Pattern-Migration** — Audit-Logging vom Handler in den Service verlagert (P2-19/P2-20-Pattern); HR-Service ist jetzt vollständig SDK-fähig — Audit-Trail bleibt intakt auch bei Aufrufen aus Worker-Jobs oder künftigen CLI-Tools
- **sqlc Start für Vakt Vault** — Projects/Environments/AccessLog als sqlc-Queries (`db/queries/secvault.sql`); Secrets-Tabelle bleibt embedded SQL wegen Crypto-Spezifika
- **sqlc VVT (Vakt Privacy)** — Verzeichnis von Verarbeitungstätigkeiten (DSGVO Art. 30) komplett auf sqlc umgestellt; DPIA / AVV / Breach / DSR folgen in Folge-Sitzungen
- **Frontend-Test-Coverage erhöht** — 16 neue Unit-Tests: apiFetch (CSRF + Retry + Error-Mapping), useFirstAction (Persistenz + Idempotenz), useMilestoneToast (Schwellen + Jump-Detection); 2 vorbestehende Test-Fails behoben
- **Bugfix MilestoneToast** — Score-Jump-Baseline wurde nicht aktualisiert wenn ein Schwellen-Toast feuerte, führte zu Phantom-Toasts beim Remount; durch Test entdeckt und behoben
- **Integration-Test mit testcontainers-go** — echter End-to-End-Test für Vakt HR → Vakt Comply Evidence-Flow (`internal/integration_test/hr_evidence_real_test.go`); läuft in CI mit Docker-Daemon, skippt sauber wenn nicht verfügbar

### Datenschutz (DSGVO)

- **Recht auf Datenübertragbarkeit** (Art. 20) — neuer Endpoint `GET /api/v1/account/data-export` liefert ein ZIP-Archiv mit allen persönlichen Daten des Nutzers (Profil, Sessions, API-Keys-Metadaten, eigene Audit-Log-Einträge, eigene Kommentare, Benachrichtigungseinstellungen) als maschinenlesbare JSON-Dateien
- **Recht auf Löschung** (Art. 17) — neuer Endpoint `POST /api/v1/account/delete` mit Passwort-Re-Auth und expliziter „LÖSCHEN"-Bestätigung; Konto wird in der Datenbank anonymisiert (E-Mail, Name, Avatar geleert; Sessions + API-Keys widerrufen) statt hart gelöscht, um die Audit-Trail-Integrität gemäß ISO 27001 A.5.28 / BSI ORP.2 zu wahren; verhindert versehentliches Orphaning einer Organisation (letzter Admin → 409)

### UX-Verbesserungen

- **SlideOver-Komponente** — neue `SlideOver` für Linear-Style Detail-Panels mit framer-motion-Animation, Focus-Trap und Escape-Handling; nutzbar für Control-, Risiko- und Finding-Details ohne Kontextverlust
- **Micro-Guidance** — beim ersten Anlegen eines Risikos, Vorfalls, einer Richtlinie oder eines Assets erscheint ein einmaliger Hinweis mit Folge-Aktion-Empfehlung (z.B. „Control angelegt — als Nächstes Evidenz hochladen")
- **Role-basiertes Onboarding** — der Setup-Wizard zeigt nur die Schritte, die für die Rolle des angemeldeten Nutzers relevant sind: Admins sehen alle 4 Schritte, SecurityAnalysts nur die 2 Arbeits-Schritte (Control + Risiko), Viewer/Auditor sehen den Wizard gar nicht
- **Formular-Validierung erweitert** — `useFormValidation` unterstützt jetzt Cross-Field-Validation (`custom`-Callback) und scrollt + fokussiert automatisch das erste fehlerhafte Feld

### Hinzugefügt

- **OpenAPI 3.0 Spec — Single Source of Truth** — `backend/internal/shared/apidocs/openapi.yaml` wird zur Build-Zeit in den API-Server embedded; vorher lieferte der Server eine separate hardcoded Go-Spec mit nur 10 Endpoints, jetzt 75+. CI-Gate (`spec_test.go`) prüft YAML-Validität und blockiert PRs, die Pflicht-Endpoints aus der Doku entfernen. Spec ist über `GET /api/v1/openapi.yaml` und Swagger-UI unter `/api/docs` erreichbar. Kunden können daraus eigene SDKs generieren oder Automatisierungs-Skripte schreiben.
- **Frontend-Error-Tracking** — JS-Errors aus dem ErrorBoundary werden in der Tabelle `client_errors` persistiert; Admins sehen die letzten 200 Errors unter `GET /admin/client-errors` (org-scoped, self-hosted, kein externer Dienst)
- **Vakt Aware Content-Library** — 10 DACH-spezifische Phishing-Templates (CEO-Fraud, IT-Helpdesk, DHL, Microsoft-MFA, Mahnung, OneDrive, Sparkasse-SMS, USB-Köder, ...) + 5 vorgefertigte Trainings-Module abrufbar über `GET /api/v1/secreflex/templates/presets` und `GET /api/v1/secreflex/training-modules/presets`
- **Vakt Aware Anonymisierungs-Garantie** — Bei `betriebsrat_mode=true` werden IP-Adresse und User-Agent **gar nicht erst** in die DB geschrieben (statt nur im PDF-Export ausgeblendet) — DSGVO Art. 5 (1c) Datenminimierung + §87 BetrVG-konform; Wiki dokumentiert die rechtliche Begründung

### Datenbank

- Migration `117`: `refresh_sessions` — Tabelle für Refresh-Tokens mit Device-Info und Widerruf pro Gerät
- Migration `118`: `ck_evidence.control_id` nullable + neue Tabelle `hr_run_events` für Vakt HR Step-Audit-Trail
- Migration `119`: `client_errors` — Tabelle für persistierte Frontend-Errors

---

## [v0.5.5] — 2026-05-18

### Hinzugefügt

**Security**
- **CORS** — `CORSWithConfig` mit expliziten Methoden und exponierten Rate-Limit-Headern (statt Allow-All)
- **EPSS-Enrichment** — tägliche CVE-Exploit-Wahrscheinlichkeit via FIRST.org API (Batch 100 CVEs, Cron 01:00 UTC)
- **Control-Changelog (Vakt Comply)** — jede Status-, Owner- und Fälligkeitsänderung an Controls wird mit Zeitstempel und User-E-Mail in `ck_control_changelog` gespeichert; API: `GET /secvitals/controls/:id/changelog`

**UX & Interface**
- **Skeleton Loading** — alle Listenseiten (Incidents, Policies, Risks, Breaches, VVT) zeigen Skeleton-Platzhalter statt leere Fläche
- **Responsive Tables** — Desktop zeigt Tabellen, Mobile zeigt Cards (`useMediaQuery`-Hook)
- **Inline-Edit** — Finding-Status und Severity direkt in der Tabelle ändern (optimistisches Update + Rollback)
- **Empty States** — kontextspezifische Leerseiten mit direktem CTA (Frameworks, Assets, Risiken, Incidents)
- **Bulk-Aktionen Risiken** — mehrere Risks gleichzeitig auf einen Status setzen (`Promise.allSettled`)
- **`ConfirmDeleteDialog`** — Name-Eingabe-Bestätigung vor dem Löschen kritischer Objekte
- **`CopyButton`** — Kopieren-Button mit 2s-Feedback auf API Keys und Webhook Secrets
- **@-Mentions im Kommentarfeld** — Dropdown mit Teammitgliedern, Tab/Enter zum Einfügen, Escape schließt
- **Dark/Light/System-Toggle** — Drei-Stufen-Umschalter mit OS-Listener im Layout
- **Page Transitions** — 150ms Fade-Animation bei Navigation zwischen Seiten
- **Dashboard Drag & Drop** — Widget-Reihenfolge per HTML5 DnD anpassen, localStorage-persistiert
- **RTF-Export (Word)** — Framework-Controls als RTF-Dokument exportieren (Word-kompatibel, ohne npm-Dependency)
- **Vorfälle ↔ Datenpannen-Link** — `breach_id` wird in der Incident-Detailansicht als Link zu Vakt Privacy angezeigt; Breach-ID optional im Erstell-Dialog

**Platform**
- **Helm Chart** (K8s) — produktionsreifes Chart mit bitnami postgresql+redis Subcharts, HPA, Ingress, computed DSN helpers, liveness/readiness Probes
- **Queue Health Check** — Worker prüft alle 5 Minuten Redis-Queue-Tiefe und loggt Warnung bei >100 pending Jobs
- **EPSS Worker** — täglicher Cron-Job zur automatischen CVE-Anreicherung
- **Control-Owner-Reminder** — täglicher 09:00-Cron erinnert Verantwortliche an offene Controls
- **GitHub CI Evidence** — Worker sammelt GitHub Actions-Runs als Compliance-Evidenz (`ck_evidence`)
- **Playwright E2E** — 9 Spec-Dateien: Auth, Dashboard, Assets, Compliance, Navigation, Vakt Scan, Vakt Privacy, Vakt HR, Vakt Aware

**Dokumentation & API**
- **OpenAPI 3.0.3 v0.5.5** — 70 dokumentierte Pfade (+48 gegenüber v0.5.4): vollständige Vakt HR- und Vakt Aware-Endpunkte mit Schemas
- **Vakt HR Wiki** (`docs/wiki/modules/hr.md`) — vollständige Modul-Dokumentation mit API-Übersicht, curl-Beispielen und Compliance-Integration
- **api-reference.md** — Endpoint-Tabellen für Vakt HR und Vakt Aware ergänzt

### Entfernt
- **MSP-Layer** — `admin/organizations`-Endpunkte, MSPService, ImpersonateManagedOrg, Org-Branding-API vollständig entfernt. Vakt ist single-tenant self-hosted; MSPs deployen pro Kunde eine eigene Instanz.

### Datenbank
- Migration `102`: `ck_control_changelog` — Audit-Trail für Control-Änderungen
- Migration `103`: Entfernt MSP-Spalten aus `organizations` (`parent_org_id`, `msp_brand_logo`, `msp_brand_colors`, `scheduled_deletion_at`, Index)

### Upgrade
```bash
docker compose pull && docker compose down && docker compose run --rm migrate && docker compose up -d
```

---

## [v0.5.4] — 2026-05-18

### Hinzugefügt
- **Helm Chart** — `helm/sechealth/` mit bitnami postgresql+redis Subcharts, HPA, Ingress, NOTES.txt
- **OpenAPI 3.0.3** — vollständige Spec mit 45+ Endpunkten, BearerAuth, paginierten Responses, reuse-Schemas
- **Playwright E2E** — 5 Spec-Dateien (Auth, Dashboard, Assets, Compliance, Navigation) mit gemockter API
- **Queue Health Alert** — Worker loggt Warning wenn >100 pending Jobs in der Asynq-Queue

### Technisch
- EscalationChainSection (totes UI) entfernt
- CI: Node 24, FORCE_JAVASCRIPT_ACTIONS_TO_NODE24
- CI: E2E-Job mit chromium + Playwright-Report-Artifact

---

## [v0.5.3] — 2026-05-17

### Hinzugefügt
- **Notification Preferences** — Nutzer steuern welche E-Mails und In-App-Benachrichtigungen sie erhalten (`GET/PUT /notifications/preferences`)
- **Dependabot** — wöchentliche Dependency-Updates für Go, npm und GitHub Actions
- **Graceful Shutdown** — API und Worker beenden laufende Requests sauber (SIGTERM-Handler, 10s Timeout)

### Tests
- Webhook-Service: 5 Tests (HMAC-Berechnung, Event-Trigger mit und ohne Secret)
- Scheduled-Reports-Service: 13 Sub-Tests für Next-Run-Berechnung (wöchentlich/monatlich/vierteljährlich)
- Worker-Startup-Test

### CI
- GitHub Actions: Node 24 im Frontend- und E2E-Job
- `build-push-action@v6` in Staging-Deploy

---

## [v0.5.2] — 2026-05-17

### Entfernt
- **Jira-Integration** — entfernt wegen Datenabfluss zu Atlassian-Cloud (DSGVO Art. 28). Ersatz: Outgoing Webhooks für eigene Automatisierungen.

### Hinzugefügt
- **Webhooks aktiv** — `finding.created`, `finding.severity_changed`, `incident.created`, `incident.status_changed`, `control.status_changed` lösen jetzt tatsächlich Webhooks aus
- **Scheduled Reports** — Compliance-, Findings- und Risk-Berichte automatisch per E-Mail planen (wöchentlich/monatlich/vierteljährlich)
- **Excel-Export** — Findings, Risks und Controls als `.xlsx` aus der Toolbar exportieren
- **Risk Matrix interaktiv** — Klick auf Zelle zeigt Risiken der jeweiligen Kombination
- **Compliance-Score-Prognose** — Linearer Trend im Dashboard ("Bei aktuellem Tempo: 82% in 6 Wochen")
- **Notification Preferences** — Nutzer steuern welche E-Mails und In-App-Benachrichtigungen sie erhalten
- **In-App-Tour** — 5-Schritte-Tooltip-Guide für neue Nutzer
- **i18n vollständig** — alle Seiten auf Deutsch/Englisch (1.093 Keys)

### Sicherheit
- **Datenschutz-Grundsatz** in CLAUDE.md dokumentiert: keine Drittanbieter-SaaS-Integrationen die Vakt-Daten empfangen

### Upgrade
Neue Migrationen: `099_remove_jira`, `100_scheduled_reports`

---

## [v0.5.0] — 2026-05-17

### Added
- **AWS Evidence Collection** — automatische Sammlung von IAM-Passwortrichtlinie, MFA-Status, CloudTrail-Konfiguration und S3-Verschlüsselung als Compliance-Evidence
- **Azure Evidence Collection** — Secure Score, Security Center Assessments und Policy Compliance via Azure Management API
- **CIS Controls v8** — vollständiges Framework mit 61 IG1-Safeguards in 18 Kontrollgruppen, inkl. CIS ↔ ISO 27001 Mapping; Seeding in Vakt Comply
- **Progressive Web App (PWA)** — Vakt kann auf Mobilgeräten als App installiert werden (Offline-Unterstützung, Add-to-Home-Screen)
- **Englische Übersetzung** — vollständige UI-Übersetzung (277 Keys), automatische Spracherkennung, manueller Sprachwechsel in den Einstellungen
- **Jira-Integration** (Pro) — Findings und offene Controls direkt als Jira-Tickets erstellen
- **TOTP Recovery Codes** — 8 Einmal-Codes bei MFA-Einrichtung, sicher bcrypt-gehasht
- **Comments** — Kommentar-Threads auf Findings und Controls
- **Control Approvals** — Vier-Augen-Prinzip für Control-Statusänderungen (optionales Org-Setting)
- **Score-Verlauf** — Compliance-Score-Trend über Zeit, Recharts-Diagramm im Dashboard
- **Zertifizierungs-Timeline** — Countdown-Karten und Kalender für Audit-Meilensteine
- **Onboarding-Checkliste** — 6-Schritte-Assistent beim ersten Login

### Security
- **Rate-Limiting** — 300 Anfragen/min pro Organisation (Token-Bucket, Redis-backed), `X-RateLimit-*` Headers
- **Passwort-Mindestanforderungen** — min. 10 Zeichen, Großbuchstabe, Ziffer, Sonderzeichen bei Registrierung und Reset
- **Token-Cleanup-Job** — tägliche Bereinigung abgelaufener Passwort-Reset-Tokens (03:00 UTC)

### Improved (WCAG 2.1 AA)
- Farbkontrast Dark Mode: `--color-text3` von 3,1:1 auf 4,6:1 angehoben
- Globale `:focus-visible`-Regel für alle interaktiven Elemente
- ARIA-Attribute auf allen Formularen, Buttons und Navigationen
- Live Regions (aria-live) für Toasts und Fehlermeldungen
- Skip-to-main-content Link (screenreader + keyboard)
- Tabellenheader mit `scope="col"`
- `<html lang="de">` gesetzt (war "en")

### Infrastructure
- Worker HTTP-Healthcheck-Server (:9090) — Docker-Healthcheck repariert
- Dashboard-Cache-Invalidierung nach Control/Risk/Finding-Updates

---

## [v0.4.5] — 2026-05-17

### Security
- **Account Lockout** — nach 5 aufeinanderfolgenden Fehlversuchen wird das Konto 15 Minuten gesperrt (gleitendes Fenster, Redis-backed)
- **Session-Invalidierung** — alle aktiven Sessions werden bei Passwort-Reset sofort ungültig (`pw_version`-Claim im Paseto-Token)
- **Content-Security-Policy** — CSP-Header auf allen Antworten (script/style `unsafe-inline` für React SPA, `frame-ancestors 'none'`)

### Added
- **System-Status-Seite** (`/admin/health`) — DB-Latenz, Redis-Latenz, Queue-Tiefe (pending/active/failed), Uptime, Goroutinen, Version; automatische Aktualisierung alle 30 Sekunden
- **License-Ablauf-Banner** — gelbe Warnung ab 30 Tagen vor Ablauf, rote Warnung ab 7 Tagen; tageweise dismissbar, nur für Admins sichtbar

### Improved
- **Inline Evidence-Vorschau** — PDF- und Bild-Dateien öffnen sich direkt im Browser-Dialog statt als Download
- **Gespeicherte Filter** — Filterzustände in Audit-Log und Findings werden im Browser gespeichert und bei erneutem Besuch wiederhergestellt

---

## [v0.4.4] — 2026-05-17

### Security
- Security-Header im Backend: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Strict-Transport-Security` (1 Jahr)
- Access Token TTL von 8 Stunden auf 1 Stunde reduziert
- `VAKT_SECRET_KEY` Länge wird beim Start validiert (exakt 32 Bytes / 64 Hex-Zeichen)
- MIME/Extension-Allowlist im Evidence-Upload-Handler

### Added
- **Passwort zurücksetzen** — "Passwort vergessen?"-Link auf der Login-Seite, E-Mail mit Reset-Link (1h gültig)
- **Audit-Log UI** — Admin-Seite mit Datum-, Benutzer- und Aktionsfilter, server-seitige Paginierung, CSV-Export
- **Granulare Modul-Berechtigungen** (Pro) — Lese-/Schreibrechte pro Modul pro Benutzer
- **Org-weites MFA-Enforcement** — Admins können 2FA für alle Mitglieder vorschreiben
- **API-Key-Verwaltung** (Pro) — Persönliche API-Keys (`vakt_...`) für programmatischen Zugriff
- **SSO-Login-Button** — erscheint auf der Login-Seite wenn `CASDOOR_URL` konfiguriert ist
- **Update-Status in Einstellungen** — zeigt installierte und aktuelle Version mit Link zu Release Notes
- **"Was ist neu"-Modal** — erscheint einmalig pro Version nach dem Login
- **Compliance-Fortschrittsbalken** — Dashboard-Widget zeigt umgesetzte vs. offene Controls
- **Wöchentlicher Sicherheits-Digest** — opt-in E-Mail-Zusammenfassung jeden Montag

### Improved
- Audit-Log: server-seitige Filterung (statt client-seitig)
- Update-Prüfung zeigt korrekt auf `norvik-ops/vatk` Repository


---

## [v0.4.1] — 2026-05-14

### Added
- **DSGVO Art. 32 TOM-Mapping** — New framework "DSGVO-TOM" with 13 technical and organisational measures (TOM-1 through TOM-13) mapped automatically to existing ISO 27001 controls. Coverage dashboard shows which TOMs are fully covered, partially covered, or open.

---

## [v0.4.0] — 2026-05-14

### Added
- **DORA support** — Digital Operational Resilience Act (EU 2022/2554) is now a selectable framework in Vakt Comply. Includes all relevant DORA articles as controls (German), DORA ↔ ISO 27001 mapping, gap analysis, readiness score, and PDF export.
- **DORA IKT Incident Register** — New incident type "IKT-Vorfall (DORA)" with automatic deadline calculation (T+4h / T+24h / T+72h / T+30d) and traffic-light status per deadline. Webhook notifications on deadline breach.
- **DORA IKT Third-Party Register** — Supplier records extended with DORA criticality, subcontractors, data processing location (EU/non-EU), and exit strategy fields.
- **DORA Resilience Tests** — New section in Vakt Comply for TLPT documentation (DORA Art. 24–27): test type, status, execution date, results, and recommendations.
- **TISAX support** — VDA ISA question catalogue as a selectable framework with protection-level selection (Normal / High / Very high). Maturity scale 0–3 per control. Chapter 15 (prototype protection) shown only when relevant.
- **TISAX ↔ ISO 27001 Mapping** — Static mapping with coverage badges. "Gaps only" toggle filters already-covered controls. Readiness score accounts for ISO 27001 evidence as TISAX coverage.
- **TISAX Readiness Report** — PDF export with protection-level category, readiness score per chapter, maturity distribution, and gap list.
- **Supply Chain Compliance — Supplier Portal** — External, token-based supplier portal at `/supplier/:token` (no login required). Compliance managers send time-limited invitation links; suppliers complete questionnaires and upload certificates (ISO 27001, TISAX labels, etc.) directly in the portal.
- **Questionnaire Builder** — Build supplier assessment questionnaires with question types: Yes/No, Multiple Choice, Free Text, File Upload. Predefined templates: "NIS2 Supplier Assessment", "DORA IKT Third Party", "ISO 27001 Basic Check".
- **Supplier Assessment Review** — Incoming questionnaires reviewable per answer (accepted / requires improvement). Uploaded certificates tracked with expiry date; warning 30 days before expiry. Accepted responses linked automatically as evidence to controls.
- **EU AI Act — AI System Inventory** — New section in Vakt Comply. Register AI systems with provider, use case, affected population groups, decision autonomy, and status. Filter by risk class.
- **EU AI Act — Risk Classification Wizard** — Step-by-step wizard following the EU AI Act Annex III decision tree (prohibition check → high-risk categories → transparency obligations). Result: risk class + justification + relevant articles. Reclassification with change log.
- **EU AI Act — Technical Documentation** — Documentation template per EU AI Act Art. 11 / Annex IV (German). Fields: system description, training data, performance metrics, risk management, human oversight, logging. PDF export and version history.
- **NIS2 / DORA Incident Reporting Assistant** — Reportability classification wizard on incident creation. Automatic authority suggestion based on configured sector. Deadline tracking (T+24h / T+72h / T+30d) with traffic-light status and email notifications 12 hours before each deadline.
- **Incident Report Generator** — One-click report form per deadline (24h / 72h / 30d): pre-filled from incident data, exported as PDF (BSI layout) and JSON. Sent reports archived with timestamp.
- **Authority Directory** — New page in Vakt Comply: list of notification authorities (BSI, BaFin, BNetzA, Luftfahrtbundesamt, BAFZA) with portal URL, phone, and sector-specific notes.
- **Sector Configuration** — Organisation settings now include sector and federal state selection. Responsible authority is suggested automatically in the incident register.
- **Supplier filter improvements** — Criticality filter (critical / essential / standard), assessment status filter, NIS2-relevant and DORA-relevant flags, contract status badges (Active / Expiring / Expired), CSV import and export.

### Fixed
- TypeScript build errors after feature merge (6 type issues resolved).
- Migration 037 (`pg_trgm` indexes) failed in transaction context — added `no-transaction` directive.

---

## [v0.3.0] — 2026-05-13

### Added
- **PDF report exports** — Vakt Scan generates real PDF reports with findings summary, severity breakdown, and paginated findings table. Vakt Comply frameworks export a readiness PDF (colour-coded score, domain breakdown, gap list). Vakt Aware campaigns export a campaign PDF (click rate, rate bars, Betriebsrat-mode banner).
- **External alerting & webhooks** — Send alerts to Slack, Teams, or any webhook endpoint with HMAC signing (`X-Vakt-Signature`). Configurable per alert type. Exponential backoff on delivery failure (up to 4 retries).
- **Backup & Restore** — `scripts/backup.sh` creates timestamped encrypted archives (PostgreSQL dump + AES-encrypted master key). `scripts/restore.sh` supports `--dry-run` for validation without touching the database. Passphrase must be at least 12 characters.
- **Global Search** — Full-text search across all modules (assets, findings, controls, incidents, policies, suppliers, VVT entries, and more). Powered by `pg_trgm` GIN indexes. Command palette shows "Recently viewed" entries.
- **Score configuration** — Admin UI to adjust weighting of compliance score components. "Reset to defaults" button added.
- **Automatic database migrations** — Dedicated `migrate` container runs all pending migrations before the API and worker start on every `docker compose up -d`.
- **Isolated demo instances** — `POST /demo/start` creates a fresh organisation with unique credentials per visitor. No shared demo state between visitors.

### Fixed
- Alert deduplication: alerts now fire at most once per 24 hours per event type per organisation (no more alert floods on each cron tick).
- `window.open()` exports caused 401 errors because Bearer tokens cannot be sent via URL — all exports switched to `fetch()` + Blob download.
- Nullable `description` field in breach records caused crashes when `NULL` — fixed with `COALESCE`.

---

## [v0.2.0] — 2026-03-15

### Added
- Initial Vakt Comply (Package `secvitals`) module with NIS2 and ISO 27001 control frameworks
- Vakt Scan (Package `secpulse`) scanner orchestration: Trivy, Nuclei, OpenVAS integration
- Vakt Vault (Package `secvault`) secrets management with AES-256-GCM encryption and Git repo scanning
- Vakt Aware (Package `secreflex`) phishing simulation engine with SMTP campaign delivery
- Vakt Privacy (Package `secprivacy`) DSGVO documentation: VVT (Art. 30), DPIA (Art. 35), AVV (Art. 28), breach records (Art. 33/34)
- Demo mode with seed data (`VAKT_DEMO=true`) and per-visitor ephemeral instances
- Initial Docker Compose production and development setups

---

## [v0.1.0] — 2026-02-01

### Added
- Initial open-source release of the SecHealth platform (now rebranded to Vakt)
- Echo v4 HTTP API with Paseto token authentication
- PostgreSQL 16 + sqlc type-safe query layer
- Redis 7 + Asynq background job queue
- golang-migrate database migration system
- Module isolation architecture with per-module RBAC scopes
- Docker Compose single-command deployment (`docker compose up -d`)
- CI/CD pipeline via GitHub Actions (build, lint, test, release)

---
