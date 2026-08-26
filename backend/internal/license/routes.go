// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RegisterRoutes mounts the license endpoints under /api/v1/license.
// The caller passes the /api/v1 group, the active License, the auth middleware,
// an optional DB pool for key persistence, and an optional Redis client for
// cache invalidation on activation.
// Returns the Handler so the caller can configure auto-renewal.
func RegisterRoutes(api *echo.Group, inst *Instance, authMW echo.MiddlewareFunc, db *pgxpool.Pool, rdb ...*redis.Client) *Handler {
	h := NewHandlerForInstance(inst)
	if db != nil {
		h = h.WithDB(db)
	}
	if len(rdb) > 0 && rdb[0] != nil {
		h = h.WithRedis(rdb[0])
	}

	// Rate limiter for the activate endpoint: 5 requests per minute per IP.
	// Prevents brute-forcing license keys.
	activateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(5.0 / 60.0),
			Burst:     5,
			ExpiresIn: 5 * time.Minute,
		},
	))

	// GET /api/v1/license — returns current license status (requires auth).
	//
	// DBMiddleware runs AFTER authMW (Echo applies route middleware left to
	// right), so org_id is on the context when it looks up license_keys. Without
	// it this route sits on the bare `api` group and only ever saw the instance
	// license — an org that activated its own key was told "community" while its
	// Pro routes worked, and four frontend consumers hid the Pro UI on that answer
	// (R1-17-L05, R1-07-B02).
	licenseCtx := DBMiddleware(db, inst, rdb...)
	api.GET("/license", h.Get, authMW, licenseCtx)
	// POST /api/v1/license/activate — validate and persist a Pro key (requires auth
	// + CSRF + Admin role + rate limit). This route is mounted on the bare `api`
	// group (not `protected` in cmd/api/routes.go), so it does not inherit
	// protected's auth.CSRFMiddleware — apply the equivalent check explicitly here
	// (S132-S11/D24-2). requireCSRF() cannot simply call auth.CSRFMiddleware():
	// internal/auth already imports internal/license (via shared/platform/features),
	// so importing internal/auth back here would create an import cycle — same
	// constraint documented on requireAdminRole() below.
	api.POST("/license/activate", h.Activate, authMW, requireCSRF(), requireAdminRole(), activateLimiter)
	return h
}

// csrfCookieName/csrfHeaderName duplicate internal/auth's CSRFCookieName/
// CSRFHeaderName constants — see the import-cycle note on requireCSRF above
// (auth already imports license, so this package cannot import auth back).
//
// "Keep in sync" as a comment is not enforcement, and a silently drifted copy
// here means a write route that checks a header nobody sends. The pairing is
// therefore pinned by TestCSRFConstantsArePinned in internal/auth/csrf_test.go,
// which fails and names THIS file if either side changes.
const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

// requireCSRF enforces the double-submit-cookie CSRF pattern on
// POST /license/activate — a duplicate of auth.CSRFMiddleware's unsafe-method
// check (see the import-cycle note above). API-key callers (Bearer sk_/vakt_)
// are exempt, matching auth.CSRFMiddleware, since they are not browser-driven.
func requireCSRF() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer sk_") || strings.HasPrefix(authHeader, "Bearer vakt_") {
				return next(c)
			}

			cookie, err := c.Cookie(csrfCookieName)
			if err != nil || cookie.Value == "" {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "CSRF token missing",
					"code":  "CSRF_MISSING",
				})
			}
			header := c.Request().Header.Get(csrfHeaderName)
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

// requireAdminRole is a lightweight middleware that checks the "roles" context
// value set by the auth middleware. Only users with the built-in "Admin" role
// (capital A — matches the DB seed in migrations/001_core_schema.up.sql and
// the role names emitted by internal/auth/middleware.go) may activate a
// license key.
//
// The license package cannot import internal/auth because that would create
// an import cycle (auth imports license via flags.go). Duplicating one
// nine-line middleware is cheaper than the package refactor needed to share
// it.  Keep this in sync with auth.RequireRole.
//
// Audit finding F10: previously this checked for lowercase "admin", which
// nothing in the codebase emits — the Pro-tier activation endpoint was
// effectively dead.
func requireAdminRole() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			roles, _ := c.Get("roles").([]string)
			for _, r := range roles {
				if r == "Admin" {
					return next(c)
				}
			}
			return c.JSON(403, map[string]string{
				"error": "admin role required",
				"code":  "AUTH_INSUFFICIENT_ROLE",
			})
		}
	}
}
