# Reverse-Proxy-Konfiguration (nginx)

Vakt nutzt Server-Sent Events (SSE) für Realtime-Updates (siehe [ADR-0019](../adr/0019-sse-statt-websocket-fuer-realtime.md)). Damit SSE-Streams sauber durch nginx kommen, müssen drei Punkte beachtet werden — sonst werden Frames gepuffert und der User sieht erst nach Sekunden/Minuten die Live-Updates.

## Minimal-Konfig für nginx

```nginx
upstream vakt_api {
    server api:8080;
    keepalive 16;
}

server {
    listen 443 ssl http2;
    server_name vakt.example.com;

    # ...TLS, security headers etc. ...

    # SSE-Streams: nginx-Puffer DEAKTIVIEREN für alle /stream-Endpoints.
    location ~ ^/api/v1/.+/stream$ {
        proxy_pass http://vakt_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE-Pflicht-Settings:
        proxy_buffering off;            # WICHTIG: keine Pufferung
        proxy_cache off;                # kein nginx-Cache
        proxy_read_timeout 1h;          # SSE-Streams leben lange
        chunked_transfer_encoding on;   # Chunked muss erlaubt sein
    }

    # Default-Block für alle anderen API-Endpoints.
    location /api/ {
        proxy_pass http://vakt_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # Standard-Buffering ist hier OK; SSE-Pfade matchen den Block oben.
    }

    # Frontend (statisches Build aus frontend/dist).
    location / {
        root /var/www/vakt/dist;
        try_files $uri $uri/ /index.html;
    }
}
```

## Warum diese Settings?

| Setting | Default | Pflicht für SSE | Begründung |
|---|---|---|---|
| `proxy_buffering` | on | **off** | nginx puffert sonst die Response, der Browser sieht erste Frames erst, wenn der Buffer voll ist. Mit `proxy_buffering off` werden Frames byte-by-byte durchgereicht. |
| `proxy_cache` | off (Default) | off | SSE darf nicht gecached werden. Standardmäßig OK, aber explizit klargestellt. |
| `proxy_read_timeout` | 60s | **1h** | Default reicht nicht. AI-Streaming, Scan-Progress oder Notifications können viele Minuten lebende Connections sein. |
| `chunked_transfer_encoding` | on | on (default) | SSE läuft über chunked-transfer-encoding. Default in nginx, aber bei Custom-Config nicht vergessen. |

Vakt setzt zusätzlich den Response-Header `X-Accel-Buffering: no` — das ist eine zweite Sicherung gegen nginx-Buffering. Bei korrektem `location`-Block oben ist es redundant, aber stört nicht.

## Test: läuft SSE durch?

Nach Deploy gegen den Live-Stack testen:

```bash
# Klassischer Endpoint: dashboard-Notifications. Erfordert valides Auth-Cookie.
curl -N -i -b "vakt_session=$YOUR_SESSION_TOKEN" \
     https://vakt.example.com/api/v1/dashboard/notifications/stream
```

Erwartetes Verhalten:
- HTTP-Status `200 OK` mit `Content-Type: text/event-stream`.
- Alle 30 Sekunden kommt `event: ping\ndata: {}\n\n`.
- Bei einer neuen Notification kommt `data: {"id":...,"title":...}\n\n` sofort.
- Connection bleibt offen, kein nginx-Timeout vor 1 h.

Wenn die Ping-Frames erst gebündelt nach Sekunden ankommen: `proxy_buffering off` fehlt im `location`-Block.

## Andere Reverse-Proxies

- **Caddy:** SSE läuft out-of-the-box, kein Config-Aufwand. Caddy puffert nicht standardmäßig.
- **Traefik:** läuft standardmäßig SSE-fähig, achten auf `RequestTimeout` und `IdleTimeout` (Default 0 = unlimited, was OK ist).
- **HAProxy:** `option http-server-close` + `timeout server 1h` setzen. Buffering ist per Default off.
- **Cloudflare:** funktioniert mit Free-Tier, hat aber **100-Sekunden-Limit für SSE-Connections** (HTTP/2 nur, HTTP/3 erlaubt länger). Workaround: Client-seitiges Auto-Reconnect (in `useNotificationStream` mit 1-s-Backoff implementiert).

## Liste aller SSE-Endpoints in Vakt

Stand: Sprint 17 (v0.10.0). Pfad-Pattern matcht `^/api/v1/.+/stream$` im nginx-Beispiel oben.

| Endpoint | Modul | Eingeführt in |
|---|---|---|
| `GET /api/v1/dashboard/notifications/stream` | Cross-Module | Sprint 17 S17-1 |
| `GET /api/v1/vaktscan/scans/:id/progress/stream` | Vakt Scan | Sprint 17 S17-2 |
| `POST /api/v1/vaktcomply/ai/chat/stream` | Vakt Comply (AI) | Sprint 15 S15-5 |

Weitere geplant in Sprint 18+ (Agent-Run-Stream).

## Referenzen

- [ADR-0019 — SSE statt WebSocket](../adr/0019-sse-statt-websocket-fuer-realtime.md)
- nginx-Doku: <https://nginx.org/en/docs/http/ngx_http_proxy_module.html>
- HTML5-Spec SSE: <https://html.spec.whatwg.org/multipage/server-sent-events.html>
