// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S87-5 (F-07, CWE-614): VAKT_FORCE_SECURE_COOKIES forces the Secure attribute
// on all session/CSRF cookies even when the request lacks TLS / X-Forwarded-Proto.
package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func newCtx(t *testing.T, tlsOn bool, xfp string) echo.Context {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if tlsOn {
		req.TLS = &tls.ConnectionState{}
	}
	if xfp != "" {
		req.Header.Set("X-Forwarded-Proto", xfp)
	}
	return e.NewContext(req, httptest.NewRecorder())
}

func TestCookieSecure_DefaultPlainHTTP(t *testing.T) {
	SetForceSecureCookies(false)
	t.Cleanup(func() { SetForceSecureCookies(false) })
	assert.False(t, CookieSecure(newCtx(t, false, "")),
		"plain HTTP without the force flag must not be Secure")
}

func TestCookieSecure_TLSRequest(t *testing.T) {
	SetForceSecureCookies(false)
	t.Cleanup(func() { SetForceSecureCookies(false) })
	assert.True(t, CookieSecure(newCtx(t, true, "")))
}

func TestCookieSecure_XForwardedProtoHTTPS(t *testing.T) {
	SetForceSecureCookies(false)
	t.Cleanup(func() { SetForceSecureCookies(false) })
	assert.True(t, CookieSecure(newCtx(t, false, "https")))
}

func TestCookieSecure_ForceFlagOverridesPlainHTTP(t *testing.T) {
	SetForceSecureCookies(true)
	t.Cleanup(func() { SetForceSecureCookies(false) })
	assert.True(t, CookieSecure(newCtx(t, false, "")),
		"force flag must mark cookies Secure even without TLS/XFP")
}

// TestSetCSRFCookie_RespectsForceFlag verifies the centralised helper actually
// flows through to the emitted Set-Cookie header.
func TestSetCSRFCookie_RespectsForceFlag(t *testing.T) {
	SetForceSecureCookies(true)
	t.Cleanup(func() { SetForceSecureCookies(false) })

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil) // plain HTTP, no XFP
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	SetCSRFCookie(c, "deadbeef")
	header := strings.Join(rec.Header().Values("Set-Cookie"), "; ")
	assert.Contains(t, header, CSRFCookieName+"=deadbeef")
	assert.Contains(t, header, "Secure", "forced Secure must appear on the CSRF cookie")
}

func TestSetCSRFCookie_DefaultPlainHTTPNotSecure(t *testing.T) {
	SetForceSecureCookies(false)
	t.Cleanup(func() { SetForceSecureCookies(false) })

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	SetCSRFCookie(c, "deadbeef")
	header := strings.Join(rec.Header().Values("Set-Cookie"), "; ")
	assert.NotContains(t, header, "Secure",
		"default plain-HTTP CSRF cookie must not be Secure (unchanged behaviour)")
}
