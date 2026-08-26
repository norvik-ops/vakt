// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// R1-21-A06, the half that can be closed without logging the caller out.
//
// DELETE /auth/sessions without X-Vakt-Session-Id is the panic button: it wipes
// EVERY session including the caller's own. Deleting the refresh rows alone left
// the access tokens already handed out valid for the rest of their hour — they
// are stateless Paseto, and only a pw_version bump makes the middleware reject
// them. "Revoke everything" has to mean the access is gone, not merely that it
// cannot be renewed.
//
// The other two paths keep the caller's session on purpose, and pw_version is
// per-user, so bumping there would log the caller out (the frontend answers 401
// with a hard logout, api/client.ts). That part needs a per-session claim in the
// token and is reported open, not claimed here.
func TestRevokeAllSessions_panicButtonInvalidatesTheAccessToken(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "panic-button-" + suffix + "@example.com"

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, role, is_active)
		VALUES ($1, 'Panic Button', 'viewer', TRUE)
		RETURNING id::text`, email).Scan(&userID))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID) })

	var before int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&before))

	rec := serveRevokeAll(t, pool, userID, "" /* no current-session header = panic button */)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var after int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&after))

	assert.Greater(t, after, before,
		"pw_version did not move — every access token minted before the panic button stays valid "+
			"for the rest of its hour, and the middleware has no other way to reject a stateless Paseto")
}

// The "keep my own session" path must NOT bump: it would invalidate the caller's
// own access token, and the frontend turns that 401 into a logout. This pins the
// deliberate limit so nobody later "fixes" it into a regression without seeing
// the trade.
func TestRevokeAllSessions_keepingTheCurrentSessionDoesNotLogTheCallerOut(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "keep-current-" + suffix + "@example.com"

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, role, is_active)
		VALUES ($1, 'Keep Current', 'viewer', TRUE)
		RETURNING id::text`, email).Scan(&userID))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID) })

	var before int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&before))

	rec := serveRevokeAll(t, pool, userID, "00000000-0000-0000-0000-000000000001")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var after int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&after))
	assert.Equal(t, before, after,
		"pw_version was bumped on the path that keeps the caller's session — the caller is now logged out")
}

func serveRevokeAll(t *testing.T, pool *pgxpool.Pool, userID, currentSessionID string) *httptest.ResponseRecorder {
	t.Helper()
	h := auth.NewSessionHandler(pool, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions", nil)
	if currentSessionID != "" {
		req.Header.Set("X-Vakt-Session-Id", currentSessionID)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", userID)
	require.NoError(t, h.RevokeAllOtherSessions(c))
	return rec
}
