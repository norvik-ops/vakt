// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Sprint 87 security-hardening regression tests:
//   - S87-2 (F-10): wildcard CORS must fail closed in non-demo mode.
//   - S87-4 (F-08): /health/ready must not leak raw infra error strings.
package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── S87-2: CORS wildcard guard ────────────────────────────────────────────

func TestInsecureWildcardCORS(t *testing.T) {
	cases := []struct {
		name    string
		origins []string
		demo    bool
		want    bool
	}{
		{"wildcard non-demo is insecure", []string{"*"}, false, true},
		{"wildcard demo is allowed", []string{"*"}, true, false},
		{"explicit origin non-demo is fine", []string{"https://vakt.example.com"}, false, false},
		{"explicit origin demo is fine", []string{"https://demo.example.com"}, true, false},
		{"multiple origins incl wildcard is not the single-wildcard case", []string{"*", "https://x"}, false, false},
		{"empty origins is fine", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, insecureWildcardCORS(tc.origins, tc.demo))
		})
	}
}

// ── S87-4: /health/ready error leakage ────────────────────────────────────

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

type fakeRedisPinger struct{ err error }

func (f fakeRedisPinger) Ping(context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background())
	cmd.SetErr(f.err)
	return cmd
}

func callReady(t *testing.T, db readinessDBPinger, rdb readinessRedisPinger) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h := readinessHandler(db, rdb, "1.2.3", zerolog.Nop())
	require.NoError(t, h(c))
	return rec
}

func TestReadinessHandler_DBErrorIsGeneric(t *testing.T) {
	leaky := errors.New("dial tcp 10.0.3.7:5432: connect: connection refused (pgx internal)")
	rec := callReady(t,
		fakePinger{err: leaky},
		fakeRedisPinger{err: nil},
	)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"database unavailable"`)
	assert.Contains(t, body, `"component":"database"`)
	// The raw driver detail must never reach the client.
	assert.NotContains(t, body, "10.0.3.7")
	assert.NotContains(t, body, "pgx internal")
	assert.NotContains(t, body, "connection refused")
}

func TestReadinessHandler_RedisErrorIsGeneric(t *testing.T) {
	leaky := errors.New("dial tcp 10.0.3.9:6379: i/o timeout")
	rec := callReady(t,
		fakePinger{err: nil},
		fakeRedisPinger{err: leaky},
	)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"redis unavailable"`)
	assert.NotContains(t, body, "10.0.3.9")
	assert.NotContains(t, body, "i/o timeout")
}

func TestReadinessHandler_OKWhenBothUp(t *testing.T) {
	rec := callReady(t,
		fakePinger{err: nil},
		fakeRedisPinger{err: nil},
	)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"status":"ready"`)
	assert.Contains(t, body, `"version":"1.2.3"`)
}
