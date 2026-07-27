//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/services/scim"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// Finding V24-a (Codeaudit v4, Story S1 / D24-1): `users.role` carried
// DEFAULT 'admin' (migration 077) while `requireAdmin` reads exactly that
// column. Every insert path that omitted `role` therefore minted an admin.
// The two IdP-driven paths — OIDC just-in-time provisioning and SCIM
// provisioning — are the dangerous ones, because the user row is created by
// an external system with no human in the loop.
//
// Why this test exists in this form:
//
// The static gate (scripts/check_user_role_insert.py) proves only that an
// `INSERT INTO users` *mentions* the `role` column. It cannot see WHICH value
// is written. Reverting `oidc.go`/`scim/service.go` to `'admin'` keeps that
// gate green, and rbac_matrix_test.go seeds its users directly rather than
// going through these constructors — so neither existing gate covers the
// regression that the finding is actually about.
//
// This test therefore asserts the VALUE, against a real Postgres with the
// real migrations applied, by driving the real provisioning entry points.
// Both halves are written so they FAIL if the default is relied upon: they
// would also have failed before migration 249, which is the point.

// TestOIDC_JIT_ProvisionsViewerRole drives auth.Service.OIDCLogin with a
// brand-new subject/email (no pre-existing local user → JIT create path,
// createOIDCUser) and asserts the resulting `users.role` is 'viewer'.
func TestOIDC_JIT_ProvisionsViewerRole(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, cleanup := startRoleTestPostgres(ctx, t)
	defer cleanup()

	// A subject/email that exists nowhere yet — forces the JIT create path.
	casdoor := newMockCasdoor(t, casdoorProfileResponse{
		Sub:           "jit-new-subject-4242",
		Email:         "newcomer@example.org",
		Name:          "Newcomer",
		EmailVerified: true,
	})
	defer casdoor.Close()

	cfg := &config.Config{
		CasdoorURL:          casdoor.URL,
		CasdoorClientID:     "test-client",
		CasdoorClientSecret: "test-secret",
		FrontendURL:         "http://localhost:5173",
	}
	svc := auth.NewService(pool, nil, mustKeyIntegration(t))

	resp, err := svc.OIDCLogin(ctx, cfg, "google", "code123", "state123", "ua")
	require.NoError(t, err, "OIDC JIT login for a new verified user must succeed")
	require.NotNil(t, resp)

	var role string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT role FROM users WHERE email = $1`,
		"newcomer@example.org",
	).Scan(&role))

	assert.Equal(t, "viewer", role,
		"V24-a: an OIDC just-in-time provisioned user MUST get users.role='viewer' — "+
			"anything else (notably 'admin') is the D24-1 privilege escalation")

	// The org_members role must agree: a split between the two columns is the
	// split-brain condition ADR-0077 exists to remove.
	var memberRole string
	// orgid-lint: global — Testabfrage, per users.email auf den in diesem Test
	// erzeugten Nutzer eingegrenzt; org_members traegt die Org selbst.
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT r.name FROM org_members om
		JOIN roles r ON r.id = om.role_id
		JOIN users u ON u.id = om.user_id
		WHERE u.email = $1
	`, "newcomer@example.org").Scan(&memberRole))
	assert.Equal(t, "Viewer", memberRole,
		"org_members.role and users.role must not disagree for an SSO-provisioned user")
}

// TestSCIM_ProvisionsViewerRole drives scim.Service.CreateUser (the IdP push
// path) and asserts the resulting `users.role` is 'viewer'.
func TestSCIM_ProvisionsViewerRole(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, cleanup := startRoleTestPostgres(ctx, t)
	defer cleanup()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('ScimCorp', 'scimcorp')
		RETURNING id::text
	`).Scan(&orgID))

	svc := scim.NewService(pool)
	created, err := svc.CreateUser(ctx, orgID, scim.SCIMUser{
		UserName:    "scim.user@example.org",
		DisplayName: "Scim User",
		Email:       "scim.user@example.org",
		Active:      true,
		ExternalID:  "ext-9001",
	})
	require.NoError(t, err, "SCIM provisioning must succeed")
	require.NotNil(t, created)

	var role string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT role FROM users WHERE email = $1`,
		"scim.user@example.org",
	).Scan(&role))

	assert.Equal(t, "viewer", role,
		"V24-a: a SCIM-provisioned user MUST get users.role='viewer' — "+
			"anything else (notably 'admin') is the D24-1 privilege escalation")

	var memberRole string
	// orgid-lint: global — Testabfrage, per users.email auf den in diesem Test
	// erzeugten Nutzer eingegrenzt; org_members traegt die Org selbst.
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT r.name FROM org_members om
		JOIN roles r ON r.id = om.role_id
		JOIN users u ON u.id = om.user_id
		WHERE u.email = $1
	`, "scim.user@example.org").Scan(&memberRole))
	assert.Equal(t, "Viewer", memberRole,
		"org_members.role and users.role must not disagree for a SCIM-provisioned user")
}

// TestUsersRoleDefaultIsViewer pins migration 249 itself. A bare insert that
// omits `role` must land on 'viewer'. This is the belt to the explicit-value
// braces above: if someone adds a seventh insert path and forgets `role`, the
// column default is the last thing standing between them and an admin.
func TestUsersRoleDefaultIsViewer(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, cleanup := startRoleTestPostgres(ctx, t)
	defer cleanup()

	var role string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name)
		VALUES ('defaulted@example.org', 'Defaulted')
		RETURNING role
	`).Scan(&role))

	assert.Equal(t, "viewer", role,
		"migration 249: users.role DEFAULT must be 'viewer', never 'admin' (D24-1)")
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// startRoleTestPostgres boots a throwaway Postgres with all migrations applied
// and returns a pool plus its cleanup. Mirrors the pattern used by the other
// *_real_test.go files in this package.
func startRoleTestPostgres(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

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

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	return pool, func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
}
