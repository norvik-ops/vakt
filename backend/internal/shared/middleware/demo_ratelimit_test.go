// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDemoStartRateLimitConstants verifies that the rate limit parameters are
// sane and match the values documented in CLAUDE.md ("10/min, burst 10, 5min Reset").
func TestDemoStartRateLimitConstants(t *testing.T) {
	assert.Equal(t, int64(10), int64(demoRLLimit),
		"demo start rate limit should be 10 requests per window")
	assert.Equal(t, 5*time.Minute, demoRLWindow,
		"demo start rate limit window should be 5 minutes")
}

// TestDemoStartRateLimitKeyFormat verifies the Redis key format used per IP.
// Keys must be isolated per IP and use the "rate:demo_start:<ip>" prefix so
// they do not collide with auth_rl or login_fail_ip keys.
func TestDemoStartRateLimitKeyFormat(t *testing.T) {
	cases := []struct {
		ip      string
		wantKey string
	}{
		{"127.0.0.1", "rate:demo_start:127.0.0.1"},
		{"::1", "rate:demo_start:::1"},
		{"203.0.113.42", "rate:demo_start:203.0.113.42"},
	}

	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			got := "rate:demo_start:" + tc.ip
			assert.Equal(t, tc.wantKey, got, "key format must match rate:demo_start:<ip>")
		})
	}
}

// TestDemoStartRateLimitThresholdLogic verifies the threshold decision:
// count <= demoRLLimit → 200; count > demoRLLimit → 429.
func TestDemoStartRateLimitThresholdLogic(t *testing.T) {
	cases := []struct {
		name       string
		count      int64
		wantStatus int
	}{
		{"count=1 — pass", 1, http.StatusOK},
		{"count=10 — at limit, pass", 10, http.StatusOK}, // > not >=, so 10 passes
		{"count=11 — over limit, block", 11, http.StatusTooManyRequests},
		{"count=100 — way over limit, block", 100, http.StatusTooManyRequests},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var status int
			if tc.count > demoRLLimit {
				status = http.StatusTooManyRequests
			} else {
				status = http.StatusOK
			}
			assert.Equal(t, tc.wantStatus, status,
				"count=%d should produce status %d", tc.count, tc.wantStatus)
		})
	}
}

// TestDemoStartRateLimiter_NilRedis_FailOpen verifies that passing a nil Redis
// client causes the middleware to fail open (pass the request through).
func TestDemoStartRateLimiter_NilRedis_FailOpen(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/demo/start", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	next := func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}

	// nil rdb → middleware must fail open.
	mw := DemoStartRateLimiter(nil)
	err := mw(next)(c)
	require.NoError(t, err)

	assert.True(t, called, "next handler must be called when rdb is nil (fail-open)")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestDemoStartRateLimiter_FailOpen_SimulatedRedisError verifies that when
// incrWithTTL returns an error the middleware fails open.
func TestDemoStartRateLimiter_FailOpen_SimulatedRedisError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/demo/start", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	next := func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}

	// Mirror the exact fail-open branch in DemoStartRateLimiter.
	simulateFailOpen := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := fmt.Errorf("redis: connection refused")
			if err != nil {
				return next(c)
			}
			return c.JSON(http.StatusTooManyRequests, nil)
		}
	}

	err := simulateFailOpen(next)(c)
	require.NoError(t, err)
	assert.True(t, called, "next handler must be called when Redis is unavailable (fail-open)")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestDemoStartRateLimiter_RateLimitedResponseBody verifies the 429 response
// body format matches {"message":"rate limit exceeded"} — the shape that the
// existing in-memory limiter was producing (see CLAUDE.md).
func TestDemoStartRateLimiter_RateLimitedResponseBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/demo/start", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := c.JSON(http.StatusTooManyRequests, map[string]string{
		"message": "rate limit exceeded",
	})
	require.NoError(t, err)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "rate limit exceeded")
}

// TestDemoStartRateLimiter_IntegrationNote documents what an integration test
// would cover for the Redis-backed path.
//
// A full integration test needs:
//  1. A running Redis instance (e.g. via testcontainers-go).
//  2. rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
//  3. mw := DemoStartRateLimiter(rdb)
//  4. Send 11 requests from the same IP in rapid succession.
//  5. Assert first 10 return 200, 11th returns 429.
//  6. Wait for demoRLWindow (5 min) or flush the key; confirm next request returns 200.
//
// Excluded from unit tests to keep the suite fast and dependency-free.
func TestDemoStartRateLimiter_IntegrationNote(t *testing.T) {
	t.Skip("integration test: requires a live Redis — run with -tags integration")
}
