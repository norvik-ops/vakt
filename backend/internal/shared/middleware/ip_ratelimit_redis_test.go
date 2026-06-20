// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestIPRateLimitRedis_NilClient_FailOpen verifies that a nil Redis client
// allows requests through when failClosed=false (auth-path default).
func TestIPRateLimitRedis_NilClient_FailOpen(t *testing.T) {
	e := echo.New()
	called := false
	mw := IPRateLimitRedis(nil, "test", 5, 0, false)
	h := mw(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	_ = h(e.NewContext(req, rec))
	assert.True(t, called, "nil client with failClosed=false must pass request through")
}

// TestIPRateLimitRedis_NilClient_FailClosed verifies that a nil Redis client
// returns 503 when failClosed=true (public, abuse-sensitive endpoints).
func TestIPRateLimitRedis_NilClient_FailClosed(t *testing.T) {
	e := echo.New()
	called := false
	mw := IPRateLimitRedis(nil, "test", 5, 0, true)
	h := mw(func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	_ = h(e.NewContext(req, rec))
	assert.False(t, called, "nil client with failClosed=true must NOT pass request through")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
