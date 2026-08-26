//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
)

// TestAdminPasswordReset_OrgScopedAndNoTokenLeak closes R1-24-RT01 / R1-10-V02
// (CRITICAL, cross-org account takeover), live-confirmed in SA-24: the admin
// route POST /admin/users/:email/password-reset-token looked up the target by
// email alone (users.email is globally UNIQUE) with only a same-org Admin role
// check, and returned the reset link — WITH the raw token — in the response
// body. An admin of org2 could therefore mint a reset token for the org1 admin
// and take over the account platform-wide.
//
// This drives the REAL auth handler + RegisterAdminRoutes (RequireRole + the
// step-up-MFA mount) against a live Postgres with TWO real orgs and proves the
// whole chain:
//
//   - org2-admin → org1-admin's email  ⇒ 404, and NO reset token is minted for
//     the victim (the org-scoping cuts it off before token generation).
//   - org1-admin → an org1 member       ⇒ 200 {sent:...}, the user IS found
//     (not 404), a token IS minted server-side, and the body carries NO
//     reset_link / no ?token= (the exact defect value).
//
// red-on-revert: drop the org_members predicate in AdminIssuePasswordReset and
// the org2→org1 case flips 404→200, turning this test red.
func TestAdminPasswordReset_OrgScopedAndNoTokenLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	// Two real orgs, each with an Admin member. Cross-org takeover means org2's
	// admin reaching org1's admin account.
	org1 := insertOrg(t, pool, "Org One", "org-one")
	org2 := insertOrg(t, pool, "Org Two", "org-two")

	org1Admin := insertOrgMember(t, pool, org1, "admin@org-one.example", "Admin")
	org1Analyst := insertOrgMember(t, pool, org1, "analyst@org-one.example", "SecurityAnalyst")
	_ = insertOrgMember(t, pool, org2, "admin@org-two.example", "Admin")

	// Build the REAL auth handler and mount the REAL admin routes. A bogus SMTP
	// host drives the token-generation path (so "no token in body" is a
	// meaningful assertion, not vacuous) while delivery fails fast with a
	// connection-refused — the token is minted and stored, never mailed, never
	// returned.
	cfg := &config.Config{
		// 32-byte hex master key so buildMFASensitive mounts the step-up guard.
		SecretKey:   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		FrontendURL: "https://vakt.example",
		SMTPHost:    "127.0.0.1",
		SMTPPort:    "1", // connection refused → delivery fails immediately
		SMTPFrom:    "noreply@vakt.example",
	}
	svc := auth.NewService(pool, nil, mustKeyIntegration(t))
	h := auth.NewHandler(svc, cfg)

	e := echo.New()
	// Stand-in for AuthMiddleware: populate the identity claims the real chain
	// sets (org_id/user_id/roles) from test-controlled headers, so one mounted
	// route can serve both the org1-admin and the org2-admin callers.
	protected := e.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", c.Request().Header.Get("X-Test-Org"))
			c.Set("user_id", c.Request().Header.Get("X-Test-User"))
			if roles := c.Request().Header.Get("X-Test-Roles"); roles != "" {
				c.Set("roles", strings.Split(roles, ","))
			}
			return next(c)
		}
	})
	auth.RegisterAdminRoutes(protected, h)

	call := func(callerOrg, callerUser, targetEmail string) (int, map[string]any) {
		req := httptest.NewRequest(http.MethodPost,
			"/admin/users/"+targetEmail+"/password-reset-token", nil)
		req.Header.Set("X-Test-Org", callerOrg)
		req.Header.Set("X-Test-User", callerUser)
		req.Header.Set("X-Test-Roles", "Admin")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	assertNoTokenLeak := func(t *testing.T, raw string) {
		t.Helper()
		assert.NotContains(t, raw, "reset_link", "response must not carry a reset_link")
		assert.NotContains(t, raw, "?token=", "response must not carry a raw reset token")
		assert.NotContains(t, raw, "/auth/reset-password", "response must not carry the reset URL")
	}

	// --- CROSS-ORG: org2 admin targets org1's admin — the takeover vector. ---
	t.Run("cross-org admin cannot issue a reset for another org's user", func(t *testing.T) {
		code, body := call(org2, "org2-admin-id", "admin@org-one.example")
		require.Equal(t, http.StatusNotFound, code,
			"org2 admin must NOT resolve org1's admin — expected 404")
		assert.Equal(t, "AUTH_USER_NOT_FOUND", body["code"])

		// And crucially: no reset token was minted for the victim.
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM password_reset_tokens WHERE user_id = $1::uuid`,
			org1Admin,
		).Scan(&n))
		assert.Equal(t, 0, n, "no token may be minted for a cross-org victim")
	})

	// --- POSITIVE CONTROL: org1 admin targets an org1 member. ---
	t.Run("same-org admin issues a reset without leaking the token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost,
			"/admin/users/analyst@org-one.example/password-reset-token", nil)
		req.Header.Set("X-Test-Org", org1)
		req.Header.Set("X-Test-User", "org1-admin-id")
		req.Header.Set("X-Test-Roles", "Admin")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code,
			"an in-org target must be found (not 404)")
		raw := rec.Body.String()
		assertNoTokenLeak(t, raw)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &body))
		assert.Equal(t, false, body["sent"], "bogus SMTP → delivery fails, sent=false")

		// The token-generation path WAS reached (a hash was stored) — so the
		// "no token in body" assertion above is proven meaningful.
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM password_reset_tokens WHERE user_id = $1::uuid`,
			org1Analyst,
		).Scan(&n))
		assert.Equal(t, 1, n, "a reset token must be minted for an in-org target")
	})

	// The org1 admin also cannot reach the org2 admin (symmetry).
	t.Run("scoping is symmetric", func(t *testing.T) {
		code, body := call(org1, "org1-admin-id", "admin@org-two.example")
		require.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "AUTH_USER_NOT_FOUND", body["code"])
	})
}

func insertOrg(t *testing.T, pool *pgxpool.Pool, name, slug string) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id::text`,
		name, slug,
	).Scan(&id))
	return id
}

// insertOrgMember creates an active user and maps it into the org with the named
// built-in role (seeded by migration 001). Returns the user id.
func insertOrgMember(t *testing.T, pool *pgxpool.Pool, orgID, email, roleName string) string {
	t.Helper()
	ctx := context.Background()
	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, is_active)
		 VALUES ($1, 'x', TRUE) RETURNING id::text`, email,
	).Scan(&userID))

	var roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles WHERE name = $1`, roleName,
	).Scan(&roleID))

	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)
	return userID
}
