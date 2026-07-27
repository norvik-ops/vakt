// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestRequireCSRF_S132S11 is the S132-S11/D24-2 regression guard for
// POST /license/activate. RegisterRoutes mounts /license on the bare `api`
// group in cmd/api/routes.go — not on `protected` — so it never inherited
// protected's auth.CSRFMiddleware. requireCSRF() closes that gap.
func TestRequireCSRF_S132S11(t *testing.T) {
	next := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
	handler := requireCSRF()(next)

	t.Run("missing token is 403", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/license/activate", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler(c))
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "CSRF_MISSING")
	})

	t.Run("mismatched cookie/header is 403", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/license/activate", strings.NewReader(`{}`))
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "cookie-token"})
		req.Header.Set(csrfHeaderName, "different-token")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler(c))
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "CSRF_MISMATCH")
	})

	t.Run("matching cookie/header reaches the handler", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/license/activate", strings.NewReader(`{}`))
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "matching-token"})
		req.Header.Set(csrfHeaderName, "matching-token")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler(c))
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("API key bypasses CSRF", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/license/activate", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer sk_deadbeef")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler(c))
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// TestRequireAdminRole_RoleNameMatchesDBSeed verifies the fix for audit
// finding F10. The middleware MUST accept the PascalCase role name "Admin"
// that the DB seed in migrations/001_core_schema.up.sql installs and that
// internal/auth/middleware.go emits.
//
// The previous implementation checked for lowercase "admin", which is never
// produced anywhere in the codebase — making the Pro license activation
// endpoint un-callable for every legitimate admin.
func TestRequireAdminRole_RoleNameMatchesDBSeed(t *testing.T) {
	cases := []struct {
		name       string
		roles      []string
		wantStatus int
	}{
		{
			name:       "Admin (PascalCase, as seeded in DB) — allowed",
			roles:      []string{"Admin"},
			wantStatus: http.StatusNoContent, // next() handler answers 204
		},
		{
			name:       "admin (lowercase — historical bug shape) — rejected",
			roles:      []string{"admin"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "SecurityAnalyst — rejected",
			roles:      []string{"SecurityAnalyst"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "Viewer — rejected",
			roles:      []string{"Viewer"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "AuditorReadOnly — rejected",
			roles:      []string{"AuditorReadOnly"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no roles — rejected",
			roles:      []string{},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "Admin alongside others — allowed",
			roles:      []string{"SecurityAnalyst", "Admin"},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/license/activate", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("roles", tc.roles)

			handler := requireAdminRole()(func(c echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})

			err := handler(c)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

// TestRequireAdminRole_NilRolesContext exercises the edge case where the
// auth middleware did not set a "roles" key at all. The middleware MUST
// treat that as "no roles" rather than panicking.
func TestRequireAdminRole_NilRolesContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/license/activate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// intentionally no c.Set("roles", ...)

	handler := requireAdminRole()(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
