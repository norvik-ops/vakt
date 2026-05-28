//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestRotateKey_EndToEnd boots a real Postgres, seeds every encrypted column
// with ciphertext under the OLD master key (using the *same* HKDF chain the
// production app uses), runs cmd/rotate-key as a child process, and asserts
// that every row decrypts under the NEW master's derived key — and only
// under that one.
//
// This is the canonical regression test for audit finding F1: a rotation
// that silently corrupts data is exactly the failure mode the tool used to
// have, and a unit test cannot catch it (the bug was that the tool decrypted
// with the raw master while the app encrypted under HKDF-derived keys).
func TestRotateKey_EndToEnd(t *testing.T) {
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

	// Fixed keys — easier to debug than rand(), but unique to this test so
	// we don't ship them in production lookalikes.
	oldMaster, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	newMaster, _ := hex.DecodeString("21222324252627282930313233343536373839404142434445464748494a4b4c")

	// Seed a minimal org + user + project + environment so the foreign keys
	// in so_secrets and friends resolve.
	var orgID, userID, projectID, envID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('RotateTest', 'rotatetest')
		RETURNING id::text`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('rotate@example.org', '$2a$10$abcdefghijklmnopqrstuv', 'Rotate Tester')
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

	// ── Seed each encrypted column under the OLD master's HKDF chain. ──

	// 1. so_secrets — two-stage HKDF (master → vault → project).
	oldVault, _ := sharedcrypto.DeriveServiceKey(oldMaster, "vakt-vault-v1")
	oldProjectKey, _ := sharedcrypto.DeriveProjectKey(oldVault, projectID)
	vaultPlain := []byte("the very secret password")
	vaultCT, _ := sharedcrypto.Encrypt(oldProjectKey, vaultPlain)
	_, err = pool.Exec(ctx, `
		INSERT INTO so_secrets (environment_id, org_id, key, encrypted_value, created_by)
		VALUES ($1::uuid, $2::uuid, 'API_KEY', $3, $4::uuid)`,
		envID, orgID, vaultCT, userID)
	require.NoError(t, err)

	// 2. totp_secrets — single-stage HKDF.
	oldTOTP, _ := sharedcrypto.DeriveServiceKey(oldMaster, "vakt-totp-v1")
	totpPlain := []byte("JBSWY3DPEHPK3PXP") // RFC 6238 test seed
	totpCT, _ := sharedcrypto.Encrypt(oldTOTP, totpPlain)
	_, err = pool.Exec(ctx, `
		INSERT INTO totp_secrets (user_id, secret, enabled)
		VALUES ($1::uuid, $2, true)`, userID, totpCT)
	require.NoError(t, err)

	// 3. notification_channels — url_encrypted + hmac_secret_encrypted.
	oldAlert, _ := sharedcrypto.DeriveServiceKey(oldMaster, "vakt-alert-v1")
	urlPlain := []byte("https://hooks.example/abc")
	urlCT, _ := sharedcrypto.Encrypt(oldAlert, urlPlain)
	hmacPlain := []byte("hmac-shared-secret")
	hmacCT, _ := sharedcrypto.Encrypt(oldAlert, hmacPlain)
	var chanID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO notification_channels (org_id, name, type, url_encrypted, events, enabled, hmac_secret_encrypted)
		VALUES ($1::uuid, 'webhook', 'webhook', $2, '{}', true, $3)
		RETURNING id::text`, orgID, urlCT, hmacCT).Scan(&chanID))

	// 4. integrations_github — hex-encoded ciphertext under github key.
	oldGH, _ := sharedcrypto.DeriveServiceKey(oldMaster, "vakt-github-v1")
	ghPlain := []byte("ghp_realisticlookinglength1234567890")
	ghCT, _ := sharedcrypto.Encrypt(oldGH, ghPlain)
	_, err = pool.Exec(ctx, `
		INSERT INTO integrations_github (org_id, repo_owner, repo_name, access_token)
		VALUES ($1::uuid, 'acme', 'app', $2)`, orgID, hex.EncodeToString(ghCT))
	require.NoError(t, err)

	// 5. org_saml_configs — TWO seeded rows: one HKDF, one legacy raw-master.
	oldSAML, _ := sharedcrypto.DeriveServiceKey(oldMaster, "vakt-saml-v1")
	samlPlain := []byte("-----BEGIN PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----")
	samlHKDFCT, _ := sharedcrypto.Encrypt(oldSAML, samlPlain)
	// Insert as if rotate-key would later find it
	_, err = pool.Exec(ctx, `
		INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled)
		VALUES ($1::uuid, 'urn:vakt:rt', 'https://acs', '<EntityDescriptor/>', '----CERT----', $2, true)`,
		orgID, samlHKDFCT)
	require.NoError(t, err)

	// Second org for the legacy migration path.
	var legacyOrg string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('LegacyOrg', 'legacy')
		RETURNING id::text`).Scan(&legacyOrg))
	samlLegacyCT, _ := sharedcrypto.Encrypt(oldMaster, samlPlain) // RAW master, pre-ADR-0038
	_, err = pool.Exec(ctx, `
		INSERT INTO org_saml_configs (org_id, entity_id, acs_url, idp_metadata, cert_pem, key_pem, enabled)
		VALUES ($1::uuid, 'urn:vakt:legacy', 'https://acs', '<EntityDescriptor/>', '----CERT----', $2, true)`,
		legacyOrg, samlLegacyCT)
	require.NoError(t, err)

	// 6. webhooks.secret — enc:v1:base64(ciphertext_under_old_webhook_key)
	oldWebhook, _ := sharedcrypto.DeriveServiceKey(oldMaster, "vakt-webhook-v1")
	whPlain := []byte("super-shared-webhook-secret")
	whCT, _ := sharedcrypto.Encrypt(oldWebhook, whPlain)
	whStored := "enc:v1:" + base64.URLEncoding.EncodeToString(whCT)
	var whID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO webhooks (org_id, name, url, secret, events, active)
		VALUES ($1::uuid, 'wh', 'https://example.test/hook', $2, '{}', true)
		RETURNING id::text`, orgID, whStored).Scan(&whID))

	// ── Run rotate-key as a subprocess. ──
	rotateBin := buildRotateKeyBinary(t)
	defer os.Remove(rotateBin)
	cmd := exec.CommandContext(ctx, rotateBin)
	cmd.Env = append(os.Environ(),
		"VAKT_DB_URL="+dsn,
		"VAKT_OLD_SECRET_KEY="+hex.EncodeToString(oldMaster),
		"VAKT_NEW_SECRET_KEY="+hex.EncodeToString(newMaster),
	)
	out, runErr := cmd.CombinedOutput()
	t.Logf("rotate-key output:\n%s", out)
	require.NoError(t, runErr, "rotate-key must exit 0")

	// ── Verify each row decrypts under the NEW master's derived key. ──

	// 1. vault
	newVault, _ := sharedcrypto.DeriveServiceKey(newMaster, "vakt-vault-v1")
	newProjectKey, _ := sharedcrypto.DeriveProjectKey(newVault, projectID)
	var rotatedVaultCT []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT encrypted_value FROM so_secrets WHERE key='API_KEY'`).Scan(&rotatedVaultCT))
	got, err := sharedcrypto.Decrypt(newProjectKey, rotatedVaultCT)
	require.NoError(t, err, "rotated so_secrets must decrypt under new project key")
	assert.Equal(t, vaultPlain, got)

	// And must NOT decrypt under the old project key any more.
	_, err = sharedcrypto.Decrypt(oldProjectKey, rotatedVaultCT)
	assert.Error(t, err, "old project key must no longer decrypt rotated vault row")

	// 2. totp
	newTOTP, _ := sharedcrypto.DeriveServiceKey(newMaster, "vakt-totp-v1")
	var rotatedTOTPCT []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT secret FROM totp_secrets WHERE user_id=$1::uuid`, userID).Scan(&rotatedTOTPCT))
	got, err = sharedcrypto.Decrypt(newTOTP, rotatedTOTPCT)
	require.NoError(t, err)
	assert.Equal(t, totpPlain, got)

	// 3. notification_channels
	newAlert, _ := sharedcrypto.DeriveServiceKey(newMaster, "vakt-alert-v1")
	var rotURL, rotHMAC []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT url_encrypted, hmac_secret_encrypted FROM notification_channels WHERE id=$1::uuid`, chanID,
	).Scan(&rotURL, &rotHMAC))
	got, err = sharedcrypto.Decrypt(newAlert, rotURL)
	require.NoError(t, err)
	assert.Equal(t, urlPlain, got)
	got, err = sharedcrypto.Decrypt(newAlert, rotHMAC)
	require.NoError(t, err)
	assert.Equal(t, hmacPlain, got)

	// 4. github (hex-encoded)
	newGH, _ := sharedcrypto.DeriveServiceKey(newMaster, "vakt-github-v1")
	var ghHex string
	require.NoError(t, pool.QueryRow(ctx, `SELECT access_token FROM integrations_github`).Scan(&ghHex))
	ghBytes, err := hex.DecodeString(ghHex)
	require.NoError(t, err)
	got, err = sharedcrypto.Decrypt(newGH, ghBytes)
	require.NoError(t, err)
	assert.Equal(t, ghPlain, got)

	// 5. SAML — both rows must decrypt under the new HKDF saml key.
	newSAML, _ := sharedcrypto.DeriveServiceKey(newMaster, "vakt-saml-v1")
	rows, err := pool.Query(ctx, `SELECT org_id::text, key_pem FROM org_saml_configs ORDER BY org_id`)
	require.NoError(t, err)
	var count int
	for rows.Next() {
		var oid string
		var pem []byte
		require.NoError(t, rows.Scan(&oid, &pem))
		got, err := sharedcrypto.Decrypt(newSAML, pem)
		require.NoError(t, err, "saml row %s must decrypt under new HKDF saml key (legacy rows are migrated in-flight)", oid)
		assert.Equal(t, samlPlain, got)
		count++
	}
	rows.Close()
	assert.Equal(t, 2, count, "both SAML rows (HKDF + legacy-migrated) must remain after rotation")

	// 6. webhooks.secret — value must decrypt under new webhook key.
	newWebhook, _ := sharedcrypto.DeriveServiceKey(newMaster, "vakt-webhook-v1")
	var rotatedWhStored string
	require.NoError(t, pool.QueryRow(ctx, `SELECT secret FROM webhooks WHERE id = $1::uuid`, whID).Scan(&rotatedWhStored))
	assert.True(t, strings.HasPrefix(rotatedWhStored, "enc:v1:"), "rotated webhook secret must keep the enc:v1: prefix")
	whB64 := rotatedWhStored[len("enc:v1:"):]
	whBytes, err := base64.URLEncoding.DecodeString(whB64)
	require.NoError(t, err)
	got, err = sharedcrypto.Decrypt(newWebhook, whBytes)
	require.NoError(t, err)
	assert.Equal(t, whPlain, got)
}

// buildRotateKeyBinary compiles cmd/rotate-key on demand and returns the
// absolute path to the temp executable. Callers must `defer os.Remove` it.
// We build instead of `go run` because exit codes are easier to assert and
// stderr is captured deterministically.
func buildRotateKeyBinary(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	cmdDir := filepath.Join(filepath.Dir(here), "..", "..", "cmd", "rotate-key")

	tmp, err := os.CreateTemp(t.TempDir(), "rotate-key-*")
	require.NoError(t, err)
	tmpPath := tmp.Name()
	_ = tmp.Close()

	build := exec.Command("go", "build", "-o", tmpPath, ".")
	build.Dir = cmdDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build rotate-key: %v\n%s", err, out)
	}
	return tmpPath
}
