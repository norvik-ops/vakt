// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// export_test.go is compiled only during `go test`. It promotes internal
// symbols to the auth_test package so white-box tests can inject fakes
// without exposing implementation details to production callers.

package auth

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// MFAEnforceMiddlewareForTest exposes mfaEnforceMiddleware so tests can
// inject a lightweight fake DB instead of requiring a real Postgres connection.
func MFAEnforceMiddlewareForTest(db mfaDB) echo.MiddlewareFunc {
	return mfaEnforceMiddleware(db)
}

// RequireModuleAccessForTest exposes requireModuleAccess so tests can
// inject a lightweight fake DB instead of requiring a real Postgres connection.
// Uncached path (rdb=nil) — preserves the legacy behaviour under test.
func RequireModuleAccessForTest(db modulePermDB, module string) echo.MiddlewareFunc {
	return requireModuleAccess(db, module, nil)
}

// RequireModuleAccessCachedForTest exposes the Redis-cached path (S90-4).
func RequireModuleAccessCachedForTest(db modulePermDB, module string, rdb *redis.Client) echo.MiddlewareFunc {
	return requireModuleAccess(db, module, rdb)
}

// LoadPermStateForTest exposes loadPermState for cache-behaviour assertions.
func LoadPermStateForTest(ctx context.Context, db modulePermDB, rdb *redis.Client, orgID, userID string) error {
	_, err := loadPermState(ctx, db, rdb, orgID, userID)
	return err
}

// ModPermKeyForTest exposes the cache key builder.
func ModPermKeyForTest(orgID, userID string) string { return modPermKey(orgID, userID) }

// HumanValidationErrorForTest exposes humanValidationError for white-box tests.
var HumanValidationErrorForTest = humanValidationError
