// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/auth"
)

// R1-14cA-13 — the first MFA stage handed out the account's identity.
//
// The password is the first factor; the second exists because the first can be
// stolen. The stage-1 response carried id, email, display name and the role in
// the organisation — so whoever held only the password learned who the account
// belongs to and how much it is worth attacking. That is the reconnaissance the
// second factor is supposed to stand in front of.
func TestLogin_firstMFAStageRevealsNothingAboutTheAccount(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "mfa-stage1-" + suffix + "@example.com"
	const password = "Str0ng-Passw0rd!x"

	orgID := seedInstanceOrg(t, pool)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role, is_active)
		VALUES ($1, $2, 'Dr. Beispiel Nachname', 'admin', TRUE)
		RETURNING id::text`, email, string(hash)).Scan(&userID))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1::uuid`, userID) })

	_, err = pool.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Admin'`, orgID, userID)
	require.NoError(t, err)

	// A confirmed TOTP secret is what puts the login into its two-stage form.
	_, err = pool.Exec(ctx, `
		INSERT INTO totp_secrets (user_id, secret, enabled) VALUES ($1::uuid, 'deadbeef', TRUE)
		ON CONFLICT (user_id) DO UPDATE SET enabled = TRUE`, userID)
	require.NoError(t, err)

	svc := auth.NewService(pool, nil, mustKey(t))
	resp, err := svc.Login(ctx, email, password, "go-test")
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.True(t, resp.MFARequired, "precondition: the account must be in the two-stage login")
	require.NotEmpty(t, resp.MFAToken, "precondition: stage 1 hands out the pending token")

	assert.Empty(t, resp.User.DisplayName,
		"the display name reached a caller who has not passed the second factor")
	assert.Empty(t, resp.User.Roles,
		"the account's role in the organisation reached a caller who has not passed the second factor")
	assert.Empty(t, resp.User.ID,
		"the user id reached a caller who has not passed the second factor")
	assert.Empty(t, resp.User.Email,
		"the address reached a caller who has not passed the second factor")

	// And no session, which is the older guarantee this must not disturb.
	assert.Empty(t, resp.AccessToken)
	assert.Empty(t, resp.RefreshToken)
}
