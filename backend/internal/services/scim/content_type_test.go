// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/license"
)

// R1-07-B07-2 — SCIM rejected the media type SCIM prescribes.
//
// RFC 7644 §3.1 fixes application/scim+json, and that is what Entra ID and Okta
// send. Echo's DefaultBinder switches on the Content-Type and only knows
// application/json, so c.Bind failed and every write route answered
// 400 invalidValue "invalid request body". Measured live on 6 of 6 write routes
// (SA-07), each one against both media types.
//
// The test drives all six through the real Register wiring — router group, auth
// middleware, rate limiter, handler, service, database — because the defect was
// not in the handlers but in what reaches them.
func TestSCIMWriteRoutes_acceptTheSCIMMediaType(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

	e := echo.New()
	// SCIM is Pro-gated; license.DBMiddleware does this in production.
	g := e.Group("/scim/v2", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", &license.License{
				Tier:     "pro",
				Features: []string{license.FeatureSCIMProvisioning},
			})
			return next(c)
		}
	})
	Register(g, pool)

	suffix := fmt.Sprintf("%d", os.Getpid())
	userEmail := "scim-ct-" + suffix + "@example.com"
	groupName := "scim-ct-group-" + suffix
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, userEmail)
	})

	const scimJSON = "application/scim+json"

	// POST /Users with the SCIM media type — this is the request Entra ID sends
	// first, and the one that answered 400.
	rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", token, scimJSON, map[string]any{
		"schemas":  []string{schemaUser},
		"userName": userEmail,
		"name":     map[string]string{"givenName": "SCIM", "familyName": "Tester"},
		"active":   true,
	})
	require.Equal(t, http.StatusCreated, rec.Code,
		"POST /Users with %s: %s", scimJSON, rec.Body.String())
	assert.Equal(t, echo.MIMEApplicationJSON, mediaTypeOf(t, rec.Header().Get(echo.HeaderContentType)),
		"the response type stays what openapi.yaml documents; see SCIMMediaTypeMiddleware")
	var created scimUserResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	userID := created.ID

	// PUT /Users/:id
	rec = doSCIM(t, e, http.MethodPut, "/scim/v2/Users/"+userID, token, scimJSON, map[string]any{
		"schemas":     []string{schemaUser},
		"userName":    userEmail,
		"displayName": "SCIM Tester",
		"active":      true,
	})
	assert.Equal(t, http.StatusOK, rec.Code, "PUT /Users/:id: %s", rec.Body.String())

	// PATCH /Users/:id
	rec = doSCIM(t, e, http.MethodPatch, "/scim/v2/Users/"+userID, token, scimJSON, map[string]any{
		"schemas": []string{schemaPatchOp},
		"Operations": []map[string]any{
			{"op": "replace", "path": "displayName", "value": "SCIM Tester 2"},
		},
	})
	assert.Equal(t, http.StatusOK, rec.Code, "PATCH /Users/:id: %s", rec.Body.String())

	// POST /Groups
	rec = doSCIM(t, e, http.MethodPost, "/scim/v2/Groups", token, scimJSON, map[string]any{
		"schemas":     []string{schemaGroup},
		"displayName": groupName,
	})
	require.Equal(t, http.StatusCreated, rec.Code, "POST /Groups: %s", rec.Body.String())
	var createdGroup scimGroupResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createdGroup))
	groupID := createdGroup.ID
	require.NotEmpty(t, groupID)

	// PUT /Groups/:id
	rec = doSCIM(t, e, http.MethodPut, "/scim/v2/Groups/"+groupID, token, scimJSON, map[string]any{
		"schemas":     []string{schemaGroup},
		"displayName": groupName,
	})
	assert.Equal(t, http.StatusOK, rec.Code, "PUT /Groups/:id: %s", rec.Body.String())

	// PATCH /Groups/:id
	rec = doSCIM(t, e, http.MethodPatch, "/scim/v2/Groups/"+groupID, token, scimJSON, map[string]any{
		"schemas": []string{schemaPatchOp},
		"Operations": []map[string]any{
			{"op": "replace", "path": "displayName", "value": groupName},
		},
	})
	assert.Equal(t, http.StatusOK, rec.Code, "PATCH /Groups/:id: %s", rec.Body.String())
}

