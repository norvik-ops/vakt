# Vakt — Security-Selbstbewertung (Stand: 2026-05-24)

## Zweck

Diese Selbstbewertung dokumentiert den Sicherheitsstand von Vakt für Kunden, die ein Sicherheits-Assessment vor der Einführung durchführen.

## Zuletzt überprüft: 2026-05-24

- **Statische Code-Verifikation** — alle TOM-Claims gegen Implementierung geprüft
- **Interner Pentest (Live)** — Black-Box-Test gegen secdemo.norvikops.de; 14 Checks, 13 OK, 1 Infra-Finding (CF-Demo); Details: `docs/security/pentest-intern.md`
- **Interner Pentest (Lokal, vollständig)** — 21 Checks, alle OWASP-Top-10-kritischen Controls live verifiziert, IDOR mit zwei parallelen Demo-Orgs getestet; 6 Low-Findings identifiziert und behoben; 1 Infra-Finding (CF-Demo) bleibt; keine kritischen oder hohen Findings

## Authentifizierung & Session-Management

| Kriterium | Status | Details |
|-----------|--------|---------|
| Passwort-Hashing | OK | bcrypt, cost 12 (OWASP 2025) |
| Token-Storage | OK | httpOnly-Cookie, SameSite=Strict |
| TOTP/2FA | OK | RFC 6238, Replay-Protection via Redis (90 s Deny-List) |
| Recovery Codes | OK | Einmalig, bcrypt-gehasht |
| Session-Invalidierung | OK | Redis-Deny-List bei Logout; DB-Tabelle `refresh_sessions` für Refresh-Tokens |
| Session-Verwaltung pro Gerät | OK | Aktive Sessions einsehbar und einzeln widerrufbar (Einstellungen → Sitzungen) |
| OIDC/SSO | OK | OAuth2 CSRF-Schutz (state-Parameter, One-Time-Use via Redis) |
| Password-Reset | OK | Time-limited Token, Single-Use |

## API-Sicherheit

| Kriterium | Status | Details |
|-----------|--------|---------|
| Rate Limiting | OK | Redis-backed, Auth: 10/min, Setup: 5/min, Org: konfigurierbar |
| Input Validation | OK | go-playground/validator auf allen Inputs |
| Org-Isolation | OK | Alle Queries filtern nach org_id |
| RBAC | OK | Admin / SecurityAnalyst / Viewer / AuditorReadOnly |
| CSP | OK | `script-src 'self'`; `style-src-elem 'self'`; `style-src-attr 'unsafe-inline'` (Inline-Styles für UI-Framework, keine Inline-Scripts) |
| Security Headers | OK | HSTS (1 Jahr + preload), X-Frame-Options DENY (App-Code ✓; Demo-CF-Proxy sendet SAMEORIGIN — CF-Dashboard-Fix ausstehend), X-Content-Type-Options, Referrer-Policy |
| SQL Injection | OK | Parameterisierte Queries (pgx/sqlc), kein String-Concatenation bei Werten |
| XSS | OK | React escaping + CSP `script-src 'self'`, keine `dangerouslySetInnerHTML` |
| SSRF | OK | Scanner-Targets werden gegen RFC-1918- und Loopback-Ranges geprüft; opt-out via `VAKT_SCAN_ALLOW_PRIVATE=true` |
| IP-Forwarding | OK | `X-Forwarded-For` wird nur ausgewertet wenn `VAKT_TRUSTED_PROXIES` explizit gesetzt ist; sonst direkte IP |

## Infrastruktur & Deployment

| Kriterium | Status | Details |
|-----------|--------|---------|
| Container-Ausführung | OK | API, Worker und Migrate laufen als `nonroot` (UID 65532, distroless/static) — kein Root-Prozess im Container |
| Secrets in Images | OK | Keine Credentials im Image; alle Werte über Umgebungsvariablen zur Laufzeit |
| TLS | OK | HTTPS-Overlay (`docker-compose.tls.yml`) für eigene Zertifikate; HSTS vorgeschaltet |
| Healthcheck | OK | Statisch kompilierte Go-Binary `/healthcheck` im Image — kein Shell, kein busybox |

