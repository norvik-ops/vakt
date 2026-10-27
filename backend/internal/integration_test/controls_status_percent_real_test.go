//go:build integration

package integration_test

import (
	"context"
	"fmt"
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

// TestControlsStatusAndPercent_OneSourceEverywhere pins Arbeitsvorrat-Paket 5
// (Kernweg 6, docs/launch-gate.md): a control shows the same status and a
// framework the same percentage wherever the user looks. The canonical source
// is ADR-0086 — policy.ResolveStatus (not_applicable > manual_status >
// evidence count) and policy.ReadinessScore (partial half-weight,
// not_applicable out of the denominator).
//
//   - R1-KW-10: GET /controls/:id returned the control without `status`, so the
//     detail page fell back to "Offen" while the framework list said
//     "Umgesetzt".
//   - R1-KW-5: the cursor branch of GET /frameworks/:id/controls (the one that
//     can page through all 93 ISO controls) returned rows without `status`.
//   - R1-KW-6: the dashboard counted manual_status = implemented over ALL
//     controls (evidence ignored, not_applicable in the denominator) — a
//     different number from the framework detail as soon as either occurs.
func TestControlsStatusAndPercent_OneSourceEverywhere(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('Paket5Org', 'paket5org')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO27001")

	// 30 controls: more than one page of 25, like ISO 27001 with 93.
	ids := make([]string, 30)
	for i := range ids {
		status := ""
		switch i {
		case 0:
			status = "implemented" // C-01: set "Umgesetzt" in the list
		case 4:
			status = "in_progress" // C-05: partially implemented → half weight
		}
		ids[i] = seedControl(ctx, t, pool, orgID, fwID, fmt.Sprintf("C-%02d", i+1), "Governance", status)
	}
	// C-02: two pieces of evidence, no manual status → "covered".
	seedEvidence(ctx, t, pool, orgID, ids[1])
	seedEvidence(ctx, t, pool, orgID, ids[1])
	// C-03: not applicable → leaves the denominator.
	_, err = pool.Exec(ctx, `UPDATE ck_controls SET not_applicable = true WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, ids[2])
	require.NoError(t, err)

	svc := vaktcomply.NewService(pool)

	// R1-KW-10 — the control detail carries the effective status.
	t.Run("R1-KW-10 Detail-Status", func(t *testing.T) {
		c01, err := svc.GetControl(ctx, orgID, ids[0])
		require.NoError(t, err)
		require.Equal(t, "implemented", c01.Status, "R1-KW-10: Detail muss den wirksamen Status tragen (manual_status)")
		c02, err := svc.GetControl(ctx, orgID, ids[1])
		require.NoError(t, err)
		require.Equal(t, "covered", c02.Status, "R1-KW-10: Detail muss den aus Nachweisen abgeleiteten Status tragen")
		require.Equal(t, 2, c02.EvidenceCount)
	})

	// R1-KW-5 — the cursor branch carries the same status as the list.
	t.Run("R1-KW-5 Cursor-Status", func(t *testing.T) {
		page, err := svc.ListControlsCursor(ctx, orgID, fwID, "", "", 25)
		require.NoError(t, err)
		require.Len(t, page, 26, "Cursor-Seite liefert limit+1 Zeilen (has_more-Erkennung)")
		byControlID := map[string]string{}
		for _, c := range page {
			byControlID[c.ControlID] = c.Status
		}
		require.Equal(t, "implemented", byControlID["C-01"], "R1-KW-5: Cursor-Zweig ohne Status")
		require.Equal(t, "covered", byControlID["C-02"])
		require.Equal(t, "not_applicable", byControlID["C-03"])
		require.Equal(t, "missing", byControlID["C-04"])
	})

	// R1-KW-6 — overview, detail and dashboard show the same number.
	t.Run("R1-KW-6 eine Zahl", func(t *testing.T) {
		report, err := svc.GetReadinessReport(ctx, orgID, fwID)
		require.NoError(t, err)
		// (1 implemented + 1 covered + 0.5 in_progress) / 29 applicable.
		require.InDelta(t, 2.5/29.0*100, report.ReadinessScore, 0.001)

		fws, err := svc.ListFrameworks(ctx, orgID)
		require.NoError(t, err)
		require.Len(t, fws, 1)
		require.InDelta(t, report.ReadinessScore, fws[0].ReadinessScore, 0.001, "Übersicht ≠ Detail")

		agg, err := dashboard.NewService(pool, vaktcomply.NewDashboardReadiness(pool)).LoadAggregate(ctx, orgID)
		require.NoError(t, err)
		require.Len(t, agg.FrameworkScores, 1)
		fs := agg.FrameworkScores[0]
		require.InDelta(t, report.ReadinessScore, fs.ScorePct, 0.001, "R1-KW-6: Dashboard ≠ Detail")
		require.Equal(t, 2, fs.ImplementedControls, "Dashboard zählt umgesetzt = implemented + durch Nachweise abgedeckt")
		require.Equal(t, 29, fs.TotalControls, "Dashboard-Nenner = anwendbare Kontrollen")
	})

	// Subreview PR #182 — the dashboard's overall number is pooled per
	// ADR-0086 over all frameworks (sums, then one ReadinessScore), with the
	// in_progress control at half weight and the not_applicable one outside
	// the denominator. Second framework: 2 controls, 1 implemented.
	t.Run("R1-KW-6 Dashboard-Gesamtzahl gepoolt", func(t *testing.T) {
		fw2 := seedFramework(ctx, t, pool, orgID, "NIS2")
		seedControl(ctx, t, pool, orgID, fw2, "N-1", "Governance", "implemented")
		seedControl(ctx, t, pool, orgID, fw2, "N-2", "Governance", "")

		agg, err := dashboard.NewService(pool, vaktcomply.NewDashboardReadiness(pool)).LoadAggregate(ctx, orgID)
		require.NoError(t, err)
		o := agg.OverallReadiness
		require.Equal(t, 3, o.Covered)
		require.Equal(t, 1, o.Partial)
		require.Equal(t, 31, o.Applicable, "nicht anwendbare Kontrolle zählt nicht")
		require.InDelta(t, 3.5/31.0*100, o.ScorePct, 0.001, "gepoolt, Teil zählt halb")
	})
}
