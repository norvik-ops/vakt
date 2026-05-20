# Vakt — Security-Selbstbewertung (Stand: 2026-05-19)

## Zweck

Diese Selbstbewertung dokumentiert den Sicherheitsstand von Vakt für Kunden, die ein Sicherheits-Assessment vor der Einführung durchführen.

## Zuletzt überprüft: 2026-05-19 (internes Review, vollständige Code-Analyse)

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
| Security Headers | OK | HSTS (1 Jahr + preload), X-Frame-Options DENY, X-Content-Type-Options, Referrer-Policy |
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

## Bekannte Einschränkungen

| Punkt | Status |
|-------|--------|
| Externer Pentest | Noch nicht durchgeführt — geplant für v1.0. Internes Review Mai 2026 abgeschlossen: 17/17 Findings behoben, Gesamtbewertung 9.2/10. |
| SOC 2 | Nicht anwendbar (self-hosted) |
| Bug-Bounty-Programm | In Planung |

## Responsible Disclosure

security@vakt.io — GPG-Key verfügbar auf keys.openpgp.org
Policy: https://github.com/matharnica/vakt/blob/main/SECURITY.md

## Meldung von Sicherheitslücken

Bitte keine öffentlichen GitHub-Issues für Sicherheitslücken. Nutze den oben genannten Kontakt.
