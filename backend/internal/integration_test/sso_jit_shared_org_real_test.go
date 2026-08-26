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

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
)

// R1-W2FIX-SSO-01, the half that needs an EMPTY instance: an SSO login must not found
// the organisation.
//
// Vakt runs one customer per server. The organisation is created once, by
// first-run setup, and setup refuses to run a second time. Self-registration
// carries the matching guard (Service.Register returns ErrRegistrationDisabled
// as soon as an organisation exists). The SSO path had neither: it created a
// fresh organisation per just-in-time provisioned user, so the second colleague
// signing in through the customer's IdP got an empty management system of their
// own instead of the shared ISMS.
//
// This case cannot live in internal/auth's suite — that one shares a single
// database with every other test in the package, so "no organisation exists" is
// not a state it can rely on. Here every test gets a fresh Postgres.
func TestOIDC_JIT_RefusesToFoundTheInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, cleanup := startRoleTestPostgres(ctx, t)
	defer cleanup()

	var orgCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgCount))
	require.Zero(t, orgCount, "precondition: a freshly migrated instance has no organisation")

	casdoor := newMockCasdoor(t, casdoorProfileResponse{
		Sub:           "founder-sub-7001",
		Email:         "founder@example.org",
		Name:          "Founder",
		EmailVerified: true,
	})
	defer casdoor.Close()

	svc := auth.NewService(pool, nil, mustKeyIntegration(t))
	cfg := &config.Config{
		CasdoorURL:          casdoor.URL,
		CasdoorClientID:     "test-client",
		CasdoorClientSecret: "test-secret",
		FrontendURL:         "http://localhost:5173",
	}

	_, err := svc.OIDCLogin(ctx, cfg, "google", "code123", "state123", "ua")
	require.Error(t, err, "SSO login on an un-setup instance must fail, not found the organisation")
	assert.ErrorIs(t, err, auth.ErrNoOrganization)

	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgCount))
	assert.Zero(t, orgCount,
		"an IdP login created the instance's organisation — founding it is first-run setup's job")
}

// Two colleagues arriving through the same IdP share the customer's ISMS.
//
// The measured shape of the defect: with an organisation already present, two
// OIDC logins left THREE organisations behind and put each user in their own.
func TestOIDC_JIT_ColleaguesShareOneOrganisation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, cleanup := startRoleTestPostgres(ctx, t)
	defer cleanup()

	// The organisation first-run setup created.
	var instanceOrg string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Kundenorg', 'kundenorg')
		RETURNING id::text`).Scan(&instanceOrg))

	svc := auth.NewService(pool, nil, mustKeyIntegration(t))

	login := func(sub, email, name string) {
		t.Helper()
		casdoor := newMockCasdoor(t, casdoorProfileResponse{
			Sub: sub, Email: email, Name: name, EmailVerified: true,
		})
		defer casdoor.Close()
		cfg := &config.Config{
			CasdoorURL:          casdoor.URL,
			CasdoorClientID:     "test-client",
			CasdoorClientSecret: "test-secret",
			FrontendURL:         "http://localhost:5173",
		}
		_, err := svc.OIDCLogin(ctx, cfg, "google", "code-"+sub, "state", "ua")
		require.NoError(t, err, "SSO login for %s failed", email)
	}

	login("colleague-a-1", "a@example.org", "Kollegin A")
	login("colleague-b-2", "b@example.org", "Kollege B")

	var orgCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgCount))
	assert.Equal(t, 1, orgCount,
		"SSO just-in-time provisioning founded further organisations — this instance has exactly one")

	rows, err := pool.Query(ctx, `
		SELECT u.email, om.org_id::text, r.name
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		JOIN roles r ON r.id = om.role_id
		WHERE u.email IN ('a@example.org', 'b@example.org')
		ORDER BY u.email`)
	require.NoError(t, err)
	defer rows.Close()

	seen := map[string][2]string{}
	for rows.Next() {
		var email, orgID, roleName string
		require.NoError(t, rows.Scan(&email, &orgID, &roleName))
		seen[email] = [2]string{orgID, roleName}
	}
	require.NoError(t, rows.Err())
	require.Len(t, seen, 2, "both SSO users must hold exactly one membership each")

	assert.Equal(t, instanceOrg, seen["a@example.org"][0],
		"the first SSO colleague must join the customer's organisation")
	assert.Equal(t, instanceOrg, seen["b@example.org"][0],
		"the second SSO colleague must join the SAME organisation — a shared ISMS is the product")
	// The authorisation half: joining the org that holds the customer's ISMS is
	// only safe at the least privilege. Flip the role lookup in createOIDCUser
	// to 'Admin' and both of these turn red.
	assert.Equal(t, "Viewer", seen["a@example.org"][1])
	assert.Equal(t, "Viewer", seen["b@example.org"][1])
}
