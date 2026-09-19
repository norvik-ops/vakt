// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package lexware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// TestWebhookRateLimiterRejectsOverLimit is the regression guard for R1-SA10-V4.
// The Lexware webhook was mounted without any rate limit, so each call could
// spawn an outbound Lexware API call. The limiter caps a burst per source IP:
// the first Burst requests pass, the next is refused with 429 before it can
// reach the handler (and thus before it can trigger an outbound call).
func TestWebhookRateLimiterRejectsOverLimit(t *testing.T) {
	const burst = 10

	e := echo.New()
	e.POST("/billing/lexware/webhook", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, newWebhookRateLimiter())

	fire := func() int {
		req := httptest.NewRequest(http.MethodPost, "/billing/lexware/webhook", nil)
		req.RemoteAddr = "203.0.113.7:5555" // same source IP for every call
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < burst; i++ {
		require.Equal(t, http.StatusOK, fire(), "request %d within burst must pass", i+1)
	}

	require.Equal(t, http.StatusTooManyRequests, fire(),
		"request beyond the burst must be refused with 429 before reaching the handler (R1-SA10-V4)")
}
