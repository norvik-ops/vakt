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

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/pdfutil/pdftest"
)

// TestZahlenKonsistent pins Kernweg 6 („Fortschritt stimmt überall"): for one
// data set the dashboard (score + aggregate, which the Comply overview reads
// too), the framework list card, the framework detail (readiness report) and
// the PDFs show the same counts and the same rounded percentage.
//
// The data is chosen so every percentage is exactly x,5: 1 of 4 controls
// partial = 12,5 % per framework and pooled. The PDFs used fmt "%.0f" (round
// half to even → 12 %), the dashboard rounded half up (13 %) — the same value
// printed as two numbers (UX-15 rest from #212). The board report truncated
// with int() on top.
//
// NIS2's partial control has no manual status, only one evidence row — the
// path by which a policy acceptance turns NIS2 controls "In Bearbeitung"
// (UX-17). It must count exactly like a manual in_progress.
func TestZahlenKonsistent(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('ZahlenOrg', 'zahlenorg')
		RETURNING id::text`).Scan(&orgID))

	// ISO: 0 umgesetzt, 1 in Bearbeitung (manuell), 3 offen, 1 nicht anwendbar.
	iso := seedFramework(ctx, t, pool, orgID, "ISO27001")
	seedControl(ctx, t, pool, orgID, iso, "A.5.1", "Org", "in_progress")
	for _, id := range []string{"A.5.2", "A.5.3", "A.5.4"} {
		seedControl(ctx, t, pool, orgID, iso, id, "Org", "")
	}
	na := seedControl(ctx, t, pool, orgID, iso, "A.5.5", "Org", "")
	_, err = pool.Exec(ctx, `UPDATE ck_controls SET not_applicable = true WHERE id = $1::uuid AND org_id = $2::uuid`, na, orgID)
	require.NoError(t, err)

	// NIS2: 0 umgesetzt, 1 in Bearbeitung nur über einen Nachweis, 3 offen.
	nis2 := seedFramework(ctx, t, pool, orgID, "NIS2")
	derived := seedControl(ctx, t, pool, orgID, nis2, "NIS2-A.1", "Gov", "")
	seedEvidence(ctx, t, pool, orgID, derived)
	for _, id := range []string{"NIS2-A.2", "NIS2-A.3", "NIS2-A.4"} {
		seedControl(ctx, t, pool, orgID, nis2, id, "Gov", "")
	}

	readiness := vaktcomply.NewDashboardReadiness(pool)
	dash := dashboard.NewService(pool, readiness)
	svc := vaktcomply.NewService(pool)

	const wantPct = 13 // 12,5 % halb aufgerundet
	const wantText = "13%"

	// Dashboard-Kopf (Umsetzungsgrad) — auch die Comply-Übersicht liest ihn.
	score, err := dash.LoadScore(ctx, orgID)
	require.NoError(t, err)
	require.InDelta(t, 12.5, score.Readiness.ScorePct, 0.0001)
	require.Equal(t, wantPct, score.Score, "Dashboard rundet 12,5 auf 13")
	require.Equal(t, 0, score.Readiness.Covered)
	require.Equal(t, 2, score.Readiness.Partial)
	require.Equal(t, 8, score.Readiness.Applicable)

	// Dashboard-Aggregat: „Framework-Fortschritt" und „Compliance-Fortschritt".
	agg, err := dash.LoadAggregate(ctx, orgID)
	require.NoError(t, err)
	require.Equal(t, score.Readiness, agg.OverallReadiness)
	aggByID := map[string]dashboard.FrameworkScore{}
	for _, fs := range agg.FrameworkScores {
		aggByID[fs.FrameworkID] = fs
	}

	// Framework-Liste (Karten auf /vaktcomply/frameworks).
	list, err := svc.ListFrameworks(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, list, 2)

	for _, fw := range list {
		// Framework-Detail: Kopf und Zählung kommen aus dem Readiness-Report.
		report, err := svc.GetReadinessReport(ctx, orgID, fw.ID)
		require.NoError(t, err)
		applicable := report.TotalControls - report.NotApplicable
		a := aggByID[fw.ID]

		require.Equal(t, 0, report.Covered, fw.Name)
		require.Equal(t, 1, report.Partial, "%s: der eine Kontrollpunkt in Bearbeitung zählt", fw.Name)
		require.Equal(t, 4, applicable, fw.Name)
		require.InDelta(t, 12.5, report.ReadinessScore, 0.0001, fw.Name)

		require.Equal(t, report.Covered, a.ImplementedControls, "%s: Dashboard ≠ Detail (umgesetzt)", fw.Name)
		require.Equal(t, report.Partial, a.PartialControls, "%s: Dashboard ≠ Detail (in Bearbeitung)", fw.Name)
		require.Equal(t, applicable, a.TotalControls, "%s: Dashboard ≠ Detail (anwendbar)", fw.Name)
		require.InDelta(t, report.ReadinessScore, a.ScorePct, 0.0001, fw.Name)

		require.Equal(t, report.Covered, fw.ImplementedControls, "%s: Karte ≠ Detail (umgesetzt)", fw.Name)
		require.Equal(t, report.Partial, fw.PartialControls, "%s: Karte ≠ Detail (in Bearbeitung)", fw.Name)
		require.Equal(t, applicable, fw.ApplicableControls, "%s: Karte ≠ Detail (anwendbar)", fw.Name)
		require.InDelta(t, report.ReadinessScore, fw.ReadinessScore, 0.0001, fw.Name)

		// Framework-PDF: dieselbe Zahl, dieselbe Rundung.
		pdf, err := vaktcomply.GenerateFrameworkPDF(report, nil, "ZahlenOrg")
		require.NoError(t, err)
		require.Contains(t, pdfTexts(t, pdf), wantText, "%s: Framework-PDF rundet anders als das Dashboard", fw.Name)
	}

	// Executive Summary (Comply-Übersicht → „Executive Summary").
	exec, err := svc.GetExecutiveSummaryData(ctx, orgID)
	require.NoError(t, err)
	require.InDelta(t, score.Readiness.ScorePct, exec.OverallScore, 0.0001)
	execPDF, err := vaktcomply.GenerateExecutiveSummaryPDF(exec)
	require.NoError(t, err)
	require.Contains(t, pdfTexts(t, execPDF), wantText, "Executive-Summary-PDF rundet anders als das Dashboard")

	// Management-Bericht: schnitt mit int() ab.
	board, err := svc.GetBoardReportData(ctx, orgID)
	require.NoError(t, err)
	require.Equal(t, wantPct, board.Score, "Management-Bericht ≠ Dashboard")

	// Audit-Bericht (Comply-Übersicht → „Audit-Bericht generieren").
	report, err := audit.Collect(ctx, pool, orgID, readiness)
	require.NoError(t, err)
	require.InDelta(t, score.Readiness.ScorePct, report.OverallScorePct, 0.0001,
		"Audit-Bericht: Umsetzungsgrad ≠ Dashboard (vorher Mittelwert über manual_status)")
	require.Equal(t, 8, report.ControlStats.Total)
	require.Equal(t, 0, report.ControlStats.Implemented)
	require.Equal(t, 2, report.ControlStats.InProgress, "Audit-Bericht zählt den abgeleiteten NIS2-Status mit")
	for _, fw := range report.Frameworks {
		a := aggByID[fw.ID]
		require.Equal(t, a.ImplementedControls, fw.Implemented, fw.Name)
		require.Equal(t, a.PartialControls, fw.InProgress, fw.Name)
		require.Equal(t, a.TotalControls, fw.TotalControls, fw.Name)
	}
	auditPDF, err := audit.Render(report)
	require.NoError(t, err)
	require.Contains(t, pdfTexts(t, auditPDF), wantText, "Audit-Bericht-PDF rundet anders als das Dashboard")
}

// TestEUAIActDashboard_DossierUnvollstaendig pins UX-35: a draft dossier with
// empty mandatory fields counted as "Dossier vorhanden" ("Ohne Dossier: 0").
func TestEUAIActDashboard_DossierUnvollstaendig(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('AIOrg', 'aiorg')
		RETURNING id::text`).Scan(&orgID))

	system := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_ai_systems (org_id, name) VALUES ($1::uuid, $2)
			RETURNING id::text`, orgID, name).Scan(&id))
		return id
	}
	// filled = alle neun Pflichtfelder; draft = nur zwei Felder.
	doc := func(sysID string, version int, status string, full bool) {
		fill := func(v string) any {
			if full {
				return v
			}
			return nil
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO ck_ai_documentation
			  (org_id, ai_system_id, version, system_description, intended_purpose,
			   training_data, data_quality, performance_metrics, system_limits,
			   risk_management, human_oversight, logging_audit_trail, status)
			VALUES ($1::uuid, $2::uuid, $3, 'Beschreibung', 'Zweck', $4, $4, $4, $4, $4, $4, $4, $5)`,
			orgID, sysID, version, fill("Text"), status)
		require.NoError(t, err)
	}

	system("Ohne Dossier")
	draft := system("Entwurf mit Lücken")
	doc(draft, 1, "draft", false)
	finalGaps := system("Final, aber Felder leer")
	doc(finalGaps, 1, "final", false)
	fullDraft := system("Alles ausgefüllt, noch Entwurf")
	doc(fullDraft, 1, "draft", true)
	done := system("Fertig")
	doc(done, 1, "final", true)
	regressed := system("Neue Version wieder Entwurf")
	doc(regressed, 1, "final", true)
	doc(regressed, 2, "draft", false)

	d, err := vaktcomply.NewService(pool).GetEUAIActDashboard(ctx, orgID)
	require.NoError(t, err)
	require.Equal(t, 6, d.TotalSystems)
	require.Equal(t, 1, d.SystemsWithoutDocs, "nur ein System hat gar kein Dossier")
	require.Equal(t, 5, d.SystemsIncompleteDocs,
		"unvollständig: ohne Dossier, Entwurf, leere Pflichtfelder, neueste Version Entwurf — nur „Fertig“ zählt nicht")
}

// pdfTexts concatenates all text runs of a PDF, separated by newlines.
func pdfTexts(t *testing.T, pdf []byte) string {
	t.Helper()
	var b strings.Builder
	for _, r := range pdftest.TextRuns(t, pdf) {
		b.WriteString(r.Text)
		b.WriteByte('\n')
	}
	return b.String()
}
