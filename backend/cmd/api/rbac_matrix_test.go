// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
)

// TestRBACMatrix_RequireAdminSurface is the S132 Spur A / G2 dynamic gate for the
// D24-1 split-brain (findings D24-1 CRIT + V24-a). It runs the REAL setupEcho()
// router and asserts, on the admin user-management surface, that a Viewer is 403
// and an Admin is not.
//
// Why the real tree, and why this surface specifically:
//
//	usermgmt.requireAdmin gates GET /admin/users, the role change, delete,
//	MFA-break-glass and the invitation routes by reading users.role (the
//	denormalised cache), NOT the org_members role the rest of the app authorises
//	against. That second source defaulted to 'admin' (migration 077), so every
//	user whose insert forgot to set the column was an admin here regardless of
//	their real Viewer membership — the escalation. requireAdmin lives at the mount
//	point, so a rebuilt router (as in internal/rbaccov) does NOT carry it: only the
//	genuine setupEcho() tree can exercise it. That is the gap this test fills;
//	rbaccov owns the comprehensive deny-by-default sweep over the module / shared /
//	integration write surface, which this test deliberately does NOT duplicate.
//
// Each of the seven routes distinguishes the fix from the bug: with the fix a
// Viewer's users.role is 'viewer' and requireAdmin answers 403; without it the
// Viewer's users.role would default to 'admin', requireAdmin would pass, and the
// handler would answer something other than 403. Write routes are sent WITH valid
// double-submit CSRF credentials on purpose — otherwise CSRF would 403 the Viewer
// before requireAdmin runs, and the test would pass for the wrong reason (and the
// Admin control below would falsely fail). The Admin control proves the test is
// not vacuously green by rejecting everyone.
func TestRBACMatrix_RequireAdminSurface(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	viewerTok := seedUserWithRole(ctx, t, pool, secret, "Viewer", "viewer")
	adminTok := seedUserWithRole(ctx, t, pool, secret, "Admin", "admin")
	e, _ := setupEcho(ctx, testConfig())

	// A dummy UUID for the parameterised routes: well-formed so it clears
	// ValidateUUIDParams and reaches requireAdmin (a malformed one would 400 first
	// and never exercise the RBAC layer). It resolves to no row, which is fine —
	// requireAdmin runs before the handler, so the Viewer is rejected regardless.
	const dummyID = "00000000-0000-0000-0000-000000000000"

	// The full requireAdmin-gated surface (usermgmt.RegisterRoutes). GETs bypass
	// CSRF (safe methods); writes carry CSRF creds so requireAdmin is the deciding
	// layer.
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPatch, "/api/v1/admin/users/" + dummyID + "/role"},
		{http.MethodDelete, "/api/v1/admin/users/" + dummyID},
		{http.MethodPost, "/api/v1/admin/users/" + dummyID + "/reset-mfa"},
		{http.MethodGet, "/api/v1/admin/invitations"},
		{http.MethodPost, "/api/v1/admin/invitations"},
		{http.MethodDelete, "/api/v1/admin/invitations/" + dummyID},
	}

	for _, r := range routes {
		r := r
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			// Viewer MUST be forbidden.
			vRec := doRBACReq(e, r.method, r.path, viewerTok)
			assert.Equal(t, http.StatusForbidden, vRec.Code,
				"Viewer must get 403 on %s %s — a non-403 means requireAdmin authorised a "+
					"Viewer, i.e. users.role was not 'viewer' (D24-1 split-brain). Body: %s",
				r.method, r.path, vRec.Body.String())

			// Admin MUST clear the RBAC layer (any non-403 is fine; the handler may
			// 4xx on the dummy id / empty body). This is the non-vacuity control: a
			// test that 403s everyone would pass the Viewer assertion but fail here.
			aRec := doRBACReq(e, r.method, r.path, adminTok)
			assert.NotEqual(t, http.StatusForbidden, aRec.Code,
				"Admin must NOT get 403 on %s %s — requireAdmin is rejecting a real Admin, "+
					"so the Viewer 403 above proves nothing. Body: %s",
				r.method, r.path, aRec.Body.String())
		})
	}
}

// doRBACReq issues a request with a bearer token and — for unsafe methods — a
// matching double-submit CSRF cookie+header pair so the request clears
// CSRFMiddleware and reaches the RBAC layer.
func doRBACReq(e http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		const csrf = "test-csrf-token"
		req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
		req.Header.Set(auth.CSRFHeaderName, csrf)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// seedUserWithRole creates an org + a user with the given org_members platform role
// (roles.name) AND the given users.role value, then mints an access token whose
// roles claim matches. Setting users.role explicitly is the whole point: it lets
// the test seed a genuine Viewer (org_members Viewer + users.role='viewer') and a
// genuine Admin, so requireAdmin — which reads users.role — is exercised for real.
func seedUserWithRole(ctx context.Context, t *testing.T, pool *pgxpool.Pool, secret, platformRole, usersRole string) string {
	t.Helper()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug)
		 VALUES ('RBAC Matrix', 'rbac-matrix-'||substr(md5(random()::text),1,8))
		 RETURNING id::text`).Scan(&orgID), "seed org")

	var userID string
	var pwVersion int64
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, role)
		 VALUES ('rbac-'||substr(md5(random()::text),1,8)||'@test.local', $1)
		 RETURNING id::text, pw_version`, usersRole).Scan(&userID, &pwVersion), "seed user")

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id)
		 SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = $3 LIMIT 1
		 RETURNING user_id::text`,
		orgID, userID, platformRole).Scan(&userID), "seed membership")

	raw, err := hex.DecodeString(secret)
	require.NoError(t, err, "decode master key")
	keyBytes, err := sharedcrypto.DeriveServiceKey(raw, "vakt-paseto-v1")
	require.NoError(t, err, "derive paseto key")
	key, err := auth.GenerateSymmetricKeyFromBytes(keyBytes)
	require.NoError(t, err, "paseto key")

	token, err := auth.IssueAccessToken(key, auth.Claims{
		UserID:    userID,
		OrgID:     orgID,
		Roles:     []string{platformRole},
		PwVersion: pwVersion,
		MFA:       true,
	})
	require.NoError(t, err, "issue token")
	return token
}
