//go:build integration

package integration_test

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestDashboardScore_IsReadiness pins UX-37 (Kernweg 6, „Fortschritt stimmt
// überall mit derselben Zahl"): the dashboard score used to be 70 + 10 per
// framework — 70/100 on an empty instance, 100/100 with three frameworks and
// 1 of 214 controls implemented. It is now the canonical org-wide readiness
// (ADR-0086) and must equal the framework detail, the dashboard's
// "Compliance-Fortschritt" card and the executive summary PDF.
func TestDashboardScore_IsReadiness(t *testing.T) {
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

	dash := dashboard.NewService(pool, vaktcomply.NewDashboardReadiness(pool))
	svc := vaktcomply.NewService(pool)

	t.Run("leere Instanz zeigt nicht 70", func(t *testing.T) {
		got, err := dash.LoadScore(ctx, orgID)
		require.NoError(t, err)
		require.Equal(t, 0, got.Score, "leere Instanz: kein Grundwert 70 mehr")
		require.False(t, got.HasData, "ohne anwendbare Kontrolle: noch keine Daten")
		require.Equal(t, int64(0), got.Components["active_frameworks"])
	})

	// 1 of 8 implemented, 1 in progress (half weight): (1 + 0.5) / 8 = 18.75 %.
	fwID := seedFramework(ctx, t, pool, orgID, "ISO27001")
	seedControl(ctx, t, pool, orgID, fwID, "S-1", "Governance", "implemented")
	seedControl(ctx, t, pool, orgID, fwID, "S-2", "Governance", "in_progress")
	for _, id := range []string{"S-3", "S-4", "S-5", "S-6", "S-7", "S-8"} {
		seedControl(ctx, t, pool, orgID, fwID, id, "Governance", "")
	}

	t.Run("1 von N umgesetzt ergibt den Readiness-Wert", func(t *testing.T) {
		got, err := dash.LoadScore(ctx, orgID)
		require.NoError(t, err)
		require.True(t, got.HasData)
		require.InDelta(t, 1.5/8.0*100, got.Readiness.ScorePct, 0.001)
		require.Equal(t, 19, got.Score, "18,75 % gerundet")
		require.Equal(t, 1, got.Readiness.Covered)
		require.Equal(t, 1, got.Readiness.Partial)
		require.Equal(t, 8, got.Readiness.Applicable)
		require.Equal(t, int64(1), got.Components["active_frameworks"])
	})

	// Three frameworks with almost nothing implemented: the old formula said
	// 100/100 here.
	fw2 := seedFramework(ctx, t, pool, orgID, "NIS2")
	fw3 := seedFramework(ctx, t, pool, orgID, "BSI")
	seedControl(ctx, t, pool, orgID, fw2, "N-1", "Governance", "")
	seedControl(ctx, t, pool, orgID, fw3, "B-1", "Governance", "")

	t.Run("Dashboard, Detail, Fortschrittskarte und PDF zeigen dieselbe Zahl", func(t *testing.T) {
		got, err := dash.LoadScore(ctx, orgID)
		require.NoError(t, err)
		require.Less(t, got.Score, 100, "drei Frameworks heben den Wert nicht mehr auf 100")
		require.InDelta(t, 1.5/10.0*100, got.Readiness.ScorePct, 0.001, "gepoolt über alle Frameworks")
		require.Equal(t, 15, got.Score)

		agg, err := dash.LoadAggregate(ctx, orgID)
		require.NoError(t, err)
		require.InDelta(t, agg.OverallReadiness.ScorePct, got.Readiness.ScorePct, 0.001, "Score ≠ Karte Compliance-Fortschritt")

		exec, err := svc.GetExecutiveSummaryData(ctx, orgID)
		require.NoError(t, err)
		require.InDelta(t, exec.OverallScore, got.Readiness.ScorePct, 0.001, "Score ≠ Executive-Summary-PDF")

		// One framework alone: the score equals that framework's readiness
		// report (the number of the framework detail and its PDF).
		report, err := svc.GetReadinessReport(ctx, orgID, fwID)
		require.NoError(t, err)
		require.Equal(t, 19, int(math.Round(report.ReadinessScore)))
	})
}
