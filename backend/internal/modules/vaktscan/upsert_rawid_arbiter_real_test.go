// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// R1-SA26-D01: UpsertSPFindingByRawID schrieb `ON CONFLICT (org_id, raw_id, scanner)`,
// aber der einzige passende Unique-Index ist partiell — `idx_vb_findings_dedup_rawid
// ... WHERE raw_id IS NOT NULL` (Migration 120). Ein partieller Index ist nur dann
// als Arbiter inferierbar, wenn sein Praedikat in der ON-CONFLICT-Klausel wiederholt
// wird. Ohne das wirft PostgreSQL SQLSTATE 42P10 ("there is no unique or exclusion
// constraint matching the ON CONFLICT specification") — fuer JEDEN Aufrufer, seit
// es die Query gibt. Betroffen: der komplette Import-Pfad (SARIF, CycloneDX, CSV,
// Wazuh) ueber service_import.go, handler_csv.go und handler_wazuh.go.
//
// Warum dieser Test ausfuehrt statt zu PREPAREn: Die Arbiter-Inferenz laeuft erst
// im Planner (`infer_arbiter_indexes`, plancat.c), nicht in der Parse-Analyse.
// PREPARE auf genau dieser Query meldet gegen das echte Schema "PREPARE" — der
// Fehler faellt ausschliesslich beim EXECUTE. Der vorhandene modulweite
// PREPARE-Gate (TestVaktscanRawSQLAgainstSchema) ist fuer diese Fehlerklasse
// per Konstruktion blind; deshalb steht dieser Test daneben, nicht darin.
//
// Geprueft wird nicht nur "kracht nicht", sondern der Zweck der Query:
//   - zweimal derselbe raw_id  ⇒ EINE Zeile, occurrence_count hochgezaehlt
//   - zweimal raw_id = NULL    ⇒ ZWEI Zeilen (genau deswegen ist der Index partiell)
func TestUpsertImportedFinding_PartialIndexArbiter(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	// pool.Close() per t.Cleanup, NICHT per defer: Go fuehrt Cleanups erst NACH
	// den defers der Testfunktion aus. Mit `defer pool.Close()` liefe die
	// Fixture-Aufraeumung unten gegen einen bereits geschlossenen Pool
	// ("closed pool") und haette nie etwas geloescht. Cleanups laufen LIFO —
	// diese Registrierung steht vor der in seedUpsertFixture, wird also zuletzt
	// ausgefuehrt: erst DELETE, dann Close.
	t.Cleanup(pool.Close)

	orgID, assetID := seedUpsertFixture(ctx, t, pool)
	repo := NewRepository(pool)

	base := Finding{
		AssetID:  assetID,
		Title:    "erster Import",
		Severity: "high",
		Status:   "open",
		Scanner:  "trivy",
		RawID:    "W0E-RAW-1",
		Sources:  []string{"sarif"},
	}

	// 1) Erster Upsert — ohne den Fix stirbt bereits dieser Aufruf mit 42P10.
	first, err := repo.UpsertImportedFinding(ctx, orgID, base)
	require.NoError(t, err, "erster Upsert muss gegen das echte Schema durchlaufen (42P10 = partieller Index nicht als Arbiter inferierbar)")
	require.Equal(t, 1, first.OccurrenceCount)

	// 2) Zweiter Upsert mit demselben raw_id — muss dieselbe Zeile treffen.
	second := base
	second.Title = "zweiter Import"
	updated, err := repo.UpsertImportedFinding(ctx, orgID, second)
	require.NoError(t, err)
	require.Equal(t, first.ID, updated.ID, "gleicher raw_id muss dieselbe Zeile aktualisieren, keine zweite anlegen")
	require.Equal(t, 2, updated.OccurrenceCount, "occurrence_count muss beim Konflikt hochgezaehlt werden")
	require.Equal(t, "zweiter Import", updated.Title)

	var withRaw int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM vb_findings WHERE org_id = $1 AND raw_id = $2`,
		orgID, base.RawID).Scan(&withRaw))
	require.Equal(t, 1, withRaw, "zwei Upserts mit gleichem raw_id duerfen genau eine Zeile hinterlassen")

	// 3) Zwei Findings ohne raw_id: RawID "" wird von spOptText zu NULL. Der
	//    partielle Index deckt NULL bewusst nicht ab — beide muessen bestehen
	//    bleiben, sonst haelt eine Organisation pro Scanner nur genau einen
	//    raw_id-losen Fund (dieselbe Klasse wie Migration 243).
	noRaw := base
	noRaw.RawID = ""
	noRaw.Scanner = "wazuh"
	noRaw.Title = "ohne raw_id A"
	a, err := repo.UpsertImportedFinding(ctx, orgID, noRaw)
	require.NoError(t, err)
	noRaw.Title = "ohne raw_id B"
	b, err := repo.UpsertImportedFinding(ctx, orgID, noRaw)
	require.NoError(t, err)
	require.NotEqual(t, a.ID, b.ID, "raw_id = NULL darf nicht dedupliziert werden — der Index ist genau deshalb partiell")

	var nullRaw int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM vb_findings WHERE org_id = $1 AND raw_id IS NULL`,
		orgID).Scan(&nullRaw))
	require.Equal(t, 2, nullRaw, "zwei raw_id-lose Funde muessen zwei Zeilen ergeben")

	// Gegenprobe zur Korrektur an SA-26: spOptText("") schreibt NULL, keinen
	// Leerstring. Ein Leerstring wuerde vom partiellen Index erfasst und die
	// beiden Zeilen oben zu einer zusammenfallen lassen.
	var emptyRaw int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM vb_findings WHERE org_id = $1 AND raw_id = ''`,
		orgID).Scan(&emptyRaw))
	require.Equal(t, 0, emptyRaw, "leerer RawID muss als NULL landen, nicht als ''")
}

// seedUpsertFixture legt eine eigene Organisation samt Asset an und loescht die
// Organisation am Testende wieder; Asset und Findings gehen per ON DELETE
// CASCADE mit. Eigene Org statt fester UUID, damit parallele Laeufe sich nicht
// die Zeilen wegzaehlen.
//
// Der Fehler der Aufraeumung wird ausgewertet, nicht verworfen, und die Zahl der
// geloeschten Zeilen wird eingefordert: eine Aufraeumung, die still nichts tut,
// laesst bei jedem CI-Lauf eine weitere Fixture-Org liegen und faellt sonst
// niemandem auf — genau so ist die erste Fassung dieses Tests durchgerutscht
// (`defer pool.Close()` schloss den Pool vor dem Cleanup, `_, _ = pool.Exec`
// schluckte das "closed pool"; gemessen: zwei Laeufe, zwei Leichen).
func seedUpsertFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) (orgID, assetID string) {
	t.Helper()

	orgID = uuid.NewString()
	slug := fmt.Sprintf("w0e-upsert-%s", orgID[:8])
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		orgID, "W0E Upsert Fixture", slug)
	require.NoError(t, err, "Fixture-Organisation anlegen")
	t.Cleanup(func() {
		tag, err := pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		if err != nil {
			t.Errorf("Fixture-Aufraeumung fehlgeschlagen, Org %s bleibt in der DB liegen: %v", orgID, err)
			return
		}
		if n := tag.RowsAffected(); n != 1 {
			t.Errorf("Fixture-Aufraeumung hat %d statt 1 Organisation geloescht (Org %s)", n, orgID)
		}
	})

	assetID = uuid.NewString()
	_, err = pool.Exec(ctx,
		`INSERT INTO vb_assets (id, org_id, name, type) VALUES ($1, $2, $3, 'server')`,
		assetID, orgID, "w0e-upsert-asset")
	require.NoError(t, err, "Fixture-Asset anlegen")

	return orgID, assetID
}
