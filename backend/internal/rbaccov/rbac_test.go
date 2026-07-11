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
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/evidence_auto"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/platform/ldap"
	"github.com/matharnica/vakt/internal/shared/platform/trustcenter"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
)

// testHexKey mirrors the constant from auth package tests.
const testHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// writeRoute is a mutating endpoint that must reject a Viewer with 403.
type writeRoute struct {
	method string
	path   string
}

// sharedWriteRoutes enumerates every mutating route across the seven shared
// platform packages that S121 Epic B gated. Each entry must return 403 for a
// Viewer token and something other than 403 for an Admin token. Paths are the
// full /api/v1-relative paths as mounted by the router builder below.
var sharedWriteRoutes = []writeRoute{
	// R1 — scheduledreports (Admin-only writes)
	{http.MethodPost, "/reports/scheduled"},
	{http.MethodPut, "/reports/scheduled/abc"},
	{http.MethodDelete, "/reports/scheduled/abc"},
	{http.MethodPost, "/reports/scheduled/abc/run"},
	// R2 — trustcenter admin
	{http.MethodPatch, "/trust-center/settings"},
	{http.MethodPost, "/trust-center/certificates"},
	{http.MethodDelete, "/trust-center/certificates/abc"},
	{http.MethodPost, "/trust-center/policies/abc/publish"},
	{http.MethodDelete, "/trust-center/policies/abc/publish"},
	// R3 — dashboard score config
	{http.MethodPut, "/dashboard/score/config"},
	// R4 — evidence_auto assign
	{http.MethodPost, "/vaktcomply/evidence/auto/abc/assign"},
	// R5 — retention config
	{http.MethodPut, "/retention/config"},
	// R6 — alerting channels + test delivery
	{http.MethodPost, "/alerting/channels"},
	{http.MethodDelete, "/alerting/channels/abc"},
	{http.MethodPut, "/alerting/channels/abc/toggle"},
	{http.MethodPost, "/alerting/channels/abc/test"},
	// R7 — ldap settings
	{http.MethodPut, "/settings/ldap"},
	{http.MethodPost, "/settings/ldap/test"},
	{http.MethodPost, "/settings/ldap/sync"},
}

// buildSharedRouter mounts every shared platform package that carries mutating
// routes, using nil backing stores. Because RequireRole runs before the handler,
// gated write routes short-circuit to 403 without touching the nil services.
func buildSharedRouter(t *testing.T) *echo.Echo {
	t.Helper()
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	paseto := auth.PasetoMiddleware(key, nil)

	e := echo.New()
	e.Use(echomw.Recover()) // catch handler panics from nil services

	scheduledreports.Register(e.Group("/reports", paseto), scheduledreports.NewHandler(nil))
	trustcenter.RegisterAdmin(e.Group("", paseto), nil)
	dashboard.Register(e.Group("/dashboard", paseto), nil, nil)
	evidence_auto.RegisterRoutes(e.Group("/vaktcomply", paseto), nil)
	retention.Register(e.Group("", paseto), nil)
	alerting.Register(e.Group("", paseto), nil,
		[]byte("0123456789abcdef0123456789abcdef"), alerting.SMTPConfig{})
	ldap.Register(e.Group("", paseto), ldap.Config{}, paseto)
	return e
}

// TestSharedPackagesRBAC is the S121 O3 coverage gate: a Viewer must be rejected
// with 403 on every mutating route of the seven shared platform packages, and an
// Admin must not be blocked by the RBAC layer. Removing any RequireRole added in
// Epic B turns this test red.
func TestSharedPackagesRBAC(t *testing.T) {
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

	e := buildSharedRouter(t)

	for _, rt := range sharedWriteRoutes {
		rt := rt
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			// Viewer must be forbidden.
			req := httptest.NewRequest(rt.method, rt.path, nil)
			req.Header.Set("Authorization", "Bearer "+viewerTok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"Viewer should get 403 on %s %s", rt.method, rt.path)

			// Admin must clear the RBAC layer (handler may 4xx/5xx on nil deps,
			// which is fine — we only assert it is not blocked by RBAC).
			req2 := httptest.NewRequest(rt.method, rt.path, nil)
			req2.Header.Set("Authorization", "Bearer "+adminTok)
			rec2 := httptest.NewRecorder()
			e.ServeHTTP(rec2, req2)
			assert.NotEqual(t, http.StatusForbidden, rec2.Code,
				"Admin should not get 403 on %s %s", rt.method, rt.path)
		})
	}
}
