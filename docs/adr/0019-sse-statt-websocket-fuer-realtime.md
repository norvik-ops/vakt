# ADR-0019: SSE statt WebSocket für Realtime-Notifications

**Status:** Accepted
**Datum:** 2026-05-21
**Entscheider:** Stefan (Maintainer)

## Kontext

Ein internes Code-Review markierte das Fehlen von Realtime-Updates als „Premium-UX-Hebel": Notifications, Scan-Progress und Demo-Status werden heute mit Polling geholt, was bei vielen Browser-Tabs Server-Load erzeugt und für den User ein „2018"-Erlebnis liefert.

In Sprint 15 (S15-5) wurde ein Server-Sent-Events-Endpoint für AI-Streaming gebaut (`POST /ai/chat/stream`). Beim Wiring fielen die Fragen an: nutzen wir das gleiche Transport-Pattern (SSE) für die geplante Realtime-Welle in Q3 2026 — oder nehmen wir WebSockets, die der Bericht eher implizit voraussetzt?

Die Entscheidung fällt jetzt, weil:
- Sprint 16 enthält `useAIStream` als wiederverwendbaren Hook — wir wollen kein zweites paralleles Transport-Pattern bauen.
- ADR-Architektur-Konsistenz: ein Pattern, ein Tooling, ein Sentry-/Grafana-Trace.
- Customer-Deployments laufen oft hinter aggressiven Reverse-Proxies (nginx, Caddy, Cloudflare-Tunnel) — Transport-Tauglichkeit ist nicht trivial.

## Entscheidung

**Wir nutzen Server-Sent Events (SSE) für alle Realtime-Pfade in Vakt.** WebSockets werden NICHT eingeführt, solange kein konkretes bidirektionales Pattern nötig ist (Chat-Backchannel, Multi-Cursor-Edit). Für die anstehenden Realtime-Use-Cases — Notification-Stream, Scan-Progress, AI-Streaming, Live-Score-Updates — reicht uni-direktional Server-to-Client.

## Alternativen

- **WebSockets** — verworfen. Bidirektional ist für unsere Use-Cases nicht nötig (kein Vakt-Feature braucht persistenten Client-Push). WebSockets brauchen zusätzlich Heartbeat-Logik, eine Upgrade-Handshake-Phase, andere CSRF-Strategie und sind durch Reverse-Proxies häufig blockiert oder benötigen explizite Konfig (nginx `proxy_buffering off`, `proxy_http_version 1.1`, `Upgrade`-Header). SSE läuft über regulares HTTP/1.1 oder HTTP/2, wird von jedem Proxy ohne Sonder-Konfig transportiert (außer Buffering, das per `X-Accel-Buffering: no` adressiert ist).
- **Long-Polling** — verworfen. Halbe Lösung mit doppeltem Connection-Setup-Overhead pro Update.
- **HTTP/2 Push** — verworfen. Wird vom Browser nicht in JavaScript exposed, sondern nur für Asset-Push genutzt.
- **gRPC-Web Streaming** — verworfen. Großer Tooling-Footprint (Proto-Definitions, Codegen-Pipeline) für minimalen Mehrwert; das Frontend ist ohnehin REST/SSE-orientiert.

## Konsequenzen

### Positive

- Ein Hook genug fürs Frontend: `useAIStream` (Sprint 15 S15-6) wird Vorlage für `useNotificationStream`, `useScanProgressStream` etc. — gleiche Tests, gleiche Fehler-Behandlung, gleiche AbortController-Cleanup-Semantik.
- Backend bleibt vollständig stateless: jeder SSE-Endpoint ist eine Handler-Funktion, kein WebSocket-Pool, kein Heartbeat-Manager.
- Authentifizierung läuft über das existierende httpOnly-Cookie + CSRF-Token-Pattern — kein separates WebSocket-Auth-Schema.
- CSP-Hardening bleibt einfach: SSE braucht `connect-src 'self'`, das ist sowieso gesetzt. WebSockets bräuchten zusätzlich `ws://` / `wss://`-Quellen.
- nginx-Konfig: nur `X-Accel-Buffering: no` als Antwort-Header — keine weiteren Upstream-Settings.

### Negative

- Kein Server-zu-Client-Cancel über das gleiche Connection-Object. Wenn ein Client (z.B. via Stop-Button) den laufenden Stream stoppen will, muss er den HTTP-Request abbrechen (AbortController) und optional einen separaten `POST /ai/cancel/:request_id` schicken. Das ist im aktuellen `useAIStream` durch AbortController gelöst — explizit auf der Backend-Seite kommt der Cancel erst über Request-Cancellation rein.
- SSE ist Text-only und UTF-8 — Binary-Frames brauchen Base64-Wrapping. Bei Notification-Streams kein Problem (JSON-Text), bei zukünftigen Use-Cases mit Binary-Daten (z.B. PDF-Streams) muss man sich entscheiden.
- Browser-Limit: Chrome erlaubt nur 6 gleichzeitige SSE-Connections pro Domain (HTTP/1.1). Bei HTTP/2 fällt das weg — Vakt nutzt mit nginx als Reverse-Proxy ohnehin HTTP/2 zur Browser-Seite, also kein praktisches Limit. Aber dokumentieren.

### Neutrale

- Existierender `POST /ai/chat/stream` (Sprint 15) wird zum Template. Erste Folge-Endpoints (`GET /notifications/stream`, `GET /secpulse/scans/:id/progress/stream`) kommen in einer eigenen Realtime-Welle, voraussichtlich Sprint 17 oder Q3 2026.
- OTel-Tracing für SSE-Streams: jeder Stream-Open ist ein eigener Span; der Span endet bei Stream-Close. Tempo-Visualisierung bleibt damit klar zuordenbar.

## Referenzen

- Sprint 15: `internal/services/ai/{client.go,handler.go}` als Referenz-Implementierung (StreamGenerate + ChatStream).
- Frontend: `frontend/src/shared/hooks/useAIStream.ts` als Hook-Vorlage für künftige Stream-Hooks.
- nginx-Tipp: `proxy_buffering off;` muss NICHT global gesetzt sein — der `X-Accel-Buffering: no`-Antwort-Header reicht.
- Roadmap: `docs/roadmap-langfristig.md` — Realtime-Use-Cases als Q3-2026-Punkt.
- Bericht-§2.2: Polling-only fühlt sich „2018" an. Diese ADR ist die direkte Antwort auf den Befund.
