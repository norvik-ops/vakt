// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// noopAuthMW is a stand-in for auth.AuthMiddleware that just sets a fake
// user_id on the context — RegisterTOTP's CSRF wiring is what's under test
// here, not the token-validation logic covered elsewhere.
func noopAuthMW(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("user_id", "00000000-0000-0000-0000-000000000001")
		return next(c)
	}
}

// reachedHandler is set to true by recovering a panic that only occurs once
// a request makes it past all middleware and into a TotpHandler method that
// dereferences the (intentionally nil) db pool — i.e. it proves the request
// was NOT stopped by CSRFMiddleware.
func postToTOTP(t *testing.T, path, csrfCookie, csrfHeader string) (status int, reachedHandler bool) {
	t.Helper()
	e := echo.New()
	g := e.Group("/auth")
	// db=nil, masterKey=nil, svc=nil: any handler that gets past CSRF and
	// touches the DB will panic — that panic IS the "reached handler" signal.
	RegisterTOTP(g, nil, nil, noopAuthMW, nil)

	// A body with a non-empty "code" clears every route's own required-field
	// check (Confirm/Disable/Verify/RecoveryLogin/RegenerateRecoveryCodes all
	// bail out with 400 before touching the DB on an empty code) so the
	// request reaches the first DB call — which panics on the nil pool.
	req := httptest.NewRequest(http.MethodPost, "/auth"+path, strings.NewReader(`{"code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	if csrfCookie != "" {
		req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrfCookie})
	}
	if csrfHeader != "" {
		req.Header.Set(CSRFHeaderName, csrfHeader)
	}
	rec := httptest.NewRecorder()

	func() {
		defer func() {
			if r := recover(); r != nil {
				reachedHandler = true
			}
		}()
		e.ServeHTTP(rec, req)
	}()

	return rec.Code, reachedHandler
}

// TestRegenerateRecoveryCodes_RequiresCode is the D24-2 regression guard:
// regenerating recovery codes must demand the caller's current TOTP code,
// not just an authenticated session — otherwise a hijacked session could
// invalidate every existing recovery code without proving possession of the
// authenticator. No DB is needed: the code-required check runs before any
// query.
func TestRegenerateRecoveryCodes_RequiresCode(t *testing.T) {
	h := NewTotpHandler(nil, nil, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/auth/2fa/recovery-codes/regenerate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "00000000-0000-0000-0000-000000000001")

	assert.NoError(t, h.RegenerateRecoveryCodes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "TOTP_BAD_REQUEST")
}

// TestRegisterTOTP_CSRF_S132S11 is the S132-S11/D24-2 regression guard: all
// six 2FA write routes must require a valid X-CSRF-Token before their
// handlers ever run, since RegisterTOTP is mounted directly on api.Group
// ("/auth") in cmd/api/routes.go and does NOT sit under the `protected`
// group's auth.CSRFMiddleware.
func TestRegisterTOTP_CSRF_S132S11(t *testing.T) {
	routes := []string{
		"/2fa/setup",
		"/2fa/confirm",
		"/2fa/disable",
		"/2fa/verify",
		"/2fa/recovery",
		"/2fa/recovery-codes/regenerate",
	}

	for _, path := range routes {
		t.Run(path+"_missing_csrf_403", func(t *testing.T) {
			status, reached := postToTOTP(t, path, "", "")
			assert.Equal(t, http.StatusForbidden, status, "missing CSRF token must 403 before reaching the handler")
			assert.False(t, reached, "handler must not run without a valid CSRF token")
		})

		t.Run(path+"_mismatched_csrf_403", func(t *testing.T) {
			status, reached := postToTOTP(t, path, "cookie-token", "different-header-token")
			assert.Equal(t, http.StatusForbidden, status)
			assert.False(t, reached, "handler must not run when CSRF cookie/header mismatch")
		})

		t.Run(path+"_valid_csrf_reaches_handler", func(t *testing.T) {
			token := GenerateCSRFToken()
			status, reached := postToTOTP(t, path, token, token)
			assert.NotEqual(t, http.StatusForbidden, status, "a valid CSRF token must not be rejected as CSRF")
			assert.True(t, reached, "a valid CSRF token must let the request reach the handler")
		})
	}
}