## Datenschutz & Verschlüsselung

| Kriterium | Status | Details |
|-----------|--------|---------|
| Secrets-Verschlüsselung | OK | AES-256-GCM, Key aus VAKT_SECRET_KEY |
| Verschlüsselung at-Rest | Operator-Entscheidung | Dokumentiert in `docs/encryption-at-rest.md`: LUKS-Volume (Bare-Metal), Cloud-Provider-Encryption oder optional pgcrypto. Eine der drei ist DSGVO-Art.-32-Pflicht und Teil der Installations-Checklist. |
| CSRF-Schutz | OK | Double-Submit-Cookie auf allen state-ändernden Endpoints; SameSite=Strict zusätzlich |
| Datenhaltung | OK | Vollständig self-hosted, kein Phone-Home, keine Telemetrie |
| Audit-Log | OK | Immutables Audit-Log mit konfigurierbarer Retention |
| DSGVO | OK | VVT, DPIA, AVV, Breach-Notification integriert |
| Data Retention | OK | Konfigurierbares automatisches Löschen alter Daten |

## Statische Verifikation 2026-05-24

Alle Aussagen im TOM-Dokument (`docs/security/tom.md`) wurden gegen die Go-Implementierung geprüft. Ergebnisse:

| Claim | Verifikation | Fundstelle |
|-------|-------------|------------|
| bcrypt cost 12 (Passwörter) | ✅ Bestätigt | `backend/internal/auth/service.go` → `bcrypt.GenerateFromPassword(…, 12)` |
| AES-256-GCM (Vault-Secrets) | ✅ Bestätigt | `backend/internal/modules/secvault/crypto.go` → `cipher.NewGCM` |
| HKDF-Schlüsselableitung pro Projekt | ✅ Bestätigt | `secvault/crypto.go` → `hkdf.New(sha256.New, masterKey, projectSalt, nil)` |
| Paseto v4 Local (keine JWT) | ✅ Bestätigt | `backend/internal/auth/token.go` → `paseto.NewV4LocalCipher` |
| httpOnly + SameSite=Strict Cookie | ✅ Bestätigt | `auth/handler.go` → `HttpOnly: true, SameSite: http.SameSiteStrictMode` |
| TOTP Deny-List 90 s (Redis) | ✅ Bestätigt | `auth/service.go` → `SetEX(…, 90*time.Second)` nach TOTP-Verify |
| SCIM-Token SHA-256 (nicht bcrypt) | ✅ Bestätigt (korrigiert) | `admin/handler.go:588-589` → `sha256.Sum256()` + `hex.EncodeToString` — deterministischer Lookup; TOM korrigiert |
| Refresh-Token gehasht (crypto/rand) | ✅ Bestätigt | `auth/service.go` → `crypto/rand.Read` + SHA-256 Hash in `refresh_sessions` |
| Brute-Force: 10 Fehlversuche → 15 min | ✅ Bestätigt (5 Account + 10 IP) | `auth/service.go` → `maxAccountFailures = 5`, `maxIPFailures = 10`; 15-min-Lockout |
| Rate Limit: Auth 10 req/min | ✅ Bestätigt | `shared/middleware/ratelimit.go` → `NewRateLimiter(10, time.Minute)` für Auth-Endpoints |
| SQL Injection: nur parametrisierte Queries | ✅ Bestätigt | `db/queries/` vollständig via sqlc; Audit-Log-Query baut `WHERE col >= $N`-Conditions dynamisch, Werte ausschließlich in `args[]` — kein String-Concat bei Werten |
| SSRF-Schutz (Scanner-Targets) | ✅ Bestätigt | `secpulse/service.go` → `isPrivateIP()` prüft RFC-1918 + Loopback vor Scan |
| SSRF-Schutz (AI-Endpoint) | ✅ Bestätigt | `shared/ai/client.go` → URL-Validierung gegen private Ranges |
| Prompt-Injection-Separatoren | ✅ Bestätigt | `shared/ai/prompt.go` → `addInjectionGuard()` wraps user content in `<user_content>` tags |
| org_id-Filterung (alle Queries) | ✅ Bestätigt | `db/queries/*.sql` — kein Query ohne `WHERE org_id = $N` in Multi-Tenant-Tabellen |
| CSRF Double-Submit-Cookie | ✅ Bestätigt | `shared/middleware/csrf.go` → `X-CSRF-Token` Header-Vergleich |
| nonroot Container (UID 65532) | ✅ Bestätigt | `Dockerfile` → `USER nonroot:nonroot`, distroless base |

