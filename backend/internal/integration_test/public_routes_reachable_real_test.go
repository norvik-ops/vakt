//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/shared/platform/trustcenter"
)

// passThroughMW is a no-op echo.MiddlewareFunc for wiring RegisterPublic in tests:
// the per-route rate limiter (R-H15/S131-C2) is production-only; the test just needs
// the routes registered.
var passThroughMW echo.MiddlewareFunc = func(next echo.HandlerFunc) echo.HandlerFunc { return next }

// TestPublicRoutesReachableWithoutToken is the S127-5 (G10) gate — the counter to
// rbaccov, which only proves write⇒403. This proves the OTHER direction for the
// deliberately-public routes: they MUST be reachable WITHOUT a token.
//
// The whole of Sprint 127 exists because these routes (Vakt Aware tracking
// pixel/click/submit + Vakt Vault share link) were commented "public" but mounted
// under `protected`, so every recipient without a session got 401 and the module
// was silently dead. No prior gate caught that. This closes the gap: if any of
// these slips back behind auth, it returns 401/403 here and the build goes red.
//
// Run against a real (empty) DB so the token-only handlers execute cleanly
// (unknown token → pixel 200 / not-found / bad-request), never 401/403.
func TestPublicRoutesReachableWithoutToken(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, _, cleanup := bootPostgresWithOrg(t)
	defer cleanup()

	awareSvc := vaktaware.NewService(pool, vaktaware.SMTPConfig{})
	vaultSvc := vaktvault.NewService(pool, make([]byte, 32), nil)

	e := echo.New()
	// Mount exactly as cmd/api/routes.go does for the public groups — NO auth mw.
	vaktaware.RegisterPublic(e.Group("/api/v1/vaktaware"), vaktaware.NewHandler(awareSvc), passThroughMW)
	vaktvault.RegisterPublic(e.Group("/api/v1/vaktvault"), vaktvault.NewHandler(vaultSvc), passThroughMW)
	// S131-D4 (R-H13/D18-06): the public Trust Center data route must live under
	// /api/v1 (Caddy only proxies /api/*), reachable without a token.
	trustcenter.Register(e.Group("/api/v1"), pool)

	publicRoutes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/vaktaware/track/sometoken"},     // open pixel
		{http.MethodGet, "/api/v1/vaktaware/t/sometoken"},         // click
		{http.MethodPost, "/api/v1/vaktaware/t/sometoken/submit"}, // form submit
		{http.MethodPost, "/api/v1/vaktaware/phish-report"},       // phish-report webhook
		{http.MethodGet, "/api/v1/vaktvault/share/sometoken"},     // vault share link
		{http.MethodGet, "/api/v1/trust/some-org-slug"},           // public trust center page data
	}

	for _, rt := range publicRoutes {
		rt := rt
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil) // NO Authorization header
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
				"%s %s must be reachable without a token — it is a public route", rt.method, rt.path)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"%s %s must not be role-gated — it is a public route", rt.method, rt.path)
		})
	}

	// The open pixel is additionally an enumeration oracle if it distinguishes a
	// valid from an invalid token (S127 §3c) — it must return 200 either way.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktaware/track/definitely-invalid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "open pixel must return 200 even for an unknown token (no oracle)")
}
