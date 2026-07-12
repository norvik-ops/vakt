// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package rbaccov_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/evidence_auto"
	"github.com/matharnica/vakt/internal/shared/comments"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/onboarding"
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
	// S124-8 (N7): comments + onboarding writes are now role-gated — cover them so
	// a future un-gating turns the deny-by-default test red. GET routes on these
	// (comments list, team members, onboarding status/progress) are reads and are
	// exempt via viewerWriteAllow below only if truly self-service.
	comments.Register(e.Group("", paseto), nil)
	onboarding.RegisterRoutes(e.Group("/onboarding", paseto), nil)
	return e
}

// TestSharedPackagesRBAC is the S121 O3 coverage gate, hardened in S123-G1:
// every mutating route of the shared platform packages is Admin-only, so BOTH a
// Viewer AND a SecurityAnalyst must be rejected with 403, and an Admin must not
// be blocked. Probing SecurityAnalyst (not just Viewer) is the K2 lesson from
// MA-01/02/03: a Viewer-only probe reports "green" while an Analyst still writes.
// Removing any RequireRole added in Epic B / S122 turns this red.
func TestSharedPackagesRBAC(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	adminTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "admin-1", OrgID: "org-1", Roles: []string{"Admin"},
	})
	require.NoError(t, err)

	deniedRoles := map[string]string{
		"Viewer":          issueTok(t, key, "Viewer"),
		"SecurityAnalyst": issueTok(t, key, "SecurityAnalyst"),
	}

	e := buildSharedRouter(t)

	for _, rt := range sharedWriteRoutes {
		rt := rt
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			// Every non-admin role must be forbidden (Admin-only surface).
			for roleName, tok := range deniedRoles {
				req := httptest.NewRequest(rt.method, rt.path, nil)
				req.Header.Set("Authorization", "Bearer "+tok)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusForbidden, rec.Code,
					"%s should get 403 on %s %s", roleName, rt.method, rt.path)
			}

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

// issueTok is a small helper to mint a token for a single role.
func issueTok(t *testing.T, key auth.SymmetricKey, role string) string {
	t.Helper()
	tok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "u-" + role, OrgID: "org-1", Roles: []string{role},
	})
	require.NoError(t, err)
	return tok
}

// writeMethods is the set of HTTP methods that mutate state; a route using any
// of these MUST be gated (or explicitly allow-listed as self-service/public).
var writeMethods = map[string]bool{
	http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true,
}

// viewerWriteAllow is the explicit, justified allowlist of write routes a Viewer
// MAY reach on the shared/integration surface. It is intentionally empty: every
// shared platform write is Admin-only. Any new entry needs a one-line reason.
// The deny-by-default test below fails the moment a new ungated shared write
// route appears that is not listed here — so a future R1–R7 / MA-01 cannot ship
// silently just because nobody remembered to add it to sharedWriteRoutes.
var viewerWriteAllow = map[string]bool{
	// Per-user self-service: any authenticated user (incl. Viewer) manages their
	// OWN in-app notifications. Not an org-write; the handler scopes by user_id.
	"POST /dashboard/notifications/read-all": true,
	"POST /dashboard/notifications/:id/read": true,
}

// TestSharedSurfaceDenyByDefault is the S123-G1 deny-by-default gate. Instead of
// trusting the curated sharedWriteRoutes list to be complete, it enumerates
// EVERY route actually registered on the shared and integration routers via
// echo.Routes() and asserts a Viewer is forbidden on each write method — unless
// the route is in viewerWriteAllow. A newly added, ungated shared write route is
// red by construction, without anyone having to remember to list it.
func TestSharedSurfaceDenyByDefault(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	viewerTok := issueTok(t, key, "Viewer")

	routers := []struct {
		name string
		e    *echo.Echo
	}{
		{"shared", buildSharedRouter(t)},
		{"integrations", buildIntegrationsRouter(t)},
	}

	for _, r := range routers {
		for _, route := range r.e.Routes() {
			if !writeMethods[route.Method] {
				continue
			}
			// Echo records parameterised paths as /x/:id; substitute a value so
			// the request actually matches the route.
			reqPath := strings.NewReplacer(":id", "probe", ":token", "probe", ":name", "probe").Replace(route.Path)
			key := route.Method + " " + route.Path
			if viewerWriteAllow[key] {
				continue
			}
			t.Run(r.name+" "+key, func(t *testing.T) {
				req := httptest.NewRequest(route.Method, reqPath, nil)
				req.Header.Set("Authorization", "Bearer "+viewerTok)
				rec := httptest.NewRecorder()
				r.e.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusForbidden, rec.Code,
					"Viewer must be 403 on write route %s (add to viewerWriteAllow with a reason if intentional)", key)
			})
		}
	}
}
