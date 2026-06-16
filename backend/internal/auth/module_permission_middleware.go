// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// modulePermDB is the minimal DB surface used by RequireModuleAccess.
// *pgxpool.Pool satisfies this interface; tests can inject a lightweight fake.
type modulePermDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// modPermTTL is how long a resolved permission state is cached in Redis (S90-4).
const modPermTTL = 45 * time.Second

// permState is the cached, serialisable permission state for one (org,user).
// Configured=false reproduces the backward-compat "no rows ⇒ grant" rule.
type permState struct {
	Configured bool            `json:"configured"`
	Modules    map[string]bool `json:"modules"`
}

// modPermKey is the Redis cache key for a user's module permissions.
func modPermKey(orgID, userID string) string {
	return "modperm:" + orgID + ":" + userID
}

// InvalidateModulePermissions clears the cached permission state for a user so a
// permission change takes effect immediately rather than after TTL. Best-effort
// — call after writing user_module_permissions. No-op when rdb is nil.
func InvalidateModulePermissions(ctx context.Context, rdb *redis.Client, orgID, userID string) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(ctx, modPermKey(orgID, userID)).Err()
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
// rdb is optional (variadic): pass a Redis client to enable per-(org,user)
// permission caching (S90-4). Omit it (or pass nil) for the legacy uncached
// behaviour — the fail-closed guarantee is identical either way.
func RequireModuleAccess(db *pgxpool.Pool, module string, rdb ...*redis.Client) echo.MiddlewareFunc {
	var r *redis.Client
	if len(rdb) > 0 {
		r = rdb[0]
	}
	return requireModuleAccess(db, module, r)
}

// requireModuleAccess is the testable implementation behind RequireModuleAccess.
// It accepts the modulePermDB interface so tests can inject a fake without a
// real Postgres connection. rdb may be nil (no cache).
func requireModuleAccess(db modulePermDB, module string, rdb *redis.Client) echo.MiddlewareFunc {
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

			state, err := loadPermState(c.Request().Context(), db, rdb, orgID, userID)
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

			// Backward-compat: no rows means permissions have not been configured
			// for this user yet — grant access so existing deployments are unaffected.
			if !state.Configured {
				return next(c)
			}

			if !state.Modules[module] {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "access denied",
					"code":  "MODULE_ACCESS_DENIED",
				})
			}

			return next(c)
		}
	}
}

// loadPermState resolves a user's module-permission state. With a warm Redis
// cache it returns without touching the DB. On a cache miss (or Redis outage)
// it queries the DB and caches the result. A DB error is returned to the caller
// so the middleware can fail closed (503) — Redis is only a cache, never a
// source of truth, so a Redis outage degrades to the uncached path, not to
// "allow".
func loadPermState(ctx context.Context, db modulePermDB, rdb *redis.Client, orgID, userID string) (permState, error) {
	if rdb != nil {
		if raw, err := rdb.Get(ctx, modPermKey(orgID, userID)).Bytes(); err == nil {
			var s permState
			if json.Unmarshal(raw, &s) == nil {
				return s, nil
			}
		}
	}

	rows, err := db.Query(ctx,
		`SELECT module, can_read FROM user_module_permissions
		 WHERE org_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID,
	)
	if err != nil {
		return permState{}, err
	}
	defer rows.Close()

	state := permState{Modules: map[string]bool{}}
	for rows.Next() {
		var mod string
		var canRead bool
		if scanErr := rows.Scan(&mod, &canRead); scanErr != nil {
			log.Warn().Err(scanErr).Msg("module permission check: scan error, skipping row")
			continue
		}
		state.Configured = true
		state.Modules[mod] = canRead
	}
	if rowErr := rows.Err(); rowErr != nil {
		return permState{}, rowErr
	}

	if rdb != nil {
		if data, mErr := json.Marshal(state); mErr == nil {
			_ = rdb.Set(ctx, modPermKey(orgID, userID), data, modPermTTL).Err()
		}
	}
	return state, nil
}
