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

	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/onboarding"
)

// TestOnboardingProgress_DerivedFromData is the S89-5 acceptance test: each step
// flips to "done" based on real org data, the percentage is computed, and the
// dismiss flag is read.
func TestOnboardingProgress_DerivedFromData(t *testing.T) {
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

	var orgID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme','acme') RETURNING id::text`).Scan(&orgID))

	stepDone := func(p onboarding.Progress, key string) bool {
		for _, s := range p.Steps {
			if s.Key == key {
				return s.Done
			}
		}
		t.Fatalf("step %q not found", key)
		return false
	}

	// Empty org → nothing done.
	p, err := onboarding.GetProgress(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Equal(t, 7, p.Total)
	assert.Equal(t, 0, p.CompletedCount)
	assert.Equal(t, 0, p.PercentDone)
	assert.False(t, p.AllComplete)
	assert.False(t, p.Dismissed)

	// Add a risk → "risks" step flips to done.
	_, err = pool.Exec(ctx,
		`INSERT INTO ck_risks (org_id, title, likelihood, impact) VALUES ($1::uuid,'R1',3,3)`, orgID)
	require.NoError(t, err)

	// Add a framework → "framework" step done.
	_, err = pool.Exec(ctx, `INSERT INTO ck_frameworks (org_id, name) VALUES ($1::uuid,'ISO 27001')`, orgID)
	require.NoError(t, err)

	p, err = onboarding.GetProgress(ctx, pool, orgID)
	require.NoError(t, err)
	assert.True(t, stepDone(p, "risks"))
	assert.True(t, stepDone(p, "framework"))
	assert.False(t, stepDone(p, "scope"))
	assert.False(t, stepDone(p, "assets"))
	assert.Equal(t, 2, p.CompletedCount)
	assert.Equal(t, 2*100/7, p.PercentDone)

	// Dismiss via the existing flag → reflected in progress.
	_, err = pool.Exec(ctx, `UPDATE organizations SET onboarding_dismissed = true WHERE id = $1::uuid`, orgID)
	require.NoError(t, err)
	p, err = onboarding.GetProgress(ctx, pool, orgID)
	require.NoError(t, err)
	assert.True(t, p.Dismissed)
}
