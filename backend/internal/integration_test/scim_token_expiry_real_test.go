//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/services/scim"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestSCIM_ExpiredTokenRejected is the R1-SA21-D9 regression: the SCIM auth
// middleware must reject a token whose expires_at has passed, even though the
// lazy auto-revocation worker (TaskSCIMTokenExpiry) has not yet run and set
// revoked_at. Before the fix the WHERE clause only checked `revoked_at IS NULL`,
// so an expired-but-not-yet-swept token authenticated for up to 24h (or forever
// without a deployed worker).
//
// Non-vacuity: the "active, no expiry" and "future expiry" cases below must pass
// (200) — if the added predicate were wrong it would reject those too.
func TestSCIM_ExpiredTokenRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	orgID := insertOrg(t, pool, "Acme", "acme-scim")

	// insertToken stores a scim_tokens row and returns the raw Bearer value.
	// expiresAt == nil → never expires; revoked marks it revoked_at = NOW().
	insertToken := func(name string, expiresAt *time.Time, revoked bool) string {
		raw := "scim_" + name + "_raw_token_value_123456"
		sum := sha256.Sum256([]byte(raw))
		var revokedAt any
		if revoked {
			revokedAt = time.Now()
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO scim_tokens (org_id, name, token_hash, expires_at, revoked_at)
			VALUES ($1::uuid, $2, $3, $4, $5)`,
			orgID, name, hex.EncodeToString(sum[:]), expiresAt, revokedAt)
		require.NoError(t, err)
		return raw
	}

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	activeTok := insertToken("active", nil, false)
	futureTok := insertToken("future", &future, false)
	expiredTok := insertToken("expired", &past, false)
	revokedTok := insertToken("revoked", nil, true)

	// call runs one request through SCIMAuthMiddleware against a 200 handler.
	call := func(tok string) int {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		h := scim.SCIMAuthMiddleware(pool)(func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
		})
		require.NoError(t, h(c))
		return rec.Code
	}

	// The fix under test: expired token is rejected.
	assert.Equal(t, http.StatusUnauthorized, call(expiredTok),
		"expired SCIM token must be rejected (R1-SA21-D9)")

	// Non-vacuity: valid tokens still authenticate.
	assert.Equal(t, http.StatusOK, call(activeTok),
		"active token without expiry must authenticate")
	assert.Equal(t, http.StatusOK, call(futureTok),
		"token with future expiry must authenticate")

	// Regression: revoked token stays rejected (unchanged behaviour).
	assert.Equal(t, http.StatusUnauthorized, call(revokedTok),
		"revoked SCIM token must stay rejected")
}
