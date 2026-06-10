// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
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
func RegisterRoutes(api *echo.Group, lic *License, authMW echo.MiddlewareFunc, db *pgxpool.Pool, rdb ...*redis.Client) *Handler {
	h := NewHandler(lic)
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

	// GET /api/v1/license — returns current license status (requires auth)
	api.GET("/license", h.Get, authMW)
	// POST /api/v1/license/activate — validate and persist a Pro key (requires auth + Admin role + rate limit).
	api.POST("/license/activate", h.Activate, authMW, requireAdminRole(), activateLimiter)
	return h
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
