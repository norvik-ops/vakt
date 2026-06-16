//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/shared/dataexport"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// readZipFiles unpacks a ZIP byte slice into a name→content map.
func readZipFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func runExport(t *testing.T, pool *pgxpool.Pool, orgID, modules string) map[string]string {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/export", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("org_id", orgID)
	require.NoError(t, dataexport.ExportHandler(pool, modules)(c))
	require.Equal(t, http.StatusOK, rec.Code)
	return readZipFiles(t, rec.Body.Bytes())
}

// TestDataExport_HRAndAwarePII is the S89-2 (PRIV-001) acceptance test: the
// org-takeout includes HR PII and pseudonymises Aware phishing results, is
// org-scoped, and respects module toggles.
func TestDataExport_HRAndAwarePII(t *testing.T) {
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

	// Org A — has HR employee + Aware target/event.
	var orgA, orgB string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('Acme','acme') RETURNING id::text`).Scan(&orgA))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('Beta','beta') RETURNING id::text`).Scan(&orgB))

	_, err = pool.Exec(ctx, `INSERT INTO hr_employees (org_id, first_name, last_name, email) VALUES ($1::uuid,'Alice','Acme','alice@acme.test')`, orgA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO hr_employees (org_id, first_name, last_name, email) VALUES ($1::uuid,'Bob','Beta','bob@beta.test')`, orgB)
	require.NoError(t, err)

	// Aware chain for org A: group → target → campaign → event.
	var groupID, targetID, campaignID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO sr_target_groups (org_id, name) VALUES ($1::uuid,'All') RETURNING id::text`, orgA).Scan(&groupID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO sr_targets (org_id, group_id, email, first_name, last_name) VALUES ($1::uuid,$2::uuid,'alice@acme.test','Alice','Acme') RETURNING id::text`, orgA, groupID).Scan(&targetID))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO sr_campaigns (org_id, name, from_name, from_email, subject) VALUES ($1::uuid,'Q1','IT','it@acme.test','Test') RETURNING id::text`, orgA).Scan(&campaignID))
	_, err = pool.Exec(ctx, `INSERT INTO sr_events (org_id, campaign_id, target_id, type, tracking_token) VALUES ($1::uuid,$2::uuid,$3::uuid,'click','tok123')`, orgA, campaignID, targetID)
	require.NoError(t, err)

	allModules := "vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,vakthr"

	// ── Full export of org A ──────────────────────────────────────────────
	files := runExport(t, pool, orgA, allModules)

	require.Contains(t, files, "hr_employees.json")
	assert.Contains(t, files["hr_employees.json"], "alice@acme.test", "HR PII must be in the export")
	assert.NotContains(t, files["hr_employees.json"], "bob@beta.test", "no foreign-org HR data")

	require.Contains(t, files, "sr_targets.json")
	assert.Contains(t, files["sr_targets.json"], "alice@acme.test", "sr_targets directory exported raw")

	require.Contains(t, files, "sr_events.json")
	// The person link must be pseudonymised — no raw target UUID, digest prefix present.
	assert.NotContains(t, files["sr_events.json"], targetID, "target_id must not appear raw in results")
	assert.Contains(t, files["sr_events.json"], "anon_", "target_id must be pseudonymised")

	// ── Org scoping: org B export has no org A data ───────────────────────
	bFiles := runExport(t, pool, orgB, allModules)
	assert.Contains(t, bFiles["hr_employees.json"], "bob@beta.test")
	assert.NotContains(t, bFiles["hr_employees.json"], "alice@acme.test", "cross-org leak")
	assert.NotContains(t, bFiles["sr_targets.json"], "alice@acme.test", "cross-org aware leak")

	// ── Module toggle: vakthr/vaktaware off → no HR/Aware files ───────────
	noHR := runExport(t, pool, orgA, "vaktscan,vaktcomply,vaktprivacy")
	assert.NotContains(t, noHR, "hr_employees.json", "HR file omitted when vakthr disabled")
	assert.NotContains(t, noHR, "sr_targets.json", "Aware file omitted when vaktaware disabled")
	// Core comply file still present.
	assert.Contains(t, noHR, "controls.json")
}
