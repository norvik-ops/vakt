//go:build integration

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

	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestFindingSLA_TerminalStateOnResolve pinnt R1-27-V03.
//
// calcFindingSLACompliance (reporting/kpi_calculator.go) weist eine als
// NIS2 Art. 21f gekennzeichnete Einhaltungsquote aus:
//
//	compliant = on_track | at_risk | resolved_on_time
//	verletzt  = overdue  | resolved_late
//
// Die beiden Zustaende, die "rechtzeitig geloest" ausdruecken, wurden von
// keinem Codepfad geschrieben. Der taegliche SLA-Cron (RunSLACheckForOrg)
// laeuft nur ueber OFFENE Findings und kann deshalb nur on_track, at_risk oder
// overdue setzen. Ein Finding, das einmal ueberfaellig war, blieb auch nach der
// Behebung fuer immer 'overdue': die Kennzahl war ein Narben-Zaehler statt
// einer Einhaltungsquote — plausibel falsch, in einem Bericht, den ein Auditor
// liest. sla_resolved_within aus derselben Migration 187 hatte ebenfalls
// weder Schreiber noch Leser.
//
// Migration 258 setzt den Endzustand beim Statuswechsel. Ohne sie faellt schon
// der erste Untertest.
func TestFindingSLA_TerminalStateOnResolve(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('SLAOrg', 'slaorg')
		RETURNING id::text`).Scan(&orgID))
	var assetID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO vb_assets (org_id, name, type) VALUES ($1::uuid, 'srv-1', 'server')
		RETURNING id::text`, orgID).Scan(&assetID))

	// seedFinding legt ein SLA-verfolgtes Finding an. dueOffset < 0 bedeutet:
	// die Frist ist bereits abgelaufen.
	seedFinding := func(title, slaStatus string, dueOffset time.Duration) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO vb_findings (org_id, asset_id, title, severity, scanner, sla_due_at, sla_status)
			VALUES ($1::uuid, $2::uuid, $3, 'high', 'trivy', NOW() + $4::interval, $5)
			RETURNING id::text`,
			orgID, assetID, title, dueOffset.String(), slaStatus).Scan(&id))
		return id
	}
	read := func(id string) (status string, within *bool) {
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COALESCE(sla_status,''), sla_resolved_within FROM vb_findings
			  WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id).Scan(&status, &within))
		return
	}

	t.Run("rechtzeitig behoben", func(t *testing.T) {
		id := seedFinding("in Frist", "on_track", 48*time.Hour)
		_, err := pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'resolved' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)

		st, within := read(id)
		assert.Equal(t, "resolved_on_time", st,
			"resolved_on_time hatte keinen Schreiber — die Quote konnte nie steigen")
		require.NotNil(t, within)
		assert.True(t, *within)
	})

	t.Run("zu spaet behoben", func(t *testing.T) {
		id := seedFinding("ueber Frist", "overdue", -48*time.Hour)
		_, err := pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'resolved' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)

		st, within := read(id)
		assert.Equal(t, "resolved_late", st)
		require.NotNil(t, within)
		assert.False(t, *within)
	})

	t.Run("Narbe verschwindet: einmal overdue, dann rechtzeitig zur Frist behoben", func(t *testing.T) {
		// Der Kern des Befunds: das Finding stand auf 'overdue', die Frist
		// liegt aber noch in der Zukunft (der Cron hatte es zuletzt kurz vor
		// Fristablauf gesehen). Ohne Endzustand blieb 'overdue' fuer immer
		// stehen und zaehlte dauerhaft gegen die Quote.
		id := seedFinding("Narbe", "overdue", 24*time.Hour)
		_, err := pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'resolved' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)

		st, _ := read(id)
		assert.Equal(t, "resolved_on_time", st,
			"nach der Behebung innerhalb der Frist darf keine Narbe zurueckbleiben")
	})

	t.Run("false_positive faellt aus der Messung", func(t *testing.T) {
		// Ein Fehlalarm war nie eine Behebungspflicht. Er als
		// 'resolved_on_time' zu zaehlen wuerde die Quote schoenen; als
		// 'resolved_late' waere er eine erfundene Verfehlung. Richtig ist:
		// raus aus dem Nenner — die Kennzahl filtert sla_status IS NOT NULL.
		id := seedFinding("Fehlalarm", "overdue", -48*time.Hour)
		_, err := pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'false_positive' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)

		st, within := read(id)
		assert.Equal(t, "", st)
		assert.Nil(t, within)
	})

	t.Run("Wiederoeffnen loest den Endzustand wieder", func(t *testing.T) {
		id := seedFinding("Rueckfall", "on_track", 48*time.Hour)
		_, err := pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'resolved' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)
		st, _ := read(id)
		require.Equal(t, "resolved_on_time", st)

		_, err = pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'open' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)
		st, within := read(id)
		assert.Equal(t, "on_track", st,
			"ein wiedereroeffnetes Finding darf nicht als geloest weiterzaehlen")
		assert.Nil(t, within)
	})

	t.Run("Finding ohne SLA-Frist bleibt unberuehrt", func(t *testing.T) {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO vb_findings (org_id, asset_id, title, severity, scanner, sla_status)
			VALUES ($1::uuid, $2::uuid, 'ohne Frist', 'low', 'trivy', NULL)
			RETURNING id::text`, orgID, assetID).Scan(&id))
		_, err := pool.Exec(ctx,
			`UPDATE vb_findings SET status = 'resolved' WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, id)
		require.NoError(t, err)

		st, within := read(id)
		assert.Equal(t, "", st, "ohne sla_due_at gibt es nichts zu bewerten")
		assert.Nil(t, within)
	})

	// Die Kennzahl selbst: dieselbe Aggregation, die calcFindingSLACompliance
	// faehrt. Ohne Endzustaende stand sie bei 25 % (drei der vier bewerteten
	// Findings standen auf 'overdue'); mit ihnen bei 75 %.
	var pct float64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT ROUND(
			100.0 * COUNT(CASE WHEN sla_status NOT IN ('overdue','resolved_late') THEN 1 END)::numeric
			/ COUNT(*), 2)
		FROM vb_findings
		WHERE org_id = $1::uuid AND sla_status IS NOT NULL`, orgID).Scan(&pct))
	assert.InDelta(t, 75.0, pct, 0.01,
		"vier bewertete Findings: drei rechtzeitig, eines zu spaet")
}
