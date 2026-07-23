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

// passThrough is a tiny next handler that returns 200 OK if the middleware
// allows the request to proceed.
func passThrough(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

// TestCSRF_SafeMethodsBypass — GET/HEAD/OPTIONS never need a CSRF token.
func TestCSRF_SafeMethodsBypass(t *testing.T) {
	mw := CSRFMiddleware()
	e := echo.New()

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		req := httptest.NewRequest(method, "/api/v1/anything", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := mw(passThrough)(c); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		assert.Equal(t, http.StatusOK, rec.Code, "method %s should bypass CSRF", method)
	}
}

// TestCSRF_APIKeyBypass — API-Key auth bypasses CSRF because the request is
// not browser-driven and the key itself authenticates the caller.
func TestCSRF_APIKeyBypass(t *testing.T) {
	mw := CSRFMiddleware()
	e := echo.New()

	for _, prefix := range []string{"sk_", "vakt_"} {
		req := httptest.NewRequest("POST", "/api/v1/anything", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+prefix+"deadbeef")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := mw(passThrough)(c); err != nil {
			t.Fatalf("%s: %v", prefix, err)
		}
		assert.Equal(t, http.StatusOK, rec.Code, "API key prefix %s should bypass CSRF", prefix)
	}
}

// TestCSRF_ExemptPath — explicitly-listed paths (webhooks, OAuth callbacks)
// are exempted by prefix.
func TestCSRF_ExemptPath(t *testing.T) {
	mw := CSRFMiddleware("/api/v1/webhooks/receive")
	e := echo.New()

	// The EXACT exempt path bypasses CSRF.
	req := httptest.NewRequest("POST", "/api/v1/webhooks/receive", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := mw(passThrough)(c); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, http.StatusOK, rec.Code, "exact exempt path must bypass CSRF")
}

// TestCSRF_ExemptPath_NoPrefixOvermatch is the S131-R-L05 regression guard: a
// path that merely has the exempt path as a PREFIX must NOT bypass CSRF. The old
// strings.HasPrefix matcher would have exempted /receive/abc as well — a latent
// trap where a future sibling route silently loses CSRF protection.
func TestCSRF_ExemptPath_NoPrefixOvermatch(t *testing.T) {
	mw := CSRFMiddleware("/api/v1/webhooks/receive")
	e := echo.New()

	req := httptest.NewRequest("POST", "/api/v1/webhooks/receive/abc", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := mw(passThrough)(c); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a sibling of the exempt path must still be CSRF-protected (no prefix over-match)")
}

// TestCSRF_MissingTokens — POST without cookie or header is 403.
func TestCSRF_MissingTokens(t *testing.T) {
	mw := CSRFMiddleware()
	e := echo.New()

	req := httptest.NewRequest("POST", "/api/v1/foo", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := mw(passThrough)(c); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "CSRF_MISSING")
}

// TestCSRF_HeaderMismatch — cookie present but X-CSRF-Token doesn't match → 403.
func TestCSRF_HeaderMismatch(t *testing.T) {
	mw := CSRFMiddleware()
	e := echo.New()

	req := httptest.NewRequest("POST", "/api/v1/foo", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "expected-token"})
	req.Header.Set(CSRFHeaderName, "different-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := mw(passThrough)(c); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "CSRF_MISMATCH")
}

// TestCSRF_HappyPath — matching cookie and header passes.
func TestCSRF_HappyPath(t *testing.T) {
	mw := CSRFMiddleware()
	e := echo.New()
	token := GenerateCSRFToken()
	assert.NotEmpty(t, token)

	req := httptest.NewRequest("POST", "/api/v1/foo", strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	req.Header.Set(CSRFHeaderName, token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := mw(passThrough)(c); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestGenerateCSRFToken_Unique — every call returns a fresh token.
func TestGenerateCSRFToken_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := GenerateCSRFToken()
		assert.Len(t, token, 64, "hex-encoded 32 bytes = 64 chars")
		assert.False(t, seen[token], "token collision after %d generations", i)
		seen[token] = true
	}
}
