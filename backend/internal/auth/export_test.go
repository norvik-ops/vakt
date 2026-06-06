// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// export_test.go is compiled only during `go test`. It promotes internal
// symbols to the auth_test package so white-box tests can inject fakes
// without exposing implementation details to production callers.

package auth

import "github.com/labstack/echo/v4"

// MFAEnforceMiddlewareForTest exposes mfaEnforceMiddleware so tests can
// inject a lightweight fake DB instead of requiring a real Postgres connection.
func MFAEnforceMiddlewareForTest(db mfaDB) echo.MiddlewareFunc {
	return mfaEnforceMiddleware(db)
}

// RequireModuleAccessForTest exposes requireModuleAccess so tests can
// inject a lightweight fake DB instead of requiring a real Postgres connection.
func RequireModuleAccessForTest(db modulePermDB, module string) echo.MiddlewareFunc {
	return requireModuleAccess(db, module)
}
