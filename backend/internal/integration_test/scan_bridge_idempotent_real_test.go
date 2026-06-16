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

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestScanBridge_Idempotent is the S88-8 acceptance test: a scanner finding
// creates evidence on the vulnerability control, and re-delivering the same
// finding (re-scan) does NOT create duplicate evidence (ck_scan_evidence_map).
func TestScanBridge_Idempotent(t *testing.T) {
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

	var orgID, fwID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id::text`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name)
		VALUES ($1, 'ISO 27001') RETURNING id::text`, orgID).Scan(&fwID))
	_, err = pool.Exec(ctx, `
		INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
		VALUES ($1::uuid, $2::uuid, 'A.8.8', 'Management of technical vulnerabilities', '', 'Technological')`,
		fwID, orgID)
	require.NoError(t, err)

	svc := vaktcomply.NewService(pool)

	// First delivery — should write evidence.
	n1, err := svc.RecordScanFindingEvidence(ctx, orgID, "finding-123", "Critical CVE-2026-1234")
	require.NoError(t, err)
	assert.Equal(t, 1, n1, "first delivery must write evidence to the vulnerability control")

	// Re-delivery of the SAME finding — must be a no-op (idempotent).
	n2, err := svc.RecordScanFindingEvidence(ctx, orgID, "finding-123", "Critical CVE-2026-1234")
	require.NoError(t, err)
	assert.Equal(t, 0, n2, "re-scan of the same finding must not duplicate evidence")

	var mapCount, evCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM ck_scan_evidence_map WHERE org_id=$1`, orgID).Scan(&mapCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM ck_evidence WHERE org_id=$1 AND source='automated'`, orgID).Scan(&evCount))
	assert.Equal(t, 1, mapCount, "exactly one scan_evidence_map row")
	assert.Equal(t, 1, evCount, "exactly one evidence row after re-scan")

	// A different finding still maps.
	n3, err := svc.RecordScanFindingEvidence(ctx, orgID, "finding-999", "Another CVE")
	require.NoError(t, err)
	assert.Equal(t, 1, n3)
}
