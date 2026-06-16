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
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/license"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// TestMiddlewareChain_EndToEnd is the S90-9 acceptance test: it wires the full
// pre-handler middleware stack of the `protected` group on a real Echo router
// (Auth → CSRF → MFA-Enforce → License → Org-Rate-Limit → Module-Permission)
// against a Testcontainer Postgres + Redis, and verifies the ORDERING and
// INTERPLAY of the stages end-to-end — not just each stage in isolation.
//
// Scenarios:
//
//	(a) valid Paseto token + matching CSRF cookie/header on POST → 200
//	(b) missing CSRF header on POST → 403 CSRF_HEADER_MISSING
//	(c) API key (Bearer vakt_) bypasses CSRF *and* MFA → 200 even with require_mfa
//	(d) org require_mfa + user without TOTP → 403 MFA_REQUIRED on a normal route,
//	    but 200 on an MFA-exempt path (/api/v1/auth/me)
//	(e) DB outage → the chain fails closed end-to-end with 503
func TestMiddlewareChain_EndToEnd(t *testing.T) {
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
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id::text
	`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('user@acme.test', '$2a$10$abcdefghijklmnopqrstuvwxyz', 'User')
		RETURNING id::text
	`).Scan(&userID))

	// API key (module scope) for scenario (c).
	rawAPIKey := "vakt_chain_secret_value_123456"
	apiSum := sha256.Sum256([]byte(rawAPIKey))
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix, scopes)
		VALUES ($1::uuid, $2::uuid, 'chain', $3, $4, $5)`,
		orgID, userID, hex.EncodeToString(apiSum[:]), rawAPIKey[:12], []string{"vaktcomply"})
	require.NoError(t, err)

	key := mustKeyIntegration(t)
	userTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: userID, OrgID: orgID, Roles: []string{"SecurityAnalyst"}, PwVersion: 0,
	})
	require.NoError(t, err)

	// Build the fully-wired protected group exactly like cmd/api/main.go.
	lic := license.Load("", false)
	e := echo.New()
	api := e.Group("/api/v1")
	protected := api.Group("", auth.AuthMiddleware(key, pool, rdb))
	protected.Use(auth.CSRFMiddleware("/api/v1/webhooks/receive"))
	protected.Use(auth.MFAEnforceMiddleware(pool))
	protected.Use(license.DBMiddleware(pool, lic, rdb))
	protected.Use(sharedmw.OrgRateLimitRedis(rdb))

	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	// MFA-exempt path (no module-permission gate — matches real wiring).
	protected.GET("/auth/me", ok)
	// Module-gated routes.
	mod := protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb))
	mod.GET("/x", ok)
	mod.POST("/x", ok)

	const csrfVal = "csrf-token-value-abc123"
	type opt struct {
		bearer  string
		csrf    bool // attach matching csrf cookie + header
		csrfHdr bool // attach header only (cookie always set when csrf true)
	}
	do := func(method, path string, o opt) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if o.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+o.bearer)
		}
		if o.csrf {
			req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfVal})
			req.Header.Set(auth.CSRFHeaderName, csrfVal)
		} else if o.csrfHdr {
			req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfVal})
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	// (a) valid token + CSRF → 200.
	recA := do(http.MethodPost, "/api/v1/vaktcomply/x", opt{bearer: userTok, csrf: true})
	assert.Equal(t, http.StatusOK, recA.Code, "valid token + CSRF must pass the whole chain")

	// (b) missing CSRF header (cookie present, header absent) on POST → 403.
	recB := do(http.MethodPost, "/api/v1/vaktcomply/x", opt{bearer: userTok, csrfHdr: true})
	assert.Equal(t, http.StatusForbidden, recB.Code, "missing CSRF header must be rejected")
	assert.Contains(t, recB.Body.String(), "CSRF", "403 must come from the CSRF stage")

	// (c) API key bypasses CSRF + MFA → 200 even with require_mfa enabled.
	_, err = pool.Exec(ctx, `UPDATE organizations SET require_mfa = true WHERE id = $1::uuid`, orgID)
	require.NoError(t, err)
	recC := do(http.MethodPost, "/api/v1/vaktcomply/x", opt{bearer: rawAPIKey})
	assert.Equal(t, http.StatusOK, recC.Code, "api key must bypass CSRF and MFA")

	// (d) require_mfa + user without TOTP → 403 on a normal route…
	recD := do(http.MethodGet, "/api/v1/vaktcomply/x", opt{bearer: userTok})
	assert.Equal(t, http.StatusForbidden, recD.Code, "MFA-required user without TOTP must be blocked")
	assert.Contains(t, recD.Body.String(), "MFA_REQUIRED")
	//     …but 200 on the MFA-exempt /auth/me path.
	recDExempt := do(http.MethodGet, "/api/v1/auth/me", opt{bearer: userTok})
	assert.Equal(t, http.StatusOK, recDExempt.Code, "MFA-exempt path must remain reachable")

	// (e) DB outage → the chain fails closed end-to-end (503). Closing the pool
	//     makes the first DB-dependent stage (MFA-enforce) fail closed; the
	//     dedicated permission-stage fail-closed path is covered by
	//     TestModulePermissionCache. Reset require_mfa first so we also prove the
	//     chain never silently allows a request it cannot fully authorise.
	_, err = pool.Exec(ctx, `UPDATE organizations SET require_mfa = false WHERE id = $1::uuid`, orgID)
	require.NoError(t, err)
	require.NoError(t, rdb.FlushAll(ctx).Err())
	pool.Close()
	recE := do(http.MethodGet, "/api/v1/vaktcomply/x", opt{bearer: userTok})
	assert.Equal(t, http.StatusServiceUnavailable, recE.Code,
		"a DB outage must make the protected chain fail closed (503), never allow")
}
