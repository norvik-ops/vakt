// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

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
	secure := c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https"
	c.SetCookie(&http.Cookie{
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
	secure := c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https"
	c.SetCookie(&http.Cookie{
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
func CSRFMiddleware(exemptPathPrefixes ...string) echo.MiddlewareFunc {
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
			for _, p := range exemptPathPrefixes {
				if strings.HasPrefix(path, p) {
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
