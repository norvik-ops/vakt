// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
)

// R1-07-B07-1 — SSO admitted exactly ONE user per instance.
//
// Casdoor answers /api/get-account with its controller envelope: status, msg,
// sub, name, data, data2 — the account itself, and with it email and
// emailVerified, sits under "data". The flat profile struct read sub (present at
// the top level) and left email empty, so the first SSO user was created with
// an empty email, and every further one collided with the users_email UNIQUE index:
// duplicate key value violates unique constraint "users_email_key" (SQLSTATE
// 23505) -> HTTP 500. Measured live against a real Casdoor (SA-07, DB effect).
//
// This test drives the same two round trips against a stub that answers in
// Casdoor's envelope shape.

// casdoorStub serves the two endpoints OIDCLogin calls. accounts maps an
// authorization code to the account object Casdoor would return under "data".
func casdoorStub(t *testing.T, accounts map[string]map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// The token endpoint hands back the code itself as the access token, so the
	// profile call below can tell the two users apart.
	mux.HandleFunc("/api/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": body["code"],
			"token_type":   "Bearer",
		})
	})

	// Casdoor's controller envelope. Sub and Name are top level, the account —
	// including email and emailVerified — is nested under data.
	mux.HandleFunc("/api/get-account", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		account, ok := accounts[token]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"msg":    "",
			"sub":    account["id"],
			"name":   account["name"],
			"data":   account,
			"data2":  map[string]any{"owner": "admin", "name": "vakt"},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestOIDCLogin_secondSSOUserIsNotLockedOut(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	// Both users are provisioned just-in-time and join the instance's
	// organisation, so one has to exist (R1-W2FIX-SSO-01).
	seedInstanceOrg(t, pool)

	suffix := fmt.Sprintf("%d", os.Getpid())
	alice := map[string]any{
		"id":            "sub-alice-" + suffix,
		"name":          "alice",
		"displayName":   "Alice Example",
		"email":         "alice-" + suffix + "@example.com",
		"emailVerified": true,
		"avatar":        "https://example.com/alice.png",
	}
	bob := map[string]any{
		"id":            "sub-bob-" + suffix,
		"name":          "bob",
		"displayName":   "Bob Example",
		"email":         "bob-" + suffix + "@example.com",
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

	first, err := svc.OIDCLogin(ctx, cfg, "keycloak", "code-alice", "state", "go-test")
	require.NoError(t, err, "first SSO login failed")
	require.NotNil(t, first)

	// The defect made itself visible here: the first user was stored with an
	// empty email, which is what the second login then collided with.
	var storedEmail string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT email FROM users WHERE oidc_subject = $1`, alice["id"],
	).Scan(&storedEmail))
	assert.Equal(t, alice["email"], storedEmail,
		"the account email lives under data in Casdoor's envelope and has to reach the users row")

	second, err := svc.OIDCLogin(ctx, cfg, "keycloak", "code-bob", "state", "go-test")
	require.NoError(t, err, "second SSO user was locked out — the users_email UNIQUE index rejected the insert")
	require.NotNil(t, second)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT email FROM users WHERE oidc_subject = $1`, bob["id"],
	).Scan(&storedEmail))
	assert.Equal(t, bob["email"], storedEmail)
}

// A verified email from the IdP has to arrive as verified. emailVerified sits
// under data as well, so it always read false — and ADR-0033 then refuses on
// principle to link an IdP account to an existing local account.
func TestOIDCLogin_linksVerifiedEmailToExistingLocalAccount(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	email := fmt.Sprintf("local-%d@example.com", os.Getpid())
	cleanupOIDCUser(t, pool, email)

	// A local account that already exists — no OIDC subject yet.
	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, display_name, role) VALUES ($1, 'Local User', 'viewer') RETURNING id::text`,
		email,
	).Scan(&userID))
	orgID := seedAuthOrg(t, pool, "b071")
	_, err := pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role_id) SELECT $1::uuid, $2::uuid, id FROM roles WHERE name = 'Viewer'`,
		orgID, userID)
	require.NoError(t, err)

	srv := casdoorStub(t, map[string]map[string]any{
		"code-local": {
			"id":            "sub-local-" + fmt.Sprint(os.Getpid()),
			"name":          "local",
			"displayName":   "Local User",
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

	_, err = svc.OIDCLogin(ctx, cfg, "keycloak", "code-local", "state", "go-test")
	require.NoError(t, err, "the IdP verified this address; ADR-0033 must not refuse the link")

	var linkedSubject string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COALESCE(oidc_subject, '') FROM users WHERE id = $1::uuid`, userID,
	).Scan(&linkedSubject))
	assert.NotEmpty(t, linkedSubject, "existing account was not linked to the OIDC subject")
}

// A profile without an email must not become a user row with an empty email.
// That row turns a mapping error into a permanent lock-out: users.email is
// UNIQUE, so the FIRST such account blocks every further email-less login.
func TestOIDCLogin_profileWithoutEmailIsRefused(t *testing.T) {
	pool := authTestDB(t)
	ctx := context.Background()

	srv := casdoorStub(t, map[string]map[string]any{
		"code-noemail": {
			"id":          "sub-noemail-" + fmt.Sprint(os.Getpid()),
			"name":        "noemail",
			"displayName": "No Email",
		},
	})
	svc := auth.NewService(pool, nil, mustKey(t))
	cfg := &config.Config{
		CasdoorURL:          srv.URL,
		CasdoorClientID:     "vakt",
		CasdoorClientSecret: "secret",
		FrontendURL:         "http://localhost:5173",
	}

	_, err := svc.OIDCLogin(ctx, cfg, "keycloak", "code-noemail", "state", "go-test")
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrOIDCNoEmail)

	var emptyEmailUsers int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE email = '' AND oidc_subject = $1`,
		"sub-noemail-"+fmt.Sprint(os.Getpid()),
	).Scan(&emptyEmailUsers))
	assert.Zero(t, emptyEmailUsers, "an email-less SSO login created a user row that blocks all later ones")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// authTestDB returns a pool against the migrated test database, or skips.
func authTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — the OIDC provisioning repro needs a migrated Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// cleanupOIDCUser removes the user with this email, before and after the test.
//
// It deliberately leaves organisations alone. It used to delete the org the user
// belonged to, because JIT provisioning founded a personal one per SSO user —
// that was R1-W2FIX-SSO-01. Since the fix the user joins the instance's organisation,
// and dropping it here would delete the very org the other tests seed.
func cleanupOIDCUser(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	drop := func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email)
	}
	drop()
	t.Cleanup(drop)
}

var authOrgSeq int

func seedAuthOrg(t *testing.T, pool *pgxpool.Pool, prefix string) string {
	t.Helper()
	authOrgSeq++
	slug := fmt.Sprintf("%s-%d-%d", prefix, os.Getpid(), authOrgSeq)
	var orgID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug,
	).Scan(&orgID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})
	return orgID
}
