//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// migrationsDir returns the absolute path to backend/db/migrations from the
// test file's runtime location (works regardless of working directory).
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// here = .../backend/internal/integration_test/hr_evidence_real_test.go
	// → ../../db/migrations
	return filepath.Join(filepath.Dir(here), "..", "..", "db", "migrations")
}

// TestHRChecklistCompletion_CreatesEvidence boots a real Postgres via
// testcontainers, runs every migration, and exercises the SecHR → SecVitals
// evidence flow end-to-end:
//
//  1. Create an org + employee + onboarding checklist with one required step
//  2. Start a run via the HR service
//  3. Complete the step → run auto-transitions to "completed"
//  4. Assert: one row in ck_evidence with source = "hr_checklist_completed"
//
// This is the canonical regression test for the cross-module Evidence-Writer
// wiring. If the HR service stops calling the evidence writer, or the writer
// stops writing the correct source string, this test goes red.
func TestHRChecklistCompletion_CreatesEvidence(t *testing.T) {
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
		// Sandboxes without Docker daemon access (e.g. some CI runners and
		// the local dev box's default config) cannot start containers. Skip
		// cleanly rather than failing — the test exists for CI environments
		// that have Docker, and lets the dev verify locally when they do.
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

	// Seed: organization + employee + onboarding checklist.
	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme')
		RETURNING id::text
	`).Scan(&orgID))

	hrRepo := vakthr.NewRepository(pool)
	hrEvidence := vaktcomply.NewHREvidenceWriter(pool)
	hrSvc := vakthr.NewService(hrRepo).WithEvidenceWriter(hrEvidence)

	actor := vakthr.Actor{OrgID: orgID, UserID: "", UserEmail: "test@acme.de", IPAddress: "127.0.0.1"}

	emp, err := hrSvc.CreateEmployee(ctx, actor, vakthr.CreateEmployeeInput{
		FirstName: "Max",
		LastName:  "Mustermann",
		Email:     "max@acme.de",
	})
	require.NoError(t, err)

	checklist, err := hrSvc.CreateChecklist(ctx, actor, vakthr.CreateChecklistInput{
		Type: "onboarding",
		Name: "Standard-Onboarding",
		Items: []vakthr.ChecklistItem{
			{ID: "step-1", Label: "Account erstellen", Required: true},
		},
	})
	require.NoError(t, err)

	run, err := hrSvc.StartChecklistRun(ctx, actor, vakthr.StartChecklistRunInput{
		EmployeeID:  emp.ID,
		ChecklistID: checklist.ID,
	})
	require.NoError(t, err)

	// Complete the required step — should transition run → "completed" AND
	// fire the evidence writer.
	updated, err := hrSvc.CompleteStep(ctx, actor, run.ID, "step-1", actor.UserEmail)
	require.NoError(t, err)
	require.Equal(t, "completed", updated.Status, "run should be completed once all required steps are done")

	// Assertion: exactly one ck_evidence row for this org with the right source.
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_evidence
		WHERE org_id = $1::uuid
		  AND source = 'hr_checklist_completed'
	`, orgID).Scan(&count))
	require.Equal(t, 1, count, "exactly one HR-completion evidence row must exist")
}
