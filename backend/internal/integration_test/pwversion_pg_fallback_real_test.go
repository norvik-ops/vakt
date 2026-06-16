//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/auth"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestPwVersion_PGFallbackOnRedisOutage is the S87-6 (F-06) regression: when
// Redis is unreachable, checkPwVersion must consult the durable pw_version in
// PostgreSQL instead of failing open. A token whose pw_version is stale relative
// to the PG value is rejected even during the outage; a current token still
// passes (no lockout of legitimate users).
func TestPwVersion_PGFallbackOnRedisOutage(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
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

	// Seed an org + user whose durable pw_version is 5 (e.g. after 5 resets).
	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id::text
	`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, pw_version)
		VALUES ('user@acme.test', '$2a$10$abcdefghijklmnopqrstuvwxyz', 'User', 5)
		RETURNING id::text
	`).Scan(&userID))

	key := mustKeyIntegration(t)

	// Redis client pointed at a dead port → every call errors (simulated outage).
	deadRedis := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})

	staleTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: userID, OrgID: orgID, Roles: []string{"Admin"}, PwVersion: 0,
	})
	require.NoError(t, err)
	currentTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: userID, OrgID: orgID, Roles: []string{"Admin"}, PwVersion: 5,
	})
	require.NoError(t, err)

	run := func(tok string) *httptest.ResponseRecorder {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktcomply/dashboard", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		h := auth.AuthMiddleware(key, pool, deadRedis)(func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
		})
		require.NoError(t, h(c))
		return rec
	}

	// Stale token: PG fallback (version 5) ≠ token (version 0) → rejected.
	staleRec := run(staleTok)
	assert.Equal(t, http.StatusUnauthorized, staleRec.Code,
		"stale pw_version must be rejected via PG fallback during Redis outage")
	assert.Contains(t, staleRec.Body.String(), "AUTH_SESSION_INVALIDATED")

	// Current token: PG fallback (version 5) == token (version 5) → allowed.
	okRec := run(currentTok)
	assert.Equal(t, http.StatusOK, okRec.Code,
		"current pw_version must pass during Redis outage (no lockout of legitimate users)")
}
