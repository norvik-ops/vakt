// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scim

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/license"
)

// R1-14cA-02 / R1-24-RT03 — offboarding did not end the running session.
//
// sessionRevoker was called in the DELETE branch only. SCIM PATCH and PUT with
// active=false answered 200 and set is_active=false, and the session that was
// already running kept writing: SA-14c-A measured POST /vaktscan/assets -> 201
// with the row present in vb_assets afterwards, SA-24 measured the offboarded
// viewer still reading /auth/me and /vaktcomply/risks. PATCH and PUT are exactly
// the two verbs Entra ID and Okta send on "disable".
//
// The revocation itself works through pw_version: the access token is stateless,
// and bumpPwVersion is what makes every later request carrying it fail
// (auth.RevokeAllSessions). So this asserts the DB effect the revoker leaves
// behind, and the chain test below drives the token through the real middleware.

const testHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

func TestSCIMDeactivation_revokesTheRunningSession(t *testing.T) {
	pool := scimTestDB(t)

	cases := []struct {
		name   string
		method string
		body   any
	}{
		{
			name:   "PATCH active=false — what Entra ID and Okta send on disable",
			method: http.MethodPatch,
			body: map[string]any{
				"schemas": []string{schemaPatchOp},
				"Operations": []map[string]any{
					{"op": "replace", "path": "active", "value": false},
				},
			},
		},
		{
			name:   "PUT active=false",
			method: http.MethodPut,
			body: map[string]any{
				"schemas": []string{schemaUser},
				"active":  false,
			},
		},
		{
			name:   "DELETE — the branch that always revoked",
			method: http.MethodDelete,
			body:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orgID := seedSCIMOrg(t, pool)
			token := seedSCIMToken(t, pool, orgID)
			e, _ := scimServer(t, pool)

			email := fmt.Sprintf("offboard-%d-%d@example.com", os.Getpid(), scimSeq)
			userID := provisionSCIMUser(t, pool, e, token, email)
			sessionsBefore := seedSession(t, pool, orgID, userID)
			require.Equal(t, 1, sessionsBefore)
			pwVersionBefore := pwVersion(t, pool, userID)

			body := tc.body
			if tc.method == http.MethodPut {
				body = map[string]any{
					"schemas":  []string{schemaUser},
					"userName": email,
					"active":   false,
				}
			}
			rec := doSCIM(t, e, tc.method, "/scim/v2/Users/"+userID, token, MediaTypeSCIMJSON, body)
			require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, rec.Code, rec.Body.String())

			assert.False(t, isActive(t, pool, userID), "the account is still active")
			assert.Zero(t, sessionCount(t, pool, userID),
				"the refresh session survived the offboarding")
			assert.Greater(t, pwVersion(t, pool, userID), pwVersionBefore,
				"pw_version was not bumped — the access token the offboarded user is holding stays valid until it expires")
		})
	}
}

