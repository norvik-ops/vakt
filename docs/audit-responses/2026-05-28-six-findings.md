# Audit Response — six findings, 2026-05-28

Source: external review delivered with six labelled findings. Verified
against the codebase and the live demo (`secdemo.norvikops.de`) the same
day. Recorded here so the next audit doesn't re-flag what's already
addressed and so the FALSE POSITIVES come with concrete evidence.

## Summary

| # | Audit label | Severity | Status |
|---|-------------|----------|--------|
| 1 | Dynamic Login Form in SPA | HIGH | **Fixed** — SRI hashes on every bundle asset via vite-plugin-subresource-integrity. |
| 2 | CSP Allows Unsafe-Inline Scripts | HIGH | **Fixed** — `script-src 'self'` (no `'unsafe-inline'`). |
| 3 | CSRF Token Not Present in SPA | MEDIUM | **False positive** — see evidence below. |
| 4 | Missing API Security Headers | MEDIUM | **Fixed** — Cross-Origin-Resource-Policy added. |
| 5 | Public Demo Credentials via UI | MEDIUM | **Fixed in source** (F041, commit `1b55291`). Live until next image build. |
| 6 | No Rate Limiting Detected | LOW | **False positive** — see evidence below. |

---

## [1] Dynamic Login Form in SPA — HIGH — **FIXED**

The audit's concern: the login form is rendered entirely by client JS, so
a compromised bundle (XSS, supply-chain, etc.) can replace it with a
credential-harvesting form before the user types.

**Two-layer mitigation in this delivery:**

1. **Strict CSP** (#2) removes `script-src 'unsafe-inline'` — closes the
   XSS-injection path. An attacker who lands a stored payload cannot
   execute new JS in the page without compromising a bundle file.
2. **Subresource Integrity (SRI)** via `vite-plugin-subresource-integrity`
   adds a SHA-512 `integrity=` hash to every `<script src>` and
   `<link rel="stylesheet">` in the built `index.html`. If
   `dist/assets/index-*.js` is swapped at the CDN, proxy, or operator
   layer, the browser refuses to execute it.

Verification:

    $ npm run build
    ...
    verify-sri: OK — 2 script + 1 stylesheet tag(s) all carry integrity=

CI guard: `frontend/scripts/verify-sri.cjs` runs as the last step of
`npm run build`. Any future regression (plugin disabled, post-processing
strips the attribute) fails the build, not silently the deploy.

**Residual risk:** SRI does not cover lazy-loaded chunks fetched by the
runtime (route-split bundles like `SecVitalsRoutes-*.js`). Vite's module
preload still relies on the same-origin guarantee + CSP `script-src
'self'` for those. A future hardening pass could add the
`modulepreload` integrity support shipped in Chrome 109+.

## [2] CSP Allows Unsafe-Inline Scripts — HIGH — **FIXED**

Confirmed live before fix:

    content-security-policy: ... script-src 'self' 'unsafe-inline'; ...

The Vite production build emits zero inline `<script>` tags, zero `eval()`,
zero `new Function(...)` (`grep -l … dist/assets/*.js` → 0 / 0). So
`'unsafe-inline'` was a holdover with no functional need.

Fix in `frontend/security-headers.inc`:

    script-src 'self'
    style-src-elem 'self'
    style-src-attr 'unsafe-inline'  -- still required (React/Tailwind set element.style at runtime)
    object-src 'none'
    base-uri 'self'

The matching backend Echo `SecureWithConfig` block was already at the
strict shape; this just brings the Nginx layer into line.

## [3] CSRF Token Not Present in SPA — MEDIUM — **FALSE POSITIVE**

CSRF is implemented as **double-submit-cookie**: the backend sets a
`csrf_token` cookie on every authenticated login / refresh, and the SPA
reads that cookie via `document.cookie` and echoes it back in the
`X-CSRF-Token` header on every state-changing request. The backend's
`CSRFMiddleware` rejects mismatches.

Evidence (curl on the live demo, 2026-05-28):

    $ curl -X POST .../api/v1/auth/login -d '{"email":"...","password":"..."}'  -D-
    set-cookie: access_token=v4.local....; HttpOnly; Secure; SameSite=Strict
    set-cookie: csrf_token=ae514ff652611fc578a9c1f280ab5f0ac010f0a7b485872db40dfa071def590f; Path=/; Secure; SameSite=Strict

The audit tool likely tested without authenticating first — the cookie is
intentionally not set for anonymous sessions because there is nothing
state-changing for an anonymous user to protect. This is correct
behaviour, not a missing control.

Code:
- `backend/internal/auth/csrf.go:24,44` — token generation + cookie setter
- `backend/internal/auth/csrf.go:71-85` — middleware
- `frontend/src/api/client.ts:83-96,127-131` — `readCsrfToken()` + `X-CSRF-Token` header

## [4] Missing API Security Headers — MEDIUM — **FIXED**

Live before fix:

    strict-transport-security: max-age=31536000; includeSubDomains; preload
    x-frame-options: SAMEORIGIN
    x-content-type-options: nosniff
    referrer-policy: strict-origin-when-cross-origin
    permissions-policy: geolocation=(), camera=(), microphone=()
    cross-origin-opener-policy: same-origin

So nearly the full modern set was already in place. What was missing:
`Cross-Origin-Resource-Policy`. Now added in `cmd/api/main.go` (Echo
middleware) and `frontend/security-headers.inc` (Nginx layer) with value
`same-origin` — Vakt is self-hosted and has no third-party consumers.

`x-xss-protection: 0` is intentional (modern best practice; the legacy
XSS auditor in old browsers caused more problems than it solved). CSP
covers what XSS-Protection used to.

## [5] Public Demo Credentials via UI — MEDIUM — **FIXED IN SOURCE**

This is audit finding F041 from the earlier wave. Fix: commit `1b55291`
adds `POST /api/v1/demo/login` which runs the demo seed and the login
flow server-side, never returning the random password to the client.

Live status as of this writing: the deployed `:demo` image still contains
the old `/demo/start` bundle (built before the commit). The fix goes live
on the next image build & deploy.

To verify after deploy: open `https://secdemo.norvikops.de`, click the
Admin role button. The login should happen in one click with no password
field briefly populated, and `grep admin_password $bundle.js` should
return zero hits.

## [6] No Rate Limiting Detected — LOW — **FALSE POSITIVE**

Both layers are live and trigger easily. Evidence (2026-05-28):

Auth login lockout (`backend/internal/auth/service.go`, 10 failures →
15-min IP block):

    $ for i in 1..12; curl -X POST .../auth/login -d '{"email":"bad","password":"bad"}'
    401 401 401 429 429 429 429 429 429 429 429 429

(Three failures from the same IP → lockout kicks in on the fourth.)

Demo-start rate limiter (`cmd/api/main.go`, 10/min burst 10):

    $ for i in 1..12; curl -X POST .../api/v1/demo/start
    200 200 200 200 200 200 200 200 200 200 429 429

The audit tool likely didn't fire enough requests to trip the threshold,
or didn't see the 429 because of how it categorised responses. Both
limits are documented in `CLAUDE.md` ("Rate-Limits, die Tests/Tools
überraschen können").
