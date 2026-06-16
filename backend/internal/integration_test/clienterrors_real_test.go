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

	"github.com/matharnica/vakt/internal/shared/clienterrors"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestClientErrors_RepositoryOrgScoping is the S90-2 acceptance test: Record +
// ListForOrg are org-scoped, include unscoped (pre-login) errors, and sanitize.
func TestClientErrors_RepositoryOrgScoping(t *testing.T) {
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

	var orgA, orgB string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('A','a') RETURNING id::text`).Scan(&orgA))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ('B','b') RETURNING id::text`).Scan(&orgB))

	repo := clienterrors.NewRepository(pool)

	require.NoError(t, repo.Record(ctx, clienterrors.RecordInput{OrgID: &orgA, Message: "err-A", URL: "/a"}))
	require.NoError(t, repo.Record(ctx, clienterrors.RecordInput{OrgID: &orgB, Message: "err-B", URL: "/b"}))
	// Pre-login error: no org.
	require.NoError(t, repo.Record(ctx, clienterrors.RecordInput{Message: "err-prelogin", URL: "/login"}))
	// Sanitization: ANSI + control chars must be stripped on store.
	require.NoError(t, repo.Record(ctx, clienterrors.RecordInput{OrgID: &orgA, Message: "boom\x1b[31m\x00evil", URL: "/x"}))

	listA, err := repo.ListForOrg(ctx, orgA)
	require.NoError(t, err)
	msgs := map[string]bool{}
	for _, e := range listA {
		msgs[e.Message] = true
	}
	assert.True(t, msgs["err-A"], "org A sees its own error")
	assert.True(t, msgs["err-prelogin"], "org A sees unscoped pre-login errors")
	assert.False(t, msgs["err-B"], "org A must NOT see org B's error")

	// Sanitized message: ANSI sequence + NUL removed.
	for _, e := range listA {
		if strings.HasPrefix(e.Message, "boom") {
			assert.NotContains(t, e.Message, "\x1b", "ANSI escape must be stripped")
			assert.NotContains(t, e.Message, "\x00", "NUL must be stripped")
			assert.Contains(t, e.Message, "evil")
		}
	}

	listB, err := repo.ListForOrg(ctx, orgB)
	require.NoError(t, err)
	for _, e := range listB {
		assert.NotEqual(t, "err-A", e.Message, "org B must NOT see org A's error")
	}
}
