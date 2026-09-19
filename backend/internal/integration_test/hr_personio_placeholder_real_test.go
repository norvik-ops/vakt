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

	"github.com/matharnica/vakt/internal/modules/vakthr"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestPersonioUpsert_SecondUnknownEmployeeNoCollision is the R1-36c-03 Teil B
// regression: the Personio `employee.departed` webhook creates a placeholder
// hr_employees row for an employee Vakt has not seen. That row used to be
// inserted with email=” — but hr_employees enforces UNIQUE(org_id, email)
// (migration 063), so the SECOND unknown departure in the same org collided on
// ” and 500'd, losing the rest of the webhook batch.
//
// The fix derives a unique placeholder email from the Personio id. This test
// upserts two DIFFERENT unknown Personio employees into ONE org and asserts both
// succeed and produce two distinct rows. Reverting the fix (email back to ”)
// turns the second upsert red on the unique-constraint violation.
func TestPersonioUpsert_SecondUnknownEmployeeNoCollision(t *testing.T) {
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
			t.Skipf("integration: Docker unavailable in this environment (%v)", err)
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
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme')
		RETURNING id::text
	`).Scan(&orgID))

	repo := vakthr.NewRepository(pool)
	departure := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	// First unknown employee — creates a placeholder row.
	id1, created1, err := repo.UpsertEmployeeByPersonioID(ctx, orgID, 42, departure)
	require.NoError(t, err, "first unknown Personio employee must upsert cleanly")
	assert.True(t, created1, "first unknown employee is a new placeholder row")
	assert.NotEmpty(t, id1)

	// Second, DIFFERENT unknown employee in the SAME org — this is the exact case
	// that used to collide on UNIQUE(org_id, email) when both got email=''.
	id2, created2, err := repo.UpsertEmployeeByPersonioID(ctx, orgID, 99, departure)
	require.NoError(t, err, "second unknown Personio employee must NOT collide (the bug)")
	assert.True(t, created2, "second unknown employee is a new placeholder row")
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "the two placeholders must be distinct rows")

	// Re-delivering the first employee's departure must find the existing row
	// (keyed on personio_employee_id), not create a duplicate.
	id1Again, created1Again, err := repo.UpsertEmployeeByPersonioID(ctx, orgID, 42, departure)
	require.NoError(t, err)
	assert.False(t, created1Again, "re-delivered webhook must match the existing row")
	assert.Equal(t, id1, id1Again, "same Personio id must resolve to the same Vakt employee")

	// Both placeholder rows carry a non-empty, distinct email.
	var email1, email2 string
	// orgid-lint: global — Test-Assertion, PK-Lookup (id) in isolierter bootPostgres-Test-Org
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT email FROM hr_employees WHERE id = $1::uuid`, id1).Scan(&email1))
	// orgid-lint: global — Test-Assertion, PK-Lookup (id) in isolierter bootPostgres-Test-Org
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT email FROM hr_employees WHERE id = $1::uuid`, id2).Scan(&email2))
	assert.NotEmpty(t, email1)
	assert.NotEmpty(t, email2)
	assert.NotEqual(t, email1, email2)
}
