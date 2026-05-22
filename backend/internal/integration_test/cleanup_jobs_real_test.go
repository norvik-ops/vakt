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
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/auth"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
)

// Sprint 22 / S22-14: integration tests for the two daily/weekly cleanup
// jobs that landed in Sprint 22. Both tests boot a real Postgres via
// testcontainers, run every migration, seed an expired row, run the job,
// and assert the row is gone.
//
// Run with:
//   go test -tags=integration ./internal/integration_test/...

func bootPostgres(t *testing.T) (*pgxpool.Pool, func()) {
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
			t.Skipf("integration: Docker unavailable in this environment (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	teardown := func() {
		pool.Close()
		_ = pgC.Terminate(ctx)
	}
	return pool, teardown
}

// TestCleanupAnonymousRuns_DeletesExpiredRows verifiziert dass der
// tägliche Cleanup-Job für anonyme NIS2-Wizard-Runs nur abgelaufene Rows
// löscht und frische Runs unberührt lässt.
func TestCleanupAnonymousRuns_DeletesExpiredRows(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	// Seed: 1 abgelaufener Run (vor 1 Tag), 1 noch gültiger Run (in 7 Tagen).
	_, err := pool.Exec(ctx, `
		INSERT INTO nis2_anonymous_runs (token, expires_at)
		VALUES
			('expired-token-1', NOW() - INTERVAL '1 day'),
			('fresh-token-2',   NOW() + INTERVAL '7 days')
	`)
	require.NoError(t, err)

	// Run cleanup.
	require.NoError(t, nis2wizard.CleanupAnonymousRuns(ctx, pool))

	// Abgelaufener Run muss weg, frischer Run muss bleiben.
	var remaining []string
	rows, err := pool.Query(ctx, `SELECT token FROM nis2_anonymous_runs ORDER BY token`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var token string
		require.NoError(t, rows.Scan(&token))
		remaining = append(remaining, token)
	}
	require.Equal(t, []string{"fresh-token-2"}, remaining,
		"cleanup should delete expired runs and keep fresh ones")
}

// TestCleanupLoginHistory_DeletesOldEntries verifiziert dass der
// wöchentliche Cleanup-Job alle login_history-Einträge älter als 90 Tage
// löscht — und neuere Einträge unangetastet lässt.
func TestCleanupLoginHistory_DeletesOldEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	// Brauchen eine Org, damit org_id FK greift (Spalte ist NULLable, aber
	// wir nehmen einen realen Pfad für die Story-Vollständigkeit).
	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme')
		RETURNING id::text
	`).Scan(&orgID))

	// Seed: 1 alter Eintrag (vor 100 Tagen), 1 frischer Eintrag (heute).
	_, err := pool.Exec(ctx, `
		INSERT INTO login_history (org_id, email, source, result, ts)
		VALUES
			($1::uuid, 'old@example.com',   'password', 'ok', NOW() - INTERVAL '100 days'),
			($1::uuid, 'fresh@example.com', 'password', 'ok', NOW())
	`, orgID)
	require.NoError(t, err)

	// Run cleanup.
	require.NoError(t, auth.CleanupLoginHistory(ctx, pool))

	// Alter Eintrag weg, frischer bleibt.
	var remaining []string
	rows, err := pool.Query(ctx, `SELECT email FROM login_history ORDER BY email`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var email string
		require.NoError(t, rows.Scan(&email))
		remaining = append(remaining, email)
	}
	require.Equal(t, []string{"fresh@example.com"}, remaining,
		"cleanup should delete entries older than 90d and keep newer ones")
}
