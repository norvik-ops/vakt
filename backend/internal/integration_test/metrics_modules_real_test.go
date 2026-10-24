//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
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

	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/metrics"
)

// TestMetrics_OmitsArchivedModules covers the R1-KW-15 follow-up: /metrics must
// not publish findings or DSR gauges of the archived Scan and Privacy modules —
// a 0 there would tell monitoring "measured, nothing open".
func TestMetrics_OmitsArchivedModules(t *testing.T) {
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

	e := echo.New()
	metrics.RegisterWithOptions(e, pool, metrics.RegisterOptions{})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	core := rec.Body.String()

	assert.NotContains(t, core, "vakt_findings_total")
	assert.NotContains(t, core, "vakt_dsr_open_total")
	assert.NotContains(t, core, "vakt_dsr_overdue_total")
	assert.Contains(t, core, "vakt_score_current", "ISMS gauges stay")
}