// A SCIM write that does NOT deactivate must not throw the user out. Otherwise
// the fix would turn every attribute sync from the IdP into a forced re-login.
func TestSCIMUpdate_withoutDeactivationKeepsTheSession(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)
	e, _ := scimServer(t, pool)

	email := fmt.Sprintf("stay-%d-%d@example.com", os.Getpid(), scimSeq)
	userID := provisionSCIMUser(t, pool, e, token, email)
	seedSession(t, pool, orgID, userID)
	pwVersionBefore := pwVersion(t, pool, userID)

	rec := doSCIM(t, e, http.MethodPatch, "/scim/v2/Users/"+userID, token, MediaTypeSCIMJSON, map[string]any{
		"schemas": []string{schemaPatchOp},
		"Operations": []map[string]any{
			{"op": "replace", "path": "displayName", "value": "Renamed By IdP"},
		},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.True(t, isActive(t, pool, userID))
	assert.Equal(t, 1, sessionCount(t, pool, userID), "a plain attribute sync logged the user out")
	assert.Equal(t, pwVersionBefore, pwVersion(t, pool, userID))
}

// The chain R1-24-RT03 measured live: a request that worked before the
// offboarding has to fail after it — through the real auth middleware, with the
// same access token.
func TestSCIMDeactivation_theRunningAccessTokenStopsWorking(t *testing.T) {
	pool := scimTestDB(t)
	rdb := scimTestRedis(t)

	orgID := seedSCIMOrg(t, pool)
	scimToken := seedSCIMToken(t, pool, orgID)
	e, _ := scimServer(t, pool)

	email := fmt.Sprintf("chain-%d-%d@example.com", os.Getpid(), scimSeq)
	userID := provisionSCIMUser(t, pool, e, scimToken, email)
	seedSession(t, pool, orgID, userID)

	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	accessToken, err := auth.IssueAccessToken(key, auth.Claims{
		UserID:    userID,
		OrgID:     orgID,
		Roles:     []string{"Viewer"},
		PwVersion: int64(pwVersion(t, pool, userID)),
	})
	require.NoError(t, err)

	// A protected route, guarded the way the product guards its modules.
	e.GET("/probe", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}, auth.AuthMiddleware(key, pool, rdb))

	probe := func() int {
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	require.Equal(t, http.StatusOK, probe(), "the session has to work before the offboarding")

	rec := doSCIM(t, e, http.MethodPatch, "/scim/v2/Users/"+userID, scimToken, MediaTypeSCIMJSON, map[string]any{
		"schemas": []string{schemaPatchOp},
		"Operations": []map[string]any{
			{"op": "replace", "path": "active", "value": false},
		},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, http.StatusUnauthorized, probe(),
		"the offboarded user kept reading with the access token they already held")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// scimServer mounts the SCIM routes with the real auth.Service as the session
// revoker — the same wiring cmd/api/routes.go uses.
func scimServer(t *testing.T, pool *pgxpool.Pool) (*echo.Echo, *auth.Service) {
	t.Helper()
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	authSvc := auth.NewService(pool, nil, key)

	e := echo.New()
	g := e.Group("/scim/v2", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", &license.License{
				Tier:     "pro",
				Features: []string{license.FeatureSCIMProvisioning},
			})
			return next(c)
		}
	})
	Register(g, pool, authSvc)
	return e, authSvc
}

func scimTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("VAKT_REDIS_URL")
	if url == "" {
		t.Skip("VAKT_REDIS_URL not set — the middleware only checks pw_version with Redis present")
	}
	opts, err := redis.ParseURL(url)
	require.NoError(t, err)
	client := redis.NewClient(opts)
	require.NoError(t, client.Ping(context.Background()).Err())
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func provisionSCIMUser(t *testing.T, pool *pgxpool.Pool, e *echo.Echo, token, email string) string {
	t.Helper()
	rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", token, MediaTypeSCIMJSON, map[string]any{
		"schemas":  []string{schemaUser},
		"userName": email,
		"active":   true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var userID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id::text FROM users WHERE email = $1`, email).Scan(&userID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID)
	})
	return userID
}

func seedSession(t *testing.T, pool *pgxpool.Pool, orgID, userID string) int {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO refresh_sessions (user_id, org_id, token_hash, device_hint, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, 'go-test', NOW() + INTERVAL '30 days')`,
		userID, orgID, fmt.Sprintf("hash-%s", userID))
	require.NoError(t, err)
	return sessionCount(t, pool, userID)
}

func sessionCount(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	// orgid-lint: global — scoped by user_id; revocation is deliberately cross-org,
	// same as auth.RevokeAllSessions
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM refresh_sessions WHERE user_id = $1::uuid`, userID).Scan(&n))
	return n
}

func pwVersion(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var v int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT pw_version FROM users WHERE id = $1::uuid`, userID).Scan(&v))
	return v
}

func isActive(t *testing.T, pool *pgxpool.Pool, userID string) bool {
	t.Helper()
	var active bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT is_active FROM users WHERE id = $1::uuid`, userID).Scan(&active))
	return active
}
