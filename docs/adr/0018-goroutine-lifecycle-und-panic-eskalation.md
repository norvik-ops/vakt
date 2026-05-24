# ADR-0018: Goroutine-Lifecycle & Panic-Eskalation

**Status:** Accepted
**Datum:** 2026-05-21
**Entscheider:** Stefan (Maintainer)

## Kontext

Ein internes Code-Review (Mai 2026) identifizierte zwei strukturelle Stabilitätsrisiken im Backend:

1. **`context.Background()` in Goroutinen statt Parent-Context** — beim Shutdown laufen Webhook-Verarbeitung, Report-Generation und Evidence-Collection weiter. Bei `docker compose down` / Kubernetes-Rolling-Restart können Tasks in inkonsistente Zustände kommen (halb-geschriebene Audit-Einträge, doppelt abgesetzte Webhook-Calls).
2. **`recover()` ohne Eskalation** — Goroutinen schlucken Panics, schreiben sie höchstens in zerolog. Im Produktivbetrieb verschwinden so Bugs in Webhooks, AI-Calls, Cross-Module-Bridges — niemand sieht es, bis ein Kunde fragt warum eine Evidence fehlt.

Eine Verifikations-Runde korrigierte die Befund-Zahlen auf ≤16 kritische `context.Background()`-Stellen und 20 `recover()`-Stellen — das Pattern stimmt, das Volumen ist überschaubar.

Es gab bisher keine codifizierte Regel, wie Goroutinen Context erben und wie Panics nach außen kommuniziert werden sollen. Sprint 14 (Reife-Sanierung Welle 2) sanierte diese Stellen — ohne ADR wäre das ad-hoc und der nächste Pull Request hätte das alte Pattern wieder eingeschleppt.

## Entscheidung

**Wir codifizieren zwei Regeln:**

1. **Parent-Context-Pflicht:** Goroutinen in `backend/internal/` erben **immer** den Parent-`context.Context` (Request-Context, Worker-Job-Context, Server-Lifecycle-Context). `context.Background()` ist nur in `backend/cmd/*` während Startup-Wiring erlaubt — nirgendwo sonst. Ein golangci-lint `forbidigo`-Eintrag blockt neue Verstöße in `internal/`.
2. **`safego.Run(ctx, name, fn)`-Pflicht:** Jede Goroutine außerhalb von `cmd/*` und Test-Code läuft über den Helper `internal/shared/safego.Run`, der:
   - `defer recover()` mit Stack-Capture,
   - Eskalation an zerolog (`log.Error()`) **und** OpenTelemetry-Error-Span (`span.RecordError(err); span.SetStatus(codes.Error, ...)`),
   - Optional an Sentry, wenn `VAKT_SENTRY_DSN` gesetzt (folgt mit ADR-0019 falls Sentry-Compat-Layer separat ADR-würdig wird),
   - Den Goroutine-Namen als Label für Filterbarkeit in Dashboards.

Direkte `go func() { … }()`-Aufrufe in `internal/` sind verboten, ausgenommen Test-Code und das `safego`-Package selbst.

## Alternativen

- **Panic-Recovery-Middleware nur im HTTP-Layer**, ohne Goroutine-Helper — verworfen, weil 80% der `recover()`-Stellen in Hintergrund-Goroutinen liegen (Webhooks, Asynq-Jobs, Cross-Module-Bridges), nicht in HTTP-Handlern.
- **Eskalation nur an zerolog, keine OTel-Spans** — verworfen, weil zerolog im JSON-Log untergeht, während ein Error-Span im Tempo/Grafana-Dashboard sichtbar ist. ADR-0011 (OTel opt-in) bleibt unberührt: wenn OTel nicht aktiviert ist, ist `span.RecordError` ein No-Op.
- **Eigene `WrappedGoroutine`-Library statt `safego`-Package** — verworfen, weil 1 Datei + 1 Funktion ausreichen. YAGNI.
- **Lint-Regel ohne Helper** — verworfen, weil ein `forbidigo`-Block ohne Alternative das Team zum Bypass per `//nolint` zwingt.

## Konsequenzen

### Positive

- Graceful Shutdown wird testbar: laufende Hintergrund-Tasks beenden ihre Arbeit innerhalb von 30 s oder brechen kontrolliert ab, wenn der Parent-Context cancelt.
- Panics in Hintergrund-Tasks erscheinen in Grafana-Tempo + zerolog + (optional) Sentry — keine Silent-Failures mehr.
- Code-Review wird einfacher: `go func()` in einem Diff ist ein klares Stopp-Signal.
- Onboarding wird klarer: neue Engineers lernen ein Pattern statt 20 Variationen.

### Negative

- 20 bestehende `recover()`-Stellen + ≤16 `context.Background()`-Stellen müssen einmalig migriert werden (Sprint 14, geschätzt 2–3 Tage).
- Ein Helper-Package mehr in `internal/shared/` — gegenläufig zum Konsolidierungs-Ziel der gleichen Sprint-Welle. Akzeptiert, weil `safego` echtes Cross-Cutting-Concern ist (Teil der ≤15 erwarteten Survivors).
- golangci-lint `forbidigo`-Regel kann False-Positives haben, wenn Code aus `vendor/` oder generierter sqlc-Code grept wird — Allowlist nötig.

### Neutrale

- Asynq-Job-Handler erhalten den Job-Context automatisch über die Asynq-Library — Migration dort hauptsächlich ein Audit, kein Rewrite.
- HTTP-Request-Handler bekommen `c.Request().Context()` ohnehin — auch dort nur Audit.

## Referenzen

- Verwandte ADRs: ADR-0011 (OpenTelemetry opt-in), ADR-0004 (Modul-Isolation)
- Code (nach Implementierung): `backend/internal/shared/safego/safego.go`
