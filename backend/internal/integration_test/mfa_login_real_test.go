//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/auth"
)

// TestMFA_TwoStageLogin is the S124-1 (SA14-01) regression guard. It proves that
// a correct password against an MFA-enrolled account yields NO session — only an
// mfa_pending token — and that completing the second factor produces a token
// whose mfa claim is true. This is what makes a stolen password insufficient.
func TestMFA_TwoStageLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Seed a user with a real password + role membership.
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectHorse1!"), 12)
	require.NoError(t, err)
	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, is_active)
		VALUES ('mfa-user@acme.test', $1, 'MFA User', TRUE) RETURNING id::text`,
		string(hash)).Scan(&userID))

	// Any existing role in the org is fine — the two-stage flow does not depend on
	// which role. Pick the first role and make the user a member.
	var roleID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM roles ORDER BY name LIMIT 1`).Scan(&roleID))
	_, err = pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID)
	require.NoError(t, err)

	key := mustKeyIntegration(t)
	svc := auth.NewService(pool, nil, key)

	// ── Stage 0: no MFA enrolled → password yields a full session (mfa=false). ──
	resp, err := svc.Login(ctx, "mfa-user@acme.test", "CorrectHorse1!", "test-agent")
	require.NoError(t, err)
	require.False(t, resp.MFARequired, "no TOTP enrolled: login must issue a session directly")
	require.NotEmpty(t, resp.AccessToken)
	claims0, err := auth.ParseAccessToken(key, resp.AccessToken)
	require.NoError(t, err)
	assert.False(t, claims0.MFA, "a password-only session must have mfa=false")

	// Enrol TOTP (enabled=true). The secret value is irrelevant for this test —
	// CompleteMFALogin is called after the handler validates the code.
	_, err = pool.Exec(ctx, `
		INSERT INTO totp_secrets (user_id, secret, enabled, backup_codes)
		VALUES ($1::uuid, 'enc-secret', TRUE, ARRAY[]::text[])`, userID)
	require.NoError(t, err)

	// ── Stage 1: password on an MFA-enrolled account → NO session, only pending. ──
	resp2, err := svc.Login(ctx, "mfa-user@acme.test", "CorrectHorse1!", "test-agent")
	require.NoError(t, err)
	assert.True(t, resp2.MFARequired, "MFA-enrolled account must require the second factor")
	assert.Empty(t, resp2.AccessToken, "no access token before the second factor is proven")
	assert.Empty(t, resp2.RefreshToken, "no refresh token before the second factor is proven")
	require.NotEmpty(t, resp2.MFAToken, "an mfa_pending token must be returned")

	// The pending token must NOT be usable as a full access token.
	_, parseErr := auth.ParseAccessToken(key, resp2.MFAToken)
	assert.Error(t, parseErr, "mfa_pending token must be rejected by ParseAccessToken")

	// It MUST parse at the pending endpoint and identify the right subject.
	uid, oid, err := auth.ParseMFAPendingToken(key, resp2.MFAToken)
	require.NoError(t, err)
	assert.Equal(t, userID, uid)
	assert.Equal(t, orgID, oid)

	// ── Stage 2: completing the second factor issues a full mfa=true session. ──
	final, err := svc.CompleteMFALogin(ctx, uid, oid, "test-agent")
	require.NoError(t, err)
	require.NotEmpty(t, final.AccessToken)
	claims, err := auth.ParseAccessToken(key, final.AccessToken)
	require.NoError(t, err)
	assert.True(t, claims.MFA, "a session that completed MFA must carry mfa=true")
	assert.Equal(t, userID, claims.UserID)
}
