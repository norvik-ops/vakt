// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/labstack/echo/v4"
)

// forceSecureCookies, when set, forces the Secure attribute on every session/
// CSRF cookie regardless of the request's TLS state or X-Forwarded-Proto header.
// Wired once at startup from VAKT_FORCE_SECURE_COOKIES (S87-5, F-07).
var forceSecureCookies atomic.Bool

// SetForceSecureCookies wires the VAKT_FORCE_SECURE_COOKIES flag into the
// cookie-Secure computation. Call once during startup, before serving traffic.
func SetForceSecureCookies(b bool) { forceSecureCookies.Store(b) }

// CookieSecure reports whether session/CSRF cookies should carry the Secure
// attribute for this request: true when the request arrived over TLS, when the
// terminating proxy signalled https via X-Forwarded-Proto, or when the operator
// forced it on via VAKT_FORCE_SECURE_COOKIES. Centralising the decision here
// keeps all cookie-issuing sites consistent (S87-5).
func CookieSecure(c echo.Context) bool {
	if forceSecureCookies.Load() {
		return true
	}
	return c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https"
}

// CSRFCookieName is the cookie that carries the double-submit CSRF token.
// It is intentionally NOT HttpOnly so the frontend can read its value and
// echo it back in the X-CSRF-Token header.
const CSRFCookieName = "csrf_token"

// CSRFHeaderName is the header the client must echo the CSRF token back in.
const CSRFHeaderName = "X-CSRF-Token"

// GenerateCSRFToken returns a 32-byte cryptographically random token as hex.
func GenerateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail on Linux. If it does, we cannot fall
		// back to a weaker source — return empty and the caller should fail.
		return ""
	}
	return hex.EncodeToString(b)
}

// SetCSRFCookie writes the CSRF token cookie on the response.
// Not HttpOnly (must be readable by frontend JS) but SameSite=Strict + Secure
// limit exposure to first-party same-origin contexts.
//
// Path is "/" — not "/api/v1" — because document.cookie path-matching (RFC 6265
// §5.4) only returns cookies whose path is a prefix of the current document URL.
// The SPA is served from "/", "/vaktcomply/...", etc.; a cookie with Path=/api/v1
// would be invisible to JS there, so the double-submit header could never be
// echoed and every state-changing request would 403 with "CSRF header missing".
func SetCSRFCookie(c echo.Context, token string) {
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep -- HttpOnly:false intentional (CSRF double-submit pattern); Secure via variable
		Name:     CSRFCookieName,
		Value:    token,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   3600,
	})
}

// ClearCSRFCookie expires the CSRF cookie (called on logout).
func ClearCSRFCookie(c echo.Context) {
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep -- HttpOnly:false intentional (CSRF double-submit pattern); Secure via variable
		Name:     CSRFCookieName,
		Value:    "",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

// CSRFMiddleware enforces the double-submit-cookie CSRF pattern on state-
// changing methods (POST/PUT/PATCH/DELETE).
//
// It is intentionally permissive in three cases:
//
//  1. Safe HTTP methods (GET, HEAD, OPTIONS) — no state change, no CSRF risk.
//  2. API key authentication (Authorization: Bearer sk_… or vakt_…) — API
//     keys are not browser-driven, so CSRF does not apply. The key itself
//     authenticates the caller.
//  3. Explicit exemption paths — e.g. webhook receivers, OAuth callbacks
//     that must accept POSTs without prior session establishment.
//
// All other state-changing requests must present both a csrf_token cookie
// AND a matching X-CSRF-Token request header (constant-time compare).
// CSRFMiddleware enforces double-submit CSRF on unsafe methods. exemptPaths are
// matched EXACTLY, not by prefix.
//
// S131-R-L05: the matcher used strings.HasPrefix, so an exemption for
// "/api/v1/webhooks/receive" also silently exempted "/api/v1/webhooks/receive/abc"
// and any future sibling — a latent prefix trap. Exact matching means an exemption
// can never widen its own scope; a route that genuinely needs exemption must be
// listed by its exact path. (Inbound webhooks that must bypass CSRF are mounted on
// public groups without this middleware — e.g. the HMAC-verified Personio webhook —
// so they need no exemption here at all.)
func CSRFMiddleware(exemptPaths ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				return next(c)
			}

			// API key auth bypasses CSRF (no browser, no cookie).
			authHeader := c.Request().Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer sk_") || strings.HasPrefix(authHeader, "Bearer vakt_") {
				return next(c)
			}

			path := c.Request().URL.Path
			for _, p := range exemptPaths {
				if path == p {
					return next(c)
				}
			}

			cookie, err := c.Cookie(CSRFCookieName)
			if err != nil || cookie.Value == "" {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "CSRF token missing",
					"code":  "CSRF_MISSING",
				})
			}
			header := c.Request().Header.Get(CSRFHeaderName)
			if header == "" {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "CSRF header missing",
					"code":  "CSRF_HEADER_MISSING",
				})
			}
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "CSRF token mismatch",
					"code":  "CSRF_MISMATCH",
				})
			}

			return next(c)
		}
	}
}