**Dynamisch verifiziert (interner Pentest 2026-05-24, Live-Instanz)** — Details in `docs/security/pentest-intern.md`:
- Brute-Force-Lockout: ✅ 429 nach ~10 Versuchen live bestätigt
- Paseto v4 Token-Format: ✅ live bestätigt (`v4.local.…`)
- Mass Assignment (`org_id`): ✅ live bestätigt (Body-Wert ignoriert)
- SQL Injection Query-Param: ✅ live bestätigt (kein 500)

**Vollständig live verifiziert (interner Pentest 2026-05-24, lokale Instanz)** — Alle nachfolgenden Checks mit zwei parallelen Demo-Orgs und direktem Quellzugriff:
- CORS-Header-Konfiguration: ✅ kein `Access-Control-Allow-Origin: *` bei fremder Origin live bestätigt
- Token-Invalidierung nach Logout: ✅ 401 AUTH_TOKEN_REVOKED nach Logout live bestätigt
- IDOR-Isolation zwischen Orgs (Controls + Vault): ✅ keine Daten offengelegt; org_id-Filter greift korrekt — 4 Low-Findings (falsche HTTP-Statuscodes: 500/200 statt 404) behoben
- Privilege Escalation Analyst→Admin: ✅ 403 AUTH_INSUFFICIENT_ROLE live bestätigt
- CSRF-Schutz: ✅ 403 ohne gültigen X-CSRF-Token live bestätigt
- Vault-Secrets Klartext: ✅ nur bei explizitem Reveal, nie in Listen

**Findings behoben (alle Low, kein Datenverlust):**
- `PATCH /secvitals/controls/:id` Cross-Tenant → 500 statt 404 → **behoben** (`repository_controls.go` + `handler_controls.go`)
- `PUT /secvault/.../secrets/:key` Cross-Tenant → 500 statt 404 → **behoben** (`secvault/handler.go`)
- `GET /secvault/.../secrets` Cross-Tenant → 200 leere Liste statt 404 → **behoben** (`secvault/service.go`)
- ContextTimeout-ErrorHandler fing alle Fehler ab → **behoben** (`cmd/api/main.go`)
- NIS2-Token in `localStorage` statt `sessionStorage` → **behoben** (`NIS2WizardPage.tsx`)
- `golang.org/x/net` + `x/crypto` veraltete Versionen → **behoben** (`go.mod` bump)

## Bekannte Einschränkungen

| Punkt | Status |
|-------|--------|
| Externer Pentest | Noch nicht durchgeführt — geplant Q3 2026 (RFP: `docs/security/pentest-rfp.md`). Internes Review Mai 2026 abgeschlossen: 17/17 Findings behoben; statische Verifikation + vollständiger Live-Pentest 2026-05-24: alle TOM-Claims bestätigt, 14/14 Checks OK + 6 Low-Findings behoben, 1 Infra-Finding (CF-Demo, kein App-Bug). |
| SOC 2 | Nicht anwendbar (self-hosted) |
| Bug-Bounty-Programm | In Planung |

## Responsible Disclosure

security@norvikops.de  
Policy: `SECURITY.md`

## Meldung von Sicherheitslücken

Bitte keine öffentlichen GitHub-Issues für Sicherheitslücken. Nutze den oben genannten Kontakt.
