//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vaktscan"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestExportFindings_CursorPagination is the S120-9 regression: the findings
// export must return every row exactly once across batch boundaries (batch
// size 500, keyset pagination) — the previous OFFSET loop degraded to O(n²)
// and CLAUDE.md claimed cursor pagination that did not exist.
func TestExportFindings_CursorPagination(t *testing.T) {
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

	var orgID, assetID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme','acme') RETURNING id::text`,
	).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO vb_assets (org_id, name, type) VALUES ($1::uuid, 'web-1', 'server') RETURNING id::text`,
		orgID,
	).Scan(&assetID))

	// 1203 findings — forces 3 cursor batches (500/500/203). Identical
	// created_at values across many rows exercise the (created_at, id)
	// tie-breaker of the keyset cursor.
	const total = 1203
	batch := &strings.Builder{}
	batch.WriteString(`INSERT INTO vb_findings (org_id, asset_id, title, severity, status, scanner, created_at) VALUES `)
	for i := 0; i < total; i++ {
		if i > 0 {
			batch.WriteString(",")
		}
		// only 7 distinct timestamps → heavy created_at collisions
		fmt.Fprintf(batch,
			`('%s'::uuid, '%s'::uuid, 'finding-%04d', 'medium', 'open', 'trivy', NOW() - make_interval(mins => %d))`,
			orgID, assetID, i, i%7,
		)
	}
	_, err = pool.Exec(ctx, batch.String())
	require.NoError(t, err)

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})

	reader, err := svc.ExportFindings(ctx, orgID, "csv", vaktscan.FindingFilter{})
	require.NoError(t, err)

	records, err := csv.NewReader(reader).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, total+1, "header + every finding exactly once")

	seen := make(map[string]struct{}, total)
	for _, rec := range records[1:] {
		id := rec[0]
		if _, dup := seen[id]; dup {
			t.Fatalf("finding %s exported twice — cursor pagination unstable", id)
		}
		seen[id] = struct{}{}
	}
	require.Len(t, seen, total, "no finding lost across batch boundaries")
}
