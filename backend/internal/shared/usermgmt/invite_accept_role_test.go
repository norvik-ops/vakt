// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package usermgmt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-14cA-11, the half that bites at 'accept' time.
//
// AcceptInvitation wrote the invitation's raw role string into users.role, which
// carries CHECK (role IN ('admin','editor','viewer')). That was safe only while
// the invitation vocabulary happened to be exactly those three. With the two
// routes speaking one vocabulary, an invitation for "Admin" reaches this column
// — and the raw write turns into a constraint violation AFTER the invitee has
// clicked the link and chosen a password, at the one point where they can do
// nothing about it.
//
// The test drives the real accept path against a real Postgres, for every role
// the invitation route accepts.
func TestAcceptInvitation_writesBothRoleColumnsForEveryInvitableRole(t *testing.T) {
	pool, orgID := testDB(t)
	svc := NewService(pool, SMTPConfig{}, "")
	ctx := context.Background()

	for _, role := range oneofValues(t, InviteInput{}, "Role") {
		t.Run(role, func(t *testing.T) {
			email := fmt.Sprintf("invitee-%s-%d@example.com", role, os.Getpid())
			token := seedInvitation(t, pool, orgID, email, role)
			t.Cleanup(func() {
				// orgid-lint: global — Aufraeumen des in diesem Unterfall
				// angelegten Nutzers, per eindeutiger E-Mail.
				_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
			})

			err := svc.AcceptInvitation(ctx, AcceptInviteInput{
				Token:    token,
				Name:     "Invited Person",
				Password: "Str0ng-Passw0rd!x",
			})
			require.NoError(t, err,
				"accepting an invitation for role %q failed — the invitee cannot get in at all", role)

			var cached, platform string
			// orgid-lint: global — Testabfrage, per users.email auf den in diesem
			// Unterfall angelegten Nutzer eingegrenzt; die Org steht in org_members.
			require.NoError(t, pool.QueryRow(ctx, `
				SELECT u.role, r.name
				FROM users u
				JOIN org_members om ON om.user_id = u.id
				JOIN roles r ON r.id = om.role_id
				WHERE u.email = $1`, email).Scan(&cached, &platform))

			want := assignableRoles[role]
			assert.Equal(t, want.platform, platform,
				"org_members holds the authoritative role and it does not match the invitation")
			assert.Equal(t, want.simple, cached,
				"users.role must be the cache value of the invited role (ADR-0077), never the raw string")
		})
	}
}

// The invitation route itself has to take the role-change vocabulary. Before
// migration 255 the column's CHECK held it to admin|editor|viewer, so widening
// only the input validation would have turned the invitation into a 500 —
// SQLSTATE 23514, with nothing in the response that says what to send instead.
func TestCreateInvitation_takesThePlatformRoleNames(t *testing.T) {
	pool, orgID := testDB(t)
	svc := NewService(pool, SMTPConfig{}, "")
	ctx := context.Background()

	for _, role := range []string{"InternalAuditor", "AuditorReadOnly", "SecurityAnalyst", "Admin", "viewer"} {
		t.Run(role, func(t *testing.T) {
			email := fmt.Sprintf("invited-%s-%d@example.com", role, os.Getpid())
			t.Cleanup(func() {
				_, _ = pool.Exec(context.Background(),
					`DELETE FROM user_invitations WHERE org_id = $1::uuid AND email = $2`, orgID, email)
			})

			inv, err := svc.CreateInvitation(ctx, orgID, "admin@example.com", InviteInput{
				Email: email,
				Role:  role,
			})
			require.NoError(t, err,
				"inviting somebody as %q failed — the role change accepts that name and the invitation does not", role)

			assert.Equal(t, assignableRoles[role].platform, inv.Role,
				"the invitation must store the platform role name, so the accept path has nothing to guess")
		})
	}
}

// seedInvitation writes a pending invitation directly and returns its plaintext
// token. CreateInvitation does not hand the token back (it goes out by mail), so
// the row is written here.
func seedInvitation(t *testing.T, pool *pgxpool.Pool, orgID, email, role string) string {
	t.Helper()
	token := fmt.Sprintf("invite-token-%s-%d", role, os.Getpid())
	sum := sha256.Sum256([]byte(token))
	_, err := pool.Exec(context.Background(), `
		INSERT INTO user_invitations (org_id, email, role, token_hash, invited_by, expires_at)
		VALUES ($1::uuid, $2, $3, $4, 'admin@example.com', NOW() + INTERVAL '7 days')`,
		orgID, email, role, hex.EncodeToString(sum[:]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM user_invitations WHERE org_id = $1::uuid AND email = $2`, orgID, email)
	})
	return token
}
