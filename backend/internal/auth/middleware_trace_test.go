// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/auth"
)

func TestTraceMiddleware_SetsResponseHeader(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := auth.TraceMiddleware()
	_ = mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})(c)

	assert.NotEmpty(t, rec.Header().Get("X-Trace-ID"), "X-Trace-ID header must be set")
}

func TestTraceMiddleware_HeaderMatchesContextValue(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var contextID string
	mw := auth.TraceMiddleware()
	_ = mw(func(c echo.Context) error {
		contextID = auth.TraceID(c)
		return c.String(http.StatusOK, "ok")
	})(c)

	assert.NotEmpty(t, contextID)
	assert.Equal(t, rec.Header().Get("X-Trace-ID"), contextID,
		"X-Trace-ID header must match the trace ID set in context")
}

func TestTraceMiddleware_UniquePerRequest(t *testing.T) {
	e := echo.New()
	mw := auth.TraceMiddleware()

	ids := make([]string, 5)
	for i := range ids {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		_ = mw(func(c echo.Context) error {
			ids[i] = auth.TraceID(c)
			return nil
		})(c)
		assert.NotEmpty(t, ids[i])
	}

	seen := make(map[string]bool)
	for _, id := range ids {
		assert.False(t, seen[id], "trace ID collision: %s appeared twice", id)
		seen[id] = true
	}
}

func TestTraceMiddleware_ValidUUID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := auth.TraceMiddleware()
	_ = mw(func(c echo.Context) error { return nil })(c)

	traceID := rec.Header().Get("X-Trace-ID")
	// UUID v4 is 36 chars (8-4-4-4-12 with hyphens)
	assert.Len(t, traceID, 36, "trace ID must be a UUID (36 chars)")
}

func TestTraceID_ReturnsEmptyWhenNotSet(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.Empty(t, auth.TraceID(c), "TraceID must return empty string when TraceMiddleware has not run")
}
