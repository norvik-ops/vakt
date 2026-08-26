// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
)

// R1-W2FIX-SSO-01 — every SSO colleague landed in their own, empty organisation.
//
// createOIDCUser founded a fresh organisation for every just-in-time
// provisioned SSO user, and the OIDC path hard-wires createIfNotFound=true.
// Vakt runs one customer per server and its whole purpose is ONE shared ISMS,
// so the second colleague to sign in via the customer's IdP did not join the
// existing management system — they got a new, empty one of their own, with the
// existing controls, risks and evidence nowhere in sight.
//
// Local self-registration never had this hole: Service.Register refuses
// outright once an organisation exists (ErrRegistrationDisabled), precisely so
// a reachable instance cannot be used to found further organisations. The SSO
// path bypassed that guard.
//
// Before PR #87 the defect was invisible, because exactly one SSO user could
// ever get in (R1-07-B07-1).

func TestOIDCLogin_secondSSOUserJoinsTheSameOrganisation(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	// The instance's organisation, as first-run setup would have created it.
	instanceOrg := seedInstanceOrg(t, pool)

	suffix := fmt.Sprintf("%d", os.Getpid())
	alice := map[string]any{
		"id":            "sharedorg-alice-" + suffix,
		"name":          "alice",
		"displayName":   "Alice Example",
		"email":         "sharedorg-alice-" + suffix + "@example.com",
		"emailVerified": true,
	}
	bob := map[string]any{
		"id":            "sharedorg-bob-" + suffix,
		"name":          "bob",
		"displayName":   "Bob Example",
		"email":         "sharedorg-bob-" + suffix + "@example.com",
		"emailVerified": true,
	}
	cleanupOIDCUser(t, pool, alice["email"].(string))
	cleanupOIDCUser(t, pool, bob["email"].(string))

	srv := casdoorStub(t, map[string]map[string]any{
		"code-alice": alice,
		"code-bob":   bob,
	})
	svc := auth.NewService(pool, nil, mustKey(t))
	cfg := &config.Config{
		CasdoorURL:          srv.URL,
		CasdoorClientID:     "vakt",
		CasdoorClientSecret: "secret",
		FrontendURL:         "http://localhost:5173",
	}

	var orgsBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgsBefore))

	_, err := svc.OIDCLogin(ctx, cfg, "keycloak", "code-alice", "state", "go-test")
	require.NoError(t, err, "first SSO login failed")
	_, err = svc.OIDCLogin(ctx, cfg, "keycloak", "code-bob", "state", "go-test")
	require.NoError(t, err, "second SSO login failed")

	var orgsAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgsAfter))
	assert.Equal(t, orgsBefore, orgsAfter,
		"SSO just-in-time provisioning founded an organisation — this instance has exactly one, "+
			"and an IdP login must join it, not create a second one next to it")

	aliceOrg := orgOf(t, pool, alice["email"].(string))
	bobOrg := orgOf(t, pool, bob["email"].(string))
	assert.Equal(t, instanceOrg, aliceOrg,
		"the first SSO user must join the instance's organisation")
	assert.Equal(t, aliceOrg, bobOrg,
		"two colleagues signing in through the same IdP must share one management system")
}

// The "instance has no organisation at all" half needs an un-setup database and
// therefore lives in internal/integration_test (sso_jit_shared_org_real_test.go),
// where each case gets a fresh Postgres. This package shares one database with
// every other test in it, so the case would be skipped here more often than run.

// The joined membership has to be the least-privileged one. This is the
// authorisation half of the fix: the user is placed into the org that already
// holds the customer's ISMS, so the role they get there is what decides whether
// an IdP account can read or write it. Flipping the role lookup to 'Admin'
// turns this red.
func TestOIDCLogin_jitUserJoinsInstanceOrgAsViewer(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	instanceOrg := seedInstanceOrg(t, pool)

	suffix := fmt.Sprintf("%d", os.Getpid())
	email := "jitrole-" + suffix + "@example.com"
	cleanupOIDCUser(t, pool, email)

	srv := casdoorStub(t, map[string]map[string]any{
		"code-jitrole": {
			"id":            "jitrole-" + suffix,
			"name":          "jitrole",
			"displayName":   "Jit Role",
			"email":         email,
			"emailVerified": true,
		},
	})
	svc := auth.NewService(pool, nil, mustKey(t))
	cfg := &config.Config{
		CasdoorURL:          srv.URL,
		CasdoorClientID:     "vakt",
		CasdoorClientSecret: "secret",
		FrontendURL:         "http://localhost:5173",
	}

	_, err := svc.OIDCLogin(ctx, cfg, "keycloak", "code-jitrole", "state", "go-test")
	require.NoError(t, err)

	var orgID, roleName, cachedRole string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT om.org_id::text, r.name, u.role
		FROM org_members om
		JOIN roles r ON r.id = om.role_id
		JOIN users u ON u.id = om.user_id
		WHERE u.email = $1`, email,
	).Scan(&orgID, &roleName, &cachedRole))

	assert.Equal(t, instanceOrg, orgID)
	assert.Equal(t, "Viewer", roleName,
		"an IdP-provisioned account joins the customer's ISMS — it must arrive as Viewer, "+
			"anything else hands an outside identity provider write access to it")
	assert.Equal(t, "viewer", cachedRole,
		"users.role must agree with org_members (D24-1 split-brain)")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// seedInstanceOrg makes sure the database has an organisation and returns the
// one the SSO path resolves as the instance's: the oldest.
func seedInstanceOrg(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var orgID string
	err := pool.QueryRow(ctx,
		`SELECT id::text FROM organizations ORDER BY created_at ASC, id ASC LIMIT 1`,
	).Scan(&orgID)
	if err == nil {
		return orgID
	}
	return seedAuthOrg(t, pool, "instance")
}

// orgOf returns the org_id of the user's single membership.
func orgOf(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var orgID string
	// orgid-lint: global — Testabfrage, per users.email auf den in diesem Test
	// erzeugten Nutzer eingegrenzt.
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT om.org_id::text FROM org_members om
		JOIN users u ON u.id = om.user_id
		WHERE u.email = $1`, email).Scan(&orgID))
	return orgID
}
