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
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/matharnica/vakt/internal/auth"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestModulePermissionCache is the S90-4 acceptance test: a warm Redis cache
// serves the permission decision without hitting the DB, invalidation makes a
// revocation take effect immediately, and a DB error on a cold cache fails
// closed (503).
func TestModulePermissionCache(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer func() { _ = redisC.Terminate(ctx) }()
	rHost, _ := redisC.Host(ctx)
	rPort, _ := redisC.MappedPort(ctx, "6379/tcp")
	rdb := redis.NewClient(&redis.Options{Addr: rHost + ":" + rPort.Port()})
	defer rdb.Close()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('A','a') RETURNING id::text`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name) VALUES ('u@a.test','x','U') RETURNING id::text`).Scan(&userID))
	// Configured: vaktcomply readable.
	_, err = pool.Exec(ctx, `
		INSERT INTO user_module_permissions (org_id, user_id, module, can_read, can_write)
		VALUES ($1::uuid,$2::uuid,'vaktcomply',true,false)`, orgID, userID)
	require.NoError(t, err)

	// Build the cached middleware around a 200 handler.
	call := func() int {
		mw := auth.RequireModuleAccess(pool, "vaktcomply", rdb)
		h := mw(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/vaktcomply/x", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		c.Set("user_id", userID)
		c.Set("roles", []string{"Viewer"})
		_ = h(c)
		return rec.Code
	}

	// 1) Cold cache → DB read → allowed (200), result cached.
	assert.Equal(t, http.StatusOK, call())

	// 2) Revoke in DB directly, WITHOUT invalidating the cache.
	_, err = pool.Exec(ctx, `UPDATE user_module_permissions SET can_read=false WHERE org_id=$1::uuid AND user_id=$2::uuid`, orgID, userID)
	require.NoError(t, err)
	//    Warm cache still says allowed → proves the DB was not queried.
	assert.Equal(t, http.StatusOK, call(), "warm cache must serve the decision without hitting the DB")

	// 3) Invalidate → next request reads DB → revocation effective immediately (403).
	auth.InvalidateModulePermissions(ctx, rdb, orgID, userID)
	assert.Equal(t, http.StatusForbidden, call(), "after invalidation the revocation must take effect")

	// 4) Fail-closed: cold cache + DB error → 503.
	require.NoError(t, rdb.FlushAll(ctx).Err())
	pool.Close() // force DB errors
	assert.Equal(t, http.StatusServiceUnavailable, call(), "cold cache + DB error must fail closed (503)")
}
