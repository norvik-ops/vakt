//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestOIDC_EmailVerified_LinkingGate verifies the ADR-0033 account-takeover
// defence: an OIDC subject whose IdP has NOT verified the email may never be
// linked to a pre-existing local account that happens to share that email.
//
// The scenario:
//
//  1. Local user `victim@example.org` exists (password-registered).
//  2. Attacker controls an OIDC account at the same IdP, claiming the same
//     email but with `emailVerified=false`.
//  3. Mock Casdoor returns the attacker's profile.
//  4. svc.OIDCLogin must fail with auth.ErrEmailNotVerified, NOT silently
//     link the attacker's `sub` to the victim's user row.
func TestOIDC_EmailVerified_LinkingGate(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Seed: one org and one pre-existing local user with email victim@example.org.
	var orgID, victimID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('VictimCorp', 'victimcorp')
		RETURNING id::text
	`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('victim@example.org', '$2a$10$abcdefghijklmnopqrstuvwxyz', 'Victim')
		RETURNING id::text
	`).Scan(&victimID))

	// Memberships entry — provisionOIDCUser reads org_members on the link path,
	// but the gate triggers BEFORE the membership lookup, so this is optional.
	// We add it so a verified-email run could succeed for the symmetric case.
	require.NoError(t, ensureMember(ctx, pool, victimID, orgID))

	// Boot a mock Casdoor that returns an attacker profile claiming the same
	// email but with emailVerified=false.
	casdoor := newMockCasdoor(t, casdoorProfileResponse{
		Sub:           "attacker-sub-9999",
		Email:         "victim@example.org",
		Name:          "Attacker",
		EmailVerified: false,
	})
	defer casdoor.Close()

	cfg := &config.Config{
		CasdoorURL:          casdoor.URL,
		CasdoorClientID:     "test-client",
		CasdoorClientSecret: "test-secret",
		FrontendURL:         "http://localhost:5173",
	}
	svc := auth.NewService(pool, nil, mustKeyIntegration(t))

	_, err = svc.OIDCLogin(ctx, cfg, "google", "code123", "state123", "ua")
	require.Error(t, err, "OIDC login MUST fail when email is not IdP-verified and a local user with that email exists")
	require.True(t, errors.Is(err, auth.ErrEmailNotVerified),
		"expected ErrEmailNotVerified, got %v", err)

	// Defence-in-depth: the victim's row must not have been silently linked.
	var oidcSubject *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT oidc_subject FROM users WHERE id = $1::uuid`,
		victimID,
	).Scan(&oidcSubject))
	assert.Nil(t, oidcSubject, "victim's oidc_subject must NOT be set by an unverified OIDC login")
}

// TestOIDC_EmailVerified_LinksOnVerified is the positive control: an IdP-
// verified email DOES link to an existing local user.
func TestOIDC_EmailVerified_LinksOnVerified(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var orgID, localID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme-link')
		RETURNING id::text
	`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('alice@example.org', '$2a$10$abcdefghijklmnopqrstuvwxyz', 'Alice')
		RETURNING id::text
	`).Scan(&localID))
	require.NoError(t, ensureMember(ctx, pool, localID, orgID))

	casdoor := newMockCasdoor(t, casdoorProfileResponse{
		Sub:           "verified-sub-1111",
		Email:         "alice@example.org",
		Name:          "Alice",
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
	require.NoError(t, err, "OIDC login with verified email must succeed")
	require.NotNil(t, resp)

	var subject *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT oidc_subject FROM users WHERE id = $1::uuid`,
		localID,
	).Scan(&subject))
	require.NotNil(t, subject)
	assert.Equal(t, "verified-sub-1111", *subject, "verified login must link the OIDC subject to the existing user")
}

// ── Helpers ─────────────────────────────────────────────────────────────────

type casdoorProfileResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"emailVerified"`
}

// newMockCasdoor spins up an httptest server that pretends to be Casdoor:
// the token endpoint returns a fixed access token, the get-account endpoint
// returns the profile passed in.
func newMockCasdoor(t *testing.T, profile casdoorProfileResponse) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
		})
	})
	mux.HandleFunc("/api/get-account", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(profile)
	})
	return httptest.NewServer(mux)
}

// ensureMember inserts an org_members row mapping userID → orgID with the
// built-in Admin role, so provisionOIDCUser's membership lookup succeeds.
func ensureMember(ctx context.Context, pool *pgxpool.Pool, userID, orgID string) error {
	var adminRoleID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Admin'`).Scan(&adminRoleID); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO org_members (user_id, org_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT DO NOTHING
	`, userID, orgID, adminRoleID)
	return err
}

// mustKeyIntegration generates a fixed Paseto key for use in tests.
func mustKeyIntegration(t *testing.T) auth.SymmetricKey {
	t.Helper()
	key, err := auth.GenerateSymmetricKey("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	return key
}
