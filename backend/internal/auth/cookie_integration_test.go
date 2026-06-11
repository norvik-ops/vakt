// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package auth_test contains integration-style tests for the httpOnly-cookie
// authentication flow.  The tests use echo + net/http/httptest without a real
// database or Redis instance so that they remain fast and self-contained.
//
// Covered scenarios:
//  1. A login-style response sets the access_token cookie with HttpOnly flag.
//  2. The same cookie carries SameSite=Strict.
//  3. A logout-style response clears the cookie (MaxAge=-1).
//  4. PasetoMiddleware accepts a valid token presented via cookie.
//  5. A request with neither cookie nor Authorization header gets 401.
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// loginHandler is a minimal echo handler that replicates the cookie-setting
// behaviour of Handler.Login on a successful authentication.  It does not
// touch the database or Redis, which makes it suitable for unit-level cookie
// attribute checks.
func loginHandler(c echo.Context) error {
	secure := c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https"
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    "test-paseto-token",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   3600,
	})
	return c.JSON(http.StatusOK, map[string]string{"access_token": "test-paseto-token"})
}

// logoutHandler is a minimal echo handler that replicates the cookie-clearing
// behaviour of Handler.Logout.
func logoutHandler(c echo.Context) error {
	secure := c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https"
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    "",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   -1,
	})
	return c.JSON(http.StatusOK, map[string]string{"status": "logged out"})
}

// setCookieHeader returns all Set-Cookie header values from a recorded response,
// concatenated into a single string for convenient substring assertions.
func setCookieHeader(rec *httptest.ResponseRecorder) string {
	return strings.Join(rec.Header().Values("Set-Cookie"), "; ")
}

// TestLoginSetsHttpOnlyCookie verifies that a successful login response includes
// the access_token cookie with the HttpOnly attribute set.
func TestLoginSetsHttpOnlyCookie(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := loginHandler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	cookieHeader := setCookieHeader(rec)
	assert.Contains(t, cookieHeader, "access_token=", "cookie name must be present")
	assert.Contains(t, cookieHeader, "HttpOnly", "access_token cookie must be HttpOnly")
}

// TestLoginCookieSameSiteStrict verifies that the access_token cookie carries
// SameSite=Strict to prevent CSRF attacks.
func TestLoginCookieSameSiteStrict(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := loginHandler(c)
	require.NoError(t, err)

	cookieHeader := setCookieHeader(rec)
	assert.Contains(t, cookieHeader, "SameSite=Strict", "access_token cookie must have SameSite=Strict")
}

// TestLogoutClearsCookie verifies that the logout response sets the access_token
// cookie with MaxAge=-1, which instructs browsers to delete it immediately.
func TestLogoutClearsCookie(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := logoutHandler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	cookieHeader := setCookieHeader(rec)
	assert.Contains(t, cookieHeader, "access_token=", "Set-Cookie must reference access_token")
	// net/http serialises MaxAge=-1 as "max-age=0" per RFC 6265.
	// Both representations signal immediate deletion; check for either.
	assert.True(t,
		strings.Contains(cookieHeader, "max-age=0") || strings.Contains(cookieHeader, "Max-Age=0"),
		"logout must clear cookie (max-age=0); got: %s", cookieHeader,
	)
}

// TestMiddlewareAcceptsCookie verifies that PasetoMiddleware correctly reads a
// valid Paseto token from the access_token httpOnly cookie and populates the
// echo context, returning HTTP 200.
func TestMiddlewareAcceptsCookie(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	claims := auth.Claims{
		UserID: "user-cookie-test",
		OrgID:  "org-cookie-test",
		Roles:  []string{"Viewer"},
	}
	tok, err := auth.IssueAccessToken(key, claims)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktcomply/dashboard", nil)
	// Present the token as an httpOnly cookie (no Authorization header).
	req.AddCookie(&http.Cookie{
		Name:  "access_token",
		Value: tok,
	})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var gotUserID, gotOrgID string
	handler := func(c echo.Context) error {
		gotUserID, _ = c.Get("user_id").(string)
		gotOrgID, _ = c.Get("org_id").(string)
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}

	h := auth.PasetoMiddleware(key, nil)(handler)
	err = h(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, claims.UserID, gotUserID, "user_id must be extracted from cookie token")
	assert.Equal(t, claims.OrgID, gotOrgID, "org_id must be extracted from cookie token")
}

// TestMiddlewareRejectsNoCookieNoHeader verifies that PasetoMiddleware returns
// HTTP 401 when neither an Authorization header nor an access_token cookie is
// present in the request.
func TestMiddlewareRejectsNoCookieNoHeader(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktcomply/dashboard", nil)
	// Intentionally no cookie and no Authorization header.
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}

	h := auth.PasetoMiddleware(key, nil)(handler)
	err = h(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "AUTH_MISSING_TOKEN")
}
