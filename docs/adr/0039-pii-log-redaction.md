# ADR-0039: PII-Logging-Redaktion — Emails als `***@domain` strukturiert ausgeben

**Status:** Akzeptiert
**Datum:** 2026-05-27
**Entscheider:** Stefan Moseler
**Sprint:** 57 (Marktreife-Welle 2)
**Related:** Audit-Befund F7

## Kontext

Vakt ist ein Compliance-Produkt mit eigenem GDPR/DSGVO-Modul (Vakt Privacy). Trotzdem loggte das Backend an 38+ Stellen die volle Email-Adresse von Endkunden auf `Info`-, `Warn`- und `Error`-Level — direkt verkettet in zerolog's strukturierte JSON-Ausgabe. Beispiele aus dem Audit:

- `polar/handler.go:232` — `log.Info().Str("email", email).Msg("polar: Pro license issued and sent")`
- `notifications/alerts.go:117` — `log.Info().Str("to", r.DPOEmail).Msg("CheckBreachDeadlines: alert sent")`
- `auth/handler.go:195` — `log.Debug().Str("email", body.Email).Msg("login failed")`

Konsequenz: jeder Operator mit Log-Zugriff sieht jede Customer-Email. Jeder externe SIEM-Sink (siem-forwarder ist Pro-Feature) sieht die Emails. Jede Log-Aggregation in einem Drittsystem (z.B. Loki, Sentry) speichert PII außerhalb des Compliance-Perimeters. Disqualifiziert Vakt für jede Org, die ein eigenes DSGVO-Konzept hat.

## Entscheidung

1. **Neuer Helper `internal/shared/logsafe`** mit `RedactEmail(string) string`, der `***@domain.tld` zurückgibt. Domain bleibt erhalten (operativer Wert: welcher Tenant, welcher Mail-Provider), Local-Part nicht (PII).
2. **Alle bestehenden Call-Sites umgestellt** auf:
   - Schema vorher: `Str("email", x)` / `Str("to", y)`
   - Schema nachher: `Str("email_redacted", logsafe.RedactEmail(x))` / `Str("to_redacted", logsafe.RedactEmail(y))`
3. **Neue Call-Sites** in `internal/auth/`, `internal/admin/`, `internal/modules/hr/`, `internal/modules/vaktcomply/`, `internal/services/scim/`, `internal/shared/emaildigest/`, `internal/shared/notifications/`, `internal/webhooks/polar/`, `internal/webhooks/lemonsqueezy/` müssen den Helper ebenfalls verwenden.
4. **Tests** in `internal/shared/logsafe/email_test.go` decken Edge-Cases (kein `@`, leeres String, multi-`@`, Unicode-Domains) ab und fixieren das Kern-Invariant: **der Local-Part darf in keinem Output auftauchen**.

### Was bewusst nicht geändert wurde

- **DB-Spalten** (`user_email` in `audit_log`, `to`-Felder in Email-Queue): hier ist die volle Email Nutzdaten und Teil des Compliance-Records. Audit-Log ist tamper-evident-Pflicht (ADR-0040, Sprint 57), aber **nicht** redacted — sonst geht der Audit-Wert verloren.
- **Outgoing-Email-Header** (`To:`-Headers in SMTP-Versand): klar, das ist der Zweck.
- **Strukturierte Response-Bodies** an authentifizierte Admin-User: kein Log, sondern Datenrückgabe.

Die Redaktion betrifft ausschließlich **strukturierte Log-Felder**, weil das der Pfad ist, der das Vakt-Compliance-Perimeter verlassen kann (Log-Aggregation, SIEM-Sinks, Log-Files auf Host-Filesystem).

## Konsequenzen

**Positiv:**
- Compliance-Self-Compliance-Gap geschlossen — Vakt loggt keine PII mehr im Operational-Pfad
- Debug-Wert bleibt erhalten: Operator sieht weiterhin Domain (welcher Kunde), Korrelations-IDs, Org-ID, Fehler-Stack
- Migration ist mechanisch sicher (Helper kapselt, keine String-Konkatenation an Call-Sites)

**Negativ / offen:**
- Debug-Sessions, in denen ein konkreter User-Login nachvollzogen werden muss, brauchen jetzt Korrelation über `user_id` oder `org_id` statt Email. Akzeptabel — wir loggen die anderen IDs bereits konsistent.
- Performance: `RedactEmail` ist String-Split, vernachlässigbar (<1µs / Call).
- Outgoing-Webhook-Handler (Polar/Lemonsqueezy) loggen jetzt nur noch Domain. Bei Support-Anfragen "der Polar-Webhook ist nicht angekommen für $email" hilft Polar-Side-Logging, nicht Vakt-Side.

## Verifikation

- `internal/shared/logsafe/email_test.go` — 12+ Sub-Cases inkl. Edge-Cases + Property-Test (Local-Part darf nicht auftauchen).
- Repository-Sweep:
  ```bash
  grep -rn 'Str("email"\|Str("user_email"\|Str("to", [a-zA-Z][a-zA-Z_.]*Email' \
    backend/internal/ | grep -v "_test.go"
  ```
  → 0 Treffer (Stand 2026-05-27, Commit Sprint 57).
- `go test ./...` grün auf allen Paketen.
- Smoke-Verifikation (post-deploy): in der laufenden Demo nach einem Login-Fehler die structured-log-Ausgabe inspizieren — `email_redacted: "***@demo-…"`, kein Local-Part-Leak.

## Abgelehnte Alternativen

- **SHA-256-Hash statt `***@domain`** — anonymer (Rainbow-Table-Schutz), aber operativ schmerzhaft (Domain-Filterung in Log-Queries unmöglich)
- **Komplett demoten auf `Debug`-Level** — würde Diagnose-Wert in Production verlieren; Operator muss in INFO/WARN sehen, dass etwas läuft
- **Per-CLAUDE.md-Rule, alle Devs müssen es selbst rausnehmen** — funktioniert in der Praxis nicht, nicht durchsetzbar im PR-Review
- **`zerolog` Custom-Hook der `email`-Felder global redacted** — fragil bei Feldumbenennung, weniger explizit, leakt bei Sub-Strings im `Msg`-Text
