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

	"github.com/matharnica/vakt/internal/auth"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestAPIKey_ReadOnlyScope is the S90-5 regression: an API key whose scope
// carries the ":ro" suffix may call read (GET/HEAD) endpoints of the module
// but is denied (403 AUTH_READONLY_KEY) on every write method, while a normal
// module-scoped key and an admin key keep full write access.
func TestAPIKey_ReadOnlyScope(t *testing.T) {
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

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id::text
	`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('user@acme.test', '$2a$10$abcdefghijklmnopqrstuvwxyz', 'User')
		RETURNING id::text
	`).Scan(&userID))

	// insertKey stores an api_keys row and returns the raw bearer token.
	insertKey := func(name string, scopes []string) string {
		raw := "vakt_" + name + "_secret_value_123456"
		sum := sha256.Sum256([]byte(raw))
		_, err := pool.Exec(ctx, `
			INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix, scopes)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`,
			orgID, userID, name, hex.EncodeToString(sum[:]), raw[:12], scopes)
		require.NoError(t, err)
		return raw
	}

	roKey := insertKey("ro", []string{"vaktcomply:ro"})
	rwKey := insertKey("rw", []string{"vaktcomply"})
	adminKey := insertKey("admin", []string{"admin"})

	key := mustKeyIntegration(t)

	// call runs one request through AuthMiddleware against a 200-returning handler.
	call := func(method, tok string) *httptest.ResponseRecorder {
		e := echo.New()
		req := httptest.NewRequest(method, "/api/v1/vaktcomply/risks", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		h := auth.AuthMiddleware(key, pool)(func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
		})
		require.NoError(t, h(c))
		return rec
	}

	writeMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	// 1. Read-only key: GET allowed, every write method → 403 AUTH_READONLY_KEY.
	getRec := call(http.MethodGet, roKey)
	assert.Equal(t, http.StatusOK, getRec.Code, "read-only key must be allowed on GET")
	for _, m := range writeMethods {
		rec := call(m, roKey)
		assert.Equalf(t, http.StatusForbidden, rec.Code, "read-only key must be denied on %s", m)
		assert.Containsf(t, rec.Body.String(), "AUTH_READONLY_KEY", "%s must return AUTH_READONLY_KEY", m)
	}

	// 2. Normal module key: full write access (regression — unchanged).
	for _, m := range append([]string{http.MethodGet}, writeMethods...) {
		rec := call(m, rwKey)
		assert.Equalf(t, http.StatusOK, rec.Code, "read-write key must be allowed on %s", m)
	}

	// 3. Admin key: full write access (regression — unchanged).
	adminRec := call(http.MethodDelete, adminKey)
	assert.Equal(t, http.StatusOK, adminRec.Code, "admin key must be allowed on write methods")
}
