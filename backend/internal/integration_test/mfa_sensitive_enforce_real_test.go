//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// TestMFASensitive_Enforcement is the S131-R-H24 regression guard. Before this
// sprint RequireMFASensitive existed with ZERO mounts (D15-04: the org toggle
// require_mfa_sensitive_calls was stored and echoed back 200, but nothing ever
// read it — security theater). This test wires the REAL middleware against a
// real database, seeds a real AES-256-GCM TOTP secret encrypted with the SAME
// derived key the enrol flow uses (vakt-totp-v1 — the key mismatch that would
// lock every org out is exactly what this proves does NOT happen), and asserts:
//
//   - toggle off        → write passes without a token
//   - toggle on, no tok  → 401 MFA_TOKEN_REQUIRED
//   - toggle on, bad tok → 401 MFA_TOKEN_INVALID
//   - toggle on, good    → write passes
//   - safe method (GET)  → passes regardless (reads never demand a TOTP)
//   - exempt toggle path → passes without a token (break-glass, D15-03)
//   - user w/o TOTP      → 403 MFA_NOT_CONFIGURED (fail closed, not open)
func TestMFASensitive_Enforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Derive the TOTP service key exactly as production does (deriveKey in
	// cmd/api/routes.go → DeriveServiceKey(masterKey, "vakt-totp-v1")).
	master, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	totpKey, err := sharedcrypto.DeriveServiceKey(master, "vakt-totp-v1")
	require.NoError(t, err)

	// Seed a user WITH an enabled TOTP secret, encrypted with totpKey.
	var userWithTOTP string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, is_active)
		VALUES ('mfa-sensitive@acme.test', 'x', 'MFA Sensitive', TRUE) RETURNING id::text`).
		Scan(&userWithTOTP))
	secret, _, err := auth.GenerateTOTPSecret("mfa-sensitive@acme.test", "Vakt")
	require.NoError(t, err)
	ct, err := sharedcrypto.Encrypt(totpKey, []byte(secret))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO totp_secrets (user_id, secret, enabled, backup_codes)
		VALUES ($1::uuid, $2, TRUE, ARRAY[]::text[])`, userWithTOTP, hex.EncodeToString(ct))
	require.NoError(t, err)

	// Seed a second user WITHOUT any TOTP secret.
	var userNoTOTP string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, is_active)
		VALUES ('mfa-none@acme.test', 'x', 'No MFA', TRUE) RETURNING id::text`).
		Scan(&userNoTOTP))

	// Build an Echo whose group carries the REAL middleware. A tiny upstream
	// middleware injects org_id/user_id the way AuthMiddleware would in prod.
	mw := sharedmw.RequireMFASensitive(pool, totpKey, auth.ValidateTOTP)
	newServer := func(userID string) *echo.Echo {
		e := echo.New()
		g := e.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("org_id", orgID)
				c.Set("user_id", userID)
				return next(c)
			}
		}, mw)
		ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
		g.POST("/api-keys", ok)               // sensitive write
		g.GET("/api-keys", ok)                // safe read
		g.PUT("/admin/org/mfa-sensitive", ok) // exempt toggle (break-glass)
		return e
	}

	do := func(e *echo.Echo, method, path, token string) int {
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("X-MFA-Token", token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	setToggle := func(on bool) {
		_, err := pool.Exec(ctx,
			`UPDATE organizations SET require_mfa_sensitive_calls = $2 WHERE id = $1::uuid`, orgID, on)
		require.NoError(t, err)
	}

	srv := newServer(userWithTOTP)

	// ── Toggle OFF: the middleware is a pure pass-through. ──
	setToggle(false)
	require.Equal(t, http.StatusOK, do(srv, http.MethodPost, "/api-keys", ""),
		"toggle off: sensitive write must pass without a token")

	// ── Toggle ON. ──
	setToggle(true)

	require.Equal(t, http.StatusUnauthorized, do(srv, http.MethodPost, "/api-keys", ""),
		"toggle on: write without X-MFA-Token must be 401")
	require.Equal(t, http.StatusUnauthorized, do(srv, http.MethodPost, "/api-keys", "000000"),
		"toggle on: write with a bad code must be 401")

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, do(srv, http.MethodPost, "/api-keys", code),
		"toggle on: write with a valid code must pass")

	require.Equal(t, http.StatusOK, do(srv, http.MethodGet, "/api-keys", ""),
		"toggle on: safe GET must pass without a token")
	require.Equal(t, http.StatusOK, do(srv, http.MethodPut, "/admin/org/mfa-sensitive", ""),
		"toggle on: the toggle-off path must stay reachable (break-glass)")

	// ── User without any TOTP → fail CLOSED, not open. ──
	require.Equal(t, http.StatusForbidden, do(newServer(userNoTOTP), http.MethodPost, "/api-keys", ""),
		"toggle on: a user without TOTP must be blocked (403 MFA_NOT_CONFIGURED), never waved through")
}
