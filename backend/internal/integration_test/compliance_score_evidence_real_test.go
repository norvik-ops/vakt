//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestComplianceScore_EvidenceStatusFollowsEvidence pins the second half of
// R1-14b-A1: GET /vaktcomply/compliance-score aggregates
// ck_controls.evidence_status, and that column had exactly one writer — the
// nightly 03:30 staleness cron (Repository.UpdateEvidenceStaleness). Attaching
// evidence therefore left the score at 0 % until the next night; an org with 27
// controls carrying evidence reported "0 % compliant" all day.
//
// Migration 255 recomputes evidence_status from a trigger on ck_evidence, so
// every producer — manual upload, the 30 collector call sites, the demo seed —
// is covered without a call site having to remember. The test drives the
// producers through plain INSERT/DELETE on ck_evidence, i.e. exactly the layer
// the trigger sits on, and never calls the cron.
//
// Reverting migration 255 leaves evidence_status at its 'missing' default and
// all three phases below fail.
func TestComplianceScore_EvidenceStatusFollowsEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
		INSERT INTO organizations (name, slug) VALUES ('ScoreOrg', 'scoreorg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")

	c1 := seedControl(ctx, t, pool, orgID, fwID, "S-1", "Access", "")
	seedControl(ctx, t, pool, orgID, fwID, "S-2", "Access", "")

	evStatus := func(controlID string) string {
		var s string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COALESCE(evidence_status, '') FROM ck_controls WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID,
			controlID).Scan(&s))
		return s
	}

	// Phase 1 — evidence attached: the control must count as covered
	// immediately, not after the next nightly run.
	var evID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (org_id, control_id, title) VALUES ($1::uuid, $2::uuid, 'ev')
		RETURNING id::text`, orgID, c1).Scan(&evID))
	require.Equal(t, "ok", evStatus(c1),
		"attaching evidence must move evidence_status without waiting for the 03:30 cron")

	// Phase 2 — score reflects it. Same aggregate the handler runs.
	var total, okCount, naCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE evidence_status = 'ok'),
		       COUNT(*) FILTER (WHERE evidence_status = 'na')
		  FROM ck_controls WHERE org_id = $1::uuid`, orgID).Scan(&total, &okCount, &naCount))
	require.Equal(t, 2, total)
	require.Equal(t, 1, okCount, "compliance score must not stay at 0 % with evidence attached")

	// Phase 3 — evidence removed again: the score must fall back, otherwise the
	// deliverable would over-report instead of under-reporting.
	_, err = pool.Exec(ctx,
		`DELETE FROM ck_evidence WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, evID)
	require.NoError(t, err)
	require.Equal(t, "missing", evStatus(c1),
		"removing the last evidence must reset evidence_status")
}
