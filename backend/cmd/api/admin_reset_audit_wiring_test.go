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

// POST /admin/users/:email/password-reset-token is break-glass: one admin
// obtains a password reset for another member. The audit entry for it was fully
// built (auth.AdminResetAuditEntry + the injection seam that works around the
// auth<->audit import cycle) but nobody ever called SetAdminResetAuditWriter, so
// the sink stayed nil and the handler's `if adminResetAuditFn != nil` quietly
// skipped it. Nothing failed, nothing logged — the reset worked and left no
// trace, which is the worst shape a missing audit entry can take.
//
// A unit test on the auth package cannot see this: the seam is nil there BY
// DESIGN, and auth cannot import audit. The wiring exists only in the
// composition root, so the test has to go through the real route on the real
// tree from setupEcho() and then look in the actual audit_log table.
func TestAdminPasswordResetIsAudited(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	orgID, adminToken := seedOrgWithAdminToken(ctx, t, pool, secret)
	targetID, targetEmail := seedOrgMember(ctx, t, pool, orgID)

	e, _ := setupEcho(ctx, testConfig())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+targetEmail+"/password-reset-token", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	// Double-submit CSRF: cookie and header must carry the same value, otherwise
	// the request dies at 403 long before the handler and the test would prove
	// nothing about the audit wiring.
	const csrf = "test-csrf-token"
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code,
		"the reset itself must succeed — otherwise this test is measuring the wrong failure. Body: %s", rec.Body.String())

	var count int
	var actor, resourceID string
	err = pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(max(user_id::text), ''), COALESCE(max(resource_id), '')
		FROM audit_log
		WHERE org_id = $1::uuid AND action = 'admin_password_reset'`, orgID).Scan(&count, &actor, &resourceID)
	require.NoError(t, err, "read audit_log")

	assert.Equal(t, 1, count,
		"an admin-issued password reset left no audit entry — auth.SetAdminResetAuditWriter is not wired in "+
			"cmd/api/routes.go. The seam defaults to nil, so this fails silently: the reset still works.")
	assert.Equal(t, targetID, resourceID, "the entry must name the user whose password was reset")
	assert.NotEmpty(t, actor, "the entry must name the admin who issued the reset")
}

// seedOrgWithAdminToken mirrors seedOrgAndToken (openapi_contract_auth_test.go)
// but also hands back the org id — the audit assertion has to scope its query to
// this run's org, or a leftover row from another test would make it pass.
func seedOrgWithAdminToken(ctx context.Context, t *testing.T, pool *pgxpool.Pool, secret string) (orgID, token string) {
	t.Helper()

	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Reset Audit Test', 'reset-audit-'||substr(md5(random()::text),1,8))
		 RETURNING id::text`).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	var userID string
	var pwVersion int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('reset-admin-'||substr(md5(random()::text),1,8)||'@test.local')
		 RETURNING id::text, pw_version`).Scan(&userID, &pwVersion); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id)
		 SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Admin' LIMIT 1`,
		orgID, userID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	raw, err := hex.DecodeString(secret)
	require.NoError(t, err, "decode master key")
	keyBytes, err := sharedcrypto.DeriveServiceKey(raw, "vakt-paseto-v1")
	require.NoError(t, err, "derive paseto key")
	key, err := auth.GenerateSymmetricKeyFromBytes(keyBytes)
	require.NoError(t, err, "paseto key")

	token, err = auth.IssueAccessToken(key, auth.Claims{
		UserID:    userID,
		OrgID:     orgID,
		Roles:     []string{"Admin"},
		PwVersion: pwVersion,
		MFA:       true,
	})
	require.NoError(t, err, "issue token")
	return orgID, token
}

// seedOrgMember creates the reset target: an active user in the same org. The
// handler resolves it by email and requires is_active plus the membership row.
func seedOrgMember(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID string) (userID, email string) {
	t.Helper()

	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, is_active) VALUES ('reset-target-'||substr(md5(random()::text),1,8)||'@test.local', TRUE)
		 RETURNING id::text, email`).Scan(&userID, &email); err != nil {
		t.Fatalf("seed target user: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id)
		 SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Viewer' LIMIT 1`,
		orgID, userID); err != nil {
		t.Fatalf("seed target membership: %v", err)
	}
	return userID, email
}
