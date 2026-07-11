// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package rbaccov_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
)

// S121-E2 (O3): the three business modules vaktscan/vaktcomply/vaktvault had no
// rbac_test.go (unlike vaktprivacy/vakthr/vaktaware). These tables assert that a
// Viewer is rejected with 403 on representative write routes and an Admin is not,
// so removing a RequireRole from any of them turns CI red.
//
// A full-feature demo license is injected into the context before the module
// routes run, so feature.Require(...) gates (which return 402, not 403) pass and
// the RBAC gate is the one under test even on Pro routes.

func fullLicenseMiddleware() echo.MiddlewareFunc {
	lic := license.Load("", true) // isDemo=true → all features enabled
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", lic)
			return next(c)
		}
	}
}

func runModuleRBAC(t *testing.T, prefix string, register func(*echo.Group), routes []writeRoute) {
	t.Helper()
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	viewerTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "viewer-1", OrgID: "org-1", Roles: []string{"Viewer"},
	})
	require.NoError(t, err)
	adminTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "admin-1", OrgID: "org-1", Roles: []string{"Admin"},
	})
	require.NoError(t, err)

	e := echo.New()
	e.Use(echomw.Recover())
	g := e.Group(prefix, auth.PasetoMiddleware(key, nil), fullLicenseMiddleware())
	register(g)

	for _, rt := range routes {
		rt := rt
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			req.Header.Set("Authorization", "Bearer "+viewerTok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"Viewer should get 403 on %s %s", rt.method, rt.path)

			req2 := httptest.NewRequest(rt.method, rt.path, nil)
			req2.Header.Set("Authorization", "Bearer "+adminTok)
			rec2 := httptest.NewRecorder()
			e.ServeHTTP(rec2, req2)
			assert.NotEqual(t, http.StatusForbidden, rec2.Code,
				"Admin should not get 403 on %s %s", rt.method, rt.path)
		})
	}
}

func TestVaktscanModuleRBAC(t *testing.T) {
	routes := []writeRoute{
		{http.MethodPost, "/vaktscan/assets"},
		{http.MethodPut, "/vaktscan/assets/abc"},
		{http.MethodDelete, "/vaktscan/assets/abc"},
		{http.MethodPost, "/vaktscan/assets/import"},
		{http.MethodPost, "/vaktscan/findings/bulk"},
		{http.MethodPatch, "/vaktscan/findings/abc"},
		{http.MethodDelete, "/vaktscan/findings/abc"},
		{http.MethodPost, "/vaktscan/suppressions"},
		{http.MethodPut, "/vaktscan/sla-config"},
		{http.MethodPost, "/vaktscan/certificates"},
		{http.MethodPost, "/vaktscan/findings/import"}, // Pro (SecPulse)
	}
	runModuleRBAC(t, "/vaktscan", func(g *echo.Group) {
		vaktscan.Register(g, &vaktscan.Handler{})
	}, routes)
}

func TestVaktvaultModuleRBAC(t *testing.T) {
	routes := []writeRoute{
		{http.MethodPost, "/vaktvault/projects"},
		{http.MethodDelete, "/vaktvault/projects/abc"},
		{http.MethodPost, "/vaktvault/projects/abc/envs"},
		{http.MethodPut, "/vaktvault/projects/abc/envs/e1/secrets/k1"},
		{http.MethodDelete, "/vaktvault/projects/abc/envs/e1/secrets/k1"},
		{http.MethodPost, "/vaktvault/projects/abc/import"},
		{http.MethodPost, "/vaktvault/tokens"},                                 // Pro (API)
		{http.MethodPost, "/vaktvault/projects/abc/envs/e1/secrets/k1/rotate"}, // Pro (SecVault)
		{http.MethodPost, "/vaktvault/git-scans"},                              // Pro
		{http.MethodPost, "/vaktvault/access-reviews"},                         // Pro
	}
	runModuleRBAC(t, "/vaktvault", func(g *echo.Group) {
		vaktvault.Register(g, &vaktvault.Handler{})
	}, routes)
}

func TestVaktcomplyModuleRBAC(t *testing.T) {
	routes := []writeRoute{
		{http.MethodPatch, "/vaktcomply/controls/bulk"},
		{http.MethodPatch, "/vaktcomply/controls/abc"},
		{http.MethodDelete, "/vaktcomply/frameworks/abc"},
		{http.MethodPost, "/vaktcomply/risks"},
		{http.MethodDelete, "/vaktcomply/risks/abc"},
		{http.MethodPost, "/vaktcomply/capas"},
		{http.MethodPatch, "/vaktcomply/capas/bulk"},
		{http.MethodDelete, "/vaktcomply/capas/abc"},
		{http.MethodPost, "/vaktcomply/policies"},
		{http.MethodPost, "/vaktcomply/incidents"},
		{http.MethodDelete, "/vaktcomply/pentests/abc"},
		{http.MethodDelete, "/vaktcomply/suppliers/abc"},
	}
	runModuleRBAC(t, "/vaktcomply", func(g *echo.Group) {
		vaktcomply.Register(g)
	}, routes)
}
