// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

// Unit tests for MFAEnforceMiddleware.
//
// These tests cover all branches that do not require a real database connection:
//   - Exempt path bypass (prevents TOTP-setup lockout regression)
//   - Missing context bypass (unauthenticated requests)
//
// The DB-dependent branches (require_mfa lookup, totp_secrets lookup) are covered
// via the mfaEnforceMiddleware internal function using a lightweight fake that
// satisfies the mfaDB interface — no testcontainers needed.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// ─── Fake DB infrastructure ───────────────────────────────────────────────────

// fakeRow implements pgx.Row for test injection.
type fakeRow struct {
	val any
	err error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		switch d := dest[0].(type) {
		case *bool:
			*d = r.val.(bool)
		}
	}
	return nil
}

// fakeMFADB simulates the two sequential QueryRow calls made by MFAEnforceMiddleware:
//  1. SELECT require_mfa FROM organizations WHERE id = $1  →  orgRow
//  2. SELECT enabled FROM totp_secrets WHERE user_id = $1 →  totpRow
type fakeMFADB struct {
	orgRow  pgx.Row
	totpRow pgx.Row
	call    int
}

func (f *fakeMFADB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	f.call++
	if f.call == 1 {
		return f.orgRow
	}
	return f.totpRow
}

// mfaRequest builds an Echo context for the given path and optional org/user context values.
func mfaRequest(t *testing.T, path, orgID, userID string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if orgID != "" {
		c.Set("org_id", orgID)
	}
	if userID != "" {
		c.Set("user_id", userID)
	}
	return c, rec
}

func okHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
}

// ─── Exempt path tests ────────────────────────────────────────────────────────

// TestMFAEnforce_ExemptPaths_AllowThrough verifies that every path in the
// exemption list is allowed regardless of org MFA policy. A missing exemption
// would lock users out of the TOTP setup flow when MFA is required.
func TestMFAEnforce_ExemptPaths_AllowThrough(t *testing.T) {
	exemptPaths := []string{
		"/api/v1/auth/2fa/setup",
		"/api/v1/auth/2fa/confirm",
		"/api/v1/auth/logout",
		"/api/v1/health",
		"/health",
	}

	// DB must NOT be called for exempt paths.
	// Passing a nil-equivalent fake: if QueryRow is called it returns an error row
	// that would cause a 503, making the test fail and revealing the regression.
	db := &fakeMFADB{
		orgRow: &fakeRow{err: errors.New("DB must not be called for exempt paths")},
	}

	for _, path := range exemptPaths {
		t.Run(path, func(t *testing.T) {
			c, rec := mfaRequest(t, path, "org-1", "user-1")
			mw := auth.MFAEnforceMiddlewareForTest(db)
			err := mw(okHandler)(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code,
				"exempt path %q must pass through without DB lookup", path)
		})
	}
}

// TestMFAEnforce_NoOrgID_AllowsThrough verifies that a request with no org_id
// in the Echo context is allowed through — the middleware must not be applied
// before AuthMiddleware has populated the context.
func TestMFAEnforce_NoOrgID_AllowsThrough(t *testing.T) {
	db := &fakeMFADB{orgRow: &fakeRow{err: errors.New("must not reach DB")}}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "", "user-1")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestMFAEnforce_NoUserID_AllowsThrough verifies that a request with no user_id
// passes through — same guard as the org_id check.
func TestMFAEnforce_NoUserID_AllowsThrough(t *testing.T) {
	db := &fakeMFADB{orgRow: &fakeRow{err: errors.New("must not reach DB")}}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "org-1", "")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ─── DB-dependent branch tests ────────────────────────────────────────────────

// TestMFAEnforce_OrgRequireMFAFalse_AllowsThrough verifies that when the
// organisation has require_mfa=false, the request passes without checking TOTP.
func TestMFAEnforce_OrgRequireMFAFalse_AllowsThrough(t *testing.T) {
	db := &fakeMFADB{
		orgRow:  &fakeRow{val: false}, // require_mfa = false
		totpRow: &fakeRow{err: errors.New("totp must not be queried when mfa not required")},
	}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "org-1", "user-1")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestMFAEnforce_OrgRequiresMFA_UserHasTOTP_AllowsThrough verifies that a user
// with TOTP enabled passes when the org requires MFA.
func TestMFAEnforce_OrgRequiresMFA_UserHasTOTP_AllowsThrough(t *testing.T) {
	db := &fakeMFADB{
		orgRow:  &fakeRow{val: true},  // require_mfa = true
		totpRow: &fakeRow{val: true},  // totp enabled = true
	}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "org-1", "user-1")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestMFAEnforce_OrgRequiresMFA_UserNoTOTP_Returns403 verifies that a user
// without TOTP enabled receives MFA_REQUIRED when the org mandates MFA.
func TestMFAEnforce_OrgRequiresMFA_UserNoTOTP_Returns403(t *testing.T) {
	db := &fakeMFADB{
		orgRow:  &fakeRow{val: true},  // require_mfa = true
		totpRow: &fakeRow{val: false}, // totp enabled = false
	}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "org-1", "user-1")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "MFA_REQUIRED")
}

// TestMFAEnforce_DBError_OrgLookup_Returns503 verifies that a database error
// during org lookup is fail-closed: returns 503 instead of granting access.
// This was a real bug fixed in v0.6.1 (MFA-Enforcement Fail-Closed).
func TestMFAEnforce_DBError_OrgLookup_Returns503(t *testing.T) {
	db := &fakeMFADB{
		orgRow: &fakeRow{err: errors.New("connection refused")},
	}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "org-1", "user-1")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"DB failure during org lookup must be fail-closed (503), not fail-open (200)")
	assert.Contains(t, rec.Body.String(), "SERVICE_UNAVAILABLE")
}

// TestMFAEnforce_DBError_TOTPLookup_Returns403 verifies that a database error
// during TOTP lookup is also fail-closed: the user is denied access rather than
// being allowed through as if MFA were satisfied.
func TestMFAEnforce_DBError_TOTPLookup_Returns403(t *testing.T) {
	db := &fakeMFADB{
		orgRow:  &fakeRow{val: true}, // require_mfa = true
		totpRow: &fakeRow{err: errors.New("totp table missing")},
	}
	c, rec := mfaRequest(t, "/api/v1/secvitals/controls", "org-1", "user-1")
	mw := auth.MFAEnforceMiddlewareForTest(db)
	require.NoError(t, mw(okHandler)(c))
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"TOTP lookup error must deny access (fail-closed), not grant it")
	assert.Contains(t, rec.Body.String(), "MFA_REQUIRED")
}
