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

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestDashboardNotifications_HideSwitchedOffModules covers the R1-KW-15
// follow-up: old hints of a disabled module (scan alert, DSR reminder) must not
// stay in the bell — they link to pages that no longer exist.
func TestDashboardNotifications_HideSwitchedOffModules(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
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

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('NotifOrg', 'notiforg')
		RETURNING id::text`).Scan(&orgID))
	for _, m := range []string{"vaktcomply", "vaktscan", "vaktprivacy", "system"} {
		_, err := pool.Exec(ctx, `
			INSERT INTO user_notifications (org_id, title, body, type, module)
			VALUES ($1::uuid, $2, '', 'info', $2)`, orgID, m)
		require.NoError(t, err)
	}

	modules := func(enabled string) []string {
		cfg := &config.Config{ModulesEnabled: enabled}
		list, err := dashboard.NewService(pool, cfg.IsModuleEnabled).LoadNotifications(ctx, orgID)
		require.NoError(t, err)
		out := []string{}
		for _, n := range list {
			out = append(out, n.Module)
		}
		return out
	}

	require.ElementsMatch(t, []string{"vaktcomply", "system"}, modules("vaktcomply"))
	require.ElementsMatch(t, []string{"vaktcomply", "vaktscan", "system"}, modules("vaktcomply,vaktscan"))
}