// The body still has to be JSON. Accepting the SCIM media type must not turn
// into accepting anything at all — a form-encoded body has to keep failing.
func TestSCIMWriteRoutes_rejectNonJSONMediaType(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

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
	Register(g, pool)

	req := httptest.NewRequest(http.MethodPost, "/scim/v2/Users",
		strings.NewReader("userName=who@example.com"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"a form-encoded body must not be provisioned as if it were SCIM")
	var stillThere int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM users WHERE email = 'who@example.com'`).Scan(&stillThere))
	assert.Zero(t, stillThere)
}

// The SCIM token is what authorises these routes, and normalising the media type
// must not move that check. Same request, no token: still 401, not a 400 about
// the body.
func TestSCIMWriteRoutes_scimMediaTypeStillNeedsATokenAndItsOrg(t *testing.T) {
	pool := scimTestDB(t)
	orgA := seedSCIMOrg(t, pool)
	orgB := seedSCIMOrg(t, pool)
	tokenB := seedSCIMToken(t, pool, orgB)

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
	Register(g, pool)

	body := map[string]any{
		"schemas":  []string{schemaUser},
		"userName": fmt.Sprintf("scim-auth-%d@example.com", os.Getpid()),
		"active":   true,
	}

	t.Run("no token", func(t *testing.T) {
		rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", "", "application/scim+json", body)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})

	t.Run("revoked token", func(t *testing.T) {
		revoked := seedSCIMToken(t, pool, orgA)
		sum := sha256.Sum256([]byte(revoked))
		// orgid-lint: global — scoped by token_hash, a globally unique credential
		_, err := pool.Exec(context.Background(),
			`UPDATE scim_tokens SET revoked_at = NOW() WHERE token_hash = $1`,
			hex.EncodeToString(sum[:]))
		require.NoError(t, err)

		rec := doSCIM(t, e, http.MethodPost, "/scim/v2/Users", revoked, "application/scim+json", body)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})

	t.Run("a member of another org stays invisible", func(t *testing.T) {
		// Someone provisioned into org A must not be readable with org B's token,
		// whatever media type the request carries.
		email := fmt.Sprintf("scim-orga-%d@example.com", os.Getpid())
		var userID string
		require.NoError(t, pool.QueryRow(context.Background(),
			`INSERT INTO users (email, role) VALUES ($1, 'viewer') RETURNING id::text`, email,
		).Scan(&userID))
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID)
		})
		_, err := pool.Exec(context.Background(),
			`INSERT INTO org_members (org_id, user_id, role_id) SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Viewer'`,
			orgA, userID)
		require.NoError(t, err)

		rec := doSCIM(t, e, http.MethodPut, "/scim/v2/Users/"+userID, tokenB, "application/scim+json", map[string]any{
			"schemas":  []string{schemaUser},
			"userName": email,
			"active":   false,
		})
		assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func mediaTypeOf(t *testing.T, header string) string {
	t.Helper()
	mediaType, _, err := mime.ParseMediaType(header)
	require.NoError(t, err, "unparsable Content-Type %q", header)
	return mediaType
}

func doSCIM(t *testing.T, e *echo.Echo, method, path, token, contentType string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func scimTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — the SCIM route tests need a migrated Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

var scimSeq int

func seedSCIMOrg(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	scimSeq++
	slug := fmt.Sprintf("scim-ct-%d-%d", os.Getpid(), scimSeq)
	var orgID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug,
	).Scan(&orgID))
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN (SELECT user_id FROM org_members WHERE org_id = $1::uuid)`, orgID)
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})
	return orgID
}

func seedSCIMToken(t *testing.T, pool *pgxpool.Pool, orgID string) string {
	t.Helper()
	scimSeq++
	token := fmt.Sprintf("scim-token-%d-%d", os.Getpid(), scimSeq)
	sum := sha256.Sum256([]byte(token))
	_, err := pool.Exec(context.Background(),
		`INSERT INTO scim_tokens (org_id, name, token_hash) VALUES ($1::uuid, $2, $3)`,
		orgID, "test", hex.EncodeToString(sum[:]))
	require.NoError(t, err)
	return token
}
