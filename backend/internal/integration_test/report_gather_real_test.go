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

	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestReportGather_FrameworkDomainGrouping verifies that audit.Collect groups
// controls into frameworks → domains correctly when loaded with the single
// org-wide query (S98-9: was an N+1, one controls query per framework). It
// seeds two frameworks with controls across multiple domains plus evidence and
// asserts the resulting structure.
func TestReportGather_FrameworkDomainGrouping(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('ReportOrg', 'reportorg')
		RETURNING id::text`).Scan(&orgID))

	// Framework A: 3 controls across 2 domains (2 in "Access", 1 in "Risk").
	// Framework B: 1 control in "Ops".
	fwA := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	fwB := seedFramework(ctx, t, pool, orgID, "NIS2")

	cA1 := seedControl(ctx, t, pool, orgID, fwA, "A-1", "Access", "implemented")
	seedControl(ctx, t, pool, orgID, fwA, "A-2", "Access", "in_progress")
	seedControl(ctx, t, pool, orgID, fwA, "A-3", "Risk", "")
	seedControl(ctx, t, pool, orgID, fwB, "B-1", "Ops", "implemented")

	// Two evidence rows on control A-1 → evidence_count must be 2.
	seedEvidence(ctx, t, pool, orgID, cA1)
	seedEvidence(ctx, t, pool, orgID, cA1)

	data, err := audit.Collect(ctx, pool, orgID)
	require.NoError(t, err)
	require.Len(t, data.Frameworks, 2, "two frameworks expected")

	byName := map[string]audit.FrameworkSection{}
	for _, f := range data.Frameworks {
		byName[f.Name] = f
	}

	a := byName["ISO 27001"]
	require.Equal(t, 3, a.TotalControls)
	require.Equal(t, 1, a.Implemented)
	require.Equal(t, 1, a.InProgress)
	require.Equal(t, 1, a.NotStarted)
	// Domains: "Access" (2 controls) + "Risk" (1 control).
	domA := map[string]audit.DomainSection{}
	for _, d := range a.Domains {
		domA[d.Name] = d
	}
	require.Len(t, domA, 2, "framework A must have 2 domains")
	require.Len(t, domA["Access"].Controls, 2)
	require.Len(t, domA["Risk"].Controls, 1)

	// evidence_count on A-1 must be 2 (the N+1 collapse must not lose the join).
	var a1 audit.ControlRow
	for _, c := range domA["Access"].Controls {
		if c.ControlID == "A-1" {
			a1 = c
		}
	}
	require.Equal(t, 2, a1.EvidenceCount, "A-1 must aggregate 2 evidence rows")

	b := byName["NIS2"]
	require.Equal(t, 1, b.TotalControls)
	require.Len(t, b.Domains, 1)
	require.Equal(t, "Ops", b.Domains[0].Name)
}

func seedFramework(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID, name string) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name) VALUES ($1::uuid, $2)
		RETURNING id::text`, orgID, name).Scan(&id))
	return id
}

func seedControl(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID, fwID, controlID, domain, status string) string {
	t.Helper()
	var id string
	var st any
	if status != "" {
		st = status
	}
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_controls (org_id, framework_id, control_id, title, domain, manual_status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		RETURNING id::text`, orgID, fwID, controlID, controlID+" title", domain, st).Scan(&id))
	return id
}

func seedEvidence(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID, controlID string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO ck_evidence (org_id, control_id, title) VALUES ($1::uuid, $2::uuid, 'ev')`,
		orgID, controlID)
	require.NoError(t, err)
}
