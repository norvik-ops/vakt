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

// TestAuditLog_PartitionedAfterMigration verifies that migration 151
// landed: pg_class reports audit_log as a partitioned table (relkind='p')
// and the expected yearly children exist.
func TestAuditLog_PartitionedAfterMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, _, cleanup := bootPartitionedPostgres(t)
	defer cleanup()
	ctx := context.Background()

	// relkind is a single-char column (pg type "char"); cast to text so
	// pgx can scan it into a Go string.
	var relkind string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT relkind::text FROM pg_class WHERE relname='audit_log'`,
	).Scan(&relkind))
	assert.Equal(t, "p", relkind, "audit_log must be a partitioned table after migration 151")

	// Every yearly partition listed in 151 must exist.
	for _, want := range []string{"audit_log_2025", "audit_log_2026", "audit_log_2027", "audit_log_2028", "audit_log_default"} {
		var found bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relname=$1)`, want,
		).Scan(&found))
		assert.True(t, found, "expected partition %s to exist", want)
	}
}

// TestAuditLog_PartitionRoutingPicksCorrectChild inserts rows that fall
// into different years and asserts that pg_inherits routes them to the
// right partition. This is the property that makes per-month archival
// possible later.
func TestAuditLog_PartitionRoutingPicksCorrectChild(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPartitionedPostgres(t)
	defer cleanup()
	ctx := context.Background()

	// Insert one row dated 2026, one dated 2027 — bypass the writer so we
	// can hand-pick the timestamps.
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_log (org_id, action, resource_type, created_at)
		VALUES ($1::uuid, 'create', 'control', '2026-06-15 12:00:00+00'),
		       ($1::uuid, 'delete', 'control', '2027-01-15 12:00:00+00')`, orgID)
	require.NoError(t, err)

	cases := map[string]int{
		"audit_log_2026": 1,
		"audit_log_2027": 1,
	}
	for partition, want := range cases {
		var got int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT count(*) FROM "+partition,
		).Scan(&got))
		assert.Equal(t, want, got, "expected %d rows in %s, got %d", want, partition, got)
	}
}

// TestAuditLog_VerifierStillWorksAfterPartitioning is the core regression:
// the per-org hash chain (ADR-0040) must keep working unchanged across the
// partitioned table. The verifier scans by (created_at, id) ASC which is
// transparent to the partition layout.
func TestAuditLog_VerifierStillWorksAfterPartitioning(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPartitionedPostgres(t)
	defer cleanup()
	ctx := context.Background()

	for _, a := range []string{"create", "update", "approve", "export"} {
		audit.Write(ctx, pool, audit.WriteEntry{
			OrgID: orgID, Action: a, ResourceType: "control",
		})
	}

	bad, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Empty(t, bad, "chain over partitioned audit_log must verify clean")
}

// TestAuditLog_LegacyViewStillCompatible: the audit_logs view that
// migration 085 created for back-compat callers must keep existing
// after the partition swap.
func TestAuditLog_LegacyViewStillCompatible(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPartitionedPostgres(t)
	defer cleanup()
	ctx := context.Background()

	audit.Write(ctx, pool, audit.WriteEntry{OrgID: orgID, Action: "test", ResourceType: "view"})

	var c int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_logs WHERE org_id=$1::uuid`, orgID,
	).Scan(&c))
	assert.GreaterOrEqual(t, c, 1)
}

func bootPartitionedPostgres(t *testing.T) (*pgxpool.Pool, string, func()) {
	t.Helper()
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
		INSERT INTO organizations (name, slug) VALUES ('PartOrg', 'partorg')
		RETURNING id::text`).Scan(&orgID))

	return pool, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(context.Background())
	}
}
