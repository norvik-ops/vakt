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

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestCollectorEvidenceAndMeasures_Idempotent pins die Duplikat-Klasse:
// R1-19-W03 (taeglicher Cron comply:bcm_evidence_sync legt bei jedem Lauf eine
// identische Evidenz-Zeile an, der Kommentar darueber behauptet "idempotently
// ... upsert-safe"), R1-20-A10 (jeder Trainings-Report-Export schreibt zehn
// identische Zeilen — gleiche Senke, anderer Produzent) und R1-06-D07
// (ck_control_measures waechst bei JEDEM API-Start um 23 Zeilen, weil
// SeedCKMeasure ein ON CONFLICT DO NOTHING ohne Arbiter auf einer Tabelle ohne
// natuerlichen Schluessel benutzt).
//
// Beide Senken bekommen den fehlenden Schluessel (Migration 257); die beiden
// Queries werden zu echten Upserts. Ohne Migration 257 legt der zweite Aufruf
// jeweils eine zweite Zeile an und beide Zaehl-Zusicherungen fallen.
func TestCollectorEvidenceAndMeasures_Idempotent(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('DupOrg', 'duporg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "BSI")
	ctrlID := seedControl(ctx, t, pool, orgID, fwID, "DER.4.A4", "BCM", "")

	repo := vaktcomply.NewRepository(pool)

	countEvidence := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM ck_evidence WHERE org_id = $1::uuid`, orgID).Scan(&n))
		return n
	}
	countMeasures := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM ck_control_measures WHERE org_id = $1::uuid`, orgID).Scan(&n))
		return n
	}

	// ── R1-19-W03 / R1-20-A10: dieselbe Collector-Evidenz dreimal ────────────
	// Genau das, was der taegliche BCM-Cron tut: gleiche Org, gleiches Control,
	// gleiche Quelle, gleicher Titel — nur der Nutzdaten-Inhalt aendert sich.
	const title = "BIA: Kritische Prozesse dokumentiert"
	for i, payload := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
		_, err := repo.AddCollectorEvidence(ctx, orgID, ctrlID, "", "automated", title, []byte(payload))
		require.NoError(t, err, "Lauf %d", i+1)
	}
	assert.Equal(t, 1, countEvidence(),
		"drei Laeufe desselben Collectors duerfen eine Zeile ergeben, nicht drei")

	// Der Upsert muss den Inhalt auffrischen, nicht nur den zweiten Schreib-
	// versuch verwerfen — sonst waere die Evidenz nach dem ersten Lauf
	// eingefroren und ein Upsert nur ein getarntes DO NOTHING.
	var stored string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT collector_data::text FROM ck_evidence WHERE org_id = $1::uuid`, orgID).Scan(&stored))
	assert.JSONEq(t, `{"n":3}`, stored, "der Upsert muss den juengsten Stand tragen")

	// Ein anderer Titel ist eine andere Tatsache und muss eine eigene Zeile
	// bekommen — sonst haette der Fix die Senke zugemauert statt entdoppelt.
	_, err = repo.AddCollectorEvidence(ctx, orgID, ctrlID, "", "automated",
		"Wiederanlaufplan (WAP) vorhanden", []byte(`{"n":1}`))
	require.NoError(t, err)
	assert.Equal(t, 2, countEvidence(), "ein anderer Titel ist eine eigene Evidenz")

	// Manuelle Uploads bleiben frei: zwei Dateien duerfen denselben Titel
	// tragen. Der Index klammert source = 'manual' bewusst aus.
	for range 2 {
		_, err := pool.Exec(ctx, `
			INSERT INTO ck_evidence (control_id, org_id, title, source)
			VALUES ($1::uuid, $2::uuid, 'Backup-Protokoll', 'manual')`, ctrlID, orgID)
		require.NoError(t, err, "manuelle Uploads duerfen nicht kollidieren")
	}
	assert.Equal(t, 4, countEvidence())

	// ── R1-06-D07: Builtin-Massnahmen mehrfach seeden ────────────────────────
	measures := []vaktcomply.CreateMeasureInput{
		{Title: "Richtliniendokument erstellen", Description: "…", Difficulty: "easy"},
		{Title: "Freigabe durch Geschäftsführung einholen", Description: "…", Difficulty: "easy"},
	}
	for i := range 3 {
		require.NoError(t, repo.SeedMeasuresForControl(ctx, orgID, ctrlID, measures), "Start %d", i+1)
	}
	assert.Equal(t, 2, countMeasures(),
		"drei API-Starts duerfen die Massnahmen nicht verdreifachen")

	// Eine selbst angelegte Massnahme (is_builtin = false) darf denselben Titel
	// tragen wie eine eingebaute — der Index deckt nur die eingebauten.
	_, err = pool.Exec(ctx, `
		INSERT INTO ck_control_measures (control_id, org_id, title, is_builtin)
		VALUES ($1::uuid, $2::uuid, 'Richtliniendokument erstellen', FALSE)`, ctrlID, orgID)
	require.NoError(t, err, "eigene Massnahmen bleiben frei")
	assert.Equal(t, 3, countMeasures())
}
