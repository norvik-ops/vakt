// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

// Unit tests for RequireModuleAccess middleware.
//
// All four core branches are tested without a real database connection:
//   1. No permission rows → backward-compat grant (permissions not configured yet)
//   2. Rows exist, module allowed (can_read=true)  → access granted
//   3. Rows exist, module NOT in list              → 403 MODULE_ACCESS_DENIED
//   4. Admin role                                  → access granted regardless of permissions
//
// A lightweight fake that satisfies the modulePermDB interface is injected via
// RequireModuleAccessForTest (defined in export_test.go). No testcontainers needed.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// ─── Fake DB infrastructure ───────────────────────────────────────────────────

// fakeModulePermRow simulates a single permission row returned by the fake.
type fakeModulePermRow struct {
	module  string
	canRead bool
}

// fakeModulePermDB satisfies modulePermDB. It returns a pre-configured set of
// rows (or an error) when Query is called.
type fakeModulePermDB struct {
	rows []fakeModulePermRow
	err  error
}

func (f *fakeModulePermDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &fakePermRows{rows: f.rows}, nil
}

// fakePermRows implements pgx.Rows for the permission rows.
type fakePermRows struct {
	rows  []fakeModulePermRow
	index int
	err   error
}

func (r *fakePermRows) Next() bool {
	r.index++
	return r.index <= len(r.rows)
}

func (r *fakePermRows) Scan(dest ...any) error {
	row := r.rows[r.index-1]
	if len(dest) >= 2 {
		if m, ok := dest[0].(*string); ok {
			*m = row.module
		}
		if b, ok := dest[1].(*bool); ok {
			*b = row.canRead
		}
	}
	return nil
}

func (r *fakePermRows) Close() {}

func (r *fakePermRows) Err() error { return r.err }

// pgx.Rows requires several additional interface methods — provide no-op
// implementations so the compiler is satisfied.
func (r *fakePermRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakePermRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakePermRows) RawValues() [][]byte                          { return nil }
func (r *fakePermRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakePermRows) Conn() *pgx.Conn                              { return nil }

// ─── Test helpers ─────────────────────────────────────────────────────────────

// modulePermRequest builds an Echo context with the given identity values.
func modulePermRequest(t *testing.T, orgID, userID string, roles []string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktscan/scans", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if orgID != "" {
		c.Set("org_id", orgID)
	}
	if userID != "" {
		c.Set("user_id", userID)
	}
	if roles != nil {
		c.Set("roles", roles)
	}
	return c, rec
}

func okModuleHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestRequireModuleAccess_NoRows_GrantsAccess verifies the backward-compatibility
// rule: when no permission rows exist for the user+org, access is granted so
// existing deployments are not broken when the middleware is first deployed.
func TestRequireModuleAccess_NoRows_GrantsAccess(t *testing.T) {
	db := &fakeModulePermDB{rows: []fakeModulePermRow{}} // empty — no rows
	c, rec := modulePermRequest(t, "org-1", "user-1", []string{"SecurityAnalyst"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code,
		"no permission rows must grant access (backward-compat)")
}

// TestRequireModuleAccess_RowsExist_ModuleAllowed_GrantsAccess verifies that a
// user with an explicit can_read=true permission for the module gets through.
func TestRequireModuleAccess_RowsExist_ModuleAllowed_GrantsAccess(t *testing.T) {
	db := &fakeModulePermDB{
		rows: []fakeModulePermRow{
			{module: "vaktscan", canRead: true},
			{module: "vaktcomply", canRead: true},
		},
	}
	c, rec := modulePermRequest(t, "org-1", "user-1", []string{"SecurityAnalyst"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireModuleAccess_RowsExist_ModuleNotAllowed_Returns403 verifies that a
// user whose permissions include other modules but NOT the requested module
// receives 403 MODULE_ACCESS_DENIED.
func TestRequireModuleAccess_RowsExist_ModuleNotAllowed_Returns403(t *testing.T) {
	db := &fakeModulePermDB{
		rows: []fakeModulePermRow{
			{module: "vaktcomply", canRead: true}, // only comply — not vaktscan
		},
	}
	c, rec := modulePermRequest(t, "org-1", "user-1", []string{"SecurityAnalyst"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without vaktscan permission must receive 403")
	assert.Contains(t, rec.Body.String(), "MODULE_ACCESS_DENIED")
}

// TestRequireModuleAccess_RowsExist_CanReadFalse_Returns403 verifies that a row
// that explicitly sets can_read=false also results in 403.
func TestRequireModuleAccess_RowsExist_CanReadFalse_Returns403(t *testing.T) {
	db := &fakeModulePermDB{
		rows: []fakeModulePermRow{
			{module: "vaktscan", canRead: false}, // row exists but no read
		},
	}
	c, rec := modulePermRequest(t, "org-1", "user-1", []string{"Viewer"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "MODULE_ACCESS_DENIED")
}

// TestRequireModuleAccess_AdminRole_GrantsAccess verifies that Admin users bypass
// the permission check entirely — even if no rows exist or rows deny access.
func TestRequireModuleAccess_AdminRole_GrantsAccess(t *testing.T) {
	// DB should NOT be called for admins (any DB error would reveal the regression).
	db := &fakeModulePermDB{err: errors.New("DB must not be called for admin users")}
	c, rec := modulePermRequest(t, "org-1", "user-admin", []string{"Admin"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code,
		"Admin role must bypass module permission check entirely")
}

// TestRequireModuleAccess_LowercaseAdmin_GrantsAccess verifies that the
// lowercase "admin" role string also triggers the bypass (belt-and-suspenders).
func TestRequireModuleAccess_LowercaseAdmin_GrantsAccess(t *testing.T) {
	db := &fakeModulePermDB{err: errors.New("DB must not be called for admin users")}
	c, rec := modulePermRequest(t, "org-1", "user-admin", []string{"admin"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequireModuleAccess_DBError_FailsOpen verifies that a database error during
// the permission check fails open (grants access) rather than locking users out
// due to an infrastructure issue. A warn log is expected (not tested here).
func TestRequireModuleAccess_DBError_FailsOpen(t *testing.T) {
	db := &fakeModulePermDB{err: errors.New("connection refused")}
	c, rec := modulePermRequest(t, "org-1", "user-1", []string{"SecurityAnalyst"})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code,
		"DB error during permission check must fail open, not lock users out")
}

// TestRequireModuleAccess_APIKey_SkipsCheck verifies that requests authenticated
// via API key bypass the module permission table (API keys use scope-based auth).
func TestRequireModuleAccess_APIKey_SkipsCheck(t *testing.T) {
	// Even if DB would deny access, API key requests must bypass the check.
	db := &fakeModulePermDB{
		rows: []fakeModulePermRow{
			{module: "vaktcomply", canRead: true}, // no vaktscan row
		},
	}
	c, rec := modulePermRequest(t, "org-1", "user-1", []string{"SecurityAnalyst"})
	c.Set("auth_method", "api_key")
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code,
		"API key auth_method must bypass module permission check")
}

// TestRequireModuleAccess_MissingContext_PassesThrough verifies that a request
// without org_id/user_id set (e.g. middleware misconfiguration) is allowed
// through rather than causing a panic or incorrect 403.
func TestRequireModuleAccess_MissingContext_PassesThrough(t *testing.T) {
	db := &fakeModulePermDB{err: errors.New("DB must not be called when context missing")}
	c, rec := modulePermRequest(t, "", "", []string{})
	mw := auth.RequireModuleAccessForTest(db, "vaktscan")
	require.NoError(t, mw(okModuleHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}
