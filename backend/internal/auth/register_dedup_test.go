// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// TestRegisterKeepsGroupMiddlewareAndAppliesCredentialLimiter is the regression
// guard for R1-SA08-01. The bug: the four credential routes were registered a
// second time on the parent group, and Echo's router keeps the last handler for
// an identical method+path — silently dropping the auth group's IP rate limiter
// for login/register/password-reset. The fix registers each credential route
// exactly once, inside the group, with the credential limiter as a per-route
// middleware.
//
// The test asserts BOTH middlewares run for /login: the group middleware (stand-in
// for authRateLimiter — the one that used to be dropped) AND the credential
// limiter passed into Register. The credential middleware short-circuits with 200
// so the handler on a zero-value *Handler is never invoked.
func TestRegisterKeepsGroupMiddlewareAndAppliesCredentialLimiter(t *testing.T) {
	e := echo.New()

	var groupHits, credHits int
	groupMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			groupHits++
			return next(c)
		}
	}
	credMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			credHits++
			// Short-circuit: prove the middleware ran without invoking the
			// handler, which would deref the zero-value Handler's nil service.
			return c.NoContent(http.StatusOK)
		}
	}

	g := e.Group("/auth", groupMW)
	Register(g, &Handler{}, credMW)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, groupHits,
		"group middleware must run for /login — a duplicate registration would silently drop it (R1-SA08-01)")
	require.Equal(t, 1, credHits, "credential limiter must run for /login")
}

// TestRegisterCredentialLimiterSkipsNonCredentialRoutes documents the scope of
// the credential limiter: it is attached only to the four credential routes, not
// to refresh/logout. Hitting /refresh must run the group middleware but NOT the
// credential middleware. /refresh is registered without cred and its handler
// would deref a nil service, so the group middleware short-circuits here.
func TestRegisterCredentialLimiterSkipsNonCredentialRoutes(t *testing.T) {
	e := echo.New()

	var credHits int
	shortCircuitGroup := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return c.NoContent(http.StatusOK) // stop before the real handler
		}
	}
	credMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			credHits++
			return next(c)
		}
	}

	g := e.Group("/auth", shortCircuitGroup)
	Register(g, &Handler{}, credMW)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 0, credHits, "credential limiter must NOT run for /refresh")
}
