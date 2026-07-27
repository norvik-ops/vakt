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

	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestAuditChain_VerifyAfterInserts inserts three audit rows and asserts the
// per-org chain verifies clean. This is the green-path acceptance test for
// migration 149 + audit.Write + audit.VerifyOrgChain.
func TestAuditChain_VerifyAfterInserts(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	for i, action := range []string{"create", "update", "delete"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID:        orgID,
			UserEmail:    "ops@example.org",
			Action:       action,
			ResourceType: "control",
			ResourceID:   "ctrl-1",
			ResourceName: "Backup Policy",
			Details:      map[string]string{"i": string(rune('0' + i))},
			IPAddress:    "127.0.0.1",
		})
	}

	bad, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Empty(t, bad, "freshly-written chain must verify clean — got bad row %q", bad)
}

// TestAuditChain_DetectsTamperedRow simulates an attacker who reaches into
// the DB and rewrites the action of a recorded audit entry. The hash chain
// must localise the tamper to that exact row.
func TestAuditChain_DetectsTamperedRow(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	// Insert a few entries.
	for _, a := range []string{"create", "update", "approve", "export"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID:        orgID,
			Action:       a,
			ResourceType: "control",
			ResourceID:   "ctrl-1",
		})
	}

	// Pick the second entry and rewrite its action — but NOT its entry_hash.
	// In a real tampering attempt the attacker rarely also rewrites the
	// stored hash (because they don't have the chain key).
	var targetID string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id::text FROM audit_log
		WHERE org_id = $1::uuid AND action = 'update'
		ORDER BY created_at ASC LIMIT 1`, orgID).Scan(&targetID))

	// orgid-lint: global — tamper-simulation fixture; targetID was already fetched via an org-scoped query above
	_, err := pool.Exec(ctx, `
		UPDATE audit_log SET action = 'EVIL'
		WHERE id = $1::uuid`, targetID)
	require.NoError(t, err)

	bad, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Equal(t, targetID, bad, "chain verifier must localise the tamper to the modified row")
}

// TestAuditChain_DetectsDeletedRow guards another tamper shape: removing an
// audit row entirely. Subsequent rows still carry their original prev_hash,
// which now points to nothing; the verifier must surface the first orphaned
// row.
func TestAuditChain_DetectsDeletedRow(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	for _, a := range []string{"a", "b", "c", "d"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID: orgID, Action: a, ResourceType: "x",
		})
	}

	// Delete the second row.
	var targetID, nextID string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id::text FROM audit_log WHERE org_id = $1::uuid AND action = 'b'`,
		orgID).Scan(&targetID))
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id::text FROM audit_log WHERE org_id = $1::uuid AND action = 'c'`,
		orgID).Scan(&nextID))

	// orgid-lint: global — tamper-simulation fixture; targetID was already fetched via an org-scoped query above
	_, err := pool.Exec(ctx, `DELETE FROM audit_log WHERE id = $1::uuid`, targetID)
	require.NoError(t, err)

	bad, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Equal(t, nextID, bad, "deletion of a chain link must be flagged on the next row")
}

// bootPostgresWithOrg boots Postgres + runs migrations + seeds one org and
// returns the pool plus a cleanup func.
func bootPostgresWithOrg(t *testing.T) (*pgxpool.Pool, string, func()) {
	t.Helper()
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
	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)

	var orgID string
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO organizations (name, slug) VALUES ('AuditOrg', 'auditorg')
		RETURNING id::text`).Scan(&orgID))

	return pool, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(context.Background())
	}
}
