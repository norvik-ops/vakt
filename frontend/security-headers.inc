# Security headers for nginx-prod.conf.
#
# Included into each location block because nginx does NOT inherit
# server-block `add_header` directives into a location block that
# declares its own `add_header`. Without this include, locations like
# `= /index.html` (which set Cache-Control) would silently drop the
# Content-Security-Policy and the rest of the security headers from
# the parent server block.
#
# Keep this file in sync with caddy/Caddyfile in
# vakt-server repo — the live demo runs both layers.

add_header X-Frame-Options "DENY" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "0" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
# Modern Cross-Origin isolation. CORP=same-origin blocks other origins from
# loading our assets/JSON via <img>/<script>/fetch; COOP=same-origin gives
# us cross-origin isolation for any future SharedArrayBuffer usage.
add_header Cross-Origin-Resource-Policy "same-origin" always;
add_header Cross-Origin-Opener-Policy "same-origin" always;
# COEP require-corp: all cross-origin resources must opt in via CORP header.
# Safe for this app because all assets are loaded from 'self' (no external CDN).
add_header Cross-Origin-Embedder-Policy "require-corp" always;
# CSP:
#   script-src 'self' — Vite production build has no inline scripts and no
#     eval(); 'unsafe-inline' was a holdover that defeated XSS hardening.
#   style-src-elem 'self' — <style> + <link rel="stylesheet"> are bundled by Vite.
#   style-src-attr 'unsafe-inline' — React / Tailwind set element.style at
#     runtime (e.g. Recharts dimensions). Cannot eliminate without an SSR
#     rewrite. Inline style="…" cannot execute JS, so the residual XSS
#     surface is small.
#   upgrade-insecure-requests — instructs browser to upgrade any http:// sub-
#     resource requests to https://, preventing mixed content.
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src-elem 'self'; style-src-attr 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; upgrade-insecure-requests;" always;
