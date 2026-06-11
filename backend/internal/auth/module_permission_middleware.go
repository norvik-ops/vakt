// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"net/http"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// modulePermDB is the minimal DB surface used by RequireModuleAccess.
// *pgxpool.Pool satisfies this interface; tests can inject a lightweight fake.
type modulePermDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// RequireModuleAccess returns an Echo middleware that enforces module-level
// access control based on the user_module_permissions table.
//
// Backward-compatibility rule: if NO permission rows exist for the user+org,
// access is GRANTED. This preserves behaviour for deployments where admins
// have not yet configured per-user permissions — all users retain access by
// default until an admin explicitly restricts them.
//
// If rows exist and none grant can_read access to `module`, the middleware
// returns 403 MODULE_ACCESS_DENIED.
//
// On DB errors the middleware fails closed: it returns 503 SERVICE_UNAVAILABLE
// rather than granting access to an unknown permission state.
//
// Admin role always bypasses the check entirely.
func RequireModuleAccess(db *pgxpool.Pool, module string) echo.MiddlewareFunc {
	return requireModuleAccess(db, module)
}

// requireModuleAccess is the testable implementation behind RequireModuleAccess.
// It accepts the modulePermDB interface so tests can inject a fake without a
// real Postgres connection.
func requireModuleAccess(db modulePermDB, module string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get("user_id").(string)
			orgID, _ := c.Get("org_id").(string)
			roles, _ := c.Get("roles").([]string)

			// API keys are automation credentials that use scope-based auth via
			// RequireScope; module permission table does not apply to them.
			if authMethod, _ := c.Get("auth_method").(string); authMethod == "api_key" {
				return next(c)
			}

			// Admin role bypasses all module-level restrictions.
			for _, r := range roles {
				if r == "admin" || r == "Admin" {
					return next(c)
				}
			}

			// If auth middleware hasn't populated context yet, pass through.
			// This should not happen in production wiring but prevents a lockout
			// if middleware order is misconfigured.
			if orgID == "" || userID == "" {
				return next(c)
			}

			rows, err := db.Query(c.Request().Context(),
				`SELECT module, can_read FROM user_module_permissions
				 WHERE org_id = $1::uuid AND user_id = $2::uuid`,
				orgID, userID,
			)
			if err != nil {
				log.Error().Err(err).
					Str("module", module).
					Str("user_id", userID).
					Str("org_id", orgID).
					Msg("module permission check: db error, failing closed")
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "permission check unavailable",
					"code":  "PERMISSION_CHECK_UNAVAILABLE",
				})
			}
			defer rows.Close()

			var hasAnyPermission bool
			var canAccess bool
			for rows.Next() {
				var mod string
				var canRead bool
				if scanErr := rows.Scan(&mod, &canRead); scanErr != nil {
					log.Warn().Err(scanErr).Str("module", module).Msg("module permission check: scan error, skipping row")
					continue
				}
				hasAnyPermission = true
				if mod == module && canRead {
					canAccess = true
				}
			}
			if rowErr := rows.Err(); rowErr != nil {
				log.Error().Err(rowErr).
					Str("module", module).
					Str("user_id", userID).
					Msg("module permission check: row iteration error, failing closed")
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "permission check unavailable",
					"code":  "PERMISSION_CHECK_UNAVAILABLE",
				})
			}

			// Backward-compat: no rows means permissions have not been configured
			// for this user yet — grant access so existing deployments are unaffected.
			if !hasAnyPermission {
				return next(c)
			}

			if !canAccess {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "access denied",
					"code":  "MODULE_ACCESS_DENIED",
				})
			}

			return next(c)
		}
	}
}
