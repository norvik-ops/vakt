//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vaktvault"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestVaultSecret_AADBinding boots a real Postgres and proves the S90-3
// Associated-Data behaviour end-to-end through the vaktvault service:
//
//  1. SetSecret writes an enc:v2: ciphertext bound to org_id+secret_id.
//  2. GetSecret round-trips that value.
//  3. A ciphertext copied verbatim into a DIFFERENT secret row (different
//     secret_id) fails to decrypt — the GCM tag check rejects the wrong AAD.
//  4. A legacy (marker-less, no-AAD) ciphertext is still decryptable, and the
//     next SetSecret lazy-upgrades it to the enc:v2: bound form.
func TestVaultSecret_AADBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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

	master, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	svc := vaktvault.NewService(pool, master, nil)

	// Seed org + user + project + environment so the FKs resolve.
	var orgID, userID, projectID, envID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('AADTest', 'aadtest')
		RETURNING id::text`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('aad@example.org', '$2a$10$abcdefghijklmnopqrstuv', 'AAD Tester')
		RETURNING id::text`).Scan(&userID))
	require.NoError(t, ensureMember(ctx, pool, userID, orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO so_projects (org_id, name, slug, created_by)
		VALUES ($1::uuid, 'P', 'p', $2::uuid)
		RETURNING id::text`, orgID, userID).Scan(&projectID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO so_environments (project_id, org_id, name)
		VALUES ($1::uuid, $2::uuid, 'prod')
		RETURNING id::text`, projectID, orgID).Scan(&envID))

	// 1. SetSecret writes an enc:v2: marked, AAD-bound ciphertext.
	sec, err := svc.SetSecret(ctx, orgID, envID, userID, "API_KEY", "s3cr3t-value")
	require.NoError(t, err)

	var stored []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT encrypted_value FROM so_secrets WHERE id = $1::uuid AND org_id = $2::uuid`,
		sec.ID, orgID).Scan(&stored))
	assert.True(t, strings.HasPrefix(string(stored), "enc:v2:"),
		"stored ciphertext must carry the enc:v2: AAD marker")

	// 2. GetSecret round-trips.
	got, err := svc.GetSecret(ctx, orgID, envID, "API_KEY", "api", "1.2.3.4")
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t-value", got.Value)

	// 3. Copy the ciphertext verbatim into a second secret row. The new row has
	//    a different secret_id → different AAD → decrypt must fail.
	var otherEnv string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO so_environments (project_id, org_id, name)
		VALUES ($1::uuid, $2::uuid, 'staging')
		RETURNING id::text`, projectID, orgID).Scan(&otherEnv))
	_, err = pool.Exec(ctx, `
		INSERT INTO so_secrets (environment_id, org_id, key, encrypted_value, created_by)
		VALUES ($1::uuid, $2::uuid, 'API_KEY', $3, $4::uuid)`,
		otherEnv, orgID, stored, userID)
	require.NoError(t, err)
	_, err = svc.GetSecret(ctx, orgID, otherEnv, "API_KEY", "api", "1.2.3.4")
	require.Error(t, err, "a ciphertext copied to a different secret row must fail to decrypt (wrong AAD)")

	// 4. Legacy backward-compat + lazy upgrade. Write a marker-less ciphertext
	//    (the pre-S90-3 format) directly into the original row.
	projectKey, err := sharedcrypto.DeriveProjectKey(master, projectID)
	require.NoError(t, err)
	legacyCT, err := sharedcrypto.Encrypt(projectKey, []byte("legacy-plaintext"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`UPDATE so_secrets SET encrypted_value = $2 WHERE id = $1::uuid AND org_id = $3::uuid`,
		sec.ID, legacyCT, orgID)
	require.NoError(t, err)

	legacyGot, err := svc.GetSecret(ctx, orgID, envID, "API_KEY", "api", "1.2.3.4")
	require.NoError(t, err, "legacy marker-less ciphertext must still decrypt")
	assert.Equal(t, "legacy-plaintext", legacyGot.Value)

	// The next write lazy-upgrades the row to the bound enc:v2: form.
	_, err = svc.SetSecret(ctx, orgID, envID, userID, "API_KEY", "rotated-value")
	require.NoError(t, err)
	var upgraded []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT encrypted_value FROM so_secrets WHERE id = $1::uuid AND org_id = $2::uuid`,
		sec.ID, orgID).Scan(&upgraded))
	assert.True(t, strings.HasPrefix(string(upgraded), "enc:v2:"),
		"after re-write the legacy value must be lazy-upgraded to enc:v2:")
}
