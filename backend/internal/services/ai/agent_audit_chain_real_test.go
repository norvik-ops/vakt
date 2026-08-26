//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// Regression test for R1-27-V02 / ESK-4.
//
// The AI agent was the only writer that reached audit_log with a raw INSERT,
// leaving prev_hash and entry_hash NULL. Its rows hung on no chain at all, and
// VerifyOrgChain skipped them silently and then declared the org intact.
//
// This test drives both agent write paths against real Postgres and asserts the
// rows are chained. Reverting either helper to a raw `INSERT INTO audit_log`
// turns it red twice over: entry_hash comes back NULL, and the chain replay
// reports "unverifiable" instead of "intact".

func TestAgentAuditWrites_ExtendTheHashChain(t *testing.T) {
	pool, orgID, cleanup := bootPostgresForAgentAudit(t)
	defer cleanup()
	ctx := context.Background()

	r := &AgentRunner{db: pool}
	req := AgentRunRequest{
		OrgID: orgID,
		RunID: "run-1",
		Goal:  "Alle offenen ISO-27001-Controls sichten",
	}

	r.auditAgentStart(ctx, req, "1. Controls laden\n2. Lücken melden")
	r.auditApprovedToolCall(ctx, req, "create_control", "")

	// Both rows must exist and both must carry a chain hash.
	var total, hashed int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*), count(entry_hash)
		FROM audit_log
		WHERE org_id = $1::uuid AND resource_type = 'ai/agent'`, orgID).Scan(&total, &hashed))
	assert.Equal(t, 2, total, "both agent audit writes must land")
	assert.Equal(t, 2, hashed,
		"every AI-agent audit row must carry an entry_hash — a raw INSERT leaves it NULL and the row hangs on no chain")

	// The actions must be the ones the agent claims to record, so a future
	// rename cannot make this test pass over the wrong rows.
	var actions []string
	rows, err := pool.Query(ctx, `
		SELECT action FROM audit_log
		WHERE org_id = $1::uuid AND resource_type = 'ai/agent'
		ORDER BY created_at ASC, id ASC`, orgID)
	require.NoError(t, err)
	for rows.Next() {
		var a string
		require.NoError(t, rows.Scan(&a))
		actions = append(actions, a)
	}
	rows.Close()
	require.NoError(t, rows.Err())
	assert.ElementsMatch(t, []string{"agent_run_start", "agent_tool_approved"}, actions)

	// And the chain the agent extended must replay cleanly — the whole point
	// of chaining is that the verifier can now make a statement about it.
	res, err := audit.VerifyOrgChain(ctx, pool, orgID)
	require.NoError(t, err)
	assert.Equal(t, audit.ChainIntact, res.Status,
		"agent-written rows must be verifiable, got %+v", res)
	assert.Equal(t, 2, res.Verified)
	assert.Zero(t, res.Unverifiable,
		"an unchained agent row would show up here as unverifiable, not as intact")
}

// bootPostgresForAgentAudit boots Postgres, runs every migration and seeds one
// org. Kept local to this package on purpose: the integration_test helpers live
// in _test.go files and are not importable from here.
func bootPostgresForAgentAudit(t *testing.T) (*pgxpool.Pool, string, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"postgres:16.14-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777",
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
	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// here = .../backend/internal/services/ai/<file> → ../../../db/migrations
	migrations := filepath.Join(filepath.Dir(here), "..", "..", "..", "db", "migrations")
	require.NoError(t, shareddb.RunMigrations(dsn, migrations))

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)

	var orgID string
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO organizations (name, slug) VALUES ('AgentAuditOrg', 'agentauditorg')
		RETURNING id::text`).Scan(&orgID))

	return pool, orgID, func() {
		pool.Close()
		_ = pgC.Terminate(context.Background())
	}
}
