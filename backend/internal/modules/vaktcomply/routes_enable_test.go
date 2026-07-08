// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
)

// TestFeatureGatedEnableRoutesBindFrameworkName verifies the static,
// feature-gated enable routes (e.g. POST /frameworks/CRA/enable) correctly
// populate the "name" param that EnableFramework reads via c.Param("name").
//
// These routes don't declare a :name path segment, so without
// enableFrameworkNamed wiring the value up explicitly, c.Param("name") is
// always empty and every one of these frameworks (CRA, EUAIACT, BSI, TISAX,
// DORA, ISO42001, ISO27017, ISO27018 — essentially all Pro/Enterprise-gated
// frameworks) 400s with "framework name is required" regardless of request
// body or auth. No DB is reachable here (nil service), so we only assert we
// get PAST the name check — a real DB call panics, which middleware.Recover
// turns into a 500; the assertion is that we do NOT see the 400
// "framework name is required" response, which is what proves routing wired
// the name up before ever reaching the service layer.
func TestFeatureGatedEnableRoutesBindFrameworkName(t *testing.T) {
	routes := []string{
		"/frameworks/CRA/enable",
		"/frameworks/EUAIACT/enable",
		"/frameworks/BSI/enable",
		"/frameworks/TISAX/enable",
		"/frameworks/DORA/enable",
		"/frameworks/ISO42001/enable",
		"/frameworks/ISO27017/enable",
		"/frameworks/ISO27018/enable",
	}

	for _, path := range routes {
		t.Run(path, func(t *testing.T) {
			e := echo.New()
			e.Use(middleware.Recover())
			// Simulate what the real auth/license middleware chain would have set
			// by request time, so the request actually reaches EnableFramework
			// instead of being rejected earlier by RequireRole (403) or
			// features.Require (402) — either of which would make this test pass
			// vacuously without ever exercising the routing fix.
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set("roles", []string{"Admin"})
					c.Set("license", &license.License{Demo: true})
					return next(c)
				}
			})
			g := e.Group("")
			registerRoutes(g, &Handler{}) // service-less handler — see Register() doc

			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.NotEqual(t, http.StatusBadRequest, rec.Code,
				"route must not reject with 400 before reaching the service layer")
			assert.NotContains(t, rec.Body.String(), "framework name is required",
				"c.Param(\"name\") must be populated for static feature-gated routes")
		})
	}
}

// TestEnableDraftFrameworkReturns403NotInternalError verifies that rejecting
// a draft-status framework (TISAX, DORA, ...) surfaces as a 403, not a
// generic 500 "failed to enable framework". The draft-status check in
// policy.Service.EnableFramework runs before any DB access, so a
// policy.Service with a nil repo is enough to exercise it without a real DB.
func TestEnableDraftFrameworkReturns403NotInternalError(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("roles", []string{"Admin"})
			c.Set("license", &license.License{Demo: true})
			return next(c)
		}
	})
	g := e.Group("")
	registerRoutes(g, &Handler{service: &Service{Policy: &policy.Service{}}})

	req := httptest.NewRequest(http.MethodPost, "/frameworks/TISAX/enable", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "CK_FRAMEWORK_DRAFT")
}

// TestEnableFrameworkCasingCannotBypassFeatureGate is a regression test for a
// real paywall bypass: Echo's router is case-sensitive, so a request for
// /frameworks/cra/enable (or any non-canonical casing) doesn't match the
// literal, feature-gated /frameworks/CRA/enable route — it falls through to
// the generic /frameworks/:name/enable route, which only checks role, not
// license. Verified live against a real license/DB stack before this fix:
// POST /frameworks/cra/enable succeeded and enabled CRA for an org with no
// Pro license. The fix re-checks the feature gate inside EnableFramework
// itself, keyed by the case-normalised name, so it can't be routed around.
func TestEnableFrameworkCasingCannotBypassFeatureGate(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("roles", []string{"Admin"})
			// Community tier: no Demo flag, no Features — CRA must stay gated.
			c.Set("license", &license.License{Tier: "community"})
			return next(c)
		}
	})
	g := e.Group("")
	registerRoutes(g, &Handler{}) // service-less — a 402 here must never reach it

	for _, path := range []string{"/frameworks/cra/enable", "/frameworks/Cra/enable", "/frameworks/CRA/enable"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusPaymentRequired, rec.Code,
				"a Community-tier license must not be able to enable CRA via any casing")
			assert.Contains(t, rec.Body.String(), "feature_not_available")
		})
	}
}
