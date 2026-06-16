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
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/auth"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestLogin_ConstantBcryptWork is the S87-3 (F-05, CWE-208) acceptance test:
// Login() must invoke bcrypt for BOTH an unknown e-mail and a known e-mail with
// a wrong password, so the response latency cannot be used to enumerate users.
// We assert via a minimum-runtime bound — a cost-12 bcrypt compare dominates the
// failed-login path, so both branches must take meaningfully longer than a bare
// DB miss would.
func TestLogin_ConstantBcryptWork(t *testing.T) {
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

	// Seed a known user with a real cost-12 bcrypt hash.
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectHorse1!"), 12)
	require.NoError(t, err)
	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id::text
	`).Scan(&orgID))
	_, err = pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, display_name, is_active)
		VALUES ('known@acme.test', $1, 'Known', TRUE)`, string(hash))
	require.NoError(t, err)

	svc := auth.NewService(pool, nil, mustKeyIntegration(t))

	// Warm up so JIT/connection setup doesn't skew the first measurement.
	_, _ = svc.Login(ctx, "warmup@acme.test", "whatever", "ua")

	measure := func(email, pw string) time.Duration {
		start := time.Now()
		_, loginErr := svc.Login(ctx, email, pw, "ua")
		elapsed := time.Since(start)
		require.Error(t, loginErr, "login must fail")
		assert.Equal(t, "invalid credentials", loginErr.Error(),
			"both failure branches must return the identical generic error")
		return elapsed
	}

	unknown := measure("does-not-exist@acme.test", "CorrectHorse1!")
	knownWrong := measure("known@acme.test", "WrongPassword9!")

	// A cost-12 bcrypt compare is the dominant cost (~tens of ms). Both branches
	// must clear a floor that a pure DB-miss (sub-millisecond) never would —
	// proving bcrypt ran in the unknown-e-mail path too.
	const bcryptFloor = 5 * time.Millisecond
	assert.Greater(t, unknown, bcryptFloor,
		"unknown e-mail must still perform bcrypt work (timing-defense)")
	assert.Greater(t, knownWrong, bcryptFloor,
		"known e-mail + wrong password performs bcrypt work")

	t.Logf("login timings: unknown=%v knownWrong=%v", unknown, knownWrong)
}
